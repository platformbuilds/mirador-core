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

[![Version](https://img.shields.io/badge/version-v9.0.0-blue.svg)](https://github.com/platformbuilds/mirador-core/releases/tag/v9.0.0)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](LICENSE)

## Overview

MIRADOR-CORE serves as the central orchestration layer for the MIRADOR observability platform, providing a unified REST API that intelligently routes queries across VictoriaMetrics, VictoriaLogs, and VictoriaTraces engines. Built with Go and designed for high performance, it enables seamless correlation between metrics, logs, and traces through a single endpoint.

## Current Version: v9.0.0 - Multi-Tenant RBAC & Identity Federation 🆕

### 🚀 Major Improvements

- **Unified Query API**: Single endpoint (`/api/v1/unified/*`) with intelligent routing across all data types
- **Cross-Engine Correlation**: Query logs, metrics, and traces together with unified syntax
- **Correlation Engine**: Advanced time-window and label-based correlation with confidence scoring
- **Enhanced Caching**: Valkey cluster integration with TTL-based result caching
- **Schema Definitions Store**: Weaviate-powered metadata storage for metrics, logs, and traces
- **Multi-Tenant RBAC**: Complete role-based access control with physical tenant isolation
- **Identity Federation**: SAML/OIDC integration placeholders for enterprise authentication
- **Performance Optimizations**: 10x RAM reduction and sub-millisecond query responses

### 📊 Query Capabilities

```bash
# Unified query across all engines
curl -X POST https://mirador-core/api/v1/unified/query \
  -H "Authorization: Bearer <token>" \
  -d '{"query": {"type": "correlation", "query": "logs:error AND metrics:high_latency"}}'

# Correlation queries - find relationships across observability data
curl -X POST https://mirador-core/api/v1/unified/correlation \
  -H "Authorization: Bearer <token>" \
  -d '{
    "query": {
      "id": "correlation-1",
      "type": "correlation",
      "query": "logs:exception WITHIN 5m OF metrics:cpu_usage > 80"
    }
  }'

# Intelligent routing - no need to know which engine to query
curl -X POST https://mirador-core/api/v1/unified/query \
  -d '{"query": {"type": "auto", "query": "service:api AND level:error"}}'
```

### 🏗️ Architecture Enhancements

- **Unified Query Engine**: Intelligent routing based on query patterns and content
- **Correlation Engine**: Parallel execution across multiple engines with result merging and confidence scoring
- **Schema Registry**: Centralized definitions for metrics, labels, and log fields
- **Enhanced Security**: RBAC improvements and tenant isolation

## Project Status

### ✅ Completed Phases (v9.0.0)

- **Phase 0**: Foundation & Architecture ✓
- **Phase 1**: RBAC Models & Schemas ✓
- **Phase 2**: Authentication System ✓
- **Phase 3**: Multi-Tenant Infrastructure ✓
- **Phase 4**: API Handlers & Middleware ✓

### 🔄 Current Progress

- **Phase 5**: Bootstrap & Validation (95% Complete)
  - ✅ RBAC Bootstrap Service implemented
  - ✅ Weaviate RBAC schema deployment
  - ✅ Default tenant and admin user creation
  - ✅ Local development infrastructure operational
  - ✅ E2E test script created and functional
  - 🔄 Authentication validation (minor tenant association issue)

- **Phase 6**: Integration Testing (50% Complete)
  - ✅ Comprehensive E2E test script created
  - ✅ Authentication and RBAC testing suites
  - ✅ Multi-tenancy isolation validation
  - 🔄 Full infrastructure testing (pending auth resolution)

### 📋 Upcoming Phases

- **Phase 7**: Identity Federation (5% Complete - Placeholders)
  - SAML/OIDC integration framework
  - Enterprise directory synchronization
  - Multi-factor authentication

- **Phase 8**: Testing & Quality Assurance
  - Load testing and performance validation
  - Security penetration testing
  - Production readiness assessment

- **Phase 9**: Documentation & Adoption
  - Complete API documentation
  - Deployment guides and runbooks
  - Training materials and examples

### 📈 Quality Gates

- **API Functionality**: All unified endpoints functional with E2E tests
- **Performance**: Unified queries within 200% of individual engine performance
- **Correlation Accuracy**: >95% accurate results across time windows and label-based correlations
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
- **Correlation Engine**: Advanced cross-engine correlation with time-window and label-based analysis
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
make localdev-up

# Generate Protocol Buffers
make proto

# Run tests
make test

# Start development server
make dev
```

### First-Time Setup: RBAC Bootstrap

After setting up the development environment, you need to bootstrap the RBAC system to create the default tenant, admin user, and roles:

```bash
# Build the bootstrap tool
make bootstrap

# Deploy Weaviate RBAC schema (required before bootstrap)
./bin/bootstrap --deploy-schema

# Run RBAC bootstrap (requires Weaviate and Valkey running)
./bin/bootstrap
```

**Default Credentials:**
- **Username:** `aarvee`
- **Password:** `ChangeMe123!`
- **Tenant:** `PLATFORMBUILDS`

⚠️ **IMPORTANT:** Change the default password immediately after first login!

The bootstrap process creates:
- Default system tenant (`PLATFORMBUILDS`)
- Global admin user (`aarvee`)
- Default roles: `global_admin`, `tenant_admin`, `tenant_editor`, `tenant_guest`
- Authentication credentials with TOTP support

The bootstrap is **idempotent** - running it multiple times will not create duplicates.

### Comprehensive E2E Testing

MIRADOR-CORE v9.0.0 includes comprehensive end-to-end testing to validate the RBAC and multi-tenancy implementation:

```bash
# Run full E2E test suite (requires running infrastructure)
make localdev-up
make localdev-wait
./localtesting/e2e-tests.sh

# Run code quality tests only (no infrastructure required)
./localtesting/e2e-tests.sh --code-tests-only

# Run API tests only (requires running server)
./localtesting/e2e-tests.sh --api-tests-only
```

**Test Coverage:**
- ✅ Bootstrap validation and schema deployment
- ✅ Authentication flows (login, logout, session management)
- ✅ RBAC policy enforcement and role-based access
- ✅ Multi-tenant data isolation
- ✅ API endpoint security validation
- ✅ Integration testing across all components

**Test Reports:**
- `deployments/localdev/e2e-report.json` - Detailed test results
- `localtesting/e2e-test-results.json` - API test outcomes
- `localtesting/test-failures-table.md` - Failure summary

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
- **Correlation**: Cross-engine correlation queries with time-window and label-based analysis

#### Unified Query Features
- **Intelligent Routing**: Automatic engine selection based on query patterns
- **Caching**: Configurable TTL-based result caching with Valkey
- **Cross-Engine Correlation**: Time-window and label-based correlation with confidence scoring
- **Unified Response Format**: Consistent JSON responses across all query types
- **Performance Monitoring**: Built-in metrics and execution time tracking

#### Correlation Query Examples 🆕

**Time-Window Correlation:**
```bash
# Find error logs within 5 minutes of CPU spikes
curl -X POST https://mirador-core/api/v1/unified/correlation \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "cpu-error-correlation",
      "type": "correlation",
      "query": "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z"
    }
  }'
```

**Label-Based Correlation:**
```bash
# Find correlations between logs and metrics for the same service
curl -X POST https://mirador-core/api/v1/unified/correlation \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "service-correlation",
      "type": "correlation",
      "query": "logs:service:checkout AND metrics:service:checkout"
    }
  }'
```

**Multi-Engine Correlation:**
```bash
# Correlate exceptions with traces and error metrics
curl -X POST https://mirador-core/api/v1/unified/correlation \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "complex-correlation",
      "type": "correlation",
      "query": "logs:exception WITHIN 10m OF traces:status:error AND metrics:error_rate > 5"
    }
  }'
```

### Metrics APIs

#### Instant Queries
```bash
# Basic MetricsQL query
curl -X POST https://mirador-core/api/v1/metrics/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "http_requests_total{job=\"api\"}",
    "time": "2025-01-01T00:00:00Z",
    "include_definitions": true
  }'
```

#### Range Queries
```bash
# Time range MetricsQL query
curl -X POST https://mirador-core/api/v1/metrics/query_range \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "rate(http_requests_total[5m])",
    "start": "2025-01-01T00:00:00Z",
    "end": "2025-01-01T01:00:00Z",
    "step": "1m",
    "include_definitions": true
  }'
```

#### Aggregate Functions
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

# Top K values
curl -X POST https://mirador-core/api/v1/metrics/query/aggregate/topk \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "rate(http_requests_total[5m])", "params": {"k": 5}}'
```

**Available Aggregate Functions**: `sum`, `avg`, `count`, `min`, `max`, `stddev`, `stdvar`, `quantile`, `topk`, `bottomk`, `count_values`, `absent`, `increase`, `delta`, `rate`, `irate`, `deriv`, `idelta`, `ideriv`, `group`, `histogram`, `and`, `or`, `unless`

#### Rollup Functions
```bash
# Rate calculation
curl -X POST https://mirador-core/api/v1/metrics/query/rollup/rate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "http_requests_total[5m]"}'

# Increase over time
curl -X POST https://mirador-core/api/v1/metrics/query/rollup/increase \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "http_requests_total[1h]"}'
```

#### Transform Functions
```bash
# Round values
curl -X POST https://mirador-core/api/v1/metrics/query/transform/round \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "http_request_duration_seconds"}'

# Clamp values between min/max
curl -X POST https://mirador-core/api/v1/metrics/query/transform/clamp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "cpu_usage", "params": {"min": 0, "max": 100}}'
```

#### Label Functions
```bash
# Replace label values
curl -X POST https://mirador-core/api/v1/metrics/query/label/label_replace \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "up",
    "params": {
      "dst": "service",
      "replacement": "$1",
      "src": "instance",
      "regex": "(.*):.*"
    }
  }'

# Keep only specific labels
curl -X POST https://mirador-core/api/v1/metrics/query/label/label_keep \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query": "http_requests_total", "params": {"labels": ["job", "instance"]}}'
```

### Logs APIs

#### Query Logs
```bash
# Logs query with Lucene syntax
curl -X POST https://mirador-core/api/v1/logs/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query_language": "lucene",
    "search_engine": "lucene",
    "query": "_time:[now-15m TO now] AND level:error AND service:api",
    "limit": 1000
  }'

# Logs query with Bleve search engine
curl -X POST https://mirador-core/api/v1/logs/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query_language": "lucene",
    "search_engine": "bleve",
    "query": "error AND status:500",
    "start": 1640995200000,
    "end": 1640998800000,
    "limit": 500
  }'
```

#### Search Logs
```bash
# Advanced search with pagination
curl -X POST https://mirador-core/api/v1/logs/search \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query_language": "lucene",
    "query": "_time:1h AND level:error",
    "search_engine": "lucene",
    "limit": 100,
    "page_after": {
      "ts": 1640998800000,
      "offset": 0
    }
  }'
```

#### Export Logs
```bash
# Export logs as CSV
curl -X POST https://mirador-core/api/v1/logs/export \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query_language": "lucene",
    "query": "_time:1h AND level:error",
    "format": "csv"
  }' \
  --output logs.csv
```

#### Logs Analytics
```bash
# Get histogram data for visualization
curl -X GET "https://mirador-core/api/v1/logs/histogram?query_language=lucene&query=_time:30m&step=60000" \
  -H "Authorization: Bearer <token>"

# Get facet counts
curl -X GET "https://mirador-core/api/v1/logs/facets?query_language=lucene&query=_time:30m&fields=level,service" \
  -H "Authorization: Bearer <token>"
```

#### Real-time Logs (WebSocket)
```bash
# Connect to WebSocket for real-time logs
wscat -H "Authorization: Bearer <token>" \
  -c "ws://mirador-core/api/v1/logs/tail?query=_time:5m&sampling=10"
# Or if you are using session tokens from the UI:
# wscat -H "X-Session-Token: <session-id>" -c "ws://mirador-core/api/v1/logs/tail?query=_time:5m"
```

### Traces APIs

#### Query Traces
```bash
# Search traces with Lucene syntax
curl -X POST https://mirador-core/api/v1/traces/search \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "query_language": "lucene",
    "search_engine": "lucene",
    "query": "_time:[now-15m TO now] AND service:checkout AND operation:CreateOrder",
    "limit": 100
  }'

# Get specific trace by ID
curl -X GET https://mirador-core/api/v1/traces/abc123def456 \
  -H "Authorization: Bearer <token>"
```

#### Trace Analysis
```bash
# Get flame graph data for a trace
curl -X GET https://mirador-core/api/v1/traces/abc123def456/flamegraph?mode=duration \
  -H "Authorization: Bearer <token>"

# Get aggregated flame graph from trace search
curl -X POST https://mirador-core/api/v1/traces/flamegraph/search \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "service": "checkout",
    "operation": "CreateOrder",
    "start": "2025-01-01T00:00:00Z",
    "end": "2025-01-01T01:00:00Z"
  }'
```

#### Trace Schema
```bash
# List all services
curl -X GET https://mirador-core/api/v1/traces/services \
  -H "Authorization: Bearer <token>"

# List operations for a service
curl -X GET https://mirador-core/api/v1/traces/services/checkout/operations \
  -H "Authorization: Bearer <token>"
```

### AI Analysis APIs

#### Predictive Analysis
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

# List predicted fractures
curl -X GET "https://mirador-core/api/v1/predict/fractures?time_range=24h&min_prob=0.7" \
  -H "Authorization: Bearer <token>"

# List active prediction models
curl -X GET https://mirador-core/api/v1/predict/models \
  -H "Authorization: Bearer <token>"
```

#### Root Cause Analysis
```bash
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
    }
  }'

# Get active correlations
curl -X GET https://mirador-core/api/v1/rca/correlations \
  -H "Authorization: Bearer <token>"

# Get service graph with latency metrics
curl -X POST https://mirador-core/api/v1/rca/service-graph \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "start": "2025-01-01T00:00:00Z",
    "end": "2025-01-01T01:00:00Z",
    "client": "web-service",
    "server": "api-service"
  }'
```

### KPI Management APIs

#### Metrics Schema
```bash
# Create/update metric definition
curl -X POST https://mirador-core/api/v1/schema/metrics \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "metric": "http_requests_total",
    "description": "Total number of HTTP requests",
    "owner": "platform-team",
    "tags": ["domain:web", "category:performance"],
    "author": "john.doe"
  }'

# Get metric definition
curl -X GET https://mirador-core/api/v1/schema/metrics/http_requests_total \
  -H "Authorization: Bearer <token>"

# Bulk upload metrics via CSV
curl -X POST https://mirador-core/api/v1/schema/metrics/bulk \
  -H "Authorization: Bearer <token>" \
  -F "file=@metrics_definitions.csv"

# Download sample CSV template
curl -X GET "https://mirador-core/api/v1/schema/metrics/bulk/sample?metrics=http_requests_total,cpu_usage" \
  -H "Authorization: Bearer <token>" \
  --output metrics_template.csv
```

#### Log Fields Schema
```bash
# Create/update log field definition
curl -X POST https://mirador-core/api/v1/schema/logs/fields \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "field": "level",
    "type": "string",
    "description": "Log level severity",
    "tags": ["category:logging", "indexed:true"],
    "examples": {
      "normal": "INFO",
      "error": "ERROR",
      "debug": "DEBUG"
    },
    "author": "jane.smith"
  }'

# Bulk upload log fields via CSV
curl -X POST https://mirador-core/api/v1/schema/logs/fields/bulk \
  -H "Authorization: Bearer <token>" \
  -F "file=@log_fields.csv"
```

#### Trace Schema
```bash
# Create/update trace service definition
curl -X POST https://mirador-core/api/v1/schema/traces/services \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "service": "checkout",
    "purpose": "Handles e-commerce checkout process",
    "owner": "commerce-team",
    "tags": ["domain:ecommerce", "language:go"],
    "author": "mike.wilson"
  }'

# Create/update trace operation definition
curl -X POST https://mirador-core/api/v1/schema/traces/operations \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "service": "checkout",
    "operation": "ProcessPayment",
    "purpose": "Processes payment for order",
    "owner": "commerce-team",
    "tags": ["method:POST", "endpoint:/api/payment"],
    "author": "mike.wilson"
  }'

# Bulk upload services/operations via CSV
curl -X POST https://mirador-core/api/v1/schema/traces/services/bulk \
  -H "Authorization: Bearer <token>" \
  -F "file=@trace_services.csv"
```

### Configuration APIs

#### Runtime Configuration
```bash
# Get current feature flags
curl -X GET https://mirador-core/api/v1/config/features \
  -H "Authorization: Bearer <token>"

# Update feature flags
curl -X PUT https://mirador-core/api/v1/config/features \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "features": {
      "rca_enabled": false,
      "predict_enabled": true,
      "user_settings_enabled": true,
      "rbac_enabled": true
    }
  }'

# Reset feature flags to defaults
curl -X POST https://mirador-core/api/v1/config/features/reset \
  -H "Authorization: Bearer <token>"
```

#### gRPC Endpoints Configuration
```bash
# Get current gRPC endpoints
curl -X GET https://mirador-core/api/v1/config/grpc/endpoints \
  -H "Authorization: Bearer <token>"

# Update gRPC endpoints
curl -X PUT https://mirador-core/api/v1/config/grpc/endpoints \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "rca_endpoint": "rca-service-new:50051",
    "predict_endpoint": "predict-service-new:50052"
  }'

# Reset gRPC endpoints to defaults
curl -X POST https://mirador-core/api/v1/config/grpc/endpoints/reset \
  -H "Authorization: Bearer <token>"
```

#### User Preferences APIs
```bash
# Get user preferences
curl -X GET https://mirador-core/api/v1/config/user-preferences \
  -H "Authorization: Bearer <token>"

# Create user preferences
curl -X POST https://mirador-core/api/v1/config/user-preferences \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "preferences": {
      "theme": "dark",
      "timezone": "UTC",
      "notifications": true
    }
  }'

# Update user preferences
curl -X PUT https://mirador-core/api/v1/config/user-preferences \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "preferences": {
      "theme": "light",
      "timezone": "America/New_York",
      "notifications": false
    }
  }'

# Delete user preferences
curl -X DELETE https://mirador-core/api/v1/config/user-preferences \
  -H "Authorization: Bearer <token>"
```

#### Dashboards APIs
```bash
# Get dashboards
curl -X GET https://mirador-core/api/v1/config/dashboards \
  -H "Authorization: Bearer <token>"

# Create dashboard
curl -X POST https://mirador-core/api/v1/config/dashboards \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API Performance Dashboard",
    "description": "Monitor API performance metrics",
    "shared": false,
    "layout": {
      "panels": []
    }
  }'

# Get specific dashboard
curl -X GET https://mirador-core/api/v1/config/dashboards/dash-123 \
  -H "Authorization: Bearer <token>"

# Update dashboard
curl -X PUT https://mirador-core/api/v1/config/dashboards/dash-123 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated API Dashboard",
    "description": "Updated dashboard for monitoring API endpoints",
    "shared": true,
    "layout": {
      "panels": []
    }
  }'

# Delete dashboard
curl -X DELETE https://mirador-core/api/v1/config/dashboards/dash-123 \
  -H "Authorization: Bearer <token>"
```

### Health & Monitoring APIs

#### Health Checks
```bash
# Basic health check
curl -X GET https://mirador-core/api/v1/health

# Readiness check (includes backend validation)
curl -X GET https://mirador-core/api/v1/ready

# Microservices status
curl -X GET https://mirador-core/api/v1/microservices/status \
  -H "Authorization: Bearer <token>"
```

#### Prometheus Metrics
```bash
# Get Prometheus metrics
curl -X GET https://mirador-core/api/v1/metrics
```

### WebSocket APIs

#### Real-time Data Streams
```bash
# Metrics stream
wscat -H "Authorization: Bearer <token>" -c "ws://mirador-core/api/v1/ws/metrics"

# Alerts stream
wscat -H "Authorization: Bearer <token>" -c "ws://mirador-core/api/v1/ws/alerts"

# Predictions stream
wscat -H "Authorization: Bearer <token>" -c "ws://mirador-core/api/v1/ws/predictions"
```

### Session Management APIs

```bash
# Get active sessions
curl -X GET https://mirador-core/api/v1/sessions/active \
  -H "Authorization: Bearer <token>"

# Invalidate session
curl -X POST https://mirador-core/api/v1/sessions/invalidate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"session_id": "session-123"}'
```

### RBAC APIs

```bash
# List roles
curl -X GET https://mirador-core/api/v1/rbac/roles \
  -H "Authorization: Bearer <token>"

# Create role
curl -X POST https://mirador-core/api/v1/rbac/roles \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "analyst",
    "permissions": ["read:metrics", "read:logs"]
  }'

# Assign user roles
curl -X PUT https://mirador-core/api/v1/rbac/users/user-123/roles \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"role_ids": ["analyst", "viewer"]}'
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
    correlation_engine: "correlation"
  correlation:
    enabled: true
    time_window_max: "24h"
    confidence_threshold: 0.7
```

## Unified Schema API

The unified schema API provides a single, consistent interface for managing all schema definitions. KPIs serve as the central schema definitions, with support for metrics, labels, log fields, traces, dashboards, layouts, and user preferences.

### Schema Types

The API supports the following schema types:
- `metric` - Metric definitions with descriptions, owners, and tags
- `label` - Label definitions with types, constraints, and descriptions
- `log_field` - Log field definitions with types and examples
- `trace_service` - Trace service definitions
- `trace_operation` - Trace operation definitions (scoped to services)
- `kpi` - KPI definitions with queries, thresholds, and visualizations
- `dashboard` - Dashboard configurations
- `layout` - KPI layout configurations within dashboards
- `user_preferences` - User interface preferences

### API Endpoints

All schema operations use the unified `/api/v1/schema/:type` endpoints:

```bash
# Create or update a schema definition
POST /api/v1/schema/{type}

# Get a specific schema definition
GET /api/v1/schema/{type}/{id}

# List schema definitions with optional filtering
GET /api/v1/schema/{type}?limit=50&offset=0&tags=domain:web

# Delete a schema definition (requires confirmation)
DELETE /api/v1/schema/{type}/{id}?confirm=1
```

### Examples

#### Metric Definitions
```bash
# Create/update metric definition
curl -X POST https://mirador-core/api/v1/schema/metric \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "http_requests_total",
    "description": "Total number of HTTP requests",
    "tags": ["domain:web", "category:performance"],
    "extensions": {
      "metric": {
        "description": "Total number of HTTP requests",
        "owner": "platform-team"
      }
    },
    "author": "john.doe"
  }'

# Get metric definition
curl -X GET https://mirador-core/api/v1/schema/metric/http_requests_total \
  -H "Authorization: Bearer <token>"

# List metrics
curl -X GET https://mirador-core/api/v1/schema/metric?limit=20 \
  -H "Authorization: Bearer <token>"
```

#### Label Definitions
```bash
# Create/update label definition
curl -X POST https://mirador-core/api/v1/schema/label \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "status",
    "tags": ["category:monitoring", "indexed:true"],
    "extensions": {
      "label": {
        "type": "string",
        "required": false,
        "allowedVals": ["success", "error", "warning"],
        "description": "Request status"
      }
    },
    "author": "jane.smith"
  }'
```

#### Log Field Definitions
```bash
# Create/update log field definition
curl -X POST https://mirador-core/api/v1/schema/log_field \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "level",
    "tags": ["category:logging", "indexed:true"],
    "extensions": {
      "logField": {
        "fieldType": "string",
        "description": "Log level severity"
      }
    },
    "author": "jane.smith"
  }'
```

#### Trace Definitions
```bash
# Create/update trace service definition
curl -X POST https://mirador-core/api/v1/schema/trace_service \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "checkout",
    "tags": ["domain:ecommerce", "language:go"],
    "extensions": {
      "trace": {
        "servicePurpose": "Handles e-commerce checkout process",
        "owner": "commerce-team"
      }
    },
    "author": "mike.wilson"
  }'

# Create/update trace operation definition
curl -X POST https://mirador-core/api/v1/schema/trace_operation \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ProcessPayment",
    "tags": ["method:POST", "endpoint:/api/payment"],
    "extensions": {
      "trace": {
        "service": "checkout",
        "servicePurpose": "Processes payment for order",
        "owner": "commerce-team"
      }
    },
    "author": "mike.wilson"
  }'
```

#### KPI Definitions
```bash
# Create/update KPI definition
curl -X POST https://mirador-core/api/v1/schema/kpi \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Error Rate",
    "kind": "percentage",
    "unit": "%",
    "format": "0.00",
    "query": "rate(errors_total[5m]) / rate(http_requests_total[5m]) * 100",
    "thresholds": {
      "warning": 5.0,
      "critical": 10.0
    },
    "tags": ["domain:api", "category:reliability"],
    "sparkline": true,
    "visibility": "public",
    "author": "ops.team"
  }'

# Get KPI definition
curl -X GET https://mirador-core/api/v1/schema/kpi/error-rate \
  -H "Authorization: Bearer <token>"
```

#### Dashboard Definitions
```bash
# Create/update dashboard definition
curl -X POST https://mirador-core/api/v1/schema/dashboard \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API Health Dashboard",
    "visibility": "team",
    "extensions": {
      "dashboard": {
        "isDefault": false
      }
    },
    "author": "dashboard.admin"
  }'
```

### Schema Definition Structure

All schema definitions follow a common structure:

```json
{
  "id": "unique-identifier",
  "name": "human-readable-name",
  "type": "schema-type",
  "tenantId": "optional-tenant-id",
  "tags": ["tag1", "tag2:key"],
  "category": "optional-category",
  "sentiment": "optional-sentiment",
  "author": "creator-username",
  "createdAt": "2024-01-01T00:00:00Z",
  "updatedAt": "2024-01-01T00:00:00Z",
  "extensions": {
    "typeSpecific": {
      // Type-specific fields
    }
  }
}
```

### Tags Format

All schema definitions use flat arrays of strings for tags:
- `["domain:web", "owner:platform-team", "category:performance"]`
- Tags support key:value pairs or simple labels
- Used for filtering and organization

### Bulk Operations

Bulk CSV upload is supported for schema definitions:

```bash
# Bulk upload schema definitions via CSV
curl -X POST https://mirador-core/api/v1/schema/{type}/bulk \
  -H "Authorization: Bearer <token>" \
  -F "file=@schema_definitions.csv"
```

CSV format includes `tags_json` column containing JSON arrays of tag strings.

### Security & Validation

- **Input Validation**: Comprehensive sanitization and validation
- **Rate Limiting**: Per-tenant request throttling
- **Audit Logging**: All schema changes are logged
- **Tenant Isolation**: Automatic tenant scoping
- **File Upload Limits**: 5MiB maximum for bulk operations

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
   - Docker & Docker Compose (required for containerized development)
   - Make

2. **Clone and Start Containerized Development**
   ```bash
   git clone https://github.com/platformbuilds/mirador-core
   cd mirador-core
   make localdev-up  # Start complete containerized development environment
   ```

3. **Verify Environment**
   ```bash
   make localdev-wait  # Wait for services to be ready
   curl http://localhost:8010/api/v1/health  # Check health
   ```

4. **Run Tests**
   ```bash
   make localdev-test  # Full E2E test suite (containers)
   ```

### 🐳 Container-Only Development

MIRADOR-CORE **strongly recommends containerized development** to ensure consistency across all environments and avoid "works on my machine" issues.

**✅ Recommended (Container-based):**
```bash
make localdev-up     # Start all services in containers
make localdev-test   # Run tests against containerized environment
make localdev-down   # Clean up containers
```

**❌ Not Recommended (Local development):**
```bash
make setup           # Install local Go dependencies
make dev             # Run server locally (requires local setup)
```

### Development Commands

**Container-based development:**
- `make localdev-up` - Start complete containerized environment
- `make localdev-wait` - Wait for services readiness
- `make localdev-test` - Run E2E tests in containers
- `make localdev-down` - Stop and clean up containers

**RBAC Bootstrap & Testing:**
- `make bootstrap` - Build RBAC bootstrap tool
- `./bin/bootstrap --deploy-schema` - Deploy Weaviate RBAC schema
- `./bin/bootstrap` - Initialize RBAC system with default data
- `./localtesting/e2e-tests.sh` - Run comprehensive E2E tests
- `./localtesting/e2e-tests.sh --code-tests-only` - Code quality validation only

**Local development (not recommended):**
- `make setup` - Install local dependencies
- `make dev` - Run server locally
- `make proto` - Generate protobuf files locally

## Deployment

### 🚀 Recommended: Kubernetes (Helm) Deployment

For production deployments, we **strongly recommend** using Kubernetes with Helm charts for scalability, reliability, and operational excellence.

#### Quick Start with Helm

```bash
# Add the MIRADOR Helm repository
helm repo add mirador https://platformbuilds.github.io/mirador-core
helm repo update

# Install with default VictoriaMetrics ecosystem
helm install mirador-core mirador/mirador-core \
  --namespace mirador-system \
  --create-namespace \
  --set image.tag=v7.0.0 \
  --set vm.endpoints="vm-cluster:8481" \
  --set vl.endpoints="vl-cluster:9428" \
  --set vt.endpoints="vt-cluster:10428" \
  --set valkey.endpoints="valkey-cluster:6379" \
  --set weaviate.host="weaviate-cluster"
```

#### Production Helm Deployment

```bash
# Create dedicated namespace
kubectl create namespace mirador-production

# Install with production configuration
helm install mirador-core mirador/mirador-core \
  --namespace mirador-production \
  --values production-values.yaml \
  --set image.tag=v7.0.0 \
  --set replicaCount=3 \
  --set resources.limits.cpu="2000m" \
  --set resources.limits.memory="4Gi" \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host="mirador.yourcompany.com" \
  --set ingress.tls[0].secretName="mirador-tls"
```

#### Helm Configuration Examples

**Multi-cluster VictoriaMetrics setup:**
```yaml
# production-values.yaml
vm:
  endpoints: "vm-cluster-1:8481,vm-cluster-2:8481,vm-cluster-3:8481"
  timeout: 30000

vl:
  endpoints: "vl-cluster-1:9428,vl-cluster-2:9428"
  timeout: 30000

vt:
  endpoints: "vt-cluster-1:10428,vt-cluster-2:10428"
  timeout: 30000

valkey:
  endpoints: "valkey-cluster-1:6379,valkey-cluster-2:6379,valkey-cluster-3:6379"
  sentinel: true

weaviate:
  enabled: true
  host: "weaviate-cluster"
  port: 80
  scheme: "http"
```

**Enterprise authentication setup:**
```yaml
# enterprise-values.yaml
auth:
  ldap:
    enabled: true
    url: "ldap://ldap.corp.company.com"
    baseDN: "dc=company,dc=com"
    userSearchFilter: "(sAMAccountName={0})"
    tlsCaBundlePath: "/etc/mirador/ldap/ca-bundle.pem" # hot-reloaded PEM bundle
    tlsSkipVerify: false

rbac:
  enabled: true
  defaultRoles:
    - name: "viewer"
      permissions: ["read:metrics", "read:logs", "read:traces"]
    - name: "analyst"
      permissions: ["read:*", "write:queries"]
    - name: "admin"
      permissions: ["*"]

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: mirador.company.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: mirador-tls
      hosts:
        - mirador.company.com
```

#### Helm Upgrade

```bash
# Upgrade to new version
helm repo update
helm upgrade mirador-core mirador/mirador-core \
  --namespace mirador-production \
  --set image.tag=v7.1.0 \
  --set replicaCount=5

# Rollback if needed
helm rollback mirador-core 1 --namespace mirador-production
```

### 🐳 Development/Testing: Docker Deployment

For development, testing, or simple deployments, Docker provides a quick way to get started.

#### Build and Run with Docker

```bash
# Build for local architecture
make docker-build

# Build multi-architecture (linux/amd64, linux/arm64)
make dockerx-build

# Run locally with basic configuration
docker run -d \
  --name mirador-core \
  -p 8010:8010 \
  -e VM_ENDPOINTS="http://host.docker.internal:8481" \
  -e VL_ENDPOINTS="http://host.docker.internal:9428" \
  -e VALKEY_CACHE_NODES="host.docker.internal:6379" \
  platformbuilds/mirador-core:latest
```

#### Docker Compose for Local Development

```yaml
# docker-compose.yml for development
version: '3.8'
services:
  mirador-core:
    image: platformbuilds/mirador-core:v7.0.0
    ports:
      - "8010:8010"
    environment:
      - ENVIRONMENT=development
      - VM_ENDPOINTS=http://victoriametrics:8481
      - VL_ENDPOINTS=http://victorialogs:9428
      - VALKEY_CACHE_NODES=victoriametrics:6379
      - WEAVIATE_HOST=weaviate
      - WEAVIATE_PORT=8080
    depends_on:
      - victoriametrics
      - victorialogs
      - weaviate
    restart: unless-stopped

  victoriametrics:
    image: victoriametrics/victoria-metrics:latest
    ports:
      - "8481:8428"
    command:
      - "--storageDataPath=/storage"
      - "--httpListenAddr=:8428"

  victorialogs:
    image: victoriametrics/victoria-logs:latest
    ports:
      - "9428:9428"
    command:
      - "--storageDataPath=/storage"
      - "--httpListenAddr=:9428"

  weaviate:
    image: semitechnologies/weaviate:latest
    ports:
      - "8080:8080"
    environment:
      - QUERY_DEFAULTS_LIMIT=25
      - AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED=true
      - PERSISTENCE_DATA_PATH='/var/lib/weaviate'
```

### Infrastructure Requirements

#### Production Infrastructure Checklist

**VictoriaMetrics Ecosystem:**
- ✅ VictoriaMetrics cluster (3+ nodes for HA)
- ✅ VictoriaLogs cluster (2+ nodes recommended)
- ✅ VictoriaTraces cluster (2+ nodes recommended)

**Supporting Services:**
- ✅ Valkey/Redis cluster (3+ nodes for HA)
- ✅ Weaviate vector database (3+ nodes for HA)
- ✅ Load balancer/Ingress controller
- ✅ Persistent storage for all services

**Network Security:**
- ✅ Service mesh (Istio/Linkerd) for mTLS
- ✅ Network policies restricting pod communication
- ✅ External access through API Gateway

**Monitoring & Observability:**
- ✅ Prometheus for metrics collection
- ✅ Grafana for dashboards
- ✅ ELK/EFK stack for centralized logging
- ✅ Distributed tracing (Jaeger/Tempo)

### Configuration

#### Environment Variables

**Core Settings:**
```bash
# Application
PORT=8010
ENVIRONMENT=production
LOG_LEVEL=info
SHUTDOWN_TIMEOUT=30s

# VictoriaMetrics ecosystem
VM_ENDPOINTS=vm-cluster-1:8481,vm-cluster-2:8481,vm-cluster-3:8481
VL_ENDPOINTS=vl-cluster-1:9428,vl-cluster-2:9428
VT_ENDPOINTS=vt-cluster-1:10428,vt-cluster-2:10428

# Caching & Storage
VALKEY_CACHE_NODES=valkey-1:6379,valkey-2:6379,valkey-3:6379
CACHE_TTL=300
WEAVIATE_ENABLED=true
WEAVIATE_HOST=weaviate-cluster
WEAVIATE_PORT=80

# AI Engines (gRPC)
RCA_ENGINE_GRPC=rca-service:50051
PREDICT_ENGINE_GRPC=predict-service:50052
ALERT_ENGINE_GRPC=alert-service:50053

# Authentication & Security
LDAP_URL=ldap://ldap.corp.company.com
LDAP_BASE_DN=dc=company,dc=com
RBAC_ENABLED=true
JWT_SECRET=<secure-random-string>
TLS_CERT_PATH=/etc/ssl/certs/mirador.crt
TLS_KEY_PATH=/etc/ssl/private/mirador.key

# Performance Tuning
MAX_CONCURRENT_QUERIES=100
QUERY_TIMEOUT=60s
CACHE_MAX_MEMORY=1GB
GOMAXPROCS=4
```

#### Multi-Source Aggregation

Configure fan-out queries across multiple backend clusters for high availability and performance:

```yaml
# config.production.yaml
database:
  # Primary VictoriaMetrics cluster
  victoria_metrics:
    endpoints: ["vm-prod-1:8481", "vm-prod-2:8481", "vm-prod-3:8481"]
    timeout: 30000
    retries: 3

  # Additional metrics sources
  metrics_sources:
    - name: metrics-archive
      endpoints: ["vm-archive-1:8481", "vm-archive-2:8481"]
      timeout: 45000

  # Primary logs cluster
  victoria_logs:
    endpoints: ["vl-prod-1:9428", "vl-prod-2:9428"]
    timeout: 30000

  # Additional logs sources
  logs_sources:
    - name: logs-archive
      endpoints: ["vl-archive-1:9428"]
      timeout: 45000

  # Traces cluster
  victoria_traces:
    endpoints: ["vt-prod-1:10428", "vt-prod-2:10428"]
    timeout: 30000

# Unified query configuration
unified_query:
  enabled: true
  default_timeout: "30s"
  max_timeout: "300s"
  cache:
    enabled: true
    default_ttl: "5m"
    max_ttl: "1h"
    max_memory: "2GB"
  routing:
    metrics_engine: "victoriametrics"
    logs_engine: "victorialogs"
    traces_engine: "victoriatraces"
    correlation_engine: "correlation"
  correlation:
    enabled: true
    time_window_max: "24h"
    confidence_threshold: 0.7
    correlation_engine: "rca"

# Rate limiting
rate_limiting:
  enabled: true
  requests_per_minute: 1000
  burst_limit: 2000
  tenant_isolation: true

# Circuit breakers
circuit_breakers:
  vm_circuit:
    failure_threshold: 5
    recovery_timeout: "60s"
    success_threshold: 3
  vl_circuit:
    failure_threshold: 3
    recovery_timeout: "30s"
    success_threshold: 2
```

### Deployment Strategies

#### Blue-Green Deployment
```bash
# Deploy new version alongside existing
helm install mirador-core-green mirador/mirador-core \
  --namespace mirador-production \
  --set image.tag=v7.1.0 \
  --set ingress.hosts[0].host="mirador-green.company.com"

# Test green environment
curl -H "Host: mirador-green.company.com" https://mirador.company.com/api/v1/health

# Switch traffic to green
kubectl patch ingress mirador-ingress \
  --namespace mirador-production \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/rules/0/host", "value": "mirador-green.company.com"}]'

# Remove blue environment
helm uninstall mirador-core --namespace mirador-production
```

#### Canary Deployment
```bash
# Deploy canary with 10% traffic
helm upgrade mirador-core mirador/mirador-core \
  --namespace mirador-production \
  --set canary.enabled=true \
  --set canary.weight=10 \
  --set image.tag=v7.1.0

# Gradually increase traffic
helm upgrade mirador-core mirador/mirador-core \
  --namespace mirador-production \
  --set canary.weight=25

# Complete rollout
helm upgrade mirador-core mirador/mirador-core \
  --namespace mirador-production \
  --set canary.enabled=false \
  --set image.tag=v7.1.0
```

### Migration from Docker to Kubernetes

If you're currently running MIRADOR-CORE with Docker and want to migrate to Kubernetes:

1. **Assess current setup:**
   ```bash
   # Check current configuration
   docker inspect mirador-core
   docker logs mirador-core --tail 100
   ```

2. **Backup data:**
   ```bash
   # Export configurations and schemas
   curl -X GET "http://localhost:8010/api/v1/schema/metrics" -o metrics_backup.json
   curl -X GET "http://localhost:8010/api/v1/schema/logs/fields" -o logs_backup.json
   ```

3. **Deploy to Kubernetes:**
   ```bash
   helm install mirador-core mirador/mirador-core \
     --namespace mirador-system \
     --create-namespace \
     --values migration-values.yaml
   ```

4. **Migrate configurations:**
   ```bash
   # Import schemas to new deployment
   curl -X POST "https://mirador.company.com/api/v1/schema/metrics/bulk" \
     -H "Authorization: Bearer <token>" \
     -F "file=@metrics_backup.json"
   ```

5. **Update DNS and switch traffic**

This migration provides better scalability, reliability, and operational capabilities while maintaining all existing functionality.

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

- **Documentation**: https://miradorstack.readthedocs.io/
- **API Reference**: https://mirador-core.github.io/api/
- **Community Forum**: https://github.com/platformbuilds/mirador-core/discussions
