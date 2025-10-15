package bleve

import (
	"fmt"
	"regexp"
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

// TranslateLuceneToLogsQL converts a Lucene query string to LogsQL format
func (t *Translator) TranslateLuceneToLogsQL(luceneQuery string) (string, error) {
	if strings.TrimSpace(luceneQuery) == "" {
		return "", nil
	}

	// Start with the input query
	logsQL := luceneQuery

	// Handle AND: replace " AND " with space (implicit AND in LogsQL)
	logsQL = strings.ReplaceAll(logsQL, " AND ", " ")

	// Handle NOT: replace " NOT " with " -"
	logsQL = strings.ReplaceAll(logsQL, " NOT ", " -")

	// Handle ! negation: replace " !" with " -"
	// This handles cases like "error !debug" -> "error -debug"
	logsQL = regexp.MustCompile(`(\w)\s+!(\w)`).ReplaceAllString(logsQL, "$1 -$2")

	// Handle range queries: [min TO max] -> >=min <=max
	// Use regex to find patterns like field:[value TO value]
	rangeRegex := regexp.MustCompile(`(\w+):\[([^\]]+)\s+TO\s+([^\]]+)\]`)
	logsQL = rangeRegex.ReplaceAllStringFunc(logsQL, func(match string) string {
		parts := rangeRegex.FindStringSubmatch(match)
		if len(parts) == 4 {
			field := parts[1]
			min := strings.TrimSpace(parts[2])
			max := strings.TrimSpace(parts[3])
			return fmt.Sprintf("%s:>=%s %s:<=%s", field, min, field, max)
		}
		return match
	})

	// Handle regex: /pattern/ -> ~"pattern"
	regexRegex := regexp.MustCompile(`/([^/]+)/`)
	logsQL = regexRegex.ReplaceAllStringFunc(logsQL, func(match string) string {
		parts := regexRegex.FindStringSubmatch(match)
		if len(parts) == 2 {
			pattern := parts[1]
			return fmt.Sprintf(`~"%s"`, pattern)
		}
		return match
	})

	// Handle field-specific regex: field:/pattern/ -> field:~"pattern"
	fieldRegexRegex := regexp.MustCompile(`(\w+):/([^/]+)/`)
	logsQL = fieldRegexRegex.ReplaceAllStringFunc(logsQL, func(match string) string {
		parts := fieldRegexRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			field := parts[1]
			pattern := parts[2]
			return fmt.Sprintf(`%s:~"%s"`, field, pattern)
		}
		return match
	})

	// Handle negation with - at start of terms
	// Already handled NOT, but for -term, it's already -term

	// For wildcards, they stay as is: error* -> error*

	// For phrases, they stay as is: "connection refused" -> "connection refused"

	// For field in list: status:(500 OR 404) stays as is

	// Trim spaces
	logsQL = strings.TrimSpace(logsQL)

	return logsQL, nil
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

	if conj, ok := q.(*query.ConjunctionQuery); ok {
		var parts []string
		for _, subq := range conj.Conjuncts {
			part, err := t.buildLogsQL(subq)
			if err != nil {
				return "", fmt.Errorf("failed to build conjunction part: %w", err)
			}
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, " "), nil
	}

	if disj, ok := q.(*query.DisjunctionQuery); ok {
		var parts []string
		for _, subq := range disj.Disjuncts {
			part, err := t.buildLogsQL(subq)
			if err != nil {
				return "", fmt.Errorf("failed to build disjunction part: %w", err)
			}
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) == 1 {
			return parts[0], nil
		}
		return strings.Join(parts, " OR "), nil
	}

	if bq, ok := q.(*query.BooleanQuery); ok {
		return t.buildBooleanLogsQL(bq)
	}

	return "", fmt.Errorf("unsupported Bleve query type: %T", q)
}

// buildBooleanLogsQL handles boolean queries (AND/OR/NOT)
func (t *Translator) buildBooleanLogsQL(bq *query.BooleanQuery) (string, error) {
	var parts []string

	// Handle Must clauses (AND logic) - these are required to match
	if bq.Must != nil {
		mustPart, err := t.buildLogsQL(bq.Must)
		if err != nil {
			return "", fmt.Errorf("failed to build Must clause: %w", err)
		}
		if mustPart != "" {
			parts = append(parts, mustPart)
		}
	}

	// Handle Should clauses (OR logic) - at least one should match
	if bq.Should != nil {
		shouldPart, err := t.buildLogsQL(bq.Should)
		if err != nil {
			return "", fmt.Errorf("failed to build Should clause: %w", err)
		}
		if shouldPart != "" {
			// Only wrap in parentheses if it's actually an OR expression (contains " OR ")
			// Don't wrap single terms even if they contain spaces
			if strings.Contains(shouldPart, " OR ") && !strings.HasPrefix(shouldPart, "(") {
				shouldPart = "(" + shouldPart + ")"
			}
			parts = append(parts, shouldPart)
		}
	}

	// Handle MustNot clauses (NOT logic) - these must not match
	if bq.MustNot != nil {
		mustNotPart, err := t.buildLogsQL(bq.MustNot)
		if err != nil {
			return "", fmt.Errorf("failed to build MustNot clause: %w", err)
		}
		if mustNotPart != "" {
			// Split MustNot terms and add - prefix to each
			terms := strings.Fields(mustNotPart)
			for _, term := range terms {
				// Skip empty terms
				term = strings.TrimSpace(term)
				if term == "" {
					continue
				}
				// Add - prefix for negation
				if !strings.HasPrefix(term, "-") {
					term = "-" + term
				}
				parts = append(parts, term)
			}
		}
	}

	// If no parts were added, this might be an empty boolean query
	if len(parts) == 0 {
		return "", nil
	}

	// Join all parts with spaces (implicit AND in LogsQL)
	result := strings.Join(parts, " ")

	// Clean up any double spaces
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result, nil
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
