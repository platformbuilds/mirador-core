// Package sync provides KPI synchronization between MariaDB and Weaviate
// for MIRADOR-CORE.
//
// # Overview
//
// This package implements a unidirectional sync mechanism that:
//   - Reads KPIs from MariaDB (source of truth)
//   - Upserts KPIs into Weaviate (vector store for semantic search)
//   - Supports both manual and scheduled synchronization
//   - Provides idempotent operations
//
// # Syncer Interface
//
// The [Syncer] interface defines sync operations:
//
//	type Syncer interface {
//	    // SyncAll performs a full synchronization of all KPIs
//	    SyncAll(ctx context.Context) error
//
//	    // SyncSince synchronizes KPIs modified since the given time
//	    SyncSince(ctx context.Context, since time.Time) error
//
//	    // Start begins scheduled synchronization
//	    Start(ctx context.Context) error
//
//	    // Stop halts scheduled synchronization
//	    Stop() error
//	}
//
// # Usage
//
// Create and start a syncer:
//
//	syncer := sync.NewSyncer(sync.Config{
//	    Interval:  5 * time.Minute,
//	    BatchSize: 100,
//	}, mariaClient, weaviateClient, logger)
//
//	// Start scheduled sync
//	if err := syncer.Start(ctx); err != nil {
//	    return err
//	}
//	defer syncer.Stop()
//
//	// Or perform manual sync
//	if err := syncer.SyncAll(ctx); err != nil {
//	    return err
//	}
//
// # Synchronization Strategy
//
// The syncer uses the following strategy:
//  1. Query KPIs from MariaDB (with optional time filter)
//  2. Batch KPIs for efficient processing
//  3. Upsert each batch into Weaviate
//  4. Track sync timestamps for incremental updates
//
// # Metrics
//
// The syncer emits Prometheus metrics:
//   - mirador_core_kpi_sync_runs_total: Sync operation counter
//   - mirador_core_kpi_sync_items_total: Items synced counter
//   - mirador_core_kpi_sync_errors_total: Sync error counter
//   - mirador_core_kpi_sync_last_timestamp: Last successful sync time
//
// # Error Handling
//
// Sync errors are logged and the syncer continues with the next batch.
// Persistent errors trigger alerts via the metrics.
//
// # Thread Safety
//
// The syncer is safe for concurrent use. Only one sync operation runs
// at a time; concurrent calls wait for the current operation to complete.
package sync
