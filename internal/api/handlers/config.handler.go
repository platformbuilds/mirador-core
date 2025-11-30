package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type ConfigHandler struct {
	cache              cache.ValkeyCluster
	logger             logger.Logger
	featureFlagService *services.RuntimeFeatureFlagService
	dynamicConfig      *services.DynamicConfigService
	schemaRepo         repo.SchemaStore
}

func NewConfigHandler(cache cache.ValkeyCluster, logger logger.Logger, dynamicConfig *services.DynamicConfigService, schemaRepo repo.SchemaStore) *ConfigHandler {
	return &ConfigHandler{
		cache:              cache,
		logger:             logger,
		dynamicConfig:      dynamicConfig,
		schemaRepo:         schemaRepo,
		featureFlagService: services.NewRuntimeFeatureFlagService(cache, logger),
	}
}

// GET /api/v1/config/datasources - Get configured data sources
func (h *ConfigHandler) GetDataSources(c *gin.Context) {
	// Mock data sources configuration (would be from database in production)
	dataSources := []models.DataSource{
		{
			ID:     "vm-metrics",
			Name:   "VictoriaMetrics",
			Type:   "metrics",
			URL:    "http://vm-select:8481",
			Status: "connected",
		},
		{
			ID:     "vl-logs",
			Name:   "VictoriaLogs",
			Type:   "logs",
			URL:    "http://vl-select:9428",
			Status: "connected",
		},
		{
			ID:     "vt-traces",
			Name:   "VictoriaTraces",
			Type:   "traces",
			URL:    "http://vt-select:10428",
			Status: "degraded", // As shown in diagram
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"datasources": dataSources,
			"total":       len(dataSources),
		},
	})
}

// POST /api/v1/config/datasources - Add a new data source (minimal stub)
// Accepts a JSON body compatible with models.DataSource.
func (h *ConfigHandler) AddDataSource(c *gin.Context) {
	var req models.DataSource
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "invalid datasource payload",
		})
		return
	}

	// Fill defaults
	if req.ID == "" {
		req.ID = fmt.Sprintf("ds_%d", time.Now().UnixNano())
	}
	if req.Status == "" {
		req.Status = "connected" // or "pending" if you prefer
	}

	// NOTE: Persisting to Valkey/DB can be added later; for now return success.
	// Example (only if your cache exposes a generic Set):
	// _ = h.cache.SetJSON(c.Request.Context(), "cfg:datasource:"+req.ID, req, time.Hour*24)

	h.logger.Info("Data source added",
		"id", req.ID, "type", req.Type, "name", req.Name, "system")

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"datasource": req,
			"createdAt":  time.Now().Format(time.RFC3339),
		},
	})
}

// GET /api/v1/config/integrations - List available/connected integrations
// Returns a simple list for UI toggles; replace with real status checks later.
func (h *ConfigHandler) GetIntegrations(c *gin.Context) {
	// Mocked integrations snapshot; extend with real status checks later
	integrations := []map[string]interface{}{
		{
			"id":        "rca-engine",
			"name":      "RCA Engine",
			"type":      "ai",
			"status":    "connected",
			"endpoints": []string{"/api/v1/unified/rca", "/api/v1/unified/service-graph"},
		},
		{
			"id":        "alertmanager",
			"name":      "Alertmanager",
			"type":      "alerts",
			"status":    "optional",
			"endpoints": []string{},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"integrations": integrations,
			"total":        len(integrations),
		},
	})
}

// GET /api/v1/config/features - Get runtime feature flags
func (h *ConfigHandler) GetFeatureFlags(c *gin.Context) {
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get feature flags", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve feature flags",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"features": flags,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// PUT /api/v1/config/features - Update runtime feature flags
func (h *ConfigHandler) UpdateFeatureFlags(c *gin.Context) {
	var updateRequest struct {
		Features map[string]bool `json:"features" binding:"required"`
	}
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid feature flags format",
		})
		return
	}

	// Get current flags
	currentFlags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get current feature flags", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve current feature flags",
		})
		return
	}

	// Update flags based on request
	for flagName, enabled := range updateRequest.Features {
		switch flagName {
		case "rca_enabled":
			currentFlags.RCAEnabled = enabled
		case "user_settings_enabled":
			currentFlags.UserSettingsEnabled = enabled
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Unknown feature flag: %s", flagName),
			})
			return
		}
	}

	// Save updated flags
	if err := h.featureFlagService.SetFeatureFlags(c.Request.Context(), currentFlags); err != nil {
		h.logger.Error("Failed to update feature flags", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to save feature flags",
		})
		return
	}

	h.logger.Info("Feature flags updated", "flags", currentFlags)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"features": currentFlags,
			"updated":  true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// POST /api/v1/config/features/reset - Reset feature flags to defaults
func (h *ConfigHandler) ResetFeatureFlags(c *gin.Context) {
	if err := h.featureFlagService.ResetFeatureFlags(c.Request.Context()); err != nil {
		h.logger.Error("Failed to reset feature flags", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to reset feature flags",
		})
		return
	}

	// Get the reset flags to return them
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get reset feature flags", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve reset feature flags",
		})
		return
	}

	h.logger.Info("Feature flags reset to defaults", "system", "system")

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"features": flags,
			"reset":    true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
