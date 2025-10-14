package queryanalysis

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// QueryType represents the type of query being analyzed
type QueryType int

const (
	QueryTypeLogs QueryType = iota
	QueryTypeTraces
)

// SearchEngine represents the search engine used
type SearchEngine int

const (
	SearchEngineLucene SearchEngine = iota
	SearchEngineBleve
)

// QueryMetrics holds performance metrics for a query
type QueryMetrics struct {
	QueryID       string        `json:"query_id"`
	QueryType     QueryType     `json:"query_type"`
	SearchEngine  SearchEngine  `json:"search_engine"`
	QueryText     string        `json:"query_text"`
	ExecutionTime time.Duration `json:"execution_time"`
	ResultCount   int           `json:"result_count"`
	Complexity    int           `json:"complexity_score"`
	Timestamp     time.Time     `json:"timestamp"`
}

// QueryAnalysis holds detailed analysis of a query
type QueryAnalysis struct {
	QueryText     string           `json:"query_text"`
	QueryType     QueryType        `json:"query_type"`
	SearchEngine  SearchEngine     `json:"search_engine"`
	Complexity    QueryComplexity  `json:"complexity"`
	Optimizations []Optimization   `json:"optimizations"`
	Risks         []string         `json:"risks"`
	Performance   PerformanceHints `json:"performance"`
}

// QueryComplexity represents the computational complexity of a query
type QueryComplexity struct {
	Score         int      `json:"score"` // 1-10 scale
	Reason        string   `json:"reason"`
	ExpensiveOps  []string `json:"expensive_ops"`
	FieldCount    int      `json:"field_count"`
	WildcardCount int      `json:"wildcard_count"`
	RangeCount    int      `json:"range_count"`
}

// Optimization represents a suggested optimization
type Optimization struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // "high", "medium", "low"
	Impact      string `json:"impact"`
}

// PerformanceHints provides performance-related guidance
type PerformanceHints struct {
	EstimatedCost string   `json:"estimated_cost"`
	Suggestions   []string `json:"suggestions"`
	BestPractices []string `json:"best_practices"`
}

// Analyzer provides query analysis functionality
type Analyzer struct {
	metrics []QueryMetrics
}

// NewAnalyzer creates a new query analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		metrics: make([]QueryMetrics, 0),
	}
}

// AnalyzeQuery performs comprehensive analysis of a query
func (a *Analyzer) AnalyzeQuery(queryText string, queryType QueryType, searchEngine SearchEngine) (*QueryAnalysis, error) {
	analysis := &QueryAnalysis{
		QueryText:    queryText,
		QueryType:    queryType,
		SearchEngine: searchEngine,
	}

	// Analyze complexity
	complexity, err := a.analyzeComplexity(queryText, searchEngine)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze complexity: %w", err)
	}
	analysis.Complexity = *complexity

	// Generate optimizations
	analysis.Optimizations = a.generateOptimizations(queryText, searchEngine, complexity)

	// Identify risks
	analysis.Risks = a.identifyRisks(queryText, searchEngine)

	// Performance hints
	analysis.Performance = a.generatePerformanceHints(complexity, searchEngine)

	return analysis, nil
}

// RecordMetrics records performance metrics for a query execution
func (a *Analyzer) RecordMetrics(queryID, queryText string, queryType QueryType, searchEngine SearchEngine, executionTime time.Duration, resultCount int) {
	complexity := a.calculateComplexityScore(queryText, searchEngine)

	metrics := QueryMetrics{
		QueryID:       queryID,
		QueryType:     queryType,
		SearchEngine:  searchEngine,
		QueryText:     queryText,
		ExecutionTime: executionTime,
		ResultCount:   resultCount,
		Complexity:    complexity,
		Timestamp:     time.Now(),
	}

	a.metrics = append(a.metrics, metrics)
}

// GetSlowQueries returns queries that exceed the given execution time threshold
func (a *Analyzer) GetSlowQueries(threshold time.Duration) []QueryMetrics {
	var slowQueries []QueryMetrics
	for _, metric := range a.metrics {
		if metric.ExecutionTime > threshold {
			slowQueries = append(slowQueries, metric)
		}
	}
	return slowQueries
}

// GetMetricsSummary returns a summary of collected metrics
func (a *Analyzer) GetMetricsSummary() map[string]interface{} {
	if len(a.metrics) == 0 {
		return map[string]interface{}{
			"total_queries": 0,
			"message":       "No metrics collected yet",
		}
	}

	totalQueries := len(a.metrics)
	totalTime := time.Duration(0)
	maxTime := time.Duration(0)
	totalResults := 0
	avgComplexity := 0

	for _, metric := range a.metrics {
		totalTime += metric.ExecutionTime
		if metric.ExecutionTime > maxTime {
			maxTime = metric.ExecutionTime
		}
		totalResults += metric.ResultCount
		avgComplexity += metric.Complexity
	}

	avgTime := totalTime / time.Duration(totalQueries)
	avgComplexity = avgComplexity / totalQueries

	return map[string]interface{}{
		"total_queries":      totalQueries,
		"average_time":       avgTime.String(),
		"max_time":           maxTime.String(),
		"total_results":      totalResults,
		"average_complexity": avgComplexity,
		"engine_breakdown":   a.getEngineBreakdown(),
	}
}

// analyzeComplexity analyzes the computational complexity of a query
func (a *Analyzer) analyzeComplexity(queryText string, searchEngine SearchEngine) (*QueryComplexity, error) {
	complexity := &QueryComplexity{}

	// Count fields (field:value pairs)
	fieldRegex := regexp.MustCompile(`(\w+):`)
	fields := fieldRegex.FindAllString(queryText, -1)
	complexity.FieldCount = len(fields)

	// Count wildcards
	wildcardRegex := regexp.MustCompile(`\*`)
	complexity.WildcardCount = len(wildcardRegex.FindAllString(queryText, -1))

	// Count ranges (different patterns for Lucene vs Bleve)
	if searchEngine == SearchEngineLucene {
		rangeRegex := regexp.MustCompile(`\[.*?\]`)
		complexity.RangeCount = len(rangeRegex.FindAllString(queryText, -1))
	} else {
		rangeRegex := regexp.MustCompile(`\[.*?\]`)
		complexity.RangeCount = len(rangeRegex.FindAllString(queryText, -1))
	}

	// Identify expensive operations
	var expensiveOps []string
	if complexity.WildcardCount > 2 {
		expensiveOps = append(expensiveOps, "multiple wildcards")
	}
	if complexity.FieldCount > 5 {
		expensiveOps = append(expensiveOps, "many field filters")
	}
	if complexity.RangeCount > 2 {
		expensiveOps = append(expensiveOps, "multiple range queries")
	}
	if strings.Contains(queryText, "OR") || strings.Contains(queryText, "|") {
		expensiveOps = append(expensiveOps, "OR operations")
	}
	if strings.Contains(queryText, "NOT") || strings.Contains(queryText, "-") {
		expensiveOps = append(expensiveOps, "negation operations")
	}

	complexity.ExpensiveOps = expensiveOps

	// Calculate complexity score (1-10)
	score := 1
	score += complexity.FieldCount / 2
	score += complexity.WildcardCount
	score += complexity.RangeCount
	if len(expensiveOps) > 0 {
		score += len(expensiveOps) * 2
	}

	if score > 10 {
		score = 10
	}
	complexity.Score = score

	// Generate reason
	if score <= 3 {
		complexity.Reason = "Low complexity - efficient query"
	} else if score <= 6 {
		complexity.Reason = "Medium complexity - monitor performance"
	} else {
		complexity.Reason = "High complexity - consider optimization"
	}

	return complexity, nil
}

// calculateComplexityScore returns a simple complexity score for metrics
func (a *Analyzer) calculateComplexityScore(queryText string, searchEngine SearchEngine) int {
	complexity, _ := a.analyzeComplexity(queryText, searchEngine)
	return complexity.Score
}

// generateOptimizations suggests optimizations for the query
func (a *Analyzer) generateOptimizations(queryText string, searchEngine SearchEngine, complexity *QueryComplexity) []Optimization {
	var optimizations []Optimization

	// Time range optimizations
	if !strings.Contains(queryText, "_time:") {
		optimizations = append(optimizations, Optimization{
			Type:        "time_filter",
			Description: "Add time range filter (_time:15m) to limit search scope",
			Priority:    "high",
			Impact:      "Reduces data scanned by 90%+ for typical use cases",
		})
	}

	// Wildcard optimizations
	if complexity.WildcardCount > 0 {
		optimizations = append(optimizations, Optimization{
			Type:        "wildcard_optimization",
			Description: "Leading wildcards (*text) are expensive - consider field-specific searches",
			Priority:    "medium",
			Impact:      "Improves query performance by avoiding full scans",
		})
	}

	// Field count optimization
	if complexity.FieldCount > 3 {
		optimizations = append(optimizations, Optimization{
			Type:        "field_reduction",
			Description: "Consider reducing field filters or using broader field matching",
			Priority:    "low",
			Impact:      "Reduces query complexity and improves execution time",
		})
	}

	// Engine-specific optimizations
	if searchEngine == SearchEngineBleve {
		if strings.Contains(queryText, `"`) {
			optimizations = append(optimizations, Optimization{
				Type:        "phrase_query",
				Description: "Phrase queries in Bleve are efficient - consider using them for exact matches",
				Priority:    "low",
				Impact:      "Better precision and performance for exact phrase searches",
			})
		}
	}

	return optimizations
}

// identifyRisks identifies potential issues with the query
func (a *Analyzer) identifyRisks(queryText string, searchEngine SearchEngine) []string {
	var risks []string

	// Check for potentially problematic patterns
	if strings.Contains(queryText, "*") && !strings.Contains(queryText, ":") {
		risks = append(risks, "Wildcard without field specification may scan all fields")
	}

	if len(queryText) > 500 {
		risks = append(risks, "Very long query may impact performance")
	}

	if strings.Count(queryText, "(") != strings.Count(queryText, ")") {
		risks = append(risks, "Unbalanced parentheses may cause parsing errors")
	}

	// Bleve-specific risks
	if searchEngine == SearchEngineBleve {
		if strings.Contains(queryText, "{") && strings.Contains(queryText, "}") {
			risks = append(risks, "Lucene-style range queries may not work optimally in Bleve")
		}
	}

	return risks
}

// generatePerformanceHints provides performance guidance
func (a *Analyzer) generatePerformanceHints(complexity *QueryComplexity, searchEngine SearchEngine) PerformanceHints {
	hints := PerformanceHints{}

	// Estimated cost based on complexity
	switch {
	case complexity.Score <= 3:
		hints.EstimatedCost = "Low"
	case complexity.Score <= 6:
		hints.EstimatedCost = "Medium"
	default:
		hints.EstimatedCost = "High"
	}

	// General suggestions
	hints.Suggestions = []string{
		"Use time ranges to limit data scanned",
		"Prefer exact matches over wildcards when possible",
		"Test queries with small result limits first",
	}

	// Engine-specific suggestions
	if searchEngine == SearchEngineBleve {
		hints.Suggestions = append(hints.Suggestions, "Bleve excels at structured field:value queries")
	} else {
		hints.Suggestions = append(hints.Suggestions, "Lucene supports complex boolean operations efficiently")
	}

	// Best practices
	hints.BestPractices = []string{
		"Always include time filters for production queries",
		"Use appropriate result limits to prevent memory issues",
		"Monitor query performance and optimize slow queries",
		"Consider query caching for frequently executed queries",
	}

	return hints
}

// getEngineBreakdown returns metrics breakdown by search engine
func (a *Analyzer) getEngineBreakdown() map[string]interface{} {
	breakdown := make(map[string]interface{})
	luceneCount := 0
	bleveCount := 0
	luceneTime := time.Duration(0)
	bleveTime := time.Duration(0)

	for _, metric := range a.metrics {
		if metric.SearchEngine == SearchEngineLucene {
			luceneCount++
			luceneTime += metric.ExecutionTime
		} else {
			bleveCount++
			bleveTime += metric.ExecutionTime
		}
	}

	breakdown["lucene_queries"] = luceneCount
	breakdown["bleve_queries"] = bleveCount

	if luceneCount > 0 {
		breakdown["lucene_avg_time"] = (luceneTime / time.Duration(luceneCount)).String()
	}
	if bleveCount > 0 {
		breakdown["bleve_avg_time"] = (bleveTime / time.Duration(bleveCount)).String()
	}

	return breakdown
}
