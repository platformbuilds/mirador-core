package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// LogFieldHandler provides API endpoints for log field definitions.
// This handler implements separate log field APIs as defined in the API contract.
type LogFieldHandler struct {
	repo   repo.SchemaStore
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewLogFieldHandler creates a new log field handler
func NewLogFieldHandler(r repo.SchemaStore, cache cache.ValkeyCluster, l logger.Logger) *LogFieldHandler {
	return &LogFieldHandler{
		repo:   r,
		cache:  cache,
		logger: l,
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

	// Convert LogField to SchemaDefinition
	schemaDef := &models.SchemaDefinition{
		ID:        logField.Field, // Use field name as ID
		Name:      logField.Field,
		Type:      models.SchemaTypeLogField,
		Category:  logField.Category,
		Sentiment: logField.Sentiment,
		Author:    logField.Author,
		Tags:      logField.Tags,
		Extensions: models.SchemaExtensions{
			LogField: &models.LogFieldExtension{
				FieldType:   logField.Type,
				Description: logField.Description,
			},
		},
	}

	err := h.repo.UpsertSchemaAsKPI(context.Background(), schemaDef, logField.Author)
	if err != nil {
		h.logger.Error("log field upsert failed", "error", err, "field", logField.Field)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert log field"})
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

	schemaDef, err := h.repo.GetSchemaAsKPI(context.Background(), "log_field", fieldName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "log field not found"})
		} else {
			h.logger.Error("log field get failed", "error", err, "field", fieldName)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get log field"})
		}
		return
	}

	// Convert SchemaDefinition back to LogField
	logField := &models.LogField{
		Field:       schemaDef.Name,
		Type:        schemaDef.Extensions.LogField.FieldType,
		Description: schemaDef.Extensions.LogField.Description,
		Tags:        schemaDef.Tags,
		Category:    schemaDef.Category,
		Sentiment:   schemaDef.Sentiment,
		Author:      schemaDef.Author,
		UpdatedAt:   schemaDef.UpdatedAt,
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

	var schemaDefs []*models.SchemaDefinition
	var total int
	var err error
	if kpirepo, ok := h.repo.(repo.KPIRepo); ok {
		kpis, totalKpis, lerr := kpirepo.ListKPIs(context.Background(), []string{"log_field"}, req.Limit, req.Offset)
		if lerr != nil {
			err = lerr
		} else {
			total = totalKpis
			schemaDefs = make([]*models.SchemaDefinition, 0, len(kpis))
			for _, k := range kpis {
				schemaDefs = append(schemaDefs, kpiToSchemaDefinition(k, models.SchemaTypeLogField))
			}
		}
	} else {
		schemaDefs, total, err = h.repo.ListSchemasAsKPIs(context.Background(), "log_field", req.Limit, req.Offset)
	}
	if err != nil {
		h.logger.Error("log field list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list log fields"})
		return
	}

	// Convert SchemaDefinitions back to LogFields
	logFields := make([]*models.LogField, len(schemaDefs))
	for i, schemaDef := range schemaDefs {
		logFields[i] = &models.LogField{
			Field:       schemaDef.Name,
			Type:        schemaDef.Extensions.LogField.FieldType,
			Description: schemaDef.Extensions.LogField.Description,
			Tags:        schemaDef.Tags,
			Category:    schemaDef.Category,
			Sentiment:   schemaDef.Sentiment,
			Author:      schemaDef.Author,
			UpdatedAt:   schemaDef.UpdatedAt,
		}
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

	err := h.repo.DeleteLogField(c.Request.Context(), fieldName)
	if err != nil {
		h.logger.Error("log field delete failed", "error", err, "field", fieldName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete log field"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
