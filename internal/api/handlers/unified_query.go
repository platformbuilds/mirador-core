package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type UnifiedQueryHandler struct {
	unifiedEngine services.UnifiedQueryEngine
	logger        logger.Logger
}

func NewUnifiedQueryHandler(unifiedEngine services.UnifiedQueryEngine, logger logger.Logger) *UnifiedQueryHandler {
	return &UnifiedQueryHandler{
		unifiedEngine: unifiedEngine,
		logger:        logger,
	}
}

// HandleUnifiedQuery handles unified queries across all engines
func (h *UnifiedQueryHandler) HandleUnifiedQuery(c *gin.Context) {
	var req models.UnifiedQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind unified query request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Execute the unified query
	result, err := h.unifiedEngine.ExecuteQuery(c.Request.Context(), req.Query)
	if err != nil {
		h.logger.Error("Failed to execute unified query", "error", err, "query_id", req.Query.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Query execution failed",
			"details":  err.Error(),
			"query_id": req.Query.ID,
		})
		return
	}

	response := models.UnifiedQueryResponse{
		Result: result,
	}

	c.JSON(http.StatusOK, response)
}

// HandleUnifiedCorrelation handles correlation queries across engines
func (h *UnifiedQueryHandler) HandleUnifiedCorrelation(c *gin.Context) {
	var req models.UnifiedQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind unified correlation request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Ensure this is a correlation query
	if req.Query.Type != models.QueryTypeCorrelation {
		req.Query.Type = models.QueryTypeCorrelation
	}

	// Execute the correlation query
	result, err := h.unifiedEngine.ExecuteCorrelationQuery(c.Request.Context(), req.Query)
	if err != nil {
		h.logger.Error("Failed to execute unified correlation", "error", err, "query_id", req.Query.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Correlation execution failed",
			"details":  err.Error(),
			"query_id": req.Query.ID,
		})
		return
	}

	response := models.UnifiedQueryResponse{
		Result: result,
	}

	c.JSON(http.StatusOK, response)
}

// HandleQueryMetadata returns metadata about supported query capabilities
func (h *UnifiedQueryHandler) HandleQueryMetadata(c *gin.Context) {
	metadata, err := h.unifiedEngine.GetQueryMetadata(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get query metadata", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve query metadata",
		})
		return
	}

	c.JSON(http.StatusOK, metadata)
}

// HandleHealthCheck returns health status of all engines
func (h *UnifiedQueryHandler) HandleHealthCheck(c *gin.Context) {
	health, err := h.unifiedEngine.HealthCheck(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get health status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve health status",
		})
		return
	}

	statusCode := http.StatusOK
	switch health.OverallHealth {
	case "unhealthy":
		statusCode = http.StatusServiceUnavailable
	case "partial":
		statusCode = http.StatusPartialContent
	}

	c.JSON(statusCode, health)
}

// HandleUnifiedSearch handles unified search across all engines
func (h *UnifiedQueryHandler) HandleUnifiedSearch(c *gin.Context) {
	var req models.UnifiedQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind unified search request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// For search, we can route to the appropriate engine based on query content
	result, err := h.unifiedEngine.ExecuteQuery(c.Request.Context(), req.Query)
	if err != nil {
		h.logger.Error("Failed to execute unified search", "error", err, "query_id", req.Query.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Search execution failed",
			"details":  err.Error(),
			"query_id": req.Query.ID,
		})
		return
	}

	response := models.UnifiedQueryResponse{
		Result: result,
	}

	c.JSON(http.StatusOK, response)
}

// HandleUQLQuery handles direct UQL query execution
func (h *UnifiedQueryHandler) HandleUQLQuery(c *gin.Context) {
	var req models.UQLQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind UQL query request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Execute the UQL query
	result, err := h.unifiedEngine.ExecuteUQLQuery(c.Request.Context(), req.Query)
	if err != nil {
		h.logger.Error("Failed to execute UQL query", "error", err, "query_id", req.Query.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "UQL query execution failed",
			"details":  err.Error(),
			"query_id": req.Query.ID,
		})
		return
	}

	response := models.UnifiedQueryResponse{
		Result: result,
	}

	c.JSON(http.StatusOK, response)
}

// HandleUQLValidate validates UQL query syntax without execution
func (h *UnifiedQueryHandler) HandleUQLValidate(c *gin.Context) {
	var req models.UQLValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind UQL validate request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// For validation, we can use the UQL parser to check syntax
	// This is a simplified validation - in practice, you'd want more comprehensive validation
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Query cannot be empty",
		})
		return
	}

	// Basic validation response
	response := models.UQLValidateResponse{
		Valid: true,
		Query: req.Query,
	}

	c.JSON(http.StatusOK, response)
}

// HandleUQLExplain provides query execution plan for UQL queries
func (h *UnifiedQueryHandler) HandleUQLExplain(c *gin.Context) {
	var req models.UQLExplainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind UQL explain request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// For explain, we would ideally get the execution plan from the optimizer
	// This is a simplified response - in practice, you'd integrate with the optimizer
	explainResult := models.UQLExplainResponse{
		Query: req.Query.Query,
		Plan: models.QueryPlan{
			Steps: []models.QueryPlanStep{
				{
					Type:        "parse",
					Description: "Parse UQL query into AST",
					Engine:      "uql_parser",
				},
				{
					Type:        "optimize",
					Description: "Apply query optimizations",
					Engine:      "uql_optimizer",
				},
				{
					Type:        "translate",
					Description: "Translate to engine-specific queries",
					Engine:      "uql_translator",
				},
				{
					Type:        "execute",
					Description: "Execute translated queries",
					Engine:      "unified_engine",
				},
			},
		},
	}

	c.JSON(http.StatusOK, explainResult)
}

// HandleUnifiedStats returns statistics about unified query operations
func (h *UnifiedQueryHandler) HandleUnifiedStats(c *gin.Context) {
	// Get health status and metadata to provide basic statistics
	health, err := h.unifiedEngine.HealthCheck(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get health status for stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve query statistics",
		})
		return
	}

	metadata, err := h.unifiedEngine.GetQueryMetadata(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get query metadata for stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve query statistics",
		})
		return
	}

	// Build basic statistics response
	stats := gin.H{
		"unified_query_engine": gin.H{
			"health":            health.OverallHealth,
			"supported_engines": metadata.SupportedEngines,
			"cache_enabled":     metadata.CacheCapabilities.Supported,
			"cache_default_ttl": metadata.CacheCapabilities.DefaultTTL.String(),
			"cache_max_ttl":     metadata.CacheCapabilities.MaxTTL.String(),
			"last_health_check": health.LastChecked,
		},
		"engines": health.EngineHealth,
	}

	c.JSON(http.StatusOK, stats)
}

// HandleFailureDetection detects component failures in the financial transaction system
func (h *UnifiedQueryHandler) HandleFailureDetection(c *gin.Context) {
	var req models.FailureDetectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind failure detection request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Execute failure detection
	result, err := h.unifiedEngine.DetectComponentFailures(c.Request.Context(), req.TimeRange, req.Components)
	if err != nil {
		h.logger.Error("Failed to detect component failures", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failure detection failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleTransactionFailureCorrelation correlates failures for specific transaction IDs
func (h *UnifiedQueryHandler) HandleTransactionFailureCorrelation(c *gin.Context) {
	var req models.TransactionFailureCorrelationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind transaction failure correlation request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Execute transaction failure correlation
	result, err := h.unifiedEngine.CorrelateTransactionFailures(c.Request.Context(), req.TransactionIDs, req.TimeRange)
	if err != nil {
		h.logger.Error("Failed to correlate transaction failures", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Transaction failure correlation failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
