# Comprehensive Integration Test Framework

## Overview

The Comprehensive Integration Test Framework provides a robust testing infrastructure for the Unified Query Engine, enabling thorough testing of query routing, cross-engine coordination, error handling, caching, and performance characteristics.

## Architecture

### Framework Components

1. **IntegrationTestFramework**: Main orchestrator for integration tests
2. **TestDataRepository**: Centralized test data management
3. **Mock Services**: Full-featured mocks for all observability engines
4. **Test Helpers**: Utility functions for common testing patterns
5. **Assertion Helpers**: Specialized assertions for unified query validation

### File Structure

```
internal/services/
├── test_helpers.go                          # Core framework and utilities
├── unified_query_engine_comprehensive_test.go  # Comprehensive integration tests
├── unified_query_engine_integration_test.go    # Basic integration tests
└── correlation_engine_test.go               # Correlation-specific tests
```

## Test Categories

### 1. Comprehensive Integration Tests

Located in `unified_query_engine_comprehensive_test.go`

**TestUnifiedQueryEngine_ComprehensiveIntegration**
- Tests full query lifecycle across all engines
- Validates metrics, logs, and traces query execution
- Ensures proper error handling with unavailable services

**TestUnifiedQueryEngine_CrossEngineDataConsistency**
- Validates timestamp synchronization across engines
- Tests multi-engine query coordination
- Ensures consistent data formats across all engines

**TestUnifiedQueryEngine_ConcurrentQueries**
- Tests parallel query execution
- Validates thread safety
- Tests mixed query type concurrency

**TestUnifiedQueryEngine_CachingBehavior**
- Tests cache miss/hit behavior
- Validates cache invalidation
- Tests multi-pattern cache management

**TestUnifiedQueryEngine_ErrorRecovery**
- Tests service unavailability recovery
- Validates context cancellation handling
- Tests timeout behavior

**TestUnifiedQueryEngine_QueryMetadata**
- Tests supported engines discovery
- Validates query capabilities reporting
- Tests cache capabilities metadata

**TestUnifiedQueryEngine_HealthChecks**
- Tests health check with unavailable services
- Validates health check performance (< 10ms)
- Tests engine-specific health status

### 2. Test Helper Functions

Located in `test_helpers.go`

#### Framework Setup

```go
// Create new integration test framework
framework := NewIntegrationTestFramework()
framework.Setup()
defer framework.TearDown()
```

#### Mock Data Configuration

```go
// Setup metrics test data
metricsResult := GenerateMetricsResult("cpu_usage", 85.5, time.Now())
framework.SetupMetricsData("cpu_usage", metricsResult)

// Setup logs test data
logsResult := GenerateLogsResult("error", "Connection failed", time.Now())
framework.SetupLogsData("error", logsResult)

// Setup traces test data
traces := GenerateTracesResult("trace-123")
framework.SetupTracesData("api-service", traces)

// Setup Bleve search data
framework.SetupBleveData("error exception", searchResults)
```

#### Test Data Generators

```go
// Generate metrics test data
result := GenerateMetricsResult(metricName string, value float64, timestamp time.Time)

// Generate logs test data
result := GenerateLogsResult(level, message string, timestamp time.Time)

// Generate traces test data
result := GenerateTracesResult(traceID string)

// Create test query
query := CreateTestUnifiedQuery(id, queryType, query string, timeRange time.Duration)
```

#### Assertion Helpers

```go
// Assert successful query
framework.AssertQuerySuccess(t, result, models.QueryTypeMetrics)

// Assert query error
framework.AssertQueryError(t, err, "expected error substring")

// Assert correlation success
framework.AssertCorrelationSuccess(t, result, 0.8) // min confidence

// Assert engine health
framework.AssertEngineHealth(t, health, models.QueryTypeMetrics, "healthy")
```

#### Utility Functions

```go
// Wait for condition with timeout
success := WaitForCondition(
    func() bool { return someCondition },
    timeout time.Duration,
    interval time.Duration,
)

// Compare query results
equal := CompareQueryResults(result1, result2)

// Create test context with timeout
ctx, cancel := CreateTestContext(30 * time.Second)
defer cancel()
```

## Running Tests

### Run All Comprehensive Tests

```bash
# Run all integration tests
go test -v ./internal/services -run Comprehensive

# Run with race detection
go test -v -race ./internal/services -run Comprehensive

# Run with coverage
go test -v -cover ./internal/services -run Comprehensive
```

### Run Specific Test Suites

```bash
# Run comprehensive integration tests
go test -v ./internal/services -run TestUnifiedQueryEngine_ComprehensiveIntegration

# Run cross-engine data consistency tests
go test -v ./internal/services -run TestUnifiedQueryEngine_CrossEngineDataConsistency

# Run concurrent queries tests
go test -v ./internal/services -run TestUnifiedQueryEngine_ConcurrentQueries

# Run caching behavior tests
go test -v ./internal/services -run TestUnifiedQueryEngine_CachingBehavior

# Run error recovery tests
go test -v ./internal/services -run TestUnifiedQueryEngine_ErrorRecovery

# Run health check tests
go test -v ./internal/services -run TestUnifiedQueryEngine_HealthChecks

# Run test helpers validation
go test -v ./internal/services -run TestTestHelpers
```

### Skip Integration Tests (Short Mode)

```bash
# Skip all integration tests
go test -short ./internal/services
```

## Performance Benchmarks

### Run Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./internal/services

# Run specific benchmark
go test -bench=BenchmarkUnifiedQueryEngine_ExecuteQuery ./internal/services

# Run benchmarks with memory profiling
go test -bench=. -benchmem ./internal/services

# Run benchmarks for specific duration
go test -bench=. -benchtime=10s ./internal/services
```

### Available Benchmarks

1. **BenchmarkUnifiedQueryEngine_ExecuteQuery**
   - Measures query execution performance
   - Target: < 10ms for simple queries

2. **BenchmarkUnifiedQueryEngine_HealthCheck**
   - Measures health check performance
   - Target: < 1ms

3. **BenchmarkUnifiedQueryEngine_GetQueryMetadata**
   - Measures metadata retrieval performance
   - Target: < 500μs

## Writing New Tests

### Basic Test Structure

```go
func TestMyFeature(t *testing.T) {
    // Skip in short mode for integration tests
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Create framework
    framework := NewIntegrationTestFramework()
    framework.Setup()
    defer framework.TearDown()

    // Setup test data
    // ... configure mocks and test data

    // Create engine components
    log := logger.New("info")
    cache := cache.NewNoopValkeyCache(log)

    // Run test cases
    t.Run("SubTest", func(t *testing.T) {
        // Test implementation
    })
}
```

### Testing with Mocks

```go
func TestWithMocks(t *testing.T) {
    framework := NewIntegrationTestFramework()
    framework.Setup()
    defer framework.TearDown()

    // Configure mock expectations
    framework.MetricsService.On("ExecuteQuery", mock.Anything, mock.Anything).
        Return(&models.MetricsQLQueryResult{Status: "success"}, nil)

    // Configure multiple calls
    framework.LogsService.On("ExecuteQuery", mock.Anything, mock.Anything).
        Return(&models.LogsQLQueryResult{}, nil).Times(3)

    // Test implementation
    // ...

    // Verify expectations (if needed)
    framework.MetricsService.AssertExpectations(t)
}
```

### Testing Concurrent Operations

```go
func TestConcurrency(t *testing.T) {
    engine := createTestEngine()

    // Use channels for synchronization
    results := make(chan error, numGoroutines)

    // Launch concurrent operations
    for i := 0; i < numGoroutines; i++ {
        go func(id int) {
            // Perform operation
            result, err := engine.ExecuteQuery(ctx, query)
            results <- err
        }(i)
    }

    // Collect and verify results
    for i := 0; i < numGoroutines; i++ {
        err := <-results
        assert.NoError(t, err)
    }
}
```

### Testing Error Conditions

```go
func TestErrorHandling(t *testing.T) {
    testCases := []struct {
        name          string
        query         *models.UnifiedQuery
        expectedError string
    }{
        {
            name: "Empty query",
            query: &models.UnifiedQuery{Query: ""},
            expectedError: "empty query",
        },
        // More test cases...
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := engine.ExecuteQuery(ctx, tc.query)
            assert.Error(t, err)
            assert.Contains(t, err.Error(), tc.expectedError)
        })
    }
}
```

## Best Practices

### 1. Test Isolation

- Each test should be independent
- Use `Setup()` and `TearDown()` for proper cleanup
- Avoid shared state between tests
- Use separate mock instances per test

### 2. Test Data Management

- Use test data generators for consistent data
- Keep test data simple and focused
- Use realistic timestamps and values
- Document special test data requirements

### 3. Assertions

- Use descriptive error messages
- Verify both success and failure cases
- Test edge cases and boundaries
- Use table-driven tests for multiple scenarios

### 4. Performance Testing

- Set realistic performance targets
- Test under various load conditions
- Monitor resource usage (memory, CPU)
- Use benchmarks for performance regression detection

### 5. Mocking

- Mock only external dependencies
- Keep mocks simple and focused
- Verify mock expectations when needed
- Don't over-mock – test real code paths when possible

### 6. Error Testing

- Test all error paths
- Verify error messages
- Test error recovery
- Test cascading failures

### 7. Concurrent Testing

- Use race detector (`-race` flag)
- Test with various concurrency levels
- Verify thread safety
- Test deadlock scenarios

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run Integration Tests
        run: |
          go test -v -race -cover ./internal/services -run Comprehensive
      
      - name: Run Benchmarks
        run: |
          go test -bench=. -benchmem ./internal/services
```

## Troubleshooting

### Common Issues

1. **Test Timeout**
   ```bash
   # Increase timeout
   go test -timeout 5m ./internal/services
   ```

2. **Race Conditions**
   ```bash
   # Enable race detector
   go test -race ./internal/services
   ```

3. **Memory Issues**
   ```bash
   # Profile memory usage
   go test -memprofile=mem.prof ./internal/services
   go tool pprof mem.prof
   ```

4. **Flaky Tests**
   - Add proper synchronization
   - Increase wait timeouts
   - Check for shared state
   - Use WaitForCondition helper

### Debug Tips

1. **Enable Verbose Logging**
   ```go
   log := logger.New("debug")
   ```

2. **Print Test State**
   ```go
   t.Logf("Current state: %+v", testObject)
   ```

3. **Use Test Fixtures**
   ```go
   // Create reusable test fixtures
   func setupTestEngine() UnifiedQueryEngine {
       // Setup code
   }
   ```

4. **Isolate Failures**
   ```bash
   # Run single test
   go test -v -run TestUnifiedQueryEngine_Comprehensive/Metrics_Query
   ```

## Performance Targets

| Test Category | Target | Acceptable | Poor |
|--------------|--------|------------|------|
| Query Execution (simple) | < 10ms | < 50ms | > 100ms |
| Health Check | < 1ms | < 5ms | > 10ms |
| Metadata Retrieval | < 500μs | < 2ms | > 5ms |
| Concurrent Queries (10) | < 100ms | < 500ms | > 1s |
| Cache Operations | < 1ms | < 5ms | > 10ms |

## Future Enhancements

1. **End-to-End Tests**: Add tests with real VictoriaMetrics/Logs/Traces instances
2. **Chaos Testing**: Add failure injection and recovery testing
3. **Load Testing Integration**: Integrate with load testing framework
4. **Contract Testing**: Add API contract validation
5. **Mutation Testing**: Add mutation testing for test quality validation

## References

- [Unified Query Architecture](../docs/unified-query-architecture.md)
- [Load Testing Guide](../docs/load-testing-guide.md)
- [Query Performance Runbook](../docs/query-performance-runbook.md)
- [Testing Documentation](../docs/testing.md)

---

**Last Updated**: 2025-11-06
**Framework Version**: 1.0.0
**Maintainer**: Mirador Core Team
