#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"

# Support: ./localdev-setup.sh stop|down to stop the stack
if [[ ${1-} == "stop" || ${1-} == "down" ]]; then
  echo "Stopping localdev stack..."
  docker compose -f "$here/docker-compose.yaml" down
  echo "Done."
  exit 0
fi

echo "=== MIRADOR-CORE local dev setup ==="
read -r -p "Enter mirador-core image tag to pull (leave blank to build locally): " MC_TAG
MC_TAG=${MC_TAG:-}

if [ -n "$MC_TAG" ]; then
  echo "Using mirador-core tag: ${MC_TAG} (will pull image)"
else
  echo "No image tag provided — will build mirador-core locally for native arch"
fi

echo "\n[1/2] Starting full local stack (Victoria + OTEL + Valkey + MIRADOR-CORE)..."
if [ -n "$MC_TAG" ]; then
  # Create a tiny override file to set the image tag
  override_file="$(mktemp -t miradorcore-override-XXXX).yaml"
  cat > "$override_file" <<EOF
services:
  mirador-core:
    image: platformbuilds/mirador-core:${MC_TAG}
EOF
  docker compose -f "$here/docker-compose.yaml" -f "$override_file" up -d
else
  docker compose -f "$here/docker-compose.yaml" up -d --build 
fi

echo "Waiting for MIRADOR-CORE to be healthy (http://localhost:8080/health)..."
for i in {1..60}; do
  if curl -fsS http://localhost:8080/health >/dev/null 2>&1; then
    echo "MIRADOR-CORE is up."
    break
  fi
  sleep 2
done

echo "\n[2/2] Auth is disabled by default in localdev (AUTH_ENABLED=false). Skipping session seeding."

# -----------------------------------------------------------------------------
# Optional: generate sample telemetry via telemetrygen
# -----------------------------------------------------------------------------
read -r -p $'\nWould you like to pump sample telemetry (metrics/logs/traces) now? [Y/n]: ' GEN_TEL
GEN_TEL=${GEN_TEL:-Y}
if [[ "$GEN_TEL" =~ ^[Yy]$ ]]; then
  if ! command -v telemetrygen >/dev/null 2>&1; then
    echo "telemetrygen not found. Attempting to install (requires Go toolchain)..."
    if command -v go >/dev/null 2>&1; then
      # best-effort install; do not fail script if install fails
      GO111MODULE=on go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest || {
        echo "Install failed; you can manually install telemetrygen and re-run later.";
      }
      # add GOPATH/bin to PATH for current shell if needed
      if [ -d "$HOME/go/bin" ]; then
        export PATH="$HOME/go/bin:$PATH"
      fi
    else
      echo "Go is not installed; skipping auto-install of telemetrygen."
    fi
  fi

  if command -v telemetrygen >/dev/null 2>&1; then
    echo "\nPumping traces (30s @ 5 rps) to OTLP gRPC 4317..."
    otelgen --otel-exporter-otlp-endpoint localhost:4317 --insecure --duration 30 --rate 50 traces multi
    telemetrygen traces --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 5 || true

    echo "\nPumping metrics (30s @ 100 rps) to OTLP gRPC 4317..."
    telemetrygen metrics --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 100 || true

    echo "\nPumping logs (30s @ 10 rps) to OTLP gRPC 4317..."
    telemetrygen logs --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 10 || true
  else
    echo "telemetrygen is not available; skipping sample telemetry generation."
  fi
fi

cat <<EOT

=== Local dev is ready ===

Services:
- VictoriaMetrics: http://localhost:8428
- VictoriaLogs:    http://localhost:9428
- VictoriaTraces:  http://localhost:10428 (enable Jaeger HTTP on 14268 if needed)
- OTEL Collector:  OTLP gRPC :4317, OTLP HTTP :4318
- MIRADOR-CORE:    http://localhost:8080

Generate telemetry (requires Go: telemetrygen installed):
  go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest

Traces (gRPC):
  telemetrygen traces --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 5

Metrics (gRPC):
  telemetrygen metrics --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 100

Logs (gRPC):
  telemetrygen logs --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 10

Query via MIRADOR-CORE (auth disabled; no token required):

- MetricsQL instant query:
  curl -sS -H "Content-Type: application/json" \
    -d '{"query":"up"}' \
    http://localhost:8080/api/v1/query | jq .

- LogsQL query (last 5 minutes):
  curl -sS -H "Content-Type: application/json" \
    -d '{"query":"_time:5m"}' \
    http://localhost:8080/api/v1/logs/query | jq .

- Traces (list services via Jaeger-compatible API):
  curl -sS http://localhost:8080/api/v1/traces/services | jq .

Cleanup:
  docker compose -f "$here/docker-compose.yaml" down

EOT
