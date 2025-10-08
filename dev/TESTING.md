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

