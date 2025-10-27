package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type HealthHandler struct {
	grpcClients *clients.GRPCClients
	vmServices  *services.VictoriaMetricsServices
	cache       cache.ValkeyCluster // may be nil for legacy behavior
	logger      logger.Logger
}

// NewHealthHandlerWithCache constructs a HealthHandler with explicit cache dependency.
func NewHealthHandlerWithCache(grpcClients *clients.GRPCClients, vmServices *services.VictoriaMetricsServices, c cache.ValkeyCluster, logger logger.Logger) *HealthHandler {
	return &HealthHandler{
		grpcClients: grpcClients,
		vmServices:  vmServices,
		cache:       c,
		logger:      logger,
	}
}

// NewHealthHandler preserves the legacy constructor signature used by tests.
// It creates a handler without a cache dependency; readiness will fall back
// to legacy checks (VM/VL/VT + engines) when cache is nil.
func NewHealthHandler(grpcClients *clients.GRPCClients, vmServices *services.VictoriaMetricsServices, logger logger.Logger) *HealthHandler {
	return &HealthHandler{
		grpcClients: grpcClients,
		vmServices:  vmServices,
		cache:       nil,
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
	// If cache is provided, readiness depends only on Valkey availability.
	if h.cache != nil {
		ready := false
		var valkeyErr error
		type cacheHealth interface{ HealthCheck(context.Context) error }
		if hc, ok := interface{}(h.cache).(cacheHealth); ok {
			valkeyErr = hc.HealthCheck(ctx)
			ready = valkeyErr == nil
		} else {
			// Fallback probe: attempt a short Set
			probeKey := fmt.Sprintf("ready:%d", time.Now().UnixNano())
			valkeyErr = h.cache.Set(ctx, probeKey, "1", 1*time.Second)
			ready = valkeyErr == nil
		}

		status := "healthy"
		httpStatus := http.StatusOK
		if !ready {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}
		resp := gin.H{
			"status":    status,
			"service":   "mirador-core",
			"version":   "v2.1.3",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		if valkeyErr != nil {
			resp["error"] = valkeyErr.Error()
		}
		c.JSON(httpStatus, resp)
		return
	}

	// Legacy readiness: check VM/VL/VT and engines (used by existing tests)
	checks := make(map[string]interface{})
	overallHealthy := true

	if err := h.vmServices.Metrics.HealthCheck(ctx); err != nil {
		checks["victoria_metrics"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
		overallHealthy = false
	} else {
		checks["victoria_metrics"] = map[string]interface{}{"status": "healthy"}
	}
	if err := h.vmServices.Logs.HealthCheck(ctx); err != nil {
		checks["victoria_logs"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
		overallHealthy = false
	} else {
		checks["victoria_logs"] = map[string]interface{}{"status": "healthy"}
	}
	if err := h.vmServices.Traces.HealthCheck(ctx); err != nil {
		checks["victoria_traces"] = map[string]interface{}{"status": "degraded", "error": err.Error()}
	} else {
		checks["victoria_traces"] = map[string]interface{}{"status": "healthy"}
	}
	if h.grpcClients.PredictEngine != nil {
		if err := h.grpcClients.PredictEngine.HealthCheck(); err != nil {
			checks["predict_engine"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
			overallHealthy = false
		} else {
			checks["predict_engine"] = map[string]interface{}{"status": "healthy"}
		}
	} else {
		checks["predict_engine"] = map[string]interface{}{"status": "disabled"}
	}
	if h.grpcClients.RCAEngine != nil {
		if err := h.grpcClients.RCAEngine.HealthCheck(); err != nil {
			checks["rca_engine"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
			overallHealthy = false
		} else {
			checks["rca_engine"] = map[string]interface{}{"status": "healthy"}
		}
	} else {
		checks["rca_engine"] = map[string]interface{}{"status": "disabled"}
	}
	if h.grpcClients.AlertEngine != nil {
		if err := h.grpcClients.AlertEngine.HealthCheck(); err != nil {
			checks["alert_engine"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
			overallHealthy = false
		} else {
			checks["alert_engine"] = map[string]interface{}{"status": "healthy"}
		}
	} else {
		checks["alert_engine"] = map[string]interface{}{"status": "disabled"}
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
	}
	c.JSON(httpStatus, response)
}

// GET /microservices/status - report health of backends (VM, VL, VT, AI engines)
func (h *HealthHandler) MicroservicesStatus(c *gin.Context) {
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
		checks["victoria_metrics"] = map[string]interface{}{"status": "healthy"}
	}

	// Check VictoriaLogs connectivity
	if err := h.vmServices.Logs.HealthCheck(ctx); err != nil {
		checks["victoria_logs"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		overallHealthy = false
	} else {
		checks["victoria_logs"] = map[string]interface{}{"status": "healthy"}
	}

	// Check VictoriaTraces connectivity (degraded allowed)
	if err := h.vmServices.Traces.HealthCheck(ctx); err != nil {
		checks["victoria_traces"] = map[string]interface{}{
			"status": "degraded",
			"error":  err.Error(),
		}
	} else {
		checks["victoria_traces"] = map[string]interface{}{"status": "healthy"}
	}

	// AI engines (optional in development, only fail overall health in production)
	isDevelopment := os.Getenv("ENVIRONMENT") == "development"

	if h.grpcClients.PredictEngine != nil {
		if err := h.grpcClients.PredictEngine.HealthCheck(); err != nil {
			checks["predict_engine"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
			// Only fail overall health in production
			if !isDevelopment {
				overallHealthy = false
			}
		} else {
			status := "healthy"
			if !h.grpcClients.PredictEnabled {
				status = "disabled"
			}
			checks["predict_engine"] = map[string]interface{}{"status": status}
		}
	} else {
		checks["predict_engine"] = map[string]interface{}{"status": "disabled"}
	}

	if h.grpcClients.RCAEngine != nil {
		if err := h.grpcClients.RCAEngine.HealthCheck(); err != nil {
			checks["rca_engine"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
			// Only fail overall health in production
			if !isDevelopment {
				overallHealthy = false
			}
		} else {
			status := "healthy"
			if !h.grpcClients.RCAEnabled {
				status = "disabled"
			}
			checks["rca_engine"] = map[string]interface{}{"status": status}
		}
	} else {
		checks["rca_engine"] = map[string]interface{}{"status": "disabled"}
	}

	if h.grpcClients.AlertEngine != nil {
		if err := h.grpcClients.AlertEngine.HealthCheck(); err != nil {
			checks["alert_engine"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
			// Only fail overall health in production
			if !isDevelopment {
				overallHealthy = false
			}
		} else {
			status := "healthy"
			if !h.grpcClients.AlertEnabled {
				status = "disabled"
			}
			checks["alert_engine"] = map[string]interface{}{"status": status}
		}
	} else {
		checks["alert_engine"] = map[string]interface{}{"status": "disabled"}
	}

	httpStatus := http.StatusOK
	status := "healthy"
	if !overallHealthy {
		status = "unhealthy"
	}
	c.JSON(httpStatus, gin.H{
		"status":    status,
		"service":   "mirador-core",
		"version":   "v2.1.3",
		"checks":    checks,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
