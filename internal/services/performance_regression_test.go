package services

import (
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/require"
)

// PerformanceRegressionTestSuite contains performance regression tests for the unified query engine
// These tests ensure that performance does not degrade below acceptable thresholds

// Performance thresholds (in milliseconds)
const (
	MaxUQLParseLatencyP95    = 50  // 50ms 95th percentile for UQL parsing
	MaxUQLOptimizeLatencyP95 = 100 // 100ms 95th percentile for UQL optimization
	MaxQueryRouteLatencyP95  = 10  // 10ms 95th percentile for query routing
	MinParseThroughput       = 100 // 100 parses per second minimum
)

// TestUQLPerformanceRegression tests UQL parsing and optimization performance
func TestUQLPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	logger := logger.New("info")

	// Test queries representing typical workloads
	testQueries := []string{
		"SELECT service, level FROM logs:error WHERE level='error'",
		"SELECT * FROM logs:error WHERE level='error' AND service='api'",
		"COUNT(*) FROM logs:error",
		"SUM(bytes) FROM logs:error WHERE status_code >= 500",
		"logs:error AND metrics:cpu_usage > 80",
		"SELECT service FROM traces:auth WHERE duration > 1000",
		"SELECT * FROM metrics:cpu_usage WHERE value > 80 ORDER BY timestamp DESC LIMIT 100",
	}

	// Test UQL parsing performance
	t.Run("UQL_Parsing_Performance", func(t *testing.T) {
		results := runUQLParsingPerformanceTest(t, testQueries)
		validateUQLParsingPerformance(t, results)
	})

	// Test UQL optimization performance
	t.Run("UQL_Optimization_Performance", func(t *testing.T) {
		results := runUQLOptimizationPerformanceTest(t, testQueries, logger)
		validateUQLOptimizationPerformance(t, results)
	})

	// Test query routing performance
	t.Run("Query_Routing_Performance", func(t *testing.T) {
		results := runQueryRoutingPerformanceTest(t, testQueries, logger)
		validateQueryRoutingPerformance(t, results)
	})
}

// TestCorrelationQueryPerformanceRegression tests correlation query parsing performance
func TestCorrelationQueryPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	correlationQueries := []string{
		"logs:error WITHIN 5m OF metrics:cpu_usage > 80",
		"logs:error AND metrics:memory_usage > 90",
		"traces:auth WITHIN 1m OF logs:unauthorized",
		"logs:error AND traces:failed_auth AND metrics:high_latency",
		"logs:service:checkout AND traces:service:checkout",
	}

	results := runCorrelationQueryParsingPerformanceTest(t, correlationQueries)
	validateCorrelationQueryParsingPerformance(t, results)
}

// runUQLParsingPerformanceTest tests UQL parsing performance
func runUQLParsingPerformanceTest(t *testing.T, queries []string) *PerformanceTestResults {
	results := &PerformanceTestResults{
		QueryLatencies: make(map[string][]time.Duration),
		StartTime:      time.Now(),
	}

	parser := models.NewUQLParser()

	// Run each query multiple times to get statistical significance
	iterations := 100
	if testing.Short() {
		iterations = 20
	}

	for _, queryStr := range queries {
		t.Logf("Testing UQL parsing performance for: %s", queryStr)

		latencies := make([]time.Duration, iterations)

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := parser.Parse(queryStr)
			duration := time.Since(start)

			require.NoError(t, err, "Failed to parse query: %s", queryStr)
			latencies[i] = duration
		}

		results.QueryLatencies[queryStr] = latencies
		results.TotalQueries += iterations
	}

	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)

	return results
}

// runUQLOptimizationPerformanceTest tests UQL optimization performance
func runUQLOptimizationPerformanceTest(t *testing.T, queries []string, logger logger.Logger) *PerformanceTestResults {
	results := &PerformanceTestResults{
		QueryLatencies: make(map[string][]time.Duration),
		StartTime:      time.Now(),
	}

	parser := models.NewUQLParser()
	optimizer := NewUQLOptimizer(logger)

	// Pre-parse queries
	parsedQueries := make(map[string]*models.UQLQuery)
	for _, queryStr := range queries {
		parsed, err := parser.Parse(queryStr)
		require.NoError(t, err, "Failed to pre-parse query: %s", queryStr)
		parsedQueries[queryStr] = parsed
	}

	iterations := 100
	if testing.Short() {
		iterations = 20
	}

	for _, queryStr := range queries {
		t.Logf("Testing UQL optimization performance for: %s", queryStr)

		latencies := make([]time.Duration, iterations)

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := optimizer.Optimize(parsedQueries[queryStr])
			duration := time.Since(start)

			require.NoError(t, err, "Failed to optimize query: %s", queryStr)
			latencies[i] = duration
		}

		results.QueryLatencies[queryStr] = latencies
		results.TotalQueries += iterations
	}

	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)

	return results
}

// runQueryRoutingPerformanceTest tests query routing performance
func runQueryRoutingPerformanceTest(t *testing.T, queries []string, logger logger.Logger) *PerformanceTestResults {
	results := &PerformanceTestResults{
		QueryLatencies: make(map[string][]time.Duration),
		StartTime:      time.Now(),
	}

	router := NewQueryRouter(logger)

	iterations := 200
	if testing.Short() {
		iterations = 50
	}

	for _, queryStr := range queries {
		t.Logf("Testing query routing performance for: %s", queryStr)

		latencies := make([]time.Duration, iterations)

		for i := 0; i < iterations; i++ {
			query := &models.UnifiedQuery{
				Query: queryStr,
			}

			start := time.Now()
			_, _, err := router.RouteQuery(query)
			duration := time.Since(start)

			require.NoError(t, err, "Failed to route query: %s", queryStr)
			latencies[i] = duration
		}

		results.QueryLatencies[queryStr] = latencies
		results.TotalQueries += iterations
	}

	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)

	return results
}

// runCorrelationQueryParsingPerformanceTest tests correlation query parsing performance
func runCorrelationQueryParsingPerformanceTest(t *testing.T, queries []string) *PerformanceTestResults {
	results := &PerformanceTestResults{
		QueryLatencies: make(map[string][]time.Duration),
		StartTime:      time.Now(),
	}

	parser := models.NewCorrelationQueryParser()

	iterations := 100
	if testing.Short() {
		iterations = 20
	}

	for _, queryStr := range queries {
		t.Logf("Testing correlation query parsing performance for: %s", queryStr)

		latencies := make([]time.Duration, iterations)

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := parser.Parse(queryStr)
			duration := time.Since(start)

			require.NoError(t, err, "Failed to parse correlation query: %s", queryStr)
			latencies[i] = duration
		}

		results.QueryLatencies[queryStr] = latencies
		results.TotalQueries += iterations
	}

	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)

	return results
}

// validateUQLParsingPerformance validates UQL parsing performance thresholds
func validateUQLParsingPerformance(t *testing.T, results *PerformanceTestResults) {
	t.Logf("UQL parsing performance test completed in %v, total queries: %d", results.Duration, results.TotalQueries)

	for queryStr, latencies := range results.QueryLatencies {
		if len(latencies) == 0 {
			continue
		}

		p95 := calculatePercentile(latencies, 95)
		avg := calculateAverage(latencies)

		t.Logf("UQL parsing %s: avg=%v, p95=%v", queryStr, avg, p95)

		if p95 > time.Duration(MaxUQLParseLatencyP95)*time.Millisecond {
			t.Errorf("UQL parsing P95 latency %v exceeds threshold %v for query: %s",
				p95, time.Duration(MaxUQLParseLatencyP95)*time.Millisecond, queryStr)
		}
	}

	throughput := float64(results.TotalQueries) / results.Duration.Seconds()
	t.Logf("UQL parsing throughput: %.2f parses/sec", throughput)

	if throughput < float64(MinParseThroughput) {
		t.Errorf("UQL parsing throughput %.2f parses/sec below minimum threshold %d parses/sec",
			throughput, MinParseThroughput)
	}
}

// validateUQLOptimizationPerformance validates UQL optimization performance thresholds
func validateUQLOptimizationPerformance(t *testing.T, results *PerformanceTestResults) {
	t.Logf("UQL optimization performance test completed in %v, total queries: %d", results.Duration, results.TotalQueries)

	for queryStr, latencies := range results.QueryLatencies {
		if len(latencies) == 0 {
			continue
		}

		p95 := calculatePercentile(latencies, 95)
		t.Logf("UQL optimization P95 latency: %v for query: %s", p95, queryStr)

		if p95 > time.Duration(MaxUQLOptimizeLatencyP95)*time.Millisecond {
			t.Errorf("UQL optimization P95 latency %v exceeds threshold %v for query: %s",
				p95, time.Duration(MaxUQLOptimizeLatencyP95)*time.Millisecond, queryStr)
		}
	}
}

// validateQueryRoutingPerformance validates query routing performance thresholds
func validateQueryRoutingPerformance(t *testing.T, results *PerformanceTestResults) {
	t.Logf("Query routing performance test completed in %v, total queries: %d", results.Duration, results.TotalQueries)

	for queryStr, latencies := range results.QueryLatencies {
		if len(latencies) == 0 {
			continue
		}

		p95 := calculatePercentile(latencies, 95)
		t.Logf("Query routing P95 latency: %v for query: %s", p95, queryStr)

		if p95 > time.Duration(MaxQueryRouteLatencyP95)*time.Millisecond {
			t.Errorf("Query routing P95 latency %v exceeds threshold %v for query: %s",
				p95, time.Duration(MaxQueryRouteLatencyP95)*time.Millisecond, queryStr)
		}
	}
}

// validateCorrelationQueryParsingPerformance validates correlation query parsing performance
func validateCorrelationQueryParsingPerformance(t *testing.T, results *PerformanceTestResults) {
	t.Logf("Correlation query parsing performance test completed in %v, total queries: %d", results.Duration, results.TotalQueries)

	for queryStr, latencies := range results.QueryLatencies {
		if len(latencies) == 0 {
			continue
		}

		p95 := calculatePercentile(latencies, 95)
		t.Logf("Correlation query parsing P95 latency: %v for query: %s", p95, queryStr)

		// Use same threshold as UQL parsing for correlation queries
		if p95 > time.Duration(MaxUQLParseLatencyP95)*time.Millisecond {
			t.Errorf("Correlation query parsing P95 latency %v exceeds threshold %v for query: %s",
				p95, time.Duration(MaxUQLParseLatencyP95)*time.Millisecond, queryStr)
		}
	}
}

// Helper functions for statistics
func calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)-1) * percentile / 100.0)
	return sorted[index]
}

func calculateAverage(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	total := time.Duration(0)
	for _, latency := range latencies {
		total += latency
	}

	return total / time.Duration(len(latencies))
}

// PerformanceTestResults holds the results of performance tests
type PerformanceTestResults struct {
	QueryLatencies map[string][]time.Duration
	TotalQueries   int
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
}
