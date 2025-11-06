---
title: "Mirador-Core MIRA Chat Interface Design"
author: "PlatformBuilds Engineering"
date: "2025-11-05"
version: "1.1"
---

# 💬 Mirador-Core **MIRA** Chat Interface — Design & Implementation Outline

> **MIRA = MiradorStack Intelligent Reasoning Assistant**

## 🎯 Goal

Implement the **MIRA** interface in `mirador-core`.  
This service connects the UI to the on-prem AI reasoning system (`mirador-rca`), enabling RCA Q&A in plain English — entirely inside **air-gapped, sovereign deployments**.

---

## 🧩 Requirements

- Transport: **HTTP/JSON** request with **SSE streaming** response (fallback: JSON).
- AuthN/Z enforced in `mirador-core` (RBAC + org/user quotas).
- **mTLS** (or network-policy allowlist) between `mirador-core` ↔ `mirador-rca`.
- Absolutely **no external egress**.
- End-to-end **tracing, metrics, and audit logs**.
- Timeouts, retries, **circuit breaking**, **backpressure**, **client-cancel**.
- PII-aware logging (mask/suppress per policy).

---

## ⚙️ Endpoint Specification (UI → mirador-core)

### Streaming (preferred)

**POST** `/api/v1/mira/chat?stream=1`  
**Headers:** `Authorization: Bearer <token>`, `Content-Type: application/json`

**Request Body**
```json
{
  "question": "Why is conversion down in the last hour?",
  "context": {
    "env": "prod",
    "timeRange": { "from": "2025-11-05T08:00:00Z", "to": "2025-11-05T09:00:00Z" }
  },
  "session_id": "optional-session-id",
  "attachments": [{ "type": "metric", "ref": "checkout_latency_p95" }],
  "preferences": { "temperature": 0.2, "max_tokens": 512 }
}
```

**SSE Response**
```
event: token
data: "Conversion dropped ~14%..."

event: meta
data: {"model":"local-llm@q4","confidence":0.78,"rcaId":"rca_123"}

event: done
data: {"usage":{"prompt_tokens":410,"completion_tokens":250}}
```

### Non-streaming fallback

**POST** `/api/v1/mira/chat`  
**200 application/json**
```json
{
  "answer": "Conversion fell ~14% due to DB pool exhaustion in Payment API.",
  "meta": { "model": "local-llm@q4", "confidence": 0.78, "rcaId": "rca_123" }
}
```

### Errors

| Code | Meaning |
|------|--------|
| 400  | Invalid payload |
| 401/403 | Unauthorized / Forbidden |
| 408/504 | Timeout |
| 429  | Rate limit exceeded |
| 503  | Upstream MIRA/RCA unavailable |

---

## 🔄 Internal Contract (mirador-core → mirador-rca)

- **Upstream URL:** `POST http://mirador-rca:8080/rca/v1/chat` (`?stream=1` for SSE)
- **Headers:** `X-Trace-Id`, `X-Caller: mirador-core`, `Content-Type: application/json`
- **Timeouts:** connect 500 ms · first-byte (SSE) 3 s · total 30 s
- **Retries:** 2 on 502/503/504 (no retry after SSE begins)
- **Circuit breaker:** open after 5 failures/minute; half-open cooldown

---

## 🧱 Environment Variables

```bash
MIRA_API_BASE=http://mirador-rca:8080/rca/v1
MIRA_CHAT_TIMEOUT_MS=30000
MIRA_CONNECT_TIMEOUT_MS=500
MIRA_SSE_FIRST_BYTE_TIMEOUT_MS=3000
MIRA_MAX_RETRIES=2
MIRA_RATE_LIMIT_RPS=5
MIRA_BURST=10
MIRA_MAX_REQUEST_BYTES=65536
```

---

## 🔐 Middleware & Guards

- **AuthN:** verify JWT/Session → user id/org id/roles.
- **RBAC:** require `can.use_mira_chat` (per env: prod/staging).
- **Rate limits:** token bucket (per user + per org) + concurrency caps.
- **Payload limits:** reject bodies > `MIRA_MAX_REQUEST_BYTES` (64 KB default).
- **Validation:** JSON Schema; strip control chars; trim excessively long fields.
- **Context defaults:** auto-fill `env` and `timeRange` if omitted.
- **Tracing:** create root span `mira.chat`; inject/propagate `traceparent`.

---

## 🧠 Go (Gin) Handler Skeleton

```go
// POST /api/v1/mira/chat
func MiraChatHandler(c *gin.Context) {
    ctx := c.Request.Context()
    traceID := ensureTraceID(c)

    var req MiraChatRequest
    if err := bindAndValidate(c, &req); err != nil {
        failBadRequest(c, err); return
    }
    if !rbac.CanUseMiraChat(c) { failForbidden(c); return }

    stream := c.Query("stream") == "1"
    miraURL := cfg.MiraAPIBase + "/chat"
    headers := map[string]string{
        "X-Trace-Id": traceID,
        "X-Caller":   "mirador-core",
        "Content-Type": "application/json",
    }

    if stream {
        c.Writer.Header().Set("Content-Type", "text/event-stream")
        c.Writer.Header().Set("Cache-Control", "no-cache")
        c.Writer.Header().Set("Connection", "keep-alive")
        flusher, _ := c.Writer.(http.Flusher)

        ctx, cancel := context.WithTimeout(ctx, cfg.MiraChatTimeout)
        defer cancel()

        err := miraClient.SSEProxy(ctx, miraURL+"?stream=1", req, headers,
            func(ev SSEEvent) {
                writeSSE(c.Writer, ev.Type, ev.Data)
                flusher.Flush()
            })
        if err != nil {
            writeSSE(c.Writer, "error", fmt.Sprintf(`{"message":%q}`, err.Error()))
        }
        writeSSE(c.Writer, "done", `{"status":"ok"}`)
        return
    }

    ctx, cancel := context.WithTimeout(ctx, cfg.MiraChatTimeout)
    defer cancel()
    resp, status, err := miraClient.JSON(ctx, miraURL, req, headers)
    if err != nil {
        mapUpstreamErr(c, err); return
    }
    c.JSON(status, resp)
}
```

**SSE proxy (concept)**
```go
func (c *RCAClient) SSEProxy(
  ctx context.Context,
  url string,
  payload any,
  headers map[string]string,
  onEvent func(SSEEvent),
) error {
  // Build POST with JSON body & headers; dial with connect timeout
  // Read streaming body; parse "event:" + "data:" lines
  // onEvent(SSEEvent{Type: typ, Data: data})
  // Handle ctx.Done() to cancel upstream
  return nil
}
```

---

## 🧩 Data Structures

```go
type TimeRange struct {
  From time.Time `json:"from"`
  To   time.Time `json:"to"`
}

type MiraChatRequest struct {
  Question    string         `json:"question" binding:"required,max=4000"`
  Context     map[string]any `json:"context,omitempty"`
  SessionID   string         `json:"session_id,omitempty"`
  Attachments []Attachment   `json:"attachments,omitempty"`
  Preferences *MiraChatPrefs `json:"preferences,omitempty"`
}

type Attachment struct {
  Type string `json:"type" binding:"oneof=metric log trace"`
  Ref  string `json:"ref"  binding:"required"`
}

type MiraChatPrefs struct {
  Temperature float32 `json:"temperature" binding:"gte=0,lte=1"`
  MaxTokens   int     `json:"max_tokens"  binding:"gte=32,lte=4096"`
}

type MiraChatResponse struct {
  Answer string   `json:"answer"`
  Meta   MiraMeta `json:"meta"`
}

type MiraMeta struct {
  Model      string  `json:"model"`
  Confidence float32 `json:"confidence"`
  RcaID      string  `json:"rcaId,omitempty"`
}

type SSEEvent struct {
  Type string // token | meta | done | error
  Data string // raw JSON or text chunk
}
```

---

## 📊 Observability (Metrics, Logs, Tracing)

**Prometheus metrics**
```
mira_chat_requests_total{status,stream}
mira_chat_inflight
mira_chat_upstream_latency_seconds_bucket
mira_chat_upstream_errors_total{code}
mira_chat_rate_limited_total
```

**VictoriaLogs (structured)**
```json
{
  "type": "mira_chat",
  "event": "done",
  "trace_id": "abcd-1234",
  "user": "alice@platformbuilds.io",
  "env": "prod",
  "model": "local-llm@q4",
  "confidence": 0.78,
  "latency_ms": 4100
}
```
> Mask/suppress sensitive `question` text per policy; store only derived fields (length, hashes).

**Tracing**
- Root span: `mira.chat`
- Child span: `mira.chat.rca`
- Attributes: `session_id`, `env`, `confidence`, `user_id`

---

## 🚦 Backpressure, Cancellation & Errors

- Abort upstream on client disconnect (`ctx` cancel).
- If RCA returns **429** with `Retry-After`, forward header; surface friendly UI hint.
- Concurrency caps (e.g., **3** active streams/user).
- Circuit breaker tripping → return **503** with guidance.
- Timeouts: client sees **504** (or **408**) with retry recommendation.

---

## 🧪 Testing Matrix

| Level | Case | Expected |
|------|------|----------|
| Unit | Invalid JSON / oversize body | 400 |
| Unit | Auth / RBAC denied | 403 |
| Unit | SSE writer emits token/meta/done | Event order + flush |
| Integration | Upstream timeout | 504, audit log written |
| Integration | Upstream 5xx w/ retries | Retries then 503 |
| Integration | Client disconnect mid-stream | Upstream conn closed |
| Load | 50 concurrent streams | P95 < 5s, no leaks |
| Security | No outbound sockets | Pass (netpol) |
| E2E | UI → `/api/v1/mira/chat` → tokens → final | Full narrative renders |

---

## 🚀 Rollout Steps

1. **Feature flag**: `FEATURE_MIRA_CHAT=true`
2. Add env vars; wire config → HTTP client & SSE proxy.
3. Provide **mock `mirador-rca`** container for CI.
4. Stage deploy; validate SSE under load (LAN).
5. Gradual prod enablement per tenant; monitor metrics:
   - `mira_chat_upstream_latency_seconds`
   - `mira_chat_upstream_errors_total`
   - `mira_chat_rate_limited_total`

---

## 🧭 Future Enhancements

- `/api/v1/mira/sessions` — list past Q&A sessions (ids/metadata).
- Tool calls: `getMetricSlice`, `getLogs`, `getTraces` via core.
- Stream compression (`zstd`) for high-latency links.
- Multi-agent pipeline (MIRA-RCA, MIRA-Anomaly, MIRA-Predict).

---

## ✅ Summary

`/api/v1/mira/chat` is the **single entrypoint** for conversational RCA.  
`mirador-core` centralizes **security, rate limiting, and observability**, and proxies to `mirador-rca` for local LLM reasoning — all **on-prem** and **air-gapped**.
