#!/usr/bin/env bash
# Reindex/KPI re-upsert utility for localdev or CI
#
# This script lists KPI definitions via the API and re-upserts them back to the
# API to ensure the `content` field and runtime vectors are recomputed.
#
# Usage:
#   ./devtools/reindex-kpis.sh --host localhost --port 8010 --dry-run --batch 100

set -euo pipefail

HOST="${1:-localhost}"
PORT="${2:-8010}"

# simple option parsing for optional flags
DRY_RUN=false
BATCH=100
while [[ $# -gt 0 ]]; do
  case $1 in
    --host) HOST="$2"; shift 2;;
    --port) PORT="$2"; shift 2;;
    --dry-run) DRY_RUN=true; shift 1;;
    --batch) BATCH="$2"; shift 2;;
    --help) echo "Usage: $0 [--host HOST] [--port PORT] [--dry-run] [--batch N]"; exit 0;;
    *) shift 1;;
  esac
done

API="http://${HOST}:${PORT}/api/v1"

echo "Reindexing KPIs against ${API} (dry-run=${DRY_RUN}, batch=${BATCH})"

page=0
limit=${BATCH}
offset=0
total=0

if ! command -v jq >/dev/null 2>&1; then
  echo "This script requires jq. Install it and re-run."
  exit 1
fi

while true; do
  echo "Fetching KPIs offset=${offset} limit=${limit}..."
  resp=$(curl -s -G "${API}/kpi/defs" --data-urlencode "limit=${limit}" --data-urlencode "offset=${offset}") || { echo "failed to fetch list"; exit 1; }
  items=$(echo "$resp" | jq -c '.kpiDefinitions[]?')
  if [ -z "$items" ]; then
    echo "no more items; finished"
    break
  fi

  count=0
  echo "$items" | while IFS= read -r k; do
    id=$(echo "$k" | jq -r '.id')
    if [ "$id" = "null" ] || [ -z "$id" ]; then
      echo "skipping item without id"
      continue
    fi
    echo "Upserting KPI id=$id"
    if $DRY_RUN; then
      echo "DRY: would POST /kpi/defs for id=$id"
    else
      curl -s -X POST "${API}/kpi/defs" -H 'Content-Type: application/json' -d $(jq -c -n --argjson k "$k" '{kpiDefinition: $k}') >/dev/null || echo "warning: upsert failed for $id"
    fi
    count=$((count+1))
  done

  if [ $count -lt $limit ]; then
    break
  fi
  offset=$((offset + limit))
done

echo "Reindex complete"
