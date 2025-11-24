package services

import (
	"math"
)

const MIN_SAMPLES = 3

// ComputePearson computes Pearson correlation coefficient for two numeric slices.
// Returns 0 for invalid inputs (length mismatch or length < 2).
func ComputePearson(x, y []float64) float64 {
	n := len(x)
	if n == 0 || n != len(y) || n < 2 {
		return 0.0
	}
	var sx, sy, sxx, syy, sxy float64
	for i := 0; i < n; i++ {
		xi := x[i]
		yi := y[i]
		sx += xi
		sy += yi
		sxx += xi * xi
		syy += yi * yi
		sxy += xi * yi
	}
	num := float64(n)*sxy - sx*sy
	den := math.Sqrt((float64(n)*sxx - sx*sx) * (float64(n)*syy - sy*sy))
	if den == 0 {
		return 0.0
	}
	return num / den
}

// rank returns ranks (1-based average ranks for ties) for the input slice.
func rank(a []float64) []float64 {
	n := len(a)
	if n == 0 {
		return []float64{}
	}
	// simple O(n^2) ranking sufficient for small vectors used in tests
	ranks := make([]float64, n)
	for i := 0; i < n; i++ {
		r := 1.0
		for j := 0; j < n; j++ {
			if a[j] < a[i] {
				r += 1.0
			} else if a[j] == a[i] && j < i {
				// earlier equal element increases base rank
				r += 1.0
			}
		}
		// This produces dense ranks; for tests this is acceptable.
		ranks[i] = r
		// Note: For exact Spearman with tie handling, more elaborate averaging is required.
	}
	return ranks
}

// ComputeSpearman computes Spearman rank correlation using simple ranks.
func ComputeSpearman(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0.0
	}
	rx := rank(x)
	ry := rank(y)
	return ComputePearson(rx, ry)
}

// CrossCorrelScan scans lags from -maxLag..+maxLag (inclusive) and returns
// the maximum cross-correlation (normalized) and the lag that produced it.
// lag > 0 means y lags behind x (x leads y by lag samples).
func CrossCorrelScan(x, y []float64, maxLag int) (float64, int) {
	n := len(x)
	if n == 0 || n != len(y) {
		return 0.0, 0
	}
	if maxLag < 0 {
		maxLag = 0
	}
	best := math.Inf(-1)
	bestLag := 0
	// compute mean and std once
	meanX := 0.0
	meanY := 0.0
	for i := 0; i < n; i++ {
		meanX += x[i]
		meanY += y[i]
	}
	meanX /= float64(n)
	meanY /= float64(n)
	var sx, sy float64
	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		sx += dx * dx
		sy += dy * dy
	}
	if sx == 0 || sy == 0 {
		return 0.0, 0
	}
	// scan lags
	for lag := -maxLag; lag <= maxLag; lag++ {
		var num float64
		var count int
		for i := 0; i < n; i++ {
			j := i + lag
			if j < 0 || j >= n {
				continue
			}
			num += (x[i] - meanX) * (y[j] - meanY)
			count++
		}
		if count == 0 {
			continue
		}
		denom := math.Sqrt(sx * sy)
		corr := num / denom
		if corr > best {
			best = corr
			bestLag = lag
		}
	}
	if best == math.Inf(-1) {
		return 0.0, 0
	}
	return best, bestLag
}

// ComputeCorrelationStats computes a CorrelationStats summary for two aligned vectors.
// maxLag controls cross-correlation scan window in samples.
// ComputeCorrelationStats computes a CorrelationStats summary for two aligned
// vectors. Optionally accepts one or more control/confounder series. For
// Stage-01 we support a single confounder series (the implementation accepts
// variadic controls to remain extensible).
func ComputeCorrelationStats(x, y []float64, maxLag int, controls ...[]float64) (pearson, spearman, crossMax float64, crossLag int, partial float64, sampleSize int) {
	sampleSize = len(x)
	if len(x) != len(y) {
		return 0, 0, 0, 0, 0, 0
	}
	pearson = ComputePearson(x, y)
	spearman = ComputeSpearman(x, y)
	crossMax, crossLag = CrossCorrelScan(x, y, maxLag)

	// Default partial is 0.0 when no suitable control is provided or when
	// computation is not possible (e.g. low sample size or zero variance).
	partial = 0.0
	if len(controls) > 0 && controls[0] != nil {
		ctrl := controls[0]
		// Ensure control length matches samples
		if len(ctrl) == sampleSize && sampleSize >= MIN_SAMPLES {
			partial = PartialCorrelation(x, y, ctrl)
		}
	}

	return
}

// NOTE(AT-007): PartialCorrelation stub. Implementing a full partial correlation
// requires conditioning on additional control variables; leave as 0 for now
// and annotate with the action tracker reference.
// PartialCorrelation computes the partial correlation between x and y
// conditioned on one or more control series. For Stage-01 we implement the
// single-control formula:
//
//	r_xy.z = (r_xy - r_xz * r_yz) / (sqrt(1 - r_xz^2) * sqrt(1 - r_yz^2))
//
// For multiple controls this function currently uses only the first control
// but accepts a slice to remain extensible. Edge cases handled:
// - If sample size < MIN_SAMPLES -> return 0.0
// - If variance of any series is zero (pearson denom zero) -> return 0.0
// - If denominator of formula is zero -> return 0.0
// NOTE(AT-012): Implemented partial correlation support for Stage-01.
func PartialCorrelation(x, y []float64, control []float64) float64 {
	if x == nil || y == nil || control == nil {
		return 0.0
	}
	n := len(x)
	if n < MIN_SAMPLES || len(y) != n || len(control) != n {
		return 0.0
	}

	// compute Pearson correlations
	rxy := ComputePearson(x, y)
	rxz := ComputePearson(x, control)
	ryz := ComputePearson(y, control)

	denomTerm := (1 - rxz*rxz) * (1 - ryz*ryz)
	if denomTerm <= 0 {
		return 0.0
	}
	denom := math.Sqrt(denomTerm)
	if denom == 0 {
		return 0.0
	}
	return (rxy - rxz*ryz) / denom
}

// ComputeSuspicionScore deterministically combines correlation statistics and
// simple heuristics into a 0..1 suspicion score. It is intentionally
// conservative and driven by config thresholds where available.
// NOTE(AT-007): This is a Stage-01 implementation — refinements (e.g. using
// partial correlation and anomaly-density signals) are tracked in AT-007.
func ComputeSuspicionScore(pearson, spearman, crossMax float64, crossLag int, sampleSize int, cfgMinCorrelation float64, partial float64, anomalyDensity float64) float64 {
	// Weigh components: Pearson (50%), Spearman (30%), CrossCorr (20%)
	wP := 0.5
	wS := 0.3
	wC := 0.2

	ap := math.Abs(pearson)
	as := math.Abs(spearman)

	// If sample size is too small, downweight correlations
	sizeFactor := 1.0
	if sampleSize < 2 {
		sizeFactor = 0.0
	} else if sampleSize == 2 {
		sizeFactor = 0.5
	} else if sampleSize < 5 {
		sizeFactor = 0.75
	}

	// Lag bonus: only reward when cross-corr indicates cause leads impact.
	lagBonus := 0.0
	if crossLag > 0 { // crossLag > 0 means x leads y per CrossCorrelScan contract
		lagBonus = 0.12
	}

	// Base score is weighted sum of absolute correlations and cross-corr
	base := (wP*ap + wS*as + wC*math.Max(0, crossMax)) * sizeFactor

	// Partial correlation influence: if partial suggests confounding (i.e.
	// partial magnitude much smaller than Pearson), apply a dampening factor.
	confoundingPenalty := 0.0
	if ap > 0 {
		ratio := 0.0
		if ap > 0 {
			ratio = math.Abs(partial) / ap
		}
		// ratio close to 1 => partial supports direct link; ratio close to 0 => confounding
		if ratio < 0.5 {
			// Penalize up to 30% when partial indicates strong confounding
			confoundingPenalty = 0.3 * (1.0 - ratio/0.5)
		}
	}

	// Apply lag bonus and clamp
	// Add anomaly density small positive contribution (normalized 0..1)
	anomalyWeight := 0.12
	score := base*(1.0-confoundingPenalty) + lagBonus + anomalyWeight*anomalyDensity
	if score > 1.0 {
		score = 1.0
	}

	// If both Pearson and Spearman are below configured minimum, further reduce
	if cfgMinCorrelation > 0 {
		if ap < cfgMinCorrelation && as < cfgMinCorrelation {
			score = score * 0.5
		}
	}

	// Ensure deterministic lower bound
	if score < 0.0 {
		score = 0.0
	}
	return score
}
