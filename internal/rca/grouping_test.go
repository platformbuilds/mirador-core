package rca

import (
	"testing"
	"time"
)

// TestGroupEnrichedAnomalies_BasicGrouping tests basic grouping by service, component, ring, and time bucket.
func TestGroupEnrichedAnomalies_BasicGrouping(t *testing.T) {
	cfg := GroupingConfig{
		BucketWidth:       10 * time.Second,
		MinEventsPerGroup: 0,
		MinSeverity:       0,
		MinAnomalyScore:   0,
		GroupByComponent:  true,
	}

	// Create synthetic events
	now := time.Now()
	events := []*EnrichedAnomalyEvent{
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae1",
				Timestamp:    now,
				Service:      "service-a",
				Component:    "cache",
				Severity:     SeverityHigh,
				AnomalyScore: 0.8,
				Tags:         map[string]string{"transaction_id": "tx1"},
			},
			Ring:                  RingImmediate,
			GraphDirection:        DirectionUpstream,
			GraphDistanceToImpact: 1,
			ImpactService:         "service-b",
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae2",
				Timestamp:    now.Add(3 * time.Second),
				Service:      "service-a",
				Component:    "cache",
				Severity:     SeverityHigh,
				AnomalyScore: 0.75,
				Tags:         map[string]string{"transaction_id": "tx2"},
			},
			Ring:                  RingImmediate,
			GraphDirection:        DirectionUpstream,
			GraphDistanceToImpact: 1,
			ImpactService:         "service-b",
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae3",
				Timestamp:    now.Add(25 * time.Second),
				Service:      "service-a",
				Component:    "cache",
				Severity:     SeverityMedium,
				AnomalyScore: 0.6,
				Tags:         map[string]string{"transaction_id": "tx3"},
			},
			Ring:                  RingShort,
			GraphDirection:        DirectionUpstream,
			GraphDistanceToImpact: 1,
			ImpactService:         "service-b",
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae4",
				Timestamp:    now,
				Service:      "service-c",
				Component:    "database",
				Severity:     SeverityMedium,
				AnomalyScore: 0.5,
				Tags:         map[string]string{"transaction_id": "tx4"},
			},
			Ring:                  RingImmediate,
			GraphDirection:        DirectionDownstream,
			GraphDistanceToImpact: 2,
			ImpactService:         "service-b",
		},
	}

	// Group events
	groups := GroupEnrichedAnomalies(events, cfg)

	// Validate grouping
	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	// Find the service-a/cache/R1 group
	var groupACache *AnomalyGroup
	for _, g := range groups {
		if g.Service == "service-a" && g.Component == "cache" && g.Ring == RingImmediate {
			groupACache = g
			break
		}
	}

	if groupACache == nil {
		t.Fatalf("Expected group service-a/cache/R1 not found")
	}

	// Verify aggregated stats
	if groupACache.EventCount != 2 {
		t.Errorf("Expected 2 events in group, got %d", groupACache.EventCount)
	}

	if groupACache.DistinctTxnCount != 2 {
		t.Errorf("Expected 2 distinct transactions, got %d", groupACache.DistinctTxnCount)
	}

	if groupACache.MaxSeverity != SeverityHigh {
		t.Errorf("Expected max severity High, got %v", groupACache.MaxSeverity)
	}

	if groupACache.AvgSeverity != SeverityHigh { // (High + High) / 2 = High
		t.Errorf("Expected avg severity High, got %v", groupACache.AvgSeverity)
	}

	if groupACache.MaxScore != 0.8 {
		t.Errorf("Expected max score 0.8, got %.2f", groupACache.MaxScore)
	}

	expectedAvgScore := (0.8 + 0.75) / 2.0
	if groupACache.AvgScore != expectedAvgScore {
		t.Errorf("Expected avg score %.2f, got %.2f", expectedAvgScore, groupACache.AvgScore)
	}

	if groupACache.GraphDirection != DirectionUpstream {
		t.Errorf("Expected direction Upstream, got %v", groupACache.GraphDirection)
	}

	if groupACache.MinGraphDistance != 1 || groupACache.MaxGraphDistance != 1 {
		t.Errorf("Expected distance 1..1, got %d..%d", groupACache.MinGraphDistance, groupACache.MaxGraphDistance)
	}
}

// TestGroupEnrichedAnomalies_TimeBucketing tests that time bucketing works correctly.
func TestGroupEnrichedAnomalies_TimeBucketing(t *testing.T) {
	cfg := GroupingConfig{
		BucketWidth:       10 * time.Second,
		MinEventsPerGroup: 0,
		MinSeverity:       0,
		MinAnomalyScore:   0,
		GroupByComponent:  false, // Ignore component
	}

	// Use a fixed time to avoid nanosecond precision issues with time.Now()
	now := time.Date(2025, 11, 17, 12, 0, 0, 0, time.UTC)
	events := []*EnrichedAnomalyEvent{
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae1",
				Timestamp:    now,
				Service:      "svc",
				Component:    "comp",
				Severity:     SeverityHigh,
				AnomalyScore: 0.8,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionUpstream,
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae2",
				Timestamp:    now.Add(5 * time.Second), // Same bucket
				Service:      "svc",
				Component:    "comp",
				Severity:     SeverityHigh,
				AnomalyScore: 0.7,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionUpstream,
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae3",
				Timestamp:    now.Add(15 * time.Second), // Different bucket
				Service:      "svc",
				Component:    "comp",
				Severity:     SeverityMedium,
				AnomalyScore: 0.6,
			},
			Ring:           RingShort,
			GraphDirection: DirectionUpstream,
		},
	}

	groups := GroupEnrichedAnomalies(events, cfg)

	// Should have 2 groups:
	// - Group 1: (svc, Immediate, bucket@now) contains ae1 and ae2
	// - Group 2: (svc, Short, bucket@now+10s) contains ae3
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups (different time buckets + rings), got %d", len(groups))
		for i, g := range groups {
			t.Logf("  Group %d: Service=%s, Ring=%s, Events=%d", i, g.Service, g.Ring, g.EventCount)
		}
	}
}

// TestGroupEnrichedAnomalies_MixedGraphDirections tests that dominant direction is selected.
func TestGroupEnrichedAnomalies_MixedGraphDirections(t *testing.T) {
	cfg := GroupingConfig{
		BucketWidth:       10 * time.Second,
		MinEventsPerGroup: 0,
		MinSeverity:       0,
		MinAnomalyScore:   0,
		GroupByComponent:  true,
	}

	now := time.Now()
	events := []*EnrichedAnomalyEvent{
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae1",
				Timestamp:    now,
				Service:      "svc",
				Component:    "comp",
				Severity:     SeverityHigh,
				AnomalyScore: 0.8,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionUpstream,
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae2",
				Timestamp:    now.Add(2 * time.Second),
				Service:      "svc",
				Component:    "comp",
				Severity:     SeverityHigh,
				AnomalyScore: 0.7,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionDownstream,
		},
	}

	groups := GroupEnrichedAnomalies(events, cfg)

	if len(groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(groups))
	}

	// Direction should be Upstream (preferring upstream when mixed)
	if groups[0].GraphDirection != DirectionUpstream {
		t.Errorf("Expected direction Upstream (preferred in mixed), got %v", groups[0].GraphDirection)
	}
}

// TestGroupEnrichedAnomalies_FilteringByMinEventsPerGroup tests filtering.
func TestGroupEnrichedAnomalies_FilteringByMinEventsPerGroup(t *testing.T) {
	cfg := GroupingConfig{
		BucketWidth:       10 * time.Second,
		MinEventsPerGroup: 2,
		MinSeverity:       0,
		MinAnomalyScore:   0,
		GroupByComponent:  true,
	}

	now := time.Now()
	events := []*EnrichedAnomalyEvent{
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae1",
				Timestamp:    now,
				Service:      "svc-a",
				Component:    "comp-a",
				Severity:     SeverityHigh,
				AnomalyScore: 0.8,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionUpstream,
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae2",
				Timestamp:    now.Add(2 * time.Second),
				Service:      "svc-a",
				Component:    "comp-a",
				Severity:     SeverityHigh,
				AnomalyScore: 0.7,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionUpstream,
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae3",
				Timestamp:    now,
				Service:      "svc-b",
				Component:    "comp-b",
				Severity:     SeverityMedium,
				AnomalyScore: 0.5,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionDownstream,
		},
	}

	groups := GroupEnrichedAnomalies(events, cfg)

	// Should only have 1 group (svc-a/comp-a with 2 events)
	// svc-b/comp-b has only 1 event, should be filtered out
	if len(groups) != 1 {
		t.Errorf("Expected 1 group after filtering, got %d", len(groups))
	}

	if groups[0].Service != "svc-a" {
		t.Errorf("Expected svc-a, got %s", groups[0].Service)
	}
}

// TestGroupEnrichedAnomalies_FilteringByMinSeverity tests severity filtering.
func TestGroupEnrichedAnomalies_FilteringByMinSeverity(t *testing.T) {
	cfg := GroupingConfig{
		BucketWidth:       10 * time.Second,
		MinEventsPerGroup: 0,
		MinSeverity:       SeverityMedium,
		MinAnomalyScore:   0,
		GroupByComponent:  true,
	}

	now := time.Now()
	events := []*EnrichedAnomalyEvent{
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae1",
				Timestamp:    now,
				Service:      "svc-a",
				Component:    "comp-a",
				Severity:     SeverityHigh,
				AnomalyScore: 0.8,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionUpstream,
		},
		{
			AnomalyEvent: &AnomalyEvent{
				ID:           "ae2",
				Timestamp:    now,
				Service:      "svc-b",
				Component:    "comp-b",
				Severity:     SeverityLow,
				AnomalyScore: 0.3,
			},
			Ring:           RingImmediate,
			GraphDirection: DirectionDownstream,
		},
	}

	groups := GroupEnrichedAnomalies(events, cfg)

	// Should only have svc-a group (High severity passes, Low is filtered)
	if len(groups) != 1 {
		t.Errorf("Expected 1 group after filtering, got %d", len(groups))
	}

	if groups[0].Service != "svc-a" {
		t.Errorf("Expected svc-a, got %s", groups[0].Service)
	}
}

// TestGroupEnrichedAnomalies_EmptyInput tests handling of empty input.
func TestGroupEnrichedAnomalies_EmptyInput(t *testing.T) {
	cfg := DefaultGroupingConfig()
	groups := GroupEnrichedAnomalies([]*EnrichedAnomalyEvent{}, cfg)

	if len(groups) != 0 {
		t.Errorf("Expected 0 groups for empty input, got %d", len(groups))
	}
}

// TestAnomalyGroup_FinalizeStats tests stat computation.
func TestAnomalyGroup_FinalizeStats(t *testing.T) {
	group := NewAnomalyGroup("svc", "comp", RingImmediate)

	now := time.Now()
	events := []*AnomalyEvent{
		{
			ID:           "ae1",
			Timestamp:    now,
			Severity:     SeverityHigh,
			AnomalyScore: 0.8,
			Tags:         map[string]string{"transaction_id": "tx1"},
		},
		{
			ID:           "ae2",
			Timestamp:    now.Add(2 * time.Second),
			Severity:     SeverityMedium,
			AnomalyScore: 0.6,
			Tags:         map[string]string{"transaction_id": "tx2"},
		},
		{
			ID:           "ae3",
			Timestamp:    now.Add(1 * time.Second),
			Severity:     SeverityCritical,
			AnomalyScore: 0.9,
			Tags:         map[string]string{"transaction_id": "tx1"}, // Duplicate txn
		},
	}

	for _, evt := range events {
		group.AddEvent(&EnrichedAnomalyEvent{
			AnomalyEvent: evt,
			Ring:         RingImmediate,
		})
	}

	group.FinalizeStats()

	if group.EventCount != 3 {
		t.Errorf("Expected 3 events, got %d", group.EventCount)
	}

	if group.DistinctTxnCount != 2 {
		t.Errorf("Expected 2 distinct transactions, got %d", group.DistinctTxnCount)
	}

	if group.MaxSeverity != SeverityCritical {
		t.Errorf("Expected max severity Critical, got %v", group.MaxSeverity)
	}

	if group.MaxScore != 0.9 {
		t.Errorf("Expected max score 0.9, got %.2f", group.MaxScore)
	}

	// AvgSeverity: (High + Medium + Critical) / 3 = (0.75 + 0.5 + 1.0) / 3 ≈ 0.75
	expectedAvgSev := Severity((float64(SeverityHigh) + float64(SeverityMedium) + float64(SeverityCritical)) / 3.0)
	if group.AvgSeverity != expectedAvgSev {
		t.Errorf("Expected avg severity %.4f, got %.4f", expectedAvgSev, group.AvgSeverity)
	}

	// AvgScore: (0.8 + 0.6 + 0.9) / 3 ≈ 0.7667
	expectedAvgScore := (0.8 + 0.6 + 0.9) / 3.0
	const epsilon = 1e-6
	if diff := group.AvgScore - expectedAvgScore; diff < 0 && diff < -epsilon || diff > 0 && diff > epsilon {
		t.Errorf("Expected avg score %.6f, got %.6f", expectedAvgScore, group.AvgScore)
	}
}
