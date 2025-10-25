package models

import (
	"time"
)

// QueryType represents the type of query being executed
type QueryType string

const (
	QueryTypeMetrics     QueryType = "metrics"
	QueryTypeLogs        QueryType = "logs"
	QueryTypeTraces      QueryType = "traces"
	QueryTypeCorrelation QueryType = "correlation"
)

// UnifiedQuery represents a query that can be executed across multiple engines
type UnifiedQuery struct {
	// Common fields
	ID        string     `json:"id"`
	Type      QueryType  `json:"type"`
	Query     string     `json:"query"`
	TenantID  string     `json:"tenant_id,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Timeout   string     `json:"timeout,omitempty"`

	// Engine-specific parameters
	Parameters map[string]interface{} `json:"parameters,omitempty"`

	// Correlation options
	CorrelationOptions *CorrelationOptions `json:"correlation_options,omitempty"`

	// Caching options
	CacheOptions *CacheOptions `json:"cache_options,omitempty"`
}

// CorrelationOptions defines how to correlate data across engines
type CorrelationOptions struct {
	Enabled         bool                   `json:"enabled"`
	CorrelationKeys []string               `json:"correlation_keys"`
	TimeWindow      time.Duration          `json:"time_window"`
	Engines         []QueryType            `json:"engines"`
	Filters         map[string]interface{} `json:"filters,omitempty"`
}

// CacheOptions defines caching behavior for queries
type CacheOptions struct {
	Enabled     bool          `json:"enabled"`
	TTL         time.Duration `json:"ttl"`
	Key         string        `json:"key,omitempty"` // Custom cache key
	BypassCache bool          `json:"bypass_cache"`  // Force fresh query
}

// UnifiedResult represents the result of a unified query
type UnifiedResult struct {
	QueryID       string                    `json:"query_id"`
	Type          QueryType                 `json:"type"`
	Status        string                    `json:"status"`
	Data          interface{}               `json:"data"`
	Metadata      *ResultMetadata           `json:"metadata"`
	Correlations  *UnifiedCorrelationResult `json:"correlations,omitempty"`
	ExecutionTime int64                     `json:"execution_time_ms"`
	Cached        bool                      `json:"cached"`
}

// ResultMetadata contains metadata about the query execution
type ResultMetadata struct {
	EngineResults map[QueryType]*EngineResult `json:"engine_results"`
	TotalRecords  int                         `json:"total_records"`
	DataSources   []string                    `json:"data_sources"`
	Warnings      []string                    `json:"warnings,omitempty"`
}

// EngineResult contains result information from a specific engine
type EngineResult struct {
	Engine        QueryType `json:"engine"`
	Status        string    `json:"status"`
	RecordCount   int       `json:"record_count"`
	ExecutionTime int64     `json:"execution_time_ms"`
	Error         string    `json:"error,omitempty"`
	DataSource    string    `json:"data_source"`
}

// CorrelationResult contains correlation data across engines
type UnifiedCorrelationResult struct {
	Correlations []Correlation      `json:"correlations"`
	Summary      CorrelationSummary `json:"summary"`
}

// Correlation represents a correlation between data points from different engines
type Correlation struct {
	ID             string                    `json:"id"`
	CorrelationKey string                    `json:"correlation_key"`
	Timestamp      time.Time                 `json:"timestamp"`
	Engines        map[QueryType]interface{} `json:"engines"`
	Confidence     float64                   `json:"confidence"`
	Metadata       map[string]interface{}    `json:"metadata,omitempty"`
}

// CorrelationSummary provides summary statistics for correlations
type CorrelationSummary struct {
	TotalCorrelations int         `json:"total_correlations"`
	AverageConfidence float64     `json:"average_confidence"`
	TimeRange         string      `json:"time_range"`
	EnginesInvolved   []QueryType `json:"engines_involved"`
}

// UnifiedQueryRequest wraps a UnifiedQuery for API requests
type UnifiedQueryRequest struct {
	Query *UnifiedQuery `json:"query"`
}

// UnifiedQueryResponse wraps a UnifiedResult for API responses
type UnifiedQueryResponse struct {
	Result *UnifiedResult `json:"result"`
}

// QueryMetadata contains information about supported query capabilities
type QueryMetadata struct {
	SupportedEngines  []QueryType            `json:"supported_engines"`
	QueryCapabilities map[QueryType][]string `json:"query_capabilities"`
	CacheCapabilities CacheCapabilities      `json:"cache_capabilities"`
}

// CacheCapabilities describes caching capabilities
type CacheCapabilities struct {
	Supported  bool          `json:"supported"`
	DefaultTTL time.Duration `json:"default_ttl"`
	MaxTTL     time.Duration `json:"max_ttl"`
}

// EngineHealthStatus represents the health status of all engines
type EngineHealthStatus struct {
	OverallHealth string               `json:"overall_health"`
	EngineHealth  map[QueryType]string `json:"engine_health"`
	LastChecked   time.Time            `json:"last_checked"`
}
