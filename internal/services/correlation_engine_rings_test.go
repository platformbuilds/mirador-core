package services

import (
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
)

func assertValidRings(t *testing.T, tr models.TimeRange, rings []models.TimeRange) {
	t.Helper()
	for i, r := range rings {
		if r.Start.Before(tr.Start) {
			t.Fatalf("ring[%d] start before tr.Start: ring.Start=%v tr.Start=%v", i, r.Start, tr.Start)
		}
		if r.End.After(tr.End) {
			t.Fatalf("ring[%d] end after tr.End: ring.End=%v tr.End=%v", i, r.End, tr.End)
		}
		if !r.Start.Before(r.End) {
			t.Fatalf("ring[%d] has non-positive duration: start=%v end=%v", i, r.Start, r.End)
		}
	}
}

func TestBuildRings_PreRingsTruncation(t *testing.T) {
	// Fixed time window: 0..60s
	tr := models.TimeRange{
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
	}

	cfg := config.EngineConfig{}
	cfg.Buckets.CoreWindowSize = 10 * time.Second
	cfg.Buckets.RingStep = 40 * time.Second // large step that would push rings outside tr
	cfg.Buckets.PreRings = 3
	cfg.Buckets.PostRings = 0

	rings := BuildRings(tr, cfg)

	// Basic sanity
	if len(rings) == 0 {
		t.Fatalf("expected at least core ring")
	}

	// All rings must be within tr and valid
	assertValidRings(t, tr, rings)

	// Count should not exceed configured total
	if len(rings) > cfg.Buckets.PreRings+1+cfg.Buckets.PostRings {
		t.Fatalf("unexpectedly many rings: got=%d max_allowed=%d", len(rings), cfg.Buckets.PreRings+1+cfg.Buckets.PostRings)
	}
}

func TestBuildRings_PostRingsTruncation(t *testing.T) {
	tr := models.TimeRange{
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
	}

	cfg := config.EngineConfig{}
	cfg.Buckets.CoreWindowSize = 10 * time.Second
	cfg.Buckets.RingStep = 40 * time.Second // large step
	cfg.Buckets.PreRings = 0
	cfg.Buckets.PostRings = 3

	rings := BuildRings(tr, cfg)

	if len(rings) == 0 {
		t.Fatalf("expected at least core ring")
	}

	assertValidRings(t, tr, rings)

	if len(rings) > cfg.Buckets.PreRings+1+cfg.Buckets.PostRings {
		t.Fatalf("unexpectedly many rings: got=%d max_allowed=%d", len(rings), cfg.Buckets.PreRings+1+cfg.Buckets.PostRings)
	}
}

func TestBuildRings_CoreOnly_Fallback(t *testing.T) {
	tr := models.TimeRange{
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
	}

	cfg := config.EngineConfig{}
	cfg.Buckets.CoreWindowSize = 0
	cfg.Buckets.RingStep = 0
	cfg.Buckets.PreRings = 0
	cfg.Buckets.PostRings = 0

	rings := BuildRings(tr, cfg)

	// Expect exactly a single core ring equal to the full tr
	if len(rings) != 1 {
		t.Fatalf("expected exactly 1 ring, got=%d", len(rings))
	}

	r := rings[0]
	if !r.Start.Equal(tr.Start) || !r.End.Equal(tr.End) {
		t.Fatalf("core fallback ring must equal full time range: got=%v want=%v", r, tr)
	}

	assertValidRings(t, tr, rings)
}
