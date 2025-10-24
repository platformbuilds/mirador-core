package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type ConfigHandler struct {
	cache              cache.ValkeyCluster
	logger             logger.Logger
	featureFlagService *services.RuntimeFeatureFlagService
	dynamicConfig      *services.DynamicConfigService
	grpcClients        *clients.GRPCClients
}

func NewConfigHandler(cache cache.ValkeyCluster, logger logger.Logger, dynamicConfig *services.DynamicConfigService, grpcClients *clients.GRPCClients) *ConfigHandler {
	return &ConfigHandler{
		cache:              cache,
		logger:             logger,
		dynamicConfig:      dynamicConfig,
		grpcClients:        grpcClients,
		featureFlagService: services.NewRuntimeFeatureFlagService(cache, logger),
	}
}

// checkUserSettingsEnabled checks if user settings feature is enabled
func (h *ConfigHandler) checkUserSettingsEnabled(c *gin.Context) bool {
	tenantID := c.GetString("tenant_id")
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to check feature flags", "tenantID", tenantID, "error", err)
		return false
	}
	return flags.UserSettingsEnabled
}

// GET /api/v1/config/user-settings - Get user-driven settings
func (h *ConfigHandler) GetUserSettings(c *gin.Context) {
	// Check if user settings feature is enabled
	if !h.checkUserSettingsEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "User settings feature is disabled",
		})
		return
	}

	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")

	// Get user session with settings (use session_id from auth middleware)
	sessionID := c.GetString("session_id")

	// Default settings for testing/development when no session exists
	defaultSettings := map[string]interface{}{
		"dashboard_layout": "default",
		"refresh_interval": 30,
		"timezone":         "UTC",
		"theme":            "light",
		"notifications": map[string]bool{
			"email": true,
			"slack": false,
			"push":  true,
		},
		"query_preferences": map[string]interface{}{
			"default_time_range": "1h",
			"auto_refresh":       true,
			"query_timeout":      "30s",
		},
	}

	// Try to get session, but don't fail if it doesn't exist (for testing)
	var sessionSettings map[string]interface{}
	var lastActivity string

	if sessionID != "" {
		session, err := h.cache.GetSession(c.Request.Context(), sessionID)
		if err == nil && session != nil {
			sessionSettings = session.Settings
			lastActivity = session.LastActivity.Format(time.RFC3339)
		}
	}

	// Use session settings if available, otherwise use defaults
	if sessionSettings == nil {
		sessionSettings = defaultSettings
		lastActivity = time.Now().Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"userId":      userID,
			"tenantId":    tenantID,
			"settings":    sessionSettings,
			"lastUpdated": lastActivity,
		},
	})
}

// PUT /api/v1/config/user-settings - Update user-driven settings
func (h *ConfigHandler) UpdateUserSettings(c *gin.Context) {
	// Check if user settings feature is enabled
	if !h.checkUserSettingsEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "User settings feature is disabled",
		})
		return
	}

	userID := c.GetString("user_id")

	var updateRequest map[string]interface{}
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid settings format",
		})
		return
	}

	// Get current session
	sessionID := c.GetString("session_id")
	session, err := h.cache.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve session",
		})
		return
	}

	// Update settings
	if session.Settings == nil {
		session.Settings = make(map[string]interface{})
	}

	// Merge new settings with existing ones
	for key, value := range updateRequest {
		session.Settings[key] = value
	}

	// Store updated session in Valkey cluster
	if err := h.cache.SetSession(c.Request.Context(), session); err != nil {
		h.logger.Error("Failed to update user settings", "userId", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to save settings",
		})
		return
	}

	h.logger.Info("User settings updated", "userId", userID, "settingsCount", len(session.Settings))

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"updated":   true,
			"settings":  session.Settings,
			"updatedAt": time.Now().Format(time.RFC3339),
		},
	})
}

// GET /api/v1/config/datasources - Get configured data sources
func (h *ConfigHandler) GetDataSources(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Mock data sources configuration (would be from database in production)
	dataSources := []models.DataSource{
		{
			ID:       "vm-metrics",
			Name:     "VictoriaMetrics",
			Type:     "metrics",
			URL:      "http://vm-select:8481",
			Status:   "connected",
			TenantID: tenantID,
		},
		{
			ID:       "vl-logs",
			Name:     "VictoriaLogs",
			Type:     "logs",
			URL:      "http://vl-select:9428",
			Status:   "connected",
			TenantID: tenantID,
		},
		{
			ID:       "vt-traces",
			Name:     "VictoriaTraces",
			Type:     "traces",
			URL:      "http://vt-select:10428",
			Status:   "degraded", // As shown in diagram
			TenantID: tenantID,
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
// If ID is empty, one is generated. TenantID is taken from request context.
func (h *ConfigHandler) AddDataSource(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

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
	req.TenantID = tenantID

	// NOTE: Persisting to Valkey/DB can be added later; for now return success.
	// Example (only if your cache exposes a generic Set):
	// _ = h.cache.SetJSON(c.Request.Context(), "cfg:datasource:"+req.ID, req, time.Hour*24)

	h.logger.Info("Data source added",
		"id", req.ID, "type", req.Type, "name", req.Name, "tenant", tenantID)

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
	tenantID := c.GetString("tenant_id")

	// Mocked integrations snapshot; extend with real status checks later
	integrations := []map[string]interface{}{
		{
			"id":        "predict-engine",
			"name":      "Predict Engine",
			"type":      "ai",
			"status":    "connected",
			"tenantId":  tenantID,
			"endpoints": []string{"/api/v1/predict/health", "/api/v1/predict/models"},
		},
		{
			"id":        "rca-engine",
			"name":      "RCA Engine",
			"type":      "ai",
			"status":    "connected",
			"tenantId":  tenantID,
			"endpoints": []string{"/api/v1/rca/investigate", "/api/v1/rca/patterns"},
		},
		{
			"id":        "alertmanager",
			"name":      "Alertmanager",
			"type":      "alerts",
			"status":    "optional",
			"tenantId":  tenantID,
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
	tenantID := c.GetString("tenant_id")

	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to get feature flags", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve feature flags",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantId": tenantID,
			"features": flags,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// PUT /api/v1/config/features - Update runtime feature flags
func (h *ConfigHandler) UpdateFeatureFlags(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

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
	currentFlags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to get current feature flags", "tenantID", tenantID, "error", err)
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
		case "predict_enabled":
			currentFlags.PredictEnabled = enabled
		case "user_settings_enabled":
			currentFlags.UserSettingsEnabled = enabled
		case "rbac_enabled":
			currentFlags.RBACEnabled = enabled
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Unknown feature flag: %s", flagName),
			})
			return
		}
	}

	// Save updated flags
	if err := h.featureFlagService.SetFeatureFlags(c.Request.Context(), tenantID, currentFlags); err != nil {
		h.logger.Error("Failed to update feature flags", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to save feature flags",
		})
		return
	}

	h.logger.Info("Feature flags updated", "tenantID", tenantID, "flags", currentFlags)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantId": tenantID,
			"features": currentFlags,
			"updated":  true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// POST /api/v1/config/features/reset - Reset feature flags to defaults
func (h *ConfigHandler) ResetFeatureFlags(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	if err := h.featureFlagService.ResetFeatureFlags(c.Request.Context(), tenantID); err != nil {
		h.logger.Error("Failed to reset feature flags", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to reset feature flags",
		})
		return
	}

	// Get the reset flags to return them
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to get reset feature flags", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve reset feature flags",
		})
		return
	}

	h.logger.Info("Feature flags reset to defaults", "tenantID", tenantID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantId": tenantID,
			"features": flags,
			"reset":    true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GET /api/v1/config/grpc/endpoints - Get current gRPC endpoint configurations
func (h *ConfigHandler) GetGRPCEndpoints(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	if h.dynamicConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"error":  "Dynamic configuration service not available",
		})
		return
	}

	// Get current config from cache, falling back to static config
	defaultConfig := &config.GRPCConfig{}
	config, err := h.dynamicConfig.GetGRPCConfig(c.Request.Context(), tenantID, defaultConfig)
	if err != nil {
		h.logger.Error("Failed to get gRPC endpoints", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve gRPC endpoint configurations",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantId":  tenantID,
			"endpoints": config,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// PUT /api/v1/config/grpc/endpoints - Update gRPC endpoint configurations
func (h *ConfigHandler) UpdateGRPCEndpoints(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	if h.dynamicConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"error":  "Dynamic configuration service not available",
		})
		return
	}

	var updateRequest struct {
		PredictEndpoint string `json:"predict_endpoint,omitempty"`
		RCAEndpoint     string `json:"rca_endpoint,omitempty"`
		AlertEndpoint   string `json:"alert_endpoint,omitempty"`
	}
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid endpoint configuration format",
		})
		return
	}

	// Get current config
	defaultConfig := &config.GRPCConfig{}
	currentConfig, err := h.dynamicConfig.GetGRPCConfig(c.Request.Context(), tenantID, defaultConfig)
	if err != nil {
		h.logger.Error("Failed to get current gRPC config", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve current configuration",
		})
		return
	}

	// Update endpoints if provided
	updated := false
	if updateRequest.PredictEndpoint != "" {
		currentConfig.PredictEngine.Endpoint = updateRequest.PredictEndpoint
		if err := h.grpcClients.UpdatePredictEndpoint(c.Request.Context(), tenantID, updateRequest.PredictEndpoint); err != nil {
			h.logger.Error("Failed to update predict endpoint", "tenantID", tenantID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Failed to update predict endpoint: %v", err),
			})
			return
		}
		updated = true
	}

	if updateRequest.RCAEndpoint != "" {
		currentConfig.RCAEngine.Endpoint = updateRequest.RCAEndpoint
		if err := h.grpcClients.UpdateRCAEndpoint(c.Request.Context(), tenantID, updateRequest.RCAEndpoint); err != nil {
			h.logger.Error("Failed to update RCA endpoint", "tenantID", tenantID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Failed to update RCA endpoint: %v", err),
			})
			return
		}
		updated = true
	}

	if updateRequest.AlertEndpoint != "" {
		currentConfig.AlertEngine.Endpoint = updateRequest.AlertEndpoint
		if err := h.grpcClients.UpdateAlertEndpoint(c.Request.Context(), tenantID, updateRequest.AlertEndpoint); err != nil {
			h.logger.Error("Failed to update alert endpoint", "tenantID", tenantID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Failed to update alert endpoint: %v", err),
			})
			return
		}
		updated = true
	}

	if !updated {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "No endpoints provided for update",
		})
		return
	}

	// Save updated config
	if err := h.dynamicConfig.SetGRPCConfig(c.Request.Context(), tenantID, currentConfig); err != nil {
		h.logger.Error("Failed to save updated gRPC config", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to save configuration",
		})
		return
	}

	h.logger.Info("Successfully updated gRPC endpoints", "tenantID", tenantID, "updates", updateRequest)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantId":  tenantID,
			"endpoints": currentConfig,
			"updated":   true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// POST /api/v1/config/grpc/endpoints/reset - Reset gRPC endpoints to defaults
func (h *ConfigHandler) ResetGRPCEndpoints(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	if h.dynamicConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"error":  "Dynamic configuration service not available",
		})
		return
	}

	// Reset to defaults (static config)
	defaultConfig := &config.GRPCConfig{}
	if err := h.dynamicConfig.ResetGRPCConfig(c.Request.Context(), tenantID, defaultConfig); err != nil {
		h.logger.Error("Failed to reset gRPC endpoints", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to reset gRPC endpoint configurations",
		})
		return
	}

	// Get the reset config to return it
	defaultCfg := &config.GRPCConfig{}
	resetConfig, err := h.dynamicConfig.GetGRPCConfig(c.Request.Context(), tenantID, defaultCfg)
	if err != nil {
		h.logger.Error("Failed to get reset gRPC config", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve reset configuration",
		})
		return
	}

	h.logger.Info("Reset gRPC endpoints to defaults", "tenantID", tenantID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantId":  tenantID,
			"endpoints": resetConfig,
			"reset":     true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
