package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorrelationQueryParser_Parse(t *testing.T) {
	parser := NewCorrelationQueryParser()

	tests := []struct {
		name        string
		query       string
		expected    *CorrelationQuery
		expectError bool
	}{
		{
			name:  "simple AND correlation",
			query: "logs:error AND metrics:high_latency",
			expected: &CorrelationQuery{
				RawQuery: "logs:error AND metrics:high_latency",
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
					{Engine: QueryTypeMetrics, Query: "high_latency"},
				},
				Operator: CorrelationOpAND,
			},
			expectError: false,
		},
		{
			name:  "simple OR correlation",
			query: "logs:error OR logs:warn",
			expected: &CorrelationQuery{
				RawQuery: "logs:error OR logs:warn",
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
					{Engine: QueryTypeLogs, Query: "warn"},
				},
				Operator: CorrelationOpOR,
			},
			expectError: false,
		},
		{
			name:  "time-window correlation",
			query: "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
			expected: &CorrelationQuery{
				RawQuery: "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
					{Engine: QueryTypeMetrics, Query: "cpu_usage", Condition: " > 80"},
				},
				Operator:   CorrelationOpAND,
				TimeWindow: func() *time.Duration { d := 5 * time.Minute; return &d }(),
			},
			expectError: false,
		},
		{
			name:  "single expression",
			query: "logs:error",
			expected: &CorrelationQuery{
				RawQuery: "logs:error",
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
				},
				Operator: CorrelationOpAND,
			},
			expectError: false,
		},
		{
			name:        "empty query",
			query:       "",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "missing engine prefix",
			query:       "error AND metrics:high_latency",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid time window",
			query:       "logs:error WITHIN invalid OF metrics:cpu",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.query)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Compare key fields
				assert.Equal(t, tt.expected.RawQuery, result.RawQuery)
				assert.Equal(t, tt.expected.Operator, result.Operator)
				assert.Len(t, result.Expressions, len(tt.expected.Expressions))

				for i, expectedExpr := range tt.expected.Expressions {
					actualExpr := result.Expressions[i]
					assert.Equal(t, expectedExpr.Engine, actualExpr.Engine)
					assert.Equal(t, expectedExpr.Query, actualExpr.Query)
					assert.Equal(t, expectedExpr.Condition, actualExpr.Condition)
				}

				if tt.expected.TimeWindow != nil {
					assert.NotNil(t, result.TimeWindow)
					assert.Equal(t, *tt.expected.TimeWindow, *result.TimeWindow)
				} else {
					assert.Nil(t, result.TimeWindow)
				}
			}
		})
	}
}

func TestCorrelationQueryParser_ParseTimeWindow(t *testing.T) {
	parser := NewCorrelationQueryParser()

	tests := []struct {
		name        string
		input       string
		expectedDur time.Duration
		expectedRem string
		expectError bool
	}{
		{
			name:        "minutes",
			input:       "5m OF metrics:cpu",
			expectedDur: 5 * time.Minute,
			expectedRem: "OF metrics:cpu",
			expectError: false,
		},
		{
			name:        "hours",
			input:       "2h OF logs:error",
			expectedDur: 2 * time.Hour,
			expectedRem: "OF logs:error",
			expectError: false,
		},
		{
			name:        "seconds",
			input:       "30s OF traces:service",
			expectedDur: 30 * time.Second,
			expectedRem: "OF traces:service",
			expectError: false,
		},
		{
			name:        "days",
			input:       "1d OF metrics:memory",
			expectedDur: 24 * time.Hour,
			expectedRem: "OF metrics:memory",
			expectError: false,
		},
		{
			name:        "invalid unit",
			input:       "5x OF metrics:cpu",
			expectedDur: 0,
			expectedRem: "",
			expectError: true,
		},
		{
			name:        "invalid format",
			input:       "invalid OF metrics:cpu",
			expectedDur: 0,
			expectedRem: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dur, rem, err := parser.parseTimeWindow(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedDur, dur)
				assert.Equal(t, tt.expectedRem, rem)
			}
		})
	}
}

func TestCorrelationQueryParser_ParseSingleExpression(t *testing.T) {
	parser := NewCorrelationQueryParser()

	tests := []struct {
		name        string
		expr        string
		expected    *CorrelationExpression
		expectError bool
	}{
		{
			name: "logs with condition",
			expr: "logs:error > 10",
			expected: &CorrelationExpression{
				Engine:    QueryTypeLogs,
				Query:     "error",
				Condition: " > 10",
			},
			expectError: false,
		},
		{
			name: "metrics with condition",
			expr: "metrics:cpu_usage == 80",
			expected: &CorrelationExpression{
				Engine:    QueryTypeMetrics,
				Query:     "cpu_usage",
				Condition: " == 80",
			},
			expectError: false,
		},
		{
			name: "traces without condition",
			expr: "traces:service:checkout",
			expected: &CorrelationExpression{
				Engine: QueryTypeTraces,
				Query:  "service:checkout",
			},
			expectError: false,
		},
		{
			name:        "missing colon",
			expr:        "logs error",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "unknown engine",
			expr:        "unknown:query",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.parseSingleExpression(tt.expr)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Engine, result.Engine)
				assert.Equal(t, tt.expected.Query, result.Query)
				assert.Equal(t, tt.expected.Condition, result.Condition)
			}
		})
	}
}

func TestCorrelationQuery_Validate(t *testing.T) {
	tests := []struct {
		name        string
		query       *CorrelationQuery
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid simple correlation",
			query: &CorrelationQuery{
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
					{Engine: QueryTypeMetrics, Query: "cpu"},
				},
				Operator: CorrelationOpAND,
			},
			expectError: false,
		},
		{
			name: "valid time-window correlation",
			query: &CorrelationQuery{
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
					{Engine: QueryTypeMetrics, Query: "cpu"},
				},
				Operator:   CorrelationOpAND,
				TimeWindow: func() *time.Duration { d := 5 * time.Minute; return &d }(),
			},
			expectError: false,
		},
		{
			name: "invalid - no expressions",
			query: &CorrelationQuery{
				Expressions: []CorrelationExpression{},
				Operator:    CorrelationOpAND,
			},
			expectError: true,
			errorMsg:    "correlation query must have at least one expression",
		},
		{
			name: "invalid - time-window with wrong expression count",
			query: &CorrelationQuery{
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
				},
				Operator:   CorrelationOpAND,
				TimeWindow: func() *time.Duration { d := 5 * time.Minute; return &d }(),
			},
			expectError: true,
			errorMsg:    "time-window correlation requires exactly 2 expressions",
		},
		{
			name: "invalid - time-window with OR operator",
			query: &CorrelationQuery{
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
					{Engine: QueryTypeMetrics, Query: "cpu"},
				},
				Operator:   CorrelationOpOR,
				TimeWindow: func() *time.Duration { d := 5 * time.Minute; return &d }(),
			},
			expectError: true,
			errorMsg:    "time-window correlation only supports AND operator",
		},
		{
			name: "invalid - missing engine",
			query: &CorrelationQuery{
				Expressions: []CorrelationExpression{
					{Query: "error"}, // missing engine
				},
				Operator: CorrelationOpAND,
			},
			expectError: true,
			errorMsg:    "missing engine",
		},
		{
			name: "invalid - missing query",
			query: &CorrelationQuery{
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs}, // missing query
				},
				Operator: CorrelationOpAND,
			},
			expectError: true,
			errorMsg:    "missing query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCorrelationQuery_String(t *testing.T) {
	tests := []struct {
		name     string
		query    *CorrelationQuery
		expected string
	}{
		{
			name: "simple AND correlation",
			query: &CorrelationQuery{
				RawQuery: "logs:error AND metrics:high_latency",
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
					{Engine: QueryTypeMetrics, Query: "high_latency"},
				},
				Operator: CorrelationOpAND,
			},
			expected: "logs:error AND metrics:high_latency",
		},
		{
			name: "time-window correlation",
			query: &CorrelationQuery{
				RawQuery: "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
				Expressions: []CorrelationExpression{
					{Engine: QueryTypeLogs, Query: "error"},
					{Engine: QueryTypeMetrics, Query: "cpu_usage", Condition: " > 80"},
				},
				Operator:   CorrelationOpAND,
				TimeWindow: func() *time.Duration { d := 5 * time.Minute; return &d }(),
			},
			expected: "(logs:error AND metrics:cpu_usage > 80) WITHIN 5m OF metrics:cpu_usage > 80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.query.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
