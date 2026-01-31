# OpenTelemetry eBPF Instrumentation (OBI) DaemonSet for OpenShift

This directory contains the Kubernetes/OpenShift manifests to deploy the OpenTelemetry eBPF Instrumentation agent as a DaemonSet on an OpenShift cluster.

## Overview

The OBI agent uses eBPF to automatically instrument applications without code changes. It:

- Captures HTTP/gRPC requests and responses
- Generates distributed traces automatically
- Detects service-to-service communication
- Sends telemetry to the OpenTelemetry Collector

## Prerequisites

- OpenShift 4.10+ cluster with eBPF support
- `oc` CLI configured with cluster-admin privileges
- **OTel Collector already deployed** (see `../otel-collector/`)
- Kernel version 4.18+ with BTF support (standard on RHCOS)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        OpenShift Node                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   App Pod   │  │   App Pod   │  │   App Pod   │         │
│  │  :8080      │  │  :8443      │  │  :9090      │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          │ eBPF hooks                       │
│                          ▼                                  │
│  ┌───────────────────────────────────────────────────────┐ │
│  │              OBI Agent (DaemonSet Pod)                │ │
│  │   - eBPF probes for HTTP/gRPC                         │ │
│  │   - Auto-generates traces                             │ │
│  │   - Discovers service topology                        │ │
│  └───────────────────────┬───────────────────────────────┘ │
│                          │ OTLP/HTTP                        │
└──────────────────────────┼──────────────────────────────────┘
                           ▼
              ┌─────────────────────────┐
              │    OTel Collector       │
              │    (ClusterIP Service)  │
              │    :4317 (gRPC)         │
              │    :4318 (HTTP)         │
              └─────────────────────────┘
```

## Files

| File | Description |
|------|-------------|
| `serviceaccount.yaml` | ServiceAccount for OBI pods |
| `clusterrole.yaml` | RBAC permissions for pod discovery |
| `clusterrolebinding.yaml` | Binds ClusterRole to ServiceAccount |
| `scc.yaml` | OpenShift Security Context Constraints (privileged) |
| `configmap.yaml` | Configuration: ports to instrument, log level |
| `daemonset.yaml` | DaemonSet deployment spec |
| `kustomization.yaml` | Kustomize configuration |

## Quick Start

### Prerequisites

Ensure the OTel Collector is deployed first:

```bash
cd ../otel-collector
oc apply -k .
```

### Deploy OBI

```bash
# Preview the manifests
oc kustomize .

# Apply to cluster
oc apply -k .
```

### Manual Deployment

```bash
# Apply Security Context Constraints (requires cluster-admin)
oc apply -f scc.yaml

# Apply remaining resources
oc apply -f serviceaccount.yaml
oc apply -f clusterrole.yaml
oc apply -f clusterrolebinding.yaml
oc apply -f configmap.yaml
oc apply -f daemonset.yaml
```

## Configuration

### Ports to Instrument

Edit `configmap.yaml` to specify which ports to instrument:

```yaml
data:
  # Comma-separated ports or ranges
  OTEL_EBPF_OPEN_PORT: "8080,8443,9090,3000"
  
  # Or use ranges
  # OTEL_EBPF_OPEN_PORT: "8080-8999"
```

The agent will automatically instrument any process listening on these ports.

### OTel Collector Endpoint

By default, OBI sends traces to:
```
http://otel-collector.mirador-observability.svc.cluster.local:4318
```

To change this, edit the `OTEL_EXPORTER_OTLP_ENDPOINT` in `daemonset.yaml`.

### Debug Mode

Enable trace printing for debugging:

```yaml
# In configmap.yaml
data:
  OTEL_EBPF_TRACE_PRINTER: "text"  # or "json"
```

Then check the OBI pod logs to see traces.

## Verifying the Deployment

### Check DaemonSet Status

```bash
oc get daemonset otel-ebpf-instrument -n mirador-observability
oc get pods -n mirador-observability -l app.kubernetes.io/name=otel-ebpf-instrument
```

### View Logs

```bash
oc logs -n mirador-observability -l app.kubernetes.io/name=otel-ebpf-instrument -f
```

### Verify eBPF Programs Loaded

```bash
# Exec into an OBI pod
oc exec -n mirador-observability -it $(oc get pod -n mirador-observability -l app.kubernetes.io/name=otel-ebpf-instrument -o jsonpath='{.items[0].metadata.name}') -- bpftool prog list
```

### Check Traces in Collector

```bash
# Check collector logs for incoming traces
oc logs -n mirador-observability -l app.kubernetes.io/name=otel-collector | grep -i trace
```

## Troubleshooting

### Pods Not Starting

1. Check SCC assignment:
   ```bash
   oc get scc otel-ebpf-instrument-scc -o yaml
   oc adm policy who-can use scc otel-ebpf-instrument-scc
   ```

2. Check events:
   ```bash
   oc get events -n mirador-observability --sort-by='.lastTimestamp'
   ```

3. Check for missing kernel features:
   ```bash
   oc logs -n mirador-observability -l app.kubernetes.io/name=otel-ebpf-instrument | grep -i error
   ```

### No Traces Generated

1. Verify ports are correct:
   ```bash
   # Check what ports your applications are using
   oc get pods -A -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[*].ports[*].containerPort}{"\n"}{end}'
   ```

2. Check OBI can reach the collector:
   ```bash
   oc exec -n mirador-observability -it $(oc get pod -n mirador-observability -l app.kubernetes.io/name=otel-ebpf-instrument -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://otel-collector.mirador-observability.svc.cluster.local:4318/health
   ```

### eBPF Permission Denied

Ensure the node supports eBPF and BTF:

```bash
# Check BTF availability (should return a file path)
oc debug node/<node-name> -- chroot /host ls -la /sys/kernel/btf/vmlinux
```

### CRI-O Socket Not Found

On some OpenShift versions, the CRI-O socket path may differ. Check:

```bash
oc debug node/<node-name> -- chroot /host ls -la /var/run/crio/
```

If the socket is at a different path, update `daemonset.yaml`:

```yaml
volumes:
  - name: crio-socket
    hostPath:
      path: /run/crio/crio.sock  # Alternative path
      type: Socket
```

## Security Considerations

The OBI agent requires privileged access because:

1. **eBPF Programs**: Requires `CAP_SYS_ADMIN` or `CAP_BPF` to load eBPF programs
2. **Host PID Namespace**: Needs access to host processes to instrument
3. **Kernel Debug FS**: Accesses `/sys/kernel/debug` for eBPF maps
4. **Process Tracing**: Uses `CAP_SYS_PTRACE` for process inspection

**Best Practices:**
- Deploy only in trusted namespaces
- Use network policies to restrict OBI's network access
- Monitor OBI pods for anomalous behavior
- Keep the image updated for security patches

## Uninstall

```bash
# Using Kustomize
oc delete -k .

# Or manually
oc delete daemonset otel-ebpf-instrument -n mirador-observability
oc delete configmap otel-ebpf-instrument-config -n mirador-observability
oc delete clusterrolebinding otel-ebpf-instrument
oc delete clusterrole otel-ebpf-instrument
oc delete serviceaccount otel-ebpf-instrument -n mirador-observability
oc delete scc otel-ebpf-instrument-scc
```

## Version Updates

To update the OBI image version, modify `kustomization.yaml`:

```yaml
images:
  - name: docker.io/otel/ebpf-instrument
    newTag: "v1.0.0"  # Use a specific version in production
```

Then reapply:
```bash
oc apply -k .
```

> **Note**: The `main` tag is used by default for latest features. For production, pin to a specific release tag.
