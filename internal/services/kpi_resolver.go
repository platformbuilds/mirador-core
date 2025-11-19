package services

import (
	"context"
	"fmt"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/rca"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// KPIResolver resolves KPI definitions and maps them into RCA signal representations.
type KPIResolver struct {
	kpiRepo repo.KPIRepo
	logger  logger.Logger
}

// NewKPIResolver creates a new KPI resolver.
func NewKPIResolver(kpiRepo repo.KPIRepo, logger logger.Logger) *KPIResolver {
	return &KPIResolver{
		kpiRepo: kpiRepo,
		logger:  logger,
	}
}

// ResolveKPIToSignal resolves a KPI ID to its definition and maps it into an ImpactSignal.
// Returns:
// - ImpactSignal: the resolved signal (non-nil if resolution succeeded or partially succeeded)
// - KPIMetadata: KPI definition info for later use in scoring (may be nil if KPI not found)
// - error: non-nil only for critical failures; missing KPI is recorded in diagnostics, not an error
// - warnings: list of non-fatal issues (e.g., KPI not found, missing query field)
func (kr *KPIResolver) ResolveKPIToSignal(
	ctx context.Context,
	kpiID string,
	diagnostics *rca.RCADiagnostics,
) (*rca.ImpactSignal, *KPIMetadata, []string, error) {
	if diagnostics == nil {
		diagnostics = rca.NewRCADiagnostics()
	}

	var warnings []string

	// Validate inputs
	if kpiID == "" {
		return nil, nil, warnings, fmt.Errorf("kpiID cannot be empty")
	}

	// Look up KPI definition
	kpiDef, err := kr.kpiRepo.GetKPI(ctx, kpiID)
	if err != nil {
		msg := fmt.Sprintf("Failed to resolve KPI '%s': %v", kpiID, err)
		kr.logger.Warn("KPI resolution error", "kpi_id", kpiID, "error", err)
		diagnostics.AddReducedAccuracyReason(msg)
		warnings = append(warnings, msg)
		// Return a fallback signal so RCA can continue
		return &rca.ImpactSignal{
			ServiceName: "unknown_service",
			MetricName:  kpiID,
			Direction:   "higher_is_worse",
			Labels:      make(map[string]string),
			Threshold:   0.0,
		}, nil, warnings, nil
	}

	if kpiDef == nil {
		msg := fmt.Sprintf("KPI '%s' not found in repository", kpiID)
		kr.logger.Warn("KPI not found", "kpi_id", kpiID)
		diagnostics.AddReducedAccuracyReason(msg)
		warnings = append(warnings, msg)
		// Return a fallback signal
		return &rca.ImpactSignal{
			ServiceName: "unknown_service",
			MetricName:  kpiID,
			Direction:   "higher_is_worse",
			Labels:      make(map[string]string),
			Threshold:   0.0,
		}, nil, warnings, nil
	}

	// Extract metric direction from KPI definition
	// If KPI sentiment is NEGATIVE (e.g., error rate), increases are bad → higher_is_worse
	// If KPI sentiment is POSITIVE (e.g., throughput), decreases are bad → lower_is_worse
	direction := "higher_is_worse" // default
	if kpiDef.Sentiment == "POSITIVE" {
		direction = "lower_is_worse"
	}

	// Build ImpactSignal from KPI definition
	signal := &rca.ImpactSignal{
		ServiceName: extractServiceFromKPI(kpiDef),
		MetricName:  kpiDef.Name,
		Direction:   direction,
		Labels:      make(map[string]string),
		Threshold:   0.0, // Threshold would be extracted from Thresholds field if needed
	}

	// Extract any labels from query definition if present
	if kpiDef.Query != nil {
		if labels, ok := kpiDef.Query["labels"].(map[string]interface{}); ok {
			for k, v := range labels {
				if strVal, ok := v.(string); ok {
					signal.Labels[k] = strVal
				}
			}
		}
	}

	// Build KPI metadata for scoring
	metadata := &KPIMetadata{
		ID:        kpiDef.ID,
		Name:      kpiDef.Name,
		Kind:      kpiDef.Kind,
		Sentiment: kpiDef.Sentiment,
	}

	kr.logger.Debug("KPI resolved successfully",
		"kpi_id", kpiID,
		"kpi_name", kpiDef.Name,
		"kind", kpiDef.Kind,
		"sentiment", kpiDef.Sentiment,
		"signal_metric", signal.MetricName,
		"signal_direction", signal.Direction)

	return signal, metadata, warnings, nil
}

// KPIMetadata holds KPI-related context for RCA scoring and diagnostics.
type KPIMetadata struct {
	ID        string // KPI definition ID
	Name      string // KPI name
	Kind      string // "impact" or "cause" (business-layer vs technical-layer)
	Sentiment string // "NEGATIVE", "POSITIVE", or "NEUTRAL" - increase sentiment
}

// extractServiceFromKPI extracts a service name hint from the KPI definition.
// This is a best-effort heuristic; returns empty string if not determinable.
func extractServiceFromKPI(kpiDef *models.KPIDefinition) string {
	// Try to extract from query field
	if kpiDef.Query != nil {
		if service, ok := kpiDef.Query["service"].(string); ok && service != "" {
			return service
		}
	}
	// Fallback: return empty; caller will need to provide the service
	return ""
}
