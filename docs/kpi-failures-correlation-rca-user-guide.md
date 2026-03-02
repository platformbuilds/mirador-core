# KPI, Failures, Correlation & RCA User Guide

**Version:** 9.0.0  
**Last Updated:** January 2026  
**Target Audience:** API Consumers & Integration Engineers

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites & Setup](#prerequisites--setup)
3. [Configuration & Deployment](#configuration--deployment)
4. [Component 1: KPI Management](#component-1-kpi-management)
5. [Component 2: Failure Detection](#component-2-failure-detection)
6. [Component 3: Correlation Analysis](#component-3-correlation-analysis)
7. [Component 4: Root Cause Analysis (RCA)](#component-4-root-cause-analysis-rca)
8. [Complete Workflows](#complete-workflows)
9. [API Reference](#api-reference)
10. [Troubleshooting](#troubleshooting)

---

## Overview

### What This Guide Covers

This guide explains how to use Mirador Core's four interconnected observability components:

```
┌──────────┐     ┌──────────┐     ┌─────────────┐     ┌──────┐
│   KPIs   │ --> │ Failures │ --> │ Correlation │ --> │ RCA  │
└──────────┘     └──────────┘     └─────────────┘     └──────┘
   Define           Detect          Analyze           Explain
   Metrics          Incidents       Patterns          Root Cause
```

### Dependency Chain

Each component builds upon the previous:

1. **KPIs (Key Performance Indicators)**: Define what metrics to monitor
2. **Failures**: Detect incidents based on KPI anomalies and error signals
3. **Correlation**: Perform statistical analysis to find relationships between KPIs
4. **RCA (Root Cause Analysis)**: Use correlation data + 5 WHY methodology to identify root causes

**Important**: Without KPIs defined, Failure Detection will have limited effectiveness. Without Failures and KPIs, RCA cannot function.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Mirador Core API                          │
│                  (localhost:8010)                           │
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
```

---

## Prerequisites & Setup

### System Requirements

**Mirador Core:**
- Go 1.21+ (for building from source)
- Docker & Docker Compose (for containerized deployment)
- 4GB RAM minimum (8GB recommended)
- 20GB disk space

**Required Backend Services:**
- VictoriaMetrics (metrics storage)
- VictoriaLogs (logs storage)
- VictoriaTraces (traces storage)
- Valkey/Redis (caching)
- Weaviate (optional, for KPI vector storage)

### Quick Start

**Using Docker Compose (Recommended):**

```bash
# 1. Clone repository
git clone https://github.com/mirastacklabs-ai/mirador-core
cd mirador-core

# 2. Start all services
make localdev-up

# 3. Wait for services to be ready (monitors health checks)
make localdev-wait

# 4. Verify Mirador Core is running
curl http://localhost:8010/api/v1/health

# 5. Seed sample KPIs (optional)
make localdev-seed-data
```

**Expected Output:**
```json
{
  "status": "healthy",
  "timestamp": "2026-01-23T10:00:00Z",
  "services": {
    "mirador-core": "ok",
    "victoriametrics": "ok",
    "victorialogs": "ok",
    "victoriatraces": "ok",
    "valkey": "ok"
  }
}
```

### Access Points

Once running, you can access:

- **Mirador Core API**: `http://localhost:8010`
- **Swagger UI**: `http://localhost:8010/swagger/index.html`
- **OpenAPI Spec**: `http://localhost:8010/api/openapi.yaml`
- **Health Check**: `http://localhost:8010/health`
- **Prometheus Metrics**: `http://localhost:8010/metrics`

---

## Configuration & Deployment

### Configuration File Structure

Mirador Core uses a YAML configuration file located at `configs/config.yaml`:

```yaml
# Basic Settings
environment: production  # or development
port: 8010
log_level: info

# VictoriaMetrics Ecosystem
database:
  victoria_metrics:
    endpoints:
      - "http://victoriametrics:8428"
    timeout: 30000
    cluster_mode: false
    
  victoria_logs:
    endpoints:
      - "http://victorialogs:9428"
    timeout: 30000
    
  victoria_traces:
    endpoints:
      - "http://victoriatraces:10428"
    timeout: 30000

# Caching (Valkey/Redis)
cache:
  nodes:
    - "valkey:6379"
  ttl: 300  # 5 minutes default
  password: ""  # Set via env: CACHE_PASSWORD
  db: 0

# CORS (for frontend integration)
cors:
  allowed_origins:
    - "https://your-mirador-ui.com"
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"

# Weaviate (optional - for KPI vector storage)
weaviate:
  enabled: true
  scheme: "http"
  host: "weaviate"
  port: 8080
  vectorizer:
    provider: "text2vec-transformers"
    model: "sentence-transformers/all-MiniLM-L6-v2"
    use_gpu: false

# Engine Configuration
engine:
  # Time window constraints
  min_window: 1m
  max_window: 1h
  
  # Payload validation
  strict_time_window_payload: true
  strict_time_window: true
  
  # Correlation settings
  correlation_threshold: 0.7
  default_graph_hops: 3
  default_max_whys: 5
  
  # Ring strategy for RCA
  ring_strategy: "default"
  
  # Query limits
  default_query_limit: 1000
```

### Environment Variables

Override configuration via environment variables:

```bash
# Database credentials
export VM_PASSWORD="your-victoriametrics-password"
export VL_PASSWORD="your-victorialogs-password"

# Cache credentials
export CACHE_PASSWORD="your-valkey-password"

# Application settings
export PORT=8010
export LOG_LEVEL=info
export ENVIRONMENT=production

# Weaviate connection
export WEAVIATE_HOST=weaviate
export WEAVIATE_PORT=8080
```

### Docker Deployment

**Production docker-compose.yml:**

```yaml
version: '3.8'

services:
  mirador-core:
    image: miradorstack/mirador-core:latest
    container_name: mirador-core
    ports:
      - "8010:8010"
    environment:
      - ENVIRONMENT=production
      - LOG_LEVEL=info
      - VM_ENDPOINT=http://victoriametrics:8428
      - VL_ENDPOINT=http://victorialogs:9428
      - VT_ENDPOINT=http://victoriatraces:10428
      - CACHE_NODES=valkey:6379
      - CACHE_PASSWORD=${CACHE_PASSWORD}
      - WEAVIATE_HOST=weaviate
      - WEAVIATE_PORT=8080
    volumes:
      - ./configs:/app/configs:ro
    depends_on:
      - victoriametrics
      - victorialogs
      - victoriatraces
      - valkey
      - weaviate
    restart: unless-stopped
    networks:
      - mirador-net
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8010/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  victoriametrics:
    image: victoriametrics/victoria-metrics:latest
    container_name: victoriametrics
    ports:
      - "8428:8428"
    volumes:
      - vmdata:/victoria-metrics-data
    command:
      - --storageDataPath=/victoria-metrics-data
      - --search.maxUniqueTimeseries=2000000
      - --memory.allowedPercent=90
    networks:
      - mirador-net
    restart: unless-stopped

  victorialogs:
    image: victoriametrics/victoria-logs:latest
    container_name: victorialogs
    ports:
      - "9428:9428"
    volumes:
      - vldata:/victoria-logs-data
    command:
      - --storageDataPath=/victoria-logs-data
    networks:
      - mirador-net
    restart: unless-stopped

  victoriatraces:
    image: victoriametrics/victoria-traces:latest
    container_name: victoriatraces
    ports:
      - "10428:10428"
    volumes:
      - vtdata:/victoria-traces-data
    command:
      - --storageDataPath=/victoria-traces-data
    networks:
      - mirador-net
    restart: unless-stopped

  valkey:
    image: valkey/valkey:latest
    container_name: valkey
    ports:
      - "6379:6379"
    command: >
      valkey-server
      --requirepass ${CACHE_PASSWORD}
      --maxmemory 2gb
      --maxmemory-policy allkeys-lru
    volumes:
      - valkeydata:/data
    networks:
      - mirador-net
    restart: unless-stopped

  weaviate:
    image: semitechnologies/weaviate:latest
    container_name: weaviate
    ports:
      - "8080:8080"
    environment:
      QUERY_DEFAULTS_LIMIT: 25
      AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED: 'true'
      PERSISTENCE_DATA_PATH: '/var/lib/weaviate'
      DEFAULT_VECTORIZER_MODULE: 'text2vec-transformers'
      ENABLE_MODULES: 'text2vec-transformers'
      TRANSFORMERS_INFERENCE_API: 'http://t2v-transformers:8080'
    volumes:
      - weaviatedata:/var/lib/weaviate
    networks:
      - mirador-net
    restart: unless-stopped

  t2v-transformers:
    image: semitechnologies/transformers-inference:sentence-transformers-all-MiniLM-L6-v2
    container_name: t2v-transformers
    environment:
      ENABLE_CUDA: '0'
    networks:
      - mirador-net
    restart: unless-stopped

networks:
  mirador-net:
    driver: bridge

volumes:
  vmdata:
  vldata:
  vtdata:
  valkeydata:
  weaviatedata:
```

**Start production environment:**

```bash
# Export required environment variables
export CACHE_PASSWORD="your-secure-password"

# Start services
docker-compose up -d

# Check logs
docker-compose logs -f mirador-core

# Verify health
curl http://localhost:8010/api/v1/health
```

### Kubernetes Deployment

For Kubernetes deployments, see `deployments/k8s/` directory which includes:

- Deployment manifests
- Service definitions
- ConfigMaps and Secrets
- Ingress configuration
- Horizontal Pod Autoscaler (HPA)

**Quick deploy:**

```bash
# Apply all K8s resources
kubectl apply -f deployments/k8s/

# Check deployment status
kubectl get pods -n mirador

# Port-forward for local access
kubectl port-forward -n mirador svc/mirador-core 8010:8010
```

---

## Component 1: KPI Management

### What are KPIs?

**Key Performance Indicators (KPIs)** are the foundation of observability in Mirador Core. They define:

- **What metrics to monitor** (e.g., API error rates, latency, transaction volume)
- **Where to find the data** (metrics, logs, traces)
- **How to query it** (formulas, query objects)
- **Business context** (impact layer vs cause layer, sentiment, domain)

KPIs enable:
- **Registry-driven monitoring**: Central source of truth for all monitored signals
- **Correlation discovery**: Automatic relationship detection between metrics
- **RCA accuracy**: Better root cause chains with pre-defined impact/cause layers
- **Natural language search**: Vector-based semantic search over KPI descriptions

### KPI Structure

A KPI definition contains:

```json
{
  "id": "kpi-uuid-123",
  "name": "api_errors_total",
  "kind": "tech",
  "layer": "impact",
  "signalType": "metrics",
  "sentiment": "negative",
  "classifier": "errors",
  "datastore": "victoriametrics",
  "queryType": "MetricsQL",
  "unit": "count",
  "format": "integer",
  "formula": "sum(rate(http_requests_total{status=~\"5..\"}[5m]))",
  "definition": "Total API errors at the gateway per minute",
  "businessImpact": "Revenue loss due to failed customer transactions",
  "description": "Tracks API gateway errors - critical service health indicator",
  "tags": ["api", "errors", "critical"],
  "dataType": "timeseries",
  "aggregationWindowHint": "1m",
  "dimensionsHint": ["service.name", "region"],
  "refreshInterval": 60,
  "isShared": true,
  "userId": "user-uuid-456"
}
```

**Required Fields:**
- `name`: Unique identifier
- `layer`: `impact` (business/user-facing) or `cause` (infrastructure/technical)
- `sentiment`: `positive`, `negative`, or `neutral`
- `signalType`: `metrics`, `logs`, `traces`, `business`, `synthetic`

**Optional But Recommended:**
- `formula` or `query`: How to fetch the data
- `businessImpact`: Why this metric matters
- `tags`: For filtering and organization

### Creating KPIs

**Single KPI Creation:**

```bash
POST /api/v1/kpi/defs
Content-Type: application/json

{
  "kpiDefinition": {
    "name": "payment_processing_errors",
    "kind": "business",
    "layer": "impact",
    "signalType": "metrics",
    "sentiment": "negative",
    "classifier": "errors",
    "datastore": "victoriametrics",
    "queryType": "MetricsQL",
    "unit": "count",
    "format": "integer",
    "formula": "sum(rate(payment_errors_total[1m]))",
    "definition": "Failed payment transactions per minute",
    "businessImpact": "Direct revenue loss - each error = failed customer transaction",
    "description": "Critical business KPI tracking payment processing health",
    "tags": ["payments", "critical", "revenue"],
    "dataType": "timeseries",
    "aggregationWindowHint": "1m",
    "dimensionsHint": ["payment_method", "region"],
    "serviceFamily": "payments",
    "domain": "transactions",
    "refreshInterval": 60,
    "isShared": true
  }
}
```

**Response (201 Created):**

```json
{
  "status": "created",
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479"
}
```

**Using Query Object (Alternative to Formula):**

```json
{
  "kpiDefinition": {
    "name": "database_latency_p99",
    "kind": "tech",
    "layer": "cause",
    "signalType": "metrics",
    "sentiment": "negative",
    "queryType": "MetricsQL",
    "query": {
      "metric": "db_query_duration_seconds",
      "aggregation": "quantile",
      "quantile": 0.99,
      "window": "5m"
    },
    "unit": "seconds",
    "format": "float",
    "definition": "99th percentile database query latency",
    "description": "Tracks database performance degradation",
    "tags": ["database", "latency", "performance"]
  }
}
```

### Bulk KPI Import

**JSON Bulk Import:**

```bash
POST /api/v1/kpi/defs/bulk-json
Content-Type: application/json

{
  "kpiDefinitions": [
    {
      "name": "api_latency_p95",
      "layer": "impact",
      "sentiment": "negative",
      "signalType": "metrics",
      "formula": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
      "description": "API 95th percentile latency"
    },
    {
      "name": "kafka_consumer_lag",
      "layer": "cause",
      "sentiment": "negative",
      "signalType": "metrics",
      "formula": "sum(kafka_consumer_lag_max) by (consumer_group)",
      "description": "Kafka consumer lag indicating processing delays"
    }
  ]
}
```

**CSV Bulk Import:**

```bash
POST /api/v1/kpi/defs/bulk-csv
Content-Type: text/csv

name,layer,sentiment,signalType,formula,description
cpu_usage_percent,cause,negative,metrics,avg(rate(node_cpu_seconds_total[5m])) * 100,CPU utilization percentage
memory_usage_percent,cause,negative,metrics,100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes),Memory utilization percentage
disk_io_wait,cause,negative,metrics,rate(node_disk_io_time_seconds_total[5m]),Disk I/O wait time
```

### Listing and Filtering KPIs

**Get all KPIs (paginated):**

```bash
GET /api/v1/kpi/defs?limit=10&offset=0
```

**Filter by layer:**

```bash
GET /api/v1/kpi/defs?layer=impact&limit=50
```

**Filter by tags:**

```bash
GET /api/v1/kpi/defs?tags=critical,payments
```

**Filter by multiple criteria:**

```bash
GET /api/v1/kpi/defs?layer=cause&sentiment=negative&signalType=metrics&classifier=latency
```

**Response:**

```json
{
  "kpiDefinitions": [
    {
      "id": "kpi-uuid-1",
      "name": "api_errors_total",
      "layer": "impact",
      "sentiment": "negative",
      "description": "Total API gateway errors"
    }
  ],
  "total": 150,
  "nextOffset": 10
}
```

### Searching KPIs (Natural Language)

Mirador Core supports vector-based semantic search over KPI descriptions:

```bash
POST /api/v1/kpi/search
Content-Type: application/json

{
  "query": "payment transaction failures affecting revenue",
  "limit": 5
}
```

**Response:**

```json
{
  "results": [
    {
      "id": "kpi-uuid-123",
      "name": "payment_processing_errors",
      "description": "Failed payment transactions per minute",
      "score": 0.92
    },
    {
      "id": "kpi-uuid-456",
      "name": "transaction_timeout_total",
      "description": "Payment transactions timing out",
      "score": 0.85
    }
  ]
}
```

### Retrieving a Single KPI

```bash
GET /api/v1/kpi/defs/{id}
```

**Example:**

```bash
GET /api/v1/kpi/defs/f47ac10b-58cc-4372-a567-0e02b2c3d479
```

**Response:**

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "name": "payment_processing_errors",
  "kind": "business",
  "layer": "impact",
  "signalType": "metrics",
  "sentiment": "negative",
  "formula": "sum(rate(payment_errors_total[1m]))",
  "definition": "Failed payment transactions per minute",
  "businessImpact": "Direct revenue loss",
  "description": "Critical business KPI tracking payment processing health",
  "tags": ["payments", "critical", "revenue"],
  "createdAt": "2026-01-20T10:00:00Z",
  "updatedAt": "2026-01-20T10:00:00Z"
}
```

### Updating a KPI

Updates use the same endpoint as creation (upsert behavior):

```bash
POST /api/v1/kpi/defs
Content-Type: application/json

{
  "kpiDefinition": {
    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
    "name": "payment_processing_errors",
    "formula": "sum(rate(payment_errors_total[5m]))",  # Changed from 1m to 5m
    "description": "Updated: 5-minute aggregation window"
  }
}
```

**Response (200 OK):**

```json
{
  "status": "ok",
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479"
}
```

### Deleting a KPI

```bash
DELETE /api/v1/kpi/defs/{id}
```

**Example:**

```bash
DELETE /api/v1/kpi/defs/f47ac10b-58cc-4372-a567-0e02b2c3d479
```

**Response (200 OK):**

```json
{
  "status": "deleted",
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479"
}
```

---

## Component 2: Failure Detection

### What is Failure Detection?

Failure Detection analyzes telemetry data (metrics, logs, traces) within a time window to identify component failures. It:

- **Detects error spans**: Traces with `error=true` tag
- **Identifies anomalies**: Metrics with `iforest_is_anomaly=true` (from AI anomaly detection)
- **Groups by service+component**: Aggregates failures per component
- **Generates unique IDs**: Deterministic UUIDs for deduplication
- **Persists to storage**: Saves failures to Weaviate for historical analysis

### When to Use Failure Detection

- **Incident investigation**: "What failed between 10:00 and 11:00?"
- **Post-mortem analysis**: Identify all affected components during an outage
- **Pre-RCA preparation**: Gather failure candidates before running RCA
- **Monitoring dashboards**: Track failure trends over time

### Detecting Failures

**Basic Detection (All Components):**

```bash
POST /api/v1/unified/failures/detect
Content-Type: application/json

{
  "time_range": {
    "start": "2026-01-23T10:00:00Z",
    "end": "2026-01-23T11:00:00Z"
  }
}
```

**Filtered Detection (Specific Components):**

```bash
POST /api/v1/unified/failures/detect
Content-Type: application/json

{
  "time_range": {
    "start": "2026-01-23T10:00:00Z",
    "end": "2026-01-23T11:00:00Z"
  },
  "components": ["kafka", "cassandra", "api-gateway"],
  "services": ["payment-service", "auth-service"]
}
```

**Response:**

```json
{
  "incidents": [
    {
      "incident_id": "incident_kafka_1737626400",
      "failure_id": "kafka-producer-kafka-20260123-100000",
      "failure_uuid": "e458d90f-f525-58a9-9e92-9f91faa73cf2",
      "time_range": {
        "start": "2026-01-23T10:00:00Z",
        "end": "2026-01-23T10:15:00Z"
      },
      "primary_component": "kafka",
      "affected_transaction_ids": ["txn-123", "txn-456"],
      "services_involved": ["payment-service", "order-service"],
      "failure_mode": "error_spans",
      "confidence": 0.92,
      "severity": "high"
    }
  ],
  "summary": {
    "total_incidents": 3,
    "time_range": {
      "start": "2026-01-23T10:00:00Z",
      "end": "2026-01-23T11:00:00Z"
    },
    "service_component_summaries": [
      {
        "service": "payment-service",
        "component": "kafka",
        "failure_id": "kafka-producer-kafka-20260123-100000",
        "failure_uuid": "e458d90f-f525-58a9-9e92-9f91faa73cf2",
        "failure_count": 42,
        "affected_transactions": 15,
        "average_anomaly_score": 0.87,
        "average_confidence": 0.92,
        "error_spans_count": 38,
        "error_metrics_count": 4,
        "last_failure_timestamp": "2026-01-23T10:14:32Z"
      }
    ],
    "metrics_error_summary": {
      "total_error_metrics": 12,
      "total_anomaly_metrics": 8,
      "error_metrics_by_name": [
        {
          "metric_name": "kafka_producer_errors_total",
          "count": 8,
          "average_value": 3.5,
          "last_timestamp": "2026-01-23T10:14:32Z"
        }
      ],
      "anomaly_metrics_by_name": [
        {
          "metric_name": "kafka_producer_latency_ms",
          "count": 5,
          "average_value": 1250.3,
          "last_timestamp": "2026-01-23T10:14:28Z"
        }
      ]
    }
  }
}
```

### Understanding Failure IDs

Each failure gets two identifiers:

1. **failure_id** (human-readable): `{service}-{component}-{YYYYMMDD-HHMMSS}`
   - Example: `kafka-producer-kafka-20260123-100000`
   - Easy to read in logs and dashboards

2. **failure_uuid** (deterministic UUID v5): `e458d90f-f525-58a9-9e92-9f91faa73cf2`
   - Unique identifier for storage and deduplication
   - Generated from: service + component + timestamp
   - Same failure detected multiple times = same UUID

### Transaction Failure Correlation

Correlate failures for specific transaction IDs to track cascading failures:

```bash
POST /api/v1/unified/failures/correlate
Content-Type: application/json

{
  "transactionIDs": ["txn-12345", "txn-67890"],
  "time_range": {
    "start": "2026-01-23T10:00:00Z",
    "end": "2026-01-23T11:00:00Z"
  }
}
```

**Response:**

```json
{
  "incidents": [
    {
      "incident_id": "incident_txn_12345",
      "failure_id": "transaction-correlation-20260123-100000",
      "affected_transaction_ids": ["txn-12345"],
      "services_involved": ["api-gateway", "payment-service", "kafka"],
      "failure_sequence": [
        {
          "service": "api-gateway",
          "component": "http-server",
          "timestamp": "2026-01-23T10:00:05Z",
          "error": "timeout waiting for payment-service"
        },
        {
          "service": "payment-service",
          "component": "kafka-producer",
          "timestamp": "2026-01-23T10:00:04Z",
          "error": "kafka broker unavailable"
        }
      ],
      "root_component": "kafka",
      "confidence": 0.95
    }
  ]
}
```

### Listing Stored Failures

Retrieve paginated list of historical failures:

```bash
POST /api/v1/unified/failures/list
Content-Type: application/json

{
  "limit": 20,
  "offset": 0,
  "filters": {
    "severity": "high",
    "component": "kafka"
  }
}
```

**Response:**

```json
{
  "failures": [
    {
      "id": "e458d90f-f525-58a9-9e92-9f91faa73cf2",
      "failure_id": "kafka-producer-kafka-20260123-100000",
      "summary": "Kafka producer failures in payment-service",
      "severity": "high",
      "timestamp": "2026-01-23T10:00:00Z",
      "component": "kafka",
      "service": "payment-service"
    }
  ],
  "total": 150,
  "nextOffset": 20
}
```

### Getting Failure Details

Retrieve full failure record with all signals and metadata:

```bash
POST /api/v1/unified/failures/get
Content-Type: application/json

{
  "id": "e458d90f-f525-58a9-9e92-9f91faa73cf2"
}
```

**Response:**

```json
{
  "failure": {
    "id": "e458d90f-f525-58a9-9e92-9f91faa73cf2",
    "failure_id": "kafka-producer-kafka-20260123-100000",
    "time_range": {
      "start": "2026-01-23T10:00:00Z",
      "end": "2026-01-23T10:15:00Z"
    },
    "primary_component": "kafka",
    "services": "payment-service",
    "affected_transaction_count": 15,
    "error_signals": [
      {
        "timestamp": "2026-01-23T10:00:05Z",
        "service": "payment-service",
        "component": "kafka-producer",
        "error_message": "broker unavailable",
        "trace_id": "abc123",
        "span_id": "def456"
      }
    ],
    "anomaly_signals": [
      {
        "timestamp": "2026-01-23T10:00:03Z",
        "metric_name": "kafka_producer_latency_ms",
        "value": 1250.3,
        "is_anomaly": true,
        "anomaly_score": 0.92
      }
    ],
    "metadata": {
      "detection_timestamp": "2026-01-23T10:16:00Z",
      "detection_confidence": 0.92,
      "total_signals": 42
    }
  }
}
```

### Deleting a Failure

Remove a failure record from storage:

```bash
POST /api/v1/unified/failures/delete
Content-Type: application/json

{
  "id": "e458d90f-f525-58a9-9e92-9f91faa73cf2"
}
```

**Response:**

```json
{
  "status": "deleted",
  "id": "e458d90f-f525-58a9-9e92-9f91faa73cf2"
}
```

---

## Component 3: Correlation Analysis

### What is Correlation Analysis?

Correlation Analysis performs **statistical analysis** between KPIs to discover relationships and patterns. It:

- **Builds temporal rings**: Divides time window into rings (R1: immediate, R2: short, R3: medium, R4: long)
- **Discovers impact KPIs**: Identifies metrics showing degradation (red anchors)
- **Finds candidate causes**: Detects correlated metrics that may explain the impact
- **Computes statistics**: Pearson, Spearman, cross-correlation, partial correlation
- **Scores candidates**: Assigns suspicion scores based on statistical strength

### Correlation vs RCA

| Aspect | Correlation | RCA |
|--------|-------------|-----|
| **Purpose** | Find statistical relationships | Explain root cause |
| **Output** | List of correlated KPIs with scores | 5-WHY chains with narrative |
| **Method** | Statistical analysis | Correlation + reasoning |
| **Use Case** | "What else changed?" | "Why did it fail?" |

**Key Insight**: RCA **uses** Correlation results internally, then adds causal reasoning.

### Running Correlation Analysis

**Time-Window Correlation (Recommended):**

```bash
POST /api/v1/unified/correlation
Content-Type: application/json

{
  "startTime": "2026-01-23T10:00:00Z",
  "endTime": "2026-01-23T11:00:00Z"
}
```

**Response:**

```json
{
  "status": "success",
  "result": {
    "correlationID": "corr_1737626400",
    "timeRange": {
      "start": "2026-01-23T10:00:00Z",
      "end": "2026-01-23T11:00:00Z"
    },
    "rings": {
      "R1_IMMEDIATE": {
        "label": "R1_IMMEDIATE",
        "description": "Anomalies very close to the peak",
        "duration": "5s",
        "start": "2026-01-23T10:55:55Z",
        "end": "2026-01-23T11:00:00Z"
      },
      "R2_SHORT": {
        "label": "R2_SHORT",
        "description": "Anomalies shortly before peak",
        "duration": "30s",
        "start": "2026-01-23T10:55:25Z",
        "end": "2026-01-23T10:55:55Z"
      },
      "R3_MEDIUM": {
        "label": "R3_MEDIUM",
        "description": "Anomalies moderately before peak",
        "duration": "2m",
        "start": "2026-01-23T10:53:25Z",
        "end": "2026-01-23T10:55:25Z"
      },
      "R4_LONG": {
        "label": "R4_LONG",
        "description": "Anomalies further back",
        "duration": "10m",
        "start": "2026-01-23T10:43:25Z",
        "end": "2026-01-23T10:53:25Z"
      }
    },
    "affectedServices": ["payment-service", "kafka"],
    "redAnchors": [
      {
        "service": "payment-service",
        "metric": "payment_processing_errors",
        "score": 0.95,
        "ring": "R1_IMMEDIATE",
        "labelFingerprint": {
          "service.name": "payment-service",
          "region": "us-east-1"
        }
      }
    ],
    "causes": [
      {
        "kpi": "kafka_producer_latency_ms",
        "service": "payment-service",
        "suspicionScore": 0.89,
        "ring": "R2_SHORT",
        "reasons": [
          "high_pearson_correlation",
          "high_spearman_correlation",
          "temporal_precedence",
          "high_anomaly_density"
        ],
        "stats": {
          "pearson": 0.87,
          "spearman": 0.91,
          "crossCorrMax": 0.88,
          "crossCorrLag": -2,
          "partial": 0.82
        },
        "labelFingerprint": {
          "service.name": "payment-service",
          "component": "kafka-producer"
        }
      },
      {
        "kpi": "kafka_broker_connection_errors",
        "service": "kafka",
        "suspicionScore": 0.92,
        "ring": "R3_MEDIUM",
        "reasons": [
          "high_pearson_correlation",
          "temporal_precedence",
          "upstream_component"
        ],
        "stats": {
          "pearson": 0.93,
          "spearman": 0.89,
          "crossCorrMax": 0.91,
          "crossCorrLag": -5,
          "partial": 0.85
        }
      }
    ],
    "confidence": 0.91,
    "createdAt": "2026-01-23T11:01:00Z"
  }
}
```

### Understanding Correlation Results

**Red Anchors:**
- Metrics showing **impact** (business/user-facing degradation)
- Typically from KPIs with `layer=impact`
- High scores = strong impact signal

**Cause Candidates:**
- Metrics that **correlate** with red anchors
- Typically from KPIs with `layer=cause`
- Ranked by suspicion score (higher = more suspicious)

**Statistics Explained:**

| Metric | Meaning | Range |
|--------|---------|-------|
| **Pearson** | Linear correlation strength | -1 to +1 |
| **Spearman** | Rank correlation (monotonic relationship) | -1 to +1 |
| **CrossCorrMax** | Maximum cross-correlation | -1 to +1 |
| **CrossCorrLag** | Time lag (negative = cause precedes impact) | seconds |
| **Partial** | Correlation after removing confounders | -1 to +1 |

**Suspicion Score Calculation:**
```
suspicionScore = weighted_average(
  pearson_correlation,
  spearman_correlation,
  cross_correlation_max,
  partial_correlation,
  anomaly_density,
  temporal_precedence
)
```

**Reasons (Why This Candidate is Suspicious):**
- `high_pearson_correlation`: Strong linear relationship
- `high_spearman_correlation`: Strong monotonic relationship
- `temporal_precedence`: Cause occurred before impact
- `high_anomaly_density`: Many anomalies in this metric
- `upstream_component`: Component is upstream in service graph

### Time Window Constraints

Correlation enforces time window limits from configuration:

```yaml
engine:
  min_window: 1m   # Minimum analysis window
  max_window: 1h   # Maximum analysis window
```

**Invalid Windows:**
```bash
# Too small
{"startTime": "2026-01-23T10:00:00Z", "endTime": "2026-01-23T10:00:30Z"}
# Error: time window too small: 30s < minWindow 1m

# Too large
{"startTime": "2026-01-23T00:00:00Z", "endTime": "2026-01-23T23:59:59Z"}
# Error: time window too large: 23h59m59s > maxWindow 1h
```

---

## Component 4: Root Cause Analysis (RCA)

### What is RCA?

Root Cause Analysis (RCA) combines:

1. **Correlation results** (statistical relationships)
2. **5 WHY methodology** (iterative questioning)
3. **Service topology** (upstream/downstream relationships)
4. **Temporal rings** (time-based evidence)

To produce **human-readable explanations** of why incidents occurred.

### RCA Process Flow

```
1. User provides time window
        ↓
2. RCA engine calls Correlation engine
        ↓
3. Correlation returns:
   - Red anchors (impacts)
   - Cause candidates (correlated metrics)
   - Statistical evidence
        ↓
4. RCA builds 5-WHY chains:
   - WHY 1: Business impact (what failed?)
   - WHY 2: Entry service degradation
   - WHY 3-5: Upstream causes (evidence-driven)
        ↓
5. Returns structured narrative with:
   - Impact summary
   - Causal chains
   - Time rings
   - Diagnostic details
```

### Running RCA

**Basic RCA Request:**

```bash
POST /api/v1/unified/rca
Content-Type: application/json

{
  "startTime": "2026-01-23T10:00:00Z",
  "endTime": "2026-01-23T11:00:00Z"
}
```

**Response:**

```json
{
  "status": "success",
  "data": {
    "impact": {
      "id": "incident_payment_service_1737626400",
      "impactService": "payment-service",
      "metricName": "payment_processing_errors",
      "timeStart": "2026-01-23T10:00:00Z",
      "timeEnd": "2026-01-23T11:00:00Z",
      "impactSummary": "Impact detected on payment-service (correlation confidence 0.91). Top-candidate kafka_producer_latency_ms: pearson=0.87 spearman=0.91 partial=0.82 cross_max=0.88 lag=-2 anomalies=HIGH",
      "severity": 0.91
    },
    "chains": [
      {
        "steps": [
          {
            "why": 1,
            "service": "payment-service",
            "kpiName": "payment_processing_errors",
            "timeRange": {
              "start": "2026-01-23T10:00:00Z",
              "end": "2026-01-23T11:00:00Z"
            },
            "ring": "R1_IMMEDIATE",
            "direction": "SAME",
            "score": 0.91,
            "evidence": [
              {
                "type": "red_anchor",
                "key": "payment-service",
                "value": "anchor_score=0.950"
              }
            ],
            "summary": "Payment processing failed: payment_processing_errors increased dramatically in R1_IMMEDIATE (0.91 confidence)"
          },
          {
            "why": 2,
            "service": "payment-service",
            "kpiName": "payment_processing_errors",
            "timeRange": {
              "start": "2026-01-23T10:00:00Z",
              "end": "2026-01-23T11:00:00Z"
            },
            "ring": "R1_IMMEDIATE",
            "direction": "UPSTREAM",
            "score": 0.95,
            "evidence": [
              {
                "type": "red_anchor",
                "key": "payment-service",
                "value": "metric=payment_processing_errors score=0.950"
              }
            ],
            "summary": "payment-service degraded: payment_processing_errors showed anomalies in R1_IMMEDIATE (0.95 confidence)"
          },
          {
            "why": 3,
            "service": "payment-service",
            "kpiName": "kafka_producer_latency_ms",
            "timeRange": {
              "start": "2026-01-23T10:00:00Z",
              "end": "2026-01-23T11:00:00Z"
            },
            "ring": "R2_SHORT",
            "direction": "UPSTREAM",
            "score": 0.89,
            "evidence": [
              {
                "type": "correlation_stats",
                "key": "kafka_producer_latency_ms",
                "value": "pearson=0.87 spearman=0.91 cross_lag=-2 suspicion=0.89"
              },
              {
                "type": "correlation_reason",
                "key": "kafka_producer_latency_ms",
                "value": "high_pearson_correlation"
              },
              {
                "type": "correlation_reason",
                "key": "kafka_producer_latency_ms",
                "value": "temporal_precedence"
              }
            ],
            "summary": "Upstream component kafka_producer_latency_ms caused payment-service degradation (0.89 suspicion, pearson=0.87)"
          },
          {
            "why": 4,
            "service": "kafka",
            "kpiName": "kafka_broker_connection_errors",
            "timeRange": {
              "start": "2026-01-23T10:00:00Z",
              "end": "2026-01-23T11:00:00Z"
            },
            "ring": "R2_SHORT",
            "direction": "UPSTREAM",
            "score": 0.92,
            "evidence": [
              {
                "type": "correlation_stats",
                "key": "kafka_broker_connection_errors",
                "value": "pearson=0.93 spearman=0.89 cross_lag=-5 suspicion=0.92"
              },
              {
                "type": "correlation_reason",
                "key": "kafka_broker_connection_errors",
                "value": "high_pearson_correlation"
              },
              {
                "type": "correlation_reason",
                "key": "kafka_broker_connection_errors",
                "value": "upstream_component"
              }
            ],
            "summary": "Upstream component kafka_broker_connection_errors caused payment-service degradation (0.92 suspicion, pearson=0.93)"
          }
        ],
        "score": 0.91,
        "confidence": 0.91
      }
    ],
    "generatedAt": "2026-01-23T11:01:30Z",
    "score": 0.91,
    "notes": [],
    "diagnostics": {},
    "timeRings": {
      "definitions": {
        "R1_IMMEDIATE": {
          "label": "R1_IMMEDIATE",
          "description": "Anomalies very close to the peak",
          "duration": "5s"
        },
        "R2_SHORT": {
          "label": "R2_SHORT",
          "description": "Anomalies shortly before peak",
          "duration": "30s"
        },
        "R3_MEDIUM": {
          "label": "R3_MEDIUM",
          "description": "Anomalies moderately before peak",
          "duration": "2m"
        },
        "R4_LONG": {
          "label": "R4_LONG",
          "description": "Anomalies further back",
          "duration": "10m"
        }
      },
      "perChain": []
    }
  },
  "timestamp": "2026-01-23T11:01:30Z"
}
```

### Understanding RCA Output

**Impact Section:**
- **impactService**: Which service was affected
- **metricName**: What metric degraded
- **impactSummary**: Human-readable summary with statistical evidence
- **severity**: Confidence score (0-1)

**Chains (5-WHY Chains):**

Each chain represents one possible root cause path:

**Step Structure:**
- **why**: Step number (1-5)
- **service**: Service involved at this step
- **kpiName**: Metric/KPI at this step
- **ring**: Temporal ring (when it occurred)
- **direction**: `SAME` (impact), `UPSTREAM` (cause), `DOWNSTREAM` (effect)
- **score**: Confidence for this step
- **evidence**: Statistical and correlation evidence
- **summary**: Human-readable explanation

**Chain Scoring:**
- Weighted average: earlier steps (WHY 1-2) weighted higher
- Higher score = more confident root cause path
- Multiple chains = multiple possible root causes (sorted by score)

**Time Rings:**
- **R1_IMMEDIATE** (5s): Events very close to peak
- **R2_SHORT** (30s): Events shortly before peak
- **R3_MEDIUM** (2m): Events moderately before peak
- **R4_LONG** (10m): Events further back

Rings help identify temporal ordering (cause precedes effect).

### Interpreting a 5-WHY Chain

Example chain interpretation:

```
WHY 1 (Business Impact): "Payment processing failed"
  → What the user experienced
  → Business/revenue impact

WHY 2 (Entry Service): "payment-service degraded"
  → Which service exhibited the problem
  → Where the impact manifested

WHY 3 (Direct Cause): "kafka_producer_latency_ms increased"
  → Immediate technical cause
  → Component directly affecting service

WHY 4 (Upstream Cause): "kafka_broker_connection_errors occurred"
  → Root infrastructure issue
  → What actually triggered the cascade

WHY 5 (Optional): Further upstream causes if available
```

### Low Confidence RCA

If no correlation data is available:

**Request:**
```json
{
  "startTime": "2026-01-01T00:00:00Z",
  "endTime": "2026-01-01T00:05:00Z"
}
```

**Response:**
```json
{
  "status": "success",
  "data": {
    "impact": {
      "id": "incident_unknown",
      "impactService": "unknown",
      "metricName": "unknown",
      "impactSummary": "No correlation data for window 2026-01-01 00:00:00 +0000 UTC - 2026-01-01 00:05:00 +0000 UTC",
      "severity": 0
    },
    "chains": [],
    "score": 0,
    "notes": ["Correlation produced no candidates; returning low-confidence RCA"]
  }
}
```

This indicates:
- No KPIs were found with degradation in this window
- Or KPI registry is empty
- Or VictoriaMetrics/Logs/Traces have no data for this period

**Resolution:**
1. Verify KPIs are defined
2. Check time window contains actual incidents
3. Ensure telemetry data exists in VictoriaMetrics/Logs/Traces

---

## Complete Workflows

### Workflow 1: Full Incident Investigation

**Scenario**: Payment processing outage on Jan 23, 2026 between 10:00-11:00

**Step 1: Define KPIs (if not already done)**

```bash
# Define impact KPI
POST /api/v1/kpi/defs
{
  "kpiDefinition": {
    "name": "payment_processing_errors",
    "layer": "impact",
    "sentiment": "negative",
    "signalType": "metrics",
    "formula": "sum(rate(payment_errors_total[1m]))",
    "description": "Failed payment transactions"
  }
}

# Define cause KPIs
POST /api/v1/kpi/defs
{
  "kpiDefinition": {
    "name": "kafka_producer_latency_ms",
    "layer": "cause",
    "sentiment": "negative",
    "signalType": "metrics",
    "formula": "histogram_quantile(0.99, kafka_producer_latency_bucket)",
    "description": "Kafka producer p99 latency"
  }
}

POST /api/v1/kpi/defs
{
  "kpiDefinition": {
    "name": "database_connection_pool_exhausted",
    "layer": "cause",
    "sentiment": "negative",
    "signalType": "metrics",
    "formula": "db_pool_active / db_pool_max > 0.95",
    "description": "Database connection pool near capacity"
  }
}
```

**Step 2: Detect Failures**

```bash
POST /api/v1/unified/failures/detect
{
  "time_range": {
    "start": "2026-01-23T10:00:00Z",
    "end": "2026-01-23T11:00:00Z"
  }
}
```

**Result**: Identified failures in:
- payment-service (kafka component)
- database-service (connection pool)
- api-gateway (timeouts)

**Step 3: Run Correlation Analysis**

```bash
POST /api/v1/unified/correlation
{
  "startTime": "2026-01-23T10:00:00Z",
  "endTime": "2026-01-23T11:00:00Z"
}
```

**Result**: Correlation found:
- payment_processing_errors (impact)
- kafka_producer_latency_ms (cause, suspicion=0.89)
- database_connection_pool_exhausted (cause, suspicion=0.75)

**Step 4: Run RCA**

```bash
POST /api/v1/unified/rca
{
  "startTime": "2026-01-23T10:00:00Z",
  "endTime": "2026-01-23T11:00:00Z"
}
```

**Result**: RCA chain:
1. WHY 1: Payment processing failed (business impact)
2. WHY 2: payment-service degraded (errors increased)
3. WHY 3: kafka_producer_latency_ms spiked (direct cause)
4. WHY 4: kafka_broker_connection_errors occurred (root cause)

**Conclusion**: Kafka broker connection failures caused producer latency, leading to payment processing errors.

### Workflow 2: Proactive Monitoring Setup

**Scenario**: Set up monitoring for a new microservice

**Step 1: Bulk Import KPIs**

```bash
# Create kpis.csv
name,layer,sentiment,signalType,formula,description
user_service_api_latency_p99,impact,negative,metrics,histogram_quantile(0.99\\, rate(http_request_duration_seconds_bucket{service=\"user-service\"}[5m])),API latency affecting users
user_service_error_rate,impact,negative,metrics,sum(rate(http_requests_total{service=\"user-service\"\\,status=~\"5..\"}[1m])) / sum(rate(http_requests_total{service=\"user-service\"}[1m])),Error rate impacting reliability
user_service_cpu_usage,cause,negative,metrics,avg(rate(process_cpu_seconds_total{service=\"user-service\"}[5m])) * 100,CPU utilization
user_service_memory_usage,cause,negative,metrics,process_resident_memory_bytes{service=\"user-service\"} / 1024 / 1024,Memory usage in MB
user_service_db_query_latency,cause,negative,metrics,histogram_quantile(0.95\\, rate(db_query_duration_seconds_bucket{service=\"user-service\"}[5m])),Database query latency

# Import
POST /api/v1/kpi/defs/bulk-csv < kpis.csv
```

**Step 2: Verify KPIs**

```bash
GET /api/v1/kpi/defs?tags=user-service&limit=10
```

**Step 3: Test Correlation**

```bash
# Run correlation for last hour
POST /api/v1/unified/correlation
{
  "startTime": "2026-01-23T10:00:00Z",
  "endTime": "2026-01-23T11:00:00Z"
}
```

**Step 4: Schedule Periodic RCA**

Set up a cron job or monitoring system to:
```bash
# Run RCA every 15 minutes for the last 15 minutes
*/15 * * * * curl -X POST http://mirador-core:8010/api/v1/unified/rca \
  -H "Content-Type: application/json" \
  -d "{\"startTime\":\"$(date -u -d '15 minutes ago' +%Y-%m-%dT%H:%M:%SZ)\",\"endTime\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
```

### Workflow 3: Historical Incident Analysis

**Scenario**: Analyze pattern of failures over past week

**Step 1: List Failures**

```bash
POST /api/v1/unified/failures/list
{
  "limit": 100,
  "offset": 0,
  "filters": {
    "start_time": "2026-01-16T00:00:00Z",
    "end_time": "2026-01-23T00:00:00Z",
    "severity": "high"
  }
}
```

**Step 2: Analyze Each Failure**

```bash
# For each failure_uuid from step 1
POST /api/v1/unified/failures/get
{
  "id": "e458d90f-f525-58a9-9e92-9f91faa73cf2"
}
```

**Step 3: Run RCA for Each Incident**

```bash
# For each incident time window
POST /api/v1/unified/rca
{
  "startTime": "<incident_start>",
  "endTime": "<incident_end>"
}
```

**Step 4: Aggregate Patterns**

Analyze RCA results to find:
- Common root causes (e.g., kafka broker issues appearing in 80% of incidents)
- Frequently affected services (e.g., payment-service)
- Temporal patterns (e.g., failures cluster around 10 AM)

---

## API Reference

### Base URL

```
http://localhost:8010
```

### Authentication

Mirador Core is designed to run behind an external API gateway or service mesh that handles authentication. No built-in authentication is required for API calls.

### Content Type

All requests must include:
```
Content-Type: application/json
```

### KPI Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/kpi/defs` | List KPI definitions |
| POST | `/api/v1/kpi/defs` | Create/update KPI definition |
| GET | `/api/v1/kpi/defs/{id}` | Get single KPI definition |
| DELETE | `/api/v1/kpi/defs/{id}` | Delete KPI definition |
| POST | `/api/v1/kpi/defs/bulk-json` | Bulk import KPIs (JSON) |
| POST | `/api/v1/kpi/defs/bulk-csv` | Bulk import KPIs (CSV) |
| POST | `/api/v1/kpi/search` | Semantic search for KPIs |

### Failure Detection Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/unified/failures/detect` | Detect component failures |
| POST | `/api/v1/unified/failures/correlate` | Correlate transaction failures |
| POST | `/api/v1/unified/failures/list` | List stored failures |
| POST | `/api/v1/unified/failures/get` | Get failure details |
| POST | `/api/v1/unified/failures/delete` | Delete failure record |

### Correlation Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/unified/correlation` | Run correlation analysis |

### RCA Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/unified/rca` | Compute root cause analysis |

### Internal Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| GET | `/metrics` | Prometheus metrics |
| GET | `/api/v1/health` | API v1 health |
| GET | `/api/openapi.yaml` | OpenAPI spec (YAML) |
| GET | `/api/openapi.json` | OpenAPI spec (JSON) |
| GET | `/swagger/index.html` | Swagger UI |

### Query Parameters

**KPI List Filtering:**
- `limit` (int): Max results (default: 10, max: 10000)
- `offset` (int): Pagination offset (default: 0)
- `tags` (string[]): Filter by tags (comma-separated)
- `layer` (string): Filter by layer (impact/cause)
- `sentiment` (string): Filter by sentiment (positive/negative/neutral)
- `signalType` (string): Filter by signal type (metrics/logs/traces/business/synthetic)
- `kind` (string): Filter by kind (business/tech)
- `classifier` (string): Filter by classifier
- `datastore` (string): Filter by datastore

**Example:**
```
GET /api/v1/kpi/defs?layer=impact&sentiment=negative&limit=50&tags=critical,payments
```

---

## Troubleshooting

### Problem: KPIs Not Being Detected by Correlation

**Symptoms:**
- Correlation returns empty results
- RCA shows "No correlation data"

**Diagnosis:**
```bash
# 1. Check if KPIs exist
GET /api/v1/kpi/defs?limit=10

# 2. Check if KPIs have formulas
GET /api/v1/kpi/defs/{id}
# Verify "formula" or "query" field is populated

# 3. Test formula manually against VictoriaMetrics
curl "http://victoriametrics:8428/api/v1/query?query=<your_formula>&time=$(date +%s)"
```

**Solutions:**
1. Ensure KPIs have valid `formula` or `query` fields
2. Verify VictoriaMetrics contains data for the formula
3. Check time window overlaps with actual data availability

### Problem: Failure Detection Returns Empty Results

**Symptoms:**
- `/unified/failures/detect` returns `"total_incidents": 0`

**Diagnosis:**
```bash
# 1. Check if traces exist
curl "http://victoriatraces:10428/api/v1/search?start=<start_epoch>&end=<end_epoch>&tags={}"

# 2. Check if metrics exist
curl "http://victoriametrics:8428/api/v1/query_range?query=up&start=<start>&end=<end>&step=60"

# 3. Verify time window format
# Must be RFC3339 UTC: "2026-01-23T10:00:00Z"
```

**Solutions:**
1. Ensure traces are being ingested to VictoriaTraces
2. Verify error spans have `error=true` tag
3. Check anomaly metrics have `iforest_is_anomaly=true` label
4. Confirm time window contains actual incident data

### Problem: RCA Returns Low Confidence

**Symptoms:**
- RCA score is 0 or very low
- Chains are empty
- Notes contain: "Correlation produced no candidates"

**Diagnosis:**
```bash
# 1. Run correlation first to see what it finds
POST /api/v1/unified/correlation
{
  "startTime": "...",
  "endTime": "..."
}

# 2. Check correlation response
# If empty, diagnose correlation (see above)

# 3. Verify KPIs have layer=impact and layer=cause defined
GET /api/v1/kpi/defs?layer=impact
GET /api/v1/kpi/defs?layer=cause
```

**Solutions:**
1. Define at least one `layer=impact` KPI
2. Define multiple `layer=cause` KPIs
3. Ensure time window contains degradation events
4. Run failure detection first to confirm incidents exist

### Problem: Time Window Validation Errors

**Symptoms:**
- `400 Bad Request: time window too small`
- `413 Payload Too Large: time window too large`

**Diagnosis:**
```bash
# Check engine configuration
curl http://localhost:8010/api/v1/unified/metadata | jq '.engineConfig'
```

**Solutions:**
1. Adjust window to respect `min_window` and `max_window`
2. Default constraints:
   - `min_window: 1m`
   - `max_window: 1h`
3. Update `configs/config.yaml` if constraints are too restrictive

### Problem: Weaviate Connection Failures

**Symptoms:**
- KPI creation fails with "weaviate unavailable"
- Failure detection works but failures aren't persisted

**Diagnosis:**
```bash
# 1. Check Weaviate health
curl http://weaviate:8080/v1/.well-known/ready

# 2. Check Mirador Core logs
docker logs mirador-core | grep -i weaviate

# 3. Verify Weaviate is enabled in config
cat configs/config.yaml | grep -A5 weaviate
```

**Solutions:**
1. Ensure Weaviate container is running: `docker ps | grep weaviate`
2. Verify network connectivity: `docker network inspect mirador-net`
3. Set `weaviate.enabled: true` in config
4. Restart Mirador Core: `docker restart mirador-core`

### Problem: High Memory Usage

**Symptoms:**
- Mirador Core container OOM killed
- Slow response times

**Diagnosis:**
```bash
# Check container memory
docker stats mirador-core

# Check VictoriaMetrics data volume
docker exec victoriametrics du -sh /victoria-metrics-data
```

**Solutions:**
1. Increase container memory limit in docker-compose.yml:
   ```yaml
   deploy:
     resources:
       limits:
         memory: 4G
   ```
2. Reduce `default_query_limit` in config.yaml
3. Enable more aggressive caching (increase TTL)
4. Reduce `max_window` to limit analysis scope

### Problem: Correlation Takes Too Long

**Symptoms:**
- Correlation requests timeout
- High CPU usage during correlation

**Diagnosis:**
```bash
# Check number of KPIs
GET /api/v1/kpi/defs | jq '.total'

# Check time window size
# Large windows = more data to analyze
```

**Solutions:**
1. Reduce time window (use 15m instead of 1h)
2. Reduce number of active KPIs (archive unused ones)
3. Increase timeout in config:
   ```yaml
   database:
     victoria_metrics:
       timeout: 60000  # 60 seconds
   ```
4. Scale Mirador Core horizontally (add more replicas)

### Getting Help

**Logs:**
```bash
# Mirador Core logs
docker logs mirador-core --tail 100 -f

# All services logs
docker-compose logs -f
```

**Health Checks:**
```bash
# Overall health
curl http://localhost:8010/api/v1/health | jq

# Service status
curl http://localhost:8010/microservices/status | jq
```

**Metrics:**
```bash
# Prometheus metrics
curl http://localhost:8010/metrics | grep mirador
```

**Support:**
- GitHub Issues: https://github.com/mirastacklabs-ai/mirador-core/issues
- Documentation: http://localhost:8010/swagger/index.html

---

## Summary

This guide covered the four core components of Mirador Core:

1. **KPIs**: Define and manage metrics (foundation)
2. **Failures**: Detect and track incidents
3. **Correlation**: Analyze statistical relationships
4. **RCA**: Explain root causes with 5-WHY methodology

**Key Takeaways:**
- Always define KPIs before running failure detection or RCA
- Use impact (`layer=impact`) and cause (`layer=cause`) KPIs for best results
- Correlation provides statistical evidence; RCA adds causal reasoning
- Time windows must respect configured min/max constraints
- All components are interconnected and build upon each other

For complete API documentation, see the [Swagger UI](http://localhost:8010/swagger/index.html).
