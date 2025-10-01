package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MetricsQLQueryHandler handles MetricsQL function query endpoints
type MetricsQLQueryHandler struct {
	queryService *services.VictoriaMetricsQueryService
	cache        cache.ValkeyCluster
	logger       logger.Logger
}

// NewMetricsQLQueryHandler creates a new MetricsQL query handler
func NewMetricsQLQueryHandler(queryService *services.VictoriaMetricsQueryService, cache cache.ValkeyCluster, logger logger.Logger) *MetricsQLQueryHandler {
	return &MetricsQLQueryHandler{
		queryService: queryService,
		cache:        cache,
		logger:       logger,
	}
}

// ExecuteRollupFunction handles rollup function queries
func (h *MetricsQLQueryHandler) ExecuteRollupFunction(c *gin.Context) {
	functionName := c.Param("function")
	h.executeFunctionQuery(c, "rollup", functionName)
}

// ExecuteTransformFunction handles transform function queries
func (h *MetricsQLQueryHandler) ExecuteTransformFunction(c *gin.Context) {
	functionName := c.Param("function")
	h.executeFunctionQuery(c, "transform", functionName)
}

// ExecuteLabelFunction handles label function queries
func (h *MetricsQLQueryHandler) ExecuteLabelFunction(c *gin.Context) {
	functionName := c.Param("function")
	h.executeFunctionQuery(c, "label", functionName)
}

// ExecuteAggregateFunction handles aggregate function queries
func (h *MetricsQLQueryHandler) ExecuteAggregateFunction(c *gin.Context) {
	functionName := c.Param("function")
	h.executeFunctionQuery(c, "aggregate", functionName)
}

// executeFunctionQuery is a helper method to execute function queries
func (h *MetricsQLQueryHandler) executeFunctionQuery(c *gin.Context, category, functionName string) {
	// Get the validated request from middleware context
	validatedReq, exists := c.Get("validated_request")
	if !exists {
		h.logger.Error("Validated request not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Request validation failed"})
		return
	}

	req, ok := validatedReq.(*models.MetricsQLFunctionRequest)
	if !ok {
		h.logger.Error("Invalid validated request type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Request validation failed"})
		return
	}

	// Validate function name (additional check beyond middleware)
	if !h.isValidFunction(functionName, category) {
		h.logger.Error("Invalid function name", "function", functionName, "category", category)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid function name for category"})
		return
	}

	// Set the function name in the request
	req.Function = functionName

	h.logger.Info("Executing MetricsQL function query",
		"function", functionName,
		"category", category,
		"query", req.Query)

	// Execute the query
	resp, err := h.queryService.ExecuteFunctionQuery(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to execute MetricsQL function query", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute query"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ExecuteRollupRangeFunction handles rollup range function queries
func (h *MetricsQLQueryHandler) ExecuteRollupRangeFunction(c *gin.Context) {
	functionName := c.Param("function")
	h.executeRangeFunctionQuery(c, "rollup", functionName)
}

// ExecuteTransformRangeFunction handles transform range function queries
func (h *MetricsQLQueryHandler) ExecuteTransformRangeFunction(c *gin.Context) {
	functionName := c.Param("function")
	h.executeRangeFunctionQuery(c, "transform", functionName)
}

// ExecuteLabelRangeFunction handles label range function queries
func (h *MetricsQLQueryHandler) ExecuteLabelRangeFunction(c *gin.Context) {
	functionName := c.Param("function")
	h.executeRangeFunctionQuery(c, "label", functionName)
}

// ExecuteAggregateRangeFunction handles aggregate range function queries
func (h *MetricsQLQueryHandler) ExecuteAggregateRangeFunction(c *gin.Context) {
	functionName := c.Param("function")
	h.executeRangeFunctionQuery(c, "aggregate", functionName)
}

// executeRangeFunctionQuery is a helper method to execute range function queries
func (h *MetricsQLQueryHandler) executeRangeFunctionQuery(c *gin.Context, category, functionName string) {
	// Get the validated range request from middleware context
	validatedReq, exists := c.Get("validated_range_request")
	if !exists {
		h.logger.Error("Validated range request not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Request validation failed"})
		return
	}

	req, ok := validatedReq.(*models.MetricsQLFunctionRangeRequest)
	if !ok {
		h.logger.Error("Invalid validated range request type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Request validation failed"})
		return
	}

	// Validate function name (additional check beyond middleware)
	if !h.isValidFunction(functionName, category) {
		h.logger.Error("Invalid function name", "function", functionName, "category", category)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid function name for category"})
		return
	}

	// Set the function name in the request
	req.Function = functionName

	h.logger.Info("Executing MetricsQL function range query",
		"function", functionName,
		"category", category,
		"query", req.Query)

	// Execute the query
	resp, err := h.queryService.ExecuteRangeFunctionQuery(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to execute MetricsQL function range query", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute query"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// isValidFunction validates if a function belongs to a category
func (h *MetricsQLQueryHandler) isValidFunction(functionName, category string) bool {
	// Define valid functions for each category
	rollupFunctions := []string{
		"absent_over_time", "avg_over_time", "changes", "count_over_time",
		"delta", "deriv", "distinct_over_time", "histogram_over_time", "holt_winters", "idelta", "increase", "irate",
		"lag", "last_over_time", "lifetime", "mad_over_time", "max_over_time", "min_over_time", "predict_linear",
		"present_over_time", "quantile_over_time", "rate", "resets", "rollup", "rollup_delta", "rollup_increase", "rollup_rate",
		"stddev_over_time", "stdvar_over_time", "sum_over_time",
		"timestamp", "tmax_over_time", "tmin_over_time", "zscore_over_time",
	}

	transformFunctions := []string{
		"abs", "acos", "acosh", "asin", "asinh", "atan", "atanh",
		"ceil", "clamp", "clamp_max", "clamp_min", "cos", "cosh",
		"day_of_month", "day_of_week", "day_of_year", "deg", "exp", "floor", "histogram_avg", "histogram_quantile", "histogram_stddev", "hour",
		"interpolate", "keep_last_value", "keep_next_value", "ln",
		"log10", "log2", "minute", "month", "now", "pi", "prometheus_buckets", "rad", "rand", "rand_normal", "range_linear", "range_vector", "remove_resets", "round", "running_avg", "running_max", "running_min", "running_sum",
		"scalar", "sgn", "sin", "sinh", "smooth_exponential", "sort", "sqrt", "tan", "tanh",
		"time", "timestamp", "timezone_offset", "union", "vector", "year",
	}

	labelFunctions := []string{
		"alias", "drop_common_labels", "label_copy", "label_del", "label_graphite_group",
		"label_join", "label_keep", "label_lowercase", "label_map", "label_match",
		"label_mismatch", "label_move", "label_replace", "label_set", "label_transform",
		"label_uppercase", "labels_equal", "label_value", "sort_by_label",
		"sort_by_label_desc",
	}

	aggregateFunctions := []string{
		"any", "avg", "bottomk", "bottomk_avg", "bottomk_max", "bottomk_min", "count", "count_values", "distinct", "geomean", "group", "histogram", "limitk", "mad", "max",
		"median", "min", "mode", "outliers_iqr", "outliers_mad", "outliersk", "quantile", "quantiles", "share", "stddev", "stdvar", "sum", "sum2", "topk", "topk_avg", "topk_max", "topk_min", "zscore",
	}

	var validFunctions []string
	switch strings.ToLower(category) {
	case "rollup":
		validFunctions = rollupFunctions
	case "transform":
		validFunctions = transformFunctions
	case "label":
		validFunctions = labelFunctions
	case "aggregate":
		validFunctions = aggregateFunctions
	default:
		return false
	}

	for _, fn := range validFunctions {
		if fn == functionName {
			return true
		}
	}
	return false
}
