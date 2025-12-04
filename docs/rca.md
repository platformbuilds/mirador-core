# RCA (Root Cause Analysis)

This page documents Mirador Core's RCA engine: how it receives requests, how it composes why-chains and narratives from correlation results, and operational considerations.

## API contract

RCA follows the same Stage-01 time-window-only request contract as Correlation. Public handlers accept only a JSON object with `startTime` and `endTime` fields:

`POST /api/v1/unified/rca`

Request (exact shape):

We maintain UTC Timestamps in order to support global deployments of applications and Coordinated Universal Time will guard us against Day Light Saving rules. For more details please refer [RFC#3339 - Date and Time on the Internet: Timestamps](https://www.rfc-editor.org/rfc/rfc3339#section-4.1){:target="_blank"}

```json
{
  "startTime": "2025-11-01T12:00:00Z",
  "endTime":   "2025-11-01T13:00:00Z"
}
```
EngineConfig and the KPI registry provide the rest of the context — RCA builds on correlation results and registry metadata, not request-scoped knobs.

## High-level flow

1. Detect impact (time T) in the supplied window.
2. Execute correlation across KPIs using the Correlation engine (time alignment, scoring).
3. Create candidate dependency graph using platform services and topology metadata.
4. Expand a why-chain for top candidates, including checks that reduce false positives (partial correlation, contextual signals from logs/traces).
5. Produce a concise narrative summarising the most probable cause(s) and recommended next steps.

## Why-chain and narrative

- Why-chains are short, prioritized sequences of cause->effect relationships derived from ranked correlations and topology data.
- Narratives are human-readable summaries that include: impact description, top candidate causes, supporting evidence (KPIs/metrics/logs/traces), and suggested investigation steps.

## Best-effort & limitations

- RCA is probabilistic — it provides ranked candidates and supporting evidence, not absolute proof.
- Avoid relying on single-method outcomes — inspect supporting evidence (logs, traces) provided in the narrative.
- Engine behaviour, depth (how many "whys"), and hop limits are configured through EngineConfig (e.g., DefaultGraphHops, DefaultMaxWhys).

## Example response (simplified)

```json
{
  "impact": { "time": "2025-11-01T12:24:00Z", "kpi_id": "kpi.http.error_rate" },
  "why_chains": [
    {
      "steps": [
        {"desc": "increase in http error rate for service A", "score": 0.92},
        {"desc": "dependent service B saw elevated latency", "score": 0.82}
      ],
      "narrative": "Top cause likely failure in service A; follow-up: check service A logs for exceptions, check recent deploys, review B->A call traces."
    }
  ],
  "metadata": { "execution_time_ms": 250, "engine_results": {...} }
}
```

## Operator guidance

- Use RCA engine logs and per-candidate evidence to validate and iterate on KPI definitions and topology mappings.
- Ensure the KPI registry and topology store are kept fresh; stale registry data leads to weaker candidate selection.

---

