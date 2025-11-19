package rca

import (
	"fmt"
	"math"
	"sort"
)

// ScoringConfig defines weights and thresholds for scoring candidate causes.
type ScoringConfig struct {
	// WeightRing is the weight of temporal proximity to incident peak (0..1).
	// Default: 0.2
	WeightRing float64

	// WeightGraphDirection is the weight of graph direction (upstream > same > downstream) (0..1).
	// Default: 0.25
	WeightGraphDirection float64

	// WeightDistance is the weight of graph distance to impact service (0..1).
	// Default: 0.15
	WeightDistance float64

	// WeightSeverity is the weight of anomaly severity (0..1).
	// Default: 0.2
	WeightSeverity float64

	// WeightAnomalyScore is the weight of isolation forest anomaly score (0..1).
	// Default: 0.1
	WeightAnomalyScore float64

	// WeightTransactionCount is the weight of breadth of impact (0..1).
	// Default: 0.1
	WeightTransactionCount float64

	// MaxCandidatesToReturn: if > 0, limits the returned ranked candidates to this count.
	// Default: 0 (no limit).
	MaxCandidatesToReturn int
}

// DefaultScoringConfig returns sensible defaults for scoring.
// Weights sum to 1.0 for a normalized score range of [0, 1].
func DefaultScoringConfig() ScoringConfig {
	return ScoringConfig{
		WeightRing:             0.2,
		WeightGraphDirection:   0.25,
		WeightDistance:         0.15,
		WeightSeverity:         0.2,
		WeightAnomalyScore:     0.1,
		WeightTransactionCount: 0.1,
		MaxCandidatesToReturn:  0,
	}
}

// ScoreCandidate computes a suspicion score for an AnomalyGroup
// based on temporal, topological, and intensity factors.
// Returns a CandidateCause with score and reasons.
func ScoreCandidate(group *AnomalyGroup, incident *IncidentContext, cfg ScoringConfig) *CandidateCause {
	cc := NewCandidateCause(group)

	// Compute individual dimension scores
	ringScore := computeRingScore(group.Ring)
	directionScore := computeDirectionScore(group.GraphDirection)
	distanceScore := computeDistanceScore(group.MinGraphDistance, group.MaxGraphDistance)
	severityScore := float64(group.MaxSeverity) // Severity is already 0..1
	anomalyScoreContribution := group.MaxScore  // MaxScore is already 0..1
	transactionCountScore := computeTransactionCountScore(group.DistinctTxnCount)

	// Normalize weights (ensure they sum to 1.0 for consistent scaling)
	totalWeight := cfg.WeightRing + cfg.WeightGraphDirection + cfg.WeightDistance +
		cfg.WeightSeverity + cfg.WeightAnomalyScore + cfg.WeightTransactionCount

	if totalWeight <= 0 {
		totalWeight = 1.0 // Fallback to equal weighting
	}

	// Compute weighted overall score
	score := (ringScore * cfg.WeightRing / totalWeight) +
		(directionScore * cfg.WeightGraphDirection / totalWeight) +
		(distanceScore * cfg.WeightDistance / totalWeight) +
		(severityScore * cfg.WeightSeverity / totalWeight) +
		(anomalyScoreContribution * cfg.WeightAnomalyScore / totalWeight) +
		(transactionCountScore * cfg.WeightTransactionCount / totalWeight)

	// Clamp score to [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	cc.Score = score
	cc.DetailedScore = &DetailedScore{
		RingScore:                ringScore,
		DirectionScore:           directionScore,
		DistanceScore:            distanceScore,
		SeverityScore:            severityScore,
		AnomalyScoreContribution: anomalyScoreContribution,
		TransactionCountScore:    transactionCountScore,
	}

	// Generate human-readable reasons
	cc.Reasons = generateReasons(group, ringScore, directionScore, distanceScore, severityScore, anomalyScoreContribution, transactionCountScore)

	return cc
}

// computeRingScore assigns a score based on temporal proximity to peak.
// Immediate ring (R1) gets highest score, degrading to R4 and out-of-scope.
func computeRingScore(ring TimeRing) float64 {
	switch ring {
	case RingImmediate:
		return 1.0 // Closest to peak = highest suspicion
	case RingShort:
		return 0.75
	case RingMedium:
		return 0.5
	case RingLong:
		return 0.25
	default: // RingOutOfScope
		return 0.0
	}
}

// computeDirectionScore assigns a score based on graph direction.
// Upstream (direct dependency) is most suspicious, followed by same-service,
// then downstream, then unknown.
func computeDirectionScore(direction GraphDirection) float64 {
	switch direction {
	case DirectionUpstream:
		return 1.0 // Direct dependency most suspicious
	case DirectionSame:
		return 0.8 // Same service highly suspicious
	case DirectionDownstream:
		return 0.3 // Downstream less suspicious
	default: // DirectionUnknown
		return 0.5 // Unknown gets middle score
	}
}

// computeDistanceScore assigns a score based on hop count to impact service.
// Smaller distance = higher score (closer = more suspicious).
// Formula: 1.0 / (1.0 + distance), with cap at 0.0 for unknown distances.
func computeDistanceScore(minDist, maxDist int) float64 {
	if minDist < 0 {
		// Unknown distance
		return 0.3 // Default to low-medium suspicion
	}

	// Prefer minimum distance for scoring (closest = most suspicious)
	if minDist == 0 {
		return 1.0 // Same service
	}

	// Use inverse decay: 1/(1+distance)
	score := 1.0 / (1.0 + float64(minDist))
	return score
}

// computeTransactionCountScore assigns a score based on breadth of impact.
// More affected transactions = higher score (more widespread = more suspicious).
// Uses logarithmic scaling to avoid over-weighting very high counts.
func computeTransactionCountScore(txnCount int) float64 {
	if txnCount == 0 {
		return 0.0
	}

	// Logarithmic scaling: log(txnCount + 1) normalized to [0, 1]
	// log(1001) ≈ 6.9, so we use that as our normalization ceiling
	logCount := math.Log(float64(txnCount) + 1.0)
	maxLog := math.Log(1001.0) // Reasonable ceiling for transaction counts

	score := logCount / maxLog
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// generateReasons creates human-readable reasons for the computed score.
func generateReasons(group *AnomalyGroup, ringScore, directionScore, distanceScore, severityScore, anomalyScore, txnScore float64) []string {
	var reasons []string

	// Ring-based reasons
	if ringScore >= 0.75 {
		reasons = append(reasons, "ring_immediate_or_short")
	} else if ringScore >= 0.5 {
		reasons = append(reasons, "ring_medium")
	} else if ringScore > 0 {
		reasons = append(reasons, "ring_long")
	}

	// Direction-based reasons
	if directionScore >= 0.8 {
		if group.GraphDirection == DirectionUpstream {
			reasons = append(reasons, "upstream_dependency")
		} else {
			reasons = append(reasons, "same_service_impact")
		}
	} else if directionScore >= 0.5 {
		reasons = append(reasons, "unknown_graph_direction")
	} else if directionScore > 0 {
		reasons = append(reasons, "downstream_service")
	}

	// Distance-based reasons
	if group.MinGraphDistance == 0 {
		reasons = append(reasons, "direct_impact_target")
	} else if group.MinGraphDistance == 1 {
		reasons = append(reasons, "direct_neighbor_one_hop")
	} else if distanceScore >= 0.3 {
		reasons = append(reasons, "within_proximity_chain")
	}

	// Severity-based reasons
	if severityScore >= 0.75 {
		reasons = append(reasons, "high_severity_events")
	} else if severityScore >= 0.5 {
		reasons = append(reasons, "medium_severity_events")
	}

	// Anomaly score reasons
	if anomalyScore >= 0.8 {
		reasons = append(reasons, "high_isolation_forest_score")
	} else if anomalyScore >= 0.5 {
		reasons = append(reasons, "medium_isolation_forest_score")
	}

	// Transaction count reasons
	if txnScore >= 0.7 && group.DistinctTxnCount > 100 {
		reasons = append(reasons, "wide_transaction_impact")
	} else if txnScore >= 0.4 && group.DistinctTxnCount > 10 {
		reasons = append(reasons, "multiple_transactions_affected")
	}

	// Event count reasons
	if group.EventCount >= 10 {
		reasons = append(reasons, "high_event_volume")
	}

	return reasons
}

// RankCandidateCauses scores and ranks a slice of AnomalyGroups.
// Returns a slice of CandidateCauses sorted by score descending (highest first).
// Each cause is assigned a rank (1 = most suspicious).
func RankCandidateCauses(groups []*AnomalyGroup, incident *IncidentContext, cfg ScoringConfig) []*CandidateCause {
	if len(groups) == 0 {
		return []*CandidateCause{}
	}

	// Score each group
	var candidates []*CandidateCause
	for _, group := range groups {
		if group != nil {
			candidate := ScoreCandidate(group, incident, cfg)
			candidates = append(candidates, candidate)
		}
	}

	// Sort by score descending, then by deterministic tiebreaker (service name, component)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score // Higher score first
		}

		// Tiebreaker: by service name
		if candidates[i].Group.Service != candidates[j].Group.Service {
			return candidates[i].Group.Service < candidates[j].Group.Service
		}

		// Second tiebreaker: by component
		return candidates[i].Group.Component < candidates[j].Group.Component
	})

	// Assign ranks and optionally limit results
	for idx, candidate := range candidates {
		candidate.Rank = idx + 1
	}

	// Apply max candidates limit if configured
	if cfg.MaxCandidatesToReturn > 0 && len(candidates) > cfg.MaxCandidatesToReturn {
		candidates = candidates[:cfg.MaxCandidatesToReturn]
	}

	return candidates
}

// ApplyKPISentimentBias adjusts candidate cause scores based on KPI metadata and sentiment.
// This implements lightweight, deterministic scoring adjustments that respect KPI business context
// without breaking OTEL-only scenarios.
//
// Behavior:
//   - If incident has KPI metadata and KPI.Sentiment == "NEGATIVE" and KPI value increased:
//     Add small positive bias to candidates (e.g., +0.05)
//   - If KPI.Sentiment == "POSITIVE" and KPI value increased:
//     Add small negative bias (e.g., -0.05, reducing candidate scores)
//   - NEUTRAL sentiment: minimal effect
//
// Bias magnitude and application strategy are controlled by config (default 0.05).
func ApplyKPISentimentBias(
	candidates []*CandidateCause,
	incident *IncidentContext,
	biasMagnitude float64,
	diagnostics *RCADiagnostics,
) {
	if len(candidates) == 0 || incident == nil {
		return
	}

	// Check if this incident has KPI metadata
	if incident.KPIMetadata == nil {
		return // No KPI context, skip bias application
	}

	kpiMeta := incident.KPIMetadata
	if !kpiMeta.ImpactIsKPI {
		return // Impact is not KPI-based, skip
	}

	// Determine bias direction based on sentiment
	var bias float64
	sentiment := kpiMeta.KPISentiment

	switch sentiment {
	case "NEGATIVE":
		// Increase in NEGATIVE KPI is bad; increase suspicion of technical causes
		bias = biasMagnitude
		if diagnostics != nil {
			diagnostics.AddReducedAccuracyReason(
				fmt.Sprintf("KPI '%s' (NEGATIVE sentiment) applied +%.2f bias to candidate scores", kpiMeta.KPIName, bias))
		}

	case "POSITIVE":
		// Increase in POSITIVE KPI is good; decrease suspicion where appropriate
		bias = -biasMagnitude
		if diagnostics != nil {
			diagnostics.AddReducedAccuracyReason(
				fmt.Sprintf("KPI '%s' (POSITIVE sentiment) applied %.2f bias to candidate scores", kpiMeta.KPIName, bias))
		}

	case "NEUTRAL":
		// Neutral sentiment: no or minimal effect
		bias = 0.0

	default:
		// Unknown sentiment: no effect
		return
	}

	if bias == 0.0 {
		return // No bias to apply
	}

	// Apply bias to each candidate, clamping to [0, 1]
	for _, candidate := range candidates {
		newScore := candidate.Score + bias
		if newScore > 1.0 {
			newScore = 1.0
		}
		if newScore < 0.0 {
			newScore = 0.0
		}
		candidate.Score = newScore
	}

	// Re-rank candidates by updated score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	for i := range candidates {
		candidates[i].Rank = i + 1
	}
}
