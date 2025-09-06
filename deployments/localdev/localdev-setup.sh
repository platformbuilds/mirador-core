#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"

echo "=== MIRADOR-CORE local dev setup ==="
read -r -p "Enter mirador-core image tag to pull (leave blank to build locally): " MC_TAG
MC_TAG=${MC_TAG:-}

if [ -n "$MC_TAG" ]; then
  echo "Using mirador-core tag: ${MC_TAG} (will pull image)"
else
  echo "No image tag provided — will build mirador-core locally for native arch"
fi

echo "\n[1/4] Starting Victoria single nodes (metrics/logs/traces)..."
docker compose -f "$here/victoria-docker-compose.yaml" up -d

echo "\n[2/4] Starting OpenTelemetry Collector..."
# Clean any legacy network named 'mirador-localdev' to avoid label mismatches
if docker network inspect mirador-localdev >/dev/null 2>&1; then
  echo "Found legacy network 'mirador-localdev' — removing to avoid label conflicts"
  docker network rm mirador-localdev >/dev/null 2>&1 || true
fi
docker compose -f "$here/otel-collector-docker-compose.yaml" up -d

echo "\n[3/4] Starting Valkey + MIRADOR-CORE..."
if [ -n "$MC_TAG" ]; then
  # Create a tiny override file to set the image tag
  override_file="$(mktemp -t miradorcore-override-XXXX).yaml"
  cat > "$override_file" <<EOF
services:
  mirador-core:
    image: platformbuilds/mirador-core:${MC_TAG}
EOF
  docker compose -f "$here/mirador-core-docker-compose.yaml" -f "$override_file" up -d
else
  docker compose -f "$here/mirador-core-docker-compose.yaml" up -d --build
fi

echo "Waiting for MIRADOR-CORE to be healthy (http://localhost:8080/health)..."
for i in {1..60}; do
  if curl -fsS http://localhost:8080/health >/dev/null 2>&1; then
    echo "MIRADOR-CORE is up."
    break
  fi
  sleep 2
done

echo "\n[4/4] Seeding a local session token in Valkey so API calls work without SSO..."
TOKEN="devtoken-$(date +%s)"
NOW_RFC3339="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
# Minimal session JSON as expected by MIRADOR-CORE
SESSION_JSON=$(cat <<JSON
{
  "id": "${TOKEN}",
  "user_id": "localdev",
  "tenant_id": "default",
  "roles": ["admin"],
  "created_at": "${NOW_RFC3339}",
  "last_activity": "${NOW_RFC3339}",
  "user_settings": {},
  "ip_address": "",
  "user_agent": "localdev"
}
JSON
)

# Store with 24h TTL (86400 seconds); valkey image ships valkey-cli
docker exec mirador-valkey sh -c "echo '$SESSION_JSON' | valkey-cli SETEX session:${TOKEN} 86400 -"

cat <<EOT

=== Local dev is ready ===

Services:
- VictoriaMetrics: http://localhost:8428
- VictoriaLogs:    http://localhost:9428
- VictoriaTraces:  http://localhost:8429 (enable Jaeger HTTP on 14268 if needed)
- OTEL Collector:  OTLP gRPC :4317, OTLP HTTP :4318
- MIRADOR-CORE:    http://localhost:8080

Session token for API calls (valid ~24h):
  ${TOKEN}

Generate telemetry (requires Go: telemetrygen installed):
  go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest

Traces (gRPC):
  telemetrygen traces --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 5

Metrics (gRPC):
  telemetrygen metrics --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 100

Logs (gRPC):
  telemetrygen logs --otlp-endpoint localhost:4317 --otlp-insecure --duration 30s --rate 10

Query via MIRADOR-CORE (use X-Session-Token header):

- MetricsQL instant query:
  curl -sS -H "X-Session-Token: ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"query":"up"}' \
    http://localhost:8080/api/v1/query | jq .

- LogsQL query (last 5 minutes):
  curl -sS -H "X-Session-Token: ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"query":"_time:5m"}' \
    http://localhost:8080/api/v1/logs/query | jq .

- Traces (list services via Jaeger-compatible API):
  curl -sS -H "X-Session-Token: ${TOKEN}" \
    http://localhost:8080/api/v1/traces/services | jq .

Cleanup:
  docker compose -f "$here/mirador-core-docker-compose.yaml" down
  docker compose -f "$here/otel-collector-docker-compose.yaml" down
  docker compose -f "$here/victoria-docker-compose.yaml" down

EOT
