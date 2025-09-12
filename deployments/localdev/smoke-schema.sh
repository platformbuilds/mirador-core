#!/usr/bin/env bash
set -euo pipefail

# Simple smoke tests for /api/v1/schema endpoints.
# Requires mirador-core running locally and Weaviate enabled.

BASE_URL="${BASE_URL:-http://localhost:8080/api/v1}"

pass=0
fail=0

say() { printf "\n==> %s\n" "$*"; }
ok()  { printf "✔ %s\n" "$*"; }
bad() { printf "✘ %s\n" "$*"; }

req() {
  # req METHOD PATH [JSON_BODY]
  local method="$1"; shift
  local path="$1"; shift
  local data="${1:-}"
  local url="$BASE_URL$path"
  if [[ -n "$data" ]]; then
    resp=$(curl -sS -o >(cat >&1) -w "\n%{http_code}" -X "$method" "$url" \
      -H 'Content-Type: application/json' \
      --data "$data")
  else
    resp=$(curl -sS -o >(cat >&1) -w "\n%{http_code}" -X "$method" "$url")
  fi
}

expect_200() {
  # expect_200 DESC METHOD PATH [JSON_BODY]
  local desc="$1"; shift
  local method="$1"; shift
  local path="$1"; shift
  local data="${1:-}"
  say "$desc" || true
  out_with_code=$(req "$method" "$path" "$data") || true
  code=$(printf "%s" "$out_with_code" | tail -n1)
  body=$(printf "%s" "$out_with_code" | sed '$d')
  if [[ "$code" == "200" ]]; then
    ok "$path ($code)"
    pass=$((pass+1))
  else
    bad "$path ($code)"
    printf "%s\n" "$body"
    fail=$((fail+1))
  fi
}

# Test payloads
TENANT=""

METRIC_BODY='{
  "tenantId": "',"$TENANT"'",
  "metric": "cpu_usage",
  "description": "CPU usage over time",
  "owner": "team-core",
  "tags": {"system":"prometheus"},
  "author": "smoke"
}'

LOG_FIELD_BODY='{
  "tenantId": "',"$TENANT"'",
  "field": "trace_id",
  "type": "string",
  "description": "Trace identifier",
  "tags": {"scope":"distributed-tracing"},
  "examples": {"value":"abc-123"},
  "author": "smoke"
}'

SERVICE_BODY='{
  "tenantId": "',"$TENANT"'",
  "service": "api-gateway",
  "purpose": "Routes and aggregates API calls",
  "owner": "team-core",
  "tags": {"tier":"edge"},
  "author": "smoke"
}'

OP_BODY='{
  "tenantId": "',"$TENANT"'",
  "service": "api-gateway",
  "operation": "get_users",
  "purpose": "GET /users",
  "owner": "team-core",
  "tags": {"method":"GET"},
  "author": "smoke"
}'

# Metrics
expect_200 "Upsert metric"         POST "/schema/metrics"            "$METRIC_BODY"
expect_200 "Get metric"            GET  "/schema/metrics/cpu_usage"
expect_200 "List metric versions"  GET  "/schema/metrics/cpu_usage/versions"

# Logs
expect_200 "Upsert log field"         POST "/schema/logs/fields"            "$LOG_FIELD_BODY"
expect_200 "Get log field"            GET  "/schema/logs/fields/trace_id"
expect_200 "List log field versions"  GET  "/schema/logs/fields/trace_id/versions"

# Traces
expect_200 "Upsert trace service"         POST "/schema/traces/services"                               "$SERVICE_BODY"
expect_200 "Get trace service"            GET  "/schema/traces/services/api-gateway"
expect_200 "List trace service versions"  GET  "/schema/traces/services/api-gateway/versions"

expect_200 "Upsert trace operation"         POST "/schema/traces/operations"                                                    "$OP_BODY"
expect_200 "Get trace operation"            GET  "/schema/traces/services/api-gateway/operations/get_users"
expect_200 "List trace operation versions"  GET  "/schema/traces/services/api-gateway/operations/get_users/versions"

printf "\nSummary: %d passed, %d failed\n" "$pass" "$fail"
if [[ "$fail" -gt 0 ]]; then
  exit 1
fi
exit 0

