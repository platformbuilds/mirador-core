package models

import "time"

// MetricsQL Models
type MetricsQLQueryRequest struct {
	Query    string `json:"query" binding:"required"`
	Time     string `json:"time,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
	TenantID string `json:"-"` // Set by middleware
}

type MetricsQLRangeQueryRequest struct {
	Query    string `json:"query" binding:"required"`
	Start    string `json:"start" binding:"required"`
	End      string `json:"end" binding:"required"`
	Step     string `json:"step" binding:"required"`
	TenantID string `json:"-"`
}

type MetricsQLQueryResult struct {
	Status        string      `json:"status"`
	Data          interface{} `json:"data"`
	SeriesCount   int         `json:"series_count"`
	ExecutionTime int64       `json:"execution_time_ms"`
}

type MetricsQLRangeQueryResult struct {
	Status         string      `json:"status"`
	Data           interface{} `json:"data"`
	DataPointCount int         `json:"data_point_count"`
}

type MetricsQLQueryResponse struct {
	Data          interface{} `json:"data"`
	ExecutionTime int64       `json:"execution_time"`
	Timestamp     time.Time   `json:"timestamp"`
}

// LogsQL Models
type LogsQLQueryRequest struct {
	Query    string `json:"query" binding:"required"`
	Limit    int    `json:"limit,omitempty"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	TenantID string `json:"-"`
}

type LogsQLQueryResult struct {
	Logs   []map[string]interface{} `json:"logs"`
	Fields []string                 `json:"fields"`
	Stats  map[string]interface{}   `json:"stats,omitempty"`
}

type LogsQLResponse struct {
	Status string                   `json:"status"`
	Data   []map[string]interface{} `json:"data"`
}

// VictoriaTraces Models
type TraceSearchRequest struct {
	Service      string    `json:"service,omitempty"`
	Operation    string    `json:"operation,omitempty"`
	Tags         string    `json:"tags,omitempty"`
	MinDuration  string    `json:"minDuration,omitempty"`
	MaxDuration  string    `json:"maxDuration,omitempty"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	Limit        int       `json:"limit,omitempty"`
	TenantID     string    `json:"-"`
}

type TraceSearchResult struct {
	Traces     []map[string]interface{} `json:"traces"`
	Total      int                      `json:"total"`
	SearchTime int64                    `json:"search_time_ms"`
}

type Trace struct {
	TraceID   string                 `json:"traceID"`
	Spans     []map[string]interface{} `json:"spans"`
	Processes map[string]interface{}   `json:"processes"`
}

// AI Engine Models
type FractureAnalysisRequest struct {
	Component  string   `json:"component" binding:"required"`
	TimeRange  string   `json:"time_range" binding:"required"`
	ModelTypes []string `json:"model_types,omitempty"`
	TenantID   string   `json:"-"`
}

type FractureAnalysisResponse struct {
	Fractures        []*SystemFracture `json:"fractures"`
	ModelsUsed       []string          `json:"models_used"`
	ProcessingTimeMs int64             `json:"processing_time_ms"`
}

type RCAInvestigationRequest struct {
	IncidentID       string    `json:"incident_id" binding:"required"`
	Symptoms         []string  `json:"symptoms" binding:"required"`
	TimeRange        TimeRange `json:"time_range" binding:"required"`
	AffectedServices []string  `json:"affected_services,omitempty"`
	AnomalyThreshold float64   `json:"anomaly_threshold,omitempty"`
	TenantID         string    `json:"-"`
}

// Configuration Models
type DataSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // metrics, logs, traces
	URL      string `json:"url"`
	Status   string `json:"status"` // connected, degraded, disconnected
	TenantID string `json:"tenant_id"`
}

type FractureListResponse struct {
	Fractures []*SystemFracture `json:"fractures"`
	Summary   FractureSummary   `json:"summary"`
}

type FractureSummary struct {
	Total         int           `json:"total"`
	HighRisk      int           `json:"high_risk"`
	MediumRisk    int           `json:"medium_risk"`
	LowRisk       int           `json:"low_risk"`
	AvgTimeToFail time.Duration `json:"avg_time_to_fail"`
}
