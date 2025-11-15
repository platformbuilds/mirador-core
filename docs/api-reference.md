# API Reference

This section provides comprehensive documentation for the MIRADOR-CORE REST API, covering all endpoints, request/response schemas, authentication, and usage examples.

## Table of Contents

- [Authentication](#authentication)
- [Unified Query API](#unified-query-api)
- [UQL (Unified Query Language) API](#uql-unified-query-language-api)
- [Metrics APIs](#metrics-apis)
- [Logs APIs](#logs-apis)
- [Traces APIs](#traces-apis)
- [AI Analysis APIs](#ai-analysis-apis)
- [KPI Management APIs](#kpi-management-apis)
- [Configuration APIs](#configuration-apis)
- [User Management APIs](#user-management-apis)
- [RBAC APIs](#rbac-apis)
- [Tenant Management APIs](#tenant-management-apis)
- [Session Management APIs](#session-management-apis)
- [WebSocket APIs](#websocket-apis)
- [Health & Monitoring](#health--monitoring)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)
- [Pagination](#pagination)

## Authentication

All API endpoints (except `/health`, `/ready`, and `/metrics` for Prometheus scraping) require authentication via RBAC (Role-Based Access Control).

### Authentication Methods

#### LDAP/AD Authentication

```bash
curl -X POST https://mirador-core/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "john.doe", "password": "password"}'
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-01-01T02:00:00Z",
  "user": {
    "id": "user-123",
    "username": "john.doe",
    "tenant_id": "tenant-456"
  }
}
```

#### OAuth 2.0 Bearer Token

```bash
curl -X GET https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

#### Token Validation

```bash
curl -X POST https://mirador-core/api/v1/auth/validate \
  -H "Authorization: Bearer <token>"
```

### RBAC Permissions

Each endpoint requires specific permissions. Permission requirements are documented for each endpoint.

**Common Permission Patterns:**
- `metrics.read` - Read access to metrics data
- `logs.read` - Read access to logs data
- `traces.read` - Read access to traces data
- `unified.read` - Access to unified query capabilities
- `rca.read` - Access to root cause analysis
- `config.read` - Read access to configuration
- `kpi.read` - Read access to KPI definitions
- `dashboard.read` - Read access to dashboards
- `rbac.admin` - RBAC administration
- `session.admin` - Session management
- `metrics.admin` - Metrics metadata administration

## Unified Query API

The Unified Query API provides intelligent routing and parallel execution across multiple observability engines (VictoriaMetrics, VictoriaLogs, VictoriaTraces).

### Execute Unified Query

Execute queries across multiple engines with intelligent routing and parallel execution.

```text
POST /api/v1/unified/query
```

**RBAC Permission:** `unified.read`

**Request Body:**
```json
{
  "query": {
    "id": "unified-query-1",
    "type": "metrics",
    "query": "http_requests_total{job=\"api\"}",
    "start_time": "2025-01-01T00:00:00Z",
    "end_time": "2025-01-01T01:00:00Z",
    "timeout": "30s",
    "cache_options": {
      "enabled": true,
      "ttl": "5m"
    }
  },
  "tenant_id": "tenant-123"
}
```

**Supported Query Types:**
- `metrics` - MetricsQL queries against VictoriaMetrics
- `logs` - LogsQL queries against VictoriaLogs
- `traces` - Trace queries against VictoriaTraces
- `correlation` - Cross-engine correlation queries

**Response:**
```json
{
  "result": {
    "query_id": "unified-query-1",
    "type": "metrics",
    "status": "success",
    "data": [
      {
        "metric": {"__name__": "http_requests_total", "job": "api"},
        "values": [[1640998800, "1234"]]
      }
    ],
    "metadata": {
      "engine": "victoriametrics",
      "execution_time_ms": 45,
      "cached": false,
      "warnings": []
    }
  }
}
```

### Execute Unified Correlation Query

Execute correlation queries that span multiple observability engines.

```text
POST /api/v1/unified/correlation
```

**RBAC Permission:** `unified.read`

**Request Body:**
```json
{
  "query": {
    "id": "correlation-1",
    "correlation_type": "temporal",
    "engines": ["logs", "metrics"],
    "queries": {
      "logs": "level:error AND service:api",
      "metrics": "http_requests_total{status=\"500\"}"
    },
    "time_window": "5m",
    "start_time": "2025-01-01T00:00:00Z",
    "end_time": "2025-01-01T01:00:00Z"
  },
  "tenant_id": "tenant-123"
}
```

### Unified Query Metadata

Get metadata about available engines and their capabilities.

```text
GET /api/v1/unified/metadata
```

**RBAC Permission:** `unified.read`

**Response:**
```json
{
  "engines": {
    "victoriametrics": {
      "status": "healthy",
      "version": "1.95.0",
      "capabilities": ["metricsql", "prometheus", "graphite"]
    },
    "victorialogs": {
      "status": "healthy",
      "version": "0.4.0",
      "capabilities": ["logs", "lucene", "bleve"]
    },
    "victoriatraces": {
      "status": "healthy",
      "version": "0.4.0",
      "capabilities": ["jaeger", "zipkin", "otlp"]
    }
  },
  "correlation_engine": {
    "status": "healthy",
    "patterns": ["temporal", "causal", "service_graph"]
  }
}
```

### Unified Search

Perform federated search across all engines.

```text
POST /api/v1/unified/search
```

**RBAC Permission:** `unified.read`

**Request Body:**
```json
{
  "query": "error AND service:api",
  "engines": ["logs", "traces"],
  "start_time": "2025-01-01T00:00:00Z",
  "end_time": "2025-01-01T01:00:00Z",
  "limit": 100
}
```

### Unified Health Check

Check health of all unified query components.

```text
GET /api/v1/unified/health
```

**RBAC Permission:** `unified.read`

### Unified Statistics

Get execution statistics and performance metrics.

```text
GET /api/v1/unified/stats
```

**RBAC Permission:** `unified.read`

## UQL (Unified Query Language) API

UQL provides a unified query language for executing complex queries across multiple observability engines with advanced correlation, aggregation, and transformation capabilities.

### Execute UQL Query

Execute a UQL query with full parsing, optimization, and translation capabilities.

```text
POST /api/v1/uql/query
```

**RBAC Permission:** `unified.read`

**Request Body:**
```json
{
  "query": {
    "id": "uql-query-1",
    "query": "SELECT service, count(*) FROM logs:error WHERE level='error' GROUP BY service",
    "start_time": "2025-01-01T00:00:00Z",
    "end_time": "2025-01-01T01:00:00Z",
    "timeout": "30s",
    "cache_options": {
      "enabled": true,
      "ttl": "5m"
    }
  },
  "tenant_id": "tenant-123"
}
```

**UQL Examples:**

```sql
-- Basic correlations
logs:error AND metrics:high_latency
logs:exception WITHIN 5m OF metrics:cpu_usage > 80

-- Advanced correlations
logs:error NEAR 1m OF traces:status:error
logs:timeout BEFORE 30s OF metrics:response_time > 5000

-- SELECT queries
SELECT service, level, count(*) FROM logs:error WHERE level='error' GROUP BY service, level
SELECT service, avg(response_time) FROM metrics:http_requests WHERE status='200' GROUP BY service

-- Aggregation queries
COUNT(*) FROM logs:error WHERE level='error'
AVG(response_time) FROM metrics:http_requests GROUP BY service
PERCENTILE_95(response_time) FROM metrics:http_requests

-- Join queries
logs:error JOIN traces:service:error ON service WITHIN 5m
metrics:http_requests > 1000 JOIN logs:error ON service NEAR 1m
```

**Response:**
```json
{
  "result": {
    "query_id": "uql-query-1",
    "type": "correlation",
    "status": "success",
    "data": [
      {
        "service": "api-gateway",
        "count": 42
      }
    ],
    "metadata": {
      "engine_results": {
        "logs": {"records": 150, "execution_time_ms": 120},
        "metrics": {"records": 25, "execution_time_ms": 80}
      },
      "total_records": 150,
      "data_sources": ["logs", "metrics"],
      "warnings": [],
      "optimization_applied": ["predicate_pushdown", "cost_based_planning"]
    },
    "execution_time_ms": 245,
    "cached": false
  }
}
```

### Validate UQL Syntax

Validate UQL query syntax without execution.

```text
POST /api/v1/uql/validate
```

**RBAC Permission:** `unified.read`

**Request Body:**
```json
{
  "query": "SELECT service, count(*) FROM logs:error WHERE level='error' GROUP BY service"
}
```

**Response:**
```json
{
  "valid": true,
  "parsed_query": {
    "type": "select",
    "engines": ["logs"],
    "fields": ["service", "count(*)"],
    "from": "logs:error",
    "where": "level='error'",
    "group_by": ["service"]
  },
  "warnings": []
}
```

### Explain UQL Query Plan

Get the execution plan for a UQL query including optimization steps.

```text
POST /api/v1/uql/explain
```

**RBAC Permission:** `unified.read`

**Request Body:**
```json
{
  "query": {
    "id": "uql-explain-1",
    "query": "logs:error WITHIN 5m OF metrics:cpu_usage > 80"
  }
}
```

**Response:**
```json
{
  "query": "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
  "plan": {
    "original_query": "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
    "parsed_ast": {
      "type": "correlation",
      "left": {"engine": "logs", "query": "error"},
      "right": {"engine": "metrics", "query": "cpu_usage > 80"},
      "operator": "WITHIN",
      "window": "5m"
    },
    "optimization_steps": [
      {
        "step": "predicate_pushdown",
        "description": "Push filters down to individual engines",
        "applied": true,
        "cost_reduction": 0.35
      },
      {
        "step": "parallel_execution",
        "description": "Execute queries in parallel",
        "applied": true,
        "engines": ["logs", "metrics"]
      }
    ],
    "translated_queries": {
      "logs": "_time:[now-5m TO now] AND error",
      "metrics": "cpu_usage > 80"
    },
    "estimated_cost": 0.45,
    "estimated_rows": 1250
  }
}
```

## Metrics APIs

Comprehensive MetricsQL API with VictoriaMetrics integration, supporting instant queries, range queries, aggregations, and metadata operations.

### Instant Query

Execute instant MetricsQL queries.

```text
POST /api/v1/metrics/query
```

**RBAC Permission:** `metrics.read`

**Request Body:**
```json
{
  "query": "http_requests_total{job=\"api\"}",
  "time": "2025-01-01T00:00:00Z",
  "include_definitions": true,
  "tenant_id": "tenant-123"
}
```

**Response:**
```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {
          "__name__": "http_requests_total",
          "job": "api",
          "instance": "api-1:8080"
        },
        "value": [1640998800, "1234"]
      }
    ]
  },
  "execution_time_ms": 45
}
```

### Range Query

Execute range MetricsQL queries with time series data.

```text
POST /api/v1/metrics/query_range
```

**RBAC Permission:** `metrics.read`

**Request Body:**
```json
{
  "query": "rate(http_requests_total[5m])",
  "start": "2025-01-01T00:00:00Z",
  "end": "2025-01-01T01:00:00Z",
  "step": "1m",
  "tenant_id": "tenant-123"
}
```

### Get Metric Names

Retrieve all available metric names.

```text
GET /api/v1/metrics/names
```

**RBAC Permission:** `metrics.read`

**Query Parameters:**
- `match[]` - Metric name patterns to match
- `limit` - Maximum number of results

### Get Series

Retrieve time series data for metrics.

```text
GET /api/v1/metrics/series
```

**RBAC Permission:** `metrics.read`

**Query Parameters:**
- `match[]` - Series selectors
- `start` - Start time
- `end` - End time

### Get Labels

Retrieve all label names or values.

```text
POST /api/v1/metrics/labels
GET /api/v1/metrics/label/{name}/values
```

**RBAC Permission:** `metrics.read`

**POST Request Body:**
```json
{
  "match[]": ["http_requests_total"],
  "start": "2025-01-01T00:00:00Z",
  "end": "2025-01-01T01:00:00Z"
}
```

### Rollup Functions

Execute rollup aggregation functions.

```text
POST /api/v1/metrics/query/rollup/{function}
POST /api/v1/metrics/query/rollup/{function}/range
```

**RBAC Permission:** `metrics.read`

**Available Functions:** `increase`, `rate`, `irate`, `increase_pure`, `rate_pure`, `irate_pure`, `avg_over_time`, `min_over_time`, `max_over_time`, `sum_over_time`, `count_over_time`, `quantile_over_time`, `stddev_over_time`, `stdvar_over_time`

### Transform Functions

Execute transformation functions.

```text
POST /api/v1/metrics/query/transform/{function}
POST /api/v1/metrics/query/transform/{function}/range
```

**RBAC Permission:** `metrics.read`

**Available Functions:** `abs`, `absent`, `ceil`, `exp`, `floor`, `ln`, `log10`, `log2`, `round`, `sqrt`, `acos`, `acosh`, `asin`, `asinh`, `atan`, `atanh`, `cos`, `cosh`, `sin`, `sinh`, `tan`, `tanh`, `deg`, `rad`, `hour`, `minute`, `month`, `year`, `day_of_month`, `day_of_week`, `days_in_month`

### Label Functions

Execute label manipulation functions.

```text
POST /api/v1/metrics/query/label/{function}
POST /api/v1/metrics/query/label/{function}/range
```

**RBAC Permission:** `metrics.read`

**Available Functions:** `label_replace`, `label_join`, `label_set`, `label_drop`, `label_keep`

### Aggregate Functions

Execute aggregation functions.

```text
POST /api/v1/metrics/query/aggregate/{function}
POST /api/v1/metrics/query/aggregate/{function}/range
```

**RBAC Permission:** `metrics.read`

**Available Functions:** `sum`, `avg`, `count`, `min`, `max`, `group`, `stddev`, `stdvar`, `topk`, `bottomk`, `quantile`, `count_values`, `absent_over_time`

## Logs APIs

Comprehensive logs API with VictoriaLogs integration, supporting LogsQL queries, Lucene/Bleve search engines, and D3-friendly endpoints.

### Execute Logs Query

Execute LogsQL queries against VictoriaLogs.

```text
POST /api/v1/logs/query
```

**RBAC Permission:** `logs.read`

**Request Body:**
```json
{
  "query": "_time:[now-15m TO now] AND level:error AND service:api",
  "query_language": "logs",
  "search_engine": "lucene",
  "start": "2025-01-01T00:00:00Z",
  "end": "2025-01-01T01:00:00Z",
  "limit": 1000,
  "tenant_id": "tenant-123"
}
```

**Supported Query Languages:**
- `logs` - Native LogsQL
- `lucene` - Lucene syntax (auto-translated to LogsQL)
- `bleve` - Bleve query syntax

### Get Streams

Retrieve available log streams.

```text
GET /api/v1/logs/streams
```

**RBAC Permission:** `logs.read`

### Get Fields

Retrieve available log fields.

```text
GET /api/v1/logs/fields
```

**RBAC Permission:** `logs.read`

### Export Logs

Export logs in various formats.

```text
POST /api/v1/logs/export
```

**RBAC Permission:** `logs.read`

**Request Body:**
```json
{
  "query": "level:error",
  "format": "json",
  "start": "2025-01-01T00:00:00Z",
  "end": "2025-01-01T01:00:00Z"
}
```

**Supported Formats:** `json`, `csv`, `ndjson`

### Store Event

Store JSON events in VictoriaLogs (for AI engines).

```text
POST /api/v1/logs/store
```

**RBAC Permission:** `logs.read`

### D3-Friendly Logs APIs

#### Histogram

Get time-bucketed log counts for histogram visualization.

```text
GET /api/v1/logs/histogram
```

**RBAC Permission:** `logs.read`

**Query Parameters:**
- `query` - Log query (LogsQL or Lucene)
- `query_language` - Query language (`lucene`, `logs`)
- `start` - Start time (Unix ms)
- `end` - End time (Unix ms)
- `step` - Bucket size in ms (default: 60000)
- `limit` - Max results
- `sampling` - Sampling rate (default: 1)

**Response:**
```json
{
  "buckets": [
    {"ts": 1640998800000, "count": 15},
    {"ts": 1640998860000, "count": 8}
  ],
  "stats": {"buckets": 60, "sampleN": 1},
  "sampled": false
}
```

#### Facets

Get field value distributions for facet visualization.

```text
GET /api/v1/logs/facets
```

**RBAC Permission:** `logs.read`

**Query Parameters:**
- `fields` - Comma-separated field names
- `query` - Filter query
- `query_language` - Query language
- `start` - Start time
- `end` - End time
- `limit` - Max values per field (default: 20)
- `sampling` - Sampling rate

**Response:**
```json
{
  "facets": [
    {
      "field": "level",
      "buckets": [
        {"key": "error", "count": 45},
        {"key": "info", "count": 120}
      ]
    }
  ],
  "stats": {"fields": 1, "sampleN": 1},
  "sampled": false
}
```

#### Search Logs

Search logs with pagination support.

```text
POST /api/v1/logs/search
```

**RBAC Permission:** `logs.read`

**Request Body:**
```json
{
  "query": "_time:1h AND level:error",
  "query_language": "lucene",
  "limit": 100,
  "page_after": {
    "ts": 1640998800000,
    "offset": 0
  },
  "start": "2025-01-01T00:00:00Z",
  "end": "2025-01-01T01:00:00Z"
}
```

**Response:**
```json
{
  "rows": [
    {
      "_time": "2025-01-01T00:15:30.123Z",
      "level": "error",
      "message": "Connection timeout",
      "service": "api"
    }
  ],
  "fields": ["_time", "level", "message", "service"],
  "next_page_after": {"ts": 1640999130123, "offset": 1},
  "stats": {"count": 100, "streaming_stats": {...}}
}
```

#### Tail Logs

WebSocket endpoint for real-time log tailing.

```text
GET /api/v1/logs/tail
```

**RBAC Permission:** `logs.read`

**Query Parameters:**
- `query` - Log query
- `query_language` - Query language

## Traces APIs

Jaeger-compatible traces API with VictoriaTraces integration.

### Get Services

List all services with traces.

```text
GET /api/v1/traces/services
```

**RBAC Permission:** `traces.read`

**Response:**
```json
{
  "data": ["api-gateway", "user-service", "payment-service"]
}
```

### Get Operations

List operations for a specific service.

```text
GET /api/v1/traces/services/{service}/operations
```

**RBAC Permission:** `traces.read`

**Response:**
```json
{
  "data": ["GET /api/users", "POST /api/payments", "GET /health"]
}
```

### Get Trace by ID

Retrieve a complete trace by its ID.

```text
GET /api/v1/traces/{traceId}
```

**RBAC Permission:** `traces.read`

**Response:**
```json
{
  "data": [
    {
      "traceID": "abc123...",
      "spans": [...],
      "processes": {...}
    }
  ]
}
```

### Search Traces

Search traces with advanced filtering.

```text
POST /api/v1/traces/search
```

**RBAC Permission:** `traces.read`

**Request Body:**
```json
{
  "query": "_time:[now-15m TO now] AND service:checkout",
  "query_language": "lucene",
  "search_engine": "lucene",
  "service": "checkout",
  "operation": "POST /checkout",
  "tags": "error=true,http.status_code=500",
  "min_duration": "100ms",
  "max_duration": "5s",
  "start": "2025-01-01T00:00:00Z",
  "end": "2025-01-01T01:00:00Z",
  "limit": 100,
  "tenant_id": "tenant-123"
}
```

**Supported Search Engines:** `lucene`, `bleve`

### Get Flame Graph

Get flame graph data for a single trace.

```text
GET /api/v1/traces/{traceId}/flamegraph
```

**RBAC Permission:** `traces.read`

**Query Parameters:**
- `mode` - Flame graph mode (`duration`, `count`, `self_duration`)

### Search Flame Graph

Aggregate flame graph over search results.

```text
POST /api/v1/traces/flamegraph/search
```

**RBAC Permission:** `traces.read`

**Request Body:**
```json
{
  "query": "service:api AND duration > 100ms",
  "start": "2025-01-01T00:00:00Z",
  "end": "2025-01-01T01:00:00Z",
  "limit": 100
}
```

## AI Analysis APIs

### Root Cause Analysis

#### Get Active Correlations

Retrieve currently active correlation patterns.

```text
GET /api/v1/rca/correlations
```

**RBAC Permission:** `rca.read`

#### Start Investigation

Initiate root cause analysis investigation.

```text
POST /api/v1/rca/investigate
```

**RBAC Permission:** `rca.read`

**Request Body:**
```json
{
  "incident_id": "INC-2025-0831-001",
  "symptoms": ["high_cpu", "connection_timeouts"],
  "time_range": {
    "start": "2025-08-31T14:00:00Z",
    "end": "2025-08-31T15:00:00Z"
  },
  "tenant_id": "tenant-123"
}
```

#### Get Failure Patterns

Retrieve known failure patterns.

```text
GET /api/v1/rca/patterns
```

**RBAC Permission:** `rca.read`

#### Get Service Graph

Generate service dependency graph.

```text
POST /api/v1/rca/service-graph
```

**RBAC Permission:** `rca.read`

#### Store Correlation

Store correlation analysis results.

```text
POST /api/v1/rca/store
```

**RBAC Permission:** `rca.read`

## KPI Management APIs

### KPI Definitions

#### Get KPI Definitions

Retrieve KPI definitions.

```text
GET /api/v1/kpi/defs
```

**RBAC Permission:** `kpi.read`

#### Create/Update KPI Definition

Create or update a KPI definition.

```text
POST /api/v1/kpi/defs
```

**RBAC Permission:** `kpi.read`

**Request Body:**
```json
{
  "id": "api_response_time",
  "name": "API Response Time",
  "description": "Average response time for API endpoints",
  "query": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
  "unit": "seconds",
  "thresholds": {
    "warning": 1.0,
    "critical": 2.0
  },
  "tags": ["domain:api", "category:performance"]
}
```

#### Delete KPI Definition

Delete a KPI definition.

```text
DELETE /api/v1/kpi/defs/{id}
```

**RBAC Permission:** `kpi.read`

### KPI Layouts

#### Get KPI Layouts

Retrieve KPI dashboard layouts.

```text
GET /api/v1/kpi/layouts
```

**RBAC Permission:** `kpi.read`

#### Batch Update KPI Layouts

Update multiple KPI layouts.

```text
POST /api/v1/kpi/layouts/batch
```

**RBAC Permission:** `kpi.read`

## Configuration APIs

### Data Sources

#### Get Data Sources

Retrieve configured data sources.

```text
GET /api/v1/config/datasources
```

**RBAC Permission:** `config.read`

#### Add Data Source

Add a new data source configuration.

```text
POST /api/v1/config/datasources
```

**RBAC Permission:** `config.read`

### Integrations

#### Get Integrations

Retrieve integration configurations.

```text
GET /api/v1/config/integrations
```

**RBAC Permission:** `config.read`

### User Preferences

#### Get User Preferences

```text
GET /api/v1/config/user-preferences
```

**RBAC Permission:** `config.read`

#### Create User Preferences

```text
POST /api/v1/config/user-preferences
```

**RBAC Permission:** `config.read`

#### Update User Preferences

```text
PUT /api/v1/config/user-preferences
```

**RBAC Permission:** `config.read`

#### Delete User Preferences

```text
DELETE /api/v1/config/user-preferences
```

**RBAC Permission:** `config.read`

### Feature Flags

#### Get Feature Flags

Retrieve runtime feature flags.

```text
GET /api/v1/config/features
```

**RBAC Permission:** `config.read`

#### Update Feature Flags

Update runtime feature flags.

```text
PUT /api/v1/config/features
```

**RBAC Permission:** `config.read`

#### Reset Feature Flags

Reset feature flags to defaults.

```text
POST /api/v1/config/features/reset
```

**RBAC Permission:** `config.read`

### gRPC Endpoints

#### Get gRPC Endpoints

Retrieve configured gRPC endpoints.

```text
GET /api/v1/config/grpc/endpoints
```

**RBAC Permission:** `config.read`

#### Update gRPC Endpoints

Update gRPC endpoint configurations.

```text
PUT /api/v1/config/grpc/endpoints
```

**RBAC Permission:** `config.read`

#### Reset gRPC Endpoints

Reset gRPC endpoints to defaults.

```text
POST /api/v1/config/grpc/endpoints/reset
```

**RBAC Permission:** `config.read`

## User Management APIs

### MiradorAuth (Global Admin Only)

#### Create MiradorAuth

```text
POST /api/v1/auth/users
```

**RBAC Permission:** Global Admin Only

#### Get MiradorAuth

```text
GET /api/v1/auth/users/{userId}
```

**RBAC Permission:** Global Admin Only

#### Update MiradorAuth

```text
PUT /api/v1/auth/users/{userId}
```

**RBAC Permission:** Global Admin Only

#### Delete MiradorAuth

```text
DELETE /api/v1/auth/users/{userId}
```

**RBAC Permission:** Global Admin Only

### AuthConfig (Tenant Admin Only)

#### Create AuthConfig

```text
POST /api/v1/auth/config
```

**RBAC Permission:** Tenant Admin Only

#### Get AuthConfig

```text
GET /api/v1/auth/config/{tenantId}
```

**RBAC Permission:** Tenant Admin Only

#### Update AuthConfig

```text
PUT /api/v1/auth/config/{tenantId}
```

**RBAC Permission:** Tenant Admin Only

#### Delete AuthConfig

```text
DELETE /api/v1/auth/config/{tenantId}
```

**RBAC Permission:** Tenant Admin Only

## RBAC APIs

### Roles

#### Get Roles

```text
GET /api/v1/rbac/roles
```

**RBAC Permission:** `rbac.admin`

#### Create Role

```text
POST /api/v1/rbac/roles
```

**RBAC Permission:** `rbac.admin`

#### Get User Roles

```text
GET /api/v1/rbac/users/{userId}/roles
```

**RBAC Permission:** `rbac.admin`

### Permissions

#### Get Permissions

```text
GET /api/v1/rbac/permissions
```

**RBAC Permission:** `rbac.admin`

#### Create Permission

```text
POST /api/v1/rbac/permissions
```

**RBAC Permission:** `rbac.admin`

#### Update Permission

```text
PUT /api/v1/rbac/permissions/{permissionId}
```

**RBAC Permission:** `rbac.admin`

#### Delete Permission

```text
DELETE /api/v1/rbac/permissions/{permissionId}
```

**RBAC Permission:** `rbac.admin`

### Groups

#### Get Groups

```text
GET /api/v1/rbac/groups
```

**RBAC Permission:** `rbac.admin`

#### Create Group

```text
POST /api/v1/rbac/groups
```

**RBAC Permission:** `rbac.admin`

#### Update Group

```text
PUT /api/v1/rbac/groups/{groupName}
```

**RBAC Permission:** `rbac.admin`

#### Delete Group

```text
DELETE /api/v1/rbac/groups/{groupName}
```

**RBAC Permission:** `rbac.admin`

#### Add Users to Group

```text
PUT /api/v1/rbac/groups/{groupName}/users
```

**RBAC Permission:** `rbac.admin`

#### Remove Users from Group

```text
DELETE /api/v1/rbac/groups/{groupName}/users
```

**RBAC Permission:** `rbac.admin`

#### Get Group Members

```text
GET /api/v1/rbac/groups/{groupName}/members
```

**RBAC Permission:** `rbac.admin`

### Role Bindings

#### Get Role Bindings

```text
GET /api/v1/rbac/role-bindings
```

**RBAC Permission:** `rbac.admin`

#### Create Role Binding

```text
POST /api/v1/rbac/role-bindings
```

**RBAC Permission:** `rbac.admin`

#### Update Role Binding

```text
PUT /api/v1/rbac/role-bindings/{bindingId}
```

**RBAC Permission:** `rbac.admin`

#### Delete Role Binding

```text
DELETE /api/v1/rbac/role-bindings/{bindingId}
```

**RBAC Permission:** `rbac.admin`

### RBAC Audit

#### Get Audit Events

```text
GET /api/v1/rbac/audit
```

**RBAC Permission:** Tenant Admin Only

#### Get Audit Event

```text
GET /api/v1/rbac/audit/{eventId}
```

**RBAC Permission:** Tenant Admin Only

#### Get Audit Summary

```text
GET /api/v1/rbac/audit/summary
```

**RBAC Permission:** Tenant Admin Only

#### Get Audit Events by Subject

```text
GET /api/v1/rbac/audit/subject/{subjectId}
```

**RBAC Permission:** Tenant Admin Only

## Tenant Management APIs

### Global Admin Routes

#### List Tenants

```text
GET /api/v1/tenants
```

**RBAC Permission:** Global Admin Only

#### Create Tenant

```text
POST /api/v1/tenants
```

**RBAC Permission:** Global Admin Only

#### Delete Tenant

```text
DELETE /api/v1/tenants/{tenantId}
```

**RBAC Permission:** Global Admin Only

### Tenant Admin Routes

#### Get Tenant

```text
GET /api/v1/tenants/{tenantId}
```

**RBAC Permission:** Tenant Admin Only

#### Update Tenant

```text
PUT /api/v1/tenants/{tenantId}
```

**RBAC Permission:** Tenant Admin Only

### Tenant-User Association

#### Create Tenant User

```text
POST /api/v1/tenants/{tenantId}/users
```

**RBAC Permission:** Tenant Admin Only

#### List Tenant Users

```text
GET /api/v1/tenants/{tenantId}/users
```

**RBAC Permission:** Tenant Admin Only

#### Get Tenant User

```text
GET /api/v1/tenants/{tenantId}/users/{userId}
```

**RBAC Permission:** Tenant Admin Only

#### Update Tenant User

```text
PUT /api/v1/tenants/{tenantId}/users/{userId}
```

**RBAC Permission:** Tenant Admin Only

#### Delete Tenant User

```text
DELETE /api/v1/tenants/{tenantId}/users/{userId}
```

**RBAC Permission:** Tenant Admin Only

## User Management APIs

### Global Admin Routes

#### List Users

```text
GET /api/v1/users
```

**RBAC Permission:** Global Admin Only

#### Create User

```text
POST /api/v1/users
```

**RBAC Permission:** Global Admin Only

#### Get User

```text
GET /api/v1/users/{id}
```

**RBAC Permission:** Global Admin Only

#### Update User

```text
PUT /api/v1/users/{id}
```

**RBAC Permission:** Global Admin Only

#### Delete User

```text
DELETE /api/v1/users/{id}
```

**RBAC Permission:** Global Admin Only

## Session Management APIs

### Get Active Sessions

```text
GET /api/v1/sessions/active
```

**RBAC Permission:** `session.admin`

### Invalidate Session

```text
POST /api/v1/sessions/invalidate
```

**RBAC Permission:** `session.admin`

### Get User Sessions

```text
GET /api/v1/sessions/user/{userId}
```

**RBAC Permission:** `session.admin`

## WebSocket APIs

Real-time streaming endpoints for live data:

### Metrics Stream

```text
GET /api/v1/ws/metrics
```

**RBAC Permission:** `metrics.read`

Streams real-time metrics data via WebSocket.

### Alerts Stream

```text
GET /api/v1/ws/alerts
```

**RBAC Permission:** `metrics.read`

Streams real-time alert notifications via WebSocket.

## Health & Monitoring

### Health Check

```text
GET /api/v1/health
```

Returns basic service health status.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-01T00:00:00Z",
  "version": "1.0.0"
}
```

### Readiness Check

```text
GET /api/v1/ready
```

Checks backend connectivity and returns readiness status.

### Microservices Status

```text
GET /api/v1/microservices/status
```

Returns status of all connected microservices.

### Prometheus Metrics

```text
GET /api/v1/metrics
```

Returns Prometheus-formatted metrics for monitoring (no authentication required for scraping).

## Error Handling

All APIs return standardized error responses:

```json
{
  "error": "Error message",
  "details": "Detailed error information",
  "code": "ERROR_CODE",
  "timestamp": "2025-01-01T00:00:00Z"
}
```

**Common Error Codes:**
- `INVALID_REQUEST` - Malformed request
- `UNAUTHORIZED` - Authentication required
- `FORBIDDEN` - Insufficient permissions
- `NOT_FOUND` - Resource not found
- `TIMEOUT` - Request timeout
- `BACKEND_ERROR` - Backend service error
- `RATE_LIMITED` - Rate limit exceeded

## Rate Limiting

APIs are subject to rate limiting based on tenant configuration:

- **Default Limits:**
  - Metrics queries: 1000 requests per minute
  - Logs queries: 500 requests per minute
  - Traces queries: 500 requests per minute
  - Unified queries: 200 requests per minute

- **Burst Limits:** 2x the rate limit for burst traffic
- **Tenant Isolation:** Rate limits are enforced per tenant
- **Headers:** Rate limit status returned in response headers
  - `X-RateLimit-Limit` - Request limit
  - `X-RateLimit-Remaining` - Remaining requests
  - `X-RateLimit-Reset` - Reset time

## Pagination

List endpoints support cursor-based pagination:

```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "limit": 100,
    "total": 1000,
    "has_more": true,
    "next_cursor": "eyJwYWdlIjoxLCJsaW1pdCI6MTAwfQ=="
  }
}
```

**Cursor Parameters:**
- `cursor` - Base64-encoded pagination cursor
- `limit` - Maximum items per page (default: 50, max: 1000)

## Caching

APIs support intelligent caching with Valkey cluster:

- **Cache Headers:**
  - `X-Cache` - Cache status (`HIT`, `MISS`)
  - `X-Cache-TTL` - Time-to-live in seconds

- **Cache Control:**
  - `Cache-Control: no-cache` - Bypass cache
  - `X-Cache-Refresh` - Force cache refresh

## Complete OpenAPI Specification

For the complete API specification with detailed schemas, see the [OpenAPI YAML file](../api/openapi.yaml) in the repository.