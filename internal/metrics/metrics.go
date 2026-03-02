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

	// MariaDB metrics
	MariaDBConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mirador_core_mariadb_connections_open",
			Help: "Number of open MariaDB connections",
		},
	)

	MariaDBConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mirador_core_mariadb_connections_in_use",
			Help: "Number of MariaDB connections currently in use",
		},
	)

	MariaDBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mirador_core_mariadb_connections_idle",
			Help: "Number of idle MariaDB connections",
		},
	)

	MariaDBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_mariadb_query_duration_seconds",
			Help:    "MariaDB query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"query_type"}, // select, insert, update, delete
	)

	MariaDBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_mariadb_queries_total",
			Help: "Total number of MariaDB queries",
		},
		[]string{"query_type", "result"}, // select/insert/update/delete, success/error
	)

	MariaDBPingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "mirador_core_mariadb_ping_duration_seconds",
			Help:    "MariaDB ping duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
		},
	)

	// Bootstrap metrics
	BootstrapOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_bootstrap_operations_total",
			Help: "Total number of bootstrap operations",
		},
		[]string{"operation", "result"}, // kpi_load/datasource_load, success/error
	)

	BootstrapDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_bootstrap_duration_seconds",
			Help:    "Bootstrap operation duration in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
		},
		[]string{"operation"},
	)

	BootstrapItemsLoaded = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mirador_core_bootstrap_items_loaded",
			Help: "Number of items loaded during bootstrap",
		},
		[]string{"item_type"}, // kpi, datasource
	)

	BootstrapLastSuccess = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mirador_core_bootstrap_last_success_timestamp",
			Help: "Unix timestamp of last successful bootstrap",
		},
		[]string{"operation"},
	)

	// KPI Sync Worker metrics
	KPISyncRunsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_kpi_sync_runs_total",
			Help: "Total number of KPI sync runs",
		},
		[]string{"result"}, // success, error, skipped
	)

	KPISyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "mirador_core_kpi_sync_duration_seconds",
			Help:    "KPI sync duration in seconds",
			Buckets: []float64{0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0, 120.0},
		},
	)

	KPISyncItemsProcessed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mirador_core_kpi_sync_items_processed_total",
			Help: "Total number of KPIs processed during sync",
		},
	)

	KPISyncItemsCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mirador_core_kpi_sync_items_created_total",
			Help: "Total number of KPIs created during sync",
		},
	)

	KPISyncItemsUpdated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mirador_core_kpi_sync_items_updated_total",
			Help: "Total number of KPIs updated during sync",
		},
	)

	KPISyncItemsDeleted = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mirador_core_kpi_sync_items_deleted_total",
			Help: "Total number of KPIs deleted during sync",
		},
	)

	KPISyncLastRun = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mirador_core_kpi_sync_last_run_timestamp",
			Help: "Unix timestamp of the last sync run",
		},
	)

	KPISyncLastSuccess = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mirador_core_kpi_sync_last_success_timestamp",
			Help: "Unix timestamp of the last successful sync",
		},
	)

	KPISyncErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mirador_core_kpi_sync_errors_total",
			Help: "Total number of sync errors",
		},
	)

	// Weaviate metrics
	WeaviateOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_weaviate_operations_total",
			Help: "Total number of Weaviate operations",
		},
		[]string{"operation", "result"}, // create/update/delete/query, success/error
	)

	WeaviateOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_weaviate_operation_duration_seconds",
			Help:    "Weaviate operation duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0},
		},
		[]string{"operation"},
	)
)
