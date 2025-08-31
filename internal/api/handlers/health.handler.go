package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirador/core/internal/grpc/clients"
	"github.com/mirador/core/internal/services"
	"github.com/mirador/core/pkg/logger"
)

type HealthHandler struct {
	grpcClients *clients.GRPCClients
	vmServices  *services.VictoriaMetricsServices
	logger      logger.Logger
}

func NewHealthHandler(grpcClients *clients.GRPCClients, vmServices *services.VictoriaMetricsServices, logger logger.Logger) *HealthHandler {
	return &HealthHandler{
		grpcClients: grpcClients,
		vmServices:  vmServices,
		logger:      logger,
	}
}

// GET /health - Quick health check
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "mirador-core",
		"version":   "v2.1.3",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GET /ready - Comprehensive readiness check
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]interface{})
	overallHealthy := true

	// Check VictoriaMetrics connectivity
	if err := h.vmServices.Metrics.HealthCheck(ctx); err != nil {
		checks["victoria_metrics"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		overallHealthy = false
	} else {
		checks["victoria_metrics"] = map[string]interface{}{
			"status": "healthy",
		}
	}

	// Check VictoriaLogs connectivity
	if err := h.vmServices.Logs.HealthCheck(ctx); err != nil {
		checks["victoria_logs"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		overallHealthy = false
	} else {
		checks["victoria_logs"] = map[string]interface{}{
			"status": "healthy",
		}
	}

	// Check VictoriaTraces connectivity
	if err := h.vmServices.Traces.HealthCheck(ctx); err != nil {
		checks["victoria_traces"] = map[string]interface{}{
			"status": "degraded", // As shown in diagram
			"error":  err.Error(),
		}
		// Don't mark overall as unhealthy for traces degradation
	} else {
		checks["victoria_traces"] = map[string]interface{}{
			"status": "healthy",
		}
	}

	// Check AI engines connectivity
	if err := h.grpcClients.PredictEngine.HealthCheck(ctx); err != nil {
		checks["predict_engine"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		overallHealthy = false
	} else {
		checks["predict_engine"] = map[string]interface{}{
			"status": "healthy",
		}
	}

	if err := h.grpcClients.RCAEngine.HealthCheck(ctx); err != nil {
		checks["rca_engine"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		overallHealthy = false
	} else {
		checks["rca_engine"] = map[string]interface{}{
			"status": "healthy",
		}
	}

	if err := h.grpcClients.AlertEngine.HealthCheck(ctx); err != nil {
		checks["alert_engine"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		overallHealthy = false
	} else {
		checks["alert_engine"] = map[string]interface{}{
			"status": "healthy",
		}
	}

	status := "healthy"
	httpStatus := http.StatusOK

	if !overallHealthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	response := gin.H{
		"status":    status,
		"service":   "mirador-core",
		"version":   "v2.1.3",
		"checks":    checks,
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    "47d 12h 34m", // Would be calculated from start time
	}

	c.JSON(httpStatus, response)
}
