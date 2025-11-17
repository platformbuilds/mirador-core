package rca

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// ========================
// Mock Implementations for Testing
// ========================

// MockAnomalyEventsProvider is a test mock for AnomalyEventsProvider.
type MockAnomalyEventsProvider struct {
	anomalies []*AnomalyEvent
	err       error
}

func (m *MockAnomalyEventsProvider) GetAnomalies(
	ctx context.Context,
	startTime time.Time,
	endTime time.Time,
	services []string,
) ([]*AnomalyEvent, error) {
	if m.err != nil {
		return nil, m.err
	}

	var filtered []*AnomalyEvent
	for _, a := range m.anomalies {
		if a.Timestamp.Before(startTime) || a.Timestamp.After(endTime) {
			continue
		}

		if len(services) > 0 {
			inServices := false
			for _, svc := range services {
				if a.Service == svc {
					inServices = true
					break
				}
			}
			if !inServices {
				continue
			}
		}

		filtered = append(filtered, a)
	}

	return filtered, nil
}

// ========================
// IncidentAnomalyCollector Tests
// ========================

func TestIncidentAnomalyCollector_Collect_Basic(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")

	// Build a simple service graph.
	graph := NewServiceGraph()
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("tps"),
		Target: ServiceNode("cassandra"),
	})

	// Create an incident.
	baseTime := time.Now()
	incident := &IncidentContext{
		ID:            "inc-1",
		ImpactService: "tps",
		ImpactSignal: ImpactSignal{
			ServiceName: "tps",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
			Threshold:   0.1,
		},
		TimeBounds: IncidentTimeWindow{
			TStart: baseTime.Add(-2 * time.Minute),
			TPeak:  baseTime,
			TEnd:   baseTime.Add(2 * time.Minute),
		},
		Severity:  0.8,
		CreatedAt: time.Now(),
	}

	// Create synthetic anomalies.
	anomalies := []*AnomalyEvent{
		// Upstream anomaly (api-gateway) immediately before peak.
		{
			ID:            "anom-1",
			Timestamp:     baseTime.Add(-2 * time.Second),
			Service:       "api-gateway",
			Component:     "network",
			SignalType:    SignalTypeMetrics,
			MetricOrField: "latency",
			Severity:      SeverityHigh,
			AnomalyScore:  0.8,
		},
		// Same service (tps) at peak.
		{
			ID:            "anom-2",
			Timestamp:     baseTime,
			Service:       "tps",
			Component:     "database",
			SignalType:    SignalTypeMetrics,
			MetricOrField: "connection_count",
			Severity:      SeverityHigh,
			AnomalyScore:  0.9,
		},
		// Downstream anomaly (cassandra) after peak.
		{
			ID:            "anom-3",
			Timestamp:     baseTime.Add(1 * time.Minute),
			Service:       "cassandra",
			Component:     "disk",
			SignalType:    SignalTypeMetrics,
			MetricOrField: "disk_io",
			Severity:      SeverityMedium,
			AnomalyScore:  0.6,
		},
		// Out-of-scope anomaly (far before incident).
		{
			ID:            "anom-4",
			Timestamp:     baseTime.Add(-15 * time.Minute),
			Service:       "api-gateway",
			Component:     "network",
			SignalType:    SignalTypeMetrics,
			MetricOrField: "latency",
			Severity:      SeverityHigh,
			AnomalyScore:  0.8,
		},
	}

	provider := &MockAnomalyEventsProvider{anomalies: anomalies}
	collector := NewIncidentAnomalyCollector(provider, graph, log)

	opts := DefaultCollectOptions()
	enriched, err := collector.Collect(ctx, incident, opts)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have collected 3 anomalies (anom-1, 2, 3; not anom-4 which is out of scope).
	if len(enriched) < 2 {
		t.Errorf("Expected at least 2 enriched anomalies, got %d", len(enriched))
	}

	// Verify graph relationships.
	for _, eae := range enriched {
		if eae.AnomalyEvent.ID == "anom-1" {
			if eae.GraphDirection != DirectionUpstream {
				t.Errorf("anom-1 should be upstream, got %s", eae.GraphDirection)
			}
			if eae.Ring != RingImmediate {
				t.Errorf("anom-1 should be in immediate ring, got %s", eae.Ring)
			}
		}

		if eae.AnomalyEvent.ID == "anom-2" {
			if eae.GraphDirection != DirectionSame {
				t.Errorf("anom-2 should be same service, got %s", eae.GraphDirection)
			}
		}

		if eae.AnomalyEvent.ID == "anom-3" {
			if eae.GraphDirection != DirectionDownstream {
				t.Errorf("anom-3 should be downstream, got %s", eae.GraphDirection)
			}
		}
	}

	t.Logf("Collected %d enriched anomalies", len(enriched))
	for _, eae := range enriched {
		t.Logf("  %s: ring=%s, direction=%s, distance=%d",
			eae.AnomalyEvent.ID, eae.Ring, eae.GraphDirection, eae.GraphDistanceToImpact)
	}
}

func TestIncidentAnomalyCollector_Collect_WithFiltering(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")

	graph := NewServiceGraph()
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})

	baseTime := time.Now()
	incident := &IncidentContext{
		ID:            "inc-1",
		ImpactService: "tps",
		ImpactSignal: ImpactSignal{
			ServiceName: "tps",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
		},
		TimeBounds: IncidentTimeWindow{
			TStart: baseTime.Add(-1 * time.Minute),
			TPeak:  baseTime,
			TEnd:   baseTime.Add(1 * time.Minute),
		},
		Severity:  0.8,
		CreatedAt: time.Now(),
	}

	anomalies := []*AnomalyEvent{
		{
			ID:           "high-sev",
			Timestamp:    baseTime,
			Service:      "tps",
			Severity:     SeverityHigh,
			AnomalyScore: 0.9,
			SignalType:   SignalTypeMetrics,
		},
		{
			ID:           "low-sev",
			Timestamp:    baseTime.Add(1 * time.Second),
			Service:      "tps",
			Severity:     SeverityLow,
			AnomalyScore: 0.3,
			SignalType:   SignalTypeMetrics,
		},
	}

	provider := &MockAnomalyEventsProvider{anomalies: anomalies}
	collector := NewIncidentAnomalyCollector(provider, graph, log)

	// Filter for high severity only.
	opts := DefaultCollectOptions()
	opts.MinSeverity = SeverityHigh

	enriched, err := collector.Collect(ctx, incident, opts)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should only have the high-severity anomaly.
	if len(enriched) != 1 {
		t.Errorf("Expected 1 anomaly after filtering, got %d", len(enriched))
	}

	if enriched[0].AnomalyEvent.ID != "high-sev" {
		t.Errorf("Expected high-sev anomaly, got %s", enriched[0].AnomalyEvent.ID)
	}
}

func TestIncidentAnomalyCollector_Collect_WithTimeRings(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")

	graph := NewServiceGraph()
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})

	baseTime := time.Now()
	incident := &IncidentContext{
		ID:            "inc-1",
		ImpactService: "tps",
		ImpactSignal: ImpactSignal{
			ServiceName: "tps",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
		},
		TimeBounds: IncidentTimeWindow{
			TStart: baseTime.Add(-10 * time.Minute),
			TPeak:  baseTime,
			TEnd:   baseTime.Add(2 * time.Minute),
		},
		Severity:  0.8,
		CreatedAt: time.Now(),
	}

	// Create anomalies in different time rings.
	anomalies := []*AnomalyEvent{
		{
			ID:           "r1",
			Timestamp:    baseTime.Add(-2 * time.Second),
			Service:      "tps",
			Severity:     SeverityHigh,
			AnomalyScore: 0.8,
			SignalType:   SignalTypeMetrics,
		},
		{
			ID:           "r2",
			Timestamp:    baseTime.Add(-20 * time.Second),
			Service:      "tps",
			Severity:     SeverityHigh,
			AnomalyScore: 0.8,
			SignalType:   SignalTypeMetrics,
		},
		{
			ID:           "r3",
			Timestamp:    baseTime.Add(-1 * time.Minute),
			Service:      "tps",
			Severity:     SeverityHigh,
			AnomalyScore: 0.8,
			SignalType:   SignalTypeMetrics,
		},
		{
			ID:           "r4",
			Timestamp:    baseTime.Add(-5 * time.Minute),
			Service:      "tps",
			Severity:     SeverityHigh,
			AnomalyScore: 0.8,
			SignalType:   SignalTypeMetrics,
		},
	}

	provider := &MockAnomalyEventsProvider{anomalies: anomalies}
	collector := NewIncidentAnomalyCollector(provider, graph, log)

	opts := DefaultCollectOptions()
	enriched, err := collector.Collect(ctx, incident, opts)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(enriched) != 4 {
		t.Errorf("Expected 4 enriched anomalies, got %d", len(enriched))
	}

	// Verify ring assignments.
	rings := make(map[string]TimeRing)
	for _, eae := range enriched {
		rings[eae.AnomalyEvent.ID] = eae.Ring
	}

	if ring, ok := rings["r1"]; !ok || ring != RingImmediate {
		t.Errorf("Expected r1 in RingImmediate, got %s", ring)
	}
	if ring, ok := rings["r2"]; !ok || ring != RingShort {
		t.Errorf("Expected r2 in RingShort, got %s", ring)
	}
	if ring, ok := rings["r3"]; !ok || ring != RingMedium {
		t.Errorf("Expected r3 in RingMedium, got %s", ring)
	}
	if ring, ok := rings["r4"]; !ok || ring != RingLong {
		t.Errorf("Expected r4 in RingLong, got %s", ring)
	}
}

func TestIncidentAnomalyCollector_Collect_InvalidIncident(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")

	graph := NewServiceGraph()
	provider := &MockAnomalyEventsProvider{anomalies: []*AnomalyEvent{}}
	collector := NewIncidentAnomalyCollector(provider, graph, log)

	// Invalid incident (missing fields).
	invalidIncident := &IncidentContext{
		ID: "",
	}

	_, err := collector.Collect(ctx, invalidIncident, DefaultCollectOptions())
	if err == nil {
		t.Error("Expected error for invalid incident, got nil")
	}
}

func TestIncidentAnomalyCollector_Sorting(t *testing.T) {
	ctx := context.Background()
	log := logger.New("info")

	graph := NewServiceGraph()
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})

	baseTime := time.Now()
	incident := &IncidentContext{
		ID:            "inc-1",
		ImpactService: "tps",
		ImpactSignal: ImpactSignal{
			ServiceName: "tps",
			MetricName:  "error_rate",
			Direction:   "higher_is_worse",
		},
		TimeBounds: IncidentTimeWindow{
			TStart: baseTime.Add(-10 * time.Minute),
			TPeak:  baseTime,
			TEnd:   baseTime.Add(2 * time.Minute),
		},
		Severity:  0.8,
		CreatedAt: time.Now(),
	}

	// Create anomalies with varying rings and severities.
	anomalies := []*AnomalyEvent{
		{
			ID:           "r4-low",
			Timestamp:    baseTime.Add(-5 * time.Minute),
			Service:      "tps",
			Severity:     SeverityLow,
			AnomalyScore: 0.3,
			SignalType:   SignalTypeMetrics,
		},
		{
			ID:           "r1-high",
			Timestamp:    baseTime.Add(-1 * time.Second),
			Service:      "tps",
			Severity:     SeverityHigh,
			AnomalyScore: 0.9,
			SignalType:   SignalTypeMetrics,
		},
		{
			ID:           "r2-high",
			Timestamp:    baseTime.Add(-20 * time.Second),
			Service:      "tps",
			Severity:     SeverityHigh,
			AnomalyScore: 0.8,
			SignalType:   SignalTypeMetrics,
		},
	}

	provider := &MockAnomalyEventsProvider{anomalies: anomalies}
	collector := NewIncidentAnomalyCollector(provider, graph, log)

	opts := DefaultCollectOptions()
	opts.SortByRingAndPriority = true

	enriched, err := collector.Collect(ctx, incident, opts)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// After sorting, r1-high should be first, then r2-high, then r4-low.
	if len(enriched) != 3 {
		t.Errorf("Expected 3 anomalies, got %d", len(enriched))
	}

	if enriched[0].AnomalyEvent.ID != "r1-high" {
		t.Errorf("Expected first to be r1-high, got %s", enriched[0].AnomalyEvent.ID)
	}

	if enriched[1].AnomalyEvent.ID != "r2-high" {
		t.Errorf("Expected second to be r2-high, got %s", enriched[1].AnomalyEvent.ID)
	}

	if enriched[2].AnomalyEvent.ID != "r4-low" {
		t.Errorf("Expected third to be r4-low, got %s", enriched[2].AnomalyEvent.ID)
	}
}
