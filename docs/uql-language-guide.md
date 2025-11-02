# Unified Query Language (UQL) Guide

## Overview

The Unified Query Language (UQL) is Mirador Core's powerful query language for unified observability data analysis. UQL provides a consistent syntax for querying across metrics, logs, traces, and correlations, enabling complex analytical workflows and cross-domain insights.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Query Types](#query-types)
3. [SELECT Queries](#select-queries)
4. [Aggregation Queries](#aggregation-queries)
5. [Correlation Queries](#correlation-queries)
6. [JOIN Queries](#join-queries)
7. [Data Sources](#data-sources)
8. [Operators and Functions](#operators-and-functions)
9. [Optimization Features](#optimization-features)
10. [Best Practices](#best-practices)
11. [Examples](#examples)

## Quick Start

### Basic SELECT Query

```sql
SELECT service, level, message
FROM logs:error
WHERE level = 'error' AND service = 'api'
LIMIT 100
```

### Aggregation Query

```sql
SELECT service, count(*) as error_count
FROM logs:error
WHERE timestamp > now() - 1h
GROUP BY service
ORDER BY error_count DESC
```

### Correlation Query

```sql
logs:error WITHIN 5m OF metrics:cpu_usage > 80
```

## Query Types

UQL supports four main query types:

### 1. SELECT Queries
Query specific fields from a single data source with filtering and projection.

### 2. AGGREGATION Queries
Perform grouping and aggregation operations across data sources.

### 3. CORRELATION Queries
Find relationships between events across different data sources.

### 4. JOIN Queries
Combine data from multiple sources based on common fields or time windows.

## SELECT Queries

### Basic Syntax

```sql
SELECT [DISTINCT] field1, field2, ...
FROM datasource[:filter]
[WHERE conditions]
[ORDER BY field [ASC|DESC], ...]
[LIMIT n]
[OFFSET n]
```

### Field Selection

```sql
-- Select specific fields
SELECT service, level, message FROM logs

-- Select all fields
SELECT * FROM logs

-- Select distinct values
SELECT DISTINCT service FROM logs:error
```

### FROM Clause

The FROM clause specifies the data source and optional filtering:

```sql
-- Basic data source
FROM logs
FROM metrics
FROM traces

-- With filtering
FROM logs:error
FROM metrics:cpu_usage
FROM traces:service:checkout
```

### WHERE Clause

Filter data using various conditions:

```sql
-- Simple equality
WHERE level = 'error'

-- Numeric comparisons
WHERE response_time > 1000
WHERE cpu_usage < 80

-- String matching
WHERE message CONTAINS 'timeout'
WHERE service LIKE 'api%'

-- Time-based conditions
WHERE timestamp > now() - 1h
WHERE timestamp BETWEEN '2025-01-01' AND '2025-01-02'

-- Multiple conditions
WHERE level = 'error' AND service = 'api' AND response_time > 5000
```

### ORDER BY Clause

Sort results by one or more fields:

```sql
ORDER BY timestamp DESC
ORDER BY error_count DESC, service ASC
```

### LIMIT and OFFSET

Control result pagination:

```sql
LIMIT 100
LIMIT 50 OFFSET 100
```

## Aggregation Queries

### Basic Syntax

```sql
SELECT field1, aggregate_function(field2) [AS alias], ...
FROM datasource
[WHERE conditions]
GROUP BY field1, field2, ...
[HAVING conditions]
[ORDER BY field [ASC|DESC], ...]
[LIMIT n]
```

### Aggregate Functions

- `count(*)` - Count rows
- `count(field)` - Count non-null values
- `sum(field)` - Sum numeric values
- `avg(field)` - Average value
- `min(field)` - Minimum value
- `max(field)` - Maximum value
- `distinct_count(field)` - Count distinct values

### Examples

```sql
-- Count errors by service
SELECT service, count(*) as error_count
FROM logs:error
GROUP BY service

-- Average response time by endpoint
SELECT endpoint, avg(response_time) as avg_response
FROM metrics:http_requests
WHERE timestamp > now() - 1h
GROUP BY endpoint

-- Top 5 services by error rate
SELECT service, count(*) as errors
FROM logs:error
WHERE timestamp > now() - 24h
GROUP BY service
ORDER BY errors DESC
LIMIT 5
```

### HAVING Clause

Filter aggregated results:

```sql
SELECT service, count(*) as error_count
FROM logs:error
GROUP BY service
HAVING error_count > 10
```

## Correlation Queries

### Basic Syntax

```sql
query1 [WITHIN time_window OF] query2
query1 [AND|OR] query2
```

### Time-Window Correlations

Find events within a time window:

```sql
-- Errors within 5 minutes of high CPU
logs:error WITHIN 5m OF metrics:cpu_usage > 80

-- Exceptions before memory spikes
logs:exception WITHIN 10m OF metrics:memory_usage > 90
```

### Label-Based Correlations

Correlate events with matching labels:

```sql
-- Same service errors and metrics
logs:service:api AND metrics:service:api

-- Multiple conditions
logs:service:checkout error AND traces:service:checkout
```

### Complex Correlations

```sql
-- Multiple time windows
(logs:error OR logs:warn) WITHIN 5m OF metrics:cpu_usage > 80

-- Cross-domain correlation
logs:service:payment timeout WITHIN 2m OF traces:service:payment AND metrics:payment_errors > 0
```

## JOIN Queries

### Basic Syntax

```sql
SELECT fields
FROM datasource1
JOIN datasource2 ON condition
[WHERE conditions]
```

### JOIN Types

- `INNER JOIN` - Only matching rows
- `LEFT JOIN` - All rows from left, matching from right
- `RIGHT JOIN` - All rows from right, matching from left
- `FULL JOIN` - All rows from both sources

### Time-Based JOINs

```sql
-- Join logs and metrics by time window
SELECT l.message, m.cpu_usage
FROM logs:error l
JOIN metrics:cpu_usage m ON l.timestamp WITHIN 1m OF m.timestamp
WHERE l.service = m.service
```

### Label-Based JOINs

```sql
-- Join by service label
SELECT l.message, t.operation, m.value
FROM logs l
JOIN traces t ON l.service = t.service
JOIN metrics m ON l.service = m.service
WHERE l.level = 'error'
```

## Data Sources

### Logs

Query structured and unstructured log data:

```sql
FROM logs                          -- All logs
FROM logs:error                    -- Logs containing 'error'
FROM logs:service:api              -- Logs from 'api' service
FROM logs:level:error              -- Error level logs
FROM logs:pod:web-123              -- Logs from specific pod
```

### Metrics

Query time-series metrics data:

```sql
FROM metrics                       -- All metrics
FROM metrics:cpu_usage             -- CPU usage metrics
FROM metrics:http_requests_total   -- HTTP request counters
FROM metrics:response_time         -- Response time metrics
```

### Traces

Query distributed tracing data:

```sql
FROM traces                        -- All traces
FROM traces:service:checkout       -- Checkout service traces
FROM traces:operation:payment      -- Payment operation traces
FROM traces:status:error           -- Error traces
```

### Correlations

Query pre-computed correlations:

```sql
FROM correlations                  -- All correlations
FROM correlations:service:api      -- API service correlations
FROM correlations:error            -- Error correlations
```

## Operators and Functions

### Comparison Operators

- `=` - Equal to
- `!=` - Not equal to
- `<` - Less than
- `<=` - Less than or equal
- `>` - Greater than
- `>=` - Greater than or equal
- `LIKE` - Pattern matching (SQL-style)
- `CONTAINS` - Substring search
- `IN` - Value in list

### Logical Operators

- `AND` - Logical AND
- `OR` - Logical OR
- `NOT` - Logical NOT

### Time Functions

- `now()` - Current timestamp
- `timestamp(field)` - Extract timestamp from field
- `duration(string)` - Parse duration string
- `date_trunc(interval, timestamp)` - Truncate timestamp

### String Functions

- `upper(string)` - Convert to uppercase
- `lower(string)` - Convert to lowercase
- `length(string)` - String length
- `substring(string, start, length)` - Extract substring
- `concat(string1, string2, ...)` - Concatenate strings

### Mathematical Functions

- `abs(number)` - Absolute value
- `round(number, decimals)` - Round to decimals
- `ceil(number)` - Ceiling
- `floor(number)` - Floor

## Optimization Features

UQL includes advanced optimization features that automatically improve query performance:

### Query Rewriting

Automatically rewrites queries for better execution:

```sql
-- Original query
SELECT * FROM logs WHERE level = 'error' AND timestamp > now() - 1h

-- Optimized execution
SELECT level, message, service FROM logs:error WHERE timestamp > now() - 1h
```

### Predicate Pushdown

Pushes filters down to data sources for early filtering:

```sql
-- Filter applied at data source level
SELECT * FROM logs WHERE service = 'api' AND level = 'error'
-- Becomes: Query logs:error with service filter
```

### Time Window Optimization

Optimizes time-based queries for efficient execution:

```sql
-- Time window queries optimized for data source capabilities
logs:error WITHIN 5m OF metrics:cpu_usage > 80
```

### Index Selection

Automatically selects optimal indexes for query execution:

```sql
-- Uses timestamp index for time-based queries
SELECT * FROM logs WHERE timestamp > now() - 1h

-- Uses field indexes for selective queries
SELECT * FROM logs WHERE service = 'api' AND level = 'error'
```

### Cost-Based Optimization

Chooses optimal execution plans based on data characteristics:

```sql
-- Selects join algorithm based on data sizes
SELECT l.message, m.value
FROM logs l
JOIN metrics m ON l.service = m.service
```

### Query Plan Caching

Caches optimized query plans for repeated executions:

```sql
-- First execution: plan optimization
SELECT service, count(*) FROM logs:error GROUP BY service

-- Subsequent executions: cached plan reuse
SELECT service, count(*) FROM logs:error GROUP BY service
```

### Join Optimization

Optimizes join order and algorithms:

```sql
-- Join order optimized based on selectivity
SELECT l.message, t.operation, m.value
FROM logs l
JOIN traces t ON l.trace_id = t.id
JOIN metrics m ON l.service = m.service
```

### Subquery Optimization

Converts subqueries to efficient joins:

```sql
-- Subquery converted to join
SELECT * FROM logs WHERE service IN (SELECT service FROM metrics WHERE value > 80)
```

### Materialized View Checks

Automatically uses materialized views when available:

```sql
-- Uses pre-computed materialized view
SELECT service, count(*) FROM logs:error WHERE timestamp > now() - 24h GROUP BY service
```

## Best Practices

### Query Design

1. **Be Specific**: Use targeted filters to reduce data volume
2. **Choose Appropriate Time Windows**: Balance precision with performance
3. **Use Indexes**: Design queries to leverage available indexes
4. **Limit Results**: Always use LIMIT for exploratory queries

### Performance Optimization

1. **Filter Early**: Apply restrictive filters in WHERE clauses
2. **Use Aggregations Wisely**: Aggregate before joining when possible
3. **Avoid Cartesian Products**: Ensure JOINs have appropriate conditions
4. **Monitor Query Performance**: Use EXPLAIN to understand execution plans

### Data Source Selection

1. **Choose Right Data Source**: Select appropriate engine for your data type
2. **Understand Data Distribution**: Know how your data is partitioned
3. **Consider Data Freshness**: Account for ingestion delays
4. **Use Appropriate Granularity**: Match query granularity to data resolution

## Examples

### Error Analysis

```sql
-- Recent errors by service
SELECT service, level, count(*) as count
FROM logs
WHERE level IN ('error', 'fatal') AND timestamp > now() - 1h
GROUP BY service, level
ORDER BY count DESC

-- Errors correlated with performance issues
logs:error WITHIN 5m OF metrics:response_time > 5000
```

### Performance Monitoring

```sql
-- Response time percentiles by endpoint
SELECT endpoint,
       percentile(response_time, 95) as p95,
       percentile(response_time, 99) as p99
FROM metrics:http_requests
WHERE timestamp > now() - 1h
GROUP BY endpoint

-- CPU spikes with application errors
metrics:cpu_usage > 80 WITHIN 2m OF logs:level:error
```

### Service Health Dashboard

```sql
-- Service error rates
SELECT service,
       count(*) as total_requests,
       sum(case when status >= 500 then 1 else 0 end) as errors,
       (errors * 100.0 / total_requests) as error_rate
FROM metrics:http_requests
WHERE timestamp > now() - 24h
GROUP BY service
HAVING total_requests > 100
ORDER BY error_rate DESC
```

### Incident Investigation

```sql
-- Timeline of events around incident
SELECT timestamp, level, message, service
FROM logs
WHERE timestamp BETWEEN '2025-01-01 10:00:00' AND '2025-01-01 11:00:00'
  AND (level = 'error' OR message CONTAINS 'timeout')
ORDER BY timestamp

-- Correlated events across domains
logs:service:api error WITHIN 10m OF traces:service:api AND metrics:api_errors > 0
```

### Capacity Planning

```sql
-- Peak usage patterns
SELECT date_trunc('hour', timestamp) as hour,
       max(cpu_usage) as peak_cpu,
       avg(memory_usage) as avg_memory
FROM metrics:system
WHERE timestamp > now() - 7d
GROUP BY hour
ORDER BY hour

-- Resource usage trends
SELECT service,
       avg(cpu_usage) as avg_cpu,
       max(cpu_usage) as max_cpu,
       percentile(cpu_usage, 95) as p95_cpu
FROM metrics:cpu_usage
WHERE timestamp > now() - 30d
GROUP BY service
ORDER BY avg_cpu DESC
```

## API Usage

### Execute UQL Query

```bash
curl -X POST http://localhost:8080/api/v1/uql/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT service, count(*) FROM logs:error GROUP BY service",
    "id": "error-analysis"
  }'
```

### Execute with Parameters

```bash
curl -X POST http://localhost:8080/api/v1/uql/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT * FROM logs WHERE service = $service AND timestamp > $start_time",
    "parameters": {
      "service": "api",
      "start_time": "2025-01-01T00:00:00Z"
    },
    "id": "parameterized-query"
  }'
```

## Response Format

```json
{
  "query_id": "error-analysis",
  "status": "success",
  "data": {
    "columns": ["service", "count"],
    "rows": [
      ["api", 1250],
      ["checkout", 890],
      ["payment", 567]
    ],
    "metadata": {
      "row_count": 3,
      "execution_time_ms": 45,
      "data_source": "logs",
      "optimized": true
    }
  },
  "execution_time_ms": 45,
  "cached": false
}
```

## Troubleshooting

### Common Issues

1. **No Results**
   - Check data source availability
   - Verify time ranges
   - Ensure correct field names

2. **Slow Queries**
   - Add more restrictive filters
   - Use appropriate indexes
   - Consider aggregation before joining

3. **Syntax Errors**
   - Validate query syntax
   - Check field and function names
   - Ensure proper quoting

4. **Optimization Issues**
   - Review query structure
   - Check data distribution
   - Consider query rewriting

## Advanced Features

### Custom Functions

UQL supports custom functions for domain-specific operations:

```sql
SELECT service, custom_error_score(message) as score
FROM logs:error
WHERE score > 0.8
```

### Window Functions

Advanced analytical functions over data windows:

```sql
SELECT service, timestamp, error_count,
       avg(error_count) OVER (PARTITION BY service ORDER BY timestamp ROWS 10 PRECEDING) as rolling_avg
FROM (
  SELECT service, date_trunc('hour', timestamp) as timestamp, count(*) as error_count
  FROM logs:error
  GROUP BY service, timestamp
)
```

### Subqueries

Complex queries with nested subqueries:

```sql
SELECT service, error_count
FROM (
  SELECT service, count(*) as error_count
  FROM logs:error
  WHERE timestamp > now() - 1h
  GROUP BY service
) t
WHERE error_count > (
  SELECT avg(error_count) FROM (
    SELECT service, count(*) as error_count
    FROM logs:error
    WHERE timestamp > now() - 24h
    GROUP BY service
  )
)
```

## Conclusion

UQL provides a powerful, unified interface for observability data analysis. By understanding the query types, optimization features, and best practices outlined in this guide, you can construct efficient queries that provide deep insights across your entire observability stack.

For more advanced use cases or specific domain requirements, consult the [API Reference](../api-reference.md) and [Correlation Engine Documentation](../correlation-engine.md).</content>
<parameter name="filePath">/Users/aarvee/repos/github/public/mirador-core/docs/uql-language-guide.md