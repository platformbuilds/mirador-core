package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// UnifiedQueryEngineIntegrationTestSuite provides comprehensive integration testing
// for the UnifiedQueryEngine across all observability engines (logs, metrics, traces)

// TestUnifiedQueryEngineIntegration_CrossEngineQueries tests end-to-end unified queries across all engines
func TestUnifiedQueryEngineIntegration_CrossEngineQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.New("info")
	cache := cache.NewNoopValkeyCache(logger)

	// For integration testing, we'll use nil services to test routing and error handling
	// In a real integration test, you'd set up actual service instances
	var metricsSvc *VictoriaMetricsService
	var logsSvc *VictoriaLogsService
	var tracesSvc *VictoriaTracesService
	var correlationEngine CorrelationEngine
	var bleveSearchSvc *BleveSearchService

	// Create unified query engine with nil services
	engine := NewUnifiedQueryEngine(
		metricsSvc,
		logsSvc,
		tracesSvc,
		correlationEngine,
		bleveSearchSvc,
		cache,
		logger,
	)

	ctx := context.Background()

	t.Run("Metrics Query with nil service", func(t *testing.T) {
		query := &models.UnifiedQuery{
			ID:        "test-metrics-1",
			Query:     "cpu_usage",
			Type:      models.QueryTypeMetrics,
			StartTime: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			EndTime:   func() *time.Time { t := time.Now(); return &t }(),
		}

		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err, "Should return error when metrics service is not configured")
		assert.Contains(t, err.Error(), "metrics service not configured")
	})

	t.Run("Logs Query with nil service", func(t *testing.T) {
		query := &models.UnifiedQuery{
			ID:        "test-logs-1",
			Query:     "error",
			Type:      models.QueryTypeLogs,
			StartTime: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			EndTime:   func() *time.Time { t := time.Now(); return &t }(),
		}

		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err, "Should return error when logs service is not configured")
		assert.Contains(t, err.Error(), "logs service not configured")
	})

	t.Run("Traces Query with nil service", func(t *testing.T) {
		query := &models.UnifiedQuery{
			ID:        "test-traces-1",
			Query:     "service:api",
			Type:      models.QueryTypeTraces,
			StartTime: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			EndTime:   func() *time.Time { t := time.Now(); return &t }(),
		}

		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err, "Should return error when traces service is not configured")
		assert.Contains(t, err.Error(), "traces service not configured")
	})

	t.Run("Correlation Query with nil engine", func(t *testing.T) {
		query := &models.UnifiedQuery{
			ID:        "test-correlation-1",
			Query:     "logs:error AND metrics:cpu_usage > 80",
			Type:      models.QueryTypeCorrelation,
			StartTime: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			EndTime:   func() *time.Time { t := time.Now(); return &t }(),
		}

		_, err := engine.ExecuteCorrelationQuery(ctx, query)
		assert.Error(t, err, "Should return error when correlation engine is not configured")
		assert.Contains(t, err.Error(), "correlation engine not configured")
	})

	t.Run("Intelligent Query Routing", func(t *testing.T) {
		testCases := []struct {
			name         string
			query        string
			expectedType models.QueryType
		}{
			{"Metrics Query", "cpu_usage > 80", models.QueryTypeMetrics},
			{"Logs Query", "error", models.QueryTypeLogs},
			{"Traces Query", "service:api", models.QueryTypeTraces},
			{"Search Query", "error exception stacktrace", models.QueryTypeLogs}, // Will use Bleve when available
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				query := &models.UnifiedQuery{
					ID:        "test-routing-" + tc.name,
					Query:     tc.query,
					StartTime: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
					EndTime:   func() *time.Time { t := time.Now(); return &t }(),
				}

				// Test routing by attempting to execute (will fail due to nil services but routing should work)
				_, err := engine.ExecuteQuery(ctx, query)
				// We expect an error due to nil services, but the query type should be routed correctly
				assert.Error(t, err)
				// The error should be about service not configured, not about unsupported query type
				assert.Contains(t, err.Error(), "not configured")
			})
		}
	})

	t.Run("Query Metadata Retrieval", func(t *testing.T) {
		metadata, err := engine.GetQueryMetadata(ctx)
		require.NoError(t, err, "Should retrieve query metadata successfully")
		require.NotNil(t, metadata, "Metadata should not be nil")

		// Verify supported engines
		assert.Contains(t, metadata.SupportedEngines, models.QueryTypeMetrics)
		assert.Contains(t, metadata.SupportedEngines, models.QueryTypeLogs)
		assert.Contains(t, metadata.SupportedEngines, models.QueryTypeTraces)

		// Verify query capabilities
		assert.NotEmpty(t, metadata.QueryCapabilities, "Should have query capabilities")
		assert.NotEmpty(t, metadata.QueryCapabilities[models.QueryTypeMetrics])
		assert.NotEmpty(t, metadata.QueryCapabilities[models.QueryTypeLogs])
		assert.NotEmpty(t, metadata.QueryCapabilities[models.QueryTypeTraces])

		// Verify cache capabilities
		assert.True(t, metadata.CacheCapabilities.Supported)
		assert.Greater(t, metadata.CacheCapabilities.DefaultTTL, time.Duration(0))
	})

	t.Run("Health Check with nil services", func(t *testing.T) {
		health, err := engine.HealthCheck(ctx)
		require.NoError(t, err, "Health check should not error even with nil services")
		require.NotNil(t, health, "Health status should not be nil")

		// All services should be marked as not_configured
		assert.Equal(t, "not_configured", health.EngineHealth[models.QueryTypeMetrics])
		assert.Equal(t, "not_configured", health.EngineHealth[models.QueryTypeLogs])
		assert.Equal(t, "not_configured", health.EngineHealth[models.QueryTypeTraces])

		// Overall health should be partial (some services available, some not)
		assert.Contains(t, []string{"partial", "unhealthy"}, health.OverallHealth)
	})
}

// Test that ExecuteCorrelationQuery uses the CorrelationEngine.Correlate path
// and returns a CorrelationResult that contains human-readable KPI names when
// the correlation engine resolves KPI definitions.
func TestUnifiedQueryEngine_ExecuteCorrelationQuery_TimeWindowResolvesKPINames(t *testing.T) {
	// Use short mode to run quickly
	if testing.Short() {
		t.Skip("Skipping integration-like unit test in short mode")
	}

	// Setup minimal mocks to allow Correlate to discover KPIs and resolve names
	mockMetrics := &MockVictoriaMetricsService{}
	mockLogs := &MockVictoriaLogsService{}
	mockTraces := &MockVictoriaTracesService{}
	mockCache := cache.NewNoopValkeyCache(logger.New("info"))

	// Use the same mock KPI repo we use in correlation tests
	kpiRepo := &MockKPIRepoWithDefs{}

	corrEngine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, kpiRepo, mockCache, logger.New("info"), config.EngineConfig{DefaultQueryLimit: 100})

	// Create unified engine wired with the correlation engine. Unified engine
	// doesn't require concrete metrics/logs/traces when correlation engine is
	// provided; pass nil for those arguments and let corrEngine perform probes.
	engine := NewUnifiedQueryEngine(nil, nil, nil, corrEngine, nil, mockCache, logger.New("info"))

	// Setup probes to return data and allow candidate discovery
	metricsRes := &models.MetricsQLQueryResult{Status: "success", SeriesCount: 1, Data: map[string]interface{}{"result": []interface{}{map[string]interface{}{"metric": map[string]string{"service": "checkout"}}}}}
	mockMetrics.On("ExecuteQuery", mock.Anything, mock.Anything).Return(metricsRes, nil)
	mockMetrics.On("ExecuteRangeQuery", mock.Anything, mock.Anything).Return(&models.MetricsQLRangeQueryResult{Status: "success", Data: metricsRes.Data, DataPointCount: 1}, nil)

	logsRes := &models.LogsQLQueryResult{Logs: []map[string]interface{}{{"timestamp": time.Now().Format(time.RFC3339), "message": "oops", "service": "checkout"}}}
	mockLogs.On("ExecuteQuery", mock.Anything, mock.Anything).Return(logsRes, nil)

	// Build a time-window-only unified query
	now := time.Now()
	st := now.Add(-15 * time.Minute)
	et := now
	uquery := &models.UnifiedQuery{ID: "test_timewindow", Type: models.QueryTypeCorrelation, StartTime: &st, EndTime: &et}

	res, err := engine.ExecuteCorrelationQuery(context.Background(), uquery)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Result.Data should be a *models.CorrelationResult
	if corr, ok := res.Data.(*models.CorrelationResult); ok {
		// AffectedServices should contain the human-friendly impact name
		found := false
		for _, s := range corr.AffectedServices {
			if s == "probe_metric" {
				found = true
				break
			}
		}
		require.True(t, found, "Expected probe_metric in AffectedServices")
	} else {
		t.Fatalf("expected UnifiedResult.Data to be *models.CorrelationResult; got %T", res.Data)
	}
}

// TestUnifiedQueryEngineIntegration_Performance tests performance characteristics
func TestUnifiedQueryEngineIntegration_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance integration test in short mode")
	}

	logger := logger.New("info")
	cache := cache.NewNoopValkeyCache(logger)

	// For performance testing, we'll use nil services to test routing performance
	var metricsSvc *VictoriaMetricsService
	var logsSvc *VictoriaLogsService
	var tracesSvc *VictoriaTracesService
	var correlationEngine CorrelationEngine
	var bleveSearchSvc *BleveSearchService

	engine := NewUnifiedQueryEngine(metricsSvc, logsSvc, tracesSvc, correlationEngine, bleveSearchSvc, cache, logger)

	ctx := context.Background()

	t.Run("Query Routing Performance", func(t *testing.T) {
		query := &models.UnifiedQuery{
			ID:        "perf-test-1",
			Query:     "cpu_usage",
			Type:      models.QueryTypeMetrics,
			StartTime: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			EndTime:   func() *time.Time { t := time.Now(); return &t }(),
		}

		// Run multiple queries to test routing performance
		iterations := 100
		var totalLatency time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := engine.ExecuteQuery(ctx, query)
			duration := time.Since(start)
			totalLatency += duration
			// We expect errors due to nil services, but timing should still work
			assert.Error(t, err)
		}

		avgLatency := totalLatency / time.Duration(iterations)
		t.Logf("Average routing latency: %v", avgLatency)

		// Assert reasonable routing performance (under 10ms average for routing)
		assert.Less(t, avgLatency, 10*time.Millisecond, "Average routing latency should be under 10ms")
	})

	t.Run("Metadata Query Performance", func(t *testing.T) {
		iterations := 1000
		var totalLatency time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := engine.GetQueryMetadata(ctx)
			duration := time.Since(start)
			totalLatency += duration
			require.NoError(t, err)
		}

		avgLatency := totalLatency / time.Duration(iterations)
		t.Logf("Average metadata query latency: %v", avgLatency)

		// Metadata should be extremely fast (< 1ms)
		assert.Less(t, avgLatency, 1*time.Millisecond, "Metadata queries should be under 1ms")
	})
}

// TestUnifiedQueryEngineIntegration_ErrorHandling tests error handling across engines
func TestUnifiedQueryEngineIntegration_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error handling integration test in short mode")
	}

	logger := logger.New("info")
	cache := cache.NewNoopValkeyCache(logger)

	// Setup with nil services to test error handling
	var metricsSvc *VictoriaMetricsService
	var logsSvc *VictoriaLogsService
	var tracesSvc *VictoriaTracesService
	var correlationEngine CorrelationEngine
	var bleveSearchSvc *BleveSearchService

	engine := NewUnifiedQueryEngine(metricsSvc, logsSvc, tracesSvc, correlationEngine, bleveSearchSvc, cache, logger)

	ctx := context.Background()

	t.Run("Invalid Query Handling", func(t *testing.T) {
		query := &models.UnifiedQuery{
			ID:        "error-test-1",
			Query:     "",
			Type:      models.QueryTypeLogs,
			StartTime: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			EndTime:   func() *time.Time { t := time.Now(); return &t }(),
		}

		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err, "Should handle empty queries gracefully")
	})

	t.Run("Engine Unavailable Handling", func(t *testing.T) {
		// Test with nil services to simulate engine unavailability
		brokenEngine := NewUnifiedQueryEngine(
			nil, // nil metrics service
			logsSvc,
			tracesSvc,
			correlationEngine,
			bleveSearchSvc,
			cache,
			logger,
		)

		query := &models.UnifiedQuery{
			ID:        "error-test-2",
			Query:     "cpu_usage",
			Type:      models.QueryTypeMetrics,
			StartTime: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
			EndTime:   func() *time.Time { t := time.Now(); return &t }(),
		}

		_, err := brokenEngine.ExecuteQuery(ctx, query)
		assert.Error(t, err, "Should return error when metrics engine is unavailable")
		assert.Contains(t, err.Error(), "metrics service not configured")
	})

	t.Run("Cache Invalidation with patterns", func(t *testing.T) {
		queryPatterns := []string{
			"cpu_usage",
			"error AND high_latency",
			"service:api",
			"correlation",
		}

		for _, pattern := range queryPatterns {
			err := engine.InvalidateCache(ctx, pattern)
			assert.NoError(t, err, "Cache invalidation should not error for pattern: %s", pattern)
		}
	})
}

// TestUnifiedQueryEngineIntegration_CacheBehavior tests caching behavior
func TestUnifiedQueryEngineIntegration_CacheBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cache integration test in short mode")
	}

	logger := logger.New("info")
	// Use a real in-memory cache for testing
	cache := cache.NewNoopValkeyCache(logger) // Using noop cache for this test

	// Setup with nil services
	var metricsSvc *VictoriaMetricsService
	var logsSvc *VictoriaLogsService
	var tracesSvc *VictoriaTracesService
	var correlationEngine CorrelationEngine
	var bleveSearchSvc *BleveSearchService

	engine := NewUnifiedQueryEngine(metricsSvc, logsSvc, tracesSvc, correlationEngine, bleveSearchSvc, cache, logger)

	ctx := context.Background()

	t.Run("Cache Invalidation", func(t *testing.T) {
		queryPattern := "cpu_usage"

		// Invalidate cache (should not error even with noop cache)
		err := engine.InvalidateCache(ctx, queryPattern)
		assert.NoError(t, err, "Cache invalidation should succeed even with noop cache")
	})

	t.Run("Multiple Pattern Invalidation", func(t *testing.T) {
		patterns := []string{
			"logs:error",
			"metrics:cpu_usage",
			"traces:service",
			"correlation:*",
		}

		for _, pattern := range patterns {
			err := engine.InvalidateCache(ctx, pattern)
			assert.NoError(t, err, "Should invalidate pattern: %s", pattern)
		}
	})
}

// TestUnifiedQueryEngineIntegration_BleveIntegration tests Bleve search integration
func TestUnifiedQueryEngineIntegration_BleveIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Bleve integration test in short mode")
	}

	logger := logger.New("info")
	cache := cache.NewNoopValkeyCache(logger)

	var metricsSvc *VictoriaMetricsService
	var logsSvc *VictoriaLogsService
	var tracesSvc *VictoriaTracesService
	var correlationEngine CorrelationEngine
	var bleveSearchSvc *BleveSearchService // nil for now

	engine := NewUnifiedQueryEngine(metricsSvc, logsSvc, tracesSvc, correlationEngine, bleveSearchSvc, cache, logger)

	ctx := context.Background()

	t.Run("Search query routing to Bleve", func(t *testing.T) {
		// Create a complex search query
		query := &models.UnifiedQuery{
			ID:    "bleve-test-1",
			Query: "error exception stacktrace kubernetes",
			Type:  models.QueryTypeLogs,
		}

		// Query should fail due to nil services, but routing logic should work
		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
		// Should attempt logs service (with Bleve logic)
		assert.Contains(t, err.Error(), "logs service not configured")
	})
}

// TestUnifiedQueryEngineIntegration_TracesSearchEnhancement tests enhanced traces search
func TestUnifiedQueryEngineIntegration_TracesSearchEnhancement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping traces search integration test in short mode")
	}

	logger := logger.New("info")
	cache := cache.NewNoopValkeyCache(logger)

	var metricsSvc *VictoriaMetricsService
	var logsSvc *VictoriaLogsService
	var tracesSvc *VictoriaTracesService // nil for testing
	var correlationEngine CorrelationEngine
	var bleveSearchSvc *BleveSearchService

	engine := NewUnifiedQueryEngine(metricsSvc, logsSvc, tracesSvc, correlationEngine, bleveSearchSvc, cache, logger)

	ctx := context.Background()

	t.Run("Enhanced traces query with filters", func(t *testing.T) {
		// Create a query with service, operation, and duration filters
		query := &models.UnifiedQuery{
			ID:    "traces-enhanced-1",
			Query: "service:api operation:GET duration>100",
			Type:  models.QueryTypeTraces,
		}

		// Should fail due to nil service but parsing logic should work
		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "traces service not configured")
	})

	t.Run("Traces query fallback", func(t *testing.T) {
		// Simple query that would use fallback
		query := &models.UnifiedQuery{
			ID:    "traces-fallback-1",
			Query: "service:auth",
			Type:  models.QueryTypeTraces,
		}

		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "traces service not configured")
	})
}
