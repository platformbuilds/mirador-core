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

**MIRA removed from localdev**

The localdev compose no longer includes MIRA endpoints inside Mirador Core. If you need an AI explanation service, run a standalone MIRA microservice and configure your environment to call it directly.

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
  - Build per-arch images and load: `make dockerx-build-local-multi VERSION=v10.0.0` → tags `...:v10.0.0-amd64` and `...:v10.0.0-arm64`.
  - Or publish a real multi-arch tag to a registry: `make dockerx-push VERSION=v10.0.0` and use that tag in compose/Helm.
  - Or export an OCI archive without pushing: `make dockerx-build VERSION=v10.0.0` → `build/mirador-core-v10.0.0.oci`.

  ## Weaviate vectorizer (text2vec-transformers)

  The localdev compose now enables the `text2vec-transformers` vectorizer by
  default in the Weaviate container. This allows Weaviate to automatically
  generate embeddings for semantic search / fuzzy KPI discovery.

  Important points:

  - The Weaviate container must include the text2vec-transformers module (many
    official images do). In some setups a separate transformer inference service
    is required — see `TRANSFORMERS_INFERENCE_API` in `docker-compose.yaml`.
  - If you prefer not to run an inference backend or your machine cannot handle
    a local model, change `DEFAULT_VECTORIZER_MODULE` to `none` in
    `docker-compose.yaml` to opt out (the codebase supports external/BYO vectors).
  - For lightweight local testing, consider using `sentence-transformers/all-MiniLM-L6-v2`.

  ## VictoriaMetrics search limits

  Local deployments may produce large synthetic rows and timeseries which can exceed
  VictoriaMetrics' default search limits. If you see errors like:

  ```
  the number of matching timeseries exceeds 30000; either narrow down the search or increase -search.max* command-line flag values
  ```

  then the local compose is set to raise the cap for unique timeseries to 500000
  for easier experimentation. If you still hit limits, consider further increasing
  `--search.maxUniqueTimeseries` or narrowing match selectors.

  Warning: raising this value increases memory and search overhead for
  VictoriaMetrics. If you bump it substantially (e.g., >500k) make sure your
  host has sufficient RAM and adjust other search flags as necessary.

  Quick restart (local docker compose) after a change to `docker-compose.yaml`:

  ```bash
  docker compose -f deployments/localdev/docker-compose.yaml up -d --build victoriametrics
  docker compose -f deployments/localdev/docker-compose.yaml logs -f victoriametrics
  ```

  An optional `text2vec-transformers` inference service is included in the
  localdev `docker-compose.yaml`. It runs an ONNX-optimized inference image
  serving `sentence-transformers/all-MiniLM-L6-v2` (good balance of quality
  and CPU efficiency). Note:

  - Weaviate is configured to enable the `text2vec-transformers` module and
    points `TRANSFORMERS_INFERENCE_API` to `http://text2vec-transformers:8080`.
  - The runtime can be heavier on first start (model download + JIT/ONNX loading).
  - If you prefer to opt out, set `DEFAULT_VECTORIZER_MODULE` to `none` in
    `docker-compose.yaml`.
