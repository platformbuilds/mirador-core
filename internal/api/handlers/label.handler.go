package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// LabelHandler provides API endpoints for label definitions.
// This handler implements separate label APIs as defined in the API contract.
type LabelHandler struct {
	repo   repo.SchemaStore
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewLabelHandler creates a new label handler
func NewLabelHandler(r repo.SchemaStore, cache cache.ValkeyCluster, l logger.Logger) *LabelHandler {
	return &LabelHandler{
		repo:   r,
		cache:  cache,
		logger: l,
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
		TenantID:  req.TenantID,
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

	if schemaDef.TenantID == "" {
		schemaDef.TenantID = c.GetString("tenant_id")
	}

	if schemaDef.ID == "" {
		schemaDef.ID = uuid.New().String()
	}

	// Use unified KPI-based storage
	err := h.repo.UpsertSchemaAsKPI(c.Request.Context(), schemaDef, schemaDef.Author)
	if err != nil {
		h.logger.Error("label upsert failed", "error", err, "name", req.Name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert label"})
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

	tenantID := c.GetString("tenant_id")

	// Use unified KPI-based retrieval
	schemaDef, err := h.repo.GetSchemaAsKPI(c.Request.Context(), tenantID, string(models.SchemaTypeLabel), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "label not found"})
		} else {
			h.logger.Error("label get failed", "error", err, "name", name)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get label"})
		}
		return
	}

	// Convert SchemaDefinition back to Label for backward compatibility
	label := &models.Label{
		TenantID:  schemaDef.TenantID,
		Name:      schemaDef.Name,
		Category:  schemaDef.Category,
		Sentiment: schemaDef.Sentiment,
		Author:    schemaDef.Author,
		UpdatedAt: schemaDef.UpdatedAt,
	}

	if schemaDef.Extensions.Label != nil {
		label.Type = schemaDef.Extensions.Label.Type
		label.Required = schemaDef.Extensions.Label.Required
		label.AllowedVals = schemaDef.Extensions.Label.AllowedVals
		label.Description = schemaDef.Extensions.Label.Description
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

	if req.TenantID == "" {
		req.TenantID = c.GetString("tenant_id")
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	// Use unified KPI-based listing
	schemaDefs, total, err := h.repo.ListSchemasAsKPIs(c.Request.Context(), req.TenantID, string(models.SchemaTypeLabel), req.Limit, req.Offset)
	if err != nil {
		h.logger.Error("label list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}

	// Convert SchemaDefinitions back to Labels for backward compatibility
	labels := make([]*models.Label, 0, len(schemaDefs))
	for _, schemaDef := range schemaDefs {
		label := &models.Label{
			TenantID:  schemaDef.TenantID,
			Name:      schemaDef.Name,
			Category:  schemaDef.Category,
			Sentiment: schemaDef.Sentiment,
			Author:    schemaDef.Author,
			UpdatedAt: schemaDef.UpdatedAt,
		}

		if schemaDef.Extensions.Label != nil {
			label.Type = schemaDef.Extensions.Label.Type
			label.Required = schemaDef.Extensions.Label.Required
			label.AllowedVals = schemaDef.Extensions.Label.AllowedVals
			label.Description = schemaDef.Extensions.Label.Description
		}

		labels = append(labels, label)
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

	tenantID := c.GetString("tenant_id")

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	// Use unified KPI-based deletion
	err := h.repo.DeleteSchemaAsKPI(c.Request.Context(), tenantID, string(models.SchemaTypeLabel), name)
	if err != nil {
		h.logger.Error("label delete failed", "error", err, "name", name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete label"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
