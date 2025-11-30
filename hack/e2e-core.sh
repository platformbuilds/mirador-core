#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

echo "[e2e-core] Running E2E pipeline via Makefile (localdev sequence)"

# Prefer invoking the Makefile sequence that performs: up -> wait -> seed -> test -> down
if command -v make >/dev/null 2>&1; then
  make localdev
else
  echo "make not found in PATH; cannot run E2E pipeline" >&2
  exit 1
fi

echo "[e2e-core] Completed"
