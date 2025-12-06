package services

import (
	"strings"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
)

func makeTestConfig() *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{
			VictoriaMetrics: config.VictoriaMetricsConfig{
				Name:      "vm_default",
				Endpoints: []string{"http://localhost:8481"},
			},
			MetricsSources: []config.VictoriaMetricsConfig{{Name: "extra_vm"}},
		},
	}
}

func TestValidateKPIDefinition_HappyPaths(t *testing.T) {
	cfg := makeTestConfig()

	// Impact KPI
	impact := &models.KPIDefinition{
		Name:           "user-revenue-drop",
		Layer:          "impact",
		SignalType:     "metrics",
		Sentiment:      "negative",
		BusinessImpact: "revenue loss",
		Definition:     "percentage of users failing checkout",
	}
	if err := ValidateKPIDefinition(cfg, impact); err != nil {
		t.Fatalf("expected impact KPI to validate, got error: %v", err)
	}

	// Cause KPI
	cause := &models.KPIDefinition{
		Name:          "api-p99-latency",
		Layer:         "cause",
		SignalType:    "metrics",
		Sentiment:     "negative",
		Classifier:    "latency",
		Datastore:     "victoriametrics",
		QueryType:     "MetricsQL",
		Formula:       "max_over_time(api_latency[5m])",
		Domain:        "payments",
		ComponentType: "springboot",
	}
	if err := ValidateKPIDefinition(cfg, cause); err != nil {
		t.Fatalf("expected cause KPI to validate, got error: %v", err)
	}

	// Cause KPI represented as a query object (no Formula) should also validate
	causeQuery := &models.KPIDefinition{
		Name:       "api-p99-latency-query",
		Layer:      "cause",
		SignalType: "metrics",
		Sentiment:  "negative",
		Classifier: "latency",
		Datastore:  "victoriametrics",
		QueryType:  "MetricsQL",
		Query: map[string]interface{}{
			"metric": "api_latency",
			"window": "5m",
		},
		Domain:        "payments",
		ComponentType: "springboot",
	}
	if err := ValidateKPIDefinition(cfg, causeQuery); err != nil {
		t.Fatalf("expected cause KPI with query object to validate, got error: %v", err)
	}
}

func TestValidateKPIDefinition_Failures(t *testing.T) {
	cfg := makeTestConfig()

	// Invalid layer
	k1 := &models.KPIDefinition{Name: "x", Layer: "invalid", SignalType: "metrics", Sentiment: "negative"}
	err := ValidateKPIDefinition(cfg, k1)
	if err == nil || !strings.Contains(err.Error(), "layer") {
		t.Fatalf("expected layer error, got: %v", err)
	}

	// Invalid signalType
	k2 := &models.KPIDefinition{Name: "x2", Layer: "cause", SignalType: "metrix", Sentiment: "negative", Classifier: "errors", Domain: "d"}
	err = ValidateKPIDefinition(cfg, k2)
	if err == nil || !strings.Contains(err.Error(), "signalType") {
		t.Fatalf("expected signalType error, got: %v", err)
	}

	// Invalid sentiment
	k3 := &models.KPIDefinition{Name: "x3", Layer: "impact", SignalType: "metrics", Sentiment: "BAD", BusinessImpact: "biz"}
	err = ValidateKPIDefinition(cfg, k3)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "sentiment") {
		t.Fatalf("expected sentiment error, got: %v", err)
	}

	// Incompatible datastore/queryType
	k4 := &models.KPIDefinition{Name: "x4", Layer: "cause", SignalType: "metrics", Sentiment: "negative", Classifier: "latency", Datastore: "victoriametrics", QueryType: "sql", Formula: "select 1", Domain: "d"}
	err = ValidateKPIDefinition(cfg, k4)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "querytype") {
		t.Fatalf("expected queryType/datastore compatibility error, got: %v", err)
	}

	// Impact KPI with no definition + no businessImpact
	k5 := &models.KPIDefinition{Name: "x5", Layer: "impact", SignalType: "metrics", Sentiment: "negative"}
	err = ValidateKPIDefinition(cfg, k5)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "businessimpact") {
		t.Fatalf("expected businessImpact/definition error, got: %v", err)
	}

	// Cause KPI with empty classifier
	k6 := &models.KPIDefinition{Name: "x6", Layer: "cause", SignalType: "metrics", Sentiment: "negative", Domain: "payments"}
	err = ValidateKPIDefinition(cfg, k6)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "classifier") {
		t.Fatalf("expected classifier error, got: %v", err)
	}

	// Missing both query and formula should fail
	k7 := &models.KPIDefinition{Name: "x7", Layer: "cause", SignalType: "metrics", Sentiment: "negative", Classifier: "errors", Datastore: "victoriametrics", QueryType: "MetricsQL"}
	err = ValidateKPIDefinition(cfg, k7)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "formula/query") {
		t.Fatalf("expected formula/query error when both missing, got: %v", err)
	}
}

func TestValidateKPIDefinition_MixedCasingAndDefinitionOnly(t *testing.T) {
	cfg := makeTestConfig()

	// Mixed casing should validate
	k := &models.KPIDefinition{
		Name:           "mixed-casing",
		Layer:          "IMPACT",
		SignalType:     "Metrics",
		Sentiment:      "POSITIVE",
		BusinessImpact: "",
		Definition:     "this KPI measures something important",
	}
	if err := ValidateKPIDefinition(cfg, k); err != nil {
		t.Fatalf("expected mixed-casing impact KPI to validate, got: %v", err)
	}

	// Impact KPI with only Definition (no BusinessImpact) should validate
	k2 := &models.KPIDefinition{
		Name:       "definition-only",
		Layer:      "impact",
		SignalType: "metrics",
		Sentiment:  "negative",
		Definition: "only a definition present",
	}
	if err := ValidateKPIDefinition(cfg, k2); err != nil {
		t.Fatalf("expected impact KPI with only definition to validate, got: %v", err)
	}
}

func TestValidateKPIDefinition_DatastoreQueryCombinations(t *testing.T) {
	cfg := makeTestConfig()

	// Datastore 'victoriametrics' with empty QueryType -> error
	a := &models.KPIDefinition{Name: "d1", Layer: "cause", SignalType: "metrics", Sentiment: "negative", Classifier: "latency", Datastore: "victoriametrics"}
	err := ValidateKPIDefinition(cfg, a)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "querytype") {
		t.Fatalf("expected queryType/datastore error when QueryType empty, got: %v", err)
	}

	// QueryType present but Datastore empty -> error
	b := &models.KPIDefinition{Name: "d2", Layer: "cause", SignalType: "metrics", Sentiment: "negative", Classifier: "errors", QueryType: "MetricsQL"}
	err = ValidateKPIDefinition(cfg, b)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "datastore") {
		t.Fatalf("expected datastore error when QueryType present but datastore empty, got: %v", err)
	}

	// victoriametrics + MetricsQL -> valid
	c := &models.KPIDefinition{Name: "d3", Layer: "cause", SignalType: "metrics", Sentiment: "negative", Classifier: "latency", Datastore: "victoriametrics", QueryType: "MetricsQL", Formula: "x"}
	if err := ValidateKPIDefinition(cfg, c); err != nil {
		t.Fatalf("expected victoriametrics+MetricsQL to validate, got: %v", err)
	}
}

func TestValidateKPIDefinition_MultipleErrorAggregation(t *testing.T) {
	cfg := makeTestConfig()

	// multiple problems: invalid layer, invalid signalType, empty name, missing sentiment
	k := &models.KPIDefinition{
		Name:       "",
		Layer:      "badlayer",
		SignalType: "badsignal",
		Sentiment:  "",
	}
	err := ValidateKPIDefinition(cfg, k)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError type, got: %T", err)
	}
	if len(ve.Problems) < 2 {
		t.Fatalf("expected multiple problems aggregated, got %d: %v", len(ve.Problems), ve.Problems)
	}
}

func TestValidateKPIDefinition_VictoriaMetricsQueryTypeNormalization(t *testing.T) {
	cfg := makeTestConfig()

	// PromQL should be accepted and normalized to MetricsQL
	p := &models.KPIDefinition{
		Name:       "vm-promql",
		Layer:      "cause",
		SignalType: "metrics",
		Sentiment:  "negative",
		Classifier: "latency",
		Datastore:  "victoriametrics",
		QueryType:  "PromQL",
		Formula:    "max_over_time(x[5m])",
	}
	if err := ValidateKPIDefinition(cfg, p); err != nil {
		t.Fatalf("expected victoriametrics+PromQL to validate, got: %v", err)
	}
	if p.QueryType != "MetricsQL" {
		t.Fatalf("expected QueryType normalized to 'MetricsQL', got: %s", p.QueryType)
	}

	// Mixed casing should also normalize
	m := &models.KPIDefinition{
		Name:       "vm-mixed",
		Layer:      "cause",
		SignalType: "metrics",
		Sentiment:  "negative",
		Classifier: "latency",
		Datastore:  "victoriametrics",
		QueryType:  "metricsql",
		Formula:    "max_over_time(x[5m])",
	}
	if err := ValidateKPIDefinition(cfg, m); err != nil {
		t.Fatalf("expected victoriametrics+metricsql to validate, got: %v", err)
	}
	if m.QueryType != "MetricsQL" {
		t.Fatalf("expected QueryType normalized to 'MetricsQL' for mixed case, got: %s", m.QueryType)
	}

	// Unsupported query type for victoriametrics should fail
	bad := &models.KPIDefinition{
		Name:       "vm-bad",
		Layer:      "cause",
		SignalType: "metrics",
		Sentiment:  "negative",
		Classifier: "latency",
		Datastore:  "victoriametrics",
		QueryType:  "SQL",
		Formula:    "select 1",
	}
	err := ValidateKPIDefinition(cfg, bad)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "querytype") {
		t.Fatalf("expected queryType/datastore compatibility error for SQL, got: %v", err)
	}
}
