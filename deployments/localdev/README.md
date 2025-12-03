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

Launch metrics/logs/traces single nodes. Data is stored in Docker named volumes.

```bash
cd public/mirador-core/deployments/localdev
docker compose -f victoria-docker-compose.yaml up -d
```

Exposed ports:
- VictoriaMetrics UI/API: http://localhost:8428
- VictoriaLogs UI/API: http://localhost:9428
- VictoriaTraces UI/API: http://localhost:10428

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

Cross-platform note (Apple Silicon, ARM64, x86_64): All localdev images are multi-arch. The compose files do not pin `platform` so Docker will pull the native image for your host automatically (arm64 on Apple Silicon, amd64 on Intel/AMD). If you need to force a specific platform, you may add a `platform:` line to a local override compose file.

MIRADOR-CORE is a pure observability engine that assumes external authentication and authorization. All requests are processed without internal auth checks - security should be handled by external proxies, API gateways, or service mesh.

**NEW: MIRA AI-Powered RCA Explanations**

The localdev setup now includes Ollama for AI-powered RCA explanations via MIRA (Mirador Intelligent Research Assistant). This is enabled by default in development:

- **Model**: `llama3.1:8b` (4.9GB, optimized for M1 Pro 16GB)
- **Auto-setup**: Model pulls automatically on first start (may take 3-5 minutes)
- **Endpoint**: `POST /api/v1/mira/rca_analyze`

```bash
cd public/mirador-core/deployments/localdev
# Build locally for native arch and start (includes Ollama)
docker compose -f mirador-core-docker-compose.yaml up -d --build
```

- MIRADOR-CORE: http://localhost:8010
- Health: http://localhost:8010/health
- Ollama: http://localhost:11434 (model server)

**Testing MIRA:**

```bash
# 1. Get RCA data first
RCA_RESPONSE=$(curl -s -X POST http://localhost:8010/api/v1/unified/rca \
  -H "Content-Type: application/json" \
  -d '{"startTime":"2025-12-03T07:30:00Z","endTime":"2025-12-03T08:30:00Z"}')

# 2. Get AI explanation
curl -X POST http://localhost:8010/api/v1/mira/rca_analyze \
  -H "Content-Type: application/json" \
  -d "{\"rcaData\": $RCA_RESPONSE}" | jq .
```

**Model Information:**
- **llama3.1:8b**: Excellent quality on M1 Pro 16GB (~ 2-4 seconds per explanation)
- **Memory**: ~5-6GB RAM usage (comfortable for 16GB system)
- **Quality**: High-quality explanations, production-ready for self-hosted deployments

For cloud-based production, switch to OpenAI (gpt-4) or Anthropic (claude-3-5-sonnet) in production config.

Tip: The `mirador-core` service is configured to `build` locally, which produces a native binary for your host (arm64 on Apple Silicon, amd64 on Intel/AMD). If you prefer to pull a published image instead, comment out the `build:` block and set `image: platformbuilds/mirador-core:<multi-arch-tag>`. On Linux, if `host.docker.internal` doesn't resolve, uncomment `extra_hosts: ["host.docker.internal:host-gateway"]` in the compose file.

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

- Health: `curl http://localhost:8010/health`
- OpenAPI: `http://localhost:8010/api/openapi.yaml`
- Metrics: `http://localhost:8010/metrics`

## Cleanup

```bash
# Stop stack
docker compose -f docker-compose.yaml down
```

## Notes & Tips
- Linux networking: if `host.docker.internal` is not resolvable, prefer a single shared user-defined network for all compose stacks and address services by name (e.g., `victoriametrics:8428`), or use `extra_hosts: ["host.docker.internal:host-gateway"]`.
- Persisted data: Victoria state is stored in Docker named volumes (`vmdata`, `vldata`, `vtdata`). Remove them to reset:
  - `docker volume rm vmdata vldata vtdata` (only after all stacks are stopped).
- MIRADOR-CORE config: local compose sets Victoria endpoints via env vars; adjust them if you move services to another network.

### Multi-Arch Build Notes (Docker Desktop / Rancher Desktop)
- Loading a single multi-arch image into the local Docker daemon is not supported (`--load` cannot import manifest lists).
- To test locally:
  - Build per-arch images and load: `make dockerx-build-local-multi VERSION=v2.1.3` → tags `...:v2.1.3-amd64` and `...:v2.1.3-arm64`.
  - Or publish a real multi-arch tag to a registry: `make dockerx-push VERSION=v2.1.3` and use that tag in compose/Helm.
  - Or export an OCI archive without pushing: `make dockerx-build VERSION=v2.1.3` → `build/mirador-core-v2.1.3.oci`.
