package rca

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/logging"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// AnomalyEventConfig holds configuration for anomaly event mapping.
// Aligns with the otel-collector isolationforest processor config.
type AnomalyEventConfig struct {
	// IsolationForest attribute names (from otel-collector config).
	IForestClassificationAttribute string // e.g., "iforest_is_anomaly"
	IForestScoreAttribute          string // e.g., "iforest_anomaly_score"

	// Thresholds for rule-based anomaly detection.
	ErrorRateThreshold  float64 // e.g., 0.1 (10% error rate)
	LatencyP95Threshold float64 // in milliseconds
	LatencyP99Threshold float64 // in milliseconds
	MetricThreshold     float64 // generic threshold for metrics
}

// DefaultAnomalyEventConfig returns a default configuration
// aligned with the otel-collector config.
func DefaultAnomalyEventConfig() AnomalyEventConfig {
	return AnomalyEventConfig{
		// Use dotted classification attribute to match the OpenTelemetry
		// collector's isolationforest processor (common config uses
		// `iforest_is_anomaly`) while the score attribute was renamed to
		// `iforest_anomaly_score` (no dot) in recent updates.
		IForestClassificationAttribute: "iforest_is_anomaly",
		IForestScoreAttribute:          "iforest_anomaly_score",
		ErrorRateThreshold:             0.1,  // 10% error rate
		LatencyP95Threshold:            1000, // 1 second
		LatencyP99Threshold:            2000, // 2 seconds
		MetricThreshold:                2.0,  // 2 std devs or similar
	}
}

// AnomalyEventMapper provides helpers for converting raw telemetry
// (traces, logs, metrics) into normalized AnomalyEvents.
type AnomalyEventMapper struct {
	config AnomalyEventConfig
	logger logging.Logger
}

// NewAnomalyEventMapper creates a new mapper with the given config.
func NewAnomalyEventMapper(config AnomalyEventConfig, logger corelogger.Logger) *AnomalyEventMapper {
	return &AnomalyEventMapper{
		config: config,
		logger: logging.FromCoreLogger(logger),
	}
}

// SpanData represents simplified span data extracted from traces.
type SpanData struct {
	ID           string
	TraceID      string
	Service      string
	SpanName     string
	DurationMs   float64
	ErrorStatus  bool
	ErrorMessage string
	Timestamp    time.Time
	Attributes   map[string]interface{} // Includes isolationforest attributes
	Tags         map[string]string
}

// MapSpanToAnomalyEvents converts a span (or set of spans) into AnomalyEvents.
// It checks for both isolationforest classification and basic error signals.
func (aem *AnomalyEventMapper) MapSpanToAnomalyEvents(span SpanData) []*AnomalyEvent {
	var events []*AnomalyEvent

	// Check for isolationforest classification.
	if isAnomalyVal, ok := span.Attributes[aem.config.IForestClassificationAttribute].(bool); ok {
		if scoreVal, ok := span.Attributes[aem.config.IForestScoreAttribute].(float64); ok {
			event := AnomalyEventFromIsolationForestSpan(
				span.ID,
				span.Service,
				span.SpanName,
				span.DurationMs,
				isAnomalyVal,
				scoreVal,
				span.Tags,
			)
			event.Timestamp = span.Timestamp
			if span.ErrorStatus {
				event.ErrorFlags["span_error"] = true
			}
			events = append(events, event)
			return events // Return early if isolationforest data is present
		}
	}

	// Fallback: Check for basic error signals.
	if span.ErrorStatus {
		event := AnomalyEventFromErrorSpan(
			span.ID,
			span.Service,
			span.SpanName,
			span.ErrorMessage,
			span.DurationMs,
			span.Tags,
		)
		event.Timestamp = span.Timestamp
		events = append(events, event)
	}

	// Check for latency-based anomalies (simple rule).
	if span.DurationMs > aem.config.LatencyP99Threshold {
		event := NewAnomalyEvent(span.Service, "span", SignalTypeTraces)
		event.SourceID = span.ID
		event.SourceType = "span"
		event.MetricOrField = "span_duration_ms"
		event.FieldValue = span.DurationMs
		event.Severity = SeverityMedium
		event.Confidence = 0.7
		event.ErrorFlags["latency_high"] = true
		event.Timestamp = span.Timestamp
		for k, v := range span.Tags {
			event.Tags[k] = v
		}
		events = append(events, event)
	}

	return events
}

// LogData represents simplified log record data.
type LogData struct {
	ID         string
	Service    string
	Message    string
	Severity   string // "ERROR", "WARN", "INFO", etc.
	Timestamp  time.Time
	Attributes map[string]interface{} // Includes isolationforest attributes
	Tags       map[string]string
}

// MapLogToAnomalyEvent converts a log record into an AnomalyEvent (or nil if not anomalous).
func (aem *AnomalyEventMapper) MapLogToAnomalyEvent(log LogData) *AnomalyEvent {
	// Check for isolationforest classification first.
	if isAnomalyVal, ok := log.Attributes[aem.config.IForestClassificationAttribute].(bool); ok {
		scoreVal, ok := log.Attributes[aem.config.IForestScoreAttribute].(float64)
		if !ok {
			scoreVal = 0.5 // default if missing
		}

		event := AnomalyEventFromLog(
			log.ID,
			log.Service,
			log.Message,
			log.Severity,
			isAnomalyVal,
			&scoreVal,
			log.Tags,
		)
		event.Timestamp = log.Timestamp
		return event
	}

	// Fallback: Check severity-based rules.
	if log.Severity == "ERROR" || log.Severity == "FATAL" {
		event := AnomalyEventFromLog(
			log.ID,
			log.Service,
			log.Message,
			log.Severity,
			false, // not from isolationforest
			nil,
			log.Tags,
		)
		event.Timestamp = log.Timestamp
		return event
	}

	return nil // Not anomalous
}

// MetricData represents simplified metric data (single value at a point in time).
type MetricData struct {
	ID         string
	Service    string
	MetricName string
	Value      float64
	Timestamp  time.Time
	Attributes map[string]interface{} // Includes isolationforest attributes
	Tags       map[string]string
}

// MapMetricToAnomalyEvent converts a metric into an AnomalyEvent (or nil if not anomalous).
func (aem *AnomalyEventMapper) MapMetricToAnomalyEvent(metric MetricData) *AnomalyEvent {
	// Check for isolationforest classification.
	if isAnomalyVal, ok := metric.Attributes[aem.config.IForestClassificationAttribute].(bool); ok {
		if scoreVal, ok := metric.Attributes[aem.config.IForestScoreAttribute].(float64); ok {
			event := AnomalyEventFromMetric(
				metric.Service,
				metric.MetricName,
				metric.Value,
				isAnomalyVal,
				scoreVal,
				metric.Tags,
			)
			event.Timestamp = metric.Timestamp
			return event
		}
	}

	// Could add simple rule-based detection here if needed.
	// For now, return nil (no anomaly detected by rules).
	return nil
}

// RawTraceSpan represents a minimal trace span structure.
// Used when parsing raw telemetry that might come from different trace formats.
type RawTraceSpan struct {
	SpanID     string
	TraceID    string
	Service    string
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Status     string // "OK", "ERROR", etc.
	Attributes map[string]interface{}
}

// MapRawSpanToSpanData converts a RawTraceSpan to SpanData format.
func (aem *AnomalyEventMapper) MapRawSpanToSpanData(rawSpan RawTraceSpan) SpanData {
	durationMs := rawSpan.EndTime.Sub(rawSpan.StartTime).Seconds() * 1000

	tags := make(map[string]string)
	if service, ok := rawSpan.Attributes["service"].(string); ok {
		tags["service"] = service
	}
	if traceID, ok := rawSpan.Attributes["trace_id"].(string); ok {
		tags["trace_id"] = traceID
	}
	if transactionID, ok := rawSpan.Attributes["transaction_id"].(string); ok {
		tags["transaction_id"] = transactionID
	}

	return SpanData{
		ID:           rawSpan.SpanID,
		TraceID:      rawSpan.TraceID,
		Service:      rawSpan.Service,
		SpanName:     rawSpan.Name,
		DurationMs:   durationMs,
		ErrorStatus:  rawSpan.Status == "ERROR",
		ErrorMessage: fmt.Sprintf("Span status: %s", rawSpan.Status),
		Timestamp:    rawSpan.StartTime,
		Attributes:   rawSpan.Attributes,
		Tags:         tags,
	}
}

// AnomalyEventFilter can filter/reduce a set of AnomalyEvents based on criteria.
type AnomalyEventFilter struct {
	minSeverity   *Severity
	minConfidence *float64
	maxAge        *time.Duration
	services      map[string]bool
	signalTypes   map[SignalType]bool
}

// NewAnomalyEventFilter creates a new filter.
func NewAnomalyEventFilter() *AnomalyEventFilter {
	return &AnomalyEventFilter{
		services:    make(map[string]bool),
		signalTypes: make(map[SignalType]bool),
	}
}

// WithMinSeverity filters to events with severity >= minSeverity.
func (f *AnomalyEventFilter) WithMinSeverity(sev Severity) *AnomalyEventFilter {
	f.minSeverity = &sev
	return f
}

// WithMinConfidence filters to events with confidence >= minConfidence.
func (f *AnomalyEventFilter) WithMinConfidence(conf float64) *AnomalyEventFilter {
	f.minConfidence = &conf
	return f
}

// WithMaxAge filters to events with age <= maxAge.
func (f *AnomalyEventFilter) WithMaxAge(maxAge time.Duration) *AnomalyEventFilter {
	f.maxAge = &maxAge
	return f
}

// WithServices filters to specific services.
func (f *AnomalyEventFilter) WithServices(services ...string) *AnomalyEventFilter {
	for _, s := range services {
		f.services[s] = true
	}
	return f
}

// WithSignalTypes filters to specific signal types.
func (f *AnomalyEventFilter) WithSignalTypes(types ...SignalType) *AnomalyEventFilter {
	for _, t := range types {
		f.signalTypes[t] = true
	}
	return f
}

// Apply filters the provided events and returns matching ones.
func (f *AnomalyEventFilter) Apply(events []*AnomalyEvent) []*AnomalyEvent {
	var filtered []*AnomalyEvent

	now := time.Now().UTC()

	for _, e := range events {
		// Severity check.
		if f.minSeverity != nil && e.Severity < *f.minSeverity {
			continue
		}

		// Confidence check.
		if f.minConfidence != nil && e.Confidence < *f.minConfidence {
			continue
		}

		// Age check.
		if f.maxAge != nil && now.Sub(e.Timestamp) > *f.maxAge {
			continue
		}

		// Service filter.
		if len(f.services) > 0 && !f.services[e.Service] {
			continue
		}

		// Signal type filter.
		if len(f.signalTypes) > 0 && !f.signalTypes[e.SignalType] {
			continue
		}

		filtered = append(filtered, e)
	}

	return filtered
}

// SerializeToJSON converts an AnomalyEvent to JSON for storage/transmission.
func SerializeToJSON(event *AnomalyEvent) ([]byte, error) {
	return json.MarshalIndent(event, "", "  ")
}

// DeserializeFromJSON converts JSON back to an AnomalyEvent.
func DeserializeFromJSON(data []byte) (*AnomalyEvent, error) {
	var event AnomalyEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
