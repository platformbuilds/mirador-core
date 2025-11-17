package rca

import (
	"testing"
	"time"
)

// TestScoreCandidate_TemporalProximity tests that ring proximity affects score.
func TestScoreCandidate_TemporalProximity(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}
	cfg := DefaultScoringConfig()

	// Create groups in different rings with identical other properties
	groupR1 := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		DistinctTxnCount: 0,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	groupR2 := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingShort,
		EventCount:       1,
		DistinctTxnCount: 0,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	scoreR1 := ScoreCandidate(groupR1, incident, cfg)
	scoreR2 := ScoreCandidate(groupR2, incident, cfg)

	if scoreR1.Score <= scoreR2.Score {
		t.Errorf("Expected R1 score > R2 score, got R1=%.4f, R2=%.4f", scoreR1.Score, scoreR2.Score)
	}
}

// TestScoreCandidate_GraphDirection tests that upstream is scored higher than downstream.
func TestScoreCandidate_GraphDirection(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}
	cfg := DefaultScoringConfig()

	groupUpstream := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	groupDownstream := &AnomalyGroup{
		Service:          "service-c",
		Component:        "api",
		Ring:             RingImmediate,
		EventCount:       1,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionDownstream,
		MinGraphDistance: 2,
		MaxGraphDistance: 2,
	}

	scoreUp := ScoreCandidate(groupUpstream, incident, cfg)
	scoreDown := ScoreCandidate(groupDownstream, incident, cfg)

	if scoreUp.Score <= scoreDown.Score {
		t.Errorf("Expected upstream score > downstream score, got Up=%.4f, Down=%.4f", scoreUp.Score, scoreDown.Score)
	}
}

// TestScoreCandidate_GraphDistance tests that closer distance scores higher.
func TestScoreCandidate_GraphDistance(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}
	cfg := DefaultScoringConfig()

	group1Hop := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	group3Hops := &AnomalyGroup{
		Service:          "service-x",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 3,
		MaxGraphDistance: 3,
	}

	score1 := ScoreCandidate(group1Hop, incident, cfg)
	score3 := ScoreCandidate(group3Hops, incident, cfg)

	if score1.Score <= score3.Score {
		t.Errorf("Expected 1-hop score > 3-hop score, got 1hop=%.4f, 3hop=%.4f", score1.Score, score3.Score)
	}
}

// TestScoreCandidate_Severity tests that higher severity scores higher.
func TestScoreCandidate_Severity(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}
	cfg := DefaultScoringConfig()

	groupHighSev := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		MaxSeverity:      SeverityCritical,
		AvgSeverity:      SeverityCritical,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	groupLowSev := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		MaxSeverity:      SeverityLow,
		AvgSeverity:      SeverityLow,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	scoreHigh := ScoreCandidate(groupHighSev, incident, cfg)
	scoreLow := ScoreCandidate(groupLowSev, incident, cfg)

	if scoreHigh.Score <= scoreLow.Score {
		t.Errorf("Expected high-severity score > low-severity score, got High=%.4f, Low=%.4f", scoreHigh.Score, scoreLow.Score)
	}
}

// TestScoreCandidate_AnomalyScore tests that higher anomaly scores score higher.
func TestScoreCandidate_AnomalyScore(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}
	cfg := DefaultScoringConfig()

	groupHighScore := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.95,
		AvgScore:         0.95,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	groupLowScore := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.3,
		AvgScore:         0.3,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	scoreHigh := ScoreCandidate(groupHighScore, incident, cfg)
	scoreLow := ScoreCandidate(groupLowScore, incident, cfg)

	if scoreHigh.Score <= scoreLow.Score {
		t.Errorf("Expected high-score > low-score, got High=%.4f, Low=%.4f", scoreHigh.Score, scoreLow.Score)
	}
}

// TestScoreCandidate_TransactionCount tests that wider impact scores higher.
func TestScoreCandidate_TransactionCount(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}
	cfg := DefaultScoringConfig()

	groupWideTxn := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       10,
		DistinctTxnCount: 500,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	groupNarrowTxn := &AnomalyGroup{
		Service:          "service-a",
		Component:        "cache",
		Ring:             RingImmediate,
		EventCount:       1,
		DistinctTxnCount: 0,
		MaxSeverity:      SeverityHigh,
		AvgSeverity:      SeverityHigh,
		MaxScore:         0.8,
		AvgScore:         0.8,
		GraphDirection:   DirectionUpstream,
		MinGraphDistance: 1,
		MaxGraphDistance: 1,
	}

	scoreWide := ScoreCandidate(groupWideTxn, incident, cfg)
	scoreNarrow := ScoreCandidate(groupNarrowTxn, incident, cfg)

	if scoreWide.Score <= scoreNarrow.Score {
		t.Errorf("Expected wide-txn score > narrow-txn score, got Wide=%.4f, Narrow=%.4f", scoreWide.Score, scoreNarrow.Score)
	}
}

// TestRankCandidateCauses_SortingAndRanking tests that candidates are properly sorted and ranked.
func TestRankCandidateCauses_SortingAndRanking(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}
	cfg := DefaultScoringConfig()

	// Create 3 groups with different scores
	groups := []*AnomalyGroup{
		{
			Service:          "service-a",
			Component:        "cache",
			Ring:             RingShort, // Less immediate
			EventCount:       1,
			MaxSeverity:      SeverityMedium,
			AvgSeverity:      SeverityMedium,
			MaxScore:         0.5,
			AvgScore:         0.5,
			GraphDirection:   DirectionDownstream,
			MinGraphDistance: 2,
			MaxGraphDistance: 2,
		},
		{
			Service:          "service-x",
			Component:        "api",
			Ring:             RingImmediate, // Immediate
			EventCount:       5,
			DistinctTxnCount: 100,
			MaxSeverity:      SeverityCritical,
			AvgSeverity:      SeverityCritical,
			MaxScore:         0.95,
			AvgScore:         0.95,
			GraphDirection:   DirectionUpstream,
			MinGraphDistance: 1,
			MaxGraphDistance: 1,
		},
		{
			Service:          "service-c",
			Component:        "db",
			Ring:             RingImmediate, // Immediate
			EventCount:       2,
			DistinctTxnCount: 10,
			MaxSeverity:      SeverityHigh,
			AvgSeverity:      SeverityHigh,
			MaxScore:         0.75,
			AvgScore:         0.75,
			GraphDirection:   DirectionSame,
			MinGraphDistance: 0,
			MaxGraphDistance: 0,
		},
	}

	ranked := RankCandidateCauses(groups, incident, cfg)

	if len(ranked) != 3 {
		t.Errorf("Expected 3 ranked candidates, got %d", len(ranked))
	}

	// Check ranks are assigned
	for i, candidate := range ranked {
		if candidate.Rank != i+1 {
			t.Errorf("Candidate %d has rank %d, expected %d", i, candidate.Rank, i+1)
		}
	}

	// Check sorting (scores should be descending)
	for i := 0; i < len(ranked)-1; i++ {
		if ranked[i].Score < ranked[i+1].Score {
			t.Errorf("Candidates not sorted by score: rank %d score %.4f < rank %d score %.4f",
				ranked[i].Rank, ranked[i].Score, ranked[i+1].Rank, ranked[i+1].Score)
		}
	}

	// Top ranked should be service-x (most suspicious)
	if ranked[0].Group.Service != "service-x" {
		t.Errorf("Expected top rank to be service-x, got %s", ranked[0].Group.Service)
	}
}

// TestRankCandidateCauses_MaxCandidatesLimit tests that max candidates limit is applied.
func TestRankCandidateCauses_MaxCandidatesLimit(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}

	cfg := DefaultScoringConfig()
	cfg.MaxCandidatesToReturn = 2

	groups := []*AnomalyGroup{
		{
			Service:        "service-1",
			Component:      "comp",
			Ring:           RingImmediate,
			EventCount:     1,
			MaxSeverity:    SeverityHigh,
			AvgSeverity:    SeverityHigh,
			MaxScore:       0.8,
			AvgScore:       0.8,
			GraphDirection: DirectionUpstream,
		},
		{
			Service:        "service-2",
			Component:      "comp",
			Ring:           RingImmediate,
			EventCount:     1,
			MaxSeverity:    SeverityHigh,
			AvgSeverity:    SeverityHigh,
			MaxScore:       0.7,
			AvgScore:       0.7,
			GraphDirection: DirectionUpstream,
		},
		{
			Service:        "service-3",
			Component:      "comp",
			Ring:           RingImmediate,
			EventCount:     1,
			MaxSeverity:    SeverityHigh,
			AvgSeverity:    SeverityHigh,
			MaxScore:       0.6,
			AvgScore:       0.6,
			GraphDirection: DirectionUpstream,
		},
	}

	ranked := RankCandidateCauses(groups, incident, cfg)

	if len(ranked) != 2 {
		t.Errorf("Expected max 2 candidates, got %d", len(ranked))
	}
}

// TestRankCandidateCauses_DeterministicTiebreaking tests that ties are broken deterministically.
func TestRankCandidateCauses_DeterministicTiebreaking(t *testing.T) {
	now := time.Now()
	incident := &IncidentContext{
		ID:            "inc1",
		ImpactService: "service-b",
		TimeBounds: IncidentTimeWindow{
			TStart: now,
			TPeak:  now.Add(10 * time.Second),
			TEnd:   now.Add(20 * time.Second),
		},
		Severity: 0.8,
	}
	cfg := DefaultScoringConfig()

	// Create two groups with identical scoring parameters
	groups := []*AnomalyGroup{
		{
			Service:          "service-z",
			Component:        "comp-z",
			Ring:             RingImmediate,
			EventCount:       1,
			MaxSeverity:      SeverityHigh,
			AvgSeverity:      SeverityHigh,
			MaxScore:         0.8,
			AvgScore:         0.8,
			GraphDirection:   DirectionUpstream,
			MinGraphDistance: 1,
			MaxGraphDistance: 1,
		},
		{
			Service:          "service-a",
			Component:        "comp-a",
			Ring:             RingImmediate,
			EventCount:       1,
			MaxSeverity:      SeverityHigh,
			AvgSeverity:      SeverityHigh,
			MaxScore:         0.8,
			AvgScore:         0.8,
			GraphDirection:   DirectionUpstream,
			MinGraphDistance: 1,
			MaxGraphDistance: 1,
		},
	}

	ranked := RankCandidateCauses(groups, incident, cfg)

	// Should be sorted by service name (service-a < service-z)
	if ranked[0].Group.Service != "service-a" {
		t.Errorf("Expected first rank service-a, got %s", ranked[0].Group.Service)
	}

	if ranked[1].Group.Service != "service-z" {
		t.Errorf("Expected second rank service-z, got %s", ranked[1].Group.Service)
	}
}
