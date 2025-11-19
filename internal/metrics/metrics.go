// ================================
// internal/metrics/metrics.go - Self-monitoring for MIRADOR-CORE
// ================================

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Request metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// gRPC Client metrics
	GRPCRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_grpc_requests_total",
			Help: "Total number of gRPC requests to AI engines",
		},
		[]string{"service", "method", "status"},
	)

	GRPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)

	// Valkey Cluster cache metrics
	CacheRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_cache_requests_total",
			Help: "Total number of cache requests",
		},
		[]string{"operation", "result"}, // get/set/delete, hit/miss/error
	)

	CacheRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_cache_request_duration_seconds",
			Help:    "Cache request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		},
		[]string{"operation"},
	)

	// Active connections and sessions
	ActiveWebSocketConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mirador_core_websocket_connections_active",
			Help: "Number of active WebSocket connections",
		},
		[]string{"stream_type"},
	)

	ActiveSessions = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mirador_core_sessions_active",
			Help: "Number of active user sessions",
		},
		[]string{},
	)

	// Query processing metrics
	QueryExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_query_execution_duration_seconds",
			Help:    "Query execution duration in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
		},
		[]string{"query_type"}, // metricsql, logsql, traces
	)

	// AI Engine integration metrics
	CorrelationsFound = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_correlations_found_total",
			Help: "Total number of correlations found by RCA engine",
		},
		[]string{"confidence_level"},
	)

	// External integration metrics
	NotificationsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_notifications_sent_total",
			Help: "Total number of notifications sent",
		},
		[]string{"integration", "type", "success"}, // slack/teams/email, alert/correlation, true/false
	)
)
