package models

import "time"

// ServiceGraphRequest captures the time window and optional filters used when
// extracting topology metrics from VictoriaMetrics.
type ServiceGraphRequest struct {
	Start          FlexibleTime `json:"start" binding:"required"`
	End            FlexibleTime `json:"end" binding:"required"`
	Client         string       `json:"client,omitempty"`
	Server         string       `json:"server,omitempty"`
	ConnectionType string       `json:"connection_type,omitempty"`
}

// ServiceGraphWindow summarises the evaluated time window.
type ServiceGraphWindow struct {
	Start           time.Time `json:"start"`
	End             time.Time `json:"end"`
	DurationSeconds int64     `json:"duration_seconds"`
}

// ServiceGraphLatency aggregates latency statistics for a directed edge.
type ServiceGraphLatency struct {
	AverageMs float64 `json:"avg_ms"`
}

// ServiceGraphEdge represents a directed dependency between two services.
type ServiceGraphEdge struct {
	Source         string              `json:"source"`
	Target         string              `json:"target"`
	ConnectionType string              `json:"connection_type,omitempty"`
	CallCount      float64             `json:"call_count"`
	CallRate       float64             `json:"call_rate"`
	ErrorCount     float64             `json:"error_count"`
	ErrorRate      float64             `json:"error_rate"`
	ServerLatency  ServiceGraphLatency `json:"server_latency_ms"`
	ClientLatency  ServiceGraphLatency `json:"client_latency_ms"`
	UnpairedSpans  float64             `json:"unpaired_spans"`
	DroppedSpans   float64             `json:"dropped_spans"`
}

// ServiceGraphData bundles the computed edges with window metadata.
type ServiceGraphData struct {
	Window ServiceGraphWindow `json:"window"`
	Edges  []ServiceGraphEdge `json:"edges"`
}
