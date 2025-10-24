#!/bin/bash

# Generate Protocol Buffer Go code for MIRADOR-CORE
# Compatible with Linux (Debian/RedHat) and macOS (Darwin)

set -euo pipefail

echo "�� Generating Protocol Buffer Go code for MIRADOR-CORE..."

# Platform detection
OS_TYPE="$(uname -s)"
case "$OS_TYPE" in
    Darwin*)    OS_NAME="macOS" ;;
    Linux*)     OS_NAME="Linux" ;;
    *)          OS_NAME="Unknown" ;;
esac

# Check if protoc is installed with cross-platform instructions
if ! command -v protoc >/dev/null 2>&1; then
    echo "❌ protoc is required but not installed."
    echo "Install it with:"
    case "$OS_NAME" in
        "macOS")
            echo "  brew install protobuf"
            ;;
        "Linux")
            if command -v apt-get >/dev/null 2>&1; then
                echo "  sudo apt-get update && sudo apt-get install protobuf-compiler"
            elif command -v yum >/dev/null 2>&1; then
                echo "  sudo yum install protobuf-compiler"
            elif command -v dnf >/dev/null 2>&1; then
                echo "  sudo dnf install protobuf-compiler"
            else
                echo "  Use your package manager to install protobuf-compiler"
            fi
            ;;
        *)
            echo "  Download from: https://github.com/protocolbuffers/protobuf/releases"
            ;;
    esac
    exit 1
fi

# Install required Go plugins
echo "📦 Installing protoc Go plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate from your existing proto files
echo "🔨 Generating Go code from existing proto files..."

# Generate protobuf files
generate_proto() {
    local proto_file="$1"
    local service_name="$2"

    if [ -f "$proto_file" ]; then
        echo "⚙️  Generating $service_name protobuf..."
        protoc --go_out=. --go_opt=paths=source_relative \
               --go-grpc_out=. --go-grpc_opt=paths=source_relative \
               "$proto_file"
    fi
}

# Generate all protobufs
generate_proto "internal/grpc/proto/alert/alert.proto" "ALERT-ENGINE"
generate_proto "internal/grpc/proto/predict/predict.proto" "PREDICT-ENGINE"
generate_proto "internal/grpc/proto/rca/rca.proto" "RCA-ENGINE"

echo "✅ Protocol Buffer code generation completed!"

# Verify generated files
echo "📋 Generated files:"
find internal/grpc/proto -name "*.pb.go" -type f | sort

echo ""
echo "🎉 You can now run 'go mod tidy' successfully!"
echo ""
echo "Next steps:"
echo "1. Run: go mod tidy"
echo "2. Run: go build"
echo "3. Check the generated .pb.go files for any needed adjustments"
