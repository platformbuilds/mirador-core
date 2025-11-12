package lucene

import (
	"fmt"
	"strings"

	"github.com/grindlemire/go-lucene"
	"github.com/grindlemire/go-lucene/pkg/lucene/expr"
)

// Target identifies which backend query language to generate.
type Target int

const (
	TargetLogsQL Target = iota
	TargetTraces
)

// IsLikelyLucene performs a cheap heuristic to identify Lucene-style queries.
// It aims to avoid false-positives with PromQL/LogsQL.
func IsLikelyLucene(s string) bool {
	qs := strings.TrimSpace(s)
	if qs == "" {
		return false
	}
	// PromQL usually contains {...} label selectors; LogsQL often uses '='.
	// Prefer Lucene when we see field:value pairs without braces.
	if strings.Contains(qs, "{") && strings.Contains(qs, "}") {
		return false
	}
	// _time:5m, field:"phrase", field:value, parentheses, AND/OR/NOT
	if strings.Contains(qs, ":") {
		return true
	}
	// Bare phrases or tokens alone aren't enough to be confident.
	return false
}

// Translate tries converting a Lucene-like query into the target DSL.
// Returns translated string and ok=false if it could not translate.
func Translate(q string, t Target) (string, bool) {
	switch t {
	case TargetLogsQL:
		return translateToLogsQL(q)
	case TargetTraces:
		// For traces we return empty string here; use TranslateTraces for structured output.
		if _, ok := TranslateTraces(q); ok {
			return "", true
		}
		return "", false
	default:
		return "", false
	}
}

// TraceFilters represents extracted filters for Jaeger HTTP API.
type TraceFilters struct {
	Service     string
	Operation   string
	Tags        map[string]string
	MinDuration string
	MaxDuration string
	Since       string // e.g., "15m" → handler converts to Start/End
	StartExpr   string // e.g., "now-15m" or RFC3339
	EndExpr     string // e.g., "now"
}

// TranslateTraces parses a Lucene-like query and extracts Jaeger-compatible filters.
// Supported fields:
// - service:<name>
// - operation:<name>
// - duration:>10ms or duration:<1s (sets MinDuration/MaxDuration)
// - _time:[start TO end] or _time:15m (relative window)
// - tag.* or span_attr.* or any other key become tags (k=v; phrases/wildcards use value as-is)
func TranslateTraces(q string) (TraceFilters, bool) {
	parsed, err := lucene.Parse(q)
	if err != nil {
		return TraceFilters{}, false
	}
	out := TraceFilters{Tags: map[string]string{}}
	extractTraceFiltersLucene(parsed, &out)
	// If nothing parsed, return false
	if out.Service == "" && out.Operation == "" && len(out.Tags) == 0 && out.MinDuration == "" && out.MaxDuration == "" && out.Since == "" && out.StartExpr == "" && out.EndExpr == "" {
		return out, false
	}
	return out, true
}

func extractTraceFiltersLucene(e *expr.Expression, out *TraceFilters) {
	// Handle boolean operations (AND/OR)
	if e.Op == expr.And || e.Op == expr.Or {
		// Recursively process left and right operands
		if leftExpr, ok := e.Left.(*expr.Expression); ok {
			extractTraceFiltersLucene(leftExpr, out)
		}
		if rightExpr, ok := e.Right.(*expr.Expression); ok {
			extractTraceFiltersLucene(rightExpr, out)
		}
		return
	}

	// Handle field:value expressions
	if e.Op == expr.Equals {
		field := ""
		value := ""

		// Extract field name from left operand
		if leftExpr, ok := e.Left.(*expr.Expression); ok && leftExpr.Op == expr.Literal {
			if col, ok := leftExpr.Left.(expr.Column); ok {
				field = string(col)
			}
		}

		// Extract value from right operand
		if rightExpr, ok := e.Right.(*expr.Expression); ok && rightExpr.Op == expr.Literal {
			if str, ok := rightExpr.Left.(string); ok {
				value = str
			}
		}

		if field != "" && value != "" {
			setTraceFilter(field, value, out)
		}
		return
	}

	// Handle range expressions
	if e.Op == expr.Range {
		field := ""

		// Extract field name from left operand
		if leftExpr, ok := e.Left.(*expr.Expression); ok && leftExpr.Op == expr.Literal {
			if col, ok := leftExpr.Left.(expr.Column); ok {
				field = string(col)
			}
		}

		// Extract range boundaries from right operand
		if rangeBoundary, ok := e.Right.(*expr.RangeBoundary); ok {
			minVal := ""
			if rangeBoundary.Min != nil {
				minVal = fmt.Sprintf("%v", rangeBoundary.Min)
			}
			maxVal := ""
			if rangeBoundary.Max != nil {
				maxVal = fmt.Sprintf("%v", rangeBoundary.Max)
			}

			if field == defaultTimeField || field == "time" {
				out.StartExpr = minVal
				out.EndExpr = maxVal
			} else if field == "duration" {
				out.MinDuration = minVal
				out.MaxDuration = maxVal
			}
		}
		return
	}

	// Handle literal expressions (bare terms)
	if e.Op == expr.Literal {
		if str, ok := e.Left.(string); ok {
			setTraceFilter("", str, out)
		}
		return
	}
}

func setTraceFilter(field, val string, out *TraceFilters) {
	key := strings.ToLower(field)
	if key == "service" {
		out.Service = val
		return
	}
	if key == "operation" {
		out.Operation = val
		return
	}
	if key == "duration" {
		v := strings.TrimSpace(val)
		// Check if it's a partial range start [min
		if strings.HasPrefix(v, "[") && !strings.Contains(v, " TO ") {
			out.MinDuration = strings.TrimPrefix(v, "[")
			return
		}
		// Check if it's a partial range end max]
		if strings.HasSuffix(v, "]") && !strings.Contains(v, " TO ") {
			out.MaxDuration = strings.TrimSuffix(v, "]")
			return
		}
		// Check if it's a range in the format [min TO max]
		if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") && strings.Contains(v, " TO ") {
			// Parse duration range
			rangeStr := strings.Trim(v, "[]")
			parts := strings.Split(rangeStr, " TO ")
			if len(parts) == 2 {
				out.MinDuration = strings.TrimSpace(parts[0])
				out.MaxDuration = strings.TrimSpace(parts[1])
				return
			}
		}
		if strings.HasPrefix(v, ">=") || strings.HasPrefix(v, ">") {
			out.MinDuration = strings.TrimLeft(v, ">=")
		} else if strings.HasPrefix(v, "<=") || strings.HasPrefix(v, "<") {
			out.MaxDuration = strings.TrimLeft(v, "<=")
		} else {
			out.MinDuration = v
		}
		return
	}
	if key == defaultTimeField || key == "time" {
		// Check if it's a date range in the format [start TO end]
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") && strings.Contains(val, " TO ") {
			// Parse date range
			rangeStr := strings.Trim(val, "[]")
			parts := strings.Split(rangeStr, " TO ")
			if len(parts) == 2 {
				out.StartExpr = strings.TrimSpace(parts[0])
				out.EndExpr = strings.TrimSpace(parts[1])
				return
			}
		}
		// Otherwise treat as duration
		out.Since = val
		return
	}
	// Normalize tag.* or span_attr.* prefixes
	key = strings.TrimPrefix(key, "tag.")
	key = strings.TrimPrefix(key, "span_attr.")
	// For bare terms (field empty), treat as tag with val as key
	if key == "" {
		// Check if it's a partial duration range end like "1s]"
		if strings.HasSuffix(val, "]") && !strings.Contains(val, " TO ") {
			out.MaxDuration = strings.TrimSuffix(val, "]")
			return
		}
		out.Tags[val] = val
		return
	}
	out.Tags[key] = val
}

// ---------------- LogsQL translation ----------------

func translateToLogsQL(luceneQuery string) (string, bool) {
	parsed, err := lucene.Parse(luceneQuery)
	if err != nil {
		return "", false
	}
	logsQL, err := buildLogsQLFromLuceneAST(parsed)
	if err != nil {
		return "", false
	}
	return logsQL, true
}

// buildLogsQLFromLuceneAST converts a go-lucene AST expression to LogsQL string
func buildLogsQLFromLuceneAST(e *expr.Expression) (string, error) {
	// Handle boolean operations (AND/OR/NOT)
	if e.Op == expr.And {
		leftExpr, ok := e.Left.(*expr.Expression)
		if !ok {
			return "", fmt.Errorf("expected expression for left operand")
		}
		left, err := buildLogsQLFromLuceneAST(leftExpr)
		if err != nil {
			return "", err
		}
		rightExpr, ok := e.Right.(*expr.Expression)
		if !ok {
			return "", fmt.Errorf("expected expression for right operand")
		}
		right, err := buildLogsQLFromLuceneAST(rightExpr)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s AND %s", left, right), nil
	}
	if e.Op == expr.Or {
		leftExpr, ok := e.Left.(*expr.Expression)
		if !ok {
			return "", fmt.Errorf("expected expression for left operand")
		}
		left, err := buildLogsQLFromLuceneAST(leftExpr)
		if err != nil {
			return "", err
		}
		rightExpr, ok := e.Right.(*expr.Expression)
		if !ok {
			return "", fmt.Errorf("expected expression for right operand")
		}
		right, err := buildLogsQLFromLuceneAST(rightExpr)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s OR %s", left, right), nil
	}
	if e.Op == expr.Not {
		rightExpr, ok := e.Right.(*expr.Expression)
		if !ok {
			return "", fmt.Errorf("expected expression for right operand")
		}
		right, err := buildLogsQLFromLuceneAST(rightExpr)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("NOT %s", right), nil
	}

	// Handle field:value expressions
	if e.Op == expr.Equals {
		field := ""
		value := ""

		// Extract field name from left operand
		if leftExpr, ok := e.Left.(*expr.Expression); ok && leftExpr.Op == expr.Literal {
			if col, ok := leftExpr.Left.(expr.Column); ok {
				field = string(col)
			}
		}

		// Extract value from right operand
		if rightExpr, ok := e.Right.(*expr.Expression); ok && rightExpr.Op == expr.Literal {
			if str, ok := rightExpr.Left.(string); ok {
				value = str
			}
		}

		if field != "" && value != "" {
			return fmt.Sprintf(`%s:%q`, field, value), nil
		}
	}

	// Handle field LIKE expressions (for wildcards)
	if e.Op == expr.Like {
		field := ""
		wildcardPattern := ""

		// Extract field name from left operand
		if leftExpr, ok := e.Left.(*expr.Expression); ok && leftExpr.Op == expr.Literal {
			if col, ok := leftExpr.Left.(expr.Column); ok {
				field = string(col)
			}
		}

		// Extract wildcard pattern from right operand
		if rightExpr, ok := e.Right.(*expr.Expression); ok && rightExpr.Op == expr.Wild {
			if pattern, ok := rightExpr.Left.(string); ok {
				wildcardPattern = pattern
			}
		}

		if field != "" && wildcardPattern != "" {
			regex := wildcardToRegex(wildcardPattern)
			return fmt.Sprintf(`%s~%q`, field, regex), nil
		}
	}

	// Handle range expressions
	if e.Op == expr.Range {
		field := ""

		// Extract field name from left operand
		if leftExpr, ok := e.Left.(*expr.Expression); ok && leftExpr.Op == expr.Literal {
			if col, ok := leftExpr.Left.(expr.Column); ok {
				field = string(col)
			}
		}

		// Extract range boundaries from right operand
		if rangeBoundary, ok := e.Right.(*expr.RangeBoundary); ok {
			minVal := "*"
			if rangeBoundary.Min != nil {
				minVal = fmt.Sprintf("%v", rangeBoundary.Min)
			}
			maxVal := "*"
			if rangeBoundary.Max != nil {
				maxVal = fmt.Sprintf("%v", rangeBoundary.Max)
			}

			return fmt.Sprintf(`%s:[%s,%s]`, field, minVal, maxVal), nil
		}
	}

	// Handle wildcard expressions
	if e.Op == expr.Wild {
		// For wildcards, the pattern is directly in e.Left
		if pattern, ok := e.Left.(string); ok {
			// Convert wildcard to regex
			regex := wildcardToRegex(pattern)
			return fmt.Sprintf(`%s~%q`, defaultMessageField, regex), nil
		}
	}

	// Handle fuzzy expressions (not supported)
	if e.Op == expr.Fuzzy {
		return "", fmt.Errorf("fuzzy queries not supported")
	}

	// Handle literal expressions (bare terms)
	if e.Op == expr.Literal {
		if str, ok := e.Left.(string); ok {
			// Check if this is a quoted phrase
			if strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`) {
				phrase := strings.Trim(str, `"`)
				return fmt.Sprintf(`%s:%q`, defaultMessageField, phrase), nil
			}
			// Single term
			return fmt.Sprintf(`%s:%q`, defaultMessageField, str), nil
		}
	}

	return "", fmt.Errorf("unsupported Lucene expression type: %v", e.Op)
}

// Constants for common field names
const (
	defaultMessageField = "_msg"
	defaultTimeField    = "_time"
)

func wildcardToRegex(s string) string {
	// convert shell-style wildcards to regex equivalents without anchors
	r := strings.Builder{}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '*':
			r.WriteString(".*")
		case '?':
			r.WriteByte('.')
		case '.', '+', '(', ')', '[', ']', '{', '}', '|', '^', '$':
			r.WriteByte('\\')
			r.WriteByte(s[i])
		default:
			r.WriteByte(s[i])
		}
	}
	return r.String()
}
