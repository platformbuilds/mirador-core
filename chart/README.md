# mirador-core Helm Chart

This chart deploys MIRADOR-CORE, the backend REST API service for the Mirador observability platform.

## Quickstart

```bash
helm upgrade --install mirador-core ./chart \
  -n mirador --create-namespace
```

Port-forward to test locally:

```bash
kubectl -n mirador port-forward svc/mirador-core 8080:8080
curl http://localhost:8080/health
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

The application’s `cache.nodes` will render to `"<release>-valkey:6379"` when `valkey.enabled=true` (you can override service names using `valkey.serviceName`/`valkey.headlessServiceName`).

### Bumping Valkey Version

The subchart version is declared in `values.yaml` at `valkey.version`. The Makefile target `helm-sync-deps` syncs this into `Chart.yaml` before running `helm dependency update`.

- To update:
  1. Edit `chart/values.yaml` and set `valkey.version` (e.g., `"^2.1.0"`).
  2. Run `make helm-dep-update` to sync and fetch the dependency.
  3. Commit changes (including `Chart.yaml` update and `charts/` lockfiles if present).

Refer to upstream chart notes: https://github.com/bitnami/charts/tree/main/bitnami/valkey

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

## Configuration

By default the chart creates a ConfigMap with a production-ready example config.
You can override with your own file using `config.existingConfigMap` and set `env.CONFIG_PATH=/etc/mirador/config.yaml`.

Secrets such as JWT, LDAP, Redis, or SMTP are expected via environment variables or mounted secrets; wire them via `envFrom` or `env.extra`.

## Notes

The application exposes:

- REST: `/health`, `/ready`, `/metrics`, `/api/openapi.yaml`, `/docs`
- gRPC client connections to AI engines are configured through the `grpc.*` section of the config.
