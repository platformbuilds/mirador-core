package loadtest

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/utils/search"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// LoadTestConfig holds configuration for load testing
type LoadTestConfig struct {
	// Test duration
	Duration time.Duration

	// Number of concurrent workers
	ConcurrentWorkers int

	// Query patterns
	QueryPatterns []QueryPattern

	// Search engine to test
	Engine string
}

// QueryPattern represents a query pattern with weight for distribution
type QueryPattern struct {
	Query  string
	Weight int
}

// LoadTestResult holds the results of a load test
type LoadTestResult struct {
	TotalDuration     time.Duration
	TotalQueries      int64
	SuccessfulQueries int64
	FailedQueries     int64
	AvgQueryTime      time.Duration
	P95QueryTime      time.Duration
	P99QueryTime      time.Duration
	QPS               float64
	CacheHitRate      float64
	Errors            []error
}

// LoadTester manages load testing for search engines
type LoadTester struct {
	config       *LoadTestConfig
	logger       logger.Logger
	searchRouter *search.SearchRouter
	results      *LoadTestResult
}

// NewLoadTester creates a new load tester
func NewLoadTester(config *LoadTestConfig, logger logger.Logger) (*LoadTester, error) {
	return &LoadTester{
		config: config,
		logger: logger,
		results: &LoadTestResult{
			Errors: make([]error, 0),
		},
	}, nil
}

// SetSearchRouter sets the search router for testing
func (lt *LoadTester) SetSearchRouter(router *search.SearchRouter) {
	lt.searchRouter = router
}

// RunLoadTest executes the load test
func (lt *LoadTester) RunLoadTest(ctx context.Context) (*LoadTestResult, error) {
	if lt.searchRouter == nil {
		return nil, fmt.Errorf("search router not set")
	}

	lt.logger.Info("Starting load test", "duration", lt.config.Duration, "workers", lt.config.ConcurrentWorkers)

	// Create cancellable context for the test
	testCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start load test
	startTime := time.Now()
	errChan := make(chan error, lt.config.ConcurrentWorkers)
	queryTimesChan := make(chan time.Duration, 10000)

	var wg sync.WaitGroup
	for i := 0; i < lt.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go lt.worker(testCtx, &wg, errChan, queryTimesChan)
	}

	// Wait for test duration or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-time.After(lt.config.Duration):
		lt.logger.Info("Load test duration completed")
		cancel() // Cancel the test context to stop workers
	case <-ctx.Done():
		lt.logger.Info("Load test cancelled")
		cancel()
	}

	// Wait for workers to finish
	<-done

	// Collect results
	close(errChan)
	close(queryTimesChan)

	// Process errors
	for err := range errChan {
		if err != nil {
			lt.results.Errors = append(lt.results.Errors, err)
			lt.results.FailedQueries++
		}
	}

	// Process query times
	var queryTimes []time.Duration
	for qt := range queryTimesChan {
		queryTimes = append(queryTimes, qt)
		lt.results.TotalQueries++
		lt.results.SuccessfulQueries++
	}

	// Calculate statistics
	lt.results.TotalDuration = time.Since(startTime)
	if len(queryTimes) > 0 {
		lt.results.AvgQueryTime = calculateAverage(queryTimes)
		lt.results.P95QueryTime = calculatePercentile(queryTimes, 95)
		lt.results.P99QueryTime = calculatePercentile(queryTimes, 99)
		lt.results.QPS = float64(lt.results.TotalQueries) / lt.results.TotalDuration.Seconds()
	}

	lt.logger.Info("Load test completed",
		"total_queries", lt.results.TotalQueries,
		"successful_queries", lt.results.SuccessfulQueries,
		"failed_queries", lt.results.FailedQueries,
		"avg_query_time", lt.results.AvgQueryTime,
		"qps", lt.results.QPS)

	return lt.results, nil
}

// worker executes queries in a loop
func (lt *LoadTester) worker(ctx context.Context, wg *sync.WaitGroup, errChan chan<- error, queryTimesChan chan<- time.Duration) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Generate random query
			query := lt.selectRandomQuery()

			// Execute query
			start := time.Now()
			err := lt.executeQuery(ctx, query)
			duration := time.Since(start)

			// Send results (non-blocking, check if channels are still open)
			if err != nil {
				select {
				case errChan <- err:
				case <-ctx.Done():
					return
				default:
					// Channel might be closed, continue
				}
			} else {
				select {
				case queryTimesChan <- duration:
				case <-ctx.Done():
					return
				default:
					// Channel might be closed, continue
				}
			}

			// Small delay to prevent overwhelming
			time.Sleep(time.Millisecond * 10)
		}
	}
}

// selectRandomQuery selects a query based on weights
func (lt *LoadTester) selectRandomQuery() string {
	totalWeight := 0
	for _, pattern := range lt.config.QueryPatterns {
		totalWeight += pattern.Weight
	}

	r := rand.Intn(totalWeight)
	cumulative := 0

	for _, pattern := range lt.config.QueryPatterns {
		cumulative += pattern.Weight
		if r < cumulative {
			return pattern.Query
		}
	}

	// Fallback to first pattern
	return lt.config.QueryPatterns[0].Query
}

// executeQuery executes a search query
func (lt *LoadTester) executeQuery(ctx context.Context, query string) error {
	// Use search router for query translation (which includes caching)
	_, err := lt.searchRouter.TranslateToLogsQLCached(ctx, lt.config.Engine, query)
	return err
}

// Helper functions for statistics
func calculateAverage(times []time.Duration) time.Duration {
	if len(times) == 0 {
		return 0
	}

	var sum time.Duration
	for _, t := range times {
		sum += t
	}
	return sum / time.Duration(len(times))
}

func calculatePercentile(times []time.Duration, percentile float64) time.Duration {
	if len(times) == 0 {
		return 0
	}

	// Simple sort and pick
	sorted := make([]time.Duration, len(times))
	copy(sorted, times)

	// Basic sort (could use sort.Sort for better performance)
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
