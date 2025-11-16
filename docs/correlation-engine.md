# Mirador Core  - Log-Metrics-Traces Correlation Engine

## Overview

The Correlation Engine enables unified querying across logs, metrics, and traces data sources, providing intelligent correlation capabilities to identify relationships between observability data points.

## Correlation Query Syntax

The correlation engine supports a unified query language that allows combining queries from different engines with correlation operators.

### Basic Syntax

```
engine:query [AND|OR] engine:query [WITHIN time_window OF engine:query]
```

### Engine Prefixes

- `logs:` - Query logs data
- `metrics:` - Query metrics data
- `traces:` - Query traces data

### Operators

- `AND` - Logical AND operation
- `OR` - Logical OR operation
- `WITHIN time_window OF` - Time-window correlation

### Time Windows

Time windows specify the maximum time difference allowed between correlated events:

- `5s` - 5 seconds
- `10m` - 10 minutes
- `1h` - 1 hour
- `2d` - 2 days

### Conditions

You can add conditions to filter data:

```
engine:query > value
engine:query < value
engine:query == value
engine:query != value
```

## Query Examples

### Simple Label-Based Correlation

```sql
logs:error AND metrics:high_latency
```

Correlates error logs with high latency metrics that share common labels (service, pod, namespace, etc.).

### Time-Window Correlation

```sql
logs:exception WITHIN 5m OF metrics:cpu_usage > 80
```

Finds exceptions in logs that occurred within 5 minutes of CPU usage spikes above 80%.

### Complex Correlation

```sql
(logs:error OR logs:warn) WITHIN 10m OF traces:status:error
```

Correlates error or warning logs with error traces within a 10-minute window.

### Multi-Engine Correlation

```sql
logs:service:checkout AND traces:service:checkout AND metrics:http_requests > 1000
```

Correlates checkout service logs, traces, and high request metrics.

## Correlation Types

### Time-Window Correlation

Time-window correlation finds relationships between events that occur within a specified time range. This is useful for:

- Root cause analysis (e.g., errors following resource spikes)
- Performance monitoring (e.g., latency spikes with error bursts)
- Incident investigation (e.g., tracing request failures)

**Algorithm:**
1. Extract timestamps from all data points
2. Compare timestamps between different engines
3. Correlate events within the specified time window
4. Calculate confidence based on temporal proximity

### Label-Based Correlation

Label-based correlation matches events that share common metadata labels. This is useful for:

- Service-level correlation (same service name)
- Infrastructure correlation (same pod, namespace, host)
- Application correlation (same deployment, container)

**Algorithm:**
1. Extract labels from data points (service, pod, namespace, etc.)
2. Compare labels between different engines
3. Weight matches by label importance
4. Calculate confidence based on label match quality

## Result Merging and Deduplication

The correlation engine includes intelligent result merging to eliminate duplicate correlations and consolidate similar findings.

### Merging Logic

1. **Grouping**: Correlations are grouped by similarity criteria:
   - Same engines involved
   - Close timestamps (within 1 minute)
   - Similar confidence scores (within 20%)

2. **Merging**: Similar correlations are merged into a single result:
   - Timestamps are averaged
   - Confidence scores are averaged
   - Data from all engines is combined
   - Metadata indicates merge operation

3. **Deduplication**: Duplicate correlations are eliminated while preserving all relevant data.

## Confidence Scoring

Correlation confidence is calculated based on multiple factors:

### Time-Window Confidence

- **Base Range**: 0.5 - 0.9
- **Factors**: Temporal proximity within the time window
- **Formula**: `0.5 + (proximity_ratio × 0.4)`
- **Cap**: Maximum 0.95 to allow for other factors

### Label-Based Confidence

- **Base Range**: 0.6 - 0.95
- **Factors**: Label match quality and importance weights
- **Weights**:
  - `service`: 1.0 (highest importance)
  - `pod`: 0.9
  - `namespace`: 0.8
  - `deployment`: 0.8
  - `container`: 0.7
  - `operation`: 0.8
  - `host`: 0.6
  - `level`: 0.3 (lowest importance)

## API Usage

### Execute Correlation Query

```text
POST /api/v1/unified/correlation
Content-Type: application/json

{
  "query": "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
  "id": "correlation-123"
}
```

### Response Format

```json
{
  "result": {
    "correlations": [
      {
        "id": "correlation-123_time_window_1",
        "timestamp": "2025-10-30T10:15:30Z",
        "engines": {
          "logs": {"query": "error logs", "count": 15},
          "metrics": {"query": "cpu_usage > 80", "count": 8}
        },
        "confidence": 0.85,
        "metadata": {
          "time_window": "5m0s",
          "correlation_type": "time_window"
        }
      }
    ],
    "summary": {
      "total_correlations": 1,
      "average_confidence": 0.85,
      "time_range": "2.5s",
      "engines_involved": ["logs", "metrics"]
    }
  }
}
```

## Performance Considerations

### Parallel Execution

- Queries across different engines execute in parallel
- Reduces total query time for multi-engine correlations
- Timeout handling prevents hanging queries

### Caching

- Correlation results can be cached
- Cache keys include query parameters and time ranges
- TTL-based expiration with configurable durations

### Resource Management

- Result merging prevents memory bloat from duplicates
- Configurable limits on correlation result sizes
- Efficient algorithms for large datasets

## Error Handling

### Validation Errors

- Invalid query syntax
- Unsupported time windows
- Missing engine configurations

### Execution Errors

- Engine service unavailability
- Query timeouts
- Data parsing failures

### Recovery

- Graceful degradation when engines are unavailable
- Partial results with warnings
- Detailed error messages for debugging

## Configuration

### Engine Configuration

```yaml
correlation:
  enabled: true
  max_time_window: 1h
  default_cache_ttl: 5m
  max_cache_ttl: 1h
  parallel_execution: true
  max_results: 1000
```

### Query Limits

- Maximum time window: 1 hour
- Maximum result set size: 1000 correlations
- Query timeout: 30 seconds
- Cache TTL range: 1 minute to 1 hour

## Monitoring and Observability

### Metrics

- `correlation_queries_total` - Total correlation queries executed
- `correlation_execution_time` - Query execution time distribution
- `correlation_confidence_avg` - Average correlation confidence
- `correlation_engines_used` - Engine usage statistics

### Logging

- Query execution details
- Correlation result summaries
- Performance metrics
- Error conditions with context

## Best Practices

### Query Design

1. **Start Simple**: Begin with basic label correlations
2. **Add Time Windows**: Use time windows for temporal relationships
3. **Combine Conditions**: Add metric thresholds for precision
4. **Test Incrementally**: Validate correlations with known events

### Performance Optimization

1. **Use Appropriate Time Windows**: Smaller windows for precision, larger for coverage
2. **Leverage Caching**: Cache frequent correlation patterns
3. **Limit Result Sets**: Use pagination for large result sets
4. **Monitor Performance**: Track query execution times

### Troubleshooting

1. **Check Engine Health**: Ensure all required engines are available
2. **Validate Query Syntax**: Use the validation endpoint
3. **Review Confidence Scores**: Low confidence may indicate weak correlations
4. **Examine Timestamps**: Verify data timestamp accuracy

## Future Enhancements

### Advanced Features

- **Machine Learning Correlation**: AI-powered correlation discovery
- **Custom Correlation Rules**: User-defined correlation patterns
- **Real-time Correlation**: Streaming correlation for live data
- **Cross-cluster Correlation**: Correlation across multiple clusters

### Performance Improvements

- **Query Optimization**: Intelligent query planning and execution
- **Distributed Processing**: Scale correlation across multiple nodes
- **Advanced Caching**: Predictive caching based on usage patterns
- **Result Streaming**: Streaming results for large correlation sets</content>
<parameter name="filePath">/Users/aarvee/repos/github/public/mirador-core/docs/correlation-engine.md