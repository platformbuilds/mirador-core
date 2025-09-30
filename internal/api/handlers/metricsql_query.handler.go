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
	var req models.MetricsQLFunctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind MetricsQL function request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate function name
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
	resp, err := h.queryService.ExecuteFunctionQuery(c.Request.Context(), &req)
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
	var req models.MetricsQLFunctionRangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind MetricsQL function range request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate function name
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
	resp, err := h.queryService.ExecuteRangeFunctionQuery(c.Request.Context(), &req)
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
		"delta", "deriv", "holt_winters", "idelta", "increase", "irate",
		"last_over_time", "max_over_time", "min_over_time", "predict_linear",
		"present_over_time", "quantile_over_time", "rate", "resets",
		"stddev_over_time", "stdvar_over_time", "sum_over_time",
		"timestamp", "tmax_over_time", "tmin_over_time",
	}

	transformFunctions := []string{
		"abs", "acos", "acosh", "asin", "asinh", "atan", "atanh",
		"ceil", "clamp", "clamp_max", "clamp_min", "cos", "cosh",
		"deg", "exp", "floor", "histogram_quantile", "hour", "ln",
		"log10", "log2", "minute", "month", "pi", "rad", "round",
		"scalar", "sgn", "sin", "sinh", "sqrt", "tan", "tanh",
		"time", "timestamp", "vector", "year",
	}

	labelFunctions := []string{
		"label_copy", "label_del", "label_join", "label_keep",
		"label_map", "label_replace", "label_set", "label_value",
	}

	aggregateFunctions := []string{
		"avg", "bottomk", "count", "count_values", "group", "max",
		"min", "quantile", "stddev", "stdvar", "sum", "topk",
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