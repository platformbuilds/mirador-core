# RCA Analysis - Verification & Action Checklist

## Pre-Fix Verification (Confirm the Problem)

```bash
# 1. Verify Weaviate is accessible
curl -i http://localhost:8080/v1/.well-known/ready
# Expected: 200 OK

# 2. Verify VictoriaMetrics has synthetic metrics
curl 'http://localhost:8428/api/v1/labels?match[]=transactions_failed_total'
# Expected: status=success, data should include transaction metrics

# 3. Check current KPI count in repository
curl 'http://127.0.0.1:8010/api/v1/kpi?limit=1' \
  -H 'Content-Type: application/json' \
  -d '{"limit": 100}' | jq '.pagination.total'
# Note this number (e.g., 5)

# 4. Verify the problem: Call RCA endpoint
curl -X 'POST' 'http://127.0.0.1:8010/api/v1/unified/rca' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
    "startTime": "2025-12-02T19:30:00Z",
    "endTime": "2025-12-02T20:30:00Z"
  }' | jq '.data | {impactService, chains}'

# Expected Result:
# {
#   "impactService": "unknown",
#   "chains": []
# }
# ← This confirms the problem exists
```

---

## Quick Fix (Immediate Solution)

### Option A: Using Script (Recommended)

```bash
# 1. Create seed script
cat > ./scripts/seed-transaction-kpis.sh << 'EOF'
#!/bin/bash
MIRADOR_API="http://127.0.0.1:8010"

# Transaction Failure Rate
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
    "sentiment": "higher_is_worse",
    "tags": ["synthetic", "transaction", "failure"]
  }'

# Transaction Success Rate
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
    "sentiment": "higher_is_better",
    "tags": ["synthetic", "transaction", "success"]
  }'

# Database Operation Rate
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
    "sentiment": "neutral",
    "tags": ["synthetic", "database", "operations"]
  }'

# Server Health Score
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
    "sentiment": "higher_is_better",
    "tags": ["synthetic", "infrastructure"]
  }'

echo "KPIs seeded successfully"
EOF

# 2. Execute script
chmod +x ./scripts/seed-transaction-kpis.sh
./scripts/seed-transaction-kpis.sh
```

### Option B: Manual curl (One-by-one)

```bash
# If script doesn't work, seed manually:

MIRADOR_API="http://127.0.0.1:8010"

# KPI 1: Transaction Failure Rate
curl -X POST "$MIRADOR_API/api/v1/kpi" \
  -H "Content-Type: application/json" \
  -d '{"name":"Transaction Failure Rate","kind":"kpi","formula":"rate(transactions_failed_total[1m]) / rate(transactions_total[1m]) * 100","layer":"impact","query_type":"metrics","datastore":"metrics","signal_type":"metric","sentiment":"higher_is_worse","tags":["synthetic","transaction","failure"]}'

# KPI 2: Database Operation Rate  
curl -X POST "$MIRADOR_API/api/v1/kpi" \
  -H "Content-Type: application/json" \
  -d '{"name":"Database Operation Rate","kind":"kpi","formula":"rate(db_ops_total[1m])","layer":"cause","query_type":"metrics","datastore":"metrics","signal_type":"metric","sentiment":"neutral","tags":["synthetic","database","operations"]}'
```

---

## Post-Fix Verification

### Step 1: Verify KPIs Seeded

```bash
# Check KPI count increased
curl 'http://127.0.0.1:8010/api/v1/kpi?limit=1' \
  -H 'Content-Type: application/json' \
  -d '{"limit": 100}' | jq '.pagination.total'
# Should be higher than before (was ~5, now should be 8+)

# List all KPIs to verify seeding
curl 'http://127.0.0.1:8010/api/v1/kpi?limit=100' \
  -H 'Content-Type: application/json' \
  -d '{"limit": 100}' | jq '.data[] | {name, layer, formula}' | head -20
# Should include "Transaction Failure Rate", "Database Operation Rate", etc.
```

### Step 2: Verify Metrics Data Exists

```bash
# Query for transaction failure metrics in VictoriaMetrics
curl 'http://localhost:8428/api/v1/query_range?query=rate(transactions_failed_total[1m])&start=2025-12-02T19:30:00Z&end=2025-12-02T20:30:00Z&step=60'
# Expected: Should return data with multiple points

# Or use simpler format:
curl 'http://localhost:8428/api/v1/query?query=transactions_failed_total'
# Expected: Should return series with current value
```

### Step 3: Restart Services (Recommended)

```bash
# Stop and start containers to reload KPIs
make localdev-down
sleep 10
make localdev-up
sleep 30  # Wait for services to start

# Verify services are ready
curl -i http://localhost:8080/v1/.well-known/ready  # Weaviate
curl -i http://localhost:8428/api/v1/query?query=up  # VictoriaMetrics
```

### Step 4: Test RCA Again

```bash
# Same request as before, but now should return chains
curl -X 'POST' 'http://127.0.0.1:8010/api/v1/unified/rca' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
    "startTime": "2025-12-02T19:30:00Z",
    "endTime": "2025-12-02T20:30:00Z"
  }' | jq '.data | {impactService, chains_count: (.chains | length)}'

# Expected Result:
# {
#   "impactService": "Transaction Failure Rate",
#   "chains_count": 1
# }
# ← This indicates SUCCESS!
```

### Step 5: Full Verification of RCA Response

```bash
# Get full response and verify structure
curl -X 'POST' 'http://127.0.0.1:8010/api/v1/unified/rca' \
  -H 'Content-Type: application/json' \
  -d '{
    "startTime": "2025-12-02T19:30:00Z",
    "endTime": "2025-12-02T20:30:00Z"
  }' | jq '.data | {
    impact_service: .impact.impactService,
    impact_summary: .impact.impactSummary,
    chains_count: (.chains | length),
    first_chain_score: .chains[0].score,
    first_why: .chains[0].steps[0].why,
    first_summary: .chains[0].steps[0].summary,
    notes: .notes
  }'

# Expected:
# {
#   "impact_service": "Transaction Failure Rate",
#   "impact_summary": "Impact detected on Transaction Failure Rate...",
#   "chains_count": 1,
#   "first_chain_score": 0.89,  # Some positive score
#   "first_why": 1,
#   "first_summary": "Transaction failures...",
#   "notes": ["Correlation identified..."]
# }
```

---

## Troubleshooting If Fix Doesn't Work

### Issue: KPIs still not appearing

```bash
# 1. Verify KPI POST was successful
curl -X POST 'http://127.0.0.1:8010/api/v1/kpi' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Test KPI",
    "kind": "kpi",
    "formula": "up",
    "layer": "test",
    "query_type": "metrics",
    "datastore": "metrics"
  }' | jq .

# Check for errors in response

# 2. Check server logs
docker logs mirador-core | tail -50 | grep -i "kpi\|weaviate"

# 3. Verify Weaviate is really accessible
curl -v http://localhost:8080/v1/.well-known/ready
```

### Issue: Metrics not found in VictoriaMetrics

```bash
# 1. Verify otel-fintrans-simulator is sending data
# Check if simulator is running
ps aux | grep otel-fintrans-simulator

# 2. List all available metrics
curl 'http://localhost:8428/api/v1/labels'

# 3. Check if transactions_* metrics exist
curl 'http://localhost:8428/api/v1/labels?match[]=transactions_'

# 4. Query raw data
curl 'http://localhost:8428/api/v1/query?query=transactions_total'

# 5. If no data, restart simulator
cd otel-fintrans-simulator
./bin/otel-fintrans-simulator \
  --config ./simulator-config.yaml \
  --transactions 50000 \
  --failure-mode mixed
```

### Issue: RCA still returns "unknown"

```bash
# 1. Check correlation engine logs
docker logs mirador-core | grep -i "correlation\|candidate" | tail -20

# Look for: "KPI discovery phase" or "metrics probe"

# 2. Enable debug logging (if available)
curl -X POST 'http://127.0.0.1:8010/api/v1/config' \
  -H 'Content-Type: application/json' \
  -d '{"log_level": "debug"}'

# 3. Re-run RCA and check logs again
docker logs mirador-core | grep -i "debug\|kpi\|probe" | tail -50

# 4. Verify time window is correct
# Current time: 2025-12-02T20:30:00Z (check if within your data range)
date -u +%Y-%m-%dT%H:%M:%SZ
```

---

## Performance Check

```bash
# Measure RCA endpoint latency
time curl -X 'POST' 'http://127.0.0.1:8010/api/v1/unified/rca' \
  -H 'Content-Type: application/json' \
  -d '{
    "startTime": "2025-12-02T19:30:00Z",
    "endTime": "2025-12-02T20:30:00Z"
  }' > /dev/null 2>&1

# Expected: Should complete in < 2 seconds
# If slower: Check correlation engine performance
```

---

## Success Criteria Checklist

- [ ] `curl http://localhost:8080/v1/.well-known/ready` returns 200 OK
- [ ] `curl 'http://localhost:8428/api/v1/labels?match[]=transactions_failed_total'` includes metric
- [ ] KPI count increased after seeding
- [ ] `curl 'http://127.0.0.1:8010/api/v1/kpi?limit=100'` includes "Transaction Failure Rate"
- [ ] RCA endpoint returns `impactService` != "unknown"
- [ ] RCA endpoint returns `chains[]` with entries
- [ ] First chain has multiple steps (why: 1, 2, 3, ...)
- [ ] Response includes statistical evidence (Pearson, Spearman)
- [ ] Response includes lag information (causality order)
- [ ] Response completes in < 2 seconds

---

## Rollback (If Something Goes Wrong)

```bash
# 1. Stop services
make localdev-down

# 2. Clear Weaviate data (optional)
docker volume rm mirador-core_weaviate-data 2>/dev/null || true

# 3. Restart without seeded KPIs
make localdev-up

# 4. RCA will return low-confidence again (expected)
```

---

## Documentation References

For more detailed information, see:
- `README_RCA_ANALYSIS.md` - Overview and index
- `RCA_ANALYSIS_SUMMARY.md` - Executive summary
- `RCA_DIAGNOSTIC_ANALYSIS.md` - Technical deep-dive
- `RCA_QUICK_FIX_GUIDE.md` - Step-by-step fix guide
- `RCA_CODE_FIXES.md` - Code-level improvements

---

## Next Steps (After Successful Fix)

1. **Document the setup process**
   - Add KPI seeding to `docs/getting-started.md`
   - Update deployment guide with "seed KPIs" step

2. **Implement code improvements**
   - Apply fixes from `RCA_CODE_FIXES.md`
   - Add startup health check
   - Improve diagnostic logging

3. **Automate KPI seeding**
   - Create initialization script for new deployments
   - Add to Docker entrypoint if needed

4. **Test with different scenarios**
   - Different failure modes (10%, 50%, 90%)
   - Different time windows (5m, 1h, 24h)
   - Multiple concurrent failures

---

**Created:** 2025-12-02  
**Status:** Complete - Ready for Implementation  
**Last Updated:** 2025-12-02

