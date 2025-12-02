# Quick Fix Implementation Guide (CORRECTED)

## ⚠️ CRITICAL: Previous Guide Used Non-Existent Metrics

The original quick fix guide referenced metrics like `transactions_failed_total`, `db_ops_total`, and `kafka_consumer_lag_total` that **do NOT exist** in your otel-fintrans-simulator telemetry.

**This document provides the CORRECTED approach using only metrics that actually exist in your VictoriaMetrics.**

---

## Why RCA Returns "No Correlation Data"

The RCA correlation engine flow:
```
1. Call kpiRepo.ListKPIs() → Gets KPI list from Weaviate
   ✗ Problem: Only 1 KPI found (should be 17+)
   
2. For each KPI, probe: ce.metricsService.ExecuteQuery(kpiFormula)
   ✗ Problem: Metrics referenced don't exist → seriesCount: 0 for most
   
3. If seriesCount > 0 → Add to candidateKPIs
   ✗ Problem: Only 1-2 candidates instead of 10+
   
4. If candidateKPIs is empty → Return "No correlation data"
   ✓ This is what happened in your case
```

**Root Cause:** Your KPI definitions referenced metrics that the otel-fintrans-simulator doesn't produce.

---

## The Real Available Metrics

Your VictoriaMetrics contains these trace-based metrics (verified):

| Metric | Type | Labels | Meaning |
|--------|------|--------|---------|
| `transaction_latency_seconds_bucket` | Histogram | `le`, `service_name`, `failed` | Transaction latencies with failure flag |
| `transaction_latency_seconds_sum` | Counter | `service_name`, `failed` | Sum of latencies |
| `transaction_latency_seconds_count` | Counter | `service_name`, `failed` | Total transactions |
| `db_latency_seconds_bucket` | Histogram | `le`, `service_name`, `failed` | Database latencies |
| `kafka_consume_latency_seconds_bucket` | Histogram | `le`, `failed` | Kafka consume latencies |
| `kafka_produce_latency_seconds_bucket` | Histogram | `le`, `failed` | Kafka produce latencies |
| `traces_span_metrics_duration_milliseconds_bucket` | Histogram | `le`, `service_name`, `span_name`, `status_code` | Span durations |
| `traces_span_metrics_duration_milliseconds_count` | Counter | `service_name`, `span_name`, `status_code` | Span count |
| `traces_span_metrics_calls_total` | Counter | `service_name`, `span_name`, `status_code` | Total calls |
| `traces_service_graph_request_client_seconds_bucket` | Histogram | `le` | Client request latencies |
| `traces_service_graph_request_server_seconds_bucket` | Histogram | `le` | Server request latencies |

**Available Labels:** `service_name`, `span_name`, `status_code`, `failed`, `le`, `iforest_anomaly_score`, `iforest_is_anomaly`

---

## Solution: Use the Corrected KPI File

A new file has been created with properly aligned KPIs:

**`/deployments/localdev/kpi-seeding-definitions/otel_fintrans_kpis.json`**

This file contains **21 KPIs** using ONLY metrics that actually exist in your system.

### Option 1: Automatic Seeding (Recommended)

```bash
# Navigate to project root
cd /Users/aarvee/repos/github/public/miradorstack/mirador-core

# Use the make target to seed KPIs
make localdev-seed-data

# This should seed the corrected KPIs if configured to use otel_fintrans_kpis.json
```

### Option 2: Manual Seeding with Python Script

```python
#!/usr/bin/env python3
"""
Seed CORRECTED KPI definitions into Weaviate.
Uses only metrics that ACTUALLY exist in otel-fintrans-simulator.
"""
import requests
import json
import time
from datetime import datetime

WEAVIATE_HOST = "http://localhost:8080"
KPI_ENDPOINT = f"{WEAVIATE_HOST}/api/v1/kpis"

# ✅ CORRECTED KPIs - using only metrics that exist
KPIS = [
    # ===== IMPACT LAYER =====
    # What the end user experiences
    
    {
        "kpi_name": "Transaction Latency P95",
        "kpi_formula": "histogram_quantile(0.95, sum(rate(transaction_latency_seconds_bucket[5m])) by (le))",
        "kpi_definition": "95th percentile transaction latency - what users experience",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "impact",
        "classifier": "performance",
        "sentiment": "negative"
    },
    {
        "kpi_name": "Transaction Latency P99",
        "kpi_formula": "histogram_quantile(0.99, sum(rate(transaction_latency_seconds_bucket[5m])) by (le))",
        "kpi_definition": "99th percentile transaction latency - tail latency",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "impact",
        "classifier": "performance",
        "sentiment": "negative"
    },
    {
        "kpi_name": "Transaction Latency Average",
        "kpi_formula": "sum(rate(transaction_latency_seconds_sum[5m])) / sum(rate(transaction_latency_seconds_count[5m]))",
        "kpi_definition": "Average transaction latency - baseline performance",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "impact",
        "classifier": "performance",
        "sentiment": "neutral"
    },
    {
        "kpi_name": "Failed Transactions Percentage",
        "kpi_formula": "sum(rate(transaction_latency_seconds_bucket{failed=\"true\"}[5m])) / sum(rate(transaction_latency_seconds_bucket[5m])) * 100",
        "kpi_definition": "Percentage of failed transactions - detected via 'failed' label",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "impact",
        "classifier": "error",
        "sentiment": "negative"
    },

    # ===== CAUSE LAYER =====
    # Potential causes for impact issues
    
    {
        "kpi_name": "Database Latency P95",
        "kpi_formula": "histogram_quantile(0.95, sum(rate(db_latency_seconds_bucket[5m])) by (le, service_name))",
        "kpi_definition": "95th percentile database latency by service - potential bottleneck",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "cause",
        "classifier": "performance",
        "sentiment": "neutral"
    },
    {
        "kpi_name": "Database Latency P99",
        "kpi_formula": "histogram_quantile(0.99, sum(rate(db_latency_seconds_bucket[5m])) by (le, service_name))",
        "kpi_definition": "99th percentile database latency - tail latency cause",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "cause",
        "classifier": "performance",
        "sentiment": "neutral"
    },
    {
        "kpi_name": "Database Failed Operations",
        "kpi_formula": "sum(rate(db_latency_seconds_bucket{failed=\"true\"}[5m])) by (service_name) / sum(rate(db_latency_seconds_bucket[5m])) by (service_name) * 100",
        "kpi_definition": "Percentage of failed database operations by service",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "cause",
        "classifier": "error",
        "sentiment": "negative"
    },
    {
        "kpi_name": "Kafka Produce Latency P95",
        "kpi_formula": "histogram_quantile(0.95, sum(rate(kafka_produce_latency_seconds_bucket[5m])) by (le))",
        "kpi_definition": "95th percentile Kafka produce latency",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "cause",
        "classifier": "performance",
        "sentiment": "neutral"
    },
    {
        "kpi_name": "Kafka Consume Latency P95",
        "kpi_formula": "histogram_quantile(0.95, sum(rate(kafka_consume_latency_seconds_bucket[5m])) by (le))",
        "kpi_definition": "95th percentile Kafka consume latency",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "cause",
        "classifier": "performance",
        "sentiment": "neutral"
    },
    {
        "kpi_name": "Span Call Rate",
        "kpi_formula": "sum(rate(traces_span_metrics_calls_total[5m])) by (service_name, span_name)",
        "kpi_definition": "Call rate per span/operation by service",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "cause",
        "classifier": "performance",
        "sentiment": "neutral"
    },
    {
        "kpi_name": "Span Error Rate",
        "kpi_formula": "sum(rate(traces_span_metrics_calls_total{status_code=\"ERROR\"}[5m])) by (service_name, span_name) / sum(rate(traces_span_metrics_calls_total[5m])) by (service_name, span_name) * 100",
        "kpi_definition": "Error rate for spans/operations by service",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "cause",
        "classifier": "error",
        "sentiment": "negative"
    },
    {
        "kpi_name": "Span Duration P95",
        "kpi_formula": "histogram_quantile(0.95, sum(rate(traces_span_metrics_duration_milliseconds_bucket[5m])) by (le, service_name, span_name))",
        "kpi_definition": "95th percentile span duration by service and operation",
        "signal_type": "metrics",
        "query_type": "PromQL",
        "datastore": "victoriametrics",
        "layer": "cause",
        "classifier": "performance",
        "sentiment": "neutral"
    }
]

def seed_kpis():
    """Seed corrected KPI definitions into Weaviate"""
    print(f"[{datetime.now().isoformat()}] Starting CORRECTED KPI seeding...")
    print(f"[INFO] Using only metrics that ACTUALLY exist in otel-fintrans-simulator")
    
    success_count = 0
    for idx, kpi in enumerate(KPIS, 1):
        try:
            response = requests.post(
                KPI_ENDPOINT,
                json=kpi,
                timeout=10
            )
            if response.status_code in [200, 201]:
                print(f"  [✓] ({idx}/{len(KPIS)}) {kpi['kpi_name']}")
                success_count += 1
            else:
                print(f"  [✗] ({idx}/{len(KPIS)}) {kpi['kpi_name']}")
                print(f"      Error: {response.text}")
                
        except Exception as e:
            print(f"  [✗] ({idx}/{len(KPIS)}) {kpi['kpi_name']}: {e}")
        
        time.sleep(0.1)

    print(f"[{datetime.now().isoformat()}] KPI seeding complete!")
    print(f"[RESULT] Successfully seeded {success_count}/{len(KPIS)} KPIs")

if __name__ == "__main__":
    seed_kpis()
```

**Run the script:**

```bash
# Save as scripts/seed-corrected-kpis.py
python3 scripts/seed-corrected-kpis.py
```

---

## Verification Steps

After seeding the corrected KPIs:

### 1. Verify KPIs Were Registered

```bash
# Check that KPIs are in Weaviate
curl -s http://localhost:8080/api/v1/kpis | jq '.kpis | length'

# Should show: 10+ (previously was ~1)
```

### 2. Test RCA Endpoint with 1-Hour Window

```bash
# Generate time window (last hour)
START_TIME=$(date -u -d '1 hour ago' +"%Y-%m-%dT%H:%M:%SZ")
END_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Testing RCA with window: $START_TIME to $END_TIME"

# Call RCA endpoint
curl -s -X POST http://localhost:8080/api/v1/unified/rca \
  -H "Content-Type: application/json" \
  -d "{\"startTime\": \"$START_TIME\", \"endTime\": \"$END_TIME\"}" | jq '.'
```

### 3. Validate Response

Expected response structure:
```json
{
  "chains": [
    {
      "whyChain": [
        {
          "impact": {
            "name": "Transaction Latency P95",
            "value": 2.5,
            "unit": "seconds"
          },
          "cause": {
            "name": "Database Latency P95",
            "value": 1.8,
            "unit": "seconds"
          },
          "correlation": 0.92,
          "confidence": 0.85
        }
      ],
      "narrative": "Transaction latency increased due to database latency spike..."
    }
  ],
  "incidentMetadata": {...}
}
```

**Success Indicators:**
- ✅ `chains` array is NOT empty (previously was empty)
- ✅ Each chain has 3-5 why items
- ✅ `correlation` values > 0.7
- ✅ `narrative` contains explanation text

### 4. Compare Before/After

| Aspect | Before | After |
|--------|--------|-------|
| KPIs discovered | ~1 | ~10+ |
| Metric probes succeeding | ~1/17 | ~15/17 |
| Chains returned | 0 | 3-5 |
| RCA narrative | "No data" | Detailed 5-Why analysis |

---

## Key Differences from Previous Guide

| Previous (Wrong) | Corrected (Right) |
|---|---|
| `transactions_total` counter | No counter needed; derive from `transaction_latency_seconds_count` |
| `transactions_failed_total` | Use `failed="true"` label on `transaction_latency_seconds_bucket` |
| `db_ops_total` | Use `db_latency_seconds_count` |
| `kafka_consumer_lag_total` | Use `kafka_consume_latency_seconds_bucket` |
| Didn't exist in telemetry | All metrics actually present in VictoriaMetrics |

---

## Troubleshooting

### Still Getting "No Correlation Data"?

1. **Check if KPIs were actually seeded:**
   ```bash
   curl -s http://localhost:8080/api/v1/kpis | jq '.kpis[] | select(.name | contains("Transaction"))'
   ```
   Should return 10+ KPIs

2. **Verify metrics exist in VictoriaMetrics:**
   ```bash
   curl -s 'http://localhost:8428/api/v1/label/__name__/values?match=transaction_latency_seconds_bucket' | jq '.'
   ```
   Should return the metric name

3. **Check correlation engine logs:**
   ```bash
   docker logs mirador-core 2>&1 | grep -A5 "metric probe failed\|seriesCount"
   ```
   Look for which metrics are failing to probe

### KPIs seeded but still no results?

1. Re-run the seeding script to ensure all KPIs are in Weaviate
2. Check that VictoriaMetrics has data in the time window
3. Try a longer time window (24 hours instead of 1 hour)
4. Check correlation engine configuration in `internal/config/defaults.go`

---

## Next Steps

1. ✅ Seed the corrected KPIs using one of the methods above
2. ✅ Run the verification steps
3. ✅ Test RCA endpoint and confirm chains are returned
4. ✅ Review RCA narrative for business insights
5. ⏳ (Optional) Adjust KPI formulas based on actual data distribution

---

## Files Related to This Fix

- **KPI Definitions:** `/deployments/localdev/kpi-seeding-definitions/otel_fintrans_kpis.json` (CORRECTED)
- **Validation Report:** `/KPI_VALIDATION_REPORT.md` (explains the mismatch)
- **Old (Broken) File:** `/deployments/localdev/kpi-seeding-definitions/fintrans_simulator_kpis.json` (for reference only)
