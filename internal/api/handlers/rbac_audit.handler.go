package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// RBACAuditHandler handles RBAC audit log-related HTTP requests
type RBACAuditHandler struct {
	rbacRepo rbac.RBACRepository
	logger   logger.Logger
}

func NewRBACAuditHandler(rbacRepo rbac.RBACRepository, logger logger.Logger) *RBACAuditHandler {
	return &RBACAuditHandler{
		rbacRepo: rbacRepo,
		logger:   logger,
	}
}

// GetAuditEvents handles GET /api/v1/rbac/audit
// Get audit events with optional filtering
func (h *RBACAuditHandler) GetAuditEvents(c *gin.Context) {
	correlationID := fmt.Sprintf("audit-events-get-%d", time.Now().UnixNano())
	tenantID := c.GetString("tenant_id")

	h.logger.Info("Getting audit events", "tenantId", tenantID, "correlation_id", correlationID)

	// Parse query parameters for filters
	filters := rbac.AuditFilters{
		Limit:  getIntQueryParam(c, "limit", 50),
		Offset: getIntQueryParam(c, "offset", 0),
	}

	// Optional filters
	if subjectID := c.Query("subjectId"); subjectID != "" {
		filters.SubjectID = &subjectID
	}
	if subjectType := c.Query("subjectType"); subjectType != "" {
		filters.SubjectType = &subjectType
	}
	if action := c.Query("action"); action != "" {
		filters.Action = &action
	}
	if resource := c.Query("resource"); resource != "" {
		filters.Resource = &resource
	}
	if resourceID := c.Query("resourceId"); resourceID != "" {
		filters.ResourceID = &resourceID
	}
	if result := c.Query("result"); result != "" {
		filters.Result = &result
	}
	if severity := c.Query("severity"); severity != "" {
		filters.Severity = &severity
	}
	if source := c.Query("source"); source != "" {
		filters.Source = &source
	}

	// Parse time range filters
	if startTimeStr := c.Query("startTime"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filters.StartTime = &startTime
		} else {
			h.logger.Warn("Invalid startTime format", "startTime", startTimeStr, "correlation_id", correlationID)
		}
	}
	if endTimeStr := c.Query("endTime"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filters.EndTime = &endTime
		} else {
			h.logger.Warn("Invalid endTime format", "endTime", endTimeStr, "correlation_id", correlationID)
		}
	}

	// Get audit events
	events, err := h.rbacRepo.GetAuditEvents(c.Request.Context(), tenantID, filters)
	if err != nil {
		h.logger.Error("Failed to get audit events", "error", err, "correlation_id", correlationID, "tenantId", tenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit events"})
		return
	}

	h.logger.Info("Audit events retrieved successfully", "count", len(events), "correlation_id", correlationID)
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
		"filters": gin.H{
			"limit":  filters.Limit,
			"offset": filters.Offset,
		},
	})
}

// GetAuditEvent handles GET /api/v1/rbac/audit/{eventId}
// Get a specific audit event by ID (if supported by repository)
func (h *RBACAuditHandler) GetAuditEvent(c *gin.Context) {
	correlationID := fmt.Sprintf("audit-event-get-%d", time.Now().UnixNano())
	eventID := c.Param("eventId")
	tenantID := c.GetString("tenant_id")

	h.logger.Info("Getting audit event", "eventId", eventID, "tenantId", tenantID, "correlation_id", correlationID)

	// For now, we'll get all events and filter by ID
	// In a real implementation, the repository might support GetAuditEventByID
	filters := rbac.AuditFilters{
		Limit:  1000, // Get a reasonable number to search through
		Offset: 0,
	}

	events, err := h.rbacRepo.GetAuditEvents(c.Request.Context(), tenantID, filters)
	if err != nil {
		h.logger.Error("Failed to get audit events", "error", err, "correlation_id", correlationID, "tenantId", tenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit event"})
		return
	}

	// Find the specific event by ID
	var foundEvent *models.AuditLog
	for _, event := range events {
		if event.ID == eventID {
			foundEvent = event
			break
		}
	}

	if foundEvent == nil {
		h.logger.Warn("Audit event not found", "eventId", eventID, "correlation_id", correlationID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Audit event not found"})
		return
	}

	h.logger.Info("Audit event retrieved successfully", "eventId", eventID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, foundEvent)
}

// GetAuditSummary handles GET /api/v1/rbac/audit/summary
// Get audit events summary statistics
func (h *RBACAuditHandler) GetAuditSummary(c *gin.Context) {
	correlationID := fmt.Sprintf("audit-summary-get-%d", time.Now().UnixNano())
	tenantID := c.GetString("tenant_id")

	h.logger.Info("Getting audit summary", "tenantId", tenantID, "correlation_id", correlationID)

	// Get recent audit events for summary (last 24 hours by default)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	filters := rbac.AuditFilters{
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     10000, // Get enough events for summary
		Offset:    0,
	}

	events, err := h.rbacRepo.GetAuditEvents(c.Request.Context(), tenantID, filters)
	if err != nil {
		h.logger.Error("Failed to get audit events for summary", "error", err, "correlation_id", correlationID, "tenantId", tenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit summary"})
		return
	}

	// Calculate summary statistics
	summary := gin.H{
		"totalEvents": len(events),
		"timeRange": gin.H{
			"startTime": startTime.Format(time.RFC3339),
			"endTime":   endTime.Format(time.RFC3339),
		},
		"byResult":      make(map[string]int),
		"bySeverity":    make(map[string]int),
		"byAction":      make(map[string]int),
		"byResource":    make(map[string]int),
		"bySubjectType": make(map[string]int),
		"recentEvents":  make([]gin.H, 0),
	}

	// Process events for statistics
	for _, event := range events {
		// Count by result
		summary["byResult"].(map[string]int)[event.Result]++

		// Count by severity
		summary["bySeverity"].(map[string]int)[event.Severity]++

		// Count by action
		summary["byAction"].(map[string]int)[event.Action]++

		// Count by resource
		summary["byResource"].(map[string]int)[event.Resource]++

		// Count by subject type
		summary["bySubjectType"].(map[string]int)[event.SubjectType]++

		// Add recent events (last 10)
		if len(summary["recentEvents"].([]gin.H)) < 10 {
			summary["recentEvents"] = append(summary["recentEvents"].([]gin.H), gin.H{
				"id":        event.ID,
				"timestamp": event.Timestamp.Format(time.RFC3339),
				"action":    event.Action,
				"resource":  event.Resource,
				"result":    event.Result,
				"severity":  event.Severity,
			})
		}
	}

	h.logger.Info("Audit summary retrieved successfully", "totalEvents", len(events), "correlation_id", correlationID)
	c.JSON(http.StatusOK, summary)
}

// GetAuditEventsBySubject handles GET /api/v1/rbac/audit/subject/{subjectId}
// Get audit events for a specific subject
func (h *RBACAuditHandler) GetAuditEventsBySubject(c *gin.Context) {
	correlationID := fmt.Sprintf("audit-events-by-subject-%d", time.Now().UnixNano())
	subjectID := c.Param("subjectId")
	tenantID := c.GetString("tenant_id")

	h.logger.Info("Getting audit events by subject", "subjectId", subjectID, "tenantId", tenantID, "correlation_id", correlationID)

	filters := rbac.AuditFilters{
		SubjectID: &subjectID,
		Limit:     getIntQueryParam(c, "limit", 50),
		Offset:    getIntQueryParam(c, "offset", 0),
	}

	// Optional time range filters
	if startTimeStr := c.Query("startTime"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filters.StartTime = &startTime
		}
	}
	if endTimeStr := c.Query("endTime"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filters.EndTime = &endTime
		}
	}

	events, err := h.rbacRepo.GetAuditEvents(c.Request.Context(), tenantID, filters)
	if err != nil {
		h.logger.Error("Failed to get audit events by subject", "error", err, "correlation_id", correlationID, "subjectId", subjectID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit events for subject"})
		return
	}

	h.logger.Info("Audit events by subject retrieved successfully", "subjectId", subjectID, "count", len(events), "correlation_id", correlationID)
	c.JSON(http.StatusOK, gin.H{
		"subjectId": subjectID,
		"events":    events,
		"total":     len(events),
	})
}
