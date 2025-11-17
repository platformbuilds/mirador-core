package rca

import (
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestAnomalyEvent_NewAnomalyEvent(t *testing.T) {
	ae := NewAnomalyEvent("tps", "database", SignalTypeMetrics)

	if ae.Service != "tps" {
		t.Errorf("Expected service=tps, got %s", ae.Service)
	}

	if ae.Component != "database" {
		t.Errorf("Expected component=database, got %s", ae.Component)
	}

	if ae.SignalType != SignalTypeMetrics {
		t.Errorf("Expected SignalType=metrics, got %s", ae.SignalType)
	}

	if ae.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if ae.ErrorFlags == nil || len(ae.ErrorFlags) != 0 {
		t.Error("Expected empty ErrorFlags map")
	}

	if ae.Tags == nil || len(ae.Tags) != 0 {
		t.Error("Expected empty Tags map")
	}
}

func TestAnomalyEvent_FromIsolationForestSpan(t *testing.T) {
	tags := map[string]string{
		"transaction_id": "txn-123",
		"user_id":        "user-456",
	}

	ae := AnomalyEventFromIsolationForestSpan(
		"span-id-1",
		"tps",
		"ProcessTransaction",
		1500.5, // 1.5 seconds
		true,   // is anomaly
		0.85,   // anomaly score
		tags,
	)

	if ae.SourceID != "span-id-1" {
		t.Errorf("Expected SourceID=span-id-1, got %s", ae.SourceID)
	}

	if ae.Service != "tps" {
		t.Errorf("Expected service=tps, got %s", ae.Service)
	}

	if ae.MetricOrField != "ProcessTransaction" {
		t.Errorf("Expected MetricOrField=ProcessTransaction, got %s", ae.MetricOrField)
	}

	if ae.FieldValue != 1500.5 {
		t.Errorf("Expected FieldValue=1500.5, got %f", ae.FieldValue)
	}

	if ae.IForestClassification == nil || !*ae.IForestClassification {
		t.Error("Expected IForestClassification=true")
	}

	if ae.IForestScore == nil || *ae.IForestScore != 0.85 {
		t.Errorf("Expected IForestScore=0.85, got %v", ae.IForestScore)
	}

	if ae.Severity != Severity(SeverityCritical) {
		t.Errorf("Expected Severity=Critical (score 0.85 > 0.8), got %f", ae.Severity)
	}

	if ae.Confidence != 0.85 {
		t.Errorf("Expected Confidence=0.85, got %f", ae.Confidence)
	}

	if ae.Tags["transaction_id"] != "txn-123" {
		t.Errorf("Expected tag transaction_id=txn-123, got %s", ae.Tags["transaction_id"])
	}
}

func TestAnomalyEvent_FromMetric(t *testing.T) {
	attributes := map[string]string{
		"pod": "pod-1",
	}

	ae := AnomalyEventFromMetric(
		"cassandra",
		"query_latency_ms",
		2500.0,
		true,
		0.92,
		attributes,
	)

	if ae.Service != "cassandra" {
		t.Errorf("Expected service=cassandra, got %s", ae.Service)
	}

	if ae.MetricOrField != "query_latency_ms" {
		t.Errorf("Expected MetricOrField=query_latency_ms, got %s", ae.MetricOrField)
	}

	if ae.SignalType != SignalTypeMetrics {
		t.Errorf("Expected SignalType=metrics, got %s", ae.SignalType)
	}

	if ae.Severity != SeverityCritical {
		t.Errorf("Expected Severity=Critical (score 0.92), got %f", ae.Severity)
	}
}

func TestAnomalyEvent_FromErrorSpan(t *testing.T) {
	tags := map[string]string{
		"span_name": "QueryDatabase",
	}

	ae := AnomalyEventFromErrorSpan(
		"span-error-1",
		"tps",
		"QueryDatabase",
		"Connection timeout",
		5000.0,
		tags,
	)

	if ae.SourceID != "span-error-1" {
		t.Errorf("Expected SourceID=span-error-1, got %s", ae.SourceID)
	}

	if ae.SignalType != SignalTypeTraces {
		t.Errorf("Expected SignalType=traces, got %s", ae.SignalType)
	}

	if !ae.ErrorFlags["span_error"] {
		t.Error("Expected ErrorFlags[span_error]=true")
	}

	if ae.Severity != SeverityHigh {
		t.Errorf("Expected Severity=High for error span, got %f", ae.Severity)
	}

	if ae.Confidence != 0.95 {
		t.Errorf("Expected Confidence=0.95 for error span, got %f", ae.Confidence)
	}
}

func TestAnomalyEvent_FromLog(t *testing.T) {
	attributes := map[string]string{
		"logger": "app",
	}

	scoreVal := 0.7
	ae := AnomalyEventFromLog(
		"log-id-1",
		"api-gateway",
		"Request processing failed",
		"ERROR",
		true,
		&scoreVal,
		attributes,
	)

	if ae.SourceID != "log-id-1" {
		t.Errorf("Expected SourceID=log-id-1, got %s", ae.SourceID)
	}

	if ae.SignalType != SignalTypeLogs {
		t.Errorf("Expected SignalType=logs, got %s", ae.SignalType)
	}

	if ae.Severity != SeverityHigh {
		t.Errorf("Expected Severity=High for ERROR log, got %f", ae.Severity)
	}

	if !ae.ErrorFlags["log_error"] {
		t.Error("Expected ErrorFlags[log_error]=true for ERROR severity")
	}

	if ae.IForestScore == nil || *ae.IForestScore != 0.7 {
		t.Errorf("Expected IForestScore=0.7, got %v", ae.IForestScore)
	}
}

func TestAnomalyEventMapper_MapSpanToAnomalyEvents_WithIForestData(t *testing.T) {
	config := DefaultAnomalyEventConfig()
	log := logger.New("info")
	mapper := NewAnomalyEventMapper(config, log)

	span := SpanData{
		ID:          "span-1",
		Service:     "tps",
		SpanName:    "ProcessTransaction",
		DurationMs:  1200.0,
		ErrorStatus: false,
		Timestamp:   time.Now().UTC(),
		Attributes: map[string]interface{}{
			"iforest_is_anomaly":    true,
			"iforest_anomaly_score": 0.88,
		},
		Tags: map[string]string{
			"transaction_id": "txn-789",
		},
	}

	events := mapper.MapSpanToAnomalyEvents(span)
	if len(events) != 1 {
		t.Errorf("Expected 1 event from iforest span, got %d", len(events))
	}

	event := events[0]
	if event.AnomalyScore != 0.88 {
		t.Errorf("Expected AnomalyScore=0.88, got %f", event.AnomalyScore)
	}

	if event.Severity != Severity(SeverityCritical) {
		t.Errorf("Expected Severity=Critical (score 0.88 > 0.8), got %f", event.Severity)
	}
}

func TestAnomalyEventMapper_MapSpanToAnomalyEvents_WithError(t *testing.T) {
	config := DefaultAnomalyEventConfig()
	log := logger.New("info")
	mapper := NewAnomalyEventMapper(config, log)

	span := SpanData{
		ID:           "span-error",
		Service:      "tps",
		SpanName:     "QueryDatabase",
		DurationMs:   500.0,
		ErrorStatus:  true,
		ErrorMessage: "Database connection failed",
		Timestamp:    time.Now().UTC(),
		Attributes:   map[string]interface{}{},
		Tags:         map[string]string{},
	}

	events := mapper.MapSpanToAnomalyEvents(span)
	if len(events) < 1 {
		t.Fatalf("Expected at least 1 event from error span, got %d", len(events))
	}

	// First event should be the error span event.
	event := events[0]
	if !event.ErrorFlags["span_error"] {
		t.Error("Expected ErrorFlags[span_error]=true")
	}
}

func TestAnomalyEventMapper_MapSpanToAnomalyEvents_HighLatency(t *testing.T) {
	config := DefaultAnomalyEventConfig()
	log := logger.New("info")
	mapper := NewAnomalyEventMapper(config, log)

	// Duration exceeds P99 threshold (2000ms).
	span := SpanData{
		ID:         "span-slow",
		Service:    "cassandra",
		SpanName:   "Query",
		DurationMs: 3500.0,
		Timestamp:  time.Now().UTC(),
		Attributes: map[string]interface{}{},
		Tags:       map[string]string{},
	}

	events := mapper.MapSpanToAnomalyEvents(span)
	if len(events) < 1 {
		t.Fatalf("Expected at least 1 latency anomaly event, got %d", len(events))
	}

	// Find the latency event.
	var latencyEvent *AnomalyEvent
	for _, e := range events {
		if e.ErrorFlags["latency_high"] {
			latencyEvent = e
			break
		}
	}

	if latencyEvent == nil {
		t.Fatal("Expected latency anomaly event")
	}

	if latencyEvent.Severity != SeverityMedium {
		t.Errorf("Expected Severity=Medium for high latency, got %f", latencyEvent.Severity)
	}
}

func TestAnomalyEventMapper_MapLogToAnomalyEvent_ErrorLog(t *testing.T) {
	config := DefaultAnomalyEventConfig()
	log := logger.New("info")
	mapper := NewAnomalyEventMapper(config, log)

	logData := LogData{
		ID:         "log-1",
		Service:    "api-gateway",
		Message:    "Request handler failed",
		Severity:   "ERROR",
		Timestamp:  time.Now().UTC(),
		Attributes: map[string]interface{}{},
		Tags:       map[string]string{},
	}

	event := mapper.MapLogToAnomalyEvent(logData)
	if event == nil {
		t.Fatal("Expected non-nil event for ERROR log")
	}

	if event.Severity != SeverityHigh {
		t.Errorf("Expected Severity=High for ERROR log, got %f", event.Severity)
	}

	if !event.ErrorFlags["log_error"] {
		t.Error("Expected ErrorFlags[log_error]=true")
	}
}

func TestAnomalyEventMapper_MapLogToAnomalyEvent_NoAnomaly(t *testing.T) {
	config := DefaultAnomalyEventConfig()
	log := logger.New("info")
	mapper := NewAnomalyEventMapper(config, log)

	logData := LogData{
		ID:         "log-info",
		Service:    "api-gateway",
		Message:    "Request processed successfully",
		Severity:   "INFO",
		Timestamp:  time.Now().UTC(),
		Attributes: map[string]interface{}{},
		Tags:       map[string]string{},
	}

	event := mapper.MapLogToAnomalyEvent(logData)
	if event != nil {
		t.Error("Expected nil event for INFO log")
	}
}

func TestAnomalyEventMapper_MapMetricToAnomalyEvent_WithIForest(t *testing.T) {
	config := DefaultAnomalyEventConfig()
	log := logger.New("info")
	mapper := NewAnomalyEventMapper(config, log)

	metric := MetricData{
		ID:         "metric-1",
		Service:    "cassandra",
		MetricName: "disk_usage_percent",
		Value:      95.5,
		Timestamp:  time.Now().UTC(),
		Attributes: map[string]interface{}{
			"iforest_is_anomaly":    true,
			"iforest_anomaly_score": 0.79,
		},
		Tags: map[string]string{},
	}

	event := mapper.MapMetricToAnomalyEvent(metric)
	if event == nil {
		t.Fatal("Expected non-nil event for iforest metric")
	}

	if event.AnomalyScore != 0.79 {
		t.Errorf("Expected AnomalyScore=0.79, got %f", event.AnomalyScore)
	}

	if event.Severity != SeverityHigh {
		t.Errorf("Expected Severity=High for score 0.79, got %f", event.Severity)
	}
}

func TestAnomalyEventFilter_Apply(t *testing.T) {
	now := time.Now().UTC()

	events := []*AnomalyEvent{
		{
			Service:    "tps",
			Severity:   SeverityHigh,
			Confidence: 0.9,
			Timestamp:  now.Add(-2 * time.Minute),
		},
		{
			Service:    "cassandra",
			Severity:   SeverityMedium,
			Confidence: 0.7,
			Timestamp:  now.Add(-5 * time.Minute),
		},
		{
			Service:    "tps",
			Severity:   SeverityLow,
			Confidence: 0.4,
			Timestamp:  now.Add(-30 * time.Minute),
		},
	}

	// Filter by service.
	filter := NewAnomalyEventFilter().WithServices("tps")
	filtered := filter.Apply(events)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 events for service=tps, got %d", len(filtered))
	}

	// Filter by severity.
	filter = NewAnomalyEventFilter().WithMinSeverity(SeverityMedium)
	filtered = filter.Apply(events)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 events with Severity >= Medium, got %d", len(filtered))
	}

	// Filter by confidence.
	filter = NewAnomalyEventFilter().WithMinConfidence(0.7)
	filtered = filter.Apply(events)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 events with Confidence >= 0.7, got %d", len(filtered))
	}

	// Filter by max age.
	filter = NewAnomalyEventFilter().WithMaxAge(10 * time.Minute)
	filtered = filter.Apply(events)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 events within 10 minutes, got %d", len(filtered))
	}

	// Chained filters.
	filter = NewAnomalyEventFilter().
		WithServices("tps").
		WithMinSeverity(SeverityMedium)
	filtered = filter.Apply(events)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 event for service=tps AND Severity >= Medium, got %d", len(filtered))
	}
}

func TestAnomalyEventSerialization(t *testing.T) {
	original := &AnomalyEvent{
		ID:         "test-id",
		Service:    "tps",
		Component:  "database",
		SignalType: SignalTypeMetrics,
		Severity:   SeverityHigh,
		Confidence: 0.85,
	}

	// Serialize to JSON.
	jsonBytes, err := SerializeToJSON(original)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Fatal("Expected non-empty JSON")
	}

	// Deserialize from JSON.
	deserialized, err := DeserializeFromJSON(jsonBytes)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if deserialized.ID != original.ID {
		t.Errorf("Expected ID=%s after deserialization, got %s", original.ID, deserialized.ID)
	}

	if deserialized.Service != original.Service {
		t.Errorf("Expected Service=%s after deserialization, got %s", original.Service, deserialized.Service)
	}

	if deserialized.Severity != original.Severity {
		t.Errorf("Expected Severity=%f after deserialization, got %f", original.Severity, deserialized.Severity)
	}
}

func TestAnomalyEvent_SeverityMapping(t *testing.T) {
	tests := []struct {
		score    float64
		expected Severity
	}{
		{0.1, SeverityLow},
		{0.5, SeverityMedium},
		{0.7, SeverityHigh},
		{0.9, SeverityCritical},
	}

	for _, tt := range tests {
		ae := AnomalyEventFromIsolationForestSpan(
			"span-1",
			"service",
			"op",
			100.0,
			true,
			tt.score,
			map[string]string{},
		)

		if ae.Severity != tt.expected {
			t.Errorf("Score %.1f: expected Severity=%f, got %f", tt.score, tt.expected, ae.Severity)
		}
	}
}
