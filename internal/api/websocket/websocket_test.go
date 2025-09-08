package websocket

import "testing"

func TestParseStreamsAndNames(t *testing.T) {
    m := parseStreams("alerts,predictions,,alerts ")
    if !m["alerts"] || !m["predictions"] { t.Fatalf("expected parsed streams") }
    names := getStreamNames(m)
    if len(names) != 2 { t.Fatalf("expected 2 names, got %d", len(names)) }
}

