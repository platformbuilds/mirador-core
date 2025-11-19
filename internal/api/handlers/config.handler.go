package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
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
	grpcClients        *clients.GRPCClients
	schemaRepo         repo.SchemaStore
}

func NewConfigHandler(cache cache.ValkeyCluster, logger logger.Logger, dynamicConfig *services.DynamicConfigService, grpcClients *clients.GRPCClients, schemaRepo repo.SchemaStore) *ConfigHandler {
	return &ConfigHandler{
		cache:              cache,
		logger:             logger,
		dynamicConfig:      dynamicConfig,
		grpcClients:        grpcClients,
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
			"endpoints": []string{"/api/v1/rca/investigate", "/api/v1/rca/patterns"},
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
	currentFlags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), "system")
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
		case "rbac_enabled":
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Unknown feature flag: %s", flagName),
			})
			return
		}
	}

	// Save updated flags
	if err := h.featureFlagService.SetFeatureFlags(c.Request.Context(), "system", currentFlags); err != nil {
		h.logger.Error("Failed to update feature flags", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to save feature flags",
		})
		return
	}

	h.logger.Info("Feature flags updated", "system", "system", "flags", currentFlags)

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
	if err := h.featureFlagService.ResetFeatureFlags(c.Request.Context(), "system"); err != nil {
		h.logger.Error("Failed to reset feature flags", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to reset feature flags",
		})
		return
	}

	// Get the reset flags to return them
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), "system")
	if err != nil {
		h.logger.Error("Failed to get reset feature flags", "system", "system", "error", err)
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

// GET /api/v1/config/user-preferences - Get user preferences
func (h *ConfigHandler) GetUserPreferences(c *gin.Context) {
	userID := c.GetString("user_id") // Assuming middleware sets this
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user authentication required"})
		return
	}

	userPrefs, err := h.getUserPreferences(userID)
	if err != nil {
		// If not found, return default preferences
		defaultPrefs := &models.UserPreferences{
			ID:                 userID,
			Theme:              "system",
			SidebarCollapsed:   false,
			DefaultDashboardID: "system",
			Timezone:           "UTC",
			KeyboardHintSeen:   false,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		c.JSON(http.StatusOK, models.UserPreferencesResponse{UserPreferences: defaultPrefs})
		return
	}

	c.JSON(http.StatusOK, models.UserPreferencesResponse{UserPreferences: userPrefs})
}

// POST /api/v1/config/user-preferences - Create user preferences
func (h *ConfigHandler) CreateUserPreferences(c *gin.Context) {
	userID := c.GetString("user_id") // Assuming middleware sets this
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user authentication required"})
		return
	}

	var req models.UserPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.UserPreferences == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user preferences is required"})
		return
	}

	userPrefs := req.UserPreferences
	userPrefs.ID = userID // Ensure ID matches authenticated user
	userPrefs.CreatedAt = time.Now()
	userPrefs.UpdatedAt = userPrefs.CreatedAt

	err := h.upsertUserPreferences(userPrefs)
	if err != nil {
		h.logger.Error("user preferences create failed", "error", err, "user", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user preferences"})
		return
	}

	c.JSON(http.StatusCreated, models.UserPreferencesResponse{UserPreferences: userPrefs})
}

// PUT /api/v1/config/user-preferences - Update user preferences
func (h *ConfigHandler) UpdateUserPreferences(c *gin.Context) {
	userID := c.GetString("user_id") // Assuming middleware sets this
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user authentication required"})
		return
	}

	var req models.UserPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.UserPreferences == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user preferences is required"})
		return
	}

	userPrefs := req.UserPreferences
	userPrefs.ID = userID // Ensure ID matches authenticated user
	userPrefs.UpdatedAt = time.Now()

	if userPrefs.CreatedAt.IsZero() {
		userPrefs.CreatedAt = userPrefs.UpdatedAt
	}

	err := h.upsertUserPreferences(userPrefs)
	if err != nil {
		h.logger.Error("user preferences update failed", "error", err, "user", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user preferences"})
		return
	}

	c.JSON(http.StatusOK, models.UserPreferencesResponse{UserPreferences: userPrefs})
}

// DELETE /api/v1/config/user-preferences - Delete user preferences
func (h *ConfigHandler) DeleteUserPreferences(c *gin.Context) {
	userID := c.GetString("user_id") // Assuming middleware sets this
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user authentication required"})
		return
	}

	q := c.Query("confirm")
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	err := h.deleteUserPreferences(userID)
	if err != nil {
		h.logger.Error("user preferences delete failed", "error", err, "user", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// Implementation methods for user preferences

func (h *ConfigHandler) upsertUserPreferences(userPrefs *models.UserPreferences) error {
	key := fmt.Sprintf("user_prefs:%s", userPrefs.ID)
	return h.cache.Set(context.Background(), key, userPrefs, 30*24*time.Hour) // 30 days TTL
}

func (h *ConfigHandler) getUserPreferences(userID string) (*models.UserPreferences, error) {
	key := fmt.Sprintf("user_prefs:%s", userID)
	data, err := h.cache.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}

	var userPrefs models.UserPreferences
	if err := json.Unmarshal(data, &userPrefs); err != nil {
		return nil, err
	}

	return &userPrefs, nil
}

func (h *ConfigHandler) deleteUserPreferences(userID string) error {
	key := fmt.Sprintf("user_prefs:%s:%s", "system", userID)
	return h.cache.Delete(context.Background(), key)
}

// GET /api/v1/config/grpc/endpoints - Get current gRPC endpoint configurations
func (h *ConfigHandler) GetGRPCEndpoints(c *gin.Context) {
	if h.dynamicConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"error":  "Dynamic configuration service not available",
		})
		return
	}

	// Get current config from cache, falling back to static config
	defaultConfig := &config.GRPCConfig{}
	config, err := h.dynamicConfig.GetGRPCConfig(c.Request.Context(), "system", defaultConfig)
	if err != nil {
		h.logger.Error("Failed to get gRPC endpoints", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve gRPC endpoint configurations",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"endpoints": config,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// PUT /api/v1/config/grpc/endpoints - Update gRPC endpoint configurations
func (h *ConfigHandler) UpdateGRPCEndpoints(c *gin.Context) {
	if h.dynamicConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"error":  "Dynamic configuration service not available",
		})
		return
	}

	var updateRequest struct {
		RCAEndpoint   string `json:"rca_endpoint,omitempty"`
		AlertEndpoint string `json:"alert_endpoint,omitempty"`
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
	currentConfig, err := h.dynamicConfig.GetGRPCConfig(c.Request.Context(), "system", defaultConfig)
	if err != nil {
		h.logger.Error("Failed to get current gRPC config", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve current configuration",
		})
		return
	}

	// Update endpoints if provided
	updated := false
	if updateRequest.RCAEndpoint != "" {
		currentConfig.RCAEngine.Endpoint = updateRequest.RCAEndpoint
		if err := h.grpcClients.UpdateRCAEndpoint(c.Request.Context(), "system", updateRequest.RCAEndpoint); err != nil {
			h.logger.Error("Failed to update RCA endpoint", "system", "system", "error", err)
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
		if err := h.grpcClients.UpdateAlertEndpoint(c.Request.Context(), "system", updateRequest.AlertEndpoint); err != nil {
			h.logger.Error("Failed to update alert endpoint", "system", "system", "error", err)
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
	if err := h.dynamicConfig.SetGRPCConfig(c.Request.Context(), "system", currentConfig); err != nil {
		h.logger.Error("Failed to save updated gRPC config", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to save configuration",
		})
		return
	}

	h.logger.Info("Successfully updated gRPC endpoints", "system", "system", "updates", updateRequest)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"endpoints": currentConfig,
			"updated":   true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// POST /api/v1/config/grpc/endpoints/reset - Reset gRPC endpoints to defaults
func (h *ConfigHandler) ResetGRPCEndpoints(c *gin.Context) {
	if h.dynamicConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"error":  "Dynamic configuration service not available",
		})
		return
	}

	// Reset to defaults (static config)
	defaultConfig := &config.GRPCConfig{}
	if err := h.dynamicConfig.ResetGRPCConfig(c.Request.Context(), "system", defaultConfig); err != nil {
		h.logger.Error("Failed to reset gRPC endpoints", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to reset gRPC endpoint configurations",
		})
		return
	}

	// Get the reset config to return it
	defaultCfg := &config.GRPCConfig{}
	resetConfig, err := h.dynamicConfig.GetGRPCConfig(c.Request.Context(), "system", defaultCfg)
	if err != nil {
		h.logger.Error("Failed to get reset gRPC config", "system", "system", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve reset configuration",
		})
		return
	}

	h.logger.Info("Reset gRPC endpoints to defaults", "system", "system")

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"endpoints": resetConfig,
			"reset":     true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
