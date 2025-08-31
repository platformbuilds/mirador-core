package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirador/core/internal/models"
	"github.com/mirador/core/pkg/cache"
	"github.com/mirador/core/pkg/logger"
)

type ConfigHandler struct {
	cache  cache.ValleyCluster
	logger logger.Logger
}

func NewConfigHandler(cache cache.ValleyCluster, logger logger.Logger) *ConfigHandler {
	return &ConfigHandler{
		cache:  cache,
		logger: logger,
	}
}

// GET /api/v1/config/user-settings - Get user-driven settings
func (h *ConfigHandler) GetUserSettings(c *gin.Context) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")

	// Get user session with settings
	session, err := h.cache.GetSession(c.Request.Context(), c.GetHeader("Authorization"))
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
			"dashboard_layout":    "default",
			"refresh_interval":    30,
			"timezone":           "UTC",
			"theme":              "light",
			"notifications":      map[string]bool{
				"email":       true,
				"slack":       false,
				"push":        true,
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
			"userId":   userID,
			"tenantId": tenantID,
			"settings": session.Settings,
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
	sessionToken := c.GetHeader("Authorization")
	session, err := h.cache.GetSession(c.Request.Context(), sessionToken)
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

	// Store updated session in Valley cluster
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
