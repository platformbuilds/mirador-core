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

// MetricHandler provides API endpoints for metric definitions.
// This handler implements separate metric APIs as defined in the API contract.
type MetricHandler struct {
	repo   repo.SchemaStore
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewMetricHandler creates a new metric handler
func NewMetricHandler(r repo.SchemaStore, cache cache.ValkeyCluster, l logger.Logger) *MetricHandler {
	return &MetricHandler{
		repo:   r,
		cache:  cache,
		logger: l,
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

	// Convert Metric to SchemaDefinition
	schemaDef := &models.SchemaDefinition{
		ID:        metric.Metric, // Use metric name as ID
		Name:      metric.Metric,
		Type:      models.SchemaTypeMetric,
		Category:  metric.Category,
		Sentiment: metric.Sentiment,
		Author:    metric.Author,
		Tags:      metric.Tags,
		Extensions: models.SchemaExtensions{
			Metric: &models.MetricExtension{
				Description: metric.Description,
				Owner:       metric.Owner,
			},
		},
	}

	err := h.repo.UpsertSchemaAsKPI(context.Background(), schemaDef, metric.Author)
	if err != nil {
		h.logger.Error("metric upsert failed", "error", err, "metric", metric.Metric)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert metric"})
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

	schemaDef, err := h.repo.GetSchemaAsKPI(context.Background(), "metric", metricName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "metric not found"})
		} else {
			h.logger.Error("metric get failed", "error", err, "metric", metricName)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metric"})
		}
		return
	}

	// Convert SchemaDefinition back to Metric
	metric := &models.Metric{
		Metric:      schemaDef.Name,
		Description: schemaDef.Extensions.Metric.Description,
		Owner:       schemaDef.Extensions.Metric.Owner,
		Tags:        schemaDef.Tags,
		Category:    schemaDef.Category,
		Sentiment:   schemaDef.Sentiment,
		Author:      schemaDef.Author,
		UpdatedAt:   schemaDef.UpdatedAt,
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

	var schemaDefs []*models.SchemaDefinition
	var total int
	var err error
	if kpirepo, ok := h.repo.(repo.KPIRepo); ok {
		kpis, totalKpis, lerr := kpirepo.ListKPIs(context.Background(), []string{"metric"}, req.Limit, req.Offset)
		if lerr != nil {
			err = lerr
		} else {
			total = totalKpis
			schemaDefs = make([]*models.SchemaDefinition, 0, len(kpis))
			for _, k := range kpis {
				schemaDefs = append(schemaDefs, kpiToSchemaDefinition(k, models.SchemaTypeMetric))
			}
		}
	} else {
		schemaDefs, total, err = h.repo.ListSchemasAsKPIs(context.Background(), "metric", req.Limit, req.Offset)
	}
	if err != nil {
		h.logger.Error("metric list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list metrics"})
		return
	}

	// Convert SchemaDefinitions back to Metrics
	metrics := make([]*models.Metric, len(schemaDefs))
	for i, schemaDef := range schemaDefs {
		metrics[i] = &models.Metric{
			Metric:      schemaDef.Name,
			Description: schemaDef.Extensions.Metric.Description,
			Owner:       schemaDef.Extensions.Metric.Owner,
			Tags:        schemaDef.Tags,
			Category:    schemaDef.Category,
			Sentiment:   schemaDef.Sentiment,
			Author:      schemaDef.Author,
			UpdatedAt:   schemaDef.UpdatedAt,
		}
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

	// Delete as KPI by id (metrics migrated to KPI-backed storage)
	err := h.repo.DeleteSchemaAsKPI(context.Background(), metricName)
	if err != nil {
		h.logger.Error("metric delete failed", "error", err, "metric", metricName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete metric"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
