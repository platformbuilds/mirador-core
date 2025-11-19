package services

import (
	"context"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MetricsMetadataSynchronizer manages periodic synchronization of metrics metadata
type MetricsMetadataSynchronizer interface {
	// Start begins the synchronization process
	Start(ctx context.Context) error

	// Stop stops the synchronization process
	Stop() error

	// SyncNow triggers an immediate sync
	SyncNow(ctx context.Context, forceFull bool) (*models.MetricMetadataSyncResult, error)

	// GetSyncState returns the current sync state
	GetSyncState() (*models.MetricMetadataSyncState, error)

	// GetSyncStatus returns the status of the current/last sync operation
	GetSyncStatus() (*models.MetricMetadataSyncStatus, error)

	// UpdateConfig updates the synchronization configuration
	UpdateConfig(config *models.MetricMetadataSyncConfig) error
}

// MetricsMetadataSynchronizerImpl implements the MetricsMetadataSynchronizer interface
type MetricsMetadataSynchronizerImpl struct {
	indexer MetricsMetadataIndexer
	cache   cache.ValkeyCluster
	config  *models.MetricMetadataSyncConfig
	logger  logger.Logger

	// State management
	syncState  *models.MetricMetadataSyncState
	syncStatus *models.MetricMetadataSyncStatus
	stateMutex sync.RWMutex

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}
	ticker *time.Ticker
	wg     sync.WaitGroup
}

// NewMetricsMetadataSynchronizer creates a new metrics metadata synchronizer
func NewMetricsMetadataSynchronizer(
	indexer MetricsMetadataIndexer,
	cache cache.ValkeyCluster,
	config *models.MetricMetadataSyncConfig,
	logger logger.Logger,
) MetricsMetadataSynchronizer {
	return &MetricsMetadataSynchronizerImpl{
		indexer:    indexer,
		cache:      cache,
		config:     config,
		logger:     logger,
		syncState:  &models.MetricMetadataSyncState{},
		syncStatus: &models.MetricMetadataSyncStatus{Status: "never_run"},
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// Start begins the synchronization process
func (s *MetricsMetadataSynchronizerImpl) Start(ctx context.Context) error {
	if !s.config.Enabled {
		s.logger.Info("Metrics metadata synchronizer is disabled")
		return nil
	}

	s.logger.Info("Starting metrics metadata synchronizer",
		"strategy", s.config.Strategy,
		"interval", s.config.Interval,
		"fullSyncInterval", s.config.FullSyncInterval)

	// Load existing sync states from cache
	if err := s.loadSyncStates(ctx); err != nil {
		s.logger.Warn("Failed to load sync states from cache", "error", err)
	}

	// Start the sync loop
	s.wg.Add(1)
	go s.syncLoop(ctx)

	s.logger.Info("Metrics metadata synchronizer started successfully")
	return nil
}

// Stop stops the synchronization process
func (s *MetricsMetadataSynchronizerImpl) Stop() error {
	s.logger.Info("Stopping metrics metadata synchronizer")

	close(s.stopCh)
	s.wg.Wait()

	// Save sync states to cache
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.saveSyncStates(ctx); err != nil {
		s.logger.Warn("Failed to save sync states to cache", "error", err)
	}

	s.logger.Info("Metrics metadata synchronizer stopped")
	return nil
}

// syncLoop runs the periodic synchronization
func (s *MetricsMetadataSynchronizerImpl) syncLoop(ctx context.Context) {
	defer s.wg.Done()
	defer close(s.doneCh)

	s.ticker = time.NewTicker(s.config.Interval)
	defer s.ticker.Stop()

	// Do initial sync
	if err := s.sync(ctx, false); err != nil {
		s.logger.Error("Initial sync failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-s.ticker.C:
			if err := s.sync(ctx, false); err != nil {
				s.logger.Error("Periodic sync failed", "error", err)
			}
		}
	}
}

// sync performs the synchronization
func (s *MetricsMetadataSynchronizerImpl) sync(ctx context.Context, forceFull bool) error {
	s.stateMutex.Lock()
	state := s.syncState
	s.stateMutex.Unlock()

	// Check if sync is already running
	if state.IsCurrentlySyncing {
		s.logger.Debug("Sync already running")
		return nil
	}

	// Determine sync strategy
	strategy := s.determineSyncStrategy(state, forceFull)

	// Mark as syncing
	state.IsCurrentlySyncing = true
	defer func() {
		state.IsCurrentlySyncing = false
	}()

	// Create sync status
	status := &models.MetricMetadataSyncStatus{
		Status:    "running",
		StartTime: time.Now(),
		Strategy:  strategy,
	}

	s.stateMutex.Lock()
	s.syncStatus = status
	s.stateMutex.Unlock()

	// Perform the sync
	result, err := s.performSync(ctx, strategy)

	// Update status
	status.EndTime = time.Now()
	status.Duration = status.EndTime.Sub(status.StartTime)

	if err != nil {
		status.Status = "failed"
		status.Errors = []string{err.Error()}

		// Update state
		state.FailedSyncs++
		state.LastError = err.Error()
		state.LastErrorTime = time.Now()

		s.logger.Error("Sync failed", "error", err)
	} else {
		status.Status = "completed"
		status.MetricsProcessed = result.MetricsProcessed
		status.MetricsAdded = result.MetricsAdded
		status.MetricsUpdated = result.MetricsUpdated
		status.MetricsRemoved = result.MetricsRemoved

		// Update state
		state.LastSyncTime = result.LastSyncTime
		state.TotalSyncs++
		state.SuccessfulSyncs++
		state.MetricsInIndex = int64(result.MetricsProcessed)

		if strategy == models.SyncStrategyFull {
			state.LastFullSyncTime = result.LastSyncTime
		}

		s.logger.Info("Sync completed",
			"strategy", strategy,
			"metricsProcessed", result.MetricsProcessed,
			"duration", result.Duration)
	}

	return err
}

// determineSyncStrategy determines which sync strategy to use
func (s *MetricsMetadataSynchronizerImpl) determineSyncStrategy(state *models.MetricMetadataSyncState, forceFull bool) models.SyncStrategy {
	if forceFull {
		return models.SyncStrategyFull
	}

	switch s.config.Strategy {
	case models.SyncStrategyFull:
		return models.SyncStrategyFull
	case models.SyncStrategyIncremental:
		return models.SyncStrategyIncremental
	case models.SyncStrategyHybrid:
		// Do full sync if it's been too long since the last full sync
		if time.Since(state.LastFullSyncTime) > s.config.FullSyncInterval {
			return models.SyncStrategyFull
		}
		return models.SyncStrategyIncremental
	default:
		return models.SyncStrategyIncremental
	}
}

// performSync executes the actual sync operation
func (s *MetricsMetadataSynchronizerImpl) performSync(ctx context.Context, strategy models.SyncStrategy) (*models.MetricMetadataSyncResult, error) {
	// Create sync request
	request := &models.MetricMetadataSyncRequest{
		ForceFullSync: strategy == models.SyncStrategyFull,
		BatchSize:     s.config.BatchSize,
	}

	// Set time range based on strategy
	if strategy == models.SyncStrategyIncremental {
		// For incremental sync, only look at recent data
		request.TimeRange = &models.TimeRange{
			Start: time.Now().Add(-s.config.TimeRangeLookback),
			End:   time.Now(),
		}
	} else {
		// For full sync, look at a broader time range
		request.TimeRange = &models.TimeRange{
			Start: time.Now().Add(-24 * time.Hour), // Last 24 hours for full sync
			End:   time.Now(),
		}
	}

	// Execute sync with retry logic
	var result *models.MetricMetadataSyncResult
	var err error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		result, err = s.indexer.SyncMetadata(ctx, request)
		if err == nil {
			break
		}

		if attempt < s.config.MaxRetries {
			s.logger.Warn("Sync attempt failed, retrying",
				"attempt", attempt+1,
				"maxRetries", s.config.MaxRetries,
				"error", err)
			time.Sleep(s.config.RetryDelay)
		}
	}

	return result, err
}

// SyncNow triggers an immediate sync
func (s *MetricsMetadataSynchronizerImpl) SyncNow(ctx context.Context, forceFull bool) (*models.MetricMetadataSyncResult, error) {
	return s.performSync(ctx, s.determineSyncStrategy(s.getSyncState(), forceFull))
}

// GetSyncState returns the current sync state
func (s *MetricsMetadataSynchronizerImpl) GetSyncState() (*models.MetricMetadataSyncState, error) {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	state := s.syncState
	if state == nil {
		return &models.MetricMetadataSyncState{}, nil
	}

	// Return a copy to avoid external modifications
	stateCopy := *state
	return &stateCopy, nil
}

// GetSyncStatus returns the status of the current/last sync operation
func (s *MetricsMetadataSynchronizerImpl) GetSyncStatus() (*models.MetricMetadataSyncStatus, error) {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	status := s.syncStatus
	if status == nil {
		return &models.MetricMetadataSyncStatus{Status: "never_run"}, nil
	}

	// Return a copy to avoid external modifications
	statusCopy := *status
	return &statusCopy, nil
}

// UpdateConfig updates the synchronization configuration
func (s *MetricsMetadataSynchronizerImpl) UpdateConfig(config *models.MetricMetadataSyncConfig) error {
	s.config = config

	// If ticker exists, update its interval
	if s.ticker != nil {
		s.ticker.Reset(s.config.Interval)
	}

	s.logger.Info("Metrics metadata synchronizer config updated",
		"enabled", config.Enabled,
		"strategy", config.Strategy,
		"interval", config.Interval)

	return nil
}

// getSyncState returns the sync state (internal method)
func (s *MetricsMetadataSynchronizerImpl) getSyncState() *models.MetricMetadataSyncState {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	state := s.syncState
	if state == nil {
		state = &models.MetricMetadataSyncState{}
		s.syncState = state
	}
	return state
}

// loadSyncStates loads sync states from cache
func (s *MetricsMetadataSynchronizerImpl) loadSyncStates(ctx context.Context) error {
	// TODO: Implement loading sync states from cache
	// For now, this is a placeholder
	return nil
}

// saveSyncStates saves sync states to cache
func (s *MetricsMetadataSynchronizerImpl) saveSyncStates(ctx context.Context) error {
	// TODO: Implement saving sync states to cache
	// For now, this is a placeholder
	return nil
}

// NewStubMetricsMetadataSynchronizer creates a stub implementation for development/testing
func NewStubMetricsMetadataSynchronizer(logger logger.Logger) MetricsMetadataSynchronizer {
	return &StubMetricsMetadataSynchronizer{
		logger: logger,
	}
}

// StubMetricsMetadataSynchronizer provides stub implementations for metrics metadata synchronization
type StubMetricsMetadataSynchronizer struct {
	logger logger.Logger
}

// Start returns nil (no-op)
func (s *StubMetricsMetadataSynchronizer) Start(ctx context.Context) error {
	s.logger.Info("StubMetricsMetadataSynchronizer.Start called")
	return nil
}

// Stop returns nil (no-op)
func (s *StubMetricsMetadataSynchronizer) Stop() error {
	s.logger.Info("StubMetricsMetadataSynchronizer.Stop called")
	return nil
}

// SyncNow returns a stub sync result
func (s *StubMetricsMetadataSynchronizer) SyncNow(ctx context.Context, forceFull bool) (*models.MetricMetadataSyncResult, error) {
	s.logger.Info("StubMetricsMetadataSynchronizer.SyncNow called", "force_full", forceFull)

	return &models.MetricMetadataSyncResult{
		MetricsProcessed: 0,
		MetricsAdded:     0,
		MetricsUpdated:   0,
		MetricsRemoved:   0,
		Duration:         0,
		LastSyncTime:     time.Now(),
		Errors:           []string{"Metrics metadata synchronization is disabled in this environment"},
	}, nil
}

// GetSyncState returns a stub sync state
func (s *StubMetricsMetadataSynchronizer) GetSyncState() (*models.MetricMetadataSyncState, error) {
	s.logger.Info("StubMetricsMetadataSynchronizer.GetSyncState called")

	return &models.MetricMetadataSyncState{
		LastSyncTime:       time.Now(),
		LastFullSyncTime:   time.Now(),
		TotalSyncs:         0,
		SuccessfulSyncs:    0,
		FailedSyncs:        0,
		MetricsInIndex:     0,
		IsCurrentlySyncing: false,
	}, nil
}

// GetSyncStatus returns a stub sync status
func (s *StubMetricsMetadataSynchronizer) GetSyncStatus() (*models.MetricMetadataSyncStatus, error) {
	s.logger.Info("StubMetricsMetadataSynchronizer.GetSyncStatus called")

	return &models.MetricMetadataSyncStatus{
		Status:           "disabled",
		StartTime:        time.Now(),
		EndTime:          time.Now(),
		Strategy:         models.SyncStrategyFull,
		MetricsProcessed: 0,
		MetricsAdded:     0,
		MetricsUpdated:   0,
		MetricsRemoved:   0,
		Errors:           []string{"Metrics metadata synchronization is disabled in this environment"},
		Duration:         0,
	}, nil
}

// UpdateConfig returns nil (no-op)
func (s *StubMetricsMetadataSynchronizer) UpdateConfig(config *models.MetricMetadataSyncConfig) error {
	s.logger.Info("StubMetricsMetadataSynchronizer.UpdateConfig called")
	return nil
}
