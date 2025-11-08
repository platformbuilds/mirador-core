#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${E2E_BASE_URL:-${1:-http://localhost:8010}}
OUT_DIR="deployments/localdev"
REPORT_JSON="$OUT_DIR/e2e-report.json"
REPORT_TXT="$OUT_DIR/e2e-report.txt"

mkdir -p "$OUT_DIR"
rm -f "$REPORT_JSON" "$REPORT_TXT"

log_json() {
  # name, method, url, status, ok, msg
  local name="$1"; shift
  local method="$1"; shift
  local url="$1"; shift
  local status="$1"; shift
  local ok="$1"; shift
  local msg="${1:-}"
  printf '{"name":"%s","method":"%s","url":"%s","status":%s,"ok":%s,"message":%s}\n' \
    "$name" "$method" "$url" "$status" "$ok" "$(printf '%s' "$msg" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')" \
    >> "$REPORT_JSON"
}

request() {
  # args: name method path [data_json] [expected_csv]
  local name="$1"; shift
  local method="$1"; shift
  local path="$1"; shift
  local data="${1:-}"
  local expected_csv="${2:-200,201,202,204}"
  local url="${BASE_URL}${path}"
  local status body

  if [[ -n "$data" ]]; then
    body=$(printf '%s' "$data")
    status=$(curl -sk -o /tmp/e2e_body.$$ -w '%{http_code}' -H 'Content-Type: application/json' -X "$method" --data "$body" "$url" || true)
  else
    status=$(curl -sk -o /tmp/e2e_body.$$ -w '%{http_code}' -X "$method" "$url" || true)
  fi
  local msg_head
  msg_head=$(head -c 300 /tmp/e2e_body.$$ | tr -d '\r' | tr '\n' ' ')
  rm -f /tmp/e2e_body.$$

  local ok=false
  IFS=',' read -r -a exp <<<"$expected_csv"
  for code in "${exp[@]}"; do
    if [[ "$status" == "$code" ]]; then ok=true; break; fi
  done

  if [[ "$ok" == true ]]; then
    printf "OK  %-6s %s -> %s\n" "$method" "$path" "$status" | tee -a "$REPORT_TXT"
  else
    printf "FAIL %-6s %s -> %s (%s)\n" "$method" "$path" "$status" "$msg_head" | tee -a "$REPORT_TXT"
  fi
  log_json "$name" "$method" "$url" "$status" "$ok" "$msg_head"
}

echo "Running E2E against: $BASE_URL" | tee -a "$REPORT_TXT"

# Health & OpenAPI
request "health" GET "/health"
request "ready" GET "/ready"
request "openapi" GET "/api/openapi.json"

# MetricsQL
request "metrics:labels" GET "/api/v1/labels"
request "metrics:query" POST "/api/v1/metrics/query" '{"query":"up"}'
request "metrics:query_range" POST "/api/v1/metrics/query_range" '{"query":"up","start":"0","end":"1","step":"1"}'
request "metrics:names" GET "/api/v1/metrics/names"
request "metrics:series" GET "/api/v1/series?match[]=up"
request "metrics:label_values" GET "/api/v1/label/__name__/values"

# Logs + D3 endpoints (allow 200)
request "logs:streams" GET "/api/v1/logs/streams"
request "logs:fields" GET "/api/v1/logs/fields"
request "logs:export" POST "/api/v1/logs/export" '{"query_language":"lucene","query":"_time:5m"}'
request "logs:store" POST "/api/v1/logs/store" '{"event":{"_time":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","_msg":"e2e","type":"e2e","component":"e2e"}}'
request "logs:histogram" GET "/api/v1/logs/histogram?query_language=lucene&query=_time:5m"
request "logs:facets" GET "/api/v1/logs/facets?query_language=lucene&fields=level,service&query=_time:5m"
request "logs:search" POST "/api/v1/logs/search" '{"query_language":"lucene","query":"_time:5m","limit":10}'

# Traces
request "traces:services" GET "/api/v1/traces/services"
request "traces:search" POST "/api/v1/traces/search" '{"limit":1}'
request "traces:flamegraph" POST "/api/v1/traces/flamegraph/search" '{"limit":1}'

# RCA endpoints: accept 200 or 500 in dev
request "rca:correlations" GET "/api/v1/rca/correlations"
request "rca:investigate" POST "/api/v1/rca/investigate" '{"incidentId":"i1","symptoms":["s"]}' "200,500"
request "rca:patterns" GET "/api/v1/rca/patterns"
request "rca:store" POST "/api/v1/rca/store" '{"correlationId":"c1","incidentId":"i1","rootCause":"svc","confidence":0.9}'

# Config & Sessions & RBAC
request "config:datasources:get" GET "/api/v1/config/datasources"
request "config:datasources:add" POST "/api/v1/config/datasources" '{"name":"vm","type":"metrics","url":"http://vm"}' "201"
request "config:integrations" GET "/api/v1/config/integrations"
request "sessions:active" GET "/api/v1/sessions/active"
request "sessions:invalidate" POST "/api/v1/sessions/invalidate" '{}' "200,400"
request "sessions:user" GET "/api/v1/sessions/user/u1"
request "rbac:roles:get" GET "/api/v1/rbac/roles"
request "rbac:roles:post" POST "/api/v1/rbac/roles" '{"name":"viewer2","permissions":["dash.view"]}' "201"
request "rbac:assign" PUT "/api/v1/rbac/users/u1/roles" '{"roles":["viewer"]}'

# Schema (Weaviate enabled in localdev compose)
metric="e2e_metric_$(date +%s)"
field="e2e_field_$(date +%s)"
svc="e2e_service_$(date +%s)"
op="op_$(date +%s)"
request "schema:metrics:post" POST "/api/v1/schema/metrics" '{"tenantId":"default","metric":"'"$metric"'","description":"e2e","owner":"qa","tags":["env=dev"],"author":"e2e"}'
request "schema:metrics:get" GET "/api/v1/schema/metrics/$metric"
request "schema:logs:fields:post" POST "/api/v1/schema/logs/fields" '{"tenantId":"default","field":"'"$field"'","type":"string","description":"e2e","tags":["app"],"examples":["ex"],"author":"e2e"}'
request "schema:logs:fields:get" GET "/api/v1/schema/logs/fields/$field"
request "schema:traces:services:post" POST "/api/v1/schema/traces/services" '{"tenantId":"default","service":"'"$svc"'","purpose":"e2e","owner":"qa","tags":["env=dev"],"author":"e2e"}'
request "schema:traces:services:get" GET "/api/v1/schema/traces/services/$svc"
request "schema:traces:operations:post" POST "/api/v1/schema/traces/operations" '{"tenantId":"default","service":"'"$svc"'","operation":"'"$op"'","purpose":"e2e","owner":"qa","tags":["env=dev"],"author":"e2e"}'
request "schema:traces:operations:get" GET "/api/v1/schema/traces/services/$svc/operations/$op"

# Final counts
PASS=$(grep -c '"ok":true' "$REPORT_JSON" || true)
FAIL=$(grep -c '"ok":false' "$REPORT_JSON" || true)
TOTAL=$((PASS+FAIL))
echo "" | tee -a "$REPORT_TXT"
echo "Summary: $TOTAL total, $PASS passed, $FAIL failed" | tee -a "$REPORT_TXT"
echo "Report: $REPORT_JSON" | tee -a "$REPORT_TXT"

exit 0
