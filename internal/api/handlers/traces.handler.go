package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/internal/services"
	"github.com/platformbuilds/miradorstack/pkg/cache"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type TracesHandler struct {
	tracesService *services.VictoriaTracesService
	cache         cache.ValkeyCluster
	logger        logger.Logger
}

func NewTracesHandler(tracesService *services.VictoriaTracesService, cache cache.ValkeyCluster, logger logger.Logger) *TracesHandler {
	return &TracesHandler{
		tracesService: tracesService,
		cache:         cache,
		logger:        logger,
	}
}

// GET /api/v1/traces/services - List all services (Jaeger-compatible)
func (h *TracesHandler) GetServices(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Check Valkey cluster cache first
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
			"limit":       request.Limit,
			"searchTime":  traces.SearchTime,
			"tracesFound": len(traces.Traces),
		},
	})
}

// GET /api/v1/traces/services/:service/operations - Get operations for a service (Jaeger-compatible)
func (h *TracesHandler) GetOperations(c *gin.Context) {
	serviceName := c.Param("service")
	tenantID := c.GetString("tenant_id")

	if serviceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Service name is required",
		})
		return
	}

	// Check Valkey cluster cache first
	cacheKey := fmt.Sprintf("trace_operations:%s:%s", tenantID, serviceName)
	if cached, err := h.cache.Get(c.Request.Context(), cacheKey); err == nil {
		var operations []string
		if json.Unmarshal(cached, &operations) == nil {
			c.Header("X-Cache", "HIT")
			c.JSON(http.StatusOK, gin.H{
				"data": operations,
			})
			return
		}
	}

	operations, err := h.tracesService.GetOperations(c.Request.Context(), serviceName, tenantID)
	if err != nil {
		h.logger.Error("Failed to get trace operations",
			"service", serviceName,
			"tenant", tenantID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve operations",
		})
		return
	}

	// Cache operations list for 5 minutes
	h.cache.Set(c.Request.Context(), cacheKey, operations, 5*time.Minute)

	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, gin.H{
		"data": operations,
	})
}
