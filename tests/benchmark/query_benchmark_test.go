package benchmark

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

func BenchmarkMetricsQLQuery(b *testing.B) {
	// Setup
	logger := logger.New("error")
	metricsService := setupTestVictoriaMetricsService(logger)

	query := models.MetricsQLQueryRequest{
		Query:    "up",
		Time:     time.Now().Format(time.RFC3339),
		TenantID: "test-tenant",
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := metricsService.ExecuteQuery(ctx, &query)
			if err != nil {
				b.Errorf("Query failed: %v", err)
			}
		}
	})
}

func BenchmarkLogQLQuery(b *testing.B) {
	logger := logger.New("error")
	logsService := setupTestVictoriaLogsService(logger)

	query := models.LogsQLQueryRequest{
		Query:    "_time:1h error",
		Limit:    100,
		TenantID: "test-tenant",
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := logsService.ExecuteQuery(ctx, &query)
			if err != nil {
				b.Errorf("LogsQL query failed: %v", err)
			}
		}
	})
}

func BenchmarkValkeyClusterCache(b *testing.B) {
	cache := setupTestValkeyCluster()
	ctx := context.Background()

	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 12345,
		"key3": []string{"a", "b", "c"},
	}

	b.Run("Set", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("benchmark_key_%d", i)
				cache.Set(ctx, key, testData, time.Minute)
				i++
			}
		})
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate cache
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("benchmark_key_%d", i)
			cache.Set(ctx, key, testData, time.Minute)
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("benchmark_key_%d", i%1000)
				cache.Get(ctx, key)
				i++
			}
		})
	})
}
