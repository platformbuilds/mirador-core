Testing strategy and commands

Categories

- Functional tests: default unit and handler tests that run with `go test ./...`.
- Performance tests: benchmarks using `testing.B` with `go test -bench=. ./...`.
- Integration tests: end-to-end-ish tests exercising real HTTP stacks via local mocks, guarded by the `integration` build tag.
- Database tests: live DB tests for Valkey/Redis, guarded by the `db` build tag and environment variables.

How to run

- All functional tests: `go test ./...`
- Benchmarks (performance): `go test -bench=. -run ^$ ./...`
- Integration tests: `go test -tags=integration ./...`
- Database tests (Valkey single): set env and run
  - `VALKEY_ADDR=127.0.0.1:6379 go test -tags=db ./pkg/cache`
- Database tests (Valkey cluster): set env and run
  - `VALKEY_NODES=127.0.0.1:7000,127.0.0.1:7001 go test -tags=db ./pkg/cache`

## Unified Query Language (UQL) Testing (v7.0.0)

### Performance Regression Tests
Automated performance regression tests ensure that unified query engine components maintain acceptable performance baselines:

```bash
# Run all performance regression tests
go test ./internal/services -run TestUQLPerformanceRegression -v

# Run UQL parsing performance tests
go test ./internal/services -run TestUQLPerformanceRegression/UQL_Parsing_Performance -v

# Run UQL optimization performance tests
go test ./internal/services -run TestUQLPerformanceRegression/UQL_Optimization_Performance -v

# Run query routing performance tests
go test ./internal/services -run TestUQLPerformanceRegression/Query_Routing_Performance -v

# Run correlation query performance tests
go test ./internal/services -run TestCorrelationQueryPerformanceRegression -v
```

**Performance Thresholds**:
- UQL Parsing: P95 latency ≤ 50ms, throughput ≥ 100 parses/sec
- UQL Optimization: P95 latency ≤ 100ms
- Query Routing: P95 latency ≤ 10ms
- Correlation Queries: P95 latency ≤ 50ms (same as UQL parsing)

**Test Coverage**:
- Representative query patterns from typical workloads
- Statistical analysis with 95th percentile latency calculations
- Throughput measurements for parsing operations
- Component-level testing (parser, optimizer, router) without full service mocking

### Unit Tests
Comprehensive unit tests for UQL components:

```bash
# Run UQL parser tests
go test ./internal/models -run TestUQLParser -v

# Run UQL optimizer tests
go test ./internal/services -run TestUQLOptimizer -v

# Run query router tests
go test ./internal/services -run TestQueryRouter -v

# Run correlation query parser tests
go test ./internal/models -run TestCorrelationQueryParser -v

# Run unified query engine tests
go test ./internal/services -run TestUnifiedQueryEngine -v
```

**Coverage**: Complete test coverage for:
- UQL syntax parsing and AST construction
- Query optimization rules and transformations
- Query routing logic for different data sources
- Correlation query parsing and validation
- Error handling for malformed queries

### Integration Tests
End-to-end testing for unified query functionality:

```bash
# Run unified query integration tests
go test -tags=integration ./internal/services -run TestUnifiedQueryEngine_Integration -v

# Run cross-component integration tests
go test -tags=integration ./internal/services -run TestUnifiedQueryPipeline -v
```

**Features Tested**:
- Full query pipeline from parsing to execution
- Multi-source query routing (logs, metrics, traces)
- Correlation query execution across data sources
- Error propagation and handling
- Performance validation in integrated scenarios

### Test Architecture
- **Component Isolation**: Tests focus on individual components (parser, optimizer, router) for reliable performance measurement
- **Statistical Validation**: Performance tests use statistical analysis to detect regressions
- **Query Diversity**: Tests cover various query patterns and complexity levels
- **CI/CD Integration**: Performance tests run in CI pipeline to catch regressions early

### Query Examples Tested
- Simple selections: `SELECT service, level FROM logs:error WHERE level='error'`
- Complex filters: `SELECT * FROM logs:error WHERE level='error' AND service='api'`
- Aggregations: `COUNT(*) FROM logs:error`, `SUM(bytes) FROM logs:error WHERE status_code >= 500`
- Correlation queries: `logs:error WITHIN 5m OF metrics:cpu_usage > 80`
- Multi-source queries: `logs:error AND metrics:cpu_usage > 80`

### Unit Tests
Comprehensive unit tests for search engine integration:

```bash
# Run all search-related unit tests
go test ./internal/utils/search -v

# Run Bleve translator tests
go test ./internal/utils/bleve -v

# Run search router tests
go test ./internal/utils/search -run TestSearchRouter -v

# Run API handler integration tests
go test ./internal/api/handlers -run TestLogsQLHandler_EngineSelection -v
go test ./internal/api/handlers -run TestTracesHandler_EngineSelection -v
```

**Coverage**: Complete test coverage for:
- Search router engine selection logic
- Bleve translator query parsing and conversion
- API handler engine parameter validation
- Backward compatibility with Lucene queries

### Integration Tests
End-to-end testing for dual search engine functionality:

```bash
# Run search integration tests
go test -tags=integration ./internal/api/handlers -run TestSearchEngineIntegration -v

# Run cross-engine comparison tests
go test -tags=integration ./internal/api/handlers -run TestEngineComparison -v
```

**Features Tested**:
- Full HTTP request/response cycle with search engine selection
- Query translation accuracy between Lucene and Bleve syntax
- VictoriaMetrics backend integration for both engines
- Error handling for unsupported query types
- Performance comparison between engines

### Search Engine Test Architecture
- **Mock VictoriaMetrics Server**: HTTP server that simulates VictoriaMetrics LogsQL/Traces API responses
- **Dual Engine Testing**: Parallel test execution for both Lucene and Bleve engines
- **Query Translation Validation**: Ensures translated queries produce equivalent results
- **Performance Benchmarks**: Latency and throughput comparisons between engines

### Backward Compatibility Tests
```bash
# Run regression tests for Lucene functionality
go test ./internal/api/handlers -run TestLuceneBackwardCompatibility -v

# Run migration tests (no engine specified defaults to Lucene)
go test ./internal/api/handlers -run TestDefaultEngineBehavior -v
```

## MetricsQL API Testing (v5.0.0)

### Unit Tests
Comprehensive unit tests for all MetricsQL aggregate functions:

```bash
# Run all MetricsQL handler unit tests
go test ./internal/api/handlers -v -run TestMetricsQLQueryHandler

# Run specific aggregate function tests
go test ./internal/api/handlers -v -run TestMetricsQLQueryHandler_Routing
go test ./internal/api/handlers -v -run TestMetricsQLQueryHandler_ParameterValidation
```

**Coverage**: 35 test cases covering all 33 aggregate functions, parameter validation, and error scenarios.

### Integration Tests
End-to-end testing with mocked VictoriaMetrics backend:

```bash
# Run MetricsQL integration tests
go test -tags=integration ./internal/api/handlers -v -run TestMetricsQLQueryHandler_Integration

# Run all integration tests
go test -tags=integration ./...
```

**Features Tested**:
- Full HTTP request/response cycle with middleware
- POST requests with JSON bodies
- VictoriaMetrics query construction and execution
- Response parsing and validation
- Error handling and edge cases

### Test Architecture
- **Mock VictoriaMetrics Server**: HTTP server that simulates VictoriaMetrics API responses
- **Function-Specific Responses**: Different mock responses for each aggregate function type
- **Middleware Integration**: Tests include validation middleware in the request pipeline
- **Parameter Handling**: Tests for functions requiring additional parameters (quantile, k values, etc.)

### Performance Testing
Load testing for MetricsQL aggregate functions:

```bash
# Run performance benchmarks
go test -bench=BenchmarkMetricsQL -benchmem ./internal/api/handlers

# Profile CPU usage
go test -bench=BenchmarkMetricsQL -cpuprofile=cpu.prof ./internal/api/handlers
go tool pprof cpu.prof
```

### Test Data
- Mock time series data representing HTTP request rates, CPU usage, memory consumption
- Realistic metric names and label combinations
- Various data distributions to test statistical functions
- Edge cases: empty results, single data points, large datasets

Notes

- Generated protobuf code and command binaries are not targeted for high unit coverage.
- External services (VictoriaMetrics, gRPC engines) are exercised via mocked HTTP servers and optional db-tagged tests.
- MetricsQL API tests ensure 100% endpoint coverage with comprehensive validation.

