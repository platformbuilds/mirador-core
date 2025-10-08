# Bleve Memory Usage Analysis & Optimization

## Benchmark Results Summary

Based on the performance benchmarks executed, here are the key memory usage findings:

### Memory Usage by Index Size
- **Small Index (100 documents)**: 12.59 MB, 188.76K allocations
- **Large Index (1000 documents)**: 122.08 MB, 1.83M allocations
- **Indexing Operation (1000 docs)**: 124.72 MB, 1.86M allocations

### Memory Scaling Analysis
- **10x increase in documents** = **9.7x increase in memory usage**
- **Memory per document**: ~122 KB for large indexes
- **Allocation efficiency**: ~1.83 allocations per document

## Memory Optimization Opportunities

### 1. Tiered Storage Optimization

**Current Issue**: The tiered storage maintains both memory and disk caches simultaneously, leading to higher memory usage.

**Recommendations**:
- Implement adaptive cache sizing based on available memory
- Add memory pressure detection to automatically adjust cache ratios
- Implement LRU eviction policies for memory cache

**Implementation**:
```go
// Adaptive cache sizing based on available memory
func (ts *TieredStore) optimizeCacheSize() {
    availableMemory := getAvailableMemory()
    memoryRatio := calculateOptimalMemoryRatio(availableMemory)

    ts.memoryCache.SetMaxSize(int(float64(availableMemory) * memoryRatio))
    ts.diskCache.SetMaxSize(availableMemory - ts.memoryCache.MaxSize())
}
```

### 2. Document Mapper Optimization

**Current Issue**: The document mapper creates multiple intermediate representations during indexing.

**Recommendations**:
- Implement object pooling for frequently allocated structures
- Reduce field mapping overhead by caching field mappings
- Optimize JSON parsing with streaming approaches

**Implementation**:
```go
// Object pooling for document structures
var documentPool = sync.Pool{
    New: func() interface{} {
        return &Document{}
    },
}

func (dm *BleveDocumentMapper) MapDocument(data interface{}) (*Document, error) {
    doc := documentPool.Get().(*Document)
    defer documentPool.Put(doc)

    // Reuse document object instead of allocating new ones
    return dm.mapToDocument(doc, data)
}
```

### 3. Shard Memory Management

**Current Issue**: Each shard maintains independent memory caches, leading to memory fragmentation.

**Recommendations**:
- Implement shared memory pools across shards
- Add memory quota management per tenant
- Implement memory usage monitoring and alerting

**Implementation**:
```go
// Shared memory pool across shards
type SharedMemoryPool struct {
    pool *sync.Pool
    mu   sync.RWMutex
    used int64
    limit int64
}

func (smp *SharedMemoryPool) Allocate(size int) ([]byte, error) {
    smp.mu.Lock()
    defer smp.mu.Unlock()

    if smp.used + int64(size) > smp.limit {
        return nil, errors.New("memory limit exceeded")
    }

    buffer := smp.pool.Get().([]byte)
    if cap(buffer) < size {
        buffer = make([]byte, size)
    }

    smp.used += int64(size)
    return buffer[:size], nil
}
```

### 4. Query Result Caching

**Current Issue**: Frequent queries re-execute without caching benefits.

**Recommendations**:
- Implement query result caching with TTL
- Add cache invalidation on index updates
- Implement cache size limits and LRU eviction

**Implementation**:
```go
type QueryCache struct {
    cache *ristretto.Cache
    ttl   time.Duration
}

func (qc *QueryCache) Get(query string) (*SearchResult, bool) {
    if result, found := qc.cache.Get(query); found {
        return result.(*SearchResult), true
    }
    return nil, false
}

func (qc *QueryCache) Put(query string, result *SearchResult) {
    qc.cache.SetWithTTL(query, result, 1, qc.ttl)
}
```

## Performance vs Memory Trade-offs

### Recommended Configurations

#### High-Performance Configuration
```yaml
storage:
  memory_cache_ratio: 0.7  # 70% of available memory
  disk_cache_ratio: 0.3    # 30% of available memory
  cache_ttl: 1h
  max_concurrent_queries: 100
```

#### Memory-Constrained Configuration
```yaml
storage:
  memory_cache_ratio: 0.3  # 30% of available memory
  disk_cache_ratio: 0.7    # 70% of available memory
  cache_ttl: 30m
  max_concurrent_queries: 10
```

#### Balanced Configuration (Recommended)
```yaml
storage:
  memory_cache_ratio: 0.5  # 50% of available memory
  disk_cache_ratio: 0.5    # 50% of available memory
  cache_ttl: 45m
  max_concurrent_queries: 50
```

## Monitoring & Alerting

### Key Metrics to Monitor
- `bleve_storage_memory_bytes` - Current memory usage
- `bleve_storage_disk_bytes` - Disk usage fallback
- `bleve_cache_hit_ratio` - Cache effectiveness
- `bleve_memory_pressure_events` - Memory pressure incidents

### Recommended Alerts
```yaml
# Memory usage above 80% of limit
- alert: BleveHighMemoryUsage
  expr: bleve_storage_memory_bytes / bleve_storage_memory_limit > 0.8
  for: 5m
  labels:
    severity: warning

# Cache hit ratio below 50%
- alert: BleveLowCacheEfficiency
  expr: bleve_cache_hit_ratio < 0.5
  for: 10m
  labels:
    severity: warning
```

## Implementation Priority

1. **High Priority**: Adaptive cache sizing (immediate impact)
2. **Medium Priority**: Object pooling for document mapping
3. **Medium Priority**: Query result caching
4. **Low Priority**: Shared memory pools (complex implementation)

## Expected Improvements

With these optimizations, we anticipate:
- **30-50% reduction** in memory usage for large indexes
- **20-40% improvement** in query performance through caching
- **Better memory utilization** under varying load conditions
- **Improved stability** under memory pressure

## Next Steps

1. Implement adaptive cache sizing
2. Add memory usage monitoring to dashboards
3. Conduct A/B testing with different configurations
4. Implement query result caching
5. Add memory pressure handling