# Getting Started

This guide will help you get MIRADOR-CORE up and running quickly.

## Prerequisites

- Go 1.21+ (for development)
- Docker & Docker Compose (for local development)
- Kubernetes cluster (for production deployment)
- Helm 3.x (for Kubernetes deployment)

## Local Development Setup

### 1. Clone the Repository

```bash
git clone https://github.com/miradorstack/mirador-core.git
cd mirador-core
```

### 2. Setup Development Environment

Install dependencies and generate required code:

```bash
# Install tools, generate protobuf code, and download dependencies
make setup
```

### 3. Start Development Stack

MIRADOR-CORE uses containerized development to ensure consistency:

```bash
# Start the complete development stack (containers only)
make localdev-up

# Wait for services to be ready
make localdev-wait
```

This command starts all required services in containers:
- **VictoriaMetrics** (metrics): `localhost:8481`
- **VictoriaLogs** (logs): `localhost:9428`
- **VictoriaTraces** (traces): `localhost:10428`
- **Weaviate** (vector database): `localhost:8080`
- **Valkey** (caching): `localhost:6379`

### 4. Seed Sample Data

```bash
# Seed synthetic OpenTelemetry data
make localdev-seed-otel

# Seed default dashboard and KPIs
make localdev-seed-data
```

### 5. Start Development Server

```bash
# Start development server (auto-rebuilds on changes)
make dev
```

The server runs at `http://localhost:8010` with:
- **API Documentation**: http://localhost:8010/swagger/
- **Health Checks**: http://localhost:8010/health
- **Metrics Endpoint**: http://localhost:8010/metrics

### 6. Run Tests

```bash
# Unit tests with race detection and coverage
make test

# Code quality checks
make lint
make fmt
make vuln

# Full E2E pipeline (complete testing)
make localdev

# API tests only
make localdev-test-api-only
```

### 7. Clean Up

```bash
# Stop all services and clean up
make localdev-down
```

## Development Environment Details

The local development stack provides:
- Full VictoriaMetrics ecosystem integration
- Comprehensive testing with synthetic data
- Hot-reload development server
- Race detection and coverage analysis
- E2E testing with actual service dependencies

**Environment Variables** (automatically configured by `make localdev-up`):
- `BASE_URL`: Base URL for the running app (default: `http://localhost:8010`)
- `E2E_BASE_URL`: Used by tests for endpoint validation

## Production Deployment

### Using Helm (Recommended)

```bash
# Add the MIRADOR Helm repository
helm repo add mirador https://platformbuilds.github.io/mirador-core
helm repo update

# Install with production configuration
helm install mirador-core mirador/mirador-core \
  --namespace mirador-system \
  --create-namespace \
  --set image.tag=v9.0.0 \
  --set vm.endpoints="vm-cluster:8481" \
  --set vl.endpoints="vl-cluster:9428" \
  --set vt.endpoints="vt-cluster:10428" \
  --set replicaCount=3
```

### Using Docker

```bash
# Build multi-architecture images
make dockerx-build

# Run with production settings
docker run -d \
  --name mirador-core \
  -p 8010:8010 \
  -e VM_ENDPOINTS="vm-cluster:8481" \
  -e VL_ENDPOINTS="vl-cluster:9428" \
  -e VT_ENDPOINTS="vt-cluster:10428" \
  -e RBAC_ENABLED=true \
  platformbuilds/mirador-core:v9.0.0
```

## Configuration

### Basic Configuration

Create a `config.yaml` file:

```yaml
server:
  port: 8010
  environment: production

database:
  victoria_metrics:
    endpoints: ["vm-cluster:8481"]
  victoria_logs:
    endpoints: ["vl-cluster:9428"]

auth:
  ldap:
    enabled: true
    url: "ldap://ldap.corp.company.com"
    base_dn: "dc=company,dc=com"
```

### Environment Variables

```bash
# Core settings
export PORT=8010
export ENVIRONMENT=production

# VictoriaMetrics ecosystem
export VM_ENDPOINTS="vm-cluster:8481"
export VL_ENDPOINTS="vl-cluster:9428"
export VT_ENDPOINTS="vt-cluster:10428"

# Authentication
export LDAP_URL="ldap://ldap.corp.company.com"
export RBAC_ENABLED=true
```

## First API Call

Once MIRADOR-CORE is running, try your first API call:

```bash
# Health check
curl -X GET http://localhost:8010/api/v1/health

# Unified query (if you have data sources configured)
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "test-query",
      "type": "metrics",
      "query": "up",
      "start_time": "2025-01-01T00:00:00Z",
      "end_time": "2025-01-01T01:00:00Z"
    }
  }'
```

## Next Steps

- [API Reference](api-reference.md) - Learn about all available endpoints
- [Configuration](configuration.md) - Detailed configuration options
- [Deployment](deployment.md) - Production deployment guides