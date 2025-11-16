# Getting Started with mirador-core

This guide walks you through running the full mirador-core stack locally and explains how to ingest your own metrics, logs, and traces. It complements the main `README.md` by focusing on the practical "day one" workflow.

## 1. Prerequisites

- **Docker Desktop / Docker Engine** with Compose v2
- **Go 1.21+** (needed only if you want to build locally or use `telemetrygen`)
- Optional tooling: `make`, `curl`, `jq`

Clone the repository:

```bash
git clone https://github.com/platformbuilds/mirador-core.git
cd mirador-core
```

## 2. Start the Victoria Stack (metrics/logs/traces)

From the repository root:

```bash
cd deployments/localdev
docker compose -f victoria-docker-compose.yaml up -d
```

This brings up single-node instances of VictoriaMetrics (`:8428`), VictoriaLogs (`:9428`), and VictoriaTraces (`:10428`). Data persists in Docker named volumes so you can stop/restart without losing history.

### Linux networking note
If `host.docker.internal` is not resolvable on your Linux host, either:
- Add `extra_hosts: ["host.docker.internal:host-gateway"]` to the relevant services, or
- Run all compose files on a shared user-defined network and address containers by service name (e.g., `victoriametrics:8428`).

## 3. Launch the OpenTelemetry Collector

The collector accepts OTLP traffic (gRPC on `4317`, HTTP on `4318`) and forwards data to the Victoria stack.

```bash
cd deployments/localdev
docker compose -f otel-collector-docker-compose.yaml up -d
```

Feel free to review `otel-collector-config.yaml` before starting it—this is where you can point the collector at alternate backends or add processors.

## 4. Run mirador-core + Valkey

Spin up mirador-core and a single-node Valkey cache:

```bash
cd deployments/localdev
docker compose -f mirador-core-docker-compose.yaml up -d --build
```

Key endpoints once the service comes up:
- Core API: http://localhost:8010
- Health: http://localhost:8010/health
- OpenAPI spec: http://localhost:8010/api/openapi.yaml
- Prometheus metrics: http://localhost:8010/metrics

Authentication is disabled in the local compose file (`AUTH_ENABLED=false`). All requests run anonymously. For testing authentication flows, enable auth and use API keys for programmatic access.

### Building natively vs. using published images
The compose file builds a native binary for your host architecture. To pull a published image instead, comment out the `build:` block, set `image: platformbuilds/mirador-core:<tag>`, and rerun `docker compose`.

## 5. Generate Sample Telemetry (optional)

Install telemetrygen once:

```bash
go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest
```

Then send traces, metrics, or logs through the collector:

```bash
# Traces	elemetrygen traces --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 5
# Metrics
telemetrygen metrics --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 100
# Logs
telemetrygen logs   --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 10
```

Switch to `--otlp-http` and `localhost:4318` if you prefer HTTP OTLP.

## 6. Verify the Stack

- VictoriaMetrics UI/API: http://localhost:8428 (try a PromQL query such as `up`)
- VictoriaLogs UI/API: http://localhost:9428 (search for the telemetrygen log entries)
- VictoriaTraces (Jaeger UI): http://localhost:10428 (ensure Jaeger HTTP ingestion is enabled on port `14268`)
- mirador-core: `curl http://localhost:8010/health`

## 7. Ingesting Your Own Metrics, Logs, and Traces

### 7.1 Send OTLP telemetry
Most users can simply point their applications or agents to the local OpenTelemetry Collector:

```
otlp grpc:  localhost:4317
otlp http:  http://localhost:4318
```

The default collector config already exports to VictoriaMetrics/Logs/Traces. To send to additional destinations, edit `deployments/localdev/otel-collector-config.yaml` and add new exporters/pipelines.

### 7.2 Push data directly to Victoria backends
If you already have Prometheus-style scrapers, Loki clients, or Jaeger writers, you can target the Victoria services directly:

- Metrics: `http://localhost:8428/api/v1/write` (Prometheus remote write) or `/prometheus/api/v1/*`
- Logs: `http://localhost:9428/insert/jsonline` (JSON lines) or `/insert/loki/api/v1/push`
- Traces: `http://localhost:10428/api/traces` (Jaeger gRPC/HTTP depending on image flags)

Adjust the ingesters to send to those endpoints, or customize the compose files to expose alternate ports/protocols you need.

### 7.3 Configure mirador-core to read different sources
mirador-core reads backend locations from `config.yaml` (or environment variables). In localdev, the compose file sets:

```
MIRADOR_CORE_DATABASE__VICTORIA_METRICS__ENDPOINTS=http://host.docker.internal:8428
MIRADOR_CORE_DATABASE__VICTORIA_LOGS__ENDPOINTS=http://host.docker.internal:9428
MIRADOR_CORE_DATABASE__VICTORIA_TRACES__ENDPOINTS=http://host.docker.internal:10428
```

To point at additional clusters:

- Append entries to `database.metrics_sources`, `database.logs_sources`, or `database.traces_sources` in `config.yaml`. mirador-core aggregates across all configured sources automatically.
- Alternatively, set environment variables such as:

```
MIRADOR_CORE_DATABASE__METRICS_SOURCES__0__ENDPOINTS=http://vm-prod-1:8428
MIRADOR_CORE_DATABASE__METRICS_SOURCES__1__ENDPOINTS=http://vm-prod-2:8428
```

- Restart mirador-core after changes so it reloads the configuration.

### 7.4 Switching to your own OTLP pipelines
If your organisation already runs an OpenTelemetry Collector or other telemetry pipeline, you can:

1. Update `database.victoria_*` endpoints to match your production clusters.
2. Disable the bundled local collector and point your apps at your existing OTLP endpoint.
3. Or run both collectors and export to the same Victoria backends.

mirador-core only needs read access to VictoriaMetrics/Logs/Traces, so as long as the data lands in those systems you can keep using your preferred ingestion path.

## 8. Stopping the Stack

```bash
docker compose -f deployments/localdev/victoria-docker-compose.yaml down
docker compose -f deployments/localdev/otel-collector-docker-compose.yaml down
docker compose -f deployments/localdev/mirador-core-docker-compose.yaml down
```

Remove the Docker volumes (`vmdata`, `vldata`, `vtdata`) if you want a clean slate.

## 9. Query Your Data with the Unified API

Once data is flowing, use the unified query API to explore it. **Note**: If authentication is enabled, you must use API keys for programmatic access.

### 9.1 Enable Authentication (Optional)

To test authentication flows:

1. Set `AUTH_ENABLED=true` in `mirador-core-docker-compose.yaml`
2. Restart the service: `docker compose down && docker compose up -d --build`
3. Login to get an API key:
```bash
curl -X POST http://localhost:8010/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "aarvee", "password": "ChangeMe123!"}'
```
4. Use the returned `api_key` (starts with `mrk_`) for all API calls

### 9.2 Check Unified API Health

```bash
curl http://localhost:8010/api/v1/unified/health
```

This returns health status for all backend engines (VictoriaMetrics, VictoriaLogs, VictoriaTraces).

### 9.2 Query Capabilities

Check what the unified API supports:

```bash
curl http://localhost:8010/api/v1/unified/metadata
```

### 9.3 Query Metrics

```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": {
      "id": "metrics-query-1",
      "type": "metrics",
      "query": "up",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z",
      "cache_options": {
        "enabled": true,
        "ttl": "5m"
      }
    }
  }'
```

### 9.4 Query Logs

```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": {
      "id": "logs-query-1",
      "type": "logs",
      "query": "_time:15m level:info",
      "timeout": "30s"
    }
  }'
```

### 9.5 Query Traces

```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": {
      "id": "traces-query-1",
      "type": "traces",
      "query": "_time:15m service:telemetrygen",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z"
    }
  }'
```

### 9.6 Correlation Queries (Cross-Engine Analysis)

Find relationships between logs, metrics, and traces:

**Time-Window Correlation:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": {
      "id": "correlation-1",
      "type": "correlation",
      "query": "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z"
    }
  }'
```

**Label-Based Correlation:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": {
      "id": "correlation-2",
      "type": "correlation",
      "query": "logs:service:telemetrygen error AND traces:service:telemetrygen"
    }
  }'
```

**Multi-Engine Correlation:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": {
      "id": "correlation-3",
      "type": "correlation",
      "query": "logs:exception WITHIN 10m OF traces:status:error AND metrics:error_rate > 5"
    }
  }'
```

### 9.7 Advanced Query Features

**Enable Caching:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": {
      "type": "metrics",
      "query": "rate(http_requests_total[5m])",
      "cache_options": {
        "enabled": true,
        "ttl": "5m"
      }
    }
  }'
```

**Custom Timeouts:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": {
      "type": "logs",
      "query": "_time:1h level:error",
      "timeout": "60s"
    }
  }'
```

## 10. Explore with Postman or OpenAPI

### Using Postman

Import the Postman collection for ready-to-use examples:

```bash
# Collection location
deployments/localdev/postman/mirador-core-unified-queries.postman_collection.json
```

The collection includes examples for:
- Metrics queries (instant, range, aggregations)
- Logs queries (search, export, analytics)
- Traces queries (search, flamegraphs)
- Correlation queries (time-window, label-based)
- Cache management
- Health checks

### Using OpenAPI/Swagger UI

Access interactive API documentation:

```bash
# OpenAPI spec
http://localhost:8010/api/openapi.yaml

# Swagger UI (if enabled)
http://localhost:8010/swagger-ui/
```

## 11. Common Workflows

### Workflow 1: Troubleshooting an Application Error

1. **Find error logs:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "logs", "query": "_time:1h level:error service:myapp"}}'
```

2. **Correlate with metrics:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "correlation", "query": "logs:service:myapp error WITHIN 5m OF metrics:service:myapp"}}'
```

3. **Find related traces:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "traces", "query": "service:myapp status:error"}}'
```

### Workflow 2: Performance Analysis

1. **Check latency metrics:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "metrics", "query": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))"}}'
```

2. **Find slow requests in logs:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "logs", "query": "_time:1h duration:>5000"}}'
```

3. **Correlate with traces:**
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "correlation", "query": "logs:duration:>5000 WITHIN 1m OF traces:operation:GET"}}'
```

### Workflow 3: Real-Time Monitoring Dashboard

Create a dashboard that queries all three data types:

```bash
# Query 1: Service health (metrics)
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "metrics", "query": "up", "cache_options": {"enabled": true, "ttl": "30s"}}}'

# Query 2: Recent errors (logs)
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "logs", "query": "_time:5m level:error", "cache_options": {"enabled": true, "ttl": "30s"}}}'

# Query 3: Active traces (traces)
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "traces", "query": "_time:5m", "cache_options": {"enabled": true, "ttl": "30s"}}}'

# Query 4: Cross-engine correlation
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"query": {"type": "correlation", "query": "logs:error WITHIN 2m OF traces:status:error"}}'
```

## 12. Troubleshooting

- Run `docker compose logs -f <service>` to inspect any container.
- If mirador-core can't reach the Victoria endpoints, make sure `host.docker.internal` resolves on your platform or switch to service names on a shared network.
- The handler logs are available via `docker compose logs -f mirador-core`. Look for errors mentioning `Failed to store ...` to debug connectivity/API issues.
- Check unified API health: `curl http://localhost:8010/api/v1/unified/health`
- Verify backend connectivity: `curl http://localhost:8428/health` (VictoriaMetrics), `curl http://localhost:9428/health` (VictoriaLogs)
- Enable debug logs: Set `LOG_LEVEL=debug` in `mirador-core-docker-compose.yaml` and restart

## 13. Next Steps

With the stack running and data flowing:

1. **Explore Unified Query Language (UQL)**: See [docs/uql-language-guide.md](docs/uql-language-guide.md) for advanced query syntax
2. **Build Dashboards**: Use unified queries to build comprehensive monitoring dashboards
3. **Set Up Alerts**: Configure alerting based on correlation queries
4. **Optimize Performance**: Tune cache settings and timeouts based on your workload
5. **Production Deployment**: Follow [docs/deployment.md](docs/deployment.md) for Kubernetes deployment
6. **Migration Guide**: If migrating from engine-specific APIs, see [docs/migration-guide.md](docs/migration-guide.md)
7. **Operations Guide**: For production operations, see [docs/unified-query-operations.md](docs/unified-query-operations.md)

With the stack running, explore the API via the OpenAPI spec or the Postman collection in `deployments/localdev/postman`. Happy debugging!
