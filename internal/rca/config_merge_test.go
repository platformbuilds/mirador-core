package rca

import (
	"testing"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
)

func TestMergeGlobalRCAConfig_OnlyDefaults(t *testing.T) {
	// Test with nil global and request configs
	merged := MergeGlobalRCAConfig(nil, nil, nil, nil)

	if len(merged.ExtraDimensions) != 0 {
		t.Errorf("Expected 0 extra dimensions, got %d", len(merged.ExtraDimensions))
	}
	if merged.AlignmentPenalty != 0.2 {
		t.Errorf("Expected default penalty 0.2, got %v", merged.AlignmentPenalty)
	}
	if merged.AlignmentBonus != 0.1 {
		t.Errorf("Expected default bonus 0.1, got %v", merged.AlignmentBonus)
	}
}

func TestMergeGlobalRCAConfig_WithGlobal(t *testing.T) {
	// Test with global config
	globalCfg := &config.RCAConfig{
		Enabled:           true,
		ExtraLabels:       []string{"env", "region"},
		ExtraLabelWeights: map[string]float64{"env": 0.5, "region": 0.3},
		AlignmentPenalty:  0.15,
		AlignmentBonus:    0.12,
	}

	merged := MergeGlobalRCAConfig(globalCfg, nil, nil, nil)

	if len(merged.ExtraDimensions) != 2 {
		t.Errorf("Expected 2 extra dimensions, got %d", len(merged.ExtraDimensions))
	}
	if merged.AlignmentPenalty != 0.15 {
		t.Errorf("Expected penalty 0.15, got %v", merged.AlignmentPenalty)
	}
	if merged.AlignmentBonus != 0.12 {
		t.Errorf("Expected bonus 0.12, got %v", merged.AlignmentBonus)
	}

	// Check weights
	if w, ok := merged.DimensionWeights["env"]; !ok || w != 0.5 {
		t.Errorf("Expected env weight 0.5, got %v", w)
	}
	if w, ok := merged.DimensionWeights["region"]; !ok || w != 0.3 {
		t.Errorf("Expected region weight 0.3, got %v", w)
	}
}

func TestMergeGlobalRCAConfig_WithRequest(t *testing.T) {
	// Test with request config
	requestCfg := &RCADimensionConfig{
		ExtraDimensions:  []string{"cluster"},
		DimensionWeights: map[string]float64{"cluster": 0.6},
		AlignmentPenalty: 0.25,
		AlignmentBonus:   0.08,
	}

	merged := MergeGlobalRCAConfig(nil, requestCfg, nil, nil)

	if len(merged.ExtraDimensions) != 1 {
		t.Errorf("Expected 1 extra dimension, got %d", len(merged.ExtraDimensions))
	}
	if merged.AlignmentPenalty != 0.25 {
		t.Errorf("Expected penalty 0.25, got %v", merged.AlignmentPenalty)
	}
}

func TestMergeGlobalRCAConfig_MergeGlobalAndRequest(t *testing.T) {
	// Test merging both global and request configs
	globalCfg := &config.RCAConfig{
		Enabled:           true,
		ExtraLabels:       []string{"env", "region"},
		ExtraLabelWeights: map[string]float64{"env": 0.5},
		AlignmentPenalty:  0.15,
	}

	requestCfg := &RCADimensionConfig{
		ExtraDimensions:  []string{"cluster"},
		DimensionWeights: map[string]float64{"cluster": 0.6},
		AlignmentPenalty: 0.2, // Override global
	}

	merged := MergeGlobalRCAConfig(globalCfg, requestCfg, nil, nil)

	// Should have all dimensions deduplicated
	if len(merged.ExtraDimensions) != 3 {
		t.Errorf("Expected 3 extra dimensions, got %d: %v", len(merged.ExtraDimensions), merged.ExtraDimensions)
	}

	// Request should override global penalty
	if merged.AlignmentPenalty != 0.2 {
		t.Errorf("Expected penalty 0.2 (from request), got %v", merged.AlignmentPenalty)
	}

	// Should have both weights
	if w, ok := merged.DimensionWeights["env"]; !ok || w != 0.5 {
		t.Errorf("Expected env weight 0.5 (from global), got %v", w)
	}
	if w, ok := merged.DimensionWeights["cluster"]; !ok || w != 0.6 {
		t.Errorf("Expected cluster weight 0.6 (from request), got %v", w)
	}
}

func TestMergeGlobalRCAConfig_WithKPIs(t *testing.T) {
	// Test KPI inclusion in diagnostics
	diagnostics := NewRCADiagnostics()

	kpiDefs := []*models.KPIDefinition{
		{
			ID:        "kpi-1",
			Name:      "Revenue",
			Kind:      "impact",
			Sentiment: "POSITIVE",
		},
		{
			ID:        "kpi-2",
			Name:      "ErrorRate",
			Kind:      "cause",
			Sentiment: "NEGATIVE",
		},
	}

	merged := MergeGlobalRCAConfig(nil, nil, kpiDefs, diagnostics)

	// Verify merge succeeds
	if merged.AlignmentPenalty != 0.2 {
		t.Errorf("Expected default penalty, got %v", merged.AlignmentPenalty)
	}

	// Check diagnostics were updated (at least one KPI should be recorded)
	reasons := diagnostics.ReducedAccuracyReasons
	if len(reasons) == 0 {
		t.Errorf("Expected at least 1 diagnostic reason for KPIs, got %d", len(reasons))
	}
}

func TestMergeGlobalRCAConfig_DimensionDeduplication(t *testing.T) {
	// Test that duplicate dimensions are properly deduplicated
	globalCfg := &config.RCAConfig{
		ExtraLabels: []string{"env", "region", "env"}, // Duplicate env
	}

	requestCfg := &RCADimensionConfig{
		ExtraDimensions: []string{"env", "cluster"}, // Another env
	}

	merged := MergeGlobalRCAConfig(globalCfg, requestCfg, nil, nil)

	// Should have 3 unique dimensions: env, region, cluster
	if len(merged.ExtraDimensions) != 3 {
		t.Errorf("Expected 3 unique dimensions, got %d: %v", len(merged.ExtraDimensions), merged.ExtraDimensions)
	}

	// Count occurrences of "env"
	envCount := 0
	for _, dim := range merged.ExtraDimensions {
		if dim == "env" {
			envCount++
		}
	}
	if envCount != 1 {
		t.Errorf("Expected 1 'env' dimension, found %d", envCount)
	}
}

func TestValidateRCAConfigWithDiagnostics_Valid(t *testing.T) {
	cfg := &RCADimensionConfig{
		ExtraDimensions:  []string{"env"},
		AlignmentPenalty: 0.2,
		AlignmentBonus:   0.1,
	}
	diagnostics := NewRCADiagnostics()

	valid := ValidateRCAConfigWithDiagnostics(cfg, diagnostics)

	if !valid {
		t.Errorf("Expected valid config")
	}
}

func TestValidateRCAConfigWithDiagnostics_InvalidPenalty(t *testing.T) {
	cfg := &RCADimensionConfig{
		AlignmentPenalty: 1.5, // Out of range
		AlignmentBonus:   0.1,
	}
	diagnostics := NewRCADiagnostics()

	valid := ValidateRCAConfigWithDiagnostics(cfg, diagnostics)

	if valid {
		t.Errorf("Expected invalid config due to out-of-range penalty")
	}

	if len(diagnostics.ReducedAccuracyReasons) == 0 {
		t.Errorf("Expected diagnostics reason for invalid config")
	}
}

func TestValidateRCAConfigWithDiagnostics_NilConfig(t *testing.T) {
	diagnostics := NewRCADiagnostics()

	valid := ValidateRCAConfigWithDiagnostics(nil, diagnostics)

	if valid {
		t.Errorf("Expected invalid config for nil input")
	}
}
