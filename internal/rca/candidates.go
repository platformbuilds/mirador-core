package rca

import (
	"fmt"
	"time"
)

// TimeRange represents a time interval.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Duration returns the duration of the time range.
func (tr TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

// AnomalyGroup represents a cohesive group of EnrichedAnomalyEvents
// aggregated by service, component, time ring, and time bucket.
// It captures statistical and topological context about the anomalies.
type AnomalyGroup struct {
	// Service name where anomalies were detected.
	Service string

	// Component within the service (e.g., "database", "cache", "api").
	Component string

	// Ring is the time ring classification of these anomalies.
	Ring TimeRing

	// TimeRange is the earliest and latest timestamp of events in this group.
	TimeRange TimeRange

	// Events are the underlying EnrichedAnomalyEvents in this group.
	Events []*EnrichedAnomalyEvent

	// Aggregated Statistics

	// EventCount is the total number of events in this group.
	EventCount int

	// DistinctTxnCount is the count of unique transaction IDs found in the Tags.
	DistinctTxnCount int

	// MaxSeverity is the highest severity among all events.
	MaxSeverity Severity

	// AvgSeverity is the average severity of all events.
	AvgSeverity Severity

	// MaxScore is the highest anomaly score.
	MaxScore float64

	// AvgScore is the average anomaly score.
	AvgScore float64

	// Graph Context (Aggregated)

	// GraphDirection represents the dominant relationship of this group to the impact service.
	// For example, if 3 events are upstream and 1 is downstream, the group is marked upstream.
	GraphDirection GraphDirection

	// MinGraphDistance is the minimum hop count to impact service.
	// -1 means unknown or not determinable.
	MinGraphDistance int

	// MaxGraphDistance is the maximum hop count to impact service.
	MaxGraphDistance int

	// ExtraDimensionValues maps dimension keys to their values for this group
	// (e.g., {"env": "prod", "region": "us-east-1", "namespace": "critical"}).
	// Used for alignment scoring when user-configured dimensions are provided.
	ExtraDimensionValues map[string]string
}

// NewAnomalyGroup creates a new AnomalyGroup with the given parameters.
func NewAnomalyGroup(service, component string, ring TimeRing) *AnomalyGroup {
	return &AnomalyGroup{
		Service:              service,
		Component:            component,
		Ring:                 ring,
		Events:               make([]*EnrichedAnomalyEvent, 0),
		MaxSeverity:          0,
		AvgSeverity:          0,
		MaxScore:             0,
		AvgScore:             0,
		GraphDirection:       DirectionUnknown,
		MinGraphDistance:     -1,
		MaxGraphDistance:     -1,
		ExtraDimensionValues: make(map[string]string),
	}
}

// AddEvent adds an EnrichedAnomalyEvent to this group and updates aggregated statistics.
func (ag *AnomalyGroup) AddEvent(event *EnrichedAnomalyEvent) {
	if event == nil {
		return
	}

	ag.Events = append(ag.Events, event)

	// Update time range
	if ag.EventCount == 0 {
		ag.TimeRange.Start = event.AnomalyEvent.Timestamp
		ag.TimeRange.End = event.AnomalyEvent.Timestamp
	} else {
		if event.AnomalyEvent.Timestamp.Before(ag.TimeRange.Start) {
			ag.TimeRange.Start = event.AnomalyEvent.Timestamp
		}
		if event.AnomalyEvent.Timestamp.After(ag.TimeRange.End) {
			ag.TimeRange.End = event.AnomalyEvent.Timestamp
		}
	}

	ag.EventCount++

	// Update severity stats
	if event.AnomalyEvent.Severity > ag.MaxSeverity {
		ag.MaxSeverity = event.AnomalyEvent.Severity
	}

	// Update anomaly score stats
	if event.AnomalyEvent.AnomalyScore > ag.MaxScore {
		ag.MaxScore = event.AnomalyEvent.AnomalyScore
	}

	// Update graph context
	if event.GraphDirection != DirectionUnknown {
		if ag.GraphDirection == DirectionUnknown {
			ag.GraphDirection = event.GraphDirection
		} else if ag.GraphDirection != event.GraphDirection {
			// If we have mixed directions, prefer upstream (more suspicious)
			if event.GraphDirection == DirectionUpstream || ag.GraphDirection == DirectionUpstream {
				ag.GraphDirection = DirectionUpstream
			} else if event.GraphDirection == DirectionSame || ag.GraphDirection == DirectionSame {
				ag.GraphDirection = DirectionSame
			}
		}
	}

	// Update graph distance
	if event.GraphDistanceToImpact >= 0 {
		if ag.MinGraphDistance < 0 {
			ag.MinGraphDistance = event.GraphDistanceToImpact
			ag.MaxGraphDistance = event.GraphDistanceToImpact
		} else {
			if event.GraphDistanceToImpact < ag.MinGraphDistance {
				ag.MinGraphDistance = event.GraphDistanceToImpact
			}
			if event.GraphDistanceToImpact > ag.MaxGraphDistance {
				ag.MaxGraphDistance = event.GraphDistanceToImpact
			}
		}
	}
}

// FinalizeStats computes average statistics after all events have been added.
func (ag *AnomalyGroup) FinalizeStats() {
	if ag.EventCount == 0 {
		return
	}

	// Compute average severity
	totalSeverity := Severity(0)
	for _, event := range ag.Events {
		totalSeverity += event.AnomalyEvent.Severity
	}
	ag.AvgSeverity = totalSeverity / Severity(ag.EventCount)

	// Compute average anomaly score
	totalScore := 0.0
	for _, event := range ag.Events {
		totalScore += event.AnomalyEvent.AnomalyScore
	}
	ag.AvgScore = totalScore / float64(ag.EventCount)

	// Count distinct transaction IDs
	txnIDs := make(map[string]bool)
	for _, event := range ag.Events {
		if txnID, exists := event.AnomalyEvent.Tags["transaction_id"]; exists && txnID != "" {
			txnIDs[txnID] = true
		}
	}
	ag.DistinctTxnCount = len(txnIDs)
}

// CandidateCause represents a ranked candidate root cause of an incident.
// It wraps an AnomalyGroup with a computed score and rank.
type CandidateCause struct {
	// Group is the underlying AnomalyGroup.
	Group *AnomalyGroup

	// Score is the computed suspicion score (typically 0.0 to 1.0, but can be unbounded).
	// Higher score = more suspicious as a root cause.
	Score float64

	// Rank is the position in the ranked list (1 = most suspicious).
	Rank int

	// Reasons are machine-readable reasons for the suspicion score.
	// Examples: ["ring_r1_high_weight", "upstream_direction", "high_anomaly_score"]
	Reasons []string

	// DetailedScore contains component-wise scores for transparency.
	DetailedScore *DetailedScore
}

// DetailedScore breaks down the overall score into component dimensions.
type DetailedScore struct {
	// RingScore: contribution from temporal proximity (0..1)
	RingScore float64

	// DirectionScore: contribution from graph direction (0..1)
	DirectionScore float64

	// DistanceScore: contribution from graph distance (0..1)
	DistanceScore float64

	// SeverityScore: contribution from anomaly severity (0..1)
	SeverityScore float64

	// AnomalyScoreContribution: contribution from anomaly score (0..1)
	AnomalyScoreContribution float64

	// TransactionCountScore: contribution from breadth of impact (0..1)
	TransactionCountScore float64

	// DimensionAlignmentScore: contribution from extra dimension alignment (-penalty..+bonus)
	DimensionAlignmentScore float64

	// DimensionAlignments: detailed breakdown of each dimension (if computed)
	DimensionAlignments []DimensionAlignment
}

// NewCandidateCause creates a new CandidateCause with default values.
func NewCandidateCause(group *AnomalyGroup) *CandidateCause {
	return &CandidateCause{
		Group:         group,
		Score:         0.0,
		Rank:          0,
		Reasons:       make([]string, 0),
		DetailedScore: &DetailedScore{},
	}
}

// String returns a human-readable representation of the candidate cause.
func (cc *CandidateCause) String() string {
	return fmt.Sprintf(
		"CandidateCause{Rank=%d, Service=%s, Component=%s, Ring=%s, Score=%.4f, Events=%d}",
		cc.Rank,
		cc.Group.Service,
		cc.Group.Component,
		cc.Group.Ring,
		cc.Score,
		cc.Group.EventCount,
	)
}

// GroupKey is a composite key for grouping anomalies.
type GroupKey struct {
	Service    string
	Component  string
	Ring       TimeRing
	TimeBucket int64 // Unix nanoseconds of bucket start
}
