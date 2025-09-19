#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
OTLP=${OTLP:-localhost:4317}

here="$(cd "$(dirname "$0")" && pwd)"

pass=0; fail=0
ok() { echo "[PASS] $1"; pass=$((pass+1)); }
err() { echo "[FAIL] $1"; fail=$((fail+1)); }

require() { command -v "$1" >/dev/null 2>&1 || { echo "Missing command: $1"; exit 1; }; }

curl_json() {
  local method=$1 path=$2 body=${3:-}
  local url="$BASE_URL$path"
  if [[ -n "$body" ]]; then
    http_code=$(curl -sS -o /tmp/test_body.$$ -w "%{http_code}" -H 'Content-Type: application/json' -X "$method" --data "$body" "$url" || true)
  else
    http_code=$(curl -sS -o /tmp/test_body.$$ -w "%{http_code}" -X "$method" "$url" || true)
  fi
  echo "$http_code"
}

echo "== mirador-core API smoke tests =="
echo "BASE_URL=$BASE_URL  OTLP=$OTLP"

# Ensure telemetrygen (optional, but recommended)
if ! command -v telemetrygen >/dev/null 2>&1; then
  echo "telemetrygen not found; attempting install (requires Go)"
  if command -v go >/dev/null 2>&1; then
    GO111MODULE=on go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest || true
    export PATH="$HOME/go/bin:$PATH"
  fi
fi

if command -v telemetrygen >/dev/null 2>&1; then
  echo "Pumping sample telemetry (30s traces@5 rps, metrics@100 rps, logs@10 rps) to $OTLP"
  telemetrygen traces --otlp-endpoint "$OTLP" --otlp-insecure --duration 30s --rate 5 >/dev/null 2>&1 || true
  telemetrygen metrics --otlp-endpoint "$OTLP" --otlp-insecure --duration 30s --rate 100 >/dev/null 2>&1 || true
  telemetrygen logs --otlp-endpoint "$OTLP" --otlp-insecure --duration 30s --rate 10 >/dev/null 2>&1 || true
else
  echo "telemetrygen unavailable; continuing without sample data"
fi

# Time helpers
now_rfc=$(date -u +%Y-%m-%dT%H:%M:%SZ)
start_rfc=$(date -u -v-10M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '10 minutes ago' +%Y-%m-%dT%H:%M:%SZ)
now_ms=$(date -u +%s%3N)
start_ms=$((now_ms - 10*60*1000))

# 1) Health
code=$(curl_json GET /health)
[[ "$code" == 200 ]] && ok "/health" || err "/health -> $code"
code=$(curl_json GET /api/v1/health)
[[ "$code" == 200 ]] && ok "/api/v1/health" || err "/api/v1/health -> $code"

# 2) Metrics: instant and range
code=$(curl_json POST /api/v1/metrics/query '{"query":"1"}')
[[ "$code" == 200 ]] && ok "MetricsQL instant" || err "MetricsQL instant -> $code"
code=$(curl_json POST /api/v1/metrics/query_range "{\"query\":\"1\",\"start\":\"$start_rfc\",\"end\":\"$now_rfc\",\"step\":\"30s\"}")
[[ "$code" == 200 ]] && ok "MetricsQL range" || err "MetricsQL range -> $code"

# 3) Metrics: labels, names, label values
code=$(curl_json GET /api/v1/labels)
[[ "$code" == 200 ]] && ok "Metrics labels" || err "Metrics labels -> $code"
code=$(curl_json GET "/api/v1/metrics/names?start=$start_rfc&end=$now_rfc")
[[ "$code" == 200 ]] && ok "Metric names" || err "Metric names -> $code"
code=$(curl_json GET "/api/v1/label/__name__/values?start=$start_rfc&end=$now_rfc")
[[ "$code" == 200 ]] && ok "Label values __name__" || err "Label values __name__ -> $code"

# 4) Logs: query (ms), fields, streams
code=$(curl_json POST /api/v1/logs/query "{\"query\":\"_time:5m\",\"start\":$start_ms,\"end\":$now_ms,\"limit\":50}")
[[ "$code" == 200 ]] && ok "Logs query" || err "Logs query -> $code"
code=$(curl_json GET /api/v1/logs/fields)
[[ "$code" == 200 ]] && ok "Logs fields" || err "Logs fields -> $code"
code=$(curl_json GET /api/v1/logs/streams)
[[ "$code" == 200 ]] && ok "Logs streams" || err "Logs streams -> $code"

# 5) Traces: services (tolerate empty)
code=$(curl_json GET /api/v1/traces/services)
[[ "$code" == 200 ]] && ok "Traces services" || err "Traces services -> $code"

echo
echo "Summary: $pass passed, $fail failed"
exit $(( fail > 0 ))
