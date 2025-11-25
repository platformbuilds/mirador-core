package models

import (
	"time"
)

// StoreCorrelationRequest is the payload accepted by
// POST /api/v1/rca/store (RCAHandler.StoreCorrelation).
// Use the canonical types defined in models.go to avoid casts.
type StoreCorrelationRequest struct {
	CorrelationID string          `json:"correlationId" binding:"required"`
	IncidentID    string          `json:"incidentId"    binding:"required"`
	RootCause     string          `json:"rootCause"     binding:"required"`
	Confidence    float64         `json:"confidence"`
	RedAnchors    []*RedAnchor    `json:"redAnchors,omitempty"` // <-- matches CorrelationEvent.RedAnchors
	Timeline      []TimelineEvent `json:"timeline,omitempty"`   // <-- matches CorrelationEvent.Timeline
}

// ================================
// Phase 4 RCA Engine Models
// ================================

// RCARequest is the HTTP request payload for POST /api/v1/unified/rca.
type RCARequest struct {
	// ImpactService is the service experiencing the impact/incident
	ImpactService string `json:"impactService" binding:"required"`

	// ImpactMetric is the metric indicating the impact (optional; defaults to "error_rate")
	ImpactMetric string `json:"impactMetric,omitempty"`

	// ImpactKPIID is the optional ID of a KPI definition to use as the impact signal.
	// If provided, the handler will resolve the KPI and use it as the impact metric.
	// Takes precedence over ImpactMetric if both are provided.
	ImpactKPIID string `json:"impactKpiId,omitempty"`

	// MetricDirection indicates whether higher or lower values are worse
	// "higher_is_worse" or "lower_is_worse"
	MetricDirection string `json:"metricDirection,omitempty"`

	// TimeStart is the start of the incident window (RFC3339 format)
	TimeStart string `json:"timeStart" binding:"required"`

	// TimeEnd is the end of the incident window (RFC3339 format)
	TimeEnd string `json:"timeEnd" binding:"required"`

	// Severity is the business impact severity (0.0 to 1.0)
	Severity float64 `json:"severity,omitempty"`

	// ImpactSummary is a short description of the incident
	ImpactSummary string `json:"impactSummary,omitempty"`

	// RCAOptions (optional)
	MaxChains         int                    `json:"maxChains,omitempty"`
	MaxStepsPerChain  int                    `json:"maxStepsPerChain,omitempty"`
	MinScoreThreshold float64                `json:"minScoreThreshold,omitempty"`
	DimensionConfig   *RCADimensionConfigDTO `json:"dimensionConfig,omitempty"`
}

// RCADimensionConfigDTO represents the dimension configuration in the HTTP request.
type RCADimensionConfigDTO struct {
	// ExtraDimensions is a list of additional label keys to consider
	ExtraDimensions []string `json:"extraDimensions,omitempty"`

	// DimensionWeights maps dimension keys to their influence weight (0..1)
	DimensionWeights map[string]float64 `json:"dimensionWeights,omitempty"`

	// AlignmentPenalty is the penalty applied when dimensions misalign (0..1)
	AlignmentPenalty float64 `json:"alignmentPenalty,omitempty"`

	// AlignmentBonus is the bonus applied when dimensions align (0..1)
	AlignmentBonus float64 `json:"alignmentBonus,omitempty"`
}

// RCAResponse is the HTTP response for POST /api/v1/unified/rca.
type RCAResponse struct {
	// Status is "success" or "error"
	Status string `json:"status"`

	// Data contains the RCAIncidentDTO if successful
	Data *RCAIncidentDTO `json:"data,omitempty"`

	// Error message if status is "error"
	Error string `json:"error,omitempty"`

	// Timestamp when the response was generated
	Timestamp time.Time `json:"timestamp"`
}

// RCAIncidentDTO is the DTO representation of RCAIncident for JSON serialization.
type RCAIncidentDTO struct {
	// Impact is the incident context
	Impact *IncidentContextDTO `json:"impact"`

	// RootCause is the selected root cause step
	RootCause *RCAStepDTO `json:"rootCause,omitempty"`

	// Chains are all candidate RCA chains
	Chains []*RCAChainDTO `json:"chains"`

	// GeneratedAt is when this RCA was computed
	GeneratedAt time.Time `json:"generatedAt"`

	// Score is the overall confidence (0..1)
	Score float64 `json:"score"`

	// Notes are optional observations
	Notes []string `json:"notes"`

	// Diagnostics contains warnings about missing labels, dimension detection, IsolationForest tuning, etc.
	Diagnostics *RCADiagnosticsDTO `json:"diagnostics,omitempty"`
}

// RCADiagnosticsDTO represents diagnostics in the HTTP response.
type RCADiagnosticsDTO struct {
	// MissingLabels lists standard labels not found in the metrics
	MissingLabels []string `json:"missingLabels,omitempty"`

	// DimensionDetectionStatus maps dimension keys to detection status
	DimensionDetectionStatus map[string]bool `json:"dimensionDetectionStatus,omitempty"`

	// IsolationForestIssues lists any potential IsolationForest tuning issues
	IsolationForestIssues []string `json:"isolationForestIssues,omitempty"`

	// ReducedAccuracyReasons explains why confidence is lower than optimal
	ReducedAccuracyReasons []string `json:"reducedAccuracyReasons,omitempty"`

	// MetricsQueryErrors captures any metrics query issues
	MetricsQueryErrors []string `json:"metricsQueryErrors,omitempty"`
}

// IncidentContextDTO is the DTO for IncidentContext.
type IncidentContextDTO struct {
	ID            string `json:"id"`
	ImpactService string `json:"impactService"`
	MetricName    string `json:"metricName"`
	// ImpactServiceUUID is the original UUID if ImpactService was resolved from a KPI
	ImpactServiceUUID string `json:"impactServiceUuid,omitempty"`
	// MetricNameUUID is the original UUID if MetricName was resolved from a KPI
	MetricNameUUID string  `json:"metricNameUuid,omitempty"`
	TimeStartStr   string  `json:"timeStart"`
	TimeEndStr     string  `json:"timeEnd"`
	ImpactSummary  string  `json:"impactSummary"`
	Severity       float64 `json:"severity"`
}

// RCAChainDTO is the DTO for RCAChain.
type RCAChainDTO struct {
	// Steps in the chain from impact to root cause
	Steps []*RCAStepDTO `json:"steps"`

	// Score of this chain
	Score float64 `json:"score"`

	// Rank (1 = best chain)
	Rank int `json:"rank"`

	// ImpactPath service names
	ImpactPath []string `json:"impactPath"`

	// DurationHops number of steps
	DurationHops int `json:"durationHops"`
}

// RCAStepDTO is the DTO for RCAStep.
type RCAStepDTO struct {
	// Why index in the chain (1 = most user-facing)
	WhyIndex int `json:"whyIndex"`

	// Service name (resolved from KPI if applicable, otherwise service identifier)
	Service string `json:"service"`

	// Component name (resolved from KPI if applicable, otherwise component identifier)
	Component string `json:"component"`

	// ServiceUUID is the original UUID if Service was resolved from a KPI
	ServiceUUID string `json:"serviceUuid,omitempty"`

	// ComponentUUID is the original UUID if Component was resolved from a KPI
	ComponentUUID string `json:"componentUuid,omitempty"`

	// KPIName is the human-readable name of the KPI if Service/Component is a KPI UUID
	KPIName string `json:"kpiName,omitempty"`

	// KPIFormula is the query formula of the KPI if Service/Component is a KPI UUID
	KPIFormula string `json:"kpiFormula,omitempty"`

	// TimeStart when this anomaly was detected
	TimeStart time.Time `json:"timeStart"`

	// TimeEnd of the anomaly window
	TimeEnd time.Time `json:"timeEnd"`

	// Ring classification
	Ring string `json:"ring"`

	// Direction (upstream/downstream/same/unknown)
	Direction string `json:"direction"`

	// Distance in hops to impact service
	Distance int `json:"distance"`

	// Evidence references
	Evidence []*EvidenceRefDTO `json:"evidence"`

	// Summary explanation
	Summary string `json:"summary"`

	// Score for this step
	Score float64 `json:"score"`
}

// EvidenceRefDTO is the DTO for EvidenceRef.
type EvidenceRefDTO struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Details string `json:"details"`
}
