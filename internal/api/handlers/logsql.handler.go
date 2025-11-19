package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/metrics"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/utils"
	lq "github.com/platformbuilds/mirador-core/internal/utils/lucene"
	"github.com/platformbuilds/mirador-core/internal/utils/search"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type LogsQLHandler struct {
	logsService  *services.VictoriaLogsService
	cache        cache.ValkeyCluster
	logger       logger.Logger
	validator    *utils.QueryValidator
	searchRouter *search.SearchRouter
	config       *config.Config
}

func NewLogsQLHandler(logsService *services.VictoriaLogsService, cache cache.ValkeyCluster, logger logger.Logger, searchRouter *search.SearchRouter, config *config.Config) *LogsQLHandler {
	return &LogsQLHandler{
		logsService:  logsService,
		cache:        cache,
		logger:       logger,
		validator:    utils.NewQueryValidator(),
		searchRouter: searchRouter,
		config:       config,
	}
}

// POST /api/v1/logs/query - Execute LogsQL query
func (h *LogsQLHandler) ExecuteQuery(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Panic in LogsQL query handler", "panic", r)
			// If response not written yet, write error
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status": "error",
					"error":  "Internal server error",
				})
			}
			// If response already written, just log
		}
	}()

	start := time.Now()
	var executionTime time.Duration

	var request models.LogsQLQueryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid LogsQL request format",
		})
		return
	}

	// Determine search engine (default to lucene for backward compatibility)
	searchEngine := request.SearchEngine
	if searchEngine == "" {
		searchEngine = "lucene"
	}

	// Determine query language (default to lucene for backward compatibility)
	queryLanguage := request.QueryLanguage
	if queryLanguage == "" {
		queryLanguage = "lucene"
	}

	// Check feature flags for Bleve access
	if searchEngine == "bleve" {
		featureFlags := h.config.GetFeatureFlags()
		if !featureFlags.BleveSearch || !featureFlags.BleveLogs {
			c.JSON(http.StatusForbidden, gin.H{
				"status": "error",
				"error":  "Bleve search engine is not enabled",
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

	// Validate that the query language is supported
	if !h.searchRouter.IsEngineSupported(queryLanguage) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Unsupported query language: %s", queryLanguage),
		})
		return
	}

	// Validate query based on query language
	if queryLanguage == "bleve" {
		if err := h.validator.ValidateBleve(request.Query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Invalid Bleve query: %s", err.Error()),
			})
			return
		}
	} else {
		// Skip Lucene validation for now, as VictoriaLogs accepts extended syntax
		// if err := h.validator.ValidateLucene(request.Query); err != nil {
		//     c.JSON(http.StatusBadRequest, gin.H{
		//         "status": "error",
		//         "error":  fmt.Sprintf("Invalid Lucene query: %s", err.Error()),
		//     })
		//     return
		// }
	}

	// If query already contains an explicit _time filter, drop Start/End to avoid conflicts.
	if strings.Contains(request.Query, "_time:") {
		request.Start = 0
		request.End = 0
	}

	c.Header("X-Search-Engine", searchEngine)
	c.Header("X-Query-Language", queryLanguage)

	// Check cache for Bleve queries before executing
	var cacheKey string
	if searchEngine == "bleve" {
		cacheKey = h.generateQueryCacheKey(&request, searchEngine)
		if cachedResult, err := h.getCachedQueryResult(c.Request.Context(), cacheKey); err == nil && cachedResult != nil {
			// Cache hit - return cached result
			c.Header("X-Cache", "HIT")
			c.Header("X-Search-Engine", searchEngine)
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"data": gin.H{
					"logs":   cachedResult.Logs,
					"fields": cachedResult.Fields,
					"stats":  cachedResult.Stats,
				},
				"metadata": gin.H{
					"executionTime": 0,
					"logCount":      len(cachedResult.Logs),
					"fieldsFound":   len(cachedResult.Fields),
					"cached":        true,
				},
			})
			return
		}
		c.Header("X-Cache", "MISS")
	}

	// Execute LogsQL query
	result, err := h.logsService.ExecuteQuery(c.Request.Context(), &request)
	if err != nil {
		executionTime := time.Since(start)
		h.logger.Error("LogsQL query execution failed",
			"query", request.Query,
			"error", err,
			"executionTime", executionTime,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "LogsQL query execution failed",
		})
		return
	}

	executionTime = time.Since(start)
	metrics.QueryExecutionDuration.WithLabelValues("logsql").Observe(executionTime.Seconds())

	// Cache the result for Bleve queries
	if searchEngine == "bleve" && cacheKey != "" {
		h.cacheQueryResult(c.Request.Context(), cacheKey, result, 10*time.Minute) // Cache for 10 minutes
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"logs":   result.Logs,
			"fields": result.Fields,
			"stats":  result.Stats,
		},
		"metadata": gin.H{
			"executionTime": executionTime.Milliseconds(),
			"logCount":      len(result.Logs),
			"fieldsFound":   len(result.Fields),
		},
	})
}

// generateQueryCacheKey generates a cache key for query results
func (h *LogsQLHandler) generateQueryCacheKey(request *models.LogsQLQueryRequest, searchEngine string) string {
	// Create a deterministic key based on query parameters
	key := fmt.Sprintf("bleve_logs:%s:%d:%d:%d",
		request.Query,
		request.Start,
		request.End,
		request.Limit)
	return key
}

// getCachedQueryResult retrieves a cached query result
func (h *LogsQLHandler) getCachedQueryResult(ctx context.Context, cacheKey string) (*models.LogsQLQueryResult, error) {
	cached, err := h.cache.GetCachedQueryResult(ctx, cacheKey)
	if err != nil {
		monitoring.RecordBleveCacheOperation("get", "error")
		return nil, err
	}

	if len(cached) == 0 {
		monitoring.RecordBleveCacheOperation("get", "miss")
		return nil, fmt.Errorf("cache miss")
	}

	var result models.LogsQLQueryResult
	if err := json.Unmarshal(cached, &result); err != nil {
		h.logger.Warn("Failed to unmarshal cached query result", "error", err)
		monitoring.RecordBleveCacheOperation("get", "error")
		return nil, err
	}

	monitoring.RecordBleveCacheOperation("get", "hit")
	return &result, nil
}

// cacheQueryResult caches a query result
func (h *LogsQLHandler) cacheQueryResult(ctx context.Context, cacheKey string, result *models.LogsQLQueryResult, ttl time.Duration) {
	data, err := json.Marshal(result)
	if err != nil {
		h.logger.Warn("Failed to marshal query result for caching", "error", err)
		monitoring.RecordBleveCacheOperation("set", "error")
		return
	}

	if err := h.cache.CacheQueryResult(ctx, cacheKey, data, ttl); err != nil {
		h.logger.Warn("Failed to cache query result", "error", err)
		monitoring.RecordBleveCacheOperation("set", "error")
		return
	}

	monitoring.RecordBleveCacheOperation("set", "success")
}

// GET /api/v1/logs/streams - Get available log streams
func (h *LogsQLHandler) GetStreams(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	streams, err := h.logsService.GetStreams(c.Request.Context(), limit)
	if err != nil {
		h.logger.Error("Failed to get log streams", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve log streams",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"streams": streams,
			"total":   len(streams),
		},
	})
}

// POST /api/v1/logs/store - Store JSON events from AI engines
func (h *LogsQLHandler) StoreEvent(c *gin.Context) {
	var event map[string]interface{}
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid JSON event format",
		})
		return
	}

	// Add metadata
	event["_time"] = time.Now().Format(time.RFC3339)
	event["stored_by"] = "mirador-core"

	// Store in VictoriaLogs
	if err := h.logsService.StoreJSONEvent(c.Request.Context(), event); err != nil {
		h.logger.Error("Failed to store JSON event", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to store event",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"stored":    true,
			"timestamp": event["_time"],
		},
	})
}

// GET /api/v1/logs/fields - Get available log fields
func (h *LogsQLHandler) GetFields(c *gin.Context) {

	fields, err := h.logsService.GetFields(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get log fields", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve log fields",
		})
		return
	}

	if fields == nil {
		fields = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"fields": fields,
		},
	})
}

// POST /api/v1/logs/export - Export logs in various formats
func (h *LogsQLHandler) ExportLogs(c *gin.Context) {

	var request models.LogExportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "Invalid export request format",
			"details": err.Error(),
		})
		return
	}

	// Translate Lucene -> LogsQL if requested or detected
	if strings.EqualFold(request.QueryLanguage, "lucene") || lq.IsLikelyLucene(request.Query) {
		validator := utils.NewQueryValidator()
		if err := validator.ValidateLucene(request.Query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"error":   fmt.Sprintf("Invalid Lucene query: %s", err.Error()),
				"details": err.Error(),
			})
			return
		}
		if translated, ok := lq.Translate(request.Query, lq.TargetLogsQL); ok {
			request.Query = translated
			c.Header("X-Query-Translated-From", "lucene")
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"error":   "Failed to translate Lucene query",
				"details": "Translation failed",
			})
			return
		}
	}

	// If query already contains an explicit _time filter, drop Start/End to avoid conflicts.
	if strings.Contains(request.Query, "_time:") {
		request.Start = 0
		request.End = 0
	}

	// Validate export format
	if request.Format == "" {
		request.Format = "json" // Default format
	}

	// Export logs via VictoriaLogs service
	exportResult, err := h.logsService.ExportLogs(c.Request.Context(), &request)
	if err != nil {
		h.logger.Error("Log export failed",
			"query", request.Query,
			"format", request.Format,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Log export failed",
		})
		return
	}

	// Stream the exported file bytes directly as an attachment.
	// This avoids inventing a temporary URL and matches the service, which
	// already fetched the full payload from VictoriaLogs.
	var contentType string
	switch request.Format {
	case "csv":
		contentType = "text/csv"
	case "json":
		contentType = "application/json"
	default:
		contentType = "application/octet-stream"
	}

	filename := exportResult.Filename
	if filename == "" {
		// Fallback filename based on format
		filename = fmt.Sprintf("logs-%s.%s", time.Now().Format("2006-01-02"), request.Format)
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.Itoa(len(exportResult.Data)))
	c.Data(http.StatusOK, contentType, exportResult.Data)
}
