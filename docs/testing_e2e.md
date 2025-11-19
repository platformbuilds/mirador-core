# End-to-End Testing & QA

This page describes the E2E testing pipeline for `mirador-core`.

## Overview

The E2E pipeline validates the following focused areas:
- Config APIs: Datasource and integration management
- KPI APIs: Create/read/list KPI definitions
- UQL endpoints: UQL validation and query execution
- Correlation & RCA: cross-engine correlation and RCA requests

## Entry points

- `make e2e` — single entry point to run the E2E pipeline locally or in CI. Internally calls `hack/e2e-core.sh`.
- `hack/e2e-core.sh` — script that performs the sequence of steps: bring up localdev, wait for readiness, seed OTEL, run e2e tests, run lint.

## Where tests live

- Go E2E tests (tagged `e2e`): `internal/api/*_e2e_test.go`
- Shell-level API tests: `localtesting/e2e-tests.sh` (used for additional smoke checks and reporting)

## Re-running in development

1. Start localdev: `make localdev-up`
2. Wait for ready: `make localdev-wait`
3. Seed OTEL: `make localdev-seed-otel`
4. Run E2E: `make e2e`

## CI

- The E2E pipeline is added to the main CI workflow as a separate `e2e` job which depends on the main `ci` job.

## Troubleshooting

- If seeding OTEL times out, validate that `OTLP` endpoint is reachable on `localhost:4317` and that Docker has enough resources.
- If any of the requests return `403`/`401` while localdev has auth disabled, check `deployments/localdev/localdev-setup.sh` and `AUTH_ENABLED` config.
- Check `localtesting/e2e-report.json` for a structured test report when using `localtesting/e2e-tests.sh`.
