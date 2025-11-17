package rca

import (
	"time"
)

// SignalType categorizes the type of anomaly signal.
type SignalType string

const (
	// SignalTypeMetrics indicates an anomaly from metrics.
	SignalTypeMetrics SignalType = "metrics"
	// SignalTypeTraces indicates an anomaly from trace spans.
	SignalTypeTraces SignalType = "traces"
	// SignalTypeLogs indicates an anomaly from logs.
	SignalTypeLogs SignalType = "logs"
	// SignalTypeChange indicates a change in the system (e.g. deployment, config change).
	SignalTypeChange SignalType = "change"
)

// Severity represents the severity of an anomaly (0.0 to 1.0).
// 0.0 = not severe, 1.0 = critical.
type Severity float64

const (
	SeverityLow      Severity = 0.25
	SeverityMedium   Severity = 0.5
	SeverityHigh     Severity = 0.75
	SeverityCritical Severity = 1.0
)

// AnomalyEvent represents a normalized anomaly detected in the system.
// It captures anomalies from:
// - isolationforest processor (metrics/traces/logs flagged as anomalous)
// - basic error signals (spans with error status, logs with severity ERROR)
// - simple rule-based detections (e.g., error rate spikes)
type AnomalyEvent struct {
	// Core identifiers and timing.
	ID        string    // Unique ID for this anomaly event.
	Timestamp time.Time // When the anomaly was detected/occurred.

	// Service and component information.
	Service   string // Name of the service where anomaly was detected.
	Component string // Component within the service (e.g., "database", "cache", "api").

	// Signal classification.
	SignalType       SignalType // Type of signal: metrics, traces, logs, change.
	MetricOrField    string     // Name of the metric/field/span attribute being analyzed.
	FieldValue       float64    // The value of the field/metric when anomaly was detected.
	FieldValueString string     // String representation (for non-numeric fields).

	// Severity and confidence scores.
	Severity     Severity // Normalized severity (0..1).
	AnomalyScore float64  // Isolation Forest anomaly score (0..1) or other normalized score.
	Confidence   float64  // Confidence in the anomaly detection (0..1).

	// Error indicators from raw telemetry.
	// Used to correlate with different signal types.
	ErrorFlags map[string]bool // e.g., {"span_error": true, "log_error": true, "metric_threshold_exceeded": true}.

	// Contextual tags and attributes.
	Tags map[string]string // e.g., {"transaction_id": "...", "failure_reason": "...", "span_name": "..."}

	// Source information for tracing back to original telemetry.
	SourceID   string // ID of the original span, log record, or metric.
	SourceType string // "span", "log", "metric".

	// Isolation Forest specific attributes (when available).
	IForestClassification *bool              // true if flagged as anomaly by isolationforest.
	IForestScore          *float64           // Raw isolationforest anomaly score.
	IForestFeatures       map[string]float64 // Features used by isolationforest (if available).
}

// NewAnomalyEvent creates a new AnomalyEvent with defaults.
func NewAnomalyEvent(service, component string, signalType SignalType) *AnomalyEvent {
	return &AnomalyEvent{
		Timestamp:       time.Now().UTC(),
		Service:         service,
		Component:       component,
		SignalType:      signalType,
		ErrorFlags:      make(map[string]bool),
		Tags:            make(map[string]string),
		IForestFeatures: make(map[string]float64),
	}
}

// AnomalyEventFromIsolationForestSpan creates an AnomalyEvent from a span with
// isolationforest classification and score attributes.
func AnomalyEventFromIsolationForestSpan(
	spanID string,
	service string,
	spanName string,
	duration float64, // in milliseconds
	isAnomaly bool,
	anomalyScore float64,
	tags map[string]string,
) *AnomalyEvent {
	ae := NewAnomalyEvent(service, "span", SignalTypeTraces)
	ae.SourceID = spanID
	ae.SourceType = "span"
	ae.MetricOrField = spanName
	ae.FieldValue = duration
	ae.IForestClassification = &isAnomaly
	ae.IForestScore = &anomalyScore
	ae.AnomalyScore = anomalyScore

	// Set severity based on anomaly score and duration.
	if isAnomaly {
		if anomalyScore > 0.8 {
			ae.Severity = SeverityCritical
		} else if anomalyScore > 0.6 {
			ae.Severity = SeverityHigh
		} else if anomalyScore > 0.4 {
			ae.Severity = SeverityMedium
		} else {
			ae.Severity = SeverityLow
		}
	} else {
		ae.Severity = SeverityLow
	}

	ae.Confidence = anomalyScore // Use anomaly score as confidence

	// Copy tags.
	for k, v := range tags {
		ae.Tags[k] = v
	}

	return ae
}

// AnomalyEventFromMetric creates an AnomalyEvent from an isolationforest metric.
func AnomalyEventFromMetric(
	service string,
	metricName string,
	metricValue float64,
	isAnomaly bool,
	anomalyScore float64,
	attributes map[string]string,
) *AnomalyEvent {
	ae := NewAnomalyEvent(service, "metric", SignalTypeMetrics)
	ae.MetricOrField = metricName
	ae.FieldValue = metricValue
	ae.IForestClassification = &isAnomaly
	ae.IForestScore = &anomalyScore
	ae.AnomalyScore = anomalyScore

	// Set severity based on anomaly score.
	if isAnomaly {
		if anomalyScore > 0.8 {
			ae.Severity = SeverityCritical
		} else if anomalyScore > 0.6 {
			ae.Severity = SeverityHigh
		} else if anomalyScore > 0.4 {
			ae.Severity = SeverityMedium
		} else {
			ae.Severity = SeverityLow
		}
	} else {
		ae.Severity = SeverityLow
	}

	ae.Confidence = anomalyScore
	ae.SourceType = "metric"

	// Copy attributes.
	for k, v := range attributes {
		ae.Tags[k] = v
	}

	return ae
}

// AnomalyEventFromLog creates an AnomalyEvent from a log record with error severity
// or isolationforest flags.
func AnomalyEventFromLog(
	logID string,
	service string,
	logMessage string,
	severity string, // "ERROR", "WARN", etc.
	isAnomaly bool,
	anomalyScore *float64,
	attributes map[string]string,
) *AnomalyEvent {
	ae := NewAnomalyEvent(service, "logs", SignalTypeLogs)
	ae.SourceID = logID
	ae.SourceType = "log"
	ae.MetricOrField = "severity"
	ae.FieldValueString = severity

	// Set error flag based on severity.
	if severity == "ERROR" || severity == "FATAL" {
		ae.ErrorFlags["log_error"] = true
	}

	// Set severity based on log severity.
	switch severity {
	case "FATAL", "CRITICAL":
		ae.Severity = SeverityCritical
	case "ERROR":
		ae.Severity = SeverityHigh
	case "WARN":
		ae.Severity = SeverityMedium
	default:
		ae.Severity = SeverityLow
	}

	if isAnomaly && anomalyScore != nil {
		ae.IForestClassification = &isAnomaly
		ae.IForestScore = anomalyScore
		ae.AnomalyScore = *anomalyScore
		ae.Confidence = *anomalyScore
	} else if severity == "ERROR" || severity == "FATAL" {
		ae.Confidence = 0.9 // High confidence for error logs
	} else {
		ae.Confidence = 0.5
	}

	// Copy attributes.
	for k, v := range attributes {
		ae.Tags[k] = v
	}

	return ae
}

// AnomalyEventFromErrorSpan creates an AnomalyEvent from a span with error status.
// This handles basic error signals from trace spans.
func AnomalyEventFromErrorSpan(
	spanID string,
	service string,
	spanName string,
	errorMessage string,
	duration float64, // in milliseconds
	tags map[string]string,
) *AnomalyEvent {
	ae := NewAnomalyEvent(service, "span", SignalTypeTraces)
	ae.SourceID = spanID
	ae.SourceType = "span"
	ae.MetricOrField = spanName
	ae.FieldValue = duration
	ae.FieldValueString = errorMessage
	ae.Severity = SeverityHigh
	ae.Confidence = 0.95 // High confidence for explicit errors
	ae.ErrorFlags["span_error"] = true

	// Copy tags.
	for k, v := range tags {
		ae.Tags[k] = v
	}

	return ae
}
