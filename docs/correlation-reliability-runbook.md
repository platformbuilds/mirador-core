# Correlation Reliability Runbook

## Overview

This runbook provides guidance for diagnosing and resolving correlation engine reliability issues in MIRADOR-CORE.

## Symptoms

- Correlation queries failing (HTTP 500, timeout errors)
- Inconsistent correlation results across queries
- Correlation engine timeouts (>60s response times)
- Empty or incomplete correlation results
- High memory usage during correlation processing

## Diagnosis

### 1. Check Correlation Engine Health Metrics

```bash
# Check correlation engine operation counts
curl -s http://localhost:9090/api/v1/query?query=rate(mirador_core_correlation_engine_queries_total[5m])

# Check correlation query latency (p95)
curl -s http://localhost:9090/api/v1/query?query=histogram_quantile(0.95,rate(mirador_core_correlation_engine_query_duration_seconds_bucket[5m]))

# Check correlation error rate
curl -s http://localhost:9090/api/v1/query?query=rate(mirador_core_correlation_engine_queries_total{status="error"}[5m])

# Check correlation engine health endpoint
curl -s http://localhost:8010/health
```

### 2. Review Correlation Query Syntax

- Verify time range is valid (`endTime > startTime`)
- Check time window is within limits (≤1 hour for real-time, ≤24h for historical)
- Ensure required parameters are present

```bash
# Test a simple correlation query
curl -X POST http://localhost:8010/api/v1/unified/correlate \
  -H "Content-Type: application/json" \
  -d '{"startTime":"2025-01-01T00:00:00Z","endTime":"2025-01-01T01:00:00Z"}'
```

### 3. Examine Backend Service Connectivity

```bash
# Check Weaviate connectivity
curl -s http://localhost:8080/v1/.well-known/ready

# Check VictoriaMetrics connectivity
curl -s http://localhost:8428/api/v1/status/tsdb

# Check MariaDB connectivity
docker exec -it localdev-mariadb mysql -u mirador -p -e "SELECT 1"

# Check backend response times
for backend in weaviate:8080 victoriametrics:8428; do
  echo "Testing $backend..."
  curl -w "time_total: %{time_total}s\n" -s -o /dev/null http://localhost:$(echo $backend | cut -d: -f2)/
done
```

### 4. Review Correlation Engine Logs

```bash
# Check for correlation errors
docker logs mirador-core 2>&1 | grep -i "correlation" | grep -i "error\|fail\|timeout"

# Check for specific query failures
docker logs mirador-core 2>&1 | grep -i "correlate\|rca" | tail -50

# Check for backend connectivity issues
docker logs mirador-core 2>&1 | grep -i "weaviate\|victoria\|mariadb" | grep -i "error\|fail"
```

## Resolution

### Correlation Queries Failing

1. **Check API contract compliance** - Ensure request body is exactly:
   ```json
   {
     "startTime": "ISO-8601 UTC timestamp",
     "endTime": "ISO-8601 UTC timestamp"
   }
   ```

2. **Validate time range**:
   - `endTime` must be after `startTime`
   - Window must not exceed `engine.maxWindow` (default: 1 hour)
   - Times should be in UTC timezone

3. **Check backend health** - Restart unhealthy backends:
   ```bash
   docker restart localdev-weaviate-1
   docker restart victoriametrics
   ```

4. **Review error logs** for specific failure reasons

### Inconsistent Correlation Results

1. **Check data availability** - Ensure telemetry data exists for time range:
   ```bash
   curl -s "http://localhost:8428/api/v1/query_range?query=up&start=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)&end=$(date -u +%Y-%m-%dT%H:%M:%SZ)&step=60s"
   ```

2. **Review KPI registrations** - Check if impacted KPIs are registered:
   ```bash
   curl -s http://localhost:8010/api/v1/kpis | jq '.data | length'
   ```

3. **Check correlation thresholds** in engine config:
   ```yaml
   engine:
     correlationThreshold: 0.7
     anomalyThreshold: 2.0
   ```

4. **Verify ring/bucket configuration** - Ensure temporal anchoring is consistent

### Correlation Engine Timeouts

1. **Reduce time window** - Use smaller time ranges for complex correlations

2. **Increase timeout values**:
   ```yaml
   engine:
     correlationTimeout: 120s
     queryTimeout: 60s
   ```

3. **Optimize backend queries**:
   - Add indexes to frequently-queried fields in Weaviate
   - Tune VictoriaMetrics query settings

4. **Scale engine resources** - Increase CPU/memory limits for mirador-core

### Empty or Incomplete Results

1. **Verify data ingestion** - Check if telemetry is being ingested:
   ```bash
   curl -s "http://localhost:8428/api/v1/query?query=count({__name__=~'.+'})"
   ```

2. **Check KPI metadata** - Ensure KPIs have proper labels and configurations

3. **Review engine config** for minimum data requirements

4. **Check time alignment** - Ensure query time range aligns with available data

## Prevention

### Best Practices for Correlation Reliability

1. **Monitor correlation health continuously**
   - Set up alerts for error rate >1%
   - Monitor p95 latency and alert if >30s
   - Track correlation success rate

2. **Ensure data quality**
   - Validate telemetry data completeness
   - Monitor data ingestion lag
   - Alert on missing KPI registrations

3. **Configure appropriate timeouts**
   - Set correlation timeout to 2x expected max duration
   - Implement circuit breakers for backend failures
   - Use retry logic with exponential backoff

4. **Maintain backend health**
   - Monitor all backend services (Weaviate, VictoriaMetrics, MariaDB)
   - Set up health checks and auto-restart policies
   - Maintain sufficient capacity headroom

5. **Regular testing**
   - Run correlation queries as part of health checks
   - Include correlation tests in CI/CD pipelines
   - Perform load testing before major releases

### Alerting Rules

```yaml
groups:
  - name: correlation-reliability
    rules:
      - alert: CorrelationEngineErrorRate
        expr: rate(mirador_core_correlation_engine_queries_total{status="error"}[5m]) / rate(mirador_core_correlation_engine_queries_total[5m]) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Correlation engine error rate exceeds 1%"
          
      - alert: CorrelationEngineTimeout
        expr: histogram_quantile(0.95, rate(mirador_core_correlation_engine_query_duration_seconds_bucket[5m])) > 60
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Correlation engine p95 latency exceeds 60s"
          
      - alert: CorrelationBackendUnhealthy
        expr: up{job=~"weaviate|victoriametrics"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Correlation backend is unhealthy"
```

## Related Documentation

- [Correlation Engine Documentation](correlation-engine.md)
- [Correlation Queries Guide](correlation-queries-guide.md)
- [RCA Documentation](rca.md)
- [Service Recovery Procedures](service-recovery-procedures.md)