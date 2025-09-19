SHELL := /bin/bash

.PHONY: help \
	localdev localdev-up localdev-down localdev-wait localdev-test localdev-seed-otel \
	build build-native build-linux-multi build-linux-amd64 build-linux-arm64 build-darwin-arm64 build-windows-amd64 build-all \
	docker docker-build docker-build-native dockerx-build dockerx-push docker-publish-release docker-publish-canary docker-publish-pr \
	release test clean proto vendor lint run dev setup tools check-tools dev-stack dev-stack-down fmt version proto-clean clean-build \
	tag-release helm-bump version-human version-ci vuln dockerx-build-local-multi buildx-ensure helm-sync-deps helm-dep-update

BASE_URL ?= http://localhost:8080

# -----------------------------
# Build and release variables
# -----------------------------
BINARY_NAME?=mirador-core
VERSION?=v2.1.3
BUILD_TIME:=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT_HASH:=$(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
LDFLAGS=-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commitHash=$(COMMIT_HASH)

# Container image settings
REGISTRY?=platformbuilds
IMAGE_NAME?=$(BINARY_NAME)
IMAGE=$(REGISTRY)/$(IMAGE_NAME)
DOCKER_PLATFORMS?=linux/amd64,linux/arm64

# Host platform (for native builds)
HOST_OS?=$(shell go env GOOS)
HOST_ARCH?=$(shell go env GOARCH)

# CI/environment metadata
BRANCH?=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "local")
SHA_SHORT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE_YYYYMMDD:=$(shell date -u +%Y.%m.%d)
DATE_CALVER:=$(shell date -u +%Y.%m.%d)
PR_NUMBER?=
CI_TAG?=

help:
	@printf "%s\n" \
	"" \
	"Mirador-Core Makefile — Commands" \
	"" \
	"Usage:" \
	"  make <target> [VAR=value]" \
	"" \
	"Localdev E2E:" \
	"  help                      Show this help with all targets." \
	"  localdev                  Full local E2E: up → wait → seed OTEL → test → down." \
	"  localdev-up               Start localdev compose stack in background." \
	"  localdev-wait             Wait for readiness at $(BASE_URL)/ready." \
	"  localdev-seed-otel        Seed synthetic OTEL metrics/logs/traces via telemetrygen." \
	"  localdev-test             Run E2E tests against a running localdev server." \
	"  localdev-down             Tear down localdev stack and remove volumes." \
	"" \
	"Development & Build:" \
	"  setup                     Install tools, generate proto, download deps." \
	"  proto                     Generate Protocol Buffer code from .proto files." \
	"  proto-clean               Remove generated .pb.go then regenerate proto code." \
	"  build                     Static linux/amd64 build to bin/$(BINARY_NAME)." \
	"  build-native              Build for host OS/Arch to bin/$(BINARY_NAME)-<os>-<arch>." \
	"  build-linux-amd64         Build for linux/amd64." \
	"  build-linux-arm64         Build for linux/arm64." \
	"  build-linux-multi         Build linux binaries for amd64 and arm64." \
	"  build-darwin-arm64        Build for macOS arm64 (Apple Silicon)." \
	"  build-windows-amd64       Build for Windows amd64 (.exe)." \
	"  build-all                 Build common targets for all platforms above." \
	"  dev-build                 Development build with debug symbols." \
	"  dev                       Run server locally via 'go run'." \
	"  run                       Alias to 'dev'." \
	"  clean-build               Clean then perform a fresh build." \
	"  openapi-json              Regenerate api/openapi.json from api/openapi.yaml." \
	"  openapi-validate          Parse YAML → JSON to ensure syntax is valid." \
	"" \
	"Testing & Quality:" \
	"  test                      Run unit tests with race detector and coverage." \
	"  fmt                       Format code (go fmt, goimports)." \
	"  lint                      Run golangci-lint on the repo." \
	"  tools                     Install dev tools (protoc-gen-*, lint, swag, govulncheck)." \
	"  check-tools               Verify required tools are installed." \
	"  vuln                      Run govulncheck vulnerability scan." \
	"" \
	"Docker Images:" \
	"  docker                    Alias for docker-build (host arch)." \
	"  docker-build              Build single-arch image for host architecture." \
	"  docker-build-native       Build native-arch image via buildx and load locally." \
	"  buildx-ensure             Ensure containerized buildx builder exists/active." \
	"  dockerx-build             Multi-arch build with buildx (no push)." \
	"  dockerx-build-local-multi Build and load per-arch images locally (-amd64/-arm64)." \
	"  dockerx-push              Multi-arch build with buildx and push." \
	"" \
	"Release & Versioning:" \
	"  release                   Run tests then dockerx-push." \
	"  version                   Print MIRADOR-CORE version/build metadata." \
	"  version-human             Print semver components for $(VERSION)." \
	"  version-ci                Compute CI-friendly version from env/branch." \
	"  tag-release               Create and push git tag $(VERSION)." \
	"  docker-publish-release    Push semver fanout: vX.Y.Z, vX.Y, vX, latest, stable." \
	"  docker-publish-canary     Push canary tag (branch.date.sha) and 'canary'." \
	"  docker-publish-pr         Push PR tag 0.0.0-pr.<PR#>.<sha> and pr-<PR#>." \
	"" \
	"Helm (optional, if chart/ exists):" \
	"  helm-bump                 Update Chart.yaml appVersion/version via VERSION/CHART_VER." \
	"  helm-sync-deps            Sync Valkey dependency version from values.yaml (yq)." \
	"  helm-dep-update           Run 'helm dependency update' in chart/." \
	"" \
	"Dev Stack (root compose):" \
	"  dev-stack                 Start root docker-compose services for dependencies." \
	"  dev-stack-down            Stop root docker-compose services." \
	"" \
	"Environment Variables:" \
	"  BASE_URL                  Base URL for the running app (default: http://localhost:8080)." \
	"                            Used by localdev-wait and passed to tests as E2E_BASE_URL." \
	"" \
	"Notes:" \
	"  - Auth is disabled by default in the localdev compose." \
	"  - localdev-down runs 'docker compose ... down -v' and removes volumes created by that compose file."



localdev: localdev-up localdev-wait localdev-seed-otel localdev-test localdev-down
	@echo "Localdev E2E completed. Reports under localdev/."

.PHONY: openapi-json openapi-validate
openapi-json:
	@python3 scripts/gen_openapi_json.py

openapi-validate:
	@python3 - <<'PY'
	import sys, json
	try:
	  import yaml
	except Exception as e:
	  print('PyYAML not installed. Install via pip install pyyaml', file=sys.stderr)
	  sys.exit(1)
	from pathlib import Path
	y = Path('api/openapi.yaml').read_text(encoding='utf-8')
	data = yaml.safe_load(y)
	assert isinstance(data, dict) and 'openapi' in data, 'Invalid OpenAPI YAML: missing openapi key'
	print('YAML parse OK; version:', data.get('openapi'))
	print('Paths count:', len((data.get('paths') or {})))
	print('Components:', 'schemas' in (data.get('components') or {}))
	print('Validation (structural) OK')
	PY

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

# -----------------------------
# Legacy/General Build, Test, Docker, Release, Helm targets
# (Merged from older Makefile; preserved as-is)
# -----------------------------

# Setup development environment
setup:
	@echo "🚀 Setting up MIRADOR-CORE development environment..."
	@./scripts/generate-proto-code.sh
	@go mod download
	@echo "✅ Setup complete! Run 'make dev' to start development server."

# Generate Protocol Buffers from existing proto files
proto:
	@echo "🔧 Generating Protocol Buffer code from existing files..."
	@./scripts/generate-proto-code.sh

# Build
build: proto ## Release-style static build for Linux/amd64 (default)
	@echo "🔨 Building MIRADOR-CORE (linux/amd64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="$(LDFLAGS)" \
		-o bin/$(BINARY_NAME) \
		cmd/server/main.go

build-native: proto ## Build native (HOST_OS/HOST_ARCH)
	@echo "🔨 Building MIRADOR-CORE (native: $(HOST_OS)/$(HOST_ARCH))..."
	CGO_ENABLED=0 GOOS=$(HOST_OS) GOARCH=$(HOST_ARCH) go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-$(HOST_OS)-$(HOST_ARCH) cmd/server/main.go

build-linux-amd64: proto ## Build linux/amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-linux-amd64 cmd/server/main.go

build-linux-arm64: proto ## Build linux/arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-linux-arm64 cmd/server/main.go

build-linux-multi: build-linux-amd64 build-linux-arm64 ## Build linux binaries for amd64 and arm64

build-darwin-arm64: proto ## Build darwin/arm64 (Apple Silicon)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-darwin-arm64 cmd/server/main.go

build-windows-amd64: proto ## Build windows/amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-windows-amd64.exe cmd/server/main.go

build-all: build-linux-amd64 build-linux-arm64 build-darwin-arm64 build-windows-amd64 ## Build all common targets

# Development build (with debug symbols)
dev-build: proto
	@echo "🔨 Building MIRADOR-CORE for development..."
	go build -o bin/$(BINARY_NAME)-dev cmd/server/main.go

# Run development server
dev: proto
	@echo "🚀 Starting MIRADOR-CORE in development mode..."
	@echo "Make sure you have the VictoriaMetrics ecosystem running!"
	@echo "Run 'docker-compose up -d' to start dependencies."
	go run cmd/server/main.go

# Alias to dev
run: dev
	@true

# Clean and regenerate everything
clean-build: clean proto
	@echo "🧹 Clean build with fresh protobuf generation..."
	@go build -o bin/$(BINARY_NAME) cmd/server/main.go

# Run tests (generate proto first)
test: proto
	@echo "🧪 Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Update dependencies
vendor:
	@echo "📦 Updating dependencies..."
	go mod tidy
	go mod vendor

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf vendor/
	rm -f coverage.out
	# Remove generated protobuf files to force regeneration
	find internal/grpc/proto -name "*.pb.go" -delete
	find internal/grpc/proto -name "*_grpc.pb.go" -delete

# Force regenerate proto files
proto-clean:
	@echo "🗑️  Removing generated protobuf files..."
	find internal/grpc/proto -name "*.pb.go" -delete
	find internal/grpc/proto -name "*_grpc.pb.go" -delete
	@echo "🔧 Regenerating protobuf files..."
	@./scripts/generate-proto-code.sh

# Install development tools
tools:
	@echo "🛠️  Installing development tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

# Check if all tools are available
check-tools:
	@echo "🔍 Checking development tools..."
	@command -v protoc >/dev/null 2>&1 || { echo "❌ protoc is not installed"; exit 1; }
	@command -v protoc-gen-go >/dev/null 2>&1 || { echo "❌ protoc-gen-go is not installed. Run 'make tools'"; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "❌ protoc-gen-go-grpc is not installed. Run 'make tools'"; exit 1; }
	@echo "✅ All tools are available"

# Lint code
lint: proto
	@echo "🔍 Running linters..."
	golangci-lint run ./...

.PHONY: vuln
vuln:
	@echo "🛡️  Running govulncheck vulnerability scan..."
	@command -v govulncheck >/dev/null 2>&1 || { echo "Installing govulncheck..."; go install golang.org/x/vuln/cmd/govulncheck@latest; }
	govulncheck ./...

# Format code
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...
	goimports -w . 2>/dev/null || true

# Build Docker image
docker: docker-build ## Alias

docker-build: ## Build single-arch docker image (host arch)
	@echo "🐳 Building Docker image $(IMAGE):$(VERSION) ..."
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg BUILD_TIME=$(BUILD_TIME) \
	  --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	  -t $(IMAGE):$(VERSION) .
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest

docker-build-native: ## Build native-arch image via buildx and load to local Docker
	@echo "🐳 Building native image $(IMAGE):$(VERSION) for linux/$(HOST_ARCH) ..."
	docker buildx build --platform linux/$(HOST_ARCH) \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg BUILD_TIME=$(BUILD_TIME) \
	  --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	  -t $(IMAGE):$(VERSION) -t $(IMAGE):latest --load .
	
.PHONY: buildx-ensure
# Ensure a containerized buildx builder is active (required for OCI exporter and multi-arch)
buildx-ensure:
	@DRIVER=$$(docker buildx inspect 2>/dev/null | awk '/^Driver:/ {print $$2}'); \
	if [ -z "$$DRIVER" ] || [ "$$DRIVER" = "docker" ]; then \
	  echo "🔧 Initializing containerized buildx builder (miradorx) ..."; \
	  docker buildx create --use --name miradorx --driver docker-container >/dev/null 2>&1 || docker buildx use miradorx; \
	  docker buildx inspect --bootstrap >/dev/null 2>&1 || true; \
	fi

dockerx-build: buildx-ensure ## Build multi-arch image with buildx (no push)
	@echo "🐳 Building multi-arch image $(IMAGE):$(VERSION) for $(DOCKER_PLATFORMS) ..."
	@if echo "$(DOCKER_PLATFORMS)" | grep -q ","; then \
	  mkdir -p build; \
	  echo "➡️  Detected multiple platforms; exporting OCI archive (Docker --load cannot import manifest lists)"; \
	  if docker buildx build --platform $(DOCKER_PLATFORMS) \
	      --build-arg VERSION=$(VERSION) \
	      --build-arg BUILD_TIME=$(BUILD_TIME) \
	      --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	      -t $(IMAGE):$(VERSION) -t $(IMAGE):latest \
	      --output=type=oci,dest=build/$(IMAGE_NAME)-$(VERSION).oci .; then \
	    echo "✅ Wrote multi-arch OCI archive: build/$(IMAGE_NAME)-$(VERSION).oci"; \
	    echo "ℹ️  To publish a multi-arch manifest, run: make dockerx-push VERSION=$(VERSION)"; \
	  else \
	    echo "❌ Failed to export OCI archive. Ensure your buildx driver supports OCI exporter."; \
	    exit 1; \
	  fi; \
	else \
	  docker buildx build --platform $(DOCKER_PLATFORMS) \
	    --build-arg VERSION=$(VERSION) \
	    --build-arg BUILD_TIME=$(BUILD_TIME) \
	    --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	    -t $(IMAGE):$(VERSION) -t $(IMAGE):latest --load .; \
	fi

.PHONY: dockerx-build-local-multi
dockerx-build-local-multi: ## Build and load per-arch images locally (tags: -amd64, -arm64)
	@echo "🐳 Building local per-arch images (amd64, arm64) ..."
	docker buildx build --platform linux/amd64 \
	  --build-arg VERSION=$(VERSION) --build-arg BUILD_TIME=$(BUILD_TIME) --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	  -t $(IMAGE):$(VERSION)-amd64 --load .
	docker buildx build --platform linux/arm64 \
	  --build-arg VERSION=$(VERSION) --build-arg BUILD_TIME=$(BUILD_TIME) --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	  -t $(IMAGE):$(VERSION)-arm64 --load .
	@echo "✅ Built: $(IMAGE):$(VERSION)-amd64 and $(IMAGE):$(VERSION)-arm64"
	@echo "ℹ️  To publish a combined manifest: docker buildx imagetools create -t $(IMAGE):$(VERSION) $(IMAGE):$(VERSION)-amd64 $(IMAGE):$(VERSION)-arm64"

dockerx-push: buildx-ensure ## Build and push multi-arch image with buildx
	@echo "🐳 Building & pushing multi-arch image $(IMAGE):$(VERSION) for $(DOCKER_PLATFORMS) ..."
	docker buildx build --platform $(DOCKER_PLATFORMS) \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg BUILD_TIME=$(BUILD_TIME) \
	  --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	  -t $(IMAGE):$(VERSION) -t $(IMAGE):latest --push .

release: test dockerx-push ## Run tests then push multi-arch image

############################################################
# Versioning & Publishing (Human + CI driven)
############################################################

# Print human-driven version info (expects VERSION=vX.Y.Z)
version-human:
	@echo "Version: $(VERSION)"
	@echo "Major: $$(echo $(VERSION) | sed -E 's/^v?([0-9]+)\..*/\1/')"
	@echo "Minor: $$(echo $(VERSION) | sed -E 's/^v?[0-9]+\.([0-9]+).*/\1/')"
	@echo "Patch: $$(echo $(VERSION) | sed -E 's/^v?[0-9]+\.[0-9]+\.([0-9]+).*/\1/')"

# Compute CI-driven version if none provided
# Priority: CI_TAG -> PR build -> branch build
version-ci:
	@if [ -n "$(CI_TAG)" ]; then \
	  V="$(CI_TAG)"; \
	elif [ -n "$(PR_NUMBER)" ]; then \
	  V="0.0.0-pr.$(PR_NUMBER).$(SHA_SHORT)"; \
	else \
	  V="0.0.0-$(BRANCH).$(DATE_CALVER).$(SHA_SHORT)"; \
	fi; \
	echo $$V

# Tag git with release version (requires VERSION=vX.Y.Z)
tag-release:
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION=vX.Y.Z required"; exit 1; fi
	@git tag -a $(VERSION) -m "Release $(VERSION)" && git push origin $(VERSION)

# Push release image with semver fanout: vX.Y.Z, vX.Y, vX, latest, stable
docker-publish-release: ## VERSION=vX.Y.Z REGISTRY=... IMAGE_NAME=... make docker-publish-release
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION=vX.Y.Z required"; exit 1; fi
	@MAJOR=$$(echo $(VERSION) | sed -E 's/^v?([0-9]+)\..*/\1/'); \
	MINOR=$$(echo $(VERSION) | sed -E 's/^v?[0-9]+\.([0-9]+).*/\1/'); \
	PATCH=$$(echo $(VERSION) | sed -E 's/^v?[0-9]+\.[0-9]+\.([0-9]+).*/\1/'); \
	BASE=$(IMAGE); \
	echo "Publishing $$BASE:$(VERSION) $$BASE:v$$MAJOR.$$MINOR $$BASE:v$$MAJOR latest stable"; \
	docker buildx build --platform $(DOCKER_PLATFORMS) \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg BUILD_TIME=$(BUILD_TIME) \
	  --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	  -t $$BASE:$(VERSION) -t $$BASE:v$$MAJOR.$$MINOR -t $$BASE:v$$MAJOR -t $$BASE:latest -t $$BASE:stable \
	  --push .

# Push canary image: tags 0.0.0-<branch>.<date>.<sha> and canary
docker-publish-canary:
	@TAG=$$(make -s version-ci); \
	BASE=$(IMAGE); \
	echo "Publishing $$BASE:$$TAG and $$BASE:canary"; \
	docker buildx build --platform $(DOCKER_PLATFORMS) \
	  --build-arg VERSION=$$TAG \
	  --build-arg BUILD_TIME=$(BUILD_TIME) \
	  --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	  -t $$BASE:$$TAG -t $$BASE:canary --push .

# Push PR image: tags 0.0.0-pr.<PR#>.<sha> and pr-<PR#>
docker-publish-pr:
	@if [ -z "$(PR_NUMBER)" ]; then echo "ERROR: PR_NUMBER=<n> required"; exit 1; fi
	@TAG="0.0.0-pr.$(PR_NUMBER).$(SHA_SHORT)"; \
	BASE=$(IMAGE); \
	echo "Publishing $$BASE:$$TAG and $$BASE:pr-$(PR_NUMBER)"; \
	docker buildx build --platform $(DOCKER_PLATFORMS) \
	  --build-arg VERSION=$$TAG \
	  --build-arg BUILD_TIME=$(BUILD_TIME) \
	  --build-arg COMMIT_HASH=$(COMMIT_HASH) \
	  -t $$BASE:$$TAG -t $$BASE:pr-$(PR_NUMBER) --push .

# Bump Helm chart appVersion and/or chart version (requires yq or sed fallback)
helm-bump: ## VERSION=vX.Y.Z CHART_VER=0.1.1 make helm-bump
	@if [ -z "$(VERSION)" -a -z "$(CHART_VER)" ]; then echo "Nothing to bump (set VERSION and/or CHART_VER)"; exit 0; fi
	@if command -v yq >/dev/null 2>&1; then \
	  if [ -n "$(VERSION)" ]; then yq -i '.appVersion = "$(VERSION)"' chart/Chart.yaml; fi; \
	  if [ -n "$(CHART_VER)" ]; then yq -i '.version = "$(CHART_VER)"' chart/Chart.yaml; fi; \
	else \
	  if [ -n "$(VERSION)" ]; then sed -i'' -E 's#^(appVersion:).*$#\1 "$(VERSION)"#' chart/Chart.yaml; fi; \
	  if [ -n "$(CHART_VER)" ]; then sed -i'' -E 's#^(version:).*$#\1 $(CHART_VER)#' chart/Chart.yaml; fi; \
	fi

# Sync Valkey dependency version in Chart.yaml from values.yaml (requires yq)
helm-sync-deps:
	@if ! command -v yq >/dev/null 2>&1; then echo "ERROR: yq required for helm-sync-deps"; exit 1; fi
	@VALKEY_VER=$$(yq -r '.valkey.version' chart/values.yaml); \
	if [ -z "$$VALKEY_VER" -o "$$VALKEY_VER" = "null" ]; then echo "No valkey.version set in values.yaml"; exit 1; fi; \
	echo "Syncing Valkey dependency version to $$VALKEY_VER"; \
	yq -i '(.dependencies[] | select(.name=="valkey").version) = strenv(VALKEY_VER)' chart/Chart.yaml

helm-dep-update: helm-sync-deps
	@cd chart && helm dependency update

# Start local development stack (root docker-compose.yml)
dev-stack:
	@echo "🐳 Starting development stack..."
	docker-compose up -d
	@echo "⏳ Waiting for services to be ready..."
	@sleep 10
	@echo "✅ Development stack is ready!"
	@echo "VictoriaMetrics: http://localhost:8481"
	@echo "VictoriaLogs: http://localhost:9428" 
	@echo "VictoriaTraces: http://localhost:10428"
	@echo "Redis: localhost:6379"

# Stop local development stack
dev-stack-down:
	@echo "🛑 Stopping development stack..."
	docker-compose down

# Version information
version:
	@echo "MIRADOR-CORE Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Commit Hash: $(COMMIT_HASH)"
