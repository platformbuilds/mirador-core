package lucene

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/grindlemire/go-lucene"
	"github.com/grindlemire/go-lucene/pkg/lucene/expr"
)

// Target identifies which backend query language to generate.
type Target int

const (
	TargetLogsQL Target = iota
	TargetTraces
)

// tokenization shared by translators
type tokKind int

const (
	tkEOF tokKind = iota
	tkLParen
	tkRParen
	tkOp
	tkField
	tkBare
)

type token struct {
	k      tokKind
	v      string
	f      string
	val    string
	quoted bool
}

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
			min := ""
			if rangeBoundary.Min != nil {
				min = fmt.Sprintf("%v", rangeBoundary.Min)
			}
			max := ""
			if rangeBoundary.Max != nil {
				max = fmt.Sprintf("%v", rangeBoundary.Max)
			}

			if field == defaultTimeField || field == "time" {
				out.StartExpr = min
				out.EndExpr = max
			} else if field == "duration" {
				out.MinDuration = min
				out.MaxDuration = max
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
			return fmt.Sprintf(`%s:"%s"`, field, value), nil
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
			return fmt.Sprintf(`%s~"%s"`, field, regex), nil
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
			min := "*"
			if rangeBoundary.Min != nil {
				min = fmt.Sprintf("%v", rangeBoundary.Min)
			}
			max := "*"
			if rangeBoundary.Max != nil {
				max = fmt.Sprintf("%v", rangeBoundary.Max)
			}

			return fmt.Sprintf(`%s:[%s,%s]`, field, min, max), nil
		}
	}

	// Handle wildcard expressions
	if e.Op == expr.Wild {
		// For wildcards, the pattern is directly in e.Left
		if pattern, ok := e.Left.(string); ok {
			// Convert wildcard to regex
			regex := wildcardToRegex(pattern)
			return fmt.Sprintf(`%s~"%s"`, defaultMessageField, regex), nil
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
				return fmt.Sprintf(`%s:"%s"`, defaultMessageField, phrase), nil
			}
			// Single term
			return fmt.Sprintf(`%s:"%s"`, defaultMessageField, str), nil
		}
	}

	return "", fmt.Errorf("unsupported Lucene expression type: %v", e.Op)
}

func buildLogsQL(q query.Query) (string, error) {
	if tq, ok := q.(*query.TermQuery); ok {
		field := tq.Field()
		if field == "" {
			field = defaultMessageField
		}
		return fmt.Sprintf(`%s:"%s"`, field, tq.Term), nil
	}
	if mq, ok := q.(*query.MatchQuery); ok {
		field := mq.Field()
		if field == "" {
			field = defaultMessageField
		}
		return fmt.Sprintf(`%s:"%s"`, field, mq.Match), nil
	}
	if pq, ok := q.(*query.PhraseQuery); ok {
		field := pq.Field()
		if field == "" {
			field = defaultMessageField
		}
		phrase := strings.Join(pq.Terms, " ")
		return fmt.Sprintf(`%s:"%s"`, field, phrase), nil
	}
	if mpq, ok := q.(*query.MatchPhraseQuery); ok {
		field := mpq.Field()
		if field == "" {
			field = defaultMessageField
		}
		return fmt.Sprintf(`%s:"%s"`, field, mpq.MatchPhrase), nil
	}
	if wq, ok := q.(*query.WildcardQuery); ok {
		field := wq.Field()
		if field == "" {
			field = defaultMessageField
		}
		regex := wildcardToRegex(wq.Wildcard)
		return fmt.Sprintf(`%s~"%s"`, field, regex), nil
	}
	if nrq, ok := q.(*query.NumericRangeQuery); ok {
		field := nrq.Field()
		min := "*"
		if nrq.Min != nil {
			min = fmt.Sprintf("%v", *nrq.Min)
		}
		max := "*"
		if nrq.Max != nil {
			max = fmt.Sprintf("%v", *nrq.Max)
		}
		return fmt.Sprintf(`%s:[%s,%s]`, field, min, max), nil
	}
	if _, ok := q.(*query.FuzzyQuery); ok {
		return "", fmt.Errorf("fuzzy queries not supported")
	}
	if conj, ok := q.(*query.ConjunctionQuery); ok {
		var parts []string
		for _, subq := range conj.Conjuncts {
			part, err := buildLogsQL(subq)
			if err != nil {
				return "", err
			}
			parts = append(parts, part)
		}
		return strings.Join(parts, " AND "), nil
	}
	if disj, ok := q.(*query.DisjunctionQuery); ok {
		var parts []string
		for _, subq := range disj.Disjuncts {
			part, err := buildLogsQL(subq)
			if err != nil {
				return "", err
			}
			parts = append(parts, part)
		}
		return strings.Join(parts, " OR "), nil
	}
	if bq, ok := q.(*query.BooleanQuery); ok {
		return buildBooleanLogsQL(bq)
	}
	return "", fmt.Errorf("unsupported Lucene query type: %T", q)
}

// buildBooleanLogsQL handles boolean queries (AND/OR/NOT)
func buildBooleanLogsQL(bq *query.BooleanQuery) (string, error) {
	// Handle simple queries that get wrapped in BooleanQuery
	// Check if this is actually a simple query wrapped in a boolean

	// If only Should clause exists and it's a disjunction with one element
	if bq.Should != nil && bq.Must == nil && bq.MustNot == nil {
		if disj, ok := bq.Should.(*query.DisjunctionQuery); ok && len(disj.Disjuncts) == 1 {
			result, err := buildLogsQL(disj.Disjuncts[0])
			if err != nil {
				return "", err
			}
			return result, nil
		}
		// Handle disjunction with multiple elements
		if disj, ok := bq.Should.(*query.DisjunctionQuery); ok && len(disj.Disjuncts) > 1 {
			var parts []string
			for _, subq := range disj.Disjuncts {
				part, err := buildLogsQL(subq)
				if err != nil {
					return "", err
				}
				parts = append(parts, part)
			}
			return strings.Join(parts, " OR "), nil
		}
	}

	// If only Must clause exists
	if bq.Must != nil && bq.Should == nil && bq.MustNot == nil {
		if conj, ok := bq.Must.(*query.ConjunctionQuery); ok && len(conj.Conjuncts) == 1 {
			result, err := buildLogsQL(conj.Conjuncts[0])
			if err != nil {
				return "", err
			}
			return result, nil
		}
		// Handle conjunction with multiple elements
		if conj, ok := bq.Must.(*query.ConjunctionQuery); ok && len(conj.Conjuncts) > 1 {
			var parts []string
			for _, subq := range conj.Conjuncts {
				part, err := buildLogsQL(subq)
				if err != nil {
					return "", err
				}
				parts = append(parts, part)
			}
			return strings.Join(parts, " AND "), nil
		}
		// If Must is not a conjunction, try to build directly
		result, err := buildLogsQL(bq.Must)
		if err != nil {
			return "", err
		}
		return result, nil
	}

	// Handle complex boolean queries with both Must and Should
	if bq.Must != nil && bq.Should != nil {
		mustPart, err := buildLogsQL(bq.Must)
		if err != nil {
			return "", err
		}
		shouldPart, err := buildLogsQL(bq.Should)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s AND %s", mustPart, shouldPart), nil
	}

	// For other complex cases, return error for now
	// TODO: Implement proper BooleanQuery handling for more complex queries
	return "", fmt.Errorf("complex boolean queries not fully supported yet")
}

func toMetricsQL(q string) (string, bool) {
	// Very small subset:
	// - __name__:metric or metric:http_requests_total or bare metric name
	// - AND-joined field:value pairs -> label selectors key="value"
	// - OR/NOT/parentheses are not supported; if present, bail out
	if strings.Contains(q, " OR ") || strings.Contains(q, " or ") ||
		strings.Contains(q, " NOT ") || strings.Contains(q, " not ") ||
		strings.ContainsAny(q, "()") {
		return "", false
	}

	s := scanner{src: q}
	metric := ""
	labels := make([][2]string, 0, 8)

	for {
		t := s.next()
		if t.k == tkEOF {
			break
		}
		switch t.k {
		case tkField:
			name := strings.ToLower(t.f)
			if name == "__name__" || name == "metric" {
				if t.val != "" {
					metric = t.val
				}
				continue
			}
			// ignore time directives for metrics
			if name == defaultTimeField || name == "time" {
				continue
			}
			if t.quoted || hasWildcard(t.val) || looksRegex(t.val) {
				// PromQL label selector supports regex: key=~"..."
				labels = append(labels, [2]string{t.f, "~\"" + wildcardToRegex(t.val) + "\""})
			} else {
				labels = append(labels, [2]string{t.f, "=\"" + t.val + "\""})
			}
		case tkBare:
			// First bare token becomes metric name, others not supported -> ignore
			if metric == "" {
				metric = t.v
			}
		case tkOp:
			// only allow AND as whitespace; explicit ops cause bail-out
			if strings.EqualFold(t.v, "AND") {
				continue
			}
			return "", false
		case tkLParen, tkRParen:
			return "", false
		}
	}

	if metric == "" {
		// Without a metric name, can't form a valid selector
		return "", false
	}

	b := strings.Builder{}
	b.WriteString(metric)
	if len(labels) > 0 {
		b.WriteByte('{')
		for i, kv := range labels {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(kv[0])
			b.WriteString(kv[1])
		}
		b.WriteByte('}')
	}
	return b.String(), true
}

// Constants for common field names
const (
	defaultMessageField = "_msg"
	defaultTimeField    = "_time"
)

type scanner struct {
	src string
	i   int
	n   int
}

func (s *scanner) next() token {
	if s.n == 0 {
		s.n = len(s.src)
	}
	// skip whitespace
	for s.i < s.n && unicode.IsSpace(rune(s.src[s.i])) {
		s.i++
	}
	if s.i >= s.n {
		return token{k: tkEOF}
	}
	ch := s.src[s.i]
	switch ch {
	case '(':
		s.i++
		return token{k: tkLParen, v: "("}
	case ')':
		s.i++
		return token{k: tkRParen, v: ")"}
	}

	// read a word
	start := s.i
	for s.i < s.n && !unicode.IsSpace(rune(s.src[s.i])) && s.src[s.i] != '(' && s.src[s.i] != ')' {
		// stop at ':' but include for field path
		if s.src[s.i] == ':' {
			break
		}
		s.i++
	}
	word := s.src[start:s.i]

	// operator?
	lw := strings.ToLower(word)
	if lw == "and" || lw == "or" || lw == "not" {
		return token{k: tkOp, v: word}
	}

	// field?
	if s.i < s.n && s.src[s.i] == ':' {
		// consume ':'
		s.i++
		// value may be quoted or bare or bracketed
		if s.i < s.n && s.src[s.i] == '"' {
			s.i++
			vStart := s.i
			for s.i < s.n && s.src[s.i] != '"' {
				s.i++
			}
			val := s.src[vStart:s.i]
			if s.i < s.n {
				s.i++
			} // consume closing quote
			return token{k: tkField, f: word, val: val, quoted: true}
		}
		// bracketed range or value (we keep literal for logs and ignore for metrics)
		if s.i < s.n && s.src[s.i] == '[' {
			// scan until closing ']'
			depth := 1
			s.i++
			vStart := s.i
			for s.i < s.n && depth > 0 {
				if s.src[s.i] == '[' {
					depth++
				}
				if s.src[s.i] == ']' {
					depth--
					if depth == 0 {
						break
					}
				}
				s.i++
			}
			val := s.src[vStart:s.i]
			if s.i < s.n {
				s.i++
			} // consume ']'
			return token{k: tkField, f: word, val: "[" + val + "]"}
		}
		// bare value
		vStart := s.i
		for s.i < s.n && !unicode.IsSpace(rune(s.src[s.i])) && s.src[s.i] != '(' && s.src[s.i] != ')' {
			s.i++
		}
		val := s.src[vStart:s.i]
		return token{k: tkField, f: word, val: val}
	}

	// bare word or quoted phrase starting at '"'
	if len(word) == 0 && s.i < s.n && s.src[s.i] == '"' {
		s.i++
		vStart := s.i
		for s.i < s.n && s.src[s.i] != '"' {
			s.i++
		}
		val := s.src[vStart:s.i]
		if s.i < s.n {
			s.i++
		}
		return token{k: tkBare, v: val}
	}

	return token{k: tkBare, v: word}
}

func hasWildcard(s string) bool { return strings.ContainsAny(s, "*?") }
func looksRegex(s string) bool {
	// naive: treat if contains typical regex metachars that aren’t wildcards
	return strings.ContainsAny(s, "[].+|(){}^")
}
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
