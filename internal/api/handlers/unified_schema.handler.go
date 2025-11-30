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

// UnifiedSchemaHandler provides a unified API for all schema types and KPIs.
// KPIs are the "new schema definitions" - all schema operations are routed through
// a single handler with type-based dispatch.
type UnifiedSchemaHandler struct {
	repo           repo.SchemaStore
	kpiRepo        repo.KPIRepo
	metricsService *services.VictoriaMetricsService
	logsService    *services.VictoriaLogsService
	cache          cache.ValkeyCluster
	logger         logger.Logger
	maxUploadBytes int64
}

// NewUnifiedSchemaHandler creates a new unified schema handler
func NewUnifiedSchemaHandler(r repo.SchemaStore, kpirepo repo.KPIRepo, ms *services.VictoriaMetricsService, ls *services.VictoriaLogsService, cache cache.ValkeyCluster, l logger.Logger, maxUploadBytes int64) *UnifiedSchemaHandler {
	if maxUploadBytes <= 0 {
		maxUploadBytes = 5 << 20
	}
	return &UnifiedSchemaHandler{
		repo:           r,
		kpiRepo:        kpirepo,
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
		def.ID = uuid.Must(uuid.NewV4()).String()
	}

	def.UpdatedAt = time.Now()
	if def.CreatedAt.IsZero() {
		def.CreatedAt = def.UpdatedAt
	}

	// Convert SchemaDefinition into KPI-backed object and persist via KPIRepo
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for unified schema handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	// Map generic schema definition into KPI model
	// Map extension-specific description into KPI.Definition where available
	var defText string
	if def.Extensions != (models.SchemaExtensions{}) {
		if def.Extensions.Metric != nil {
			defText = def.Extensions.Metric.Description
		} else if def.Extensions.LogField != nil {
			defText = def.Extensions.LogField.Description
		} else if def.Extensions.Trace != nil {
			defText = def.Extensions.Trace.ServicePurpose
		}
	}

	kpi := &models.KPIDefinition{
		Name:       def.Name,
		Kind:       "tech",
		Tags:       append(def.Tags, string(schemaType)),
		Category:   def.Category,
		Sentiment:  def.Sentiment,
		Definition: defText,
		CreatedAt:  def.CreatedAt,
		UpdatedAt:  def.UpdatedAt,
	}
	// For trace operation type, populate namespace from extension if available
	if schemaType == models.SchemaTypeTraceOperation && def.Extensions.Trace != nil {
		if def.Extensions.Trace.Service != "" {
			kpi.Namespace = def.Extensions.Trace.Service
		}
		if def.Extensions.Trace.Operation != "" {
			kpi.Name = def.Extensions.Trace.Operation
		}
		kpi.Definition = def.Extensions.Trace.ServicePurpose
	}

	if kpi.ID == "" {
		id, err := services.GenerateDeterministicKPIID(kpi)
		if err == nil {
			kpi.ID = id
		}
	}

	if _, _, err := h.kpiRepo.CreateKPI(c.Request.Context(), kpi); err != nil {
		h.logger.Error("schema upsert failed", "error", err, "type", schemaType)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upsert failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": kpi.ID})
}

// GetSchemaDefinition retrieves a schema definition by ID and type
func (h *UnifiedSchemaHandler) GetSchemaDefinition(c *gin.Context) {
	schemaType := models.SchemaType(c.Param("type"))
	id := c.Param("id")

	if schemaType == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "schema type and id are required"})
		return
	}

	// For trace operation, id may be in format service:operation
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for unified schema handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	var detID string
	if schemaType == models.SchemaTypeTraceOperation && strings.Contains(id, ":") {
		parts := strings.SplitN(id, ":", 2)
		tmp := &models.KPIDefinition{Name: parts[1], Namespace: parts[0]}
		pid, _ := services.GenerateDeterministicKPIID(tmp)
		detID = pid
	} else {
		detID = id
	}

	kdef, err := h.kpiRepo.GetKPI(c.Request.Context(), detID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			h.logger.Error("schema get failed", "error", err, "type", schemaType, "id", id)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get failed"})
		}
		return
	}

	// Map KPIDefinition back to SchemaDefinition for response
	c.JSON(http.StatusOK, models.SchemaDefinitionResponse{SchemaDefinition: kpiToSchemaDefinition(kdef, schemaType)})
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
		kpis, totalKpis, lerr := kpirepo.ListKPIs(c.Request.Context(), models.KPIListRequest{Tags: []string{string(schemaType)}, Limit: req.Limit, Offset: req.Offset})
		if lerr != nil {
			err = lerr
		} else {
			total = int(totalKpis)
			definitions = make([]*models.SchemaDefinition, 0, len(kpis))
			for _, k := range kpis {
				definitions = append(definitions, kpiToSchemaDefinition(k, schemaType))
			}
		}
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
		// Labels remain stored in Label class; delete via repo
		err = h.repo.DeleteLabel(c.Request.Context(), id)
	case models.SchemaTypeMetric:
		err = h.repo.DeleteMetric(c.Request.Context(), id)
	case models.SchemaTypeLogField:
		// Migrate: delete KPI-backed log field via KPIRepo
		if h.kpiRepo == nil {
			h.logger.Error("KPIRepo not configured for unified schema handler")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
			return
		}
		tmp := &models.KPIDefinition{Name: id}
		detID, _ := services.GenerateDeterministicKPIID(tmp)
		_, err = h.kpiRepo.DeleteKPI(c.Request.Context(), detID)
	case models.SchemaTypeTraceService:
		err = h.repo.DeleteTraceService(c.Request.Context(), id)
	case models.SchemaTypeTraceOperation:
		// For trace operations, id is expected to be "service:operation"
		parts := strings.Split(id, ":")
		if len(parts) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "trace operation id must be in format 'service:operation'"})
			return
		}
		if h.kpiRepo == nil {
			h.logger.Error("KPIRepo not configured for unified schema handler")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
			return
		}
		tmp := &models.KPIDefinition{Name: parts[1], Namespace: parts[0]}
		detID, _ := services.GenerateDeterministicKPIID(tmp)
		_, err = h.kpiRepo.DeleteKPI(c.Request.Context(), detID)
	case models.SchemaTypeKPI:
		if h.kpiRepo == nil {
			h.logger.Error("KPIRepo not configured for unified schema handler")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
			return
		}
		_, err = h.kpiRepo.DeleteKPI(c.Request.Context(), id)
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
		models.SchemaTypeTraceService, models.SchemaTypeTraceOperation, models.SchemaTypeKPI:
		return true
	default:
		return false
	}
}
