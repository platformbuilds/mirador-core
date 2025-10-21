package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/utils"
	"github.com/platformbuilds/mirador-core/internal/utils/search"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type TracesHandler struct {
	tracesService *services.VictoriaTracesService
	cache         cache.ValkeyCluster
	logger        logger.Logger
	searchRouter  *search.SearchRouter
	config        *config.Config
}

func NewTracesHandler(tracesService *services.VictoriaTracesService, cache cache.ValkeyCluster, logger logger.Logger, searchRouter *search.SearchRouter, config *config.Config) *TracesHandler {
	return &TracesHandler{
		tracesService: tracesService,
		cache:         cache,
		logger:        logger,
		searchRouter:  searchRouter,
		config:        config,
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
		// Degraded mode: log and return empty list so UI can continue to load.
		h.logger.Error("Failed to get trace services", "tenant", tenantID, "error", err)
		c.Header("X-Backend-Degraded", "victoria_traces")
		c.Header("X-Cache", "MISS")
		c.JSON(http.StatusOK, gin.H{
			"data":     []string{},
			"metadata": gin.H{"degraded": true, "backend": "victoria_traces"},
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

	// Determine search engine (default to lucene for backward compatibility)
	searchEngine := request.SearchEngine
	if searchEngine == "" {
		searchEngine = "lucene"
	}

	// Check feature flags for Bleve access
	if searchEngine == "bleve" {
		featureFlags := h.config.GetFeatureFlags(request.TenantID)
		if !featureFlags.BleveSearch || !featureFlags.BleveTraces {
			c.JSON(http.StatusForbidden, gin.H{
				"status": "error",
				"error":  "Bleve search engine is not enabled for this tenant",
			})
			return
		}
	}

	// Validate that the requested engine is supported
	if !h.searchRouter.IsEngineSupported(searchEngine) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Unsupported search engine: %s", searchEngine),
		})
		return
	}

	// Validate query based on search engine (only if query is provided)
	if request.Query != "" {
		validator := utils.NewQueryValidator()
		if searchEngine == "bleve" {
			if err := validator.ValidateBleve(request.Query); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status": "error",
					"error":  fmt.Sprintf("Invalid Bleve query: %s", err.Error()),
				})
				return
			}
		} else {
			// Default to Lucene validation for backward compatibility
			if err := validator.ValidateLucene(request.Query); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status": "error",
					"error":  fmt.Sprintf("Invalid Lucene query: %s", err.Error()),
				})
				return
			}
		}
	}

	c.Header("X-Query-Translated-From", request.Query)
	c.Header("X-Search-Engine", searchEngine)

	// Translate the query to trace filters only if query is provided
	if request.Query != "" {
		translator, err := h.searchRouter.GetTranslator(searchEngine)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Failed to get translator for engine %s: %s", searchEngine, err.Error()),
			})
			return
		}

		// Translate the query to trace filters
		filters, err := translator.TranslateToTraces(request.Query)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Failed to translate query with %s engine: %s", searchEngine, err.Error()),
			})
			return
		}

		// Apply the translated filters to the request
		if filters.Service != "" {
			request.Service = filters.Service
		}
		if filters.Operation != "" {
			request.Operation = filters.Operation
		}
		// Tags: join as comma-separated key=value
		if len(filters.Tags) > 0 {
			parts := make([]string, 0, len(filters.Tags))
			for k, v := range filters.Tags {
				// preserve value as-is; quote not needed for Jaeger tags param (string)
				parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			}
			// stable order helps caching
			sort.Strings(parts)
			request.Tags = strings.Join(parts, ",")
		}
		if filters.MinDuration != "" {
			request.MinDuration = filters.MinDuration
		}
		if filters.MaxDuration != "" {
			request.MaxDuration = filters.MaxDuration
		}
		// Time window: prefer explicit [start TO end], else use Since shorthand
		now := time.Now().UTC()
		if filters.StartExpr != "" || filters.EndExpr != "" {
			// Parse simple forms: now, now-<dur>, RFC3339 timestamps
			if t, ok := parseNowLike(filters.StartExpr, now); ok {
				request.Start = models.FlexibleTime{Time: t}
			}
			if t, ok := parseNowLike(filters.EndExpr, now); ok {
				request.End = models.FlexibleTime{Time: t}
			}
		} else if filters.Since != "" {
			if dur, err := time.ParseDuration(filters.Since); err == nil {
				request.End = models.FlexibleTime{Time: now}
				request.Start = models.FlexibleTime{Time: now.Add(-dur)}
			}
		}
	}

	traces, err := h.tracesService.SearchTraces(c.Request.Context(), &request)
	if err != nil {
		// Degraded mode: return success with empty results
		h.logger.Error("Trace search failed", "tenant", request.TenantID, "error", err)
		c.Header("X-Backend-Degraded", "victoria_traces")
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"traces": []interface{}{},
				"total":  0,
			},
			"metadata": gin.H{
				"limit":       request.Limit,
				"searchTime":  0,
				"tracesFound": 0,
				"degraded":    true,
				"backend":     "victoria_traces",
			},
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

// parseNowLike parses simple expressions useful for Lucene time windows:
// - "now" → now
// - "now-15m" → now - 15m
// - RFC3339 / RFC3339Nano timestamps
func parseNowLike(expr string, now time.Time) (time.Time, bool) {
	e := strings.TrimSpace(expr)
	if e == "" {
		return time.Time{}, false
	}
	lower := strings.ToLower(e)
	if lower == "now" {
		return now, true
	}
	if strings.HasPrefix(lower, "now-") {
		d := strings.TrimPrefix(lower, "now-")
		if dur, err := time.ParseDuration(d); err == nil {
			return now.Add(-dur), true
		}
	}
	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, e); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339Nano, e); err == nil {
		return t, true
	}
	return time.Time{}, false
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
		// Degraded mode: log and return empty list
		h.logger.Error("Failed to get trace operations",
			"service", serviceName,
			"tenant", tenantID,
			"error", err,
		)
		c.Header("X-Backend-Degraded", "victoria_traces")
		c.Header("X-Cache", "MISS")
		c.JSON(http.StatusOK, gin.H{
			"data":     []string{},
			"metadata": gin.H{"degraded": true, "backend": "victoria_traces"},
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

// -------------------- Endpoints --------------------

// GET /api/v1/traces/:traceId/flamegraph - D3-friendly flame graph for a single trace
func (h *TracesHandler) GetFlameGraph(c *gin.Context) {
	traceID := c.Param("traceId")
	tenantID := c.GetString("tenant_id")
	mode := c.DefaultQuery("mode", string(utils.FlameDuration))
	if traceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Trace ID is required"})
		return
	}
	tr, err := h.tracesService.GetTrace(c.Request.Context(), traceID, tenantID)
	if err != nil {
		h.logger.Error("Failed to get trace for flamegraph", "traceId", traceID, "tenant", tenantID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": "Trace not found"})
		return
	}
	fg := utils.BuildFlameGraphFromJaegerWithMode(tr.TraceID, tr.Spans, tr.Processes, mode)
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   fg,
	})
}

// POST /api/v1/traces/flamegraph/search - Aggregate flame graph over search results
func (h *TracesHandler) SearchFlameGraph(c *gin.Context) {
	var request models.TraceSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Invalid trace search request"})
		return
	}
	request.TenantID = c.GetString("tenant_id")
	mode := c.DefaultQuery("mode", string(utils.FlameDuration))

	res, err := h.tracesService.SearchTraces(c.Request.Context(), &request)
	if err != nil {
		h.logger.Error("Trace search for flamegraph failed", "tenant", request.TenantID, "error", err)
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": gin.H{"name": "aggregate (0 traces)", "value": 0}})
		return
	}

	root := utils.FlameNode{Name: "aggregate (0 traces)", Value: 0}
	count := 0
	for _, it := range res.Traces {
		spansRaw, ok1 := it["spans"].([]interface{})
		procsRaw, ok2 := it["processes"].(map[string]interface{})
		traceID, _ := it["traceID"].(string)
		if !ok1 || !ok2 {
			continue
		}
		spans := make([]map[string]any, 0, len(spansRaw))
		for _, s := range spansRaw {
			if m, ok := s.(map[string]interface{}); ok {
				spans = append(spans, m)
			}
		}
		fg := utils.BuildFlameGraphFromJaegerWithMode(traceID, spans, procsRaw, mode)
		if count == 0 {
			root.Name = "aggregate (1 trace)"
		} else {
			root.Name = "aggregate (" + strconv.FormatInt(int64(count+1), 10) + " traces)"
		}
		utils.MergeFlameTrees(&root, fg)
		count++
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": root})
}
