# Service Recovery Procedures

## Overview

This document outlines procedures for recovering MIRADOR-CORE services after failures.

## Service Components

### Unified Query Engine
**Recovery Steps:**
1. Check service health endpoints
2. Restart query engine service
3. Verify backend connectivity
4. Test query functionality

### Correlation Engine
**Recovery Steps:**
1. Check correlation engine status
2. Restart correlation service
3. Verify data source connections
4. Test correlation queries

### API Gateway
**Recovery Steps:**
1. Check API gateway health
2. Restart gateway service
3. Verify authentication services
4. Test API endpoints

## Automated Recovery

### Health Check Configuration

The following health checks enable automated recovery in container orchestration:

```yaml
# Docker Compose / Kubernetes liveness probe
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8010/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 60s
```

### Kubernetes Deployment with Auto-Recovery

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mirador-core
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: mirador-core
        livenessProbe:
          httpGet:
            path: /health
            port: 8010
          initialDelaySeconds: 60
          periodSeconds: 30
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8010
          initialDelaySeconds: 10
          periodSeconds: 10
          failureThreshold: 3
```

### Circuit Breaker Pattern

MIRADOR-CORE implements circuit breakers for backend services:

```yaml
# EngineConfig circuit breaker settings
engine:
  circuitBreaker:
    enabled: true
    threshold: 5        # Failure count before opening
    timeout: 30s        # Time before attempting recovery
    halfOpenRequests: 3 # Requests to test before closing
```

When a backend circuit opens:
1. Requests fail fast with cached data if available
2. After timeout, circuit enters half-open state
3. If test requests succeed, circuit closes
4. If test requests fail, circuit reopens

### Automatic Restart Policies

```yaml
# Docker Compose restart policy
services:
  mirador-core:
    restart: unless-stopped
    
# Kubernetes restart policy
spec:
  restartPolicy: Always
```

## Manual Recovery

### Step-by-Step Recovery Procedure

#### 1. Assess the Failure

```bash
# Check service status
docker ps -a | grep mirador-core

# Check recent logs
docker logs mirador-core --tail 100

# Check health endpoint
curl -s http://localhost:8010/health | jq .

# Check metrics endpoint
curl -s http://localhost:8010/metrics | head -50
```

#### 2. Identify Root Cause

Common failure scenarios:

| Symptom | Likely Cause | Check Command |
|---------|-------------|---------------|
| Container exits immediately | Configuration error | `docker logs mirador-core` |
| Health check failing | Backend connectivity | `curl http://localhost:8080/v1/.well-known/ready` |
| High memory/OOM | Memory leak or overload | `docker stats mirador-core` |
| Connection refused | Port conflict or bind error | `netstat -tlpn \| grep 8010` |

#### 3. Recover Backend Services First

```bash
# Check and restart Weaviate
docker restart localdev-weaviate-1
sleep 10
curl -s http://localhost:8080/v1/.well-known/ready

# Check and restart VictoriaMetrics
docker restart victoriametrics
sleep 5
curl -s http://localhost:8428/api/v1/status/tsdb

# Check and restart MariaDB
docker restart localdev-mariadb
sleep 10
docker exec localdev-mariadb mysql -u mirador -pmirador -e "SELECT 1"

# Check and restart Redis/Valkey
docker restart localdev-valkey-1
sleep 5
redis-cli -h localhost -p 6379 ping
```

#### 4. Recover MIRADOR-CORE

```bash
# Stop the service
docker stop mirador-core

# Remove the container (preserves data volumes)
docker rm mirador-core

# Rebuild if code changes were made
docker rmi localdev-mirador-core

# Start fresh
docker compose -f deployments/localdev/docker-compose.yaml up -d mirador-core

# Verify recovery
sleep 30
curl -s http://localhost:8010/health | jq .
```

#### 5. Validate Recovery

```bash
# Test health endpoints
curl -s http://localhost:8010/health
curl -s http://localhost:8010/health/ready

# Test query endpoint
curl -X POST http://localhost:8010/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"type":"kpi","query":"limit 5"}'

# Test correlation endpoint
curl -X POST http://localhost:8010/api/v1/unified/correlate \
  -H "Content-Type: application/json" \
  -d '{"startTime":"2025-01-01T00:00:00Z","endTime":"2025-01-01T01:00:00Z"}'

# Check metrics are being collected
curl -s http://localhost:8010/metrics | grep mirador_core_http_requests_total
```

### Emergency Recovery: Full Stack

If multiple services are failing:

```bash
# Full stack restart (caution: brief downtime)
make localdev-down
sleep 10
make localdev-up

# Wait for services to initialize
sleep 60

# Verify all services
docker ps

# Re-seed data if needed
make localdev-seed-data
```

## Prevention

### Best Practices for Service Reliability

1. **Implement proper health checks**
   - Liveness: Is the process running?
   - Readiness: Can it serve traffic?
   - Startup: Has initial setup completed?

2. **Use graceful shutdown**
   ```go
   // Already implemented in main.go
   quit := make(chan os.Signal, 1)
   signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
   <-quit
   server.GracefulShutdown(ctx)
   ```

3. **Configure appropriate timeouts**
   ```yaml
   server:
     readTimeout: 30s
     writeTimeout: 60s
     idleTimeout: 120s
     shutdownTimeout: 30s
   ```

4. **Monitor key metrics**
   - Error rate: Alert if >1% over 5 minutes
   - Latency p95: Alert if >2s
   - Memory usage: Alert if >80% of limit
   - CPU usage: Alert if >90% sustained

5. **Maintain runbooks**
   - Document all recovery procedures
   - Test recovery procedures regularly
   - Update runbooks after incidents

6. **Implement redundancy**
   - Run multiple replicas
   - Use load balancing
   - Implement failover for backends

### Post-Incident Checklist

After any service recovery:

- [ ] Confirm all health checks passing
- [ ] Verify data integrity
- [ ] Check for data loss during downtime
- [ ] Review logs for root cause
- [ ] Update monitoring/alerting if gaps found
- [ ] Document incident and resolution
- [ ] Update runbooks if procedures changed

## Related Documentation

- [Query Performance Runbook](query-performance-runbook.md)
- [Cache Performance Runbook](cache-performance-runbook.md)
- [Correlation Reliability Runbook](correlation-reliability-runbook.md)
- [Monitoring and Observability](monitoring-observability.md)