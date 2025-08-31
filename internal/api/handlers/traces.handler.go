package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirador/core/internal/models"
	"github.com/mirador/core/internal/services"
	"github.com/mirador/core/pkg/cache"
	"github.com/mirador/core/pkg/logger"
)

type TracesHandler struct {
	tracesService *services.VictoriaTracesService
	cache         cache.ValleyCluster
	logger        logger.Logger
}

func NewTracesHandler(tracesService *services.VictoriaTracesService, cache cache.ValleyCluster, logger logger.Logger) *TracesHandler {
	return &TracesHandler{
		tracesService: tracesService,
		cache:         cache,
		logger:        logger,
	}
}

// GET /api/v1/traces/services - List all services (Jaeger-compatible)
func (h *TracesHandler) GetServices(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Check Valley cluster cache first
	cacheKey := fmt.Sprintf("trace_services:%s", tenantID)
	if cached, err := h.cache.Get(c.Request.Context(), cacheKey); err == nil {
		var services []string
		if json.Unmarshal(cached, &services) == nil {
			c.Header("X-Cache", "HIT")
			c.JSON(http.StatusOK, gin.H{
				"data": services,
			})
			return
		}
	}

	services, err := h.tracesService.GetServices(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to get trace services", "tenant", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve services",
		})
		return
	}

	// Cache services list for 5 minutes
	h.cache.Set(c.Request.Context(), cacheKey, services, 5*time.Minute)

	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, gin.H{
		"data": services,
	})
}

// GET /api/v1/traces/:traceId - Get specific trace (Jaeger-compatible)
func (h *TracesHandler) GetTrace(c *gin.Context) {
	traceID := c.Param("traceId")
	tenantID := c.GetString("tenant_id")

	if traceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Trace ID is required",
		})
		return
	}

	// Get trace from VictoriaTraces
	trace, err := h.tracesService.GetTrace(c.Request.Context(), traceID, tenantID)
	if err != nil {
		h.logger.Error("Failed to get trace", "traceId", traceID, "tenant", tenantID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"status": "error",
			"error":  "Trace not found",
		})
		return
	}

	// Jaeger-compatible response format
	c.JSON(http.StatusOK, gin.H{
		"data": []map[string]interface{}{
			{
				"traceID":   trace.TraceID,
				"spans":     trace.Spans,
				"processes": trace.Processes,
				"warnings":  nil,
			},
		},
		"total":  0,
		"limit":  0,
		"offset": 0,
		"errors": nil,
	})
}

// POST /api/v1/traces/search - Search traces with filters
func (h *TracesHandler) SearchTraces(c *gin.Context) {
	var request models.TraceSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid trace search request",
		})
		return
	}

	request.TenantID = c.GetString("tenant_id")
	
	traces, err := h.tracesService.SearchTraces(c.Request.Context(), &request)
	if err != nil {
		h.logger.Error("Trace search failed", "tenant", request.TenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Trace search failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"traces": traces.Traces,
			"total":  traces.Total,
		},
		"metadata": gin.H{
			"limit":         request.Limit,
			"searchTime":    traces.SearchTime,
			"tracesFound":   len(traces.Traces),
		},
	})
}
