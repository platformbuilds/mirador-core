package rca

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// CollectOptions configures the behavior of anomaly collection.
type CollectOptions struct {
	// TimePaddingBefore adds extra time window before IncidentContext.TStart.
	// Allows capturing precursor anomalies. Default: 0.
	TimePaddingBefore time.Duration

	// TimePaddingAfter adds extra time window after IncidentContext.TEnd.
	// Allows capturing cascading anomalies. Default: 0.
	TimePaddingAfter time.Duration

	// MinSeverity filters anomalies below this severity. Default: 0 (all).
	MinSeverity Severity

	// MinAnomalyScore filters anomalies with score below this. Default: 0 (all).
	MinAnomalyScore float64

	// SignalTypesToInclude restricts anomalies to specific signal types.
	// Empty = all types included.
	SignalTypesToInclude []SignalType

	// ServicesToInclude restricts anomalies to specific services.
	// Empty = all services included (but will be filtered by service graph).
	ServicesToInclude []string

	// MaxEventsPerRing limits the number of events per time ring returned.
	// 0 = no limit.
	MaxEventsPerRing int

	// SortByRingAndPriority: if true, results are sorted by ring proximity then severity.
	SortByRingAndPriority bool

	// TimeRingConfig: configuration for assigning time rings.
	// If nil, uses DefaultTimeRingConfig().
	TimeRingConfig *TimeRingConfig
}

// DefaultCollectOptions returns sensible defaults.
func DefaultCollectOptions() CollectOptions {
	return CollectOptions{
		TimePaddingBefore:     0,
		TimePaddingAfter:      0,
		MinSeverity:           0,
		MinAnomalyScore:       0,
		SignalTypesToInclude:  []SignalType{}, // All types
		ServicesToInclude:     []string{},     // All services
		MaxEventsPerRing:      0,              // No limit
		SortByRingAndPriority: true,
		TimeRingConfig:        nil, // Will use default
	}
}

// AnomalyEventsProvider is an interface for fetching AnomalyEvents.
// This abstracts where anomalies come from (in-memory, database, etc.).
type AnomalyEventsProvider interface {
	// GetAnomalies returns AnomalyEvents in the given time range, optionally filtered by service.
	GetAnomalies(
		ctx context.Context,
		startTime time.Time,
		endTime time.Time,
		services []string, // Empty = all services
	) ([]*AnomalyEvent, error)
}

// IncidentAnomalyCollector gathers and enriches anomalies around an incident.
// It integrates:
// - AnomalyEvents from traces/logs/metrics
// - ServiceGraph topology information
// - Time ring assignment
// to produce EnrichedAnomalyEvents ready for RCA analysis.
type IncidentAnomalyCollector struct {
	anomalyProvider AnomalyEventsProvider
	serviceGraph    *ServiceGraph
	logger          logger.Logger
}

// NewIncidentAnomalyCollector creates a new collector.
func NewIncidentAnomalyCollector(
	provider AnomalyEventsProvider,
	graph *ServiceGraph,
	logger logger.Logger,
) *IncidentAnomalyCollector {
	return &IncidentAnomalyCollector{
		anomalyProvider: provider,
		serviceGraph:    graph,
		logger:          logger,
	}
}

// Collect gathers and enriches anomalies relative to an incident.
// Returns a list of EnrichedAnomalyEvents, potentially sorted by ring and priority.
func (iac *IncidentAnomalyCollector) Collect(
	ctx context.Context,
	incident *IncidentContext,
	opts CollectOptions,
) ([]*EnrichedAnomalyEvent, error) {
	if err := incident.Validate(); err != nil {
		return nil, fmt.Errorf("invalid incident context: %w", err)
	}

	// Use provided TimeRingConfig or default.
	ringCfg := opts.TimeRingConfig
	if ringCfg == nil {
		cfg := DefaultTimeRingConfig()
		ringCfg = &cfg
	}

	// Compute the time window for anomaly fetching.
	startTime := incident.TimeBounds.TStart.Add(-opts.TimePaddingBefore)
	endTime := incident.TimeBounds.TEnd.Add(opts.TimePaddingAfter)

	iac.logger.Debug("Collecting anomalies for incident",
		"incident_id", incident.ID,
		"window_start", startTime.Format(time.RFC3339),
		"window_end", endTime.Format(time.RFC3339))

	// Fetch raw anomaly events from the provider.
	rawAnomalies, err := iac.anomalyProvider.GetAnomalies(ctx, startTime, endTime, opts.ServicesToInclude)
	if err != nil {
		return nil, fmt.Errorf("failed to get anomalies: %w", err)
	}

	iac.logger.Debug("Fetched raw anomalies",
		"count", len(rawAnomalies),
		"incident_id", incident.ID)

	// Filter anomalies based on options.
	filtered := iac.filterAnomalies(rawAnomalies, opts)

	iac.logger.Debug("Filtered anomalies",
		"count", len(filtered),
		"incident_id", incident.ID)

	// Enrich each anomaly with time ring and graph context.
	enriched := iac.enrichAnomalies(filtered, incident, ringCfg)

	iac.logger.Debug("Enriched anomalies",
		"count", len(enriched),
		"incident_id", incident.ID)

	// Apply per-ring limits if configured.
	if opts.MaxEventsPerRing > 0 {
		enriched = iac.limitEventsPerRing(enriched, opts.MaxEventsPerRing)
	}

	// Sort if requested.
	if opts.SortByRingAndPriority {
		enriched = iac.sortByRingAndPriority(enriched)
	}

	return enriched, nil
}

// filterAnomalies applies filtering logic based on CollectOptions.
func (iac *IncidentAnomalyCollector) filterAnomalies(
	anomalies []*AnomalyEvent,
	opts CollectOptions,
) []*AnomalyEvent {
	var filtered []*AnomalyEvent

	// Build signal type filter map.
	signalTypeFilter := make(map[SignalType]bool)
	if len(opts.SignalTypesToInclude) > 0 {
		for _, st := range opts.SignalTypesToInclude {
			signalTypeFilter[st] = true
		}
	}

	for _, anomaly := range anomalies {
		// Severity check.
		if anomaly.Severity < opts.MinSeverity {
			continue
		}

		// Anomaly score check.
		if anomaly.AnomalyScore < opts.MinAnomalyScore {
			continue
		}

		// Signal type check.
		if len(signalTypeFilter) > 0 && !signalTypeFilter[anomaly.SignalType] {
			continue
		}

		filtered = append(filtered, anomaly)
	}

	return filtered
}

// enrichAnomalies adds time ring and graph context to each anomaly.
func (iac *IncidentAnomalyCollector) enrichAnomalies(
	anomalies []*AnomalyEvent,
	incident *IncidentContext,
	ringCfg *TimeRingConfig,
) []*EnrichedAnomalyEvent {
	var enriched []*EnrichedAnomalyEvent

	impactService := ServiceNode(incident.ImpactService)
	anomalyService := ServiceNode("")

	for _, anomaly := range anomalies {
		// Assign time ring.
		ring := AssignTimeRing(incident.TimeBounds.TPeak, anomaly.Timestamp, *ringCfg)

		// Determine graph relationship.
		anomalyService = ServiceNode(anomaly.Service)
		direction, distance := iac.analyzeGraphRelationship(anomalyService, impactService)

		// Create enriched event.
		eae := NewEnrichedAnomalyEvent(
			anomaly,
			ring,
			direction,
			distance,
			incident.ImpactService,
		)

		enriched = append(enriched, eae)
	}

	return enriched
}

// analyzeGraphRelationship determines the graph direction and distance from
// an anomalous service to the impact service.
func (iac *IncidentAnomalyCollector) analyzeGraphRelationship(
	anomalyService, impactService ServiceNode,
) (GraphDirection, int) {
	// Same service.
	if anomalyService == impactService {
		return DirectionSame, 0
	}

	// Check if anomalyService is upstream (can reach impactService).
	if iac.serviceGraph.IsUpstream(anomalyService, impactService) {
		// Compute shortest path to get distance.
		path, found := iac.serviceGraph.ShortestPath(anomalyService, impactService)
		if found {
			distance := len(path) - 1 // Path includes both endpoints.
			return DirectionUpstream, distance
		}
		// Shouldn't happen if IsUpstream returned true, but be safe.
		return DirectionUpstream, -1
	}

	// Check if impactService is upstream to anomalyService (anomaly is downstream).
	if iac.serviceGraph.IsUpstream(impactService, anomalyService) {
		// Compute shortest path to get distance.
		path, found := iac.serviceGraph.ShortestPath(impactService, anomalyService)
		if found {
			distance := len(path) - 1
			return DirectionDownstream, distance
		}
		return DirectionDownstream, -1
	}

	// Not connected in the graph.
	return DirectionUnknown, -1
}

// limitEventsPerRing applies a limit to events in each ring.
func (iac *IncidentAnomalyCollector) limitEventsPerRing(
	events []*EnrichedAnomalyEvent,
	maxPerRing int,
) []*EnrichedAnomalyEvent {
	// Group by ring.
	ringGroups := make(map[TimeRing][]*EnrichedAnomalyEvent)
	for _, event := range events {
		ringGroups[event.Ring] = append(ringGroups[event.Ring], event)
	}

	// Take top maxPerRing from each ring (assume already sorted by priority).
	var result []*EnrichedAnomalyEvent
	for _, ring := range []TimeRing{RingImmediate, RingShort, RingMedium, RingLong} {
		group := ringGroups[ring]
		if len(group) > maxPerRing {
			group = group[:maxPerRing]
		}
		result = append(result, group...)
	}

	return result
}

// sortByRingAndPriority sorts enriched events by ring proximity then by severity/priority.
func (iac *IncidentAnomalyCollector) sortByRingAndPriority(
	events []*EnrichedAnomalyEvent,
) []*EnrichedAnomalyEvent {
	sorted := make([]*EnrichedAnomalyEvent, len(events))
	copy(sorted, events)

	sort.Slice(sorted, func(i, j int) bool {
		// Primary: ring priority (closer to peak is higher).
		ringI := sorted[i].Ring.Priority()
		ringJ := sorted[j].Ring.Priority()
		if ringI != ringJ {
			return ringI < ringJ
		}

		// Secondary: severity (higher is first).
		if sorted[i].AnomalyEvent.Severity != sorted[j].AnomalyEvent.Severity {
			return sorted[i].AnomalyEvent.Severity > sorted[j].AnomalyEvent.Severity
		}

		// Tertiary: anomaly score (higher is first).
		if sorted[i].AnomalyEvent.AnomalyScore != sorted[j].AnomalyEvent.AnomalyScore {
			return sorted[i].AnomalyEvent.AnomalyScore > sorted[j].AnomalyEvent.AnomalyScore
		}

		// Quaternary: timestamp (earlier is first).
		return sorted[i].AnomalyEvent.Timestamp.Before(sorted[j].AnomalyEvent.Timestamp)
	})

	return sorted
}
