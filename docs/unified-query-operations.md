# Unified Query Platform Operations Guide

## Overview

This guide provides comprehensive operational procedures for managing the unified query platform in Mirador Core v7.0.0+. It covers monitoring, scaling, troubleshooting, and maintaining high availability of the unified observability system.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Monitoring](#monitoring)
3. [Performance Tuning](#performance-tuning)
4. [Scaling](#scaling)
5. [High Availability](#high-availability)
6. [Troubleshooting](#troubleshooting)
7. [Capacity Planning](#capacity-planning)
8. [Incident Response](#incident-response)
9. [Maintenance Procedures](#maintenance-procedures)
10. [Security Operations](#security-operations)

## Architecture Overview

### Component Topology

```
┌─────────────────────────────────────────────────────┐
│                  Load Balancer                       │
│               (Ingress / API Gateway)                │
└──────────────────┬──────────────────────────────────┘
                   │
    ┌──────────────┴──────────────┐
    │                              │
┌───▼────────┐              ┌─────▼──────┐
│ Mirador    │              │  Mirador   │
│ Core Pod 1 │   ...        │  Core Pod N│
└────┬───────┘              └─────┬──────┘
     │                             │
     └──────────┬──────────────────┘
                │
    ┌───────────┼────────────┐
    │           │            │
┌───▼───┐  ┌───▼───┐  ┌────▼────┐
│Victoria│  │Victoria│  │ Victoria│
│Metrics │  │ Logs   │  │ Traces  │
│Cluster │  │Cluster │  │ Cluster │
└────────┘  └────────┘  └─────────┘
    │           │            │
    └───────────┴────────────┘
                │
         ┌──────▼──────┐
         │   Valkey    │
         │   Cluster   │
         │  (Caching)  │
         └─────────────┘
```

### Data Flow

1. **Query Ingestion**: Load balancer routes requests to Mirador Core pods
2. **Query Routing**: Unified query engine analyzes and routes to appropriate backend
3. **Parallel Execution**: Queries execute across VictoriaMetrics/Logs/Traces clusters
4. **Result Aggregation**: Results unified and cached in Valkey
5. **Response**: Unified response returned to client

## Monitoring

### Key Metrics

#### Unified Query Metrics

Monitor these Prometheus metrics exposed at `/metrics`:

**Query Performance:**
```promql
# Query latency (p50, p95, p99)
histogram_quantile(0.95, 
  rate(mirador_unified_query_duration_seconds_bucket[5m])
)

# Query throughput
rate(mirador_unified_query_total[5m])

# Query success rate
rate(mirador_unified_query_success_total[5m]) / 
rate(mirador_unified_query_total[5m])
```

**Engine Health:**
```promql
# Engine availability
mirador_engine_health{engine="victoriametrics"}
mirador_engine_health{engine="victorialogs"}
mirador_engine_health{engine="victoriatraces"}

# Engine query latency
rate(mirador_engine_query_duration_seconds_sum[5m]) / 
rate(mirador_engine_query_duration_seconds_count[5m])
```

**Cache Performance:**
```promql
# Cache hit rate
rate(mirador_cache_hits_total[5m]) / 
(rate(mirador_cache_hits_total[5m]) + rate(mirador_cache_misses_total[5m]))

# Cache memory usage
mirador_cache_memory_bytes / mirador_cache_memory_limit_bytes

# Cache evictions
rate(mirador_cache_evictions_total[5m])
```

### Unified Query Language (UQL) Metrics

Monitor UQL-specific performance metrics:

**Parsing Performance:**
```promql
# UQL parsing latency
histogram_quantile(0.95, 
  rate(mirador_uql_parsing_duration_seconds_bucket[5m])
)

# Parsing success rate
rate(mirador_uql_parsing_success_total[5m]) / 
rate(mirador_uql_parsing_total[5m])
```

**Optimization Performance:**
```promql
# Query optimization latency
histogram_quantile(0.95, 
  rate(mirador_uql_optimization_duration_seconds_bucket[5m])
)

# Optimization passes applied
rate(mirador_uql_optimization_passes_total[5m])
```

**Translation Performance:**
```promql
# Query translation latency by target engine
histogram_quantile(0.95, 
  rate(mirador_uql_translation_duration_seconds_bucket{engine="promql"}[5m])
)
```

### Grafana Dashboards

#### Unified Query Performance Dashboard

Deploy the provided Grafana dashboard:

```bash
kubectl apply -f deployments/grafana/dashboards/unified-query-performance.json
```

**Key Panels:**
- Query latency over time (p50, p95, p99)
- Query throughput by type (metrics, logs, traces, correlation)
- Success rate and error breakdown
- Cache hit rate and efficiency
- Engine health status
- Resource utilization

#### Correlation Engine Dashboard

```bash
kubectl apply -f deployments/grafana/dashboards/correlation-engine.json
```

**Key Panels:**
- Correlation query volume
- Time-window correlation latency
- Label-based correlation success rate
- Confidence score distribution
- Cross-engine coordination metrics

### Alerting Rules

Deploy Prometheus alerting rules:

```yaml
# deployments/grafana/alerting-rules.yml
groups:
  - name: unified_query_alerts
    rules:
      # High error rate
      - alert: UnifiedQueryHighErrorRate
        expr: |
          (rate(mirador_unified_query_errors_total[5m]) / 
           rate(mirador_unified_query_total[5m])) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High unified query error rate"
          description: "Error rate is {{ $value | humanizePercentage }} (threshold: 5%)"

      # High latency
      - alert: UnifiedQueryHighLatency
        expr: |
          histogram_quantile(0.95, 
            rate(mirador_unified_query_duration_seconds_bucket[5m])
          ) > 5
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High unified query latency"
          description: "P95 latency is {{ $value }}s (threshold: 5s)"

      # Engine unavailable
      - alert: QueryEngineDown
        expr: mirador_engine_health < 1
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Query engine unavailable"
          description: "Engine {{ $labels.engine }} is unhealthy"

      # Low cache hit rate
      - alert: UnifiedQueryLowCacheHitRate
        expr: |
          (rate(mirador_cache_hits_total[5m]) / 
           (rate(mirador_cache_hits_total[5m]) + rate(mirador_cache_misses_total[5m]))) < 0.4
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Low cache hit rate"
          description: "Cache hit rate is {{ $value | humanizePercentage }} (threshold: 40%)"

      # Cache memory pressure
      - alert: CacheMemoryPressure
        expr: |
          (mirador_cache_memory_bytes / mirador_cache_memory_limit_bytes) > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Cache memory pressure"
          description: "Cache memory usage is {{ $value | humanizePercentage }}"
```

### Log Monitoring

Monitor application logs for issues:

```bash
# Query error logs
kubectl logs -l app=mirador-core -n mirador-system | grep "ERROR"

# Query slow queries
kubectl logs -l app=mirador-core -n mirador-system | grep "slow_query"

# Query cache issues
kubectl logs -l app=mirador-core -n mirador-system | grep "cache"
```

## Performance Tuning

### Query Optimization

#### Enable Query Caching

Configure appropriate cache TTLs:

```yaml
# config.yaml
unified_query:
  cache:
    enabled: true
    default_ttl: "5m"
    max_ttl: "1h"
    max_memory: "2GB"
```

**Caching Recommendations:**
- **Dashboard queries**: 1-5 minutes
- **Analytical queries**: 5-15 minutes
- **Historical queries**: 15-60 minutes
- **Real-time queries**: Disable caching

#### Query Timeouts

Set appropriate timeouts based on query complexity:

```yaml
unified_query:
  default_timeout: "30s"
  max_timeout: "300s"
```

**Timeout Guidelines:**
- **Simple queries**: 10-30 seconds
- **Aggregation queries**: 30-60 seconds
- **Correlation queries**: 60-120 seconds
- **Complex multi-engine**: 120-300 seconds

### Resource Limits

Configure pod resource limits appropriately:

```yaml
# deployments/k8s/complete-deployment.yaml
resources:
  requests:
    cpu: "1000m"
    memory: "2Gi"
  limits:
    cpu: "2000m"
    memory: "4Gi"
```

**Sizing Guidelines:**
- **Small deployment** (< 1000 qps): 1 CPU, 2GB RAM
- **Medium deployment** (1000-5000 qps): 2 CPU, 4GB RAM
- **Large deployment** (5000-10000 qps): 4 CPU, 8GB RAM
- **Extra large** (> 10000 qps): 8 CPU, 16GB RAM

### Connection Pooling

Tune connection pools to backend engines:

```yaml
# config.yaml
database:
  victoria_metrics:
    max_connections: 100
    idle_connections: 10
    connection_timeout: "30s"
    
  victoria_logs:
    max_connections: 100
    idle_connections: 10
    connection_timeout: "30s"
    
  victoria_traces:
    max_connections: 50
    idle_connections: 5
    connection_timeout: "30s"
```

### Valkey Cache Optimization

Optimize cache configuration:

```yaml
# config.yaml
cache:
  valkey:
    endpoints:
      - "valkey-1:6379"
      - "valkey-2:6379"
      - "valkey-3:6379"
    max_connections: 1000
    read_timeout: "3s"
    write_timeout: "3s"
    pool_size: 100
    max_retries: 3
```

## Scaling

### Horizontal Scaling

#### Scale Mirador Core Pods

```bash
# Scale up
kubectl scale deployment mirador-core -n mirador-system --replicas=5

# Auto-scaling based on CPU
kubectl autoscale deployment mirador-core \
  -n mirador-system \
  --min=3 --max=10 \
  --cpu-percent=70
```

#### Scale Backend Engines

**VictoriaMetrics:**
```bash
# Add vmselect nodes
kubectl scale statefulset vmselect -n victoria-system --replicas=5

# Add vmstorage nodes (requires data rebalancing)
kubectl scale statefulset vmstorage -n victoria-system --replicas=10
```

**VictoriaLogs:**
```bash
# Add vlselect nodes
kubectl scale statefulset vlselect -n victoria-system --replicas=3
```

**VictoriaTraces:**
```bash
# Add vtselect nodes
kubectl scale statefulset vtselect -n victoria-system --replicas=3
```

### Vertical Scaling

Increase resource limits for existing pods:

```bash
# Update deployment with more resources
kubectl set resources deployment mirador-core \
  -n mirador-system \
  --limits=cpu=4000m,memory=8Gi \
  --requests=cpu=2000m,memory=4Gi
```

### Cache Scaling

**Valkey Cluster Scaling:**
```bash
# Add Valkey nodes
kubectl scale statefulset valkey -n cache-system --replicas=6

# Rebalance cluster slots
kubectl exec -it valkey-0 -n cache-system -- redis-cli --cluster rebalance
```

### Load Balancing

Configure load balancer for optimal distribution:

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mirador-ingress
  annotations:
    nginx.ingress.kubernetes.io/load-balance: "round_robin"
    nginx.ingress.kubernetes.io/upstream-hash-by: "$remote_addr"
spec:
  rules:
    - host: mirador.company.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: mirador-core
                port:
                  number: 8010
```

## High Availability

### Multi-Zone Deployment

Deploy across multiple availability zones:

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mirador-core
spec:
  replicas: 6
  template:
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - mirador-core
              topologyKey: topology.kubernetes.io/zone
```

### Circuit Breakers

Configure circuit breakers for backend engines:

```yaml
# config.yaml
circuit_breakers:
  vm_circuit:
    failure_threshold: 5
    success_threshold: 3
    recovery_timeout: "60s"
    
  vl_circuit:
    failure_threshold: 3
    success_threshold: 2
    recovery_timeout: "30s"
    
  vt_circuit:
    failure_threshold: 3
    success_threshold: 2
    recovery_timeout: "30s"
```

### Health Checks

Configure liveness and readiness probes:

```yaml
# deployment.yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8010
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8010
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 2
```

### Backup and Recovery

**Configuration Backup:**
```bash
# Backup ConfigMaps
kubectl get configmap mirador-config -n mirador-system -o yaml > mirador-config-backup.yaml

# Backup Secrets
kubectl get secret mirador-secrets -n mirador-system -o yaml > mirador-secrets-backup.yaml
```

**State Recovery:**
```bash
# Restore configuration
kubectl apply -f mirador-config-backup.yaml
kubectl apply -f mirador-secrets-backup.yaml

# Restart pods
kubectl rollout restart deployment/mirador-core -n mirador-system
```

## Troubleshooting

### Common Issues

#### Issue 1: High Query Latency

**Symptoms:**
- P95 latency > 5 seconds
- Slow dashboard loading
- Query timeouts

**Diagnosis:**
```bash
# Check query latency
kubectl logs -l app=mirador-core -n mirador-system | grep "slow_query"

# Check backend engine health
curl https://mirador-core/api/v1/unified/health

# Check cache hit rate
curl https://mirador-core/api/v1/metrics | grep cache_hits
```

**Resolution:**
1. Enable query caching with appropriate TTLs
2. Optimize slow queries (reduce time ranges, add filters)
3. Scale backend engines horizontally
4. Increase resource limits for Mirador Core pods
5. Check backend engine performance

#### Issue 2: Engine Unavailable

**Symptoms:**
- `mirador_engine_health{engine="X"}` = 0
- Queries failing with engine errors
- Increased error rate

**Diagnosis:**
```bash
# Check engine connectivity
kubectl exec -it mirador-core-pod -n mirador-system -- \
  curl http://victoriametrics:8428/health

# Check engine logs
kubectl logs -l app=victoriametrics -n victoria-system

# Check network policies
kubectl get networkpolicy -n mirador-system
```

**Resolution:**
1. Verify backend engine is running
2. Check network connectivity
3. Verify DNS resolution
4. Check authentication/authorization
5. Review firewall/network policies
6. Restart affected engine pods

#### Issue 3: Low Cache Hit Rate

**Symptoms:**
- Cache hit rate < 40%
- High backend load
- Inconsistent query performance

**Diagnosis:**
```bash
# Check cache metrics
curl https://mirador-core/api/v1/metrics | grep "cache_"

# Check cache configuration
kubectl get configmap mirador-config -n mirador-system -o yaml | grep cache

# Check Valkey cluster health
kubectl exec -it valkey-0 -n cache-system -- redis-cli cluster info
```

**Resolution:**
1. Increase cache TTL for frequently queries
2. Increase cache memory limits
3. Scale Valkey cluster
4. Review query patterns
5. Enable query result caching in clients

#### Issue 4: Memory Pressure

**Symptoms:**
- OOMKilled pods
- High memory usage
- Cache evictions

**Diagnosis:**
```bash
# Check memory usage
kubectl top pods -n mirador-system

# Check OOMKilled events
kubectl get events -n mirador-system | grep OOMKilled

# Check cache memory
curl https://mirador-core/api/v1/metrics | grep cache_memory
```

**Resolution:**
1. Increase pod memory limits
2. Reduce cache size
3. Optimize query result sizes
4. Scale horizontally instead of vertically
5. Enable result streaming for large queries

#### Issue 5: Correlation Query Failures

**Symptoms:**
- Correlation queries timeout
- Inconsistent correlation results
- High correlation latency

**Diagnosis:**
```bash
# Check correlation metrics
curl https://mirador-core/api/v1/metrics | grep correlation

# Check correlation engine logs
kubectl logs -l app=mirador-core -n mirador-system | grep correlation

# Test correlation query
curl -X POST https://mirador-core/api/v1/unified/query \
  -d '{"query": {"type": "correlation", "query": "logs:error WITHIN 5m OF metrics:cpu > 80"}}'
```

**Resolution:**
1. Increase correlation query timeout
2. Reduce time window for correlation
3. Ensure all referenced engines are healthy
4. Optimize backend queries
5. Check label matching configuration

#### Issue 6: UQL Parsing Errors

**Symptoms:**
- UQL queries fail with parsing errors
- Syntax validation failures
- Unexpected query behavior

**Diagnosis:**
```bash
# Check UQL parsing metrics
curl https://mirador-core/api/v1/metrics | grep uql_parsing

# Check parsing error logs
kubectl logs -l app=mirador-core -n mirador-system | grep "UQL parsing failed"

# Validate query syntax
curl -X POST https://mirador-core/api/v1/uql/validate \
  -d '{"query": "SELECT * FROM logs WHERE invalid syntax"}'
```

**Resolution:**
1. Check UQL query syntax against language guide
2. Validate field names and data sources
3. Ensure proper quoting and escaping
4. Use UQL validation endpoint for debugging
5. Check for unsupported operators or functions

#### Issue 7: UQL Optimization Failures

**Symptoms:**
- UQL queries execute slowly despite optimization
- Optimization passes not applied
- Query plans not generated

**Diagnosis:**
```bash
# Check optimization metrics
curl https://mirador-core/api/v1/metrics | grep uql_optimization

# Check optimization logs
kubectl logs -l app=mirador-core -n mirador-system | grep "optimization"

# Generate query plan
curl -X POST https://mirador-core/api/v1/uql/explain \
  -d '{"query": "SELECT * FROM logs WHERE service = '\''api'\''"}'
```

**Resolution:**
1. Enable optimization features in configuration
2. Check data source statistics for cost-based optimization
3. Review query structure for optimization opportunities
4. Update optimizer configuration if needed
5. Consider manual query rewriting for complex cases

### Debug Mode

Enable debug logging for troubleshooting:

```bash
# Enable debug logs
kubectl set env deployment/mirador-core \
  -n mirador-system \
  LOG_LEVEL=debug

# View debug logs
kubectl logs -f -l app=mirador-core -n mirador-system
```

### Performance Profiling

Enable Go profiling for performance analysis:

```bash
# Enable pprof endpoint
kubectl set env deployment/mirador-core \
  -n mirador-system \
  PPROF_ENABLED=true

# Port-forward to access pprof
kubectl port-forward -n mirador-system svc/mirador-core 6060:6060

# Collect CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Collect memory profile
go tool pprof http://localhost:6060/debug/pprof/heap
```

## Capacity Planning

### Query Load Estimation

Calculate expected query load:

```
Daily Queries = (Dashboard Panels × Refresh Rate × Users × Hours)
              + (Ad-hoc Queries × Users × Hours)
              + (Alert Evaluations × Rules × Hours)

QPS = Daily Queries / 86400
```

**Example:**
- 100 dashboard panels, 10s refresh, 50 users, 8 hours: 144,000 queries
- 10 ad-hoc queries per user, 50 users, 8 hours: 4,000 queries
- 100 alert rules, 30s evaluation: 28,800 queries

Total: ~177,000 queries/day = ~2 QPS average

Peak load (assume 5x average): ~10 QPS

### Resource Requirements

**Mirador Core:**
- **CPU**: 1 core per 1000 QPS sustained
- **Memory**: 2GB base + 1GB per 1000 QPS
- **Network**: 100Mbps per 1000 QPS

**Valkey Cache:**
- **Memory**: 50MB per 1000 cached results
- **Connections**: 100 connections per Mirador Core pod

**Backend Engines:**
- Follow VictoriaMetrics/Logs/Traces capacity planning guides

### Growth Planning

Plan capacity for 12-18 months growth:

```
Projected QPS = Current QPS × (1 + Annual Growth Rate) ^ Years
Required Pods = ceil(Projected QPS / QPS per Pod)
```

**Example:**
- Current: 1000 QPS
- Annual growth: 50%
- Timeline: 18 months (1.5 years)
- Projected: 1000 × 1.5^1.5 = ~1837 QPS
- QPS per pod: 1000
- Required pods: 2 (with HA: 4)

## Incident Response

### Incident Response Playbook

#### Critical: All Queries Failing

1. **Assess Impact**
   - Check error rate: `rate(mirador_unified_query_errors_total[1m])`
   - Verify client reports

2. **Immediate Actions**
   - Check pod health: `kubectl get pods -n mirador-system`
   - Check backend engines: `curl https://mirador-core/api/v1/unified/health`
   - Review recent deployments: `kubectl rollout history deployment/mirador-core`

3. **Mitigation**
   - Rollback if recent deployment: `kubectl rollout undo deployment/mirador-core`
   - Scale up if resource constrained: `kubectl scale deployment/mirador-core --replicas=10`
   - Restart pods if hung: `kubectl rollout restart deployment/mirador-core`

4. **Communication**
   - Notify stakeholders
   - Post status updates
   - Coordinate with backend engine teams

#### High: Single Engine Unavailable

1. **Assess Impact**
   - Identify affected engine: `mirador_engine_health{engine="X"}`
   - Check query types affected

2. **Immediate Actions**
   - Verify engine health externally
   - Check network connectivity
   - Review engine logs

3. **Mitigation**
   - Coordinate with engine team
   - Route queries to healthy replicas
   - Consider enabling degraded mode (if available)

4. **Communication**
   - Notify affected users
   - Update status page

#### Medium: High Latency

1. **Assess Impact**
   - Check P95/P99 latency: `histogram_quantile(0.95, ...)`
   - Identify affected query types

2. **Immediate Actions**
   - Check resource utilization
   - Check backend engine performance
   - Review recent query patterns

3. **Mitigation**
   - Enable aggressive caching
   - Temporarily increase timeouts
   - Scale horizontally
   - Throttle low-priority queries

### Post-Incident Review

Conduct blameless postmortems:

1. **Timeline**: Document events chronologically
2. **Root Cause**: Identify underlying cause(s)
3. **Impact**: Quantify user impact
4. **Action Items**: Create preventive measures
5. **Documentation**: Update runbooks

## Maintenance Procedures

### Rolling Updates

Perform zero-downtime updates:

```bash
# Update to new version
kubectl set image deployment/mirador-core \
  -n mirador-system \
  mirador-core=platformbuilds/mirador-core:v7.1.0

# Monitor rollout
kubectl rollout status deployment/mirador-core -n mirador-system

# Verify health
curl https://mirador-core/api/v1/health
```

### Configuration Updates

Update configuration without downtime:

```bash
# Update ConfigMap
kubectl edit configmap mirador-config -n mirador-system

# Trigger rolling restart
kubectl rollout restart deployment/mirador-core -n mirador-system
```

### Certificate Rotation

Rotate TLS certificates:

```bash
# Update certificate secret
kubectl create secret tls mirador-tls-new \
  --cert=mirador.crt \
  --key=mirador.key \
  -n mirador-system

# Update deployment to use new secret
kubectl set env deployment/mirador-core \
  -n mirador-system \
  TLS_SECRET_NAME=mirador-tls-new

# Rolling restart
kubectl rollout restart deployment/mirador-core -n mirador-system

# Verify
curl -v https://mirador-core/api/v1/health
```

### Database Maintenance

Coordinate with backend engine teams for:
- Index optimization
- Data compaction
- Retention policy updates
- Storage expansion

## Security Operations

### Access Control

Review and audit access controls:

```bash
# List RBAC roles
kubectl get roles,rolebindings -n mirador-system

# Audit API access logs
kubectl logs -l app=mirador-core -n mirador-system | grep "api_access"
```

### Secret Management

Rotate secrets regularly:

```bash
# Generate new JWT secret
kubectl create secret generic mirador-secrets-new \
  --from-literal=jwt-secret=$(openssl rand -base64 32) \
  -n mirador-system

# Update deployment
kubectl set env deployment/mirador-core \
  -n mirador-system \
  JWT_SECRET_NAME=mirador-secrets-new
```

### Vulnerability Scanning

Regular security scans:

```bash
# Scan container images
trivy image platformbuilds/mirador-core:v7.0.0

# Scan Kubernetes manifests
kubectl scan deployment/mirador-core -n mirador-system
```

### Audit Logging

Enable and monitor audit logs:

```yaml
# config.yaml
audit:
  enabled: true
  log_queries: true
  log_access: true
  retention_days: 90
```

## Conclusion

This operations guide provides the foundation for running a production-ready unified query platform. Regular monitoring, proactive capacity planning, and systematic troubleshooting ensure high availability and optimal performance.

For additional support:
- **Documentation**: https://miradorstack.readthedocs.io/
- **Community**: https://github.com/platformbuilds/mirador-core/discussions
- **Professional Services**: Contact the Mirador team for enterprise support
