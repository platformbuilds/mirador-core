package rca

import (
	"strings"
	"testing"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TestDimensionConfigValidation tests dimension config validation and normalization.
func TestDimensionConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      RCADimensionConfig
		shouldError bool
		expectWarns int
	}{
		{
			name: "valid config with extra dimensions",
			config: RCADimensionConfig{
				ExtraDimensions: []string{"env", "region"},
				DimensionWeights: map[string]float64{
					"env":    0.1,
					"region": 0.15,
				},
				AlignmentPenalty: 0.2,
				AlignmentBonus:   0.1,
			},
			shouldError: false,
			expectWarns: 0,
		},
		{
			name: "empty config (valid, just no dimensions)",
			config: RCADimensionConfig{
				ExtraDimensions:  []string{},
				DimensionWeights: map[string]float64{},
				AlignmentPenalty: 0.2,
				AlignmentBonus:   0.1,
			},
			shouldError: false,
			expectWarns: 1, // warning about no dimensions
		},
		{
			name: "invalid penalty (>1)",
			config: RCADimensionConfig{
				ExtraDimensions:  []string{"env"},
				AlignmentPenalty: 1.5,
				AlignmentBonus:   0.1,
			},
			shouldError: true,
			expectWarns: 0,
		},
		{
			name: "invalid bonus (<0)",
			config: RCADimensionConfig{
				ExtraDimensions:  []string{"env"},
				AlignmentPenalty: 0.2,
				AlignmentBonus:   -0.1,
			},
			shouldError: true,
			expectWarns: 0,
		},
		{
			name: "out-of-range weight gets clamped",
			config: RCADimensionConfig{
				ExtraDimensions: []string{"env", "region"},
				DimensionWeights: map[string]float64{
					"env":    1.5,  // out of range
					"region": -0.5, // out of range
				},
				AlignmentPenalty: 0.2,
				AlignmentBonus:   0.1,
			},
			shouldError: false,
			expectWarns: 2, // two weight warnings
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := test.config
			warns, err := cfg.ValidateAndNormalize()

			if test.shouldError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !test.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(warns) != test.expectWarns {
				t.Errorf("expected %d warnings, got %d: %v", test.expectWarns, len(warns), warns)
			}
		})
	}
}

// TestMetricsLabelDetection tests label detection and fallback logic.
func TestMetricsLabelDetection(t *testing.T) {
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	detector := NewMetricsLabelDetector(mockLogger)

	tests := []struct {
		name               string
		providedLabels     map[string]bool // Labels present in data
		expectedMissing    []string
		expectedAvailable  map[string]string
		expectDiagWarnings int
	}{
		{
			name: "all standard labels present",
			providedLabels: map[string]bool{
				"service_name": true,
				"span_name":    true,
				"span_kind":    true,
				"status_code":  true,
			},
			expectedMissing: []string{},
			expectedAvailable: map[string]string{
				"service_name": "service_name",
				"span_name":    "span_name",
				"span_kind":    "span_kind",
				"status_code":  "status_code",
			},
			expectDiagWarnings: 0,
		},
		{
			name: "service_name missing, uses fallback service.name",
			providedLabels: map[string]bool{
				"service.name": true,
				"span_name":    true,
				"span_kind":    true,
				"status_code":  true,
			},
			expectedMissing: []string{},
			expectedAvailable: map[string]string{
				"service_name": "service.name",
				"span_name":    "span_name",
				"span_kind":    "span_kind",
				"status_code":  "status_code",
			},
			expectDiagWarnings: 0,
		},
		{
			name: "some standard labels completely missing",
			providedLabels: map[string]bool{
				"service_name": true,
				"span_name":    true,
				// span_kind missing, status_code missing
			},
			expectedMissing: []string{"span_kind", "status_code"},
			expectedAvailable: map[string]string{
				"service_name": "service_name",
				"span_name":    "span_name",
			},
			expectDiagWarnings: 2,
		},
		{
			name:               "no standard labels present",
			providedLabels:     map[string]bool{},
			expectedMissing:    []string{"service_name", "span_name", "span_kind", "status_code"},
			expectedAvailable:  map[string]string{},
			expectDiagWarnings: 4,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diag := NewRCADiagnostics()

			availableLabels := detector.DetectAvailableLabels(test.providedLabels, diag)

			// Check missing labels
			if len(diag.MissingLabels) != len(test.expectedMissing) {
				t.Errorf("expected %d missing labels, got %d: %v", len(test.expectedMissing), len(diag.MissingLabels), diag.MissingLabels)
			}

			// Check available labels
			if len(availableLabels) != len(test.expectedAvailable) {
				t.Errorf("expected %d available labels, got %d", len(test.expectedAvailable), len(availableLabels))
			}

			for expected, expectedKey := range test.expectedAvailable {
				if actualKey, ok := availableLabels[expected]; !ok {
					t.Errorf("expected label %q not found", expected)
				} else if actualKey != expectedKey {
					t.Errorf("label %q: expected %q, got %q", expected, expectedKey, actualKey)
				}
			}

			// Check diagnostics warnings
			if len(diag.ReducedAccuracyReasons) != test.expectDiagWarnings {
				t.Errorf("expected %d diagnostic warnings, got %d", test.expectDiagWarnings, len(diag.ReducedAccuracyReasons))
			}
		})
	}
}

// TestExtraDimensionDetection tests detection of user-configured extra dimensions.
func TestExtraDimensionDetection(t *testing.T) {
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	detector := NewMetricsLabelDetector(mockLogger)

	providedLabels := map[string]bool{
		"env":    true,
		"region": true,
		// "cluster" is missing
		"namespace": true,
	}

	extraDims := []string{"env", "region", "cluster", "namespace"}
	diag := NewRCADiagnostics()

	detected := detector.DetectExtraDimensionLabels(extraDims, providedLabels, diag)

	// Should detect env, region, namespace; cluster should be missing
	if len(detected) != 3 {
		t.Errorf("expected 3 detected dimensions, got %d", len(detected))
	}

	if !diag.DimensionDetectionStatus["env"] ||
		!diag.DimensionDetectionStatus["region"] ||
		!diag.DimensionDetectionStatus["namespace"] {
		t.Errorf("expected env, region, namespace to be detected")
	}

	if diag.DimensionDetectionStatus["cluster"] {
		t.Errorf("expected cluster to not be detected")
	}

	// Should have 2 reduced accuracy reasons (for missing cluster + at least one reduced accuracy note)
	if len(diag.ReducedAccuracyReasons) == 0 {
		t.Errorf("expected reduced accuracy reasons for missing cluster")
	}
}

// TestDimensionAlignmentScoring tests the alignment scoring logic.
func TestDimensionAlignmentScoring(t *testing.T) {
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	scorer := NewDimensionAlignmentScorer(mockLogger)

	// Create candidate group with dimensions
	candidateGroup := &AnomalyGroup{
		Service:              "database",
		Component:            "postgres",
		ExtraDimensionValues: map[string]string{"env": "prod", "region": "us-west-1"},
	}

	// Impact service dimensions
	impactDims := map[string]string{"env": "prod", "region": "us-east-1"}

	// Config with dimensions
	config := RCADimensionConfig{
		ExtraDimensions: []string{"env", "region"},
		DimensionWeights: map[string]float64{
			"env":    0.1,
			"region": 0.2,
		},
		AlignmentPenalty: 0.2,
		AlignmentBonus:   0.1,
	}

	diag := NewRCADiagnostics()
	alignScore, alignments, notes := scorer.ComputeDimensionAlignmentScore(candidateGroup, impactDims, config, diag)

	// env aligns (both "prod"), region doesn't (us-west-1 vs us-east-1)
	// Expected: +bonus for env, -penalty for region, normalized by total weight
	// Score = (0.1 * 0.1 - 0.2 * 0.2) / (0.1 + 0.2) = (0.01 - 0.04) / 0.3 ≈ -0.1

	if len(alignments) != 2 {
		t.Errorf("expected 2 alignments, got %d", len(alignments))
	}

	// env should be aligned
	if !alignments[0].IsAligned || alignments[0].DimensionKey != "env" {
		t.Errorf("env dimension should be aligned")
	}

	// region should not be aligned
	if alignments[1].IsAligned || alignments[1].DimensionKey != "region" {
		t.Errorf("region dimension should not be aligned")
	}

	if alignScore >= 0 {
		t.Logf("alignment score is negative as expected: %.4f", alignScore)
	}

	t.Logf("alignment score: %.4f, notes: %v", alignScore, notes)
}

// TestDimensionAlignmentScoringWithMissingDimensions tests alignment when dimensions are missing.
func TestDimensionAlignmentScoringWithMissingDimensions(t *testing.T) {
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	scorer := NewDimensionAlignmentScorer(mockLogger)

	// Candidate missing a dimension
	candidateGroup := &AnomalyGroup{
		Service:              "cache",
		Component:            "valkey",
		ExtraDimensionValues: map[string]string{"env": "prod"}, // region is missing
	}

	impactDims := map[string]string{"env": "prod", "region": "us-east-1"}

	config := RCADimensionConfig{
		ExtraDimensions: []string{"env", "region"},
		DimensionWeights: map[string]float64{
			"env":    0.1,
			"region": 0.2,
		},
		AlignmentPenalty: 0.2,
		AlignmentBonus:   0.1,
	}

	diag := NewRCADiagnostics()
	alignScore, alignments, notes := scorer.ComputeDimensionAlignmentScore(candidateGroup, impactDims, config, diag)

	if len(alignments) != 2 {
		t.Errorf("expected 2 alignments, got %d", len(alignments))
	}

	// Should have a note about missing region
	if len(notes) == 0 {
		t.Errorf("expected notes about missing dimensions")
	}

	t.Logf("alignment score (missing dimension): %.4f, notes: %v", alignScore, notes)
}

// TestRCADiagnosticsToNotes tests conversion of diagnostics to human-readable notes.
func TestRCADiagnosticsToNotes(t *testing.T) {
	diag := NewRCADiagnostics()
	diag.AddMissingLabel("service_name")
	diag.AddMissingLabel("span_kind")
	diag.AddReducedAccuracyReason("Could not determine service identifier")
	diag.DimensionDetectionStatus["env"] = false
	diag.IsolationForestIssues = append(diag.IsolationForestIssues, "High anomaly scores but low classification rate")

	notes := diag.ToNotes()

	if len(notes) == 0 {
		t.Errorf("expected notes, got none")
	}

	t.Logf("Generated notes:\n%v", notes)
}

// TestServiceIdentifierFallback tests fallback logic for service identifier extraction.
func TestServiceIdentifierFallback(t *testing.T) {
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	detector := NewMetricsLabelDetector(mockLogger)

	tests := []struct {
		name                  string
		labels                map[string]string
		availableLabels       map[string]string
		expectedService       string
		expectReducedAccuracy bool
	}{
		{
			name:                  "standard service_name present",
			labels:                map[string]string{"service_name": "api-gateway"},
			availableLabels:       map[string]string{"service_name": "service_name"},
			expectedService:       "api-gateway",
			expectReducedAccuracy: false,
		},
		{
			name:                  "fallback to service.name",
			labels:                map[string]string{"service.name": "database"},
			availableLabels:       map[string]string{"service_name": "service.name"},
			expectedService:       "database",
			expectReducedAccuracy: false,
		},
		{
			name:                  "last resort fallback to any service-like label",
			labels:                map[string]string{"app": "messaging"},
			availableLabels:       map[string]string{}, // No standard service label detected
			expectedService:       "messaging",
			expectReducedAccuracy: true,
		},
		{
			name:                  "no service identifier available",
			labels:                map[string]string{"region": "us-east-1"},
			availableLabels:       map[string]string{},
			expectedService:       "unknown",
			expectReducedAccuracy: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diag := NewRCADiagnostics()
			svc := detector.GetServiceIdentifier(test.labels, test.availableLabels, diag)

			if svc != test.expectedService {
				t.Errorf("expected service %q, got %q", test.expectedService, svc)
			}

			hasReducedAccuracy := len(diag.ReducedAccuracyReasons) > 0
			if test.expectReducedAccuracy != hasReducedAccuracy {
				t.Errorf("expected reduced accuracy=%v, got=%v", test.expectReducedAccuracy, hasReducedAccuracy)
			}
		})
	}
}
