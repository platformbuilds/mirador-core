SHELL := /bin/bash

.PHONY: help localdev localdev-up localdev-down localdev-wait localdev-test localdev-seed-otel

BASE_URL ?= http://localhost:8080

help:
	@printf "%s\n" \
	"" \
	"Mirador-Core Makefile — Localdev & E2E" \
	"" \
	"Usage:" \
	"  make <target> [VAR=value]" \
	"" \
	"Common Targets:" \
	"  help                 Show this help with available targets and options." \
	"  localdev             Full local E2E flow: up → wait → seed OTEL → test → down." \
	"  localdev-up          Build and start localdev Docker stack (Compose) in background." \
	"  localdev-wait        Wait until the app is ready (probes $(BASE_URL)/ready)." \
	"  localdev-seed-otel   Seed synthetic OpenTelemetry metrics/logs/traces via telemetrygen." \
	"  localdev-test        Run end-to-end tests against a running localdev server." \
	"  localdev-down        Tear down the localdev stack and remove volumes." \
	"" \
	"Key Paths & Files:" \
	"  deployments/localdev/docker-compose.yaml            Compose services (VM, VL, VT, Valkey, Weaviate, OTEL, app)" \
	"  deployments/localdev/scripts/wait-for-url.sh        Readiness probe helper" \
	"  deployments/localdev/e2e                            E2E test suite (Go)" \
	"  localdev/e2e-report.json                            JSON test report output" \
	"  localdev/e2e-report.xml                             Optional JUnit XML report (if go-junit-report present)" \
	"" \
	"Environment Variables:" \
	"  BASE_URL          Base URL for the running app (default: http://localhost:8080)." \
	"                    Used by localdev-wait and passed to tests as E2E_BASE_URL." \
	"" \
	"External Tools:" \
	"  telemetrygen      Auto-installed on first use by localdev-seed-otel." \
	"                    Source: github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen" \
	"  go-junit-report   Optional, converts JSON test output to JUnit XML." \
	"                    Install: go install github.com/jstemmer/go-junit-report/v2@latest" \
	"" \
	"Examples:" \
	"  make help" \
	"  make localdev" \
	"  make localdev BASE_URL=http://127.0.0.1:8080" \
	"  make localdev-up && make localdev-wait && make localdev-seed-otel && make localdev-test" \
	"" \
	"Notes:" \
	"  - Auth is disabled by default in the localdev compose." \
	"  - localdev-down runs 'docker compose ... down -v' and removes volumes created by that compose file."

localdev: localdev-up localdev-wait localdev-seed-otel localdev-test localdev-down
	@echo "Localdev E2E completed. Reports under localdev/."

localdev-up:
	mkdir -p localdev
	# Pull images only if missing (prevents re-pulling on every run)
	docker compose -f deployments/localdev/docker-compose.yaml up -d --build --pull=missing

localdev-wait:
	@deployments/localdev/scripts/wait-for-url.sh $(BASE_URL)/ready 120 2

localdev-test:
	mkdir -p deployments/localdev
	E2E_BASE_URL=$(BASE_URL) bash deployments/localdev/scripts/run-e2e.sh
	@echo "=========================================================="
	@echo "=== E2E Summary (deployments/localdev/e2e-report.json) ==="
	@echo "=========================================================="
	@REPORT=deployments/localdev/e2e-report.json; \
	if [ -f "$$REPORT" ]; then \
	  ALL=$$(wc -l < "$$REPORT" | tr -d ' '); \
	  PASSED=$$(grep -c '\"ok\":true' "$$REPORT" || true); \
	  FAILED=$$(grep -c '\"ok\":false' "$$REPORT" || true); \
	  echo "Tests=$$ALL total, $$PASSED passed, $$FAILED failed"; \
	  echo "=========================================================="; \
	  echo; echo "Failed tests:"; \
	  FAIL_LIST=$$(grep '\"ok\":false' "$$REPORT" | grep -o '\"name\":\"[^\"]*\"' | cut -d: -f2- | tr -d '\"' | sed '/^$$/d' | sort -u); \
	  if [ -n "$$FAIL_LIST" ]; then \
	    while IFS= read -r T; do \
	      MSG=$$(grep -F '\"name\":\"'"$$T"'\"' "$$REPORT" | tail -1 | sed -E 's/.*\"message\":\"//; s/\"\}\s*$$//'); \
	      echo "  - $$T: $$MSG"; \
	    done <<< "$$FAIL_LIST"; \
	  else \
	    echo "  (none)"; \
	  fi; \
	  echo; echo "See $$REPORT for full details."; \
	else \
	  echo "Report not found: $$REPORT"; \
	fi
	@echo "=========================================================="

localdev-seed-otel:
	@echo "Seeding synthetic OpenTelemetry data via telemetrygen..."
	@command -v telemetrygen >/dev/null 2>&1 || { echo "Installing telemetrygen..."; go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest; }
	telemetrygen metrics --otlp-endpoint localhost:4317 --otlp-insecure --duration 10s --rate 200 || true
	telemetrygen logs --otlp-endpoint localhost:4317 --otlp-insecure --duration 10s --rate 20 || true
	telemetrygen traces --otlp-endpoint localhost:4317 --otlp-insecure --duration 10s --rate 10 || true

localdev-down:
	@docker compose -f deployments/localdev/docker-compose.yaml down -v
