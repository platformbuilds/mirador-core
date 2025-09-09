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

## Vitess Subchart (No Operator)

This chart can deploy a minimal but production‑leaning Vitess stack without an operator
for dev/small environments. It includes:

- etcd (Bitnami subchart) — 3 nodes by default
- vtctld — Vitess control plane HTTP/gRPC
- vtgate — MySQL protocol router (replicas)
- vttablet — StatefulSets per shard (PRIMARY + REPLICA tablets)

Enable Vitess (external) or the embedded Vitess subchart and auto‑wire the app to vtgate:

```yaml
vitess-external:
  enabled: true
  host: vtgate.mirador.svc.cluster.local
  port: 15306
  keyspace: mirador
  shard: "0"
  user: ""
  password: ""

vitess-embedded:
  enabled: true
```

Vitess embedded values (mirrored into vitess-minimal subchart):

- `topology.keyspace`: keyspace name (default `mirador`)
- `topology.shards`: list of shard names (e.g., `["0"]` or `["-80","80-"]`)
- `vtgate.replicas`: vtgate deployment replicas (default 2)
- `vttablet.replicasPerShard`: tablets per shard (default 2 → PRIMARY+1 REPLICA)
- `persistence.size`: PVC size for tablets (default 5Gi)
- `persistence.storageClass`: optional StorageClass for vttablet PVCs (default unset)
- `etcd.enabled`: enable Bitnami etcd (default true), `etcd.replicaCount: 3`
  - `etcd.persistence.storageClass`: optional StorageClass for etcd PVCs (default unset)

The application env `VITESS_HOST` resolves to `<release>-vitess-minimal-vtgate`
when `vitess.subchart.enabled=true` and `vitess.host` is empty.

#### Setting StorageClass from parent chart

You can configure the StorageClass for the Vitess subchart directly in this
parent chart's `values.yaml` under the `vitess-minimal` key:

```yaml
vitess-minimal:
  persistence:
    storageClass: fast-ssd   # for vttablet PVCs
  etcd:
    persistence:
      storageClass: standard # for etcd PVCs
```

### Vitess Credentials

Set credentials via a Secret (recommended):

```yaml
vitess:
  enabled: true
  existingSecret: my-vitess-secret
  passwordKey: password
  user: appuser
```

If `vitess.password` is set in values (dev only), the chart auto‑creates
`<release>-mirador-core-vitess` with the password and wires it to the pod env.

### Backups

The subchart provides a simple backup CronJob that calls `vtctldclient Backup` per shard.
You must configure vttablet backup flags (e.g., S3) via `vttablet.mysql.extraArgs`.

Enable backups and set schedule:

```yaml
vitess:
  subchart:
    enabled: true

vitess-minimal:
  backup:
    enabled: true
    schedule: "0 2 * * *"
    vtctldClientImage: vitess/vtctldclient:v18.0.2
    extraArgs: []

  # Pass backup storage flags to vttablet (example for S3 w/ xtrabackup)
  vttablet:
    mysql:
      extraArgs:
        - "--backup_storage_implementation=s3"
        - "--s3_backup_aws_region=us-east-1"
        - "--s3_backup_bucket=my-vitess-backups"
        # If credentials are needed, mount as env/secret and pass driver flags accordingly
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
