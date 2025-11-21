package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"

	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// LabelHandler provides API endpoints for label definitions.
// This handler implements separate label APIs as defined in the API contract.
type LabelHandler struct {
	repo    repo.SchemaStore
	kpiRepo repo.KPIRepo
	cache   cache.ValkeyCluster
	logger  logger.Logger
}

// NewLabelHandler creates a new label handler
func NewLabelHandler(r repo.SchemaStore, kpirepo repo.KPIRepo, cache cache.ValkeyCluster, l logger.Logger) *LabelHandler {
	return &LabelHandler{
		repo:    r,
		kpiRepo: kpirepo,
		cache:   cache,
		logger:  l,
	}
}

// CreateOrUpdateLabel creates or updates a label definition
func (h *LabelHandler) CreateOrUpdateLabel(c *gin.Context) {
	// Convert LabelRequest to SchemaDefinitionRequest
	var req models.LabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label name is required"})
		return
	}

	// Convert to unified schema definition
	schemaDef := &models.SchemaDefinition{
		Name:      req.Name,
		Type:      models.SchemaTypeLabel,
		Author:    req.Author,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Extensions: models.SchemaExtensions{
			Label: &models.LabelExtension{
				Type:        req.Type,
				Required:    req.Required,
				AllowedVals: req.AllowedValues,
				Description: req.Description,
			},
		},
	}

	if schemaDef.ID == "" {
		schemaDef.ID = uuid.Must(uuid.NewV4()).String()
	}

	// Convert to KPI and persist via KPIRepo
	kpi := &models.KPIDefinition{
		Name:      req.Name,
		Kind:      "tech",
		Tags:      schemaDef.Tags,
		Category:  schemaDef.Category,
		Sentiment: schemaDef.Sentiment,
		CreatedAt: schemaDef.CreatedAt,
		UpdatedAt: schemaDef.UpdatedAt,
	}
	if kpi.ID == "" {
		id, err := services.GenerateDeterministicKPIID(kpi)
		if err == nil {
			kpi.ID = id
		}
	}
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not available for label handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "kpi repo unavailable"})
		return
	}
	if _, _, err := h.kpiRepo.CreateKPI(c.Request.Context(), kpi); err != nil {
		h.logger.Error("label create failed", "error", err, "name", req.Name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create label"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "name": req.Name})
}

// GetLabel retrieves a label definition by name
func (h *LabelHandler) GetLabel(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label name is required"})
		return
	}

	// Retrieve KPI by deterministic id
	tmp := &models.KPIDefinition{Name: name}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for label", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get label"})
		return
	}
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not available for label handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "kpi repo unavailable"})
		return
	}
	schemaDefKPI, err := h.kpiRepo.GetKPI(c.Request.Context(), detID)
	if err != nil {
		h.logger.Error("label get failed", "error", err, "name", name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get label"})
		return
	}
	if schemaDefKPI == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "label not found"})
		return
	}

	// Convert SchemaDefinition back to Label for backward compatibility
	label := &models.Label{
		Name:      schemaDefKPI.Name,
		Category:  schemaDefKPI.Category,
		Sentiment: schemaDefKPI.Sentiment,
		UpdatedAt: schemaDefKPI.UpdatedAt,
	}

	c.JSON(http.StatusOK, models.LabelResponse{Label: label})
}

// ListLabels lists label definitions with optional filtering
func (h *LabelHandler) ListLabels(c *gin.Context) {
	var req models.LabelListRequest
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

	// Use KPIRepo to list KPI-backed schema types (labels are stored as KPIs)
	var labels []*models.Label
	var total int
	var err error

	if kpirepo, ok := h.repo.(repo.KPIRepo); ok {
		kpis, totalKpis, lerr := kpirepo.ListKPIs(c.Request.Context(), models.KPIListRequest{Tags: []string{string(models.SchemaTypeLabel)}, Limit: req.Limit, Offset: req.Offset})
		if lerr != nil {
			err = lerr
		} else {
			total = int(totalKpis)
			labels = make([]*models.Label, 0, len(kpis))
			for _, k := range kpis {
				labels = append(labels, &models.Label{
					Name:      k.Name,
					Category:  k.Category,
					Sentiment: k.Sentiment,
					UpdatedAt: k.UpdatedAt,
					// Type, Required, AllowedVals, Description are not present in KPIDefinition; omit for now
				})
			}
		}
	}

	if err != nil {
		h.logger.Error("label list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}

	nextOffset := req.Offset + len(labels)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.LabelListResponse{
		Labels:     labels,
		Total:      total,
		NextOffset: nextOffset,
	})
}

// DeleteLabel deletes a label definition by name
func (h *LabelHandler) DeleteLabel(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label name is required"})
		return
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	// Delete the label
	err := h.repo.DeleteLabel(c.Request.Context(), name)
	if err != nil {
		h.logger.Error("label delete failed", "error", err, "name", name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete label"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
