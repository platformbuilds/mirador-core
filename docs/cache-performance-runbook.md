# Cache Performance Runbook

## Overview

This runbook provides guidance for diagnosing and resolving cache performance issues in MIRADOR-CORE.

## Symptoms

- Low cache hit rates (<70%)
- Slow cache operations (>50ms for get operations)
- Memory issues related to caching
- Increased latency despite cache being enabled
- Cache eviction storms

## Diagnosis

### 1. Check Cache Hit/Miss Ratios

```bash
# Check overall cache hit rate via Prometheus
curl -s http://localhost:9090/api/v1/query?query=rate(mirador_core_cache_requests_total{result="hit"}[5m])/(rate(mirador_core_cache_requests_total{result="hit"}[5m])+rate(mirador_core_cache_requests_total{result="miss"}[5m]))

# Check cache operations by type
curl -s http://localhost:9090/api/v1/query?query=rate(mirador_core_cache_requests_total[5m])

# View cache operation latency (p95)
curl -s http://localhost:9090/api/v1/query?query=histogram_quantile(0.95,rate(mirador_core_cache_request_duration_seconds_bucket[5m]))
```

### 2. Review Cache Configuration

```bash
# Check current cache settings in config
grep -A 10 "cache:" configs/config.yaml

# Verify Redis/Valkey connectivity
redis-cli -h localhost -p 6379 ping

# Check Redis memory usage
redis-cli -h localhost -p 6379 INFO memory
```

### 3. Examine Cache Storage Usage

```bash
# Check Redis key count
redis-cli -h localhost -p 6379 DBSIZE

# Check memory usage by prefix
redis-cli -h localhost -p 6379 --scan --pattern "mirador:*" | wc -l

# Check TTL distribution
redis-cli -h localhost -p 6379 DEBUG OBJECT mirador:kpi:*
```

### 4. Check for Cache Errors

```bash
# Check for cache errors in logs
docker logs mirador-core 2>&1 | grep -i "cache\|redis\|valkey" | grep -i "error\|fail\|timeout"

# Check Redis slow log
redis-cli -h localhost -p 6379 SLOWLOG GET 10
```

## Resolution

### Low Cache Hit Rates

1. **Analyze query patterns** - Identify if queries are cacheable:
   ```bash
   # Check for unique query variations
   docker logs mirador-core 2>&1 | grep "query" | awk '{print $NF}' | sort | uniq -c | sort -rn
   ```

2. **Increase TTL** for stable data:
   ```yaml
   cache:
     ttl: 600  # Increase from default 300
   ```

3. **Review cache key strategy** - Ensure keys are normalized (timestamps rounded, etc.)

4. **Enable cache warming** - Pre-populate cache for common queries on startup

### Slow Cache Operations

1. **Check network latency** to Redis:
   ```bash
   redis-cli -h localhost -p 6379 --latency
   ```

2. **Enable pipelining** for batch operations

3. **Check for large values**:
   ```bash
   redis-cli -h localhost -p 6379 DEBUG OBJECT <key>
   ```

4. **Scale Redis** - Add read replicas or use Redis Cluster

### Memory Issues

1. **Set maxmemory policy**:
   ```conf
   maxmemory 2gb
   maxmemory-policy allkeys-lru
   ```

2. **Reduce TTL** for less important data

3. **Enable compression** for large values

4. **Implement tiered caching** - Use local memory cache for hot data

### High Eviction Rates

1. **Increase Redis memory**:
   ```bash
   redis-cli -h localhost -p 6379 CONFIG SET maxmemory 4gb
   ```

2. **Review TTLs** - Set shorter TTLs for low-value cache entries

3. **Add more cache nodes** for horizontal scaling

## Prevention

### Best Practices for Cache Optimization

1. **Design cacheable queries**
   - Normalize timestamps to bucket boundaries (5min, 15min)
   - Use consistent parameter ordering
   - Avoid highly variable parameters in cache keys

2. **Set appropriate TTLs**
   - Real-time data: 30-60 seconds
   - Aggregated data: 5-15 minutes
   - Historical data: 1-24 hours
   - Static metadata: 1-7 days

3. **Monitor cache effectiveness**
   - Target >80% hit rate for read-heavy workloads
   - Alert on hit rate drops >10% in 15 minutes
   - Track cache memory usage trends

4. **Implement cache-aside pattern**
   - Check cache first
   - On miss, fetch from source and populate cache
   - Use atomic operations to prevent thundering herd

5. **Configure eviction policies**
   ```conf
   # Recommended for query caching
   maxmemory-policy volatile-lru
   
   # For mixed workloads
   maxmemory-policy allkeys-lru
   ```

### Alerting Rules

```yaml
groups:
  - name: cache-performance
    rules:
      - alert: CacheHitRateLow
        expr: |
          (rate(mirador_core_cache_requests_total{result="hit"}[5m]) / 
           (rate(mirador_core_cache_requests_total{result="hit"}[5m]) + 
            rate(mirador_core_cache_requests_total{result="miss"}[5m]))) < 0.7
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Cache hit rate below 70%"
          
      - alert: CacheLatencyHigh
        expr: histogram_quantile(0.95, rate(mirador_core_cache_request_duration_seconds_bucket[5m])) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Cache p95 latency exceeds 50ms"
          
      - alert: CacheErrorRate
        expr: rate(mirador_core_cache_requests_total{result="error"}[5m]) / rate(mirador_core_cache_requests_total[5m]) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Cache error rate exceeds 1%"
```

## Related Documentation

- [Query Performance Runbook](query-performance-runbook.md)
- [Unified Query Operations](unified-query-operations.md)
- [Monitoring and Observability](monitoring-observability.md)