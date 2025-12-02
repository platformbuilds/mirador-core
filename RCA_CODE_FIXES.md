# Code-Level Fixes for RCA Correlation Engine

## Overview

This document contains the exact code changes needed to fix the RCA engine's KPI discovery and metric probing issues.

---

## Fix 1: Add Diagnostic Logging for Empty KPI Registry

**File:** `internal/services/correlation_engine.go`

**Location:** Line ~205, in the `Correlate()` method after `kpiRepo.ListKPIs()` call

**Current Code:**
```go
if ce.kpiRepo != nil {
    // List all KPIs relevant to correlation (use high limit to capture all registry KPIs)
    kpis, _, err := ce.kpiRepo.ListKPIs(ctx, models.KPIListRequest{Limit: 1000})
    if err != nil {
        if ce.logger != nil {
            ce.logger.Warn("failed to list KPIs from registry", "err", err)
        }
    } else {
        // For each KPI, attempt a lightweight probe...
```

**Fixed Code:**
```go
if ce.kpiRepo != nil {
    // List all KPIs relevant to correlation (use high limit to capture all registry KPIs)
    kpis, _, err := ce.kpiRepo.ListKPIs(ctx, models.KPIListRequest{Limit: 1000})
    if err != nil {
        if ce.logger != nil {
            ce.logger.Error("CRITICAL: Failed to list KPIs from registry - RCA will not function",
                "error", err,
                "action", "Verify Weaviate is running and accessible. Seed KPIs via /api/v1/kpi endpoints.",
            )
        }
    } else if len(kpis) == 0 {
        // ADD THIS BLOCK
        if ce.logger != nil {
            ce.logger.Error("CRITICAL: KPI registry is empty - RCA will not function",
                "expected", "KPI definitions for metrics/logs/traces",
                "action", "Seed KPIs using scripts/seed-transaction-kpis.sh or POST to /api/v1/kpi",
                "documentation", "See RCA_QUICK_FIX_GUIDE.md for KPI seeding instructions",
            )
        }
    } else {
        // For each KPI, attempt a lightweight probe...
```

---

## Fix 2: Add Progress Tracking for KPI Discovery

**File:** `internal/services/correlation_engine.go`

**Location:** After the KPI registry loop completes (~line 370)

**Add Before Return:**
```go
// ADD THIS LOGGING BLOCK before the final corr := &models.CorrelationResult{...}

if ce.logger != nil {
    ce.logger.Info("KPI discovery phase completed",
        "time_window", fmt.Sprintf("%s to %s", tr.Start.Format(time.RFC3339), tr.End.Format(time.RFC3339)),
        "total_kpis_probed", len(kpis),
        "impact_kpis_found", len(impactKPIs),
        "candidate_kpis_found", len(candidateKPIs),
        "candidate_kpi_list", candidateKPIs,
    )
    
    if len(candidateKPIs) == 0 {
        ce.logger.Error("CRITICAL: No candidate KPIs found after probing",
            "impact_kpis", impactKPIs,
            "check_list", []string{
                "Verify KPIs are registered in Weaviate",
                "Check that metric formulas are correct and return data",
                "Ensure synthetic data is being sent to VictoriaMetrics",
                "Review logs for metric query failures",
            },
        )
    }
}
```

---

## Fix 3: Fix Metric Probing to Use Range Queries (IMPORTANT)

**File:** `internal/services/correlation_engine.go`

**Location:** Line ~250 in the metrics probing section

**Current Code (WRONG):**
```go
if strings.Contains(sig, "metric") || strings.Contains(ds, "metric") || strings.Contains(strings.ToLower(kp.QueryType), "metric") {
    qstr := kp.Formula
    if qstr == "" {
        // Try to derive simple query from Query map if formula absent
        if qmap := kp.Query; qmap != nil {
            if raw, ok := qmap["query"].(string); ok {
                qstr = raw
            }
        }
    }
    if qstr != "" && ce.metricsService != nil {
        req := &models.MetricsQLQueryRequest{
            Query: qstr,
            Time:  probeEnd.Format(time.RFC3339),  // ← WRONG: Single point in time!
        }
        res, err := ce.metricsService.ExecuteQuery(ctx, req)
        if err != nil {
            // error handling
        } else if res != nil && res.SeriesCount > 0 {
            // success handling
        }
    }
}
```

**Fixed Code (RIGHT):**
```go
if strings.Contains(sig, "metric") || strings.Contains(ds, "metric") || strings.Contains(strings.ToLower(kp.QueryType), "metric") {
    qstr := kp.Formula
    if qstr == "" {
        // Try to derive simple query from Query map if formula absent
        if qmap := kp.Query; qmap != nil {
            if raw, ok := qmap["query"].(string); ok {
                qstr = raw
            }
        }
    }
    if qstr != "" && ce.metricsService != nil {
        // Use the middle ring for probing to get data representative of the window
        probeStart := tr.Start
        probeEnd := tr.End
        if len(rings) > 0 {
            middleIdx := len(rings) / 2
            probeStart = rings[middleIdx].Start
            probeEnd = rings[middleIdx].End
        }
        
        req := &models.MetricsQLQueryRequest{
            Query: qstr,
            Start: probeStart.Format(time.RFC3339),  // ← FIXED: Range query
            End:   probeEnd.Format(time.RFC3339),
            Step:  "60s",  // 1-minute granularity
        }
        res, err := ce.metricsService.ExecuteQuery(ctx, req)
        if err != nil {
            if ce.logger != nil {
                ce.logger.Debug("metrics probe failed for KPI", 
                    "kpi", kp.ID, "name", kp.Name, "query", qstr, "err", err)
            }
        } else if res != nil && res.SeriesCount > 0 {
            if ce.logger != nil {
                ce.logger.Debug("metrics probe SUCCESS for KPI", 
                    "kpi", kp.ID, "name", kp.Name, 
                    "series_count", res.SeriesCount, "layer", kp.Layer)
            }
            // extract labels from metrics result
            ur := &models.UnifiedResult{Data: res.Data}
            dls := ce.extractLabelsFromMetricsResult(ur)
            for _, dl := range dls {
                for k, v := range dl.Labels {
                    if labelIndex[kpiID][k] == nil {
                        labelIndex[kpiID][k] = make(map[string]struct{})
                    }
                    labelIndex[kpiID][k][v] = struct{}{}
                }
            }
            candidateKPIs = append(candidateKPIs, kp.ID)
            // If KPI classifies as impact via Layer hint, promote
            if strings.ToLower(kp.Layer) == "impact" {
                impactKPIs = append(impactKPIs, kp.ID)
            }
        }
    }
}
```

---

## Fix 4: Add Health Check at Server Startup

**File:** `internal/api/server.go` (or appropriate server bootstrap location)

**Location:** In `Start()` method or `NewServer()` before returning

**Add:**
```go
// Perform KPI registry health check
func (s *Server) checkKPIRegistryHealth(ctx context.Context) error {
    if s.kpiRepo == nil {
        s.logger.Warn("KPI repository not configured; correlation/RCA will be unavailable")
        return nil
    }
    
    kpis, _, err := s.kpiRepo.ListKPIs(ctx, models.KPIListRequest{Limit: 1})
    if err != nil {
        s.logger.Warn("KPI registry health check failed",
            "error", err,
            "action", "Ensure Weaviate is running and accessible")
        return nil // Non-fatal
    }
    
    if len(kpis) == 0 {
        s.logger.Warn(
            "IMPORTANT: KPI registry is empty. "+
            "RCA/Correlation engines will not produce results. "+
            "Please seed KPIs by running: make localdev-seed-data or "+
            "by POSTing KPI definitions to /api/v1/kpi endpoints. "+
            "See documentation in RCA_QUICK_FIX_GUIDE.md",
        )
    } else {
        s.logger.Info("KPI registry health check passed",
            "kpi_count", kpis, // This will be populated after list call above
        )
    }
    
    return nil
}

// Call this in Start() or NewServer():
if err := s.checkKPIRegistryHealth(ctx); err != nil {
    s.logger.Error("KPI registry health check failed", "error", err)
    // Continue anyway (non-fatal)
}
```

---

## Fix 5: Improve Correlation Result Message When No Data

**File:** `internal/services/correlation_engine.go`

**Location:** Line ~380 in the Correlate() method

**Current Code:**
```go
if len(candidateKPIs) == 0 && len(impactKPIs) == 0 {
    incident := &IncidentContext{
        ID:            "incident_unknown",
        ImpactService: "unknown",
        ImpactSignal: ImpactSignal{
            ServiceName: "unknown",
            MetricName:  "unknown",
            Direction:   "higher_is_worse",
        },
        TimeBounds:    IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End},
        ImpactSummary: fmt.Sprintf("No correlation data for window %s - %s", tr.Start.String(), tr.End.String()),
        Severity: func() float64 {
            if corr == nil {
                return 0.0
            }
            return corr.Confidence
        }(),
        CreatedAt: time.Now().UTC(),
    }
    res := NewRCAIncident(incident)
    res.Notes = append(res.Notes, "Correlation produced no candidates; returning low-confidence RCA")
    return res, nil
}
```

**Improved Code:**
```go
if len(candidateKPIs) == 0 && len(impactKPIs) == 0 {
    diagnosticMsg := fmt.Sprintf(
        "No correlation data found for window %s - %s. "+
        "Possible causes: "+
        "1) KPI registry is empty (seed KPIs via /api/v1/kpi), "+
        "2) No telemetry data in backends for this time window, "+
        "3) Metric queries are invalid or returning no data. "+
        "See logs for 'metrics probe' entries to debug.",
        tr.Start.String(), tr.End.String(),
    )
    
    incident := &IncidentContext{
        ID:            "incident_unknown",
        ImpactService: "unknown",
        ImpactSignal: ImpactSignal{
            ServiceName: "unknown",
            MetricName:  "unknown",
            Direction:   "higher_is_worse",
        },
        TimeBounds:    IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End},
        ImpactSummary: diagnosticMsg,
        Severity: func() float64 {
            if corr == nil {
                return 0.0
            }
            return corr.Confidence
        }(),
        CreatedAt: time.Now().UTC(),
    }
    res := NewRCAIncident(incident)
    res.Notes = append(res.Notes, 
        "No candidates found; check KPI registry and verify metric data exists in backends")
    return res, nil
}
```

---

## Fix 6: Validate MetricsQLQueryRequest Structure Supports Range Queries

**File:** `internal/models/metrics.go`

**Location:** In `MetricsQLQueryRequest` struct definition

**Verify Structure (may already have these fields):**
```go
type MetricsQLQueryRequest struct {
    Query string `json:"query"`
    // For single-point queries:
    Time string `json:"time,omitempty"`
    
    // For range queries (ADD if missing):
    Start string `json:"start,omitempty"`
    End   string `json:"end,omitempty"`
    Step  string `json:"step,omitempty"`  // e.g., "60s"
}
```

If these fields are missing, add them to enable range query support.

---

## Fix 7: Update VictoriaMetrics Query Handler to Accept Range Parameters

**File:** `internal/api/handlers/metricsql_query_handler.go` or similar

**Location:** In the metrics query execution function

**Verify it handles:**
```go
// Check if Start/End provided (range query)
if req.Start != "" && req.End != "" {
    // Range query path
    queryStr := fmt.Sprintf("%s[%s]", req.Query, req.Step)
    params := url.Values{
        "query": []string{queryStr},
        "start": []string{req.Start},
        "end":   []string{req.End},
        "step":  []string{req.Step},
    }
    // Execute against /api/v1/query_range endpoint
} else if req.Time != "" {
    // Point query path
    params := url.Values{
        "query": []string{req.Query},
        "time":  []string{req.Time},
    }
    // Execute against /api/v1/query endpoint
}
```

---

## Testing the Fixes

### Unit Test: Verify KPI Discovery Logic

**File:** `internal/services/correlation_engine_test.go` (new test or add to existing)

```go
func TestCorrelateWithEmptyKPIRegistry(t *testing.T) {
    // Setup: Mock KPI repo that returns empty list
    mockKPIRepo := &MockKPIRepo{
        ListKPIsFunc: func(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
            return []*models.KPIDefinition{}, 0, nil  // Empty registry
        },
    }
    
    engine := NewCorrelationEngine(
        mockMetricsService,
        nil, nil,
        mockKPIRepo,
        mockCache,
        mockLogger,
        config.EngineConfig{},
    )
    
    tr := models.TimeRange{
        Start: time.Now().Add(-1 * time.Hour),
        End:   time.Now(),
    }
    
    result, err := engine.Correlate(context.Background(), tr)
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, 0, len(result.Causes), "Should have no candidate causes when KPI registry is empty")
}

func TestCorrelateWithRangeQueryMetrics(t *testing.T) {
    // Setup: Mock KPI repo with test KPIs
    mockKPIRepo := &MockKPIRepo{
        ListKPIsFunc: func(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
            return []*models.KPIDefinition{
                {
                    ID:      "kpi-test-1",
                    Name:    "Test Metric",
                    Formula: "test_metric",
                    Layer:   "impact",
                },
            }, 1, nil
        },
    }
    
    // Mock metrics service - should receive range query parameters
    mockMetricsService := &MockMetricsService{
        ExecuteQueryFunc: func(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
            // Verify that Start/End are provided (not just Time)
            assert.NotEmpty(t, req.Start, "Range queries should have Start parameter")
            assert.NotEmpty(t, req.End, "Range queries should have End parameter")
            assert.NotEmpty(t, req.Step, "Range queries should have Step parameter")
            
            return &models.MetricsQLQueryResult{
                SeriesCount: 1,
                Data: map[string]interface{}{
                    "result": []map[string]interface{}{},
                },
            }, nil
        },
    }
    
    engine := NewCorrelationEngine(
        mockMetricsService,
        nil, nil,
        mockKPIRepo,
        mockCache,
        mockLogger,
        config.EngineConfig{},
    )
    
    tr := models.TimeRange{
        Start: time.Now().Add(-1 * time.Hour),
        End:   time.Now(),
    }
    
    result, err := engine.Correlate(context.Background(), tr)
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
    // After fix, should find at least one candidate
    assert.Greater(t, len(result.Causes), 0)
}
```

### Integration Test: Full RCA Flow

**Command:**
```bash
# 1. Verify KPIs seeded
curl 'http://127.0.0.1:8010/api/v1/kpi?limit=1' | jq '.pagination.total'

# 2. Verify metrics exist
curl 'http://127.0.0.1:8428/api/v1/labels?match[]=transactions_failed_total' | jq .

# 3. Execute RCA with 1-hour window
curl -X POST 'http://127.0.0.1:8010/api/v1/unified/rca' \
  -H 'Content-Type: application/json' \
  -d '{
    "startTime": "2025-12-02T19:30:00Z",
    "endTime": "2025-12-02T20:30:00Z"
  }' | jq .

# 4. Verify response has chains
# Expected: data.chains should be non-empty array
```

---

## Deployment Checklist

- [ ] Apply Fix 1: Diagnostic logging
- [ ] Apply Fix 2: Progress tracking  
- [ ] Apply Fix 3: Range query fixing (MOST IMPORTANT)
- [ ] Apply Fix 4: Server health check
- [ ] Apply Fix 5: Improved error messages
- [ ] Verify Fix 6: MetricsQLQueryRequest structure
- [ ] Verify Fix 7: Handler range query support
- [ ] Add unit tests from Fix 7 section
- [ ] Rebuild: `make build`
- [ ] Restart: `make localdev-down && make localdev-up`
- [ ] Seed KPIs: `./scripts/seed-transaction-kpis.sh`
- [ ] Run integration test
- [ ] Verify RCA response contains chains with why-steps

