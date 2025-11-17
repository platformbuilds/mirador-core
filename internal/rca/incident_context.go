package rca

import (
	"fmt"
	"time"
)

// ImpactSignal represents a metric or signal that defines user/business impact.
// It captures what is "broken" or degraded during an incident.
type ImpactSignal struct {
	// ServiceName is the service being impacted.
	ServiceName string

	// MetricName is the name of the metric indicating impact
	// (e.g., "error_rate", "p95_latency", "failed_transactions_rate").
	MetricName string

	// Labels are optional key-value pairs to identify the signal more specifically
	// (e.g., {"endpoint": "/api/v1/transactions", "method": "POST"}).
	Labels map[string]string

	// Direction indicates whether higher or lower values are worse.
	// "higher_is_worse": metric spikes upward = bad (e.g., error_rate, latency)
	// "lower_is_worse": metric drops downward = bad (e.g., throughput, availability)
	Direction string // "higher_is_worse" or "lower_is_worse"

	// Threshold is the value at which the metric is considered "impacted".
	Threshold float64
}

// IncidentTimeWindow represents the time interval of an incident.
type IncidentTimeWindow struct {
	// TStart is when the impact first became detectable (threshold breach).
	TStart time.Time

	// TPeak is when the impact was at its worst.
	TPeak time.Time

	// TEnd is when the impact recovered (or now if not yet recovered).
	TEnd time.Time
}

// Duration returns the total duration of the incident window (TEnd - TStart).
func (itw *IncidentTimeWindow) Duration() time.Duration {
	return itw.TEnd.Sub(itw.TStart)
}

// IsValid checks that the time window is logically ordered.
func (itw *IncidentTimeWindow) IsValid() bool {
	if itw.TStart.IsZero() || itw.TPeak.IsZero() || itw.TEnd.IsZero() {
		return false
	}
	if !itw.TStart.Before(itw.TPeak) {
		return false
	}
	if !itw.TPeak.Before(itw.TEnd) && itw.TPeak != itw.TEnd {
		return false
	}
	return true
}

// IncidentContext represents the core incident information needed for RCA.
// It defines the "thing that's broken" and when it was broken.
type IncidentContext struct {
	// ID is a unique identifier for this incident (e.g., UUID).
	ID string

	// ImpactService is the primary service experiencing degradation.
	ImpactService string

	// ImpactSignal describes the metric that defines the impact.
	ImpactSignal ImpactSignal

	// TimeBounds is the time interval of the incident.
	TimeBounds IncidentTimeWindow

	// ImpactSummary is a brief human-readable description of what happened.
	ImpactSummary string

	// Severity (0.0 to 1.0) indicates how severe the incident was.
	Severity float64

	// CreatedAt is when the incident was created/detected.
	CreatedAt time.Time
}

// Validate checks that the incident context is complete and valid.
func (ic *IncidentContext) Validate() error {
	if ic.ID == "" {
		return fmt.Errorf("incident ID cannot be empty")
	}
	if ic.ImpactService == "" {
		return fmt.Errorf("impact service cannot be empty")
	}
	if ic.ImpactSignal.ServiceName == "" {
		return fmt.Errorf("impact signal service name cannot be empty")
	}
	if ic.ImpactSignal.MetricName == "" {
		return fmt.Errorf("impact signal metric name cannot be empty")
	}
	if ic.ImpactSignal.Direction != "higher_is_worse" && ic.ImpactSignal.Direction != "lower_is_worse" {
		return fmt.Errorf("impact signal direction must be 'higher_is_worse' or 'lower_is_worse'")
	}
	if !ic.TimeBounds.IsValid() {
		return fmt.Errorf("time bounds are invalid or not properly ordered")
	}
	return nil
}

// String returns a string representation of the incident.
func (ic *IncidentContext) String() string {
	return fmt.Sprintf(
		"Incident{ID=%s, Service=%s, Metric=%s, TStart=%s, TPeak=%s, TEnd=%s, Severity=%.2f}",
		ic.ID,
		ic.ImpactService,
		ic.ImpactSignal.MetricName,
		ic.TimeBounds.TStart.Format(time.RFC3339),
		ic.TimeBounds.TPeak.Format(time.RFC3339),
		ic.TimeBounds.TEnd.Format(time.RFC3339),
		ic.Severity,
	)
}
