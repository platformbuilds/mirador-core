// ================================
// internal/api/handlers/logsql.handler.go - LogsQL API Handler
// ================================

package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/miradorstack/internal/metrics"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/internal/services"
	"github.com/platformbuilds/miradorstack/internal/utils"
	"github.com/platformbuilds/miradorstack/pkg/cache"
	"github.com/platformbuilds/miradorstack/pkg/logger"
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
