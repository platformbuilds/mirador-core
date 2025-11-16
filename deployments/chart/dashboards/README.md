# Mirador Core - Bleve Performance Dashboards

This directory contains Grafana dashboard configurations for monitoring Bleve search performance in Mirador Core v6.0.0.

## Dashboards

### 1. Bleve Search Performance Dashboard
**File**: `grafana-dashboard.yaml`

Provides comprehensive monitoring of Bleve search operations including:

- **Index Operations Rate**: Real-time indexing throughput
- **Search Operation Duration**: P95 and P50 search latencies
- **Storage Usage**: Memory and disk usage patterns
- **Cluster Health**: Active nodes and leadership changes
- **Query Performance Heatmap**: Performance distribution by query type

### 2. Bleve Benchmark Results Dashboard
**File**: `benchmark-dashboard.yaml`

Visualizes performance benchmark results:

- **Indexing Performance**: Documents/second, memory usage trends
- **Search Performance**: Query latency distributions
- **Memory Usage Patterns**: Small vs large index comparisons
- **Query Complexity Analysis**: Performance by query type
- **Concurrent Operations**: Multi-threaded performance metrics

## Installation

### Using Helm Chart

The dashboards are automatically deployed with the Mirador Core Helm chart:

```bash
helm install mirador ./deployments/chart \
  --set grafana.enabled=true \
  --set prometheus.enabled=true
```

### Manual Installation

1. Apply the ConfigMap to your Kubernetes cluster:
```bash
kubectl apply -f deployments/chart/templates/grafana-dashboard.yaml
```

2. Import the dashboard in Grafana:
   - Go to Grafana UI → Dashboards → Import
   - Upload the JSON from the ConfigMap data

## Metrics Reference

### Bleve-Specific Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `bleve_index_operations_total` | Counter | Total number of index operations |
| `bleve_search_operation_duration` | Histogram | Search operation duration in seconds |
| `bleve_storage_memory_bytes` | Gauge | Memory usage |
| `bleve_storage_disk_bytes` | Gauge | Disk usage |
| `bleve_cluster_nodes` | Gauge | Number of active cluster nodes |
| `bleve_leadership_changes_total` | Counter | Total leadership changes |

### Benchmark Metrics

| Metric | Description |
|--------|-------------|
| `bleve_benchmark_indexing_duration` | Time to index N documents |
| `bleve_benchmark_search_duration` | Time to execute search queries |
| `bleve_benchmark_memory_usage` | Memory allocation during operations |
| `bleve_benchmark_query_complexity` | Performance by query type |

## Alerting Rules

Recommended alerts for Bleve performance monitoring:

```yaml
# High search latency
- alert: BleveHighSearchLatency
  expr: histogram_quantile(0.95, rate(bleve_search_operation_duration_bucket[5m])) > 1
  for: 5m
  labels:
    severity: warning

# Low indexing throughput
- alert: BleveLowIndexingThroughput
  expr: rate(bleve_index_operations_total[5m]) < 10
  for: 10m
  labels:
    severity: warning

# High memory usage
- alert: BleveHighMemoryUsage
  expr: bleve_storage_memory_bytes > 1e9
  for: 5m
  labels:
    severity: critical
```

## Troubleshooting

### Dashboard Not Loading

1. Verify Prometheus is scraping Mirador Core metrics
2. Check Grafana data sources are configured correctly
3. Ensure the ConfigMap is deployed to the correct namespace

### Missing Metrics

1. Confirm Bleve metrics are enabled in Mirador Core configuration
2. Check Prometheus target health
3. Verify metric names match the dashboard queries

### Performance Issues

1. Review the benchmark results dashboard for baseline comparisons
2. Check cluster node count and leadership stability
3. Monitor storage usage patterns for optimization opportunities