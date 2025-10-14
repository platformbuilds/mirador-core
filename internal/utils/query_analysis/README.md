# Query Analysis Tools

This package provides comprehensive tools for analyzing search query performance and identifying optimization opportunities in Mirador Core.

## Features

- **Query Complexity Analysis**: Analyzes query structure and assigns complexity scores
- **Performance Metrics Collection**: Records and analyzes query execution metrics
- **Optimization Suggestions**: Provides actionable recommendations for query improvement
- **Risk Identification**: Detects potentially problematic query patterns
- **Slow Query Detection**: Identifies queries exceeding performance thresholds

## Usage

### Basic Query Analysis

```go
analyzer := queryanalysis.NewAnalyzer()

analysis, err := analyzer.AnalyzeQuery(
    "level:error service:web",
    queryanalysis.QueryTypeLogs,
    queryanalysis.SearchEngineLucene,
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Complexity Score: %d/10\n", analysis.Complexity.Score)
fmt.Printf("Reason: %s\n", analysis.Complexity.Reason)
```

### Performance Metrics Recording

```go
analyzer := queryanalysis.NewAnalyzer()

// Record query execution metrics
analyzer.RecordMetrics(
    "query-id-123",
    "level:error service:web",
    queryanalysis.QueryTypeLogs,
    queryanalysis.SearchEngineLucene,
    150*time.Millisecond, // execution time
    500,                  // result count
)

// Get performance summary
summary := analyzer.GetMetricsSummary()
fmt.Printf("Total Queries: %v\n", summary["total_queries"])
fmt.Printf("Average Time: %v\n", summary["average_time"])
```

### Slow Query Detection

```go
// Find queries exceeding 500ms threshold
slowQueries := analyzer.GetSlowQueries(500 * time.Millisecond)
for _, query := range slowQueries {
    fmt.Printf("Slow Query: %s took %v\n",
        query.QueryText, query.ExecutionTime)
}
```

## Query Types

- `QueryTypeLogs`: For log search queries
- `QueryTypeTraces`: For trace search queries

## Search Engines

- `SearchEngineLucene`: Traditional Lucene query syntax
- `SearchEngineBleve`: Structured field:value syntax

## Complexity Scoring

Queries are scored from 1-10 based on:
- Number of field filters
- Use of wildcards
- Range query complexity
- Boolean operations (OR, NOT)
- Negation operations

### Score Interpretation

- **1-3**: Low complexity - efficient queries
- **4-6**: Medium complexity - monitor performance
- **7-10**: High complexity - consider optimization

## Optimization Suggestions

The analyzer provides prioritized optimization suggestions:

- **High Priority**: Add time range filters (`_time:15m`)
- **Medium Priority**: Optimize wildcard usage
- **Low Priority**: Reduce field filter count

## Integration Example

```go
// In your search handler
func (h *SearchHandler) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
    // Analyze query before execution
    analysis, _ := h.queryAnalyzer.AnalyzeQuery(
        req.Query, req.QueryType, req.SearchEngine)

    // Log complexity for monitoring
    if analysis.Complexity.Score > 7 {
        log.Warnf("High complexity query detected: %s (score: %d)",
            req.Query, analysis.Complexity.Score)
    }

    // Execute search
    start := time.Now()
    results, err := h.searcher.Search(ctx, req)
    executionTime := time.Since(start)

    // Record metrics
    h.queryAnalyzer.RecordMetrics(
        req.ID, req.Query, req.QueryType, req.SearchEngine,
        executionTime, len(results.Hits))

    return results, err
}
```

## Best Practices

1. **Always include time filters** in production queries to limit data scanned
2. **Monitor query complexity** and optimize high-scoring queries
3. **Use appropriate result limits** to prevent memory issues
4. **Cache frequent queries** when possible
5. **Test queries with small limits** before production use

## Performance Hints

The analyzer provides performance guidance based on query characteristics:

- **Estimated Cost**: Low/Medium/High based on complexity
- **Suggestions**: Engine-specific performance tips
- **Best Practices**: General query optimization guidelines