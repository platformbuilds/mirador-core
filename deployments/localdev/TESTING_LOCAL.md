**Overview**
- Purpose: Spins up a local development stack (VictoriaMetrics, VictoriaLogs, VictoriaTraces, Weaviate, Valkey, OTEL Collector), starts MIRADOR‑CORE, then pumps synthetic OpenTelemetry data and runs end‑to‑end API tests with a report.
- Scope: Local developer validation with auth disabled by default.

**What It Sets Up**
- Docker Compose (`deployments/localdev/docker-compose.yaml`) with services:
  - VictoriaMetrics: `http://localhost:8428`
  - VictoriaLogs: `http://localhost:9428`
  - VictoriaTraces: `http://localhost:10428`
  - OpenTelemetry Collector: OTLP gRPC `4317`, HTTP `4318`
  - Valkey (Redis-compatible): `localhost:6379`
  - Weaviate for schema storage: `http://localhost:8081`
  - MIRADOR‑CORE: `http://localhost:8080`
- E2E tests under `deployments/localdev/e2e`.
- Readiness helper: `deployments/localdev/scripts/wait-for-url.sh`.

**Prerequisites**
- Docker + Docker Compose v2
- Go 1.23+
- `curl`
- Optional reporters: `go-junit-report`, `telemetrygen` (installed automatically by Makefile when seeding)

**Quick Start**
- From the repository root:
  - `make localdev`
  - This will: bring up services → wait for server → seed OTEL data → run E2E tests → tear down services.

**Make Targets (root Makefile)**
- `localdev`:
  - Alias for `localdev-up` → `localdev-wait` → `localdev-seed-otel` → `localdev-test` → `localdev-down`.
- `localdev-up`:
  - `docker compose -f deployments/localdev/docker-compose.yaml up -d --build`
- `localdev-wait`:
  - `deployments/localdev/scripts/wait-for-url.sh http://localhost:8080/ready`
- `localdev-seed-otel`:
  - Seeds OpenTelemetry traces/metrics/logs to `localhost:4317` via `telemetrygen`.
- `localdev-test`:
  - Runs `go test -v ./deployments/localdev/e2e` with `E2E_BASE_URL=http://localhost:8080`.
  - Emits JSON report at `localdev/e2e-report.json`; optional JUnit XML at `localdev/e2e-report.xml` if `go-junit-report` is present.
- `localdev-down`:
  - `docker compose -f deployments/localdev/docker-compose.yaml down -v`

**E2E Coverage**
- Health and OpenAPI:
  - `GET /health`, `GET /ready`, `GET /api/openapi.json`
- MetricsQL (VictoriaMetrics):
  - `GET /api/v1/labels`, `POST /api/v1/metrics/query`
- Logs/D3 endpoints (VictoriaLogs):
  - `GET /api/v1/logs/streams`, `GET /api/v1/logs/histogram`, `POST /api/v1/logs/search`

**Manual Steps**
- Bring up services: `docker compose -f deployments/localdev/docker-compose.yaml up -d --build`
- Wait until ready: `deployments/localdev/scripts/wait-for-url.sh http://localhost:8080/ready`
- Seed OTEL data manually:
  - `go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest`
  - `telemetrygen metrics --otlp-endpoint localhost:4317 --otlp-insecure --duration 10s --rate 200`
  - `telemetrygen logs --otlp-endpoint localhost:4317 --otlp-insecure --duration 10s --rate 20`
  - `telemetrygen traces --otlp-endpoint localhost:4317 --otlp-insecure --duration 10s --rate 10`
- Run E2E: `E2E_BASE_URL=http://localhost:8080 go test -v ./deployments/localdev/e2e`
- Tear down: `docker compose -f deployments/localdev/docker-compose.yaml down -v`

**Customization**
- Change base URL: `make localdev BASE_URL=http://127.0.0.1:8080`
- Keep stack up after tests: run individual targets (`localdev-up`, `localdev-wait`, `localdev-seed-otel`, `localdev-test`) and skip `localdev-down`.
- Auth: To test auth flows, enable in compose env and provide tokens/sessions.

**Troubleshooting**
- Ports busy: change host ports in `deployments/localdev/docker-compose.yaml`.
- Collector can’t reach backends: check `deployments/localdev/otel-collector-config.yaml` endpoints.
- App not ready: inspect container logs `docker logs mirador-core`.
