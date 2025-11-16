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

# KPI Layouts
GET    /api/v1/kpi/layouts
POST   /api/v1/kpi/layouts/batch

# KPI Dashboards
GET    /api/v1/kpi/dashboards
POST   /api/v1/kpi/dashboards
PUT    /api/v1/kpi/dashboards/:id
DELETE /api/v1/kpi/dashboards/:id
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
- `dashboard`
- `layout`
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

### Managing KPI Layouts

**Current KPI API:**
```bash
# Get layouts
GET /api/v1/kpi/layouts?tenantId=tenant1

# Batch update layouts
POST /api/v1/kpi/layouts/batch
{
  "layouts": [
    {
      "kpiId": "error_rate",
      "position": {"x": 0, "y": 0, "w": 6, "h": 4},
      "chartType": "line"
    },
    {
      "kpiId": "response_time",
      "position": {"x": 6, "y": 0, "w": 6, "h": 4},
      "chartType": "gauge"
    }
  ]
}
```

### Managing Dashboards

**Current KPI API:**
```bash
# Create dashboard
POST /api/v1/kpi/dashboards
{
  "name": "API Performance Dashboard",
  "description": "Monitor API performance metrics",
  "shared": false,
  "layout": {
    "panels": []
  }
}

# Update dashboard
PUT /api/v1/kpi/dashboards/{dashboardId}
{
  "name": "Updated API Dashboard",
  "description": "Updated dashboard for monitoring API endpoints",
  "shared": true,
  "layout": {
    "panels": []
  }
}

# Delete dashboard
DELETE /api/v1/kpi/dashboards/{dashboardId}
```

## Migration Timeline

- **Phase 1** ✅: Implement KPI APIs alongside legacy schema APIs
- **Phase 2** ✅: Test KPI API functionality  
- **Phase 3** ✅: Remove all legacy schema APIs (completed)

## Benefits of KPI Management APIs

1. **Unified Management:** Single interface for KPI definitions, layouts, and dashboards
2. **Rich Metadata:** KPIs include thresholds, formatting, units, and visualization preferences
3. **Dashboard Integration:** Direct support for dashboard creation and management
4. **Layout Control:** Granular control over KPI positioning and chart types
5. **User Preferences:** Integrated user preference management via `/config/user-preferences`
6. **Multi-tenancy:** Built-in tenant isolation and access control

## Current API Endpoints

All KPI management is now handled through the `/api/v1/kpi/*` endpoints:

### KPI Definitions
- `GET /api/v1/kpi/defs` - List all KPI definitions
- `POST /api/v1/kpi/defs` - Create or update a KPI definition
- `DELETE /api/v1/kpi/defs/:id` - Delete a KPI definition

### KPI Layouts
- `GET /api/v1/kpi/layouts` - Get KPI layout configurations
- `POST /api/v1/kpi/layouts/batch` - Batch update KPI layouts

### KPI Dashboards
- `GET /api/v1/kpi/dashboards` - List dashboards
- `POST /api/v1/kpi/dashboards` - Create a new dashboard
- `PUT /api/v1/kpi/dashboards/:id` - Update an existing dashboard
- `DELETE /api/v1/kpi/dashboards/:id` - Delete a dashboard

## Migration Checklist

- [x] Review existing schema API usage in your codebase
- [x] Test KPI API endpoints with your data
- [x] Update client code to use new KPI API structure
- [x] Update documentation and API specifications
- [x] Remove all legacy schema API endpoints
- [x] Update monitoring and alerting for new API patterns
- [x] Verify KPI definitions, layouts, and dashboards are working correctly