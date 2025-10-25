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
git clone https://github.com/platformbuilds/mirador-core.git
cd mirador-core
```

### 2. Set Up Development Environment

```bash
# Install dependencies and set up local stack
make setup
make localdev-up
```

### 3. Start the Development Server

```bash
# Run with hot reload
make dev
```

The server will be available at `http://localhost:8010`.

### 4. Verify Installation

```bash
# Check health endpoint
curl http://localhost:8010/api/v1/health

# Check readiness
curl http://localhost:8010/api/v1/ready
```

### 5. Run Tests (Optional)

```bash
# Run full E2E test suite
make localdev-test

# Or run API tests only
make localdev-test-api-only
```

### 6. Tear Down Development Environment

```bash
# Stop all services and clean up
make localdev-down
```

## Production Deployment

### Using Helm (Recommended)

```bash
# Add the MIRADOR Helm repository
helm repo add mirador https://platformbuilds.github.io/mirador-core
helm repo update

# Install with default configuration
helm install mirador-core mirador/mirador-core \
  --namespace mirador-system \
  --create-namespace \
  --set vm.endpoints="vm-cluster:8481" \
  --set vl.endpoints="vl-cluster:9428"
```

### Using Docker

```bash
# Build and run
make docker-build
docker run -p 8010:8010 \
  -e VM_ENDPOINTS="vm-cluster:8481" \
  -e VL_ENDPOINTS="vl-cluster:9428" \
  platformbuilds/mirador-core:latest
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
- [Development](development.md) - Contributing to MIRADOR-CORE