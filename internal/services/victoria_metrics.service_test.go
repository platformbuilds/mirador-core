package services

import (
    "strings"
    "testing"
)

func TestCountSeries(t *testing.T) {
    data := map[string]any{
        "result": []any{
            map[string]any{"metric": map[string]any{"__name__": "up"}, "value": []any{123, "1"}},
            map[string]any{"metric": map[string]any{"__name__": "up"}, "value": []any{124, "0"}},
        },
    }
    if got := countSeries(data); got != 2 {
        t.Fatalf("countSeries= %d; want 2", got)
    }
}

func TestCountDataPoints(t *testing.T) {
    data := map[string]any{
        "result": []any{
            map[string]any{"values": []any{[]any{1, "1"}, []any{2, "1"}}},
            map[string]any{"value": []any{3, "0"}},
        },
    }
    if got := countDataPoints(data); got != 3 {
        t.Fatalf("countDataPoints= %d; want 3", got)
    }
}

func TestReadBodySnippet(t *testing.T) {
    s := strings.Repeat("a", 100)
    out := readBodySnippet(strings.NewReader(s))
    if out != s {
        t.Fatalf("unexpected snippet: %q", out)
    }
    // ensure it does not panic on large input
    big := strings.Repeat("x", 100_000)
    _ = readBodySnippet(strings.NewReader(big))
}
