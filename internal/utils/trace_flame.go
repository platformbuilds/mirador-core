package utils

import (
	"strconv"
	"strings"
	"time"
)

// FlameNode represents a node for d3-flame-graph.
// name: label to display; value: duration in milliseconds.
type FlameNode struct {
	Name     string      `json:"name"`
	Value    int64       `json:"value"`
	Children []FlameNode `json:"children,omitempty"`
	SpanID   string      `json:"spanId,omitempty"`
	Service  string      `json:"service,omitempty"`
}

// FlameMode controls the numeric value used for node width.
// "duration" = total span duration; "self" = duration minus children durations (approximation).
type FlameMode string

const (
	FlameDuration FlameMode = "duration"
	FlameSelf     FlameMode = "self"
)

// BuildFlameGraphFromJaeger converts a Jaeger trace payload
// (spans as []map[string]any) into a FlameNode tree suitable for d3-flame-graph.
// It relies on CHILD_OF references to link parent/child relationships.
func BuildFlameGraphFromJaeger(traceID string, spans []map[string]any, processes map[string]any) FlameNode {
	return BuildFlameGraphFromJaegerWithMode(traceID, spans, processes, string(FlameDuration))
}

// BuildFlameGraphFromJaegerWithMode builds a flame graph with the given sizing mode.
func BuildFlameGraphFromJaegerWithMode(traceID string, spans []map[string]any, processes map[string]any, mode string) FlameNode {
	// Index spans by id and parent
	type spanWrap struct {
		id       string
		parentID string
		name     string
		duration int64 // ms
		process  string
		startMs  int64 // for ordering
		raw      map[string]any
	}

	byID := map[string]*spanWrap{}
	var roots []*spanWrap

	toInt64 := func(v any) int64 {
		switch x := v.(type) {
		case float64:
			return int64(x)
		case float32:
			return int64(x)
		case int64:
			return x
		case int:
			return int64(x)
		case uint64:
			return int64(x)
		case string:
			// try RFC3339 time first
			if strings.Contains(x, "T") && (strings.Contains(x, "-") || strings.Contains(x, ":")) {
				if t, err := time.Parse(time.RFC3339Nano, x); err == nil {
					return t.UnixMicro()
				}
				if t, err := time.Parse(time.RFC3339, x); err == nil {
					return t.UnixMicro()
				}
			}
			// Parse numeric string; ignore non-digits suffix/prefix gracefully (e.g., "123000" or "123ms")
			var digits []rune
			for _, r := range x {
				if r >= '0' && r <= '9' {
					digits = append(digits, r)
				}
			}
			if len(digits) == 0 {
				return 0
			}
			var n int64 = 0
			for _, r := range digits {
				n = n*10 + int64(r-'0')
			}
			return n
		default:
			return 0
		}
	}

	// helpers to read start/duration in microseconds from various possible keys
	getStartMicros := func(span map[string]any) int64 {
		// Try common Jaeger keys
		if v, ok := span["startTime"]; ok {
			return toInt64(v)
		}
		if v, ok := span["startTimeMicros"]; ok {
			return toInt64(v)
		}
		if v, ok := span["start_time"]; ok {
			return toInt64(v)
		}
		// Unix nanos/millis variants
		if v, ok := span["startTimeUnixNano"]; ok {
			return toInt64(v) / 1000
		}
		if v, ok := span["startTimeUnixMillis"]; ok {
			return toInt64(v) * 1000
		}
		return 0
	}
	getDurationMicros := func(span map[string]any) int64 {
		if v, ok := span["duration"]; ok {
			return toInt64(v)
		}
		if v, ok := span["durationMicros"]; ok {
			return toInt64(v)
		}
		if v, ok := span["duration_us"]; ok {
			return toInt64(v)
		}
		if v, ok := span["durationMs"]; ok {
			return toInt64(v) * 1000
		}
		if v, ok := span["duration_ms"]; ok {
			return toInt64(v) * 1000
		}
		if v, ok := span["durationNanos"]; ok {
			return toInt64(v) / 1000
		}
		if v, ok := span["duration_ns"]; ok {
			return toInt64(v) / 1000
		}
		return 0
	}

	var minStart int64 = 0
	var maxEnd int64 = 0
	for _, s := range spans {
		id, _ := s["spanID"].(string)
		name, _ := s["operationName"].(string)
		dur := getDurationMicros(s) // microseconds
		proc, _ := s["processID"].(string)
		st := getStartMicros(s) // microseconds
		parent := ""
		if refs, ok := s["references"].([]interface{}); ok {
			for _, r := range refs {
				if rm, ok := r.(map[string]interface{}); ok {
					if rt, _ := rm["refType"].(string); rt == "CHILD_OF" {
						if pid, _ := rm["spanID"].(string); pid != "" {
							parent = pid
							break
						}
					}
				}
			}
		}
		// convert to ms; ensure minimum width 1ms if span exists
		ms := dur / 1_000
		if ms <= 0 && dur > 0 {
			ms = 1
		}
		startMs := st / 1_000
		w := &spanWrap{id: id, parentID: parent, name: name, duration: ms, process: proc, startMs: startMs, raw: s}
		byID[id] = w

		// track root duration window
		if startMs > 0 {
			if minStart == 0 || startMs < minStart {
				minStart = startMs
			}
			if end := startMs + ms; end > maxEnd {
				maxEnd = end
			}
		}
	}
	for _, w := range byID {
		if w.parentID == "" || byID[w.parentID] == nil {
			roots = append(roots, w)
		}
	}

	var build func(w *spanWrap) FlameNode
	build = func(w *spanWrap) FlameNode {
		label := w.name
		if proc, ok := processes[w.process].(map[string]interface{}); ok {
			if srv, ok := proc["serviceName"].(string); ok && srv != "" {
				label = srv + "." + w.name
			}
		}
		node := FlameNode{
			Name:   label,
			Value:  w.duration,
			SpanID: w.id,
		}
		// service name from processes map if available
		if proc, ok := processes[w.process].(map[string]interface{}); ok {
			if srv, ok := proc["serviceName"].(string); ok {
				node.Service = srv
			}
		}
		// children (ordered by start time)
		var kids []*spanWrap
		for _, c := range byID {
			if c.parentID == w.id {
				kids = append(kids, c)
			}
		}
		if len(kids) > 1 {
			// simple insertion sort by startMs to avoid importing sort for slices of pointers
			for i := 1; i < len(kids); i++ {
				j := i
				for j > 0 && kids[j-1].startMs > kids[j].startMs {
					kids[j-1], kids[j] = kids[j], kids[j-1]
					j--
				}
			}
		}
		for _, c := range kids {
			node.Children = append(node.Children, build(c))
		}
		if mode == string(FlameSelf) {
			var childSum int64
			for _, ch := range node.Children {
				childSum += ch.Value
			}
			self := w.duration - childSum
			if self < 0 {
				self = 0
			}
			node.Value = self
		}
		return node
	}

	// create synthetic root if multiple roots
	if len(roots) == 1 {
		root := build(roots[0])
		// set total duration at root based on min/max bounds when available
		if minStart > 0 && maxEnd > minStart {
			root.Value = maxEnd - minStart
		}
		// include total duration in label
		root.Name = root.Name + " (" + itoa(root.Value) + " ms)"
		return root
	}
	root := FlameNode{Name: "trace " + traceID, Value: 0}
	for _, r := range roots {
		root.Children = append(root.Children, build(r))
	}
	if minStart > 0 && maxEnd > minStart {
		root.Value = maxEnd - minStart
	}
	root.Name = root.Name + " (" + itoa(root.Value) + " ms)"
	return root
}

// MergeFlameTrees merges b into a by node Name recursively, summing values.
func MergeFlameTrees(a *FlameNode, b FlameNode) {
	a.Value += b.Value
	// index existing children by name
	idx := make(map[string]*FlameNode, len(a.Children))
	for i := range a.Children {
		idx[a.Children[i].Name] = &a.Children[i]
	}
	for _, cb := range b.Children {
		if ca, ok := idx[cb.Name]; ok {
			MergeFlameTrees(ca, cb)
		} else {
			a.Children = append(a.Children, cb)
		}
	}
}

// itoa converts int64 to decimal string without importing strconv again here (already imported).
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
