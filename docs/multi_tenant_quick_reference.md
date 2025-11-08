# Multi-Tenant Quick Reference Guide

**For Developers implementing multi-tenant features in Mirador-Core**

---

## Quick Architecture Overview

```
Client Request → Middleware (tenant extraction) → Router (service lookup) → Tenant Deployment
```

---

## Essential Imports

```go
import (
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/repo"
    "github.com/platformbuilds/mirador-core/internal/services"
    "github.com/platformbuilds/mirador-core/internal/utils"
)
```

---

## Working with Tenant Context

### Extract Tenant from Gin Context

```go
func (h *Handler) SomeHandler(c *gin.Context) {
    // Get full tenant object
    tenant := c.MustGet("tenant").(*models.Tenant)
    
    // Or just get tenant ID
    tenantID := c.GetString("tenant_id")
    
    // Create context with tenant
    ctx := utils.WithTenantID(c.Request.Context(), tenant.ID)
}
```

### Add Tenant to Go Context

```go
ctx := utils.WithTenantID(context.Background(), "platformbuilds")
tenantID := utils.GetTenantID(ctx) // Returns "platformbuilds"

tenant := &models.Tenant{ID: "chikacafe", Name: "Chika Cafe"}
ctx = utils.WithTenant(ctx, tenant)
retrievedTenant := utils.GetTenant(ctx) // Returns tenant object
```

---

## Using Router Services

### VictoriaMetrics Router

```go
// Initialize (usually in server setup)
metricsRouter := services.NewVictoriaMetricsRouter(tenantRepo, logger)

// Query metrics for tenant
ctx := utils.WithTenantID(context.Background(), "platformbuilds")
result, err := metricsRouter.ExecuteQueryWithTenant(ctx, &models.MetricsQLQueryRequest{
    Query: "up",
})

// Range query
rangeResult, err := metricsRouter.ExecuteRangeQueryWithTenant(ctx, &models.MetricsQLRangeQueryRequest{
    Query: "up",
    Start: "2025-11-08T00:00:00Z",
    End:   "2025-11-08T23:59:59Z",
    Step:  "5m",
})

// Get series
series, err := metricsRouter.GetSeriesWithTenant(ctx, &models.SeriesRequest{
    Match: []string{`{job="api"}`},
})

// Health check
err := metricsRouter.HealthCheckWithTenant(ctx, "platformbuilds")
```

### VictoriaLogs Router

```go
logsRouter := services.NewVictoriaLogsRouter(tenantRepo, logger)

ctx := utils.WithTenantID(context.Background(), "chikacafe")

// Query logs
result, err := logsRouter.ExecuteQueryWithTenant(ctx, &models.LogsQLQueryRequest{
    Query: "*",
    Start: time.Now().Add(-1*time.Hour).UnixMilli(),
    End:   time.Now().UnixMilli(),
    Limit: 100,
})

// Store log event
event := map[string]interface{}{
    "_time": time.Now().Format(time.RFC3339),
    "_msg":  "User login",
    "user":  "admin",
}
err := logsRouter.StoreJSONEventWithTenant(ctx, event)
```

### VictoriaTraces Router

```go
tracesRouter := services.NewVictoriaTracesRouter(tenantRepo, logger)

ctx := utils.WithTenantID(context.Background(), "platformbuilds")

// Search traces
result, err := tracesRouter.SearchTracesWithTenant(ctx, &models.TraceSearchRequest{
    Service:   "api-gateway",
    Operation: "GET /users",
    Start:     models.FlexibleTime{Time: time.Now().Add(-1*time.Hour)},
    End:       models.FlexibleTime{Time: time.Now()},
    Limit:     50,
})

// Get operations
operations, err := tracesRouter.GetOperationsWithTenant(ctx, "api-gateway")
```

---

## Working with Tenant Repository

### Create Tenant

```go
tenant := &models.Tenant{
    Name:        "chikacafe",
    DisplayName: "Chika Cafe",
    AdminEmail:  "tony@chikacafe.com",
    Status:      models.TenantStatusActive,
    Deployments: models.TenantDeployments{
        Metrics: models.DeploymentConfig{
            Endpoints: []string{"http://vm-chikacafe.svc:8428"},
            Timeout:   30000,
        },
        Logs: models.DeploymentConfig{
            Endpoints: []string{"http://vl-chikacafe.svc:9428"},
            Timeout:   30000,
        },
        Traces: models.DeploymentConfig{
            Endpoints: []string{"http://vt-chikacafe.svc:7428"},
            Timeout:   30000,
        },
    },
    Quotas: models.TenantQuotas{
        MetricsRetentionDays: 90,
        LogsRetentionDays:    30,
        MaxQueriesPerMinute:  1000,
    },
}

err := tenantRepo.CreateTenant(ctx, tenant)
```

### Get Tenant

```go
tenant, err := tenantRepo.GetTenant(ctx, "platformbuilds")
if err != nil {
    // Handle not found
}
```

### Update Tenant

```go
tenant.DisplayName = "Platform Builds Inc."
err := tenantRepo.UpdateTenant(ctx, tenant)
```

### List Tenants

```go
tenants, total, err := tenantRepo.ListTenants(ctx, 50, 0) // limit, offset
```

### Validate User Access

```go
hasAccess, err := tenantRepo.ValidateUserAccess(ctx, "akhil", "chikacafe")
if !hasAccess {
    // Deny access
}
```

---

## Adding Tenant Support to New Handlers

### Template for New Handler

```go
type NewHandler struct {
    metricsRouter *services.VictoriaMetricsRouter
    logsRouter    *services.VictoriaLogsRouter
    tracesRouter  *services.VictoriaTracesRouter
    tenantRepo    repo.TenantRepository
    logger        logger.Logger
}

func (h *NewHandler) HandleRequest(c *gin.Context) {
    // 1. Extract tenant from context (middleware already validated)
    tenant := c.MustGet("tenant").(*models.Tenant)
    
    // 2. Parse request
    var req RequestModel
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // 3. Create context with tenant
    ctx := utils.WithTenantID(c.Request.Context(), tenant.ID)
    
    // 4. Use router services (they handle tenant routing)
    result, err := h.metricsRouter.ExecuteQueryWithTenant(ctx, &metricsReq)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    // 5. Return response
    c.JSON(200, result)
}
```

---

## Common Patterns

### Multi-Source Query (Metrics + Logs)

```go
func (h *Handler) CorrelatedQuery(c *gin.Context) {
    tenant := c.MustGet("tenant").(*models.Tenant)
    ctx := utils.WithTenantID(c.Request.Context(), tenant.ID)
    
    // Query metrics
    metrics, err := h.metricsRouter.ExecuteQueryWithTenant(ctx, &metricsReq)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    // Query logs
    logs, err := h.logsRouter.ExecuteQueryWithTenant(ctx, &logsReq)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    // Correlate results
    response := map[string]interface{}{
        "metrics": metrics,
        "logs":    logs,
    }
    
    c.JSON(200, response)
}
```

### Checking Tenant Features

```go
tenant := c.MustGet("tenant").(*models.Tenant)

if !tenant.Features.UnifiedQueryEngine {
    c.JSON(403, gin.H{"error": "Feature not enabled for tenant"})
    return
}
```

### Enforcing Quotas

```go
tenant := c.MustGet("tenant").(*models.Tenant)

// Check query limit
canQuery, err := tenantRepo.CheckQuotaLimit(ctx, tenant.ID, "queries_per_minute")
if !canQuery {
    c.JSON(429, gin.H{"error": "Rate limit exceeded"})
    return
}
```

---

## Testing

### Mock Tenant Context

```go
func TestHandler(t *testing.T) {
    // Create test tenant
    tenant := &models.Tenant{
        ID:   "test-tenant",
        Name: "test",
        Deployments: models.TenantDeployments{
            Metrics: models.DeploymentConfig{
                Endpoints: []string{"http://localhost:8428"},
            },
        },
    }
    
    // Set up Gin test context
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Set("tenant", tenant)
    c.Set("tenant_id", tenant.ID)
    
    // Test handler
    handler.HandleRequest(c)
    
    assert.Equal(t, 200, w.Code)
}
```

### Integration Test with Router

```go
func TestMetricsRouter(t *testing.T) {
    // Create mock tenant repo
    mockRepo := &MockTenantRepository{
        tenants: map[string]*models.Tenant{
            "test": {
                ID:   "test",
                Name: "test",
                Deployments: models.TenantDeployments{
                    Metrics: models.DeploymentConfig{
                        Endpoints: []string{"http://localhost:8428"},
                        Timeout:   30000,
                    },
                },
            },
        },
    }
    
    router := services.NewVictoriaMetricsRouter(mockRepo, logger)
    
    ctx := utils.WithTenantID(context.Background(), "test")
    result, err := router.ExecuteQueryWithTenant(ctx, &models.MetricsQLQueryRequest{
        Query: "up",
    })
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

---

## Common Errors and Solutions

### Error: "tenant context required"
**Solution:** Ensure tenant ID is set in context:
```go
ctx := utils.WithTenantID(ctx, tenantID)
```

### Error: "no metrics endpoints configured"
**Solution:** Tenant deployment config is incomplete. Update tenant:
```go
tenant.Deployments.Metrics.Endpoints = []string{"http://..."}
tenantRepo.UpdateTenant(ctx, tenant)
```

### Error: "failed to get metrics service"
**Solution:** Tenant doesn't exist or is invalid. Check tenant status:
```go
tenant, err := tenantRepo.GetTenant(ctx, tenantID)
if tenant.Status != models.TenantStatusActive {
    // Handle inactive tenant
}
```

### Router cache not updating
**Solution:** Invalidate cache after tenant config changes:
```go
metricsRouter.InvalidateTenantCache(tenantID)
```

---

## Best Practices

1. **Always use routers** - Don't create VictoriaMetrics/Logs/Traces services directly
2. **Context propagation** - Always pass tenant context down the call chain
3. **Error handling** - Provide meaningful errors with tenant context
4. **Logging** - Include tenant ID in all log messages
5. **Cache invalidation** - Invalidate router cache when tenant config changes
6. **Health checks** - Verify deployment health before critical operations

---

## Debugging

### Enable Debug Logging

```go
logger.Debug("Querying metrics",
    "tenant_id", tenantID,
    "query", query,
    "endpoints", tenant.Deployments.Metrics.Endpoints)
```

### Check Router Cache

```go
// After getting service
h.logger.Info("Service cache status",
    "tenant_id", tenantID,
    "cached", svc != nil)
```

### Verify Tenant Config

```go
tenant, err := tenantRepo.GetTenant(ctx, tenantID)
h.logger.Info("Tenant config",
    "id", tenant.ID,
    "metrics_endpoints", tenant.Deployments.Metrics.Endpoints,
    "logs_endpoints", tenant.Deployments.Logs.Endpoints,
    "traces_endpoints", tenant.Deployments.Traces.Endpoints)
```

---

## Additional Resources

- [Full Strategy Document](./MULTI_TENANT_STRATEGY.md)
- [Implementation Summary](./MULTI_TENANT_IMPLEMENTATION_SUMMARY.md)
- [API Reference](./api-reference.md)
- [Deployment Guide](./deployment.md)

---

**Questions?** Check the full strategy document or ask the team!
