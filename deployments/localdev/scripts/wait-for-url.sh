#!/usr/bin/env bash
set -euo pipefail

URL="$1"
ATTEMPTS="${2:-60}"
SLEEP="${3:-2}"

for i in $(seq 1 "$ATTEMPTS"); do
  if curl -fsS "$URL" >/dev/null 2>&1; then
    echo "ready: $URL"
    exit 0
  fi
  sleep "$SLEEP"
done

echo "timeout waiting for $URL" >&2
exit 1

