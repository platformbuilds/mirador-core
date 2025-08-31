package models

import "time"

// SystemFracture represents a predicted system failure
type SystemFracture struct {
	ID                  string        `json:"id"`
	Component           string        `json:"component"`
	FractureType        string        `json:"fracture_type"` // fatigue, overload, degradation
	TimeToFracture      time.Duration `json:"time_to_fracture"`
	Severity            string        `json:"severity"` // high, medium, low
	Probability         float64       `json:"probability"`
	Confidence          float64       `json:"confidence"`
	ContributingFactors []string      `json:"contributing_factors"`
	Recommendation      string        `json:"recommendation"`
	PredictedAt         time.Time     `json:"predicted_at"`
}

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

// PredictionEvent for VictoriaLogs storage
type PredictionEvent struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Component    string                 `json:"component"`
	PredictedAt  time.Time              `json:"predicted_at"`
	IncidentTime time.Time              `json:"incident_time"`
	Probability  float64                `json:"probability"`
	Severity     string                 `json:"severity"`
	Confidence   float64                `json:"confidence"`
	TenantID     string                 `json:"tenant_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
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

// UserSession for Valley cluster session management
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
