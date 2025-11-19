package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type AlertHandler struct {
	integrationsService *services.IntegrationsService
	cache               cache.ValkeyCluster
	logger              logger.Logger
}

func NewAlertHandler(
	integrationsService *services.IntegrationsService,
	cache cache.ValkeyCluster,
	logger logger.Logger,
) *AlertHandler {
	return &AlertHandler{
		integrationsService: integrationsService,
		cache:               cache,
		logger:              logger,
	}
}

// GET /api/v1/alerts - Get active alerts (disabled)
func (h *AlertHandler) GetAlerts(c *gin.Context) {
	// Alert functionality via external AI engines has been removed
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "error",
		"error":   "Alert retrieval via external AI engines is no longer supported",
		"message": "Use local monitoring and alerting systems",
	})
}

// POST /api/v1/alerts - Create new alert rule (disabled)
func (h *AlertHandler) CreateAlert(c *gin.Context) {
	// Alert functionality via external AI engines has been removed
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "error",
		"error":   "Alert creation via external AI engines is no longer supported",
		"message": "Use local monitoring and alerting systems",
	})
}

// PUT /api/v1/alerts/:id/acknowledge - Acknowledge alert (disabled)
func (h *AlertHandler) AcknowledgeAlert(c *gin.Context) {
	// Alert functionality via external AI engines has been removed
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "error",
		"error":   "Alert acknowledgment via external AI engines is no longer supported",
		"message": "Use local monitoring and alerting systems",
	})
}
