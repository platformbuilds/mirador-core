# Deployment Guide

This guide covers deploying MIRADOR-CORE in various environments.

## Prerequisites

- Kubernetes cluster (v1.19+)
- Helm 3.x
- Docker registry access
- PostgreSQL database
- Redis cache
- LDAP/AD for authentication (optional)

## Quick Start with Helm

```bash
# Add the MIRADOR-CORE Helm repository
helm repo add mirador-core https://charts.mirador-core.io
helm repo update

# Install with default configuration
helm install mirador-core mirador-core/mirador-core

# Install with custom values
helm install mirador-core mirador-core/mirador-core \
  --values custom-values.yaml \
  --namespace observability
```

## Configuration

### Basic Configuration

```yaml
# custom-values.yaml
global:
  imageRegistry: "your-registry.com"
  imagePullSecrets:
    - name: registry-secret

config:
  database:
    host: "postgresql.observability.svc.cluster.local"
    port: 5432
    database: "mirador_core"
    username: "mirador_user"

  redis:
    host: "redis.observability.svc.cluster.local"
    port: 6379

  auth:
    provider: "ldap"  # ldap, oauth2, or local
    ldap:
      url: "ldaps://ldap.company.com"
      baseDN: "dc=company,dc=com"
      userSearchFilter: "(sAMAccountName=%s)"

  features:
    rca_enabled: true
    predict_enabled: true
    unified_query_enabled: true
```

### Advanced Configuration

```yaml
# Advanced configuration with resource limits
config:
  metrics:
    retention: "90d"
    resolution: "15s"

  logs:
    retention: "30d"
    compression: "gzip"

  traces:
    retention: "7d"
    sampling:
      rate: 0.1

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: mirador-core.company.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: mirador-core-tls
      hosts:
        - mirador-core.company.com

resources:
  requests:
    memory: "2Gi"
    cpu: "1000m"
  limits:
    memory: "4Gi"
    cpu: "2000m"

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80
```

## Environment-Specific Deployments

### Development Environment

```bash
# Deploy to local Kubernetes (e.g., minikube, k3s)
helm install mirador-core ./deployments/chart \
  --values dev-values.yaml \
  --namespace mirador-dev \
  --create-namespace
```

### Production Environment

```bash
# Production deployment with high availability
helm install mirador-core ./deployments/chart \
  --values prod-values.yaml \
  --namespace mirador-prod \
  --create-namespace \
  --wait \
  --timeout 10m
```

## Docker Compose (Local Development)

For local development without Kubernetes:

```yaml
# docker-compose.override.yml
version: '3.8'
services:
  mirador-core:
    image: mirador-core:latest
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgresql://user:pass@postgres:5432/mirador_core
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=mirador_core
      - POSTGRES_USER=mirador_user
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
```

## Database Setup

### PostgreSQL

```text
-- Create database and user
CREATE DATABASE mirador_core;
CREATE USER mirador_user WITH ENCRYPTED PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE mirador_core TO mirador_user;

-- Enable required extensions
\c mirador_core;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";
CREATE EXTENSION IF NOT EXISTS "timescaledb";
```

### Schema Initialization

The application will automatically create and migrate the database schema on startup. For manual KPI management:

```bash
# Run schema migrations
kubectl exec -it deployment/mirador-core -- ./bin/migrate up

# Check migration status
kubectl exec -it deployment/mirador-core -- ./bin/migrate status
```

## Monitoring and Observability

### Prometheus Metrics

MIRADOR-CORE exposes Prometheus metrics at `/metrics`:

```yaml
# prometheus-config.yaml
scrape_configs:
  - job_name: 'mirador-core'
    static_configs:
      - targets: ['mirador-core:8080']
    scrape_interval: 15s
```

### Health Checks

```bash
# Health endpoint
curl http://mirador-core.company.com/api/v1/health

# Readiness endpoint
curl http://mirador-core.company.com/api/v1/ready

# Deep health check
curl http://mirador-core.company.com/api/v1/health/deep
```

## Backup and Recovery

### Database Backup

```bash
# Create backup
kubectl exec -it deployment/postgres -- pg_dump -U mirador_user mirador_core > backup.sql

# Restore from backup
kubectl exec -it deployment/postgres -- psql -U mirador_user mirador_core < backup.sql
```

### Configuration Backup

```bash
# Backup Helm release
helm get values mirador-core > mirador-core-backup.yaml

# Restore from backup
helm upgrade mirador-core ./deployments/chart --values mirador-core-backup.yaml
```

## Troubleshooting

### Common Issues

1. **Database Connection Failed**
   ```bash
   # Check database connectivity
   kubectl exec -it deployment/mirador-core -- nc -zv postgres 5432
   ```

2. **Redis Connection Failed**
   ```bash
   # Check Redis connectivity
   kubectl exec -it deployment/mirador-core -- redis-cli -h redis ping
   ```

3. **Pod Crashing**
   ```bash
   # Check pod logs
   kubectl logs -f deployment/mirador-core

   # Check pod events
   kubectl describe pod -l app=mirador-core
   ```

4. **High Memory Usage**
   ```bash
   # Check memory usage
   kubectl top pods -l app=mirador-core

   # Adjust resource limits
   kubectl patch deployment mirador-core --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value":"8Gi"}]'
   ```

### Logs and Debugging

```bash
# View application logs
kubectl logs -f deployment/mirador-core -c mirador-core

# View system logs
kubectl logs -f deployment/mirador-core -c sidecar

# Enable debug logging
kubectl set env deployment/mirador-core LOG_LEVEL=DEBUG

# Access pod for debugging
kubectl exec -it deployment/mirador-core -- /bin/bash
```

## Security Considerations

### Network Policies

```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: mirador-core-policy
spec:
  podSelector:
    matchLabels:
      app: mirador-core
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - podSelector:
        matchLabels:
          app: redis
    ports:
    - protocol: TCP
      port: 6379
```

### TLS Configuration

```yaml
# Enable TLS
ingress:
  enabled: true
  tls:
    - secretName: mirador-core-tls
      hosts:
        - mirador-core.company.com

# Certificate management with cert-manager
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: mirador-core-tls
spec:
  secretName: mirador-core-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
  - mirador-core.company.com
```

## Performance Tuning

### Resource Optimization

```yaml
# Optimized resource configuration
resources:
  requests:
    memory: "4Gi"
    cpu: "2000m"
  limits:
    memory: "8Gi"
    cpu: "4000m"

# JVM tuning for Java-based components
env:
  - name: JAVA_OPTS
    value: "-Xmx6g -Xms2g -XX:+UseG1GC -XX:MaxGCPauseMillis=200"
```

### Scaling Configuration

```yaml
# Horizontal Pod Autoscaler
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## Upgrading

### Helm Upgrade

```bash
# Check for available updates
helm repo update
helm search repo mirador-core

# Upgrade to latest version
helm upgrade mirador-core mirador-core/mirador-core

# Upgrade with custom values
helm upgrade mirador-core mirador-core/mirador-core --values custom-values.yaml
```

### Rolling Updates

```bash
# Perform rolling update
kubectl rollout restart deployment/mirador-core

# Monitor rollout status
kubectl rollout status deployment/mirador-core

# Rollback if needed
kubectl rollout undo deployment/mirador-core
```

## Support

For deployment issues or questions:

1. Check the [troubleshooting guide](#troubleshooting)
2. Review [GitHub Issues](https://github.com/platformbuilds/mirador-core/issues)
3. Contact the platform team at platform@company.com