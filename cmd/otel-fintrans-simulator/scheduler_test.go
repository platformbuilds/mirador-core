package main

import (
    "testing"
    "time"
)

func TestFailureSchedulerProbabilityAt(t *testing.T) {
    cfg := FailureConfig{
        Mode: "bursty",
        Rate: 0.2,
        Bursts: []Burst{{Start: "0s", Duration: "1m", Multiplier: 5.0}},
        Seed: nil,
    }

    now := time.Now()
    fs, err := newFailureScheduler(cfg, now)
    if err != nil {
        t.Fatalf("newFailureScheduler: %v", err)
    }

    // inside burst: multiplier 5 -> probability = 1.0 (0.2*5)
    pInside := fs.ProbabilityAt(now.Add(10 * time.Second))
    if pInside < 0.9999 {
        t.Fatalf("expected near-1.0 prob inside burst, got %v", pInside)
    }

    // outside burst: should be base rate
    pOutside := fs.ProbabilityAt(now.Add(2 * time.Minute))
    if pOutside != 0.2 {
        t.Fatalf("expected 0.2 chance outside burst, got %v", pOutside)
    }
}

func TestFailureSchedulerShouldFailDeterministic(t *testing.T) {
    seed := int64(42)
    cfg := FailureConfig{
        Mode: "bursty",
        Rate: 0.5,
        Seed: &seed,
        Bursts: []Burst{{Start: "0s", Duration: "1m", Multiplier: 2.0}},
    }

    now := time.Now()
    fs, err := newFailureScheduler(cfg, now)
    if err != nil {
        t.Fatalf("newFailureScheduler: %v", err)
    }

    // With seed 42 the RNG is deterministic. Check a few calls at fixed time are reproducible
    t1 := now.Add(10 * time.Second)
    t2 := now.Add(2 * time.Minute)

    // deterministic calls
    out1 := fs.ShouldFail(t1)
    out2 := fs.ShouldFail(t2)
    // reset scheduler with same seed to verify same results
    fs2, err := newFailureScheduler(cfg, now)
    if err != nil {
        t.Fatalf("newFailureScheduler: %v", err)
    }

    if fs2.ShouldFail(t1) != out1 || fs2.ShouldFail(t2) != out2 {
        t.Fatalf("ShouldFail results are not deterministic across same-seed schedulers")
    }
}
