package rca

import (
	"testing"
	"time"
)

// ========================
// TimeRing Tests
// ========================

func TestTimeRing_AssignTimeRing_BeforePeak(t *testing.T) {
	cfg := DefaultTimeRingConfig()
	peakTime := time.Now()

	tests := []struct {
		name         string
		eventTime    time.Time
		expectedRing TimeRing
		desc         string
	}{
		{
			name:         "within R1 (immediate)",
			eventTime:    peakTime.Add(-2 * time.Second),
			expectedRing: RingImmediate,
			desc:         "2s before peak should be R1",
		},
		{
			name:         "within R2 (short)",
			eventTime:    peakTime.Add(-15 * time.Second),
			expectedRing: RingShort,
			desc:         "15s before peak should be R2",
		},
		{
			name:         "within R3 (medium)",
			eventTime:    peakTime.Add(-1 * time.Minute),
			expectedRing: RingMedium,
			desc:         "1m before peak should be R3",
		},
		{
			name:         "within R4 (long)",
			eventTime:    peakTime.Add(-5 * time.Minute),
			expectedRing: RingLong,
			desc:         "5m before peak should be R4",
		},
		{
			name:         "out of scope",
			eventTime:    peakTime.Add(-20 * time.Minute),
			expectedRing: RingOutOfScope,
			desc:         "20m before peak should be out of scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ring := AssignTimeRing(peakTime, tt.eventTime, cfg)
			if ring != tt.expectedRing {
				t.Errorf("%s: expected %s, got %s", tt.desc, tt.expectedRing, ring)
			}
		})
	}
}

func TestTimeRing_AssignTimeRing_AfterPeak(t *testing.T) {
	cfg := DefaultTimeRingConfig()
	peakTime := time.Now()

	// Event after peak should be out of scope by default, unless within TimeAfterPeak.
	eventTimeShortAfter := peakTime.Add(10 * time.Second)
	ring := AssignTimeRing(peakTime, eventTimeShortAfter, cfg)
	if ring != RingImmediate {
		// With AllowEventsAfterPeak=true (default), should be R1.
		t.Errorf("Event 10s after peak with AllowEventsAfterPeak=true: expected R1, got %s", ring)
	}

	eventTimeLongAfter := peakTime.Add(2 * time.Minute)
	ring = AssignTimeRing(peakTime, eventTimeLongAfter, cfg)
	if ring != RingOutOfScope {
		t.Errorf("Event 2m after peak: expected out of scope, got %s", ring)
	}
}

func TestTimeRing_AssignTimeRing_DisallowAfterPeak(t *testing.T) {
	cfg := DefaultTimeRingConfig()
	cfg.AllowEventsAfterPeak = false
	peakTime := time.Now()

	eventTimeAfter := peakTime.Add(1 * time.Second)
	ring := AssignTimeRing(peakTime, eventTimeAfter, cfg)
	if ring != RingOutOfScope {
		t.Errorf("Event after peak with AllowEventsAfterPeak=false: expected out of scope, got %s", ring)
	}
}

func TestTimeRing_Priority(t *testing.T) {
	tests := []struct {
		ring     TimeRing
		priority int
	}{
		{RingImmediate, 0},
		{RingShort, 1},
		{RingMedium, 2},
		{RingLong, 3},
		{RingOutOfScope, 999},
	}

	for _, tt := range tests {
		if tt.ring.Priority() != tt.priority {
			t.Errorf("Ring %s: expected priority %d, got %d", tt.ring, tt.priority, tt.ring.Priority())
		}
	}
}

func TestTimeRing_IsInScope(t *testing.T) {
	inScopeRings := []TimeRing{RingImmediate, RingShort, RingMedium, RingLong}
	for _, ring := range inScopeRings {
		if !ring.IsInScope() {
			t.Errorf("Ring %s should be in scope", ring)
		}
	}

	if RingOutOfScope.IsInScope() {
		t.Error("RingOutOfScope should not be in scope")
	}
}

// ========================
// IncidentContext Tests
// ========================

func TestIncidentContext_Validation(t *testing.T) {
	baseTime := time.Now()

	tests := []struct {
		name    string
		ctx     *IncidentContext
		isValid bool
		desc    string
	}{
		{
			name: "valid incident",
			ctx: &IncidentContext{
				ID:            "inc-1",
				ImpactService: "tps",
				ImpactSignal: ImpactSignal{
					ServiceName: "tps",
					MetricName:  "error_rate",
					Direction:   "higher_is_worse",
					Threshold:   0.1,
				},
				TimeBounds: IncidentTimeWindow{
					TStart: baseTime,
					TPeak:  baseTime.Add(1 * time.Minute),
					TEnd:   baseTime.Add(2 * time.Minute),
				},
				Severity:  0.8,
				CreatedAt: time.Now(),
			},
			isValid: true,
			desc:    "valid incident should pass validation",
		},
		{
			name: "missing ID",
			ctx: &IncidentContext{
				ID:            "",
				ImpactService: "tps",
				ImpactSignal: ImpactSignal{
					ServiceName: "tps",
					MetricName:  "error_rate",
					Direction:   "higher_is_worse",
				},
			},
			isValid: false,
			desc:    "missing ID should fail",
		},
		{
			name: "invalid direction",
			ctx: &IncidentContext{
				ID:            "inc-1",
				ImpactService: "tps",
				ImpactSignal: ImpactSignal{
					ServiceName: "tps",
					MetricName:  "error_rate",
					Direction:   "invalid_direction",
				},
			},
			isValid: false,
			desc:    "invalid direction should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if tt.isValid && err != nil {
				t.Errorf("%s: expected valid but got error: %v", tt.desc, err)
			}
			if !tt.isValid && err == nil {
				t.Errorf("%s: expected invalid but got no error", tt.desc)
			}
		})
	}
}

func TestIncidentTimeWindow_Duration(t *testing.T) {
	start := time.Now()
	peak := start.Add(1 * time.Minute)
	end := start.Add(3 * time.Minute)

	itw := IncidentTimeWindow{
		TStart: start,
		TPeak:  peak,
		TEnd:   end,
	}

	expected := 3 * time.Minute
	if itw.Duration() != expected {
		t.Errorf("Expected duration %v, got %v", expected, itw.Duration())
	}
}

// ========================
// EnrichedAnomalyEvent Tests
// ========================

func TestEnrichedAnomalyEvent_IsCandidate(t *testing.T) {
	baseAnomaly := &AnomalyEvent{
		ID:       "anom-1",
		Service:  "tps",
		Severity: SeverityHigh,
	}

	tests := []struct {
		name        string
		ring        TimeRing
		isCandidate bool
	}{
		{
			name:        "in-scope immediate",
			ring:        RingImmediate,
			isCandidate: true,
		},
		{
			name:        "in-scope long",
			ring:        RingLong,
			isCandidate: true,
		},
		{
			name:        "out of scope",
			ring:        RingOutOfScope,
			isCandidate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eae := NewEnrichedAnomalyEvent(baseAnomaly, tt.ring, DirectionUpstream, 1, "tps")
			if eae.IsCandidate() != tt.isCandidate {
				t.Errorf("%s: expected candidate=%v, got %v", tt.name, tt.isCandidate, eae.IsCandidate())
			}
		})
	}
}

func TestEnrichedAnomalyEvent_IsHighPriority(t *testing.T) {
	tests := []struct {
		name           string
		ring           TimeRing
		direction      GraphDirection
		severity       Severity
		isHighPriority bool
	}{
		{
			name:           "immediate + upstream + high",
			ring:           RingImmediate,
			direction:      DirectionUpstream,
			severity:       SeverityHigh,
			isHighPriority: true,
		},
		{
			name:           "short + same + critical",
			ring:           RingShort,
			direction:      DirectionSame,
			severity:       SeverityCritical,
			isHighPriority: true,
		},
		{
			name:           "medium + upstream + high",
			ring:           RingMedium,
			direction:      DirectionUpstream,
			severity:       SeverityHigh,
			isHighPriority: false, // Not R1 or R2
		},
		{
			name:           "immediate + downstream + high",
			ring:           RingImmediate,
			direction:      DirectionDownstream,
			severity:       SeverityHigh,
			isHighPriority: false, // Not upstream or same
		},
		{
			name:           "immediate + upstream + low",
			ring:           RingImmediate,
			direction:      DirectionUpstream,
			severity:       SeverityLow,
			isHighPriority: false, // Low severity
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseAnomaly := NewAnomalyEvent("tps", "database", SignalTypeMetrics)
			baseAnomaly.Severity = tt.severity

			eae := NewEnrichedAnomalyEvent(baseAnomaly, tt.ring, tt.direction, 1, "tps")
			if eae.IsHighPriority() != tt.isHighPriority {
				t.Errorf("%s: expected high_priority=%v, got %v", tt.name, tt.isHighPriority, eae.IsHighPriority())
			}
		})
	}
}
