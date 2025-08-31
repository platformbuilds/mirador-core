package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirador/core/internal/grpc/clients"
	"github.com/mirador/core/internal/models"
	"github.com/mirador/core/internal/services"
	"github.com/mirador/core/pkg/cache"
	"github.com/mirador/core/pkg/logger"
)

type RCAHandler struct {
	rcaClient   *clients.RCAEngineClient
	logsService *services.VictoriaLogsService
	cache       cache.ValkeyCluster
	logger      logger.Logger
}

func NewRCAHandler(
	rcaClient *clients.RCAEngineClient,
	logsService *services.VictoriaLogsService,
	cache cache.ValkeyCluster,
	logger logger.Logger,
) *RCAHandler {
	return &RCAHandler{
		rcaClient:   rcaClient,
		logsService: logsService,
		cache:       cache,
		logger:      logger,
	}
}

// POST /api/v1/rca/investigate - Start RCA investigation with red anchors pattern
func (h *RCAHandler) StartInvestigation(c *gin.Context) {
	var request models.RCAInvestigationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid investigation request",
		})
		return
	}

	// Set tenant context
	request.TenantID = c.GetString("tenant_id")

	// Call MIRADOR-RCA-ENGINE for correlation analysis
	correlation, err := h.rcaClient.InvestigateIncident(c.Request.Context(), &request)
	if err != nil {
		h.logger.Error("RCA investigation failed", "incident", request.IncidentID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "RCA investigation failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"correlation": gin.H{
				"id":               correlation.CorrelationID,
				"incidentId":       correlation.IncidentID,
				"rootCause":        correlation.RootCause,
				"confidence":       correlation.Confidence,
				"affectedServices": correlation.AffectedServices,
				"timeline":         correlation.Timeline,
				"redAnchors":       correlation.RedAnchors, // Anomaly score pattern
				"recommendations":  correlation.Recommendations,
			},
			"investigation": gin.H{
				"startedAt":       correlation.CreatedAt,
				"processingTime":  time.Since(correlation.CreatedAt).Milliseconds(),
				"dataSourcesUsed": []string{"metrics", "logs", "traces"},
				"anchorsFound":    len(correlation.RedAnchors),
			},
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// POST /api/v1/rca/store - Store correlation back to VictoriaLogs as JSON
func (h *RCAHandler) StoreCorrelation(c *gin.Context) {
	var storeRequest models.StoreCorrelationRequest
	if err := c.ShouldBindJSON(&storeRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid store request",
		})
		return
	}

	// Create correlation event for VictoriaLogs storage
	correlationEvent := models.CorrelationEvent{
		ID:         storeRequest.CorrelationID,
		Type:       "rca_correlation",
		IncidentID: storeRequest.IncidentID,
		RootCause:  storeRequest.RootCause,
		Confidence: storeRequest.Confidence,
		RedAnchors: storeRequest.RedAnchors,
		Timeline:   storeRequest.Timeline,
		CreatedAt:  time.Now(),
		TenantID:   c.GetString("tenant_id"),
	}

	// Store as JSON event in VictoriaLogs via MIRADOR-CORE (as per diagram)
	logEntry := map[string]interface{}{
		"_time":       correlationEvent.CreatedAt.Format(time.RFC3339),
		"_msg":        fmt.Sprintf("RCA correlation completed for incident %s", correlationEvent.IncidentID),
		"level":       "info",
		"type":        "rca_correlation",
		"incident_id": correlationEvent.IncidentID,
		"correlation": correlationEvent,
	}

	if err := h.logsService.StoreJSONEvent(c.Request.Context(), logEntry, correlationEvent.TenantID); err != nil {
		h.logger.Error("Failed to store correlation", "correlationId", correlationEvent.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to store correlation event",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"stored":        true,
			"correlationId": correlationEvent.ID,
			"storedAt":      correlationEvent.CreatedAt,
			"format":        "JSON",
			"destination":   "VictoriaLogs",
		},
	})
}
