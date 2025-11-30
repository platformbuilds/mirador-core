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
	CorrelationID    string   `json:"correlation_id"`
	IncidentID       string   `json:"incident_id"`
	RootCause        string   `json:"root_cause"`
	Confidence       float64  `json:"confidence"`
	AffectedServices []string `json:"affected_services"`
	// Causes is an additive field containing candidate causes with computed
	// suspicion scores and correlation statistics. This field is optional and
	// preserves backwards compatibility of existing responses.
	Causes          []CauseCandidate `json:"causes,omitempty"`
	Timeline        []TimelineEvent  `json:"timeline"`
	RedAnchors      []*RedAnchor     `json:"red_anchors"`
	Recommendations []string         `json:"recommendations"`
	CreatedAt       time.Time        `json:"created_at"`
}

// CorrelationStats holds statistical correlation outputs for an Impact<->Cause pair.
type CorrelationStats struct {
	Pearson      float64 `json:"pearson"`
	Spearman     float64 `json:"spearman"`
	CrossCorrMax float64 `json:"cross_correlation_max"`
	CrossCorrLag int     `json:"cross_correlation_lag"`
	// NOTE(AT-007): Partial correlation currently a placeholder; see action tracker AT-007
	Partial    float64 `json:"partial"`
	SampleSize int     `json:"sample_size"`
	PValue     float64 `json:"p_value"`
	Confidence float64 `json:"confidence"`
}

// CauseCandidate represents a candidate cause KPI or service with computed
// suspicion score and correlation stats.
type CauseCandidate struct {
	// KPI is the human-readable identifier for the candidate KPI/service.
	// For backward-compatibility this field will contain the human-friendly
	// name when available (previously it contained the raw UUID or id).
	KPI string `json:"kpi"`
	// KPIUUID retains the original KPI identifier (UUID or registry id) when
	// the engine resolves a KPI definition. This preserves machine-usable
	// identifiers for clients that relied on the raw id.
	KPIUUID string `json:"kpiUuid,omitempty"`
	// KPIFormula contains the KPI formula or query string when available.
	KPIFormula     string            `json:"kpiFormula,omitempty"`
	Service        string            `json:"service,omitempty"`
	SuspicionScore float64           `json:"suspicion_score"`
	Reasons        []string          `json:"reasons,omitempty"`
	Stats          *CorrelationStats `json:"stats,omitempty"`
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
	Service string `json:"service,omitempty"`
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
	CorrelationID string `json:"correlation_id"`
	Correct       bool   `json:"correct"`
	Notes         string `json:"notes,omitempty"`
}

type FeedbackResponse struct {
	CorrelationID string `json:"correlation_id"`
	Accepted      bool   `json:"accepted"`
}
