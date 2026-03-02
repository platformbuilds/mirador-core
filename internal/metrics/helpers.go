// Package metrics provides helper functions for recording metrics.
package metrics

import (
	"database/sql"
	"time"
)

// Metric result constants
const (
	ResultSuccess = "success"
	ResultError   = "error"
	ResultHit     = "hit"
	ResultMiss    = "miss"
)

// RecordMariaDBStats updates MariaDB connection pool metrics from sql.DBStats.
func RecordMariaDBStats(stats sql.DBStats) {
	MariaDBConnectionsOpen.Set(float64(stats.OpenConnections))
	MariaDBConnectionsInUse.Set(float64(stats.InUse))
	MariaDBConnectionsIdle.Set(float64(stats.Idle))
}

// RecordMariaDBQuery records a MariaDB query metric.
func RecordMariaDBQuery(queryType string, duration time.Duration, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	MariaDBQueriesTotal.WithLabelValues(queryType, result).Inc()
	MariaDBQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// RecordMariaDBPing records a MariaDB ping duration.
func RecordMariaDBPing(duration time.Duration) {
	MariaDBPingDuration.Observe(duration.Seconds())
}

// RecordBootstrapOperation records a bootstrap operation metric.
func RecordBootstrapOperation(operation string, duration time.Duration, itemsLoaded int, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	BootstrapOperationsTotal.WithLabelValues(operation, result).Inc()
	BootstrapDuration.WithLabelValues(operation).Observe(duration.Seconds())
	BootstrapItemsLoaded.WithLabelValues(operation).Set(float64(itemsLoaded))
	if err == nil {
		BootstrapLastSuccess.WithLabelValues(operation).Set(float64(time.Now().Unix()))
	}
}

// RecordKPISyncRun records a KPI sync run metric.
func RecordKPISyncRun(duration time.Duration, result string) {
	KPISyncRunsTotal.WithLabelValues(result).Inc()
	KPISyncDuration.Observe(duration.Seconds())
	KPISyncLastRun.Set(float64(time.Now().Unix()))
	if result == ResultSuccess {
		KPISyncLastSuccess.Set(float64(time.Now().Unix()))
	}
	if result == ResultError {
		KPISyncErrors.Inc()
	}
}

// RecordKPISyncItems records KPI sync item counts.
func RecordKPISyncItems(processed, created, updated, deleted int) {
	KPISyncItemsProcessed.Add(float64(processed))
	KPISyncItemsCreated.Add(float64(created))
	KPISyncItemsUpdated.Add(float64(updated))
	KPISyncItemsDeleted.Add(float64(deleted))
}

// RecordWeaviateOperation records a Weaviate operation metric.
func RecordWeaviateOperation(operation string, duration time.Duration, err error) {
	result := ResultSuccess
	if err != nil {
		result = ResultError
	}
	WeaviateOperationsTotal.WithLabelValues(operation, result).Inc()
	WeaviateOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordCacheOperation records a cache operation metric.
func RecordCacheOperation(operation string, duration time.Duration, hit bool, err error) {
	result := ResultMiss
	if hit {
		result = ResultHit
	}
	if err != nil {
		result = ResultError
	}
	CacheRequestsTotal.WithLabelValues(operation, result).Inc()
	CacheRequestDuration.WithLabelValues(operation).Observe(duration.Seconds())
}
