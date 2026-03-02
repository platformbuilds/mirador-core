# MariaDB Integration

This document describes how MIRADOR-CORE integrates with MariaDB for reading tenant data sources and KPI definitions.

## Overview

MIRADOR-CORE can connect to a MariaDB database (shared with mirador-ui) to read:

- **Data Sources**: VictoriaMetrics, VictoriaLogs, VictoriaTraces, Prometheus, and other telemetry endpoints
- **KPI Definitions**: KPI metadata, queries, thresholds, and configuration

This enables a unified configuration experience where data sources and KPIs configured in mirador-ui are automatically available in mirador-core.

### Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   mirador-ui    │────▶│    MariaDB      │◀────│  mirador-core   │
│  (read/write)   │     │ (tenant_slug)   │     │  (read-only)    │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                                        │
                                                        ▼
                                                ┌─────────────────┐
                                                │    Weaviate     │
                                                │  (KPI sync)     │
                                                └─────────────────┘
```

**Key Design Decisions:**

1. **One deployment = One tenant**: Each mirador-core deployment connects to a single tenant database
2. **Read-only access**: mirador-core never writes to MariaDB
3. **Graceful degradation**: If MariaDB is unavailable, mirador-core falls back to config.yaml static endpoints
4. **Background sync**: KPIs are synced from MariaDB to Weaviate via a background worker

## Configuration

### Basic Configuration

Add the `mariadb` section to your `config.yaml`:

```yaml
mariadb:
  enabled: true
  host: "mariadb.example.com"
  port: 3306
  database: "tenant_acme"      # Tenant-specific database name
  username: "mirador_core_ro"  # Read-only user
  password: ""                 # Set via environment variable
  
  # Connection pool settings (optional)
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 5m
  
  # KPI sync to Weaviate (optional)
  sync:
    enabled: true
    interval: 5m              # Sync interval
    batch_size: 100           # Batch size for sync operations
  
  # Bootstrap configuration for backward compatibility
  # When enabled, mirador-core creates tables and syncs data sources
  # from config.yaml to MariaDB on startup
  bootstrap:
    enabled: true
    create_tables_if_missing: true      # Create data_sources and kpis tables
    sync_datasources_from_config: true  # Sync config.yaml endpoints to MariaDB
```

### Bootstrap Feature (Backward Compatibility)

The bootstrap feature ensures seamless migration for existing users:

1. **Table Creation**: If the `data_sources` and `kpis` tables don't exist, mirador-core creates them automatically
2. **Data Source Sync**: Data sources defined in `config.yaml` are synced to MariaDB:
   - If a URL already exists in MariaDB → validated and skipped
   - If a URL doesn't exist in MariaDB → created from config

This enables existing users to continue using `config.yaml` while gradually migrating to MariaDB-backed configuration.

**Bootstrap Configuration Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `bootstrap.enabled` | Enable bootstrap on startup | `true` |
| `bootstrap.create_tables_if_missing` | Create tables if not present | `true` |
| `bootstrap.sync_datasources_from_config` | Sync config.yaml to MariaDB | `true` |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MARIADB_ENABLED` | Enable MariaDB integration | `false` |
| `MARIADB_HOST` | MariaDB host | `localhost` |
| `MARIADB_PORT` | MariaDB port | `3306` |
| `MARIADB_DATABASE` | Database name | `tenant_default` |
| `MARIADB_USERNAME` | Username | `mirador_core_ro` |
| `MARIADB_PASSWORD` | Password | - |
| `MARIADB_MAX_OPEN_CONNS` | Max open connections | `10` |
| `MARIADB_MAX_IDLE_CONNS` | Max idle connections | `5` |
| `MARIADB_CONN_MAX_LIFETIME` | Connection max lifetime | `5m` |
| `MARIADB_SYNC_ENABLED` | Enable KPI sync | `true` |
| `MARIADB_SYNC_INTERVAL` | Sync interval | `5m` |
| `MARIADB_SYNC_BATCH_SIZE` | Batch size | `100` |
| `MARIADB_BOOTSTRAP_ENABLED` | Enable bootstrap | `true` |
| `MARIADB_BOOTSTRAP_CREATE_TABLES` | Create tables if missing | `true` |
| `MARIADB_BOOTSTRAP_SYNC_DATASOURCES` | Sync datasources from config | `true` |

### Kubernetes Secrets

For production deployments, use Kubernetes secrets for credentials:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mirador-core-mariadb
type: Opaque
stringData:
  password: "your-secure-password"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mirador-core
spec:
  template:
    spec:
      containers:
        - name: mirador-core
          env:
            - name: MARIADB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: mirador-core-mariadb
                  key: password
```

## Database Schema

### data_sources Table

mirador-core reads from the `data_sources` table:

```sql
CREATE TABLE data_sources (
    id                          VARCHAR(36) PRIMARY KEY,
    name                        VARCHAR(255) NOT NULL,
    type                        VARCHAR(50) NOT NULL,
    project_identifier          VARCHAR(255),
    url                         VARCHAR(500) NOT NULL,
    api_key                     VARCHAR(255),
    username                    VARCHAR(255),
    password                    VARCHAR(255),
    
    -- Health check configuration
    health_url                  VARCHAR(500),
    health_expected_status      INT DEFAULT 200,
    health_body_type            VARCHAR(50),
    health_body_match_mode      VARCHAR(50),
    health_body_text_pattern    VARCHAR(255),
    health_body_json_key        VARCHAR(255),
    health_body_json_expected_value VARCHAR(255),
    health_check_interval_ms    INT DEFAULT 60000,
    
    is_active                   BOOLEAN DEFAULT TRUE,
    ai_config                   JSON,
    created_at                  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at                  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

**Supported data source types:**

| Type | Description |
|------|-------------|
| `prometheus` | Prometheus/VictoriaMetrics metrics endpoint |
| `victorialogs` | VictoriaLogs endpoint |
| `victoriatraces` | VictoriaTraces endpoint |
| `jaeger` | Jaeger tracing endpoint |
| `loki` | Grafana Loki logs endpoint |
| `miradorcore` | Another mirador-core instance |
| `aiengine` | AI/ML engine endpoint |

### kpis Table

mirador-core reads from the `kpis` table:

```sql
CREATE TABLE kpis (
    id                  VARCHAR(36) PRIMARY KEY,
    name                VARCHAR(255) NOT NULL,
    description         TEXT,
    data_type           VARCHAR(50) NOT NULL,
    definition          TEXT,
    formula             TEXT,
    data_source_id      VARCHAR(36),
    kpi_datastore_id    VARCHAR(36),
    unit                VARCHAR(50),
    thresholds          JSON,
    refresh_interval    INT DEFAULT 60,
    is_shared           BOOLEAN DEFAULT FALSE,
    user_id             VARCHAR(36) NOT NULL,
    
    -- Classification fields
    namespace           VARCHAR(100) NOT NULL,
    kind                VARCHAR(50) NOT NULL,
    layer               VARCHAR(50) NOT NULL,
    classifier          VARCHAR(100),
    signal_type         VARCHAR(50) NOT NULL,
    sentiment           VARCHAR(50),
    component_type      VARCHAR(100),
    
    -- Query configuration
    query               JSON,
    examples            TEXT,
    dimensions_hint     JSON,
    query_type          VARCHAR(50),
    datastore           VARCHAR(100),
    service_family      VARCHAR(100),
    
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    FOREIGN KEY (data_source_id) REFERENCES data_sources(id)
);
```

## Data Source Discovery

When MariaDB is enabled, mirador-core dynamically reads data source endpoints:

### Metrics Endpoints

```go
// GetMetricsEndpoints returns VictoriaMetrics/Prometheus URLs
endpoints, err := dataSourceRepo.GetMetricsEndpoints(ctx)
// Returns: ["http://vm1:8428", "http://vm2:8428"]
```

### Logs Endpoints

```go
// GetLogsEndpoints returns VictoriaLogs URLs
endpoints, err := dataSourceRepo.GetLogsEndpoints(ctx)
// Returns: ["http://vmlogs:9428"]
```

### Traces Endpoints

```go
// GetTracesEndpoints returns VictoriaTraces URLs
endpoints, err := dataSourceRepo.GetTracesEndpoints(ctx)
// Returns: ["http://vmtraces:9429"]
```

### Endpoint Priority

When both MariaDB and config.yaml define endpoints:

1. **MariaDB takes precedence** when enabled and connected
2. **config.yaml is fallback** when MariaDB is unavailable or disabled
3. **Dynamic refresh** occurs when MariaDB reconnects

## KPI Synchronization

### Background Sync Worker

When `sync.enabled` is true, a background worker syncs KPIs from MariaDB to Weaviate:

```
MariaDB (kpis table) ──sync──▶ Weaviate (KPIDefinition class)
```

**Sync behavior:**

1. **Incremental sync**: Only KPIs modified since last sync are processed
2. **Upsert logic**: Existing KPIs are updated, new KPIs are created
3. **Batch processing**: KPIs are processed in configurable batches
4. **Retry on failure**: Transient failures are retried with backoff

### Monitoring Sync Status

Check sync status via the health endpoint:

```bash
curl http://localhost:8010/api/v1/health/deep | jq '.components.kpi_sync'
```

Response:
```json
{
  "status": "healthy",
  "last_sync": "2026-03-02T15:30:00Z",
  "kpis_synced": 125,
  "next_sync": "2026-03-02T15:35:00Z"
}
```

### Manual Sync Trigger

Trigger an immediate sync via API:

```bash
curl -X POST http://localhost:8010/api/v1/admin/sync/kpis
```

## Health Checks

### MariaDB Health Status

The `/api/v1/health` endpoint includes MariaDB status:

```bash
curl http://localhost:8010/api/v1/health | jq '.components.mariadb'
```

Response when connected:
```json
{
  "enabled": true,
  "connected": true,
  "host": "mariadb.example.com",
  "database": "tenant_acme"
}
```

Response when disconnected:
```json
{
  "enabled": true,
  "connected": false,
  "host": "mariadb.example.com",
  "database": "tenant_acme",
  "error": "mariadb: connection refused"
}
```

### Deep Health Check

The `/api/v1/health/deep` endpoint performs an active ping:

```bash
curl http://localhost:8010/api/v1/health/deep
```

## Error Handling

### Graceful Degradation

mirador-core implements graceful degradation for MariaDB failures:

| Scenario | Behavior |
|----------|----------|
| MariaDB unavailable at startup | Log warning, use config.yaml endpoints |
| MariaDB becomes unavailable | Continue with cached endpoints, attempt reconnect |
| MariaDB query fails | Return 503, log error, don't crash |
| MariaDB disabled | Use config.yaml endpoints only |

### Error Responses

When MariaDB is unavailable, API calls that require it return 503:

```json
{
  "error": "config_database_unavailable",
  "code": "MARIADB_UNAVAILABLE",
  "message": "The configuration database is temporarily unavailable. Operation 'list_data_sources' cannot be completed. Please try again later."
}
```

When MariaDB is disabled but endpoint requires it:

```json
{
  "error": "config_database_disabled",
  "code": "MARIADB_DISABLED",
  "message": "The configuration database is not enabled for this deployment. Operation 'list_data_sources' is not available."
}
```

### Auto-Reconnect

mirador-core automatically attempts to reconnect to MariaDB:

1. **On first use**: If initial connection failed, retry on first query
2. **On ping failure**: If connection is lost, attempt reconnect
3. **Exponential backoff**: Retries use exponential backoff to avoid thundering herd

## Database User Setup

### Create Read-Only User

Create a dedicated read-only user for mirador-core:

```sql
-- Create the read-only user
CREATE USER 'mirador_core_ro'@'%' IDENTIFIED BY 'secure_password';

-- Grant SELECT on tenant database
GRANT SELECT ON tenant_acme.* TO 'mirador_core_ro'@'%';

-- (Optional) Grant SELECT on specific tables only
GRANT SELECT ON tenant_acme.data_sources TO 'mirador_core_ro'@'%';
GRANT SELECT ON tenant_acme.kpis TO 'mirador_core_ro'@'%';

-- Apply privileges
FLUSH PRIVILEGES;
```

### Connection Limits

Configure connection limits to prevent resource exhaustion:

```sql
-- Limit connections for the read-only user
ALTER USER 'mirador_core_ro'@'%' WITH MAX_USER_CONNECTIONS 20;
```

## Troubleshooting

### Connection Issues

**Problem**: MariaDB connection refused

```
mariadb: dial tcp 127.0.0.1:3306: connect: connection refused
```

**Solutions:**
1. Verify MariaDB is running: `systemctl status mariadb`
2. Check host/port configuration
3. Verify network connectivity: `nc -zv mariadb.example.com 3306`
4. Check firewall rules

**Problem**: Access denied

```
mariadb: Error 1045 (28000): Access denied for user 'mirador_core_ro'@'...'
```

**Solutions:**
1. Verify credentials
2. Check user grants: `SHOW GRANTS FOR 'mirador_core_ro'@'%';`
3. Ensure password is correctly set via environment variable

### Sync Issues

**Problem**: KPIs not syncing to Weaviate

**Diagnostic steps:**
1. Check sync status: `curl http://localhost:8010/api/v1/health/deep`
2. Check logs for sync errors: `grep "kpi_sync" /var/log/mirador-core.log`
3. Verify Weaviate connectivity
4. Check that `sync.enabled` is true

**Problem**: Stale KPI data

**Solutions:**
1. Trigger manual sync: `POST /api/v1/admin/sync/kpis`
2. Check `updated_at` column in MariaDB
3. Verify sync interval is appropriate

### Performance Issues

**Problem**: Slow queries

**Solutions:**
1. Add indexes on frequently queried columns:
   ```sql
   CREATE INDEX idx_kpis_namespace ON kpis(namespace);
   CREATE INDEX idx_kpis_updated_at ON kpis(updated_at);
   CREATE INDEX idx_data_sources_type ON data_sources(type, is_active);
   ```
2. Increase connection pool size
3. Enable query caching in MariaDB

## Migration from config.yaml

Migrating from static config.yaml endpoints to MariaDB is **automatic** when using the bootstrap feature.

### Automatic Migration (Recommended)

With bootstrap enabled (default), mirador-core automatically:

1. **Creates tables if missing**: On first startup, creates `data_sources` and `kpis` tables
2. **Syncs config.yaml data sources**: Checks each endpoint in config.yaml against MariaDB
3. **Creates missing entries**: If an endpoint URL doesn't exist in MariaDB, creates it
4. **Validates existing entries**: If an endpoint URL already exists, logs validation and moves on

Simply enable MariaDB with bootstrap in your config.yaml:

```yaml
# config.yaml
mariadb:
  enabled: true
  host: "mariadb.example.com"
  database: "tenant_acme"
  username: "mirador_core_rw"  # Requires write access for bootstrap
  password: "your_password"
  bootstrap:
    enabled: true
    create_tables_if_missing: true
    sync_datasources_from_config: true
```

On startup, mirador-core will:

```
INFO Starting MariaDB bootstrap...
INFO Created data_sources table
INFO Created kpis table
INFO Syncing data sources from config.yaml...
INFO Data source already exists in MariaDB: http://vm1:8428
INFO Created data source in MariaDB: http://vm2:8428 (victoria_metrics)
INFO Bootstrap completed successfully
```

### Manual Migration (Advanced)

For more control over the migration process, disable bootstrap and follow these steps:

#### Step 1: Create Data Sources in mirador-ui

Create the data sources in mirador-ui that match your config.yaml:

```yaml
# Before (config.yaml)
database:
  victoria_metrics:
    endpoints:
      - "http://vm1:8428"
      - "http://vm2:8428"
```

Create equivalent entries in mirador-ui's Data Sources UI.

#### Step 2: Enable MariaDB in mirador-core

```yaml
# config.yaml
mariadb:
  enabled: true
  host: "mariadb.example.com"
  database: "tenant_acme"
  username: "mirador_core_ro"
  bootstrap:
    enabled: false  # Disable automatic bootstrap
  # ... other settings
```

#### Step 3: Verify Migration

```bash
# Check discovered endpoints
curl http://localhost:8010/api/v1/health/deep | jq '.components.victoria_metrics'

# Verify KPI sync
curl http://localhost:8010/api/v1/health/deep | jq '.components.kpi_sync'
```

### Post-Migration: Read-Only Access

After bootstrap completes, you can switch to read-only credentials:

```yaml
mariadb:
  enabled: true
  username: "mirador_core_ro"  # Switch to read-only user
  bootstrap:
    enabled: false  # Disable bootstrap after initial sync
```

### Remove Static Endpoints (Optional)

Once verified, you can remove static endpoints from config.yaml. MariaDB will be the source of truth.

## Best Practices

### Security

1. **Use read-only credentials**: Never grant write access to mirador-core
2. **Use TLS connections**: Enable SSL/TLS for MariaDB connections in production
3. **Rotate credentials**: Regularly rotate the MariaDB password
4. **Network isolation**: Restrict MariaDB access to mirador-core's network segment

### Performance

1. **Connection pooling**: Configure appropriate pool sizes for your load
2. **Index optimization**: Ensure proper indexes on `data_sources` and `kpis` tables
3. **Sync interval tuning**: Balance freshness vs. database load

### Monitoring

1. **Monitor connection health**: Alert on MariaDB connection failures
2. **Track sync latency**: Monitor time between KPI updates and sync
3. **Watch for errors**: Set up alerts for repeated query failures

### High Availability

1. **Use MariaDB replication**: Connect to a read replica for HA
2. **Handle failover**: mirador-core auto-reconnects on failover
3. **Multiple mirador-core instances**: Each instance maintains its own connection pool
