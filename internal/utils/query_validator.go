package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/grindlemire/go-lucene"
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

	// Parse with go-lucene to validate syntax
	_, err := lucene.Parse(query)
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

func (v *QueryValidator) ValidateBleve(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("empty Bleve query")
	}

	// Parse with Bleve to validate syntax
	_, err := bleve.NewQueryStringQuery(query).Parse()
	if err != nil {
		return fmt.Errorf("invalid Bleve query syntax: %w", err)
	}

	// Enhanced security validation for Bleve queries
	lowerQuery := strings.ToLower(query)

	// Check for script injection patterns
	scriptPatterns := []string{
		"<script", "</script>", "javascript:", "vbscript:", "data:",
		"eval(", "exec(", "system(", "shell_exec(", "passthru(",
		"proc_open(", "popen(", "pcntl_exec(", "assert(",
		"create_function(", "include(", "require(", "file_get_contents(",
		"file_put_contents(", "unlink(", "rmdir(", "mkdir(",
		"chmod(", "chown(", "symlink(", "link(",
	}
	for _, pattern := range scriptPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return fmt.Errorf("potentially dangerous script injection pattern detected: %s", pattern)
		}
	}

	// Check for SQL injection patterns
	sqlPatterns := []string{
		"drop ", "delete ", "insert ", "update ", "alter ", "create ",
		"truncate ", "union ", "select ", "exec ", "execute ",
		"union select", "information_schema", "sysobjects", "syscolumns",
		"xp_", "sp_", "dbo.", "master.", "tempdb.",
	}
	for _, pattern := range sqlPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return fmt.Errorf("potentially dangerous SQL injection pattern detected: %s", pattern)
		}
	}

	// Check for command injection patterns
	cmdPatterns := []string{
		"; ", "| ", "& ", "&& ", "|| ", "`", "$(", "${",
		"rm ", "del ", "format ", "fdisk ", "mkfs",
		"shutdown ", "reboot ", "halt ", "poweroff",
		"wget ", "curl ", "nc ", "netcat ", "telnet",
	}
	for _, pattern := range cmdPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return fmt.Errorf("potentially dangerous command injection pattern detected: %s", pattern)
		}
	}

	// Check for URL-encoded injection attempts
	urlEncodedPatterns := []string{
		"%3cscript%3e", "%3c/script%3e", "%3cscript", "%3c%2fscript%3e",
		"%3c%73%63%72%69%70%74%3e",       // <script> in hex
		"%6a%61%76%61%73%63%72%69%70%74", // javascript in hex
		"%3c%69%66%72%61%6d%65",          // <iframe in hex
		"%3c%6f%62%6a%65%63%74",          // <object in hex
		"%3c%65%6d%62%65%64",             // <embed in hex
	}
	for _, pattern := range urlEncodedPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return fmt.Errorf("potentially dangerous URL-encoded injection pattern detected: %s", pattern)
		}
	}

	// Check for path traversal attempts
	pathTraversalPatterns := []string{
		"../", "..\\", "/etc/", "/bin/", "/usr/", "/var/", "/home/",
		"c:\\", "d:\\", "e:\\", "windows\\", "system32\\",
		"passwd", "shadow", "hosts", "resolv.conf",
	}
	for _, pattern := range pathTraversalPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return fmt.Errorf("potentially dangerous path traversal pattern detected: %s", pattern)
		}
	}

	// Check for excessive query complexity (potential DoS)
	if len(query) > 10000 {
		return fmt.Errorf("query too long: maximum allowed length is 10000 characters")
	}

	// Check for too many operators (potential DoS)
	operatorCount := strings.Count(query, " AND ") + strings.Count(query, " OR ") +
		strings.Count(query, " NOT ") + strings.Count(query, "+") + strings.Count(query, "-")
	if operatorCount > 50 {
		return fmt.Errorf("query too complex: too many operators (max 50 allowed)")
	}

	// Check for nested parentheses depth (potential DoS)
	maxDepth := 0
	currentDepth := 0
	for _, char := range query {
		if char == '(' {
			currentDepth++
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		} else if char == ')' {
			currentDepth--
			if currentDepth < 0 {
				return fmt.Errorf("unbalanced parentheses in query")
			}
		}
	}
	if maxDepth > 10 {
		return fmt.Errorf("query too complex: maximum parentheses nesting depth is 10")
	}

	return nil
}
