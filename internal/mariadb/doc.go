// Package mariadb provides MariaDB client functionality for MIRADOR-CORE,
// supporting read-only queries against the KPI database.
//
// # Overview
//
// This package implements a thread-safe MariaDB client with:
//   - Connection pooling
//   - Health checks (ping)
//   - Prepared statement caching
//   - Metrics collection
//   - Graceful shutdown
//
// # Client Interface
//
// The [Client] interface defines available operations:
//
//	type Client interface {
//	    // Ping checks database connectivity
//	    Ping(ctx context.Context) error
//
//	    // GetKPIs retrieves KPIs matching the filter criteria
//	    GetKPIs(ctx context.Context, filter KPIFilter) ([]KPI, error)
//
//	    // GetKPIByID retrieves a single KPI by ID
//	    GetKPIByID(ctx context.Context, id string) (*KPI, error)
//
//	    // Close releases database connections
//	    Close() error
//	}
//
// # Configuration
//
// Configure the client via [Config]:
//
//	cfg := mariadb.Config{
//	    Host:         "localhost",
//	    Port:         3306,
//	    User:         "mirador",
//	    Password:     "secret",
//	    Database:     "mirador",
//	    MaxOpenConns: 10,
//	    MaxIdleConns: 5,
//	    ConnMaxLife:  time.Hour,
//	}
//
//	client, err := mariadb.NewClient(cfg, logger)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
//
// # Metrics
//
// The client emits Prometheus metrics:
//   - mirador_core_mariadb_connections_open: Current open connections
//   - mirador_core_mariadb_connections_in_use: Connections in use
//   - mirador_core_mariadb_query_duration_seconds: Query latency histogram
//   - mirador_core_mariadb_ping_total: Ping operation counter
//
// # Thread Safety
//
// The client is safe for concurrent use. Connection pooling is handled
// automatically by the underlying database/sql package.
//
// # Error Handling
//
// All methods return errors that can be inspected using standard Go
// error handling. Database-specific errors are wrapped with context.
package mariadb
