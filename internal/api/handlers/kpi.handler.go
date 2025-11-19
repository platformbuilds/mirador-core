package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
) // KPIRepo extends SchemaStore with KPI-specific operations
// KPIRepo interface is defined in the repo package

// KPIHandler provides API endpoints for KPI definitions, layouts, dashboards, and user preferences.
// This handler implements the separate KPI APIs as defined in the API contract.
type KPIHandler struct {
	repo   repo.KPIRepo
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewKPIHandler creates a new KPI handler
func NewKPIHandler(r repo.SchemaStore, cache cache.ValkeyCluster, l logger.Logger) *KPIHandler {
	kpiRepo, ok := r.(repo.KPIRepo)
	if !ok {
		l.Error("SchemaStore does not implement KPIRepo interface - KPI functionality will not be available")
		return nil
	}
	return &KPIHandler{
		repo:   kpiRepo,
		cache:  cache,
		logger: l,
	}
} // ------------------- KPI Definitions API -------------------

// GetKPIDefinitions retrieves all KPI definitions with optional filtering
// @Summary Get KPI definitions
// @Description Retrieve a paginated list of KPI definitions with optional filtering by tags
// @Tags kpi-definitions
// @Accept json
// @Produce json
// @Param tags query []string false "Filter by tags (comma-separated)" collectionFormat(csv)
// @Param limit query int false "Maximum number of results (default: 10)" minimum(1) maximum(100)
// @Param offset query int false "Pagination offset (default: 0)" minimum(0)
// @Success 200 {object} models.KPIListResponse
// @Failure 400 {object} map[string]string "error: invalid query parameters"
// @Failure 500 {object} map[string]string "error: failed to list KPIs"
// @Router /api/v1/kpi/defs [get]
// (no internal auth) NOTE: security removed — MIRADOR-CORE is intended to run behind an external gateway
func (h *KPIHandler) GetKPIDefinitions(c *gin.Context) {
	var req models.KPIListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	kpis, total, err := h.listKPIs(c.Request.Context(), req.Tags, req.Limit, req.Offset)
	if err != nil {
		h.logger.Error("KPI list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list KPIs"})
		return
	}

	nextOffset := req.Offset + len(kpis)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.KPIListResponse{
		KPIDefinitions: kpis,
		Total:          total,
		NextOffset:     nextOffset,
	})
}

// CreateOrUpdateKPIDefinition creates or updates a KPI definition
// @Summary Create or update KPI definition
// @Description Create a new KPI definition or update an existing one. If ID is not provided, a new UUID will be generated.
// @Tags kpi-definitions
// @Accept json
// @Produce json
// @Param kpi body models.KPIDefinitionRequest true "KPI definition payload"
// @Success 200 {object} map[string]interface{} "status: ok, id: kpi_id"
// @Failure 400 {object} map[string]string "error: invalid payload or validation error"
// @Failure 500 {object} map[string]string "error: failed to upsert KPI"
// @Router /api/v1/kpi/defs [post]
// (no internal auth)
func (h *KPIHandler) CreateOrUpdateKPIDefinition(c *gin.Context) {
	var req models.KPIDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.KPIDefinition == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kpi definition is required"})
		return
	}

	kpi := req.KPIDefinition

	// Validate sentiment field
	if kpi.Sentiment != "" && kpi.Sentiment != "NEGATIVE" && kpi.Sentiment != "POSITIVE" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sentiment must be either 'NEGATIVE' or 'POSITIVE'"})
		return
	}

	if kpi.ID == "" {
		kpi.ID = uuid.New().String()
	}

	kpi.UpdatedAt = time.Now()
	if kpi.CreatedAt.IsZero() {
		kpi.CreatedAt = kpi.UpdatedAt
	}

	err := h.upsertKPI(c.Request.Context(), kpi)
	if err != nil {
		h.logger.Error("KPI upsert failed", "error", err, "id", kpi.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert KPI"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": kpi.ID})
}

// DeleteKPIDefinition deletes a KPI definition by ID
// @Summary Delete KPI definition
// @Description Delete a KPI definition by its ID. Requires confirmation via query parameter.
// @Tags kpi-definitions
// @Accept json
// @Produce json
// @Param id path string true "KPI definition ID"
// @Param confirm query string true "Confirmation flag (1, true, or yes)" Enums(1,true,yes)
// @Success 200 {object} map[string]interface{} "status: deleted"
// @Failure 400 {object} map[string]string "error: missing id or confirmation required"
// @Failure 500 {object} map[string]string "error: failed to delete KPI"
// @Router /api/v1/kpi/defs/{id} [delete]
// (no internal auth)
func (h *KPIHandler) DeleteKPIDefinition(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "KPI id is required"})
		return
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	err := h.deleteKPI(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("KPI delete failed", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete KPI"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ------------------- Implementation methods (extracted from unified handler) -------------------

func (h *KPIHandler) upsertKPI(ctx context.Context, kpi *models.KPIDefinition) error {
	return h.repo.UpsertKPI(ctx, kpi)
}

func (h *KPIHandler) getKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	return h.repo.GetKPI(ctx, id)
}

func (h *KPIHandler) listKPIs(ctx context.Context, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error) {
	return h.repo.ListKPIs(ctx, tags, limit, offset)
}

func (h *KPIHandler) deleteKPI(ctx context.Context, id string) error {
	return h.repo.DeleteKPI(ctx, id)
}

func (h *KPIHandler) batchUpsertKPILayouts(ctx context.Context, dashboardID string, layouts map[string]interface{}) error {
	return h.repo.BatchUpsertKPILayouts(ctx, dashboardID, layouts)
}
