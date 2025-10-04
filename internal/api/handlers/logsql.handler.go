package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/metrics"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/utils"
	lq "github.com/platformbuilds/mirador-core/internal/utils/lucene"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type LogsQLHandler struct {
	logsService *services.VictoriaLogsService
	cache       cache.ValkeyCluster
	logger      logger.Logger
	validator   *utils.QueryValidator
}

func NewLogsQLHandler(logsService *services.VictoriaLogsService, cache cache.ValkeyCluster, logger logger.Logger) *LogsQLHandler {
	return &LogsQLHandler{
		logsService: logsService,
		cache:       cache,
		logger:      logger,
		validator:   utils.NewQueryValidator(),
	}
}

// POST /api/v1/logs/query - Execute LogsQL query
func (h *LogsQLHandler) ExecuteQuery(c *gin.Context) {
	start := time.Now()
	tenantID := c.GetString("tenant_id")

	var request models.LogsQLQueryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid LogsQL request format",
		})
		return
	}

	// Translate Lucene -> LogsQL if requested or detected
	if strings.EqualFold(request.QueryLanguage, "lucene") || lq.IsLikelyLucene(request.Query) {
		validator := utils.NewQueryValidator()
		if err := validator.ValidateLucene(request.Query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Invalid Lucene query: %s", err.Error()),
			})
			return
		}
		if translated, ok := lq.Translate(request.Query, lq.TargetLogsQL); ok {
			request.Query = translated
			c.Header("X-Query-Translated-From", "lucene")
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Failed to translate Lucene query",
			})
			return
		}
	}

	// If query already contains an explicit _time filter, drop Start/End to avoid conflicts.
	if strings.Contains(request.Query, "_time:") {
		request.Start = 0
		request.End = 0
	}

	// Validate LogsQL query
	if err := h.validator.ValidateLogsQL(request.Query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Invalid LogsQL query: %s", err.Error()),
		})
		return
	}

	// Execute LogsQL query
	request.TenantID = tenantID
	result, err := h.logsService.ExecuteQuery(c.Request.Context(), &request)
	if err != nil {
		executionTime := time.Since(start)
		h.logger.Error("LogsQL query execution failed",
			"query", request.Query,
			"tenant", tenantID,
			"error", err,
			"executionTime", executionTime,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "LogsQL query execution failed",
		})
		return
	}

	executionTime := time.Since(start)
	metrics.QueryExecutionDuration.WithLabelValues("logsql", tenantID).Observe(executionTime.Seconds())

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

// GET /api/v1/logs/streams - Get available log streams
func (h *LogsQLHandler) GetStreams(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	streams, err := h.logsService.GetStreams(c.Request.Context(), tenantID, limit)
	if err != nil {
		h.logger.Error("Failed to get log streams", "tenant", tenantID, "error", err)
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

	tenantID := c.GetString("tenant_id")

	// Add metadata
	event["_time"] = time.Now().Format(time.RFC3339)
	event["tenant_id"] = tenantID
	event["stored_by"] = "mirador-core"

	// Store in VictoriaLogs
	if err := h.logsService.StoreJSONEvent(c.Request.Context(), event, tenantID); err != nil {
		h.logger.Error("Failed to store JSON event", "tenant", tenantID, "error", err)
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
			"tenantId":  tenantID,
		},
	})
}

// GET /api/v1/logs/fields - Get available log fields
func (h *LogsQLHandler) GetFields(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	fields, err := h.logsService.GetFields(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to get log fields", "tenant", tenantID, "error", err)
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
	tenantID := c.GetString("tenant_id")

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

	// Set tenant context
	request.TenantID = tenantID

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
			"tenant", tenantID,
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
