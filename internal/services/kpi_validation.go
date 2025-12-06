package services

import (
	"fmt"
	"strings"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
)

// FieldError represents a single validation problem for a field
type FieldError struct {
	Field   string
	Message string
}

// ValidationError aggregates multiple FieldErrors
type ValidationError struct {
	Problems []FieldError
}

// Error implements error
func (v *ValidationError) Error() string {
	if v == nil || len(v.Problems) == 0 {
		return ""
	}
	parts := make([]string, 0, len(v.Problems))
	for _, p := range v.Problems {
		parts = append(parts, fmt.Sprintf("%s: %s", p.Field, p.Message))
	}
	return strings.Join(parts, "; ")
}

// add adds a problem to the ValidationError
func (v *ValidationError) add(field, msg string) {
	v.Problems = append(v.Problems, FieldError{Field: field, Message: msg})
}

// isEmpty helper
func (v *ValidationError) isEmpty() bool {
	return v == nil || len(v.Problems) == 0
}

// ValidateKPIDefinition validates the semantic invariants for a KPIDefinition.
func ValidateKPIDefinition(cfg *config.Config, k *models.KPIDefinition) error {
	var ve ValidationError
	if k == nil {
		ve.add("kpiDefinition", "kpi definition is nil")
		return &ve
	}

	// 1. Layer
	layer := strings.ToLower(strings.TrimSpace(k.Layer))
	if layer == "" {
		ve.add("layer", "layer is required and must be 'impact' or 'cause'")
	} else if layer != "impact" && layer != "cause" {
		ve.add("layer", "invalid layer value; allowed: impact, cause")
	}

	// 2. SignalType
	sig := strings.ToLower(strings.TrimSpace(k.SignalType))
	allowedSignal := map[string]bool{
		"metrics": true, "traces": true, "logs": true, "business": true, "synthetic": true,
		// legacy/compat
		"metricdef": true, "metriclabeldef": true, "metricformula": true,
		"logaggrformula": true, "logfielddef": true, "tracesservicedef": true, "tracesoperationdef": true,
	}
	if sig == "" {
		ve.add("signalType", "signalType is required")
	} else if !allowedSignal[sig] {
		ve.add("signalType", "invalid signalType; allowed: metrics, traces, logs, business, synthetic (legacy variants accepted)")
	}

	// 3. Kind
	if k.Kind != "" {
		kind := strings.ToLower(strings.TrimSpace(k.Kind))
		if kind != "business" && kind != "tech" {
			ve.add("kind", "if present, kind must be 'business' or 'tech'")
		}
	}

	// 4. Sentiment
	sentiment := strings.ToLower(strings.TrimSpace(k.Sentiment))
	if sentiment == "" {
		ve.add("sentiment", "sentiment is required and must be one of: positive, negative, neutral")
	} else if sentiment != "positive" && sentiment != "negative" && sentiment != "neutral" {
		ve.add("sentiment", "invalid sentiment; allowed: positive, negative, neutral")
	}

	// 5. Datastore + QueryType compatibility
	ds := strings.ToLower(strings.TrimSpace(k.Datastore))
	qt := strings.ToLower(strings.TrimSpace(k.QueryType))

	// Build allowed datastore names from config (victoriametrics and named metrics sources)
	allowedDatastores := map[string]bool{}
	// include literal "victoriametrics" if default block present
	if cfg != nil {
		if cfg.Database.VictoriaMetrics.Name != "" || len(cfg.Database.VictoriaMetrics.Endpoints) > 0 {
			allowedDatastores["victoriametrics"] = true
			if cfg.Database.VictoriaMetrics.Name != "" {
				allowedDatastores[strings.ToLower(cfg.Database.VictoriaMetrics.Name)] = true
			}
		}
		for _, s := range cfg.Database.MetricsSources {
			if s.Name != "" {
				allowedDatastores[strings.ToLower(s.Name)] = true
			}
		}
	}

	if ds != "" {
		if !allowedDatastores[ds] {
			ve.add("datastore", "datastore is not configured in server config")
		} else {
			if ds == "victoriametrics" {
				if qt == "" {
					ve.add("queryType", "queryType is required for datastore 'victoriametrics' and must be 'MetricsQL' or 'PromQL'")
				} else {
					// Accept both MetricsQL and PromQL for VictoriaMetrics; normalize to canonical "MetricsQL"
					qtl := strings.ToLower(qt)
					if qtl == "metricsql" || qtl == "promql" {
						// normalize to canonical form for downstream consumers
						k.QueryType = "MetricsQL"
					} else {
						ve.add("queryType", "for datastore 'victoriametrics' queryType must be 'MetricsQL' or 'PromQL'")
					}
				}
			}
		}
	} else {
		// QueryType specified without a datastore is invalid
		if qt != "" {
			ve.add("datastore", "datastore must be set when queryType is provided")
		}
	}

	// 6. Impact vs Cause semantics
	if layer == "impact" {
		if strings.TrimSpace(k.BusinessImpact) == "" && strings.TrimSpace(k.Definition) == "" {
			ve.add("businessImpact/definition", "impact KPIs must include businessImpact or definition")
		}
	}
	if layer == "cause" {
		if strings.TrimSpace(k.Classifier) == "" {
			ve.add("classifier", "cause KPIs must include a classifier")
		}
		// Domain / ComponentType are recommended for better RCA mapping but are
		// not strictly required. We accept cause KPIs without them to avoid
		// blocking existing seeds. Keep this as a non-fatal guidance.
		// If desired in future, convert this into a warning mechanism.
		// if strings.TrimSpace(k.Domain) == "" && strings.TrimSpace(k.ComponentType) == "" {
		//     ve.add("domain/componentType", "cause KPIs should set either domain or componentType")
		// }
	}

	// 7. Name / Namespace
	// Name must be present; validator should be called before
	// GenerateDeterministicKPIID so that empty-name cases are rejected
	// before deterministic ID generation.
	if strings.TrimSpace(k.Name) == "" {
		ve.add("name", "name is required")
	}

	// 8. Formula / Query - a KPI must include either a 'formula' (string) or a
	// 'query' (object). Additionally, if a QueryType is set but only a Query is
	// *not* provided then a Formula is required for datastore-backed query types
	// (e.g., VictoriaMetrics). These rules prevent ambiguous/empty payloads and
	// make errors explicit.
	hasFormula := strings.TrimSpace(k.Formula) != ""
	hasQuery := k.Query != nil && len(k.Query) > 0

	// Only require a query/formula for data-backed KPIs (i.e. cause KPIs or
	// non-business signal types such as metrics/traces/logs). Impact KPIs are
	// often business descriptions so we skip this check for impact layer.
	if layer != "impact" {
		// For data-backed definitions, at least one of Query or Formula must be present
		if !hasFormula && !hasQuery {
			ve.add("formula/query", "either 'formula' (string) or 'query' (object) must be provided for data-backed KPIs (e.g. cause/metrics/traces/logs)")
		}

		if qt != "" && !hasFormula && !hasQuery {
			// If queryType is specified but neither a formula nor a query object was
			// provided, keep the existing queryType -> formula requirement but make
			// the error message clearer and consistent with the new combined rule.
			ve.add("queryType", "queryType is set but no 'formula' or 'query' provided; provide a query object or a formula string")
		}
	}

	if ve.isEmpty() {
		return nil
	}
	return &ve
}
