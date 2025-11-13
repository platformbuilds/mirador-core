# Mirador Core v7.0.0 - Monitoring and Observability Guide

This document provides comprehensive guidance for monitoring and observing Mirador Core v7.0.0, including metrics collection, distributed tracing, performance dashboards, and alerting rules.

## Table of Contents

1. [Overview](#overview)
2. [Metrics Collection](#metrics-collection)
3. [Distributed Tracing](#distributed-tracing)
4. [Grafana Dashboards](#grafana-dashboards)
5. [Alerting Rules](#alerting-rules)
6. [Configuration](#configuration)
7. [Troubleshooting](#troubleshooting)

## Overview

Mirador Core v7.0.0 implements comprehensive monitoring and observability capabilities to ensure operational visibility and reliability of the unified observability platform. The monitoring stack includes:

- **Prometheus** for metrics collection and storage
- **OpenTelemetry** for distributed tracing
- **Grafana** for visualization and dashboards
- **AlertManager** for alerting and notifications

## Metrics Collection

### Unified Query Engine Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `mirador_core_unified_query_operations_total` | Counter | Total number of unified query operations | `query_type`, `status`, `engine_routed` |
| `mirador_core_unified_query_operation_duration_seconds` | Histogram | Duration of unified query operations | `query_type`, `status` |
| `mirador_core_unified_query_cache_operations_total` | Counter | Cache operations for unified queries | `result` (hit/miss) |
| `mirador_core_unified_query_correlation_operations_total` | Counter | Correlation operations within unified queries | `correlation_type`, `status` |

### Correlation Engine Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `mirador_core_correlation_operations_total` | Counter | Total correlation operations | `correlation_type`, `status` |
| `mirador_core_correlation_duration_seconds` | Histogram | Duration of correlation operations | `correlation_type` |
| `mirador_core_correlation_engine_query_duration_seconds` | Histogram | Duration of individual engine queries | `engine_type` |
| `mirador_core_correlation_parallel_execution_duration_seconds` | Histogram | Duration of parallel execution coordination | `engines_count` |
| `mirador_core_correlation_result_merging_duration_seconds` | Histogram | Duration of result merging operations | `correlations_count` |
| `mirador_core_correlation_cache_operations_total` | Counter | Cache operations for correlations | `result` (hit/miss) |
| `mirador_core_correlation_errors_total` | Counter | Correlation-specific errors | `error_type` |
| `mirador_core_correlation_memory_usage_bytes` | Gauge | Current memory usage | |
| `mirador_core_correlation_cpu_usage_seconds_total` | Counter | CPU usage in seconds | |

### Tracing Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `mirador_core_traces_started_total` | Counter | Total traces started | |
| `mirador_core_traces_completed_total` | Counter | Total traces completed | |
| `mirador_core_traces_active_total` | Gauge | Currently active traces | |
| `mirador_core_traces_sampled_total` | Counter | Traces that were sampled | |
| `mirador_core_query_trace_duration_seconds` | Histogram | Duration of query traces | `query_type` |
| `mirador_core_correlation_trace_duration_seconds` | Histogram | Duration of correlation traces | `correlation_type` |
| `mirador_core_spans_created_total` | Counter | Total spans created | `operation` |
| `mirador_core_trace_errors_total` | Counter | Trace-related errors | `error_type` |
| `mirador_core_trace_exports_total` | Counter | Trace export operations | `status` (success/failure) |
| `mirador_core_service_calls_total` | Counter | Service-to-service calls | `source_service`, `target_service` |
| `mirador_core_span_attributes_total` | Counter | Span attributes usage | `attribute_name` |

### General Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `mirador_core_errors_total` | Counter | General application errors | `component` |

## Distributed Tracing

### Trace Structure

Mirador Core implements hierarchical tracing with the following span structure:

```
unified-query-{query_type}
├── query-parsing
├── engine-routing
├── cache-lookup
├── {engine_type}-query
│   ├── query-execution
│   └── result-processing
├── correlation-{correlation_type} (if applicable)
│   ├── parallel-execution
│   │   ├── {engine_type}-query
│   │   └── {engine_type}-query
│   └── result-merging
└── response-formatting
```

### Span Attributes

All spans include the following attributes:
- `service.name`: "mirador-core"
- `service.version`: "v9.0.0"
- `operation.name`: Specific operation being traced
- `query.id`: Unique query identifier
- `user.id`: User identifier (if available)
- `query.type`: Type of query (metrics, logs, traces)
- `engine.type`: Engine used for execution

### Sampling Configuration

Tracing uses adaptive sampling based on:
- Query complexity (number of engines involved)
- Query latency (high latency queries are sampled more)
- Error rate (failed queries are always sampled)
- System load (reduced sampling under high load)

## Grafana Dashboards

### Available Dashboards

1. **Unified Query Performance** (`unified-query-performance.json`)
   - Query operations overview and success rates
   - Query latency by type (50th and 95th percentiles)
   - Engine routing distribution
   - Cache hit rates
   - Correlation operations and latency
   - Error rates by component

2. **Correlation Engine Performance** (`correlation-engine-performance.json`)
   - Correlation operations rate and success rates
   - Correlation latency by type
   - Engine query performance breakdown
   - Parallel execution coordination metrics
   - Result merging performance
   - Cache performance and error tracking
   - Resource usage (memory and CPU)

3. **Distributed Tracing** (`distributed-tracing.json`)
   - Active traces and completion rates
   - Query and correlation trace durations
   - Span count by operation
   - Trace errors and export success rates
   - Service dependency visualization
   - Trace sampling and buffer usage

### Dashboard Installation

1. Import the JSON files into Grafana:
   ```bash
   # Using Grafana CLI
   grafana-cli dashboards import --file deployments/grafana/dashboards/unified-query-performance.json
   grafana-cli dashboards import --file deployments/grafana/dashboards/correlation-engine-performance.json
   grafana-cli dashboards import --file deployments/grafana/dashboards/distributed-tracing.json
   ```

2. Or import manually through the Grafana UI:
   - Navigate to Dashboard → Import
   - Upload each JSON file
   - Configure data sources (Prometheus for metrics, Jaeger/Tempo for traces)

## Alerting Rules

### Alert Categories

#### Performance Alerts
- **HighQueryLatency**: Query latency exceeds 5 seconds (95th percentile)
- **QuerySuccessRateLow**: Query success rate drops below 95%
- **CacheHitRateLow**: Cache hit rate drops below 70%
- **HighCorrelationLatency**: Correlation latency exceeds 10 seconds
- **CorrelationSuccessRateLow**: Correlation success rate drops below 90%
- **EngineQueryTimeout**: Individual engine queries exceed 30 seconds

#### Reliability Alerts
- **TraceExportFailure**: Trace export failure rate exceeds 5%
- **HighTraceErrorRate**: Trace error rate exceeds 10 errors/second
- **HighErrorRate**: General error rate exceeds 5 errors/second per component
- **ServiceDown**: Mirador Core service is unavailable

#### Resource Alerts
- **HighMemoryUsage**: Memory usage exceeds 85% of limit
- **HighCPUUsage**: CPU usage exceeds 80%
- **QueryThroughputDrop**: Query throughput drops below 10 ops/sec
- **CorrelationThroughputDrop**: Correlation throughput drops below 5 ops/sec

### Alert Configuration

Alerts are configured in `deployments/grafana/alerting-rules.yml` and should be loaded into Prometheus AlertManager.

Example AlertManager configuration:
```yaml
route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'mirador-alerts'
  routes:
  - match:
      severity: critical
    receiver: 'mirador-critical'

receivers:
- name: 'mirador-alerts'
  slack_configs:
  - api_url: 'YOUR_SLACK_WEBHOOK_URL'
    channel: '#mirador-alerts'
    title: '{{ .GroupLabels.alertname }}'
    text: '{{ .CommonAnnotations.description }}'
```

## Configuration

### Prometheus Configuration

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "deployments/grafana/alerting-rules.yml"

scrape_configs:
  - job_name: 'mirador-core'
    static_configs:
      - targets: ['mirador-core:9090']
    metrics_path: '/metrics'
```

### OpenTelemetry Configuration

```yaml
# tracing configuration in config.yaml
tracing:
  enabled: true
  service_name: "mirador-core"
  service_version: "v9.0.0"
  sampling_ratio: 0.1
  jaeger_endpoint: "http://jaeger:14268/api/traces"
  otlp_endpoint: "http://otel-collector:4318"

  # Resource attributes
  resource_attributes:
    service.name: "mirador-core"
    service.version: "v9.0.0"
    service.instance.id: "${HOSTNAME}"
    deployment.environment: "${ENVIRONMENT}"
```

### Grafana Configuration

```yaml
# grafana/provisioning/datasources/prometheus.yml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    url: http://prometheus:9090
    access: proxy
    isDefault: true

  - name: Jaeger
    type: jaeger
    url: http://jaeger:16686
    access: proxy
```

## Troubleshooting

### Common Issues

#### Metrics Not Appearing in Prometheus

1. Check if the `/metrics` endpoint is accessible:
   ```bash
   curl http://mirador-core:9090/metrics
   ```

2. Verify Prometheus scrape configuration targets the correct port and path

3. Check Mirador Core logs for metrics collection errors

#### Traces Not Appearing in Jaeger

1. Verify OpenTelemetry configuration is correct
2. Check network connectivity to Jaeger endpoint
3. Ensure proper sampling configuration
4. Review Mirador Core logs for tracing errors

#### Dashboard Panels Showing No Data

1. Confirm data sources are properly configured in Grafana
2. Check metric names match exactly (case-sensitive)
3. Verify time range includes data collection period
4. Ensure Prometheus has scraped recent metrics

#### Alerts Not Firing

1. Check Prometheus AlertManager configuration
2. Verify alerting rules are loaded: `promtool check rules alerting-rules.yml`
3. Confirm alert conditions are met by querying metrics directly
4. Check AlertManager logs for delivery issues

### Performance Tuning

#### High Cardinality Metrics

Monitor for high cardinality in label combinations, especially:
- `query_type` × `engine_routed` combinations
- `correlation_type` × `engines_count` combinations
- `operation` × `attribute_name` in tracing

#### Sampling Optimization

Adjust sampling ratios based on:
- Traffic volume
- Storage capacity
- Required observability granularity
- Performance impact tolerance

#### Resource Usage

Monitor and tune:
- Memory usage for correlation result caching
- CPU usage during parallel execution
- Network bandwidth for trace exports
- Storage requirements for metrics retention

### Log Analysis

Key log patterns to monitor:
- `ERROR.*correlation.*timeout` - Engine timeouts
- `WARN.*cache.*miss.*rate` - Cache performance issues
- `ERROR.*trace.*export` - Tracing export failures
- `WARN.*memory.*usage.*high` - Resource pressure

### Runbooks

- <a href="query-performance-runbook.md">Query Performance Runbook</a>
- <a href="correlation-reliability-runbook.md">Correlation Reliability Runbook</a>
- <a href="cache-performance-runbook.md">Cache Performance Runbook</a>
- <a href="tracing-troubleshooting.md">Tracing Troubleshooting Guide</a>
- <a href="service-recovery-procedures.md">Service Recovery Procedures</a>