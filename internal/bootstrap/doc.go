// Package bootstrap provides startup data initialization for MIRADOR-CORE.
//
// # Overview
//
// This package handles loading initial data from persistent stores (MariaDB)
// into fast-access stores (Weaviate) during application startup. It ensures
// the vector store has up-to-date KPI data for semantic search operations.
//
// # Bootstrapper Interface
//
// The [Bootstrapper] interface defines bootstrap operations:
//
//	type Bootstrapper interface {
//	    // Bootstrap performs full data initialization
//	    Bootstrap(ctx context.Context) error
//
//	    // BootstrapKPIs synchronizes KPI data from MariaDB to Weaviate
//	    BootstrapKPIs(ctx context.Context) error
//	}
//
// # Usage
//
// The bootstrapper is typically invoked during server startup:
//
//	bootstrapper := bootstrap.New(bootstrap.Config{
//	    BatchSize:   100,
//	    Concurrency: 4,
//	}, mariaClient, weaviateClient, logger)
//
//	if err := bootstrapper.Bootstrap(ctx); err != nil {
//	    logger.Error("bootstrap failed", zap.Error(err))
//	    // Decide whether to continue or fail startup
//	}
//
// # Bootstrap Process
//
// The bootstrap process:
//  1. Connects to MariaDB and retrieves all KPIs
//  2. Batches KPIs for efficient processing
//  3. Upserts batches into Weaviate concurrently
//  4. Reports progress via metrics
//
// # Idempotency
//
// Bootstrap operations are idempotent - running bootstrap multiple times
// produces the same result. Existing KPIs in Weaviate are updated with
// the latest data from MariaDB.
//
// # Metrics
//
// Bootstrap emits the following metrics:
//   - mirador_core_bootstrap_operations_total: Counter by type and result
//   - mirador_core_bootstrap_duration_seconds: Duration histogram
//   - mirador_core_bootstrap_items_total: Items processed counter
//
// # Error Handling
//
// Bootstrap errors are logged but may not be fatal depending on
// configuration. The application can operate with cached data while
// bootstrap completes in the background.
//
// # Configuration
//
// Bootstrap is configured via the application config:
//
//	bootstrap:
//	  enabled: true
//	  batchSize: 100
//	  concurrency: 4
//	  timeout: 5m
package bootstrap
