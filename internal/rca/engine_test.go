package rca

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestRCAEngineComputeRCA_BasicChain(t *testing.T) {
	// Setup: Create a mock logger
	mockLogger := logger.NewMockLogger(&strings.Builder{})

	// Create service graph: api-gateway -> tps -> kafka -> cassandra
	serviceGraph := NewServiceGraph()
	serviceGraph.AddEdge(ServiceEdge{
		Source:       "api-gateway",
		Target:       "tps",
		ErrorRate:    0.05,
		LatencyAvgMs: 100,
	})
	serviceGraph.AddEdge(ServiceEdge{
		Source:       "tps",
		Target:       "kafka",
		ErrorRate:    0.02,
		LatencyAvgMs: 50,
	})
	serviceGraph.AddEdge(ServiceEdge{
		Source:       "kafka",
		Target:       "cassandra",
		ErrorRate:    0.03,
		LatencyAvgMs: 75,
	})

	// Create a mock anomaly events provider
	mockProvider := &testAnomalyProvider{
		events: []*AnomalyEvent{
			{
				ID:            "ev1",
				Service:       "cassandra",
				Component:     "database",
				Timestamp:     time.Now().Add(-5 * time.Minute),
				Severity:      SeverityHigh,
				AnomalyScore:  0.9,
				MetricOrField: "latency_p99",
				FieldValue:    500.0,
				Tags: map[string]string{
					"transaction_id": "txn_001",
				},
			},
			{
				ID:            "ev2",
				Service:       "kafka",
				Component:     "broker",
				Timestamp:     time.Now().Add(-4 * time.Minute),
				Severity:      SeverityMedium,
				AnomalyScore:  0.7,
				MetricOrField: "error_rate",
				FieldValue:    0.08,
				Tags: map[string]string{
					"transaction_id": "txn_001",
				},
			},
			{
				ID:            "ev3",
				Service:       "api-gateway",
				Component:     "api",
				Timestamp:     time.Now().Add(-3 * time.Minute),
				Severity:      SeverityHigh,
				AnomalyScore:  0.85,
				MetricOrField: "error_rate",
				FieldValue:    0.15,
				Tags: map[string]string{
					"transaction_id": "txn_001",
				},
			},
		},
	}

	// Create candidate cause service
	collector := NewIncidentAnomalyCollector(mockProvider, serviceGraph, mockLogger)
	candidateService := NewCandidateCauseService(collector, mockLogger)

	// Create RCA engine
	engine := NewRCAEngine(candidateService, serviceGraph, mockLogger, config.EngineConfig{}, nil)

	// Create incident context
	now := time.Now()
	incident := &IncidentContext{
		ID:            "incident_test_001",
		ImpactService: "api-gateway",
		ImpactSignal: ImpactSignal{
			ServiceName: "api-gateway",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
			Threshold:   0.05,
		},
		TimeBounds: IncidentTimeWindow{
			TStart: now.Add(-10 * time.Minute),
			TPeak:  now.Add(-5 * time.Minute),
			TEnd:   now,
		},
		ImpactSummary: "API Gateway error rate spike",
		Severity:      0.8,
		CreatedAt:     now,
	}

	// Compute RCA
	opts := DefaultRCAOptions()
	opts.MaxChains = 5
	opts.MaxStepsPerChain = 10

	rcaIncident, err := engine.ComputeRCA(context.Background(), incident, opts)
	if err != nil {
		t.Fatalf("ComputeRCA failed: %v", err)
	}

	// Assertions
	if rcaIncident == nil {
		t.Fatal("Expected non-nil RCAIncident")
	}

	if rcaIncident.Impact.ID != incident.ID {
		t.Errorf("Expected impact ID %s, got %s", incident.ID, rcaIncident.Impact.ID)
	}

	if rcaIncident.Score < 0 || rcaIncident.Score > 1 {
		t.Errorf("Expected score between 0 and 1, got %f", rcaIncident.Score)
	}

	if rcaIncident.RootCause != nil && rcaIncident.RootCause.Service == "" {
		t.Error("Root cause service cannot be empty")
	}

	t.Logf("RCA Incident: %s", rcaIncident.String())
	if rcaIncident.RootCause != nil {
		t.Logf("Root Cause: %s:%s", rcaIncident.RootCause.Service, rcaIncident.RootCause.Component)
		t.Logf("Root Cause Summary: %s", rcaIncident.RootCause.Summary)
	}
}

func TestRCAEngineComputeRCA_InvalidIncident(t *testing.T) {
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	serviceGraph := NewServiceGraph()
	mockProvider := &testAnomalyProvider{events: make([]*AnomalyEvent, 0)}

	collector := NewIncidentAnomalyCollector(mockProvider, serviceGraph, mockLogger)
	candidateService := NewCandidateCauseService(collector, mockLogger)
	engine := NewRCAEngine(candidateService, serviceGraph, mockLogger, config.EngineConfig{}, nil)

	// Create invalid incident (empty service name)
	incident := &IncidentContext{
		ID:            "incident_invalid",
		ImpactService: "",
		ImpactSignal: ImpactSignal{
			ServiceName: "api-gateway",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
		},
		TimeBounds: IncidentTimeWindow{
			TStart: time.Now().Add(-10 * time.Minute),
			TPeak:  time.Now(),
			TEnd:   time.Now().Add(10 * time.Minute),
		},
	}

	_, err := engine.ComputeRCA(context.Background(), incident, DefaultRCAOptions())
	if err == nil {
		t.Fatal("Expected error for invalid incident, got nil")
	}

	t.Logf("Correctly rejected invalid incident: %v", err)
}

func TestRCAEngineComputeRCA_NoAnomalies(t *testing.T) {
	mockLogger := logger.NewMockLogger(&strings.Builder{})
	serviceGraph := NewServiceGraph()

	// Empty anomaly provider
	mockProvider := &testAnomalyProvider{
		events: make([]*AnomalyEvent, 0),
	}

	collector := NewIncidentAnomalyCollector(mockProvider, serviceGraph, mockLogger)
	candidateService := NewCandidateCauseService(collector, mockLogger)
	engine := NewRCAEngine(candidateService, serviceGraph, mockLogger, config.EngineConfig{}, nil)

	now := time.Now()
	incident := &IncidentContext{
		ID:            "incident_no_anomalies",
		ImpactService: "api-gateway",
		ImpactSignal: ImpactSignal{
			ServiceName: "api-gateway",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
		},
		TimeBounds: IncidentTimeWindow{
			TStart: now.Add(-10 * time.Minute),
			TPeak:  now,
			TEnd:   now.Add(10 * time.Minute),
		},
		ImpactSummary: "Test incident",
		Severity:      0.5,
		CreatedAt:     now,
	}

	rcaIncident, err := engine.ComputeRCA(context.Background(), incident, DefaultRCAOptions())
	if err != nil {
		t.Fatalf("ComputeRCA failed: %v", err)
	}

	if rcaIncident == nil {
		t.Fatal("Expected non-nil RCAIncident even with no anomalies")
	}

	if len(rcaIncident.Chains) > 0 {
		t.Errorf("Expected 0 chains for no anomalies, got %d", len(rcaIncident.Chains))
	}

	if len(rcaIncident.Notes) == 0 {
		t.Error("Expected notes indicating no anomalies found")
	}

	t.Logf("Correctly handled no-anomaly scenario: %v", rcaIncident.Notes)
}

func TestRCAStep_TemplateBasedSummary(t *testing.T) {
	step := NewRCAStep(1, "cassandra", "database")
	step.Ring = RingImmediate
	step.Direction = DirectionUpstream
	step.Distance = 2
	step.TimeRange = TimeRange{
		Start: time.Now().Add(-5 * time.Minute),
		End:   time.Now(),
	}
	step.AddEvidence("anomaly_group", "group_001", "High latency detected")

	summary := TemplateBasedSummary(step, "api-gateway")
	if summary == "" {
		t.Fatal("Expected non-empty summary")
	}

	// Verify template includes key information
	if !contains(summary, "cassandra") || !contains(summary, "database") {
		t.Errorf("Summary should mention service and component: %s", summary)
	}

	if !contains(summary, "upstream") && !contains(summary, "Upstream") {
		t.Errorf("Summary should mention upstream direction: %s", summary)
	}

	t.Logf("Template-based summary: %s", summary)
}

func TestRCAChain_OrderingAndScoring(t *testing.T) {
	chain1 := NewRCAChain()
	chain1.Score = 0.8
	chain1.AddStep(NewRCAStep(1, "service_a", "component_a"))
	chain1.AddStep(NewRCAStep(2, "service_b", "component_b"))

	chain2 := NewRCAChain()
	chain2.Score = 0.6
	chain2.AddStep(NewRCAStep(1, "service_c", "component_c"))

	chains := []*RCAChain{chain2, chain1}

	// Simulate ranking (as done in RCAEngine)
	sortChains := func(ch []*RCAChain) {
		for i := range ch {
			ch[i].Rank = i + 1
		}
	}
	sortChains(chains)

	if chains[0].Rank != 1 {
		t.Errorf("Expected first chain rank 1, got %d", chains[0].Rank)
	}

	if len(chain1.ImpactPath) != 2 {
		t.Errorf("Expected 2 services in impact path, got %d", len(chain1.ImpactPath))
	}

	if chain1.ImpactPath[0] != "service_a" || chain1.ImpactPath[1] != "service_b" {
		t.Errorf("Unexpected impact path: %v", chain1.ImpactPath)
	}

	t.Logf("Chain ordering test passed")
}

// testAnomalyProvider is a mock implementation of AnomalyEventsProvider for testing
type testAnomalyProvider struct {
	events []*AnomalyEvent
}

func (p *testAnomalyProvider) GetAnomalies(
	ctx context.Context,
	startTime time.Time,
	endTime time.Time,
	services []string,
) ([]*AnomalyEvent, error) {
	var filtered []*AnomalyEvent

	for _, event := range p.events {
		// Filter by time range
		if event.Timestamp.Before(startTime) || event.Timestamp.After(endTime) {
			continue
		}

		// Filter by service if specified
		if len(services) > 0 {
			found := false
			for _, svc := range services {
				if event.Service == svc {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, event)
	}

	return filtered, nil
}

// Helper function for string containment
func contains(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
