package weavstore

import "time"

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
