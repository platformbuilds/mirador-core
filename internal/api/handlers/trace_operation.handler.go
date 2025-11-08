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

// TraceOperationHandler provides API endpoints for trace operation definitions.
// This handler implements separate trace operation APIs as defined in the API contract.
type TraceOperationHandler struct {
	repo   repo.SchemaStore
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewTraceOperationHandler creates a new trace operation handler
func NewTraceOperationHandler(r repo.SchemaStore, cache cache.ValkeyCluster, l logger.Logger) *TraceOperationHandler {
	return &TraceOperationHandler{
		repo:   r,
		cache:  cache,
		logger: l,
	}
}

// CreateOrUpdateTraceOperation creates or updates a trace operation definition
func (h *TraceOperationHandler) CreateOrUpdateTraceOperation(c *gin.Context) {
	var req models.TraceOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.TraceOperation == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "traceOperation is required"})
		return
	}

	traceOperation := req.TraceOperation
	if traceOperation.TenantID == "" {
		traceOperation.TenantID = c.GetString("tenant_id")
	}

	// Convert TraceOperation to SchemaDefinition
	// Use composite ID "service:operation" for unique identification
	compositeID := traceOperation.Service + ":" + traceOperation.Operation
	schemaDef := &models.SchemaDefinition{
		ID:        compositeID,
		Name:      traceOperation.Operation,
		Type:      models.SchemaTypeTraceOperation,
		TenantID:  traceOperation.TenantID,
		Category:  traceOperation.Category,
		Sentiment: traceOperation.Sentiment,
		Author:    traceOperation.Author,
		Tags:      traceOperation.Tags,
		Extensions: models.SchemaExtensions{
			Trace: &models.TraceExtension{
				Service:        traceOperation.Service,
				Operation:      traceOperation.Operation,
				ServicePurpose: traceOperation.ServicePurpose,
				Owner:          traceOperation.Owner,
			},
		},
	}

	err := h.repo.UpsertSchemaAsKPI(context.Background(), schemaDef, traceOperation.Author)
	if err != nil {
		h.logger.Error("trace operation upsert failed", "error", err, "service", traceOperation.Service, "operation", traceOperation.Operation)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert trace operation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": traceOperation.Service, "operation": traceOperation.Operation})
}

// GetTraceOperation retrieves a trace operation definition by service and operation name
func (h *TraceOperationHandler) GetTraceOperation(c *gin.Context) {
	serviceName := c.Param("service")
	operationName := c.Param("operation")
	if serviceName == "" || operationName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service and operation names are required"})
		return
	}

	tenantID := c.GetString("tenant_id")

	compositeID := serviceName + ":" + operationName
	schemaDef, err := h.repo.GetSchemaAsKPI(context.Background(), tenantID, "trace_operation", compositeID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "trace operation not found"})
		} else {
			h.logger.Error("trace operation get failed", "error", err, "service", serviceName, "operation", operationName)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trace operation"})
		}
		return
	}

	// Convert SchemaDefinition back to TraceOperation
	traceOperation := &models.TraceOperation{
		TenantID:       schemaDef.TenantID,
		Service:        schemaDef.Extensions.Trace.Service,
		Operation:      schemaDef.Extensions.Trace.Operation,
		ServicePurpose: schemaDef.Extensions.Trace.ServicePurpose,
		Owner:          schemaDef.Extensions.Trace.Owner,
		Tags:           schemaDef.Tags,
		Category:       schemaDef.Category,
		Sentiment:      schemaDef.Sentiment,
		Author:         schemaDef.Author,
		UpdatedAt:      schemaDef.UpdatedAt,
	}

	c.JSON(http.StatusOK, models.TraceOperationResponse{TraceOperation: traceOperation})
}

// ListTraceOperations lists trace operation definitions with optional filtering
func (h *TraceOperationHandler) ListTraceOperations(c *gin.Context) {
	var req models.TraceOperationListRequest
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

	schemaDefs, total, err := h.repo.ListSchemasAsKPIs(context.Background(), req.TenantID, "trace_operation", req.Limit, req.Offset)
	if err != nil {
		h.logger.Error("trace operation list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list trace operations"})
		return
	}

	// Convert SchemaDefinitions back to TraceOperations
	traceOperations := make([]*models.TraceOperation, len(schemaDefs))
	for i, schemaDef := range schemaDefs {
		traceOperations[i] = &models.TraceOperation{
			TenantID:       schemaDef.TenantID,
			Service:        schemaDef.Extensions.Trace.Service,
			Operation:      schemaDef.Extensions.Trace.Operation,
			ServicePurpose: schemaDef.Extensions.Trace.ServicePurpose,
			Owner:          schemaDef.Extensions.Trace.Owner,
			Tags:           schemaDef.Tags,
			Category:       schemaDef.Category,
			Sentiment:      schemaDef.Sentiment,
			Author:         schemaDef.Author,
			UpdatedAt:      schemaDef.UpdatedAt,
		}
	}

	nextOffset := req.Offset + len(traceOperations)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.TraceOperationListResponse{
		TraceOperations: traceOperations,
		Total:           total,
		NextOffset:      nextOffset,
	})
}

// DeleteTraceOperation deletes a trace operation definition by service and operation name
func (h *TraceOperationHandler) DeleteTraceOperation(c *gin.Context) {
	serviceName := c.Param("service")
	operationName := c.Param("operation")
	if serviceName == "" || operationName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service and operation names are required"})
		return
	}

	tenantID := c.GetString("tenant_id")

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	compositeID := serviceName + ":" + operationName
	err := h.repo.DeleteSchemaAsKPI(context.Background(), tenantID, "trace_operation", compositeID)
	if err != nil {
		h.logger.Error("trace operation delete failed", "error", err, "service", serviceName, "operation", operationName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete trace operation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
