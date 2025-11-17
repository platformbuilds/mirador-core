package rca

import (
	"time"
)

// TimeRing represents the classification of an anomaly relative to the incident peak time.
// It helps group anomalies into temporal rings around the impact.
type TimeRing string

const (
	// RingImmediate (R1): Anomalies very close to peak (last 5 seconds before peak).
	RingImmediate TimeRing = "R1_IMMEDIATE"

	// RingShort (R2): Anomalies shortly before peak (5-30 seconds before).
	RingShort TimeRing = "R2_SHORT"

	// RingMedium (R3): Anomalies moderately before peak (30 seconds - 2 minutes).
	RingMedium TimeRing = "R3_MEDIUM"

	// RingLong (R4): Anomalies further back (2 minutes - lookback window).
	RingLong TimeRing = "R4_LONG"

	// RingOutOfScope: Anomalies outside the analysis window.
	RingOutOfScope TimeRing = "R_OUT_OF_SCOPE"
)

// TimeRingConfig defines the time boundaries for each ring.
// All durations are relative to TPeak (negative values = before peak).
type TimeRingConfig struct {
	// R1 boundary: [TPeak - R1Duration, TPeak]
	// Default: 5 seconds
	R1Duration time.Duration

	// R2 boundary: [TPeak - R2Duration, TPeak - R1Duration)
	// Default: 30 seconds
	R2Duration time.Duration

	// R3 boundary: [TPeak - R3Duration, TPeak - R2Duration)
	// Default: 2 minutes
	R3Duration time.Duration

	// R4Duration represents the maximum lookback window.
	// [TPeak - R4Duration, TPeak - R3Duration)
	// Default: 10 minutes (a reasonable default for incident analysis)
	R4Duration time.Duration

	// AllowEventsAfterPeak: if true, allows anomalies up to some time after peak
	// (useful for catching late-arriving telemetry). Default: false.
	AllowEventsAfterPeak bool

	// TimeAfterPeak: if AllowEventsAfterPeak is true, allows events up to this
	// duration after TPeak. Default: 30 seconds.
	TimeAfterPeak time.Duration
}

// DefaultTimeRingConfig returns sensible defaults aligned with incident analysis.
// These can be tuned per-incident if needed.
func DefaultTimeRingConfig() TimeRingConfig {
	return TimeRingConfig{
		R1Duration:           5 * time.Second,
		R2Duration:           30 * time.Second,
		R3Duration:           2 * time.Minute,
		R4Duration:           10 * time.Minute,
		AllowEventsAfterPeak: true,
		TimeAfterPeak:        30 * time.Second,
	}
}

// AssignTimeRing determines which ring an event belongs to, given the peak time and config.
// peakTime: the TPeak of the incident
// eventTime: the timestamp of the event/anomaly
// cfg: the time ring configuration
//
// Returns the appropriate TimeRing, or RingOutOfScope if event is outside the window.
func AssignTimeRing(peakTime, eventTime time.Time, cfg TimeRingConfig) TimeRing {
	timeDiff := peakTime.Sub(eventTime) // Positive if event is before peak

	// Check if event is after peak (late arrival).
	if timeDiff < 0 {
		afterPeakDiff := eventTime.Sub(peakTime)
		if cfg.AllowEventsAfterPeak && afterPeakDiff <= cfg.TimeAfterPeak {
			// Treat post-peak events as immediate (closest to peak)
			return RingImmediate
		}
		// Event is too far after peak
		return RingOutOfScope
	}

	// Event is before peak. Classify by distance.
	if timeDiff <= cfg.R1Duration {
		return RingImmediate
	}
	if timeDiff <= cfg.R2Duration {
		return RingShort
	}
	if timeDiff <= cfg.R3Duration {
		return RingMedium
	}
	if timeDiff <= cfg.R4Duration {
		return RingLong
	}

	// Event is outside the lookback window.
	return RingOutOfScope
}

// String returns a human-readable name for the time ring.
func (tr TimeRing) String() string {
	return string(tr)
}

// IsInScope returns true if the ring represents an event within the analysis scope.
func (tr TimeRing) IsInScope() bool {
	return tr != RingOutOfScope
}

// Priority returns a numeric priority for sorting (lower = closer to peak = higher priority).
// This is useful for ranking anomalies by proximity to the incident.
func (tr TimeRing) Priority() int {
	switch tr {
	case RingImmediate:
		return 0
	case RingShort:
		return 1
	case RingMedium:
		return 2
	case RingLong:
		return 3
	default:
		return 999
	}
}

// TimeRingOrderings is a helpful utility to sort multiple rings by temporal proximity to peak.
type TimeRingOrdering struct {
	Ring        TimeRing
	EventCount  int
	AvgSeverity float64
}

// SortByProximity sorts rings in order of closest to peak.
func SortByProximity(rings []TimeRing) []TimeRing {
	sorted := make([]TimeRing, len(rings))
	copy(sorted, rings)

	// Bubble sort (simple, for small lists)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority() < sorted[i].Priority() {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}
