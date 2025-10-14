package queryanalysis

// This file contains examples of how to use the query analysis tools.
// These examples demonstrate the main functionality for analyzing query performance
// and identifying optimization opportunities.

import (
	"fmt"
	"log"
	"time"
)

// ExampleAnalyzeQuery demonstrates basic query analysis
func ExampleAnalyzeQuery() {
	analyzer := NewAnalyzer()

	// Analyze a simple Lucene query
	analysis, err := analyzer.AnalyzeQuery("level:error service:web", QueryTypeLogs, SearchEngineLucene)
	if err != nil {
		log.Printf("Error analyzing query: %v", err)
		return
	}

	fmt.Printf("Query: %s\n", analysis.QueryText)
	fmt.Printf("Complexity Score: %d/10 (%s)\n", analysis.Complexity.Score, analysis.Complexity.Reason)
	fmt.Printf("Field Count: %d\n", analysis.Complexity.FieldCount)

	if len(analysis.Optimizations) > 0 {
		fmt.Println("Suggested Optimizations:")
		for _, opt := range analysis.Optimizations {
			fmt.Printf("  - %s (%s priority): %s\n", opt.Type, opt.Priority, opt.Description)
		}
	}

	if len(analysis.Risks) > 0 {
		fmt.Println("Potential Risks:")
		for _, risk := range analysis.Risks {
			fmt.Printf("  - %s\n", risk)
		}
	}
}

// ExampleRecordMetrics demonstrates performance metrics recording
func ExampleRecordMetrics() {
	analyzer := NewAnalyzer()

	// Simulate recording metrics for different queries
	analyzer.RecordMetrics("query-1", "level:error", QueryTypeLogs, SearchEngineLucene, 150*time.Millisecond, 500)
	analyzer.RecordMetrics("query-2", "_time:15m service:api operation:GET", QueryTypeTraces, SearchEngineBleve, 75*time.Millisecond, 50)
	analyzer.RecordMetrics("query-3", "complex query with many fields", QueryTypeLogs, SearchEngineLucene, 2*time.Second, 10000)

	// Get summary of all recorded metrics
	summary := analyzer.GetMetricsSummary()
	fmt.Printf("Total Queries: %v\n", summary["total_queries"])
	fmt.Printf("Average Time: %v\n", summary["average_time"])
	fmt.Printf("Max Time: %v\n", summary["max_time"])

	// Get slow queries (queries taking longer than 500ms)
	slowQueries := analyzer.GetSlowQueries(500 * time.Millisecond)
	fmt.Printf("Slow Queries (>500ms): %d\n", len(slowQueries))

	for _, query := range slowQueries {
		fmt.Printf("  - Query '%s' took %v\n", query.QueryText, query.ExecutionTime)
	}
}

// ExampleComplexQueryAnalysis demonstrates analysis of a complex query
func ExampleComplexQueryAnalysis() {
	analyzer := NewAnalyzer()

	complexQuery := "_time:30m +level:(error OR warn) -service:(health OR metrics) host:prod* status:[400 TO 599]"

	analysis, err := analyzer.AnalyzeQuery(complexQuery, QueryTypeLogs, SearchEngineBleve)
	if err != nil {
		log.Printf("Error analyzing complex query: %v", err)
		return
	}

	fmt.Printf("Complex Query Analysis:\n")
	fmt.Printf("Query: %s\n", analysis.QueryText)
	fmt.Printf("Complexity: %d/10 - %s\n", analysis.Complexity.Score, analysis.Complexity.Reason)
	fmt.Printf("Fields: %d, Wildcards: %d, Ranges: %d\n",
		analysis.Complexity.FieldCount,
		analysis.Complexity.WildcardCount,
		analysis.Complexity.RangeCount)

	if len(analysis.Complexity.ExpensiveOps) > 0 {
		fmt.Printf("Expensive Operations: %v\n", analysis.Complexity.ExpensiveOps)
	}

	fmt.Printf("Estimated Cost: %s\n", analysis.Performance.EstimatedCost)

	fmt.Println("Performance Suggestions:")
	for _, suggestion := range analysis.Performance.Suggestions {
		fmt.Printf("  - %s\n", suggestion)
	}
}

// ExampleQueryOptimization demonstrates how to use optimization suggestions
func ExampleQueryOptimization() {
	analyzer := NewAnalyzer()

	// Query without time filter (common optimization opportunity)
	queryWithoutTime := "level:error service:web host:prod01"

	analysis, err := analyzer.AnalyzeQuery(queryWithoutTime, QueryTypeLogs, SearchEngineLucene)
	if err != nil {
		log.Printf("Error analyzing query: %v", err)
		return
	}

	fmt.Printf("Query Optimization Example:\n")
	fmt.Printf("Original Query: %s\n", queryWithoutTime)
	fmt.Printf("Complexity Score: %d/10\n", analysis.Complexity.Score)

	if len(analysis.Optimizations) > 0 {
		fmt.Println("Optimization Opportunities:")
		for i, opt := range analysis.Optimizations {
			fmt.Printf("%d. %s\n", i+1, opt.Description)
			fmt.Printf("   Priority: %s, Impact: %s\n", opt.Priority, opt.Impact)
		}

		// Suggest optimized version
		optimizedQuery := "_time:15m " + queryWithoutTime
		fmt.Printf("Suggested Optimized Query: %s\n", optimizedQuery)

		optimizedAnalysis, _ := analyzer.AnalyzeQuery(optimizedQuery, QueryTypeLogs, SearchEngineLucene)
		fmt.Printf("Optimized Complexity Score: %d/10\n", optimizedAnalysis.Complexity.Score)
	}
}
