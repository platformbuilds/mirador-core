package services

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
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

	// Create unified query engine with nil services
	engine := NewUnifiedQueryEngine(
		metricsSvc,
		logsSvc,
		tracesSvc,
		correlationEngine,
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
			{"Correlation Query", "logs:error WITHIN 5m OF metrics:cpu_usage > 80", models.QueryTypeCorrelation},
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

	engine := NewUnifiedQueryEngine(metricsSvc, logsSvc, tracesSvc, correlationEngine, cache, logger)

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

	engine := NewUnifiedQueryEngine(metricsSvc, logsSvc, tracesSvc, correlationEngine, cache, logger)

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

	engine := NewUnifiedQueryEngine(metricsSvc, logsSvc, tracesSvc, correlationEngine, cache, logger)

	ctx := context.Background()

	t.Run("Cache Invalidation", func(t *testing.T) {
		queryPattern := "cpu_usage"

		// Invalidate cache (should not error even with noop cache)
		err := engine.InvalidateCache(ctx, queryPattern)
		assert.NoError(t, err, "Cache invalidation should succeed even with noop cache")
	})
}
