package rca

import (
	"testing"
)

func TestApplyKPISentimentBias_NegativeSentiment(t *testing.T) {
	// Test with NEGATIVE sentiment (higher is worse)
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.5,
			Rank:  1,
		},
		{
			Group: &AnomalyGroup{Service: "svc2"},
			Score: 0.3,
			Rank:  2,
		},
	}

	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata: &KPIIncidentMetadata{
			KPIName:      "ErrorRate",
			KPISentiment: "NEGATIVE",
			ImpactIsKPI:  true,
		},
	}

	diagnostics := NewRCADiagnostics()
	ApplyKPISentimentBias(candidates, incident, 0.05, diagnostics)

	// Both scores should increase by 0.05
	expectedScore1 := 0.55
	expectedScore2 := 0.35

	if candidates[0].Score != expectedScore1 {
		t.Errorf("Candidate 1: expected score %.2f, got %.2f", expectedScore1, candidates[0].Score)
	}
	if candidates[1].Score != expectedScore2 {
		t.Errorf("Candidate 2: expected score %.2f, got %.2f", expectedScore2, candidates[1].Score)
	}

	// Ranks should be re-ordered (still same order since 0.55 > 0.35)
	if candidates[0].Rank != 1 {
		t.Errorf("Candidate 0 rank should be 1, got %d", candidates[0].Rank)
	}
	if len(candidates) > 1 && candidates[1].Rank != 2 {
		t.Errorf("Candidate 1 rank should be 2, got %d", candidates[1].Rank)
	}

	// Check diagnostics
	if len(diagnostics.ReducedAccuracyReasons) == 0 {
		t.Errorf("Expected diagnostics reason for KPI bias application")
	}
}

func TestApplyKPISentimentBias_PositiveSentiment(t *testing.T) {
	// Test with POSITIVE sentiment (lower is worse)
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.6,
			Rank:  1,
		},
		{
			Group: &AnomalyGroup{Service: "svc2"},
			Score: 0.5,
			Rank:  2,
		},
	}

	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata: &KPIIncidentMetadata{
			KPIName:      "Throughput",
			KPISentiment: "POSITIVE",
			ImpactIsKPI:  true,
		},
	}

	diagnostics := NewRCADiagnostics()
	ApplyKPISentimentBias(candidates, incident, 0.05, diagnostics)

	// Both scores should decrease by 0.05
	expectedScore0 := 0.55
	expectedScore1 := 0.45

	// Note: after sort, highest score (0.55) should be first
	const tolerance = 0.0001
	if (candidates[0].Score-expectedScore0) > tolerance || (expectedScore0-candidates[0].Score) > tolerance {
		t.Errorf("Candidate 0: expected score %.2f, got %.2f", expectedScore0, candidates[0].Score)
	}
	if (candidates[1].Score-expectedScore1) > tolerance || (expectedScore1-candidates[1].Score) > tolerance {
		t.Errorf("Candidate 1: expected score %.2f, got %.2f", expectedScore1, candidates[1].Score)
	}

	// Ranks should be updated
	if candidates[0].Rank != 1 {
		t.Errorf("Candidate 0 rank should be 1, got %d", candidates[0].Rank)
	}
	if len(candidates) > 1 && candidates[1].Rank != 2 {
		t.Errorf("Candidate 1 rank should be 2, got %d", candidates[1].Rank)
	}
}

func TestApplyKPISentimentBias_NeutralSentiment(t *testing.T) {
	// Test with NEUTRAL sentiment (no effect)
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.5,
			Rank:  1,
		},
	}

	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata: &KPIIncidentMetadata{
			KPIName:      "CustomMetric",
			KPISentiment: "NEUTRAL",
			ImpactIsKPI:  true,
		},
	}

	diagnostics := NewRCADiagnostics()
	ApplyKPISentimentBias(candidates, incident, 0.05, diagnostics)

	// Score should remain unchanged
	if candidates[0].Score != 0.5 {
		t.Errorf("Expected score 0.5, got %.2f", candidates[0].Score)
	}
}

func TestApplyKPISentimentBias_ScoreClamping_Upper(t *testing.T) {
	// Test that scores are clamped to [0, 1]
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.98,
			Rank:  1,
		},
	}
	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata: &KPIIncidentMetadata{
			KPIName:      "ErrorRate",
			KPISentiment: "NEGATIVE",
			ImpactIsKPI:  true,
		},
	}

	diagnostics := NewRCADiagnostics()
	ApplyKPISentimentBias(candidates, incident, 0.05, diagnostics)

	// Score should be clamped to 1.0
	if len(candidates) > 0 && candidates[0].Score != 1.0 {
		t.Errorf("Expected score clamped to 1.0, got %.2f", candidates[0].Score)
	}
}

func TestApplyKPISentimentBias_ScoreClamping_Lower(t *testing.T) {
	// Test that scores are clamped to [0, 1]
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.02,
			Rank:  1,
		},
	}

	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata: &KPIIncidentMetadata{
			KPIName:      "Throughput",
			KPISentiment: "POSITIVE",
			ImpactIsKPI:  true,
		},
	}

	diagnostics := NewRCADiagnostics()
	ApplyKPISentimentBias(candidates, incident, 0.05, diagnostics)

	// Score should be clamped to 0.0
	if candidates[0].Score != 0.0 {
		t.Errorf("Expected score clamped to 0.0, got %.2f", candidates[0].Score)
	}
}

func TestApplyKPISentimentBias_NoKPIMetadata(t *testing.T) {
	// Test that bias is not applied when KPIMetadata is nil
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.5,
			Rank:  1,
		},
	}

	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata:   nil, // No KPI metadata
	}

	diagnostics := NewRCADiagnostics()
	ApplyKPISentimentBias(candidates, incident, 0.05, diagnostics)

	// Score should remain unchanged
	if candidates[0].Score != 0.5 {
		t.Errorf("Expected score 0.5, got %.2f", candidates[0].Score)
	}
}

func TestApplyKPISentimentBias_NotKPIBased(t *testing.T) {
	// Test that bias is not applied when ImpactIsKPI is false
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.5,
			Rank:  1,
		},
	}

	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata: &KPIIncidentMetadata{
			KPIName:      "ErrorRate",
			KPISentiment: "NEGATIVE",
			ImpactIsKPI:  false, // Not KPI-based
		},
	}

	diagnostics := NewRCADiagnostics()
	ApplyKPISentimentBias(candidates, incident, 0.05, diagnostics)

	// Score should remain unchanged
	if candidates[0].Score != 0.5 {
		t.Errorf("Expected score 0.5, got %.2f", candidates[0].Score)
	}
}

func TestApplyKPISentimentBias_Reordering(t *testing.T) {
	// Test that candidates are reordered after scoring
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.3,
			Rank:  2,
		},
		{
			Group: &AnomalyGroup{Service: "svc2"},
			Score: 0.5,
			Rank:  1,
		},
	}

	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata: &KPIIncidentMetadata{
			KPIName:      "ErrorRate",
			KPISentiment: "NEGATIVE",
			ImpactIsKPI:  true,
		},
	}

	diagnostics := NewRCADiagnostics()
	ApplyKPISentimentBias(candidates, incident, 0.3, diagnostics)

	// After applying +0.3 bias:
	// svc1: 0.3 + 0.3 = 0.6
	// svc2: 0.5 + 0.3 = 0.8
	// After re-ranking: svc2 (0.8) should be first, svc1 (0.6) should be second

	if candidates[0].Score != 0.8 { // svc2 should be first after sort
		t.Errorf("First candidate (highest score): expected score 0.8, got %.2f", candidates[0].Score)
	}
	if candidates[1].Score != 0.6 { // svc1 should be second after sort
		t.Errorf("Second candidate: expected score 0.6, got %.2f", candidates[1].Score)
	}

	// After sorting, highest score should be first
	if candidates[0].Score < candidates[1].Score {
		t.Errorf("Expected first candidate to have higher score; got %.2f and %.2f", candidates[0].Score, candidates[1].Score)
	}

	// Verify ranks were updated
	if candidates[0].Rank != 1 {
		t.Errorf("First candidate rank should be 1, got %d", candidates[0].Rank)
	}
	if len(candidates) > 1 && candidates[1].Rank != 2 {
		t.Errorf("Second candidate rank should be 2, got %d", candidates[1].Rank)
	}
}

func TestApplyKPISentimentBias_EmptyIncident(t *testing.T) {
	// Test that function handles empty candidates gracefully
	var candidates []*CandidateCause

	incident := &IncidentContext{
		ImpactService: "api",
	}

	diagnostics := NewRCADiagnostics()

	// Should not panic
	ApplyKPISentimentBias(candidates, incident, 0.05, diagnostics)
}

func TestApplyKPISentimentBias_NilIncident(t *testing.T) {
	// Test that function handles nil incident gracefully
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.5,
			Rank:  1,
		},
	}

	diagnostics := NewRCADiagnostics()

	// Should not panic
	ApplyKPISentimentBias(candidates, nil, 0.05, diagnostics)
}

func TestApplyKPISentimentBias_NilDiagnostics(t *testing.T) {
	// Test that function handles nil diagnostics gracefully
	// The function should check if diagnostics is nil before calling methods
	candidates := []*CandidateCause{
		{
			Group: &AnomalyGroup{Service: "svc1"},
			Score: 0.5,
			Rank:  1,
		},
	}

	incident := &IncidentContext{
		ImpactService: "api",
		KPIMetadata: &KPIIncidentMetadata{
			KPIName:      "ErrorRate",
			KPISentiment: "NEGATIVE",
			ImpactIsKPI:  true,
		},
	}

	// Should not panic with nil diagnostics
	ApplyKPISentimentBias(candidates, incident, 0.05, nil)

	// Score should not be adjusted since the function returns early if diagnostics is nil
	// (we check for nil diagnostics at the top)
	// Actually, looking at the implementation, it doesn't check for nil diagnostics,
	// so we need to verify the score is still adjusted
	if candidates[0].Score != 0.55 {
		t.Errorf("Expected score 0.55, got %.2f", candidates[0].Score)
	}
}
