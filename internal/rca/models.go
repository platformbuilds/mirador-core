package rca

import (
	"fmt"
	"time"
)

// EvidenceRef is a pointer to evidence supporting an RCA step.
// It references anomaly groups, metrics, logs, traces, or changes.
type EvidenceRef struct {
	// Type is the kind of evidence: "anomaly_group", "metric", "log", "trace", "change"
	Type string

	// ID is an opaque identifier for this piece of evidence
	// (e.g., group key, metric name+labels, trace ID, change event ID)
	ID string

	// Details is a short explanation of why this evidence is relevant
	Details string
}

// RCAStep represents one "why" in the 5-Why chain.
// It explains an observed anomaly or failure at a particular service/component.
type RCAStep struct {
	// WhyIndex is the position in the 5-Why chain (1 = most user-facing, N = deepest upstream)
	WhyIndex int

	// Service is the service where this anomaly/failure was detected
	Service string

	// Component is the component within the service (e.g., "database", "cache", "api")
	Component string

	// TimeRange is the time interval when this anomaly was detected
	TimeRange TimeRange

	// Ring is the temporal proximity classification of this step relative to incident peak
	Ring TimeRing

	// Direction is the graph relationship of this service to the impact service
	Direction GraphDirection

	// Distance is the hop count from this service to the impact service
	// -1 means unknown/not determinable
	Distance int

	// Evidence is a list of references supporting this step
	Evidence []EvidenceRef

	// Summary is a human-readable explanation of what happened at this step
	// Template-based, not free-form AI-generated text
	Summary string

	// Score is the confidence/suspicion score for this step (0..1)
	Score float64
}

// RCAChain represents a sequence of RCASteps from impact to root cause.
// A chain represents one possible causal path explaining the incident.
type RCAChain struct {
	// Steps are ordered from Why 1 (impact) to Why N (deepest root cause)
	Steps []*RCAStep

	// Score is the aggregate confidence score for this entire chain (0..1)
	Score float64

	// Rank is the position in the ranked list of chains (1 = best chain)
	Rank int

	// ImpactPath is a list of service names in order from impact service to root cause
	// e.g., ["api-gateway", "tps", "kafka", "cassandra"]
	ImpactPath []string

	// DurationHops is the total number of hops/steps in this chain
	DurationHops int
}

// RCAIncident is the top-level result of RCA analysis.
// It contains the impact context, one or more candidate chains, and the selected root cause.
type RCAIncident struct {
	// Impact is the original incident context (service, metric, time window, severity)
	Impact *IncidentContext

	// RootCause is the primary root cause step selected from all chains
	RootCause *RCAStep

	// Chains are all candidate RCA chains ranked by score
	Chains []*RCAChain

	// GeneratedAt is when this RCA analysis was performed
	GeneratedAt time.Time

	// Score is the overall confidence/quality score (0..1)
	// Typically the score of the best chain
	Score float64

	// Notes are optional observations, assumptions, or caveats
	Notes []string

	// Diagnostics captures warnings about missing labels, dimension detection, IsolationForest tuning issues, etc.
	Diagnostics *RCADiagnostics
}

// NewRCAStep creates a new RCAStep with default values.
func NewRCAStep(whyIndex int, service, component string) *RCAStep {
	return &RCAStep{
		WhyIndex:  whyIndex,
		Service:   service,
		Component: component,
		Distance:  -1,
		Evidence:  make([]EvidenceRef, 0),
		Score:     0.0,
	}
}

// AddEvidence adds an evidence reference to this step.
func (step *RCAStep) AddEvidence(evType, evID, details string) {
	step.Evidence = append(step.Evidence, EvidenceRef{
		Type:    evType,
		ID:      evID,
		Details: details,
	})
}

// NewRCAChain creates a new RCAChain with default values.
func NewRCAChain() *RCAChain {
	return &RCAChain{
		Steps:        make([]*RCAStep, 0),
		Score:        0.0,
		Rank:         0,
		ImpactPath:   make([]string, 0),
		DurationHops: 0,
	}
}

// AddStep adds an RCAStep to this chain.
func (chain *RCAChain) AddStep(step *RCAStep) {
	chain.Steps = append(chain.Steps, step)
	if step != nil && step.Service != "" {
		chain.ImpactPath = append(chain.ImpactPath, step.Service)
	}
	chain.DurationHops = len(chain.Steps)
}

// NewRCAIncident creates a new RCAIncident with default values.
func NewRCAIncident(impact *IncidentContext) *RCAIncident {
	return &RCAIncident{
		Impact:      impact,
		RootCause:   nil,
		Chains:      make([]*RCAChain, 0),
		GeneratedAt: time.Now().UTC(),
		Score:       0.0,
		Notes:       make([]string, 0),
		Diagnostics: NewRCADiagnostics(),
	}
}

// AddChain adds an RCAChain to this incident.
func (incident *RCAIncident) AddChain(chain *RCAChain) {
	chain.Rank = len(incident.Chains) + 1
	incident.Chains = append(incident.Chains, chain)
}

// SetRootCauseFromBestChain sets RootCause to the final step of the best-scoring chain.
func (incident *RCAIncident) SetRootCauseFromBestChain() {
	if len(incident.Chains) > 0 && len(incident.Chains[0].Steps) > 0 {
		bestChain := incident.Chains[0]
		incident.RootCause = bestChain.Steps[len(bestChain.Steps)-1]
		incident.Score = bestChain.Score
	}
}

// String returns a debug string representation.
func (ri *RCAIncident) String() string {
	rootCauseStr := "none"
	if ri.RootCause != nil {
		rootCauseStr = fmt.Sprintf("%s:%s", ri.RootCause.Service, ri.RootCause.Component)
	}
	return fmt.Sprintf(
		"RCAIncident{Impact=%s, RootCause=%s, Chains=%d, Score=%.2f}",
		ri.Impact.ID,
		rootCauseStr,
		len(ri.Chains),
		ri.Score,
	)
}

// TemplateBasedSummary generates a template-based summary for an RCAStep.
// This is deterministic and not AI-generated.
// If diagnostics are provided and contain significant issues, the summary may include notes about reduced accuracy.
func TemplateBasedSummary(step *RCAStep, impactService string, diagnostics ...*RCADiagnostics) string {
	var baseSummary string

	if step.Service == impactService {
		baseSummary = fmt.Sprintf(
			"Why %d: %s experienced %s anomalies in the %s (detected %s, severity contributed from %d evidence points)",
			step.WhyIndex,
			step.Service,
			step.Component,
			step.Ring.String(),
			step.TimeRange.Start.Format("15:04:05"),
			len(step.Evidence),
		)
	} else {
		directionStr := "related"
		if step.Direction == DirectionUpstream {
			directionStr = "upstream"
		} else if step.Direction == DirectionSame {
			directionStr = "same service"
		}

		distanceStr := "unknown distance"
		if step.Distance >= 0 {
			distanceStr = fmt.Sprintf("%d hops away", step.Distance)
		}

		baseSummary = fmt.Sprintf(
			"Why %d: %s (%s) at %s showed anomalies in %s (%s, %s). This likely caused failures in %s. Evidence: %d corroborating anomalies.",
			step.WhyIndex,
			step.Service,
			step.Component,
			step.TimeRange.Start.Format("15:04:05"),
			step.Ring.String(),
			directionStr,
			distanceStr,
			impactService,
			len(step.Evidence),
		)
	}

	// Append diagnostics note if provided and significant
	if len(diagnostics) > 0 && diagnostics[0] != nil && diagnostics[0].HasSignificantIssues() {
		baseSummary += " [Note: RCA accuracy may be reduced due to missing metrics labels or configuration issues.]"
	}

	return baseSummary
}
