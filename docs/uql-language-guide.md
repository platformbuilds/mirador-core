# Unified Query Language (UQL) Guide

## Overview

The Unified Query Language (UQL) is Mirador Core's sophisticated query language for unified observability data analysis. UQL provides a consistent syntax for querying across metrics, logs, traces, and correlations, enabling complex analytical workflows and cross-domain insights through a powerful compilation pipeline.

## Table of Contents

1. [Quick Start](#quick-start)
2. [UQL Compilation Pipeline](#uql-compilation-pipeline)
3. [Query Types](#query-types)
4. [SELECT Queries](#select-queries)
5. [Aggregation Queries](#aggregation-queries)
6. [Correlation Queries](#correlation-queries)
7. [JOIN Queries](#join-queries)
8. [Data Sources](#data-sources)
9. [Operators and Functions](#operators-and-functions)
10. [Advanced Correlation Operators](#advanced-correlation-operators)
11. [Optimization Features](#optimization-features)
12. [Best Practices](#best-practices)
13. [Examples](#examples)

## UQL Compilation Pipeline

UQL queries undergo a sophisticated multi-stage compilation process that optimizes and translates queries for efficient execution across multiple data engines.

### Pipeline Stages

1. **Parsing**: UQL syntax is parsed into an Abstract Syntax Tree (AST)
2. **Semantic Analysis**: Query validation and type checking
3. **Optimization**: Multi-pass query optimization with cost-based planning
4. **Translation**: Engine-specific query translation via translator registry
5. **Execution Planning**: Parallel execution planning with intelligent routing
6. **Execution**: Parallel query execution with result correlation

### Parser Features

- Full SQL-like syntax support with extensions for observability
- Advanced correlation operators (WITHIN, NEAR, BEFORE, AFTER)
- Multi-engine data source specifications
- Parameterized queries with type safety

### Optimizer Features

The UQL optimizer employs multiple optimization passes:

- **Predicate Pushdown**: Filters pushed to data sources for early evaluation
- **Join Reordering**: Optimal join order based on selectivity and cost
- **Index Selection**: Automatic selection of optimal indexes
- **Query Rewriting**: Automatic query restructuring for better performance
- **Materialized View Matching**: Automatic pre-computed view usage
- **Cost-Based Planning**: Execution plan selection based on data statistics

### Translator Registry

UQL supports translation to multiple query languages:

- **VictoriaMetrics**: Metrics queries translated to MetricsQL
- **VictoriaLogs**: Log queries translated to LogsQL
- **VictoriaTraces**: Trace queries translated to trace-specific syntax
- **Bleve/Elasticsearch**: Full-text search queries
- **Custom Engines**: Extensible translator interface for new engines

## Quick Start

### Basic SELECT Query

```text
SELECT service, level, message
FROM logs:error
WHERE level = 'error' AND service = 'api'
LIMIT 100
```

### Aggregation Query

```text
SELECT service, count(*) as error_count
FROM logs:error
WHERE timestamp > now() - 1h
GROUP BY service
ORDER BY error_count DESC
```

### Correlation Query

```text
logs:error WITHIN 5m OF metrics:cpu_usage > 80
```

### Advanced Correlation with Multiple Engines

```text
logs:service:api:error NEAR 10m OF traces:service:api AND metrics:api_errors > 5
```

## Query Types

UQL supports four main query types with advanced features:

### 1. SELECT Queries
Query specific fields from single or multiple data sources with advanced filtering, projection, and transformation capabilities.

### 2. AGGREGATION Queries
Perform complex grouping and aggregation operations with custom functions and windowing.

### 3. CORRELATION Queries
Find sophisticated relationships between events across different data sources using temporal and label-based correlations.

### 4. JOIN Queries
Combine data from multiple sources with time-based and label-based joins, supporting complex analytical workflows.

## SELECT Queries

### Enhanced Syntax

```text
SELECT [DISTINCT] [TOP n] field1, field2, ... [AS alias]
FROM datasource[:filter][:subfilter]
[WHERE conditions]
[GROUP BY field1, field2, ...]
[HAVING conditions]
[ORDER BY field [ASC|DESC], ...]
[LIMIT n [OFFSET m]]
[WITH (option = value, ...)]
```

### Advanced Field Selection

```text
-- Select specific fields with aliases
SELECT service as svc, level, message
FROM logs

-- Select all fields
SELECT * FROM logs

-- Select distinct values
SELECT DISTINCT service FROM logs:error

-- Top N results
SELECT TOP 10 service, count(*) as errors
FROM logs:error
GROUP BY service
ORDER BY errors DESC
```

### FROM Clause with Multi-Level Filtering

The FROM clause supports hierarchical filtering for precise data targeting:

```text
-- Basic data source
FROM logs
FROM metrics
FROM traces

-- Single level filtering
FROM logs:error
FROM metrics:cpu_usage
FROM traces:service:checkout

-- Multi-level filtering
FROM logs:service:api:error
FROM metrics:namespace:kube-system:cpu_usage
FROM traces:service:payment:operation:charge
```

### Advanced WHERE Clause

UQL supports complex filtering with multiple operators and functions:

```text
-- Comparison operators
WHERE level = 'error'
WHERE response_time > 1000
WHERE cpu_usage BETWEEN 50 AND 90

-- String operations
WHERE message CONTAINS 'timeout'
WHERE service LIKE 'api%'
WHERE service REGEXP '^api-[0-9]+$'

-- Time-based conditions
WHERE timestamp > now() - 1h
WHERE timestamp BETWEEN '2025-01-01' AND '2025-01-02'
WHERE timestamp > date('2025-01-01')

-- Array and JSON operations
WHERE tags CONTAINS 'production'
WHERE metadata['version'] = '1.2.3'

-- Multiple conditions with precedence
WHERE (level = 'error' OR level = 'fatal') AND service = 'api' AND response_time > 5000
```

### Query Options (WITH Clause)

Control query execution behavior:

```text
-- Timeout control
SELECT * FROM logs:error WITH (timeout = '30s')

-- Result formatting
SELECT * FROM metrics WITH (format = 'timeseries', step = '1m')

-- Execution hints
SELECT * FROM traces WITH (engine = 'jaeger', max_spans = 1000)
```

## Aggregation Queries

### Enhanced Syntax

```text
SELECT field1, aggregate_function(field2) [AS alias], ...
FROM datasource
[WHERE conditions]
GROUP BY field1, field2, ... [WITH ROLLUP|CUBE]
[HAVING conditions]
[ORDER BY field [ASC|DESC], ...]
[LIMIT n]
[WITH (option = value, ...)]
```

### Advanced Aggregate Functions

- `count(*)` - Count all rows
- `count(field)` - Count non-null values
- `count_distinct(field)` - Count distinct values
- `sum(field)` - Sum numeric values
- `avg(field)` - Average value
- `min(field)` - Minimum value
- `max(field)` - Maximum value
- `stddev(field)` - Standard deviation
- `variance(field)` - Variance
- `percentile(field, p)` - Percentile value
- `histogram(field, buckets)` - Histogram aggregation

### Advanced Grouping

```text
-- ROLLUP for subtotals
SELECT service, level, count(*)
FROM logs
GROUP BY service, level WITH ROLLUP

-- CUBE for all combinations
SELECT service, level, count(*)
FROM logs
GROUP BY service, level WITH CUBE

-- Time-based grouping
SELECT date_trunc('hour', timestamp) as hour, service, count(*)
FROM logs:error
GROUP BY hour, service
```

### Window Functions

```text
-- Moving averages
SELECT service, timestamp, error_count,
       avg(error_count) OVER (PARTITION BY service ORDER BY timestamp ROWS 5 PRECEDING) as moving_avg
FROM (
  SELECT service, date_trunc('hour', timestamp) as timestamp, count(*) as error_count
  FROM logs:error
  GROUP BY service, timestamp
)

-- Ranking functions
SELECT service, error_count,
       rank() OVER (ORDER BY error_count DESC) as rank,
       percent_rank() OVER (ORDER BY error_count DESC) as percent_rank
FROM (
  SELECT service, count(*) as error_count
  FROM logs:error
  GROUP BY service
)
```

## Correlation Queries

### Advanced Correlation Syntax

```text
query1 correlation_operator [time_window] OF query2
query1 [AND|OR|NOT] query2
(query1) [AND|OR|NOT] (query2)
```

### Temporal Correlation Operators

#### WITHIN Operator
Find events within a flexible time window:

```text
-- Errors within 5 minutes of high CPU (either direction)
logs:error WITHIN 5m OF metrics:cpu_usage > 80

-- Multiple correlations
logs:exception WITHIN 10m OF (metrics:memory_usage > 90 OR metrics:disk_usage > 95)
```

#### NEAR Operator
Events occurring near each other regardless of order:

```text
-- Errors near CPU spikes
logs:error NEAR 5m OF metrics:cpu_usage > 80

-- Multi-domain correlation
logs:service:api:error NEAR 2m OF traces:service:api AND metrics:api_errors > 0
```

#### BEFORE Operator
Events that occur before other events:

```text
-- Errors before CPU spikes
logs:error BEFORE 5m OF metrics:cpu_usage > 80

-- Warning sequence
logs:warn BEFORE 2m OF logs:error
```

#### AFTER Operator
Events that occur after other events:

```text
-- Recovery after errors
logs:recovery AFTER 10m OF logs:error

-- Follow-up events
metrics:cpu_usage < 50 AFTER 5m OF logs:restart
```

### Label-Based Correlations

Correlate events with matching metadata:

```text
-- Same service across domains
logs:service:api AND metrics:service:api

-- Multiple label matches
logs:service:checkout:level:error AND traces:service:checkout AND metrics:service:checkout

-- Complex label conditions
logs:namespace:production:service:api AND metrics:namespace:production:service:api
```

### Advanced Correlation Patterns

```text
-- Sequential events
logs:start BEFORE 1m OF logs:process BEFORE 1m OF logs:complete

-- Conditional correlations
logs:error WITHIN 5m OF (metrics:cpu_usage > 80 AND metrics:memory_usage > 85)

-- Exclusion correlations
logs:error WITHIN 10m OF metrics:cpu_usage > 80 AND NOT logs:maintenance

-- Time-windowed sequences
(logs:warn BEFORE 2m OF logs:error) WITHIN 10m OF metrics:anomaly > 0.8
```

## JOIN Queries

### Enhanced JOIN Syntax

```text
SELECT fields
FROM datasource1 [alias1]
join_type JOIN datasource2 [alias2] ON condition
[WHERE conditions]
[WITH (join_algorithm = 'hash'|'merge'|'nested_loop')]
```

### JOIN Types

- `INNER JOIN` - Only matching rows
- `LEFT JOIN` - All rows from left, matching from right
- `RIGHT JOIN` - All rows from right, matching from left
- `FULL OUTER JOIN` - All rows from both sources
- `CROSS JOIN` - Cartesian product (use with caution)

### Time-Based JOINs

```text
-- Join within time windows
SELECT l.message, m.cpu_usage, l.timestamp as log_time, m.timestamp as metric_time
FROM logs:error l
INNER JOIN metrics:cpu_usage m ON l.service = m.service
  AND l.timestamp WITHIN 1m OF m.timestamp
WHERE l.timestamp > now() - 1h

-- Asymmetric time windows
SELECT l.message, t.operation
FROM logs:error l
LEFT JOIN traces t ON l.trace_id = t.id
  AND t.timestamp BETWEEN l.timestamp - 5m AND l.timestamp + 1m
```

### Label-Based JOINs

```text
-- Join by service labels
SELECT l.message, m.value, t.operation
FROM logs l
INNER JOIN metrics m ON l.service = m.service AND l.namespace = m.namespace
LEFT JOIN traces t ON l.trace_id = t.id
WHERE l.level = 'error'

-- Complex label matching
SELECT l.message, m.value
FROM logs l
JOIN metrics m ON l.labels['deployment'] = m.labels['deployment']
  AND l.labels['version'] = m.labels['version']
```

### Advanced JOIN Features

```text
-- JOIN with hints
SELECT l.message, m.cpu_usage
FROM logs:error l
INNER JOIN metrics:cpu_usage m ON l.service = m.service
WITH (join_algorithm = 'hash', max_memory = '1GB')

-- Multi-way joins
SELECT l.message, m.cpu_usage, t.duration
FROM logs:error l
JOIN metrics:cpu_usage m ON l.service = m.service AND l.timestamp WITHIN 5m OF m.timestamp
JOIN traces t ON l.trace_id = t.id
WHERE m.cpu_usage > 80
```

## Data Sources

### Logs Data Source

Advanced log querying with full-text search and structured filtering:

```text
FROM logs                          -- All logs
FROM logs:error                    -- Logs containing 'error'
FROM logs:service:api              -- Logs from 'api' service
FROM logs:level:error              -- Error level logs
FROM logs:pod:web-123              -- Logs from specific pod
FROM logs:namespace:production     -- Logs from production namespace
FROM logs:container:nginx          -- Logs from nginx containers
```

### Metrics Data Source

Time-series metrics with advanced aggregation:

```text
FROM metrics                       -- All metrics
FROM metrics:cpu_usage             -- CPU usage metrics
FROM metrics:http_requests_total   -- HTTP request counters
FROM metrics:response_time         -- Response time metrics
FROM metrics:memory_usage          -- Memory usage metrics
FROM metrics:disk_usage            -- Disk usage metrics
```

### Traces Data Source

Distributed tracing with service and operation filtering:

```text
FROM traces                        -- All traces
FROM traces:service:checkout       -- Checkout service traces
FROM traces:operation:payment      -- Payment operation traces
FROM traces:status:error           -- Error traces
FROM traces:duration:>1000         -- Traces longer than 1000ms
FROM traces:tags:error=true        -- Traces with error tags
```

### Correlations Data Source

Pre-computed correlation data:

```text
FROM correlations                  -- All correlations
FROM correlations:service:api      -- API service correlations
FROM correlations:error            -- Error correlations
FROM correlations:performance      -- Performance correlations
FROM correlations:anomaly          -- Anomaly correlations
```

## Operators and Functions

### Comparison Operators

- `=` - Equal to
- `!=` - Not equal to
- `<` - Less than
- `<=` - Less than or equal
- `>` - Greater than
- `>=` - Greater than or equal
- `LIKE` - Pattern matching (SQL-style wildcards)
- `REGEXP` - Regular expression matching
- `CONTAINS` - Substring search
- `IN` - Value in list
- `BETWEEN` - Range check
- `IS NULL` - Null check
- `IS NOT NULL` - Not null check

### Logical Operators

- `AND` - Logical AND
- `OR` - Logical OR
- `NOT` - Logical NOT
- `XOR` - Logical XOR

### Correlation Operators

- `WITHIN time_window OF` - Events within time window (flexible order)
- `NEAR time_window OF` - Events near each other (any order)
- `BEFORE time_window OF` - Events before other events
- `AFTER time_window OF` - Events after other events

### Mathematical Functions

- `abs(n)` - Absolute value
- `round(n, decimals)` - Round to decimal places
- `ceil(n)` - Ceiling
- `floor(n)` - Floor
- `sqrt(n)` - Square root
- `pow(base, exp)` - Power function
- `log(n)` - Natural logarithm
- `log10(n)` - Base-10 logarithm

### String Functions

- `upper(s)` - Convert to uppercase
- `lower(s)` - Convert to lowercase
- `length(s)` - String length
- `substring(s, start, length)` - Extract substring
- `concat(s1, s2, ...)` - Concatenate strings
- `replace(s, old, new)` - Replace substrings
- `trim(s)` - Remove whitespace
- `split(s, delimiter)` - Split string into array

### Time Functions

- `now()` - Current timestamp
- `timestamp(field)` - Extract timestamp from field
- `date(s)` - Parse date string
- `date_trunc(interval, ts)` - Truncate timestamp
- `date_add(ts, interval)` - Add time interval
- `date_diff(unit, ts1, ts2)` - Time difference
- `duration(s)` - Parse duration string
- `format_duration(d)` - Format duration

### Array/JSON Functions

- `array_length(arr)` - Array length
- `array_contains(arr, val)` - Check array membership
- `json_extract(json, path)` - Extract JSON value
- `json_extract_array(json, path)` - Extract JSON array
- `json_keys(json)` - Get JSON object keys

### Statistical Functions

- `percentile(field, p)` - Calculate percentile
- `stddev(field)` - Standard deviation
- `variance(field)` - Variance
- `covariance(x, y)` - Covariance between fields
- `correlation(x, y)` - Correlation coefficient

## Advanced Correlation Operators

### Multi-Engine Correlations

Correlations can span multiple data engines with automatic routing:

```text
-- Cross-engine correlation
logs:service:api:error NEAR 5m OF metrics:service:api:cpu_usage > 80 AND traces:service:api:status:error

-- Engine-specific correlations
logs:engine:elasticsearch NEAR 10m OF metrics:engine:victoriametrics AND traces:engine:jaeger
```

### Temporal Sequence Patterns

Define complex event sequences:

```text
-- Error escalation pattern
logs:level:warn BEFORE 5m OF logs:level:error BEFORE 10m OF logs:level:fatal

-- Recovery patterns
logs:error BEFORE 30m OF logs:recovery AFTER 5m OF metrics:restart

-- Cyclic patterns
(metrics:cpu_usage > 80 BEFORE 10m OF metrics:cpu_usage < 20) REPEATING EVERY 1h
```

### Conditional Correlations

Correlations with conditional logic:

```text
-- Conditional based on service load
logs:error WITHIN 5m OF (metrics:cpu_usage > 80 AND metrics:request_rate > 1000)

-- Environment-specific correlations
logs:env:production:error NEAR 2m OF metrics:env:production:memory_usage > 90

-- Multi-condition correlations
logs:timeout WITHIN 1m OF (metrics:response_time > 5000 OR traces:duration > 10000)
```

### Correlation with Context

Include contextual information in correlations:

```text
-- Correlation with user context
logs:user:123:error WITHIN 10m OF traces:user:123:operation:checkout

-- Session-based correlations
logs:session:abc123 NEAR 30m OF metrics:session:abc123:cart_value > 1000

-- Request ID correlations
logs:request_id:xyz WITHIN 5m OF traces:request_id:xyz AND metrics:request_id:xyz
```

## Optimization Features

### Multi-Pass Optimization

UQL employs a sophisticated multi-pass optimization strategy:

1. **Syntactic Optimization**: Query rewriting for canonical forms
2. **Predicate Optimization**: Filter pushdown and reordering
3. **Join Optimization**: Join order selection and algorithm choice
4. **Index Optimization**: Automatic index selection and usage
5. **Cost-Based Optimization**: Plan selection based on data statistics
6. **Materialized View Optimization**: Automatic pre-computed view usage

### Intelligent Query Routing

Queries are automatically routed to optimal data engines:

```text
-- Automatic routing to VictoriaLogs
SELECT * FROM logs WHERE message CONTAINS 'error'

-- Automatic routing to VictoriaMetrics
SELECT avg(cpu_usage) FROM metrics WHERE service = 'api'

-- Cross-engine correlation with intelligent routing
logs:error NEAR 5m OF metrics:cpu_usage > 80
```

### Parallel Execution

Complex queries are executed in parallel across engines:

```text
-- Parallel execution of correlation
logs:service:api NEAR 10m OF metrics:service:api AND traces:service:api

-- Parallel aggregation across shards
SELECT service, count(*) FROM logs:error GROUP BY service
```

### Query Plan Caching

Optimized query plans are cached for repeated executions:

```text
-- First execution: full optimization
SELECT service, count(*) FROM logs:error WHERE timestamp > now() - 1h GROUP BY service

-- Subsequent executions: cached plan
SELECT service, count(*) FROM logs:error WHERE timestamp > now() - 1h GROUP BY service
```

### Adaptive Optimization

Query optimization adapts based on runtime statistics:

```text
-- Adaptive join algorithm selection
SELECT l.message, m.cpu_usage
FROM logs l
JOIN metrics m ON l.service = m.service
-- Optimizer may switch from hash join to nested loop based on data size
```

## Best Practices

### Query Design

1. **Be Specific with Filters**: Use targeted filters to reduce data volume
2. **Choose Appropriate Time Windows**: Balance analysis needs with performance
3. **Leverage Indexes**: Design queries to utilize available indexes
4. **Use LIMIT for Exploration**: Always limit results for exploratory queries
5. **Consider Data Distribution**: Understand how data is partitioned across engines

### Performance Optimization

1. **Filter Early**: Apply restrictive filters in WHERE clauses
2. **Use Appropriate Aggregations**: Aggregate before joining when possible
3. **Avoid Cartesian Products**: Ensure JOINs have proper conditions
4. **Monitor Query Performance**: Use EXPLAIN to analyze execution plans
5. **Use Query Hints**: Provide execution hints when needed

### Correlation Best Practices

1. **Define Clear Time Windows**: Choose time windows appropriate for your domain
2. **Use Appropriate Operators**: Select WITHIN, NEAR, BEFORE, or AFTER based on causality
3. **Consider Data Latency**: Account for ingestion delays in correlations
4. **Validate Correlation Logic**: Test correlations with known event sequences
5. **Monitor Correlation Performance**: Track execution time and result accuracy

### Multi-Engine Considerations

1. **Understand Engine Capabilities**: Know strengths of each data engine
2. **Use Engine-Specific Features**: Leverage specialized functions when available
3. **Balance Load**: Distribute queries across engines appropriately
4. **Handle Engine Differences**: Account for query language variations
5. **Monitor Cross-Engine Performance**: Track query performance across engines

## Examples

### Error Analysis

```text
-- Recent errors by service with trends
SELECT service, level, count(*) as count,
       count(*) / lag(count(*)) OVER (PARTITION BY service ORDER BY date_trunc('hour', timestamp)) as trend
FROM logs
WHERE level IN ('error', 'fatal') AND timestamp > now() - 1h
GROUP BY service, level, date_trunc('hour', timestamp)
ORDER BY count DESC

-- Errors correlated with performance degradation
logs:service:api:error NEAR 5m OF metrics:service:api:response_time > 5000
```

### Performance Monitoring

```text
-- Response time analysis with percentiles
SELECT endpoint,
       count(*) as requests,
       avg(response_time) as avg_response,
       percentile(response_time, 50) as p50,
       percentile(response_time, 95) as p95,
       percentile(response_time, 99) as p99
FROM metrics:http_requests
WHERE timestamp > now() - 1h
GROUP BY endpoint
ORDER BY p95 DESC

-- Resource usage correlation
metrics:cpu_usage > 80 NEAR 2m OF metrics:memory_usage > 85 AND logs:service:api:level:error
```

### Service Health Dashboard

```text
-- Comprehensive service health
SELECT service,
       count(*) as total_requests,
       sum(case when status >= 500 then 1 else 0 end) as server_errors,
       sum(case when status >= 400 then 1 else 0 end) as client_errors,
       avg(response_time) as avg_response,
       percentile(response_time, 95) as p95_response,
       (server_errors * 100.0 / total_requests) as error_rate
FROM metrics:http_requests
WHERE timestamp > now() - 24h
GROUP BY service
HAVING total_requests > 100
ORDER BY error_rate DESC, p95_response DESC
```

### Incident Investigation

```text
-- Timeline analysis with multi-domain correlation
SELECT timestamp, level, message, service, 'log' as source
FROM logs
WHERE timestamp BETWEEN '2025-01-01 10:00:00' AND '2025-01-01 11:00:00'
  AND (level IN ('error', 'fatal') OR message CONTAINS 'timeout')
UNION ALL
SELECT timestamp, 'metric' as level, concat('cpu_usage: ', cast(cpu_usage as string)) as message, service, 'metric' as source
FROM metrics:cpu_usage
WHERE timestamp BETWEEN '2025-01-01 10:00:00' AND '2025-01-01 11:00:00' AND cpu_usage > 80
ORDER BY timestamp

-- Root cause analysis with trace correlation
logs:service:api:error WITHIN 5m OF traces:service:api:operation:checkout:status:error
```

### Capacity Planning

```text
-- Peak usage pattern analysis
SELECT date_trunc('hour', timestamp) as hour,
       max(cpu_usage) as peak_cpu,
       avg(cpu_usage) as avg_cpu,
       percentile(cpu_usage, 95) as p95_cpu,
       max(memory_usage) as peak_memory,
       avg(memory_usage) as avg_memory
FROM metrics:system
WHERE timestamp > now() - 7d
GROUP BY hour
ORDER BY hour

-- Resource usage forecasting
SELECT service,
       avg(cpu_usage) as avg_cpu,
       max(cpu_usage) as max_cpu,
       stddev(cpu_usage) as cpu_stddev,
       trend_slope(cpu_usage) as cpu_trend
FROM metrics:cpu_usage
WHERE timestamp > now() - 30d
GROUP BY service
ORDER BY avg_cpu DESC
```

### Advanced Analytics

```text
-- Anomaly detection with statistical analysis
SELECT service, timestamp, cpu_usage,
       (cpu_usage - avg(cpu_usage) OVER (PARTITION BY service ORDER BY timestamp ROWS 24 PRECEDING)) /
       stddev(cpu_usage) OVER (PARTITION BY service ORDER BY timestamp ROWS 24 PRECEDING) as z_score
FROM metrics:cpu_usage
WHERE timestamp > now() - 7d
  AND abs(z_score) > 3  -- Anomalous values

-- Service dependency analysis
SELECT dependent_service, dependency_service, count(*) as correlation_count
FROM (
  SELECT l1.service as dependent_service, l2.service as dependency_service
  FROM logs:error l1
  JOIN logs:error l2 ON l1.request_id = l2.request_id
    AND l1.service != l2.service
    AND l1.timestamp WITHIN 1m OF l2.timestamp
)
GROUP BY dependent_service, dependency_service
ORDER BY correlation_count DESC
```

## API Usage

### Execute UQL Query

```bash
curl -X POST http://localhost:8080/api/v1/uql/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT service, count(*) FROM logs:error GROUP BY service",
    "id": "error-analysis",
    "timeout": "30s"
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

### Execute Correlation Query

```bash
curl -X POST http://localhost:8080/api/v1/uql/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "logs:error NEAR 5m OF metrics:cpu_usage > 80",
    "id": "correlation-analysis",
    "options": {
      "max_results": 1000,
      "include_metadata": true
    }
  }'
```

### Get Query Plan

```bash
curl -X POST http://localhost:8080/api/v1/uql/explain \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT service, count(*) FROM logs:error GROUP BY service"
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
      "data_sources": ["logs"],
      "engines_used": ["victorialogs"],
      "optimized": true,
      "parallel_execution": false
    }
  },
  "execution_time_ms": 45,
  "cached": false,
  "optimization_info": {
    "passes_applied": ["predicate_pushdown", "index_selection"],
    "estimated_cost": 1250,
    "actual_cost": 1180
  }
}
```

## Troubleshooting

### Common Issues

1. **No Results**
   - Check data source availability and permissions
   - Verify time ranges and data ingestion
   - Ensure correct field names and syntax
   - Check tenant isolation settings

2. **Slow Queries**
   - Add more restrictive filters
   - Use appropriate indexes and query hints
   - Consider query optimization and parallel execution
   - Check data source performance

3. **Syntax Errors**
   - Validate query syntax with the parser
   - Check field and function names
   - Ensure proper quoting and escaping
   - Use parameterized queries for complex values

4. **Optimization Issues**
   - Review query structure and statistics
   - Check data distribution and partitioning
   - Consider manual query rewriting
   - Monitor execution plans

5. **Correlation Problems**
   - Verify time window sizes are appropriate
   - Check data synchronization across engines
   - Validate correlation logic with test data
   - Monitor correlation performance metrics

### Debug Commands

```bash
# Get query execution plan
curl -X POST http://localhost:8080/api/v1/uql/explain -d '{"query": "SELECT * FROM logs"}'

# Check query parsing
curl -X POST http://localhost:8080/api/v1/uql/parse -d '{"query": "SELECT * FROM logs"}'

# Get optimization statistics
curl -X GET http://localhost:8080/api/v1/uql/stats

# Validate query syntax
curl -X POST http://localhost:8080/api/v1/uql/validate -d '{"query": "SELECT * FROM logs"}'
```

## Advanced Features

### Custom Functions

UQL supports custom user-defined functions:

```text
-- Register custom function
CREATE FUNCTION error_severity(message TEXT) RETURNS INTEGER AS $$
  CASE
    WHEN message CONTAINS 'panic' THEN 5
    WHEN message CONTAINS 'fatal' THEN 4
    WHEN message CONTAINS 'error' THEN 3
    WHEN message CONTAINS 'warn' THEN 2
    ELSE 1
  END
$$;

-- Use custom function
SELECT message, error_severity(message) as severity
FROM logs
WHERE severity >= 3
ORDER BY severity DESC;
```

### Subqueries and CTEs

Complex queries with common table expressions:

```text
WITH error_summary AS (
  SELECT service, count(*) as error_count
  FROM logs:error
  WHERE timestamp > now() - 1h
  GROUP BY service
),
service_metrics AS (
  SELECT service, avg(cpu_usage) as avg_cpu
  FROM metrics:cpu_usage
  WHERE timestamp > now() - 1h
  GROUP BY service
)
SELECT e.service, e.error_count, m.avg_cpu
FROM error_summary e
JOIN service_metrics m ON e.service = m.service
WHERE e.error_count > 10
ORDER BY e.error_count DESC;
```

### Recursive Queries

Handle hierarchical data and recursive relationships:

```text
WITH RECURSIVE service_dependencies AS (
  -- Base case: services with no dependencies
  SELECT service, ARRAY[]::TEXT[] as dependency_path
  FROM service_registry
  WHERE dependencies IS NULL

  UNION ALL

  -- Recursive case: add dependent services
  SELECT s.service, d.dependency_path || s.depends_on
  FROM service_registry s
  JOIN service_dependencies d ON s.depends_on = d.service
)
SELECT * FROM service_dependencies;
```

### Query Templates

Reusable query templates with parameters:

```text
-- Define template
CREATE TEMPLATE service_health AS
SELECT service,
       count(*) as requests,
       avg(response_time) as avg_response,
       percentile(response_time, 95) as p95_response,
       sum(case when status >= 500 then 1 else 0 end) * 100.0 / count(*) as error_rate
FROM metrics:http_requests
WHERE timestamp > $time_range
  AND service = $service_name
GROUP BY service;

-- Execute template
EXECUTE service_health (
  time_range => now() - 1h,
  service_name => 'api'
);
```

## Conclusion

UQL provides a powerful, unified interface for complex observability data analysis through its sophisticated compilation pipeline, multi-engine support, and advanced correlation capabilities. By understanding the compilation process, optimization features, and best practices outlined in this guide, you can construct efficient queries that provide deep insights across your entire observability stack.

For more advanced use cases, consult the [API Reference](api-reference.md), [Correlation Engine Documentation](correlation-engine.md), and [Unified Query Architecture](unified-query-architecture.md).</content>
<parameter name="filePath">/Users/aarvee/repos/github/public/mirador-core/docs/uql-language-guide.md