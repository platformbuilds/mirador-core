#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
compose="$here/docker-compose.yaml"

usage() {
  cat <<USAGE
Cleanup localdev stack for mirador-core

Usage:
  $(basename "$0") [--volumes] [--prune]

Options:
  --volumes   Remove Victoria* named volumes (vmdata, vldata, vtdata)
  --prune     Also prune dangling images/containers (docker system prune -f)

Examples:
  $(basename "$0") --volumes
  $(basename "$0") --volumes --prune
USAGE
}

VOL=false
PRUNE=false
for arg in "$@"; do
  case "$arg" in
    -h|--help) usage; exit 0 ;;
    --volumes) VOL=true ;;
    --prune) PRUNE=true ;;
    *) echo "Unknown arg: $arg"; usage; exit 1 ;;
  esac
done

echo "Stopping localdev stack..."
docker compose -f "$compose" down || true

if $VOL; then
  echo "Removing Victoria* volumes (vmdata, vldata, vtdata)..."
  docker volume rm vmdata vldata vtdata 2>/dev/null || true
fi

if $PRUNE; then
  echo "Pruning dangling images/containers..."
  docker system prune -f || true
fi

echo "Done."

