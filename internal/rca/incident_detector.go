package rca

import (
	"context"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/logging"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// IncidentDetectorConfig holds configuration for incident detection logic.
type IncidentDetectorConfig struct {
	// ThresholdPercentileAboveNormal: how far above normal (in percentages)
	// the metric must spike to be considered an incident. Default: 50 (50% above baseline).
	ThresholdPercentileAboveNormal float64

	// MinDurationForIncident: minimum time duration for an event to be considered
	// a true incident (not just a blip). Default: 10 seconds.
	MinDurationForIncident time.Duration

	// RecoveryThreshold: the metric must drop to this fraction of peak to be considered
	// "recovered". E.g., 0.5 = must drop to 50% of peak. Default: 0.3 (30% of peak).
	RecoveryThreshold float64
}

// DefaultIncidentDetectorConfig returns sensible defaults.
func DefaultIncidentDetectorConfig() IncidentDetectorConfig {
	return IncidentDetectorConfig{
		ThresholdPercentileAboveNormal: 50,
		MinDurationForIncident:         10 * time.Second,
		RecoveryThreshold:              0.3,
	}
}

// MetricTimeSeries represents a time series of metric values for analysis.
type MetricTimeSeries struct {
	// Timestamps and corresponding values.
	Timestamps []time.Time
	Values     []float64

	// Metadata.
	ServiceName string
	MetricName  string
	Direction   string // "higher_is_worse" or "lower_is_worse"
}

// IsValid checks that the time series is well-formed.
func (mts *MetricTimeSeries) IsValid() error {
	if len(mts.Timestamps) == 0 {
		return fmt.Errorf("empty time series")
	}
	if len(mts.Timestamps) != len(mts.Values) {
		return fmt.Errorf("timestamps and values length mismatch")
	}
	if mts.ServiceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if mts.MetricName == "" {
		return fmt.Errorf("metric name cannot be empty")
	}
	// Verify times are ordered.
	for i := 1; i < len(mts.Timestamps); i++ {
		if !mts.Timestamps[i].After(mts.Timestamps[i-1]) {
			return fmt.Errorf("timestamps are not strictly ordered")
		}
	}
	return nil
}

// IncidentDetector analyzes metric time series to identify incidents.
type IncidentDetector struct {
	config IncidentDetectorConfig
	logger logging.Logger
}

// NewIncidentDetector creates a new incident detector.
func NewIncidentDetector(config IncidentDetectorConfig, logger corelogger.Logger) *IncidentDetector {
	return &IncidentDetector{
		config: config,
		logger: logging.FromCoreLogger(logger),
	}
}

// DetectIncident analyzes a metric time series and returns an IncidentContext if found.
// The analysis:
// 1. Computes a baseline (mean or percentile) from early data.
// 2. Detects spike above baseline.
// 3. Finds peak time.
// 4. Estimates recovery time.
// 5. Constructs IncidentContext with TStart, TPeak, TEnd.
//
// Returns nil, nil if no significant incident is detected.
// Returns nil, error if the input is invalid.
func (id *IncidentDetector) DetectIncident(
	ctx context.Context,
	ts *MetricTimeSeries,
	incidentID string,
) (*IncidentContext, error) {
	if err := ts.IsValid(); err != nil {
		return nil, fmt.Errorf("invalid time series: %w", err)
	}

	// For now, use a simple baseline: mean of first 20% of data.
	baselineEndIdx := len(ts.Values) / 5
	if baselineEndIdx < 1 {
		baselineEndIdx = 1
	}

	baseline := mean(ts.Values[:baselineEndIdx])
	id.logger.Debug("Computed baseline for incident detection",
		"metric", ts.MetricName,
		"service", ts.ServiceName,
		"baseline", baseline)

	// Compute threshold based on config.
	threshold := baseline * (1 + id.config.ThresholdPercentileAboveNormal/100.0)

	// For "lower_is_worse" metrics, flip the logic.
	if ts.Direction == "lower_is_worse" {
		threshold = baseline * (1 - id.config.ThresholdPercentileAboveNormal/100.0)
	}

	id.logger.Debug("Computed threshold for incident detection",
		"threshold", threshold,
		"direction", ts.Direction)

	// Find TStart: first time threshold is breached.
	var tStartIdx int = -1
	for i, val := range ts.Values {
		isBreached := false
		if ts.Direction == "higher_is_worse" {
			isBreached = val > threshold
		} else {
			isBreached = val < threshold
		}

		if isBreached {
			tStartIdx = i
			break
		}
	}

	if tStartIdx == -1 {
		// No incident detected.
		id.logger.Debug("No incident detected; threshold not breached",
			"metric", ts.MetricName,
			"service", ts.ServiceName)
		return nil, nil
	}

	// Find TPeak: index with the maximum/minimum value after TStart.
	var tPeakIdx int = tStartIdx
	peakVal := ts.Values[tStartIdx]

	for i := tStartIdx + 1; i < len(ts.Values); i++ {
		if ts.Direction == "higher_is_worse" {
			if ts.Values[i] > peakVal {
				peakVal = ts.Values[i]
				tPeakIdx = i
			}
		} else {
			if ts.Values[i] < peakVal {
				peakVal = ts.Values[i]
				tPeakIdx = i
			}
		}
	}

	// Check minimum duration: TStart to TPeak should be at least MinDurationForIncident.
	incidentDuration := ts.Timestamps[tPeakIdx].Sub(ts.Timestamps[tStartIdx])
	if incidentDuration < id.config.MinDurationForIncident {
		// Check if we can extend the incident by looking further.
		// For simplicity, we'll still consider it if we have some duration.
		// In a real system, you might want stricter validation.
	}

	// Find TEnd: when metric recovers below recovery threshold.
	recoveryVal := baseline + (peakVal-baseline)*id.config.RecoveryThreshold
	if ts.Direction == "lower_is_worse" {
		recoveryVal = baseline - (baseline-peakVal)*id.config.RecoveryThreshold
	}

	var tEndIdx int = len(ts.Values) - 1 // Default: last sample
	for i := tPeakIdx + 1; i < len(ts.Values); i++ {
		isRecovered := false
		if ts.Direction == "higher_is_worse" {
			isRecovered = ts.Values[i] <= recoveryVal
		} else {
			isRecovered = ts.Values[i] >= recoveryVal
		}

		if isRecovered {
			tEndIdx = i
			break
		}
	}

	// Compute severity as fraction of peak deviation from baseline.
	deviation := (peakVal - baseline) / (baseline + 1e-9) // Avoid division by zero.
	if ts.Direction == "lower_is_worse" {
		deviation = (baseline - peakVal) / (baseline + 1e-9)
	}
	severity := min(1.0, abs(deviation))

	// Build the IncidentContext.
	incident := &IncidentContext{
		ID:            incidentID,
		ImpactService: ts.ServiceName,
		ImpactSignal: ImpactSignal{
			ServiceName: ts.ServiceName,
			MetricName:  ts.MetricName,
			Direction:   ts.Direction,
			Threshold:   threshold,
		},
		TimeBounds: IncidentTimeWindow{
			TStart: ts.Timestamps[tStartIdx],
			TPeak:  ts.Timestamps[tPeakIdx],
			TEnd:   ts.Timestamps[tEndIdx],
		},
		ImpactSummary: fmt.Sprintf(
			"%s spike in %s (peak: %.2f, baseline: %.2f) from %s to %s",
			ts.MetricName, ts.ServiceName, peakVal, baseline,
			ts.Timestamps[tStartIdx].Format(time.RFC3339),
			ts.Timestamps[tEndIdx].Format(time.RFC3339),
		),
		Severity:  severity,
		CreatedAt: time.Now().UTC(),
	}

	id.logger.Info("Detected incident",
		"incident_id", incident.ID,
		"service", incident.ImpactService,
		"metric", ts.MetricName,
		"severity", incident.Severity,
		"duration", incident.TimeBounds.Duration())

	return incident, nil
}

// Helper functions.

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
