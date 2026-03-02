# Query Performance Runbook

## Overview

This runbook provides guidance for diagnosing and resolving query performance issues in MIRADOR-CORE.

## Symptoms

- Slow query response times (>500ms for simple queries, >2s for complex queries)
- High CPU usage during queries (>80% sustained)
- Memory spikes during query execution
- Increasing query timeout errors in logs
- Degraded cache hit rates (<70%)

## Diagnosis

### 1. Check Query Execution Metrics

```bash
# Check unified query response times via Prometheus
curl -s http://localhost:9090/api/v1/query?query=histogram_quantile(0.95,rate(mirador_core_http_request_duration_seconds_bucket{endpoint=~"/api/v1/query.*"}[5m]))

# Check query operation counts and success rates
curl -s http://localhost:9090/api/v1/query?query=rate(mirador_core_http_requests_total{endpoint=~"/api/v1/query.*"}[5m])

# View cache hit rates
curl -s http://localhost:9090/api/v1/query?query=rate(mirador_core_cache_requests_total{result="hit"}[5m])/(rate(mirador_core_cache_requests_total{result="hit"}[5m])+rate(mirador_core_cache_requests_total{result="miss"}[5m]))
```

### 2. Review Query Complexity

- Check UQL query complexity (number of joins, filters, aggregations)
- Review time range spans (large time ranges are more expensive)
- Examine cardinality of filtered dimensions

### 3. Examine System Resources

```bash
# Check container resource usage
docker stats mirador-core

# Check Go runtime metrics
curl -s http://localhost:8010/metrics | grep go_

# Check connection pool metrics
curl -s http://localhost:8010/metrics | grep mirador_core_mariadb
```

### 4. Review Logs for Errors

```bash
# Check for slow query warnings
docker logs mirador-core 2>&1 | grep -i "slow\|timeout\|error"

# Check Weaviate query logs
docker logs localdev-weaviate-1 2>&1 | grep -i "slow\|timeout"
```

## Resolution

### Slow Query Response Times

1. **Enable query caching** - Ensure cache is properly configured:
   ```yaml
   cache:
     type: redis
     ttl: 300
     maxSize: 1000
   ```

2. **Reduce time window** - For analysis queries, limit to 1-hour windows when possible

3. **Add query filters** - Filter by specific services or namespaces to reduce data scanned

4. **Check backend latency**:
   ```bash
   # Test Weaviate latency
   curl -w "@curl-format.txt" -s http://localhost:8080/v1/.well-known/ready
   
   # Test VictoriaMetrics latency
   curl -w "@curl-format.txt" -s http://localhost:8428/api/v1/status/tsdb
   ```

### High CPU Usage

1. **Reduce concurrent queries** - Check `maxConcurrentQueries` in engine config
2. **Optimize correlation calculations** - Reduce `graphHops` or `maxWhys` if excessive
3. **Scale horizontally** - Add more mirador-core replicas behind load balancer

### Memory Spikes

1. **Limit result set sizes** - Set appropriate `limit` values in queries
2. **Enable streaming** - Use WebSocket streaming for large result sets
3. **Adjust Go runtime settings**:
   ```bash
   export GOGC=50  # More aggressive garbage collection
   ```

### Timeout Errors

1. **Increase timeout values** in engine config:
   ```yaml
   engine:
     queryTimeout: 30s
     correlationTimeout: 60s
   ```

2. **Check backend health** - Ensure Weaviate/VictoriaMetrics are responsive

3. **Add circuit breaker** - Enable circuit breaker for failing backends

## Prevention

### Best Practices for Query Optimization

1. **Use appropriate time windows**
   - Keep windows under 1 hour for real-time analysis
   - Use pre-aggregated data for longer time ranges

2. **Enable and monitor caching**
   - Target >80% cache hit rate for read-heavy workloads
   - Set appropriate TTLs based on data freshness requirements

3. **Implement query limits**
   - Set `maxResults` limits on all API endpoints
   - Use pagination for large result sets

4. **Monitor and alert**
   - Set alerts for p95 latency >1s
   - Alert on cache hit rate <70%
   - Alert on query error rate >5%

5. **Regular capacity planning**
   - Monitor query volume trends
   - Scale resources before reaching capacity limits

### Alerting Rules

```yaml
groups:
  - name: query-performance
    rules:
      - alert: SlowQueryP95
        expr: histogram_quantile(0.95, rate(mirador_core_http_request_duration_seconds_bucket{endpoint=~"/api/v1/query.*"}[5m])) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Query p95 latency is high"
          
      - alert: HighQueryErrorRate
        expr: rate(mirador_core_http_requests_total{endpoint=~"/api/v1/query.*",status_code!="200"}[5m]) / rate(mirador_core_http_requests_total{endpoint=~"/api/v1/query.*"}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Query error rate exceeds 5%"
```

## Related Documentation

- [Unified Query Architecture](unified-query-architecture.md)
- [Unified Query Operations](unified-query-operations.md)
- [Cache Performance Runbook](cache-performance-runbook.md)