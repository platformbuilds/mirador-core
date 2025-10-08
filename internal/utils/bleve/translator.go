package bleve

import (
	"fmt"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// Target identifies which backend query language to generate.
type Target int

const (
	TargetLogsQL Target = iota
	TargetTraces
)

// Translator handles Bleve query translation to various backend formats
type Translator struct {
	// Configuration and state can be added here
}

// NewTranslator creates a new Bleve translator instance
func NewTranslator() *Translator {
	return &Translator{}
}

// IsLikelyBleve performs a cheap heuristic to identify Bleve-style queries.
// Bleve queries are typically more structured than Lucene queries.
func IsLikelyBleve(s string) bool {
	qs := strings.TrimSpace(s)
	if qs == "" {
		return false
	}

	// Lucene-specific patterns that should return false
	if strings.Contains(qs, "{") && strings.Contains(qs, "}") {
		return false // Lucene range queries like {2024-01-01 TO 2024-12-31}
	}
	if strings.Contains(qs, "[") && strings.Contains(qs, "TO") && strings.Contains(qs, "]") {
		return false // Lucene range queries like [100 TO 500]
	}

	// Bleve queries often use structured syntax like +field:value -field:value
	// Look for Bleve-specific operators
	if strings.Contains(qs, "+") || strings.Contains(qs, "-") {
		return true
	}

	// Check for field:value pairs (common in both but more structured in Bleve)
	colonCount := strings.Count(qs, ":")
	if colonCount > 0 && !strings.Contains(qs, "\"") {
		// If we have field:value patterns without quotes, likely Bleve
		return true
	}

	return false
}

// TranslateToLogsQL converts a Bleve query to LogsQL format
func (t *Translator) TranslateToLogsQL(bleveQuery string) (string, error) {
	qsq := bleve.NewQueryStringQuery(bleveQuery)
	parsedQuery, err := qsq.Parse()
	if err != nil {
		return "", fmt.Errorf("failed to parse Bleve query: %w", err)
	}

	logsQL, err := t.buildLogsQL(parsedQuery)
	if err != nil {
		return "", fmt.Errorf("failed to build LogsQL: %w", err)
	}

	return logsQL, nil
}

// TranslateToTraces converts a Bleve query to trace filters
func (t *Translator) TranslateToTraces(bleveQuery string) (TraceFilters, error) {
	qsq := bleve.NewQueryStringQuery(bleveQuery)
	parsedQuery, err := qsq.Parse()
	if err != nil {
		return TraceFilters{}, fmt.Errorf("failed to parse Bleve query: %w", err)
	}

	out := TraceFilters{Tags: map[string]string{}}
	t.extractTraceFilters(parsedQuery, &out)

	// If nothing parsed, return error
	if out.Service == "" && out.Operation == "" && len(out.Tags) == 0 && out.MinDuration == "" && out.MaxDuration == "" && out.Since == "" && out.StartExpr == "" && out.EndExpr == "" {
		return out, fmt.Errorf("no valid trace filters found in query")
	}

	return out, nil
}

// buildLogsQL converts a Bleve query to LogsQL format
func (t *Translator) buildLogsQL(q query.Query) (string, error) {
	if tq, ok := q.(*query.TermQuery); ok {
		field := tq.Field()
		if field == "" {
			field = "_msg"
		}
		return fmt.Sprintf(`%s:"%s"`, field, tq.Term), nil
	}

	if mq, ok := q.(*query.MatchQuery); ok {
		field := mq.Field()
		if field == "" {
			field = "_msg"
		}
		return fmt.Sprintf(`%s:"%s"`, field, mq.Match), nil
	}

	if pq, ok := q.(*query.PhraseQuery); ok {
		field := pq.Field()
		if field == "" {
			field = "_msg"
		}
		phrase := strings.Join(pq.Terms, " ")
		return fmt.Sprintf(`%s:"%s"`, field, phrase), nil
	}

	if wq, ok := q.(*query.WildcardQuery); ok {
		field := wq.Field()
		if field == "" {
			field = "_msg"
		}
		regex := t.wildcardToRegex(wq.Wildcard)
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

	if drq, ok := q.(*query.DateRangeQuery); ok {
		field := drq.Field()
		start := "*"
		end := "*"
		// DateRangeQuery handling - simplified for now
		return fmt.Sprintf(`%s:[%s,%s]`, field, start, end), nil
	}

	if mpq, ok := q.(*query.MatchPhraseQuery); ok {
		field := mpq.Field()
		if field == "" {
			field = "_msg"
		}
		phrase := mpq.MatchPhrase
		return fmt.Sprintf(`%s:"%s"`, field, phrase), nil
	}

	if bq, ok := q.(*query.BooleanQuery); ok {
		return t.buildBooleanLogsQL(bq)
	}

	return "", fmt.Errorf("unsupported Bleve query type: %T", q)
}

// buildBooleanLogsQL handles boolean queries (AND/OR/NOT)
func (t *Translator) buildBooleanLogsQL(bq *query.BooleanQuery) (string, error) {
	// Handle simple queries that get wrapped in BooleanQuery
	// Check if this is actually a simple query wrapped in a boolean

	// If only Must clause exists and it's a conjunction with one element
	if bq.Must != nil && bq.Should == nil && bq.MustNot == nil {
		if conj, ok := bq.Must.(*query.ConjunctionQuery); ok && len(conj.Conjuncts) == 1 {
			return t.buildLogsQL(conj.Conjuncts[0])
		}
		// If Must is not a conjunction, try to build directly
		return t.buildLogsQL(bq.Must)
	}

	// If only Should clause exists
	if bq.Should != nil && bq.Must == nil && bq.MustNot == nil {
		if disj, ok := bq.Should.(*query.DisjunctionQuery); ok && len(disj.Disjuncts) == 1 {
			return t.buildLogsQL(disj.Disjuncts[0])
		}
	}

	// For complex boolean queries, return error for now
	// TODO: Implement proper BooleanQuery handling for complex queries
	return "", fmt.Errorf("complex boolean queries not fully supported yet")
}

// wildcardToRegex converts Bleve wildcard pattern to regex
func (t *Translator) wildcardToRegex(wildcard string) string {
	// Escape special regex characters except * and ?
	escaped := strings.ReplaceAll(wildcard, ".", "\\.")
	escaped = strings.ReplaceAll(escaped, "+", "\\+")
	escaped = strings.ReplaceAll(escaped, "^", "\\^")
	escaped = strings.ReplaceAll(escaped, "$", "\\$")
	escaped = strings.ReplaceAll(escaped, "(", "\\(")
	escaped = strings.ReplaceAll(escaped, ")", "\\)")
	escaped = strings.ReplaceAll(escaped, "[", "\\[")
	escaped = strings.ReplaceAll(escaped, "]", "\\]")
	escaped = strings.ReplaceAll(escaped, "{", "\\{")
	escaped = strings.ReplaceAll(escaped, "}", "\\}")

	// Convert * and ? to regex equivalents
	escaped = strings.ReplaceAll(escaped, "*", ".*")
	escaped = strings.ReplaceAll(escaped, "?", ".")

	return "^" + escaped + "$"
}

// TraceFilters represents extracted filters for trace queries
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

// extractTraceFilters extracts trace filters from a Bleve query
func (t *Translator) extractTraceFilters(q query.Query, out *TraceFilters) {
	if tq, ok := q.(*query.TermQuery); ok {
		field := tq.Field()
		val := tq.Term
		t.setTraceFilter(field, val, out)
		return
	}

	if mq, ok := q.(*query.MatchQuery); ok {
		field := mq.Field()
		val := mq.Match
		t.setTraceFilter(field, val, out)
		return
	}

	if pq, ok := q.(*query.PhraseQuery); ok {
		field := pq.Field()
		val := strings.Join(pq.Terms, " ")
		t.setTraceFilter(field, val, out)
		return
	}

	if wq, ok := q.(*query.WildcardQuery); ok {
		field := wq.Field()
		val := wq.Wildcard
		t.setTraceFilter(field, val, out)
		return
	}

	if nrq, ok := q.(*query.NumericRangeQuery); ok {
		field := nrq.Field()
		// For duration, handle range
		if field == "duration" {
			min := ""
			if nrq.Min != nil {
				min = fmt.Sprintf("%v", *nrq.Min)
			}
			max := ""
			if nrq.Max != nil {
				max = fmt.Sprintf("%v", *nrq.Max)
			}
			if min != "" {
				out.MinDuration = min
			}
			if max != "" {
				out.MaxDuration = max
			}
		}
		return
	}

	if drq, ok := q.(*query.DateRangeQuery); ok {
		field := drq.Field()
		if field == "_time" || field == "time" {
			// Set start and end expressions as RFC3339 timestamps
			if !drq.Start.Time.IsZero() {
				out.StartExpr = drq.Start.Time.Format(time.RFC3339)
			}
			if !drq.End.Time.IsZero() {
				out.EndExpr = drq.End.Time.Format(time.RFC3339)
			}
		}
		return
	}

	if bq, ok := q.(*query.BooleanQuery); ok {
		// Handle BooleanQuery by recursively processing its parts
		if bq.Must != nil {
			if conj, ok := bq.Must.(*query.ConjunctionQuery); ok {
				for _, subq := range conj.Conjuncts {
					t.extractTraceFilters(subq, out)
				}
			}
		}
		if bq.Should != nil {
			if disj, ok := bq.Should.(*query.DisjunctionQuery); ok {
				for _, subq := range disj.Disjuncts {
					t.extractTraceFilters(subq, out)
				}
			}
		}
		return
	}
}

// setTraceFilter sets a trace filter value based on field name
func (t *Translator) setTraceFilter(field, val string, out *TraceFilters) {
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
		if strings.HasPrefix(v, ">=") || strings.HasPrefix(v, ">") {
			out.MinDuration = strings.TrimLeft(v, ">=")
		} else if strings.HasPrefix(v, "<=") || strings.HasPrefix(v, "<") {
			out.MaxDuration = strings.TrimLeft(v, "<=")
		} else {
			out.MinDuration = v
		}
		return
	}
	if key == "_time" || key == "time" {
		out.Since = val
		return
	}
	// Normalize tag.* or span_attr.* prefixes
	key = strings.TrimPrefix(key, "tag.")
	key = strings.TrimPrefix(key, "span_attr.")
	out.Tags[key] = val
}
