package models

import (
	"strings"
	"time"
)

// MetricType represents the type of a Prometheus metric
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
	MetricTypeUnknown   MetricType = "unknown"
)

// MetricMetadataDocument represents a metric's metadata for indexing in Bleve
type MetricMetadataDocument struct {
	// Core metric information
	ID         string     `json:"id"`          // Unique identifier (tenant:metric_name)
	MetricName string     `json:"metric_name"` // The metric name (e.g., "http_requests_total")
	TenantID   string     `json:"tenant_id"`   // Tenant identifier
	Type       MetricType `json:"type"`        // Metric type (counter, gauge, histogram, summary)

	// Descriptive information
	Description string `json:"description,omitempty"` // Human-readable description
	Unit        string `json:"unit,omitempty"`        // Unit of measurement (e.g., "seconds", "bytes")

	// Label information
	Labels     map[string][]string `json:"labels"`      // Map of label names to their possible values
	LabelNames []string            `json:"label_names"` // All label names for this metric

	// Search and discovery fields
	SearchText string   `json:"search_text"` // Concatenated text for full-text search
	Tags       []string `json:"tags"`        // Additional tags for categorization

	// Metadata timestamps
	FirstSeen   time.Time `json:"first_seen"`   // When this metric was first discovered
	LastSeen    time.Time `json:"last_seen"`    // When this metric was last seen
	LastUpdated time.Time `json:"last_updated"` // When this document was last updated

	// Additional metadata
	IsActive    bool   `json:"is_active"`    // Whether this metric is currently active
	SampleCount int64  `json:"sample_count"` // Number of samples seen
	DataSource  string `json:"data_source"`  // Source system (e.g., "victoria-metrics")
}

// MetricMetadataSearchRequest represents a search request for metrics metadata
type MetricMetadataSearchRequest struct {
	Query    string            `json:"query"`               // Search query (supports Bleve query syntax)
	TenantID string            `json:"tenant_id,omitempty"` // Filter by tenant
	Types    []MetricType      `json:"types,omitempty"`     // Filter by metric types
	Labels   map[string]string `json:"labels,omitempty"`    // Filter by specific label values
	Limit    int               `json:"limit,omitempty"`     // Maximum number of results
	Offset   int               `json:"offset,omitempty"`    // Pagination offset
}

// MetricMetadataSearchResult represents the result of a metrics metadata search
type MetricMetadataSearchResult struct {
	Metrics    []*MetricMetadataDocument `json:"metrics"`     // Matching metrics
	TotalCount int                       `json:"total_count"` // Total number of matches
	QueryTime  int64                     `json:"query_time"`  // Query execution time in milliseconds
}

// MetricMetadataSyncRequest represents a request to sync metrics metadata
type MetricMetadataSyncRequest struct {
	TenantID      string     `json:"tenant_id,omitempty"`  // Tenant to sync (empty for all)
	ForceFullSync bool       `json:"force_full_sync"`      // Force full resync instead of incremental
	TimeRange     *TimeRange `json:"time_range,omitempty"` // Time range to scan for metrics
	BatchSize     int        `json:"batch_size,omitempty"` // Batch size for processing
}

// MetricMetadataSyncResult represents the result of a metadata sync operation
type MetricMetadataSyncResult struct {
	TenantID         string    `json:"tenant_id"`
	MetricsProcessed int       `json:"metrics_processed"`
	MetricsAdded     int       `json:"metrics_added"`
	MetricsUpdated   int       `json:"metrics_updated"`
	MetricsRemoved   int       `json:"metrics_removed"`
	Duration         int64     `json:"duration_ms"`
	LastSyncTime     time.Time `json:"last_sync_time"`
	Errors           []string  `json:"errors,omitempty"`
}

// MetricMetadataHealthStatus represents the health status of the metadata indexer
type MetricMetadataHealthStatus struct {
	IsHealthy      bool      `json:"is_healthy"`
	LastSyncTime   time.Time `json:"last_sync_time"`
	TotalMetrics   int64     `json:"total_metrics"`
	ActiveMetrics  int64     `json:"active_metrics"`
	IndexSizeBytes int64     `json:"index_size_bytes"`
	SyncErrors     []string  `json:"sync_errors,omitempty"`
}

// SyncStrategy represents different synchronization strategies
type SyncStrategy string

const (
	SyncStrategyFull        SyncStrategy = "full"        // Full resync of all metrics
	SyncStrategyIncremental SyncStrategy = "incremental" // Only sync changes since last sync
	SyncStrategyHybrid      SyncStrategy = "hybrid"      // Incremental with periodic full sync
)

// MetricMetadataSyncConfig represents configuration for metadata synchronization
type MetricMetadataSyncConfig struct {
	Enabled           bool          `json:"enabled"`
	Strategy          SyncStrategy  `json:"strategy"`
	Interval          time.Duration `json:"interval"`            // How often to run sync
	FullSyncInterval  time.Duration `json:"full_sync_interval"`  // How often to do full sync (for hybrid)
	BatchSize         int           `json:"batch_size"`          // Batch size for processing
	MaxRetries        int           `json:"max_retries"`         // Max retry attempts
	RetryDelay        time.Duration `json:"retry_delay"`         // Delay between retries
	TimeRangeLookback time.Duration `json:"time_range_lookback"` // How far back to look for metrics
}

// MetricMetadataSyncState represents the current state of synchronization
type MetricMetadataSyncState struct {
	TenantID           string    `json:"tenant_id"`
	LastSyncTime       time.Time `json:"last_sync_time"`
	LastFullSyncTime   time.Time `json:"last_full_sync_time"`
	TotalSyncs         int64     `json:"total_syncs"`
	SuccessfulSyncs    int64     `json:"successful_syncs"`
	FailedSyncs        int64     `json:"failed_syncs"`
	MetricsInIndex     int64     `json:"metrics_in_index"`
	LastError          string    `json:"last_error,omitempty"`
	LastErrorTime      time.Time `json:"last_error_time,omitempty"`
	IsCurrentlySyncing bool      `json:"is_currently_syncing"`
}

// MetricMetadataSyncStatus represents the status of a sync operation
type MetricMetadataSyncStatus struct {
	TenantID         string        `json:"tenant_id"`
	Status           string        `json:"status"` // "running", "completed", "failed"
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time,omitempty"`
	Strategy         SyncStrategy  `json:"strategy"`
	MetricsProcessed int           `json:"metrics_processed"`
	MetricsAdded     int           `json:"metrics_added"`
	MetricsUpdated   int           `json:"metrics_updated"`
	MetricsRemoved   int           `json:"metrics_removed"`
	Errors           []string      `json:"errors,omitempty"`
	Duration         time.Duration `json:"duration,omitempty"`
}

// NewMetricMetadataDocument creates a new MetricMetadataDocument with default values
func NewMetricMetadataDocument(metricName, tenantID string) *MetricMetadataDocument {
	now := time.Now()
	doc := &MetricMetadataDocument{
		ID:          tenantID + ":" + metricName,
		MetricName:  metricName,
		TenantID:    tenantID,
		Type:        InferMetricType(metricName),
		Labels:      make(map[string][]string),
		LabelNames:  []string{},
		Tags:        []string{},
		FirstSeen:   now,
		LastSeen:    now,
		LastUpdated: now,
		IsActive:    true,
		DataSource:  "victoria-metrics",
	}
	doc.updateSearchText()
	return doc
}

// UpdateLabels updates the label information for this metric
func (m *MetricMetadataDocument) UpdateLabels(labels map[string][]string) {
	m.Labels = labels
	m.LabelNames = make([]string, 0, len(labels))
	for name := range labels {
		m.LabelNames = append(m.LabelNames, name)
	}
	m.LastUpdated = time.Now()
	m.updateSearchText()
}

// MarkSeen updates the last seen timestamp and marks as active
func (m *MetricMetadataDocument) MarkSeen() {
	now := time.Now()
	m.LastSeen = now
	m.LastUpdated = now
	m.IsActive = true
	m.SampleCount++
}

// MarkInactive marks the metric as inactive
func (m *MetricMetadataDocument) MarkInactive() {
	m.IsActive = false
	m.LastUpdated = time.Now()
}

// updateSearchText updates the search_text field for full-text search
func (m *MetricMetadataDocument) updateSearchText() {
	text := m.MetricName + " " + string(m.Type)
	if m.Description != "" {
		text += " " + m.Description
	}
	if m.Unit != "" {
		text += " " + m.Unit
	}
	for name, values := range m.Labels {
		text += " " + name
		for _, value := range values {
			text += " " + value
		}
	}
	for _, tag := range m.Tags {
		text += " " + tag
	}
	m.SearchText = text
}

// InferMetricType attempts to infer the metric type from the metric name
func InferMetricType(metricName string) MetricType {
	// Common patterns for different metric types
	if containsAny(metricName, "_total", "_count", "_sum") {
		return MetricTypeCounter
	}
	if containsAny(metricName, "_bucket", "_histogram") {
		return MetricTypeHistogram
	}
	if containsAny(metricName, "_quantile", "_summary") {
		return MetricTypeSummary
	}
	// Default to gauge for most other metrics
	return MetricTypeGauge
}

// containsAny checks if the string contains any of the given substrings
func containsAny(s string, substrings ...string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
