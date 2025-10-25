# API Reference

This section provides detailed documentation for the MIRADOR-CORE REST API.

## Authentication

All API endpoints (except `/health` and `/ready`) require authentication.

### LDAP/AD Authentication

```bash
curl -X POST https://mirador-core/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "john.doe", "password": "password"}'
```

### OAuth 2.0 Token

```bash
curl -X GET https://mirador-core/api/v1/query \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

## Unified Query API

### Execute Unified Query

Execute a unified query across multiple observability engines.

```http
POST /api/v1/unified/query
```

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
  }
}
```

**Supported Query Types:**
- `metrics` - MetricsQL queries
- `logs` - LogsQL queries
- `traces` - Trace queries
- `correlation` - Cross-engine correlation queries

## Metrics APIs

### Instant Query

```http
POST /api/v1/metrics/query
```

**Request Body:**
```json
{
  "query": "http_requests_total{job=\"api\"}",
  "time": "2025-01-01T00:00:00Z",
  "include_definitions": true
}
```

### Range Query

```http
POST /api/v1/metrics/query_range
```

**Request Body:**
```json
{
  "query": "rate(http_requests_total[5m])",
  "start": "2025-01-01T00:00:00Z",
  "end": "2025-01-01T01:00:00Z",
  "step": "1m"
}
```

### Aggregate Functions

```http
POST /api/v1/metrics/query/aggregate/{function}
```

Available functions: `sum`, `avg`, `count`, `min`, `max`, `quantile`, `topk`, `bottomk`, etc.

**Example - Sum aggregation:**
```bash
curl -X POST https://mirador-core/api/v1/metrics/query/aggregate/sum \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "rate(http_requests_total[5m])"}'
```

## Logs APIs

### Query Logs

```http
POST /api/v1/logs/query
```

**Request Body:**
```json
{
  "query_language": "lucene",
  "search_engine": "lucene",
  "query": "_time:[now-15m TO now] AND level:error AND service:api",
  "limit": 1000
}
```

### Search Logs

```http
POST /api/v1/logs/search
```

**Request Body:**
```json
{
  "query_language": "lucene",
  "query": "_time:1h AND level:error",
  "limit": 100,
  "page_after": {
    "ts": 1640998800000,
    "offset": 0
  }
}
```

## Traces APIs

### Search Traces

```http
POST /api/v1/traces/search
```

**Request Body:**
```json
{
  "query_language": "lucene",
  "query": "_time:[now-15m TO now] AND service:checkout",
  "limit": 100
}
```

### Get Trace by ID

```http
GET /api/v1/traces/{traceId}
```

## AI Analysis APIs

### Root Cause Analysis

```http
POST /api/v1/rca/investigate
```

**Request Body:**
```json
{
  "incident_id": "INC-2025-0831-001",
  "symptoms": ["high_cpu", "connection_timeouts"],
  "time_range": {
    "start": "2025-08-31T14:00:00Z",
    "end": "2025-08-31T15:00:00Z"
  }
}
```

### Predictive Analysis

```http
POST /api/v1/predict/analyze
```

**Request Body:**
```json
{
  "component": "payment-service",
  "time_range": "24h",
  "model_types": ["isolation_forest", "lstm_trend"]
}
```

## Schema Management APIs

### Create Metric Definition

```http
POST /api/v1/schema/metrics
```

**Request Body:**
```json
{
  "metric": "http_requests_total",
  "description": "Total number of HTTP requests",
  "owner": "platform-team",
  "tags": ["domain:web", "category:performance"],
  "author": "john.doe"
}
```

### Bulk Upload Metrics

```http
POST /api/v1/schema/metrics/bulk
```

Upload a CSV file with metric definitions.

## Configuration APIs

### Update Runtime Features

```http
PUT /api/v1/config/features
```

**Request Body:**
```json
{
  "features": {
    "rca_enabled": false,
    "predict_enabled": true,
    "user_settings_enabled": true,
    "rbac_enabled": true
  }
}
```

## Health & Monitoring

### Health Check

```http
GET /api/v1/health
```

Returns basic service health status.

### Readiness Check

```http
GET /api/v1/ready
```

Checks backend connectivity and returns readiness status.

### Prometheus Metrics

```http
GET /api/v1/metrics
```

Returns Prometheus-formatted metrics for monitoring.

## Error Responses

All APIs return standardized error responses:

```json
{
  "error": "Error message",
  "details": "Detailed error information",
  "code": "ERROR_CODE"
}
```

## Rate Limiting

APIs are subject to rate limiting based on tenant configuration:

- Default: 1000 requests per minute
- Burst limit: 2000 requests
- Tenant isolation: Enabled

## Pagination

List endpoints support pagination:

```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "limit": 100,
    "total": 1000,
    "has_more": true
  }
}
```

## WebSocket APIs

Real-time streaming endpoints:

- `/api/v1/ws/metrics` - Real-time metrics stream
- `/api/v1/ws/alerts` - Real-time alerts stream
- `/api/v1/ws/predictions` - Real-time predictions stream

## Complete OpenAPI Specification

For the complete API specification, see the [OpenAPI YAML file](../api/openapi.yaml) in the repository.