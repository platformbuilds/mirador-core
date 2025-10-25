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

	// Set tenant ID from context (middleware should set this)
	if tenantID, exists := c.Get("tenant_id"); exists {
		if tid, ok := tenantID.(string); ok {
			req.Query.TenantID = tid
		}
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

	// Set tenant ID from context
	if tenantID, exists := c.Get("tenant_id"); exists {
		if tid, ok := tenantID.(string); ok {
			req.Query.TenantID = tid
		}
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
	if health.OverallHealth == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if health.OverallHealth == "partial" {
		statusCode = http.StatusPartialContent
	}

	c.JSON(statusCode, health)
}

// RegisterRoutes registers the unified query routes
func (h *UnifiedQueryHandler) RegisterRoutes(router *gin.RouterGroup) {
	unified := router.Group("/unified")
	{
		unified.POST("/query", h.HandleUnifiedQuery)
		unified.POST("/correlation", h.HandleUnifiedCorrelation)
		unified.GET("/metadata", h.HandleQueryMetadata)
		unified.GET("/health", h.HandleHealthCheck)
	}
}
