package rca

import (
	"fmt"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
)

// MergeGlobalRCAConfig merges a global RCA config, request-level dimension config, and KPI definitions
// into a single, coherent RCADimensionConfig for use by the RCA engine.
//
// Merge strategy:
// 1. Start with defaults
// 2. Apply global config settings (from config.yaml)
// 3. Override/extend with request-level config (from RCARequest)
// 4. Incorporate KPI metadata if provided
//
// Non-fatal failures:
// - If a configured metric/label/KPI is missing at runtime, emit diagnostics warnings
// - Never crash or hard-fail; always return a usable config with degraded confidence where needed
func MergeGlobalRCAConfig(
	globalCfg *config.RCAConfig,
	requestCfg *RCADimensionConfig,
	kpiDefs []*models.KPIDefinition,
	diagnostics *RCADiagnostics,
) RCADimensionConfig {
	if diagnostics == nil {
		diagnostics = NewRCADiagnostics()
	}

	// Start with defaults
	merged := DefaultDimensionConfig()

	// Apply global config if provided and enabled
	if globalCfg != nil {
		// Merge extra dimensions
		if len(globalCfg.ExtraLabels) > 0 {
			// Deduplicate and merge
			labelSet := make(map[string]bool)
			for _, label := range merged.ExtraDimensions {
				labelSet[label] = true
			}
			for _, label := range globalCfg.ExtraLabels {
				labelSet[label] = true
			}
			merged.ExtraDimensions = make([]string, 0, len(labelSet))
			for label := range labelSet {
				merged.ExtraDimensions = append(merged.ExtraDimensions, label)
			}
		}

		// Merge dimension weights
		if globalCfg.ExtraLabelWeights != nil {
			for label, weight := range globalCfg.ExtraLabelWeights {
				merged.DimensionWeights[label] = weight
			}
		}

		// Override alignment penalty/bonus if specified
		if globalCfg.AlignmentPenalty > 0 {
			merged.AlignmentPenalty = globalCfg.AlignmentPenalty
		}
		if globalCfg.AlignmentBonus > 0 {
			merged.AlignmentBonus = globalCfg.AlignmentBonus
		}
	}

	// Override/extend with request-level config
	if requestCfg != nil {
		// Merge extra dimensions from request
		if len(requestCfg.ExtraDimensions) > 0 {
			labelSet := make(map[string]bool)
			for _, label := range merged.ExtraDimensions {
				labelSet[label] = true
			}
			for _, label := range requestCfg.ExtraDimensions {
				labelSet[label] = true
			}
			merged.ExtraDimensions = make([]string, 0, len(labelSet))
			for label := range labelSet {
				merged.ExtraDimensions = append(merged.ExtraDimensions, label)
			}
		}

		// Merge dimension weights from request
		if requestCfg.DimensionWeights != nil {
			for label, weight := range requestCfg.DimensionWeights {
				merged.DimensionWeights[label] = weight
			}
		}

		// Request-level alignment settings override global
		if requestCfg.AlignmentPenalty > 0 {
			merged.AlignmentPenalty = requestCfg.AlignmentPenalty
		}
		if requestCfg.AlignmentBonus > 0 {
			merged.AlignmentBonus = requestCfg.AlignmentBonus
		}
	}

	// Record KPI context for later use in scoring
	// Store KPI definitions in diagnostics for reference during scoring
	if len(kpiDefs) > 0 {
		for _, kpiDef := range kpiDefs {
			if kpiDef == nil {
				continue
			}
			// Optionally add KPI-related dimensions if Kind is "impact" or "cause"
			// For now, we just record their presence for diagnostics
			if kpiDef.Kind == "impact" {
				diagnostics.AddReducedAccuracyReason(
					fmt.Sprintf("KPI '%s' (impact-layer) included in correlation context", kpiDef.Name),
				)
			}
		}
	}

	return merged
}

// ValidateRCAConfigWithDiagnostics validates the merged RCA config and records any issues in diagnostics.
// Returns true if the config is usable, false if there are critical issues.
func ValidateRCAConfigWithDiagnostics(
	cfg *RCADimensionConfig,
	diagnostics *RCADiagnostics,
) bool {
	if cfg == nil {
		return false
	}

	warnings, err := cfg.ValidateAndNormalize()
	if err != nil {
		diagnostics.AddReducedAccuracyReason(fmt.Sprintf("RCA config validation error: %v", err))
		return false
	}

	for _, w := range warnings {
		diagnostics.AddReducedAccuracyReason(w)
	}

	return true
}
