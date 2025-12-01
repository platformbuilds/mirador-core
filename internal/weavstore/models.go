package weavstore

import "time"

// FailureSignal represents a single error or anomaly signal in a failure record
type FailureSignal struct {
	SignalType string         `json:"signal_type"` // "span" or "metric"
	MetricName string         `json:"metric_name"` // Only for metric signals
	Service    string         `json:"service"`
	Component  string         `json:"component"`
	Data       map[string]any `json:"data"` // Raw signal data
	Timestamp  time.Time      `json:"timestamp"`
}

// TimeRange represents a time window
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FailureRecord represents a complete failure detection record for Weaviate storage.
// This contains verbose, unprocessed raw signals for historical reference and analysis.
type FailureRecord struct {
	FailureUUID        string          `json:"failure_uuid"`        // Unique identifier (UUID v5)
	FailureID          string          `json:"failure_id"`          // Human-readable identifier
	TimeRange          TimeRange       `json:"time_range"`          // Detection time window
	Services           []string        `json:"services"`            // Affected services
	Components         []string        `json:"components"`          // Affected components
	RawErrorSignals    []FailureSignal `json:"raw_error_signals"`   // Unprocessed error signals
	RawAnomalySignals  []FailureSignal `json:"raw_anomaly_signals"` // Unprocessed anomaly signals
	DetectionTimestamp time.Time       `json:"detection_timestamp"` // When detection occurred
	DetectorVersion    string          `json:"detector_version"`    // Version of detection engine
	ConfidenceScore    float64         `json:"confidence_score"`    // Confidence in detection (0-1)
	CreatedAt          time.Time       `json:"created_at"`          // Record creation time
	UpdatedAt          time.Time       `json:"updated_at"`          // Last update time
}

// KPIDefinition represents a KPI definition in Weaviate.
// This is a local copy to avoid direct model imports (depguard compliance).
type KPIDefinition struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Kind            string           `json:"kind"`
	Namespace       string           `json:"namespace"`
	Source          string           `json:"source"`
	SourceID        string           `json:"sourceId"`
	Unit            string           `json:"unit"`
	Format          string           `json:"format"`
	Query           map[string]any   `json:"query"`
	Layer           string           `json:"layer"`
	SignalType      string           `json:"signalType"`
	Classifier      string           `json:"classifier"`
	Datastore       string           `json:"datastore"`
	QueryType       string           `json:"queryType"`
	Formula         string           `json:"formula"`
	Thresholds      []Threshold      `json:"thresholds"`
	Tags            []string         `json:"tags"`
	Definition      string           `json:"definition"`
	Sentiment       string           `json:"sentiment"`
	Category        string           `json:"category"`
	RetryAllowed    bool             `json:"retryAllowed"`
	Domain          string           `json:"domain"`
	ServiceFamily   string           `json:"serviceFamily"`
	ComponentType   string           `json:"componentType"`
	BusinessImpact  string           `json:"businessImpact"`
	EmotionalImpact string           `json:"emotionalImpact"`
	Examples        []map[string]any `json:"examples"`
	Sparkline       map[string]any   `json:"sparkline"`
	Visibility      string           `json:"visibility"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// Threshold represents a threshold configuration for a KPI.
type Threshold struct {
	Level       string  `json:"level"`
	Operator    string  `json:"operator"`
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

// KPIListRequest represents a request to list KPIs with pagination.
type KPIListRequest struct {
	Limit  int64 `json:"limit"`
	Offset int   `json:"offset"`
}
