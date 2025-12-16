package summariser

import (
	"testing"
)

func TestGenerateTextParts_Health(t *testing.T) {
	domain := map[string]interface{}{"type": "health", "service": "checkout", "state": "degraded"}
	parts := GenerateTextParts(domain)
	if len(parts) == 0 {
		t.Fatalf("expected text parts; got none")
	}
}

func TestCardForDomain(t *testing.T) {
	domain := map[string]interface{}{"type": "rca", "rootCause": "db"}
	card, err := CardForDomain(domain)
	if err != nil {
		t.Fatalf("CardForDomain returned error: %v", err)
	}
	if card["kind"] != "rca" {
		t.Fatalf("expected card kind rca; got %v", card["kind"])
	}
}
