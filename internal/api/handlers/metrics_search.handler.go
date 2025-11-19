package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type MetricsSearchHandler struct {
	metricsIndexer services.MetricsMetadataIndexer
	logger         logger.Logger
}

func NewMetricsSearchHandler(metricsIndexer services.MetricsMetadataIndexer, logger logger.Logger) *MetricsSearchHandler {
	return &MetricsSearchHandler{
		metricsIndexer: metricsIndexer,
		logger:         logger,
	}
}

// HandleMetricsSearch handles searching for metrics using Bleve
func (h *MetricsSearchHandler) HandleMetricsSearch(c *gin.Context) {
	var req models.MetricMetadataSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind metrics search request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Check if metrics indexer is properly configured
	if h.metricsIndexer == nil {
		h.logger.Warn("Metrics metadata indexer not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Metrics metadata search is not available",
			"details": "Metrics metadata indexing is not configured for this environment",
		})
		return
	}

	// Set default limit if not specified
	if req.Limit <= 0 {
		req.Limit = 50
	}

	// Set maximum limit
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	// Execute the search
	result, err := h.metricsIndexer.SearchMetrics(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to search metrics", "error", err, "query", req.Query)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Metrics search failed",
			"details": err.Error(),
			"query":   req.Query,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleMetricsSync handles syncing metrics metadata
func (h *MetricsSearchHandler) HandleMetricsSync(c *gin.Context) {
	var req models.MetricMetadataSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind metrics sync request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Check if metrics indexer is properly configured
	if h.metricsIndexer == nil {
		h.logger.Warn("Metrics metadata indexer not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Metrics metadata sync is not available",
			"details": "Metrics metadata indexing is not configured for this environment",
		})
		return
	}

	// Set default time range if not specified (last 24 hours)
	if req.TimeRange == nil {
		now := time.Now()
		yesterday := now.Add(-24 * time.Hour)
		req.TimeRange = &models.TimeRange{
			Start: yesterday,
			End:   now,
		}
	}

	// Set default batch size
	if req.BatchSize <= 0 {
		req.BatchSize = 1000
	}

	// Execute the sync
	result, err := h.metricsIndexer.SyncMetadata(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to sync metrics metadata", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Metrics metadata sync failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleMetricsHealth returns health status of the metrics metadata system
func (h *MetricsSearchHandler) HandleMetricsHealth(c *gin.Context) {
	// Check if metrics indexer is properly configured
	if h.metricsIndexer == nil {
		h.logger.Warn("Metrics metadata indexer not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Metrics metadata health check is not available",
			"details": "Metrics metadata indexing is not configured for this environment",
		})
		return
	}

	health, err := h.metricsIndexer.GetHealthStatus(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get metrics health status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve metrics health status",
		})
		return
	}

	statusCode := http.StatusOK
	if !health.IsHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, health)
}

// RegisterRoutes registers the metrics search routes
func (h *MetricsSearchHandler) RegisterRoutes(router *gin.RouterGroup) {
	metrics := router.Group("/metrics")
	{
		metrics.POST("/search", h.HandleMetricsSearch)
		metrics.POST("/sync", h.HandleMetricsSync)
		metrics.GET("/health", h.HandleMetricsHealth)
	}
}
