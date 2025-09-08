package benchmark

import (
    "os"
    "testing"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// TestBenchmarksPrereqs ensures the benchmark package has at least one test so
// `go test ./...` does not report "[no tests to run]" for this package.
// It also provides a hook to verify environment before running heavy benches.
func TestBenchmarksPrereqs(t *testing.T) {
    // If BENCH_ENABLE is set, we could optionally validate endpoints are provided.
    if os.Getenv("BENCH_ENABLE") != "" {
        _ = logger.New("error")
        // No network calls here; constructors are exercised in benchmarks themselves.
    }
}

