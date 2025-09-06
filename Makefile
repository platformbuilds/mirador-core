.PHONY: build build-linux-amd64 build-linux-arm64 build-darwin-arm64 build-windows-amd64 build-all \
	docker docker-build dockerx-build dockerx-push docker-publish-release docker-publish-canary docker-publish-pr \
	release test clean proto vendor lint run dev setup tools check-tools dev-stack dev-stack-down fmt version proto-clean clean-build \
	tag-release helm-bump version-human version-ci

# Variables
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

# CI/environment metadata
BRANCH?=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "local")
SHA_SHORT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE_YYYYMMDD:=$(shell date -u +%Y.%m.%d)
DATE_CALVER:=$(shell date -u +%Y.%m.%d)
PR_NUMBER?=
CI_TAG?=

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
build: proto ## Release-style static build for Linux/amd64
	@echo "🔨 Building MIRADOR-CORE (linux/amd64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="$(LDFLAGS)" \
		-o bin/$(BINARY_NAME) \
		cmd/server/main.go

build-linux-amd64: proto ## Build linux/amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-linux-amd64 cmd/server/main.go

build-linux-arm64: proto ## Build linux/arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-linux-arm64 cmd/server/main.go

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

# Format code
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...
	goimports -w . 2>/dev/null || true

# Build Docker image
docker: docker-build ## Alias

docker-build: ## Build single-arch docker image (host arch)
	@echo "🐳 Building Docker image $(IMAGE):$(VERSION) ..."
	docker build -t $(IMAGE):$(VERSION) .
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest

dockerx-build: ## Build multi-arch image with buildx (no push)
	@echo "🐳 Building multi-arch image $(IMAGE):$(VERSION) for $(DOCKER_PLATFORMS) ..."
	docker buildx build --platform $(DOCKER_PLATFORMS) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest --load .

dockerx-push: ## Build and push multi-arch image with buildx
	@echo "🐳 Building & pushing multi-arch image $(IMAGE):$(VERSION) for $(DOCKER_PLATFORMS) ..."
	docker buildx build --platform $(DOCKER_PLATFORMS) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest --push .

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
	  -t $$BASE:$(VERSION) -t $$BASE:v$$MAJOR.$$MINOR -t $$BASE:v$$MAJOR -t $$BASE:latest -t $$BASE:stable \
	  --push .

# Push canary image: tags 0.0.0-<branch>.<date>.<sha> and canary
docker-publish-canary:
	@TAG=$$(make -s version-ci); \
	BASE=$(IMAGE); \
	echo "Publishing $$BASE:$$TAG and $$BASE:canary"; \
	docker buildx build --platform $(DOCKER_PLATFORMS) -t $$BASE:$$TAG -t $$BASE:canary --push .

# Push PR image: tags 0.0.0-pr.<PR#>.<sha> and pr-<PR#>
docker-publish-pr:
	@if [ -z "$(PR_NUMBER)" ]; then echo "ERROR: PR_NUMBER=<n> required"; exit 1; fi
	@TAG="0.0.0-pr.$(PR_NUMBER).$(SHA_SHORT)"; \
	BASE=$(IMAGE); \
	echo "Publishing $$BASE:$$TAG and $$BASE:pr-$(PR_NUMBER)"; \
	docker buildx build --platform $(DOCKER_PLATFORMS) -t $$BASE:$$TAG -t $$BASE:pr-$(PR_NUMBER) --push .

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
.PHONY: helm-sync-deps
helm-sync-deps:
	@if ! command -v yq >/dev/null 2>&1; then echo "ERROR: yq required for helm-sync-deps"; exit 1; fi
	@VALKEY_VER=$$(yq -r '.valkey.version' chart/values.yaml); \
	if [ -z "$$VALKEY_VER" -o "$$VALKEY_VER" = "null" ]; then echo "No valkey.version set in values.yaml"; exit 1; fi; \
	echo "Syncing Valkey dependency version to $$VALKEY_VER"; \
	yq -i '(.dependencies[] | select(.name=="valkey").version) = strenv(VALKEY_VER)' chart/Chart.yaml

.PHONY: helm-dep-update
helm-dep-update: helm-sync-deps
	@cd chart && helm dependency update

# Start local development stack
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

# Full development setup
dev-setup: check-tools tools proto dev-stack
	@echo "🎉 Complete development environment ready!"
	@echo "Run 'make dev' to start MIRADOR-CORE"

# Version information
version:
	@echo "MIRADOR-CORE Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Commit Hash: $(COMMIT_HASH)"
