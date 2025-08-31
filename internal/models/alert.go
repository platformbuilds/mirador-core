// ================================
// Complete internal/models/alert.go file
// ================================

package models

import "time"

type Alert struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Severity    string            `json:"severity"` // critical, warning, info
	Component   string            `json:"component"`
	Message     string            `json:"message"`
	Status      string            `json:"status"` // active, acknowledged, resolved
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Timestamp   time.Time         `json:"timestamp"`
	TenantID    string            `json:"tenant_id"`
	CreatedBy   string            `json:"created_by,omitempty"`
}

// AlertQuery represents a query for alerts
type AlertQuery struct {
	TenantID  string `json:"tenant_id"`
	Limit     int    `json:"limit,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Status    string `json:"status,omitempty"` // active, acknowledged, resolved
	Component string `json:"component,omitempty"`
}

type AlertRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Query       string            `json:"query"`      // MetricsQL, LogsQL, or Traces query
	QueryType   string            `json:"query_type"` // metricsql, logsql, traces
	Condition   string            `json:"condition"`  // e.g., "> 0.8", "< 100"
	Severity    string            `json:"severity"`
	Enabled     bool              `json:"enabled"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	TenantID    string            `json:"tenant_id"`
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ProcessedAlert struct {
	OriginalID    string    `json:"original_id"`
	ProcessedID   string    `json:"processed_id"`
	Action        string    `json:"action"` // fire, suppress, escalate, cluster
	ClusterID     string    `json:"cluster_id,omitempty"`
	Escalation    string    `json:"escalation,omitempty"`
	Notifications []string  `json:"notifications"` // slack, teams, email
	ProcessedAt   time.Time `json:"processed_at"`
}

type AlertAcknowledgment struct {
	AlertID        string    `json:"alert_id"`
	AcknowledgedBy string    `json:"acknowledged_by"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
	Comment        string    `json:"comment,omitempty"`
}

type Notification struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // alert, prediction, correlation
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Component string    `json:"component"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
	TenantID  string    `json:"tenant_id"`
}
