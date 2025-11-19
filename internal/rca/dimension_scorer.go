package rca

import (
	"fmt"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// DimensionAlignmentScorer computes alignment scores for candidates based on extra dimensions.
type DimensionAlignmentScorer struct {
	logger logger.Logger
}

// NewDimensionAlignmentScorer creates a new scorer.
func NewDimensionAlignmentScorer(logger logger.Logger) *DimensionAlignmentScorer {
	return &DimensionAlignmentScorer{
		logger: logger,
	}
}

// ComputeDimensionAlignmentScore evaluates how well a candidate aligns with the impact service
// on user-configured extra dimensions.
// Returns:
// - alignmentScore: float64 in range [-dimensionConfig.AlignmentPenalty, dimensionConfig.AlignmentBonus]
// - alignments: detailed breakdown of each dimension
// - notes: any issues encountered
func (das *DimensionAlignmentScorer) ComputeDimensionAlignmentScore(
	candidateGroup *AnomalyGroup,
	impactServiceDimensions map[string]string, // dimension -> value from impact service
	dimensionConfig RCADimensionConfig,
	diagnostics *RCADiagnostics,
) (float64, []DimensionAlignment, []string) {
	var alignments []DimensionAlignment
	var notes []string

	if len(dimensionConfig.ExtraDimensions) == 0 {
		// No extra dimensions configured, no alignment score
		return 0.0, alignments, notes
	}

	totalAlignmentScore := 0.0
	alignedCount := 0
	totalWeight := 0.0

	for _, dimKey := range dimensionConfig.ExtraDimensions {
		weight := dimensionConfig.GetDimensionWeight(dimKey)
		totalWeight += weight

		impactValue, impactHas := impactServiceDimensions[dimKey]
		candidateValue, candidateHas := candidateGroup.ExtraDimensionValues[dimKey]

		alignment := DimensionAlignment{
			DimensionKey:       dimKey,
			ImpactServiceValue: impactValue,
			CandidateValue:     candidateValue,
			Weight:             weight,
			IsAligned:          false,
		}

		if !impactHas || !candidateHas {
			// Dimension missing in one or both services; treat as unaligned but not penalizing
			notes = append(notes, fmt.Sprintf("Dimension '%s' missing from impact service or candidate (impact=%v, candidate=%v)", dimKey, impactHas, candidateHas))
			if diagnostics != nil {
				diagnostics.AddReducedAccuracyReason(fmt.Sprintf("Dimension '%s' not present in both services for alignment check", dimKey))
			}
		} else if impactValue == candidateValue {
			// Aligned
			alignment.IsAligned = true
			alignedCount++
			totalAlignmentScore += weight * dimensionConfig.AlignmentBonus
		} else {
			// Misaligned
			totalAlignmentScore -= weight * dimensionConfig.AlignmentPenalty
		}

		alignments = append(alignments, alignment)
	}

	// Normalize score
	var normalizedScore float64
	if totalWeight > 0 {
		normalizedScore = totalAlignmentScore / totalWeight
	}

	// Log alignment summary
	das.logger.Debug("Dimension alignment scored",
		"candidate_service", candidateGroup.Service,
		"aligned_count", alignedCount,
		"total_dimensions", len(dimensionConfig.ExtraDimensions),
		"alignment_score", normalizedScore)

	return normalizedScore, alignments, notes
}

// RecordDimensionValuesFromEvent extracts dimension values from an anomaly event
// and aggregates them into the group (e.g., majority vote on dimension values).
func (das *DimensionAlignmentScorer) RecordDimensionValuesFromEvent(
	group *AnomalyGroup,
	event *EnrichedAnomalyEvent,
	extraDimensions []string,
) {
	if group == nil || event == nil || len(extraDimensions) == 0 {
		return
	}

	// Extract dimension values from event tags
	for _, dim := range extraDimensions {
		if val, ok := event.AnomalyEvent.Tags[dim]; ok && val != "" {
			// Simple strategy: if not yet set, set it; otherwise prefer consistency
			if existingVal, exists := group.ExtraDimensionValues[dim]; !exists {
				group.ExtraDimensionValues[dim] = val
			} else if existingVal != val {
				// Dimension value varies within the group; log a warning
				das.logger.Debug("Dimension value varies within group",
					"service", group.Service,
					"dimension", dim,
					"previous_value", existingVal,
					"new_value", val)
				// For simplicity, keep the first value; in production, might use majority vote
			}
		}
	}
}

// ExtractImpactServiceDimensions extracts dimension values from enriched anomaly events
// detected in the impact service itself. Aggregates by majority vote or first-seen.
func (das *DimensionAlignmentScorer) ExtractImpactServiceDimensions(
	impactServiceGroups []*AnomalyGroup,
	extraDimensions []string,
) map[string]string {
	result := make(map[string]string)

	if len(impactServiceGroups) == 0 || len(extraDimensions) == 0 {
		return result
	}

	// Simple aggregation: use values from the first group that has them
	for _, group := range impactServiceGroups {
		if len(result) == len(extraDimensions) {
			break // All dimensions found
		}

		for _, dim := range extraDimensions {
			if _, alreadySet := result[dim]; !alreadySet {
				if val, ok := group.ExtraDimensionValues[dim]; ok && val != "" {
					result[dim] = val
				}
			}
		}
	}

	return result
}
