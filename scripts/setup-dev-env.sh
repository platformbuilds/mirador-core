#!/bin/bash

echo "Setting up MIRADOR-CORE development environment..."

# Check prerequisites
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo "❌ $1 is required but not installed"
        exit 1
    else
        echo "✅ $1 is available"
    fi
}

echo "Checking prerequisites..."
check_command go
check_command docker
check_command kubectl
check_command protoc

# Install Go tools
echo "Installing Go development tools..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/swaggo/swag/cmd/swag@latest

# Setup local development dependencies
echo "Setting up local dependencies..."

# Start Redis cluster for Valkey caching
docker-compose up -d redis-cluster

# Start VictoriaMetrics ecosystem
docker-compose up -d victoriametrics victorialogs victoriatraces

# Wait for services to be ready
echo "Waiting for services to start..."
sleep 10

# Verify connections
echo "Verifying service connections..."
curl -f http://localhost:8481/health || echo "⚠️  VictoriaMetrics not ready"
curl -f http://localhost:9428/health || echo "⚠️  VictoriaLogs not ready"  
curl -f http://localhost:10428/health || echo "⚠️  VictoriaTraces not ready"

# Generate Protocol Buffers
echo "Generating Protocol Buffer code..."
make proto

# Install dependencies
echo "Installing Go dependencies..."
go mod download

echo "✅ Development environment setup complete!"
echo ""
echo "Next steps:"
echo "1. Run tests: make test"
echo "2. Start development server: make dev"
echo "3. View API docs: http://localhost:8080/api/openapi.json"
echo "4. Check health: curl http://localhost:8080/health"
