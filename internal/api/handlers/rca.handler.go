package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/rca"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type RCAHandler struct {
	logsService        *services.VictoriaLogsService
	serviceGraph       services.ServiceGraphFetcher
	cache              cache.ValkeyCluster
	logger             logger.Logger
	featureFlagService *services.RuntimeFeatureFlagService
	rcaEngine          services.RCAEngine
}

func NewRCAHandler(
	logsService *services.VictoriaLogsService,
	serviceGraph services.ServiceGraphFetcher,
	cache cache.ValkeyCluster,
	logger logger.Logger,
	rcaEngine services.RCAEngine,
) *RCAHandler {
	return &RCAHandler{
		logsService:        logsService,
		serviceGraph:       serviceGraph,
		cache:              cache,
		logger:             logger,
		rcaEngine:          rcaEngine,
		featureFlagService: services.NewRuntimeFeatureFlagService(cache, logger),
	}
}

// checkFeatureEnabled checks if the RCA feature is enabled for the current
func (h *RCAHandler) checkFeatureEnabled(c *gin.Context) bool {
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to check feature flags", "error", err)
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

	// RCA investigation via external AI engines has been removed
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "error",
		"error":   "RCA investigation via external AI engines is no longer supported",
		"message": "Use the local RCA engine via /api/v1/unified/rca endpoint instead",
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

	if err := h.logsService.StoreJSONEvent(c.Request.Context(), logEntry); err != nil {
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

// GET /api/v1/rca/correlations - List active correlations (disabled)
func (h *RCAHandler) GetActiveCorrelations(c *gin.Context) {
	// Check if RCA feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RCA feature is disabled",
		})
		return
	}

	// External AI engine correlation listing has been removed
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "error",
		"error":   "Correlation listing via external AI engines is no longer supported",
		"message": "Use the local RCA engine for analysis",
	})
}

// GET /api/v1/rca/patterns - List known failure patterns (disabled)
func (h *RCAHandler) GetFailurePatterns(c *gin.Context) {
	// Check if RCA feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RCA feature is disabled",
		})
		return
	}

	// External AI engine pattern retrieval has been removed
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "error",
		"error":   "Failure pattern retrieval via external AI engines is no longer supported",
		"message": "Use the local RCA engine for analysis",
	})
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
	data, err := h.serviceGraph.FetchServiceGraph(c.Request.Context(), &request)
	if err != nil {
		h.logger.Error("service graph fetch failed", "error", err)
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

// POST /api/v1/unified/rca - Phase 4: Full RCA Engine endpoint
// Accepts an RCARequest and returns an RCAIncident JSON structure
func (h *RCAHandler) HandleComputeRCA(c *gin.Context) {
	var request models.RCARequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Error("Failed to parse RCA request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Invalid request format: %v", err),
		})
		return
	}

	// Validate required fields
	if request.ImpactService == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "impactService is required",
		})
		return
	}

	// Parse time window
	tStart, err := time.Parse(time.RFC3339, request.TimeStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Invalid timeStart format: %v", err),
		})
		return
	}

	tEnd, err := time.Parse(time.RFC3339, request.TimeEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Invalid timeEnd format: %v", err),
		})
		return
	}

	if !tStart.Before(tEnd) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "timeStart must be before timeEnd",
		})
		return
	}

	// Set defaults
	if request.ImpactMetric == "" {
		request.ImpactMetric = "error_rate"
	}
	if request.MetricDirection == "" {
		request.MetricDirection = "higher_is_worse"
	}
	if request.Severity == 0 {
		request.Severity = 0.7 // default medium-high severity
	}
	if request.ImpactSummary == "" {
		request.ImpactSummary = fmt.Sprintf("Incident on %s (metric: %s)", request.ImpactService, request.ImpactMetric)
	}

	// Build IncidentContext
	tPeak := tStart.Add(tEnd.Sub(tStart) / 2) // Use midpoint as peak
	incidentContext := &rca.IncidentContext{
		ID:            fmt.Sprintf("incident_%d", time.Now().UnixNano()),
		ImpactService: request.ImpactService,
		ImpactSignal: rca.ImpactSignal{
			ServiceName: request.ImpactService,
			MetricName:  request.ImpactMetric,
			Direction:   request.MetricDirection,
			Labels:      make(map[string]string),
			Threshold:   0.0,
		},
		TimeBounds: rca.IncidentTimeWindow{
			TStart: tStart,
			TPeak:  tPeak,
			TEnd:   tEnd,
		},
		ImpactSummary: request.ImpactSummary,
		Severity:      request.Severity,
		CreatedAt:     time.Now().UTC(),
	}

	// Build RCAOptions
	opts := rca.DefaultRCAOptions()
	if request.MaxChains > 0 {
		opts.MaxChains = request.MaxChains
	}
	if request.MaxStepsPerChain > 0 {
		opts.MaxStepsPerChain = request.MaxStepsPerChain
	}
	if request.MinScoreThreshold > 0 {
		opts.MinScoreThreshold = request.MinScoreThreshold
	}

	// Call RCA engine
	rcaIncident, err := h.rcaEngine.ComputeRCA(c.Request.Context(), incidentContext, opts)
	if err != nil {
		h.logger.Error("RCA computation failed", "incident_id", incidentContext.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("RCA computation failed: %v", err),
		})
		return
	}

	// Convert to DTO
	dto := h.convertRCAIncidentToDTO(rcaIncident)

	// Return response
	response := models.RCAResponse{
		Status:    "success",
		Data:      dto,
		Timestamp: time.Now().UTC(),
	}

	c.JSON(http.StatusOK, response)
}

// convertRCAIncidentToDTO converts internal RCAIncident to DTO for JSON serialization
func (h *RCAHandler) convertRCAIncidentToDTO(ri *rca.RCAIncident) *models.RCAIncidentDTO {
	if ri == nil {
		return nil
	}

	dto := &models.RCAIncidentDTO{
		GeneratedAt: ri.GeneratedAt,
		Score:       ri.Score,
		Notes:       ri.Notes,
		Chains:      make([]*models.RCAChainDTO, 0),
	}

	// Convert impact
	if ri.Impact != nil {
		dto.Impact = &models.IncidentContextDTO{
			ID:            ri.Impact.ID,
			ImpactService: ri.Impact.ImpactService,
			MetricName:    ri.Impact.ImpactSignal.MetricName,
			TimeStartStr:  ri.Impact.TimeBounds.TStart.Format(time.RFC3339),
			TimeEndStr:    ri.Impact.TimeBounds.TEnd.Format(time.RFC3339),
			ImpactSummary: ri.Impact.ImpactSummary,
			Severity:      float64(ri.Impact.Severity),
		}
	}

	// Convert root cause
	if ri.RootCause != nil {
		dto.RootCause = h.convertRCAStepToDTO(ri.RootCause)
	}

	// Convert chains
	for _, chain := range ri.Chains {
		chainDTO := &models.RCAChainDTO{
			Score:        chain.Score,
			Rank:         chain.Rank,
			ImpactPath:   chain.ImpactPath,
			DurationHops: chain.DurationHops,
			Steps:        make([]*models.RCAStepDTO, 0),
		}

		for _, step := range chain.Steps {
			stepDTO := h.convertRCAStepToDTO(step)
			if stepDTO != nil {
				chainDTO.Steps = append(chainDTO.Steps, stepDTO)
			}
		}

		dto.Chains = append(dto.Chains, chainDTO)
	}

	return dto
}

// convertRCAStepToDTO converts an RCAStep to DTO
func (h *RCAHandler) convertRCAStepToDTO(step *rca.RCAStep) *models.RCAStepDTO {
	if step == nil {
		return nil
	}

	dto := &models.RCAStepDTO{
		WhyIndex:  step.WhyIndex,
		Service:   step.Service,
		Component: step.Component,
		TimeStart: step.TimeRange.Start,
		TimeEnd:   step.TimeRange.End,
		Ring:      step.Ring.String(),
		Direction: string(step.Direction),
		Distance:  step.Distance,
		Summary:   step.Summary,
		Score:     step.Score,
		Evidence:  make([]*models.EvidenceRefDTO, 0),
	}

	for _, ev := range step.Evidence {
		evDTO := &models.EvidenceRefDTO{
			Type:    ev.Type,
			ID:      ev.ID,
			Details: ev.Details,
		}
		dto.Evidence = append(dto.Evidence, evDTO)
	}

	return dto
}
