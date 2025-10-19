package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type ConfigHandler struct {
	cache  cache.ValkeyCluster
	logger logger.Logger
}

func NewConfigHandler(cache cache.ValkeyCluster, logger logger.Logger) *ConfigHandler {
	return &ConfigHandler{
		cache:  cache,
		logger: logger,
	}
}

// GET /api/v1/config/user-settings - Get user-driven settings
func (h *ConfigHandler) GetUserSettings(c *gin.Context) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")

	// Get user session with settings (use session_id from auth middleware)
	sessionID := c.GetString("session_id")
	session, err := h.cache.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve user settings",
		})
		return
	}

	// Default settings if none exist
	if session.Settings == nil {
		session.Settings = map[string]interface{}{
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
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"userId":      userID,
			"tenantId":    tenantID,
			"settings":    session.Settings,
			"lastUpdated": session.LastActivity,
		},
	})
}

// PUT /api/v1/config/user-settings - Update user-driven settings
func (h *ConfigHandler) UpdateUserSettings(c *gin.Context) {
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
// Returns a simple list for UI toggles; replace with real discovery later.
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
