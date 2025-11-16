package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MetricsSyncHandler handles metrics metadata synchronization endpoints
type MetricsSyncHandler struct {
	synchronizer services.MetricsMetadataSynchronizer
	logger       logger.Logger
}

// NewMetricsSyncHandler creates a new metrics sync handler
func NewMetricsSyncHandler(synchronizer services.MetricsMetadataSynchronizer, logger logger.Logger) *MetricsSyncHandler {
	return &MetricsSyncHandler{
		synchronizer: synchronizer,
		logger:       logger,
	}
}

// HandleSyncNow triggers an immediate sync for the default tenant
func (h *MetricsSyncHandler) HandleSyncNow(c *gin.Context) {
	tenantID := "default" // Single-tenant system uses default tenant

	// Parse forceFull parameter
	forceFullStr := c.DefaultQuery("forceFull", "false")
	forceFull, err := strconv.ParseBool(forceFullStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid forceFull parameter"})
		return
	}

	h.logger.Info("Triggering immediate sync", "tenantID", tenantID, "forceFull", forceFull)

	result, err := h.synchronizer.SyncNow(c.Request.Context(), tenantID, forceFull)
	if err != nil {
		h.logger.Error("Failed to trigger sync", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Sync completed successfully",
		"result":  result,
	})
}

// HandleGetSyncState returns the current sync state for the default tenant
func (h *MetricsSyncHandler) HandleGetSyncState(c *gin.Context) {
	tenantID := "default" // Single-tenant system uses default tenant

	state, err := h.synchronizer.GetSyncState(tenantID)
	if err != nil {
		h.logger.Error("Failed to get sync state", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}

// HandleGetSyncStatus returns the status of the current/last sync operation
func (h *MetricsSyncHandler) HandleGetSyncStatus(c *gin.Context) {
	tenantID := "default" // Single-tenant system uses default tenant

	status, err := h.synchronizer.GetSyncStatus(tenantID)
	if err != nil {
		h.logger.Error("Failed to get sync status", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// HandleUpdateConfig updates the synchronization configuration
func (h *MetricsSyncHandler) HandleUpdateConfig(c *gin.Context) {
	var config models.MetricMetadataSyncConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.synchronizer.UpdateConfig(&config); err != nil {
		h.logger.Error("Failed to update sync config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Sync configuration updated successfully")
	c.JSON(http.StatusOK, gin.H{"message": "Sync configuration updated successfully"})
}
