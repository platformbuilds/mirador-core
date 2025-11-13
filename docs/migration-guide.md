# Migration Guide: Transitioning to Unified Query API

## Overview

This guide helps you migrate from using separate engine-specific APIs (VictoriaMetrics, VictoriaLogs, VictoriaTraces) to the unified query API introduced in Mirador Core v7.0.0. The unified API provides intelligent routing, cross-engine correlation, and a consistent interface across all observability data types.

## Table of Contents

1. [Why Migrate?](#why-migrate)
2. [Migration Strategy](#migration-strategy)
3. [Compatibility](#compatibility)
4. [Step-by-Step Migration](#step-by-step-migration)
5. [Query Translation Examples](#query-translation-examples)
6. [Common Patterns](#common-patterns)
7. [Testing Your Migration](#testing-your-migration)
8. [Rollback Plan](#rollback-plan)
9. [Performance Considerations](#performance-considerations)
10. [Troubleshooting](#troubleshooting)
11. [Weaviate KPI Management](#weaviate-kpi-management)

## Why Migrate?

### Benefits of the Unified Query API

**1. Simplified Integration**
- Single endpoint for all query types
- Consistent request/response format
- Reduced client complexity

**2. Intelligent Routing**
- Automatic engine selection based on query patterns
- Optimized execution paths
- Reduced manual configuration

**3. Cross-Engine Correlation**
- Query logs, metrics, and traces together
- Time-window correlation with confidence scoring
- Label-based relationship discovery

**4. Enhanced Caching**
- Unified caching layer with configurable TTL
- Cross-engine cache optimization
- Improved query performance

**5. Better Observability**
- Unified metrics and tracing
- Consistent error handling
- Centralized query monitoring

## Migration Strategy

### Recommended Approach: Gradual Migration

We recommend a **phased migration approach** rather than a "big bang" cutover:

#### Phase 1: Parallel Operation (Weeks 1-2)
- Deploy v7.0.0 alongside existing implementation
- Add unified API calls in new features
- Keep existing engine-specific calls unchanged
- Monitor performance and validate results

#### Phase 2: Feature-by-Feature Migration (Weeks 3-6)
- Migrate one feature/service at a time
- Start with read-only queries
- Validate each migration before proceeding
- Keep rollback capability at each step

#### Phase 3: Complete Transition (Weeks 7-8)
- Migrate remaining queries
- Remove engine-specific client code
- Update monitoring and alerting
- Deprecate old API usage

#### Phase 4: Cleanup (Week 9+)
- Remove legacy code paths
- Optimize unified query patterns
- Update documentation and training materials

### Migration Timeline

```
Week 1-2:  Deploy v7.0.0, parallel operation
Week 3-4:  Migrate metrics queries
Week 5-6:  Migrate logs and traces queries
Week 7:    Migrate correlation/complex queries
Week 8:    Testing and validation
Week 9+:   Cleanup and optimization
```

## Compatibility

### Backward Compatibility

**✅ Fully Compatible:**
- All existing engine-specific APIs remain functional
- No breaking changes to request/response formats
- Existing authentication mechanisms work unchanged
- Current metrics and monitoring continue to work

**⚠️ Deprecated (Still Supported):**
- Direct engine-specific query endpoints
- Separate cache management APIs
- Individual engine health checks (use unified health endpoint)

**🔜 Future Deprecation (v8.0.0+):**
- Engine-specific correlation APIs
- Separate query metadata endpoints
- Individual engine configuration APIs

### Version Requirements

- **Minimum Version**: Mirador Core v9.0.0
- **Recommended Version**: Mirador Core v9.0.0+
- **VictoriaMetrics**: v1.93.0+
- **VictoriaLogs**: v0.5.0+
- **VictoriaTraces**: v0.3.0+

## Step-by-Step Migration

### Step 1: Update Dependencies

Update your Mirador Core client or SDK to v7.0.0+:

```bash
# Go
go get github.com/platformbuilds/mirador-core-go-client@v7.0.0

# Python
pip install mirador-core-client>=7.0.0

# Node.js
npm install @mirador/core-client@^7.0.0
```

### Step 2: Update Configuration

Add unified query configuration to your client:

```yaml
# config.yaml
mirador:
  api_url: "https://mirador-core.company.com"
  
  # Enable unified query API
  unified_query:
    enabled: true
    timeout: "30s"
    
  # Optional: Keep engine-specific endpoints for fallback
  engines:
    metrics: "https://mirador-core.company.com/api/v1/metrics"
    logs: "https://mirador-core.company.com/api/v1/logs"
    traces: "https://mirador-core.company.com/api/v1/traces"
```

### Step 3: Create Query Translation Layer

Build an abstraction layer to support both old and new APIs:

```go
// Go example
type QueryClient interface {
    ExecuteQuery(ctx context.Context, query Query) (*Result, error)
}

// Adapter for unified API
type UnifiedQueryClient struct {
    baseURL string
    client  *http.Client
}

func (c *UnifiedQueryClient) ExecuteQuery(ctx context.Context, q Query) (*Result, error) {
    req := &UnifiedQueryRequest{
        Query: UnifiedQuery{
            ID:        q.ID,
            Type:      q.Type,
            Query:     q.QueryString,
            StartTime: q.StartTime,
            EndTime:   q.EndTime,
            Timeout:   q.Timeout,
        },
    }
    
    resp, err := c.client.Post(
        c.baseURL+"/api/v1/unified/query",
        "application/json",
        toJSON(req),
    )
    if err != nil {
        return nil, err
    }
    
    return parseUnifiedResponse(resp)
}
```

```python
# Python example
class QueryClient(ABC):
    @abstractmethod
    def execute_query(self, query: Query) -> Result:
        pass

class UnifiedQueryClient(QueryClient):
    def __init__(self, base_url: str):
        self.base_url = base_url
        self.session = requests.Session()
    
    def execute_query(self, query: Query) -> Result:
        request = {
            "query": {
                "id": query.id,
                "type": query.type,
                "query": query.query_string,
                "start_time": query.start_time.isoformat(),
                "end_time": query.end_time.isoformat(),
                "timeout": query.timeout,
            }
        }
        
        response = self.session.post(
            f"{self.base_url}/api/v1/unified/query",
            json=request,
        )
        response.raise_for_status()
        
        return self._parse_response(response.json())
```

### Step 4: Implement Feature Flags

Use feature flags to control migration progress:

```go
// Go example
type QueryConfig struct {
    UseUnifiedAPI bool `env:"USE_UNIFIED_API" default:"false"`
    Rollout       int  `env:"UNIFIED_API_ROLLOUT" default:"0"` // 0-100%
}

func (s *Service) ExecuteQuery(ctx context.Context, q Query) (*Result, error) {
    if s.config.UseUnifiedAPI || shouldUseUnified(s.config.Rollout) {
        return s.unifiedClient.ExecuteQuery(ctx, q)
    }
    return s.legacyClient.ExecuteQuery(ctx, q)
}

func shouldUseUnified(rolloutPercent int) bool {
    return rand.Intn(100) < rolloutPercent
}
```

### Step 5: Migrate Query by Query

Start with simple queries and gradually increase complexity:

```go
// 1. Simple metrics query
// Old way
metricsResp, err := metricsClient.Query(ctx, "http_requests_total{job=\"api\"}")

// New way
unifiedResp, err := unifiedClient.ExecuteQuery(ctx, UnifiedQuery{
    Type:  "metrics",
    Query: "http_requests_total{job=\"api\"}",
})

// 2. Logs query with time range
// Old way
logsResp, err := logsClient.Search(ctx, LogsQuery{
    Query:     "level:error AND service:api",
    StartTime: time.Now().Add(-1 * time.Hour),
    EndTime:   time.Now(),
})

// New way
unifiedResp, err := unifiedClient.ExecuteQuery(ctx, UnifiedQuery{
    Type:      "logs",
    Query:     "level:error AND service:api",
    StartTime: time.Now().Add(-1 * time.Hour),
    EndTime:   time.Now(),
})

// 3. Correlation query (NEW capability!)
unifiedResp, err := unifiedClient.ExecuteQuery(ctx, UnifiedQuery{
    Type:  "correlation",
    Query: "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
})
```

### Step 6: Update Tests

Ensure your tests cover both old and new APIs during migration:

```go
func TestQueryMigration(t *testing.T) {
    tests := []struct {
        name       string
        query      Query
        useUnified bool
    }{
        {
            name: "metrics query - legacy",
            query: Query{
                Type:  "metrics",
                Query: "up",
            },
            useUnified: false,
        },
        {
            name: "metrics query - unified",
            query: Query{
                Type:  "metrics",
                Query: "up",
            },
            useUnified: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var result *Result
            var err error
            
            if tt.useUnified {
                result, err = unifiedClient.ExecuteQuery(ctx, tt.query)
            } else {
                result, err = legacyClient.ExecuteQuery(ctx, tt.query)
            }
            
            assert.NoError(t, err)
            assert.NotNil(t, result)
            
            // Validate response format
            assert.NotEmpty(t, result.Data)
        })
    }
}
```

## Query Translation Examples

### Metrics Queries

#### Instant Query

**Before (Engine-Specific):**
```bash
curl -X POST https://mirador-core/api/v1/metrics/query \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": "http_requests_total{job=\"api\"}",
    "time": "2025-01-01T00:00:00Z"
  }'
```

**After (Unified):**
```bash
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": {
      "type": "metrics",
      "query": "http_requests_total{job=\"api\"}",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T00:00:00Z"
    }
  }'
```

#### Range Query

**Before:**
```bash
curl -X POST https://mirador-core/api/v1/metrics/query_range \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": "rate(http_requests_total[5m])",
    "start": "2025-01-01T00:00:00Z",
    "end": "2025-01-01T01:00:00Z",
    "step": "1m"
  }'
```

**After:**
```bash
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": {
      "type": "metrics",
      "query": "rate(http_requests_total[5m])",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z",
      "cache_options": {
        "enabled": true,
        "ttl": "5m"
      }
    }
  }'
```

### Logs Queries

#### Search Query

**Before:**
```bash
curl -X POST https://mirador-core/api/v1/logs/search \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query_language": "lucene",
    "query": "_time:15m level:error service:api",
    "limit": 100
  }'
```

**After:**
```bash
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": {
      "type": "logs",
      "query": "_time:15m level:error service:api",
      "timeout": "30s"
    }
  }'
```

### Traces Queries

#### Trace Search

**Before:**
```bash
curl -X POST https://mirador-core/api/v1/traces/search \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query_language": "lucene",
    "query": "_time:15m service:checkout",
    "limit": 100
  }'
```

**After:**
```bash
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": {
      "type": "traces",
      "query": "_time:15m service:checkout",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z"
    }
  }'
```

### Correlation Queries (NEW)

**New Capability - Time-Window Correlation:**
```bash
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": {
      "type": "correlation",
      "query": "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z"
    }
  }'
```

**New Capability - Label-Based Correlation:**
```bash
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": {
      "type": "correlation",
      "query": "logs:service:checkout error AND traces:service:checkout AND metrics:checkout_errors > 0"
    }
  }'
```

## Common Patterns

### Pattern 1: Dashboard Queries

**Before - Multiple API Calls:**
```go
// Fetch metrics
metricsResp, _ := metricsClient.Query(ctx, "http_requests_total")

// Fetch logs
logsResp, _ := logsClient.Search(ctx, "level:error")

// Fetch traces
tracesResp, _ := tracesClient.Search(ctx, "service:api")

// Manually correlate in application code
dashboard := correlateDashboardData(metricsResp, logsResp, tracesResp)
```

**After - Unified Query:**
```go
// Single unified query with correlation
unifiedResp, _ := unifiedClient.ExecuteQuery(ctx, UnifiedQuery{
    Type:  "correlation",
    Query: "metrics:http_requests_total AND logs:level:error AND traces:service:api",
})

dashboard := buildDashboard(unifiedResp)
```

### Pattern 2: Incident Investigation

**Before:**
```go
// Step 1: Find error logs
errors := logsClient.Search(ctx, "level:error")

// Step 2: Check related metrics (manual correlation)
for _, err := range errors {
    metrics := metricsClient.QueryRange(ctx, fmt.Sprintf(
        "cpu_usage{service=\"%s\"}", err.Service,
    ), err.Timestamp.Add(-5*time.Minute), err.Timestamp.Add(5*time.Minute))
    
    // Step 3: Find related traces
    traces := tracesClient.Search(ctx, fmt.Sprintf(
        "service:%s", err.Service,
    ))
}
```

**After:**
```go
// Single correlation query
investigation := unifiedClient.ExecuteQuery(ctx, UnifiedQuery{
    Type:  "correlation",
    Query: "logs:error WITHIN 10m OF metrics:cpu_usage > 80 AND traces:status:error",
})

// Results include confidence scores and automatic time-window correlation
```

### Pattern 3: Real-Time Monitoring

**Before:**
```go
// Poll multiple endpoints
ticker := time.NewTicker(10 * time.Second)
for range ticker.C {
    metrics := metricsClient.Query(ctx, "up")
    logs := logsClient.Search(ctx, "level:error")
    
    if detectAnomaly(metrics, logs) {
        alert()
    }
}
```

**After:**
```go
// Single unified query with intelligent caching
ticker := time.NewTicker(10 * time.Second)
for range ticker.C {
    result := unifiedClient.ExecuteQuery(ctx, UnifiedQuery{
        Type:  "correlation",
        Query: "metrics:up < 1 AND logs:error",
        CacheOptions: &CacheOptions{
            Enabled: true,
            TTL:     "30s",
        },
    })
    
    if result.HasAnomalies() {
        alert()
    }
}
```

## Testing Your Migration

### Validation Checklist

- [ ] Query results match between old and new APIs
- [ ] Performance metrics comparable or improved
- [ ] Error handling works correctly
- [ ] Caching behavior is appropriate
- [ ] Monitoring and alerting updated
- [ ] Documentation updated
- [ ] Team training completed

### Comparison Testing

Create comparison tests to validate migration:

```go
func TestQueryMigrationComparison(t *testing.T) {
    testCases := []struct {
        name      string
        query     string
        queryType string
    }{
        {"simple metrics", "up", "metrics"},
        {"range query", "rate(http_requests_total[5m])", "metrics"},
        {"logs search", "level:error", "logs"},
        {"trace search", "service:api", "traces"},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Execute with legacy API
            legacyResult, legacyErr := executeLegacyQuery(tc.query, tc.queryType)
            
            // Execute with unified API
            unifiedResult, unifiedErr := executeUnifiedQuery(tc.query, tc.queryType)
            
            // Compare results
            assert.Equal(t, legacyErr != nil, unifiedErr != nil, "Error status should match")
            
            if legacyErr == nil && unifiedErr == nil {
                assert.Equal(t, len(legacyResult.Data), len(unifiedResult.Data), "Result count should match")
                // Add more specific comparisons based on data type
            }
        })
    }
}
```

### Load Testing

Compare performance under load:

```bash
# Legacy API load test
hey -n 10000 -c 100 -m POST \
  -H "Authorization: Bearer <token>" \
  -D metrics_query.json \
  https://mirador-core/api/v1/metrics/query

# Unified API load test
hey -n 10000 -c 100 -m POST \
  -H "Authorization: Bearer <token>" \
  -D unified_query.json \
  https://mirador-core/api/v1/unified/query

# Compare results: p50, p95, p99 latencies, throughput, error rate
```

## Rollback Plan

### Preparation

1. **Keep Legacy Code**: Don't delete old API client code until migration is complete
2. **Feature Flags**: Implement feature flags for easy rollback
3. **Monitoring**: Set up alerts for migration issues
4. **Backups**: Ensure configuration backups are available

### Rollback Procedure

If issues occur, rollback using feature flags:

```bash
# Disable unified API via environment variable
kubectl set env deployment/myapp USE_UNIFIED_API=false

# Or via ConfigMap
kubectl patch configmap myapp-config -p '{"data":{"use_unified_api":"false"}}'

# Restart pods to pick up changes
kubectl rollout restart deployment/myapp
```

### Gradual Rollback

Roll back percentage of traffic:

```bash
# Reduce unified API traffic to 0%
kubectl set env deployment/myapp UNIFIED_API_ROLLOUT=0

# Monitor for issues, then increase gradually
kubectl set env deployment/myapp UNIFIED_API_ROLLOUT=25
```

## Performance Considerations

### Expected Performance Improvements

**Unified API Performance:**
- ✅ Intelligent caching: 40-60% reduction in backend queries
- ✅ Query optimization: 15-30% faster execution
- ✅ Reduced client overhead: Single connection pool
- ✅ Better resource utilization: Connection reuse

**When Performance May Differ:**
- ⚠️ First-time queries (cache warming)
- ⚠️ Complex correlation queries (new capability, no baseline)
- ⚠️ Large result sets (unified format may differ slightly)

### Optimization Tips

1. **Enable Caching**
```json
{
  "query": {
    "cache_options": {
      "enabled": true,
      "ttl": "5m"
    }
  }
}
```

2. **Use Appropriate Timeouts**
```json
{
  "query": {
    "timeout": "30s"  // Match your query complexity
  }
}
```

3. **Leverage Correlation**
```json
{
  "query": {
    "type": "correlation",
    "query": "logs:error WITHIN 5m OF metrics:cpu > 80"
  }
}
```

## Troubleshooting

### Common Issues

#### Issue 1: Different Result Format

**Problem**: Unified API returns results in different structure

**Solution**: Update response parsing logic
```go
// Old format
type LegacyMetricsResponse struct {
    Status string           `json:"status"`
    Data   MetricsResultSet `json:"data"`
}

// New format
type UnifiedResponse struct {
    QueryID         string      `json:"query_id"`
    Status          string      `json:"status"`
    Data            interface{} `json:"data"`
    ExecutionTimeMs int64       `json:"execution_time_ms"`
}
```

#### Issue 2: Authentication Errors

**Problem**: Authentication fails with unified API

**Solution**: Ensure token includes unified API permissions
```bash
# Check token permissions
curl -H "Authorization: Bearer <token>" \
  https://mirador-core/api/v1/unified/metadata
```

#### Issue 3: Query Syntax Errors

**Problem**: Queries that worked with engine-specific APIs fail with unified API

**Solution**: Review query type specification
```json
{
  "query": {
    "type": "metrics",  // Must explicitly specify type
    "query": "http_requests_total"
  }
}
```

#### Issue 4: Slower Performance

**Problem**: Unified queries slower than engine-specific queries

**Solution**: 
1. Enable caching with appropriate TTL
2. Check query timeout settings
3. Monitor backend engine health
4. Review query complexity

```bash
# Check unified query health
curl https://mirador-core/api/v1/unified/health

# Check query metadata and capabilities
curl https://mirador-core/api/v1/unified/metadata
```

### Getting Help

- **Documentation**: https://miradorstack.readthedocs.io/
- **API Reference**: [API Reference](api-reference.md)
- **Community**: https://github.com/platformbuilds/mirador-core/discussions
- **Support**: Open an issue on GitHub

## Weaviate KPI Management

This section covers the process for evolving and managing Weaviate schemas in mirador-core, including adding new classes, modifying existing ones, and handling schema migrations safely.

### Schema Architecture Overview

Mirador-core uses Weaviate as its vector database backend for storing API entities like KPIDefinition, KPILayout, Dashboard, and UserPreferences. The schema is defined in Go code and deployed automatically through the `EnsureSchema()` function.

**Key Files:**
- `internal/storage/weaviate/schema.go` - Schema class definitions
- `internal/repo/schema_weaviate.go` - Schema deployment logic

### Schema Change Process

#### 1. Schema Definition Changes

**Location**: `internal/storage/weaviate/schema.go`

Modify the `GetAllClasses()` function to add new classes or update existing class definitions:

```go
func GetAllClasses() []models.Class {
    return []models.Class{
        // Existing classes...
        {"KPIDefinition", class("KPIDefinition", props(
            text("id"), text("kind"), text("name"), text("unit"), text("format"),
            object("query"), object("thresholds"), stringArray("tags"), object("sparkline"),
            text("ownerUserId"), text("visibility"), date("createdAt"), date("updatedAt"),
        ))},
        
        // Add new class
        {"NewEntity", class("NewEntity", props(
            text("id"), text("name"), text("description"),
            date("createdAt"), date("updatedAt"),
        ))},
    }
}
```

#### 2. Deployment Logic Updates

**Location**: `internal/repo/schema_weaviate.go`

Update the `EnsureSchema()` function to include new classes in the deployment:

```go
func (r *WeaviateRepo) EnsureSchema(ctx context.Context) error {
    classDefinitions := []struct {
        name string
        def  map[string]any
    }{
        // Existing classes...
        {"KPIDefinition", class("KPIDefinition", props(/* ... */))},
        
        // Add new class definition
        {"NewEntity", class("NewEntity", props(
            text("id"), text("name"), text("description"),
            date("createdAt"), date("updatedAt"),
        ))},
    }
    
    // Create classes individually
    for _, classDef := range classDefinitions {
        if err := r.ensureClass(ctx, classDef.name, classDef.def); err != nil {
            return fmt.Errorf("failed to create class %s: %w", classDef.name, err)
        }
    }
    return nil
}
```

#### 3. Migration Strategies

Since Weaviate doesn't support altering existing classes, use these strategies for schema evolution:

##### Option A: New Classes (Recommended)

Create new class versions alongside existing ones:

```go
// Add versioned class
{"KPIDefinition_v2", class("KPIDefinition_v2", props(
    text("id"), text("kind"), text("name"), text("unit"), text("format"),
    object("query"), object("thresholds"), stringArray("tags"), object("sparkline"),
    text("ownerUserId"), text("visibility"), 
    text("newProperty"), // New field
    date("createdAt"), date("updatedAt"),
))},
```

##### Option B: Property Extensions

Add new optional properties to existing classes (Weaviate allows this):

```go
// Extend existing class with new optional property
{"KPIDefinition", class("KPIDefinition", props(
    text("id"), text("kind"), text("name"), text("unit"), text("format"),
    object("query"), object("thresholds"), stringArray("tags"), object("sparkline"),
    text("ownerUserId"), text("visibility"), 
    text("newOptionalProperty"), // New optional property
    date("createdAt"), date("updatedAt"),
))},
```

##### Option C: Data Migration

For breaking changes, create migration scripts:

```go
// Migration function to copy data from old to new class
func (r *WeaviateRepo) MigrateKPIDefinitionToV2(ctx context.Context) error {
    // 1. Query all existing KPIDefinition objects
    // 2. Transform data to new schema format
    // 3. Create new objects in KPIDefinition_v2 class
    // 4. Optionally mark old objects as migrated
    // 5. Update application code to use new class
}
```

### Safety Features

The schema deployment system includes built-in safety measures:

#### Existence Checking

The `ensureClass()` function automatically checks if a class exists before attempting creation:

```go
func (r *WeaviateRepo) ensureClass(ctx context.Context, className string, classDef map[string]any) error {
    exists, err := r.classExists(ctx, className)
    if err != nil {
        return fmt.Errorf("failed to check if class %s exists: %w", className, err)
    }
    
    if exists {
        // Class already exists, skip creation (safe!)
        return nil
    }
    
    // Create the class
    return r.t.EnsureClasses(ctx, []map[string]any{classDef})
}
```

#### Idempotent Operations

Schema deployment is **idempotent** - it can be run multiple times safely without side effects.

### Testing Schema Changes

#### Unit Tests

Test schema definitions and deployment:

```go
func TestSchemaDefinitions(t *testing.T) {
    classes := GetAllClasses()
    
    // Verify expected classes exist
    classNames := make([]string, len(classes))
    for i, c := range classes {
        classNames[i] = c.Class
    }
    
    assert.Contains(t, classNames, "KPIDefinition")
    assert.Contains(t, classNames, "Dashboard")
    assert.Contains(t, classNames, "UserPreferences")
}

func TestEnsureSchema(t *testing.T) {
    repo := setupTestRepo(t)
    
    // Should not error even if schema already exists
    err := repo.EnsureSchema(context.Background())
    assert.NoError(t, err)
    
    // Verify classes were created
    exists, err := repo.classExists(context.Background(), "KPIDefinition")
    assert.NoError(t, err)
    assert.True(t, exists)
}
```

#### Integration Tests

Test with actual Weaviate instance:

```go
func TestSchemaDeployment(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    repo := setupWeaviateRepo(t)
    
    // Deploy schema
    err := repo.EnsureSchema(context.Background())
    assert.NoError(t, err)
    
    // Verify all expected classes exist
    expectedClasses := []string{
        "KPIDefinition", "KPILayout", "Dashboard", "UserPreferences",
    }
    
    for _, className := range expectedClasses {
        exists, err := repo.classExists(context.Background(), className)
        assert.NoError(t, err, "Failed to check class %s", className)
        assert.True(t, exists, "Class %s should exist", className)
    }
}
```

### Deployment Checklist

Before deploying schema changes:

- [ ] Schema definitions updated in `schema.go`
- [ ] Deployment logic updated in `schema_weaviate.go`
- [ ] Unit tests pass
- [ ] Integration tests pass with test Weaviate instance
- [ ] Migration strategy documented
- [ ] Rollback plan prepared
- [ ] Data backup taken (if migrating existing data)
- [ ] Application code updated to handle new schema
- [ ] API contract documentation updated

### Rollback Procedures

#### For New Classes

Simply remove the class definition from code and redeploy:

```go
// Remove from GetAllClasses()
func GetAllClasses() []models.Class {
    return []models.Class{
        // Remove NewEntity class definition
        // {"NewEntity", class("NewEntity", props(...))}, // REMOVED
    }
}
```

#### For Property Extensions

New optional properties can be safely removed (Weaviate allows this).

#### For Breaking Changes

If migration is needed, implement rollback migration:

```go
func (r *WeaviateRepo) RollbackKPIDefinitionV2(ctx context.Context) error {
    // Migrate data back to original schema
    // Remove v2 objects
    // Restore original application logic
}
```

### Monitoring Schema Health

Monitor schema deployment and health:

```go
// Check schema health
func (r *WeaviateRepo) GetSchemaHealth(ctx context.Context) (*SchemaHealth, error) {
    // Verify all expected classes exist
    // Check class properties match expectations
    // Report any inconsistencies
}
```

### Best Practices

1. **Version Class Names**: Use semantic versioning for class names when breaking changes are needed
2. **Test Thoroughly**: Always test schema changes against a test Weaviate instance
3. **Document Changes**: Update API contract documentation for any schema changes
4. **Plan Migrations**: Have a clear migration and rollback strategy
5. **Monitor Deployments**: Watch for schema deployment errors in production
6. **Backup Data**: Ensure data is backed up before major schema changes
7. **Gradual Rollout**: Test schema changes in staging before production deployment

### Common Schema Issues

#### Issue: Class Already Exists

**Problem**: Schema deployment fails because class exists with different definition

**Solution**: Weaviate doesn't allow altering existing classes. Create a new version:

```go
// Instead of modifying existing class
{"KPIDefinition", class("KPIDefinition", props(/* modified */))},

// Create new version
{"KPIDefinition_v2", class("KPIDefinition_v2", props(/* new definition */))},
```

#### Issue: Property Type Mismatch

**Problem**: Attempting to change property data type

**Solution**: Create new class version with correct types. Weaviate property types cannot be changed.

#### Issue: Schema Deployment Timeout

**Problem**: Large schemas take too long to deploy

**Solution**: 
- Deploy classes individually with proper error handling
- Increase timeout settings
- Monitor deployment progress

### Getting Help

- **Schema Documentation**: Check Weaviate official documentation
- **Community**: GitHub Discussions for mirador-core
- **API Contract**: Review `dev/MIRADOR-CORE-API-CONTRACT.md`

## Next Steps

After completing migration:

1. **Explore New Capabilities**: Try correlation queries to gain cross-domain insights
2. **Optimize Caching**: Tune cache TTLs based on your workload
3. **Monitor Performance**: Track unified query metrics in your dashboards
4. **Cleanup**: Remove legacy code paths and dependencies
5. **Documentation**: Update internal documentation and runbooks
6. **Training**: Educate team on new unified query patterns

## Conclusion

Migrating to the unified query API provides significant benefits in terms of simplicity, performance, and new capabilities. By following this guide's gradual migration approach, you can safely transition your applications while maintaining backward compatibility and the ability to rollback if needed.

For specific migration scenarios not covered in this guide, please consult the [API Reference](api-reference.md) or reach out to the community for assistance.
