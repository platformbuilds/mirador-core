package benchmark

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/internal/services"
	"github.com/platformbuilds/miradorstack/pkg/cache"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

// setupTestVictoriaMetricsService creates a VictoriaMetricsService for tests
func setupTestVictoriaMetricsService(log logger.Logger) *services.VictoriaMetricsService {
	cfg := config.VictoriaMetricsConfig{
		Endpoints: []string{"http://localhost:8428"}, // VM default endpoint
		Timeout:   2000,                              // ms
	}
	return services.NewVictoriaMetricsService(cfg, log)
}

// setupTestVictoriaLogsService creates a VictoriaLogsService for tests
func setupTestVictoriaLogsService(log logger.Logger) *services.VictoriaLogsService {
	cfg := config.VictoriaLogsConfig{
		Endpoints: []string{"http://localhost:9428"}, // VL default endpoint
		Timeout:   2000,
	}
	return services.NewVictoriaLogsService(cfg, log)
}

// setupTestValkeyCluster creates a ValkeyCluster using localhost Redis
func setupTestValkeyCluster() cache.ValkeyCluster {
	nodes := []string{"127.0.0.1:6379"}
	c, err := cache.NewValkeyCluster(nodes, time.Minute)
	if err != nil {
		panic("failed to connect to test valkey cluster: " + err.Error())
	}
	return c
}

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
