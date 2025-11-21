package models

import (
	"time"
)

// KPIDefinition represents a KPI definition stored in Weaviate
type KPIDefinition struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // "business" or "tech"
	Name string `json:"name"`
	// Namespace groups related KPIs (e.g. file or collection name).
	// Example: "apigw_springboot_top_10_kpi".
	Namespace string `json:"namespace,omitempty"`
	// Source identifies where this KPI definition originated (seed file, tool, etc.).
	Source string `json:"source,omitempty"`
	// SourceID is a short identifier within the source (e.g. CSV row key).
	SourceID string                 `json:"sourceId,omitempty"`
	Unit     string                 `json:"unit"`
	Format   string                 `json:"format"`
	Query    map[string]interface{} `json:"query"` // JSON query definition
	// Layer indicates whether the KPI is an impact or cause signal. Allowed: "impact", "cause".
	Layer string `json:"layer,omitempty"`
	// SignalType is the high-level signal kind (metrics, traces, logs, business, synthetic, ...).
	SignalType string `json:"signalType,omitempty"`
	// Classifier is the measurement category (latency, errors, tps, cpu_utilization, anomaly_score, ...).
	Classifier string `json:"classifier,omitempty"`
	// Datastore is the telemetry/metrics store where this KPI is sourced from (victoriametrics, clickhouse, ...).
	Datastore string `json:"datastore,omitempty"`
	// QueryType indicates the query language or type (MetricsQL, SQL, PromQL, etc.).
	QueryType string `json:"queryType,omitempty"`
	// Formula holds the raw query/formula string; treated as opaque by the engines.
	Formula    string      `json:"formula,omitempty"`
	Thresholds []Threshold `json:"thresholds"` // JSON thresholds array
	Tags       []string    `json:"tags"`
	Definition string      `json:"definition"` // Definition of what the signal means
	Sentiment  string      `json:"sentiment"`  // "NEGATIVE", "POSITIVE", or "NEUTRAL" - increase sentiment
	// Category is a free-form category for additional grouping/classification.
	Category string `json:"category,omitempty"`
	// RetryAllowed indicates whether automated retry logic is permitted for incidents related to this KPI.
	RetryAllowed bool `json:"retryAllowed,omitempty"`
	// Domain indicates the business or technical domain this KPI applies to (payments, kafka, cassandra, infra, ...).
	Domain string `json:"domain,omitempty"`
	// ServiceFamily groups related services (apigw, oltp, issuer-bank, ...).
	ServiceFamily string `json:"serviceFamily,omitempty"`
	// ComponentType indicates the type of component this KPI maps to (springboot, kafka-broker, cassandra-node, node, valkey, ...).
	ComponentType string                 `json:"componentType,omitempty"`
	Sparkline     map[string]interface{} `json:"sparkline"`  // JSON sparkline config
	Visibility    string                 `json:"visibility"` // "private", "team", "org"
	// BusinessImpact explains the user/business consequence when this KPI degrades.
	BusinessImpact string `json:"businessImpact,omitempty"`
	// EmotionalImpact is an optional short severity/emotive hint for narrative generation.
	EmotionalImpact string `json:"emotionalImpact,omitempty"`
	// Examples contains example values/contexts; stored as arbitrary JSON objects.
	Examples []map[string]interface{} `json:"examples,omitempty"`
	// AggregationWindowHint suggests a preferred aggregation window (e.g. "1m", "5m").
	AggregationWindowHint string `json:"aggregationWindowHint,omitempty"`
	// DimensionsHint lists key dimensions expected on this KPI (e.g. ["service.name", "orgId"]).
	DimensionsHint []string  `json:"dimensionsHint,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// KPIDefinitionRequest represents a request to create/update a KPI definition
type KPIDefinitionRequest struct {
	KPIDefinition *KPIDefinition `json:"kpiDefinition"`
}

// KPIDefinitionResponse represents a response containing a KPI definition
type KPIDefinitionResponse struct {
	KPIDefinition *KPIDefinition `json:"kpiDefinition"`
}

// (deprecated) KPIListRequest was the original simple list request. Use the
// KPIListRequest alias (below) which includes semantic filters.

// Extended semantic filters for KPI discovery by engines
type KPIListSemanticFilters struct {
	Layer         string `json:"layer,omitempty"`
	SignalType    string `json:"signalType,omitempty"`
	Classifier    string `json:"classifier,omitempty"`
	Datastore     string `json:"datastore,omitempty"`
	Sentiment     string `json:"sentiment,omitempty"`
	Domain        string `json:"domain,omitempty"`
	ServiceFamily string `json:"serviceFamily,omitempty"`
	ComponentType string `json:"componentType,omitempty"`
}

// KPIListRequest represents a request to list KPI definitions including semantic filters
type KPIListRequestV2 struct {
	Kind   string   `form:"kind" json:"kind,omitempty"`
	Tags   []string `form:"tags" json:"tags,omitempty"`
	Limit  int      `form:"limit" json:"limit,omitempty"`
	Offset int      `form:"offset" json:"offset,omitempty"`

	// Semantic filters
	Layer         string `form:"layer" json:"layer,omitempty"`
	SignalType    string `form:"signalType" json:"signalType,omitempty"`
	Classifier    string `form:"classifier" json:"classifier,omitempty"`
	Datastore     string `form:"datastore" json:"datastore,omitempty"`
	Sentiment     string `form:"sentiment" json:"sentiment,omitempty"`
	Domain        string `form:"domain" json:"domain,omitempty"`
	ServiceFamily string `form:"serviceFamily" json:"serviceFamily,omitempty"`
	ComponentType string `form:"componentType" json:"componentType,omitempty"`
}

// keep original name for compatibility in code that imported KPIListRequest
type KPIListRequest = KPIListRequestV2

// KPIListResponse represents a response containing a list of KPI definitions
type KPIListResponse struct {
	KPIDefinitions []*KPIDefinition `json:"kpiDefinitions"`
	Total          int              `json:"total"`
	NextOffset     int              `json:"nextOffset,omitempty"`
}
