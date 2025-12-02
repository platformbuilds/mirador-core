package weavstore

import (
	"reflect"
	"testing"
	"time"
)

// timeAlmostEqual checks if two times are equal, ignoring monotonic clock readings
// (which are lost during JSON serialization)
func timeAlmostEqual(t1, t2 time.Time) bool {
	return t1.UTC().Format(time.RFC3339Nano) == t2.UTC().Format(time.RFC3339Nano)
}

// failureSignalAlmostEqual checks if two failure signals are equal, ignoring monotonic clock readings
func failureSignalAlmostEqual(a, b *FailureSignal) bool {
	if a.SignalType != b.SignalType {
		return false
	}
	if a.MetricName != b.MetricName {
		return false
	}
	if a.Service != b.Service {
		return false
	}
	if a.Component != b.Component {
		return false
	}
	if !reflect.DeepEqual(a.Data, b.Data) {
		return false
	}
	return timeAlmostEqual(a.Timestamp, b.Timestamp)
}

func TestFailureSignalsConversionRoundTrip(t *testing.T) {
	now := time.Now()
	in := []FailureSignal{
		{
			SignalType: "metric",
			MetricName: "http_requests_failed_total",
			Service:    "api-gateway",
			Component:  "gateway",
			Data: map[string]any{
				"status_code": "STATUS_CODE_ERROR",
				"value":       155.0,
			},
			Timestamp: now,
		},
		{
			SignalType: "span",
			Service:    "kafka-producer",
			Component:  "producer",
			Data: map[string]any{
				"error_message": "Connection timeout",
				"trace_id":      "trace-001",
			},
			Timestamp: now.Add(-1 * time.Minute),
		},
	}

	maps := failureSignalsToMapArray(in)
	if maps == nil || len(maps) != len(in) {
		t.Fatalf("expected maps length %d got %d", len(in), len(maps))
	}

	out := propsToFailureSignals(maps)
	if len(out) != len(in) {
		t.Fatalf("expected output length %d, got %d", len(in), len(out))
	}

	// Use custom comparison that ignores monotonic clock differences
	for i := range in {
		if !failureSignalAlmostEqual(&in[i], &out[i]) {
			t.Fatalf("signal %d mismatch:\nexpected: %+v\nactual:   %+v", i, in[i], out[i])
		}
	}
}

func TestPropsToFailureSignalsHandlesInterfaceSlice(t *testing.T) {
	// Simulate the SDK decoding into []interface{} of map[string]interface{}
	raw := make([]interface{}, 2)
	raw[0] = map[string]interface{}{
		"signalType": "metric",
		"metricName": "response_time_ms",
		"service":    "api-gateway",
		"component":  "gateway",
		"data": map[string]interface{}{
			"value": 250.0,
		},
		"timestamp": time.Now().Format(time.RFC3339Nano),
	}
	raw[1] = map[string]interface{}{
		"signalType": "span",
		"service":    "kafka-producer",
		"component":  "producer",
		"data": map[string]interface{}{
			"error_message": "Timeout",
		},
		"timestamp": time.Now().Format(time.RFC3339Nano),
	}

	out := propsToFailureSignals(raw)
	if len(out) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(out))
	}
	if out[0].SignalType != "metric" || out[1].SignalType != "span" {
		t.Fatalf("unexpected signal types: %+v", out)
	}
	if out[0].MetricName != "response_time_ms" {
		t.Fatalf("unexpected metric name: %s", out[0].MetricName)
	}
}

func TestFailureRecordsEquality(t *testing.T) {
	now := time.Now()
	f1 := &FailureRecord{
		FailureUUID:        "a1b2c3d4-e5f6-5a7b-8c9d-0e1f2a3b4c5d",
		FailureID:          "kafka-producer-20251201-103000",
		TimeRange:          TimeRange{Start: now, End: now.Add(1 * time.Hour)},
		Services:           []string{"kafka-producer", "kafka-broker"},
		Components:         []string{"producer", "broker"},
		DetectorVersion:    "1.0",
		ConfidenceScore:    0.85,
		DetectionTimestamp: now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	f2 := &FailureRecord{
		FailureUUID:        "a1b2c3d4-e5f6-5a7b-8c9d-0e1f2a3b4c5d",
		FailureID:          "kafka-producer-20251201-103000",
		TimeRange:          TimeRange{Start: now, End: now.Add(1 * time.Hour)},
		Services:           []string{"kafka-producer", "kafka-broker"},
		Components:         []string{"producer", "broker"},
		DetectorVersion:    "1.0",
		ConfidenceScore:    0.85,
		DetectionTimestamp: now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if !failureEqual(f1, f2) {
		t.Fatalf("expected equal failure records")
	}

	// Modify one field
	f2.ConfidenceScore = 0.90
	if failureEqual(f1, f2) {
		t.Fatalf("expected unequal failure records after modification")
	}
}

func TestMapToFailureSignal(t *testing.T) {
	now := time.Now()
	m := map[string]any{
		"signalType": "metric",
		"metricName": "http_requests_failed_total",
		"service":    "api-gateway",
		"component":  "gateway",
		"data": map[string]any{
			"status_code": "STATUS_CODE_ERROR",
		},
		"timestamp": now.Format(time.RFC3339Nano),
	}

	sig := mapToFailureSignal(m)
	if sig.SignalType != "metric" {
		t.Fatalf("expected signal type 'metric', got '%s'", sig.SignalType)
	}
	if sig.MetricName != "http_requests_failed_total" {
		t.Fatalf("expected metric name 'http_requests_failed_total', got '%s'", sig.MetricName)
	}
	if sig.Service != "api-gateway" {
		t.Fatalf("expected service 'api-gateway', got '%s'", sig.Service)
	}
	if !sig.Timestamp.Equal(now) {
		t.Fatalf("timestamp mismatch")
	}
}
