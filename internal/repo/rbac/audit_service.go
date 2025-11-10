package rbac

import (
	"context"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
)

// AuditService provides structured audit logging for RBAC operations
type AuditService struct {
	repository RBACRepository
}

// NewAuditService creates a new audit service
func NewAuditService(repository RBACRepository) *AuditService {
	return &AuditService{
		repository: repository,
	}
}

// AuditEvent represents an audit event to be logged
type AuditEvent struct {
	TenantID       string
	SubjectID      string
	SubjectType    string
	Action         string
	Resource       string
	ResourceID     string
	Result         string
	Details        models.AuditLogDetails
	Severity       string
	Source         string
	CorrelationID  string
	RetentionClass string
}

// LogRoleCreated logs role creation events
func (s *AuditService) LogRoleCreated(ctx context.Context, tenantID, userID string, role *models.Role, correlationID string) error {
	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   userID,
		SubjectType: "user",
		Action:      "role.create",
		Resource:    "rbac.role",
		ResourceID:  role.ID,
		Result:      "success",
		Details: models.AuditLogDetails{
			Method:   "POST",
			Endpoint: "/api/v1/rbac/roles",
			OldValues: map[string]interface{}{
				"role": nil,
			},
			NewValues: map[string]interface{}{
				"role": map[string]interface{}{
					"name":        role.Name,
					"description": role.Description,
					"permissions": role.Permissions,
					"isSystem":    role.IsSystem,
				},
			},
			Metadata: map[string]interface{}{
				"justification": "Role creation for tenant access control",
			},
		},
		Severity:       "medium",
		Source:         "rbac_service",
		CorrelationID:  correlationID,
		RetentionClass: "standard",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// LogRoleUpdated logs role update events
func (s *AuditService) LogRoleUpdated(ctx context.Context, tenantID, userID string, oldRole, newRole *models.Role, correlationID string) error {
	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   userID,
		SubjectType: "user",
		Action:      "role.update",
		Resource:    "rbac.role",
		ResourceID:  newRole.ID,
		Result:      "success",
		Details: models.AuditLogDetails{
			Method:   "PUT",
			Endpoint: "/api/v1/rbac/roles",
			OldValues: map[string]interface{}{
				"description": oldRole.Description,
				"permissions": oldRole.Permissions,
			},
			NewValues: map[string]interface{}{
				"description": newRole.Description,
				"permissions": newRole.Permissions,
			},
			Metadata: map[string]interface{}{
				"justification": "Role modification for access control updates",
			},
		},
		Severity:       "medium",
		Source:         "rbac_service",
		CorrelationID:  correlationID,
		RetentionClass: "standard",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// LogRoleDeleted logs role deletion events
func (s *AuditService) LogRoleDeleted(ctx context.Context, tenantID, userID, roleName string, correlationID string) error {
	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   userID,
		SubjectType: "user",
		Action:      "role.delete",
		Resource:    "rbac.role",
		ResourceID:  roleName,
		Result:      "success",
		Details: models.AuditLogDetails{
			Method:   "DELETE",
			Endpoint: "/api/v1/rbac/roles",
			OldValues: map[string]interface{}{
				"role": roleName,
			},
			NewValues: map[string]interface{}{
				"role": nil,
			},
			Metadata: map[string]interface{}{
				"justification": "Role removal from tenant access control",
			},
		},
		Severity:       "high",
		Source:         "rbac_service",
		CorrelationID:  correlationID,
		RetentionClass: "extended",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// LogUserRoleAssigned logs user role assignment events
func (s *AuditService) LogUserRoleAssigned(ctx context.Context, tenantID, userID, targetUserID string, roles []string, correlationID string) error {
	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   userID,
		SubjectType: "user",
		Action:      "user.role.assign",
		Resource:    "rbac.user_role",
		ResourceID:  targetUserID,
		Result:      "success",
		Details: models.AuditLogDetails{
			Method:   "POST",
			Endpoint: "/api/v1/rbac/users/roles",
			OldValues: map[string]interface{}{
				"roles": []string{},
			},
			NewValues: map[string]interface{}{
				"roles": roles,
			},
			Metadata: map[string]interface{}{
				"targetUser":    targetUserID,
				"justification": "User role assignment for access control",
			},
		},
		Severity:       "high",
		Source:         "rbac_service",
		CorrelationID:  correlationID,
		RetentionClass: "extended",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// LogUserRoleRemoved logs user role removal events
func (s *AuditService) LogUserRoleRemoved(ctx context.Context, tenantID, userID, targetUserID string, roles []string, correlationID string) error {
	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   userID,
		SubjectType: "user",
		Action:      "user.role.remove",
		Resource:    "rbac.user_role",
		ResourceID:  targetUserID,
		Result:      "success",
		Details: models.AuditLogDetails{
			Method:   "DELETE",
			Endpoint: "/api/v1/rbac/users/roles",
			OldValues: map[string]interface{}{
				"roles": roles,
			},
			NewValues: map[string]interface{}{
				"roles": []string{},
			},
			Metadata: map[string]interface{}{
				"targetUser":    targetUserID,
				"justification": "User role removal for access control changes",
			},
		},
		Severity:       "high",
		Source:         "rbac_service",
		CorrelationID:  correlationID,
		RetentionClass: "extended",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// LogPermissionCheck logs permission evaluation events
func (s *AuditService) LogPermissionCheck(ctx context.Context, tenantID, userID, resource, action string, allowed bool, correlationID string) error {
	result := "denied"
	if allowed {
		result = "allowed"
	}

	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   userID,
		SubjectType: "user",
		Action:      "permission.check",
		Resource:    resource,
		ResourceID:  "",
		Result:      result,
		Details: models.AuditLogDetails{
			Method:   "GET",
			Endpoint: "/api/v1/rbac/check",
			Metadata: map[string]interface{}{
				"action":  action,
				"allowed": allowed,
			},
		},
		Severity:       "low",
		Source:         "rbac_service",
		CorrelationID:  correlationID,
		RetentionClass: "standard",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// LogAccessDenied logs access denial events
func (s *AuditService) LogAccessDenied(ctx context.Context, tenantID, userID, resource, action string, correlationID string) error {
	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   userID,
		SubjectType: "user",
		Action:      "access.denied",
		Resource:    resource,
		ResourceID:  "",
		Result:      "denied",
		Details: models.AuditLogDetails{
			Method:   "GET",
			Endpoint: "/api/v1/rbac/check",
			Metadata: map[string]interface{}{
				"action": action,
				"reason": "insufficient_permissions",
			},
		},
		Severity:       "medium",
		Source:         "rbac_service",
		CorrelationID:  correlationID,
		RetentionClass: "standard",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// LogSystemEvent logs system-level RBAC events
func (s *AuditService) LogSystemEvent(ctx context.Context, tenantID string, action, resource string, details map[string]interface{}, correlationID string) error {
	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   "system",
		SubjectType: "system",
		Action:      action,
		Resource:    resource,
		ResourceID:  "",
		Result:      "success",
		Details: models.AuditLogDetails{
			Method:   "SYSTEM",
			Endpoint: "internal",
			Metadata: details,
		},
		Severity:       "low",
		Source:         "rbac_system",
		CorrelationID:  correlationID,
		RetentionClass: "standard",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// LogError logs RBAC operation errors
func (s *AuditService) LogError(ctx context.Context, tenantID, userID, action, resource string, err error, correlationID string) error {
	event := &models.AuditLog{
		TenantID:    tenantID,
		Timestamp:   time.Now(),
		SubjectID:   userID,
		SubjectType: "user",
		Action:      action,
		Resource:    resource,
		ResourceID:  "",
		Result:      "error",
		Details: models.AuditLogDetails{
			Method:       "UNKNOWN",
			Endpoint:     "UNKNOWN",
			ErrorMessage: err.Error(),
			Metadata: map[string]interface{}{
				"error_type": fmt.Sprintf("%T", err),
			},
		},
		Severity:       "high",
		Source:         "rbac_service",
		CorrelationID:  correlationID,
		RetentionClass: "extended",
	}

	return s.repository.LogAuditEvent(ctx, event)
}

// GetAuditEvents retrieves audit events with filtering
func (s *AuditService) GetAuditEvents(ctx context.Context, tenantID string, filters AuditFilters) ([]*models.AuditLog, error) {
	return s.repository.GetAuditEvents(ctx, tenantID, filters)
}

// ValidateAuditCompliance checks if audit logging meets compliance requirements
func (s *AuditService) ValidateAuditCompliance(ctx context.Context, tenantID string, timeRange time.Duration) error {
	// Check if audit events are being logged within the required time range
	since := time.Now().Add(-timeRange)

	filters := AuditFilters{
		StartTime: &since,
		Limit:     1, // Just check if any events exist
	}

	events, err := s.repository.GetAuditEvents(ctx, tenantID, filters)
	if err != nil {
		return fmt.Errorf("failed to validate audit compliance: %w", err)
	}

	if len(events) == 0 {
		return fmt.Errorf("no audit events found in the last %v", timeRange)
	}

	// Additional compliance checks could be added here
	// - Check for required event types
	// - Validate retention policies
	// - Check for tampering attempts

	return nil
}

// CleanupOldAuditEvents removes audit events based on retention policies
func (s *AuditService) CleanupOldAuditEvents(ctx context.Context, tenantID string, retentionPeriod time.Duration) error {
	// This would typically be called by a scheduled job
	// Implementation would depend on the underlying storage capabilities
	// For now, this is a placeholder for future implementation

	monitoring.RecordAPIOperation("cleanup_audit_events", "rbac.audit", time.Since(time.Now()), true)
	return nil
}

// GetAuditSummary provides a summary of audit activity
func (s *AuditService) GetAuditSummary(ctx context.Context, tenantID string, timeRange time.Duration) (map[string]interface{}, error) {
	since := time.Now().Add(-timeRange)

	filters := AuditFilters{
		StartTime: &since,
		Limit:     1000, // Reasonable limit for summary
	}

	events, err := s.repository.GetAuditEvents(ctx, tenantID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit summary: %w", err)
	}

	summary := map[string]interface{}{
		"total_events":       len(events),
		"time_range":         timeRange.String(),
		"events_by_action":   make(map[string]int),
		"events_by_result":   make(map[string]int),
		"events_by_severity": make(map[string]int),
		"events_by_resource": make(map[string]int),
	}

	for _, event := range events {
		// Count by action
		if summary["events_by_action"].(map[string]int)[event.Action] == 0 {
			summary["events_by_action"].(map[string]int)[event.Action] = 1
		} else {
			summary["events_by_action"].(map[string]int)[event.Action]++
		}

		// Count by result
		if summary["events_by_result"].(map[string]int)[event.Result] == 0 {
			summary["events_by_result"].(map[string]int)[event.Result] = 1
		} else {
			summary["events_by_result"].(map[string]int)[event.Result]++
		}

		// Count by severity
		if summary["events_by_severity"].(map[string]int)[event.Severity] == 0 {
			summary["events_by_severity"].(map[string]int)[event.Severity] = 1
		} else {
			summary["events_by_severity"].(map[string]int)[event.Severity]++
		}

		// Count by resource
		if summary["events_by_resource"].(map[string]int)[event.Resource] == 0 {
			summary["events_by_resource"].(map[string]int)[event.Resource] = 1
		} else {
			summary["events_by_resource"].(map[string]int)[event.Resource]++
		}
	}

	return summary, nil
}
