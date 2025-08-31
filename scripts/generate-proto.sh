#!/bin/bash

# Generate Protocol Buffer code for MIRADOR-CORE
# This script generates Go code from .proto files

set -e

echo "Generating Protocol Buffer code for MIRADOR-CORE..."

# Ensure protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "protoc is required but not installed. Please install Protocol Buffers compiler."
    exit 1
fi

# Install Go plugins for protoc
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Create output directories
mkdir -p internal/grpc/proto/{predict,rca,alert}

# Generate Predict Engine proto
echo "Generating PREDICT-ENGINE protobuf..."
protoc --go_out=internal/grpc/proto/predict \
       --go_opt=paths=source_relative \
       --go-grpc_out=internal/grpc/proto/predict \
       --go-grpc_opt=paths=source_relative \
       proto/predict.proto

# Generate RCA Engine proto  
echo "Generating RCA-ENGINE protobuf..."
protoc --go_out=internal/grpc/proto/rca \
       --go_opt=paths=source_relative \
       --go-grpc_out=internal/grpc/proto/rca \
       --go-grpc_opt=paths=source_relative \
       proto/rca.proto

# Generate Alert Engine proto
echo "Generating ALERT-ENGINE protobuf..."
protoc --go_out=internal/grpc/proto/alert \
       --go_opt=paths=source_relative \
       --go-grpc_out=internal/grpc/proto/alert \
       --go-grpc_opt=paths=source_relative \
       proto/alert.proto

echo "Protocol Buffer code generation completed successfully!"

# Verify generated files
echo "Generated files:"
find internal/grpc/proto -name "*.pb.go" -type f
