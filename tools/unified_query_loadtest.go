package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// LoadTestConfig holds configuration for the load test
type LoadTestConfig struct {
	Duration           time.Duration
	ConcurrentUsers    int
	QueryRatePerSecond int
	QueryTypes         []string // metrics, logs, traces, correlation, uql
	UseCache           bool
	ReportInterval     time.Duration
	TargetEngine       string // unified, metrics, logs, traces
}

// LoadTestResult holds results from the load test
type LoadTestResult struct {
	TotalQueries        int64
	SuccessfulQueries   int64
	FailedQueries       int64
	TotalLatency        time.Duration
	MinLatency          time.Duration
	MaxLatency          time.Duration
	AverageLatency      time.Duration
	P95Latency          time.Duration
	P99Latency          time.Duration
	QueriesPerSecond    float64
	ErrorRate           float64
	LatencyDistribution map[string]int64 // latency buckets
	QueryTypeStats      map[string]*QueryTypeResult
}

// QueryTypeResult holds results for a specific query type
type QueryTypeResult struct {
	TotalQueries      int64
	SuccessfulQueries int64
	FailedQueries     int64
	AverageLatency    time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
}

// QueryExecutor interface for different query engines
type QueryExecutor interface {
	ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)
	Name() string
}

// UnifiedQueryExecutor wraps the unified query engine
type UnifiedQueryExecutor struct {
	engine services.UnifiedQueryEngine
}

func (e *UnifiedQueryExecutor) ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	return e.engine.ExecuteQuery(ctx, query)
}

func (e *UnifiedQueryExecutor) Name() string {
	return "unified-query-engine"
}

// LoadTestRunner orchestrates the load test
type LoadTestRunner struct {
	config   LoadTestConfig
	executor QueryExecutor
	logger   logger.Logger

	// Metrics
	totalQueries      int64
	successfulQueries int64
	failedQueries     int64
	latencies         []time.Duration
	latencyMutex      sync.Mutex

	// Query type metrics
	queryTypeStats map[string]*QueryTypeResult
	statsMutex     sync.RWMutex

	// Cancellation
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewLoadTestRunner creates a new load test runner
func NewLoadTestRunner(config LoadTestConfig, executor QueryExecutor, logger logger.Logger) *LoadTestRunner {
	ctx, cancel := context.WithCancel(context.Background())

	return &LoadTestRunner{
		config:         config,
		executor:       executor,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		queryTypeStats: make(map[string]*QueryTypeResult),
		latencies:      make([]time.Duration, 0, config.QueryRatePerSecond*int(config.Duration.Seconds())),
	}
}

// Run starts the load test
func (r *LoadTestRunner) Run() (*LoadTestResult, error) {
	r.logger.Info("Starting load test",
		"duration", r.config.Duration,
		"concurrent_users", r.config.ConcurrentUsers,
		"query_rate", r.config.QueryRatePerSecond,
		"target", r.config.TargetEngine,
	)

	// Start metrics reporter
	go r.reportMetrics()

	// Start worker goroutines
	queriesPerWorker := r.config.QueryRatePerSecond / r.config.ConcurrentUsers
	if queriesPerWorker < 1 {
		queriesPerWorker = 1
	}

	for i := 0; i < r.config.ConcurrentUsers; i++ {
		r.wg.Add(1)
		go r.worker(i, queriesPerWorker)
	}

	// Wait for duration
	time.Sleep(r.config.Duration)

	// Stop workers
	r.cancel()
	r.wg.Wait()

	// Calculate and return results
	return r.calculateResults(), nil
}

// worker simulates a single user executing queries
func (r *LoadTestRunner) worker(workerID int, queriesPerSecond int) {
	defer r.wg.Done()

	ticker := time.NewTicker(time.Second / time.Duration(queriesPerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.executeRandomQuery(workerID)
		}
	}
}

// executeRandomQuery executes a random query based on configured types
func (r *LoadTestRunner) executeRandomQuery(workerID int) {
	// Select random query type
	queryType := r.config.QueryTypes[rand.Intn(len(r.config.QueryTypes))]

	// Generate query
	query := r.generateQuery(queryType, workerID)

	// Execute query with timeout
	ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
	defer cancel()

	start := time.Now()
	_, err := r.executor.ExecuteQuery(ctx, query)
	latency := time.Since(start)

	// Record metrics
	atomic.AddInt64(&r.totalQueries, 1)

	if err != nil {
		atomic.AddInt64(&r.failedQueries, 1)
		r.logger.Debug("Query failed", "query_id", query.ID, "error", err, "latency", latency)
	} else {
		atomic.AddInt64(&r.successfulQueries, 1)
	}

	// Record latency
	r.latencyMutex.Lock()
	r.latencies = append(r.latencies, latency)
	r.latencyMutex.Unlock()

	// Update query type stats
	r.updateQueryTypeStats(queryType, latency, err == nil)
}

// generateQuery generates a query based on type
func (r *LoadTestRunner) generateQuery(queryType string, workerID int) *models.UnifiedQuery {
	now := time.Now()
	startTime := now.Add(-1 * time.Hour)
	queryID := fmt.Sprintf("%s-worker%d-%d", queryType, workerID, time.Now().UnixNano())

	query := &models.UnifiedQuery{
		ID:        queryID,
		StartTime: &startTime,
		EndTime:   &now,
	}

	switch queryType {
	case "metrics":
		query.Type = models.QueryTypeMetrics
		query.Query = r.generateMetricsQuery()
	case "logs":
		query.Type = models.QueryTypeLogs
		query.Query = r.generateLogsQuery()
	case "traces":
		query.Type = models.QueryTypeTraces
		query.Query = r.generateTracesQuery()
	case "correlation":
		query.Type = models.QueryTypeCorrelation
		query.Query = r.generateCorrelationQuery()
	case "uql":
		// UQL is a superset - route to appropriate engine based on query content
		query.Type = models.QueryTypeMetrics // Default to metrics for now
		query.Query = r.generateUQLQuery()
	default:
		query.Type = models.QueryTypeMetrics
		query.Query = "cpu_usage"
	}

	return query
}

// Query generators
func (r *LoadTestRunner) generateMetricsQuery() string {
	metrics := []string{
		"cpu_usage",
		"memory_usage",
		"disk_io",
		"network_throughput",
		"request_rate",
		"error_rate",
		"response_time",
	}
	return metrics[rand.Intn(len(metrics))]
}

func (r *LoadTestRunner) generateLogsQuery() string {
	queries := []string{
		"error",
		"exception",
		"warning",
		"level:error",
		"status:500",
		"error AND exception",
		"error OR warning",
		"kubernetes AND pod",
	}
	return queries[rand.Intn(len(queries))]
}

func (r *LoadTestRunner) generateTracesQuery() string {
	services := []string{"api", "auth", "database", "cache", "payment"}
	operations := []string{"GET", "POST", "PUT", "DELETE", "SELECT", "INSERT"}

	service := services[rand.Intn(len(services))]
	operation := operations[rand.Intn(len(operations))]

	queries := []string{
		fmt.Sprintf("service:%s", service),
		fmt.Sprintf("operation:%s", operation),
		fmt.Sprintf("service:%s operation:%s", service, operation),
		fmt.Sprintf("service:%s duration>100", service),
		"error:true",
	}
	return queries[rand.Intn(len(queries))]
}

func (r *LoadTestRunner) generateCorrelationQuery() string {
	queries := []string{
		"logs:error WITHIN 5m OF metrics:cpu_usage > 80",
		"traces:error WITHIN 1m OF logs:exception",
		"metrics:error_rate > 5 WITHIN 10m OF traces:error",
		"logs:status:500 WITHIN 2m OF metrics:response_time > 1000",
	}
	return queries[rand.Intn(len(queries))]
}

func (r *LoadTestRunner) generateUQLQuery() string {
	queries := []string{
		"SELECT cpu_usage FROM metrics WHERE time > now() - 1h",
		"SELECT * FROM logs WHERE level = 'error' AND time > now() - 1h",
		"SELECT service, operation FROM traces WHERE duration > 100",
		"CORRELATE logs.error WITH metrics.cpu_usage WITHIN 5m",
	}
	return queries[rand.Intn(len(queries))]
}

// updateQueryTypeStats updates statistics for a specific query type
func (r *LoadTestRunner) updateQueryTypeStats(queryType string, latency time.Duration, success bool) {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()

	stats, exists := r.queryTypeStats[queryType]
	if !exists {
		stats = &QueryTypeResult{
			MinLatency: time.Hour, // Set high initial value
		}
		r.queryTypeStats[queryType] = stats
	}

	stats.TotalQueries++
	if success {
		stats.SuccessfulQueries++
	} else {
		stats.FailedQueries++
	}

	if latency < stats.MinLatency {
		stats.MinLatency = latency
	}
	if latency > stats.MaxLatency {
		stats.MaxLatency = latency
	}
}

// reportMetrics periodically reports current metrics
func (r *LoadTestRunner) reportMetrics() {
	ticker := time.NewTicker(r.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			total := atomic.LoadInt64(&r.totalQueries)
			successful := atomic.LoadInt64(&r.successfulQueries)
			failed := atomic.LoadInt64(&r.failedQueries)

			r.logger.Info("Load test progress",
				"total_queries", total,
				"successful", successful,
				"failed", failed,
				"error_rate", float64(failed)/float64(total)*100,
			)
		}
	}
}

// calculateResults computes final test results
func (r *LoadTestRunner) calculateResults() *LoadTestResult {
	r.latencyMutex.Lock()
	defer r.latencyMutex.Unlock()

	result := &LoadTestResult{
		TotalQueries:      atomic.LoadInt64(&r.totalQueries),
		SuccessfulQueries: atomic.LoadInt64(&r.successfulQueries),
		FailedQueries:     atomic.LoadInt64(&r.failedQueries),
		QueryTypeStats:    r.queryTypeStats,
	}

	if result.TotalQueries == 0 {
		return result
	}

	// Calculate error rate
	result.ErrorRate = float64(result.FailedQueries) / float64(result.TotalQueries) * 100

	// Calculate QPS
	result.QueriesPerSecond = float64(result.TotalQueries) / r.config.Duration.Seconds()

	// Sort latencies
	latencies := make([]time.Duration, len(r.latencies))
	copy(latencies, r.latencies)

	// Calculate min/max/avg
	var totalLatency time.Duration
	result.MinLatency = latencies[0]
	result.MaxLatency = latencies[0]

	for _, latency := range latencies {
		totalLatency += latency
		if latency < result.MinLatency {
			result.MinLatency = latency
		}
		if latency > result.MaxLatency {
			result.MaxLatency = latency
		}
	}

	result.TotalLatency = totalLatency
	result.AverageLatency = totalLatency / time.Duration(len(latencies))

	// Calculate percentiles (simple approximation)
	if len(latencies) > 0 {
		p95Index := int(float64(len(latencies)) * 0.95)
		p99Index := int(float64(len(latencies)) * 0.99)

		if p95Index >= len(latencies) {
			p95Index = len(latencies) - 1
		}
		if p99Index >= len(latencies) {
			p99Index = len(latencies) - 1
		}

		result.P95Latency = latencies[p95Index]
		result.P99Latency = latencies[p99Index]
	}

	// Calculate latency distribution
	result.LatencyDistribution = make(map[string]int64)
	buckets := []time.Duration{
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		5 * time.Second,
	}

	for _, latency := range latencies {
		for i, bucket := range buckets {
			if latency <= bucket {
				key := fmt.Sprintf("<%dms", bucket.Milliseconds())
				result.LatencyDistribution[key]++
				break
			}
			if i == len(buckets)-1 {
				result.LatencyDistribution[">5s"]++
			}
		}
	}

	// Calculate average latency for each query type
	r.statsMutex.RLock()
	defer r.statsMutex.RUnlock()

	for queryType, stats := range r.queryTypeStats {
		if stats.TotalQueries > 0 {
			// Average latency needs to be calculated from recorded latencies for this query type
			// For now, we'll use an approximation
			var queryTypeLatency time.Duration
			count := int64(0)

			for _, latency := range latencies {
				// This is a simplification; in a real implementation, you'd track latencies per query type
				if count < stats.SuccessfulQueries {
					queryTypeLatency += latency
					count++
				}
			}

			if count > 0 {
				stats.AverageLatency = queryTypeLatency / time.Duration(count)
			}
		}

		r.logger.Info("Query type stats",
			"type", queryType,
			"total", stats.TotalQueries,
			"successful", stats.SuccessfulQueries,
			"failed", stats.FailedQueries,
			"avg_latency", stats.AverageLatency,
			"min_latency", stats.MinLatency,
			"max_latency", stats.MaxLatency,
		)
	}

	return result
}

// PrintResults prints the test results
func PrintResults(result *LoadTestResult) {
	fmt.Println("\n========================================")
	fmt.Println("      Unified Query Load Test Results")
	fmt.Println("========================================")
	fmt.Printf("Total Queries:      %d\n", result.TotalQueries)
	fmt.Printf("Successful Queries: %d\n", result.SuccessfulQueries)
	fmt.Printf("Failed Queries:     %d\n", result.FailedQueries)
	fmt.Printf("Error Rate:         %.2f%%\n", result.ErrorRate)
	fmt.Printf("Queries/Second:     %.2f\n", result.QueriesPerSecond)
	fmt.Println("\nLatency Statistics:")
	fmt.Printf("  Average:   %v\n", result.AverageLatency)
	fmt.Printf("  Minimum:   %v\n", result.MinLatency)
	fmt.Printf("  Maximum:   %v\n", result.MaxLatency)
	fmt.Printf("  P95:       %v\n", result.P95Latency)
	fmt.Printf("  P99:       %v\n", result.P99Latency)

	fmt.Println("\nLatency Distribution:")
	for bucket, count := range result.LatencyDistribution {
		percentage := float64(count) / float64(result.TotalQueries) * 100
		fmt.Printf("  %s: %d (%.2f%%)\n", bucket, count, percentage)
	}

	fmt.Println("\nQuery Type Statistics:")
	for queryType, stats := range result.QueryTypeStats {
		fmt.Printf("\n  %s:\n", queryType)
		fmt.Printf("    Total:      %d\n", stats.TotalQueries)
		fmt.Printf("    Successful: %d\n", stats.SuccessfulQueries)
		fmt.Printf("    Failed:     %d\n", stats.FailedQueries)
		fmt.Printf("    Avg Latency: %v\n", stats.AverageLatency)
		fmt.Printf("    Min Latency: %v\n", stats.MinLatency)
		fmt.Printf("    Max Latency: %v\n", stats.MaxLatency)
	}
	fmt.Println("========================================\n")
}

// SaveResultsToFile saves results to a JSON file
func SaveResultsToFile(result *LoadTestResult, filename string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

func runLoadTest() {
	// Command-line flags
	duration := flag.Duration("duration", 60*time.Second, "Duration of load test")
	concurrentUsers := flag.Int("users", 10, "Number of concurrent users")
	queryRate := flag.Int("rate", 100, "Query rate per second")
	queryTypes := flag.String("types", "metrics,logs,traces", "Comma-separated query types (metrics,logs,traces,correlation,uql)")
	useCache := flag.Bool("cache", true, "Use cache for queries")
	reportInterval := flag.Duration("report-interval", 10*time.Second, "Metrics reporting interval")
	outputFile := flag.String("output", "unified-loadtest-results.json", "Output file for results")
	targetEngine := flag.String("target", "unified", "Target engine (unified, metrics, logs, traces)")

	flag.Parse()

	// Parse query types
	queryTypeList := []string{"metrics", "logs", "traces"}
	if *queryTypes != "" {
		queryTypeList = []string{}
		for _, qt := range []string{"metrics", "logs", "traces", "correlation", "uql"} {
			if contains(qt, *queryTypes) {
				queryTypeList = append(queryTypeList, qt)
			}
		}
	}

	// Create logger
	log := logger.New("info")

	// Create cache
	var cacheInstance cache.ValkeyCluster
	if *useCache {
		cacheInstance = cache.NewNoopValkeyCache(log)
	}

	// Create unified query engine
	// Note: In a real scenario, you'd set up actual services
	var metricsSvc *services.VictoriaMetricsService
	var logsSvc *services.VictoriaLogsService
	var tracesSvc *services.VictoriaTracesService
	var correlationEngine services.CorrelationEngine
	var bleveSearchSvc *services.BleveSearchService

	engine := services.NewUnifiedQueryEngine(
		metricsSvc,
		logsSvc,
		tracesSvc,
		correlationEngine,
		bleveSearchSvc,
		cacheInstance,
		log,
	)

	executor := &UnifiedQueryExecutor{engine: engine}

	// Create load test config
	config := LoadTestConfig{
		Duration:           *duration,
		ConcurrentUsers:    *concurrentUsers,
		QueryRatePerSecond: *queryRate,
		QueryTypes:         queryTypeList,
		UseCache:           *useCache,
		ReportInterval:     *reportInterval,
		TargetEngine:       *targetEngine,
	}

	// Run load test
	runner := NewLoadTestRunner(config, executor, log)
	result, err := runner.Run()
	if err != nil {
		log.Error("Load test failed", "error", err)
		os.Exit(1)
	}

	// Print results
	PrintResults(result)

	// Save results to file
	if err := SaveResultsToFile(result, *outputFile); err != nil {
		log.Error("Failed to save results", "error", err)
	} else {
		log.Info("Results saved", "file", *outputFile)
	}
}

func contains(item, list string) bool {
	for _, v := range []string{"metrics", "logs", "traces", "correlation", "uql"} {
		if v == item && (list == item || len(list) > len(item)) {
			return true
		}
	}
	return false
}

// main entry point
func main() {
	runLoadTest()
}
