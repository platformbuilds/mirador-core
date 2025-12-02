# Quick Fix Implementation Guide

## TL;DR – Why RCA Returns "No Correlation Data"

**Your synthetic telemetry with 25% failures works, but the RCA engine can't find the KPI definitions.**

The RCA correlation engine has this flow:
```
1. Call kpiRepo.ListKPIs() → Gets KPI list from Weaviate
2. For each KPI, probe: ce.metricsService.ExecuteQuery(kpiFormula)
3. If seriesCount > 0 → Add to candidateKPIs
4. If candidateKPIs is empty → Return "No correlation data"
```

**Problem:** Step 1 returns very few KPIs (only 1 found: "Server Health Score"), 
so Step 3 finds almost no candidates, and Step 4 returns early.

---

## The Immediate Fix: Seed Transaction Failure KPIs

Your logs show queries like:
```json
"query": "rate(transactions_failed_total[1m])"  → seriesCount: 0
"query": "rate(transactions_total[1m])"         → seriesCount: 0
```

These metrics **exist in VictoriaMetrics** (synthetic data is there), but **the KPI definitions are missing from Weaviate**.

### Solution:

Create/execute this KPI seeding script:

**File:** `scripts/seed-transaction-kpis.sh` (new file)

```bash
#!/bin/bash

MIRADOR_API="http://127.0.0.1:8010"

# KPI 1: Transaction Failure Rate (Impact)
curl -X POST "$MIRADOR_API/api/v1/kpi" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Transaction Failure Rate",
    "kind": "kpi",
    "formula": "rate(transactions_failed_total[1m]) / rate(transactions_total[1m]) * 100",
    "layer": "impact",
    "query_type": "metrics",
    "datastore": "metrics",
    "signal_type": "metric",
    "classifier": "error_rate",
    "sentiment": "higher_is_worse",
    "service_family": "payment-system",
    "tags": ["synthetic", "transaction", "failure"]
  }'

# KPI 2: Transaction Success Rate (Impact)
curl -X POST "$MIRADOR_API/api/v1/kpi" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Transaction Success Rate",
    "kind": "kpi",
    "formula": "rate(transactions_total{success=\"true\"}[1m]) / rate(transactions_total[1m]) * 100",
    "layer": "impact",
    "query_type": "metrics",
    "datastore": "metrics",
    "signal_type": "metric",
    "classifier": "availability",
    "sentiment": "higher_is_better",
    "service_family": "payment-system",
    "tags": ["synthetic", "transaction", "success"]
  }'

# KPI 3: Transaction Latency P95 (Cause)
curl -X POST "$MIRADOR_API/api/v1/kpi" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Transaction Latency P95",
    "kind": "kpi",
    "formula": "histogram_quantile(0.95, rate(transaction_latency_seconds_bucket[1m]))",
    "layer": "cause",
    "query_type": "metrics",
    "datastore": "metrics",
    "signal_type": "metric",
    "classifier": "latency",
    "sentiment": "higher_is_worse",
    "service_family": "payment-system",
    "tags": ["synthetic", "transaction", "latency"]
  }'

# KPI 4: Database Operation Rate (Cause)
curl -X POST "$MIRADOR_API/api/v1/kpi" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Database Operation Rate",
    "kind": "kpi",
    "formula": "rate(db_ops_total[1m])",
    "layer": "cause",
    "query_type": "metrics",
    "datastore": "metrics",
    "signal_type": "metric",
    "classifier": "throughput",
    "sentiment": "neutral",
    "service_family": "data-layer",
    "tags": ["synthetic", "database", "operations"]
  }'

# KPI 5: Server Health Score (Infrastructure)
curl -X POST "$MIRADOR_API/api/v1/kpi" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Server Health Score",
    "kind": "kpi",
    "formula": "((1 - (node_load1 / count without(cpu, mode) (node_cpu_seconds_total{mode=\"idle\"}) or vector(0.8))) + (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes or vector(0.2)) + (node_filesystem_avail_bytes{fstype!~\"tmpfs|ramfs\"} / node_filesystem_size_bytes{fstype!~\"tmpfs|ramfs\"} or vector(0.1))) / 3 * 100",
    "layer": "cause",
    "query_type": "metrics",
    "datastore": "metrics",
    "signal_type": "metric",
    "classifier": "infrastructure_health",
    "sentiment": "higher_is_better",
    "tags": ["synthetic", "infrastructure"]
  }'

echo "KPIs seeded successfully"
```

**Execute:**
```bash
chmod +x scripts/seed-transaction-kpis.sh
./scripts/seed-transaction-kpis.sh
```

---

## Verify the Fix Worked

```bash
# 1. Check KPIs are in Weaviate
curl 'http://127.0.0.1:8010/api/v1/kpi?limit=100' | jq '.data | length'
# Expected output: Should show increased KPI count

# 2. Query one of the metrics directly to confirm data exists
curl 'http://127.0.0.1:8428/api/v1/query_range?query=rate(transactions_failed_total[1m])&start=2025-12-02T19:30:00Z&end=2025-12-02T20:30:00Z&step=60'
# Expected: Data series returned

# 3. Call RCA again
curl -X 'POST' http://127.0.0.1:8010/api/v1/unified/rca \
  -H 'Content-Type: application/json' \
  -d '{
    "startTime": "2025-12-02T19:30:00Z",
    "endTime": "2025-12-02T20:30:00Z"
  }' | jq .

# Expected: Response should have:
# - impact.impactService != "unknown"
# - chains[] array with multiple elements
# - Each chain contains why-steps with statistical evidence
```

---

## Code Issues That Need Fixing (Post-Seeding)

Even after seeding KPIs, there are code bugs that should be fixed:

### Issue 1: Metric Probes Use Single-Point Queries

**File:** `internal/services/correlation_engine.go` (~line 250)

**Current (WRONG):**
```go
req := &models.MetricsQLQueryRequest{
    Query: qstr,
    Time:  probeEnd.Format(time.RFC3339),  // ← Single point only!
}
res, err := ce.metricsService.ExecuteQuery(ctx, req)
```

**Problem:** Only queries at the END time, misses data trends

**Should be (but check MetricsQLQueryRequest struct first):**
```go
req := &models.MetricsQLQueryRequest{
    Query: qstr,
    Start: probeStart.Format(time.RFC3339),
    End:   probeEnd.Format(time.RFC3339),
    Step:  "60s",
}
```

### Issue 2: No Diagnostic When KPI Registry Empty

**File:** `internal/services/correlation_engine.go` (~line 205)

**Current:**
```go
kpis, _, err := ce.kpiRepo.ListKPIs(ctx, models.KPIListRequest{Limit: 1000})
if err != nil {
    ce.logger.Warn("failed to list KPIs from registry", "err", err)
    // Silently continues with empty kpis list ← BAD!
}
```

**Should add:**
```go
if len(kpis) == 0 {
    ce.logger.Error("CRITICAL: No KPIs in registry. RCA requires KPI definitions.",
        "action", "Seed KPIs via /api/v1/kpi endpoints or run make localdev-seed-data")
}
```

---

## Step-by-Step Execution Order

1. **Verify synthetic data exists:**
   ```bash
   curl 'http://localhost:8428/api/v1/labels?match[]=transactions_failed_total'
   # Should return: {"status":"success","data":["transactions_failed_total"]}
   ```

2. **Check current KPI count:**
   ```bash
   curl 'http://127.0.0.1:8010/api/v1/kpi?limit=1' | jq '.pagination.total'
   # Note the number (e.g., 5)
   ```

3. **Seed new KPIs:**
   ```bash
   ./scripts/seed-transaction-kpis.sh
   ```

4. **Verify KPIs seeded:**
   ```bash
   curl 'http://127.0.0.1:8010/api/v1/kpi?limit=1' | jq '.pagination.total'
   # Should be higher than before (e.g., 10)
   ```

5. **Restart Mirador if needed:**
   ```bash
   make localdev-down
   make localdev-up
   ```

6. **Send fresh synthetic data:**
   ```bash
   cd /path/to/otel-fintrans-simulator
   ./bin/otel-fintrans-simulator \
     --config ./simulator-config.yaml \
     --transactions 50000 \
     --failure-mode mixed
   ```

7. **Call RCA endpoint again:**
   ```bash
   curl -X 'POST' http://127.0.0.1:8010/api/v1/unified/rca \
     -H 'Content-Type: application/json' \
     -d '{
       "startTime": "2025-12-02T19:30:00Z",
       "endTime": "2025-12-02T20:30:00Z"
     }'
   ```

8. **Inspect response:**
   - Should have `chains[]` with multiple why-steps
   - Each step should have statistical correlation evidence
   - Impact should be identified as "Transaction Failure Rate" or similar
   - Root causes should include database/infrastructure metrics

---

## Expected RCA Response After Fix

```json
{
  "status": "success",
  "data": {
    "impact": {
      "id": "incident_1234567890",
      "impactService": "Transaction Failure Rate",  // ← Now resolved!
      "metricName": "rate(transactions_failed_total[1m]) / rate(transactions_total[1m]) * 100",
      "severity": 0.87,
      "impactSummary": "Impact detected on Transaction Failure Rate (correlation confidence 0.85). Top-candidate Database Operation Rate: pearson=0.82 spearman=0.79 partial=0.81 cross_max=0.78 lag=2 anomalies=HIGH"
    },
    "chains": [
      {
        "rank": 1,
        "score": 0.89,
        "steps": [
          {
            "whyIndex": 1,
            "service": "payment-system",
            "component": "Transaction Processor",
            "summary": "Transaction failure rate increased to 25% (was 1%)",
            "evidence": [
              {
                "type": "metric",
                "id": "transactions_failed_total",
                "details": "Rate increased from 50/min to 1250/min"
              }
            ],
            "score": 0.92
          },
          {
            "whyIndex": 2,
            "service": "payment-system",
            "component": "Database",
            "summary": "Database operation latency increased significantly (p95: 500ms → 2500ms)",
            "evidence": [
              {
                "type": "correlation",
                "id": "transaction_latency",
                "details": "Pearson=0.82, lag=2 steps (causes precede impact)"
              }
            ],
            "score": 0.88
          },
          {
            "whyIndex": 3,
            "service": "data-layer",
            "component": "DB Connection Pool",
            "summary": "Connection pool exhausted (active_connections=100/100)",
            "evidence": [
              {
                "type": "anomaly",
                "id": "db_connections",
                "details": "Score: 0.94"
              }
            ],
            "score": 0.85
          }
        ]
      }
    ],
    "notes": [
      "Correlation identified high-confidence suspects",
      "Timeline: T+0s (impact detected) → T-2m (DB latency increased) → T-5m (connection pool stress began)"
    ]
  },
  "timestamp": "2025-12-02T20:30:00Z"
}
```

---

## Troubleshooting If Still Not Working

**Symptom: Still returns "No correlation data"**

1. Verify KPIs actually seeded:
   ```bash
   curl 'http://127.0.0.1:8010/api/v1/kpi' -H "Content-Type: application/json" -d '{"limit": 100}' | jq '.data[] | {name, formula}' | head -20
   ```

2. Check Weaviate connection:
   ```bash
   curl 'http://localhost:8080/v1/.well-known/ready'
   ```

3. Check VictoriaMetrics connection and data:
   ```bash
   curl 'http://localhost:8428/api/v1/query?query=up'
   ```

4. Check logs for specific errors:
   ```bash
   docker logs mirador-core | grep -i "kpi\|correlation\|correlate" | tail -50
   ```

5. Add debug logging manually:
   ```bash
   # Edit correlation_engine.go to log candidateKPIs before return
   # Rebuild: make build
   # Restart: make localdev-down && make localdev-up
   ```

---

## Expected Metrics in Synthetic Data

The otel-fintrans-simulator should be producing these metrics:
- `transactions_total` (counter)
- `transactions_failed_total` (counter)
- `transaction_latency_seconds` (histogram)
- `transaction_latency_seconds_bucket` (histogram bucket)
- `transaction_latency_seconds_sum` (histogram sum)
- `transaction_latency_seconds_count` (histogram count)

Verify with:
```bash
curl 'http://localhost:8428/api/v1/labels' | jq '.data[] | select(contains("transaction")|not|not)'
# Or simpler:
curl 'http://localhost:8428/api/v1/labels' | jq '.data | map(select(contains("transaction")))'
```

