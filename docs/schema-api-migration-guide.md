# Schema API Migration Guide

This guide documents the completed migration from separate schema APIs to the unified schema API where KPIs serve as the central schema definitions.

## Migration Status: ✅ COMPLETE

The migration from legacy schema APIs to the unified schema API has been completed. All legacy endpoints have been removed and only the unified API is now available.

## API Changes

### Legacy API Structure (REMOVED)
```
POST /api/v1/schema/labels
POST /api/v1/schema/metrics
POST /api/v1/schema/logs/fields
POST /api/v1/schema/traces/services
POST /api/v1/schema/traces/operations
GET  /api/v1/schema/labels/:name
GET  /api/v1/schema/metrics/:metric
etc.
```

### Current Unified API Structure
```
POST /api/v1/schema/:type
GET  /api/v1/schema/:type/:id
GET  /api/v1/schema/:type
DELETE /api/v1/schema/:type/:id
```

Where `:type` can be:
- `label`
- `metric`
- `log_field`
- `trace_service`
- `trace_operation`
- `kpi`
- `dashboard`
- `layout`
- `user_preferences`

## Migration Status: ✅ COMPLETE

The migration from legacy schema APIs to the unified schema API has been completed. All legacy endpoints have been removed and only the unified API is now available.

## API Changes

### Legacy API Structure (REMOVED)
```
POST /api/v1/schema/labels
POST /api/v1/schema/metrics
POST /api/v1/schema/logs/fields
POST /api/v1/schema/traces/services
POST /api/v1/schema/traces/operations
GET  /api/v1/schema/labels/:name
GET  /api/v1/schema/metrics/:metric
etc.
```

### Current Unified API Structure
```
POST /api/v1/schema/:type
GET  /api/v1/schema/:type/:id
GET  /api/v1/schema/:type
DELETE /api/v1/schema/:type/:id
```

Where `:type` can be:
- `label`
- `metric`
- `log_field`
- `trace_service`
- `trace_operation`
- `kpi`
- `dashboard`
- `layout`
- `user_preferences`

## Migration Examples

### Creating a Label

**Legacy API (REMOVED):**
```bash
POST /api/v1/schema/labels
{
  "tenantId": "tenant1",
  "name": "instance",
  "type": "string",
  "required": false,
  "allowedValues": {"prod": "production", "dev": "development"},
  "description": "Pod or host instance label",
  "category": "infrastructure",
  "sentiment": "NEUTRAL",
  "author": "user@example.com"
}
```

**Current Unified API:**
```bash
POST /api/v1/schema/label
{
  "id": "instance",
  "name": "instance",
  "type": "label",
  "tenantId": "tenant1",
  "category": "infrastructure",
  "sentiment": "NEUTRAL",
  "author": "user@example.com",
  "extensions": {
    "label": {
      "type": "string",
      "required": false,
      "allowedValues": {"prod": "production", "dev": "development"},
      "description": "Pod or host instance label"
    }
  }
}
```

### Creating a Metric

**Legacy API (REMOVED):**
```bash
POST /api/v1/schema/metrics
{
  "tenantId": "tenant1",
  "metric": "http_requests_total",
  "description": "Total HTTP requests",
  "owner": "team@company.com",
  "tags": ["web", "api"],
  "category": "business",
  "sentiment": "POSITIVE",
  "author": "user@example.com"
}
```

**Current Unified API:**
```bash
POST /api/v1/schema/metric
{
  "id": "http_requests_total",
  "name": "http_requests_total",
  "type": "metric",
  "tenantId": "tenant1",
  "tags": ["web", "api"],
  "category": "business",
  "sentiment": "POSITIVE",
  "author": "user@example.com",
  "extensions": {
    "metric": {
      "description": "Total HTTP requests",
      "owner": "team@company.com"
    }
  }
}
```

### Creating a KPI

**Current Unified API:**
```bash
POST /api/v1/schema/kpi
{
  "id": "error_rate",
  "name": "Error Rate",
  "type": "kpi",
  "kind": "business",
  "unit": "%",
  "format": "0.00",
  "query": {
    "type": "prometheus",
    "query": "rate(errors_total[5m]) / rate(http_requests_total[5m]) * 100"
  },
  "thresholds": [
    {
      "level": "warning",
      "operator": "gt",
      "value": 5.0,
      "color": "#FFA500"
    },
    {
      "level": "critical",
      "operator": "gt",
      "value": 10.0,
      "color": "#FF0000"
    }
  ],
  "tags": ["reliability", "slo"],
  "ownerUserId": "user123",
  "visibility": "team",
  "tenantId": "tenant1"
}
```

## Migration Timeline

- **Phase 1** ✅: Implement unified API alongside legacy APIs
- **Phase 2** ✅: Test unified API functionality  
- **Phase 3** ✅: Remove legacy APIs (completed - mirador-core not in production)

## Benefits of Unified API

1. **Consistency:** Single API pattern for all schema types
2. **Extensibility:** Easy to add new schema types
3. **KPIs as Schema:** All schema definitions are now KPIs with rich metadata
4. **Unified Management:** Single interface for KPI management
5. **Better Organization:** Type-based routing with consistent patterns

## Backward Compatibility

The old API endpoints are maintained for backward compatibility. You can migrate at your own pace:

1. **Phase 1:** Continue using old APIs while testing new unified API
2. **Phase 2:** Gradually migrate code to use unified API
3. **Phase 3:** Old APIs can be deprecated after full migration

## Benefits of Unified API

1. **Consistency:** Single API pattern for all schema types
2. **Extensibility:** Easy to add new schema types
3. **KPIs as Schema:** All schema definitions are now KPIs with rich metadata
4. **Unified Management:** Single interface for KPI management
5. **Better Organization:** Type-based routing with consistent patterns

## Migration Checklist

- [ ] Review existing schema API usage in your codebase
- [ ] Test unified API endpoints with your data
- [ ] Update client code to use new API structure
- [ ] Update documentation and API specifications
- [ ] Test with all schema types (labels, metrics, log fields, traces, KPIs)
- [ ] Verify backward compatibility still works during transition
- [ ] Update monitoring and alerting for new API patterns