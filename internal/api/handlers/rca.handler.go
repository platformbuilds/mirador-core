package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type RCAHandler struct {
	rcaClient          clients.RCAClient
	logsService        *services.VictoriaLogsService
	serviceGraph       services.ServiceGraphFetcher
	cache              cache.ValkeyCluster
	logger             logger.Logger
	featureFlagService *services.RuntimeFeatureFlagService
}

func NewRCAHandler(
	rcaClient clients.RCAClient,
	logsService *services.VictoriaLogsService,
	serviceGraph services.ServiceGraphFetcher,
	cache cache.ValkeyCluster,
	logger logger.Logger,
) *RCAHandler {
	return &RCAHandler{
		rcaClient:          rcaClient,
		logsService:        logsService,
		serviceGraph:       serviceGraph,
		cache:              cache,
		logger:             logger,
		featureFlagService: services.NewRuntimeFeatureFlagService(cache, logger),
	}
}

// checkFeatureEnabled checks if the RCA feature is enabled for the current tenant
func (h *RCAHandler) checkFeatureEnabled(c *gin.Context) bool {
	tenantID := c.GetString("tenant_id")
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to check feature flags", "tenantID", tenantID, "error", err)
		return false
	}
	return flags.RCAEnabled
}

// POST /api/v1/rca/investigate - Start RCA investigation with red anchors pattern
func (h *RCAHandler) StartInvestigation(c *gin.Context) {
	// Check if RCA feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RCA feature is disabled",
		})
		return
	}

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
			// Return the struct directly so json tags (snake_case) are preserved
			"correlation": correlation,
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
	// Check if RCA feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RCA feature is disabled",
		})
		return
	}

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

// GET /api/v1/rca/correlations - List active correlations (stubbed; fill with gRPC later)
func (h *RCAHandler) GetActiveCorrelations(c *gin.Context) {
	// Check if RCA feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RCA feature is disabled",
		})
		return
	}

	tenantID := c.GetString("tenant_id")

	// Optional filters
	service := c.Query("service")
	limitStr := c.DefaultQuery("limit", "50")

	// TODO: Replace with h.rcaClient.ListActiveCorrelations(...)
	// Minimal response to unblock compilation and UI wiring
	resp := gin.H{
		"status":   "success",
		"tenantId": tenantID,
		"filters":  gin.H{"service": service, "limit": limitStr},
		"data": gin.H{
			"correlations": []interface{}{}, // fill with real items later
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, resp)
}

// GET /api/v1/rca/patterns - List known failure patterns (stubbed; fill with gRPC later)
func (h *RCAHandler) GetFailurePatterns(c *gin.Context) {
	// Check if RCA feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RCA feature is disabled",
		})
		return
	}

	tenantID := c.GetString("tenant_id")

	// Optional filters
	service := c.Query("service")
	since := c.Query("since") // e.g., RFC3339 or unix seconds

	// TODO: Replace with h.rcaClient.GetPatterns(...) once proto/engine is ready
	resp := gin.H{
		"status":   "success",
		"tenantId": tenantID,
		"filters":  gin.H{"service": service, "since": since},
		"data": gin.H{
			"patterns": []interface{}{}, // fill with real patterns later
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, resp)
}

// POST /api/v1/rca/service-graph - Aggregate service dependency metrics.
func (h *RCAHandler) GetServiceGraph(c *gin.Context) {
	// Check if RCA feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RCA feature is disabled",
		})
		return
	}

	if h.serviceGraph == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"error":  "service graph metrics not configured",
		})
		return
	}

	var request models.ServiceGraphRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "invalid service graph request",
		})
		return
	}

	if request.Start.IsZero() || request.End.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "start and end must be provided",
		})
		return
	}

	tenantID := c.GetString("tenant_id")
	data, err := h.serviceGraph.FetchServiceGraph(c.Request.Context(), tenantID, &request)
	if err != nil {
		h.logger.Error("service graph fetch failed", "tenant", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "failed to fetch service graph",
		})
		return
	}

	if data == nil {
		data = &models.ServiceGraphData{Edges: []models.ServiceGraphEdge{}}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"generated_at": time.Now().UTC(),
		"window":       data.Window,
		"edges":        data.Edges,
	})
}
