# RCA Analyze using MIRA AI

## Summary
RCA Analyze (MIRA) translates technical Root Cause Analysis (RCA) output produced by Mirador's unified RCA/correlation engines into concise, non-technical narratives suitable for stakeholders. It supports synchronous and asynchronous modes, multiple AI providers (OpenAI, Anthropic, vLLM, Ollama), and a chunked processing + caching strategy to reduce cost and improve reliability.

Key endpoints:
- `POST /api/v1/mira/rca_analyze` — synchronous explanation (5 minute timeout)
- `POST /api/v1/mira/rca_analyze_async` — submit async task (returns `taskId`)
- `GET  /api/v1/mira/rca_analyze/{taskId}` — poll async task status/result

This document describes architecture, API contracts, async flow, config knobs, examples, integration points, and operational troubleshooting.

---

## High-level Architecture (text diagram)

Client (UI / automation)
  ↓
1) Option A — Synchronous
  - POST `/api/v1/unified/rca` → receive RCA data
  - POST `/api/v1/mira/rca_analyze` (rcaData) → MIRA service → returns explanation
2) Option B — Asynchronous
  - POST `/api/v1/unified/rca` → receive RCA data
  - POST `/api/v1/mira/rca_analyze_async` (rcaData, optional callbackUrl) → returns `taskId`
  - Client polls `GET /api/v1/mira/rca_analyze/{taskId}` for progress and final result

Internal processing (background for async):
  - Validate RCA data → TOON conversion → prompt rendering → chunked AI generation (per-chunk calls) → stitch results → persist `task` state to Valkey → optional webhook callback
Persistent stores:
  - Valkey (task state, cache)
  - (Optional) logs/metrics, provider telemetry

Error flows:
  - Bad input → 400
  - AI provider error/timeouts → 500 / task `failed`
  - Rate limits from provider → 429 / `failed` or queued

---

## API Overview

Groupings and endpoints with one-line descriptions.

- Unified Correlation / RCA (inputs that produce RCA data)
  - POST `/api/v1/unified/rca` — Produce canonical RCA analysis for a time window (request body must be `{ "startTime", "endTime" }`).
  - POST `/api/v1/unified/correlation` — Run unified correlation for a time window.

- KPI / Failures (related data sources)
  - GET/POST `/api/v1/kpi/defs`, etc. — KPI definitions and management.
  - POST `/api/v1/unified/failures/detect` — Detect failures in a time range.
  - POST `/api/v1/unified/failures/list` — List persisted failures.

- MIRA RCA Analyze (AI-powered narrative)
  - POST `/api/v1/mira/rca_analyze` — Sync explanation; returns `status: success` + `data.explanation`.
  - POST `/api/v1/mira/rca_analyze_async` — Submit async job; returns `taskId` and `statusUrl`.
  - GET  `/api/v1/mira/rca_analyze/{taskId}` — Retrieve task status, progress, and result.

---

## Request & Response Schemas (summary)

Notes: the canonical input for MIRA endpoints is the RCA `data` object returned by `/api/v1/unified/rca` (RCAIncidentDTO). Below are the important fields used by MIRA.

Request body (both sync and async):
- JSON object with required property `rcaData` (object).
  - rcaData.impact: { id, impactService, metricName, timeStart, timeEnd, impactSummary, severity }
  - rcaData.rootCause: { whyIndex, service, component, ring, summary, score }
  - rcaData.chains: array of candidate chains (each has steps, score, rank)
  - rcaData.generatedAt, diagnostics, timeRings, etc.

Async submit (additional fields):
- `callbackUrl` (string, uri) — optional webhook to receive completion notification.

Synchronous response (200):
- `{ "status": "success", "data": { "explanation": "<string>", "provider": "<name>", "model": "<name>", "tokensUsed": <int>, "cached": <bool> } }`

Async submit response (202 Accepted):
- `{ "status":"accepted", "taskId":"<uuid>", "statusUrl": "/api/v1/mira/rca_analyze/<taskId>", "message": "Task submitted..." }`

Async status response (200):
- Fields:
  - `taskId` (uuid)
  - `status` (pending | processing | completed | failed)
  - `progress` (object when processing/completed):
    - `totalChunks`, `completedChunks`, `currentChunk`, `currentStage`, `lastUpdated`
  - `submittedAt`, `startedAt`, `completedAt` timestamps
  - `result` (object when completed):
    - `status`, `data.explanation`, `provider`, `model`, `tokensUsed`, `cached`, `totalChunks`, `generationTimeMs`
  - `error` (string when failed)
  - `callbackUrl` (string if provided on submit)

404 (status): task not found or TTL expired (tasks persisted to Valkey with 24h TTL).

---

## Async Flow: MIRA RCA Analyze (step-by-step)

1. Client prepares RCA data:
   - Call `POST /api/v1/unified/rca` with `{ startTime, endTime }` to get full RCA analysis (or obtain RCA from other flows).
2. Submit async analysis:
  - POST `/api/v1/mira/rca_analyze_async` with body `{ "name": "<human-readable-name>", "rcaData": <RCA data>, "callbackUrl": <optional> }` (name is required)
   - Server returns 202 Accepted with JSON `{ taskId, statusUrl }`.
3. Server enqueues task:
   - A background goroutine/process reads the task, sets status → `processing`, and writes status to Valkey (key: `mira:rca:task:{taskId}`).
4. Processing stages (progress is updated in Valkey):
   - `toon_conversion` — RCA converted to TOON format for token savings.
   - `prompt_rendering` — LLM prompt(s) rendered (includes "5 Whys" guidance and chain context).
   - `chunk_processing_started` → `chunk_N_processing` / `chunk_N_completed` — chunk-by-chunk AI generation.
   - `stitching_final_explanation` — partial outputs combined into final narrative.
5. Completion:
   - Status set to `completed`, result stored in `result` field. TTL remains (default 24h). Optionally POST callback URL with final payload.
6. Failure:
   - If a provider error, retries exhausted, or internal error occurs, status set to `failed` and `error` is populated; client sees the failure on GET.
7. Client polls:
   - GET `/api/v1/mira/rca_analyze/{taskId}` to observe `status` and `progress`. Recommended polling interval: 5–10s.

---

## Configuration & Environment Variables

Mirador's MIRA settings are configured in `configs/*.yaml` and environment variables:

- Backend config keys (conceptual):
  - `mira.enabled` (bool)
  - `mira.provider` (string) — `openai`, `anthropic`, `vllm`, `ollama`
  - `mira.timeout` (duration) — sync timeouts; production default is `5m`
  - `mira.cache_strategy.ttl` (int seconds) — Valkey TTL for cached responses
  - `mira.rate_limit.requests_per_minute` (int) — provider rate limiting

- Provider secrets (passed via env / secrets):
  - OpenAI: `OPENAI_API_KEY` (or platform-specific secret)
  - Anthropic: `ANTHROPIC_API_KEY`
  - vLLM / Ollama: configured endpoints and any needed credentials

- Task storage & cache:
  - Valkey / in-memory cluster used for caching and task persistence (no sensitive data in taskId)

- Webhook callback:
  - `callbackUrl` (client-provided) — server will POST final result to this endpoint upon completion if valid.

Security: Never commit keys to repo; use deployment secrets (Kubernetes secrets, environment variables, Docker Compose secrets).

---

## Examples

Note: examples are adapted from `api/openapi.json` and the implementation summary.

1) Synchronous request (short runs — < 5 minutes)

```bash
# Step: get unified RCA (example)
RCA_RESPONSE=$(curl -s -X POST http://localhost:8010/api/v1/unified/rca \
  -H "Content-Type: application/json" \
  -d '{"startTime":"2025-12-03T07:30:00Z","endTime":"2025-12-03T08:30:00Z"}')

# Send RCA data to MIRA (sync)
echo "$RCA_RESPONSE" | jq '{ rcaData: .data }' | \
  curl -s -X POST http://localhost:8010/api/v1/mira/rca_analyze \
  -H "Content-Type: application/json" \
  -d @-
```

2) Async submit + poll

```bash
# Submit async task
        TASK_RESPONSE=$(curl -s -X POST http://localhost:8010/api/v1/mira/rca_analyze_async \
          -H "Content-Type: application/json" \
          -d '{
            "name": "db-outage-analysis",
            "rcaData": {...},
            "callbackUrl": "https://webhook.example.com/mira-callback"
          }')

TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.taskId')

# Poll loop (example)
while true; do
  STATUS=$(curl -s http://localhost:8010/api/v1/mira/rca_analyze/$TASK_ID | jq -r '.status')
  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
    curl -s http://localhost:8010/api/v1/mira/rca_analyze/$TASK_ID | jq .
    break
  fi
  sleep 5
done
```

Example success response (trimmed):
```json
{
  "taskId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "progress": { "totalChunks": 3, "completedChunks": 3, "currentStage": "completed" },
  "result": {
    "status": "success",
    "data": {
      "explanation": "First, users experienced failed transactions (Why #1, IMPACT layer)...",
      "provider": "openai",
      "model": "gpt-4",
      "tokensUsed": 3891,
      "cached": false
    }
  },
  "submittedAt": "2025-12-03T14:30:00Z",
  "completedAt": "2025-12-03T14:35:42Z"
}
```

Error response (invalid input):
```json
{
  "status": "error",
  "error": "invalid_rca_data: missing required field: data"
}
```

Task not found (404):
```json
{
  "status": "error",
  "error": "task_not_found",
  "taskId": "..."
}
```

---

## Integration with Unified Correlation & Unified RCA

- Typical flow:
  1. UI or automation calls `POST /api/v1/unified/rca` with canonical `{ startTime, endTime }`.
  2. Use the RCA `data` field as input to MIRA (sync or async) into the json key `rcaData` as is as a json dictionary
  3. MIRA returns a non-technical narrative which the UI attaches to RCA results (5-Whys narrative).

- Use cases:
  - Executive summary in incident pages.
  - Attach to failure records stored in Weaviate.
  - Include in alerts and post-incident reports.

- Assumptions:
  - Unified RCA returns a canonical `data` object (RCAIncidentDTO). MIRA expects that shape (see OpenAPI).
  - MIRA only reads `rcaData` (no other request-scoped knobs permitted in unified endpoints).
  - The RCA engine is responsible for building chains, scores, and ring contexts; MIRA consumes those to form narratives.

---

## Operational Notes & Troubleshooting

- Timeouts:
  - Synchronous API has a 5-minute timeout by configuration. Use the async API for longer work.
- Progress visibility:
  - Poll `GET /api/v1/mira/rca_analyze/{taskId}` for `progress` while status is `processing`.
- Rate limits & provider errors:
  - Providers may return 429. The system has rate-limit middleware (prod). On repeated provider failures the task will be marked `failed`.
- Logs:
  - Async tasks include `task_id` in logs. Search logs for `task_id` to trace a task lifecycle.
- Cache:
  - MIRA uses Valkey caching keyed by prompt hash to reduce duplicate AI API calls. `cached: true` in responses indicates cache hit.
- Validation:
  - Validate RCA schema before submitting; malformed RCA data results in 400.
- TTL & cleanup:
  - Tasks stored in Valkey expire after 24 hours; after that a GET returns 404.
- Webhooks:
  - If you provide `callbackUrl` the server will POST final result. Ensure webhook endpoint is reachable and accepts JSON. Teams/Slack/Telegram anything that you prefer, as long as `mirador-core` is able to reach this URL, you will be notified once the analysis is complete. 

How to verify in dev:
- Enable MIRA in `configs/config.development.yaml` (Ollama recommended for dev).
- Run local stack (`make localdev-up` or docker-compose commands in `deployments/localdev`).
- Submit RCA + async task; verify poll returns processing and final completed result.
- Check Valkey keys and logs for `task_id`.

---

## Troubleshooting checklist (quick)
- 400 on submit: confirm `rcaData` exists and matches RCA structure.
- 429: check provider rate limits; consider using async or lower throughput.
- Task stuck `processing` without progress: check background worker logs, Valkey connectivity.
- No `taskId` returned: confirm server registered `mira` routes and `mira.enabled` is true in config.
- Long generation time: confirm chunking strategy and provider performance; consider switching providers or using async+webhook.

---

## References
- OpenAPI spec: `api/openapi.json` (endpoints `/api/v1/mira/rca_analyze*`)
- Design & implementation notes: `dev/mira/rca_response/IMPLEMENTATION-SUMMARY.md` and `DESIGN.md`
- Config: `configs/config.yaml`, `configs/config.development.yaml`

---

## Operational checklist for deployments
- Ensure provider API keys are set as secrets (OpenAI / Anthropic).
- Ensure Valkey is configured and reachable (task persistence + cache).
- Verify `mira.enabled: true` for intended environments.
- Ensure rate-limiting middleware is in-place for production.
- Confirm logging/observability: task metrics, provider latencies, token counts.

---
