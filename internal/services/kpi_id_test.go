package services

import (
	"testing"

	"github.com/platformbuilds/mirador-core/internal/models"
)

func TestGenerateDeterministicKPIID_SameSourceSourceID(t *testing.T) {
	a := &models.KPIDefinition{Source: "seed-file", SourceID: "row-1", Name: "My KPI"}
	b := &models.KPIDefinition{Source: "seed-file", SourceID: "row-1", Name: "My KPI"}

	idA, err := GenerateDeterministicKPIID(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idB, err := GenerateDeterministicKPIID(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if idA != idB {
		t.Fatalf("expected same id for identical source+sourceId, got %s and %s", idA, idB)
	}
}

func TestGenerateDeterministicKPIID_DifferentSourcePairs(t *testing.T) {
	a := &models.KPIDefinition{Source: "seed-file", SourceID: "row-1"}
	b := &models.KPIDefinition{Source: "seed-file", SourceID: "row-2"}

	idA, err := GenerateDeterministicKPIID(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idB, err := GenerateDeterministicKPIID(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if idA == idB {
		t.Fatalf("expected different ids for different sourceId values, both were %s", idA)
	}
}

func TestGenerateDeterministicKPIID_FallbackNamespaceName(t *testing.T) {
	a := &models.KPIDefinition{Namespace: "ns1", Name: "KPI One"}
	b := &models.KPIDefinition{Namespace: "ns1", Name: "KPI One"}

	idA, err := GenerateDeterministicKPIID(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idB, err := GenerateDeterministicKPIID(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if idA != idB {
		t.Fatalf("expected same id for identical namespace+name fallback, got %s and %s", idA, idB)
	}
}
