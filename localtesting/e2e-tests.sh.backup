#!/bin/bash

# Comprehensive E2E API Tests for Mirador Core
# Compatible with Linux (Debian/RedHat) and macOS (Darwin)

set -euo pipefail

# Configuration
BASE_URL=${BASE_URL:-"http://localhost:8010"}
API_BASE="$BASE_URL/api/v1"
TENANT_ID=${TENANT_ID:-"platformbuilds"}  # Default bootstrap tenant
VERBOSE=${VERBOSE:-false}
RESULTS_FILE="e2e-test-results.json"
FAILURES_TABLE_FILE="test-failures-table.md"
RUN_CODE_TESTS=${RUN_CODE_TESTS:-true}

# Authentication
AUTH_TOKEN=""
ADMIN_USERNAME="aarvee"
ADMIN_PASSWORD="password123"

# Authenticate and get JWT token
authenticate() {
    log_info "Authenticating as admin user..."
    
    local login_data='{
        "username": "'$ADMIN_USERNAME'",
        "password": "'$ADMIN_PASSWORD'"
    }'
    
    local response
    response=$(curl -s -X POST "$API_BASE/auth/login" \
        -H "Content-Type: application/json" \
        -H "x-tenant-id: $TENANT_ID" \
        -d "$login_data")
    
    if echo "$response" | jq -e '.status == "success"' >/dev/null 2>&1; then
        AUTH_TOKEN=$(echo "$response" | jq -r '.data.jwt_token')
        log_success "Authentication successful, got JWT token"
        return 0
    else
        log_error "Authentication failed: $response"
        return 1
    fi
}

# Platform detection for cross-platform compatibility
OS_TYPE="$(uname -s)"
case "$OS_TYPE" in
    Darwin*)    OS_NAME="macOS" ;;
    Linux*)     OS_NAME="Linux" ;;
    *)          OS_NAME="Unknown" ;;
esac

# Cross-platform date and timestamp functions
get_timestamp_ms() {
    # Get current timestamp in milliseconds
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import time; print(int(time.time() * 1000))"
    elif command -v node >/dev/null 2>&1; then
        node -e "console.log(Date.now())"
    elif [[ "$OS_NAME" == "macOS" ]]; then
        # macOS date command
        echo $(($(date +%s) * 1000))
    else
        # Linux date command
        date +%s%3N
    fi
}

get_iso_timestamp() {
    # Get ISO timestamp for queries
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "from datetime import datetime; print(datetime.utcnow().isoformat() + 'Z')"
    elif [[ "$OS_NAME" == "macOS" ]]; then
        date -u +%Y-%m-%dT%H:%M:%SZ
    else
        date -u +%Y-%m-%dT%H:%M:%SZ
    fi
}

get_iso_timestamp_offset() {
    # Get ISO timestamp with offset (for range queries)
    local offset_hours="$1"
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "from datetime import datetime, timedelta; print((datetime.utcnow() - timedelta(hours=$offset_hours)).isoformat() + 'Z')"
    elif [[ "$OS_NAME" == "macOS" ]]; then
        date -u -v-${offset_hours}H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ
    else
        date -u -d "${offset_hours} hours ago" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ
    fi
}

get_unix_timestamp() {
    # Get unix timestamp for logs/traces
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import time; print(int(time.time()))"
    else
        date +%s
    fi
}

get_unix_timestamp_offset() {
    # Get unix timestamp with offset
    local offset_hours="$1"
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import time; print(int(time.time() - ($offset_hours * 3600)))"
    elif [[ "$OS_NAME" == "macOS" ]]; then
        echo $(($(date +%s) - ($offset_hours * 3600)))
    else
        date -d "${offset_hours} hours ago" +%s 2>/dev/null || echo $(($(date +%s) - ($offset_hours * 3600)))
    fi
}

# Check required tools
check_dependencies() {
    local missing_tools=()
    
    # Check essential tools
    command -v curl >/dev/null 2>&1 || missing_tools+=("curl")
    command -v jq >/dev/null 2>&1 || missing_tools+=("jq")
    
    # Check for at least one JSON/time tool
    if ! command -v python3 >/dev/null 2>&1 && ! command -v node >/dev/null 2>&1; then
        log_warning "Neither python3 nor node found. Date operations may be limited."
    fi
    
    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        echo "Install them with:"
        case "$OS_NAME" in
            "macOS")
                echo "  brew install curl jq"
                ;;
            "Linux")
                if command -v apt-get >/dev/null 2>&1; then
                    echo "  sudo apt-get update && sudo apt-get install curl jq"
                elif command -v yum >/dev/null 2>&1; then
                    echo "  sudo yum install curl jq"
                elif command -v dnf >/dev/null 2>&1; then
                    echo "  sudo dnf install curl jq"
                else
                    echo "  Use your package manager to install curl and jq"
                fi
                ;;
        esac
        exit 1
    fi
}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
CODE_TESTS_PASSED=0
CODE_TESTS_FAILED=0

# JSON results array and failures tracking
echo "[]" > "$RESULTS_FILE"
FAILED_TESTS_DATA=()

# Test phase tracking
CURRENT_PHASE=""

# Helper functions
log() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" >&2
    fi
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_phase() {
    CURRENT_PHASE="$1"
    echo -e "${PURPLE}🔄${NC} ${CYAN}$1${NC}"
}

# Track failed tests for detailed reporting
add_failure_data() {
    local test_name="$1"
    local endpoint="$2"
    local expected_status="$3"
    local actual_status="$4"
    local error_message="$5"
    local suggested_fix="$6"
    
    FAILED_TESTS_DATA+=("$test_name|$endpoint|$expected_status|$actual_status|$error_message|$suggested_fix")
}

# Function to add test result to JSON
add_test_result() {
    local name="$1"
    local method="$2"
    local url="$3"
    local status="$4"
    local success="$5"
    local response_time="$6"
    local error_msg="${7:-}"
    
    local result=$(cat <<EOF
{
    "name": "$name",
    "method": "$method", 
    "url": "$url",
    "status": $status,
    "success": $success,
    "response_time_ms": $response_time,
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "error": "$error_msg"
}
EOF
)
    
    # Add to results file
    jq ". += [$result]" "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"
}

# Function to perform HTTP request with curl
http_request() {
    local method="$1"
    local url="$2"
    local expected_status="${3:-200}"
    local data="${4:-}"
    local test_name="${5:-$method $url}"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    log "Testing: $test_name"
    log "Request: $method $url"
    
    local start_time=$(get_timestamp_ms)
    local response
    local status_code
    local success=false
    local error_msg=""
    local suggested_fix=""
    
    # Build curl command
    local curl_cmd=(curl -s -w "%{http_code}" -X "$method" "$url")
    
    # Add headers
    curl_cmd+=(-H "Accept: application/json")
    curl_cmd+=(-H "Content-Type: application/json")
    curl_cmd+=(-H "x-tenant-id: $TENANT_ID")
    
    # Add authentication header if we have a token
    if [[ -n "$AUTH_TOKEN" ]]; then
        curl_cmd+=(-H "Authorization: Bearer $AUTH_TOKEN")
    else
        # Fallback to dummy token for public endpoints
        curl_cmd+=(-H "Authorization: Bearer test-token")
    fi
    
    # Add data for POST/PUT requests
    if [[ -n "$data" && ("$method" == "POST" || "$method" == "PUT") ]]; then
        curl_cmd+=(-d "$data")
    fi
    
    # Execute request and capture response
    if response=$("${curl_cmd[@]}" 2>&1); then
        # Extract status code (last 3 characters)
        status_code="${response: -3}"
        # Remove status code from response body
        response="${response%???}"
        
        local end_time=$(get_timestamp_ms)
        local response_time=$((end_time - start_time))
        
        # Check if status matches expected
        if [[ "$status_code" == "$expected_status" ]]; then
            success=true
            PASSED_TESTS=$((PASSED_TESTS + 1))
            log_success "$test_name - Status: $status_code (${response_time}ms)"
            
            # Try to pretty print JSON response if verbose
            if [[ "$VERBOSE" == "true" ]]; then
                if echo "$response" | jq . >/dev/null 2>&1; then
                    log "Response: $(echo "$response" | jq -c .)"
                else
                    log "Response: $response"
                fi
            fi
        else
            FAILED_TESTS=$((FAILED_TESTS + 1))
            error_msg="Expected status $expected_status, got $status_code"
            
            # Generate suggested fix based on status code and endpoint
            suggested_fix=$(generate_suggested_fix "$status_code" "$url" "$method")
            
            log_error "$test_name - $error_msg"
            log "Response: $response"
            
            # Track failure for detailed reporting
            add_failure_data "$test_name" "$url" "$expected_status" "$status_code" "$error_msg" "$suggested_fix"
        fi
        
        add_test_result "$test_name" "$method" "$url" "$status_code" "$success" "$response_time" "$error_msg"
    else
        local end_time=$(get_timestamp_ms)
        local response_time=$((end_time - start_time))
        FAILED_TESTS=$((FAILED_TESTS + 1))
        error_msg="Request failed: $response"
        suggested_fix="Check network connectivity and service availability"
        
        log_error "$test_name - $error_msg"
        add_failure_data "$test_name" "$url" "$expected_status" "0" "$error_msg" "$suggested_fix"
        add_test_result "$test_name" "$method" "$url" "0" "false" "$response_time" "$error_msg"
    fi
}

# Generate suggested fixes based on error patterns
generate_suggested_fix() {
    local status_code="$1"
    local url="$2"
    local method="$3"
    
    case "$status_code" in
        400)
            if [[ "$url" == *"/series"* ]]; then
                echo "Add match[] parameter: ?match[]=up"
            elif [[ "$url" == *"/traces/search"* ]]; then
                echo "Provide required query parameters (start/end or _time in query)"
            else
                echo "Check request format and required parameters"
            fi
            ;;
        404)
            if [[ "$url" == *"/schema/"* ]]; then
                echo "Schema endpoints may be disabled. Check feature flags or enable schema store"
            elif [[ "$method" == "DELETE" ]]; then
                echo "Endpoint may not support DELETE. Check OpenAPI spec for allowed methods"
            else
                echo "Endpoint not found. Verify API version and endpoint path"
            fi
            ;;
        405)
            echo "Method not allowed. Check OpenAPI spec for supported HTTP methods"
            ;;
        500)
            if [[ "$url" == *"/predict/"* ]]; then
                echo "Predict engine not running. Start predict microservice or disable predict tests"
            else
                echo "Internal server error. Check service logs and microservice dependencies"
            fi
            ;;
        503)
            if [[ "$url" == *"/predict/"* ]]; then
                echo "Predict engine unhealthy. Check predict microservice status and dependencies"
            else
                echo "Service unavailable. Check service health and dependencies"
            fi
            ;;
        *)
            echo "Unexpected status code. Check service logs and API documentation"
            ;;
    esac
}

# Wait for service to be ready
wait_for_service() {
    log_info "Waiting for service to be ready..."
    local max_attempts=5
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s "$BASE_URL/ready" >/dev/null 2>&1; then
            log_success "Service is ready!"
            return 0
        fi
        log "Attempt $attempt/$max_attempts - Service not ready, waiting..."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    # If service is still not ready, try to start infrastructure automatically
    log_warning "Service failed to become ready after $max_attempts attempts. Attempting to start infrastructure..."
    
    # Run make localdev-up
    log_info "Running 'make localdev-up' to start infrastructure..."
    if make localdev-up; then
        log_success "Infrastructure started successfully"
    else
        log_error "Failed to start infrastructure with 'make localdev-up'"
        exit 1
    fi
    
    # Run make localdev-seed-otel
    log_info "Running 'make localdev-seed-otel' to seed OTEL data..."
    if make localdev-seed-otel; then
        log_success "OTEL data seeded successfully"
    else
        log_error "Failed to seed OTEL data with 'make localdev-seed-otel'"
        exit 1
    fi
    
    # Wait again for service to be ready after starting infrastructure
    log_info "Waiting for service to be ready after infrastructure startup..."
    attempt=1
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s "$BASE_URL/ready" >/dev/null 2>&1; then
            log_success "Service is ready!"
            return 0
        fi
        log "Attempt $attempt/$max_attempts - Service not ready, waiting..."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    log_error "Service failed to become ready even after starting infrastructure"
    exit 1
}

# Test functions organized by category

# Health & Status Tests
test_health_endpoints() {
    log_info "Testing Health & Status Endpoints..."
    
    http_request "GET" "$BASE_URL/health" "200" "" "Health Check"
    http_request "GET" "$API_BASE/health" "200" "" "API Health Check"
    http_request "GET" "$API_BASE/ready" "200" "" "Readiness Check"
    http_request "GET" "$API_BASE/microservices/status" "200" "" "Microservices Status"
}

# Metrics API Tests
test_metrics_endpoints() {
    log_info "Testing Metrics Endpoints..."
    
    # Basic metrics endpoints
    http_request "GET" "$API_BASE/metrics/names" "200" "" "Get Metric Names"
    http_request "GET" "$API_BASE/metrics/names?limit=10" "200" "" "Get Metric Names with Limit"
    http_request "GET" "$API_BASE/label/__name__/values" "200" "" "Get __name__ Label Values"
    http_request "GET" "$API_BASE/series?match[]=up" "200" "" "Get Series"
    
    # Metrics query endpoints
    local current_time=$(get_iso_timestamp)
    local query_data='{"query": "up", "time": "'$current_time'"}'
    http_request "POST" "$API_BASE/metrics/query" "200" "$query_data" "Execute Instant Query"
    
    local start_time=$(get_iso_timestamp_offset 1)
    local end_time=$(get_iso_timestamp)
    local range_query_data='{"query": "up", "start": "'$start_time'", "end": "'$end_time'", "step": "60s"}'
    http_request "POST" "$API_BASE/metrics/query_range" "200" "$range_query_data" "Execute Range Query"
    
    # Labels endpoint (requires JSON payload)
    local labels_data='{"metric": "up"}'
    http_request "POST" "$API_BASE/metrics/labels" "200" "$labels_data" "Get Labels for Metric"
}

# Metrics Function Tests (Aggregate, Transform, Rollup)
test_metrics_functions() {
    log_info "Testing Metrics Function Endpoints..."
    
    # Aggregate functions
    local agg_functions=("sum" "avg" "count" "min" "max" "median" "quantile")
    for func in "${agg_functions[@]}"; do
        local func_data='{"query": "'$func'(up)"}'
        http_request "POST" "$API_BASE/metrics/query/aggregate/$func" "200" "$func_data" "Aggregate Function: $func"
    done
    
    # Transform functions  
    local transform_functions=("abs" "ceil" "floor" "round" "sqrt")
    for func in "${transform_functions[@]}"; do
        local func_data='{"query": "'$func'(up)"}'
        http_request "POST" "$API_BASE/metrics/query/transform/$func" "200" "$func_data" "Transform Function: $func"
    done
    
    # Rollup functions
    local rollup_functions=("rate" "increase" "delta" "avg_over_time" "max_over_time")
    for func in "${rollup_functions[@]}"; do
        local func_data='{"query": "'$func'(up[5m])"}'
        http_request "POST" "$API_BASE/metrics/query/rollup/$func" "200" "$func_data" "Rollup Function: $func"
    done
}

# Logs API Tests
test_logs_endpoints() {
    log_info "Testing Logs Endpoints..."
    
    http_request "GET" "$API_BASE/logs/fields" "200" "" "Get Log Fields"
    http_request "GET" "$API_BASE/logs/streams" "200" "" "Get Log Streams"
    
    # Logs query
    local start_timestamp=$(get_unix_timestamp_offset 1)
    local end_timestamp=$(get_unix_timestamp)
    local logs_query='{"query": "*", "start": '$start_timestamp', "end": '$end_timestamp', "limit": 100}'
    http_request "POST" "$API_BASE/logs/query" "200" "$logs_query" "Execute Logs Query"
    
    # Logs search
    http_request "POST" "$API_BASE/logs/search" "200" "$logs_query" "Search Logs"
    
    # Log export  
    http_request "POST" "$API_BASE/logs/export" "200" "$logs_query" "Export Logs"
}

# Traces API Tests
test_traces_endpoints() {
    log_info "Testing Traces Endpoints..."
    
    http_request "GET" "$API_BASE/traces/services" "200" "" "Get Trace Services"
    
    # Traces search
    local start_timestamp=$(get_unix_timestamp_offset 1)
    local end_timestamp=$(get_unix_timestamp)
    local trace_query='{"start": '$start_timestamp', "end": '$end_timestamp', "limit": 100}'
    http_request "POST" "$API_BASE/traces/search" "200" "$trace_query" "Search Traces"
    
    # Flame graph search
    http_request "POST" "$API_BASE/traces/flamegraph/search" "200" "$trace_query" "Search Flame Graphs"
}

# Unified Query Tests
test_unified_query_endpoints() {
    log_info "Testing Unified Query Endpoints..."
    
    # Unified search across all telemetry data
    local start_timestamp=$(get_unix_timestamp_offset 1)
    local end_timestamp=$(get_unix_timestamp)
    local unified_query='{"query": "*", "start": '$start_timestamp', "end": '$end_timestamp', "limit": 50}'
    http_request "POST" "$API_BASE/unified/search" "200" "$unified_query" "Unified Search"
    
    # Unified query with filters
    local filtered_query='{"query": "error", "filters": {"service": "api", "level": "ERROR"}, "start": '$start_timestamp', "end": '$end_timestamp', "limit": 25}'
    http_request "POST" "$API_BASE/unified/query" "200" "$filtered_query" "Unified Query with Filters"
    
    # Get unified statistics
    http_request "GET" "$API_BASE/unified/stats" "200" "" "Unified Statistics"
}

# RCA (Root Cause Analysis) Tests
test_rca_endpoints() {
    log_info "Testing RCA Endpoints..."
    
    http_request "GET" "$API_BASE/rca/correlations" "200" "" "Get RCA Correlations"
    http_request "GET" "$API_BASE/rca/patterns" "200" "" "Get Failure Patterns"
    
    # Service graph
    local start_timestamp=$(get_unix_timestamp_offset 1)
    local end_timestamp=$(get_unix_timestamp)
    local service_graph_data='{"services": ["web", "api"], "start": '$start_timestamp', "end": '$end_timestamp'}'
    http_request "POST" "$API_BASE/rca/service-graph" "200" "$service_graph_data" "Get Service Graph"
}

# Predict API Tests  
test_predict_endpoints() {
    log_info "Testing Predict Endpoints..."
    
    http_request "GET" "$API_BASE/predict/health" "200" "" "Predict Engine Health"
    http_request "GET" "$API_BASE/predict/models" "200" "" "Get Active Models"
    http_request "GET" "$API_BASE/predict/fractures" "200" "" "Get Predicted Fractures"
}

# Configuration API Tests
test_config_endpoints() {
    log_info "Testing Configuration Endpoints..."
    
    http_request "GET" "$API_BASE/config/datasources" "200" "" "Get Data Sources"
    http_request "GET" "$API_BASE/config/integrations" "200" "" "Get Integrations"
}

# Schema API Tests
test_schema_endpoints() {
    log_info "Testing Schema Endpoints..."

    # Test sample CSV downloads (these work and validate our camelCase headers)
    http_request "GET" "$API_BASE/schema/metrics/bulk/sample" "200" "" "Download Metrics Sample CSV"
    http_request "GET" "$API_BASE/schema/logs/fields/bulk/sample" "200" "" "Download Log Fields Sample CSV"
    http_request "GET" "$API_BASE/schema/traces/services/bulk/sample" "200" "" "Download Trace Services Sample CSV"
    http_request "GET" "$API_BASE/schema/labels/bulk/sample" "200" "" "Download Labels Sample CSV"

    # Test individual schema operations (create sample data)
    local timestamp=$(get_unix_timestamp)
    local metric_data='{"metric": "e2e_metric_'$timestamp'", "description": "E2E test metric", "author": "test"}'
    http_request "POST" "$API_BASE/schema/metrics" "200" "$metric_data" "Create Metric Schema"

    local log_field_data='{"field": "e2e_field_'$timestamp'", "type": "string", "description": "E2E test field", "author": "test"}'
    http_request "POST" "$API_BASE/schema/logs/fields" "200" "$log_field_data" "Create Log Field Schema"

    local trace_service_data='{"service": "e2e_service_'$timestamp'", "purpose": "E2E test service", "author": "test"}'
    http_request "POST" "$API_BASE/schema/traces/services" "200" "$trace_service_data" "Create Trace Service Schema"

    local trace_operation_data='{"service": "e2e_service_'$timestamp'", "operation": "e2e_operation_'$timestamp'", "purpose": "E2E test operation", "author": "test"}'
    http_request "POST" "$API_BASE/schema/traces/operations" "200" "$trace_operation_data" "Create Trace Operation Schema"

    # Test retrieving individual schema items
    http_request "GET" "$API_BASE/schema/metrics/e2e_metric_$timestamp" "200" "" "Get Metric Schema"
    http_request "GET" "$API_BASE/schema/logs/fields/e2e_field_$timestamp" "200" "" "Get Log Field Schema"
    http_request "GET" "$API_BASE/schema/traces/services/e2e_service_$timestamp" "200" "" "Get Trace Service Schema"
}

# Session Management Tests
test_session_endpoints() {
    log_info "Testing Session Management Endpoints..."
    
    http_request "GET" "$API_BASE/sessions/active" "200" "" "Get Active Sessions"
}

# RBAC Tests
test_rbac_endpoints() {
    log_info "Testing RBAC Endpoints..."
    
    http_request "GET" "$API_BASE/rbac/roles" "200" "" "Get RBAC Roles"
}

# Tenant-User Association Tests (Phase 2: Tenant Management & Isolation)
test_tenant_user_endpoints() {
    log_info "Testing Tenant-User Association Endpoints..."
    
    # Create a test tenant first
    local test_tenant_id="e2e-test-tenant-$(get_unix_timestamp)"
    local tenant_data='{
        "name": "'$test_tenant_id'",
        "displayName": "E2E Test Tenant",
        "description": "Tenant created for E2E testing",
        "adminEmail": "admin@'$test_tenant_id'.com",
        "adminName": "E2E Admin",
        "status": "active"
    }'
    http_request "POST" "$API_BASE/tenants" "201" "$tenant_data" "Create Test Tenant"
    
    # List tenants
    http_request "GET" "$API_BASE/tenants" "200" "" "List Tenants"
    
    # Get specific tenant
    http_request "GET" "$API_BASE/tenants/$test_tenant_id" "200" "" "Get Test Tenant"
    
    # Create tenant-user association
    local test_user_id="e2e-test-user-$(get_unix_timestamp)"
    local tenant_user_data='{
        "userId": "'$test_user_id'",
        "tenantRole": "tenant_editor",
        "status": "active"
    }'
    http_request "POST" "$API_BASE/tenants/$test_tenant_id/users" "201" "$tenant_user_data" "Create Tenant-User Association"
    
    # List tenant users
    http_request "GET" "$API_BASE/tenants/$test_tenant_id/users" "200" "" "List Tenant Users"
    
    # Get specific tenant-user association
    http_request "GET" "$API_BASE/tenants/$test_tenant_id/users/$test_user_id" "200" "" "Get Tenant-User Association"
    
    # Update tenant-user association
    local update_data='{
        "tenantRole": "tenant_admin",
        "status": "active"
    }'
    http_request "PUT" "$API_BASE/tenants/$test_tenant_id/users/$test_user_id" "200" "$update_data" "Update Tenant-User Association"
    
    # Delete tenant-user association
    http_request "DELETE" "$API_BASE/tenants/$test_tenant_id/users/$test_user_id" "200" "" "Delete Tenant-User Association"
    
    # Update tenant
    local tenant_update_data='{
        "displayName": "Updated E2E Test Tenant",
        "description": "Updated tenant for E2E testing"
    }'
    http_request "PUT" "$API_BASE/tenants/$test_tenant_id" "200" "$tenant_update_data" "Update Test Tenant"
    
    # Delete tenant (cleanup)
    http_request "DELETE" "$API_BASE/tenants/$test_tenant_id" "200" "" "Delete Test Tenant"
    
    # Test error cases
    local invalid_tenant_data='{
        "name": "",
        "adminEmail": "invalid-email"
    }'
    http_request "POST" "$API_BASE/tenants" "400" "$invalid_tenant_data" "Create Invalid Tenant (should fail)"
    
    # Test non-existent tenant
    http_request "GET" "$API_BASE/tenants/non-existent-tenant" "404" "" "Get Non-Existent Tenant (should fail)"
    
    # Test duplicate tenant-user association
    local duplicate_user_data='{
        "userId": "duplicate-user",
        "tenantRole": "tenant_guest"
    }'
    http_request "POST" "$API_BASE/tenants/$TENANT_ID/users" "201" "$duplicate_user_data" "Create First Tenant-User Association"
    http_request "POST" "$API_BASE/tenants/$TENANT_ID/users" "400" "$duplicate_user_data" "Create Duplicate Tenant-User Association (should fail)"
    
    # Cleanup duplicate association
    http_request "DELETE" "$API_BASE/tenants/$TENANT_ID/users/duplicate-user" "200" "" "Cleanup Duplicate Association"
}

# Compatibility Tests (legacy endpoints)
test_compatibility_endpoints() {
    log_info "Testing Legacy/Compatibility Endpoints..."
    
    # Legacy metrics endpoints
    http_request "GET" "$BASE_URL/metrics/names" "200" "" "Legacy: Get Metric Names"
    http_request "GET" "$BASE_URL/series?match[]=up" "200" "" "Legacy: Get Series"
    http_request "GET" "$BASE_URL/labels" "200" "" "Legacy: Get Labels"
    
    # Legacy query endpoints
    local query_data='{"query": "up"}'
    http_request "POST" "$BASE_URL/query" "200" "$query_data" "Legacy: Execute Query"
}

# Documentation endpoints
test_documentation_endpoints() {
    log_info "Testing Documentation Endpoints..."
    
    http_request "GET" "$BASE_URL/swagger/index.html" "200" "" "Swagger UI"
    http_request "GET" "$BASE_URL/api/openapi.yaml" "200" "" "OpenAPI Spec (YAML)"
    http_request "GET" "$BASE_URL/api/openapi.json" "200" "" "OpenAPI Spec (JSON)"
}

# Metrics Metadata Integration Tests (Phase 2)
test_metrics_metadata_endpoints() {
    log_info "Testing Metrics Metadata Integration Endpoints..."
    
    # Metrics search endpoint
    local search_data='{"query": "cpu", "tenant_id": "default", "limit": 10}'
    http_request "POST" "$API_BASE/metrics/search" "200" "$search_data" "Search Metrics Metadata"
    
    # Metrics sync endpoint
    local sync_data='{"tenant_id": "default", "force_full_sync": false}'
    http_request "POST" "$API_BASE/metrics/sync" "200" "$sync_data" "Sync Metrics Metadata"
    
    # Metrics health endpoint
    http_request "GET" "$API_BASE/metrics/health" "200" "" "Metrics Metadata Health"
    
    # Sync management endpoints
    http_request "POST" "$API_BASE/metrics/sync/default" "200" "" "Trigger Sync for Default Tenant"
    http_request "GET" "$API_BASE/metrics/sync/default/state" "200" "" "Get Sync State for Default Tenant"
    http_request "GET" "$API_BASE/metrics/sync/default/status" "200" "" "Get Sync Status for Default Tenant"
    
    # Sync configuration update
    local config_data='{"enabled": true, "strategy": "hybrid", "interval": "900000000000", "full_sync_interval": "86400000000000"}'
    http_request "PUT" "$API_BASE/metrics/sync/config" "200" "$config_data" "Update Sync Configuration"
}

# Code Quality Tests
run_code_quality_tests() {
    if [[ "$RUN_CODE_TESTS" != "true" ]]; then
        log_info "Skipping code quality tests (RUN_CODE_TESTS=false)"
        return 0
    fi
    
    log_phase "Running Code Quality Tests"
    
    # Check if we're in a Go project
    if [[ ! -f "go.mod" ]]; then
        log_warning "go.mod not found. Skipping Go tests."
        return 0
    fi
    
    local code_errors=0
    
    # 1. Run Go unit tests
    log_info "Running Go unit tests..."
    if go test ./... -v; then
        log_success "Go unit tests passed"
        CODE_TESTS_PASSED=$((CODE_TESTS_PASSED + 1))
    else
        log_error "Go unit tests failed"
        CODE_TESTS_FAILED=$((CODE_TESTS_FAILED + 1))
        code_errors=$((code_errors + 1))
    fi
    
    # 2. Run Go fmt check
    log_info "Checking Go formatting..."
    local fmt_files
    fmt_files=$(gofmt -l . 2>/dev/null | grep -v vendor/ | grep -v "\.pb\.go$" || true)
    if [[ -z "$fmt_files" ]]; then
        log_success "Go formatting check passed"
        CODE_TESTS_PASSED=$((CODE_TESTS_PASSED + 1))
    else
        log_error "Go formatting issues found:"
        echo "$fmt_files"
        CODE_TESTS_FAILED=$((CODE_TESTS_FAILED + 1))
        code_errors=$((code_errors + 1))
    fi
    
    # 3. Run Go vet
    log_info "Running go vet..."
    if go vet ./...; then
        log_success "Go vet passed"
        CODE_TESTS_PASSED=$((CODE_TESTS_PASSED + 1))
    else
        log_error "Go vet failed"
        CODE_TESTS_FAILED=$((CODE_TESTS_FAILED + 1))
        code_errors=$((code_errors + 1))
    fi
    
    # 4. Run govulncheck (vulnerability scan)
    log_info "Running vulnerability scan..."
    if command -v govulncheck >/dev/null 2>&1; then
        if govulncheck ./...; then
            log_success "Vulnerability scan passed (no known vulnerabilities)"
            CODE_TESTS_PASSED=$((CODE_TESTS_PASSED + 1))
        else
            log_error "Vulnerability scan found issues"
            CODE_TESTS_FAILED=$((CODE_TESTS_FAILED + 1))
            code_errors=$((code_errors + 1))
        fi
    else
        log_warning "govulncheck not found. Installing..."
        if go install golang.org/x/vuln/cmd/govulncheck@latest; then
            if govulncheck ./...; then
                log_success "Vulnerability scan passed (no known vulnerabilities)"
                CODE_TESTS_PASSED=$((CODE_TESTS_PASSED + 1))
            else
                log_error "Vulnerability scan found issues"
                CODE_TESTS_FAILED=$((CODE_TESTS_FAILED + 1))
                code_errors=$((code_errors + 1))
            fi
        else
            log_warning "Failed to install govulncheck. Skipping vulnerability scan."
        fi
    fi
    
    # 5. Check for Go module tidiness
    log_info "Checking Go module tidiness..."
    if go mod tidy && git diff --quiet go.mod go.sum; then
        log_success "Go modules are tidy"
        CODE_TESTS_PASSED=$((CODE_TESTS_PASSED + 1))
    else
        log_warning "Go modules may need tidying (go mod tidy)"
        # Don't fail for this, just warn
    fi
    
    if [[ $code_errors -gt 0 ]]; then
        log_error "Code quality tests failed with $code_errors errors"
        return 1
    else
        log_success "All code quality tests passed"
        return 0
    fi
}

# Generate failures table in Markdown format
generate_failures_table() {
    if [[ ${#FAILED_TESTS_DATA[@]} -eq 0 ]]; then
        echo "# Test Results: All Tests Passed! 🎉" > "$FAILURES_TABLE_FILE"
        echo "" >> "$FAILURES_TABLE_FILE"
        echo "No failed tests to report." >> "$FAILURES_TABLE_FILE"
        return 0
    fi
    
    log_info "Generating detailed failures table..."
    
    cat > "$FAILURES_TABLE_FILE" << 'EOF'
# API Test Failures Report

## Failed Tests Summary

The following table provides detailed information about failed API tests, including the specific endpoints, error reasons, and suggested fixes.

| Test Name | API Endpoint | Expected Status | Actual Status | Error Reason | Suggested Fix |
|-----------|--------------|-----------------|---------------|--------------|---------------|
EOF
    
    for failure_data in "${FAILED_TESTS_DATA[@]}"; do
        IFS='|' read -r test_name endpoint expected_status actual_status error_reason suggested_fix <<< "$failure_data"
        echo "| $test_name | \`$endpoint\` | $expected_status | $actual_status | $error_reason | $suggested_fix |" >> "$FAILURES_TABLE_FILE"
    done
    
    cat >> "$FAILURES_TABLE_FILE" << 'EOF'

## Common Issues and Solutions

### Microservice Dependencies
- **Predict Engine (503/500)**: Predict microservice not running. This is optional for basic functionality.
- **RCA Engine**: Root Cause Analysis features require additional microservices.

### Configuration Issues
- **Schema Endpoints (404)**: KPI management may be disabled. Check feature flags.
- **User Settings (500)**: User management requires proper authentication configuration.

### API Usage
- **Series Endpoint (400)**: Requires `match[]` parameter, e.g., `?match[]=up`
- **Traces Search (400)**: Needs proper time range parameters

### Infrastructure
- **Network Issues**: Check service connectivity and firewall rules
- **Service Health**: Verify all required services are running and healthy

EOF
    
    log_success "Failures table generated: $FAILURES_TABLE_FILE"
    
    # Print a pretty, terminal-friendly version of the failures table
    echo ""
    echo "📋 FAILED API TESTS SUMMARY"
    echo "=========================="
    
    if [[ ${#FAILED_TESTS_DATA[@]} -gt 0 ]]; then
        local count=1
        for failure_data in "${FAILED_TESTS_DATA[@]}"; do
            IFS='|' read -r test_name endpoint expected_status actual_status error_reason suggested_fix <<< "$failure_data"
            
            # Extract just the endpoint path for cleaner display
            local short_endpoint=$(echo "$endpoint" | sed 's|http://localhost:8010/api/v1||' | sed 's|http://localhost:8010||')
            
            echo ""
            echo "❌ $count. $test_name"
            echo "   📍 Endpoint: $short_endpoint"
            echo "   📊 Status: $actual_status (expected $expected_status)"
            echo "   💡 Issue: $error_reason"
            echo "   🔧 Fix: $suggested_fix"
            ((count++))
        done
        
        echo ""
        echo "📖 COMMON ISSUES & SOLUTIONS:"
        echo "-----------------------------"
        echo "🔸 Predict Engine (503/500): Predict microservice not running (optional)"
        echo "🔸 Schema Endpoints (404): KPI management may be disabled"
        echo "🔸 User Settings (500): User management requires authentication config"
        echo "🔸 Traces Search (400): Needs proper time range parameters"
        echo "🔸 Invalid Method (404): Endpoint may not support the HTTP method"
    else
        echo "✅ No failed tests to report!"
    fi
    
    echo "=========================="
}

# Generate summary report
generate_summary() {
    log_info "Generating comprehensive test summary..."
    
    # Generate failures table
    generate_failures_table
    
    local api_success_rate=0
    if [[ $TOTAL_TESTS -gt 0 ]]; then
        api_success_rate=$(echo "scale=2; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc 2>/dev/null || echo "0")
    fi
    
    local total_code_tests=$((CODE_TESTS_PASSED + CODE_TESTS_FAILED))
    local code_success_rate=0
    if [[ $total_code_tests -gt 0 ]]; then
        code_success_rate=$(echo "scale=2; $CODE_TESTS_PASSED * 100 / $total_code_tests" | bc 2>/dev/null || echo "0")
    fi
    
    echo
    echo "=============================================================="
    echo "                COMPREHENSIVE TEST SUMMARY"
    echo "=============================================================="
    echo "🔍 Platform: $OS_NAME ($OS_TYPE)"
    echo "📅 Timestamp: $(get_iso_timestamp)"
    echo ""
    
    if [[ "$RUN_CODE_TESTS" == "true" ]]; then
        echo "📋 CODE QUALITY TESTS:"
        echo "   Passed: $CODE_TESTS_PASSED"
        echo "   Failed: $CODE_TESTS_FAILED"
        echo "   Success Rate: ${code_success_rate}%"
        echo ""
    fi
    
    echo "🌐 API ENDPOINT TESTS:"
    echo "   Total Tests: $TOTAL_TESTS"
    echo "   Passed: $PASSED_TESTS"
    echo "   Failed: $FAILED_TESTS"
    echo "   Success Rate: ${api_success_rate}%"
    echo ""
    echo "📊 RESULTS FILES:"
    echo "   JSON Results: $RESULTS_FILE"
    echo "   Failures Table: $FAILURES_TABLE_FILE"
    echo "=============================================================="
    
    # Add comprehensive summary to JSON results
    local summary=$(cat <<EOF
{
    "comprehensive_summary": {
        "platform": "$OS_NAME",
        "os_type": "$OS_TYPE",
        "timestamp": "$(get_iso_timestamp)",
        "code_quality": {
            "tests_run": $RUN_CODE_TESTS,
            "passed": $CODE_TESTS_PASSED,
            "failed": $CODE_TESTS_FAILED,
            "success_rate": $code_success_rate
        },
        "api_tests": {
            "total_tests": $TOTAL_TESTS,
            "passed_tests": $PASSED_TESTS,
            "failed_tests": $FAILED_TESTS,
            "success_rate": $api_success_rate
        },
        "files": {
            "json_results": "$RESULTS_FILE",
            "failures_table": "$FAILURES_TABLE_FILE"
        }
    }
}
EOF
)
    
    jq ". += [$summary]" "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"
    
    # Return appropriate exit code
    local overall_failed=0
    if [[ "$RUN_CODE_TESTS" == "true" && $CODE_TESTS_FAILED -gt 0 ]]; then
        overall_failed=1
    fi
    if [[ $FAILED_TESTS -gt 0 ]]; then
        overall_failed=1
    fi
    
    if [[ $overall_failed -gt 0 ]]; then
        log_error "Some tests failed. Check the detailed results and failures table."
        echo ""
        echo "📋 Quick Actions:"
        echo "   • Review failures table above (already printed)"
        echo "   • Check logs: check service logs for detailed error info"
        echo "   • Verify setup: ensure all required services are running"
        return 1
    else
        log_success "All tests passed! 🎉"
        return 0
    fi
}

# Main execution
main() {
    echo "🚀 Starting Comprehensive Mirador Core Testing Pipeline"
    echo "=============================================================="
    echo "📋 Configuration:"
    echo "   Platform: $OS_NAME ($OS_TYPE)"
    echo "   Base URL: $BASE_URL"
    echo "   Tenant ID: $TENANT_ID"
    echo "   Verbose: $VERBOSE"
    echo "   Code Tests: $RUN_CODE_TESTS"
    echo "=============================================================="
    echo
    
    # Check dependencies first
    check_dependencies
    
    # Phase 1: Code Quality Tests (if enabled)
    if [[ "$RUN_CODE_TESTS" == "true" ]]; then
        if ! run_code_quality_tests; then
            log_error "Code quality tests failed. Stopping pipeline."
            exit 1
        fi
        echo
    fi
    
    # Phase 2: Infrastructure Setup
    log_phase "Infrastructure Setup"
    log_info "Checking service readiness..."
    wait_for_service
    
    # Phase 2.5: Authentication
    log_phase "Authentication"
    if ! authenticate; then
        log_error "Failed to authenticate, cannot proceed with API tests"
        exit 1
    fi
    echo
    
    # Phase 3: API Testing
    log_phase "API Endpoint Testing"
    
    # Core functionality tests
    test_health_endpoints
    test_metrics_endpoints
    test_metrics_functions  
    test_logs_endpoints
    test_traces_endpoints
    test_unified_query_endpoints
    test_metrics_metadata_endpoints  # Phase 2: Metrics Metadata Integration
    
    # Advanced features tests
    test_rca_endpoints
    test_predict_endpoints
    test_config_endpoints
    test_schema_endpoints
    test_session_endpoints
    test_rbac_endpoints
    test_tenant_user_endpoints  # Phase 2: Tenant Management & Isolation
    
    # Compatibility and documentation tests
    test_compatibility_endpoints
    test_documentation_endpoints
    
    # Phase 4: Results Generation
    log_phase "Results and Reporting"
    generate_summary
}

# Handle script arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -b|--base-url)
            BASE_URL="$2"
            API_BASE="$BASE_URL/api/v1"
            shift 2
            ;;
        -t|--tenant)
            TENANT_ID="$2"
            shift 2
            ;;
        -o|--output)
            RESULTS_FILE="$2"
            shift 2
            ;;
        --no-code-tests)
            RUN_CODE_TESTS=false
            shift
            ;;
        --code-tests-only)
            RUN_CODE_TESTS=true
            # Set a flag to run only code tests
            CODE_TESTS_ONLY=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Comprehensive E2E testing pipeline for Mirador Core"
            echo ""
            echo "Options:"
            echo "  -v, --verbose              Enable verbose output"
            echo "  -b, --base-url URL         Set base URL (default: http://localhost:8010)"
            echo "  -t, --tenant ID            Set tenant ID (default: default)"
            echo "  -o, --output FILE          Set results file (default: e2e-test-results.json)"
            echo "  --no-code-tests            Skip code quality tests"
            echo "  --code-tests-only          Run only code quality tests"
            echo "  -h, --help                 Show this help message"
            echo ""
            echo "Environment Variables:"
            echo "  RUN_CODE_TESTS=false       Skip code quality tests"
            echo "  BASE_URL                   Override base URL"
            echo "  VERBOSE=true               Enable verbose mode"
            echo ""
            echo "Example Usage:"
            echo "  $0 --verbose                              # Full pipeline with verbose output"
            echo "  $0 --no-code-tests                       # API tests only"
            echo "  $0 --code-tests-only                     # Code quality tests only"
            echo "  $0 -b https://staging.example.com        # Test against staging"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Handle special mode
if [[ "${CODE_TESTS_ONLY:-false}" == "true" ]]; then
    echo "🔍 Running Code Quality Tests Only"
    check_dependencies
    if run_code_quality_tests; then
        log_success "Code quality tests completed successfully"
        exit 0
    else
        log_error "Code quality tests failed"
        exit 1
    fi
fi

# Run main function
main "$@"