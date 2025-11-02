package models

import "time"

// RedAnchor represents anomaly score pattern for RCA
type RedAnchor struct {
	Service   string    `json:"service"`
	Metric    string    `json:"metric"`
	Score     float64   `json:"anomaly_score"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
	DataType  string    `json:"data_type"` // metrics, logs, traces
}

// CorrelationResult from RCA-ENGINE analysis
type CorrelationResult struct {
	CorrelationID    string          `json:"correlation_id"`
	IncidentID       string          `json:"incident_id"`
	RootCause        string          `json:"root_cause"`
	Confidence       float64         `json:"confidence"`
	AffectedServices []string        `json:"affected_services"`
	Timeline         []TimelineEvent `json:"timeline"`
	RedAnchors       []*RedAnchor    `json:"red_anchors"`
	Recommendations  []string        `json:"recommendations"`
	CreatedAt        time.Time       `json:"created_at"`
}

// CorrelationEvent for VictoriaLogs storage
type CorrelationEvent struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	IncidentID string          `json:"incident_id"`
	RootCause  string          `json:"root_cause"`
	Confidence float64         `json:"confidence"`
	RedAnchors []*RedAnchor    `json:"red_anchors"`
	Timeline   []TimelineEvent `json:"timeline"`
	CreatedAt  time.Time       `json:"created_at"`
	TenantID   string          `json:"tenant_id,omitempty"`
}

// UserSession for Valkey cluster session management
type UserSession struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	TenantID     string                 `json:"tenant_id"`
	Roles        []string               `json:"roles"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActivity time.Time              `json:"last_activity"`
	Settings     map[string]interface{} `json:"user_settings"` // User-driven settings
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
}

// TimelineEvent represents events in incident correlation
type TimelineEvent struct {
	Time         time.Time `json:"time"`
	Event        string    `json:"event"`
	Service      string    `json:"service"`
	Severity     string    `json:"severity"`
	AnomalyScore float64   `json:"anomaly_score"`
	DataSource   string    `json:"data_source"` // metrics, logs, traces
}

// RCA List Correlations models
type ListCorrelationsRequest struct {
	TenantID  string     `json:"tenant_id"`
	Service   string     `json:"service,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	PageSize  int32      `json:"page_size,omitempty"`
	PageToken string     `json:"page_token,omitempty"`
}

type ListCorrelationsResponse struct {
	Correlations  []CorrelationResult `json:"correlations"`
	NextPageToken string              `json:"next_page_token,omitempty"`
}

// RCA Patterns models
type GetPatternsRequest struct {
	TenantID string `json:"tenant_id"`
	Service  string `json:"service,omitempty"`
}

type Pattern struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	Services        []string         `json:"services"`
	AnchorTemplates []AnchorTemplate `json:"anchor_templates"`
	Prevalence      float64          `json:"prevalence"`
	LastSeen        time.Time        `json:"last_seen"`
	Quality         Quality          `json:"quality"`
}

type AnchorTemplate struct {
	Service        string  `json:"service"`
	SignalType     string  `json:"signal_type"`
	Selector       string  `json:"selector"`
	TypicalLeadLag float64 `json:"typical_lead_lag"`
	Threshold      float64 `json:"threshold"`
}

type Quality struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
}

type GetPatternsResponse struct {
	Patterns []Pattern `json:"patterns"`
}

// RCA Feedback models
type FeedbackRequest struct {
	TenantID      string `json:"tenant_id"`
	CorrelationID string `json:"correlation_id"`
	Correct       bool   `json:"correct"`
	Notes         string `json:"notes,omitempty"`
}

type FeedbackResponse struct {
	CorrelationID string `json:"correlation_id"`
	Accepted      bool   `json:"accepted"`
}
