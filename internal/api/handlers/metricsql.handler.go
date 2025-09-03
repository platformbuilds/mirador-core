package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/miradorstack/internal/metrics"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/internal/services"
	"github.com/platformbuilds/miradorstack/internal/utils"
	"github.com/platformbuilds/miradorstack/pkg/cache"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type MetricsQLHandler struct {
	metricsService *services.VictoriaMetricsService
	cache          cache.ValkeyCluster
	logger         logger.Logger
	validator      *utils.QueryValidator
}

func NewMetricsQLHandler(metricsService *services.VictoriaMetricsService, cache cache.ValkeyCluster, logger logger.Logger) *MetricsQLHandler {
	return &MetricsQLHandler{
		metricsService: metricsService,
		cache:          cache,
		logger:         logger,
		validator:      utils.NewQueryValidator(),
	}
}

// POST /api/v1/query - Execute instant MetricsQL query
func (h *MetricsQLHandler) ExecuteQuery(c *gin.Context) {
	start := time.Now()
	tenantID := c.GetString("tenant_id")

	var request models.MetricsQLQueryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "400", tenantID).Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "Invalid query request format",
			"details": err.Error(),
		})
		return
	}

	// Validate MetricsQL query syntax
	if err := h.validator.ValidateMetricsQL(request.Query); err != nil {
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "400", tenantID).Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Invalid MetricsQL query: %s", err.Error()),
		})
		return
	}

	// Check Valkey cluster cache for query results
	queryHash := generateQueryHash(request.Query, request.Time, tenantID)
	if cached, err := h.cache.GetCachedQueryResult(c.Request.Context(), queryHash); err == nil {
		var cachedResult models.MetricsQLQueryResponse
		if json.Unmarshal(cached, &cachedResult) == nil {
			metrics.CacheRequestsTotal.WithLabelValues("get", "hit").Inc()
			c.Header("X-Cache", "HIT")
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"data":   cachedResult.Data,
				"metadata": gin.H{
					"executionTime": cachedResult.ExecutionTime,
					"cached":        true,
					"cacheHit":      true,
				},
			})
			return
		}
	}
	metrics.CacheRequestsTotal.WithLabelValues("get", "miss").Inc()

	// Execute query via VictoriaMetrics
	request.TenantID = tenantID
	result, err := h.metricsService.ExecuteQuery(c.Request.Context(), &request)
	if err != nil {
		executionTime := time.Since(start)
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "500", tenantID).Inc()
		metrics.QueryExecutionDuration.WithLabelValues("metricsql", tenantID).Observe(executionTime.Seconds())

		h.logger.Error("MetricsQL query execution failed",
			"query", request.Query,
			"tenant", tenantID,
			"error", err,
			"executionTime", executionTime,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Query execution failed",
		})
		return
	}

	executionTime := time.Since(start)

	// Cache successful results in Valkey cluster for faster fetch
	if result.Status == "success" {
		cacheResponse := models.MetricsQLQueryResponse{
			Data:          result.Data,
			ExecutionTime: executionTime.Milliseconds(),
			Timestamp:     time.Now(),
		}
		h.cache.CacheQueryResult(c.Request.Context(), queryHash, cacheResponse, 2*time.Minute)
		metrics.CacheRequestsTotal.WithLabelValues("set", "success").Inc()
	}

	// Record metrics
	metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "200", tenantID).Inc()
	metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, c.FullPath(), tenantID).Observe(executionTime.Seconds())
	metrics.QueryExecutionDuration.WithLabelValues("metricsql", tenantID).Observe(executionTime.Seconds())

	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   result.Data,
		"metadata": gin.H{
			"executionTime": executionTime.Milliseconds(),
			"seriesCount":   result.SeriesCount,
			"cached":        false,
			"timestamp":     time.Now().Format(time.RFC3339),
		},
	})
}

// POST /api/v1/query_range - Execute range MetricsQL query
func (h *MetricsQLHandler) ExecuteRangeQuery(c *gin.Context) {
	start := time.Now()
	tenantID := c.GetString("tenant_id")

	var request models.MetricsQLRangeQueryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid range query request",
		})
		return
	}

	// Validate query
	if err := h.validator.ValidateMetricsQL(request.Query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Invalid MetricsQL query: %s", err.Error()),
		})
		return
	}

	// Execute range query
	request.TenantID = tenantID
	result, err := h.metricsService.ExecuteRangeQuery(c.Request.Context(), &request)
	if err != nil {
		executionTime := time.Since(start)
		h.logger.Error("MetricsQL range query failed",
			"query", request.Query,
			"timeRange", fmt.Sprintf("%s to %s", request.Start, request.End),
			"error", err,
			"executionTime", executionTime,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Range query execution failed",
		})
		return
	}

	executionTime := time.Since(start)
	metrics.QueryExecutionDuration.WithLabelValues("metricsql_range", tenantID).Observe(executionTime.Seconds())

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   result.Data,
		"metadata": gin.H{
			"executionTime": executionTime.Milliseconds(),
			"dataPoints":    result.DataPointCount,
			"timeRange":     fmt.Sprintf("%s to %s", request.Start, request.End),
			"step":          request.Step,
		},
	})
}

func generateQueryHash(query, timeParam, tenantID string) string {
	data := fmt.Sprintf("%s:%s:%s", query, timeParam, tenantID)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (h *MetricsQLHandler) GetSeries(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Parse query parameters
	match := c.QueryArray("match[]")
	start := c.Query("start")
	end := c.Query("end")

	if len(match) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "At least one match[] parameter is required",
		})
		return
	}

	// Create series request
	request := &models.SeriesRequest{
		Match:    match,
		Start:    start,
		End:      end,
		TenantID: tenantID,
	}

	series, err := h.metricsService.GetSeries(c.Request.Context(), request)
	if err != nil {
		h.logger.Error("Failed to get series", "tenant", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve series",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   series,
	})
}

func (h *MetricsQLHandler) GetLabels(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Parse query parameters
	start := c.Query("start")
	end := c.Query("end")
	match := c.QueryArray("match[]")

	request := &models.LabelsRequest{
		Start:    start,
		End:      end,
		Match:    match,
		TenantID: tenantID,
	}

	labels, err := h.metricsService.GetLabels(c.Request.Context(), request)
	if err != nil {
		h.logger.Error("Failed to get labels", "tenant", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve labels",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   labels,
	})
}

// GET /api/v1/label/:name/values - Get values for a specific label
func (h *MetricsQLHandler) GetLabelValues(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	labelName := c.Param("name")

	if labelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Label name is required",
		})
		return
	}

	// Parse query parameters
	start := c.Query("start")
	end := c.Query("end")
	match := c.QueryArray("match[]")

	request := &models.LabelValuesRequest{
		Label:    labelName,
		Start:    start,
		End:      end,
		Match:    match,
		TenantID: tenantID,
	}

	values, err := h.metricsService.GetLabelValues(c.Request.Context(), request)
	if err != nil {
		h.logger.Error("Failed to get label values",
			"label", labelName,
			"tenant", tenantID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve label values",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   values,
	})
}
