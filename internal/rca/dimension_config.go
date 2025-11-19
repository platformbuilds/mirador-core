package rca

import (
	"fmt"
	"strings"
)

// DimensionAlignment represents how well a candidate aligns with user-configured dimensions.
type DimensionAlignment struct {
	// DimensionKey is the name of the configured dimension (e.g., "env", "region", "namespace")
	DimensionKey string

	// ImpactServiceValue is the value of this dimension in the impact service
	ImpactServiceValue string

	// CandidateValue is the value of this dimension in the candidate service
	CandidateValue string

	// IsAligned is true if both values match
	IsAligned bool

	// Weight is the user-configured weight for this dimension (0..1)
	Weight float64
}

// RCADimensionConfig defines user-configurable dimensions for correlation and RCA.
// These dimensions influence how candidates are scored and chains are built.
type RCADimensionConfig struct {
	// ExtraDimensions is a list of additional label keys to consider
	// (e.g., ["env", "region", "cluster", "namespace"]).
	ExtraDimensions []string `json:"extraDimensions,omitempty"`

	// DimensionWeights maps dimension keys to their influence weight (0..1).
	// If a dimension is not listed here, default weight is 0.1.
	// All weights are normalized internally.
	DimensionWeights map[string]float64 `json:"dimensionWeights,omitempty"`

	// AlignmentPenalty is the penalty applied when extra dimensions misalign (0..1).
	// Default: 0.2 (20% penalty per misaligned dimension).
	AlignmentPenalty float64 `json:"alignmentPenalty,omitempty"`

	// AlignmentBonus is the bonus applied when extra dimensions align (0..1).
	// Default: 0.1 (10% bonus per aligned dimension).
	AlignmentBonus float64 `json:"alignmentBonus,omitempty"`
}

// DefaultDimensionConfig returns a sensible default configuration.
func DefaultDimensionConfig() RCADimensionConfig {
	return RCADimensionConfig{
		ExtraDimensions:  []string{},
		DimensionWeights: make(map[string]float64),
		AlignmentPenalty: 0.2,
		AlignmentBonus:   0.1,
	}
}

// GetDimensionWeight returns the weight for a given dimension, with default fallback.
func (dc *RCADimensionConfig) GetDimensionWeight(dimension string) float64 {
	if w, ok := dc.DimensionWeights[dimension]; ok {
		return w
	}
	return 0.1 // default weight
}

// ValidateAndNormalize checks the config and ensures weights are reasonable.
// Returns an error if invalid, or a list of warnings if recoverable.
func (dc *RCADimensionConfig) ValidateAndNormalize() ([]string, error) {
	var warnings []string

	if dc.AlignmentPenalty < 0 || dc.AlignmentPenalty > 1 {
		return nil, fmt.Errorf("alignment penalty must be between 0 and 1, got %.2f", dc.AlignmentPenalty)
	}

	if dc.AlignmentBonus < 0 || dc.AlignmentBonus > 1 {
		return nil, fmt.Errorf("alignment bonus must be between 0 and 1, got %.2f", dc.AlignmentBonus)
	}

	// Check for negative or out-of-range weights
	for dim, weight := range dc.DimensionWeights {
		if weight < 0 || weight > 1 {
			warnings = append(warnings, fmt.Sprintf("dimension weight for %q is out of range [0,1]: %.2f; clamping", dim, weight))
			if weight < 0 {
				dc.DimensionWeights[dim] = 0
			} else {
				dc.DimensionWeights[dim] = 1
			}
		}
	}

	if len(dc.ExtraDimensions) == 0 {
		warnings = append(warnings, "no extra dimensions configured; using defaults only")
	}

	return warnings, nil
}

// String returns a debug representation.
func (dc *RCADimensionConfig) String() string {
	dims := strings.Join(dc.ExtraDimensions, ", ")
	if dims == "" {
		dims = "(none)"
	}
	return fmt.Sprintf("DimensionConfig{ExtraDimensions=[%s], DimensionWeights=%d, Penalty=%.2f, Bonus=%.2f}",
		dims, len(dc.DimensionWeights), dc.AlignmentPenalty, dc.AlignmentBonus)
}

// RCADiagnostics captures diagnostics and warnings during RCA computation.
type RCADiagnostics struct {
	// MissingLabels lists standard labels that were not found in the metrics
	// (e.g., "service_name", "span_kind", "status_code").
	MissingLabels []string

	// DimensionDetectionStatus maps dimension keys to whether they were detected in the data.
	DimensionDetectionStatus map[string]bool

	// IsolationForestIssues captures potential misalignment between iforest_is_anomaly
	// classification and iforest_anomaly_score anomaly scores.
	// E.g., "High anomaly scores but low classification rate suggests possible tuning issue"
	IsolationForestIssues []string

	// ReducedAccuracyReasons explains why the RCA confidence is lower than optimal.
	ReducedAccuracyReasons []string

	// MetricsQueryErrors captures any errors encountered while querying metrics
	MetricsQueryErrors []string
}

// NewRCADiagnostics creates a new diagnostics object.
func NewRCADiagnostics() *RCADiagnostics {
	return &RCADiagnostics{
		MissingLabels:            make([]string, 0),
		DimensionDetectionStatus: make(map[string]bool),
		IsolationForestIssues:    make([]string, 0),
		ReducedAccuracyReasons:   make([]string, 0),
		MetricsQueryErrors:       make([]string, 0),
	}
}

// AddMissingLabel records that a standard label is missing.
func (rd *RCADiagnostics) AddMissingLabel(label string) {
	for _, existing := range rd.MissingLabels {
		if existing == label {
			return // already recorded
		}
	}
	rd.MissingLabels = append(rd.MissingLabels, label)
}

// AddReducedAccuracyReason records a reason for reduced confidence.
func (rd *RCADiagnostics) AddReducedAccuracyReason(reason string) {
	for _, existing := range rd.ReducedAccuracyReasons {
		if existing == reason {
			return // already recorded
		}
	}
	rd.ReducedAccuracyReasons = append(rd.ReducedAccuracyReasons, reason)
}

// ToNotes converts diagnostics into human-readable notes for the RCA response.
func (rd *RCADiagnostics) ToNotes() []string {
	var notes []string

	if len(rd.MissingLabels) > 0 {
		labels := strings.Join(rd.MissingLabels, ", ")
		notes = append(notes, fmt.Sprintf("Standard metric labels missing: %s. RCA accuracy may be reduced.", labels))
	}

	for dim, detected := range rd.DimensionDetectionStatus {
		if !detected {
			notes = append(notes, fmt.Sprintf("Configured dimension '%s' not detected in metric data; RCA may not consider this dimension.", dim))
		}
	}

	for _, issue := range rd.IsolationForestIssues {
		notes = append(notes, fmt.Sprintf("IsolationForest: %s", issue))
	}

	for _, reason := range rd.ReducedAccuracyReasons {
		notes = append(notes, reason)
	}

	for _, err := range rd.MetricsQueryErrors {
		notes = append(notes, fmt.Sprintf("Metrics query warning: %s", err))
	}

	return notes
}

// HasSignificantIssues returns true if there are issues that materially affect RCA accuracy.
func (rd *RCADiagnostics) HasSignificantIssues() bool {
	return len(rd.MissingLabels) > 0 || len(rd.IsolationForestIssues) > 0 || len(rd.ReducedAccuracyReasons) > 0
}
