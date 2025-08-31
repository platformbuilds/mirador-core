.PHONY: build test clean docker proto vendor lint run dev

# Variables
BINARY_NAME=mirador-core
VERSION=v2.1.3
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT_HASH=$(shell git rev-parse --short HEAD)
LDFLAGS=-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commitHash=$(COMMIT_HASH)

# Build
build:
	@echo "Building MIRADOR-CORE..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="$(LDFLAGS)" \
		-o bin/$(BINARY_NAME) \
		cmd/server/main.go

# Development build (with debug symbols)
dev-build:
	@echo "Building MIRADOR-CORE for development..."
	go build -o bin/$(BINARY_NAME)-dev cmd/server/main.go

# Run development server
dev:
	@echo "Starting MIRADOR-CORE in development mode..."
	go run cmd/server/main.go

# Generate Protocol Buffers
proto:
	@echo "Generating Protocol Buffer code..."
	./scripts/generate-proto.sh

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./tests/integration/...

# Benchmark tests
benchmark:
	@echo "Running benchmark tests..."
	go test -v -bench=. -benchmem ./tests/benchmark/...

# Lint code
lint:
	@echo "Running linters..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Update dependencies
vendor:
	@echo "Updating dependencies..."
	go mod tidy
	go mod vendor

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t mirador/core:$(VERSION) .
	docker tag mirador/core:$(VERSION) mirador/core:latest

# Push Docker image
docker-push:
	@echo "Pushing Docker image..."
	docker push mirador/core:$(VERSION)
	docker push mirador/core:latest

# Deploy to development environment
deploy-dev:
	@echo "Deploying to development..."
	kubectl apply -f deployments/k8s/namespace.yaml
	kubectl apply -f deployments/k8s/configmap.yaml
	kubectl apply -f deployments/k8s/deployment.yaml
	kubectl apply -f deployments/k8s/service.yaml

# Deploy to production
deploy-prod:
	@echo "Deploying to production..."
	kubectl apply -f deployments/k8s/production/

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf vendor/
	rm -f coverage.out

# Generate API documentation
docs:
	@echo "Generating API documentation..."
	swag init -g cmd/server/main.go -o api/swagger

# Database migrations (for storing user preferences, etc.)
migrate-up:
	@echo "Running database migrations..."
	migrate -path migrations -database postgres://user:pass@localhost/mirador up

migrate-down:
	@echo "Rolling back database migrations..."
	migrate -path migrations -database postgres://user:pass@localhost/mirador down

# Load test
load-test:
	@echo "Running load tests..."
	k6 run tests/load/api-load-test.js

# Security scan
security-scan:
	@echo "Running security scan..."
	gosec ./...
	govulncheck ./...

# Full CI pipeline
ci: lint test test-integration benchmark security-scan build

# Local development setup
setup-dev:
	@echo "Setting up development environment..."
	go mod download
	./scripts/setup-dev-env.sh

# Version information
version:
	@echo "MIRADOR-CORE Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Commit Hash: $(COMMIT_HASH)"
