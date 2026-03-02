// Package metrics provides Prometheus metrics definitions and helpers
// for MIRADOR-CORE self-monitoring.
//
// # Overview
//
// This package centralizes all Prometheus metric definitions and provides
// helper functions for recording metrics consistently across the codebase.
//
// # Metric Categories
//
// Metrics are organized by component:
//
// HTTP Metrics:
//   - [HTTPRequestsTotal]: Counter of HTTP requests by method, endpoint, status
//   - [HTTPRequestDuration]: Histogram of request latency
//
// gRPC Metrics:
//   - [GRPCRequestsTotal]: Counter of gRPC calls by service, method, status
//   - [GRPCRequestDuration]: Histogram of gRPC call latency
//
// Cache Metrics:
//   - [CacheRequestsTotal]: Counter by operation (get/set) and result (hit/miss)
//   - [CacheRequestDuration]: Histogram of cache operation latency
//
// MariaDB Metrics:
//   - [MariaDBConnectionsOpen]: Gauge of current open connections
//   - [MariaDBConnectionsInUse]: Gauge of connections in use
//   - [MariaDBQueryDuration]: Histogram of query latency
//   - [MariaDBPingTotal]: Counter of ping operations
//
// Bootstrap Metrics:
//   - [BootstrapOperationsTotal]: Counter of bootstrap operations
//   - [BootstrapDuration]: Histogram of bootstrap latency
//   - [BootstrapItemsTotal]: Counter of items bootstrapped
//
// KPI Sync Metrics:
//   - [KPISyncRunsTotal]: Counter of sync operations
//   - [KPISyncItemsTotal]: Counter of items synced
//   - [KPISyncLastTimestamp]: Gauge of last sync time
//   - [KPISyncErrorsTotal]: Counter of sync errors
//
// # Helper Functions
//
// Use helper functions for consistent metric recording:
//
//	// Record MariaDB connection stats
//	metrics.RecordMariaDBStats(openConns, inUseConns)
//
//	// Record a query duration
//	metrics.RecordMariaDBQuery("select", duration)
//
//	// Record a cache operation
//	metrics.RecordCacheOperation("get", metrics.ResultHit, duration)
//
//	// Record a bootstrap operation
//	metrics.RecordBootstrapOperation("kpi", duration, itemCount, nil)
//
// # Constants
//
// Use predefined constants for consistent labeling:
//
//	const (
//	    ResultSuccess = "success"
//	    ResultError   = "error"
//	    ResultHit     = "hit"
//	    ResultMiss    = "miss"
//	)
//
// # Registration
//
// Metrics are automatically registered via [promauto]. The /metrics endpoint
// exposes all metrics in Prometheus format.
//
// # Alerting
//
// See docs/monitoring-observability.md for recommended alerting rules based
// on these metrics.
package metrics
