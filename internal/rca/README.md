# RCA Foundation - Phase 1

## Overview

This directory contains the foundation abstractions for the Root Cause Analysis (RCA) engine in MIRADOR-CORE. Phase 1 focuses on building internal data structures and mappers that bridge raw telemetry (from OpenTelemetry) with normalized anomaly representations, without yet implementing the full RCA logic.

## Key Components

### 1. ServiceGraph (`topology.go`, `topology_test.go`)

The `ServiceGraph` abstraction captures the directed topology of service dependencies from OpenTelemetry servicegraph connector metrics.

**Types:**
- `ServiceNode`: A service name (string type)
- `ServiceEdge`: A directed dependency with aggregated statistics:
  - Request counts and failure counts
  - Request/failure rates (normalized per second)
  - Error rates (fraction of failed requests)
  - Latency statistics (avg, p50, p95, p99)
- `ServiceGraph`: A directed graph of service dependencies

**Key Methods:**
- `AddEdge(edge)`: Add or update an edge in the graph
- `Neighbors(service)`: Get all adjacent services (both incoming and outgoing)
- `Downstream(service)`: Get services called by the given service
- `Upstream(service)`: Get services that call the given service
- `IsUpstream(from, to)`: Check if there's a directed path from `from` to `to`
- `ShortestPath(from, to)`: Find the shortest path between two services (BFS)
- `AllNodes()`, `AllEdges()`: Get all nodes/edges
- `Size()`, `EdgeCount()`: Get graph dimensions

**Metrics Consumed:**
From the otel-collector `servicegraph` connector, the following metrics are queried:
- `traces_service_graph_request_total` (total requests)
- `traces_service_graph_request_failed_total` (failed requests)
- `traces_service_graph_request_server_sum` / `_count` (server latency)
- `traces_service_graph_request_client_sum` / `_count` (client latency)

All metrics have labels: `client`, `server`, `connection_type`

### 2. ServiceGraphBuilder (`service_graph_builder.go`, `service_graph_builder_test.go`)

The `ServiceGraphBuilder` constructs a `ServiceGraph` by querying metrics from VictoriaMetrics (or any `MetricsQuerier` implementation).

**Key Methods:**
- `BuildGraph(ctx, tenantID, start, end)`: Query servicegraph metrics and build the dependency graph for a time range
- `querySamples(ctx, tenantID, metricName, start, end)`: Internal helper to query a single metric and parse Prometheus instant vector responses

**Usage Example:**
```go
builder := NewServiceGraphBuilder(metricsService, logger)
graph, err := builder.BuildGraph(ctx, "default", startTime, endTime)
if err != nil {
    // handle error
}

// Query the graph
neighbors := graph.Neighbors(rca.ServiceNode("api-gateway"))
path, found := graph.ShortestPath(rca.ServiceNode("api-gateway"), rca.ServiceNode("cassandra"))
```

### 3. AnomalyEvent (`anomaly_event.go`, `anomaly_event_test.go`)

The `AnomalyEvent` is a normalized representation of an anomaly detected in the system. It captures:
- **Source**: Service, component, timestamp
- **Classification**: Signal type (metrics/traces/logs/change), field name/value
- **Scores**: Anomaly score (from isolationforest), confidence, severity
- **Flags**: Error indicators (span error, log error, latency spike, etc.)
- **Context**: Tags (transaction_id, trace_id, etc.), source ID

**Types:**
- `SignalType`: Enum (metrics, traces, logs, change)
- `Severity`: Float64 (0.0-1.0) with constants:
  - `SeverityLow` (0.25)
  - `SeverityMedium` (0.5)
  - `SeverityHigh` (0.75)
  - `SeverityCritical` (1.0)

**Constructor Functions:**
- `NewAnomalyEvent(service, component, signalType)`: Create a new event
- `AnomalyEventFromIsolationForestSpan(spanID, service, spanName, duration, isAnomaly, anomalyScore, tags)`: Map isolationforest-flagged span to event
- `AnomalyEventFromMetric(service, metricName, value, isAnomaly, anomalyScore, attributes)`: Map isolationforest-flagged metric to event
- `AnomalyEventFromLog(logID, service, message, severity, isAnomaly, anomalyScore, attributes)`: Map log (error or iforest-flagged) to event
- `AnomalyEventFromErrorSpan(spanID, service, spanName, errorMessage, duration, tags)`: Map error span to event

**Isoforest Attributes (from otel-collector):**
- Classification attribute: `iforest_is_anomaly` (boolean)
- Score attribute: `iforest_anomaly_score` (float, 0-1)

### 4. AnomalyEventMapper (`anomaly_event_mapper.go`)

The `AnomalyEventMapper` provides helpers to convert raw telemetry (from traces, logs, metrics) into normalized `AnomalyEvent` objects.

**Key Types:**
- `AnomalyEventConfig`: Configuration for anomaly detection (thresholds, attribute names)
- `SpanData`: Simplified span representation
- `LogData`: Simplified log record representation
- `MetricData`: Simplified metric representation
- `RawTraceSpan`: Raw span with minimal required fields
- `AnomalyEventFilter`: Filter and reduce anomaly events by criteria

**Key Methods:**
- `MapSpanToAnomalyEvents(span)`: Convert trace span to anomaly events (checks for iforest flags, error status, high latency)
- `MapLogToAnomalyEvent(log)`: Convert log record to anomaly event (checks severity and iforest flags)
- `MapMetricToAnomalyEvent(metric)`: Convert metric to anomaly event (checks iforest flags)
- `MapRawSpanToSpanData(rawSpan)`: Convert raw span to `SpanData` format

**Filtering:**
```go
filter := NewAnomalyEventFilter().
    WithServices("tps", "cassandra").
    WithMinSeverity(rca.SeverityHigh).
    WithMaxAge(10 * time.Minute)

filtered := filter.Apply(allEvents)
```

**Serialization:**
- `SerializeToJSON(event)`: Convert event to JSON bytes
- `DeserializeFromJSON(data)`: Convert JSON bytes back to event

## Integration Points

### With Existing Code

1. **MetricsQuerier Interface**: `ServiceGraphBuilder` uses this interface to query metrics. This is implemented by `VictoriaMetricsService` in the services package, making it easy to swap for testing or other backends.

2. **Failure Components**: The existing `FailureComponent` enum (api-gateway, tps, keydb, kafka, cassandra) aligns with the service names that appear in servicegraph metrics.

3. **TimeRange**: Uses the same `TimeRange` type as existing correlation code.

## Testing

The package includes comprehensive unit tests:

- **Topology Tests** (`topology_test.go`):
  - Graph construction and edge management
  - Neighbor/upstream/downstream queries
  - Path finding (shortest path, transitive reachability)
  - Complex topology scenarios (financial transaction system)

- **ServiceGraphBuilder Tests** (`service_graph_builder_test.go`):
  - Basic graph building from metrics
  - Failure rate calculations
  - Invalid time range handling
  - Complex topology aggregation
  - Response parsing

- **AnomalyEvent Tests** (`anomaly_event_test.go`):
  - Event construction
  - Isoforest event mapping
  - Error signal mapping
  - Latency anomaly detection
  - Log and metric mapping
  - Event filtering
  - Serialization/deserialization

**Run tests:**
```bash
go test ./internal/rca -v
```

## OTel Collector Configuration

The abstractions align with the following otel-collector configuration (from `deployments/localdev/otel-collector-config.yaml`):

### ServiceGraph Connector
```yaml
servicegraph:
  latency_histogram_buckets: [100ms, 250ms, 1s, 5s, 10s]
  store:
    ttl: 2s
    max_items: 1000
```

**Emitted metrics:**
- `traces_service_graph_request_total`
- `traces_service_graph_request_failed_total`
- `traces_service_graph_request_server_sum` / `_count`
- `traces_service_graph_request_client_sum` / `_count`
- `traces_service_graph_unpaired_spans_total`
- `traces_service_graph_dropped_spans_total`

### Isolation Forest Processor
```yaml
isolationforest:
  forest_size: 150
  subsample_size: 512
  contamination_rate: 0.05
  score_attribute: iforest_anomaly_score
  classification_attribute: iforest_is_anomaly
  features:
    traces: [duration]
    metrics: [value]
    logs: [severity_number]
```

**Applied to:** traces, logs pipelines

## What's NOT Included (Phase 1)

This phase does NOT include:
- Full RCA (root cause analysis) logic or 5-Why chains
- New HTTP endpoints (no `/rca` endpoint yet)
- Storage or persistence of events
- Aggregation or correlation of events into incidents
- Recommendations or remediation suggestions
- Integration with external systems (AI engines, alerts, etc.)

These will be added in subsequent phases.

## Future Enhancement Opportunities

1. **Latency Histograms**: Extract p50, p95, p99 from servicegraph histogram metrics
2. **Advanced Topology Analysis**: Community detection, critical path identification
3. **Temporal Analysis**: Track topology changes over time
4. **Multi-Tenant**: Ensure all operations respect tenant boundaries
5. **Caching**: Cache graphs and events for performance
6. **Streaming Updates**: Real-time topology updates as new metrics arrive

## Design Principles

1. **Clarity over Performance**: Focus on correctness and understandability first
2. **Testability**: All components are independently testable with mock implementations
3. **Reusability**: Use existing interfaces (MetricsQuerier) and models (TimeRange, FailureComponent)
4. **Alignment**: Follow existing MIRADOR-CORE conventions and patterns
5. **Minimal Surface Area**: Only expose what's needed; keep internal implementation details private
