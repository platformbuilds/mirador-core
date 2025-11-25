package services

import (
	"math"
	"testing"
)

func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func TestComputePearsonAndSpearman(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10} // perfect linear (y = 2x)

	p := ComputePearson(x, y)
	if !approxEqual(p, 1.0, 1e-9) {
		t.Fatalf("expected pearson ~1.0, got %v", p)
	}

	s := ComputeSpearman(x, y)
	if !approxEqual(s, 1.0, 1e-9) {
		t.Fatalf("expected spearman ~1.0, got %v", s)
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

func TestCrossCorrelScanLagDetection(t *testing.T) {
	// x leads y by 1 sample: y[i] = x[i-1] for i>=1
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{0, 1, 2, 3, 4}

	max, lag := CrossCorrelScan(x, y, 3)
	if max <= 0 {
		t.Fatalf("expected positive cross-correlation, got %v", max)
	}
	// lag should be within the tested lag window
	if lag < -3 || lag > 3 {
		t.Fatalf("lag out of expected bounds, got %d", lag)
	}
}

func TestCrossCorrelScanNonzero(t *testing.T) {
	x := []float64{0, 1, 0, 0, 0}
	y := []float64{0, 0, 1, 0, 0}
	max, _ := CrossCorrelScan(x, y, 2)
	if math.Abs(max) < 1e-9 {
		t.Fatalf("expected non-zero cross-correlation, got %v", max)
	}
}

func TestPartialCorrelationBasic(t *testing.T) {
	// Construct series where x and y both correlate with control, but
	// direct correlation differs from partial.
	// control increases linearly, x = control + noise, y = 2*control + noise2
	control := []float64{1, 2, 3, 4, 5}
	x := []float64{1.1, 2.0, 2.9, 4.2, 5.1}
	y := []float64{2.0, 4.1, 6.2, 8.0, 10.1}

	// Pearson between x and y should be high
	rxy := ComputePearson(x, y)
	if rxy < 0.9 {
		t.Fatalf("expected high pearson between x and y, got %v", rxy)
	}

	// Partial correlation should be a finite value in [-1,1]
	p := PartialCorrelation(x, y, control)
	if math.IsNaN(p) || math.Abs(p) > 1.000001 {
		t.Fatalf("partial correlation out of bounds, got %v", p)
	}
}

func TestPartialCorrelation_SimpleConfounder(t *testing.T) {
	// Construct series where X and Y are both driven by Z (confounder).
	// Naive Pearson between X and Y will be high; partial conditioned on Z
	// should be small.
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

func TestComputeCorrelationStatsAndSuspicion(t *testing.T) {
	x := []float64{1, 2, 3, 4}
	y := []float64{2, 4, 6, 8}
	ctrl := []float64{1, 1, 1, 1} // zero variance control should be ignored

	pear, spear, crossMax, crossLag, partial, n := ComputeCorrelationStats(x, y, 2, ctrl)
	if n != 4 {
		t.Fatalf("expected sample size 4, got %d", n)
	}
	if !approxEqual(pear, 1.0, 1e-9) {
		t.Fatalf("expected pearson ~1.0, got %v", pear)
	}
	if !approxEqual(spear, 1.0, 1e-9) {
		t.Fatalf("expected spearman ~1.0, got %v", spear)
	}
	if crossMax <= 0 {
		t.Fatalf("expected positive crossMax, got %v", crossMax)
	}

	// Compute suspicion score, with a min-correlation threshold that should not lower the score
	score := ComputeSuspicionScore(pear, spear, crossMax, crossLag, n, 0.1, partial, 0.0)
	if score <= 0 || score > 1.0 {
		t.Fatalf("expected score in (0,1], got %v", score)
	}

	// If min correlation is higher than both correlations, score should be reduced
	score2 := ComputeSuspicionScore(0.05, 0.05, crossMax, crossLag, n, 0.5, partial, 0.0)
	if score2 >= score {
		t.Fatalf("expected score2 < score when below min correlation; got %v >= %v", score2, score)
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
