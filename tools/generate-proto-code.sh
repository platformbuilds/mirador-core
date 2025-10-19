#!/bin/bash

# Generate Protocol Buffer Go code for MIRADOR-CORE
# Compatible with Linux (Debian/RedHat) and macOS (Darwin)

set -euo pipefail

echo "🔧 Generating Protocol Buffer Go code for MIRADOR-CORE..."

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

# Ensure output directories exist
echo "📁 Creating output directories..."
mkdir -p internal/grpc/proto/predict
mkdir -p internal/grpc/proto/rca  
mkdir -p internal/grpc/proto/alert

# Generate from your existing proto files
echo "🔨 Generating Go code from existing proto files..."

# Generate PREDICT-ENGINE protobuf (from your existing proto/predict.proto or internal/grpc/proto/predict.proto)
if [ -f "internal/grpc/proto/predict.proto" ]; then
    echo "📊 Generating PREDICT-ENGINE protobuf..."
    protoc --go_out=. --go_opt=paths=source_relative \
           --go-grpc_out=. --go-grpc_opt=paths=source_relative \
           internal/grpc/proto/predict.proto
elif [ -f "proto/predict.proto" ]; then
    echo "📊 Generating PREDICT-ENGINE protobuf..."
    protoc --go_out=internal/grpc/proto/predict --go_opt=paths=source_relative \
           --go-grpc_out=internal/grpc/proto/predict --go-grpc_opt=paths=source_relative \
           proto/predict.proto
else
    echo "⚠️  predict.proto not found. Skipping PREDICT-ENGINE generation."
fi

# Generate RCA-ENGINE protobuf (from your existing proto/rca.proto)
if [ -f "internal/grpc/proto/rca.proto" ]; then
    echo "🔍 Generating RCA-ENGINE protobuf..."
    protoc --go_out=. --go_opt=paths=source_relative \
           --go-grpc_out=. --go-grpc_opt=paths=source_relative \
           internal/grpc/proto/rca.proto
elif [ -f "proto/rca.proto" ]; then
    echo "🔍 Generating RCA-ENGINE protobuf..."
    protoc --go_out=internal/grpc/proto/rca --go_opt=paths=source_relative \
           --go-grpc_out=internal/grpc/proto/rca --go-grpc_opt=paths=source_relative \
           proto/rca.proto
else
    echo "⚠️  rca.proto not found. Skipping RCA-ENGINE generation."
fi

# Generate ALERT-ENGINE protobuf (you may need to create this proto file)
if [ -f "internal/grpc/proto/alert.proto" ]; then
    echo "🚨 Generating ALERT-ENGINE protobuf..."
    protoc --go_out=. --go_opt=paths=source_relative \
           --go-grpc_out=. --go-grpc_opt=paths=source_relative \
           internal/grpc/proto/alert.proto
elif [ -f "proto/alert.proto" ]; then
    echo "🚨 Generating ALERT-ENGINE protobuf..."
    protoc --go_out=internal/grpc/proto/alert --go_opt=paths=source_relative \
           --go-grpc_out=internal/grpc/proto/alert --go-grpc_opt=paths=source_relative \
           proto/alert.proto
else
    echo "⚠️  alert.proto not found. Creating placeholder..."
    # Create a basic alert.proto if it doesn't exist
    cat > internal/grpc/proto/alert.proto << 'EOF'
syntax = "proto3";

package mirador.alert;

option go_package = "github.com/platformbuilds/mirador-core/internal/grpc/proto/alert";

service AlertEngineService {
  rpc ProcessAlert(ProcessAlertRequest) returns (ProcessAlertResponse);
  rpc GetAlertRules(GetAlertRulesRequest) returns (GetAlertRulesResponse);
  rpc GetHealth(GetHealthRequest) returns (GetHealthResponse);
}

message ProcessAlertRequest {
  Alert alert = 1;
  string tenant_id = 2;
}

message ProcessAlertResponse {
  ProcessedAlert processed_alert = 1;
}

message Alert {
  string id = 1;
  string severity = 2;
  string component = 3;
  string message = 4;
  int64 timestamp = 5;
  map<string, string> labels = 6;
  map<string, string> annotations = 7;
}

message ProcessedAlert {
  string id = 1;
  string action = 2;
  string cluster_id = 3;
  string escalation = 4;
  repeated string notifications = 5;
}

message GetAlertRulesRequest {
  string tenant_id = 1;
}

message GetAlertRulesResponse {
  repeated AlertRule rules = 1;
}

message AlertRule {
  string id = 1;
  string name = 2;
  string query = 3;
  string condition = 4;
  string severity = 5;
  bool enabled = 6;
  map<string, string> labels = 7;
  map<string, string> annotations = 8;
}

message GetHealthRequest {}

message GetHealthResponse {
  string status = 1;
  int32 active_alerts = 2;
  int32 rules_count = 3;
  string last_update = 4;
}
EOF

    echo "🔨 Generating ALERT-ENGINE protobuf from created file..."
    protoc --go_out=. --go_opt=paths=source_relative \
           --go-grpc_out=. --go-grpc_opt=paths=source_relative \
           internal/grpc/proto/alert.proto
fi

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
