package services

import (
    "encoding/json"
    "strings"
    "testing"
)

func TestNormalizeToMillis(t *testing.T) {
    if got := normalizeToMillis(1700000000); got == 0 { // seconds -> ms
        t.Fatalf("normalizeToMillis should convert seconds to ms")
    }
    if got := normalizeToMillis(1700000000000); got != 1700000000000 { // already ms
        t.Fatalf("expected unchanged ms value")
    }
}

func TestReadErrBody(t *testing.T) {
    // raw text
    if msg := readErrBody(strings.NewReader("something failed")); msg != "something failed" {
        t.Fatalf("unexpected msg: %q", msg)
    }
    // JSON with error
    jb, _ := json.Marshal(map[string]any{"error": "bad"})
    if msg := readErrBody(strings.NewReader(string(jb))); msg != "bad" {
        t.Fatalf("unexpected json error msg: %q", msg)
    }
}

