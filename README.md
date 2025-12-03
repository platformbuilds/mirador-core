# MIRADOR-CORE

**Advanced Observability Platform Backend** - Unified REST API Service for Metrics, Logs, and Traces

[![Version](https://img.shields.io/badge/version-v9.0.0-blue.svg)](https://github.com/platformbuilds/mirador-core/releases/tag/v9.0.0)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](LICENSE)
[![GitHub CI](https://github.com/platformbuilds/mirador-core/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/platformbuilds/mirador-core/actions/workflows/ci.yml)

## Overview

MIRADOR-CORE serves as the central orchestration layer for advanced observability platforms, providing a unified REST API that intelligently routes queries across VictoriaMetrics, VictoriaLogs, and VictoriaTraces engines. Built with Go and designed for enterprise performance, it enables seamless correlation between metrics, logs, and traces through unified endpoints.

## What MIRADOR-CORE Does

**Unified Observability Gateway**: Single API surface for all observability data types
**Intelligent Query Routing**: Automatic engine selection based on query patterns and syntax
**AI-Powered Analysis**: Root cause analysis and predictive fracture detection via gRPC engines
**Schema Management**: Centralized metadata store for metrics, labels, logs, traces, and KPIs
**High Performance**: Valkey cluster caching with auto-failover and sub-millisecond responses

## Key Features

### Core Observability
- **MetricsQL Support**: Enhanced PromQL with 150+ aggregate/transform/rollup functions
- **LogsQL Integration**: Pipe-based log analysis supporting billions of entries via Lucene/Bleve
- **Distributed Tracing**: Jaeger-compatible trace queries with flame graph generation
- **Unified Query Language (UQL)**: Cross-engine correlation queries with time-window analysis

### AI-Powered Analysis
- **MIRA (Mirador Intelligent Research Assistant)**: AI-powered translation of technical RCA output into non-technical narratives
- **Multi-Provider Support**: OpenAI, Anthropic, vLLM, and Ollama integration for flexible deployment
- **Smart Caching**: Automatic response caching with 70%+ hit rate for cost optimization
- **TOON Format**: Token-efficient data representation reducing AI token usage by 30-60%

### Performance & Reliability
- **Valkey Cluster Caching**: Distributed caching with automatic failover and TTL management
- **Horizontal Scaling**: Stateless design with load balancing and health checks
- **Circuit Breakers**: Fault tolerance for external dependencies
- **gRPC Communication**: High-performance internal service communication

## Architecture

MIRADOR-CORE implements a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                    REST API Gateway                         │
│  /unified/* | /metrics/* | /logs/* | /traces/*             │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│               Unified Query Engine                          │
│  Query Router │ Correlation Engine │ UQL Parser             │
└─────────────────────────────────────────────────────────────┘
                              │
┌──────────────┬──────────────┬──────────────┬─────────────────┐
│VictoriaMetrics│ VictoriaLogs │VictoriaTraces│  AI Engines     │
│   (Metrics)   │   (Logs)     │   (Traces)   │  (RCA/Predict) │
└──────────────┴──────────────┴──────────────┴─────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│           Infrastructure Layer                              │
│  Valkey Cluster │ Weaviate │ Monitoring                     │
└─────────────────────────────────────────────────────────────┘
```

### Core Components
- **Query Router**: Intelligent routing based on query syntax and patterns
- **Correlation Engine**: Cross-engine analysis with temporal and causal relationships
- **Schema Repository**: Centralized metadata management via Weaviate
- **Cache Layer**: Distributed caching with Valkey cluster integration



## Quick Start

### Prerequisites
- **Go 1.21+**: For building from source
- **Docker**: For containerized development and testing
- **VictoriaMetrics Ecosystem**: VM (metrics), VL (logs), VT (traces) clusters
- **Weaviate**: Vector database for schema storage
- **Valkey**: Cluster for caching

### Development Setup

1. **Clone and Start Environment**
```bash
git clone https://github.com/platformbuilds/mirador-core
cd mirador-core

# Start complete containerized development environment
make localdev-up
make localdev-wait  # Wait for services to be ready
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

The primary interface for executing queries across all observability engines:

```bash
# Execute unified query with intelligent routing
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -d '{"query": "rate(http_requests_total[5m])", "engines": ["metrics"]}'
```

**Complete API Documentation**: [Swagger UI](http://localhost:8010/swagger/) | [OpenAPI Spec](./api/openapi.yaml)
