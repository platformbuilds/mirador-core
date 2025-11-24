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

// TraceServiceHandler provides API endpoints for trace service definitions.
// This handler implements separate trace service APIs as defined in the API contract.
type TraceServiceHandler struct {
	repo    repo.SchemaStore
	kpiRepo repo.KPIRepo
	cache   cache.ValkeyCluster
	logger  logging.Logger
}

// NewTraceServiceHandler creates a new trace service handler
func NewTraceServiceHandler(r repo.SchemaStore, kpirepo repo.KPIRepo, cache cache.ValkeyCluster, l corelogger.Logger) *TraceServiceHandler {
	return &TraceServiceHandler{
		repo:    r,
		kpiRepo: kpirepo,
		cache:   cache,
		logger:  logging.FromCoreLogger(l),
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

	// Convert TraceService to SchemaDefinition
	kpi := &models.KPIDefinition{
		Name:      traceService.Service,
		Kind:      "tech",
		Tags:      traceService.Tags,
		Category:  traceService.Category,
		Sentiment: traceService.Sentiment,
		CreatedAt: traceService.UpdatedAt,
		UpdatedAt: traceService.UpdatedAt,
	}
	if kpi.ID == "" {
		id, err := services.GenerateDeterministicKPIID(kpi)
		if err == nil {
			kpi.ID = id
		}
	}
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for trace service handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "kpi repo unavailable"})
		return
	}
	if _, _, err := h.kpiRepo.CreateKPI(context.Background(), kpi); err != nil {
		h.logger.Error("failed to create trace service kpi", "error", err, "service", traceService.Service)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save trace service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "trace service upserted"})
}

// GetTraceService retrieves a trace service definition by service name
func (h *TraceServiceHandler) GetTraceService(c *gin.Context) {
	serviceName := c.Param("service")
	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service name is required"})
		return
	}

	tmp := &models.KPIDefinition{Name: serviceName}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for trace service", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trace service"})
		return
	}
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for trace service handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "kpi repo unavailable"})
		return
	}
	kdef, err := h.kpiRepo.GetKPI(context.Background(), detID)
	if err != nil {
		h.logger.Error("trace service get failed", "error", err, "service", serviceName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trace service"})
		return
	}
	if kdef == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trace service not found"})
		return
	}

	traceService := &models.TraceService{
		Service:   kdef.Name,
		Tags:      kdef.Tags,
		Category:  kdef.Category,
		Sentiment: kdef.Sentiment,
		UpdatedAt: kdef.UpdatedAt,
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

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	var traceServices []*models.TraceService
	var total int
	var err error
	if kpirepo, ok := h.repo.(repo.KPIRepo); ok {
		kpis, totalKpis, lerr := kpirepo.ListKPIs(context.Background(), models.KPIListRequest{Tags: []string{"trace_service"}, Limit: req.Limit, Offset: req.Offset})
		if lerr != nil {
			err = lerr
		} else {
			total = int(totalKpis)
			traceServices = make([]*models.TraceService, 0, len(kpis))
			for _, k := range kpis {
				traceServices = append(traceServices, &models.TraceService{
					Service:   k.Name,
					Tags:      k.Tags,
					Category:  k.Category,
					Sentiment: k.Sentiment,
					UpdatedAt: k.UpdatedAt,
					// Description, ServicePurpose, Owner are not present in KPIDefinition; omit for now
				})
			}
		}
	}

	if err != nil {
		h.logger.Error("trace service list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list trace services"})
		return
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

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	tmp := &models.KPIDefinition{Name: serviceName}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for trace service delete", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete trace service"})
		return
	}
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for trace service handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "kpi repo unavailable"})
		return
	}
	err = h.kpiRepo.DeleteKPI(context.Background(), detID)
	if err != nil {
		h.logger.Error("trace service delete failed", "error", err, "service", serviceName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete trace service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
