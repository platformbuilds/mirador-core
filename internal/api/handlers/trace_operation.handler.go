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

// TraceOperationHandler provides API endpoints for trace operation definitions.
// This handler implements separate trace operation APIs as defined in the API contract.
type TraceOperationHandler struct {
	repo    repo.SchemaStore
	kpiRepo repo.KPIRepo
	cache   cache.ValkeyCluster
	logger  logging.Logger
}

// NewTraceOperationHandler creates a new trace operation handler
func NewTraceOperationHandler(r repo.SchemaStore, kpirepo repo.KPIRepo, cache cache.ValkeyCluster, l corelogger.Logger) *TraceOperationHandler {
	return &TraceOperationHandler{
		repo:    r,
		kpiRepo: kpirepo,
		cache:   cache,
		logger:  logging.FromCoreLogger(l),
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

	// Convert TraceOperation into a KPI-backed object and persist via KPIRepo
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for trace operation handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	kpi := &models.KPIDefinition{
		Name:      traceOperation.Operation,
		Namespace: traceOperation.Service,
		Kind:      "tech",
		Tags:      traceOperation.Tags,
		Category:  traceOperation.Category,
		Sentiment: traceOperation.Sentiment,
		CreatedAt: traceOperation.UpdatedAt,
		UpdatedAt: traceOperation.UpdatedAt,
	}

	if kpi.ID == "" {
		id, err := services.GenerateDeterministicKPIID(kpi)
		if err == nil {
			kpi.ID = id
		}
	}

	if _, _, err := h.kpiRepo.CreateKPI(context.Background(), kpi); err != nil {
		h.logger.Error("trace operation create failed", "error", err, "service", traceOperation.Service, "operation", traceOperation.Operation)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create trace operation"})
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

	// Build deterministic KPI ID from service+operation and fetch via KPIRepo
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for trace operation handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	tmp := &models.KPIDefinition{Name: operationName, Namespace: serviceName}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for trace operation", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trace operation"})
		return
	}

	kdef, err := h.kpiRepo.GetKPI(context.Background(), detID)
	if err != nil {
		h.logger.Error("trace operation get failed", "error", err, "service", serviceName, "operation", operationName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trace operation"})
		return
	}
	if kdef == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trace operation not found"})
		return
	}

	traceOperation := &models.TraceOperation{
		Service:        kdef.Namespace,
		Operation:      kdef.Name,
		ServicePurpose: kdef.Definition,
		Tags:           kdef.Tags,
		Category:       kdef.Category,
		Sentiment:      kdef.Sentiment,
		UpdatedAt:      kdef.UpdatedAt,
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

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	var traceOps []*models.TraceOperation
	var total int
	var err error
	if h.kpiRepo != nil {
		kpis, totalKpis, lerr := h.kpiRepo.ListKPIs(context.Background(), models.KPIListRequest{Tags: []string{"trace_operation"}, Limit: req.Limit, Offset: req.Offset})
		if lerr != nil {
			err = lerr
		} else {
			total = int(totalKpis)
			traceOps = make([]*models.TraceOperation, 0, len(kpis))
			for _, k := range kpis {
				traceOps = append(traceOps, &models.TraceOperation{
					Operation: k.Name,
					Service:   k.Namespace,
					Tags:      k.Tags,
					Category:  k.Category,
					Sentiment: k.Sentiment,
					UpdatedAt: k.UpdatedAt,
				})
			}
		}
	}

	if err != nil {
		h.logger.Error("trace operation list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list trace operations"})
		return
	}

	nextOffset := req.Offset + len(traceOps)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.TraceOperationListResponse{
		TraceOperations: traceOps,
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

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for trace operation handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	tmp := &models.KPIDefinition{Name: operationName, Namespace: serviceName}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for trace operation delete", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete trace operation"})
		return
	}

	_, err = h.kpiRepo.DeleteKPI(c.Request.Context(), detID)
	if err != nil {
		h.logger.Error("trace operation delete failed", "error", err, "service", serviceName, "operation", operationName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete trace operation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
