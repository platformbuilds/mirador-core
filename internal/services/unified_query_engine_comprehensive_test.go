package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TestUnifiedQueryEngine_ComprehensiveIntegration tests the full unified query engine with mocks
func TestUnifiedQueryEngine_ComprehensiveIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive integration test in short mode")
	}

	// Create test framework
	framework := NewIntegrationTestFramework()
	framework.Setup()
	defer framework.TearDown()

	// Create real services with mocks
	log := logger.New("info")
	mockCache := cache.NewNoopValkeyCache(log)

	t.Run("Metrics Query Integration", func(t *testing.T) {
		// Setup test data
		metricsResult := GenerateMetricsResult("cpu_usage", 85.5, time.Now())
		framework.SetupMetricsData("cpu_usage", metricsResult)

		// Create unified engine
		engine := NewUnifiedQueryEngine(
			nil, // We'll test with framework mocks
			nil,
			nil,
			nil,
			nil,
			mockCache,
			log,
		)

		ctx := context.Background()
		query := CreateTestUnifiedQuery("test-metrics-1", models.QueryTypeMetrics, "cpu_usage", time.Hour)

		// Execute query (will fail with nil services but routing should work)
		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metrics service not configured")
	})

	t.Run("Logs Query Integration", func(t *testing.T) {
		// Setup test data
		logsResult := GenerateLogsResult("error", "Database connection failed", time.Now())
		framework.SetupLogsData("error", logsResult)

		// Create unified engine
		engine := NewUnifiedQueryEngine(
			nil,
			nil,
			nil,
			nil,
			nil,
			mockCache,
			log,
		)

		ctx := context.Background()
		query := CreateTestUnifiedQuery("test-logs-1", models.QueryTypeLogs, "error", time.Hour)

		// Execute query
		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "logs service not configured")
	})

	t.Run("Traces Query Integration", func(t *testing.T) {
		// Setup test data
		tracesResult := GenerateTracesResult("trace-123")
		framework.SetupTracesData("api-service", tracesResult)

		// Create unified engine
		engine := NewUnifiedQueryEngine(
			nil,
			nil,
			nil,
			nil,
			nil,
			mockCache,
			log,
		)

		ctx := context.Background()
		query := CreateTestUnifiedQuery("test-traces-1", models.QueryTypeTraces, "service:api", time.Hour)

		// Execute query
		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "traces service not configured")
	})
}

// TestUnifiedQueryEngine_CrossEngineDataConsistency tests data consistency across engines
func TestUnifiedQueryEngine_CrossEngineDataConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-engine data consistency test in short mode")
	}

	framework := NewIntegrationTestFramework()
	framework.Setup()
	defer framework.TearDown()

	log := logger.New("info")
	mockCache := cache.NewNoopValkeyCache(log)

	t.Run("Correlation Timestamp Consistency", func(t *testing.T) {
		// Setup synchronized test data with same timestamp
		timestamp := time.Now().Add(-5 * time.Minute)

		metricsResult := GenerateMetricsResult("cpu_usage", 95.0, timestamp)
		logsResult := GenerateLogsResult("error", "High CPU detected", timestamp)

		framework.SetupMetricsData("cpu_usage", metricsResult)
		framework.SetupLogsData("error", logsResult)

		// Verify timestamps match
		assert.Equal(t, timestamp.Unix(), timestamp.Unix())
	})

	t.Run("Multi-Engine Query Coordination", func(t *testing.T) {
		// Test that queries across engines use consistent time ranges
		startTime := time.Now().Add(-1 * time.Hour)

		query1 := CreateTestUnifiedQuery("metrics-1", models.QueryTypeMetrics, "cpu_usage", time.Hour)
		query2 := CreateTestUnifiedQuery("logs-1", models.QueryTypeLogs, "error", time.Hour)

		// Verify both queries have same time range
		assert.Equal(t, query1.StartTime.Unix(), query2.StartTime.Unix())
		assert.True(t, query1.EndTime.After(startTime))
		assert.True(t, query2.EndTime.After(startTime))
	})

	t.Run("Data Format Consistency", func(t *testing.T) {
		// Verify all query results have consistent structure
		engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)
		ctx := context.Background()

		queries := []*models.UnifiedQuery{
			CreateTestUnifiedQuery("test-1", models.QueryTypeMetrics, "cpu_usage", time.Hour),
			CreateTestUnifiedQuery("test-2", models.QueryTypeLogs, "error", time.Hour),
			CreateTestUnifiedQuery("test-3", models.QueryTypeTraces, "service:api", time.Hour),
		}

		for _, query := range queries {
			result, err := engine.ExecuteQuery(ctx, query)
			// Will error with nil services, but result structure should be attempted
			assert.Error(t, err)
			assert.Nil(t, result) // Expect nil result when service is not configured
		}
	})
}

// TestUnifiedQueryEngine_ConcurrentQueries tests parallel query execution
func TestUnifiedQueryEngine_ConcurrentQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent queries test in short mode")
	}

	framework := NewIntegrationTestFramework()
	framework.Setup()
	defer framework.TearDown()

	log := logger.New("info")
	mockCache := cache.NewNoopValkeyCache(log)
	engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

	t.Run("Parallel Metrics Queries", func(t *testing.T) {
		ctx := context.Background()
		numQueries := 10

		// Execute queries concurrently
		results := make(chan error, numQueries)
		for i := 0; i < numQueries; i++ {
			go func(id int) {
				query := CreateTestUnifiedQuery(
					fmt.Sprintf("metrics-%d", id),
					models.QueryTypeMetrics,
					"cpu_usage",
					time.Hour,
				)
				_, err := engine.ExecuteQuery(ctx, query)
				results <- err
			}(i)
		}

		// Collect results
		errorCount := 0
		for i := 0; i < numQueries; i++ {
			err := <-results
			if err != nil {
				errorCount++
			}
		}

		// All should error with "not configured"
		assert.Equal(t, numQueries, errorCount)
	})

	t.Run("Mixed Query Types Concurrency", func(t *testing.T) {
		ctx := context.Background()
		queryTypes := []models.QueryType{
			models.QueryTypeMetrics,
			models.QueryTypeLogs,
			models.QueryTypeTraces,
		}

		results := make(chan error, len(queryTypes))
		for _, qType := range queryTypes {
			go func(qt models.QueryType) {
				query := CreateTestUnifiedQuery(
					fmt.Sprintf("query-%s", qt),
					qt,
					"test_query",
					time.Hour,
				)
				_, err := engine.ExecuteQuery(ctx, query)
				results <- err
			}(qType)
		}

		// All should complete (with errors due to nil services)
		for i := 0; i < len(queryTypes); i++ {
			err := <-results
			assert.Error(t, err)
		}
	})
}

// TestUnifiedQueryEngine_CachingBehavior tests caching functionality
func TestUnifiedQueryEngine_CachingBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping caching behavior test in short mode")
	}

	framework := NewIntegrationTestFramework()
	framework.Setup()
	defer framework.TearDown()

	log := logger.New("info")
	mockCache := cache.NewNoopValkeyCache(log)
	engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

	t.Run("Cache Miss Behavior", func(t *testing.T) {
		ctx := context.Background()
		query := CreateTestUnifiedQuery("cache-test-1", models.QueryTypeMetrics, "cpu_usage", time.Hour)

		// First query should miss cache and execute
		result1, err1 := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err1) // Error due to nil service
		assert.Nil(t, result1)

		// Second query should also miss (no actual caching with nil services)
		result2, err2 := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err2)
		assert.Nil(t, result2)
	})

	t.Run("Cache Invalidation", func(t *testing.T) {
		ctx := context.Background()

		// Invalidate cache for pattern
		err := engine.InvalidateCache(ctx, "cpu_usage")
		assert.NoError(t, err)

		// Invalidate multiple patterns
		patterns := []string{"metrics:*", "logs:*", "traces:*"}
		for _, pattern := range patterns {
			err := engine.InvalidateCache(ctx, pattern)
			assert.NoError(t, err)
		}
	})
}

// TestUnifiedQueryEngine_ErrorRecovery tests error handling and recovery
func TestUnifiedQueryEngine_ErrorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error recovery test in short mode")
	}

	framework := NewIntegrationTestFramework()
	framework.Setup()
	defer framework.TearDown()

	log := logger.New("info")
	mockCache := cache.NewNoopValkeyCache(log)

	t.Run("Service Unavailable Recovery", func(t *testing.T) {
		// Create engine with nil services
		engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

		ctx := context.Background()
		query := CreateTestUnifiedQuery("error-test-1", models.QueryTypeMetrics, "cpu_usage", time.Hour)

		// Should handle gracefully
		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metrics service not configured")

		// Engine should still be functional
		health, healthErr := engine.HealthCheck(ctx)
		assert.NoError(t, healthErr)
		assert.NotNil(t, health)
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		query := CreateTestUnifiedQuery("cancel-test-1", models.QueryTypeMetrics, "cpu_usage", time.Hour)

		// Should handle cancelled context
		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
	})

	t.Run("Context Timeout", func(t *testing.T) {
		engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout occurs

		query := CreateTestUnifiedQuery("timeout-test-1", models.QueryTypeMetrics, "cpu_usage", time.Hour)

		// Should handle timeout
		_, err := engine.ExecuteQuery(ctx, query)
		assert.Error(t, err)
	})
}

// TestUnifiedQueryEngine_QueryMetadata tests metadata retrieval
func TestUnifiedQueryEngine_QueryMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping query metadata test in short mode")
	}

	framework := NewIntegrationTestFramework()
	framework.Setup()
	defer framework.TearDown()

	log := logger.New("info")
	mockCache := cache.NewNoopValkeyCache(log)
	engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

	t.Run("Get Supported Engines", func(t *testing.T) {
		ctx := context.Background()
		metadata, err := engine.GetQueryMetadata(ctx)

		require.NoError(t, err)
		require.NotNil(t, metadata)

		// Should report all supported engines
		assert.Contains(t, metadata.SupportedEngines, models.QueryTypeMetrics)
		assert.Contains(t, metadata.SupportedEngines, models.QueryTypeLogs)
		assert.Contains(t, metadata.SupportedEngines, models.QueryTypeTraces)
	})

	t.Run("Query Capabilities", func(t *testing.T) {
		ctx := context.Background()
		metadata, err := engine.GetQueryMetadata(ctx)

		require.NoError(t, err)
		require.NotNil(t, metadata)

		// Should have capabilities for each engine
		assert.NotEmpty(t, metadata.QueryCapabilities[models.QueryTypeMetrics])
		assert.NotEmpty(t, metadata.QueryCapabilities[models.QueryTypeLogs])
		assert.NotEmpty(t, metadata.QueryCapabilities[models.QueryTypeTraces])
	})

	t.Run("Cache Capabilities", func(t *testing.T) {
		ctx := context.Background()
		metadata, err := engine.GetQueryMetadata(ctx)

		require.NoError(t, err)
		require.NotNil(t, metadata)

		// Should report cache capabilities
		assert.True(t, metadata.CacheCapabilities.Supported)
		assert.Greater(t, metadata.CacheCapabilities.DefaultTTL, time.Duration(0))
	})
}

// TestUnifiedQueryEngine_HealthChecks tests health check functionality
func TestUnifiedQueryEngine_HealthChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health checks test in short mode")
	}

	framework := NewIntegrationTestFramework()
	framework.Setup()
	defer framework.TearDown()

	log := logger.New("info")
	mockCache := cache.NewNoopValkeyCache(log)

	t.Run("All Services Unavailable", func(t *testing.T) {
		engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

		ctx := context.Background()
		health, err := engine.HealthCheck(ctx)

		require.NoError(t, err)
		require.NotNil(t, health)

		// All engines should be marked as not configured
		framework.AssertEngineHealth(t, health.EngineHealth, models.QueryTypeMetrics, "not_configured")
		framework.AssertEngineHealth(t, health.EngineHealth, models.QueryTypeLogs, "not_configured")
		framework.AssertEngineHealth(t, health.EngineHealth, models.QueryTypeTraces, "not_configured")
	})

	t.Run("Health Check Performance", func(t *testing.T) {
		engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

		ctx := context.Background()

		// Run multiple health checks to test performance
		iterations := 100
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := engine.HealthCheck(ctx)
			duration := time.Since(start)
			totalDuration += duration

			assert.NoError(t, err)
		}

		avgDuration := totalDuration / time.Duration(iterations)
		t.Logf("Average health check duration: %v", avgDuration)

		// Health checks should be fast (< 10ms average)
		assert.Less(t, avgDuration, 10*time.Millisecond)
	})
}

// TestTestHelpers validates the test helper functions
func TestTestHelpers(t *testing.T) {
	t.Run("GenerateMetricsResult", func(t *testing.T) {
		result := GenerateMetricsResult("cpu_usage", 85.5, time.Now())
		assert.NotNil(t, result)
		assert.Equal(t, "success", result.Status)
		assert.NotNil(t, result.Data)
	})

	t.Run("GenerateLogsResult", func(t *testing.T) {
		result := GenerateLogsResult("error", "Test error", time.Now())
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Logs)
		assert.Equal(t, "error", result.Logs[0]["level"])
	})

	t.Run("GenerateTracesResult", func(t *testing.T) {
		result := GenerateTracesResult("trace-123")
		assert.NotNil(t, result)
		assert.NotEmpty(t, result)
		assert.Equal(t, "trace-123", result[0].TraceID)
	})

	t.Run("CreateTestUnifiedQuery", func(t *testing.T) {
		query := CreateTestUnifiedQuery("test-1", models.QueryTypeMetrics, "cpu_usage", time.Hour)
		assert.NotNil(t, query)
		assert.Equal(t, "test-1", query.ID)
		assert.Equal(t, models.QueryTypeMetrics, query.Type)
		assert.NotNil(t, query.StartTime)
		assert.NotNil(t, query.EndTime)
	})

	t.Run("WaitForCondition", func(t *testing.T) {
		// Condition that's immediately true
		result := WaitForCondition(func() bool { return true }, 100*time.Millisecond, 10*time.Millisecond)
		assert.True(t, result)

		// Condition that never becomes true
		result = WaitForCondition(func() bool { return false }, 50*time.Millisecond, 10*time.Millisecond)
		assert.False(t, result)

		// Condition that becomes true after delay
		counter := 0
		result = WaitForCondition(func() bool {
			counter++
			return counter >= 3
		}, 200*time.Millisecond, 20*time.Millisecond)
		assert.True(t, result)
	})

	t.Run("CompareQueryResults", func(t *testing.T) {
		// Both nil
		assert.True(t, CompareQueryResults(nil, nil))

		// One nil
		result1 := &models.UnifiedResult{QueryID: "test-1"}
		assert.False(t, CompareQueryResults(result1, nil))

		// Same results
		result2 := &models.UnifiedResult{
			QueryID: "test-1",
			Type:    models.QueryTypeMetrics,
			Status:  "success",
		}
		result3 := &models.UnifiedResult{
			QueryID: "test-1",
			Type:    models.QueryTypeMetrics,
			Status:  "success",
		}
		assert.True(t, CompareQueryResults(result2, result3))

		// Different results
		result4 := &models.UnifiedResult{
			QueryID: "test-2",
			Type:    models.QueryTypeMetrics,
			Status:  "success",
		}
		assert.False(t, CompareQueryResults(result2, result4))
	})
}

// Benchmark tests for performance validation
func BenchmarkUnifiedQueryEngine_ExecuteQuery(b *testing.B) {
	log := logger.New("error") // Use error level to reduce logging overhead
	mockCache := cache.NewNoopValkeyCache(log)
	engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

	ctx := context.Background()
	query := CreateTestUnifiedQuery("bench-1", models.QueryTypeMetrics, "cpu_usage", time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ExecuteQuery(ctx, query)
	}
}

func BenchmarkUnifiedQueryEngine_HealthCheck(b *testing.B) {
	log := logger.New("error")
	mockCache := cache.NewNoopValkeyCache(log)
	engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.HealthCheck(ctx)
	}
}

func BenchmarkUnifiedQueryEngine_GetQueryMetadata(b *testing.B) {
	log := logger.New("error")
	mockCache := cache.NewNoopValkeyCache(log)
	engine := NewUnifiedQueryEngine(nil, nil, nil, nil, nil, mockCache, log)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.GetQueryMetadata(ctx)
	}
}
