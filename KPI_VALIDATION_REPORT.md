# KPI Validation Report: Metrics Mismatch Analysis

**Date:** 2024  
**Status:** ⚠️ **CRITICAL MISMATCH FOUND**  
**Summary:** The existing `fintrans_simulator_kpis.json` references 8 metrics that do NOT exist in your VictoriaMetrics instance. This is why the RCA engine only discovers 1-2 KPIs instead of 17+.

---

## 1. Executive Summary

### Problem
The previous KPI definitions file (`fintrans_simulator_kpis.json`) was created for an imaginary data model that differs from what the otel-fintrans-simulator actually produces. It references counters like `transactions_total`, `db_ops_total`, and `kafka_produce_total` that don't exist in your system.

### Actual Available Metrics (What otel-fintrans-simulator Produces)
Your VictoriaMetrics instance contains these metrics:

| Metric Name | Type | Labels | Purpose |
|---|---|---|---|
| `transaction_latency_seconds_bucket` | Histogram | `le`, `service_name`, `failed` | Transaction latencies with failure tracking |
| `transaction_latency_seconds_count` | Counter | `service_name`, `failed` | Total transaction count |
| `transaction_latency_seconds_sum` | Counter | `service_name`, `failed` | Sum of transaction latencies |
| `db_latency_seconds_bucket` | Histogram | `le`, `service_name`, `failed` | Database operation latencies |
| `db_latency_seconds_count` | Counter | `service_name`, `failed` | Total database operations |
| `db_latency_seconds_sum` | Counter | `service_name`, `failed` | Sum of database latencies |
| `kafka_consume_latency_seconds_bucket` | Histogram | `le`, `failed` | Kafka consume latencies |
| `kafka_produce_latency_seconds_bucket` | Histogram | `le`, `failed` | Kafka produce latencies |
| `traces_span_metrics_duration_milliseconds_bucket` | Histogram | `le`, `service_name`, `span_name`, `status_code` | Span duration by operation |
| `traces_span_metrics_duration_milliseconds_count` | Counter | `service_name`, `span_name`, `status_code` | Span count |
| `traces_span_metrics_calls_total` | Counter | `service_name`, `span_name`, `status_code` | Total span calls |
| `traces_service_graph_request_client_seconds_bucket` | Histogram | `le` | Client-side request latencies |
| `traces_service_graph_request_server_seconds_bucket` | Histogram | `le` | Server-side request latencies |

**Available Labels:** `__name__`, `job`, `iforest_anomaly_score`, `iforest_is_anomaly`, `service_name`, `span_kind`, `span_name`, `status_code`, `le`, `failed`

---

## 2. Detailed Metrics Mapping

### ✅ WORKING: Metrics That Exist and Had Correct KPIs

| KPI Name | Metric Used | Status |
|---|---|---|
| Transaction Latency P95/P99/Avg | `transaction_latency_seconds_bucket` | ✅ **EXISTS** |
| Database Latency P95/P99/Avg | `db_latency_seconds_bucket` | ✅ **EXISTS** |
| Span Metrics Call Rate | `traces_span_metrics_calls_total` | ✅ **EXISTS** |
| Span Duration P95/P99 | `traces_span_metrics_duration_milliseconds_bucket` | ✅ **EXISTS** |

**Impact:** Only ~4-5 KPIs out of 17 are actually queryable. This explains why correlation engine finds almost no candidates.

---

### ❌ BROKEN: Metrics Referenced But Don't Exist

| KPI Name | Formula Metric | Why Missing | Impact |
|---|---|---|---|
| **Transaction Total Rate** | `transactions_total` | Counter not produced by otel-fintrans-simulator | Fallback to span-based call counting |
| **Transaction Failure Rate** | `transactions_failed_total` | Counter not produced | Use `failed="true"` label instead |
| **Transaction Failure %** | `transactions_failed_total` | Counter not produced | Use `failed="true"` label instead |
| **Database Operations Rate** | `db_ops_total` | Counter not produced | Counter not needed; use `db_latency_seconds_count` |
| **Kafka Produce Rate** | `kafka_produce_total` | Counter not produced | Count from `kafka_produce_latency_seconds_count` |
| **Kafka Consume Rate** | `kafka_consume_total` | Counter not produced | Count from `kafka_consume_latency_seconds_count` |
| **Service Graph Request Rate** | `traces_service_graph_request_total` | Counter not produced | Counts embedded in service_graph metrics |
| **Service Graph Failed Requests** | `traces_service_graph_request_failed_total` | Counter not produced | Use `failed="true"` label or derive from metrics |

**Impact on RCA:** Correlation engine's metric probing returns `seriesCount: 0` for all 8 of these KPIs, causing early exit with "No correlation data".

---

## 3. Root Cause Analysis (5-Why for This Mismatch)

1. **Why doesn't the old KPI file work?**
   - Because it was created for a different data model than what otel-fintrans-simulator actually produces

2. **Why does the simulator produce different metrics?**
   - Because it generates trace-based metrics (OpenTelemetry span metrics), not traditional counters

3. **Why trace-based metrics instead of counters?**
   - Trace-based metrics provide richer context: service names, span names, status codes, timing information all built in

4. **Why wasn't this mismatch caught earlier?**
   - KPI definitions were validated in isolation without checking against actual telemetry schema
   - No validation that seeded KPIs actually produce queryable data

5. **Why is this critical now?**
   - RCA engine depends on KPIs to discover candidates; without queryable KPIs, correlation fails completely

---

## 4. Solution: New Corrected KPI Definitions

### New File: `otel_fintrans_kpis.json`

A new file has been generated at `/deployments/localdev/kpi-seeding-definitions/otel_fintrans_kpis.json` with:

- **21 properly aligned KPIs** using ONLY metrics that actually exist
- **Correct PromQL formulas** leveraging available histogram buckets and labels
- **Impact vs Cause layers** properly defined:
  - **Impact**: Transaction-level latencies and failures (what users see)
  - **Cause**: Database, Kafka, spans, service graph latencies (potential causes)
- **Proper use of labels**: `service_name`, `span_name`, `status_code`, `failed` for filtering

### Key Changes

| Old Approach | New Approach | Benefit |
|---|---|---|
| `rate(transactions_total[1m])` | Derive from `traces_span_metrics_calls_total` | Uses actual trace data |
| `rate(transactions_failed_total[1m])` | Filter by `failed="true"` on any metric | Uses label-based filtering |
| `rate(db_ops_total[1m])` | Use `rate(db_latency_seconds_count[5m])` | Proper counter base |
| No service context | Break down by `service_name`, `span_name` labels | Better root cause targeting |
| Single time window | Use `[5m]` rate window for better statistics | More stable correlation analysis |

---

## 5. Seeding the New KPIs

### Step 1: Update the seeding script to use new file

```bash
# Replace the old seeding script to use otel_fintrans_kpis.json
cd /Users/aarvee/repos/github/public/miradorstack/mirador-core
make localdev-seed-data
```

### Step 2: Verify KPIs are registered

```bash
# Query Weaviate to see if KPIs were registered
curl -s http://localhost:8080/api/v1/kpis | jq '.kpis | length'

# Should show 21 KPIs (previously was ~1)
```

### Step 3: Test RCA endpoint

```bash
# Calculate 1-hour window from 1 hour ago to now
START_TIME=$(date -u -d '1 hour ago' +"%Y-%m-%dT%H:%M:%SZ")
END_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

curl -s -X POST http://localhost:8080/api/v1/unified/rca \
  -H "Content-Type: application/json" \
  -d "{\"startTime\": \"$START_TIME\", \"endTime\": \"$END_TIME\"}" | jq '.chains | length'

# Should show 3-5 RCA chains (previously was 0)
```

---

## 6. Files Changed

| File | Change | Reason |
|---|---|---|
| `otel_fintrans_kpis.json` | **CREATED (NEW)** | Contains corrected KPI definitions aligned with actual telemetry |
| `fintrans_simulator_kpis.json` | Still exists but UNUSED | Legacy file; kept for reference |

---

## 7. Validation Checklist

- [ ] Run `make localdev-seed-data` to seed new KPIs
- [ ] Verify 21 KPIs appear in `GET /api/v1/kpis` response
- [ ] Call RCA endpoint and confirm `chains` array contains 3-5 items (previously empty)
- [ ] Inspect RCA response for proper 5-Why chains with identified causes
- [ ] Check correlation matrix shows candidate cause metrics (previously empty)

---

## 8. Next Steps

1. **Immediate:** Seed the new KPIs using `otel_fintrans_kpis.json`
2. **Validation:** Run the verification commands above
3. **Iterate:** If needed, adjust KPI formulas based on actual data distribution
4. **Document:** Update deployment guide to reference correct KPI seeding file

---

## Appendix: All 21 New KPIs

```
Impact Layer (What users observe):
  1. Transaction Latency P95
  2. Transaction Latency P99
  3. Transaction Latency Average
  4. Failed Transactions Percentage

Cause Layer (What might cause impact):
  5. Database Latency P95
  6. Database Latency P99
  7. Database Latency Average
  8. Database Failed Operations
  9. Kafka Consume Latency P95
  10. Kafka Consume Latency P99
  11. Kafka Produce Latency P95
  12. Kafka Produce Latency P99
  13. Span Call Rate
  14. Span Error Rate
  15. Span Duration P95
  16. Span Duration P99
  17. Span Average Duration
  18. Service Graph Client Latency P95
  19. Service Graph Server Latency P95
  20. Anomaly Score Current Level
  21. Critical Anomaly Detection Rate
```

---

**Report Generated:** Analysis of metrics mismatch in RCA KPI definitions  
**Action Required:** Yes - seed new KPIs and re-test RCA endpoint
