package rca

import (
	"time"
)

// GroupingConfig configures the behavior of anomaly grouping.
type GroupingConfig struct {
	// BucketWidth is the time duration for grouping anomalies into buckets.
	// Anomalies within the same bucket are grouped together.
	// Default: 10 seconds.
	BucketWidth time.Duration

	// MinEventsPerGroup filters out groups with fewer than this many events.
	// Default: 0 (no minimum).
	MinEventsPerGroup int

	// MinSeverity filters out groups where all events are below this severity.
	// Default: 0 (no minimum).
	MinSeverity Severity

	// MinAnomalyScore filters out groups where max anomaly score is below this.
	// Default: 0 (no minimum).
	MinAnomalyScore float64

	// GroupByComponent: if true, groups are keyed by (Service, Component, Ring, TimeBucket).
	// If false, groups are keyed by (Service, Ring, TimeBucket) (ignoring component).
	// Default: true.
	GroupByComponent bool
}

// DefaultGroupingConfig returns sensible defaults for grouping.
func DefaultGroupingConfig() GroupingConfig {
	return GroupingConfig{
		BucketWidth:       10 * time.Second,
		MinEventsPerGroup: 0,
		MinSeverity:       0,
		MinAnomalyScore:   0,
		GroupByComponent:  true,
	}
}

// GroupEnrichedAnomalies partitions EnrichedAnomalyEvents into AnomalyGroups
// based on service, component (optional), time ring, and time bucket.
// Returns a slice of AnomalyGroups, potentially filtered by config constraints.
func GroupEnrichedAnomalies(events []*EnrichedAnomalyEvent, cfg GroupingConfig) []*AnomalyGroup {
	if len(events) == 0 {
		return []*AnomalyGroup{}
	}

	// Map from GroupKey to AnomalyGroup
	groupMap := make(map[GroupKey]*AnomalyGroup)

	// Process each event
	for _, event := range events {
		if event == nil || event.AnomalyEvent == nil {
			continue
		}

		// Compute the time bucket for this event
		timeBucket := computeTimeBucket(event.AnomalyEvent.Timestamp, cfg.BucketWidth)

		// Create the group key
		key := GroupKey{
			Service:    event.AnomalyEvent.Service,
			Ring:       event.Ring,
			TimeBucket: timeBucket,
		}

		if cfg.GroupByComponent {
			key.Component = event.AnomalyEvent.Component
		}

		// Get or create group
		if _, exists := groupMap[key]; !exists {
			groupMap[key] = NewAnomalyGroup(key.Service, key.Component, key.Ring)
		}

		// Add event to group
		groupMap[key].AddEvent(event)
	}

	// Convert map to slice and finalize stats
	var result []*AnomalyGroup
	for _, group := range groupMap {
		group.FinalizeStats()

		// Apply filtering constraints
		if !passesFilters(group, cfg) {
			continue
		}

		result = append(result, group)
	}

	return result
}

// computeTimeBucket computes the start time of the bucket for a given timestamp.
// All timestamps in the same bucket interval map to the same bucket start.
func computeTimeBucket(ts time.Time, bucketWidth time.Duration) int64 {
	if bucketWidth <= 0 {
		bucketWidth = 10 * time.Second
	}

	bucketNanos := bucketWidth.Nanoseconds()
	tsNanos := ts.UnixNano()

	// Floor division to get bucket start
	bucketStart := (tsNanos / bucketNanos) * bucketNanos

	return bucketStart
}

// passesFilters checks if an AnomalyGroup passes all filtering constraints.
func passesFilters(group *AnomalyGroup, cfg GroupingConfig) bool {
	// Check minimum event count
	if cfg.MinEventsPerGroup > 0 && group.EventCount < cfg.MinEventsPerGroup {
		return false
	}

	// Check minimum severity
	if cfg.MinSeverity > 0 && group.MaxSeverity < cfg.MinSeverity {
		return false
	}

	// Check minimum anomaly score
	if cfg.MinAnomalyScore > 0 && group.MaxScore < cfg.MinAnomalyScore {
		return false
	}

	return true
}
