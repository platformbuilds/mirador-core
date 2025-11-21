package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MetricHandler provides API endpoints for metric definitions.
// This handler implements separate metric APIs as defined in the API contract.
type MetricHandler struct {
	repo    repo.SchemaStore
	kpiRepo repo.KPIRepo
	cache   cache.ValkeyCluster
	logger  logger.Logger
}

// NewMetricHandler creates a new metric handler
func NewMetricHandler(r repo.SchemaStore, kpirepo repo.KPIRepo, cache cache.ValkeyCluster, l logger.Logger) *MetricHandler {
	return &MetricHandler{
		repo:    r,
		kpiRepo: kpirepo,
		cache:   cache,
		logger:  l,
	}
}

// CreateOrUpdateMetric creates or updates a metric definition
func (h *MetricHandler) CreateOrUpdateMetric(c *gin.Context) {
	var req models.MetricRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.Metric == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric is required"})
		return
	}

	metric := req.Metric

	// Convert Metric to KPI-backed object and persist via KPIRepo
	kpi := &models.KPIDefinition{
		Name:      metric.Metric,
		Kind:      "tech",
		Tags:      metric.Tags,
		Category:  metric.Category,
		Sentiment: metric.Sentiment,
		CreatedAt: metric.UpdatedAt,
		UpdatedAt: metric.UpdatedAt,
	}

	if kpi.ID == "" {
		id, err := services.GenerateDeterministicKPIID(kpi)
		if err == nil {
			kpi.ID = id
		}
	}

	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for metric handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}

	if _, _, err := h.kpiRepo.CreateKPI(context.Background(), kpi); err != nil {
		h.logger.Error("metric create failed", "error", err, "metric", metric.Metric)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create metric"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "metric": metric.Metric})
}

// GetMetric retrieves a metric definition by name
func (h *MetricHandler) GetMetric(c *gin.Context) {
	metricName := c.Param("metric")
	if metricName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric name is required"})
		return
	}

	// Build deterministic ID for metric-based KPI and fetch
	tmp := &models.KPIDefinition{Name: metricName}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for metric", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metric"})
		return
	}
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for metric handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}
	kdef, err := h.kpiRepo.GetKPI(context.Background(), detID)
	if err != nil {
		h.logger.Error("metric get failed", "error", err, "metric", metricName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metric"})
		return
	}
	if kdef == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "metric not found"})
		return
	}

	metric := &models.Metric{
		Metric:    kdef.Name,
		Tags:      kdef.Tags,
		Category:  kdef.Category,
		Sentiment: kdef.Sentiment,
		UpdatedAt: kdef.UpdatedAt,
	}

	c.JSON(http.StatusOK, models.MetricResponse{Metric: metric})
}

// ListMetrics lists metric definitions with optional filtering
func (h *MetricHandler) ListMetrics(c *gin.Context) {
	var req models.MetricListRequest
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

	var metrics []*models.Metric
	var total int
	var err error
	if kpirepo, ok := h.repo.(repo.KPIRepo); ok {
		kpis, totalKpis, lerr := kpirepo.ListKPIs(context.Background(), models.KPIListRequest{Tags: []string{"metric"}, Limit: req.Limit, Offset: req.Offset})
		if lerr != nil {
			err = lerr
		} else {
			total = int(totalKpis)
			metrics = make([]*models.Metric, 0, len(kpis))
			for _, k := range kpis {
				metrics = append(metrics, &models.Metric{
					Metric:    k.Name,
					Tags:      k.Tags,
					Category:  k.Category,
					Sentiment: k.Sentiment,
					UpdatedAt: k.UpdatedAt,
					// Description, Owner are not present in KPIDefinition; omit for now
				})
			}
		}
	}

	if err != nil {
		h.logger.Error("metric list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list metrics"})
		return
	}

	nextOffset := req.Offset + len(metrics)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.MetricListResponse{
		Metrics:    metrics,
		Total:      total,
		NextOffset: nextOffset,
	})
}

// DeleteMetric deletes a metric definition by name
func (h *MetricHandler) DeleteMetric(c *gin.Context) {
	metricName := c.Param("metric")
	if metricName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric name is required"})
		return
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	// Delete as KPI by deterministic id
	tmp := &models.KPIDefinition{Name: metricName}
	detID, err := services.GenerateDeterministicKPIID(tmp)
	if err != nil {
		h.logger.Error("failed to compute deterministic id for metric delete", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete metric"})
		return
	}
	if h.kpiRepo == nil {
		h.logger.Error("KPIRepo not configured for metric handler")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository unavailable"})
		return
	}
	err = h.kpiRepo.DeleteKPI(context.Background(), detID)
	if err != nil {
		h.logger.Error("metric delete failed", "error", err, "metric", metricName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete metric"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
