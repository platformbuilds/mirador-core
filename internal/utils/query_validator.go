package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blevesearch/bleve/v2"
)

type QueryValidator struct {
	metricsQLPatterns []string
	logsQLPatterns    []string
	tracesPatterns    []string
}

func NewQueryValidator() *QueryValidator {
	return &QueryValidator{
		metricsQLPatterns: []string{
			`^[a-zA-Z_][a-zA-Z0-9_]*`, // Metric name pattern
			`\{[^}]*\}`,               // Label selector pattern
			`\[[0-9]+[smhd]\]`,        // Time range pattern
		},
		logsQLPatterns: []string{
			`_time:[0-9]+[smhd]`, // Time filter pattern
			`\|`,                 // Pipe operator
			`stats\s+by\s*\(`,    // Stats aggregation
		},
		tracesPatterns: []string{
			`trace_id:`,  // Trace ID filter
			`span_attr:`, // Span attribute filter
			`duration:`,  // Duration filter
		},
	}
}

func (v *QueryValidator) ValidateMetricsQL(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("empty MetricsQL query")
	}

	// Check for dangerous functions
	dangerousFunctions := []string{"eval", "exec", "system"}
	for _, dangerous := range dangerousFunctions {
		if strings.Contains(strings.ToLower(query), dangerous) {
			return fmt.Errorf("dangerous function detected: %s", dangerous)
		}
	}

	// Validate basic MetricsQL syntax
	if !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*`).MatchString(query) &&
		!strings.Contains(query, "(") { // Function calls
		return fmt.Errorf("invalid MetricsQL syntax")
	}

	return nil
}

func (v *QueryValidator) ValidateLogsQL(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("empty LogsQL query")
	}

	// Check for SQL injection patterns
	sqlPatterns := []string{"drop", "delete", "insert", "update", "alter", "create"}
	lowerQuery := strings.ToLower(query)
	for _, pattern := range sqlPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return fmt.Errorf("potentially dangerous SQL pattern detected: %s", pattern)
		}
	}

	return nil
}

func (v *QueryValidator) ValidateTracesQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("empty traces query")
	}

	// Basic validation for traces query
	if !strings.Contains(query, "trace_id:") &&
		!strings.Contains(query, "span_attr:") &&
		!strings.Contains(query, "_time:") {
		return fmt.Errorf("traces query must contain at least one valid filter")
	}

	return nil
}

func (v *QueryValidator) ValidateLucene(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("empty Lucene query")
	}

	// Parse with Bleve to validate syntax
	_, err := bleve.NewQueryStringQuery(query).Parse()
	if err != nil {
		return fmt.Errorf("invalid Lucene query syntax: %w", err)
	}

	// Check for dangerous patterns
	dangerousPatterns := []string{"<script", "javascript:", "eval(", "exec(", "system("}
	lowerQuery := strings.ToLower(query)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return fmt.Errorf("potentially dangerous pattern detected: %s", pattern)
		}
	}

	return nil
}
