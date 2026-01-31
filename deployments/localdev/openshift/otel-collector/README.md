# OpenTelemetry Collector DaemonSet for OpenShift

This directory contains the Kubernetes/OpenShift manifests to deploy the OpenTelemetry Collector as a DaemonSet on an OpenShift cluster.

## Overview

The collector is deployed as a DaemonSet to ensure one collector instance runs on each node. This setup:

- Collects OTLP traces, metrics, and logs from applications on each node
- Gathers host metrics and Kubernetes metadata
- Enriches telemetry with Kubernetes attributes
- Forwards data to VictoriaMetrics, VictoriaLogs, and VictoriaTraces

## Prerequisites

- OpenShift 4.10+ cluster
- `oc` CLI configured with cluster-admin privileges
- VictoriaMetrics stack deployed (or update endpoints accordingly)

## Files

| File | Description |
|------|-------------|
| `namespace.yaml` | Creates the `mirador-observability` namespace |
| `serviceaccount.yaml` | ServiceAccount for the collector pods |
| `clusterrole.yaml` | RBAC permissions for Kubernetes API access |
| `clusterrolebinding.yaml` | Binds ClusterRole to ServiceAccount |
| `scc.yaml` | OpenShift Security Context Constraints |
| `configmap.yaml` | Collector configuration |
| `endpoints-configmap.yaml` | Backend endpoint URLs (customize this) |
| `daemonset.yaml` | DaemonSet deployment spec |
| `service.yaml` | Services for OTLP ingestion |
| `servicemonitor.yaml` | Optional: Prometheus ServiceMonitor |
| `kustomization.yaml` | Kustomize configuration |

## Quick Start

### Using Kustomize (Recommended)

```bash
# Preview the manifests
oc kustomize .

# Apply to cluster
oc apply -k .
```

### Manual Deployment

```bash
# Create namespace first
oc apply -f namespace.yaml

# Apply Security Context Constraints (requires cluster-admin)
oc apply -f scc.yaml

# Apply remaining resources
oc apply -f serviceaccount.yaml
oc apply -f clusterrole.yaml
oc apply -f clusterrolebinding.yaml
oc apply -f endpoints-configmap.yaml
oc apply -f configmap.yaml
oc apply -f daemonset.yaml
oc apply -f service.yaml
```

## Configuration

### Backend Endpoints

Edit `endpoints-configmap.yaml` to point to your VictoriaMetrics stack:

```yaml
data:
  victoriametrics: "http://victoriametrics.your-namespace.svc.cluster.local:8428"
  victorialogs: "http://victorialogs.your-namespace.svc.cluster.local:9428"
  victoriatraces: "http://victoriatraces.your-namespace.svc.cluster.local:10428"
```

### Collector Configuration

The collector configuration in `configmap.yaml` includes:

- **Receivers**:
  - OTLP (gRPC :4317, HTTP :4318)
  - kubeletstats (node/pod/container metrics)
  - hostmetrics (CPU, memory, disk, network)

- **Processors**:
  - `memory_limiter`: Prevents OOM
  - `k8sattributes`: Adds Kubernetes metadata
  - `resource`: Adds custom attributes
  - `batch`: Batches for efficiency
  - `isolationforest`: Anomaly detection

- **Exporters**:
  - `prometheusremotewrite`: Metrics → VictoriaMetrics
  - `otlphttp/logs`: Logs → VictoriaLogs
  - `otlphttp/traces`: Traces → VictoriaTraces

### Resource Limits

Default resource requests/limits in `daemonset.yaml`:

```yaml
resources:
  requests:
    cpu: 200m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 2Gi
```

Adjust based on your workload volume.

## Sending Telemetry

Applications can send telemetry to the collector using the node's host network:

```yaml
# Environment variables for your application pods
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://$(HOST_IP):4317"
  - name: HOST_IP
    valueFrom:
      fieldRef:
        fieldPath: status.hostIP
```

Or use the cluster service:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://otel-collector.mirador-observability.svc.cluster.local:4317"
```

## Monitoring the Collector

### Check DaemonSet Status

```bash
oc get daemonset otel-collector -n mirador-observability
oc get pods -n mirador-observability -l app.kubernetes.io/name=otel-collector
```

### View Logs

```bash
oc logs -n mirador-observability -l app.kubernetes.io/name=otel-collector -f
```

### Collector Metrics

The collector exposes metrics on port 8888. If you have Prometheus/OpenShift Monitoring:

```bash
oc apply -f servicemonitor.yaml
```

## Troubleshooting

### Pods Not Starting

1. Check SCC assignment:
   ```bash
   oc get scc otel-collector-scc -o yaml
   oc adm policy who-can use scc otel-collector-scc
   ```

2. Check events:
   ```bash
   oc get events -n mirador-observability --sort-by='.lastTimestamp'
   ```

### No Data Reaching Backends

1. Verify endpoint connectivity:
   ```bash
   oc exec -n mirador-observability -it $(oc get pod -n mirador-observability -l app.kubernetes.io/name=otel-collector -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://victoriametrics:8428/-/healthy
   ```

2. Check collector logs for errors:
   ```bash
   oc logs -n mirador-observability -l app.kubernetes.io/name=otel-collector | grep -i error
   ```

### Permission Denied Errors

Ensure the SCC is properly bound:
```bash
oc adm policy add-scc-to-user otel-collector-scc -z otel-collector -n mirador-observability
```

## Uninstall

```bash
# Using Kustomize
oc delete -k .

# Or manually
oc delete -f .
oc delete namespace mirador-observability
```

## Version Updates

To update the collector version, modify `kustomization.yaml`:

```yaml
images:
  - name: otel/opentelemetry-collector-contrib
    newTag: "0.140.0"  # New version
```

Then reapply:
```bash
oc apply -k .
```
