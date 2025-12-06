# KPI (Key Performance Indicators)

This page explains Mirador Core's KPI model, how KPIs are discovered and stored, and best practices for writing and maintaining KPI definitions used by correlation and RCA engines.

## What is a KPI in Mirador Core

- A KPI is a platform-level observable definition — a canonical name, a query or formula that derives a value, and metadata that describes the observable (service, namespace, unit, aggregation, importance / impact type).
- KPIs are used across Mirador for discovery, correlation and narrative generation. Engines and handlers avoid hardcoded metric names and rely on KPI metadata + discovery to find platform-relevant signals.

## KPI metadata (typical fields)

- id: unique identifier for the KPI
- name: user-friendly display name
- description: short summary of what the KPI measures
- engine / data_source: metrics / logs / traces — defines where to execute the query
- query or formula: the expression used to fetch or compute the KPI
- labels: suggested semantic labels (service, pod, namespace) to match runtime telemetry
- impact: optional severity/importance indicator used by the Correlation and RCA engines

## KPI discovery & seeding

Mirador Core uses a central KPI repository (KPIRepo) to list and manage known KPIs. When handlers or engines run analytic workflows (e.g., correlation across the platform), they may call KPIRepo.ListKPIs to build queries covering the whole platform.

The platform includes helper tooling to seed standard KPIs used by Mirador. These seeded KPIs provide consistent, platform-wide observability perspectives and help ensure correlation/RCA engines can reference canonical signals rather than ad-hoc metric names.

## How KPIs are used

- Autonomous queries: When a correlation request has an empty body, Mirador can synthesize a correlation query across the entire KPI catalog.
- KPI-driven RCA: KPIs guide RCA engine candidate selection, label detection, and narrative construction.
- Instrumentation & templates: Keeping KPIs small, composable and well-labeled helps the engines reduce false positives and produce more actionable narratives.

## Best practices for KPI authors

- Use meaningful, canonical names and a concise description.
- Prefer returning scalar time-series or single-value aggregates for KPI formulas — these work best for correlation and anomaly detection.
- Attach canonical semantic labels (service, namespace) instead of custom-only labels so engines can synthesize queries across environments.
- Avoid hardcoded instance names; use label-driven patterns (e.g., service.name=~"api-.*") where appropriate.
- Define `sentiment` if the value is increasing, like increase in latency is a `negative` sentiment
- Define `serviceFamily` for sure, this is the greater family a KPI belongs to. RCA engine uses this to group and analyze
- Define `layer` always as in `impact` or `cause`. Generally Business Metric get impacted because of Technical Issues, hence Bunsiess is `impact` and Tech is `cause`

## Example KPI (metrics)

### Business KPI Metric (Impact Layer)
```json
    {
      "kpi_name": "Technical Failure Impact Score",
      "kpi_formula": "sum by (OrgName) (rate(transaction_total{success=\"false\", error_code=~\"TD.*\"}[1h])) * avg(transaction_amount)",
      "kpi_definition": "Monetary impact of technical failures per bank, considering transaction volume and average amount. High scores indicate significant revenue loss and customer inconvenience.",
      "layer": "impact",
      "classifier": "revenue_at_risk",
      "sentiment": "negative",
      "signal_type": "metrics",
      "query_type": "PromQL",
      "datastore": "victoriametrics",
      "emotional_impact": "Customer anger and loss of trust",
      "business_impact": "Direct revenue loss and recovery costs",
      "serviceFamily": "business_oltp"
    }
```

### Technical KPI Metric (Cause Layer)
```json
    {
      "kpi_name": "Kafka Consume Latency",
      "kpi_formula": "sum(kafka_consume_latency_seconds_sum) / sum(kafka_consume_latency_seconds_count)",
      "kpi_definition": "Average time taken to consume messages from Kafka. High latency indicates consumer processing bottlenecks.",
      "layer": "cause",
      "classifier": "message_latency",
      "sentiment": "negative",
      "signal_type": "metrics",
      "query_type": "PromQL",
      "datastore": "victoriametrics",
      "serviceFamily": "kafka"
    }

    ## Searching KPIs (human-friendly)

    Mirador supports a human-friendly, natural-language KPI search endpoint for non-technical users:

    - POST /api/v1/kpi/search

    Example request body:

    ```json
    {
      "query": "Which KPIs show payment latency spikes?",
      "filters": { "tags": ["payments"], "layer": "cause" },
      "limit": 5,
      "mode": "hybrid",
      "explain": true
    }
    ```

    What you get back:
    - A ranked list of KPIs with short snippets, tags, and a relevance score (0..1).
    - Hybrid mode uses both vector (semantic) and keyword matching; the default vectorizer is CPU-friendly (small transformer models) so it works on most deployments without requiring GPU.

    Operator notes:
    - The `content` field (internal) is a concatenation of name, definition, formula, tags and examples — it is used as the text surface for vectorization. Use `devtools/reindex-kpis.sh` to reindex/re-upsert existing KPIs after schema changes.
```

## Operator notes

- KPI definitions are stored in the repository-backed KPI store and must be kept lean and tested.
- Tests and simulator fixtures are allowed to hardcode example KPI names, but engine code must rely on config and registry for KPI names and not embed environment-specific strings.

---
