# Unified Query Engine Load Testing Guide

## Overview

This guide describes how to perform load testing on the Unified Query Engine to evaluate performance, throughput, latency, and reliability under various workload conditions.

## Load Test Tool

The `unified_query_loadtest.go` tool provides comprehensive load testing capabilities for the Unified Query Engine, supporting all query types (metrics, logs, traces, correlation) with realistic workload patterns.

### Location

```
tools/unified_query_loadtest.go
```

### Features

- **Multi-Engine Testing**: Test metrics, logs, traces, and correlation queries
- **Concurrent Users**: Simulate multiple concurrent users executing queries
- **Configurable Load**: Adjust query rate, duration, and query mix
- **Performance Metrics**: Track latency (min, max, avg, P95, P99), throughput, and error rates
- **Query Type Statistics**: Detailed breakdowns per query type
- **JSON Results Export**: Export results for analysis and reporting
- **Real-time Progress**: Monitor test progress with periodic metrics

## Building the Load Test Tool

```bash
# Build the load test binary
go build -o bin/unified_loadtest tools/unified_query_loadtest.go

# Or use the Makefile (if available)
make build-loadtest
```

## Running Load Tests

### Basic Usage

```bash
# Run with default settings (60s duration, 10 users, 100 QPS)
./bin/unified_loadtest

# Run with custom parameters
./bin/unified_loadtest \
  -duration 5m \
  -users 50 \
  -rate 500 \
  -types metrics,logs,traces \
  -output results/loadtest-$(date +%Y%m%d-%H%M%S).json
```

### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-duration` | `60s` | Duration of the load test (e.g., 1m, 5m, 1h) |
| `-users` | `10` | Number of concurrent users |
| `-rate` | `100` | Total query rate per second |
| `-types` | `metrics,logs,traces` | Comma-separated query types to test |
| `-cache` | `true` | Enable caching for queries |
| `-report-interval` | `10s` | Interval for progress reporting |
| `-output` | `unified-loadtest-results.json` | Output file for results |
| `-target` | `unified` | Target engine (unified, metrics, logs, traces) |

### Query Types

The load test supports the following query types:

1. **metrics**: Time-series metric queries
   - Example: `cpu_usage`, `memory_usage`, `error_rate`
   
2. **logs**: Log search queries
   - Example: `error`, `level:error`, `error AND exception`
   
3. **traces**: Distributed trace queries
   - Example: `service:api`, `operation:GET`, `duration>100`
   
4. **correlation**: Cross-engine correlation queries
   - Example: `logs:error WITHIN 5m OF metrics:cpu_usage > 80`
   
5. **uql**: Unified Query Language queries (routed to appropriate engine)
   - Example: `SELECT cpu_usage FROM metrics WHERE time > now() - 1h`

## Load Test Scenarios

### 1. Baseline Performance Test

Test basic query performance with moderate load:

```bash
./bin/unified_loadtest \
  -duration 5m \
  -users 10 \
  -rate 100 \
  -types metrics,logs,traces \
  -output baseline-test.json
```

**Expected Results:**
- P95 latency < 100ms
- P99 latency < 500ms
- Error rate < 1%
- Throughput ≥ 100 QPS

### 2. High Throughput Test

Test system under high query load:

```bash
./bin/unified_loadtest \
  -duration 10m \
  -users 50 \
  -rate 1000 \
  -types metrics,logs,traces \
  -output high-throughput-test.json
```

**Expected Results:**
- P95 latency < 200ms
- P99 latency < 1s
- Error rate < 5%
- Throughput ≥ 900 QPS

### 3. Correlation Query Test

Test correlation engine performance:

```bash
./bin/unified_loadtest \
  -duration 5m \
  -users 20 \
  -rate 200 \
  -types correlation \
  -output correlation-test.json
```

**Expected Results:**
- P95 latency < 500ms (correlation queries are more expensive)
- P99 latency < 2s
- Error rate < 2%
- Successful correlation ratio ≥ 95%

### 4. Mixed Workload Test

Test realistic mixed query patterns:

```bash
./bin/unified_loadtest \
  -duration 15m \
  -users 30 \
  -rate 500 \
  -types metrics,logs,traces,correlation \
  -output mixed-workload-test.json
```

**Expected Results:**
- P95 latency < 150ms (overall)
- P99 latency < 800ms (overall)
- Error rate < 2%
- All query types performing within SLAs

### 5. Stress Test

Test system limits and failure modes:

```bash
./bin/unified_loadtest \
  -duration 5m \
  -users 100 \
  -rate 2000 \
  -types metrics,logs,traces,correlation \
  -output stress-test.json
```

**Objectives:**
- Identify breaking point (max QPS before degradation)
- Verify graceful degradation under overload
- Confirm no data loss or corruption
- Test circuit breaker and backpressure mechanisms

### 6. Endurance Test

Test long-running stability:

```bash
./bin/unified_loadtest \
  -duration 1h \
  -users 20 \
  -rate 300 \
  -types metrics,logs,traces \
  -output endurance-test.json
```

**Objectives:**
- Verify memory stability (no leaks)
- Confirm cache effectiveness over time
- Check for performance degradation
- Monitor resource utilization trends

### 7. Cache Performance Test

Test with and without caching:

```bash
# With cache
./bin/unified_loadtest \
  -duration 5m \
  -users 10 \
  -rate 200 \
  -cache=true \
  -output cache-enabled-test.json

# Without cache
./bin/unified_loadtest \
  -duration 5m \
  -users 10 \
  -rate 200 \
  -cache=false \
  -output cache-disabled-test.json
```

**Expected Results:**
- Cache hit rate > 70%
- 50-80% latency reduction with cache
- 5-10x throughput improvement for cached queries

## Analyzing Results

### Result Structure

The JSON output contains:

```json
{
  "TotalQueries": 30000,
  "SuccessfulQueries": 29850,
  "FailedQueries": 150,
  "ErrorRate": 0.5,
  "QueriesPerSecond": 500.0,
  "AverageLatency": "45ms",
  "MinLatency": "5ms",
  "MaxLatency": "850ms",
  "P95Latency": "120ms",
  "P99Latency": "350ms",
  "LatencyDistribution": {
    "<10ms": 5000,
    "<50ms": 15000,
    "<100ms": 7500,
    "<500ms": 2250,
    "<1s": 200,
    ">5s": 50
  },
  "QueryTypeStats": {
    "metrics": {
      "TotalQueries": 10000,
      "SuccessfulQueries": 9950,
      "AverageLatency": "35ms"
    },
    "logs": {
      "TotalQueries": 10000,
      "SuccessfulQueries": 9900,
      "AverageLatency": "55ms"
    }
  }
}
```

### Key Metrics

1. **Throughput (QPS)**
   - Actual queries per second achieved
   - Compare to target rate to identify bottlenecks

2. **Latency Percentiles**
   - P50 (median): Typical user experience
   - P95: Experience for 95% of queries
   - P99: Worst-case for most queries
   - Max: Absolute worst-case

3. **Error Rate**
   - Percentage of failed queries
   - Should be < 1% for production workloads
   - Higher rates indicate capacity issues

4. **Latency Distribution**
   - Shows query distribution across latency buckets
   - Helps identify if latency is consistent or variable

5. **Per-Engine Metrics**
   - Performance breakdown by query type
   - Identifies underperforming engines

### Performance Targets

| Metric | Target | Acceptable | Poor |
|--------|--------|------------|------|
| P95 Latency (metrics) | < 50ms | < 100ms | > 200ms |
| P95 Latency (logs) | < 100ms | < 200ms | > 500ms |
| P95 Latency (traces) | < 150ms | < 300ms | > 1s |
| P95 Latency (correlation) | < 500ms | < 1s | > 2s |
| Error Rate | < 0.1% | < 1% | > 5% |
| Throughput | ≥ Target Rate | ≥ 90% of Target | < 80% of Target |

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Load Tests

on:
  schedule:
    - cron: '0 2 * * *' # Daily at 2 AM
  workflow_dispatch:

jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build Load Test
        run: go build -o bin/unified_loadtest tools/unified_query_loadtest.go
      
      - name: Run Baseline Load Test
        run: |
          ./bin/unified_loadtest \
            -duration 5m \
            -users 10 \
            -rate 100 \
            -output loadtest-results.json
      
      - name: Analyze Results
        run: |
          # Parse results and fail if metrics don't meet targets
          P95=$(jq -r '.P95Latency' loadtest-results.json)
          ERROR_RATE=$(jq -r '.ErrorRate' loadtest-results.json)
          
          # Fail if P95 > 200ms or error rate > 1%
          if [[ "$P95" > "200ms" ]] || [[ "$ERROR_RATE" > "1.0" ]]; then
            echo "Load test failed to meet performance targets"
            exit 1
          fi
      
      - name: Upload Results
        uses: actions/upload-artifact@v3
        with:
          name: load-test-results
          path: loadtest-results.json
```

## Monitoring During Load Tests

### Key Observability Metrics

1. **System Metrics**
   - CPU utilization per service
   - Memory usage and GC frequency
   - Disk I/O and network throughput
   - Connection pool utilization

2. **Application Metrics**
   - Query execution time by engine
   - Cache hit/miss ratio
   - Query queue depth
   - Active goroutines

3. **Database Metrics**
   - VictoriaMetrics query latency
   - VictoriaLogs query latency
   - VictoriaTraces query latency
   - Database connection count

## Troubleshooting

### High Latency

**Symptoms**: P95/P99 latency exceeds targets

**Possible Causes**:
1. Database overload
2. Network latency
3. Inefficient queries
4. Resource contention

**Investigation**:
```bash
# Check per-engine latency
jq '.QueryTypeStats' loadtest-results.json

# Profile the application during load test
go tool pprof http://localhost:6060/debug/pprof/profile

# Check database query performance
# (use VictoriaMetrics/Logs/Traces query analysis tools)
```

### High Error Rate

**Symptoms**: Error rate > 1%

**Possible Causes**:
1. Service capacity exceeded
2. Database connection pool exhausted
3. Timeouts too aggressive
4. Network issues

**Investigation**:
```bash
# Check error types in logs
grep ERROR /var/log/mirador-core.log | tail -100

# Check connection pool metrics
curl http://localhost:8080/metrics | grep pool

# Verify database health
curl http://victoria-metrics:8428/api/v1/status/tsdb
```

### Low Throughput

**Symptoms**: Actual QPS significantly below target

**Possible Causes**:
1. CPU saturation
2. Lock contention
3. Slow queries
4. Network bandwidth limits

**Investigation**:
```bash
# Profile CPU usage
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Check goroutine count
curl http://localhost:6060/debug/pprof/goroutine?debug=2

# Analyze query patterns
jq '.QueryTypeStats | to_entries[] | {type: .key, avg_latency: .value.AverageLatency}' loadtest-results.json
```

## Best Practices

1. **Ramp Up Gradually**
   - Start with low load and increase incrementally
   - Allows identification of scaling boundaries

2. **Test Realistic Workloads**
   - Match production query patterns
   - Use representative query complexity

3. **Monitor System Resources**
   - CPU, memory, network, disk I/O
   - Identify resource bottlenecks early

4. **Test with Cold and Warm Cache**
   - Understand cache impact on performance
   - Plan for cache invalidation scenarios

5. **Validate Results**
   - Compare against baseline metrics
   - Look for performance regressions

6. **Document Findings**
   - Record capacity limits
   - Note optimal configuration parameters
   - Track performance over time

## Performance Optimization Tips

Based on load test results, consider these optimizations:

1. **Query Optimization**
   - Add appropriate indices
   - Optimize correlation query patterns
   - Use query result caching effectively

2. **Resource Scaling**
   - Scale horizontally (add replicas)
   - Scale vertically (increase resources)
   - Optimize connection pools

3. **Caching Strategy**
   - Tune cache TTLs
   - Implement query result caching
   - Use cache warming for common queries

4. **Parallel Execution**
   - Enable parallel query execution
   - Optimize goroutine pool sizes
   - Balance parallelism vs resource usage

## Conclusion

Regular load testing ensures the Unified Query Engine meets performance requirements and scales appropriately. Use this guide to establish baseline performance, identify optimization opportunities, and validate system capacity before production deployment.

For questions or issues, consult:
- [Query Performance Runbook](query-performance-runbook.md)
- [Monitoring & Observability](monitoring-observability.md)
- [Unified Query Architecture](unified-query-architecture.md)
