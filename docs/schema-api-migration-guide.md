# Schema API Migration Guide

This guide documents the completed migration from separate schema APIs to the unified KPI Management APIs where KPIs serve as the central schema definitions.

## Migration Status: ✅ COMPLETE

The migration from legacy schema APIs to the unified KPI Management APIs has been completed. All legacy schema endpoints have been removed and only the KPI APIs are now available.

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

### Legacy Unified Schema API (REMOVED)
```
POST /api/v1/schema/:type
GET  /api/v1/schema/:type/:id
GET  /api/v1/schema/:type
DELETE /api/v1/schema/:type/:id
```

### Current KPI Management API Structure
```
# KPI Definitions
GET    /api/v1/kpi/defs
POST   /api/v1/kpi/defs
DELETE /api/v1/kpi/defs/:id
```

## Migration Status: ✅ COMPLETE

The migration from legacy schema APIs to the KPI Management APIs has been completed. All legacy endpoints have been removed and only the KPI APIs are now available.

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
- `user_preferences`

## Migration Examples

### Creating a KPI Definition

**Current KPI API:**
```bash
POST /api/v1/kpi/defs
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

### Getting KPI Definitions

**Current KPI API:**
```bash
GET /api/v1/kpi/defs?tenantId=tenant1
```

Response:
```json
{
  "kpis": [
    {
      "id": "error_rate",
      "name": "Error Rate",
      "type": "kpi",
      "kind": "business",
      "unit": "%",
      "format": "0.00",
      "query": "rate(http_requests_total{status=~\"5..\"}[5m]) / rate(http_requests_total[5m]) * 100",
      "thresholds": [
        {"level": "warning", "value": 5.0},
        {"level": "critical", "value": 10.0}
      ],
      "tags": ["reliability", "slo"],
      "ownerUserId": "user123",
      "visibility": "team"
    }
  ],
  "total": 1
}
```

## Migration Timeline

- **Phase 1** ✅: Implement KPI APIs alongside legacy schema APIs
- **Phase 2** ✅: Test KPI API functionality  
- **Phase 3** ✅: Remove all legacy schema APIs (completed)

## Benefits of KPI Management APIs

1. **Unified Management:** Single interface for KPI definitions
2. **Rich Metadata:** KPIs include thresholds, formatting, units, and visualization preferences
3. **User Preferences:** Integrated user preference management via `/config/user-preferences`
4. **Multi-tenancy:** Built-in tenant isolation and access control

## Current API Endpoints

All KPI management is now handled through the `/api/v1/kpi/*` endpoints:

### KPI Definitions
- `GET /api/v1/kpi/defs` - List all KPI definitions
- `POST /api/v1/kpi/defs` - Create or update a KPI definition
- `DELETE /api/v1/kpi/defs/:id` - Delete a KPI definition

## Migration Checklist

- [x] Review existing schema API usage in your codebase
- [x] Test KPI API endpoints with your data
- [x] Update client code to use new KPI API structure
- [x] Update documentation and API specifications
- [x] Remove all legacy schema API endpoints
- [x] Update monitoring and alerting for new API patterns
- [x] Verify KPI definitions are working correctly