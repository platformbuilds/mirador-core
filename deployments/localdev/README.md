# Local Development & Testing

This folder contains minimal Docker Compose setups for running a full local loop:

- VictoriaMetrics/Logs/Traces single nodes
- OpenTelemetry Collector (OTLP gRPC/HTTP in → Victoria backends out)
- MIRADOR-CORE service and a single-node Valkey
- Optional: synthetic load via telemetrygen (Go tool)

## Prerequisites
- Docker + Docker Compose v2
- Go 1.21+ (for telemetrygen)

## 1) Start Victoria stack

Launch metrics/logs/traces single nodes. Data is stored in local folders under this directory.

```bash
cd public/mirador-core/deployments/localdev
docker compose -f victoria-docker-compose.yaml up -d
```

Exposed ports:
- VictoriaMetrics UI/API: http://localhost:8428
- VictoriaLogs UI/API: http://localhost:9428
- VictoriaTraces UI/API: http://localhost:8429

Note: Jaeger HTTP ingestion for traces is commonly on 14268. If your image doesn’t expose it by default, map `14268:14268` and enable Jaeger HTTP in the container flags.

## 2) Start OpenTelemetry Collector

The collector listens on OTLP gRPC (4317) and HTTP (4318), and exports to the Victoria stack.

```bash
cd public/mirador-core/deployments/localdev
# Review otel-collector-config.yaml if you need to change endpoints
docker compose -f otel-collector-docker-compose.yaml up -d
```

- On macOS/Windows, the config uses `host.docker.internal` to reach Victoria services running on the host.
- On Linux, either:
  - Adjust `otel-collector-config.yaml` exporters to use Victoria service names and run both compose files on the same user-defined network; or
  - Add `extra_hosts: ["host.docker.internal:host-gateway"]` to the collector service and keep the provided config.

## 3) Start MIRADOR-CORE with Valkey

Run a single-node Valkey and MIRADOR-CORE. MIRADOR-CORE will look for Victoria endpoints on the host, and Valkey on the local compose network.

Auth toggle (optional): You can disable auth for local testing by setting the config flag `auth.enabled: false` (Helm) or environment variable `AUTH_ENABLED=false` (Docker). When disabled, requests run as `anonymous` on tenant `default` and no token is required.

```bash
cd public/mirador-core/deployments/localdev
docker compose -f mirador-core-docker-compose.yaml up -d
```

- MIRADOR-CORE: http://localhost:8080
- Health: http://localhost:8080/health

Tip: To build MIRADOR-CORE from source instead of pulling the image, uncomment the `build:` block in `mirador-core-docker-compose.yaml`.

## 4) Generate Synthetic OTEL Data (telemetrygen)

Install telemetrygen once:

```bash
go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest
```

Generate traces (gRPC):

```bash
telemetrygen traces \
  --otlp-endpoint localhost:4317 \
  --otlp-insecure \
  --duration 30s \
  --rate 5
```

Generate metrics (gRPC):

```bash
telemetrygen metrics \
  --otlp-endpoint localhost:4317 \
  --otlp-insecure \
  --duration 30s \
  --rate 100
```

Generate logs (gRPC):

```bash
telemetrygen logs \
  --otlp-endpoint localhost:4317 \
  --otlp-insecure \
  --duration 30s \
  --rate 10
```

(If you prefer HTTP/JSON OTLP, use `--otlp-http` and point to `localhost:4318`.)

## 5) Verify Data

- Metrics (VictoriaMetrics): http://localhost:8428
  - Explore recent metrics; you should see telemetrygen-related series.
- Logs (VictoriaLogs): http://localhost:9428
  - The collector config uses the Loki-compatible push API; search for recent log entries.
- Traces (VictoriaTraces): http://localhost:8429
  - If Jaeger HTTP is exposed on 14268 and enabled, traces should be ingested;
    otherwise enable Jaeger ingestion in the traces container and re-run.

## 6) Check MIRADOR-CORE

- Health: `curl http://localhost:8080/health`
- OpenAPI: `http://localhost:8080/api/openapi.yaml`
- Metrics: `http://localhost:8080/metrics`

## 7) Cleanup

```bash
# Stop stacks
docker compose -f mirador-core-docker-compose.yaml down
docker compose -f otel-collector-docker-compose.yaml down
docker compose -f victoria-docker-compose.yaml down
```

## Notes & Tips
- Linux networking: if `host.docker.internal` is not resolvable, prefer a single shared user-defined network for all compose stacks and address services by name (e.g., `victoriametrics:8428`).
- Persisted data: local folders (`./vmdata`, `./vldata`, `./vtdata`) hold Victoria state; delete them for a clean slate.
- MIRADOR-CORE config: local compose sets Victoria endpoints via env vars; adjust them if you move services to another network.
