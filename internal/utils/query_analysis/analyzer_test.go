package queryanalysis

import (
	"testing"
	"time"
)

func TestAnalyzeQuery(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		name         string
		query        string
		queryType    QueryType
		searchEngine SearchEngine
		expectError  bool
	}{
		{
			name:         "simple lucene query",
			query:        "level:error",
			queryType:    QueryTypeLogs,
			searchEngine: SearchEngineLucene,
			expectError:  false,
		},
		{
			name:         "complex bleve query",
			query:        "_time:15m +level:error -service:database host:prod*",
			queryType:    QueryTypeLogs,
			searchEngine: SearchEngineBleve,
			expectError:  false,
		},
		{
			name:         "trace query",
			query:        "service:api operation:GET duration:[100ms TO *]",
			queryType:    QueryTypeTraces,
			searchEngine: SearchEngineLucene,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.AnalyzeQuery(tt.query, tt.queryType, tt.searchEngine)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if analysis != nil {
				if analysis.QueryText != tt.query {
					t.Errorf("expected query text %q, got %q", tt.query, analysis.QueryText)
				}

				if analysis.Complexity.Score < 1 || analysis.Complexity.Score > 10 {
					t.Errorf("complexity score out of range: %d", analysis.Complexity.Score)
				}

				if len(analysis.Optimizations) == 0 && analysis.Complexity.Score > 5 {
					t.Logf("High complexity query %q has no optimizations suggested", tt.query)
				}
			}
		})
	}
}

func TestRecordMetrics(t *testing.T) {
	analyzer := NewAnalyzer()

	// Record some metrics
	analyzer.RecordMetrics("query1", "level:error", QueryTypeLogs, SearchEngineLucene, 100*time.Millisecond, 50)
	analyzer.RecordMetrics("query2", "_time:15m service:api", QueryTypeTraces, SearchEngineBleve, 200*time.Millisecond, 25)

	// Check metrics were recorded
	summary := analyzer.GetMetricsSummary()
	totalQueries, ok := summary["total_queries"].(int)
	if !ok || totalQueries != 2 {
		t.Errorf("expected 2 total queries, got %v", summary["total_queries"])
	}
}

func TestGetSlowQueries(t *testing.T) {
	analyzer := NewAnalyzer()

	// Record metrics with different execution times
	analyzer.RecordMetrics("fast", "level:info", QueryTypeLogs, SearchEngineLucene, 50*time.Millisecond, 100)
	analyzer.RecordMetrics("slow", "complex query", QueryTypeLogs, SearchEngineLucene, 500*time.Millisecond, 10)

	// Get slow queries (threshold 200ms)
	slowQueries := analyzer.GetSlowQueries(200 * time.Millisecond)

	if len(slowQueries) != 1 {
		t.Errorf("expected 1 slow query, got %d", len(slowQueries))
	}

	if len(slowQueries) > 0 && slowQueries[0].QueryID != "slow" {
		t.Errorf("expected slow query ID 'slow', got %s", slowQueries[0].QueryID)
	}
}

func TestAnalyzeComplexity(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		name         string
		query        string
		searchEngine SearchEngine
		minScore     int
		maxScore     int
	}{
		{
			name:         "simple query",
			query:        "error",
			searchEngine: SearchEngineLucene,
			minScore:     1,
			maxScore:     3,
		},
		{
			name:         "complex query",
			query:        "_time:15m +level:error -service:database host:prod* status:[400 TO 599]",
			searchEngine: SearchEngineBleve,
			minScore:     5,
			maxScore:     10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complexity, err := analyzer.analyzeComplexity(tt.query, tt.searchEngine)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if complexity.Score < tt.minScore || complexity.Score > tt.maxScore {
				t.Errorf("complexity score %d not in expected range [%d, %d]", complexity.Score, tt.minScore, tt.maxScore)
			}

			if complexity.FieldCount != len(complexity.ExpensiveOps) && complexity.FieldCount > 0 {
				// Just log this - it's informational
				t.Logf("Query %q has %d fields and %d expensive ops", tt.query, complexity.FieldCount, len(complexity.ExpensiveOps))
			}
		})
	}
}
