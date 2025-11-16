# Creating Correlation Queries in Mirador Core

## Overview

Correlation queries allow you to find relationships between data from different observability sources (logs, metrics, and traces) in Mirador Core. This guide provides detailed instructions on how to construct effective correlation queries.

## Quick Start

### Basic Correlation Query

```text
logs:error AND metrics:high_latency
```

This query finds error logs that correlate with high latency metrics based on shared labels.

### Time-Window Correlation

```text
logs:exception WITHIN 5m OF metrics:cpu_usage > 80
```

This query finds exceptions that occurred within 5 minutes of CPU usage spikes above 80%.

## Query Structure

### Basic Syntax

```
[engine:]query [operator] [engine:]query [WITHIN time_window OF [engine:]query]
```

### Components

1. **Engine Prefix**: `logs:`, `metrics:`, or `traces:`
2. **Query**: The search term or expression
3. **Operator**: `AND`, `OR`, or `WITHIN ... OF`
4. **Time Window**: Duration for time-based correlation (optional)

## Engine-Specific Query Formats

### Logs Queries

Logs queries use text search patterns:

```
logs:error                    # Simple text search
logs:"connection timeout"     # Phrase search
logs:level:error              # Field-based search
logs:service:checkout         # Service-specific logs
```

**Supported Log Query Patterns:**
- Simple text: `logs:error`
- Field search: `logs:field:value`
- Multiple terms: `logs:error exception`
- Service context: `logs:service:checkout error`

### Metrics Queries

Metrics queries use PromQL-style expressions:

```
metrics:cpu_usage                    # Metric name
metrics:cpu_usage > 80              # With threshold
metrics:http_requests_total         # Counter metrics
metrics:response_time < 1000        # Performance metrics
```

**Supported Metric Conditions:**
- Greater than: `> 80`
- Less than: `< 1000`
- Equal to: `== 500`
- Not equal to: `!= 0`

### Traces Queries

Traces queries use service and operation patterns:

```
traces:service:checkout             # Service traces
traces:operation:payment           # Operation traces
traces:status:error                # Error traces
traces:service:api operation:auth  # Combined service and operation
```

## Correlation Operators

### AND Operator

Combines queries that must both be true:

```text
logs:error AND metrics:cpu_usage > 80
```

**Use Cases:**
- Find errors that coincide with resource issues
- Correlate application errors with infrastructure problems
- Identify patterns where multiple conditions occur together

### OR Operator

Combines queries where either can be true:

```text
(logs:error OR logs:warn) AND metrics:memory_usage > 90
```

**Use Cases:**
- Find any significant log events with resource issues
- Correlate multiple types of events with metrics
- Broad pattern matching across different severity levels

### WITHIN ... OF Operator

Correlates events within a time window:

```text
logs:exception WITHIN 5m OF metrics:cpu_usage > 80
```

**Use Cases:**
- Root cause analysis (what happened before/after an event)
- Performance impact analysis
- Incident timeline reconstruction

## Time Windows

### Supported Formats

- `30s` - 30 seconds
- `5m` - 5 minutes
- `1h` - 1 hour
- `2d` - 2 days

### Choosing Time Windows

**Small Windows (seconds to minutes):**
- For immediate cause-effect relationships
- API timeouts and database errors
- Microservice communication issues

```text
logs:timeout WITHIN 30s OF metrics:http_requests_total
```

**Medium Windows (minutes):**
- For application-level issues
- Resource exhaustion scenarios
- Batch processing problems

```text
logs:memory_error WITHIN 5m OF metrics:memory_usage > 90
```

**Large Windows (hours):**
- For trend analysis
- Long-running process issues
- Capacity planning insights

```text
logs:deployment WITHIN 1h OF metrics:error_rate > 5
```

## Advanced Query Patterns

### Multi-Engine Correlation

Correlate across all three data sources:

```text
logs:service:checkout AND traces:service:checkout AND metrics:http_requests > 1000
```

### Nested Expressions

Use parentheses for complex logic:

```text
(logs:error OR logs:exception) WITHIN 10m OF (metrics:cpu_usage > 80 OR metrics:memory_usage > 90)
```

### Service-Specific Correlation

Focus on specific services:

```text
logs:service:payment error AND traces:service:payment AND metrics:payment_errors_total > 0
```

### Infrastructure Correlation

Correlate application issues with infrastructure:

```text
logs:pod:web-123 crash WITHIN 2m OF metrics:node_cpu{node="k8s-node-01"} > 95
```

## Step-by-Step Query Construction

### Step 1: Identify the Problem

**Example Problem:** "Find database connection errors that occur when CPU usage is high"

### Step 2: Choose Primary Data Source

**Primary:** Logs (connection errors)
**Secondary:** Metrics (CPU usage)

### Step 3: Write Basic Queries

```text
logs:connection error
metrics:cpu_usage > 80
```

### Step 4: Determine Correlation Type

**Time-based correlation** - errors might occur when CPU is already high

```text
logs:"connection error" WITHIN 5m OF metrics:cpu_usage > 80
```

### Step 5: Add Context

**Add service context:**

```text
logs:service:api "connection error" WITHIN 5m OF metrics:cpu_usage{service="api"} > 80
```

### Step 6: Test and Refine

**Check results and adjust:**
- Time window too small/large?
- Conditions too strict/lenient?
- Missing important context?

## Common Query Patterns

### Performance Issues

```text
-- Slow requests with high CPU
logs:response_time > 5000 WITHIN 1m OF metrics:cpu_usage > 70

-- Memory issues with application errors
logs:OutOfMemoryError WITHIN 5m OF metrics:memory_usage > 85

-- Database timeouts with connection pool exhaustion
logs:"connection timeout" WITHIN 2m OF metrics:db_connections_active > 50
```

### Error Analysis

```text
-- Application errors with infrastructure issues
logs:level:error WITHIN 10m OF metrics:disk_usage > 90

-- 5xx errors with service degradation
logs:status:500 WITHIN 5m OF traces:operation:api_call

-- Exception bursts with resource spikes
(logs:exception OR logs:error) WITHIN 3m OF metrics:cpu_usage > 80
```

### Incident Investigation

```text
-- Deployment issues
logs:deployment WITHIN 30m OF metrics:error_rate > 10

-- Network problems
logs:"connection refused" WITHIN 1m OF metrics:network_errors_total > 0

-- Security events
logs:unauthorized WITHIN 5m OF traces:operation:login
```

### Business Logic Correlation

```text
-- Payment failures with checkout service
logs:service:checkout payment_failed WITHIN 2m OF traces:service:payment

-- Order processing issues
logs:service:orders error WITHIN 5m OF metrics:order_processing_time > 30000

-- User authentication problems
logs:login_failed WITHIN 1m OF metrics:auth_attempts_total > 100
```

## Best Practices

### Query Design

1. **Start Simple**: Begin with basic correlations and add complexity gradually
2. **Use Specific Terms**: Prefer specific error messages over generic terms
3. **Consider Time Windows**: Choose time windows based on your system's characteristics
4. **Add Context**: Include service, pod, or namespace information when possible

### Performance Considerations

1. **Limit Time Windows**: Smaller windows perform better and are more precise
2. **Use Specific Conditions**: Thresholds reduce result sets and improve relevance
3. **Avoid Over-Correlation**: Don't correlate everything - focus on meaningful relationships
4. **Test Query Performance**: Monitor execution times and adjust as needed

### Result Interpretation

1. **Check Confidence Scores**: Higher confidence indicates stronger correlations
2. **Validate Timestamps**: Ensure correlation timestamps make logical sense
3. **Look for Patterns**: Identify recurring correlation patterns
4. **Cross-Reference**: Validate correlations with known system behavior

## Troubleshooting

### No Results

**Possible Causes:**
- Time window too small
- Conditions too restrictive
- Data not available in specified time range
- Engine services not responding

**Solutions:**
```text
-- Expand time window
logs:error WITHIN 1h OF metrics:cpu_usage > 50

-- Relax conditions
logs:error WITHIN 5m OF metrics:cpu_usage > 60

-- Check data availability
logs:error AND metrics:cpu_usage
```

### Too Many Results

**Possible Causes:**
- Time window too large
- Conditions too broad
- Generic search terms

**Solutions:**
```text
-- Reduce time window
logs:error WITHIN 1m OF metrics:cpu_usage > 80

-- Add specificity
logs:service:api error WITHIN 5m OF metrics:cpu_usage{service="api"} > 80

-- Use more specific terms
logs:"connection refused" WITHIN 2m OF metrics:db_connections_active > 45
```

### Low Confidence Scores

**Possible Causes:**
- Weak correlations
- Inconsistent labeling
- Clock skew between systems

**Solutions:**
- Adjust time windows
- Check label consistency
- Use more specific queries
- Consider label-based correlation instead

## API Usage Examples

### Basic Correlation Query

```bash
curl -X POST http://localhost:8080/api/v1/unified/correlation \
  -H "Content-Type: application/json" \
  -d '{
    "query": "logs:error AND metrics:high_latency",
    "id": "basic-correlation"
  }'
```

### Time-Window Correlation

```bash
curl -X POST http://localhost:8080/api/v1/unified/correlation \
  -H "Content-Type: application/json" \
  -d '{
    "query": "logs:exception WITHIN 5m OF metrics:cpu_usage > 80",
    "id": "time-window-correlation"
  }'
```

### Complex Multi-Engine Query

```bash
curl -X POST http://localhost:8080/api/v1/unified/correlation \
  -H "Content-Type: application/json" \
  -d '{
    "query": "(logs:error OR logs:warn) WITHIN 10m OF traces:status:error AND metrics:error_rate > 5",
    "id": "complex-correlation"
  }'
```

## Response Format

```json
{
  "result": {
    "correlations": [
      {
        "id": "correlation-123_time_window_1",
        "timestamp": "2025-10-30T10:15:30Z",
        "engines": {
          "logs": {
            "timestamp": "2025-10-30T10:15:25Z",
            "message": "Connection timeout to database",
            "service": "api",
            "level": "error"
          },
          "metrics": {
            "name": "cpu_usage",
            "value": 85.5,
            "labels": {"service": "api"}
          }
        },
        "confidence": 0.87,
        "metadata": {
          "time_window": "5m0s",
          "correlation_type": "time_window",
          "time_diff": "5s"
        }
      }
    ],
    "summary": {
      "total_correlations": 1,
      "average_confidence": 0.87,
      "time_range": "2.3s",
      "engines_involved": ["logs", "metrics"]
    }
  }
}
```

## Next Steps

1. **Experiment**: Try the example queries with your data
2. **Customize**: Adapt queries for your specific services and metrics
3. **Monitor**: Track query performance and result quality
4. **Automate**: Build alerts and dashboards based on correlation patterns
5. **Extend**: Create custom correlation rules for your domain

## Support

For additional help:
- Check the [Correlation Engine Documentation](../docs/correlation-engine.md)
- Review [API Reference](../docs/api-reference.md)
- Contact the platform team for advanced use cases</content>
<parameter name="filePath">/Users/aarvee/repos/github/public/mirador-core/docs/correlation-queries-guide.md