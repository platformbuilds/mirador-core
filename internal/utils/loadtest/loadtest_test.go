package loadtest

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/utils/search"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TestLoadTestBasic tests the basic load testing functionality
func TestLoadTestBasic(t *testing.T) {
	logger := logger.New("info")

	// Create search router with mock cache
	searchConfig := &search.SearchConfig{
		DefaultEngine: "bleve",
		EnableBleve:   true,
		EnableLucene:  false,
		Cache:         cache.NewNoopValkeyCache(logger), // Use noop cache for testing
		CacheTTL:      30 * time.Minute,
	}

	searchRouter, err := search.NewSearchRouter(searchConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create search router: %v", err)
	}

	// Create load test configuration
	config := &LoadTestConfig{
		Duration:          5 * time.Second, // Short test
		ConcurrentWorkers: 2,
		QueryPatterns: []QueryPattern{
			{"error", 50},
			{"service:api-gateway", 30},
			{"level:info", 20},
		},
		Engine: "bleve",
	}

	// Create load tester
	tester, err := NewLoadTester(config, logger)
	if err != nil {
		t.Fatalf("Failed to create load tester: %v", err)
	}

	tester.SetSearchRouter(searchRouter)

	// Run load test
	ctx := context.Background()
	results, err := tester.RunLoadTest(ctx)
	if err != nil {
		t.Fatalf("Load test failed: %v", err)
	}

	// Verify results
	if results.TotalQueries == 0 {
		t.Error("Expected some queries to be executed")
	}

	if results.TotalDuration == 0 {
		t.Error("Expected non-zero duration")
	}

	t.Logf("Load test results: %d queries, %v avg time, %.2f QPS",
		results.TotalQueries, results.AvgQueryTime, results.QPS)
}

// BenchmarkLoadTest benchmarks the load testing framework
func BenchmarkLoadTest(b *testing.B) {
	logger := logger.New("error") // Reduce log noise

	searchConfig := &search.SearchConfig{
		DefaultEngine: "bleve",
		EnableBleve:   true,
		EnableLucene:  false,
		Cache:         cache.NewNoopValkeyCache(logger),
		CacheTTL:      30 * time.Minute,
	}

	searchRouter, err := search.NewSearchRouter(searchConfig, logger)
	if err != nil {
		b.Fatalf("Failed to create search router: %v", err)
	}

	config := &LoadTestConfig{
		Duration:          1 * time.Second,
		ConcurrentWorkers: 1,
		QueryPatterns: []QueryPattern{
			{"error", 100},
		},
		Engine: "bleve",
	}

	tester, err := NewLoadTester(config, logger)
	if err != nil {
		b.Fatalf("Failed to create load tester: %v", err)
	}

	tester.SetSearchRouter(searchRouter)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := tester.RunLoadTest(ctx)
		if err != nil {
			b.Fatalf("Load test failed: %v", err)
		}
	}
}
