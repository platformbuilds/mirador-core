# Mirador Core v6.0.0 - Bleve Search Integration Implementation

## Overview

This document provides a comprehensive overview of the Bleve search integration implementation in Mirador Core v6.0.0. The project adds full Bleve search capabilities alongside existing Lucene functionality for logs and traces APIs, enabling users to choose between search engines while maintaining API compatibility.

## Table of Contents

1. [Project Objectives](#project-objectives)
2. [Architecture Overview](#architecture-overview)
3. [Design Decisions](#design-decisions)
4. [Implementation Details](#implementation-details)
5. [Key Components](#key-components)
6. [API Changes](#api-changes)
7. [Configuration](#configuration)
8. [Testing Strategy](#testing-strategy)
9. [Performance Considerations](#performance-considerations)
10. [Future Roadmap](#future-roadmap)
11. [Migration Guide](#migration-guide)

## Project Objectives

### Primary Goals
- **Parallel Search Engines**: Enable users to use the same APIs but switch between Lucene and Bleve search engines
- **API Compatibility**: Maintain backward compatibility with existing Lucene-based queries
- **Scalable Architecture**: Support both in-memory and disk-based indexes with cluster mode capabilities
- **Horizontal Scalability**: Implement distributed indexing with sharding and load balancing
- **Deep Metrics Monitoring**: Comprehensive monitoring and observability for search operations

### Success Criteria
- Zero regression in existing Lucene functionality
- Performance within 150% latency of Lucene for 95th percentile queries
- Linear scaling with added nodes up to 10-node cluster
- Memory usage below 2GB per instance at 100K documents per shard
- High availability with zero data loss during node failures

## Architecture Overview

### Core Components

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   API Layer     │    │  Search Router   │    │  Translators    │
│                 │    │                  │    │                 │
│ • Logs Handler  │◄──►│ • Engine         │◄──►│ • Lucene        │
│ • Traces Handler│    │   Selection      │    │ • Bleve         │
│ • Request/      │    │ • Query Routing  │    │                 │
│   Response      │    │                  │    │                 │
│   Models        │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌────────────────────┐
                    │ VictoriaMetrics    │
                    │ Backend            │
                    │ • LogsQL           │
                    │ • Traces API       │
                    └────────────────────┘
```

### Distributed Architecture (Future)

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Node 1        │    │   Node 2        │    │   Node 3        │
│ • Shard 1-3     │    │ • Shard 4-6     │    │ • Shard 7-9     │
│ • In-Memory     │    │ • In-Memory     │    │ • In-Memory     │
│ • Disk Storage  │    │ • Disk Storage  │    │ • Disk Storage  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌────────────────────┐
                    │   Valkey Cluster   │
                    │ • Metadata Store   │
                    │ • Coordination     │
                    │ • Query Cache      │
                    │ • Distributed Locks│
                    └────────────────────┘
```

## Design Decisions

### 1. API Design
- **Request Body Selection**: Search engine selection via `search_engine` field in request body rather than URL parameters
- **Backward Compatibility**: Default to Lucene when no engine specified
- **Unified Response Format**: Same response structure regardless of search engine used

### 2. Abstraction Layer
- **Translator Interface**: Common interface for all search engine translators
- **Search Router**: Central component for engine selection and query routing
- **Wrapper Pattern**: Used to resolve interface mismatches between different translator implementations

### 3. Query Translation Strategy
- **Direct Translation**: Convert Bleve queries to VictoriaMetrics LogsQL/Traces format
- **Feature Parity**: Support major query types (term, match, phrase, wildcard, numeric range, boolean)
- **Incremental Implementation**: Start with core query types, expand based on usage patterns

### 4. Configuration Approach
- **Feature Flags**: Enable/disable Bleve functionality globally or per-tenant
- **Flexible Configuration**: Support different configurations for different environments
- **Default Settings**: Sensible defaults with override capabilities

### 5. Storage Strategy
- **Tiered Storage**: In-memory cache backed by persistent disk storage
- **Consistent Hashing**: For shard distribution across cluster nodes
- **Metadata Store**: Valkey for cluster coordination and index metadata

## Implementation Details

### Phase 1: Core Architecture (COMPLETED)

#### 1. Search Router Implementation
**File**: `internal/utils/search/router.go`

```go
type Translator interface {
    TranslateToLogsQL(query string) (string, error)
    TranslateToTraces(query string) (map[string]interface{}, error)
}

type SearchRouter struct {
    luceneTranslator Translator
    bleveTranslator  Translator
}

func (r *SearchRouter) GetTranslator(engine string) (Translator, error) {
    switch engine {
    case "lucene":
        return r.luceneTranslator, nil
    case "bleve":
        return r.bleveTranslator, nil
    default:
        return r.luceneTranslator, nil // Default to Lucene
    }
}
```

#### 2. Bleve Translator Implementation
**File**: `internal/utils/bleve/translator.go`

Key Features:
- Query parsing using Bleve v2 QueryStringQuery
- Support for term, match, phrase, wildcard, and boolean queries
- Translation to VictoriaMetrics LogsQL format
- Error handling for unsupported query types

#### 3. Model Updates
**File**: `internal/models/queries.go`

Added `search_engine` field to:
- `LogsQLQueryRequest`
- `TraceSearchRequest`

```go
type LogsQLQueryRequest struct {
    Query       string `json:"query"`
    SearchEngine string `json:"search_engine,omitempty"` // "lucene" or "bleve"
    // ... other fields
}
```

#### 4. API Handler Updates

**Logs Handler** (`internal/api/handlers/logsql.handler.go`):
- Engine validation from request body
- Translator selection via search router
- Query translation and execution

**Traces Handler** (`internal/api/handlers/traces.handler.go`):
- Similar pattern to logs handler
- Trace filter extraction from translated queries

#### 5. Configuration Updates
**Files**: `internal/config/config.go`, `internal/config/defaults.go`

Added `SearchConfig` struct with:
- Default search engine settings
- Feature flag controls
- Engine-specific configurations

### Phase 2: Distributed Index Architecture (PLANNED)

#### Valkey Integration
- Metadata storage for index coordination
- Distributed locks for cluster operations
- Query result caching
- Cluster state management

#### Tiered Storage Strategy
- In-memory cache for frequently accessed data
- Disk-based persistent storage for full index
- Automatic data migration between tiers
- Configurable memory limits

#### Sharded Architecture
- Consistent hashing for shard distribution
- Horizontal scaling across multiple nodes
- Index rebalancing during scale events
- Fault tolerance and recovery mechanisms

## Key Components

### Search Router
**Purpose**: Central routing component for search engine selection
**Key Methods**:
- `GetTranslator(engine string)`: Returns appropriate translator
- `ValidateEngine(engine string)`: Validates engine selection
- Engine registration and management

### Bleve Translator
**Purpose**: Converts Bleve queries to VictoriaMetrics format
**Supported Query Types**:
- Term queries
- Match queries
- Phrase queries
- Wildcard queries
- Numeric range queries
- Boolean queries (AND, OR, NOT)

### API Handlers
**Logs Handler**: Processes LogsQL queries with engine selection
**Traces Handler**: Processes trace search requests with engine selection

Both handlers:
- Extract search engine from request body
- Validate engine selection
- Route to appropriate translator
- Handle translation errors gracefully

## API Changes

### Request Format Changes

#### Logs API
```json
POST /api/v1/logs/query
{
  "query": "error AND status:500",
  "search_engine": "bleve",
  "start": "2024-01-01T00:00:00Z",
  "end": "2024-01-02T00:00:00Z"
}
```

#### Traces API
```json
POST /api/v1/traces/search
{
  "query": "service.name:auth AND error",
  "search_engine": "bleve",
  "start": "2024-01-01T00:00:00Z",
  "end": "2024-01-02T00:00:00Z"
}
```

### Backward Compatibility
- Existing requests without `search_engine` default to Lucene
- Response format remains unchanged
- All existing Lucene queries continue to work

## Configuration

### Search Configuration Structure
```go
type SearchConfig struct {
    DefaultEngine    string `yaml:"default_engine"`
    EnableBleve      bool   `yaml:"enable_bleve"`
    BleveConfig      BleveConfig `yaml:",inline"`
}

type BleveConfig struct {
    IndexPath        string `yaml:"index_path"`
    BatchSize        int    `yaml:"batch_size"`
    MaxMemoryMB      int    `yaml:"max_memory_mb"`
}
```

### Default Configuration
```yaml
search:
  default_engine: "lucene"
  enable_bleve: true
  index_path: "/tmp/bleve"
  batch_size: 1000
  max_memory_mb: 512
```

## Testing Strategy

### Unit Tests
- **Translator Tests**: Comprehensive test suite for Bleve translator
- **Router Tests**: Engine selection and routing logic
- **Handler Tests**: API handler integration tests

### Integration Tests
- **API Endpoints**: End-to-end tests for both engines
- **Query Translation**: Validate query translation accuracy
- **Performance Tests**: Compare latency between engines

### Backward Compatibility Tests
- **Regression Tests**: Ensure existing Lucene functionality unchanged
- **Migration Tests**: Validate smooth transition between engines

## Performance Considerations

### Current Implementation
- **Memory Usage**: Minimal additional memory for core routing
- **Latency**: Translation overhead for Bleve queries
- **CPU Usage**: Query parsing and translation computation

### Future Optimizations
- **Query Caching**: Cache translated queries in Valkey
- **Index Optimization**: Memory/disk tier optimization
- **Parallel Processing**: Concurrent query processing across shards

### Monitoring Metrics
- Query translation time
- Engine selection distribution
- Error rates by engine
- Memory usage by component

## Future Roadmap

### Phase 2: Distributed Index Architecture
- [ ] Valkey integration for metadata
- [ ] Tiered storage implementation
- [ ] Sharded index architecture
- [ ] Cluster coordination mechanisms
- [ ] Index rebalancing and recovery

### Phase 3: Performance and Monitoring
- [ ] Prometheus metrics integration
- [ ] Grafana dashboard templates
- [ ] Query analysis tools
- [ ] Performance benchmarks
- [ ] Memory optimization

### Phase 4: User Experience
- [x] API documentation updates
- [x] Postman collection creation
- [x] Operations documentation
- [x] Grafana plugin development (handled in separate [miradorstack-grafana-plugin](https://github.com/platformbuilds/miradorstack-grafana-plugin) repository)

### Phase 5: Production Readiness
- [ ] Load testing
- [ ] Security review
- [ ] A/B testing capabilities
- [ ] Production deployment guides

## Migration Guide

### For API Consumers

#### Option 1: Continue Using Lucene (No Changes Required)
Existing API calls work without modification:
```json
POST /api/v1/logs/query
{
  "query": "+error -debug",
  "start": "2024-01-01T00:00:00Z",
  "end": "2024-01-02T00:00:00Z"
}
```

#### Option 2: Switch to Bleve
Add `search_engine` field to requests:
```json
POST /api/v1/logs/query
{
  "query": "error AND NOT debug",
  "search_engine": "bleve",
  "start": "2024-01-01T00:00:00Z",
  "end": "2024-01-02T00:00:00Z"
}
```

### Query Syntax Differences

#### Lucene Syntax
```
+error -debug service:auth
```

#### Bleve Syntax
```
error AND NOT debug AND service:auth
```

### For Operators

#### Configuration Updates
Update `config.yaml`:
```yaml
search:
  enable_bleve: true
  default_engine: "lucene"  # or "bleve"
```

#### Monitoring
New metrics available:
- `mirador_search_queries_total{engine="bleve|lucene"}`
- `mirador_search_translation_duration_seconds{engine="bleve"}`
- `mirador_search_errors_total{engine="bleve|lucene"}`

## Implementation Notes

### Technical Challenges Resolved
1. **Bleve API Compatibility**: Adapted to Bleve v2 API changes
2. **Interface Mismatches**: Used wrapper pattern for translator compatibility
3. **Query Translation Complexity**: Implemented incremental translation approach
4. **Configuration Integration**: Added search config to existing config system

### Key Learnings
1. **API Design**: Request body engine selection provides better flexibility
2. **Abstraction**: Router pattern enables clean separation of concerns
3. **Incremental Implementation**: Start simple, expand based on real usage
4. **Backward Compatibility**: Critical for smooth adoption

### Risk Mitigation
1. **Performance**: Comprehensive benchmarking before production
2. **Compatibility**: Extensive backward compatibility testing
3. **Monitoring**: Deep observability for troubleshooting
4. **Documentation**: Clear migration guides and API documentation

---

## Status Summary

**Phase 1: Core Architecture** ✅ **COMPLETED**
- Search router implementation
- Bleve translator with core query support
- API handler updates for both logs and traces
- Model updates with search engine selection
- Configuration system integration
- Successful compilation and basic validation

**Phase 2: Distributed Index Architecture** 🔄 **PLANNED**
- Valkey integration for cluster coordination
- Tiered storage strategy
- Sharded indexing with consistent hashing
- Index rebalancing and fault tolerance

**Phase 3: Performance and Monitoring** 🔄 **PLANNED**
- Prometheus metrics collection
- Grafana dashboard development
- Performance benchmarking suite

**Phase 4: User Experience** 🔄 **PLANNED**
- API documentation updates
- Postman collections
- Operations guides

**Phase 5: Production Readiness** 🔄 **PLANNED**
- Load testing and security review
- A/B testing capabilities
- Production deployment validation

**Ready for**: Phase 2 implementation (distributed indexing)
**Risk Level**: Medium (core functionality complete, distributed complexity ahead)
**Next Milestone**: Valkey integration and tiered storage implementation