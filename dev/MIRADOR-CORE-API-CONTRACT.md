# Mirador Core API Contract

**Version:** 1.0.0  
**Date:** 6 November 2025  
**Purpose:** API specification for mirador-core microservice integration with mirador-ui

---

## Overview

This document defines the exact JSON schemas and API endpoints that **mirador-core** must implement to integrate with the **mirador-ui** dashboard application. All schemas are based on the production TypeScript types and validated structures currently used in mirador-ui.

**Critical Architecture Principle:**  
**mirador-ui is a completely stateless service** - it does not persist any data locally (no localStorage, no cookies, no client-side persistence). All application state, user preferences, dashboards, layouts, and configurations MUST be stored in and retrieved from the mirador-core backend. This ensures:
- Multiple browser sessions stay synchronized
- UI instances are truly stateless and horizontally scalable
- All data survives browser refresh, cache clear, or device switching
- Single source of truth for all application state

---

## Base URL

```
http://mirador-core:8080/api/v1
```

---

## 1. KPI Definitions API

### 1.1 Get All KPI Definitions

**Endpoint:** `GET /api/v1/kpi/defs`

**Query Parameters:** None

**Response:** `200 OK`

```json
{
  "defs": [
    {
      "id": "kpi-conv-rate",
      "kind": "business",
      "name": "Conversion Rate",
      "unit": "%",
      "format": "pct",
      "query": {
        "type": "formula",
        "expr": "orders / sessions * 100",
        "inputs": {
          "orders": {
            "type": "metric",
            "ref": "checkout.orders.count",
            "aggregator": "sum",
            "range": { "from": "now-1h", "to": "now" }
          },
          "sessions": {
            "type": "metric",
            "ref": "web.sessions.count",
            "aggregator": "sum",
            "range": { "from": "now-1h", "to": "now" }
          }
        }
      },
      "thresholds": [
        { "when": "<", "value": 2, "status": "warn" },
        { "when": "<", "value": 1.5, "status": "crit" }
      ],
      "tags": ["funnel", "revenue"],
      "sparkline": { "windowMins": 60 },
      "ownerUserId": "user-123",
      "visibility": "org"
    },
    {
      "id": "kpi-response-time",
      "kind": "tech",
      "name": "API Response Time",
      "unit": "ms",
      "format": "duration",
      "query": {
        "type": "metric",
        "ref": "api.latency.p95",
        "aggregator": "p95",
        "range": { "from": "now-5m", "to": "now" }
      },
      "thresholds": [
        { "when": ">", "value": 500, "status": "warn" },
        { "when": ">", "value": 1000, "status": "crit" }
      ],
      "tags": ["performance", "api"],
      "sparkline": { "windowMins": 30 },
      "ownerUserId": "user-456",
      "visibility": "team"
    }
  ]
}
```

---

### 1.2 Create or Update KPI Definition

**Endpoint:** `POST /api/v1/kpi/defs`

**Request Body:**

```json
{
  "id": "kpi-revenue-daily",
  "kind": "business",
  "name": "Daily Revenue",
  "unit": "$",
  "format": "currency",
  "query": {
    "type": "metric",
    "ref": "billing.revenue.total",
    "aggregator": "sum",
    "range": { "from": "now-24h", "to": "now" }
  },
  "thresholds": [
    { "when": "<", "value": 10000, "status": "warn" },
    { "when": "<", "value": 5000, "status": "crit" }
  ],
  "tags": ["revenue", "billing"],
  "sparkline": { "windowMins": 1440 },
  "ownerUserId": "user-789",
  "visibility": "org"
}
```

**Response:** `200 OK`

Returns the same object as submitted.

**Notes:**
- If `id` exists, performs an update (upsert)
- If `id` is new, creates a new KPI definition
- All fields except `id`, `kind`, `name`, and `query` are optional

---

### 1.3 Delete KPI Definition

**Endpoint:** `DELETE /api/v1/kpi/defs/:id`

**Path Parameters:**
- `id` (string, required): KPI definition ID

**Response:** `200 OK`

```json
{
  "ok": true
}
```

**Side Effects:**
- Should cascade delete related layouts in `kpi_layouts` table
- Should NOT delete historical metric data

---

## 2. KPI Layouts API

### 2.1 Get Layouts for Dashboard

**Endpoint:** `GET /api/v1/kpi/layouts`

**Query Parameters:**
- `dashboard` (string, optional): Dashboard ID (default: "default")

**Response:** `200 OK`

```json
{
  "layouts": {
    "kpi-conv-rate": { "x": 0, "y": 0, "w": 4, "h": 3 },
    "kpi-response-time": { "x": 4, "y": 0, "w": 3, "h": 2 },
    "kpi-revenue-daily": { "x": 0, "y": 3, "w": 2, "h": 2 }
  }
}
```

**Notes:**
- Returns empty object `{}` if no layouts exist for the dashboard
- Each key is a KPI definition ID
- Grid system: 12 columns, unlimited rows

---

### 2.2 Batch Save Layouts

**Endpoint:** `POST /api/v1/kpi/layouts/batch`

**Request Body:**

```json
{
  "dashboardId": "default",
  "layouts": {
    "kpi-conv-rate": { "x": 0, "y": 0, "w": 4, "h": 3 },
    "kpi-response-time": { "x": 4, "y": 0, "w": 3, "h": 2 },
    "kpi-revenue-daily": { "x": 7, "y": 0, "w": 5, "h": 2 }
  }
}
```

**Response:** `200 OK`

```json
{
  "ok": true
}
```

**Notes:**
- Performs upsert for each layout (insert or update)
- Should use transaction to ensure atomicity
- Only updates provided KPI layouts, does not delete missing ones

---

## 3. User Preferences API

### 3.1 Get User Preferences

**Endpoint:** `GET /api/v1/user/preferences`

**Query Parameters:**
- `userId` (string, optional): User ID (default: current authenticated user)

**Response:** `200 OK`

```json
{
  "userId": "user-123",
  "theme": "dark",
  "sidebarCollapsed": false,
  "defaultDashboard": "default",
  "timezone": "America/Los_Angeles",
  "keyboardHintSeen": true,
  "miradorCoreEndpoint": "https://api.mirador-core.com",
  "preferences": {
    "compactMode": false,
    "animationsEnabled": true
  }
}
```

**Notes:**
- If user has no preferences, return default values:
  ```json
  {
    "userId": "user-123",
    "theme": "system",
    "sidebarCollapsed": false,
    "defaultDashboard": "default",
    "timezone": "UTC",
    "keyboardHintSeen": false,
    "miradorCoreEndpoint": null,
    "preferences": {}
  }
  ```
- `preferences` field can contain arbitrary JSON for extensibility
- This endpoint should be called on app initialization to restore all UI state

---

### 3.2 Update User Preferences

**Endpoint:** `POST /api/v1/user/preferences`

**Request Body:**

```json
{
  "userId": "user-123",
  "theme": "light",
  "sidebarCollapsed": true,
  "keyboardHintSeen": true,
  "preferences": {
    "compactMode": true
  }
}
```

**Response:** `200 OK`

```json
{
  "ok": true
}
```

**Notes:**
- Performs partial update (merge with existing preferences)
- Only provided fields are updated
- This endpoint is called whenever user changes any UI preference (theme, sidebar, etc.)
- Should handle high-frequency updates gracefully (e.g., debouncing on server side)

---

## 4. Dashboard Management API

### 4.1 Get All Dashboards

**Endpoint:** `GET /api/v1/dashboards`

**Query Parameters:**
- `userId` (string, optional): Filter by owner (default: current user)
- `visibility` (enum, optional): Filter by visibility ("private", "team", "org")

**Response:** `200 OK`

```json
{
  "dashboards": [
    {
      "id": "default",
      "name": "Default Dashboard",
      "ownerUserId": "system",
      "visibility": "org",
      "createdAt": "2025-11-01T00:00:00Z",
      "updatedAt": "2025-11-06T12:00:00Z"
    },
    {
      "id": "dash-revenue-2025",
      "name": "Revenue Dashboard 2025",
      "ownerUserId": "user-123",
      "visibility": "team",
      "createdAt": "2025-11-05T10:30:00Z",
      "updatedAt": "2025-11-06T09:15:00Z"
    }
  ]
}
```

### 4.2 Create Dashboard

**Endpoint:** `POST /api/v1/dashboards`

**Request Body:**

```json
{
  "id": "dash-operations",
  "name": "Operations Dashboard",
  "ownerUserId": "user-456",
  "visibility": "team"
}
```

**Response:** `201 Created`

Returns the created dashboard object.

### 4.3 Update Dashboard

**Endpoint:** `PUT /api/v1/dashboards/:id`

**Request Body:**

```json
{
  "name": "Updated Dashboard Name",
  "visibility": "org"
}
```

**Response:** `200 OK`

Returns the updated dashboard object.

### 4.4 Delete Dashboard

**Endpoint:** `DELETE /api/v1/dashboards/:id`

**Response:** `200 OK`

```json
{
  "ok": true
}
```

**Side Effects:**
- Cascade deletes all associated KPI layouts
- Cannot delete the "default" dashboard (system protected)

---

## 5. Data Models

### 5.1 KpiDef Object

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `id` | string | ✓ | - | Unique identifier (e.g., "kpi-conv-rate") |
| `kind` | enum | ✓ | - | "business" or "tech" |
| `name` | string | ✓ | - | Display name for the KPI |
| `unit` | string | | null | Unit label (e.g., "%", "ms", "$") |
| `format` | enum | | "number" | "number", "pct", "currency", "duration" |
| `query` | KpiQuery | ✓ | - | Query definition (metric or formula) |
| `thresholds` | KpiThreshold[] | | null | Alert thresholds for status colors |
| `tags` | string[] | | null | Categorization tags |
| `sparkline` | object | | null | `{ "windowMins": number }` |
| `ownerUserId` | string | | null | User who created the KPI |
| `visibility` | enum | | "org" | "private", "team", "org" |

---

### 5.2 KpiQuery Object (Type: "metric")

**Structure:**

```json
{
  "type": "metric",
  "ref": "api.latency.p95",
  "aggregator": "p95",
  "range": { "from": "now-5m", "to": "now" }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | "metric" | ✓ | Query type identifier |
| `ref` | string | ✓ | Metric identifier/path |
| `aggregator` | enum | | "avg", "sum", "p95", "p99" |
| `range` | TimeRange | ✓ | Time range for the query |

**TimeRange Object:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `from` | string | ✓ | Start time (ISO8601 or relative like "now-1h") |
| `to` | string | ✓ | End time (ISO8601 or relative like "now") |

---

### 5.3 KpiQuery Object (Type: "formula")

**Structure:**

```json
{
  "type": "formula",
  "expr": "orders / sessions * 100",
  "inputs": {
    "orders": {
      "type": "metric",
      "ref": "checkout.orders.count",
      "aggregator": "sum",
      "range": { "from": "now-1h", "to": "now" }
    },
    "sessions": {
      "type": "metric",
      "ref": "web.sessions.count",
      "aggregator": "sum",
      "range": { "from": "now-1h", "to": "now" }
    }
  }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | "formula" | ✓ | Query type identifier |
| `expr` | string | ✓ | Math expression (uses input variable names) |
| `inputs` | Record<string, KpiQuery> | ✓ | Named inputs (can be metric or nested formula) |

**Expression Rules:**
- Variables must match keys in `inputs`
- Supports: `+`, `-`, `*`, `/`, `()`, numbers
- Variables: alphanumeric + underscore (e.g., "orders", "total_sessions")
- Regex validation: `^[0-9a-zA-Z_\s\.\+\-\*\/\(\)]+$`

---

### 5.4 KpiThreshold Object

**Structure:**

```json
{
  "when": "<",
  "value": 2,
  "status": "warn"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `when` | enum | ✓ | Comparison operator: ">", ">=", "<", "<=", "between", "outside" |
| `value` | number \| [number, number] | ✓ | Threshold value (single for operators, tuple for between/outside) |
| `status` | enum | ✓ | Status color: "ok" (green), "warn" (yellow), "crit" (red) |

**Examples:**

```json
// Simple threshold
{ "when": ">", "value": 500, "status": "warn" }

// Between range (inclusive)
{ "when": "between", "value": [80, 100], "status": "ok" }

// Outside range
{ "when": "outside", "value": [10, 90], "status": "crit" }
```

---

### 5.5 Layout Object

**Structure:**

```json
{
  "x": 0,
  "y": 0,
  "w": 4,
  "h": 3
}
```

**Fields:**

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| `x` | integer | ✓ | 0-11 | Grid column position (0-indexed) |
| `y` | integer | ✓ | ≥0 | Grid row position (0-indexed) |
| `w` | integer | ✓ | 2-12 | Width in grid units |
| `h` | integer | ✓ | 2-12 | Height in grid units |

**Grid System:**
- 12-column grid (Bootstrap-style)
- Minimum card size: 2×2
- Maximum card size: 12×12 (full width)
- Rows are auto-generated (no limit)

---

### 5.6 Dashboard Object

**Structure:**

```json
{
  "id": "default",
  "name": "Default Dashboard",
  "ownerUserId": "system",
  "visibility": "org",
  "createdAt": "2025-11-01T00:00:00Z",
  "updatedAt": "2025-11-06T12:00:00Z"
}
```

**Fields:**

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| `id` | string | ✓ | unique | Dashboard identifier |
| `name` | string | ✓ | 1-255 chars | Display name |
| `ownerUserId` | string | ✓ | - | User who created the dashboard |
| `visibility` | enum | ✓ | private/team/org | Access level |
| `createdAt` | string | ✓ | ISO8601 | Creation timestamp |
| `updatedAt` | string | ✓ | ISO8601 | Last update timestamp |

---

### 5.7 UserPreferences Object

**Structure:**

```json
{
  "userId": "user-123",
  "theme": "dark",
  "sidebarCollapsed": false,
  "defaultDashboard": "default",
  "timezone": "America/Los_Angeles",
  "keyboardHintSeen": true,
  "miradorCoreEndpoint": "https://api.mirador-core.com",
  "preferences": {
    "compactMode": false,
    "animationsEnabled": true,
    "autoRefreshInterval": 30000
  }
}
```

**Fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `userId` | string | ✓ | - | User identifier |
| `theme` | enum | | "system" | "light", "dark", "system" |
| `sidebarCollapsed` | boolean | | false | Sidebar collapse state |
| `defaultDashboard` | string | | "default" | Default dashboard ID on login |
| `timezone` | string | | "UTC" | User timezone (IANA format) |
| `keyboardHintSeen` | boolean | | false | Whether user has seen keyboard hint |
| `miradorCoreEndpoint` | string | | null | Custom API endpoint (admin override) |
| `preferences` | object | | {} | Extensible JSON for future preferences |

---

## 6. Error Handling

### 6.1 Error Response Format

All error responses must follow this structure:

```json
{
  "error": "Validation failed: Invalid query expression"
}
```

### 6.2 HTTP Status Codes

| Code | Meaning | When to Use |
|------|---------|-------------|
| `200` | OK | Successful request |
| `201` | Created | Resource created successfully |
| `400` | Bad Request | Validation error, missing required fields |
| `404` | Not Found | Resource doesn't exist |
| `500` | Internal Server Error | Unexpected server error |
| `503` | Service Unavailable | Database connection failed |

### 6.3 Common Error Scenarios

**Missing Required Field:**
```json
{
  "error": "Validation failed: 'id' is required"
}
```

**Invalid Enum Value:**
```json
{
  "error": "Validation failed: 'kind' must be 'business' or 'tech'"
}
```

**Invalid Formula Expression:**
```json
{
  "error": "Validation failed: Invalid formula expression"
}
```

**Database Unavailable:**
```json
{
  "error": "Database not available"
}
```

---

## 7. Database Schema Reference

For mirador-core implementation, here's the recommended MySQL schema:

### 7.1 kpi_definitions

```sql
CREATE TABLE kpi_definitions (
  id VARCHAR(255) PRIMARY KEY,
  kind ENUM('business', 'tech') NOT NULL,
  name VARCHAR(255) NOT NULL,
  unit VARCHAR(50),
  format ENUM('number', 'pct', 'currency', 'duration') DEFAULT 'number',
  query_type ENUM('metric', 'formula') NOT NULL,
  query_data JSON NOT NULL,
  thresholds JSON,
  tags JSON,
  sparkline_config JSON,
  owner_user_id VARCHAR(255),
  visibility ENUM('private', 'team', 'org') DEFAULT 'org',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_kind (kind),
  INDEX idx_owner (owner_user_id),
  INDEX idx_visibility (visibility)
);
```

### 7.2 kpi_layouts

```sql
CREATE TABLE kpi_layouts (
  kpi_id VARCHAR(255) NOT NULL,
  dashboard_id VARCHAR(255) NOT NULL,
  x INT NOT NULL,
  y INT NOT NULL,
  w INT NOT NULL,
  h INT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (kpi_id, dashboard_id),
  FOREIGN KEY (kpi_id) REFERENCES kpi_definitions(id) ON DELETE CASCADE,
  FOREIGN KEY (dashboard_id) REFERENCES dashboards(id) ON DELETE CASCADE
);
```

### 7.3 dashboards

```sql
CREATE TABLE dashboards (
  id VARCHAR(255) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  owner_user_id VARCHAR(255),
  visibility ENUM('private', 'team', 'org') DEFAULT 'org',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### 7.4 user_preferences

```sql
CREATE TABLE user_preferences (
  user_id VARCHAR(255) PRIMARY KEY,
  theme ENUM('light', 'dark', 'system') DEFAULT 'system',
  sidebar_collapsed BOOLEAN DEFAULT FALSE,
  default_dashboard VARCHAR(255),
  timezone VARCHAR(100) DEFAULT 'UTC',
  keyboard_hint_seen BOOLEAN DEFAULT FALSE,
  mirador_core_endpoint VARCHAR(500),
  preferences JSON,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (default_dashboard) REFERENCES dashboards(id) ON DELETE SET NULL
);
```

**Important Notes:**
- All UI state is stored here - no localStorage on client side
- The `preferences` JSON field allows extensibility for new UI features
- `mirador_core_endpoint` allows admin users to override API endpoint
- `keyboard_hint_seen` prevents showing hints repeatedly

---

---

## 8. TypeScript Type Definitions

For reference, here are the TypeScript types from mirador-ui:

```typescript
export type KpiKind = "business" | "tech";

export type KpiQuery =
  | {
      type: "metric";
      ref: string;
      aggregator?: "avg" | "sum" | "p95" | "p99";
      range: { from: string; to: string };
    }
  | {
      type: "formula";
      expr: string;
      inputs: Record<string, KpiQuery>;
    };

export type KpiThreshold = {
  when: ">" | ">=" | "<" | "<=" | "between" | "outside";
  value: number | [number, number];
  status: "ok" | "warn" | "crit";
};

export type KpiDef = {
  id: string;
  kind: KpiKind;
  name: string;
  unit?: string;
  format?: "number" | "pct" | "currency" | "duration";
  query: KpiQuery;
  thresholds?: KpiThreshold[];
  tags?: string[];
  sparkline?: { windowMins: number };
  ownerUserId?: string;
  visibility?: "private" | "team" | "org";
  layout?: { w: number; h: number; x?: number; y?: number };
};

export type Dashboard = {
  id: string;
  name: string;
  ownerUserId: string;
  visibility: "private" | "team" | "org";
  createdAt: string;
  updatedAt: string;
};

export type UserPreferences = {
  userId: string;
  theme?: "light" | "dark" | "system";
  sidebarCollapsed?: boolean;
  defaultDashboard?: string;
  timezone?: string;
  keyboardHintSeen?: boolean;
  miradorCoreEndpoint?: string;
  preferences?: Record<string, any>;
};
```

---

## 9. Validation Rules

---

## 9. Validation Rules

### 9.1 Zod Schema (Reference)

The frontend uses Zod for validation. Here are the rules:

```typescript
// ID validation
id: z.string().min(1)

// Name validation
name: z.string().min(1)

// Formula expression validation
expr: z.string().regex(/^[0-9a-zA-Z_\s\.\+\-\*\/\(\)]+$/)

// Sparkline window validation
windowMins: z.number().int().positive()

// Layout dimensions validation
w: z.number().int().positive().min(2).max(12)
h: z.number().int().positive().min(2).max(12)
x: z.number().int().min(0).max(11)
y: z.number().int().min(0)
```

### 9.2 Business Rules

1. **KPI ID must be unique** across all definitions
2. **Metric ref** should follow dot-notation convention (e.g., "service.metric.aggregation")
3. **Formula inputs** must be used in the expression
4. **Thresholds** should be ordered by severity (ok → warn → crit)
5. **Layout coordinates** must not cause overlapping (UI handles this, but good to validate)
6. **Dashboard "default"** must always exist and cannot be deleted
7. **Visibility hierarchy**: private < team < org
8. **User preferences must persist** - mirador-ui is stateless and relies on backend for all state
9. **Theme changes should sync immediately** to provide consistent experience across sessions

---

## 10. Example Requests

### 10.1 Create Business KPI

```bash
curl -X POST http://mirador-core:8080/api/v1/kpi/defs \
  -H "Content-Type: application/json" \
  -d '{
    "id": "kpi-cart-abandonment",
    "kind": "business",
    "name": "Cart Abandonment Rate",
    "unit": "%",
    "format": "pct",
    "query": {
      "type": "formula",
      "expr": "(added - purchased) / added * 100",
      "inputs": {
        "added": {
          "type": "metric",
          "ref": "cart.items.added",
          "aggregator": "sum",
          "range": { "from": "now-24h", "to": "now" }
        },
        "purchased": {
          "type": "metric",
          "ref": "cart.items.purchased",
          "aggregator": "sum",
          "range": { "from": "now-24h", "to": "now" }
        }
      }
    },
    "thresholds": [
      { "when": ">", "value": 70, "status": "warn" },
      { "when": ">", "value": 85, "status": "crit" }
    ],
    "tags": ["funnel", "checkout"],
    "visibility": "org"
  }'
```

### 10.2 Create Tech KPI

```bash
curl -X POST http://mirador-core:8080/api/v1/kpi/defs \
  -H "Content-Type: application/json" \
  -d '{
    "id": "kpi-error-rate",
    "kind": "tech",
    "name": "API Error Rate",
    "unit": "%",
    "format": "pct",
    "query": {
      "type": "formula",
      "expr": "errors / total * 100",
      "inputs": {
        "errors": {
          "type": "metric",
          "ref": "api.requests.5xx",
          "aggregator": "sum",
          "range": { "from": "now-5m", "to": "now" }
        },
        "total": {
          "type": "metric",
          "ref": "api.requests.total",
          "aggregator": "sum",
          "range": { "from": "now-5m", "to": "now" }
        }
      }
    },
    "thresholds": [
      { "when": ">", "value": 1, "status": "warn" },
      { "when": ">", "value": 5, "status": "crit" }
    ],
    "tags": ["reliability", "api"],
    "sparkline": { "windowMins": 15 }
  }'
```

### 10.3 Save Dashboard Layout

```bash
curl -X POST http://mirador-core:8080/api/v1/kpi/layouts/batch \
  -H "Content-Type: application/json" \
  -d '{
    "dashboardId": "default",
    "layouts": {
      "kpi-conv-rate": { "x": 0, "y": 0, "w": 4, "h": 3 },
      "kpi-cart-abandonment": { "x": 4, "y": 0, "w": 4, "h": 3 },
      "kpi-error-rate": { "x": 8, "y": 0, "w": 4, "h": 3 }
    }
  }'
```

### 10.4 Update User Preferences (Theme Change)

```bash
curl -X POST http://mirador-core:8080/api/v1/user/preferences \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "user-123",
    "theme": "dark"
  }'
```

### 10.5 Update User Preferences (Sidebar State)

```bash
curl -X POST http://mirador-core:8080/api/v1/user/preferences \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "user-123",
    "sidebarCollapsed": true,
    "keyboardHintSeen": true
  }'
```

### 10.6 Create Custom Dashboard

```bash
curl -X POST http://mirador-core:8080/api/v1/dashboards \
  -H "Content-Type: application/json" \
  -d '{
    "id": "dash-revenue-2025",
    "name": "Revenue Dashboard 2025",
    "ownerUserId": "user-123",
    "visibility": "team"
  }'
```

---

## 11. Integration Notes

### 11.1 Environment Variables

mirador-ui expects these environment variables (for proxy configuration):

```bash
VITE_API_BASE_URL=http://mirador-core:8080/api/v1
```

### 11.2 CORS Configuration

mirador-core must enable CORS for the UI origin:

```javascript
// Express.js example
app.use(cors({
  origin: ['http://localhost:3000', 'http://mirador-ui:3000'],
  credentials: true
}));
```

### 11.3 Authentication

Current implementation assumes no authentication. For production:

1. Add `Authorization: Bearer <token>` header support
2. Extract user ID from token for user preferences
3. Filter KPIs by visibility and user permissions

### 11.4 Stateless UI Architecture

**Critical Implementation Requirements:**

1. **No Client-Side Persistence:**
   - mirador-ui MUST NOT use localStorage, sessionStorage, or cookies for state
   - All state reads/writes go through mirador-core API
   - This ensures horizontal scalability and session consistency

2. **Initial App Load Sequence:**
   ```typescript
   // On app mount
   1. Fetch user preferences: GET /api/v1/user/preferences
   2. Apply theme, sidebar state, etc. from backend response
   3. Fetch KPI definitions: GET /api/v1/kpi/defs
   4. Fetch layouts for default dashboard: GET /api/v1/kpi/layouts?dashboard={defaultDashboard}
   ```

3. **State Update Flow:**
   ```typescript
   // Example: User toggles theme
   1. Update local React state (optimistic UI update)
   2. POST /api/v1/user/preferences with new theme
   3. On success: Keep UI as-is
   4. On failure: Revert to previous theme, show error
   ```

4. **Dashboard Navigation:**
   ```typescript
   // Example: User switches dashboard
   1. Update URL/route to new dashboard ID
   2. GET /api/v1/kpi/layouts?dashboard={newDashboardId}
   3. Render KPIs with fetched layouts
   ```

5. **Performance Considerations:**
   - Cache API responses in memory (React state/Valtio) during session
   - Debounce frequent updates (e.g., layout changes during drag)
   - Use optimistic UI updates for better perceived performance
   - Re-fetch on visibility change (user returns to tab) to stay fresh

### 11.5 Database Initialization

Default data to seed on first startup:

```sql
-- Default dashboard
INSERT INTO dashboards (id, name, visibility) 
VALUES ('default', 'Default Dashboard', 'org');

-- Example KPIs (optional)
INSERT INTO kpi_definitions (id, kind, name, format, query_type, query_data, thresholds, tags)
VALUES 
('kpi-default-conv', 'business', 'Conversion Rate', 'pct', 'formula', 
 '{"type":"formula","expr":"orders / sessions * 100","inputs":{...}}',
 '[{"when":"<","value":2,"status":"warn"}]',
 '["funnel","revenue"]');
```

**Important:** Also seed a default user with preferences:

```sql
-- Default user preferences
INSERT INTO user_preferences (user_id, theme, sidebar_collapsed, default_dashboard, timezone, keyboard_hint_seen)
VALUES ('default-user', 'system', FALSE, 'default', 'UTC', FALSE);
```

---

## 12. State Migration Checklist

For mirador-ui frontend developers to remove all localStorage usage:

### 12.1 Files to Modify

| File | Current State | Required Changes |
|------|---------------|------------------|
| `src/state/kpi-store.ts` | Uses localStorage for KPI defs | Remove lsLoad/lsSave, use API exclusively |
| `src/state/widgets.ts` | Uses localStorage for layouts | Replace persist/restore with API calls |
| `src/contexts/ThemeContext.tsx` | Uses localStorage for theme | Fetch/save theme via user preferences API |
| `src/App.tsx` | Uses localStorage for sidebar, hints | Fetch/save via user preferences API |
| `src/pages/Admin/index.tsx` | Uses localStorage for API endpoint | Fetch/save via user preferences API |

### 12.2 Required API Integrations

1. **On App Mount (`App.tsx`):**
   ```typescript
   useEffect(() => {
     async function initApp() {
       // Fetch user preferences first
       const prefs = await fetch('/api/v1/user/preferences').then(r => r.json());
       setTheme(prefs.theme || 'system');
       setSidebarCollapsed(prefs.sidebarCollapsed || false);
       setKeyboardHintSeen(prefs.keyboardHintSeen || false);
       
       // Then fetch KPIs and layouts
       await ensureKpiDefsLoaded(); // Already implemented
       await loadLayouts(prefs.defaultDashboard || 'default');
     }
     initApp();
   }, []);
   ```

2. **Theme Changes (`ThemeContext.tsx`):**
   ```typescript
   const toggleTheme = async () => {
     const newTheme = theme === 'light' ? 'dark' : 'light';
     setTheme(newTheme); // Optimistic update
     await fetch('/api/v1/user/preferences', {
       method: 'POST',
       body: JSON.stringify({ theme: newTheme })
     });
   };
   ```

3. **Layout Changes (`widgets.ts`):**
   ```typescript
   setLayout: async (id, layout) => {
     widgetState.layoutById[id] = layout;
     // Debounce batch save
     debouncedSaveLayouts();
   },
   
   saveLayouts: async () => {
     await fetch('/api/v1/kpi/layouts/batch', {
       method: 'POST',
       body: JSON.stringify({
         dashboardId: currentDashboard,
         layouts: widgetState.layoutById
       })
     });
   }
   ```

### 12.3 Testing Checklist

- [ ] Open app in incognito mode → preferences load from backend
- [ ] Change theme → persists after refresh
- [ ] Toggle sidebar → state preserved after refresh
- [ ] Drag KPI cards → layout persists after refresh
- [ ] Create new KPI → appears after refresh
- [ ] Delete KPI → removed after refresh
- [ ] Clear browser cache → no data loss
- [ ] Open same user in 2 tabs → changes sync (requires periodic refetch or websocket)

---

## 13. Changelog

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2025-11-06 | Initial specification based on mirador-ui requirements |
| 1.1.0 | 2025-11-06 | Added stateless architecture requirements, dashboard management API, expanded user preferences, state migration checklist |

---

## 14. Contact & Support

**Project:** Mirador Stack  
**Component:** mirador-core API  
**Maintainer:** Platform Engineering Team  
**Documentation:** `/development/MIRADOR-CORE-API-CONTRACT.md`

For questions or clarifications, refer to:
- TypeScript types: `/src/lib/kpi-types.ts`
- Validation schemas: `/src/lib/kpi-schema.ts`
- Current mock API: `/mock/server-mysql.js`
