package rca

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// fakeCorrEngine is a tiny adapter that satisfies the minimal Correlate
// dependency used by RCAEngineImpl.ComputeRCAByTimeRange. It returns the
// pre-populated CorrelationResult provided at construction time.
type fakeCorrEngine struct {
	res *models.CorrelationResult
}

func (f *fakeCorrEngine) Correlate(ctx context.Context, tr models.TimeRange) (*models.CorrelationResult, error) {
	return f.res, nil
}

func TestRCA_SelectsTopSuspicionCauseAsFocal(t *testing.T) {
	// Build a synthetic CorrelationResult with three causes having different suspicion scores
	now := time.Now()
	corr := &models.CorrelationResult{
		CorrelationID:    "test-corr-1",
		Confidence:       0.8,
		AffectedServices: []string{}, // empty so RCA must pick from Causes
		Causes: []models.CauseCandidate{
			{KPI: "kpi.low", Service: "svc-low", SuspicionScore: 0.2, Reasons: []string{"weak"}},
			{KPI: "kpi.high", Service: "svc-high", SuspicionScore: 0.95, Reasons: []string{"strong"}, Stats: &models.CorrelationStats{Pearson: 0.9, Spearman: 0.88, CrossCorrMax: 0.8, CrossCorrLag: 1, SampleSize: 4}},
			{KPI: "kpi.mid", Service: "svc-mid", SuspicionScore: 0.5, Reasons: []string{"moderate"}},
		},
		RedAnchors: []*models.RedAnchor{{Service: "svc-high", Metric: "kpi.high", Score: 0.9, Timestamp: now}},
		CreatedAt:  now,
	}

	// Wire fake correlation engine
	fake := &fakeCorrEngine{res: corr}

	// Create RCA engine with fake corr engine. CandidateCauseService and ServiceGraph can be nil for this test.
	log := logger.New("error")
	rcaEngine := NewRCAEngine(nil, nil, log, config.EngineConfig{}, fake)

	// Call ComputeRCAByTimeRange which should pick the top SuspicionScore candidate as focal cause
	tr := TimeRange{Start: now.Add(-5 * time.Minute), End: now}
	r, err := rcaEngine.ComputeRCAByTimeRange(context.Background(), tr)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Basic checks: chains were produced and a root cause was selected
	require.Greater(t, len(r.Chains), 0)
	require.NotNil(t, r.RootCause)

	// The RCA incident ImpactSummary should include a concise snippet referencing
	// the top candidate KPI (the implementation appends a template-based stat
	// mention). Validate deterministically that top-candidate KPI is present.
	require.Contains(t, r.Impact.ImpactSummary, "kpi.high")
}

func TestRCA_IncludesPartialAndAnomalyHint(t *testing.T) {
	now := time.Now()
	corr := &models.CorrelationResult{
		CorrelationID:    "test-corr-2",
		Confidence:       0.9,
		AffectedServices: []string{"svc-x"},
		Causes: []models.CauseCandidate{
			{KPI: "kpi.x", Service: "svc-x", SuspicionScore: 0.9, Reasons: []string{"strong", "high_anomaly_density"}, Stats: &models.CorrelationStats{Pearson: 0.85, Spearman: 0.82, Partial: 0.2, CrossCorrMax: 0.7, CrossCorrLag: 1, SampleSize: 6}},
		},
		RedAnchors: []*models.RedAnchor{{Service: "svc-x", Metric: "kpi.x", Score: 0.92, Timestamp: now}},
		CreatedAt:  now,
	}

	fake := &fakeCorrEngine{res: corr}
	log := logger.New("error")
	rcaEngine := NewRCAEngine(nil, nil, log, config.EngineConfig{}, fake)

	tr := TimeRange{Start: now.Add(-5 * time.Minute), End: now}
	r, err := rcaEngine.ComputeRCAByTimeRange(context.Background(), tr)
	require.NoError(t, err)
	require.NotNil(t, r)

	// ImpactSummary should include partial value and anomaly hint
	require.Contains(t, r.Impact.ImpactSummary, "partial=")
	require.Contains(t, r.Impact.ImpactSummary, "anomalies=HIGH")
}
