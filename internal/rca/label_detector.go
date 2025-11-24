package rca

import (
	"fmt"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MetricsLabelDetector detects which label keys are available in metrics data at runtime.
// It handles variation in label naming conventions and provides fallback logic.
type MetricsLabelDetector struct {
	logger    logger.Logger
	labelsCfg config.LabelSchemaConfig
}

// NewMetricsLabelDetector creates a new detector.
func NewMetricsLabelDetector(logger logger.Logger, labelsCfg config.LabelSchemaConfig) *MetricsLabelDetector {
	return &MetricsLabelDetector{
		logger:    logger,
		labelsCfg: labelsCfg,
	}
}

// LabelMapping defines how to detect and use different label conventions.
type LabelMapping struct {
	// StandardName is the preferred/canonical label key (for example, a stable
	// canonical label used by the engine)
	StandardName string

	// Alternatives are alternative names that may be used in incoming telemetry
	// payloads (configured via EngineConfig or discovery mechanisms).
	Alternatives []string
}

// StandardLabelMappings returns the standard OpenTelemetry label conventions.
func StandardLabelMappings() map[string]LabelMapping {
	return map[string]LabelMapping{
		"service_name": {
			StandardName: "service_name",
			// Alternatives for service are configurable at runtime via EngineConfig.Labels.Service.
			// Keep an empty placeholder here; detectors should override this with configured values.
			Alternatives: []string{},
		},
		"span_name": {
			StandardName: "span_name",
			Alternatives: []string{"span.name", "operation_name", "operation"},
		},
		"span_kind": {
			StandardName: "span_kind",
			Alternatives: []string{"span.kind", "kind"},
		},
		"status_code": {
			StandardName: "status_code",
			Alternatives: []string{"status.code", "http_status", "status"},
		},
	}
}

// DetectAvailableLabels checks which standard labels are present in a given set of label keys.
// It returns:
// - availableLabels: map of standard label -> actual label key found in data
// - missingLabels: list of standard labels not found
// - diagnostics: warnings about missing labels
func (mld *MetricsLabelDetector) DetectAvailableLabels(
	providedLabels map[string]bool, // labels present in the data (key -> exists)
	diagnostics *RCADiagnostics,
) map[string]string { // standardLabel -> actualLabel
	mappings := StandardLabelMappings()
	// If service label candidates are provided via config, prefer them for detection
	if len(mld.labelsCfg.Service) > 0 {
		// copy to avoid mutating the returned map's slice shared elsewhere
		svcAlts := make([]string, len(mld.labelsCfg.Service))
		copy(svcAlts, mld.labelsCfg.Service)
		if mapping, ok := mappings["service_name"]; ok {
			mapping.Alternatives = svcAlts
			mappings["service_name"] = mapping
		} else {
			mappings["service_name"] = LabelMapping{StandardName: "service_name", Alternatives: svcAlts}
		}
	}
	availableLabels := make(map[string]string)

	for standardName, mapping := range mappings {
		// Try standard name first
		if providedLabels[mapping.StandardName] {
			availableLabels[standardName] = mapping.StandardName
			continue
		}

		// Try alternatives
		found := false
		for _, alt := range mapping.Alternatives {
			if providedLabels[alt] {
				availableLabels[standardName] = alt
				mld.logger.Debug("Using alternative label",
					"standard_name", standardName,
					"alternative", alt)
				found = true
				break
			}
		}

		if !found && diagnostics != nil {
			// Record missing semantic role (e.g. "service") rather than raw wire keys
			diagnostics.AddMissingLabel(standardName)
			diagnostics.AddReducedAccuracyReason(
				fmt.Sprintf("Standard label '%s' (or alternatives %v) not found in metrics; RCA may be less accurate", standardName, mapping.Alternatives))
		}
	}

	if diagnostics != nil && len(availableLabels) < len(mappings) {
		mld.logger.Warn("Some standard labels missing from metrics",
			"available", len(availableLabels),
			"total_standard", len(mappings))
	}

	return availableLabels
}

// DetectExtraDimensionLabels checks which user-configured extra dimensions are available.
// It returns:
// - availableDimensions: map of dimension -> actual label key found in data
// - diagnostics: records which dimensions were detected
func (mld *MetricsLabelDetector) DetectExtraDimensionLabels(
	extraDimensions []string,
	providedLabels map[string]bool,
	diagnostics *RCADiagnostics,
) map[string]string { // dimension -> actualLabel
	availableDimensions := make(map[string]string)

	for _, dim := range extraDimensions {
		if providedLabels[dim] {
			availableDimensions[dim] = dim
			if diagnostics != nil {
				diagnostics.DimensionDetectionStatus[dim] = true
			}
		} else {
			// Dimension not found; record as undetected
			mld.logger.Debug("Configured dimension not found in metrics", "dimension", dim)
			if diagnostics != nil {
				diagnostics.DimensionDetectionStatus[dim] = false
				diagnostics.AddReducedAccuracyReason(
					fmt.Sprintf("Configured dimension '%s' not found in metrics data", dim))
			}
		}
	}

	return availableDimensions
}

// BuildLabelPreferences returns a function that can be used to prefer labels
// based on a mapping of standard names to actual label keys.
func (mld *MetricsLabelDetector) BuildLabelPreferences(
	labelMapping map[string]string, // standardName -> actualLabel
) map[string]string { // returns preference map for scoring
	return labelMapping
}

// GetServiceIdentifier extracts a stable service identifier from labels.
// Tries configured standard 'service' role first, then falls back to configured alternatives.
func (mld *MetricsLabelDetector) GetServiceIdentifier(
	labels map[string]string,
	availableLabels map[string]string, // standardName -> actualLabel
	diagnostics *RCADiagnostics,
) string {
	// Try to get service_name using detected label (canonical)
	if actualLabelKey, ok := availableLabels["service_name"]; ok {
		if svc, ok := labels[actualLabelKey]; ok && svc != "" {
			// Only return, do not set reduced accuracy if canonical label is present
			return svc
		}
	}

	// Fallback: try configured service label candidates (from EngineConfig.Labels.Service)
	for _, possibleLabel := range mld.labelsCfg.Service {
		if svc, ok := labels[possibleLabel]; ok && svc != "" {
			if diagnostics != nil {
				diagnostics.AddReducedAccuracyReason(
					fmt.Sprintf("Using configured fallback service label '%s' instead of canonical 'service' label", possibleLabel))
			}
			mld.logger.Debug("Using configured fallback service label", "label", possibleLabel)
			return svc
		}
	}

	// As a last resort, check a canonical semantic key "service" if present
	if svc, ok := labels["service"]; ok && svc != "" {
		if diagnostics != nil {
			diagnostics.AddReducedAccuracyReason("Using canonical 'service' label as last-resort service identifier")
		}
		mld.logger.Debug("Using canonical 'service' label as fallback")
		return svc
	}

	if diagnostics != nil {
		diagnostics.AddMissingLabel("service_name")
		diagnostics.AddReducedAccuracyReason("Could not determine service identifier from available labels")
	}

	mld.logger.Warn("Unable to identify service from labels", "available_labels", len(labels))
	return "unknown"
}
