package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"context"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/metrics"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/utils"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type MetricsQLHandler struct {
	metricsService *services.VictoriaMetricsService
	cache          cache.ValkeyCluster
	logger         logger.Logger
	validator      *utils.QueryValidator
	schemaRepo     SchemaProvider
}

// SchemaProvider is the minimal interface the handler uses to fetch schema definitions.
type SchemaProvider interface {
	GetMetric(ctx context.Context, metric string) (*repo.MetricDef, error)
	GetMetricLabelDefs(ctx context.Context, metric string, labels []string) (map[string]*repo.MetricLabelDef, error)
}

func NewMetricsQLHandler(metricsService *services.VictoriaMetricsService, cache cache.ValkeyCluster, logger logger.Logger) *MetricsQLHandler {
	return &MetricsQLHandler{
		metricsService: metricsService,
		cache:          cache,
		logger:         logger,
		validator:      utils.NewQueryValidator(),
	}
}

func NewMetricsQLHandlerWithSchema(metricsService *services.VictoriaMetricsService, cache cache.ValkeyCluster, logger logger.Logger, schema SchemaProvider) *MetricsQLHandler {
	h := NewMetricsQLHandler(metricsService, cache, logger)
	h.schemaRepo = schema
	return h
}

// GET /api/v1/metrics/names - List metric names (__name__) from VictoriaMetrics
func (h *MetricsQLHandler) GetMetricNames(c *gin.Context) {
	limitStr := c.Query("limit")
	limit := 0
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	req := &models.LabelValuesRequest{
		Label: "__name__",
		Start: c.Query("start"),
		End:   c.Query("end"),
		Match: c.QueryArray("match[]"),
		Limit: limit,
	}
	names, err := h.metricsService.GetLabelValues(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get metric names", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "Failed to retrieve metric names"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   names,
	})
}

// POST /api/v1/query - Execute instant MetricsQL query
func (h *MetricsQLHandler) ExecuteQuery(c *gin.Context) {
	start := time.Now()

	var request models.MetricsQLQueryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "400").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "Invalid query request format",
			"details": err.Error(),
		})
		return
	}

	// Validate MetricsQL query syntax
	if err := h.validator.ValidateMetricsQL(request.Query); err != nil {
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "400").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Invalid MetricsQL query: %s", err.Error()),
		})
		return
	}

	// Include optional flags into cache key to avoid cross-pollution
	includeDefs := true
	if request.IncludeDefinitions != nil {
		includeDefs = *request.IncludeDefinitions
	}
	if q := c.Query("include_definitions"); q != "" {
		if q == "0" || q == "false" {
			includeDefs = false
		}
	}
	var labelKeys []string
	if len(request.LabelKeys) > 0 {
		labelKeys = request.LabelKeys
	}
	if lk := c.Query("label_keys"); lk != "" {
		labelKeys = append(labelKeys, lk)
	}

	// Check Valkey cluster cache for query results
	keySalt := fmt.Sprintf("defs=%t|labels=%v", includeDefs, labelKeys)
	queryHash := generateQueryHash(request.Query+"|"+keySalt, request.Time)
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
				"definitions": cachedResult.Definitions,
			})
			return
		}
	}
	metrics.CacheRequestsTotal.WithLabelValues("get", "miss").Inc()

	// Execute query via VictoriaMetrics
	result, err := h.metricsService.ExecuteQuery(c.Request.Context(), &request)
	if err != nil {
		executionTime := time.Since(start)
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "500").Inc()
		metrics.QueryExecutionDuration.WithLabelValues("metricsql").Observe(executionTime.Seconds())

		h.logger.Error("MetricsQL query execution failed",
			"query", request.Query,
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
		var defs map[string]interface{}
		if includeDefs {
			defs = h.buildDefinitionsFiltered(c.Request.Context(), result.Data, labelKeys)
		}
		cacheResponse := models.MetricsQLQueryResponse{
			Data:          result.Data,
			ExecutionTime: executionTime.Milliseconds(),
			Timestamp:     time.Now(),
			Definitions:   defs,
		}
		h.cache.CacheQueryResult(c.Request.Context(), queryHash, cacheResponse, 2*time.Minute)
		metrics.CacheRequestsTotal.WithLabelValues("set", "success").Inc()
	}

	// Record metrics
	metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), "200").Inc()
	metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(executionTime.Seconds())
	metrics.QueryExecutionDuration.WithLabelValues("metricsql").Observe(executionTime.Seconds())

	// definitions_minimal: only include metric-level defs, skip per-metric label defs
	minimal := false
	if request.DefinitionsMinimal != nil {
		minimal = *request.DefinitionsMinimal
	}
	if q := c.Query("definitions_minimal"); q != "" {
		if q == "1" || q == "true" {
			minimal = true
		}
	}
	var defs map[string]interface{}
	if includeDefs {
		if minimal {
			defs = h.buildMetricOnlyDefinitions(c.Request.Context(), result.Data)
		} else {
			defs = h.buildDefinitionsFiltered(c.Request.Context(), result.Data, labelKeys)
		}
	}
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
		"definitions": defs,
	})
}

// POST /api/v1/query_range - Execute range MetricsQL query
func (h *MetricsQLHandler) ExecuteRangeQuery(c *gin.Context) {
	start := time.Now()

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
	metrics.QueryExecutionDuration.WithLabelValues("metricsql_range").Observe(executionTime.Seconds())

	includeDefs := true
	if request.IncludeDefinitions != nil {
		includeDefs = *request.IncludeDefinitions
	}
	if q := c.Query("include_definitions"); q != "" {
		if q == "0" || q == "false" {
			includeDefs = false
		}
	}
	var labelKeys []string
	if len(request.LabelKeys) > 0 {
		labelKeys = request.LabelKeys
	}
	if lk := c.Query("label_keys"); lk != "" {
		labelKeys = append(labelKeys, lk)
	}
	minimal := false
	if request.DefinitionsMinimal != nil {
		minimal = *request.DefinitionsMinimal
	}
	if q := c.Query("definitions_minimal"); q != "" {
		if q == "1" || q == "true" {
			minimal = true
		}
	}
	var defs map[string]interface{}
	if includeDefs {
		if minimal {
			defs = h.buildMetricOnlyDefinitions(c.Request.Context(), result.Data)
		} else {
			defs = h.buildDefinitionsFiltered(c.Request.Context(), result.Data, labelKeys)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   result.Data,
		"metadata": gin.H{
			"executionTime": executionTime.Milliseconds(),
			"dataPoints":    result.DataPointCount,
			"timeRange":     fmt.Sprintf("%s to %s", request.Start, request.End),
			"step":          request.Step,
		},
		"definitions": defs,
	})
}

// buildDefinitionsFiltered inspects VM data to extract metric names and label keys, optionally filters label keys, and returns definitions.
func (h *MetricsQLHandler) buildDefinitionsFiltered(ctx context.Context, data interface{}, allowedLabelKeys []string) map[string]interface{} {
	if h.schemaRepo == nil || data == nil {
		return nil
	}
	metricsSet := map[string]struct{}{}
	labelsPerMetric := map[string]map[string]struct{}{}
	allowAll := len(allowedLabelKeys) == 0
	allowed := map[string]struct{}{}
	for _, k := range allowedLabelKeys {
		allowed[k] = struct{}{}
	}
	if m, ok := data.(map[string]interface{}); ok {
		if arr, ok := m["result"].([]interface{}); ok {
			for _, it := range arr {
				if series, ok := it.(map[string]interface{}); ok {
					if metr, ok := series["metric"].(map[string]interface{}); ok {
						if name, ok := metr["__name__"].(string); ok && name != "" {
							metricsSet[name] = struct{}{}
							if _, ok := labelsPerMetric[name]; !ok {
								labelsPerMetric[name] = map[string]struct{}{}
							}
							for k := range metr {
								if k == "__name__" {
									continue
								}
								if allowAll {
									labelsPerMetric[name][k] = struct{}{}
								} else {
									if _, ok := allowed[k]; ok {
										labelsPerMetric[name][k] = struct{}{}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	// metric defs
	metricDefs := map[string]interface{}{}
	for name := range metricsSet {
		if md, err := h.schemaRepo.GetMetric(ctx, name); err == nil && md != nil {
			metricDefs[name] = md
		} else {
			metricDefs[name] = map[string]string{"definition": "No definition provided. Use /api/v1/schema/metrics to add one."}
		}
	}
	// label defs per metric
	labelsDefsPerMetric := map[string]interface{}{}
	for metricName := range metricsSet {
		lblset := labelsPerMetric[metricName]
		if len(lblset) == 0 {
			continue
		}
		names := make([]string, 0, len(lblset))
		for l := range lblset {
			names = append(names, l)
		}
		mdefs, err := h.schemaRepo.GetMetricLabelDefs(ctx, metricName, names)
		if err != nil {
			continue
		}
		inner := map[string]interface{}{}
		for _, ln := range names {
			if d, ok := mdefs[ln]; ok {
				inner[ln] = d
			} else {
				inner[ln] = map[string]string{"definition": "No definition provided. Use /api/v1/schema/metrics to add label definition."}
			}
		}
		labelsDefsPerMetric[metricName] = inner
	}
	return map[string]interface{}{
		"metrics": metricDefs,
		"labels":  labelsDefsPerMetric,
	}
}

// buildMetricOnlyDefinitions extracts metric names and returns only metric-level definitions.
func (h *MetricsQLHandler) buildMetricOnlyDefinitions(ctx context.Context, data interface{}) map[string]interface{} {
	if h.schemaRepo == nil || data == nil {
		return nil
	}
	metricsSet := map[string]struct{}{}
	if m, ok := data.(map[string]interface{}); ok {
		if arr, ok := m["result"].([]interface{}); ok {
			for _, it := range arr {
				if series, ok := it.(map[string]interface{}); ok {
					if metr, ok := series["metric"].(map[string]interface{}); ok {
						if name, ok := metr["__name__"].(string); ok && name != "" {
							metricsSet[name] = struct{}{}
						}
					}
				}
			}
		}
	}
	metricDefs := map[string]interface{}{}
	for name := range metricsSet {
		if md, err := h.schemaRepo.GetMetric(ctx, name); err == nil && md != nil {
			metricDefs[name] = md
		} else {
			metricDefs[name] = map[string]string{"definition": "No definition provided. Use /api/v1/schema/metrics to add one."}
		}
	}
	return map[string]interface{}{
		"metrics": metricDefs,
	}
}

func generateQueryHash(query, timeParam) string {
	data := fmt.Sprintf("%s:%s", query, timeParam)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (h *MetricsQLHandler) GetSeries(c *gin.Context) {

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
		Match: match,
		Start: start,
		End:   end,
	}

	series, err := h.metricsService.GetSeries(c.Request.Context(), request)
	if err != nil {
		h.logger.Error("Failed to get series", "error", err)
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

	// Parse JSON payload
	var requestBody struct {
		Metric string `json:"metric" binding:"required"`
		Start  string `json:"start,omitempty"`
		End    string `json:"end,omitempty"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "Invalid request format. 'metric' field is required",
			"details": err.Error(),
		})
		return
	}

	// Construct PromQL selector for the metric
	match := []string{fmt.Sprintf("{__name__=\"%s\"}", requestBody.Metric)}

	request := &models.LabelsRequest{
		Start: requestBody.Start,
		End:   requestBody.End,
		Match: match,
	}

	labels, err := h.metricsService.GetLabels(c.Request.Context(), request)
	if err != nil {
		h.logger.Error("Failed to get labels", "metric", requestBody.Metric, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve labels",
		})
		return
	}

	c.JSON(http.StatusOK, labels)
}

// GET /api/v1/label/:name/values - Get values for a specific label
func (h *MetricsQLHandler) GetLabelValues(c *gin.Context) {
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
	limitStr := c.Query("limit")
	limit := 0
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	request := &models.LabelValuesRequest{
		Label: labelName,
		Start: start,
		End:   end,
		Match: match,
		Limit: limit,
	}

	values, err := h.metricsService.GetLabelValues(c.Request.Context(), request)
	if err != nil {
		h.logger.Error("Failed to get label values",
			"label", labelName,
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
