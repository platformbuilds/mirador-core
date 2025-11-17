package rca

// GraphDirection represents the direction of a service relative to the impacted service
// in the service dependency graph.
type GraphDirection string

const (
	// DirectionUpstream: The anomalous service is upstream (a dependency of impact service).
	DirectionUpstream GraphDirection = "upstream"

	// DirectionDownstream: The anomalous service is downstream (depends on impact service).
	DirectionDownstream GraphDirection = "downstream"

	// DirectionSame: The anomaly is in the same service as the impact.
	DirectionSame GraphDirection = "same"

	// DirectionUnknown: The relationship is unknown or not determinable from the graph.
	DirectionUnknown GraphDirection = "unknown"
)

// EnrichedAnomalyEvent extends AnomalyEvent with RCA-specific context
// derived from time rings and service graph analysis.
type EnrichedAnomalyEvent struct {
	// Base anomaly event (from Phase 1).
	AnomalyEvent *AnomalyEvent

	// Ring is the time ring this event belongs to relative to incident peak.
	Ring TimeRing

	// GraphDirection is the relationship of the anomalous service to the impacted service.
	GraphDirection GraphDirection

	// GraphDistanceToImpact is the hop count to reach the impact service.
	// -1 means unknown or not determinable.
	// 0 means same service.
	// 1+ means number of hops away.
	GraphDistanceToImpact int

	// ImpactService is the name of the service being impacted (for reference).
	ImpactService string
}

// NewEnrichedAnomalyEvent creates a new EnrichedAnomalyEvent from a base AnomalyEvent.
func NewEnrichedAnomalyEvent(
	anomaly *AnomalyEvent,
	ring TimeRing,
	direction GraphDirection,
	distanceToImpact int,
	impactService string,
) *EnrichedAnomalyEvent {
	return &EnrichedAnomalyEvent{
		AnomalyEvent:          anomaly,
		Ring:                  ring,
		GraphDirection:        direction,
		GraphDistanceToImpact: distanceToImpact,
		ImpactService:         impactService,
	}
}

// IsCandidate returns true if the enriched event is a potential RCA candidate.
// A candidate is typically in-scope (not out of scope) and has meaningful graph context.
// This is a convenience predicate for filtering.
func (eae *EnrichedAnomalyEvent) IsCandidate() bool {
	if eae.Ring == RingOutOfScope {
		return false
	}
	// Could add more criteria here (e.g., must be upstream or same service, high severity, etc.)
	return true
}

// IsHighPriority returns true if the event is high-priority for analysis.
// High-priority events are typically:
// - In the immediate/short rings (R1, R2)
// - Same service as impact or upstream
// - High severity
func (eae *EnrichedAnomalyEvent) IsHighPriority() bool {
	if !eae.IsCandidate() {
		return false
	}

	// Immediate or short-term rings are higher priority.
	if eae.Ring != RingImmediate && eae.Ring != RingShort {
		return false
	}

	// Same service or upstream is higher priority (direct cause more likely).
	if eae.GraphDirection != DirectionSame && eae.GraphDirection != DirectionUpstream {
		return false
	}

	// At least medium severity.
	if eae.AnomalyEvent.Severity < SeverityMedium {
		return false
	}

	return true
}
