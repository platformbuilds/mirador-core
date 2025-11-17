package rca

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// ========================
// IncidentDetector Tests
// ========================

func TestIncidentDetector_DetectIncident_HigherIsWorse(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")

	// Create a synthetic time series with a spike.
	// Baseline: ~50 for first 20% of points.
	// Spike: rises to 150 around 40% mark, then recovers.
	baseTime := time.Now()
	numPoints := 100

	timestamps := make([]time.Time, numPoints)
	values := make([]float64, numPoints)

	for i := 0; i < numPoints; i++ {
		timestamps[i] = baseTime.Add(time.Duration(i) * time.Second)

		if i < 20 {
			// Baseline
			values[i] = 50.0
		} else if i < 30 {
			// Rise to peak
			values[i] = 50.0 + float64(i-20)*5.0 // Ramp up
		} else if i < 50 {
			// Peak and decline
			if i < 40 {
				values[i] = 100.0 + float64(40-i)*2.5 // Stay elevated
			} else {
				values[i] = 100.0 - float64(i-40)*2.5 // Decline
			}
		} else {
			// Recovery
			values[i] = 50.0 + (100.0-50.0)*float64(50-i)/10.0 // Back to baseline
		}
	}

	ts := &MetricTimeSeries{
		Timestamps:  timestamps,
		Values:      values,
		ServiceName: "tps",
		MetricName:  "error_rate",
		Direction:   "higher_is_worse",
	}

	detector := NewIncidentDetector(DefaultIncidentDetectorConfig(), log)
	incident, err := detector.DetectIncident(ctx, ts, "inc-1")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if incident == nil {
		t.Fatal("Expected incident to be detected, got nil")
	}

	// Verify basic properties.
	if incident.ID != "inc-1" {
		t.Errorf("Expected ID=inc-1, got %s", incident.ID)
	}

	if incident.ImpactService != "tps" {
		t.Errorf("Expected service=tps, got %s", incident.ImpactService)
	}

	// Verify time ordering.
	if !incident.TimeBounds.TStart.Before(incident.TimeBounds.TPeak) {
		t.Error("Expected TStart < TPeak")
	}

	if !incident.TimeBounds.TPeak.Before(incident.TimeBounds.TEnd) {
		t.Error("Expected TPeak < TEnd")
	}

	// Severity should be significant (spike is large).
	if incident.Severity < 0.5 {
		t.Errorf("Expected significant severity, got %f", incident.Severity)
	}

	t.Logf("Detected incident: %s", incident.String())
}

func TestIncidentDetector_DetectIncident_LowerIsWorse(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")

	// Create time series with throughput drop (lower is worse).
	baseTime := time.Now()
	numPoints := 100

	timestamps := make([]time.Time, numPoints)
	values := make([]float64, numPoints)

	for i := 0; i < numPoints; i++ {
		timestamps[i] = baseTime.Add(time.Duration(i) * 500 * time.Millisecond)

		if i < 20 {
			values[i] = 1000.0 // High baseline
		} else if i < 50 {
			// Drop to 250 (well below the 50% threshold of 500)
			values[i] = 1000.0 - float64(i-20)*25.0
		} else {
			values[i] = 250.0 + float64(i-50)*15.0 // Recovery
		}
	}

	ts := &MetricTimeSeries{
		Timestamps:  timestamps,
		Values:      values,
		ServiceName: "api-gateway",
		MetricName:  "requests_per_sec",
		Direction:   "lower_is_worse",
	}

	detector := NewIncidentDetector(DefaultIncidentDetectorConfig(), log)
	incident, err := detector.DetectIncident(ctx, ts, "inc-2")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if incident == nil {
		t.Fatal("Expected incident to be detected, got nil")
	}

	if incident.ID != "inc-2" {
		t.Errorf("Expected ID=inc-2, got %s", incident.ID)
	}

	t.Logf("Detected incident (lower_is_worse): %s", incident.String())
}

func TestIncidentDetector_DetectIncident_NoIncident(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")

	// Create stable time series (no anomaly).
	baseTime := time.Now()
	numPoints := 100

	timestamps := make([]time.Time, numPoints)
	values := make([]float64, numPoints)

	for i := 0; i < numPoints; i++ {
		timestamps[i] = baseTime.Add(time.Duration(i) * time.Second)
		values[i] = 50.0 // Stable
	}

	ts := &MetricTimeSeries{
		Timestamps:  timestamps,
		Values:      values,
		ServiceName: "cache",
		MetricName:  "hit_rate",
		Direction:   "higher_is_worse",
	}

	detector := NewIncidentDetector(DefaultIncidentDetectorConfig(), log)
	incident, err := detector.DetectIncident(ctx, ts, "inc-3")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if incident != nil {
		t.Errorf("Expected no incident, but detected: %s", incident.String())
	}
}

func TestIncidentDetector_DetectIncident_InvalidTimeSeries(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")
	detector := NewIncidentDetector(DefaultIncidentDetectorConfig(), log)

	// Empty time series.
	ts := &MetricTimeSeries{
		Timestamps:  []time.Time{},
		Values:      []float64{},
		ServiceName: "tps",
		MetricName:  "error_rate",
		Direction:   "higher_is_worse",
	}

	_, err := detector.DetectIncident(ctx, ts, "inc-4")
	if err == nil {
		t.Error("Expected error for empty time series")
	}

	// Length mismatch.
	ts = &MetricTimeSeries{
		Timestamps:  []time.Time{time.Now()},
		Values:      []float64{},
		ServiceName: "tps",
		MetricName:  "error_rate",
		Direction:   "higher_is_worse",
	}

	_, err = detector.DetectIncident(ctx, ts, "inc-5")
	if err == nil {
		t.Error("Expected error for length mismatch")
	}
}

func TestMetricTimeSeries_Validation(t *testing.T) {
	baseTime := time.Now()

	tests := []struct {
		name    string
		ts      *MetricTimeSeries
		isValid bool
	}{
		{
			name: "valid time series",
			ts: &MetricTimeSeries{
				Timestamps:  []time.Time{baseTime, baseTime.Add(1 * time.Second)},
				Values:      []float64{50.0, 60.0},
				ServiceName: "tps",
				MetricName:  "error_rate",
				Direction:   "higher_is_worse",
			},
			isValid: true,
		},
		{
			name: "empty timestamps",
			ts: &MetricTimeSeries{
				Timestamps:  []time.Time{},
				Values:      []float64{},
				ServiceName: "tps",
				MetricName:  "error_rate",
			},
			isValid: false,
		},
		{
			name: "length mismatch",
			ts: &MetricTimeSeries{
				Timestamps:  []time.Time{baseTime, baseTime.Add(1 * time.Second)},
				Values:      []float64{50.0},
				ServiceName: "tps",
				MetricName:  "error_rate",
			},
			isValid: false,
		},
		{
			name: "unordered timestamps",
			ts: &MetricTimeSeries{
				Timestamps:  []time.Time{baseTime.Add(2 * time.Second), baseTime},
				Values:      []float64{50.0, 60.0},
				ServiceName: "tps",
				MetricName:  "error_rate",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ts.IsValid()
			if tt.isValid && err != nil {
				t.Errorf("Expected valid, got error: %v", err)
			}
			if !tt.isValid && err == nil {
				t.Errorf("Expected invalid, got no error")
			}
		})
	}
}

func TestIncidentDetectorConfig_Defaults(t *testing.T) {
	cfg := DefaultIncidentDetectorConfig()

	if cfg.ThresholdPercentileAboveNormal != 50 {
		t.Errorf("Expected default threshold=50, got %f", cfg.ThresholdPercentileAboveNormal)
	}

	if cfg.MinDurationForIncident != 10*time.Second {
		t.Errorf("Expected default min duration=10s, got %v", cfg.MinDurationForIncident)
	}

	if cfg.RecoveryThreshold != 0.3 {
		t.Errorf("Expected default recovery threshold=0.3, got %f", cfg.RecoveryThreshold)
	}
}
