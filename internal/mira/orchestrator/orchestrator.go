package orchestrator

import (
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/mira/intent"
)

type Orchestrator struct{}

func New() *Orchestrator { return &Orchestrator{} }

// HandleIntent executes the minimal logic required to return domain-shaped results.
// In Phase-1 this is a mocked implementation that returns canned responses.
func (o *Orchestrator) HandleIntent(ir intent.IntentResult) (map[string]interface{}, error) {
	switch ir.CapabilityID {
	case intent.APP_HEALTH_OVERVIEW:
		return o.mockHealth(ir)
	case intent.PERF_DETECT_FAILURES:
		return o.mockDetectFailures()
	case intent.RCA_PERFORM:
		return o.mockRCA()
	case intent.KPI_SEARCH:
		return o.mockKpiSearch(ir)
	// Phase-2 capabilities (not yet implemented)
	case intent.KPI_CREATE, intent.KPI_UPDATE, intent.PERF_LIST_FAILURES,
		intent.PERF_EXPLAIN_FAILURE, intent.RCA_LIST, intent.RCA_EXPLAIN, intent.GENERAL_CHAT:
		return map[string]interface{}{"text": fmt.Sprintf("Capability %s not yet implemented (Phase-2)", ir.CapabilityID)}, nil
	default:
		return map[string]interface{}{"text": fmt.Sprintf("Unknown capability %s", ir.CapabilityID)}, nil
	}
}

func (o *Orchestrator) mockHealth(ir intent.IntentResult) (map[string]interface{}, error) {
	svc := ir.Parameters["service"]
	if svc == "" {
		svc = "<unspecified>"
	}
	return map[string]interface{}{
		"type":      "health",
		"service":   svc,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"state":     "healthy",
		"details": map[string]interface{}{
			"errors":     0,
			"latency_ms": 120,
		},
	}, nil
}

func (o *Orchestrator) mockDetectFailures() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type":  "failures",
		"count": 1,
		"failures": []map[string]interface{}{
			{
				"failureId": "fail-123",
				"service":   "checkout",
				"summary":   "Increased 500s observed in checkout",
				"start":     time.Now().Add(-15 * time.Minute).UTC().Format(time.RFC3339),
			},
		},
	}, nil
}

func (o *Orchestrator) mockRCA() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type":      "rca",
		"rcaId":     "rca-123",
		"rootCause": "db connection pool exhaustion (mock)",
		"evidence":  []string{"increased db wait time", "errors rising concurrently"},
	}, nil
}

func (o *Orchestrator) mockKpiSearch(ir intent.IntentResult) (map[string]interface{}, error) {
	q := ir.Parameters["query"]
	if q == "" {
		q = "top metrics"
	}
	return map[string]interface{}{
		"type":  "kpis",
		"query": q,
		"results": []map[string]interface{}{
			{"kpiId": "kpi-1", "name": "http_requests_total", "value": 1234},
			{"kpiId": "kpi-2", "name": "http_errors_total", "value": 12},
		},
	}, nil
}
