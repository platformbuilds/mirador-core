# Unified Query

This page is the concise reference for Mirador Core's Unified Query engine. It describes the main endpoints, exact request shapes the HTTP handlers accept (code is the source of truth), supported data engines (metrics / logs / traces), and short examples you can use to quickly get started.

## Overview

Mirador Core exposes a single, flexible Unified Query API that lets you query metrics, logs, traces and run higher-level operations (search, correlation) from one place. The Unified Query engine routes requests to the appropriate backend(s), merges results, and returns consistent metadata and diagnostics.

Supported endpoints (examples):

- GET /api/v1/unified/metadata — information about supported engines and capabilities
- GET /api/v1/unified/health — health status for unified query and sub-engines
- GET /api/v1/unified/stats — runtime stats and cache summary
- POST /api/v1/unified/query — execute a unified query (metrics/logs/traces)
- POST /api/v1/unified/search — higher-level search operation across logs/traces
- POST /api/v1/unified/correlation — run correlation operations (delegates to correlation engine)
- POST /api/v1/unified/uql (UQL execution / validate / explain are available under UQL routes)

## Common request/response patterns

1) Unified Query — accepted payload shapes

The Unified Query HTTP handlers accept one of two JSON shapes:

- Wrapped form: `{ "query": { ...UnifiedQuery fields... } }`  (this is the canonical request wrapper `UnifiedQueryRequest`)
- Direct form: a top-level UnifiedQuery JSON object (e.g. `{"id":"...","query":"..."}`)

When unmarshalling a UnifiedQuery the code expects the UnifiedQuery JSON fields to be the ones defined in the model: `id`, `type`, `query`, `start_time`, `end_time`, `timeout`, `parameters`, `correlation_options`, `cache_options`.

Important: the `start_time` / `end_time` fields in a `UnifiedQuery` are the snake_case fields used by the UnifiedQuery model and must be parseable as RFC3339 timestamps (ISO-8601). Example: "2025-11-01T12:00:00Z".

Response (simplified):
```json
{
  "query_id": "uuid",
  "type": "metrics",
  "status": "success",
  "data": { ... },
  "metadata": { "engine_results": { ... } }
}
```
Examples — wrapped and direct forms

Wrapped (recommended):
```json
{
  "query": {
    "id": "req-123",
    "type": "metrics",
    "query": "up{job=\"api\"}",
    "start_time": "2025-11-01T12:00:00Z",
    "end_time": "2025-11-01T12:05:00Z"
  }
}
```
Direct form (accepted):
```json
{
  "id": "req-123",
  "type": "metrics",
  "query": "up{job=\"api\"}",
  "start_time": "2025-11-01T12:00:00Z",
  "end_time": "2025-11-01T12:05:00Z"
}
```json
Response metadata includes execution timing, per-engine record counts, and data_sources used.

Notes on metrics ranges / step

When a UnifiedQuery contains `start_time` and `end_time`, the engine treats it as a range query for metrics. The current implementation uses a default step of `15s` for range queries (the server constructs an internal MetricsQLRangeQueryRequest with Step="15s"). That step is not currently configurable via the top-level UnifiedQuery fields, unless you use engine-specific APIs (for now assume the 15s default).

Logs & traces

Logs/traces queries should be supplied with `query` text and (optionally) `start_time`/`end_time` where applicable. Logs are executed as time-window searches using epoch milliseconds internally; the public UnifiedQuery JSON uses `start_time`/`end_time` RFC3339 strings which the server parses into time values.

## Correlation & RCA operations

Important difference: the Correlation / RCA endpoints expose a canonical *time-window-only* public contract — they accept exactly the JSON shape `{ "startTime": "<RFC3339>", "endTime": "<RFC3339>" }` (camelCase keys). The correlation handlers may accept legacy UnifiedQuery shapes when strict mode is disabled, but the Stage-01 canonical public contract is the time-window-only shape — see the Correlation documentation for details.

## Best practices & tips

 - Use `/api/v1/unified/metadata` to discover supported capabilities before building client code.
 - When constructing UnifiedQuery objects use the expected JSON keys: `start_time` / `end_time` (snake_case) when sending a UnifiedQuery; correlation/time-window endpoints use `startTime` / `endTime` (camelCase). This distinction is important — the handler uses `TimeWindowRequest` for correlation endpoints and `UnifiedQuery` JSON for general queries.
 - Include start/end times for deterministic results. Range queries are often preferred for metrics and traces while logs may use stream or search.
 - Inspect `metadata.engine_results` in responses for source-level diagnostics and performance numbers.

## Troubleshooting

- If a backend fails, the unified response will include per-engine status so you can detect partial failures.
- Check `/api/v1/unified/health` & `/api/v1/unified/stats` for runtime health and cache stats.

---

