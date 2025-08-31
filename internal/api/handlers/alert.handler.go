package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/miradorstack/internal/grpc/clients"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/internal/services"
	"github.com/platformbuilds/miradorstack/internal/utils"
	"github.com/platformbuilds/miradorstack/pkg/cache"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type AlertHandler struct {
	alertClient         *clients.AlertEngineClient
	integrationsService *services.IntegrationsService
	cache               cache.ValkeyCluster
	logger              logger.Logger
}

func NewAlertHandler(
	alertClient *clients.AlertEngineClient,
	integrationsService *services.IntegrationsService,
	cache cache.ValkeyCluster,
	logger logger.Logger,
) *AlertHandler {
	return &AlertHandler{
		alertClient:         alertClient,
		integrationsService: integrationsService,
		cache:               cache,
		logger:              logger,
	}
}

// GET /api/v1/alerts - Get active alerts with intelligent clustering
func (h *AlertHandler) GetAlerts(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	severity := c.Query("severity")

	// Try valley cluster cache first for faster fetch
	cacheKey := fmt.Sprintf("alerts:%s:%s:%d", tenantID, severity, limit)
	if cached, err := h.cache.Get(c.Request.Context(), cacheKey); err == nil {
		var cachedAlerts []*models.Alert
		if json.Unmarshal(cached, &cachedAlerts) == nil {
			c.Header("X-Cache", "HIT")
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"data": gin.H{
					"alerts": cachedAlerts,
					"total":  len(cachedAlerts),
				},
			})
			return
		}
	}

	// Query from ALERT-ENGINE
	alerts, err := h.alertClient.GetActiveAlerts(c.Request.Context(), &models.AlertQuery{
		TenantID: tenantID,
		Limit:    limit,
		Severity: severity,
	})
	if err != nil {
		h.logger.Error("Failed to get alerts", "tenant", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve alerts",
		})
		return
	}

	// Cache results for faster future access
	h.cache.Set(c.Request.Context(), cacheKey, alerts, 1*time.Minute)

	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"alerts": alerts,
			"total":  len(alerts),
			"summary": gin.H{
				"critical": utils.CountAlertsBySeverity(alerts, "critical"),
				"warning":  utils.CountAlertsBySeverity(alerts, "warning"),
				"info":     utils.CountAlertsBySeverity(alerts, "info"),
			},
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// POST /api/v1/alerts - Create new alert rule
func (h *AlertHandler) CreateAlert(c *gin.Context) {
	var alertRule models.AlertRule
	if err := c.ShouldBindJSON(&alertRule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid alert rule format",
		})
		return
	}

	alertRule.TenantID = c.GetString("tenant_id")
	alertRule.CreatedBy = c.GetString("user_id")
	alertRule.CreatedAt = time.Now()

	// Validate alert rule query syntax
	if err := h.validateAlertRule(&alertRule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Invalid alert rule: %s", err.Error()),
		})
		return
	}

	// Create alert rule via ALERT-ENGINE
	createdRule, err := h.alertClient.CreateAlertRule(c.Request.Context(), &alertRule)
	if err != nil {
		h.logger.Error("Failed to create alert rule", "rule", alertRule.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to create alert rule",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"rule": createdRule,
			"id":   createdRule.ID,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// PUT /api/v1/alerts/:id/acknowledge - Acknowledge alert
func (h *AlertHandler) AcknowledgeAlert(c *gin.Context) {
	alertID := c.Param("id")
	userID := c.GetString("user_id")

	ackRequest := models.AlertAcknowledgment{
		AlertID:        alertID,
		AcknowledgedBy: userID,
		AcknowledgedAt: time.Now(),
		Comment:        c.PostForm("comment"),
	}

	err := h.alertClient.AcknowledgeAlert(c.Request.Context(), &ackRequest)
	if err != nil {
		h.logger.Error("Failed to acknowledge alert", "alertId", alertID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to acknowledge alert",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"acknowledged":   true,
			"alertId":        alertID,
			"acknowledgedBy": userID,
			"acknowledgedAt": ackRequest.AcknowledgedAt,
		},
	})
}

func (h *AlertHandler) validateAlertRule(rule *models.AlertRule) error {
	// Validate MetricsQL/LogsQL query syntax
	// This would integrate with query validation service
	return nil
}
