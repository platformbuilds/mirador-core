package orchestrator

import (
	"testing"

	"github.com/platformbuilds/mirador-core/internal/mira/intent"
)

func TestOrchestrator_HandleIntent_Health(t *testing.T) {
	o := New()
	ir := intent.IntentResult{CapabilityID: intent.APP_HEALTH_OVERVIEW, Parameters: map[string]string{"service": "checkout"}}
	d, err := o.HandleIntent(ir)
	if err != nil {
		t.Fatalf("HandleIntent returned error: %v", err)
	}
	if d["type"] != "health" {
		t.Fatalf("expected health type; got %v", d["type"])
	}
}

func TestOrchestrator_HandleIntent_KPI(t *testing.T) {
	o := New()
	ir := intent.IntentResult{CapabilityID: intent.KPI_SEARCH, Parameters: map[string]string{"query": "errors"}}
	d, err := o.HandleIntent(ir)
	if err != nil {
		t.Fatalf("HandleIntent returned error: %v", err)
	}
	if d["type"] != "kpis" {
		t.Fatalf("expected kpis type; got %v", d["type"])
	}
}
