# mirador-core Helm Chart

This chart deploys MIRADOR-CORE, the backend REST API service for the Mirador observability platform.

## Quickstart

```bash
helm upgrade --install mirador-core ./chart \
  -n mirador --create-namespace
```

Port-forward to test locally:

```bash
kubectl -n mirador port-forward svc/mirador-core 8010:8010
curl http://localhost:8010/health
```

## Common Values

- `replicaCount`: number of replicas (default 3)
- `image.repository`, `image.tag`, `image.pullPolicy`
- `service.type` (default ClusterIP), `service.ports.http.port`, `service.ports.grpc.port`
- `ingress.enabled`, `ingress.hosts[]`
- `resources.requests/limits`
- `env` and `envFrom` for environment variables and secrets
- `config.enabled`, `config.existingConfigMap`, `config.content` for `/etc/mirador/config.yaml`
- `alertRules.enabled`, `alertRules.existingConfigMap`, `alertRules.content` for `/etc/mirador/alert-rules.yaml`
- `rbac.create`, `rbac.clusterWide` and `serviceAccount.create`
- `autoscaling.enabled` for HPA, `podDisruptionBudget.*` for PDB
- `networkPolicy.enabled` to restrict traffic; customize ingress/egress
- `serviceMonitor.enabled` and `prometheusRule.enabled` for Prometheus Operator
- `topologySpreadConstraints`, `priorityClassName`, `revisionHistoryLimit`
- `search.default_engine`: default search engine (`lucene` or `bleve`, default `lucene`)
- `search.enable_bleve`: enable Bleve search engine (default `true`)
- `search.enable_lucene`: enable Lucene search engine (default `true`)

## Embedded Valkey (Subchart)

This chart can deploy Valkey (Bitnami) as a subchart and automatically wire the application to it.

- Enable subchart and use the in-cluster service:

```yaml
valkey:
  enabled: true
  architecture: standalone
  auth:
    enabled: false
```

The application’s `cache.nodes` will render to `"<release>-valkey-headless:6379"` when `valkey.enabled=true` (you can override service names using `valkey.serviceName`/`valkey.headlessServiceName`). In addition, the Deployment exports `VALKEY_CACHE_NODES=<release>-valkey-headless:6379` to force single-node mode in the app when using the embedded Valkey.

### Valkey Image Overrides

You can pin or override the Bitnami Valkey image used by the subchart via `values.yaml`:

```yaml
valkey:
  enabled: true
  # Image overrides (optional; subchart defaults are used if omitted)
  image:
    registry: docker.io
    repository: bitnami/valkey
    tag: "8.0.2"
    # Recommended: pin a digest to satisfy Bitnami security checks
    # and for supply-chain integrity
    digest: "sha256:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    pullPolicy: IfNotPresent
    # pullSecrets:
    #   - myRegistrySecret
```

Or via Helm CLI flags:

```bash
helm upgrade --install mirador-core ./chart \
  --set valkey.enabled=true \
  --set valkey.image.registry=docker.io \
  --set valkey.image.repository=bitnami/valkey \
  --set valkey.image.tag=8.0.2 \
  --set valkey.image.pullPolicy=IfNotPresent
```

If you do not provide a digest and override the repository/tag, the Bitnami
common library may require you to explicitly allow insecure images. You can
either pin a digest (recommended) or set a global flag:

```bash
# Less secure: allow non-digested/custom images across subcharts
helm upgrade --install mirador-core ./chart \
  --set global.security.allowInsecureImages=true
```

You can also set it in `values.yaml`:

```yaml
global:
  security:
    allowInsecureImages: true
```

To scope this to the Valkey subchart only (without impacting other Bitnami
dependencies), use the subchart-scoped global block:

```yaml
valkey:
  global:
    security:
      allowInsecureImages: false # or true
```

CLI equivalent:

```bash
helm upgrade --install mirador-core ./chart \
  --set valkey.global.security.allowInsecureImages=false
```

### Bumping Valkey Version

The subchart version is declared in `values.yaml` at `valkey.version`. The Makefile target `helm-sync-deps` syncs this into `Chart.yaml` before running `helm dependency update`.

- To update:
  1. Edit `chart/values.yaml` and set `valkey.version` (e.g., `"^2.1.0"`).
  2. Run `make helm-dep-update` to sync and fetch the dependency.
  3. Commit changes (including `Chart.yaml` update and `charts/` lockfiles if present).

Refer to upstream chart notes: https://github.com/bitnami/charts/tree/main/bitnami/valkey

## Weaviate Subchart

This chart can deploy a Weaviate instance to back schema definitions and AI/RAG features.
Enable via values:

```yaml
weaviate:
  enabled: true
  persistence:
    enabled: true
    size: 20Gi
```

Notes:
- Backups run per shard as individual CronJobs.
- For production, consider encryption, IAM roles/IRSA, and strong retention policies.

### Wait for Valkey (Init Container)

By default this chart does not add an extra image just to wait for Valkey.
`waitFor.valkey.enabled` is `false` to avoid additional dependencies because
mirador-core already tolerates late Valkey by using an in-memory cache and
auto-reconnecting when Valkey becomes available.

If you still want a hard wait, enable the optional init container that pings
Valkey using `redis-cli` from the Bitnami Redis image:

```yaml
waitFor:
  valkey:
    enabled: true
    image:
      repository: bitnami/redis
      tag: "7.2.5-debian-12-r0"
      pullPolicy: IfNotPresent
    timeoutSeconds: 120
    intervalSeconds: 5
```

This init container is only added when `valkey.enabled=true`.

### Production Hints
- Use `valkey.architecture: replication` with persistence enabled:

```yaml
valkey:
  enabled: true
  version: "^2.1.0"
  architecture: replication
  auth:
    enabled: true
    existingSecret: my-valkey-auth
  primary:
    persistence:
      enabled: true
  replica:
    replicaCount: 2
    persistence:
      enabled: true

# Application secrets (env vars consumed by mirador-core)
secrets:
  create: false
  name: mirador-env

envFrom:
  - secretRef:
      name: mirador-env
```

If you rely on the Valkey subchart’s auto-generated Secret instead of your own, set:

```yaml
valkey:
  enabled: true
  auth:
    enabled: true
    # secretName defaults to <release>-valkey, passwordKey defaults to valkey-password

# The chart will set REDIS_PASSWORD from that Secret automatically.
```

To manage app credentials in a single Secret created by this chart, set `secrets.create: true` and fill `secrets.data.JWT_SECRET`, `LDAP_PASSWORD`, `SMTP_PASSWORD`, `REDIS_PASSWORD`, `VM_PASSWORD`.
 
## Production Setup Checklist
 
- High availability
  - Set `replicaCount >= 3`, enable `autoscaling.enabled` with sensible bounds.
  - Enable `podDisruptionBudget` to preserve availability during voluntary disruptions.
  - Use `topologySpreadConstraints` or anti-affinity to spread across zones/nodes.
- Networking
  - Enable `networkPolicy.enabled` and tailor allowed ingress/egress per environment.
  - Terminate TLS at your ingress; set `ingress.tls` appropriately. For gRPC, ensure HTTP/2 is enabled on your ingress controller and add any required annotations.
- Observability
  - Leave `podAnnotations` Prometheus scrape hints, or enable `serviceMonitor.enabled` for Prometheus Operator.
  - Optionally define `prometheusRule.groups` with SLO/SLA alerts for your environment.
- Security
  - Run as non-root (defaults provided in `podSecurityContext`).
  - Set a strict `securityContext` (drop capabilities, read-only root FS) if compatible with your runtime.
  - Manage application secrets externally and wire via `envFrom`, or set `secrets.create=true` with pre-provisioned values.
- Config and rollouts
  - ConfigMap and alert-rules changes trigger rollouts via checksum annotations.
  - Tune `terminationGracePeriodSeconds`, `revisionHistoryLimit` to your SLOs.
- Dependencies
  - For production Valkey, prefer `architecture: replication` with persistence enabled and strong auth.
  - Consider using a managed Redis/Valkey and set `mirador.cache.nodes` to external endpoints.

### Kubernetes Service Discovery (vmselect/vlselect/vtselect)

This chart can template discovery settings directly into `config.yaml` via the `tpl` function.
Enable per-backend discovery (uses cluster DNS A/SRV records) so mirador-core auto-updates its endpoint lists on scale out/in:

```yaml
discovery:
  vm:
    enabled: true
    service: vm-select.vm-select.svc.cluster.local
    port: 8481
    scheme: http
    refreshSeconds: 30
    useSRV: false
  vl:
    enabled: true
    service: vl-select.vl-select.svc.cluster.local
    port: 9428
    scheme: http
    refreshSeconds: 30
    useSRV: false
  vt:
    enabled: true
    service: vt-select.vt-select.svc.cluster.local
    port: 10428
    scheme: http
    refreshSeconds: 30
    useSRV: false
```

Notes:
- Prefer headless Services for vmselect/vlselect/vtselect to expose per-pod A records.
- Alternatively set `useSRV: true` and publish SRV records for the Service.
- Static `database.*.endpoints` remain valid and are used as seed; discovery replaces the list dynamically at runtime.

### Multi-Source Aggregation

You can configure multiple backend clusters per data type and Mirador will fan-out queries and aggregate results.

Values keys (rendered into `/etc/mirador/config.yaml`):

```yaml
mirador:
  database:
    victoria_metrics:
      name: primary
      endpoints: ["http://vm-a:8481"]
      timeout: 30000
    metrics_sources:
      - name: fin_metrics
        endpoints: ["http://vm-fin-0:8481"]
      - name: os_metrics
        discovery:
          enabled: true
          service: vm-select.vm-os.svc.cluster.local
          port: 8481
          scheme: http
          refreshSeconds: 30
          useSRV: false

    victoria_logs:
      name: primary
      endpoints: ["http://vl-a:9428"]
      timeout: 30000
    logs_sources:
      - name: fin_logs
        endpoints: ["http://vl-fin-0:9428"]

    victoria_traces:
      name: primary
      endpoints: ["http://vt-a:10428"]
      timeout: 30000
    traces_sources:
      - name: os_traces
        discovery:
          enabled: true
          service: vt-select.vt-os.svc.cluster.local
          port: 10428
          scheme: http
          refreshSeconds: 30
          useSRV: false
```

Behavior summary:
- Metrics: concatenates series across sources; datapoint counts are summed; duplicates may appear.
- Logs: concatenates rows; unions field names; aggregates stats.
- Traces: unions services; concatenates searches; first-found trace is returned.

## Enabling HPA, PDB, NetworkPolicy, and Monitoring

Example values override for production:

```yaml
replicaCount: 3
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 2000m
    memory: 2Gi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

podDisruptionBudget:
  enabled: true
  minAvailable: 1

networkPolicy:
  enabled: true
  ingress:
    from:
      - namespaceSelector: {}
        podSelector: {}
  egress:
    to:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: kube-system
        podSelector:
          matchLabels:
            k8s-app: kube-dns
    ports:
      - protocol: UDP
        port: 53
      - protocol: TCP
        port: 53

serviceMonitor:
  enabled: true
  interval: 30s
  scrapeTimeout: 10s
```

## Configuration

By default the chart creates a ConfigMap with a production-ready example config.
You can override with your own file using `config.existingConfigMap` and set `env.CONFIG_PATH=/etc/mirador/config.yaml`.

Secrets such as JWT, LDAP, Redis, or SMTP are expected via environment variables or mounted secrets; wire them via `envFrom` or `env.extra`.

## Notes

The application exposes:

- REST: `/health`, `/ready`, `/metrics`, `/api/openapi.yaml`, `/swagger`
- gRPC client connections to AI engines are configured through the `grpc.*` section of the config.

## Multi-Arch Images

The `platformbuilds/mirador-core` image is intended to be published as a multi-arch manifest (linux/amd64 and linux/arm64). When the image tag is multi-arch:

- `docker run` and Kubernetes automatically pull the correct architecture for the host/node.
- No `nodeSelector` or platform pin is necessary for architecture selection.

Build and push a multi-arch image from this repo using Docker Buildx:

```bash
# Set registry/org and version as needed
make dockerx-push REGISTRY=platformbuilds IMAGE_NAME=mirador-core VERSION=v2.1.3

# Verify manifest includes both amd64 and arm64
docker buildx imagetools inspect platformbuilds/mirador-core:v2.1.3
```

Then set the chart values to reference that tag:

```yaml
image:
  repository: platformbuilds/mirador-core
  tag: v2.1.3 # multi-arch manifest
  pullPolicy: IfNotPresent
```

## RBAC Bootstrap and Authentication (v8.0.0+)

### RBAC Features

Mirador Core v8.0.0 introduces comprehensive Role-Based Access Control (RBAC) with automatic bootstrap of default admin users, tenants, and roles.

#### Default Configuration

By default, RBAC bootstrap is **enabled** and will create:
- A default admin user (`admin` / `ChangeMe123!`)
- A default tenant (`default`)
- Default roles: `global_admin`, `tenant_admin`, `viewer`, `analyst`

**⚠️ SECURITY WARNING:** Change the default admin password immediately after first deployment!

#### Quick Start with RBAC

```bash
# Deploy with default RBAC configuration
helm upgrade --install mirador-core ./chart \
  -n mirador --create-namespace \
  --set mirador.weaviate.enabled=true \
  --set weaviate.enabled=true

# Access the system with default credentials (change immediately!)
curl -X POST http://mirador-core.mirador:8010/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"ChangeMe123!"}'
```

#### Customize Admin Credentials

Override via Helm values:

```yaml
secrets:
  create: true
  data:
    ADMIN_PASSWORD: "YourSecurePassword123!"

mirador:
  auth:
    rbac:
      bootstrap:
        admin_user:
          email: "admin@yourcompany.com"
```

#### Authentication Backends

##### LDAP/AD Integration

```yaml
mirador:
  auth:
    ldap:
      enabled: true
      url: "ldap://ldap.company.com:389"
      base_dn: "dc=company,dc=com"
```

##### OAuth 2.0 / OIDC

```yaml
mirador:
  auth:
    oauth:
      enabled: true
      provider: "okta"
      client_id: "your-client-id"
```

#### Weaviate Requirement

RBAC features **require** Weaviate:

```yaml
weaviate:
  enabled: true
  persistence:
    enabled: true

mirador:
  weaviate:
    enabled: true
    rbac_schema:
      enabled: true
```

See full RBAC documentation in the chart values.yaml file.
