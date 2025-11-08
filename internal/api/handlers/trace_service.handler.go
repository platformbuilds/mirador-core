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

// TraceServiceHandler provides API endpoints for trace service definitions.
// This handler implements separate trace service APIs as defined in the API contract.
type TraceServiceHandler struct {
	repo   repo.SchemaStore
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewTraceServiceHandler creates a new trace service handler
func NewTraceServiceHandler(r repo.SchemaStore, cache cache.ValkeyCluster, l logger.Logger) *TraceServiceHandler {
	return &TraceServiceHandler{
		repo:   r,
		cache:  cache,
		logger: l,
	}
}

// CreateOrUpdateTraceService creates or updates a trace service definition
func (h *TraceServiceHandler) CreateOrUpdateTraceService(c *gin.Context) {
	var req models.TraceServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.TraceService == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "traceService is required"})
		return
	}

	traceService := req.TraceService
	if traceService.TenantID == "" {
		traceService.TenantID = c.GetString("tenant_id")
	}

	// Convert TraceService to SchemaDefinition
	schemaDef := &models.SchemaDefinition{
		ID:        traceService.Service, // Use service name as ID
		Name:      traceService.Service,
		Type:      models.SchemaTypeTraceService,
		TenantID:  traceService.TenantID,
		Category:  traceService.Category,
		Sentiment: traceService.Sentiment,
		Author:    traceService.Author,
		Tags:      traceService.Tags,
		Extensions: models.SchemaExtensions{
			Trace: &models.TraceExtension{
				Service:        traceService.Service,
				ServicePurpose: traceService.ServicePurpose,
				Owner:          traceService.Owner,
			},
		},
	}

	err := h.repo.UpsertSchemaAsKPI(context.Background(), schemaDef, traceService.Author)
	if err != nil {
		h.logger.Error("trace service upsert failed", "error", err, "service", traceService.Service)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert trace service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": traceService.Service})
}

// GetTraceService retrieves a trace service definition by service name
func (h *TraceServiceHandler) GetTraceService(c *gin.Context) {
	serviceName := c.Param("service")
	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service name is required"})
		return
	}

	tenantID := c.GetString("tenant_id")

	schemaDef, err := h.repo.GetSchemaAsKPI(context.Background(), tenantID, "trace_service", serviceName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "trace service not found"})
		} else {
			h.logger.Error("trace service get failed", "error", err, "service", serviceName)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trace service"})
		}
		return
	}

	// Convert SchemaDefinition back to TraceService
	traceService := &models.TraceService{
		TenantID:       schemaDef.TenantID,
		Service:        schemaDef.Name,
		ServicePurpose: schemaDef.Extensions.Trace.ServicePurpose,
		Owner:          schemaDef.Extensions.Trace.Owner,
		Tags:           schemaDef.Tags,
		Category:       schemaDef.Category,
		Sentiment:      schemaDef.Sentiment,
		Author:         schemaDef.Author,
		UpdatedAt:      schemaDef.UpdatedAt,
	}

	c.JSON(http.StatusOK, models.TraceServiceResponse{TraceService: traceService})
}

// ListTraceServices lists trace service definitions with optional filtering
func (h *TraceServiceHandler) ListTraceServices(c *gin.Context) {
	var req models.TraceServiceListRequest
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

	schemaDefs, total, err := h.repo.ListSchemasAsKPIs(context.Background(), req.TenantID, "trace_service", req.Limit, req.Offset)
	if err != nil {
		h.logger.Error("trace service list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list trace services"})
		return
	}

	// Convert SchemaDefinitions back to TraceServices
	traceServices := make([]*models.TraceService, len(schemaDefs))
	for i, schemaDef := range schemaDefs {
		traceServices[i] = &models.TraceService{
			TenantID:       schemaDef.TenantID,
			Service:        schemaDef.Name,
			ServicePurpose: schemaDef.Extensions.Trace.ServicePurpose,
			Owner:          schemaDef.Extensions.Trace.Owner,
			Tags:           schemaDef.Tags,
			Category:       schemaDef.Category,
			Sentiment:      schemaDef.Sentiment,
			Author:         schemaDef.Author,
			UpdatedAt:      schemaDef.UpdatedAt,
		}
	}

	nextOffset := req.Offset + len(traceServices)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.TraceServiceListResponse{
		TraceServices: traceServices,
		Total:         total,
		NextOffset:    nextOffset,
	})
}

// DeleteTraceService deletes a trace service definition by service name
func (h *TraceServiceHandler) DeleteTraceService(c *gin.Context) {
	serviceName := c.Param("service")
	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service name is required"})
		return
	}

	tenantID := c.GetString("tenant_id")

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	err := h.repo.DeleteSchemaAsKPI(context.Background(), tenantID, "trace_service", serviceName)
	if err != nil {
		h.logger.Error("trace service delete failed", "error", err, "service", serviceName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete trace service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
