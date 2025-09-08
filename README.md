# MIRADOR-CORE

Advanced Observability Platform - Backend REST API Service

## Architecture Overview

MIRADOR-CORE serves as the central orchestration layer for the MIRADOR observability platform, providing:

- **Unified REST API** with OpenAPI 3.0 specification
- **AI Engine Integration** via gRPC + Protocol Buffers
- **VictoriaMetrics Ecosystem** connectivity (Metrics, Logs, Traces)
- **Enterprise Authentication** (LDAP/AD, OAuth 2.0, RBAC)
- **Valkey Cluster Caching** for high-performance data access
- **Real-time WebSocket Streams** for live data

## Key Features

### 🧠 AI-Powered Analysis
- **PREDICT-ENGINE**: System fracture/fatigue prediction with ML models
- **RCA-ENGINE**: Root cause analysis using red anchors correlation pattern
- **ALERT-ENGINE**: Intelligent alert management with noise reduction

### 📊 Unified Query Interface
- **MetricsQL**: Enhanced PromQL with 150+ functions
- **LogsQL**: Pipe-based log analysis with billions of entries support
- **VictoriaTraces**: Distributed tracing with Jaeger compatibility

### 🚀 High Performance
- **10x less RAM** usage compared to traditional solutions
- **Valkey Cluster Caching** for sub-millisecond query responses
- **Horizontal scaling** with load balancing
- **gRPC communication** for internal service communication

## Quick Start

### Prerequisites
- Go 1.21+
- Docker & Kubernetes
- Redis Cluster (Valkey Cluster)
- VictoriaMetrics ecosystem

### Development Setup
```bash
# Clone repository
git clone https://github.com/company/mirador-core
cd mirador-core

# Setup development environment
make setup-dev

# Generate Protocol Buffers
make proto

# Run tests
make test

# Start development server
make dev
```

### Docker Deployment
```bash
# Build Docker image
make docker

# Deploy to Kubernetes
make deploy-dev
```

## API Documentation

### Authentication
All API endpoints (except `/health` and `/ready`) require authentication:

```bash
# LDAP/AD Authentication
curl -X POST https://mirador-core/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "john.doe", "password": "password"}'

# OAuth 2.0 Token
curl -X GET https://mirador-core/api/v1/query \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### MetricsQL Queries
```bash
# Execute MetricsQL query
curl -X POST https://mirador-core/api/v1/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "rate(http_requests_total[5m])"}'

# Range query with time series data
curl -X POST https://mirador-core/api/v1/query_range \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "avg_over_time(cpu_usage[10m])",
    "start": "2025-08-31T10:00:00Z",
    "end": "2025-08-31T11:00:00Z",
    "step": "1m"
  }'
```

### AI Fracture Prediction
```bash
# Analyze system fractures/fatigue
curl -X POST https://mirador-core/api/v1/predict/analyze \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "component": "payment-service",
    "time_range": "24h",
    "model_types": ["isolation_forest", "lstm_trend"]
  }'
```

### Root Cause Analysis
```bash
# Start RCA investigation with red anchors pattern
curl -X POST https://mirador-core/api/v1/rca/investigate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "incident_id": "INC-2025-0831-001",
    "symptoms": ["high_cpu", "connection_timeouts"],
    "time_range": {
      "start": "2025-08-31T14:00:00Z",
      "end": "2025-08-31T15:00:00Z"
    },
    "affected_services": ["payment-service", "database"]
  }'
```

## Configuration

### Environment Variables
```bash
# Core settings
export PORT=8080
export ENVIRONMENT=production
export LOG_LEVEL=info

# VictoriaMetrics ecosystem
export VM_ENDPOINTS=http://vm-select-0:8481,http://vm-select-1:8481
export VL_ENDPOINTS=http://vl-select-0:9428,http://vl-select-1:9428
export VT_ENDPOINTS=http://vt-select-0:10428,http://vt-select-1:10428

# AI Engines (gRPC endpoints)
export PREDICT_ENGINE_GRPC=predict-engine:9091
export RCA_ENGINE_GRPC=rca-engine:9092
export ALERT_ENGINE_GRPC=alert-engine:9093

# Valkey Cluster caching
export VALKEY_CACHE_NODES=redis-1:6379,redis-2:6379,redis-3:6379
export CACHE_TTL=300

# Authentication
export LDAP_URL=ldap://ldap.company.com
export LDAP_BASE_DN=dc=company,dc=com
export RBAC_ENABLED=true

# External integrations
export SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...
export TEAMS_WEBHOOK_URL=https://company.webhook.office.com/...
```

## Monitoring

MIRADOR-CORE exposes Prometheus metrics at `/metrics`:

- `mirador_core_http_requests_total` - HTTP request count
- `mirador_core_grpc_requests_total` - gRPC request count  
- `mirador_core_cache_requests_total` - Cache operation count
- `mirador_core_sessions_active` - Active user sessions
- `mirador_core_predictions_generated_total` - AI predictions count

## Architecture Components

### Data Flow
1. **Telemetry Ingestion** → VictoriaMetrics ecosystem
2. **AI Analysis** → gRPC + protobuf communication
3. **Valkey Cluster Caching** → Faster data access
4. **REST API** → MIRADOR-UI consumption
5. **External Integrations** → Slack, MS Teams, Email

### Security
- **RBAC**: Role-based access control with LDAP/AD
- **Session Management**: Valkey cluster-based sessions
- **Tenant Isolation**: Multi-tenant data segregation
- **API Rate Limiting**: Per-tenant request limits

### Performance
- **Load Balancing**: Round-robin across VictoriaMetrics nodes
- **Connection Pooling**: Efficient resource utilization  
- **Query Caching**: Valkey cluster query result caching
- **Horizontal Scaling**: Stateless microservice design

## Architecture Diagram (Text)

### High-Level Diagram
- Clients: MIRADOR-UI (browser), Prometheus (scraper), engineers (API/CLI)
- Core: mirador-core (Gin HTTP API + WebSocket), middleware (Auth/JWT/Session, RBAC, RateLimiter, RequestLogger, Metrics), handlers (MetricsQL, LogsQL + D3, Traces, Predict, RCA, Alerts, Config, Sessions, RBAC), services (VictoriaMetricsServices, gRPC Clients, NotificationService), caching (Valkey cluster), config/secrets/watcher, logger, internal metrics
- Backends: VictoriaMetrics (metrics), VictoriaLogs (logs), VictoriaTraces (traces/Jaeger), AI Engines (Predict, RCA, Alert via gRPC), Identity (LDAP/AD, OAuth/OIDC), External (Slack, MS Teams, SMTP)

```
[ MIRADOR‑UI ]                      [ Prometheus ]
     |  HTTPS (REST, WS)                   |  HTTP /metrics
     v                                     v
+-------------------- Ingress / LB ---------------------+
                        |
                        v
                 +--------------+
                 | mirador-core |
                 |  Gin Server  |
                 +--------------+
                        |
       +----------------+-----------------------------+
       |                |                             |
       v                v                             v
 +--------------+  +-----------+                 +-----------+
 | Middleware   |  | Handlers  |                 | WebSocket |
 | - Auth       |  | - Metrics |                 |  Streams  |
 | - RBAC       |  | - Logs    |                 | (metrics, |
 | - RateLimit  |  | - Traces  |                 | alerts,   |
 | - ReqLogger  |  | - Predict |                 | predicts) |
 | - Metrics    |  | - RCA     |                 +-----------+
 +--------------+  | - Alerts  |
                   | - Config  |
                   | - Session |
                   | - RBAC    |
                   +-----+-----+
                         |
                         v
           +-------------------------------+
           |  Internal Services Layer      |
           |-------------------------------|
           | VictoriaMetricsServices       |
           |  - Metrics (HTTP→VM Select)   |
           |  - Logs (HTTP→VL Select)      |
           |  - Traces (HTTP→Jaeger API)   |
           |                               |
           | gRPC Clients                  |
           |  - Predict-Engine             |
           |  - RCA-Engine                 |
           |  - Alert-Engine               |
           |                               |
           | NotificationService           |
           +--------+------------+---------+
                    |            |
                    v            v
            +---------------+   +-------------------+
            | Valkey Cluster|   | External Notifiers|
            | - Sessions    |   | Slack / Teams /   |
            | - Query Cache |   | SMTP Email        |
            | - Rate Limits |   +-------------------+
            +-------+-------+
                    |
    +---------------+------------------------------+
    |                                              |
    v                                              v
 [ VictoriaMetrics ]                       [ VictoriaLogs / VictoriaTraces ]
   - /select/0/prometheus/api/v1/...        - /select/logsql/... , /insert/jsonline
                                            - /select/jaeger/api/
```

### Security & Identity
- OAuth/JWT and/or Valkey-backed session tokens; tenant context injected into requests.
- LDAP/AD support for enterprise auth; RBAC enforced at handler level.
- CORS configured; security headers added; per-tenant rate limiting enabled.

### Primary Flows
- MetricsQL (instant/range): UI → `/api/v1/query|query_range` → middleware (auth/RBAC/rate limit) → Valkey cache → VictoriaMetrics (round‑robin + retry/backoff) → cache set → response.
- LogsQL & D3: UI → `/api/v1/logs/query|histogram|facets|search|tail` → VictoriaLogs streaming JSON (gzip‑aware) → on‑the‑fly aggregations (buckets/facets/paging) → response; `/logs/store` persists AI events.
- Traces: UI → `/api/v1/traces/services|operations|:traceId|search` → pass‑through to VictoriaTraces (Jaeger HTTP) with optional caching for lists.
- Predict: UI → `/api/v1/predict/analyze` → gRPC to Predict‑Engine → store prediction JSON events to VictoriaLogs → optional notifications → list via `/predict/fractures` (logs query + cache).
- RCA: UI → `/api/v1/rca/investigate` → gRPC to RCA‑Engine → timeline + red anchors → optional store via `/api/v1/rca/store`.
- Alerts: UI → `/api/v1/alerts` (GET with cache; POST to create rule) and `/api/v1/alerts/:id/acknowledge` → gRPC to Alert‑Engine → optional WS broadcast.

### Cross‑Cutting Concerns
- Caching: Valkey stores sessions, query results, and rate‑limit counters.
- Observability: Prometheus metrics for HTTP, gRPC, cache, sessions, query durations.
- Configuration: Viper layered config, env overrides (e.g., `VM_ENDPOINTS`), secrets loader, file watcher for reloads.
- Resilience: Round‑robin endpoints and exponential backoff for VictoriaMetrics; structured error logging.

### Ports / Protocols
- REST & WebSocket on `:8080`
- gRPC to AI engines: Predict `:9091`, RCA `:9092`, Alert `:9093`
- Backends (default): VM `:8481`, VL `:9428`, VT `:10428`
- Prometheus scrape: `/metrics`

### Production Hardening
- Restrict WebSocket `CheckOrigin` and CORS origins.
- Manage JWT secrets and engine endpoints via secrets/env.
- Tune per‑tenant rate limits; consider retries for VictoriaLogs.
- Enhance query validation where stricter inputs are required.

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License

## Build & Release (Multi-Platform)

MIRADOR-CORE targets Go 1.23 and builds as a statically linked binary (CGO disabled) suitable for containers. All common build workflows are available via the Makefile.

- Native build (host OS/arch):
  - `make dev-build` (debug) or `make build` (Linux/amd64 release-style)

- Cross-compile (Makefile targets):
  - Linux/amd64: `make build-linux-amd64`
  - Linux/arm64: `make build-linux-arm64`
  - macOS/arm64: `make build-darwin-arm64`
  - Windows/amd64: `make build-windows-amd64`
  - All of the above: `make build-all`

- Docker images:
  - Single-arch build (host arch): `make docker-build`
  - Native-arch build via buildx (loads into Docker): `make docker-build-native`
  - Multi-arch build (no push): `make dockerx-build` (exports `build/mirador-core-<version>.oci` archive)
  - Multi-arch build & push (amd64+arm64): `make dockerx-push`
  - Local per-arch builds (loads into Docker): `make dockerx-build-local-multi` → tags `<repo>:<version>-amd64` and `...-arm64`
  - Full release (tests + multi-arch push): `make release`

Notes
- The Makefile injects versioning via `-ldflags` (version, commit, build time).
- Regenerate protobuf stubs when proto files change: `make proto` (requires `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`).

Rancher Desktop / Docker Desktop note
- The Docker `--load` exporter cannot load multi-arch manifest lists into the local daemon. Use `make dockerx-build-local-multi` to load per-arch images, or `make dockerx-push` to publish a real multi-arch tag, or use `make dockerx-build` to export an OCI archive without pushing.

### Makefile Targets
- `make setup`: generate protobufs and download modules (first-time setup).
- `make proto`: regenerate protobuf stubs from existing `.proto` files.
- `make build`: release-style static build for Linux/amd64 (CGO disabled), outputs `bin/mirador-core`.
- `make dev-build`: local debug build for your host, outputs `bin/mirador-core-dev`.
- `make build-linux-amd64` / `make build-linux-arm64` / `make build-darwin-arm64` / `make build-windows-amd64` / `make build-all`.
- `make dev`: run the server locally (ensure dependencies are up with `make dev-stack`).
- `make docker-build`: build and tag Docker image `${REGISTRY}/${IMAGE_NAME}:${VERSION}` and `:latest`.
- `make dockerx-build` / `make dockerx-push`: multi-arch image build (with/without push) using buildx (`DOCKER_PLATFORMS` default: linux/amd64,linux/arm64).
- `make docker-publish-release VERSION=vX.Y.Z`: multi-arch build & push with SemVer fanout tags (`vX.Y.Z`, `vX.Y`, `vX`, `latest`, `stable`).
- `make docker-publish-canary` (CI): computes `0.0.0-<branch>.<date>.<sha>` and pushes that tag + `canary`.
- `make docker-publish-pr PR_NUMBER=123` (CI): pushes `0.0.0-pr.123.<sha>` + `pr-123`.
- `make release`: run tests then multi-arch build & push of `${VERSION}`.
- `make test`: run tests with race detector and coverage.
- `make clean` / `make proto-clean`: remove build artifacts and (re)generate protobufs.
- `make tools` / `make check-tools`: install and verify proto/grpc toolchain.
- `make dev-stack` / `make dev-stack-down`: start/stop local Victoria* + Redis via docker-compose.
- `make vendor`: tidy and vendor dependencies.

## Kubernetes Deployment (Helm)

The chart is bundled at `chart/`. Typical install:

```
helm upgrade --install mirador-core ./chart -n mirador --create-namespace \
  --set image.repository=platformbuilds/mirador-core \
  --set image.tag=v2.1.3
```

Enable dynamic discovery of vmselect/vlselect/vtselect pods (recommended for scale-out):

```
helm upgrade --install mirador-core ./chart -n mirador --create-namespace \
  -f - <<'VALUES'
discovery:
  vm: { enabled: true, service: vm-select.vm-select.svc.cluster.local, port: 8481, scheme: http, refreshSeconds: 30, useSRV: false }
  vl: { enabled: true, service: vl-select.vl-select.svc.cluster.local, port: 9428, scheme: http, refreshSeconds: 30, useSRV: false }
  vt: { enabled: true, service: vt-select.vt-select.svc.cluster.local, port: 10428, scheme: http, refreshSeconds: 30, useSRV: false }
VALUES
```

Headless Services for Victoria* selectors are recommended so cluster DNS exposes per-pod A records.

API Docs
- OpenAPI: `http://<host>/api/openapi.yaml`
- Swagger UI: `http://<host>/swagger`

## Production Readiness Checklist

- Security
  - JWT secret provided via secret/env; disable default secrets in non-dev (`JWT_SECRET`).
  - Lock CORS to allowed origins; tighten WebSocket `CheckOrigin` to your domains.
  - Enforce RBAC roles; validate and sanitize user input (queries) as needed.
  - Run as non-root (chart defaults) and prefer read-only root FS.

- Reliability & Scale
  - Enable discovery for vmselect/vlselect/vtselect (auto-updates endpoints on scale changes).
  - Configure probes (`/health`, `/ready`), CPU/memory requests/limits (chart defaults provided).
  - Set replicaCount>=3 for HA and define PodDisruptionBudget (add via chart if required).
  - Use Valkey/Redis cluster with proper persistence/HA; set `cache.ttl` appropriately.

- Observability
  - Scrape `/metrics` (Prometheus annotations included by default in the chart).
  - Centralize logs; consider structured log shipping from container stdout.

- Networking
  - Ingress/TLS termination at your gateway; prefer HTTP/2 for gRPC backends.
  - Rate limiting per tenant is enabled; tune thresholds as needed.

- Configuration & Secrets
  - Externalize config via Helm values or ConfigMap; secrets via Kubernetes Secrets (`envFrom`).
  - Prefer headless Services or SRV for backend discovery.

- Supply Chain
  - Build with `CGO_ENABLED=0` and minimal base image.
  - Optionally build multi-arch images with `docker buildx`.
