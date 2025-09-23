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
- Core API: http://localhost:8080
- Health: http://localhost:8080/health
- OpenAPI spec: http://localhost:8080/api/openapi.yaml
- Prometheus metrics: http://localhost:8080/metrics

Authentication is disabled in the local compose file (`AUTH_ENABLED=false`). All requests run as the `default` tenant unless you set an `X-Tenant-ID` header.

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
- mirador-core: `curl http://localhost:8080/health`

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

## 9. Troubleshooting

- Run `docker compose logs -f <service>` to inspect any container.
- If mirador-core can’t reach the Victoria endpoints, make sure `host.docker.internal` resolves on your platform or switch to service names on a shared network.
- The handler logs are available via `docker compose logs -f mirador-core`. Look for errors mentioning `Failed to store ...` to debug connectivity/API issues.

With the stack running, explore the API via the OpenAPI spec or the Postman collection in `deployments/localdev/postman`. Happy debugging!
