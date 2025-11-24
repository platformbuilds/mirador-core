package services

import (
	"math"
	"testing"
)

func TestComputePearsonPerfectPositive(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}
	v := ComputePearson(x, y)
	if math.Abs(v-1.0) > 1e-9 {
		t.Fatalf("expected Pearson ~1.0 got %v", v)
	}
}

func TestComputePearsonNegative(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{10, 8, 6, 4, 2}
	v := ComputePearson(x, y)
	if math.Abs(v+1.0) > 1e-9 {
		t.Fatalf("expected Pearson ~-1.0 got %v", v)
	}
}

func TestComputeSpearmanPerfect(t *testing.T) {
	x := []float64{10, 20, 30, 40, 50}
	y := []float64{5, 4, 3, 2, 1}
	v := ComputeSpearman(x, y)
	if math.Abs(v+1.0) > 1e-9 {
		t.Fatalf("expected Spearman ~-1.0 got %v", v)
	}
}

func TestCrossCorrelScan(t *testing.T) {
	x := []float64{0, 1, 0, 0, 0}
	y := []float64{0, 0, 1, 0, 0}
	// y leads x by 1 in this construction (or lag=-1 depending on convention)
	max, lag := CrossCorrelScan(x, y, 2)
	if math.Abs(max) < 1e-9 {
		t.Fatalf("expected non-zero cross-correlation, got %v", max)
	}
	_ = lag
}

func TestPartialCorrelation_SimpleConfounder(t *testing.T) {
	// Construct series where X and Y are both driven by Z (confounder).
	// Naive Pearson between X and Y will be high; partial conditioned on Z
	// should be near 0 (or zero when denom degenerates).
	z := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	x := []float64{1.1, 1.9, 3.05, 4.2, 4.95, 6.1, 7.0, 8.05}     // z + tiny noise
	y := []float64{2.05, 4.1, 6.0, 8.15, 10.0, 12.05, 14.1, 16.0} // 2*z + tiny noise

	p := ComputePearson(x, y)
	if p < 0.8 {
		t.Fatalf("expected high naive Pearson due to confounder, got %v", p)
	}

	// Use our PartialCorrelation helper (single control)
	pc := PartialCorrelation(x, y, z)
	if pc >= 0.3 {
		t.Fatalf("expected partial correlation to be small (<0.3) when conditioning on confounder, got %v", pc)
	}
}

func TestComputeSuspicionScore_WithAnomalyAndPartial(t *testing.T) {
	pearson := 0.9
	spearman := 0.88
	crossMax := 0.8
	crossLag := 1
	sampleSize := 6
	cfgMinCorrelation := 0.2

	// Case A: no anomaly, partial supports direct link
	partialA := 0.85
	anomalyA := 0.0
	scoreA := ComputeSuspicionScore(pearson, spearman, crossMax, crossLag, sampleSize, cfgMinCorrelation, partialA, anomalyA)

	// Case B: same stats but high anomaly density and partial indicates confounding
	partialB := 0.1
	anomalyB := 0.8
	scoreB := ComputeSuspicionScore(pearson, spearman, crossMax, crossLag, sampleSize, cfgMinCorrelation, partialB, anomalyB)

	// When partial indicates strong confounding and anomaly density is high,
	// the confounding penalty may reduce the overall suspicion score. We
	// therefore assert that the partial-supporting case scores at least as
	// high as the confounded case.
	if scoreA < scoreB {
		t.Fatalf("expected partial-supporting score >= confounded score; scoreA=%v scoreB=%v", scoreA, scoreB)
	}
}
