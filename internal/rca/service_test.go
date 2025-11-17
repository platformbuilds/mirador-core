package rca

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TestCandidateCauseService_Integration tests end-to-end flow.
func TestCandidateCauseService_Integration(t *testing.T) {
	now := time.Now()

	// Create synthetic anomalies
	anomalies := []*AnomalyEvent{
		// Upstream service, R1, high severity
		{
			ID:            "ae1",
			Timestamp:     now.Add(-2 * time.Second),
			Service:       "cache-service",
			Component:     "redis",
			SignalType:    SignalTypeMetrics,
			MetricOrField: "latency",
			FieldValue:    150.0,
			Severity:      SeverityCritical,
			AnomalyScore:  0.9,
			Confidence:    0.95,
			Tags:          map[string]string{"transaction_id": "tx-001"},
		},
		// Upstream service, R1, high severity (same service)
		{
			ID:            "ae2",
			Timestamp:     now.Add(-1 * time.Second),
			Service:       "cache-service",
			Component:     "redis",
			SignalType:    SignalTypeTraces,
			MetricOrField: "span_duration",
			FieldValue:    120.0,
			Severity:      SeverityHigh,
			AnomalyScore:  0.85,
			Confidence:    0.90,
			Tags:          map[string]string{"transaction_id": "tx-002"},
		},
		// Same service (app-server), R2, medium severity
		{
			ID:            "ae3",
			Timestamp:     now.Add(-15 * time.Second),
			Service:       "app-server",
			Component:     "api",
			SignalType:    SignalTypeLogs,
			MetricOrField: "error_rate",
			FieldValue:    0.15,
			Severity:      SeverityMedium,
			AnomalyScore:  0.55,
			Confidence:    0.60,
			Tags:          map[string]string{"transaction_id": "tx-003"},
		},
		// Downstream service, R1, low severity
		{
			ID:            "ae4",
			Timestamp:     now.Add(-3 * time.Second),
			Service:       "logging-service",
			Component:     "processor",
			SignalType:    SignalTypeMetrics,
			MetricOrField: "queue_depth",
			FieldValue:    10.0,
			Severity:      SeverityLow,
			AnomalyScore:  0.3,
			Confidence:    0.5,
			Tags:          map[string]string{"transaction_id": "tx-004"},
		},
	}

	// Create a simple service graph
	// app-server depends on cache-service
	// logging-service depends on app-server
	graph := NewServiceGraph()
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("app-server"),
		Target: ServiceNode("cache-service"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("logging-service"),
		Target: ServiceNode("app-server"),
	})

	// Create mock anomaly provider
	provider := &MockAnomalyEventsProvider{
		anomalies: anomalies,
	}

	// Create logger
	testLogger := logger.New("info")

	// Create incident context
	incident := &IncidentContext{
		ID:            "incident-001",
		ImpactService: "app-server",
		ImpactSignal: ImpactSignal{
			ServiceName: "app-server",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
			Threshold:   0.10,
		},
		TimeBounds: IncidentTimeWindow{
			TStart: now.Add(-5 * time.Second),
			TPeak:  now,
			TEnd:   now.Add(5 * time.Second),
		},
		Severity: 0.8,
	}

	// Create collector
	collector := NewIncidentAnomalyCollector(provider, graph, testLogger)

	// Create service
	svc := NewCandidateCauseService(collector, testLogger)

	// Compute candidates
	opts := DefaultCandidateCauseOptions()
	candidates, err := svc.ComputeCandidates(context.Background(), incident, opts)

	if err != nil {
		t.Fatalf("ComputeCandidates failed: %v", err)
	}

	if len(candidates) == 0 {
		t.Fatalf("Expected at least 1 candidate, got 0")
	}

	// Verify that cache-service is ranked first (upstream, R1, high severity/score)
	if candidates[0].Group.Service != "cache-service" {
		t.Errorf("Expected top candidate to be cache-service, got %s", candidates[0].Group.Service)
	}

	if candidates[0].Rank != 1 {
		t.Errorf("Expected top candidate rank 1, got %d", candidates[0].Rank)
	}

	// Verify reasons are populated
	if len(candidates[0].Reasons) == 0 {
		t.Errorf("Expected reasons for top candidate, got none")
	}

	t.Logf("Top candidate: %s (rank %d, score %.4f)",
		candidates[0].Group.Service, candidates[0].Rank, candidates[0].Score)
	t.Logf("Reasons: %v", candidates[0].Reasons)
}

// TestCandidateCauseService_EmptyAnomalies tests handling of no anomalies.
func TestCandidateCauseService_EmptyAnomalies(t *testing.T) {
	now := time.Now()

	provider := &MockAnomalyEventsProvider{
		anomalies: []*AnomalyEvent{},
	}

	graph := NewServiceGraph()

	testLogger := logger.New("info")

	incident := &IncidentContext{
		ID:            "incident-001",
		ImpactService: "app-server",
		ImpactSignal: ImpactSignal{
			ServiceName: "app-server",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
		},
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(5 * time.Second),
			TEnd:   now.Add(10 * time.Second),
		},
	}

	collector := NewIncidentAnomalyCollector(provider, graph, testLogger)
	svc := NewCandidateCauseService(collector, testLogger)

	opts := DefaultCandidateCauseOptions()
	candidates, err := svc.ComputeCandidates(context.Background(), incident, opts)

	if err != nil {
		t.Fatalf("ComputeCandidates failed: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates for no anomalies, got %d", len(candidates))
	}
}

// TestCandidateCauseService_Filtering tests that filtering options are applied.
func TestCandidateCauseService_Filtering(t *testing.T) {
	now := time.Now()

	anomalies := []*AnomalyEvent{
		{
			ID:           "ae1",
			Timestamp:    now.Add(-2 * time.Second),
			Service:      "service-a",
			Component:    "comp-a",
			Severity:     SeverityHigh,
			AnomalyScore: 0.8,
		},
		{
			ID:           "ae2",
			Timestamp:    now.Add(-1 * time.Second),
			Service:      "service-a",
			Component:    "comp-a",
			Severity:     SeverityHigh,
			AnomalyScore: 0.8,
		},
		{
			ID:           "ae3",
			Timestamp:    now.Add(-3 * time.Second),
			Service:      "service-b",
			Component:    "comp-b",
			Severity:     SeverityLow,
			AnomalyScore: 0.2,
		},
	}

	provider := &MockAnomalyEventsProvider{
		anomalies: anomalies,
	}

	graph := NewServiceGraph()
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("service-a"),
		Target: ServiceNode("service-b"),
	})

	testLogger := logger.New("info")

	incident := &IncidentContext{
		ID:            "incident-001",
		ImpactService: "service-a",
		ImpactSignal: ImpactSignal{
			ServiceName: "service-a",
			MetricName:  "metric",
			Direction:   "higher_is_worse",
		},
		TimeBounds: IncidentTimeWindow{
			TStart: now.Add(-5 * time.Second),
			TPeak:  now,
			TEnd:   now.Add(5 * time.Second),
		},
	}

	collector := NewIncidentAnomalyCollector(provider, graph, testLogger)
	svc := NewCandidateCauseService(collector, testLogger)

	// Use filtering to require minimum severity of High
	opts := DefaultCandidateCauseOptions()
	opts.CollectOptions.MinSeverity = SeverityHigh
	opts.GroupingConfig.MinEventsPerGroup = 2 // Require at least 2 events per group

	candidates, err := svc.ComputeCandidates(context.Background(), incident, opts)

	if err != nil {
		t.Fatalf("ComputeCandidates failed: %v", err)
	}

	// Should only have service-a (Low severity service-b filtered by minimum severity)
	if len(candidates) == 0 {
		t.Errorf("Expected 1 candidate after filtering, got 0. Service-a has 2 high-severity events and should pass MinEventsPerGroup=2 filter")
		return
	}

	if len(candidates) != 1 {
		t.Errorf("Expected 1 candidate after filtering, got %d", len(candidates))
	}

	if candidates[0].Group.Service != "service-a" {
		t.Errorf("Expected service-a, got %s", candidates[0].Group.Service)
	}

	if candidates[0].Group.EventCount != 2 {
		t.Errorf("Expected exactly 2 events in group, got %d", candidates[0].Group.EventCount)
	}
}

// TestCandidateCauseService_DetailedScores tests that detailed scores are populated.
func TestCandidateCauseService_DetailedScores(t *testing.T) {
	now := time.Now()

	anomalies := []*AnomalyEvent{
		{
			ID:           "ae1",
			Timestamp:    now.Add(-1 * time.Second),
			Service:      "service-a",
			Component:    "comp",
			Severity:     SeverityCritical,
			AnomalyScore: 0.9,
		},
	}

	provider := &MockAnomalyEventsProvider{
		anomalies: anomalies,
	}

	graph := NewServiceGraph()

	testLogger := logger.New("info")

	incident := &IncidentContext{
		ID:            "incident-001",
		ImpactService: "service-a",
		ImpactSignal: ImpactSignal{
			ServiceName: "service-a",
			MetricName:  "metric",
			Direction:   "higher_is_worse",
		},
		TimeBounds: IncidentTimeWindow{
			TStart: now.Add(-5 * time.Second),
			TPeak:  now,
			TEnd:   now.Add(5 * time.Second),
		},
	}

	collector := NewIncidentAnomalyCollector(provider, graph, testLogger)
	svc := NewCandidateCauseService(collector, testLogger)

	opts := DefaultCandidateCauseOptions()
	candidates, err := svc.ComputeCandidates(context.Background(), incident, opts)

	if err != nil {
		t.Fatalf("ComputeCandidates failed: %v", err)
	}

	if len(candidates) == 0 {
		t.Fatalf("Expected at least 1 candidate")
	}

	candidate := candidates[0]

	// Verify detailed score is populated
	if candidate.DetailedScore == nil {
		t.Fatalf("DetailedScore is nil")
	}

	if candidate.DetailedScore.RingScore <= 0 {
		t.Errorf("Expected positive RingScore, got %.4f", candidate.DetailedScore.RingScore)
	}

	if candidate.DetailedScore.SeverityScore != float64(SeverityCritical) {
		t.Errorf("Expected SeverityScore %.4f, got %.4f", float64(SeverityCritical), candidate.DetailedScore.SeverityScore)
	}

	if candidate.DetailedScore.AnomalyScoreContribution != 0.9 {
		t.Errorf("Expected AnomalyScoreContribution 0.9, got %.4f", candidate.DetailedScore.AnomalyScoreContribution)
	}

	t.Logf("Detailed scores: Ring=%.4f, Direction=%.4f, Distance=%.4f, Severity=%.4f, AnomalyScore=%.4f, TxnCount=%.4f",
		candidate.DetailedScore.RingScore,
		candidate.DetailedScore.DirectionScore,
		candidate.DetailedScore.DistanceScore,
		candidate.DetailedScore.SeverityScore,
		candidate.DetailedScore.AnomalyScoreContribution,
		candidate.DetailedScore.TransactionCountScore,
	)
}
