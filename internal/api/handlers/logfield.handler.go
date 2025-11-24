package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/logging"
	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// LogFieldHandler provides API endpoints for log field definitions.
// This handler implements separate log field APIs as defined in the API contract.
type LogFieldHandler struct {
	repo    repo.SchemaStore
	kpiRepo repo.KPIRepo
	cache   cache.ValkeyCluster
	logger  logging.Logger
}

// NewLogFieldHandler creates a new log field handler
func NewLogFieldHandler(r repo.SchemaStore, kpirepo repo.KPIRepo, cache cache.ValkeyCluster, l corelogger.Logger) *LogFieldHandler {
	return &LogFieldHandler{
		repo:    r,
		kpiRepo: kpirepo,
		cache:   cache,
		logger:  logging.FromCoreLogger(l),
	}
}

// CreateOrUpdateLogField creates or updates a log field definition
func (h *LogFieldHandler) CreateOrUpdateLogField(c *gin.Context) {
	var req models.LogFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.LogField == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "log field is required"})
		return
	}

	logField := req.LogField

	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for log field handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	kpi := &models.KPIDefinition{
		Name:       logField.Field,
		Kind:       "tech",
		Tags:       append(logField.Tags, string(models.SchemaTypeLogField)),
		Category:   logField.Category,
		Sentiment:  logField.Sentiment,
		Definition: logField.Description,
		CreatedAt:  logField.UpdatedAt,
		UpdatedAt:  logField.UpdatedAt,
	}

	if kpi.ID == "" {
		id, err := services.GenerateDeterministicKPIID(kpi)
		if err == nil {
			kpi.ID = id
		}
	}

	if _, _, err := h.kpiRepo.CreateKPI(context.Background(), kpi); err != nil {
		h.logger.Error("log field create failed", "error", err, "field", logField.Field)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create log field"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "field": logField.Field})
}

// GetLogField retrieves a log field definition by name
func (h *LogFieldHandler) GetLogField(c *gin.Context) {
	fieldName := c.Param("field")
	if fieldName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "field name is required"})
		return
	}

	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for log field handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	tmp := &models.KPIDefinition{Name: fieldName}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for log field", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get log field"})
		return
	}

	kdef, err := h.kpiRepo.GetKPI(context.Background(), detID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "log field not found"})
		} else {
			h.logger.Error("log field get failed", "error", err, "field", fieldName)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get log field"})
		}
		return
	}

	logField := &models.LogField{
		Field:       kdef.Name,
		Type:        "", // type not stored in KPI
		Description: kdef.Definition,
		Tags:        kdef.Tags,
		Category:    kdef.Category,
		Sentiment:   kdef.Sentiment,
		UpdatedAt:   kdef.UpdatedAt,
	}

	c.JSON(http.StatusOK, models.LogFieldResponse{LogField: logField})
}

// ListLogFields lists log field definitions with optional filtering
func (h *LogFieldHandler) ListLogFields(c *gin.Context) {
	var req models.LogFieldListRequest
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

	var logFields []*models.LogField
	var total int
	var err error
	if h.kpiRepo != nil {
		kpis, totalKpis, lerr := h.kpiRepo.ListKPIs(context.Background(), models.KPIListRequest{Tags: []string{"log_field"}, Limit: req.Limit, Offset: req.Offset})
		if lerr != nil {
			err = lerr
		} else {
			total = int(totalKpis)
			logFields = make([]*models.LogField, 0, len(kpis))
			for _, k := range kpis {
				logFields = append(logFields, &models.LogField{
					Field:     k.Name,
					Tags:      k.Tags,
					Category:  k.Category,
					Sentiment: k.Sentiment,
					UpdatedAt: k.UpdatedAt,
					// Type, Description are not present in KPIDefinition; omit for now
				})
			}
		}
	}

	if err != nil {
		h.logger.Error("log field list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list log fields"})
		return
	}

	nextOffset := req.Offset + len(logFields)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.LogFieldListResponse{
		LogFields:  logFields,
		Total:      total,
		NextOffset: nextOffset,
	})
}

// DeleteLogField deletes a log field definition by name
func (h *LogFieldHandler) DeleteLogField(c *gin.Context) {
	fieldName := c.Param("field")
	if fieldName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "field name is required"})
		return
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for log field handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	tmp := &models.KPIDefinition{Name: fieldName}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for log field delete", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete log field"})
		return
	}

	err = h.kpiRepo.DeleteKPI(c.Request.Context(), detID)
	if err != nil {
		h.logger.Error("log field delete failed", "error", err, "field", fieldName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete log field"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
