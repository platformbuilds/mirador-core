package lucene

import (
	"testing"
)

func TestTranslateToLogsQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple term",
			input:    "error",
			expected: `_msg:"error"`,
			wantErr:  false,
		},
		{
			name:     "field term",
			input:    "level:error",
			expected: `level:"error"`,
			wantErr:  false,
		},
		{
			name:     "phrase",
			input:    `"connection timeout"`,
			expected: `_msg:"connection timeout"`,
			wantErr:  false,
		},
		{
			name:     "field phrase",
			input:    `message:"connection timeout"`,
			expected: `message:"connection timeout"`,
			wantErr:  false,
		},
		{
			name:     "boolean AND",
			input:    "error AND timeout",
			expected: `_msg:"error" AND _msg:"timeout"`,
			wantErr:  false,
		},
		{
			name:     "boolean OR",
			input:    "error OR timeout",
			expected: `_msg:"error" OR _msg:"timeout"`,
			wantErr:  false,
		},
		{
			name:     "wildcard",
			input:    "error*",
			expected: `_msg~".*error.*"`,
			wantErr:  false,
		},
		{
			name:     "field wildcard",
			input:    "level:err*",
			expected: `level~".*err.*"`,
			wantErr:  false,
		},
		{
			name:     "numeric range",
			input:    "duration:[100 TO 500]",
			expected: `duration:[100,500]`,
			wantErr:  false,
		},
		{
			name:     "complex query",
			input:    `level:error AND (message:"timeout" OR message:"failed")`,
			expected: `level:"error" AND (message:"timeout" OR message:"failed")`,
			wantErr:  false,
		},
		{
			name:     "unsupported fuzzy",
			input:    "error~0.8",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := translateToLogsQL(tt.input)
			if ok != tt.wantErr {
				t.Errorf("translateToLogsQL() ok = %v, wantErr %v", ok, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("translateToLogsQL() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTranslateTraces(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TraceFilters
		wantOk   bool
	}{
		{
			name:  "service filter",
			input: "service:web",
			expected: TraceFilters{
				Service: "web",
				Tags:    map[string]string{},
			},
			wantOk: true,
		},
		{
			name:  "operation filter",
			input: "operation:login",
			expected: TraceFilters{
				Operation: "login",
				Tags:      map[string]string{},
			},
			wantOk: true,
		},
		{
			name:  "duration range",
			input: "duration:[100ms TO 1s]",
			expected: TraceFilters{
				MinDuration: "100ms",
				MaxDuration: "1s",
				Tags:        map[string]string{},
			},
			wantOk: true,
		},
		{
			name:  "time filter",
			input: "_time:15m",
			expected: TraceFilters{
				Since: "15m",
				Tags:  map[string]string{},
			},
			wantOk: true,
		},
		{
			name:  "tag filter",
			input: "tag.user:john",
			expected: TraceFilters{
				Tags: map[string]string{"user": "john"},
			},
			wantOk: true,
		},
		{
			name:  "span_attr filter",
			input: "span_attr.key:value",
			expected: TraceFilters{
				Tags: map[string]string{"key": "value"},
			},
			wantOk: true,
		},
		{
			name:  "complex filters",
			input: "service:api AND operation:search AND tag.env:prod",
			expected: TraceFilters{
				Service:   "api",
				Operation: "search",
				Tags:      map[string]string{"env": "prod"},
			},
			wantOk: true,
		},
		{
			name:     "invalid query",
			input:    "invalid~fuzzy",
			expected: TraceFilters{},
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := TranslateTraces(tt.input)
			if ok != tt.wantOk {
				t.Errorf("TranslateTraces() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if !tt.wantOk {
				return
			}
			if got.Service != tt.expected.Service ||
				got.Operation != tt.expected.Operation ||
				got.MinDuration != tt.expected.MinDuration ||
				got.MaxDuration != tt.expected.MaxDuration ||
				got.Since != tt.expected.Since {
				t.Errorf("TranslateTraces() filters = %v, want %v", got, tt.expected)
			}
			if len(got.Tags) != len(tt.expected.Tags) {
				t.Errorf("TranslateTraces() tags length = %v, want %v", len(got.Tags), len(tt.expected.Tags))
			}
			for k, v := range tt.expected.Tags {
				if got.Tags[k] != v {
					t.Errorf("TranslateTraces() tag %s = %v, want %v", k, got.Tags[k], v)
				}
			}
		})
	}
}

func BenchmarkTranslateToLogsQL(b *testing.B) {
	queries := []string{
		"error",
		"level:error",
		`"connection timeout"`,
		"error AND timeout",
		"service:api*",
		"duration:[100 TO 500]",
		`level:error AND (message:"timeout" OR message:"failed")`,
	}

	for _, query := range queries {
		b.Run(query, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				translateToLogsQL(query)
			}
		})
	}
}

func BenchmarkTranslateTraces(b *testing.B) {
	queries := []string{
		"service:web",
		"operation:login",
		"duration:[100ms TO 1s]",
		"service:api AND operation:search",
		"tag.env:prod",
	}

	for _, query := range queries {
		b.Run(query, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				TranslateTraces(query)
			}
		})
	}
}
