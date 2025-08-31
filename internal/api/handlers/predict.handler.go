package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/miradorstack/internal/grpc/clients"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/internal/services"
	"github.com/platformbuilds/miradorstack/pkg/cache"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type PredictHandler struct {
	predictClient *clients.PredictEngineClient
	logsService   *services.VictoriaLogsService
	cache         cache.ValkeyCluster
	logger        logger.Logger
}

func NewPredictHandler(
	predictClient *clients.PredictEngineClient,
	logsService *services.VictoriaLogsService,
	cache cache.ValkeyCluster,
	logger logger.Logger,
) *PredictHandler {
	return &PredictHandler{
		predictClient: predictClient,
		logsService:   logsService,
		cache:         cache,
		logger:        logger,
	}
}

// POST /api/v1/predict/analyze - Analyze potential system fractures
func (h *PredictHandler) AnalyzeFractures(c *gin.Context) {
	var request models.FractureAnalysisRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate tenant access
	tenantID := c.GetString("tenant_id")
	request.TenantID = tenantID

	// Call MIRADOR-PREDICT-ENGINE via gRPC + protobuf
	response, err := h.predictClient.AnalyzeFractures(c.Request.Context(), &request)
	if err != nil {
		h.logger.Error("Fracture analysis failed", "component", request.Component, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Fracture analysis failed",
		})
		return
	}

	// Store each prediction back to VictoriaLogs as JSON event
	// (as specified in the architecture diagram)
	for _, fracture := range response.Fractures {
		predictionEvent := models.PredictionEvent{
			ID:           fracture.ID,
			Type:         "system_fracture_prediction",
			Component:    fracture.Component,
			PredictedAt:  fracture.PredictedAt,
			IncidentTime: fracture.PredictedAt.Add(fracture.TimeToFracture),
			Probability:  fracture.Probability,
			Severity:     fracture.Severity,
			Confidence:   fracture.Confidence,
			TenantID:     tenantID,
			Metadata: map[string]interface{}{
				"fracture_type":        fracture.FractureType,
				"contributing_factors": fracture.ContributingFactors,
				"recommendation":       fracture.Recommendation,
			},
		}

		// Store to VictoriaLogs via MIRADOR-CORE (as shown in diagram)
		if err := h.storeEventToVictoriaLogs(c.Request.Context(), predictionEvent); err != nil {
			h.logger.Error("Failed to store prediction event", "predictionId", fracture.ID, "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"fractures": response.Fractures,
			"metadata": gin.H{
				"modelsUsed":     response.ModelsUsed,
				"processingTime": response.ProcessingTimeMs,
				"totalFractures": len(response.Fractures),
				"storedEvents":   len(response.Fractures),
			},
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GET /api/v1/predict/fractures - Get list of predicted fractures
func (h *PredictHandler) GetPredictedFractures(c *gin.Context) {
	timeRange := c.DefaultQuery("time_range", "24h")
	minProbability := c.DefaultQuery("min_probability", "0.7")

	// Use valley cluster cache for faster fetch (as noted in diagram)
	cacheKey := fmt.Sprintf("fractures:%s:%s:%s", c.GetString("tenant_id"), timeRange, minProbability)
	if cached, err := h.cache.Get(c.Request.Context(), cacheKey); err == nil {
		var cachedResponse models.FractureListResponse
		if json.Unmarshal(cached, &cachedResponse) == nil {
			c.Header("X-Cache", "HIT")
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"data":   cachedResponse,
			})
			return
		}
	}

	// Query from VictoriaLogs for stored prediction events
	query := fmt.Sprintf(`_time:%s type:"system_fracture_prediction" probability:>%s`, timeRange, minProbability)
	fractures, err := h.logsService.QueryPredictionEvents(c.Request.Context(), query, c.GetString("tenant_id"))
	if err != nil {
		h.logger.Error("Failed to query fracture predictions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve fracture predictions",
		})
		return
	}

	response := models.FractureListResponse{
		Fractures: fractures,
		Summary: models.FractureSummary{
			Total:         len(fractures),
			HighRisk:      countByRisk(fractures, "high"),
			MediumRisk:    countByRisk(fractures, "medium"),
			LowRisk:       countByRisk(fractures, "low"),
			AvgTimeToFail: calculateAvgTimeToFailure(fractures),
		},
	}

	// Cache the response in Valkey cluster
	h.cache.Set(c.Request.Context(), cacheKey, response, 5*time.Minute)

	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"data":      response,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// storeEventToVictoriaLogs stores prediction events as JSON in VictoriaLogs
func (h *PredictHandler) storeEventToVictoriaLogs(ctx context.Context, event models.PredictionEvent) error {
	// Convert to LogsQL-compatible JSON format
	logEntry := map[string]interface{}{
		"_time":      event.PredictedAt.Format(time.RFC3339),
		"_msg":       fmt.Sprintf("System fracture prediction for %s", event.Component),
		"level":      "info",
		"type":       event.Type,
		"component":  event.Component,
		"prediction": event,
	}

	return h.logsService.StoreJSONEvent(ctx, logEntry, event.TenantID)
}

// countByRisk returns how many fractures fall into the given risk level.
// It prefers the explicit Severity on the fracture; if empty, it derives
// the level from Probability (>=0.90=high, >=0.70=medium, else low).
func countByRisk(fractures []*models.SystemFracture, level string) int {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		return 0
	}
	n := 0
	for _, f := range fractures {
		if f == nil {
			continue
		}
		if deriveRiskLevel(f) == level {
			n++
		}
	}
	return n
}

func deriveRiskLevel(f *models.SystemFracture) string {
	// 1) Use explicit severity if present
	if s := strings.ToLower(strings.TrimSpace(f.Severity)); s != "" {
		switch s {
		case "critical", "high":
			return "high"
		case "medium", "moderate":
			return "medium"
		case "low":
			return "low"
		}
	}
	// 2) Fallback to probability thresholds
	p := f.Probability
	switch {
	case p >= 0.90:
		return "high"
	case p >= 0.70:
		return "medium"
	default:
		return "low"
	}
}

// calculateAvgTimeToFailure returns the average TimeToFracture across all
// fractures that have a positive duration.
func calculateAvgTimeToFailure(fractures []*models.SystemFracture) time.Duration {
	var sum time.Duration
	var count int64
	for _, f := range fractures {
		if f == nil {
			continue
		}
		if f.TimeToFracture > 0 {
			sum += f.TimeToFracture
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return time.Duration(int64(sum) / count)
}
