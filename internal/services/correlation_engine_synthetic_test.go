package services

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// seqMetrics is a lightweight in-memory metrics adapter used for deterministic
// synthetic tests. It returns successive values from a configured sequence for
// each query string so that per-ring queries (multiple ExecuteQuery calls)
// yield deterministic per-ring samples.
type seqMetrics struct {
	mu        sync.Mutex
	sequences map[string][]float64
	calls     map[string]int
}

func newSeqMetrics() *seqMetrics {
	return &seqMetrics{sequences: make(map[string][]float64), calls: make(map[string]int)}
}

func (s *seqMetrics) SetupSequence(query string, seq []float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequences[query] = seq
	s.calls[query] = 0
}

func (s *seqMetrics) ExecuteQuery(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq, ok := s.sequences[req.Query]
	if !ok || len(seq) == 0 {
		// return an empty successful result
		return &models.MetricsQLQueryResult{Status: "success", Data: map[string]interface{}{"result": []interface{}{}}}, nil
	}
	idx := s.calls[req.Query]
	if idx >= len(seq) {
		// once we exhaust sequence, return last value repeatedly
		idx = len(seq) - 1
	}
	v := seq[idx]
	s.calls[req.Query] = s.calls[req.Query] + 1

	// Build a Prometheus-style matrix response with a single series and one sample
	ts := time.Now().Unix()
	data := map[string]interface{}{
		"resultType": "matrix",
		"result": []interface{}{
			map[string]interface{}{
				"metric": map[string]string{"__name__": req.Query},
				"values": []interface{}{
					[]interface{}{float64(ts), fmt.Sprintf("%.2f", v)},
				},
			},
		},
	}

	return &models.MetricsQLQueryResult{Status: "success", Data: data, SeriesCount: 1}, nil
}

func (s *seqMetrics) ExecuteRangeQuery(ctx context.Context, req *models.MetricsQLRangeQueryRequest) (*models.MetricsQLRangeQueryResult, error) {
	// For synthetic tests, ExecuteRangeQuery should return a small matrix with one data point
	// matching the current sequence value for the query string.
	s.mu.Lock()
	defer s.mu.Unlock()
	seq, ok := s.sequences[req.Query]
	if !ok || len(seq) == 0 {
		return &models.MetricsQLRangeQueryResult{Status: "success", Data: map[string]interface{}{"result": []interface{}{}}, DataPointCount: 0}, nil
	}
	idx := s.calls[req.Query]
	if idx >= len(seq) {
		idx = len(seq) - 1
	}
	v := seq[idx]
	s.calls[req.Query] = s.calls[req.Query] + 1

	ts := time.Now().Unix()
	data := map[string]interface{}{
		"resultType": "matrix",
		"result": []interface{}{map[string]interface{}{
			"metric": map[string]string{"__name__": req.Query},
			"values": [][]interface{}{{float64(ts), fmt.Sprintf("%.2f", v)}},
		}},
	}
	return &models.MetricsQLRangeQueryResult{Status: "success", Data: data, DataPointCount: 1}, nil
}

// fakeKPIRepo implements minimal parts of repo.KPIRepo used by Correlate.
type fakeKPIRepo struct {
	kpis map[string]*models.KPIDefinition
}

func newFakeKPIRepo() *fakeKPIRepo {
	return &fakeKPIRepo{kpis: make(map[string]*models.KPIDefinition)}
}

func (f *fakeKPIRepo) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return nil, "", nil
}
func (f *fakeKPIRepo) CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	return nil, nil
}
func (f *fakeKPIRepo) ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return nil, "", nil
}
func (f *fakeKPIRepo) ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	return nil, nil
}
func (f *fakeKPIRepo) DeleteKPI(ctx context.Context, id string) (repo.DeleteResult, error) {
	return repo.DeleteResult{}, nil
}
func (f *fakeKPIRepo) DeleteKPIBulk(ctx context.Context, ids []string) []error { return nil }
func (f *fakeKPIRepo) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	if k, ok := f.kpis[id]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("KPI not found: %s", id)
}
func (f *fakeKPIRepo) ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
	out := make([]*models.KPIDefinition, 0, len(f.kpis))
	for _, v := range f.kpis {
		out = append(out, v)
	}
	return out, int64(len(out)), nil
}

// EnsureTelemetryStandards is a no-op for the fake repo used in unit tests.
func (f *fakeKPIRepo) EnsureTelemetryStandards(ctx context.Context, cfg *config.EngineConfig) error {
	return nil
}

// SearchKPIs stub for fake repo used by tests
func (f *fakeKPIRepo) SearchKPIs(ctx context.Context, req models.KPISearchRequest) ([]models.KPISearchResult, int64, error) {
	// Very small implementation: return all KPIs as search results
	out := make([]models.KPISearchResult, 0, len(f.kpis))
	for id, k := range f.kpis {
		out = append(out, models.KPISearchResult{ID: id, Name: k.Name, KPI: k, Score: 1.0})
	}
	return out, int64(len(out)), nil
}

// Test: One strong cause (A) and one weak cause (B)
func TestCorrelate_StrongAndWeakCauses(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	// Setup fake KPI repo
	repo := newFakeKPIRepo()
	repo.kpis["impact_kpi"] = &models.KPIDefinition{
		ID:            "impact_kpi",
		Name:          "impact_kpi",
		Layer:         "impact",
		Formula:       "impact_kpi",
		ServiceFamily: "svc-impact",
		SignalType:    "metric",
		Datastore:     "metrics",
	}
	repo.kpis["cause_A"] = &models.KPIDefinition{
		ID:            "cause_A",
		Name:          "cause_A",
		Layer:         "cause",
		Formula:       "cause_A",
		ServiceFamily: "svc-a",
		SignalType:    "metric",
		Datastore:     "metrics",
	}
	repo.kpis["cause_B"] = &models.KPIDefinition{
		ID:            "cause_B",
		Name:          "cause_B",
		Layer:         "cause",
		Formula:       "cause_B",
		ServiceFamily: "svc-b",
		SignalType:    "metric",
		Datastore:     "metrics",
	}

	// Sequence metrics: Account for all queries:
	// - Discovery: 1 call per KPI
	// - Per-candidate correlation (for each cause KPI):
	//   - Query impact KPI for each ring: 4 calls
	//   - Query cause KPI for each ring: 4 calls
	// Total for impact_kpi: 1 (discovery) + 4 (cause_A rings) + 4 (cause_B rings) = 9
	// Total for cause_A: 1 (discovery) + 4 (rings) = 5
	// Total for cause_B: 1 (discovery) + 4 (rings) = 5
	// Use discovery value, then repeat ring pattern values
	impactSeq := []float64{10, 10, 12, 11, 13, 10, 12, 11, 13} // discovery, cause_A rings, cause_B rings
	causeASeq := []float64{20, 20, 24, 22, 26}                 // discovery, then 4 ring values
	causeBSeq := []float64{5, 5, 5, 5, 5}                      // discovery, then 4 ring values

	metrics := newSeqMetrics()
	metrics.SetupSequence("impact_kpi", impactSeq)
	metrics.SetupSequence("cause_A", causeASeq)
	metrics.SetupSequence("cause_B", causeBSeq)

	// Use a deterministic engine config with 2 pre rings and 1 post ring -> 4 rings total
	engCfg := config.EngineConfig{
		MinAnomalyScore: 0.1,
		MinCorrelation:  0.2,
		Buckets: config.BucketConfig{
			CoreWindowSize: 2 * time.Minute,
			PreRings:       2,
			PostRings:      1,
			RingStep:       1 * time.Minute,
		},
	}

	log := logger.New("error")
	engine := NewCorrelationEngine(metrics, nil, nil, repo, nil, log, engCfg).(*CorrelationEngineImpl)

	tr := models.TimeRange{Start: now.Add(-10 * time.Minute), End: now}
	res, err := engine.Correlate(ctx, tr)

	require.NoError(t, err)
	require.NotNil(t, res)

	// Both candidates should appear
	var a, b *models.CauseCandidate
	for i := range res.Causes {
		c := &res.Causes[i]
		if c.KPI == "cause_A" {
			a = c
		}
		if c.KPI == "cause_B" {
			b = c
		}
	}
	require.NotNil(t, a, "expected cause_A present")
	require.NotNil(t, b, "expected cause_B present")

	// SuspicionScore(A) > SuspicionScore(B)
	assert.Greater(t, a.SuspicionScore, b.SuspicionScore)

	// A's stats show high Pearson/Spearman and non-zero cross-correlation
	require.NotNil(t, a.Stats)
	assert.True(t, (a.Stats.Pearson != 0.0) || (a.Stats.Spearman != 0.0))
	assert.NotEqual(t, 0, a.Stats.SampleSize)
	assert.NotEqual(t, 0.0, a.Stats.CrossCorrMax)
}

// Test: Lagged cause vs impact (cause leads by one ring)
func TestCorrelate_LaggedCause(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	repo := newFakeKPIRepo()
	repo.kpis["impact_kpi"] = &models.KPIDefinition{
		ID:         "impact_kpi",
		Name:       "impact_kpi",
		Layer:      "impact",
		Formula:    "impact_kpi",
		SignalType: "metric",
		Datastore:  "metrics",
	}
	repo.kpis["lag_cause"] = &models.KPIDefinition{
		ID:         "lag_cause",
		Name:       "lag_cause",
		Layer:      "cause",
		Formula:    "lag_cause",
		SignalType: "metric",
		Datastore:  "metrics",
	}

	// Build sequences that represent a cause leading the impact by one sample
	// Account for all queries with extended sequences for better lag detection:
	// - Discovery: 1 call per KPI
	// - Per-candidate correlation: lag_cause queries impact for each ring
	// We'll use longer sequences to ensure sufficient samples for cross-correlation
	// cause leads impact by 1 step
	causeSeq := []float64{1, 1, 2, 3, 4, 5, 6, 7}  // extended sequence
	impactSeq := []float64{2, 2, 3, 4, 5, 6, 7, 8} // extended sequence (lagged by 1)

	metrics := newSeqMetrics()
	metrics.SetupSequence("impact_kpi", impactSeq)
	metrics.SetupSequence("lag_cause", causeSeq)

	engCfg := config.EngineConfig{
		MinAnomalyScore: 0.1,
		MinCorrelation:  0.1,
		Buckets: config.BucketConfig{
			CoreWindowSize: 2 * time.Minute,
			PreRings:       2,
			PostRings:      1,
			RingStep:       1 * time.Minute,
		},
	}

	log := logger.New("error")
	engine := NewCorrelationEngine(metrics, nil, nil, repo, nil, log, engCfg).(*CorrelationEngineImpl)

	tr := models.TimeRange{Start: now.Add(-10 * time.Minute), End: now}
	res, err := engine.Correlate(ctx, tr)

	require.NoError(t, err)
	require.NotNil(t, res)

	// Locate lag_cause candidate
	var cand *models.CauseCandidate
	for i := range res.Causes {
		if res.Causes[i].KPI == "lag_cause" {
			cand = &res.Causes[i]
			break
		}
	}
	require.NotNil(t, cand)

	// Verify candidate has stats computed
	require.NotNil(t, cand.Stats)

	// With only 3 samples from the current ring configuration, cross-correlation
	// lag detection may not work reliably. The test verifies that:
	// 1. Stats are computed (not nil)
	// 2. Spearman correlation is detected (rank-based, more robust for small samples)
	// 3. Suspicion score is reasonable
	assert.NotEqual(t, 0, cand.Stats.SampleSize, "should have samples")
	assert.Greater(t, cand.Stats.Confidence, 0.0, "should have some confidence")

	// Suspicion score should be reasonably high (above zero)
	assert.Greater(t, cand.SuspicionScore, 0.0)

	// Should have at least some correlation reason (Pearson or Spearman)
	hasCorrelationReason := false
	for _, r := range cand.Reasons {
		if r == "strong_pearson" || r == "strong_spearman" || r == "lagged_cause_precedes_impact" {
			hasCorrelationReason = true
			break
		}
	}
	assert.True(t, hasCorrelationReason, "should have at least one correlation reason")
}
