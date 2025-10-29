# Metrics Metadata Integration Documentation

## Overview

The Metrics Metadata Integration is a Phase 2 feature of MIRADOR-CORE v7.0.0 that enables enhanced metrics discovery and exploration capabilities. This system automatically indexes metrics metadata from VictoriaMetrics into Bleve search indexes, providing fast, searchable access to metric names, descriptions, labels, and other metadata.

## Architecture

### Core Components

#### 1. MetricsMetadataIndexer
- **Purpose**: Extracts metrics metadata from VictoriaMetrics and indexes it into Bleve
- **Location**: `internal/services/metrics_metadata_indexer.go`
- **Key Features**:
  - Extracts metric series data from VictoriaMetrics `/api/v1/series` endpoint
  - Transforms metrics into searchable documents with labels, descriptions, and metadata
  - Supports batch processing for large metric volumes
  - Integrates with existing ShardManager for distributed indexing

#### 2. MetricsMetadataSynchronizer
- **Purpose**: Maintains synchronization between VictoriaMetrics and Bleve indexes
- **Location**: `internal/services/metrics_metadata_synchronizer.go`
- **Key Features**:
  - **Periodic Sync**: Configurable intervals (default: 15 minutes)
  - **Incremental Sync**: Updates only recent changes (lookback: 1 hour)
  - **Hybrid Sync**: Combines full and incremental strategies
  - **Full Sync**: Complete refresh (daily by default)
  - **Retry Logic**: Exponential backoff with configurable retries
  - **Multi-tenant Support**: Per-tenant synchronization states

#### 3. MetricsSearchHandler
- **Purpose**: Provides HTTP API endpoints for metrics discovery
- **Location**: `internal/api/handlers/metrics_search.go`
- **Key Features**:
  - Search metrics by name, labels, or description
  - Health check endpoints for sync status
  - Integration with existing authentication and middleware

#### 4. MetricsSyncHandler
- **Purpose**: Provides HTTP API endpoints for synchronization management
- **Location**: `internal/api/handlers/metrics_sync.go`
- **Key Features**:
  - Trigger immediate sync operations
  - Monitor sync state and status
  - Update synchronization configuration

### Data Models

#### MetricMetadataDocument
```go
type MetricMetadataDocument struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Type        string            `json:"type"`
    Help        string            `json:"help"`
    Unit        string            `json:"unit"`
    Labels      map[string]string `json:"labels"`
    LabelKeys   []string          `json:"label_keys"`
    TenantID    string            `json:"tenant_id"`
    LastSeen    time.Time         `json:"last_seen"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
}
```

#### Synchronization Models

##### MetricMetadataSyncConfig
```go
type MetricMetadataSyncConfig struct {
    Enabled           bool          `json:"enabled"`
    Strategy          SyncStrategy  `json:"strategy"`
    Interval          time.Duration `json:"interval"`
    FullSyncInterval  time.Duration `json:"full_sync_interval"`
    BatchSize         int           `json:"batch_size"`
    MaxRetries        int           `json:"max_retries"`
    RetryDelay        time.Duration `json:"retry_delay"`
    TimeRangeLookback time.Duration `json:"time_range_lookback"`
}
```

##### MetricMetadataSyncState
```go
type MetricMetadataSyncState struct {
    TenantID         string    `json:"tenant_id"`
    LastSyncTime     time.Time `json:"last_sync_time"`
    LastFullSyncTime time.Time `json:"last_full_sync_time"`
    TotalSyncs       int64     `json:"total_syncs"`
    SuccessfulSyncs  int64     `json:"successful_syncs"`
    FailedSyncs      int64     `json:"failed_syncs"`
    MetricsInIndex   int64     `json:"metrics_in_index"`
    IsCurrentlySyncing bool    `json:"is_currently_syncing"`
    LastError        string    `json:"last_error,omitempty"`
    LastErrorTime    *time.Time `json:"last_error_time,omitempty"`
}
```

##### MetricMetadataSyncStatus
```go
type MetricMetadataSyncStatus struct {
    TenantID         string        `json:"tenant_id"`
    Status           string        `json:"status"`
    StartTime        time.Time     `json:"start_time"`
    EndTime          time.Time     `json:"end_time,omitempty"`
    Duration         time.Duration `json:"duration,omitempty"`
    Strategy         SyncStrategy  `json:"strategy"`
    MetricsProcessed int           `json:"metrics_processed"`
    MetricsAdded     int           `json:"metrics_added"`
    MetricsUpdated   int           `json:"metrics_updated"`
    MetricsRemoved   int           `json:"metrics_removed"`
    Errors           []string      `json:"errors,omitempty"`
}
```

## Synchronization Strategies

### 1. Full Sync Strategy
- **Description**: Complete refresh of all metrics metadata
- **Use Case**: Initial sync, periodic full refresh
- **Frequency**: Configurable (default: daily)
- **Performance**: High resource usage, comprehensive coverage

### 2. Incremental Sync Strategy
- **Description**: Updates only recently changed metrics
- **Use Case**: Frequent updates with minimal overhead
- **Frequency**: Configurable (default: 15 minutes)
- **Performance**: Low resource usage, partial coverage

### 3. Hybrid Sync Strategy (Default)
- **Description**: Combines full and incremental syncs
- **Use Case**: Balanced approach for most deployments
- **Frequency**: Incremental + periodic full sync
- **Performance**: Optimal resource usage with comprehensive coverage

## API Endpoints

### Metrics Discovery API

#### Search Metrics
```http
POST /api/v1/metrics/search
Content-Type: application/json

{
  "query": "cpu_usage",
  "tenant_id": "default",
  "limit": 50,
  "offset": 0
}
```

**Response:**
```json
{
  "results": [
    {
      "id": "cpu_usage_total",
      "name": "cpu_usage_total",
      "type": "counter",
      "help": "Total CPU usage",
      "unit": "seconds",
      "labels": {
        "instance": "localhost:9090",
        "job": "node"
      },
      "label_keys": ["instance", "job"],
      "tenant_id": "default",
      "last_seen": "2025-10-25T10:00:00Z"
    }
  ],
  "total": 1,
  "took": "15ms"
}
```

#### Sync Metrics
```http
POST /api/v1/metrics/sync
Content-Type: application/json

{
  "tenant_id": "default",
  "force_full_sync": false
}
```

#### Health Check
```http
GET /api/v1/metrics/health
```

### Synchronization Management API

#### Trigger Immediate Sync
```http
POST /api/v1/metrics/sync/{tenantId}?forceFull=false
```

**Response:**
```json
{
  "message": "Sync completed successfully",
  "result": {
    "metrics_processed": 150,
    "metrics_added": 5,
    "metrics_updated": 10,
    "metrics_removed": 2,
    "duration": "2.5s"
  }
}
```

#### Get Sync State
```http
GET /api/v1/metrics/sync/{tenantId}/state
```

**Response:**
```json
{
  "tenant_id": "default",
  "last_sync_time": "2025-10-25T10:15:00Z",
  "last_full_sync_time": "2025-10-25T06:00:00Z",
  "total_syncs": 96,
  "successful_syncs": 94,
  "failed_syncs": 2,
  "metrics_in_index": 1250,
  "is_currently_syncing": false
}
```

#### Get Sync Status
```http
GET /api/v1/metrics/sync/{tenantId}/status
```

**Response:**
```json
{
  "tenant_id": "default",
  "status": "completed",
  "start_time": "2025-10-25T10:15:00Z",
  "end_time": "2025-10-25T10:17:30Z",
  "duration": "2m30s",
  "strategy": "incremental",
  "metrics_processed": 150,
  "metrics_added": 5,
  "metrics_updated": 10,
  "metrics_removed": 2
}
```

#### Update Sync Configuration
```http
PUT /api/v1/metrics/sync/config
Content-Type: application/json

{
  "enabled": true,
  "strategy": "hybrid",
  "interval": "900000000000",
  "full_sync_interval": "86400000000000",
  "batch_size": 1000,
  "max_retries": 3,
  "retry_delay": "30000000000",
  "time_range_lookback": "3600000000000"
}
```

## Configuration

### Default Configuration
```yaml
# Server configuration
metrics_metadata_sync:
  enabled: true
  strategy: hybrid
  interval: 15m
  full_sync_interval: 24h
  batch_size: 1000
  max_retries: 3
  retry_delay: 30s
  time_range_lookback: 1h
```

### Environment Variables
- `METRICS_METADATA_SYNC_ENABLED`: Enable/disable synchronization (default: true)
- `METRICS_METADATA_SYNC_STRATEGY`: Sync strategy (full, incremental, hybrid)
- `METRICS_METADATA_SYNC_INTERVAL`: Sync interval (default: 15m)
- `METRICS_METADATA_SYNC_FULL_INTERVAL`: Full sync interval (default: 24h)
- `METRICS_METADATA_SYNC_BATCH_SIZE`: Batch size for processing (default: 1000)
- `METRICS_METADATA_SYNC_MAX_RETRIES`: Maximum retry attempts (default: 3)
- `METRICS_METADATA_SYNC_RETRY_DELAY`: Delay between retries (default: 30s)
- `METRICS_METADATA_SYNC_LOOKBACK`: Time range lookback for incremental sync (default: 1h)

## Integration with Existing Systems

### VictoriaMetrics Integration
- Uses existing VictoriaMetrics client from `internal/services/victoria_metrics.go`
- Leverages `/api/v1/series` endpoint for metric discovery
- Supports tenant-specific metric isolation
- Handles VictoriaMetrics authentication and connection pooling

### Bleve Search Integration
- Integrates with existing ShardManager for distributed indexing
- Uses Bleve's full-text search capabilities for metric discovery
- Supports tenant-specific index isolation
- Leverages existing search infrastructure and caching

### Valkey Caching Integration
- Uses Valkey cluster for sync state persistence
- Caches sync states across server restarts
- Supports distributed state management
- Integrates with existing caching infrastructure

### Server Lifecycle Integration
- Automatic start/stop with server lifecycle
- Graceful shutdown handling
- Resource cleanup on termination
- Integration with existing health checks and monitoring

## Monitoring and Observability

### Metrics
- `mirador_metrics_metadata_sync_duration`: Sync operation duration
- `mirador_metrics_metadata_sync_total`: Total sync operations
- `mirador_metrics_metadata_sync_success`: Successful sync operations
- `mirador_metrics_metadata_sync_errors`: Sync operation errors
- `mirador_metrics_metadata_indexed`: Number of metrics in index

### Health Checks
- Sync service health status
- Last sync time monitoring
- Error rate tracking
- Index consistency validation

### Logging
- Sync operation start/completion events
- Error details and retry attempts
- Performance metrics and timing
- Configuration changes

## Best Practices

### Configuration
1. **Choose appropriate sync strategy**:
   - Use `hybrid` for most production environments
   - Use `incremental` for high-frequency updates
   - Use `full` for small datasets or when consistency is critical

2. **Tune sync intervals**:
   - Balance freshness requirements with resource usage
   - Consider metric update frequency in your environment
   - Monitor sync performance and adjust accordingly

3. **Configure batch sizes**:
   - Larger batches improve throughput but increase memory usage
   - Smaller batches reduce memory pressure but may slow sync operations
   - Test with your specific workload to find optimal size

### Operations
1. **Monitor sync health**:
   - Regularly check sync status and error rates
   - Set up alerts for sync failures
   - Monitor index consistency

2. **Handle tenant isolation**:
   - Ensure proper tenant ID configuration
   - Monitor per-tenant sync performance
   - Implement tenant-specific sync policies if needed

3. **Performance optimization**:
   - Schedule full syncs during low-traffic periods
   - Monitor VictoriaMetrics and Bleve resource usage
   - Consider horizontal scaling for high-volume environments

## Troubleshooting

### Common Issues

#### Sync Operations Failing
**Symptoms**: Sync status shows "failed", errors in logs
**Causes**:
- VictoriaMetrics connection issues
- Bleve indexing problems
- Insufficient permissions
- Network timeouts

**Solutions**:
1. Check VictoriaMetrics connectivity
2. Verify Bleve index health
3. Review authentication configuration
4. Increase timeout values
5. Check resource availability

#### Outdated Metrics in Search
**Symptoms**: Search results don't reflect recent metric changes
**Causes**:
- Sync interval too long
- Sync operations failing silently
- Index corruption

**Solutions**:
1. Reduce sync interval
2. Check sync status and logs
3. Trigger manual sync
4. Rebuild index if corrupted

#### High Memory Usage
**Symptoms**: Memory consumption spikes during sync
**Causes**:
- Large batch sizes
- Full sync operations
- Memory leaks in processing

**Solutions**:
1. Reduce batch size
2. Schedule full syncs during maintenance windows
3. Monitor and optimize memory usage
4. Consider horizontal scaling

#### Slow Sync Performance
**Symptoms**: Sync operations take too long
**Causes**:
- Large number of metrics
- Network latency
- Resource constraints
- Inefficient queries

**Solutions**:
1. Optimize VictoriaMetrics queries
2. Increase batch sizes appropriately
3. Scale infrastructure resources
4. Use incremental sync more frequently

### Debug Commands

#### Check Sync Status
```bash
curl -X GET "http://localhost:8080/api/v1/metrics/sync/default/status"
```

#### Trigger Manual Sync
```bash
curl -X POST "http://localhost:8080/api/v1/metrics/sync/default?forceFull=true"
```

#### Check Index Health
```bash
curl -X GET "http://localhost:8080/api/v1/metrics/health"
```

#### View Sync Logs
```bash
# Check application logs for sync-related entries
grep "metrics.*sync" /var/log/mirador-core.log
```

## Migration and Deployment

### Enabling Metrics Metadata Integration

1. **Update Configuration**:
   ```yaml
   # Add to your config.yaml
   metrics_metadata_sync:
     enabled: true
     strategy: hybrid
   ```

2. **Deploy Changes**:
   - Deploy the updated MIRADOR-CORE binary
   - The synchronizer will start automatically with server startup
   - Initial sync may take time depending on metric volume

3. **Verify Operation**:
   - Check sync status endpoints
   - Verify metrics appear in search results
   - Monitor logs for any errors

### Rolling Back

1. **Disable Synchronization**:
   ```yaml
   metrics_metadata_sync:
     enabled: false
   ```

2. **Stop Server Gracefully**:
   - Allow current sync operations to complete
   - Server will stop synchronizer during shutdown

3. **Clean Up (Optional)**:
   - Remove metrics metadata from Bleve indexes
   - Clear sync state from Valkey cache

## Future Enhancements

### Planned Features
- **Real-time Sync**: Event-driven synchronization via VictoriaMetrics webhooks
- **Advanced Filtering**: Custom sync rules based on metric patterns
- **Sync Scheduling**: Time-based scheduling with maintenance windows
- **Index Optimization**: Automatic index optimization and cleanup
- **Multi-region Support**: Cross-region synchronization capabilities

### Performance Improvements
- **Parallel Processing**: Concurrent sync operations for multiple tenants
- **Incremental Indexing**: More granular change detection
- **Compression**: Reduced network and storage overhead
- **Caching Optimization**: Smarter cache invalidation strategies

---

This documentation covers the complete Metrics Metadata Integration feature for MIRADOR-CORE v7.0.0 Phase 2. For additional support or questions, please refer to the main MIRADOR-CORE documentation or contact the platform team.