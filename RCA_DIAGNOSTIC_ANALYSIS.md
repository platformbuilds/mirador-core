# RCA Endpoint Failure Analysis - Synthetic Telemetry with 25% Failures

## Executive Summary

**Problem:** The RCA endpoint returns "No correlation data for window" despite synthetic telemetry data showing 25% failure rates.

**Root Cause:** The correlation engine's KPI discovery phase (`Correlate()` in `correlation_engine.go`) is **failing to discover any KPIs from the KPI repository**, causing it to return early with zero correlation candidates.

**Key Finding:** In the logs, you can see:
- Multiple `rate(transaction_failed_total[1m])` queries returning `seriesCount:0`
- Final log shows: `"notes": ["Correlation produced no candidates; returning low-confidence RCA"]`
- This indicates KPIs are being probed but **finding NO data** because either:
  1. KPIs are not registered in Weaviate/KPI repository
  2. The queries are not being executed against the correct time ranges
  3. The synthetic data is not actually present in VictoriaMetrics

---

## Detailed Root Cause Analysis

### 1. Flow Analysis: RCA → Correlation → KPI Discovery

```
POST /api/v1/unified/rca
  ↓
RCAHandler.HandleComputeRCA()
  ↓
RCAEngine.ComputeRCAByTimeRange(tr)
  ↓
CorrelationEngine.Correlate(ctx, tr)  ← THIS IS WHERE FAILURE OCCURS
  ↓
KPI Discovery Phase (lines ~200-380 in correlation_engine.go)
  • Call: ce.kpiRepo.ListKPIs(ctx, models.KPIListRequest{Limit: 1000})
  • For each KPI, execute probe queries against metrics/logs/traces backends
  • If seriesCount > 0, add to candidateKPIs or impactKPIs
  ↓
Issue: candidateKPIs remains empty []
  ↓
Early return with:
  corr.Causes = []  // empty candidates
  corr.AffectedServices = []  // empty
  corr.RedAnchors = []  // empty
  ↓
RCAEngine receives nil correlation data and builds low-confidence incident
  with "incident_unknown" and "No correlation data for window"
```

### 2. The Critical Code Section

**File:** `internal/services/correlation_engine.go:200-380` (`Correlate()` method)

```go
// KPI-first discovery: list KPIs from Stage-00 KPI registry
kpis, _, err := ce.kpiRepo.ListKPIs(ctx, models.KPIListRequest{Limit: 1000})
if err != nil {
    ce.logger.Warn("failed to list KPIs from registry", "err", err)
    // Falls back to empty list - NO KPIs DISCOVERED
}

// For each KPI in the list, probe the backend
for _, kp := range kpis {
    // Probe metrics, logs, traces backends
    res, err := ce.metricsService.ExecuteQuery(ctx, req)
    if err == nil && res != nil && res.SeriesCount > 0 {
        candidateKPIs = append(candidateKPIs, kp.ID)
    }
}

// If no candidates were found:
if len(candidateKPIs) == 0 && len(impactKPIs) == 0 {
    return &models.CorrelationResult{
        Causes: []models.CauseCandidate{},  // EMPTY
        AffectedServices: [],  // EMPTY
        // ... returns early with low confidence
    }
}
```

### 3. Diagnostic Evidence from Your Logs

From the RCA endpoint response, all queries return `seriesCount:0`:

```json
{"level":"info","timestamp":"2025-12-02T14:14:35.608Z",
 "message":"MetricsQL query executed",
 "query":"redis_master_repl_offset - redis_slave_repl_offset",
 "seriesCount":0}  ← NO DATA FOUND

{"level":"info","timestamp":"2025-12-02T14:14:35.614Z",
 "message":"MetricsQL query executed",
 "query":"rate(db_ops_total[1m])",
 "seriesCount":0}  ← NO DATA FOUND

{"level":"debug","timestamp":"2025-12-02T14:14:35.661Z",
 "message":"metrics probe SUCCESS for KPI",
 "kpi":"a302420b-0836-5ce9-b8b8-694aab1fc999",
 "name":"Server Health Score",
 "series_count":1}  ← ONLY 1 KPI FOUND

{"level":"debug","timestamp":"2025-12-02T14:14:35.679Z",
 "message":"Attempting to resolve UUID to KPI",
 "candidate":"unknown"}

{"level":"info","timestamp":"2025-12-02T14:14:35.681Z",
 "message":"KPI not found while attempting resolution",
 "candidate":"unknown"}
```

**Key Observations:**
1. Only 1 KPI was successfully discovered: `Server Health Score` (a302420b...)
2. **It returned `seriesCount:1` (SUCCESS)** but this single KPI wasn't enough to produce candidates
3. The `candidateKPIs` list likely contains only this one KPI
4. **The synthetic transaction failure data (which is what's generating the 25% failures) is NOT being found**

---

## The Missing Piece: Where Are the Transaction Failure KPIs?

Your synthetic telemetry is sending transaction failure data, but the correlation engine is **not finding these KPIs** because:

### Hypothesis 1: KPIs Not Registered in Weaviate (Most Likely)

The synthetic telemetry simulator creates metrics like:
- `transaction_failed_total`
- `transactions_total`
- These are NOT automatically registered as KPIs

**Evidence:**
- Your queries include hardcoded metric names like `rate(transactions_failed_total[1m])`
- These are in the query list but return `seriesCount:0`
- Weaviate KPI registry is empty or doesn't contain these definitions

**Fix Required:**
```yaml
# KPI definitions need to be in Weaviate/KPI repository
- ID: kpi-transaction-failure-rate
  Name: "Transaction Failure Rate"
  Formula: "rate(transactions_failed_total[1m]) / rate(transactions_total[1m]) * 100"
  Layer: "impact"
  QueryType: "metrics"
  DataStore: "metrics"
```

### Hypothesis 2: Queries Against Wrong Time Range

The probes use:
```go
probeStart := tr.Start
probeEnd := tr.End
```

But VictoriaMetrics might be queried with:
```
time=probeEnd.Format(time.RFC3339)
```

This only queries at a **single point in time** (the end time), not across the range.

**Fix Required:** Use `start` and `end` parameters for range queries:
```go
// Instead of single point query:
Query: kp.Formula,
Time:  probeEnd.Format(time.RFC3339),

// Should be:
Query: kp.Formula,
Start: probeStart.Format(time.RFC3339),
End:   probeEnd.Format(time.RFC3339),
```

---

## Critical Issue: Fallback to Empty Probes

From `internal/config/defaults.go`:

```go
Engine: EngineConfig{
    // NOTE(HCB-001): Probes removed per AGENTS.md §3.6 - must be populated 
    // via KPI registry or external config.
    Probes: []string{},  // ← EMPTY!
    ServiceCandidates: []string{},  // ← EMPTY!
}
```

**The implementation enforces registry-driven KPI discovery**, but if the registry is empty (Weaviate has no KPIs), the entire system fails silently.

---

## The 5-Why Analysis

### Why 1: No RCA chains returned?
→ Because Correlation engine found no candidate causes

### Why 2: Correlation found no candidates?
→ Because KPI discovery returned empty candidateKPIs list

### Why 3: KPI discovery failed?
→ Because either:
  - **KPI repository (Weaviate) is empty** (no KPI definitions)
  - **OR KPI probes are querying single points in time instead of ranges**
  - **OR synthetic telemetry isn't actually reaching VictoriaMetrics**

### Why 4: Why is Weaviate empty?
→ KPIs must be manually created/seeded; they don't auto-populate from metrics

### Why 5: Why wasn't this caught earlier?
→ The system designed for registry-driven discovery (good architecture)
  but lacks safeguards when registry is empty (bad operational UX)

---

## Verification Steps

To confirm root cause, run these diagnostic commands:

```bash
# 1. Check if KPIs exist in Weaviate
curl -X POST http://localhost:8080/v1/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ Get { KPIDefinition { ID Name Formula } } }"}'

# Expected: Should return KPI definitions including transaction metrics
# Actual: Likely returns empty list []

# 2. Check if metrics exist in VictoriaMetrics
curl 'http://localhost:8428/api/v1/labels?match[]=transactions_failed_total'

# Expected: Should list the metric
# Actual: Check if metric is present

# 3. Query specific metric
curl 'http://localhost:8428/api/v1/query_range?query=rate(transactions_failed_total[1m])&start=2025-12-02T19:30:00Z&end=2025-12-02T20:30:00Z&step=60'

# Expected: Should return data series
# Actual: Check if data is present in time window
```

---

## Recommended Fixes (In Priority Order)

### Fix 1: Seed KPI Definitions into Weaviate (CRITICAL)

The synthetic telemetry system needs to auto-register its metrics as KPIs:

**Location:** `scripts/localdev_seed_kpis.py` or new script

```python
kpis = [
    {
        "ID": "kpi-transaction-failure-rate",
        "Name": "Transaction Failure Rate",
        "Formula": "rate(transactions_failed_total[1m]) / rate(transactions_total[1m]) * 100",
        "Layer": "impact",
        "QueryType": "metrics",
        "DataStore": "metrics",
        "SignalType": "metric",
        "Classifier": "error_rate",
    },
    {
        "ID": "kpi-transaction-latency-p95",
        "Name": "Transaction Latency P95",
        "Formula": "histogram_quantile(0.95, rate(transaction_latency_seconds_bucket[1m]))",
        "Layer": "impact",
        "QueryType": "metrics",
        "DataStore": "metrics",
        "SignalType": "metric",
    },
    # ... more KPI definitions for all synthetic metrics
]

# POST to /api/v1/kpi (bulk create)
for kpi in kpis:
    POST /api/v1/kpi with kpi definition
```

### Fix 2: Fix MetricsQL Probing to Use Range Queries (HIGH)

**File:** `internal/services/correlation_engine.go:~250`

Current code (WRONG):
```go
req := &models.MetricsQLQueryRequest{
    Query: qstr,
    Time:  probeEnd.Format(time.RFC3339),  // ← Single point in time
}
```

Should be (CORRECT):
```go
req := &models.MetricsQLQueryRequest{
    Query: qstr,
    Start: probeStart.Format(time.RFC3339),  // ← Range query
    End:   probeEnd.Format(time.RFC3339),
    Step:  "60s",
}
```

**Files to Update:**
- `internal/services/correlation_engine.go:~250` (metrics probe)
- `internal/services/correlation_engine.go:~310` (logs probe)
- `internal/services/correlation_engine.go:~330` (traces probe)

### Fix 3: Add Health Check for Empty KPI Registry (MEDIUM)

Create a startup diagnostic:

```go
// In server startup (bootstrap)
kpis, _, _ := kpiRepo.ListKPIs(ctx, models.KPIListRequest{Limit: 1})
if len(kpis) == 0 {
    logger.Warn(
        "CRITICAL: KPI registry is empty. RCA/Correlation will not function. "+
        "Please run: make localdev-seed-data or POST KPI definitions to /api/v1/kpi/bulk",
    )
}
```

### Fix 4: Add Diagnostic Logging to Correlation (MEDIUM)

```go
ce.logger.Info("KPI discovery phase starting", 
    "time_window", fmt.Sprintf("%s - %s", tr.Start, tr.End),
    "num_kpis_in_registry", len(kpis),
)

ce.logger.Info("KPI discovery phase completed",
    "impact_kpis_found", len(impactKPIs),
    "candidate_kpis_found", len(candidateKPIs),
    "kpi_ids", candidateKPIs,
)

if len(candidateKPIs) == 0 {
    ce.logger.Error("CRITICAL: No candidate KPIs found. RCA will have no candidates.",
        "time_window", fmt.Sprintf("%s - %s", tr.Start, tr.End),
        "num_probed", len(kpis),
        "check", "Verify KPIs are registered in Weaviate and data exists in backends",
    )
}
```

---

## Implementation Plan

### Phase 1: Immediate Fix (Today)
1. Seed KPI definitions for synthetic metrics to Weaviate
   - Script: `make localdev-seed-kpis` (verify it creates impact/cause KPIs)
   - Verify: Query Weaviate GraphQL, confirm > 10 KPIs returned

2. Verify synthetic data in VictoriaMetrics
   ```bash
   # Check metrics exist
   curl 'http://localhost:8428/api/v1/labels?match[]=transactions_failed_total'
   ```

### Phase 2: Code Fixes (Short-term)
1. Update `Correlate()` method to use proper range queries
2. Add diagnostic logging
3. Add startup health check

### Phase 3: Validation
1. Restart services
2. Re-run RCA endpoint with same 1-hour window
3. Verify: RCA response contains `chains` with why-steps
4. Verify: Each chain has statistical evidence (Pearson, Spearman, partial correlation)

---

## Additional Context: Why 5-Whys Engine Should Work Once Discovered

Once KPIs are discovered, the RCA engine will:

1. **Correlate** phase:
   - Builds temporal rings (R1: immediate, R2: short, R3: medium, R4: long)
   - Computes Pearson, Spearman, Cross-correlation (with lag) for each Impact–Cause pair
   - Includes Partial correlation (conditioning on confounders to reduce false positives)
   - Scores candidates using `ComputeSuspicionScore()` function
   
2. **Template-based Why-Chain** generation:
   - Selects top candidate by suspicion score
   - Builds template explanations:
     - "Why 1: Transaction failures observed (metric: X changed from Y to Z)"
     - "Why 2: Service Y latency increased (correlation: Pearson=0.85)"
     - "Why 3: Database connection pool exhausted (anomaly: 0.92)"
     - "Why 4: Resource contention in Kubernetes (topology: 3 hops upstream)"
     - "Why 5: Process leak undetected by monitoring (RCA narrative)"

3. **Returns structured RCA response:**
   ```json
   {
     "chains": [
       {
         "steps": [
           {"why": 1, "summary": "..."},
           {"why": 2, "summary": "..."},
           ...
         ],
         "score": 0.89
       }
     ]
   }
   ```

---

## AGENTS.md Compliance Check

Your system SHOULD be compliant with AGENTS.md but has a **configuration gap**:

✅ **Compliant:**
- Uses time-window-only API (`{startTime, endTime}`)
- Implements bucket/ring strategy (`BuildRings()`)
- Computes statistical correlation (Pearson, Spearman, Partial, Cross-correlation)
- Uses registry-driven KPI discovery (no hardcoded metric names in engine)
- Temporal anchoring with adaptive rings

❌ **Gap:**
- **No initial KPI seeding** – system expects pre-populated Weaviate registry
- **Documentation doesn't clarify KPI setup requirement** – users don't know to seed KPIs first
- **No operational health check** – silent failure when registry empty

---

## Testing After Fix

```bash
# 1. Seed KPIs
make localdev-seed-data

# 2. Send synthetic failures
cd otel-fintrans-simulator && \
  ./bin/otel-fintrans-simulator \
    --config ./simulator-config.yaml \
    --data-interval 1s \
    --failure-mode mixed \
    --transactions 50000 \
    --concurrency 2000

# 3. Call RCA endpoint (1 hour window ending now)
curl -X 'POST' http://127.0.0.1:8010/api/v1/unified/rca \
  -H 'Content-Type: application/json' \
  -d '{
    "startTime": "2025-12-02T19:30:00Z",
    "endTime": "2025-12-02T20:30:00Z"
  }'

# Expected: RCA response with chains containing 5-why analysis
# with high-confidence statistics
```

---

## Summary Table

| Issue | Root Cause | Impact | Fix |
|-------|-----------|--------|-----|
| No candidates found | KPI registry empty/no data | RCA returns low-confidence | Seed KPI definitions |
| Low seriesCount | Queries use single-point time | Metric probes miss data | Use range queries |
| Silent failure | No diagnostics on empty KPIs | Hard to debug | Add health checks |
| Wrong time windows | Probes don't align with buckets | Correlations invalid | Fix time range logic |

