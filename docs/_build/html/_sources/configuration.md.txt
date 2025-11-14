# Configuration Guide

This guide covers all configuration options available in MIRADOR-CORE.

## Configuration Sources

MIRADOR-CORE supports multiple configuration sources with the following priority (highest to lowest):

1. Environment variables
2. Configuration file (`config.yaml`)
3. Default values

## Configuration File

The main configuration file is `config.yaml`. Sample configurations are provided in the `configs/` directory:

- `config.development.yaml` - Development environment
- `config.production.yaml` - Production environment
- `config.yaml` - Default configuration

## Core Configuration

### Server Configuration

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  readTimeout: "30s"
  writeTimeout: "30s"
  idleTimeout: "120s"
  maxHeaderBytes: 1048576
  tls:
    enabled: false
    certFile: "/path/to/cert.pem"
    keyFile: "/path/to/key.pem"
```

**Environment Variables:**
- `SERVER_HOST`
- `SERVER_PORT`
- `SERVER_READ_TIMEOUT`
- `SERVER_WRITE_TIMEOUT`
- `SERVER_IDLE_TIMEOUT`
- `SERVER_MAX_HEADER_BYTES`
- `SERVER_TLS_ENABLED`
- `SERVER_TLS_CERT_FILE`
- `SERVER_TLS_KEY_FILE`

### Database Configuration

```yaml
database:
  host: "localhost"
  port: 5432
  database: "mirador_core"
  username: "mirador_user"
  password: "secure_password"
  sslMode: "require"
  maxConnections: 20
  maxIdleConnections: 5
  connectionMaxLifetime: "1h"
  connectionMaxIdleTime: "30m"
```

**Environment Variables:**
- `DATABASE_HOST`
- `DATABASE_PORT`
- `DATABASE_NAME`
- `DATABASE_USER`
- `DATABASE_PASSWORD`
- `DATABASE_SSL_MODE`
- `DATABASE_MAX_CONNECTIONS`
- `DATABASE_MAX_IDLE_CONNECTIONS`
- `DATABASE_CONNECTION_MAX_LIFETIME`
- `DATABASE_CONNECTION_MAX_IDLE_TIME`

### Redis Configuration

```yaml
redis:
  host: "localhost"
  port: 6379
  password: ""
  database: 0
  poolSize: 10
  minIdleConns: 2
  connMaxLifetime: "1h"
  connMaxIdleTime: "30m"
  tls: false
```

**Environment Variables:**
- `REDIS_HOST`
- `REDIS_PORT`
- `REDIS_PASSWORD`
- `REDIS_DATABASE`
- `REDIS_POOL_SIZE`
- `REDIS_MIN_IDLE_CONNS`
- `REDIS_CONN_MAX_LIFETIME`
- `REDIS_CONN_MAX_IDLE_TIME`
- `REDIS_TLS`

## Authentication Configuration

### LDAP/AD Configuration

```yaml
auth:
  provider: "ldap"
  ldap:
    url: "ldaps://ldap.company.com:636"
    baseDN: "dc=company,dc=com"
    bindDN: "cn=serviceaccount,ou=serviceaccounts,dc=company,dc=com"
    bindPassword: "service_password"
    userSearchFilter: "(sAMAccountName=%s)"
    userSearchBase: "ou=users,dc=company,dc=com"
    groupSearchFilter: "(member=%s)"
    groupSearchBase: "ou=groups,dc=company,dc=com"
    attributes:
      username: "sAMAccountName"
      email: "mail"
      displayName: "displayName"
      memberOf: "memberOf"
    start_tls: false
    tls_skip_verify: false
    tls_ca_bundle_path: "/etc/mirador/ldap/ca-bundle.pem"
```

`tlsCaBundlePath` accepts a PEM-encoded bundle that Mirador watches for changes. In Kubernetes mount the bundle via a ConfigMap (projected as a file) and updates will be applied automatically without restarting the service. In non-Kubernetes deployments update the file in place; the watcher validates the new bundle and swaps it in once parsing succeeds. If parsing fails the previous trust store remains active and the error is logged.

### OAuth 2.0 Configuration

```yaml
auth:
  provider: "oauth2"
  oauth2:
    provider: "google"  # google, github, okta, azuread, custom
    clientID: "your-client-id"
    clientSecret: "your-client-secret"
    redirectURL: "https://mirador-core.company.com/auth/callback"
    scopes: ["openid", "profile", "email"]
    endpoints:
      authURL: "https://accounts.google.com/o/oauth2/auth"
      tokenURL: "https://oauth2.googleapis.com/token"
      userInfoURL: "https://www.googleapis.com/oauth2/v2/userinfo"
```

### Local Authentication (Development)

```yaml
auth:
  provider: "local"
  local:
    users:
      - username: "admin"
        password: "admin123"
        email: "admin@company.com"
        roles: ["admin"]
      - username: "user"
        password: "user123"
        email: "user@company.com"
        roles: ["user"]
```

## Data Source Configuration

### VictoriaMetrics Configuration

```yaml
victoriametrics:
  metrics:
    url: "http://victoriametrics:8428"
    timeout: "30s"
    retries: 3
    rateLimit:
      requestsPerSecond: 100
      burst: 200

  logs:
    url: "http://victorialogs:9428"
    timeout: "30s"
    retries: 3
    searchEngine: "lucene"  # lucene or bleve

  traces:
    url: "http://victoriatraces:9429"
    timeout: "30s"
    retries: 3
```

### Additional Data Sources

```yaml
datasources:
  prometheus:
    - name: "prometheus-main"
      url: "http://prometheus:9090"
      timeout: "30s"
      enabled: true

  elasticsearch:
    - name: "elasticsearch-logs"
      urls: ["http://elasticsearch:9200"]
      username: "elastic"
      password: "elastic123"
      timeout: "30s"
      enabled: true

  jaeger:
    - name: "jaeger-traces"
      url: "http://jaeger:16686"
      timeout: "30s"
      enabled: true
```

## Feature Flags

```yaml
features:
  unifiedQuery: true
  rca: true
  predictiveAnalysis: true
  schemaManagement: true
  rbac: true
  auditLogging: true
  metricsCaching: true
  queryOptimization: true
  userSettings: true
  notifications: true
  dashboards: true
```

**Environment Variables:**
- `FEATURE_UNIFIED_QUERY`
- `FEATURE_RCA`
- `FEATURE_PREDICTIVE_ANALYSIS`
- `FEATURE_SCHEMA_MANAGEMENT`
- `FEATURE_RBAC`
- `FEATURE_AUDIT_LOGGING`
- `FEATURE_METRICS_CACHING`
- `FEATURE_QUERY_OPTIMIZATION`
- `FEATURE_USER_SETTINGS`
- `FEATURE_NOTIFICATIONS`
- `FEATURE_DASHBOARDS`

## Metrics and Monitoring

### Application Metrics

```yaml
metrics:
  enabled: true
  path: "/metrics"
  namespace: "mirador_core"
  subsystem: "api"
  buckets: [0.1, 0.5, 1, 2.5, 5, 10]
```

### Health Checks

```yaml
health:
  enabled: true
  path: "/health"
  deepCheckPath: "/health/deep"
  readinessPath: "/ready"
  livenessPath: "/live"
  checks:
    database: true
    redis: true
    datasources: true
    dependencies: true
```

### Logging Configuration

```yaml
logging:
  level: "info"  # debug, info, warn, error
  format: "json"  # json or text
  output: "stdout"  # stdout, stderr, or file path
  file:
    path: "/var/log/mirador-core.log"
    maxSize: "100MB"
    maxAge: "30d"
    maxBackups: 10
    compress: true
```

**Environment Variables:**
- `LOG_LEVEL`
- `LOG_FORMAT`
- `LOG_OUTPUT`
- `LOG_FILE_PATH`
- `LOG_FILE_MAX_SIZE`
- `LOG_FILE_MAX_AGE`
- `LOG_FILE_MAX_BACKUPS`
- `LOG_FILE_COMPRESS`

## Caching Configuration

```yaml
cache:
  enabled: true
  ttl: "5m"
  maxSize: "1GB"
  redis:
    prefix: "mirador:cache:"
  memory:
    enabled: true
    size: "512MB"
```

## Rate Limiting

```yaml
rateLimit:
  enabled: true
  requestsPerMinute: 1000
  burst: 2000
  cleanupInterval: "1m"
  storage: "redis"  # memory or redis
```

## KPI Management

```yaml
schema:
  enabled: true
  validation: true
  autoDiscovery: true
  retention:
    metrics: "90d"
    logs: "30d"
    traces: "7d"
  indexing:
    batchSize: 1000
    workers: 4
    queueSize: 10000
```

## AI and Analytics

### Root Cause Analysis

```yaml
rca:
  enabled: true
  models:
    - name: "isolation_forest"
      enabled: true
      contamination: 0.1
    - name: "correlation_analysis"
      enabled: true
      threshold: 0.8
  cache:
    enabled: true
    ttl: "1h"
```

### Predictive Analysis

```yaml
predictive:
  enabled: true
  models:
    - name: "lstm_trend"
      enabled: true
      lookback: "24h"
      forecast: "1h"
    - name: "anomaly_detection"
      enabled: true
      sensitivity: 0.95
  cache:
    enabled: true
    ttl: "30m"
```

## Security Configuration

### CORS Configuration

```yaml
cors:
  enabled: true
  allowedOrigins: ["https://mirador-core.company.com"]
  allowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowedHeaders: ["*"]
  allowCredentials: true
  maxAge: "12h"
```

### Security Headers

```yaml
security:
  headers:
    hsts:
      enabled: true
      maxAge: "31536000"
      includeSubdomains: true
      preload: false
    csp:
      enabled: true
      policy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'"
    xFrameOptions: "DENY"
    xContentTypeOptions: "nosniff"
    referrerPolicy: "strict-origin-when-cross-origin"
```

## Performance Tuning

### Query Optimization

```yaml
query:
  timeout: "30s"
  maxConcurrent: 100
  maxResults: 10000
  optimization:
    enabled: true
    parallelExecution: true
    queryRewrite: true
    resultCaching: true
```

### Resource Limits

```yaml
resources:
  maxMemory: "4GB"
  maxCPU: 4
  goroutines:
    max: 10000
    warningThreshold: 8000
```

## Integration Configuration

### Webhook Configuration

```yaml
webhooks:
  enabled: true
  endpoints:
    - url: "https://slack-webhook.company.com"
      events: ["alert", "incident"]
      secret: "webhook-secret"
    - url: "https://pagerduty.company.com"
      events: ["critical_alert"]
      secret: "pagerduty-secret"
```

### Notification Configuration

```yaml
notifications:
  enabled: true
  providers:
    email:
      enabled: true
      smtp:
        host: "smtp.company.com"
        port: 587
        username: "noreply@company.com"
        password: "smtp-password"
        tls: true
    slack:
      enabled: true
      webhookURL: "https://hooks.slack.com/services/..."
      channel: "#alerts"
      username: "MIRADOR-CORE"
```

## Development Configuration

### Debug Configuration

```yaml
debug:
  enabled: false
  pprof:
    enabled: true
    path: "/debug/pprof"
  trace:
    enabled: false
    samplingRate: 0.01
  metrics:
    detailed: false
```

### Development Overrides

```yaml
development:
  hotReload: true
  verboseLogging: true
  mockData: false
  skipAuth: false
  allowInsecureTLS: false
```

## Environment-Specific Examples

### Development Environment

```yaml
# config.development.yaml
logging:
  level: "debug"
  format: "text"

features:
  rca: false
  predictiveAnalysis: false

auth:
  provider: "local"

debug:
  enabled: true
```

### Production Environment

```yaml
# config.production.yaml
logging:
  level: "info"
  format: "json"
  file:
    path: "/var/log/mirador-core.log"

server:
  tls:
    enabled: true
    certFile: "/etc/ssl/certs/mirador-core.crt"
    keyFile: "/etc/ssl/private/mirador-core.key"

rateLimit:
  enabled: true
  requestsPerMinute: 5000
  burst: 10000

cache:
  enabled: true
  ttl: "10m"
  maxSize: "2GB"
```

## Configuration Validation

MIRADOR-CORE validates configuration on startup. Invalid configurations will prevent the service from starting with detailed error messages.

```bash
# Validate configuration file
./mirador-core validate-config --config config.yaml

# Check configuration with environment variables
DATABASE_HOST=prod-db ./mirador-core validate-config
```

## Hot Reloading

Some configuration changes support hot reloading without service restart:

- Feature flags
- Logging levels
- Cache settings
- Rate limiting rules

```bash
# Reload configuration
curl -X POST http://localhost:8080/admin/reload-config \
  -H "Authorization: Bearer <admin-token>"
```

## Secrets Management

Sensitive configuration should be managed through environment variables or external secret stores:

```bash
# Using environment variables
export DATABASE_PASSWORD="secure-password"
export REDIS_PASSWORD="redis-password"
export LDAP_BIND_PASSWORD="ldap-password"

# Using Docker secrets
docker run -e DATABASE_PASSWORD_FILE=/run/secrets/db-password mirador-core

# Using Kubernetes secrets
kubectl create secret generic mirador-secrets \
  --from-literal=database-password=secure-password \
  --from-literal=redis-password=redis-password
```

## Monitoring Configuration Changes

Configuration changes are logged and can be monitored:

```bash
# View configuration change history
curl http://localhost:8080/admin/config/history \
  -H "Authorization: Bearer <admin-token>"

# Get current configuration (redacted)
curl http://localhost:8080/admin/config/current \
  -H "Authorization: Bearer <admin-token>"
```