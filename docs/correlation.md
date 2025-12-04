# Correlation

This page describes Mirador Core's Correlation engine — how it accepts requests, the canonical request contract, time windows, rings/buckets, scoring, and examples.

## Purpose

The Correlation engine compares candidate KPIs and signals across a time window to identify candidate causes for an observed impact. It is statistical, transparent, and grounded in the platform KPI registry.

## API contract (Stage-01 canonical)

For Stage-01 the Correlation and RCA endpoints accept a canonical time-window payload only. The public contract is strict — handlers should accept exactly the JSON body shown below and reject extra fields.

`POST /api/v1/unified/correlate` (or POST /api/v1/unified/correlation in some deployments)

Request body (exact):

We maintain UTC Timestamps in order to support global deployments of applications and Coordinated Universal Time will guard us against Day Light Saving rules. For more details please refer [RFC#3339 - Date and Time on the Internet: Timestamps](https://www.rfc-editor.org/rfc/rfc3339#section-4.1){:target="_blank"}

```json
{
  "startTime": "2025-11-01T12:00:00Z",
  "endTime":   "2025-11-01T13:00:00Z"
}
```
NOTE: This time-window-only contract is deliberate — correlation computations are window-based and engine-level tuning/config must live in EngineConfig, not request payloads.

## Time anchoring: impact time & rings

Correlation uses a detected impact time T inside the window. Around T the engine builds temporal rings (buckets) — for example "impact", "near-impact" and "baseline" rings — and aligns candidate KPIs into those buckets for fair comparison.

Rings/buckets provide:

- consistent alignment for correlation measures
- per-ring suspicion/scoring for candidate ranking
- support for cross-correlation and lag analysis

## Statistical methods used

The engine uses several transparent statistical methods (configurable via EngineConfig):

- Pearson correlation for linear relationships
- Spearman rank correlation for monotonic associations
- Cross-correlation with lag to detect lead/lag relationships
- Partial correlation to reduce confounding variable effects

Each method contributes to a composite suspicion score used to rank candidate causes.

## Output

Correlation results return:

- summary (total correlations, average confidence, time_range)
- per-candidate details (KPI id/name, correlation scores by method, aligned time offsets)
- engine metadata (execution time, per-source record counts)

Example response (simplified):
```json
{
  "summary": {
    "total_correlations": 3,
    "average_confidence": 0.82
  },
  "correlations": [
    { "kpi_id": "kpi.http.latency", "score": 0.95, "method": "pearson", "lag_ms": 120 },
    { "kpi_id": "kpi.db.connections", "score": 0.78, "method": "spearman" }
  ],
  "metadata": { "time_range": "1h0m0s", "engine_results": { ... } }
}
```
## Error and validation rules

- The handler will reject payloads where endTime <= startTime.
- Time windows outside configured EngineConfig bounds (MinWindow, MaxWindow) may be rejected or truncated based on config.
- For Stage-01 the API contract is strict — payloads containing additional fields can cause request validation to fail.

## Notes for operators and developers

- Engine configuration (rings, thresholds, default_graph_hops, etc.) lives in EngineConfig and must not be provided in request body.
- The Correlation engine must not hardcode KPI names; it should derive candidates via KPIRepo or discovery services.
- Tests and CI exercise bucket/ring alignment, correlation methods, and narrative output — consult the correlation-RCA design docs in dev/correlation-RCA-engine/current for full details.

---

