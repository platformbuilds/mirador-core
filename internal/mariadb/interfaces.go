// Package mariadb provides MariaDB integration for mirador-core.
// This file defines interfaces for dependency injection and testing.
package mariadb

import (
	"context"
	"time"
)

// DataSourceReader provides read-only access to data sources.
// This interface enables testing without a real database connection.
type DataSourceReader interface {
	// GetByID retrieves a single data source by ID.
	GetByID(ctx context.Context, id string) (*DataSource, error)

	// ListByType retrieves all active data sources of a given type.
	ListByType(ctx context.Context, dsType DataSourceType) ([]*DataSource, error)

	// ListAll retrieves all active data sources.
	ListAll(ctx context.Context) ([]*DataSource, error)

	// GetMetricsEndpoints returns all active VictoriaMetrics/Prometheus endpoints.
	GetMetricsEndpoints(ctx context.Context) ([]string, error)

	// GetLogsEndpoints returns all active VictoriaLogs endpoints.
	GetLogsEndpoints(ctx context.Context) ([]string, error)

	// GetTracesEndpoints returns all active VictoriaTraces/Jaeger endpoints.
	GetTracesEndpoints(ctx context.Context) ([]string, error)

	// GetMetricsSourcesWithCreds returns metrics endpoints with authentication info.
	GetMetricsSourcesWithCreds(ctx context.Context) ([]DataSourceWithCredentials, error)
}

// KPIReader provides read-only access to KPIs.
// This interface enables testing without a real database connection.
type KPIReader interface {
	// GetByID retrieves a single KPI by ID.
	GetByID(ctx context.Context, id string) (*KPI, error)

	// List retrieves KPIs with optional filtering and pagination.
	// Returns the KPIs, total count, and error.
	List(ctx context.Context, opts *KPIListOptions) ([]*KPI, int64, error)

	// ListAll retrieves all KPIs (for sync purposes).
	ListAll(ctx context.Context) ([]*KPI, error)

	// ListUpdatedSince retrieves KPIs updated after a given timestamp (for incremental sync).
	ListUpdatedSince(ctx context.Context, since time.Time) ([]*KPI, error)

	// Count returns the total number of KPIs.
	Count(ctx context.Context) (int64, error)
}

// MariaDBClient provides MariaDB connection management.
// This interface enables testing connection logic without a real database.
type MariaDBClient interface {
	// IsEnabled returns true if MariaDB is enabled in configuration.
	IsEnabled() bool

	// IsConnected returns true if the client has an active connection.
	IsConnected() bool

	// Ping verifies the connection is still alive.
	Ping(ctx context.Context) error

	// Reconnect attempts to re-establish the connection.
	Reconnect() error

	// Close closes the database connection.
	Close() error

	// HealthCheck returns the current health status.
	HealthCheck(ctx context.Context) Health
}

// Compile-time interface compliance checks
var (
	_ DataSourceReader = (*DataSourceRepo)(nil)
	_ KPIReader        = (*KPIRepo)(nil)
	_ MariaDBClient    = (*Client)(nil)
)
