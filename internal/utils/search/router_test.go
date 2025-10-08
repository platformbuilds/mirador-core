package search

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestSearchRouter_RunABTest(t *testing.T) {
	// Create logger
	logger := logger.New("info")

	// Create search config
	config := &SearchConfig{
		DefaultEngine: "lucene",
		EnableBleve:   true,
		EnableLucene:  true,
		Cache:         cache.NewNoopValkeyCache(logger),
		CacheTTL:      30 * time.Minute,
	}

	// Create search router
	router, err := NewSearchRouter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create search router: %v", err)
	}

	// Run A/B test
	ctx := context.Background()
	result, err := router.RunABTest(ctx, "service:checkout")
	if err != nil {
		t.Fatalf("A/B test failed: %v", err)
	}

	// Verify results
	if result.Query != "service:checkout" {
		t.Errorf("Expected query 'service:checkout', got '%s'", result.Query)
	}

	if result.LuceneResult.Duration == 0 {
		t.Error("Lucene duration should be > 0")
	}

	if result.BleveResult.Duration == 0 {
		t.Error("Bleve duration should be > 0")
	}

	// Check that we have results (may not be equal due to different implementations)
	if result.LuceneResult.LogsQL == "" && result.LuceneResult.Error == "" {
		t.Error("Lucene should have either LogsQL result or error")
	}

	if result.BleveResult.LogsQL == "" && result.BleveResult.Error == "" {
		t.Error("Bleve should have either LogsQL result or error")
	}

	t.Logf("A/B Test completed successfully")
	t.Logf("Lucene LogsQL: %s", result.LuceneResult.LogsQL)
	t.Logf("Bleve LogsQL: %s", result.BleveResult.LogsQL)
	t.Logf("LogsQL Equal: %v", result.Comparison.LogsQLEqual)
	t.Logf("Traces Equal: %v", result.Comparison.TracesEqual)
}
