# Multi-Tenant Implementation Summary

**Date:** November 8, 2025  
**Architecture:** Separate Deployments with Intelligent Routing

---

## Overview

Mirador-Core implements multi-tenancy using **separate Victoria* deployments per tenant** with intelligent routing at the application layer. This provides complete physical isolation while maintaining a unified API.

## Architecture Diagram

```
┌─────────────────────────────────────────────────┐
│         MIRADOR-CORE (Smart Router)             │
│                                                 │
│  Tenant A → Router → VM/VL/VT Deployment A     │
│  Tenant B → Router → VM/VL/VT Deployment B     │
│  Tenant C → Router → VM/VL/VT Deployment C     │
└─────────────────────────────────────────────────┘
```

## Key Components

### 1. Tenant Model with Deployments

```go
type Tenant struct {
    ID          string
    Name        string
    Deployments TenantDeployments  // Separate endpoints per tenant
    Status      TenantStatus
    Quotas      TenantQuotas
    Features    TenantFeatures
}

type TenantDeployments struct {
    Metrics DeploymentConfig  // VictoriaMetrics endpoints
    Logs    DeploymentConfig  // VictoriaLogs endpoints
    Traces  DeploymentConfig  // VictoriaTraces endpoints
}
```

### 2. Router Services

Three router services handle intelligent routing:

- **VictoriaMetricsRouter**: Routes metrics queries to tenant deployments
- **VictoriaLogsRouter**: Routes log queries to tenant deployments
- **VictoriaTracesRouter**: Routes trace queries to tenant deployments

Each router:
- Caches tenant service instances
- Loads tenant config from repository
- Creates service instances with tenant-specific endpoints
- Handles health checks and failover

### 3. Request Flow

```
1. Request arrives with X-Tenant-ID header
2. Middleware validates tenant and loads config
3. Router looks up cached service or creates new one
4. Service connects to tenant-specific endpoints
5. Query executes on isolated deployment
6. Results returned to client
```

## Implementation Checklist

### Phase 1: Foundation (Week 1-2)
- [ ] Create `models.Tenant` with `TenantDeployments`
- [ ] Create `models.DeploymentConfig` with health check config
- [ ] Implement `TenantRepository` interface
- [ ] Implement Weaviate-based tenant repository

### Phase 2: Router Services (Week 3-4)
- [ ] Implement `VictoriaMetricsRouter` with caching
- [ ] Implement `VictoriaLogsRouter` with caching
- [ ] Implement `VictoriaTracesRouter` with caching
- [ ] Add cache invalidation mechanisms

### Phase 3: API Handlers (Week 5)
- [ ] Create tenant CRUD handlers
- [ ] Update existing handlers to use routers
- [ ] Add deployment validation
- [ ] Implement tenant provisioning workflow

### Phase 4: Infrastructure (Week 6-7)
- [ ] Create K8s namespace templates
- [ ] Create Helm value templates per tier
- [ ] Implement tenant provisioner service
- [ ] Add deployment health monitoring

### Phase 5: Testing & Operations (Week 8)
- [ ] Unit tests for routers
- [ ] Integration tests with multiple tenants
- [ ] Load testing
- [ ] Documentation and runbooks

## Code Snippets

### Creating a Tenant with Deployments

```go
tenant := &models.Tenant{
    Name: "platformbuilds",
    DisplayName: "PLATFORMBUILDS",
    AdminEmail: "aarvee@platformbuilds.io",
    Deployments: models.TenantDeployments{
        Metrics: models.DeploymentConfig{
            Endpoints: []string{
                "http://vm-platformbuilds-1.svc.cluster.local:8428",
                "http://vm-platformbuilds-2.svc.cluster.local:8428",
            },
            Timeout: 30000,
            Namespace: "tenant-platformbuilds-metrics",
        },
        Logs: models.DeploymentConfig{
            Endpoints: []string{
                "http://vl-platformbuilds-1.svc.cluster.local:9428",
            },
            Timeout: 30000,
            Namespace: "tenant-platformbuilds-logs",
        },
        Traces: models.DeploymentConfig{
            Endpoints: []string{
                "http://vt-platformbuilds-1.svc.cluster.local:7428",
            },
            Timeout: 30000,
            Namespace: "tenant-platformbuilds-traces",
        },
    },
}

err := tenantRepo.CreateTenant(ctx, tenant)
```

### Using the Router

```go
// In handler
func (h *MetricsHandler) Query(c *gin.Context) {
    ctx := utils.WithTenantID(c.Request.Context(), tenantID)
    
    // Router automatically selects correct deployment
    result, err := h.metricsRouter.ExecuteQueryWithTenant(ctx, request)
}
```

## Infrastructure Setup

### Per-Tenant K8s Resources

Each tenant gets:
- 3 dedicated namespaces (metrics, logs, traces)
- Isolated StatefulSets for each Victoria* component
- Separate PVCs for data storage
- Individual services and ingress (optional)

### Example Deployment

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tenant-chikacafe-metrics
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: vm-chikacafe-metrics
  namespace: tenant-chikacafe-metrics
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: victoriametrics
        image: victoriametrics/victoria-metrics:latest
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
```

## Benefits

✅ **Complete Isolation**: No shared infrastructure
✅ **Independent Scaling**: Per-tenant resource allocation
✅ **Security**: Network-level separation
✅ **Compliance**: Meets regulatory requirements
✅ **Performance**: No noisy neighbor issues
✅ **Flexibility**: Different configs per tenant
✅ **Cost Transparency**: Direct allocation

## Migration from AccountID Approach

If migrating from shared deployments with AccountID:

1. **Parallel Deployment**: Create new tenant-specific deployments
2. **Data Migration**: Export/import using Victoria* tools
3. **Configuration Update**: Update tenant records with new endpoints
4. **Gradual Cutover**: Switch routing tenant by tenant
5. **Validation**: Verify data integrity post-migration
6. **Cleanup**: Decommission shared deployments

## Operations

### Tenant Provisioning

```bash
# 1. Create tenant in Mirador-Core
curl -X POST /api/v1/tenants -d @tenant.json

# 2. Provision infrastructure (automated)
# - Creates K8s namespaces
# - Deploys Victoria* via Helm
# - Updates tenant config with endpoints

# 3. Validate deployment
curl -X GET /api/v1/tenants/{id}/health
```

### Health Monitoring

```go
// Automatic health checks every 30s
monitor.StartMonitoring(ctx, 30*time.Second)

// Manual health check
health := monitor.GetTenantHealth(tenantID)
```

### Cache Management

```go
// Invalidate cache when tenant config changes
metricsRouter.InvalidateTenantCache(tenantID)
logsRouter.InvalidateTenantCache(tenantID)
tracesRouter.InvalidateTenantCache(tenantID)
```

## Security Considerations

1. **Network Policies**: Enforce namespace isolation
2. **RBAC**: Limit access to tenant namespaces
3. **Secrets**: Store credentials in K8s secrets
4. **TLS**: Enable mTLS between Mirador and deployments
5. **Audit**: Log all tenant operations

## Performance Optimization

1. **Service Caching**: Cache tenant service instances
2. **Connection Pooling**: Reuse HTTP connections
3. **Health Check Caching**: Cache health status
4. **Query Routing**: Route to nearest healthy endpoint
5. **Retry Logic**: Automatic failover to backup endpoints

## Monitoring Metrics

Track these per tenant:
- Query latency (p50, p95, p99)
- Query volume (requests/min)
- Error rate
- Deployment health status
- Resource utilization
- Storage usage

## Next Steps

1. ✅ Review architecture documentation
2. ⏳ Set up development environment
3. ⏳ Implement Phase 1 (tenant model)
4. ⏳ Implement Phase 2 (router services)
5. ⏳ Create infrastructure templates
6. ⏳ Deploy test tenant deployments
7. ⏳ Integration testing
8. ⏳ Production rollout

---

For detailed implementation, see [MULTI_TENANT_STRATEGY.md](./MULTI_TENANT_STRATEGY.md)
