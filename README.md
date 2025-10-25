<!-- CI Status & Reports (GitHub + GitLab) -->

<!-- GitHub Actions -->
[![GitHub CI](https://github.com/platformbuilds/mirador-core/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/platformbuilds/mirador-core/actions/workflows/ci.yml)
[![CodeQL](https://github.com/platformbuilds/mirador-core/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/platformbuilds/mirador-core/actions/workflows/codeql.yml)
[![Coverage (artifacts)](https://img.shields.io/badge/coverage-see%20artifacts-informational)](https://github.com/platformbuilds/mirador-core/actions/workflows/ci.yml)
[![Govulncheck](https://img.shields.io/badge/govulncheck-report-informational)](https://github.com/platformbuilds/mirador-core/actions/workflows/ci.yml)

<!-- GitLab CI (optional mirror) -->
[![Pipeline Status](https://gitlab.com/platformbuilds/mirador-core/badges/main/pipeline.svg)](#https://gitlab.com/platformbuilds/mirador-core/-/pipelines?scope=branches&ref=main)
[![Coverage](https://gitlab.com/platformbuilds/mirador-core/badges/main/coverage.svg)](#https://gitlab.com/platformbuilds/mirador-core/-/graphs/main/charts)
[![Vulnerability Report](https://img.shields.io/badge/GitLab%20Vulnerabilities-Report-blue)](#https://gitlab.com/platformbuilds/mirador-core/-/security/vulnerabilities)
[![Test Report](https://img.shields.io/badge/GitLab%20Tests-Latest%20Pipeline-lightgrey)](#https://gitlab.com/platformbuilds/mirador-core/-/pipelines?scope=branches&ref=main)

# MIRADOR-CORE

**Advanced Observability Platform Backend** - Unified REST API Service for Metrics, Logs, and Traces

[![Version](https://img.shields.io/badge/version-v7.0.0-blue.svg)](https://github.com/platformbuilds/mirador-core/releases/tag/v7.0.0)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](LICENSE)

## Overview

MIRADOR-CORE serves as the central orchestration layer for the MIRADOR observability platform, providing a unified REST API that intelligently routes queries across VictoriaMetrics, VictoriaLogs, and VictoriaTraces engines. Built with Go and designed for high performance, it enables seamless correlation between metrics, logs, and traces through a single endpoint.

## Current Version: v7.0.0 - Unified Observability Platform 🆕

### 🚀 Major Improvements

- **Unified Query API**: Single endpoint (`/api/v1/unified/*`) with intelligent routing across all data types
- **Cross-Engine Correlation**: Query logs, metrics, and traces together with unified syntax
- **Enhanced Caching**: Valkey cluster integration with TTL-based result caching
- **Schema Definitions Store**: Weaviate-powered metadata storage for metrics, logs, and traces
- **Performance Optimizations**: 10x RAM reduction and sub-millisecond query responses

### 📊 Query Capabilities

```bash
# Unified query across all engines
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -d '{"query": {"type": "correlation", "query": "logs:error AND metrics:high_latency"}}'

# Intelligent routing - no need to know which engine to query
curl -X POST https://mirador-core/api/v1/unified/query \
  -d '{"query": {"type": "auto", "query": "service:api AND level:error"}}'
```

### 🏗️ Architecture Enhancements

- **Unified Query Engine**: Intelligent routing based on query patterns and content
- **Correlation Engine**: Parallel execution across multiple engines with result merging
- **Schema Registry**: Centralized definitions for metrics, labels, and log fields
- **Enhanced Security**: RBAC improvements and tenant isolation

## Project Status

### ✅ Completed Phases

- **Phase 1**: Foundation & Architecture ✓
- **Phase 1.5**: Unified API Implementation ✓
- **Phase 2**: Metrics Metadata Integration (In Progress)
- **Phase 3**: Log-Metrics-Traces Correlation Engine (Planned)
- **Phase 4**: Performance & Caching (Planned)
- **Phase 5**: Unified Query Language (Planned)
- **Phase 6**: Monitoring & Observability (Planned)
- **Phase 7**: Testing & Quality Assurance (Planned)
- **Phase 8**: Documentation & Adoption (Planned)

### 🎯 Current Focus (Phase 2)

- **Metrics Metadata Indexing**: Index metric definitions in Bleve for enhanced discovery
- **Search API**: `/api/v1/metrics/search` endpoint for metric exploration
- **Metadata Synchronization**: Keep definitions in sync between VictoriaMetrics and Bleve
- **Discovery Capabilities**: Fuzzy search and auto-completion for metric names

### 📈 Quality Gates

- **API Functionality**: All unified endpoints functional with E2E tests
- **Performance**: Unified queries within 200% of individual engine performance
- **Correlation Accuracy**: >95% accurate results across time windows
- **Backward Compatibility**: All existing APIs remain functional

## Key Features

### 🧠 AI-Powered Analysis
- **PREDICT-ENGINE**: System fracture/fatigue prediction using ML models
- **RCA-ENGINE**: Root cause analysis with red anchors correlation patterns
- **ALERT-ENGINE**: Intelligent alert management with noise reduction

### 📊 Unified Observability
- **MetricsQL**: Enhanced PromQL with 150+ aggregate functions
- **LogsQL**: Pipe-based log analysis supporting billions of entries
- **VictoriaTraces**: Distributed tracing with Jaeger compatibility
- **Dual Search Engines**: Choose between Lucene and Bleve for logs/traces

### 🚀 Enterprise Performance
- **10x RAM Reduction**: Optimized memory usage vs traditional solutions
- **Valkey Cluster Caching**: Sub-millisecond query responses
- **Horizontal Scaling**: Load balancing and stateless design
- **gRPC Communication**: Efficient internal service communication

### 🔒 Enterprise Security
- **LDAP/AD Integration**: Enterprise authentication
- **OAuth 2.0 Support**: Modern identity provider integration
- **RBAC**: Role-based access control with fine-grained permissions
- **Multi-Tenant**: Complete data isolation between tenants

## Quick Start

### Prerequisites
- Go 1.21+
- Docker & Kubernetes (for full deployment)
- VictoriaMetrics ecosystem (VM, VL, VT)
- Valkey/Redis Cluster (optional, for caching)

### Development Setup
```bash
# Clone repository
git clone https://github.com/platformbuilds/mirador-core
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
make docker-build

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

### Unified Query API (v7.0.0) 🆕

MIRADOR-CORE v7.0.0 introduces the **Unified Query API**, enabling intelligent routing across logs, metrics, traces, and correlation queries through a single endpoint.

#### Core Endpoints

```bash
# Execute unified query (intelligent routing)
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "unified-query-1",
      "type": "metrics",
      "query": "http_requests_total{job=\"api\"}",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z",
      "timeout": "30s",
      "cache_options": {
        "enabled": true,
        "ttl": "5m"
      }
    }
  }'

# Get query capabilities and supported engines
curl -X GET https://mirador-core/api/v1/unified/metadata \
  -H "Authorization: Bearer <token>"

# Get health status of all engines
curl -X GET https://mirador-core/api/v1/unified/health \
  -H "Authorization: Bearer <token>"
```

#### Query Types Supported
- **Metrics**: MetricsQL queries routed to VictoriaMetrics
- **Logs**: LogsQL queries routed to VictoriaLogs (Lucene/Bleve)
- **Traces**: Trace queries routed to VictoriaTraces
- **Correlation**: Cross-engine correlation queries (future implementation)

#### Unified Query Features
- **Intelligent Routing**: Automatic engine selection based on query patterns
- **Caching**: Configurable TTL-based result caching with Valkey
- **Cross-Engine Correlation**: Future support for complex correlation queries
- **Unified Response Format**: Consistent JSON responses across all query types
- **Performance Monitoring**: Built-in metrics and execution time tracking

### MetricsQL Aggregate Functions (v5.0.0+)

MIRADOR-CORE supports comprehensive MetricsQL aggregate functions for advanced time series analysis:

```bash
# Sum aggregation
curl -X POST https://mirador-core/api/v1/metrics/query/aggregate/sum \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "rate(http_requests_total[5m])"}'

# Quantile with parameter
curl -X POST https://mirador-core/api/v1/metrics/query/aggregate/quantile \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "rate(http_requests_total[5m])", "params": {"quantile": 0.95}}'
```

**Available Functions**: `sum`, `avg`, `count`, `min`, `max`, `median`, `stddev`, `stdvar`, `mad`, `zscore`, `skewness`, `kurtosis`, `topk`, `bottomk`, `quantile`, `percentile`, `histogram`, `distinct`, `count_values`, `mode`, `mode_multi`, `cov`, `corr`, `entropy`, `range`, `iqr`, `trimean`, `increase`, `rate`, `irate`, `delta`, `idelta`, `geomean`, `harmean`

### Search Engines (v5.1.0+)

#### Lucene Query Syntax
Full Lucene support for logs and traces with familiar syntax:

```bash
# Logs with Lucene
curl -X POST https://mirador-core/api/v1/logs/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "level:error AND message:\"connection timeout\"", "time_range": "1h"}'

# Traces with Lucene
curl -X POST https://mirador-core/api/v1/traces/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "service:payment AND operation:charge", "time_range": "1h"}'
```

#### Bleve Search Engine (v6.0.0+)
Alternative search engine with fuzzy matching capabilities:

```bash
# Specify Bleve engine
curl -X POST https://mirador-core/api/v1/logs/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "error AND status:500",
    "search_engine": "bleve",
    "time_range": "1h"
  }'
```

### AI Analysis APIs

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

# Start RCA investigation
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

### Schema Definitions APIs (v7.0.0+)

Manage metadata for metrics, logs, and traces:

```bash
# Upsert metric definition
curl -X POST https://mirador-core/api/v1/schema/metrics \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "metric": "http_requests_total",
    "description": "Total number of HTTP requests",
    "owner": "platform-team",
    "tags": ["domain:web", "owner:platform"]
  }'

# Bulk upload via CSV
curl -X POST https://mirador-core/api/v1/schema/metrics/bulk \
  -H "Authorization: Bearer <token>" \
  -F "file=@metrics.csv"
```

## Configuration

### Environment Variables
```bash
# Core settings
export PORT=8010
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

# Schema store
export WEAVIATE_ENABLED=true
export WEAVIATE_HOST=weaviate
export WEAVIATE_PORT=8080
```

### Multi-Source Aggregation

Configure fan-out queries across multiple backend clusters:

```yaml
database:
  # Primary VictoriaMetrics cluster
  victoria_metrics:
    endpoints: ["http://vm-fin-0:8481", "http://vm-fin-1:8481"]
    timeout: 30000

  # Additional metrics sources
  metrics_sources:
    - name: os_metrics
      endpoints: ["http://vm-os-0:8481"]
      timeout: 30000

  # Primary logs cluster
  victoria_logs:
    endpoints: ["http://vl-fin-0:9428", "http://vl-fin-1:9428"]
    timeout: 30000

  # Additional logs sources
  logs_sources:
    - name: os_logs
      endpoints: ["http://vl-os-0:9428"]
      timeout: 30000
```

### Unified Query Configuration

```yaml
unified_query:
  enabled: true
  default_timeout: "30s"
  cache:
    enabled: true
    default_ttl: "5m"
    max_ttl: "1h"
  routing:
    metrics_engine: "victoriametrics"
    logs_engine: "victorialogs"
    traces_engine: "victoriatraces"
```

## Schema Definitions APIs

These APIs allow defining metric definitions (and label definitions) and log field definitions for contextualization and future LLM use.

Routes (under `/api/v1`):

- Metrics definitions
  - `POST /schema/metrics` — upsert metric definition
    - Body: `{ tenantId?, metric, description?, owner?, tags?, author? }`
  - `GET /schema/metrics/{metric}` — get current definition
  - `GET /schema/metrics/{metric}/versions` — list version metadata
  - `GET /schema/metrics/{metric}/versions/{version}` — fetch specific version payload
  - `POST /schema/metrics/bulk` — bulk upsert via CSV (secure upload)
    - Required header and columns:
      - `tenant_id` (optional; defaults to request tenant)
      - `metric` (required)
      - `description`, `owner`, `tags_json` (JSON array of strings)
      - `label`, `label_type`, `label_required`, `label_allowed_json`, `label_description`
      - `author`
    - Tags note: All schema `tags` are flat arrays of strings. In CSV, `tags_json` must be a JSON array of strings. Example: `["domain:web", "owner:team-observability"]`.
    - Security controls: 5MiB limit, MIME allowlist, UTF‑8 validation, CSV injection mitigation, in‑memory only (no disk writes)
  - `GET /schema/metrics/bulk/sample` — download a sample CSV template
    - Optional: `?metrics=http_requests_total,process_cpu_seconds_total` pre-fills rows for listed metrics with discovered label keys

- Label definitions (for a metric)
  - Included in the metric upsert flow; label CRUD can be added similarly if needed.

- Log field definitions
  - `POST /schema/logs/fields` — upsert log field definition
    - Body: `{ tenantId?, field, type?, description?, tags?, examples?, author? }`
  - `GET /schema/logs/fields/{field}` — get current definition
  - `GET /schema/logs/fields/{field}/versions` — list versions
  - `GET /schema/logs/fields/{field}/versions/{version}` — fetch version payload
  - `POST /schema/logs/fields/bulk` — bulk upsert via CSV (secure upload)
    - Columns: `tenant_id, category, logfieldname, logfieldtype, logfielddefinition, sentiment, tags_json (JSON array), examples_json, author`
    - Tags note: `tags_json` must be a JSON array of strings. Example: `["category:security", "format:json", "indexed:true"]`.
    - Security: 5MiB limit, MIME allowlist, UTF‑8 validation, CSV injection mitigation, daily per‑tenant quota
  - `GET /schema/logs/fields/bulk/sample` — download a sample CSV template (one row per discovered log field)

- Traces schema (services & operations)
  - Services
    - `POST /schema/traces/services` — upsert trace service definition
      - Body: `{ tenantId?, service, purpose?, owner?, tags?, author? }`
    - `GET /schema/traces/services/{service}` — get current definition
    - `GET /schema/traces/services/{service}/versions` — list version metadata
    - `GET /schema/traces/services/{service}/versions/{version}` — fetch specific version payload
    - `POST /schema/traces/services/bulk` — bulk upsert via CSV (secure upload)
      - Columns: `tenant_id, service, purpose, owner, tags_json (JSON array), author`
      - Tags note: `tags_json` must be a JSON array of strings. Example: `["environment:production", "team:platform"]`.
      - Security: 5MiB limit, MIME allowlist + sniffing, UTF‑8 validation, CSV injection mitigation, header strict mode (reject unknown columns), 10k row cap, in‑memory only (no disk writes), per‑tenant daily quota (429)
  - Operations
    - `POST /schema/traces/operations` — upsert trace operation definition
      - Body: `{ tenantId?, service, operation, purpose?, owner?, tags?, author? }`
    - `GET /schema/traces/services/{service}/operations/{operation}` — get current definition
    - `GET /schema/traces/services/{service}/operations/{operation}/versions` — list version metadata
    - `GET /schema/traces/services/{service}/operations/{operation}/versions/{version}` — fetch specific version payload
    - `POST /schema/traces/operations/bulk` — bulk upsert via CSV (secure upload)
      - Columns: `tenant_id, service, operation, purpose, owner, tags_json (JSON array), author`
      - Tags note: `tags_json` must be a JSON array of strings. Example: `["method:GET", "endpoint:/api/v1/users"]`.
      - Security: 5MiB limit, MIME allowlist + sniffing, UTF‑8 validation, CSV injection mitigation, header strict mode (reject unknown columns), 10k row cap, in‑memory only (no disk writes), per‑tenant daily quota (429). Each row must reference an existing service (operations are per service).

- Labels (independent)
  - `POST /schema/labels` — upsert label definition (not tied to a metric)
    - Body: `{ tenantId?, name, type?, required?, allowedValues?, description?, author? }`
  - `GET /schema/labels/{name}` — get current label definition
  - `GET /schema/labels/{name}/versions` — list version metadata
  - `GET /schema/labels/{name}/versions/{version}` — fetch specific version payload
  - `DELETE /schema/labels/{name}` — delete label definition
  - `POST /schema/labels/bulk` — bulk upsert via CSV (secure upload)
    - Columns: `tenant_id, name, type, required, allowed_json, description, author`
    - Tags note: `allowed_json` is a JSON object of constraints or allowed values
    - Security: 5MiB limit, MIME allowlist, UTF‑8 validation, CSV injection mitigation, daily per‑tenant quota
  - `GET /schema/labels/bulk/sample` — download a sample CSV template for labels

Configuration: Bulk CSV Upload Size Limit
- Config key: `uploads.bulk_max_bytes` (bytes). Default 5 MiB.
- Ways to set:
  - Helm values (`chart/values.yaml` → `.Values.mirador.uploads.bulk_max_bytes`), templated into `/etc/mirador/config.yaml`.
  - Env vars: `BULK_UPLOAD_MAX_BYTES` or `BULK_UPLOAD_MAX_MIB` (takes precedence over file).
  - Local dev compose sets `BULK_UPLOAD_MAX_BYTES` by default; adjust as needed.

## Logs (Lucene) & Traces

- Logs APIs accept Lucene using `query_language: "lucene"` and a Lucene expression in `query`.
  - Examples:
    - Instant logs query: `POST /api/v1/logs/query` with `{ "query_language": "lucene", "query": "_time:15m level:error service:web" }`
    - Range D3 endpoints: `GET /api/v1/logs/histogram?query_language=lucene&query=_time:30m&step=60000`
    - Export: `POST /api/v1/logs/export` with `{ "query_language": "lucene", "query": "_time:5m", "format": "csv" }`

- Traces are Jaeger-compatible for retrieval (services, operations, search, flamegraph).
  - To discover trace IDs with Lucene, first search logs with a Lucene filter on `trace_id`, then fetch traces by ID:
    1) `POST /api/v1/logs/search` with `{ "query_language": "lucene", "query": "_time:15m trace_id:*" }`
    2) `GET /api/v1/traces/{traceId}` or `POST /api/v1/traces/search` using Jaeger filters (service, operation, tags, durations).

## MetricsQL Enrichment (Definitions)

The query endpoints can include definitions for metrics and labels, sourced from the schema store:

- `POST /api/v1/metrics/query`
- `POST /api/v1/metrics/query_range`

Optional controls (body or query params):

- `include_definitions` (bool, default true): return definitions when true.
- `definitions_minimal` (bool, default false): only include metric-level definitions, skip label definitions.
- `label_keys` (array in body or CSV in query): restrict label keys to consider.

Response shape when enabled:

```
definitions:
  metrics:
    <metricName>: { ...MetricDef or placeholder... }
  labels:
    <metricName>:
      <labelKey>: { ...LabelDef or placeholder... }
```

Placeholders indicate no definition has been provided yet and reference the schema APIs to create one.

Schema Tags format
- All schema APIs now use flat arrays of strings for `tags` (not key/value maps).
- Request bodies: supply `tags` as an array of strings, e.g. `["domain:web", "owner:platform"]`.
- Bulk CSV: the `tags_json` column must contain a JSON array of strings.

## Vulnerability Scanning

- Run a local vulnerability scan using Go's official tool:
  - `make vuln` (installs `govulncheck` if missing, then scans `./...`)
- CI runs `govulncheck` as part of `.github/workflows/ci.yml` after build and tests.
- Notes:
  - Requires network access to fetch vulnerability database.
  - Scans source and modules to flag known CVEs and advisories.

## Development

### Local Development Setup

1. **Prerequisites**
   - Go 1.21+
   - Docker & Docker Compose
   - Make

2. **Clone and Setup**
   ```bash
   git clone https://github.com/platformbuilds/mirador-core
   cd mirador-core
   make setup
   ```

3. **Start Local Stack**
   ```bash
   make dev-stack  # VictoriaMetrics, Valkey, Weaviate
   make dev        # Start mirador-core server
   ```

4. **Run Tests**
   ```bash
   make test       # Unit tests
   make localdev   # Full E2E test suite
   ```

### Development Commands

- `make dev-build` - Build debug binary
- `make dev` - Run server with hot reload
- `make proto` - Regenerate protobuf files
- `make vuln` - Run vulnerability scan
- `make test` - Run unit tests with coverage

## Deployment

### Docker Deployment

```bash
# Build single architecture
make docker-build

# Build multi-architecture
make dockerx-build

# Run locally
docker run -p 8010:8010 platformbuilds/mirador-core:latest
```

### Kubernetes (Helm)

```bash
# Add repository
helm repo add mirador https://platformbuilds.github.io/mirador-core
helm repo update

# Install
helm install mirador-core mirador/mirador-core \
  --set image.tag=v7.0.0 \
  --set vm.endpoints="vm-select:8481" \
  --set vl.endpoints="vl-select:9428"
```

### Configuration

Environment variables and Helm values for production deployment:

```yaml
# VictoriaMetrics ecosystem
VM_ENDPOINTS: "vm-cluster-1:8481,vm-cluster-2:8481"
VL_ENDPOINTS: "vl-cluster-1:9428,vl-cluster-2:9428"
VT_ENDPOINTS: "vt-cluster-1:10428,vt-cluster-2:10428"

# Caching
VALKEY_CACHE_NODES: "redis-1:6379,redis-2:6379"
CACHE_TTL: "300"

# Authentication
LDAP_URL: "ldap://ldap.company.com"
RBAC_ENABLED: "true"

# Schema store
WEAVIATE_ENABLED: "true"
WEAVIATE_HOST: "weaviate"
```

## Monitoring

### Prometheus Metrics

MIRADOR-CORE exposes comprehensive metrics at `/metrics`:

- **HTTP Metrics**: `mirador_core_http_requests_total`, `mirador_core_http_duration_seconds`
- **gRPC Metrics**: `mirador_core_grpc_requests_total`, `mirador_core_grpc_duration_seconds`
- **Cache Metrics**: `mirador_core_cache_hits_total`, `mirador_core_cache_misses_total`
- **Session Metrics**: `mirador_core_sessions_active`, `mirador_core_sessions_created_total`
- **AI Metrics**: `mirador_core_predictions_generated_total`, `mirador_core_rca_investigations_total`

### Health Checks

- `/health` - Basic health check
- `/ready` - Readiness probe for Kubernetes
- `/metrics` - Prometheus metrics endpoint

### Logging

Structured JSON logging with configurable levels:

```json
{
  "level": "info",
  "timestamp": "2025-01-01T12:00:00Z",
  "service": "mirador-core",
  "request_id": "req-123",
  "message": "Query executed successfully",
  "duration_ms": 150,
  "query_type": "metrics"
}
```

## Security

### Authentication & Authorization

- **LDAP/AD Integration**: Enterprise directory authentication
- **OAuth 2.0**: Modern identity provider support
- **JWT Tokens**: Stateless authentication with configurable expiration
- **RBAC**: Role-based access control with fine-grained permissions

### Security Features

- **Rate Limiting**: Per-tenant request throttling
- **CORS**: Configurable cross-origin resource sharing
- **Input Validation**: Comprehensive query sanitization
- **Audit Logging**: Security event tracking
- **TLS**: End-to-end encryption support

### Production Security Checklist

- ✅ JWT secrets configured via environment/secrets
- ✅ CORS restricted to allowed origins
- ✅ RBAC roles properly configured
- ✅ Input validation enabled
- ✅ TLS certificates configured
- ✅ Security headers added
- ✅ Rate limiting tuned per tenant

## Contributing

### Development Workflow

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feature/amazing-feature`
3. **Make** your changes with tests
4. **Run** tests: `make test`
5. **Commit** changes: `git commit -m 'Add amazing feature'`
6. **Push** to branch: `git push origin feature/amazing-feature`
7. **Open** Pull Request

### Code Standards

- **Go**: Follow standard Go conventions and `gofmt`
- **Testing**: 80%+ code coverage required
- **Documentation**: Update README and API docs for changes
- **Security**: Run `make vuln` before submitting PRs

### Commit Guidelines

- Use conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`
- Keep commits focused and atomic
- Reference issues: `Fixes #123`

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Support

### Community Support

- **GitHub Issues**: Bug reports and feature requests
- **Discussions**: General questions and community help
- **Documentation**: Comprehensive guides and API reference

### Enterprise Support

- **Professional Services**: Custom development and consulting
- **Training**: MIRADOR platform training and certification
- **SLA**: Enterprise-grade support agreements

### Resources

- **Documentation**: https://mirador-core.readthedocs.io/
- **API Reference**: https://mirador-core.github.io/api/
- **Community Forum**: https://github.com/platformbuilds/mirador-core/discussions
