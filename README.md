# MIRADOR-CORE

**Advanced Observability Platform Backend** - Unified REST API Service for Metrics, Logs, and Traces

[![Version](https://img.shields.io/badge/version-v9.0.0-blue.svg)](https://github.com/platformbuilds/mirador-core/releases/tag/v9.0.0)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](LICENSE)
[![GitHub CI](https://github.com/platformbuilds/mirador-core/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/platformbuilds/mirador-core/actions/workflows/ci.yml)
[![Documentation](https://img.shields.io/badge/docs-readthedocs-blue.svg)](https://mirador-core.readthedocs.io/)

## Overview

MIRADOR-CORE serves as the central orchestration layer for advanced observability platforms, providing a unified REST API that intelligently routes queries across VictoriaMetrics, VictoriaLogs, and VictoriaTraces engines. Built with Go and designed for enterprise performance, it enables seamless correlation between metrics, logs, and traces through unified endpoints.

### The Four Pillars of Mirador Core

```
┌──────────┐     ┌──────────┐     ┌─────────────┐     ┌──────┐
│   KPIs   │ --> │ Failures │ --> │ Correlation │ --> │ RCA  │
└──────────┘     └──────────┘     └─────────────┘     └──────┘
   Define           Detect          Analyze           Explain
   Metrics          Incidents       Patterns          Root Cause
```

1. **KPI Management**: Define and manage key performance indicators across your infrastructure
2. **Failure Detection**: Automatically detect incidents based on KPI anomalies and error signals
3. **Correlation Analysis**: Perform statistical analysis to find relationships between KPIs
4. **Root Cause Analysis (RCA)**: Use correlation data + 5 WHY methodology to identify root causes

## What MIRADOR-CORE Does

- **Unified Observability Gateway**: Single API surface for all observability data types
- **Intelligent Query Routing**: Automatic engine selection based on query patterns and syntax
- **KPI-Driven Monitoring**: Comprehensive KPI management with failure detection and correlation
- **AI-Powered RCA**: Root cause analysis with MIRA (Mirador Intelligent Research Assistant)
- **Schema Management**: Centralized metadata store for metrics, labels, logs, traces, and KPIs
- **High Performance**: Valkey cluster caching with auto-failover and sub-millisecond responses

## Key Features

### Core Observability
- **MetricsQL Support**: Enhanced PromQL with 150+ aggregate/transform/rollup functions
- **LogsQL Integration**: Pipe-based log analysis supporting billions of entries via Lucene/Bleve
- **Distributed Tracing**: Jaeger-compatible trace queries with flame graph generation
- **Unified Query Language (UQL)**: Cross-engine correlation queries with time-window analysis
- **Unified Query API**: Single endpoint for metrics, logs, and traces with intelligent routing

### KPI & Failure Management
- **Comprehensive KPI Repository**: Define, manage, and search KPIs with vector storage (Weaviate)
- **Bulk Import/Export**: CSV and JSON bulk operations for KPI management
- **Failure Detection**: Automated incident detection based on KPI thresholds and anomalies
- **Correlation Engine**: Statistical analysis (Pearson, Spearman, cross-correlation) for KPI relationships
- **Time-Window Analysis**: Configurable rings/buckets for temporal correlation

### AI-Powered Analysis
- **MIRA (Mirador Intelligent Research Assistant)**: AI-powered translation of technical RCA output into non-technical narratives
- **Multi-Provider Support**: OpenAI, Anthropic, vLLM, and Ollama integration for flexible deployment
- **Smart Caching**: Automatic response caching with 70%+ hit rate for cost optimization
- **5 WHY Methodology**: Automated root cause chain generation with evidence and narrative

### Performance & Reliability
- **Valkey Cluster Caching**: Distributed caching with automatic failover and TTL management
- **Horizontal Scaling**: Stateless design with load balancing and health checks
- **Circuit Breakers**: Fault tolerance for external dependencies
- **Sub-millisecond Responses**: Optimized query execution with caching

## Architecture

MIRADOR-CORE implements a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                   Mirador Core API                          │
│  /unified/* | /kpi/* | /correlation/* | /rca/*             │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
    ┌───▼────┐      ┌──────▼───────┐     ┌────▼─────┐
    │  KPI   │      │  Correlation │     │   RCA    │
    │  Repo  │      │   Engine     │     │  Engine  │
    └───┬────┘      └──────┬───────┘     └────┬─────┘
        │                  │                   │
        │          ┌───────▼────────┐          │
        │          │ Failure        │          │
        └──────────► Detection      ◄──────────┘
                   └────────┬───────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
   ┌────▼────┐      ┌──────▼───────┐    ┌─────▼─────┐
   │Victoria │      │ Victoria     │    │ Victoria  │
   │ Metrics │      │ Logs         │    │ Traces    │
   └─────────┘      └──────────────┘    └───────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
   ┌────▼────┐      ┌──────▼───────┐    ┌─────▼─────┐
   │ Valkey  │      │  Weaviate    │    │   MIRA    │
   │ (Cache) │      │  (KPI Store) │    │ (AI/RCA)  │
   └─────────┘      └──────────────┘    └───────────┘
```

### Core Components
- **Unified Query Router**: Intelligent routing based on query syntax and patterns (MetricsQL, LogsQL, UQL)
- **KPI Repository**: Vector-backed KPI storage with search, filtering, and bulk operations
- **Correlation Engine**: Statistical analysis with configurable rings/buckets and multiple correlation methods
- **RCA Engine**: Root cause analysis using 5 WHY methodology with AI narrative generation
- **Failure Detection**: Automated incident detection from KPI anomalies and error signals
- **Cache Layer**: Distributed caching with Valkey cluster for sub-millisecond responses
- **MIRA Integration**: External AI service for natural language RCA explanations



## Quick Start

### Prerequisites
- **Go 1.21+**: For building from source
- **Docker**: For containerized development and testing

# Seed sample KPIs (optional)
make localdev-seed-data

# Seed OpenTelemetry data (optional)
make localdev-seed-otel
```

2. **Verify Installation**
```bash
# Check health
curl http://localhost:8010/api/v1/health

# Expected response:
# {
#   "status": "healthy",
#   "timestamp": "2026-01-23T10:00:00Z",
#   "services": {
#     "mirador-core": "ok",
#     "victoriametrics": "ok",
#     "victorialogs": "ok",
#     "victoriatraces": "ok",
#     "valkey": "ok"
#   }
# }
```

3. **Access the API**
- **REST API**: http://localhost:8010/api/v1/
- **Swagger UI**: http://localhost:8010/swagger/index.html
- **OpenAPI Spec**: http://localhost:8010/api/openapi.yaml
- **Health Check**: http://localhost:8010/health
- **Prometheus Metrics**: http://localhost:8010/metrics
```

2. **Verify Installation**
```bash
# Check health
curl http://localhost:8010/api/v1/health

# Run comprehensive tests
make localdev-test
```

3. **Access the API**
- **REST API**: http://localhost:8010/api/v1/
- **Swagger UI**: http://localhost:8010/swagger/index.html

### End-to-End (E2E) Pipeline

This repository provides a dedicated E2E pipeline for core API coverage (Config, KPI, UQL, Correlation, RCA). The single entry point is the `make e2e` target, which:

- Starts the `localdev` Docker stack (if not already running)
- Seeds OpenTelemetry telemetry via `make localdev-seed-otel`
- Runs Go e2e tests (build tag `e2e`) and API smoke checks
- Runs `golangci-lint` for code quality validation

Run locally:

```bash
# Start localdev and wait for readiness
make localdev-up
make localdev-wait

# Seed OTEL data (synthetic telemetry)
make localdev-seed-otel

# Run the E2E pipeline
make e2e
```

The Go tests are found in `internal/api/*_e2e_test.go` and run via `go test -tags e2e` to avoid interfering with default unit tests.

### Building from Source

```bash
# Install dependencies and build
make setup
make build

# Run locally (requires external services)
./bin/server

# Or with Docker
make docker-build
docker run -p 8010:8010 platformbuilds/mirador-core:latest
```

## Configuration

### Environment Variables

**Core Settings:**
```bash
PORT=8010                    # Server port
ENVIRONMENT=production       # Environment mode
LOG_LEVEL=info              # Logging level

# VictoriaMetrics ecosystem
VM_ENDPOINTS=vm-cluster:8481
VL_ENDPOINTS=vl-cluster:9428  
VT_ENDPOINTS=vt-cluster:10428

# Caching and storage
VALKEY_CACHE_NODES=valkey-1:6379,valkey-2:6379
WEAVIATE_HOST=weaviate-cluster
WEAVIATE_PORT=8080


```

### Configuration Files

MIRADOR-CORE uses YAML configuration with environment variable overrides:

- `configs/config.yaml` - Base configuration
- `configs/config.development.yaml` - Development overrides  
- `configs/config.production.yaml` - Production overrides

Key configuration sections include database sources, unified query options, and performance tuning parameters.

## API Reference

### Unified Query API

Execute queries across metrics, logs, and traces with a single endpoint:

```bash
# Metrics query (MetricsQL)
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "req-123",
      "type": "metrics",
      "query": "rate(http_requests_total[5m])",
      "start_time": "2026-01-23T10:00:00Z",
      "end_time": "2026-01-23T10:05:00Z"
    }
  }'

# Logs query (LogsQL)
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "type": "logs",
      "query": "error | json | severity:error",
      "start_time": "2026-01-23T10:00:00Z",
      "end_time": "2026-01-23T10:05:00Z"
    }
  }'
```

### KPI Management

Manage key performance indicators with full CRUD operations:

```bash
# Create KPI
curl -X POST http://localhost:8010/api/v1/kpi/defs \
  -H "Content-Type: application/json" \
  -d '{
    "id": "http_errors",
    "name": "HTTP Errors",
    "kind": "tech",
    "definition": "Rate of HTTP 5xx errors",
    "query": {"promql": "rate(http_requests_total{status=~\"5..\"}[5m])"}
  }'

# Search KPIs
curl -X POST http://localhost:8010/api/v1/kpi/search \
  -H "Content-Type: application/json" \
  -d '{"query": "http errors", "mode": "semantic", "limit": 10}'

# Bulk import KPIs
curl -X POST http://localhost:8010/api/v1/kpi/defs/bulk-json \
  -H "Content-Type: application/json" \
  -d @kpis.json
```

### Correlation Analysis

Analyze relationships between KPIs over time windows:

```bash
# Run correlation analysis
curl -X POST http://localhost:8010/api/v1/unified/correlation \
  -H "Content-Type: application/json" \
  -d '{
    "startTime": "2026-01-23T10:00:00Z",
    "endTime": "2026-01-23T11:00:00Z"
  }'

# Response includes:
# - Correlated KPI pairs with Pearson/Spearman coefficients
# - Time-lag analysis
# - Bucket-level aggregations
# - Suspicion scores
```

### Root Cause Analysis (RCA)

Perform automated root cause analysis using 5 WHY methodology:

```bash
# Run RCA
curl -X POST http://localhost:8010/api/v1/unified/rca \
  -H "Content-Type: application/json" \
  -d '{
    "startTime": "2026-01-23T10:00:00Z",
    "endTime": "2026-01-23T11:00:00Z"
  }'

# Response includes:
# - Root cause chain (5 WHYs)
# - Supporting evidence
# - AI-generated narrative (via MIRA)
# - Impact analysis
```

## Documentation

**Complete Documentation**: [ReadTheDocs](https://mirador-core.readthedocs.io/)

Key documentation sections:
- **[User Guide](docs/kpi-failures-correlation-rca-user-guide.md)**: Comprehensive guide for KPI, Failures, Correlation, and RCA
- **[Getting Started](docs/getting-started.md)**: Quick start and development setup
- **[Deployment Guide](docs/deployment.md)**: Production deployment with Kubernetes/Helm
- **[Unified Query](docs/unified-query.md)**: Unified Query API reference
- **[Configuration](docs/configuration.md)**: Configuration options and environment variables
- **[API Documentation](http://localhost:8010/swagger/)**: Interactive Swagger UI

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/platformbuilds/mirador-core/issues)
- **Documentation**: [ReadTheDocs](https://mirador-core.readthedocs.io/)
- **API Reference**: [Swagger UI](http://localhost:8010/swagger/)
