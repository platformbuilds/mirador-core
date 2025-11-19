package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// UnifiedSchemaHandler provides a unified API for all schema types and KPIs.
// KPIs are the "new schema definitions" - all schema operations are routed through
// a single handler with type-based dispatch.
type UnifiedSchemaHandler struct {
	repo           repo.SchemaStore
	metricsService *services.VictoriaMetricsService
	logsService    *services.VictoriaLogsService
	cache          cache.ValkeyCluster
	logger         logger.Logger
	maxUploadBytes int64
}

// NewUnifiedSchemaHandler creates a new unified schema handler
func NewUnifiedSchemaHandler(r repo.SchemaStore, ms *services.VictoriaMetricsService, ls *services.VictoriaLogsService, cache cache.ValkeyCluster, l logger.Logger, maxUploadBytes int64) *UnifiedSchemaHandler {
	if maxUploadBytes <= 0 {
		maxUploadBytes = 5 << 20
	}
	return &UnifiedSchemaHandler{
		repo:           r,
		metricsService: ms,
		logsService:    ls,
		cache:          cache,
		logger:         l,
		maxUploadBytes: maxUploadBytes,
	}
}

// ------------------- Unified Schema CRUD Operations -------------------

// UpsertSchemaDefinition creates or updates any type of schema definition
func (h *UnifiedSchemaHandler) UpsertSchemaDefinition(c *gin.Context) {
	schemaType := models.SchemaType(c.Param("type"))
	if schemaType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "schema type is required"})
		return
	}

	if !isValidSchemaType(schemaType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported schema type"})
		return
	}

	if !isValidSchemaType(schemaType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid schema type"})
		return
	}

	var req models.SchemaDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.SchemaDefinition == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "schema definition is required"})
		return
	}

	def := req.SchemaDefinition
	def.Type = schemaType

	if def.ID == "" {
		def.ID = uuid.New().String()
	}

	def.UpdatedAt = time.Now()
	if def.CreatedAt.IsZero() {
		def.CreatedAt = def.UpdatedAt
	}

	// Use unified KPI-based storage for all schema types
	err := h.repo.UpsertSchemaAsKPI(c.Request.Context(), def, def.Author)
	if err != nil {
		h.logger.Error("schema upsert failed", "error", err, "type", schemaType)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upsert failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": def.ID})
}

// GetSchemaDefinition retrieves a schema definition by ID and type
func (h *UnifiedSchemaHandler) GetSchemaDefinition(c *gin.Context) {
	schemaType := models.SchemaType(c.Param("type"))
	id := c.Param("id")

	if schemaType == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "schema type and id are required"})
		return
	}

	def, err := h.repo.GetSchemaAsKPI(c.Request.Context(), string(schemaType), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			h.logger.Error("schema get failed", "error", err, "type", schemaType, "id", id)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get failed"})
		}
		return
	}

	c.JSON(http.StatusOK, models.SchemaDefinitionResponse{SchemaDefinition: def})
}

// ListSchemaDefinitions lists schema definitions with optional filtering
func (h *UnifiedSchemaHandler) ListSchemaDefinitions(c *gin.Context) {
	schemaType := models.SchemaType(c.Param("type"))

	var req models.SchemaListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}
	// Use KPIRepo listing for unified schema types when available
	var definitions []*models.SchemaDefinition
	var total int
	var err error
	if kpirepo, ok := h.repo.(repo.KPIRepo); ok {
		kpis, totalKpis, lerr := kpirepo.ListKPIs(c.Request.Context(), []string{string(schemaType)}, req.Limit, req.Offset)
		if lerr != nil {
			err = lerr
		} else {
			total = totalKpis
			definitions = make([]*models.SchemaDefinition, 0, len(kpis))
			for _, k := range kpis {
				definitions = append(definitions, kpiToSchemaDefinition(k, schemaType))
			}
		}
	} else {
		definitions, total, err = h.repo.ListSchemasAsKPIs(c.Request.Context(), string(schemaType), req.Limit, req.Offset)
	}
	if err != nil {
		h.logger.Error("schema list failed", "error", err, "type", schemaType)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}

	nextOffset := req.Offset + len(definitions)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.SchemaListResponse{
		SchemaDefinitions: definitions,
		Total:             total,
		NextOffset:        nextOffset,
	})
}

// DeleteSchemaDefinition deletes a schema definition by ID and type
func (h *UnifiedSchemaHandler) DeleteSchemaDefinition(c *gin.Context) {
	schemaType := models.SchemaType(c.Param("type"))
	id := c.Param("id")

	if schemaType == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "schema type and id are required"})
		return
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	// Delete based on schema type
	var err error
	switch schemaType {
	case models.SchemaTypeLabel:
		err = h.repo.DeleteLabel(c.Request.Context(), id)
	case models.SchemaTypeMetric:
		err = h.repo.DeleteMetric(c.Request.Context(), id)
	case models.SchemaTypeLogField:
		err = h.repo.DeleteLogField(c.Request.Context(), id)
	case models.SchemaTypeTraceService:
		err = h.repo.DeleteTraceService(c.Request.Context(), id)
	case models.SchemaTypeTraceOperation:
		// For trace operations, id is expected to be "service:operation"
		parts := strings.Split(id, ":")
		if len(parts) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "trace operation id must be in format 'service:operation'"})
			return
		}
		err = h.repo.DeleteTraceOperation(c.Request.Context(), parts[0], parts[1])
	case models.SchemaTypeKPI:
		err = h.repo.DeleteSchemaAsKPI(c.Request.Context(), id)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported schema type"})
		return
	}

	if err != nil {
		h.logger.Error("schema delete failed", "error", err, "type", schemaType, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// isValidSchemaType checks if the given schema type is supported
func isValidSchemaType(schemaType models.SchemaType) bool {
	switch schemaType {
	case models.SchemaTypeLabel, models.SchemaTypeMetric, models.SchemaTypeLogField,
		models.SchemaTypeTraceService, models.SchemaTypeTraceOperation, models.SchemaTypeKPI,
		models.SchemaTypeDashboard, models.SchemaTypeLayout, models.SchemaTypeUserPreferences:
		return true
	default:
		return false
	}
}
