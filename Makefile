.PHONY: build test clean docker proto vendor lint run dev setup

# Variables
BINARY_NAME=mirador-core
VERSION=v2.1.3
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT_HASH=$(shell git rev-parse --short HEAD)
LDFLAGS=-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commitHash=$(COMMIT_HASH)

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
build: proto
	@echo "🔨 Building MIRADOR-CORE..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="$(LDFLAGS)" \
		-o bin/$(BINARY_NAME) \
		cmd/server/main.go

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
docker: build
	@echo "🐳 Building Docker image..."
	docker build -t platformbuilds/mirador-core:$(VERSION) .
	docker tag platformbuilds/mirador-core:$(VERSION) platformbuilds/mirador-core:latest

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