package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/platformbuilds/mirador-core/internal/utils/loadtest"
	"github.com/platformbuilds/mirador-core/internal/utils/search"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Default query patterns based on realistic usage
var defaultQueryPatterns = []loadtest.QueryPattern{
	{"error", 30},               // 30% - Error searches
	{"service:api-gateway", 20}, // 20% - Service-specific searches
	{"level:info", 15},          // 15% - Level searches
	{"response_time:>1000", 10}, // 10% - Performance queries
	{"user_id:123", 10},         // 10% - User-specific searches
	{"level:(info OR warn) AND service:(user-service OR order-service)", 10}, // 10% - Complex queries
	{"message:test*", 5}, // 5% - Wildcard searches
}

func main() {
	// Parse command line flags
	duration := flag.Duration("duration", 5*time.Minute, "Test duration")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	enableBleve := flag.Bool("bleve", true, "Enable Bleve search engine")
	enableLucene := flag.Bool("lucene", true, "Enable Lucene search engine")
	cacheEnabled := flag.Bool("cache", true, "Enable query caching")
	cacheTTL := flag.Duration("cache-ttl", 30*time.Minute, "Cache TTL")
	valkeyNodes := flag.String("valkey-nodes", "", "Valkey nodes (comma-separated)")
	outputFile := flag.String("output", "loadtest-results.json", "Output file for results")

	flag.Parse()

	// Create logger
	logger := logger.New("info")

	// Create search router with cache if enabled
	var cacheInstance cache.ValkeyCluster
	if *cacheEnabled && *valkeyNodes != "" {
		// Parse valkey nodes
		nodes := parseValkeyNodes(*valkeyNodes)
		var err error
		cacheInstance, err = cache.NewValkeyCluster(nodes, *cacheTTL)
		if err != nil {
			log.Fatalf("Failed to create Valkey cache: %v", err)
		}
	} else {
		// Use noop cache
		cacheInstance = cache.NewNoopValkeyCache(logger)
	}

	// Create search config
	searchConfig := &search.SearchConfig{
		DefaultEngine: "bleve",
		EnableBleve:   *enableBleve,
		EnableLucene:  *enableLucene,
		Cache:         cacheInstance,
		CacheTTL:      *cacheTTL,
	}

	searchRouter, err := search.NewSearchRouter(searchConfig, logger)
	if err != nil {
		log.Fatalf("Failed to create search router: %v", err)
	}

	// Create load test configuration
	config := &loadtest.LoadTestConfig{
		Duration:          *duration,
		ConcurrentWorkers: *workers,
		QueryPatterns:     defaultQueryPatterns,
		Engine:            "bleve",
	}

	// Create load tester
	tester, err := loadtest.NewLoadTester(config, logger)
	if err != nil {
		log.Fatalf("Failed to create load tester: %v", err)
	}

	tester.SetSearchRouter(searchRouter)

	// Run load test
	ctx := context.Background()
	fmt.Printf("Starting load test with %d workers for %v...\n", *workers, *duration)

	results, err := tester.RunLoadTest(ctx)
	if err != nil {
		log.Fatalf("Load test failed: %v", err)
	}

	// Output results
	fmt.Printf("\nLoad Test Results:\n")
	fmt.Printf("================\n")
	fmt.Printf("Duration: %v\n", results.TotalDuration)
	fmt.Printf("Total Queries: %d\n", results.TotalQueries)
	fmt.Printf("Successful Queries: %d\n", results.SuccessfulQueries)
	fmt.Printf("Failed Queries: %d\n", results.FailedQueries)
	fmt.Printf("Average Query Time: %v\n", results.AvgQueryTime)
	fmt.Printf("95th Percentile: %v\n", results.P95QueryTime)
	fmt.Printf("99th Percentile: %v\n", results.P99QueryTime)
	fmt.Printf("QPS: %.2f\n", results.QPS)
	fmt.Printf("Cache Hit Rate: %.2f%%\n", results.CacheHitRate*100)

	if len(results.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(results.Errors))
		for i, err := range results.Errors {
			if i >= 5 { // Limit error output
				fmt.Printf("... and %d more errors\n", len(results.Errors)-5)
				break
			}
			fmt.Printf("  - %v\n", err)
		}
	}

	// Save results to file
	if err := saveResultsToFile(results, *outputFile); err != nil {
		log.Printf("Failed to save results to file: %v", err)
	} else {
		fmt.Printf("Results saved to %s\n", *outputFile)
	}
}

func parseValkeyNodes(nodesStr string) []string {
	// Simple comma-separated parsing
	var nodes []string
	current := ""
	for _, r := range nodesStr {
		if r == ',' {
			if current != "" {
				nodes = append(nodes, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		nodes = append(nodes, current)
	}
	return nodes
}

func saveResultsToFile(results *loadtest.LoadTestResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Simple JSON output (could be enhanced with proper JSON marshaling)
	fmt.Fprintf(file, `{
  "duration": "%v",
  "total_queries": %d,
  "successful_queries": %d,
  "failed_queries": %d,
  "avg_query_time": "%v",
  "p95_query_time": "%v",
  "p99_query_time": "%v",
  "qps": %.2f,
  "cache_hit_rate": %.2f,
  "errors": %d
}`, results.TotalDuration,
		results.TotalQueries,
		results.SuccessfulQueries,
		results.FailedQueries,
		results.AvgQueryTime,
		results.P95QueryTime,
		results.P99QueryTime,
		results.QPS,
		results.CacheHitRate,
		len(results.Errors))

	return nil
}
