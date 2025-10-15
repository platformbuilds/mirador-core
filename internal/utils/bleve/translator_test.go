package bleve

import (
	"testing"
)

func TestIsLikelyBleve(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "empty query",
			query:    "",
			expected: false,
		},
		{
			name:     "whitespace only",
			query:    "   ",
			expected: false,
		},
		{
			name:     "plus operator",
			query:    "+field:value",
			expected: true,
		},
		{
			name:     "minus operator",
			query:    "-field:value",
			expected: true,
		},
		{
			name:     "field:value pair",
			query:    "service:api",
			expected: true,
		},
		{
			name:     "multiple field:value pairs",
			query:    "service:api level:error",
			expected: true,
		},
		{
			name:     "lucene range query",
			query:    "duration:[100 TO 500]",
			expected: false, // contains { which indicates Lucene
		},
		{
			name:     "simple term",
			query:    "error",
			expected: false,
		},
		{
			name:     "phrase query",
			query:    "\"connection timeout\"",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLikelyBleve(tt.query)
			if result != tt.expected {
				t.Errorf("IsLikelyBleve(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestTranslateToLogsQL(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		name        string
		query       string
		expected    string
		expectError bool
	}{
		{
			name:        "term query",
			query:       "error",
			expected:    `_msg:"error"`,
			expectError: false,
		},
		{
			name:        "field term query",
			query:       "level:error",
			expected:    `level:"error"`,
			expectError: false,
		},
		{
			name:        "match query",
			query:       "message:timeout",
			expected:    `message:"timeout"`,
			expectError: false,
		},
		{
			name:        "phrase query",
			query:       `"connection timeout"`,
			expected:    `_msg:"connection timeout"`,
			expectError: false,
		},
		{
			name:        "field phrase query",
			query:       `message:"server error"`,
			expected:    `message:"server error"`,
			expectError: false,
		},
		{
			name:        "wildcard query",
			query:       "service:api*",
			expected:    `service~"^api.*$"`,
			expectError: false,
		},
		{
			name:        "numeric range query",
			query:       "duration:>100",
			expected:    `duration:[100,*]`,
			expectError: false,
		},
		{
			name:        "boolean query",
			query:       "+error +timeout",
			expected:    `_msg:"error" _msg:"timeout"`,
			expectError: false,
		},
		{
			name:        "invalid query",
			query:       "invalid:::query",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := translator.TranslateToLogsQL(tt.query)

			if tt.expectError {
				if err == nil {
					t.Errorf("TranslateToLogsQL(%q) expected error but got none", tt.query)
				}
				return
			}

			if err != nil {
				t.Errorf("TranslateToLogsQL(%q) unexpected error: %v", tt.query, err)
				return
			}

			if result != tt.expected {
				t.Errorf("TranslateToLogsQL(%q) = %q, want %q", tt.query, result, tt.expected)
			}
		})
	}
}

func TestTranslateToTraces(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		name        string
		query       string
		expected    TraceFilters
		expectError bool
	}{
		{
			name:  "service filter",
			query: "service:payment",
			expected: TraceFilters{
				Service: "payment",
				Tags:    map[string]string{},
			},
			expectError: false,
		},
		{
			name:  "operation filter",
			query: "operation:charge",
			expected: TraceFilters{
				Operation: "charge",
				Tags:      map[string]string{},
			},
			expectError: false,
		},
		{
			name:  "duration filter",
			query: "duration:>1000",
			expected: TraceFilters{
				MinDuration: "1000",
				Tags:        map[string]string{},
			},
			expectError: false,
		},
		{
			name:  "tag filter",
			query: "tag.env:production",
			expected: TraceFilters{
				Tags: map[string]string{
					"env": "production",
				},
			},
			expectError: false,
		},
		{
			name:  "span attribute filter",
			query: "span_attr.version:1.2.3",
			expected: TraceFilters{
				Tags: map[string]string{
					"version": "1.2.3",
				},
			},
			expectError: false,
		},
		{
			name:  "multiple filters",
			query: "service:api operation:search tag.env:prod",
			expected: TraceFilters{
				Service:   "api",
				Operation: "search",
				Tags: map[string]string{
					"env": "prod",
				},
			},
			expectError: false,
		},
		{
			name:  "numeric range duration",
			query: "duration:>100",
			expected: TraceFilters{
				MinDuration: "100",
				Tags:        map[string]string{},
			},
			expectError: false,
		},
		{
			name:        "empty query",
			query:       "",
			expected:    TraceFilters{},
			expectError: true,
		},
		{
			name:        "invalid query",
			query:       "invalid:::query",
			expected:    TraceFilters{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := translator.TranslateToTraces(tt.query)

			if tt.expectError {
				if err == nil {
					t.Errorf("TranslateToTraces(%q) expected error but got none", tt.query)
				}
				return
			}

			if err != nil {
				t.Errorf("TranslateToTraces(%q) unexpected error: %v", tt.query, err)
				return
			}

			if result.Service != tt.expected.Service {
				t.Errorf("TranslateToTraces(%q) Service = %q, want %q", tt.query, result.Service, tt.expected.Service)
			}

			if result.Operation != tt.expected.Operation {
				t.Errorf("TranslateToTraces(%q) Operation = %q, want %q", tt.query, result.Operation, tt.expected.Operation)
			}

			if result.MinDuration != tt.expected.MinDuration {
				t.Errorf("TranslateToTraces(%q) MinDuration = %q, want %q", tt.query, result.MinDuration, tt.expected.MinDuration)
			}

			if result.MaxDuration != tt.expected.MaxDuration {
				t.Errorf("TranslateToTraces(%q) MaxDuration = %q, want %q", tt.query, result.MaxDuration, tt.expected.MaxDuration)
			}

			if len(result.Tags) != len(tt.expected.Tags) {
				t.Errorf("TranslateToTraces(%q) Tags length = %d, want %d", tt.query, len(result.Tags), len(tt.expected.Tags))
			}

			for k, v := range tt.expected.Tags {
				if result.Tags[k] != v {
					t.Errorf("TranslateToTraces(%q) Tags[%q] = %q, want %q", tt.query, k, result.Tags[k], v)
				}
			}
		})
	}
}

func TestWildcardToRegex(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		name     string
		wildcard string
		expected string
	}{
		{
			name:     "simple wildcard",
			wildcard: "api*",
			expected: "^api.*$",
		},
		{
			name:     "single character wildcard",
			wildcard: "test?",
			expected: "^test.$",
		},
		{
			name:     "multiple wildcards",
			wildcard: "*test*",
			expected: "^.*test.*$",
		},
		{
			name:     "escape special chars",
			wildcard: "test.com*",
			expected: "^test\\.com.*$",
		},
		{
			name:     "complex pattern",
			wildcard: "v[1-2].*",
			expected: "^v\\[1-2\\]\\..*$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.wildcardToRegex(tt.wildcard)
			if result != tt.expected {
				t.Errorf("wildcardToRegex(%q) = %q, want %q", tt.wildcard, result, tt.expected)
			}
		})
	}
}

func TestNewTranslator(t *testing.T) {
	translator := NewTranslator()
	if translator == nil {
		t.Error("NewTranslator() returned nil")
	}
}

func TestTranslateLuceneToLogsQL(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		name        string
		query       string
		expected    string
		expectError bool
	}{
		{
			name:        "basic word search",
			query:       "error",
			expected:    "error",
			expectError: false,
		},
		{
			name:        "field-specific search",
			query:       "level:ERROR",
			expected:    "level:ERROR",
			expectError: false,
		},
		{
			name:        "phrase search",
			query:       `"connection refused"`,
			expected:    `"connection refused"`,
			expectError: false,
		},
		{
			name:        "AND logic",
			query:       "error AND timeout",
			expected:    "error timeout",
			expectError: false,
		},
		{
			name:        "OR logic",
			query:       "error OR timeout",
			expected:    "error OR timeout",
			expectError: false,
		},
		{
			name:        "NOT logic",
			query:       "error NOT debug",
			expected:    "error -debug",
			expectError: false,
		},
		{
			name:        "grouping",
			query:       "(error OR fail) AND timeout",
			expected:    "(error OR fail) timeout",
			expectError: false,
		},
		{
			name:        "wildcard search",
			query:       "error*",
			expected:    "error*",
			expectError: false,
		},
		{
			name:        "field value in list",
			query:       "status:(500 OR 404)",
			expected:    "status:(500 OR 404)",
			expectError: false,
		},
		{
			name:        "range query",
			query:       "duration:[100 TO 200]",
			expected:    "duration:>=100 duration:<=200",
			expectError: false,
		},
		{
			name:        "regular expression",
			query:       "message:/timeout.*/",
			expected:    `message:~"timeout.*"`,
			expectError: false,
		},
		{
			name:        "negation",
			query:       "-debug",
			expected:    "-debug",
			expectError: false,
		},
		{
			name:        "bang negation",
			query:       "error !debug",
			expected:    "error -debug",
			expectError: false,
		},
		{
			name:        "complex query",
			query:       "error AND (timeout OR fail) NOT debug",
			expected:    "error (timeout OR fail) -debug",
			expectError: false,
		},
		{
			name:        "empty query",
			query:       "",
			expected:    "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := translator.TranslateLuceneToLogsQL(tt.query)

			if tt.expectError {
				if err == nil {
					t.Errorf("TranslateLuceneToLogsQL(%q) expected error but got none", tt.query)
				}
				return
			}

			if err != nil {
				t.Errorf("TranslateLuceneToLogsQL(%q) unexpected error: %v", tt.query, err)
				return
			}

			if result != tt.expected {
				t.Errorf("TranslateLuceneToLogsQL(%q) = %q, want %q", tt.query, result, tt.expected)
			}
		})
	}
}
