#!/bin/bash

# Week 2: Integration Testing + Security Testing for Mirador Core v9.0.0
# This script extends the existing E2E tests with comprehensive security validation
# and deeper integration testing for RBAC enforcement and multi-tenant isolation.

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BASE_URL="${BASE_URL:-http://localhost:8010}"
E2E_REPORT="${PROJECT_ROOT}/localtesting/week2-e2e-report.json"
FAILURES_TABLE="${PROJECT_ROOT}/localtesting/week2-test-failures-table.md"
SECURITY_REPORT="${PROJECT_ROOT}/localtesting/week2-security-audit.json"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SECURITY_VULNERABILITIES=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test results storage
declare -a TEST_RESULTS
declare -a SECURITY_ISSUES

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_security_vulnerability() {
    echo -e "${RED}[VULN]${NC} $1"
    ((SECURITY_VULNERABILITIES++))
}

# HTTP request helper with authentication
http_request() {
    local method="$1"
    local url="$2"
    local data="${3:-}"
    local token="${4:-}"
    local expected_status="${5:-200}"

    local curl_cmd="curl -s -w '\n%{http_code}' -X $method"
    if [[ -n "$token" ]]; then
        curl_cmd="$curl_cmd -H 'Authorization: Bearer $token'"
    fi
    if [[ -n "$data" ]]; then
        curl_cmd="$curl_cmd -H 'Content-Type: application/json' -d '$data'"
    fi
    curl_cmd="$curl_cmd '$BASE_URL$url'"

    local response
    response=$(eval "$curl_cmd" 2>/dev/null || echo "")

    # Extract body and status code
    local body=""
    local http_code="000"

    if [[ -n "$response" ]]; then
        # Get the last line as status code
        http_code=$(echo "$response" | tail -n1 | tr -d '\n\r')
        # Get everything except the last line as body
        body=$(echo "$response" | head -n -1)
    fi

    # Ensure http_code is numeric
    if ! [[ "$http_code" =~ ^[0-9]+$ ]]; then
        http_code="000"
    fi

    if [[ "$http_code" -ne "$expected_status" ]]; then
        log_error "Expected status $expected_status, got $http_code for $method $url"
        log_error "Response: $body"
        return 1
    fi

    echo "$body"
}

# Authentication functions
authenticate() {
    local username="$1"
    local password="$2"

    local response
    response=$(http_request POST "/api/v1/auth/login" "{\"username\":\"$username\",\"password\":\"$password\"}")

    if [[ $? -ne 0 ]]; then
        log_error "Authentication failed for user: $username"
        return 1
    fi

    # Extract token from response
    echo "$response" | jq -r '.token // empty'
}

# Test user creation and management
create_test_user() {
    local username="$1"
    local password="$2"
    local tenant_id="$3"
    local role="$4"
    local token="$5"

    local user_data="{\"username\":\"$username\",\"password\":\"$password\",\"tenant_id\":\"$tenant_id\",\"role\":\"$role\"}"

    http_request POST "/api/v1/admin/users" "$user_data" "$token" 201
}

# Test tenant creation
create_test_tenant() {
    local tenant_name="$1"
    local token="$2"

    local tenant_data="{\"name\":\"$tenant_name\",\"description\":\"Test tenant for security validation\"}"

    local response
    response=$(http_request POST "/api/v1/admin/tenants" "$tenant_data" "$token" 201)

    echo "$response" | jq -r '.id // empty'
}

# Security testing functions
test_privilege_escalation() {
    local test_name="Privilege Escalation Test"
    ((TOTAL_TESTS++))

    log_info "Running $test_name..."

    # Create a regular user
    local regular_user_token
    regular_user_token=$(authenticate "testuser" "testpass")

    if [[ -z "$regular_user_token" ]]; then
        log_error "$test_name: Failed to authenticate regular user"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Try to access admin endpoints with regular user token
    local admin_endpoints=(
        "/api/v1/admin/users"
        "/api/v1/admin/tenants"
        "/api/v1/admin/rbac/roles"
        "/api/v1/admin/rbac/policies"
    )

    local escalation_attempts=0
    local blocked_attempts=0

    for endpoint in "${admin_endpoints[@]}"; do
        ((escalation_attempts++))
        if http_request GET "$endpoint" "" "$regular_user_token" 403 >/dev/null 2>&1; then
            ((blocked_attempts++))
        else
            log_security_vulnerability "$test_name: Regular user could access admin endpoint: $endpoint"
        fi
    done

    if [[ $blocked_attempts -eq $escalation_attempts ]]; then
        log_success "$test_name: All privilege escalation attempts blocked"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"passed\"}")
        ((PASSED_TESTS++))
        return 0
    else
        log_error "$test_name: $((escalation_attempts - blocked_attempts)) privilege escalation vulnerabilities found"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Privilege escalation possible\"}")
        ((FAILED_TESTS++))
        return 1
    fi
}

test_session_hijacking() {
    local test_name="Session Hijacking Test"
    ((TOTAL_TESTS++))

    log_info "Running $test_name..."

    # Authenticate as admin
    local admin_token
    admin_token=$(authenticate "aarvee" "admin123")

    if [[ -z "$admin_token" ]]; then
        log_error "$test_name: Failed to authenticate admin"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Admin authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Try to use the token in various ways that could indicate session hijacking vulnerabilities
    local vulnerabilities_found=0

    # Test 1: Token reuse after logout (if logout endpoint exists)
    if http_request POST "/api/v1/auth/logout" "" "$admin_token" 200 >/dev/null 2>&1; then
        # Try to use token after logout
        if http_request GET "/api/v1/admin/users" "" "$admin_token" 401 >/dev/null 2>&1; then
            log_success "$test_name: Token invalidated after logout"
        else
            log_security_vulnerability "$test_name: Token still valid after logout"
            ((vulnerabilities_found++))
        fi
    fi

    # Test 2: Check if tokens are properly invalidated
    local new_token
    new_token=$(authenticate "aarvee" "admin123")

    # Try using old token
    if http_request GET "/api/v1/admin/users" "" "$admin_token" 401 >/dev/null 2>&1; then
        log_success "$test_name: Old tokens properly invalidated"
    else
        log_security_vulnerability "$test_name: Old tokens still valid - session hijacking possible"
        ((vulnerabilities_found++))
    fi

    if [[ $vulnerabilities_found -eq 0 ]]; then
        log_success "$test_name: No session hijacking vulnerabilities found"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"passed\"}")
        ((PASSED_TESTS++))
        return 0
    else
        log_error "$test_name: $vulnerabilities_found session hijacking vulnerabilities found"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Session hijacking possible\"}")
        ((FAILED_TESTS++))
        return 1
    fi
}

test_cross_tenant_access() {
    local test_name="Cross-Tenant Access Test"
    ((TOTAL_TESTS++))

    log_info "Running $test_name..."

    # Create two test tenants
    local admin_token
    admin_token=$(authenticate "aarvee" "admin123")

    if [[ -z "$admin_token" ]]; then
        log_error "$test_name: Failed to authenticate admin"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Admin authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    local tenant1_id
    tenant1_id=$(create_test_tenant "security-test-tenant-1" "$admin_token")

    local tenant2_id
    tenant2_id=$(create_test_tenant "security-test-tenant-2" "$admin_token")

    if [[ -z "$tenant1_id" || -z "$tenant2_id" ]]; then
        log_error "$test_name: Failed to create test tenants"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Tenant creation failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Create users in different tenants
    create_test_user "tenant1user" "pass123" "$tenant1_id" "user" "$admin_token"
    create_test_user "tenant2user" "pass123" "$tenant2_id" "user" "$admin_token"

    # Authenticate as tenant1 user
    local tenant1_token
    tenant1_token=$(authenticate "tenant1user" "pass123")

    if [[ -z "$tenant1_token" ]]; then
        log_error "$test_name: Failed to authenticate tenant1 user"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Tenant1 user authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Try to access tenant2 resources with tenant1 token
    local cross_tenant_endpoints=(
        "/api/v1/tenants/$tenant2_id/logs"
        "/api/v1/tenants/$tenant2_id/metrics"
        "/api/v1/tenants/$tenant2_id/traces"
    )

    local access_attempts=0
    local blocked_attempts=0

    for endpoint in "${cross_tenant_endpoints[@]}"; do
        ((access_attempts++))
        if http_request GET "$endpoint" "" "$tenant1_token" 403 >/dev/null 2>&1; then
            ((blocked_attempts++))
        else
            log_security_vulnerability "$test_name: Cross-tenant access allowed: $endpoint"
        fi
    done

    if [[ $blocked_attempts -eq $access_attempts ]]; then
        log_success "$test_name: Cross-tenant access properly blocked"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"passed\"}")
        ((PASSED_TESTS++))
        return 0
    else
        log_error "$test_name: $((access_attempts - blocked_attempts)) cross-tenant access vulnerabilities found"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Cross-tenant access possible\"}")
        ((FAILED_TESTS++))
        return 1
    fi
}

test_rbac_enforcement() {
    local test_name="RBAC Enforcement Test"
    ((TOTAL_TESTS++))

    log_info "Running $test_name..."

    local admin_token
    admin_token=$(authenticate "aarvee" "admin123")

    if [[ -z "$admin_token" ]]; then
        log_error "$test_name: Failed to authenticate admin"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Admin authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Create test tenant and user
    local test_tenant_id
    test_tenant_id=$(create_test_tenant "rbac-test-tenant" "$admin_token")

    if [[ -z "$test_tenant_id" ]]; then
        log_error "$test_name: Failed to create test tenant"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Tenant creation failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    create_test_user "rbacuser" "pass123" "$test_tenant_id" "user" "$admin_token"

    local user_token
    user_token=$(authenticate "rbacuser" "pass123")

    if [[ -z "$user_token" ]]; then
        log_error "$test_name: Failed to authenticate test user"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"User authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Test RBAC enforcement on various endpoints
    local rbac_tests=(
        "GET:/api/v1/admin/users:403"  # Regular user cannot list all users
        "GET:/api/v1/admin/tenants:403"  # Regular user cannot list all tenants
        "GET:/api/v1/tenants/$test_tenant_id/logs:200"  # User can access own tenant logs
        "GET:/api/v1/tenants/$test_tenant_id/metrics:200"  # User can access own tenant metrics
        "POST:/api/v1/admin/users:403"  # Regular user cannot create users
        "DELETE:/api/v1/admin/tenants/$test_tenant_id:403"  # Regular user cannot delete tenants
    )

    local rbac_violations=0

    for test_case in "${rbac_tests[@]}"; do
        IFS=':' read -r method endpoint expected_status <<< "$test_case"

        if ! http_request "$method" "$endpoint" "" "$user_token" "$expected_status" >/dev/null 2>&1; then
            log_security_vulnerability "$test_name: RBAC violation - $method $endpoint should return $expected_status"
            ((rbac_violations++))
        fi
    done

    if [[ $rbac_violations -eq 0 ]]; then
        log_success "$test_name: RBAC enforcement working correctly"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"passed\"}")
        ((PASSED_TESTS++))
        return 0
    else
        log_error "$test_name: $rbac_violations RBAC enforcement violations found"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"RBAC violations detected\"}")
        ((FAILED_TESTS++))
        return 1
    fi
}

test_concurrent_access() {
    local test_name="Concurrent Multi-Tenant Access Test"
    ((TOTAL_TESTS++))

    log_info "Running $test_name..."

    local admin_token
    admin_token=$(authenticate "aarvee" "admin123")

    if [[ -z "$admin_token" ]]; then
        log_error "$test_name: Failed to authenticate admin"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Admin authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Create multiple test tenants and users
    local tenant_ids=()
    local user_tokens=()

    for i in {1..3}; do
        local tenant_id
        tenant_id=$(create_test_tenant "concurrent-test-tenant-$i" "$admin_token")
        tenant_ids+=("$tenant_id")

        create_test_user "concurrent-user-$i" "pass123" "$tenant_id" "user" "$admin_token"

        local user_token
        user_token=$(authenticate "concurrent-user-$i" "pass123")
        user_tokens+=("$user_token")
    done

    # Run concurrent requests to test isolation
    local concurrent_requests=0
    local isolation_violations=0

    for i in {0..2}; do
        for j in {0..2}; do
            if [[ $i -ne $j ]]; then
                ((concurrent_requests++))
                # User i tries to access tenant j's data
                local endpoint="/api/v1/tenants/${tenant_ids[$j]}/logs"
                if ! http_request GET "$endpoint" "" "${user_tokens[$i]}" 403 >/dev/null 2>&1; then
                    log_security_vulnerability "$test_name: Concurrent access violation - user $i accessed tenant $j data"
                    ((isolation_violations++))
                fi
            fi
        done
    done

    if [[ $isolation_violations -eq 0 ]]; then
        log_success "$test_name: Concurrent multi-tenant isolation maintained"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"passed\"}")
        ((PASSED_TESTS++))
        return 0
    else
        log_error "$test_name: $isolation_violations concurrent isolation violations found"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Concurrent isolation breached\"}")
        ((FAILED_TESTS++))
        return 1
    fi
}

test_authentication_bypass() {
    local test_name="Authentication Bypass Test"
    ((TOTAL_TESTS++))

    log_info "Running $test_name..."

    local bypass_attempts=0
    local successful_bypasses=0

    # Test 1: Empty token
    ((bypass_attempts++))
    if http_request GET "/api/v1/admin/users" "" "" 401 >/dev/null 2>&1; then
        : # Expected 401
    else
        log_security_vulnerability "$test_name: Empty token bypass possible"
        ((successful_bypasses++))
    fi

    # Test 2: Malformed token
    ((bypass_attempts++))
    if http_request GET "/api/v1/admin/users" "" "invalid.jwt.token" 401 >/dev/null 2>&1; then
        : # Expected 401
    else
        log_security_vulnerability "$test_name: Malformed token bypass possible"
        ((successful_bypasses++))
    fi

    # Test 3: Expired token simulation (if supported)
    ((bypass_attempts++))
    if http_request GET "/api/v1/admin/users" "" "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE1MTYyMzkwMjJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" 401 >/dev/null 2>&1; then
        : # Expected 401
    else
        log_security_vulnerability "$test_name: Expired token bypass possible"
        ((successful_bypasses++))
    fi

    if [[ $successful_bypasses -eq 0 ]]; then
        log_success "$test_name: No authentication bypass vulnerabilities found"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"passed\"}")
        ((PASSED_TESTS++))
        return 0
    else
        log_error "$test_name: $successful_bypasses authentication bypass vulnerabilities found"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Authentication bypass possible\"}")
        ((FAILED_TESTS++))
        return 1
    fi
}

test_end_to_end_authentication_flow() {
    local test_name="End-to-End Authentication Flow Test"
    ((TOTAL_TESTS++))

    log_info "Running $test_name..."

    local admin_token
    admin_token=$(authenticate "aarvee" "admin123")

    if [[ -z "$admin_token" ]]; then
        log_error "$test_name: Failed to authenticate admin user"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Admin authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Create test tenant
    local test_tenant_id
    test_tenant_id=$(create_test_tenant "auth-flow-test-tenant" "$admin_token")

    if [[ -z "$test_tenant_id" ]]; then
        log_error "$test_name: Failed to create test tenant"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Tenant creation failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Create test user
    create_test_user "authflowuser" "testpass123" "$test_tenant_id" "user" "$admin_token"

    # Test complete authentication flow
    local user_token
    user_token=$(authenticate "authflowuser" "testpass123")

    if [[ -z "$user_token" ]]; then
        log_error "$test_name: Failed to authenticate test user"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"User authentication failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Test authenticated access
    local response
    response=$(http_request GET "/api/v1/tenants/$test_tenant_id/logs" "" "$user_token")

    if [[ $? -ne 0 ]]; then
        log_error "$test_name: Failed to access tenant resources with valid token"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Authenticated access failed\"}")
        ((FAILED_TESTS++))
        return 1
    fi

    # Test token-based authorization
    if http_request GET "/api/v1/admin/users" "" "$user_token" 403 >/dev/null 2>&1; then
        log_success "$test_name: End-to-end authentication and authorization working"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"passed\"}")
        ((PASSED_TESTS++))
        return 0
    else
        log_error "$test_name: Authorization bypass in authenticated flow"
        TEST_RESULTS+=("{\"test\":\"$test_name\",\"status\":\"failed\",\"error\":\"Authorization bypass\"}")
        ((FAILED_TESTS++))
        return 1
    fi
}

generate_security_report() {
    log_info "Generating security audit report..."

    local report="{
  \"week\": \"2\",
  \"phase\": \"Integration Testing + Security Testing\",
  \"timestamp\": \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\",
  \"summary\": {
    \"total_tests\": $TOTAL_TESTS,
    \"passed_tests\": $PASSED_TESTS,
    \"failed_tests\": $FAILED_TESTS,
    \"security_vulnerabilities\": $SECURITY_VULNERABILITIES,
    \"pass_rate\": $((PASSED_TESTS * 100 / TOTAL_TESTS))%
  },
  \"security_findings\": ["

    local first=true
    for issue in "${SECURITY_ISSUES[@]}"; do
        if [[ $first == true ]]; then
            first=false
        else
            report+=","
        fi
        report+="$issue"
    done

    report+="],
  \"test_results\": ["

    first=true
    for result in "${TEST_RESULTS[@]}"; do
        if [[ $first == true ]]; then
            first=false
        else
            report+=","
        fi
        report+="$result"
    done

    report+="]
}"

    echo "$report" | jq '.' > "$SECURITY_REPORT"
    log_info "Security audit report saved to: $SECURITY_REPORT"
}

generate_failures_table() {
    log_info "Generating test failures table..."

    {
        echo "# Week 2 Test Failures Summary"
        echo ""
        echo "| Test Name | Status | Error |"
        echo "|-----------|--------|-------|"
    } > "$FAILURES_TABLE"

    for result in "${TEST_RESULTS[@]}"; do
        local status
        status=$(echo "$result" | jq -r '.status')
        if [[ "$status" == "failed" ]]; then
            local test_name
            test_name=$(echo "$result" | jq -r '.test')
            local error_msg
            error_msg=$(echo "$result" | jq -r '.error // "Unknown error"')
            echo "| $test_name | $status | $error_msg |" >> "$FAILURES_TABLE"
        fi
    done

    log_info "Test failures table saved to: $FAILURES_TABLE"
}

# Main test execution
main() {
    log_info "Starting Week 2: Integration Testing + Security Testing"
    log_info "Base URL: $BASE_URL"
    log_info "Security Report: $SECURITY_REPORT"
    log_info "Failures Table: $FAILURES_TABLE"

    # Run all security and integration tests
    test_end_to_end_authentication_flow
    test_privilege_escalation
    test_session_hijacking
    test_cross_tenant_access
    test_rbac_enforcement
    test_concurrent_access
    test_authentication_bypass

    # Generate reports
    generate_security_report
    generate_failures_table

    # Summary
    local pass_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))

    echo ""
    log_info "Week 2 Testing Complete"
    log_info "Total Tests: $TOTAL_TESTS"
    log_info "Passed: $PASSED_TESTS"
    log_info "Failed: $FAILED_TESTS"
    log_info "Security Vulnerabilities: $SECURITY_VULNERABILITIES"
    log_info "Pass Rate: ${pass_rate}%"

    if [[ $SECURITY_VULNERABILITIES -eq 0 && $pass_rate -ge 80 ]]; then
        log_success "Week 2 requirements met - proceeding to Week 3"
        exit 0
    else
        log_error "Week 2 requirements not fully met"
        exit 1
    fi
}

# Run main function
main "$@"