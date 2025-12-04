# API Reference — Quick Guide (KPIs, Failures, Correlation, RCA, MIRA)

This document collects canonical request/response examples and API contracts for primary Mirador Core flows developers will use while building the UI and integrations.

Canonical artifacts (single source of truth):
- OpenAPI / Swagger: `api/openapi.json`, `api/openapi.yaml`, `api/swagger.json`, `api/swagger.yaml` (contains full schemas and examples used below).
- Runtime handlers and DTOs: `internal/api/server.go`, `internal/api/handlers/` and `internal/models/`, `internal/rca/`.
- Feature docs: `docs/rca-analyze-mira-ai.md`.

Usage note: the OpenAPI spec (`api/openapi.json`) includes inline examples for many endpoints — consult that file for detailed schemas.

---

**Contents**
- KPIs
- Failures
- Unified Correlation
- Unified RCA
- MIRA RCA Analyze (async + task polling)

---

## 1) KPIs

Purpose: manage and retrieve KPI definitions used by Mirador for discovery, correlation and RCA.

Primary endpoints
- `GET  /api/v1/kpi/defs` — List KPI definitions
- `POST /api/v1/kpi/defs` — Create or update a KPI definition
- `POST /api/v1/kpi/defs/bulk-json` — Bulk ingest KPI definitions (JSON array)
- `POST /api/v1/kpi/defs/bulk-csv` — Bulk ingest KPI definitions (CSV upload)

Example: Create / Update KPI
Request (`POST /api/v1/kpi/defs`)
```json
{
  "kpiDefinition": {
    "name": "api_errors_total",
    "kind": "tech",
    "namespace": "apigw_springboot_top_10_kpi",
    "unit": "count",
    "format": "integer",
    "layer": "impact",
    "signalType": "metrics",
    "classifier": "errors",
    "definition": "Total number of API errors observed at the gateway per minute.",
    "sentiment": "negative",
    "businessImpact": "If errors increase, end users may experience failed requests and revenue impact.",
    "tags": ["apigw","errors","pagerduty"],
    "examples": [ { "service.name": "api-gateway", "region": "us-east-1", "value": 42 } ]
  }
}
```
Response (201 Created example)
```json
{
  "status": "created",
  "id": "api_errors_total--apigw_springboot_top_10_kpi"
}
```

Notes:
- The OpenAPI spec contains richer examples and schema details for optional fields and bulk endpoints under `api/openapi.json`.

---

## 2) Failures

Purpose: detect, store and query failure (incident) records. These are used as inputs to RCA and for investigation.

Primary endpoints
- `POST /api/v1/unified/failures/detect` — Detect component failures in a time range
- `POST /api/v1/unified/failures/list` — List stored failure records (paginated)
- `POST /api/v1/unified/failures/get` — Get full failure record details
- `POST /api/v1/unified/failures/delete` — Delete a stored failure record

Example: Detect failures
Request (`POST /api/v1/unified/failures/detect`)
```json
{
  "time_range": {
    "start": "2025-12-01T10:00:00Z",
    "end": "2025-12-01T11:00:00Z"
  },
  "components": ["api-gateway","kafka"],
  "services": ["fintrans-simulator"]
}
```
Response (200 OK - partial example)
```json
{
  "incidents": [
    {
      "incident_id": "incident_kafka_1733022000",
      "failure_id": "kafka-producer-kafka-20251201-103000",
      "failure_uuid": "e458d90f-f525-58a9-9e92-9f91faa73cf2",
      "time_range": { "start": "2025-12-01T10:00:00Z", "end": "2025-12-01T10:30:00Z" },
      "primary_component": "kafka-producer",
      "services_involved": ["fintrans-simulator"],
      "confidence": 0.87,
      "severity": "high"
    }
  ],
  "summary": { "total_incidents": 1 }
}
```

Example: Get full failure details
Request (`POST /api/v1/unified/failures/get`)
```json
{ "failure_id": "fintrans-simulator-api-gatewaycall-tps-20251201-101045" }
```
Response (200 OK - trimmed)
```json
{
  "failure": {
    "failure_uuid": "...",
    "failure_id": "fintrans-simulator-api-gatewaycall-tps-20251201-101045",
    "time_range": { "start": "2025-12-01T10:00:00Z", "end": "2025-12-01T10:30:00Z" },
    "services": ["fintrans-simulator"],
    "components": ["api-gateway.call_tps"],
    "raw_error_signals": [ { "signal_type":"span", "service":"api-gateway", "timestamp":"..." } ],
    "raw_anomaly_signals": [ { "signal_type":"metric", "metric_name":"api_errors_total", "timestamp":"..." } ],
    "confidence_score": 0.85
  }
}
```

Notes:
- Failure records are persisted in Weaviate and can be paginated via `/unified/failures/list`.
- Use `failure_uuid` for deterministic lookups, `failure_id` for human-friendly display.

---

## 3) Unified Correlation

Purpose: run Mirador's unified correlation engine across a canonical time window to discover candidate cause KPIs for an impact.

Primary endpoint
- `POST /api/v1/unified/correlation` — Run unified correlation for a time window

Contract (Stage-01 rule): public API request must be exactly a time-window object. Do not add extra fields.

Request (canonical)
```json
{ "startTime": "2025-11-29T12:00:00Z", "endTime": "2025-11-29T12:15:00Z" }
```
Response (shape varies by implementation; example pattern)
```json
{
  "status": "success",
  "data": {
    "impact": { "metricName": "sum(db_ops_total)", "service": "db-service", "timeStart": "...", "timeEnd": "..." },
    "candidates": [
      { "kpi": "db_conn_errors_total", "correlationScore": 0.82, "lagMs": -120000 },
      { "kpi": "api_timeouts_total", "correlationScore": 0.61 }
    ]
  }
}
```

Notes:
- Exact response fields may include aligned timeseries, statistical scores (Pearson/Spearman), and candidate ranking. Consult `api/openapi.json` or `internal/api/handlers` for precise field names for your version.
- Use the correlation results to drive RCA workflows.

---

## 4) Unified RCA

Purpose: Run Mirador's RCA engine on a canonical time window to produce causal chains, scores and a candidate root cause.

Primary endpoint
- `POST /api/v1/unified/rca` — Run unified RCA (request must be `{ startTime, endTime }`)

Request
```json
{ "startTime": "2025-12-03T07:30:00Z", "endTime": "2025-12-03T08:30:00Z" }
```

Successful response (example - trimmed; the `.data` object is the canonical RCAIncidentDTO)
```json
{
  "status": "success",
  "data": {
    "impact": {
      "id": "corr_1764756501",
      "impactService": "DB Operations",
      "metricName": "sum(db_ops_total)",
      "timeStart": "2025-12-03T07:30:00Z",
      "timeEnd": "2025-12-03T08:30:00Z",
      "impactSummary": "Impact detected on DB Operations",
      "severity": 0.9
    },
    "rootCause": {
      "whyIndex": 5,
      "service": "process",
      "component": "deployment",
      "ring": "R2_SHORT",
      "summary": "Why 5: process (deployment) showed anomalies",
      "score": 0.27
    },
    "chains": [
      { "steps": [ { "whyIndex":1, "service":"DB Operations", "component":"sum(db_ops_total)", "score":0.9 } ], "score": 0.63, "rank": 1 }
    ],
    "generatedAt": "2025-12-03T10:08:21Z",
    "score": 0.63,
    "diagnostics": {}
  }
}
```

Notes:
- The `data` field is intended to be consumed by MIRA for narrative generation; MIRA expects the `.data` object (see next section).
- This endpoint is the canonical RCA entrypoint and must accept only `startTime`/`endTime` per project rules.

---

## 5) MIRA — RCA Analyze (sync & async + task polling)

Purpose: convert RCA `.data` into human-readable narratives using AI providers. Two modes are supported: synchronous (short runs) and asynchronous (long-running with progress tracking and optional webhooks).

Endpoints
- `POST /api/v1/mira/rca_analyze` — Synchronous analyze (5 minute timeout)
- `POST /api/v1/mira/rca_analyze_async` — Submit async task (returns `taskId`)
- `GET  /api/v1/mira/rca_analyze/{taskId}` — Poll task status and results

Input contract
- Required: `rcaData` — the `.data` object returned by `/api/v1/unified/rca` (RCAIncidentDTO)
- Optional (async submit): `callbackUrl` — webhook URL to POST final result

Example: Sync analyze (short)
Request (POST /api/v1/mira/rca_analyze)
```json
{ "rcaData": { /* full RCA data — impact, chains, rootCause, etc. */ } }
```
Response (200 OK example)
```json
{
  "status": "success",
  "data": {
    "explanation": "Your application experienced a spike in failed transactions at 2:30 PM...",
    "provider": "openai",
    "model": "gpt-4",
    "tokensUsed": 245,
    "cached": false
  }
}
```

Example: Async submit
Request (`POST /api/v1/mira/rca_analyze_async`)
```json
{
  "rcaData": { /* the RCA data */ },
  "callbackUrl": "https://webhook.example.com/mira-callback"
}
```
Response (202 Accepted)
```json
{
  "status": "accepted",
  "taskId": "550e8400-e29b-41d4-a716-446655440000",
  "statusUrl": "/api/v1/mira/rca_analyze/550e8400-e29b-41d4-a716-446655440000",
  "message": "Task submitted for processing. Poll statusUrl for results."
}
```

Polling status
Request (GET /api/v1/mira/rca_analyze/{taskId})

Possible statuses:
- `pending` — queued
- `processing` — in-progress (progress object present)
- `completed` — finished successfully (result present)
- `failed` — failed (error present)

Example: processing response
```json
{
  "taskId":"550e8400-e29b-41d4-a716-446655440000",
  "status":"processing",
  "progress":{
    "totalChunks":3,
    "completedChunks":1,
    "currentChunk":2,
    "currentStage":"chunk_2_processing",
    "lastUpdated":"2025-12-03T14:32:15Z"
  },
  "submittedAt":"2025-12-03T14:30:00Z",
  "startedAt":"2025-12-03T14:30:01Z"
}
```

Example: completed response (trimmed)
```json
{
  "taskId":"550e8400-e29b-41d4-a716-446655440000",
  "status":"completed",
  "progress":{ "totalChunks":3, "completedChunks":3, "currentStage":"completed" },
  "result":{
    "status":"success",
    "data":{
      "explanation":"First, users experienced failed transactions (Why #1, IMPACT layer)...",
      "provider":"openai",
      "model":"gpt-4",
      "tokensUsed":3891,
      "cached":false,
      "totalChunks":3,
      "generationTimeMs":341000
    }
  },
  "submittedAt":"2025-12-03T14:30:00Z",
  "startedAt":"2025-12-03T14:30:01Z",
  "completedAt":"2025-12-03T14:35:42Z"
}
```

Error (task failed)
```json
{
  "taskId":"550e8400-e29b-41d4-a716-446655440000",
  "status":"failed",
  "error":"AI provider timeout after 3 retries",
  "submittedAt":"2025-12-03T14:30:00Z",
  "startedAt":"2025-12-03T14:30:01Z",
  "completedAt":"2025-12-03T14:32:18Z"
}
```

Notes & implementation tips
- Tasks are persisted in Valkey (key: `mira:rca:task:{taskId}`) with 24h TTL — after that a `GET` returns 404.
- Recommended client polling interval: 5–10 seconds.
- When using webhooks (`callbackUrl`), ensure your endpoint accepts POST with JSON; the server will POST the final payload on completion.
- Synchronous analyze has a 5-minute timeout — prefer async for longer runs.

---

## Helpful references & how to regenerate schemas
- OpenAPI JSON/YAML: `api/openapi.json`, `api/openapi.yaml` (primary canonical artifacts).
- Postman collection: `api/mirador-core.postman_collection.json` (import into Postman to test).
- Code-first generator: `api/docs.go` (inspect route metadata generation).
- Regeneration scripts: check `tools/` and `scripts/` for `gen_openapi_json.py`, `gen_postman_collection.py`.


---

## Appendix: quick lookup table
- KPI defs: `GET /api/v1/kpi/defs`, `POST /api/v1/kpi/defs`
- Failures: `POST /api/v1/unified/failures/detect`, `/list`, `/get`, `/delete`
- Unified correlation: `POST /api/v1/unified/correlation` (time-window only)
- Unified RCA: `POST /api/v1/unified/rca` (time-window only)
- MIRA Analyze (sync): `POST /api/v1/mira/rca_analyze`
- MIRA Analyze (async): `POST /api/v1/mira/rca_analyze_async` → `GET /api/v1/mira/rca_analyze/{taskId}`

For full field-level schemas and more examples, open `api/openapi.json` (contains exhaustive request/response schemas and concrete examples).
