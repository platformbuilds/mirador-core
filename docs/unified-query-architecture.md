# Unified Query Engine Architecture

## Overview

The Unified Query Engine is the core component of MIRADOR-CORE v7.0.0 that provides intelligent routing and execution of queries across multiple observability engines. It abstracts the complexity of dealing with different data sources (metrics, logs, traces) and provides a single, consistent API for all observability queries.

## Architecture Components

### 1. UnifiedQueryEngine Interface

The `UnifiedQueryEngine` interface defines the contract for unified query operations:

```go
type UnifiedQueryEngine interface {
    // ExecuteQuery executes a unified query across the appropriate engines
    ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

    // ExecuteCorrelationQuery executes a correlation query across multiple engines
    ExecuteCorrelationQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

    // GetQueryMetadata returns metadata about supported query types and capabilities
    GetQueryMetadata(ctx context.Context) (*models.QueryMetadata, error)

    // HealthCheck checks the health of all underlying engines
    HealthCheck(ctx context.Context) (*models.EngineHealthStatus, error)

    // InvalidateCache invalidates cached results for a query pattern
    InvalidateCache(ctx context.Context, queryPattern string) error
}
```

### 2. Query Router

The `QueryRouter` analyzes query patterns and routes them to the optimal engine based on query characteristics:

#### Routing Logic

The router uses pattern matching to determine the best engine for each query:

- **Metrics Queries**: Detected by patterns like `rate(`, `increase(`, `histogram`, `counter`, `gauge`, etc.
- **Traces Queries**: Detected by patterns like `service:`, `operation:`, `span`, `trace`, etc.
- **Logs Queries**: Default routing for general queries and search patterns

#### Example Routing Decisions

```go
// Metrics query - routes to VictoriaMetrics
"rate(http_requests_total[5m])" → QueryTypeMetrics

// Traces query - routes to VictoriaTraces
"service:auth operation:login" → QueryTypeTraces

// Logs query - routes to VictoriaLogs
"error AND status:500" → QueryTypeLogs
```

### 3. Engine Abstraction Layer

The unified query engine abstracts three main observability engines:

#### VictoriaMetrics Service
- Handles metrics queries (instant, range, series)
- Supports MetricsQL query language
- Provides metadata about available metrics

#### VictoriaLogs Service
- Handles logs queries and streaming
- Supports LogsQL query language
- Provides field extraction and filtering

#### VictoriaTraces Service
- Handles distributed tracing queries
- Supports trace correlation and flame graphs
- Provides service and operation metadata

## Data Flow

### Query Execution Flow

```
1. Query Request
       ↓
2. Query Router Analysis
       ↓
3. Engine Selection
       ↓
4. Cache Check
       ↓
5. Engine Execution
       ↓
6. Result Unification
       ↓
7. Cache Storage
       ↓
8. Response
```

### Detailed Flow

1. **Query Reception**: HTTP request received at `/api/v1/unified/query`
2. **Router Analysis**: QueryRouter analyzes query pattern and determines optimal engine
3. **Cache Lookup**: Check Valkey cache for existing results
4. **Engine Dispatch**: Route to appropriate engine (Metrics, Logs, or Traces)
5. **Result Processing**: Transform engine-specific results to unified format
6. **Caching**: Store results in cache with TTL
7. **Response**: Return unified result to client

## Query Models

### UnifiedQuery

The core query model that supports all engine types:

```go
type UnifiedQuery struct {
    ID        string                 `json:"id"`
    Type      QueryType              `json:"type"`
    Query     string                 `json:"query"`
    TenantID  string                 `json:"tenant_id,omitempty"`
    StartTime *time.Time             `json:"start_time,omitempty"`
    EndTime   *time.Time             `json:"end_time,omitempty"`
    Timeout   string                 `json:"timeout,omitempty"`
    Parameters map[string]interface{} `json:"parameters,omitempty"`
    CorrelationOptions *CorrelationOptions `json:"correlation_options,omitempty"`
    CacheOptions *CacheOptions       `json:"cache_options,omitempty"`
}
```

### UnifiedResult

The unified response format that normalizes results from all engines:

```go
type UnifiedResult struct {
    QueryID       string                    `json:"query_id"`
    Type          QueryType                 `json:"type"`
    Status        string                    `json:"status"`
    Data          interface{}               `json:"data"`
    Metadata      *ResultMetadata           `json:"metadata"`
    Correlations  *UnifiedCorrelationResult `json:"correlations,omitempty"`
    ExecutionTime int64                     `json:"execution_time_ms"`
    Cached        bool                      `json:"cached"`
}
```

## Caching Strategy

### Cache Architecture

The unified query engine implements a comprehensive caching strategy:

- **Query Result Cache**: Caches complete query results with configurable TTL
- **Metadata Cache**: Caches engine metadata and capabilities
- **Cross-Engine Invalidation**: Intelligent cache invalidation across engines

### Cache Configuration

```yaml
unified_query:
  enabled: true
  cache_ttl: 5m
  max_cache_ttl: 1h
  default_limit: 1000
```

## Health Monitoring

### Engine Health Checks

The unified query engine provides comprehensive health monitoring:

- **Individual Engine Health**: Status of each underlying engine
- **Overall System Health**: Aggregated health status
- **Last Checked Timestamp**: When health was last verified

### Health Response

```json
{
  "overall_health": "healthy",
  "engine_health": {
    "metrics": "healthy",
    "logs": "healthy",
    "traces": "healthy"
  },
  "last_checked": "2025-10-25T10:30:00Z"
}
```

## Error Handling

### Error Types

The unified query engine handles various error scenarios:

- **Routing Errors**: Invalid query patterns or unsupported engines
- **Engine Errors**: Failures in individual engines (VictoriaMetrics, VictoriaLogs, VictoriaTraces)
- **Timeout Errors**: Queries exceeding configured timeouts
- **Cache Errors**: Cache connection or serialization failures

### Error Response Format

```json
{
  "error": "Query execution failed",
  "details": "VictoriaMetrics service unavailable",
  "query_id": "query-12345"
}
```

## Performance Optimizations

### Intelligent Routing

- **Pattern-Based Routing**: Routes queries to engines based on content analysis
- **Load Balancing**: Distributes queries across multiple engine instances
- **Query Optimization**: Applies engine-specific optimizations before execution

### Caching Optimizations

- **Adaptive TTL**: Adjusts cache lifetime based on query frequency
- **Size-Based Limits**: Prevents cache bloat with size limits
- **Background Invalidation**: Asynchronous cache cleanup

## Integration Points

### HTTP API Layer

The unified query engine integrates with the HTTP API through handlers:

- `UnifiedQueryHandler`: Main query execution handler
- `HandleUnifiedQuery`: POST `/api/v1/unified/query`
- `HandleUnifiedCorrelation`: POST `/api/v1/unified/correlation`
- `HandleQueryMetadata`: GET `/api/v1/unified/metadata`
- `HandleHealthCheck`: GET `/api/v1/unified/health`

### Service Layer

Integration with existing services:

- **VictoriaMetricsServices**: Metrics query execution
- **Cache (Valkey)**: Distributed caching layer
- **Logger**: Structured logging for observability

## Configuration

### Unified Query Configuration

```go
type UnifiedQueryConfig struct {
    Enabled           bool          `yaml:"enabled" default:"true"`
    CacheTTL          time.Duration `yaml:"cache_ttl" default:"5m"`
    MaxCacheTTL       time.Duration `yaml:"max_cache_ttl" default:"1h"`
    DefaultLimit      int           `yaml:"default_limit" default:"1000"`
    EnableCorrelation bool          `yaml:"enable_correlation" default:"false"`
}
```

## Future Extensions

### Planned Enhancements

- **Correlation Engine**: Cross-engine correlation queries
- **Query Language**: Unified query language with advanced operators
- **Metrics Metadata**: Indexed metrics discovery
- **Performance Monitoring**: Detailed query performance metrics

### Extensibility

The architecture is designed for easy extension:

- **New Engines**: Add new observability engines by implementing engine interfaces
- **Custom Routers**: Implement custom routing logic for specialized use cases
- **Query Types**: Extend query types for new observability patterns

## Monitoring and Observability

### Metrics

The unified query engine exposes Prometheus metrics:

- `unified_query_requests_total`: Total number of unified queries
- `unified_query_duration_seconds`: Query execution duration
- `unified_query_cache_hits_total`: Cache hit counter
- `unified_query_engine_routing`: Query routing decisions

### Logging

Structured logging provides observability:

- Query routing decisions
- Engine execution times
- Cache hit/miss ratios
- Error conditions and stack traces

## Conclusion

The Unified Query Engine provides a robust, scalable foundation for unified observability queries across the VictoriaMetrics ecosystem. Its intelligent routing, comprehensive caching, and extensible architecture enable efficient querying of metrics, logs, and traces through a single, consistent API.</content>
<parameter name="filePath">/Users/aarvee/repos/github/public/mirador-core/docs/unified-query-architecture.md