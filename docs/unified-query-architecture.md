# Unified Query Engine Architecture

## Overview

The Unified Query Engine is the core component of MIRADOR-CORE that provides intelligent routing, parallel execution, and correlation of queries across multiple observability engines (VictoriaMetrics, VictoriaLogs, VictoriaTraces). It abstracts the complexity of dealing with different data sources and provides a single, consistent API for all observability queries, including advanced UQL (Unified Query Language) support.

## Architecture Components

### 1. UnifiedQueryEngine Interface

The `UnifiedQueryEngine` interface defines the contract for unified query operations:

```text
type UnifiedQueryEngine interface {
    // ExecuteQuery executes a unified query across the appropriate engines
    ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

    // ExecuteCorrelationQuery executes a correlation query across multiple engines
    ExecuteCorrelationQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

    // ExecuteUQLQuery executes a UQL query by parsing, optimizing, translating, and executing it
    ExecuteUQLQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

    // GetQueryMetadata returns metadata about supported query types and capabilities
    GetQueryMetadata(ctx context.Context) (*models.QueryMetadata, error)

    // HealthCheck checks the health of all underlying engines
    HealthCheck(ctx context.Context) (*models.EngineHealthStatus, error)

    // InvalidateCache invalidates cached results for a query pattern
    InvalidateCache(ctx context.Context, queryPattern string) error
}
```

### 2. Query Router

The `QueryRouter` analyzes query patterns and routes them to the optimal engine based on query characteristics, supporting intelligent routing patterns and parallel execution.

#### Routing Logic

The router uses sophisticated pattern matching and metadata analysis:

- **Metrics Queries**: Detected by MetricsQL patterns (`rate(`, `increase(`, `histogram_quantile`, etc.)
- **Traces Queries**: Detected by Jaeger/OTLP patterns (`service:`, `operation:`, `span.`, etc.)
- **Logs Queries**: Detected by LogsQL patterns and general search terms
- **Correlation Queries**: Multi-engine queries with temporal operators (`WITHIN`, `NEAR`, `BEFORE`, `AFTER`)

#### Advanced Routing Features

```text
type QueryRouter struct {
    // Pattern-based routing with regex matching
    patterns map[QueryType][]*regexp.Regexp

    // Metadata-driven routing using engine capabilities
    metadataRouter *MetadataRouter

    // Parallel execution coordinator
    parallelExecutor *ParallelExecutor

    // Health-aware routing
    healthChecker *EngineHealthChecker
}
```

#### Example Routing Decisions

```text
// Metrics query - routes to VictoriaMetrics with instant/range detection
"rate(http_requests_total[5m])" → QueryTypeMetrics (Range)

// Traces query - routes to VictoriaTraces with service/operation filtering
"service:auth operation:login status:error" → QueryTypeTraces

// Logs query - routes to VictoriaLogs with Lucene syntax
"level:error AND service:api" → QueryTypeLogs

// Correlation query - parallel execution across engines
"logs:error WITHIN 5m OF metrics:cpu_usage > 80" → QueryTypeCorrelation
```

### 3. UQL Processing Pipeline

The Unified Query Language (UQL) processing pipeline handles advanced query parsing, optimization, and translation with a complete compiler-like architecture.

#### UQL Parser (`uql_parser.go`)
- **Grammar Definition**: Comprehensive grammar supporting SELECT, correlation, aggregation, JOIN, and temporal operators
- **AST Generation**: Parses queries into Abstract Syntax Trees with full type checking
- **Query Types**: Supports declarative queries, correlations, aggregations, and federated searches

#### UQL Optimizer (`uql_optimizer.go`)
- **Multi-Pass Optimization**: Configurable optimization passes including:
  - `PredicatePushdown`: Push filters down to individual engines
  - `CostBasedPlanning`: Estimate execution costs and choose optimal plans
  - `QueryRewriting`: Rewrite queries for better performance
  - `Parallelization`: Identify opportunities for parallel execution
- **Statistics Tracking**: Maintains optimization metrics and performance data
- **Query Plan Generation**: Creates detailed execution plans with cost estimation

#### UQL Translator Registry (`uql_translator.go`)
- **Multi-Engine Translation**: Translates UQL to engine-specific languages:
  - UQL → PromQL (VictoriaMetrics)
  - UQL → LogsQL (VictoriaLogs)
  - UQL → Trace filters (VictoriaTraces)
- **Plugin Architecture**: Extensible translator system with engine-specific plugins
- **Parameter Binding**: Handles query parameters, time windows, and aggregation functions

#### Processing Flow

```
UQL Query String
       ↓
   UQL Parser
   (Lexical Analysis → AST)
       ↓
  UQL Optimizer
  (Predicate Pushdown, Cost Planning, Parallelization)
       ↓
UQL Translator Registry
  (Multi-Engine Translation)
       ↓
Engine-Specific Queries
  (PromQL, LogsQL, Trace Filters)
       ↓
   Parallel Execution
   (QueryRouter coordinates execution)
       ↓
   Result Correlation
   (UnifiedResult aggregation)
```

### 4. Correlation Engine

The `CorrelationEngine` provides advanced cross-engine correlation capabilities:

```text
type CorrelationEngine struct {
    // Service graph analysis
    serviceGraph *ServiceGraphAnalyzer

    // Temporal correlation with time windows
    temporalCorrelator *TemporalCorrelator

    // Causal relationship detection
    causalAnalyzer *CausalAnalyzer

    // Pattern-based correlation rules
    patternMatcher *PatternMatcher
}
```

#### Correlation Types

- **Temporal Correlations**: Events within time windows (`WITHIN 5m OF`)
- **Causal Correlations**: Cause-effect relationships between services
- **Service Graph Correlations**: Dependency-based correlations
- **Pattern-Based Correlations**: Known failure patterns and red flags

## Data Flow

### Query Execution Flow

```
1. Query Request (HTTP API)
       ↓
2. Authentication & RBAC
       ↓
3. Query Type Detection
       ↓
4. UQL Processing Pipeline (if UQL)
   ├── Parse → Validate → Optimize → Translate
       ↓
5. Query Router Analysis
   ├── Pattern Matching
   ├── Metadata Analysis
   └── Health Checking
       ↓
6. Engine Selection & Parallel Dispatch
   ├── VictoriaMetrics (MetricsQL)
   ├── VictoriaLogs (LogsQL)
   └── VictoriaTraces (Jaeger API)
       ↓
7. Cache Check (Valkey Cluster)
       ↓
8. Parallel Execution
   ├── executeParallelQuery()
   ├── Result Aggregation
   └── Timeout Handling
       ↓
9. Result Unification
   ├── Format Normalization
   ├── Metadata Enrichment
   └── Warning Aggregation
       ↓
10. Cache Storage
       ↓
11. Response (UnifiedResult)
```

### Parallel Execution Architecture

The engine supports sophisticated parallel execution patterns:

```text
func (e *UnifiedQueryEngine) executeParallelQuery(ctx context.Context, queries map[QueryType]string) (*UnifiedResult, error) {
    // Create execution context with timeout
    execCtx, cancel := context.WithTimeout(ctx, e.config.Timeout)
    defer cancel()

    // Fan-out to multiple engines
    results := make(chan *EngineResult, len(queries))
    errors := make(chan error, len(queries))

    for engine, query := range queries {
        go func(engine QueryType, query string) {
            result, err := e.executeOnEngine(execCtx, engine, query)
            if err != nil {
                errors <- err
                return
            }
            results <- result
        }(engine, query)
    }

    // Fan-in results with correlation
    return e.correlateResults(execCtx, results, errors)
}
```

## Query Models

### UnifiedQuery

The core query model supporting all engine types and advanced features:

```text
type UnifiedQuery struct {
    ID          string                    `json:"id"`
    Type        QueryType                 `json:"type"`
    Query       string                    `json:"query"`
    TenantID    string                    `json:"tenant_id,omitempty"`
    StartTime   *time.Time                `json:"start_time,omitempty"`
    EndTime     *time.Time                `json:"end_time,omitempty"`
    Timeout     string                    `json:"timeout,omitempty"`
    Parameters  map[string]interface{}    `json:"parameters,omitempty"`
    CorrelationOptions *CorrelationOptions `json:"correlation_options,omitempty"`
    CacheOptions *CacheOptions           `json:"cache_options,omitempty"`
    UQLOptions  *UQLOptions              `json:"uql_options,omitempty"`
}
```

### UQL Query Structure

Advanced UQL query representation:

```text
type UQLQuery struct {
    RawQuery    string
    AST         *QueryAST
    Engines     []QueryType
    TimeWindow  *TimeWindow
    Aggregations []Aggregation
    Correlations []Correlation
    Filters     []Filter
    Joins       []Join
    Optimizations []OptimizationPass
}
```

### UnifiedResult

The unified response format with comprehensive metadata:

```text
type UnifiedResult struct {
    QueryID       string                    `json:"query_id"`
    Type          QueryType                 `json:"type"`
    Status        string                    `json:"status"`
    Data          interface{}               `json:"data"`
    Metadata      *ResultMetadata           `json:"metadata"`
    Correlations  *UnifiedCorrelationResult `json:"correlations,omitempty"`
    ExecutionTime int64                     `json:"execution_time_ms"`
    Cached        bool                      `json:"cached"`
    Warnings      []string                  `json:"warnings,omitempty"`
    EngineResults map[QueryType]*EngineResult `json:"engine_results,omitempty"`
}
```

## Caching Strategy

### Multi-Level Caching Architecture

The unified query engine implements sophisticated caching:

- **Query Result Cache**: Caches complete query results with TTL
- **Parsed Query Cache**: Caches parsed UQL ASTs
- **Translated Query Cache**: Caches engine-specific translations
- **Metadata Cache**: Caches engine capabilities and schemas
- **Negative Cache**: Caches failed queries to prevent retries

### Valkey Cluster Integration

```text
type ValkeyCache struct {
    cluster *redis.ClusterClient
    ttl     time.Duration
    keyPrefix string
}

func (c *ValkeyCache) Get(ctx context.Context, key string) (interface{}, error) {
    return c.cluster.Get(ctx, c.keyPrefix+key).Result()
}

func (c *ValkeyCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    return c.cluster.Set(ctx, c.keyPrefix+key, value, ttl).Err()
}
```

### Cache Invalidation Strategy

- **Time-Based**: TTL-based expiration
- **Event-Based**: Invalidation on data updates
- **Pattern-Based**: Wildcard invalidation for related queries
- **Cross-Engine**: Coordinated invalidation across engines

## Health Monitoring

### Comprehensive Health Checks

```text
type EngineHealthStatus struct {
    OverallHealth string                     `json:"overall_health"`
    EngineHealth  map[QueryType]string       `json:"engine_health"`
    LastChecked   time.Time                  `json:"last_checked"`
    ResponseTime  map[QueryType]time.Duration `json:"response_time"`
    ErrorDetails  map[QueryType]string       `json:"error_details,omitempty"`
}
```

### Health Check Implementation

```text
func (e *UnifiedQueryEngine) HealthCheck(ctx context.Context) (*EngineHealthStatus, error) {
    status := &EngineHealthStatus{
        EngineHealth: make(map[QueryType]string),
        ResponseTime: make(map[QueryType]time.Duration),
        ErrorDetails: make(map[QueryType]string),
    }

    // Parallel health checks
    var wg sync.WaitGroup
    var mu sync.Mutex

    for _, engine := range []QueryType{QueryTypeMetrics, QueryTypeLogs, QueryTypeTraces} {
        wg.Add(1)
        go func(engine QueryType) {
            defer wg.Done()

            start := time.Now()
            healthy, err := e.checkEngineHealth(ctx, engine)
            duration := time.Since(start)

            mu.Lock()
            status.ResponseTime[engine] = duration
            if healthy {
                status.EngineHealth[engine] = "healthy"
            } else {
                status.EngineHealth[engine] = "unhealthy"
                if err != nil {
                    status.ErrorDetails[engine] = err.Error()
                }
            }
            mu.Unlock()
        }(engine)
    }

    wg.Wait()

    // Determine overall health
    status.OverallHealth = "healthy"
    for _, health := range status.EngineHealth {
        if health != "healthy" {
            status.OverallHealth = "degraded"
            break
        }
    }

    status.LastChecked = time.Now()
    return status, nil
}
```

## Error Handling

### Error Types and Recovery

The unified query engine handles various error scenarios with graceful degradation:

- **Routing Errors**: Fallback to alternative engines
- **Engine Errors**: Degraded mode with partial results
- **Timeout Errors**: Configurable timeouts with partial result returns
- **Cache Errors**: Cache miss fallback to direct execution
- **Correlation Errors**: Independent engine execution when correlation fails

### Error Response Format

```json
{
  "error": "Query execution failed",
  "details": "VictoriaMetrics service unavailable",
  "query_id": "query-12345",
  "partial_results": {"metrics": [], "logs": []},
  "degraded_engines": ["metrics"],
  "retry_after": 30
}
```

## Performance Optimizations

### Intelligent Routing

- **Pattern-Based Routing**: Content analysis for optimal engine selection
- **Load Balancing**: Query distribution across engine instances
- **Query Optimization**: Engine-specific optimizations before execution
- **Parallel Execution**: Concurrent query execution across engines

### Advanced Caching

- **Adaptive TTL**: Dynamic cache lifetime based on query patterns
- **Size-Based Limits**: Memory management with result size limits
- **Compression**: Result compression for large datasets
- **Background Invalidation**: Asynchronous cache maintenance

### Query Optimization

```text
type UQLOptimizer struct {
    passes []OptimizationPass
    stats  *OptimizationStats
}

type OptimizationPass struct {
    Name        string
    Enabled     bool
    CostBenefit float64
    Apply       func(*UQLQuery) error
}

func (o *UQLOptimizer) Optimize(query *UQLQuery) error {
    for _, pass := range o.passes {
        if pass.Enabled {
            start := time.Now()
            if err := pass.Apply(query); err != nil {
                return err
            }
            o.stats.RecordPass(pass.Name, time.Since(start))
        }
    }
    return nil
}
```

## Integration Points

### HTTP API Layer

Integration with comprehensive HTTP handlers:

```text
type UnifiedQueryHandler struct {
    engine *UnifiedQueryEngine
    logger logger.Logger
}

func (h *UnifiedQueryHandler) HandleUnifiedQuery(c *gin.Context) {
    // Authentication & RBAC
    // Query parsing & validation
    // Engine execution
    // Result formatting
}

func (h *UnifiedQueryHandler) HandleUQLQuery(c *gin.Context) {
    // UQL parsing & optimization
    // Translation & execution
    // Advanced result processing
}
```

### Service Layer Integration

- **VictoriaMetricsServices**: MetricsQL execution with PromQL compatibility
- **VictoriaLogsServices**: LogsQL execution with Lucene/Bleve support
- **VictoriaTracesServices**: Jaeger-compatible trace queries
- **CorrelationEngine**: Cross-engine correlation analysis
- **Valkey Cluster**: Distributed caching and session management
- **RBAC Enforcer**: Comprehensive access control

## Configuration

### Unified Query Configuration

```yaml
unified_query:
  enabled: true
  cache_ttl: 5m
  max_cache_ttl: 1h
  default_limit: 1000
  enable_correlation: true
  uql:
    enabled: true
    optimization:
      predicate_pushdown: true
      cost_based_planning: true
      parallel_execution: true
  engines:
    victoriametrics:
      timeout: 30s
      retries: 3
    victorialogs:
      timeout: 45s
      retries: 2
    victoriatraces:
      timeout: 30s
      retries: 3
```

## Monitoring and Observability

### Comprehensive Metrics

```text
// Prometheus metrics
unified_query_requests_total{engine="metrics", status="success"} counter
unified_query_duration_seconds{engine="metrics", quantile="0.95"} histogram
unified_query_cache_hits_total{operation="get"} counter
unified_query_engine_routing{from="unified", to="metrics"} counter
unified_uql_parse_duration_seconds histogram
unified_uql_optimize_duration_seconds histogram
unified_uql_translate_duration_seconds{engine="promql"} histogram
```

### Structured Logging

```json
{
  "level": "info",
  "component": "unified_query_engine",
  "operation": "execute_uql",
  "query_id": "uql-12345",
  "query": "SELECT service, count(*) FROM logs:error WHERE level='error'",
  "engines": ["logs"],
  "execution_time_ms": 245,
  "cached": false,
  "optimization_applied": ["predicate_pushdown", "parallel_execution"]
}
```

## Future Extensions

### Advanced Features

- **Federated Queries**: Cross-cluster query execution
- **Machine Learning Integration**: Predictive query optimization
- **Custom Functions**: User-defined aggregation functions
- **Real-time Streaming**: Continuous query execution
- **Query Templates**: Parameterized query templates

### Extensibility Architecture

The architecture supports easy extension through plugin interfaces:

```text
type EnginePlugin interface {
    Name() string
    SupportsQuery(query string) bool
    ExecuteQuery(ctx context.Context, query *EngineQuery) (*EngineResult, error)
    HealthCheck(ctx context.Context) error
}

type TranslatorPlugin interface {
    SourceLanguage() string
    TargetLanguage() string
    Translate(query string, params map[string]interface{}) (string, error)
}
```

## Conclusion

The Unified Query Engine provides a sophisticated, production-ready foundation for unified observability across the VictoriaMetrics ecosystem. Its advanced UQL processing pipeline, intelligent routing, parallel execution capabilities, and comprehensive caching strategy enable efficient, scalable querying of metrics, logs, and traces through a single, powerful API.</content>
<parameter name="filePath">/Users/aarvee/repos/github/public/mirador-core/docs/unified-query-architecture.md