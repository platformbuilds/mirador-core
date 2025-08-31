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
export VALLEY_CACHE_NODES=redis-1:6379,redis-2:6379,redis-3:6379
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

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License
