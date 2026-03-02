// Package sync provides background synchronization between MariaDB and Weaviate.
package sync

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/mirastacklabs-ai/mirador-core/internal/config"
	"github.com/mirastacklabs-ai/mirador-core/internal/mariadb"
	"github.com/mirastacklabs-ai/mirador-core/internal/weavstore"
)

// KPISyncWorker synchronizes KPIs from MariaDB to Weaviate.
// It runs periodically in the background to keep Weaviate's semantic
// index in sync with the source of truth in MariaDB.
type KPISyncWorker struct {
	mariaDBRepo   *mariadb.KPIRepo
	weaviateStore weavstore.KPIStore
	logger        *zap.Logger
	cfg           config.MariaDBSyncConfig

	mu            sync.RWMutex
	lastSyncTime  time.Time
	lastSyncCount int
	lastSyncError error
	running       bool
	stopCh        chan struct{}
}

// NewKPISyncWorker creates a new KPI sync worker.
func NewKPISyncWorker(
	mariaDBRepo *mariadb.KPIRepo,
	weaviateStore weavstore.KPIStore,
	cfg config.MariaDBSyncConfig,
	logger *zap.Logger,
) *KPISyncWorker {
	return &KPISyncWorker{
		mariaDBRepo:   mariaDBRepo,
		weaviateStore: weaviateStore,
		cfg:           cfg,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background sync loop.
func (w *KPISyncWorker) Start(ctx context.Context) {
	if !w.cfg.Enabled {
		if w.logger != nil {
			w.logger.Info("kpi-sync: worker disabled by configuration")
		}
		return
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	if w.logger != nil {
		w.logger.Info("kpi-sync: starting background sync worker",
			zap.Duration("interval", w.cfg.Interval),
			zap.Int("batch_size", w.cfg.BatchSize),
		)
	}

	// Run initial sync immediately
	w.runSync(ctx)

	// Start ticker for periodic sync
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("kpi-sync: stopping due to context cancellation")
			return
		case <-w.stopCh:
			w.logger.Info("kpi-sync: stopping due to stop signal")
			return
		case <-ticker.C:
			w.runSync(ctx)
		}
	}
}

// Stop signals the worker to stop.
func (w *KPISyncWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		close(w.stopCh)
		w.running = false
	}
}

// runSync performs a single sync cycle.
func (w *KPISyncWorker) runSync(ctx context.Context) {
	startTime := time.Now()

	w.mu.RLock()
	lastSync := w.lastSyncTime
	w.mu.RUnlock()

	var kpis []*mariadb.KPI
	var err error

	// Use incremental sync if we have a last sync time
	if !lastSync.IsZero() {
		kpis, err = w.mariaDBRepo.ListUpdatedSince(ctx, lastSync)
		if err != nil {
			w.recordError(err)
			if w.logger != nil {
				w.logger.Error("kpi-sync: failed to fetch updated KPIs from MariaDB",
					zap.Error(err),
					zap.Time("since", lastSync),
				)
			}
			return
		}
	} else {
		// Full sync on first run
		kpis, err = w.mariaDBRepo.ListAll(ctx)
		if err != nil {
			w.recordError(err)
			if w.logger != nil {
				w.logger.Error("kpi-sync: failed to fetch all KPIs from MariaDB",
					zap.Error(err),
				)
			}
			return
		}
	}

	if len(kpis) == 0 {
		if w.logger != nil {
			w.logger.Debug("kpi-sync: no KPIs to sync")
		}
		w.recordSuccess(0)
		return
	}

	// Sync to Weaviate in batches
	synced := 0
	failed := 0

	for _, kpi := range kpis {
		weavKPI := convertMariaDBToWeaviate(kpi)

		_, status, err := w.weaviateStore.CreateOrUpdateKPI(ctx, weavKPI)
		if err != nil {
			if w.logger != nil {
				w.logger.Warn("kpi-sync: failed to sync KPI to Weaviate",
					zap.String("kpi_id", kpi.ID),
					zap.String("kpi_name", kpi.Name),
					zap.Error(err),
				)
			}
			failed++
			continue
		}

		synced++
		if w.logger != nil {
			w.logger.Debug("kpi-sync: synced KPI",
				zap.String("kpi_id", kpi.ID),
				zap.String("status", status),
			)
		}
	}

	duration := time.Since(startTime)

	if w.logger != nil {
		w.logger.Info("kpi-sync: sync cycle completed",
			zap.Int("total", len(kpis)),
			zap.Int("synced", synced),
			zap.Int("failed", failed),
			zap.Duration("duration", duration),
		)
	}

	w.recordSuccess(synced)
}

// recordSuccess records a successful sync.
func (w *KPISyncWorker) recordSuccess(count int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastSyncTime = time.Now()
	w.lastSyncCount = count
	w.lastSyncError = nil
}

// recordError records a sync error.
func (w *KPISyncWorker) recordError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastSyncError = err
}

// Status represents the current sync status.
type Status struct {
	Enabled       bool      `json:"enabled"`
	Running       bool      `json:"running"`
	LastSyncTime  time.Time `json:"last_sync_time,omitempty"`
	LastSyncCount int       `json:"last_sync_count"`
	LastError     string    `json:"last_error,omitempty"`
	Interval      string    `json:"interval"`
}

// GetStatus returns the current sync status.
func (w *KPISyncWorker) GetStatus() Status {
	w.mu.RLock()
	defer w.mu.RUnlock()

	s := Status{
		Enabled:       w.cfg.Enabled,
		Running:       w.running,
		LastSyncTime:  w.lastSyncTime,
		LastSyncCount: w.lastSyncCount,
		Interval:      w.cfg.Interval.String(),
	}

	if w.lastSyncError != nil {
		s.LastError = w.lastSyncError.Error()
	}

	return s
}

// TriggerSync manually triggers an immediate sync.
func (w *KPISyncWorker) TriggerSync(ctx context.Context) error {
	if !w.cfg.Enabled {
		return mariadb.ErrMariaDBDisabled
	}

	go w.runSync(ctx)
	return nil
}

// convertMariaDBToWeaviate converts a MariaDB KPI to Weaviate KPIDefinition.
func convertMariaDBToWeaviate(k *mariadb.KPI) *weavstore.KPIDefinition {
	wk := &weavstore.KPIDefinition{
		ID:              k.ID,
		Name:            k.Name,
		Description:     k.Description,
		Definition:      k.Definition,
		Formula:         k.Formula,
		DataSourceID:    k.DataSourceID,
		KPIDatastoreID:  k.KPIDatastoreID,
		Unit:            k.Unit,
		RefreshInterval: k.RefreshInterval,
		IsShared:        k.IsShared,
		UserID:          k.UserID,
		Namespace:       k.Namespace,
		Kind:            k.Kind,
		Layer:           k.Layer,
		Classifier:      k.Classifier,
		SignalType:      k.SignalType,
		Sentiment:       k.Sentiment,
		ComponentType:   k.ComponentType,
		Examples:        k.Examples,
		QueryType:       k.QueryType,
		Datastore:       k.Datastore,
		ServiceFamily:   k.ServiceFamily,
		DataType:        string(k.DataType),
		CreatedAt:       k.CreatedAt,
		UpdatedAt:       k.UpdatedAt,
		// Source is set to "mariadb" to indicate origin
		Source: "mariadb",
	}

	// Parse Query JSON
	if len(k.Query) > 0 {
		var query map[string]any
		if err := json.Unmarshal(k.Query, &query); err == nil {
			wk.Query = query
		}
	}

	// Parse Thresholds JSON
	if len(k.Thresholds) > 0 {
		var thresholds []weavstore.Threshold
		if err := json.Unmarshal(k.Thresholds, &thresholds); err == nil {
			wk.Thresholds = thresholds
		}
	}

	return wk
}
