#!/bin/bash
# check-hardcoded-violations.sh
# 
# Purpose: Detect hardcoded metric names, service names, and label keys in engine code
# Usage: ./scripts/check-hardcoded-violations.sh [--fix]
# Exit codes:
#   0 = No violations found
#   1 = Violations detected (lists violating files and patterns)
#   2 = Usage/configuration error
#
# This script enforces AGENTS.md §3.6 rules:
# - No hardcoded metric/KPI names in correlation/RCA engines
# - No hardcoded service names in engine logic
# - No hardcoded label keys (except semantic canonical keys)
#
# Exemptions:
# - Test files (*_test.go) may use hardcoded strings for assertions
# - Simulator code (cmd/otel-fintrans-simulator) is exempt (see HCB-008)
# - Test fixtures (internal/services/testdata/) are exempt
# - Config defaults (internal/config/defaults.go) must use empty slices with NOTE comments

set -euo pipefail

# ANSI color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Violation tracking
VIOLATIONS_FOUND=0
VIOLATION_FILES=()

# Forbidden patterns - these should NEVER appear in engine code
# Add common observability metric patterns
FORBIDDEN_PATTERNS=(
    # Metric names
    "transactions_total"
    "transactions_failed_total"
    "api_gateway_requests"
    "http_requests_total"
    "http_errors_total"
    "cpu_usage"
    "memory_usage"
    "disk_io_total"
    "network_bytes_total"
    "db_ops_total"
    "kafka_produce_total"
    "kafka_consume_total"
    "transaction_latency_seconds"
    "db_latency_seconds"
    
    # Service names
    '"api-gateway"'
    '"kafka-producer"'
    '"kafka-consumer"'
    '"postgres"'
    '"redis"'
    '"tps"'
    '"payment-service"'
    '"notification-service"'
    
    # Label keys (non-canonical)
    '"service.name"'
    '"kubernetes.pod_name"'
    '"container.name"'
    '"host.name"'
)

# Files to check (engine code only, exclude tests and exempted paths)
ENGINE_FILES=(
    "internal/services/correlation_engine.go"
    "internal/rca/engine.go"
    "internal/rca/rca.handler.go"
    "internal/services/metrics_*.go"
    "internal/discovery/*.go"
)

# Exempted paths (simulators, test fixtures, tests, and documentation files)
EXEMPTED_PATHS=(
    "cmd/otel-fintrans-simulator"
    "*_test.go"
    "testdata"
    "test_fixtures.go"
    "test_fixtures_example_test.go"
    "internal/rca/label_detector.go"  # Contains hardcoded examples in documentation
    "internal/rca/models.go"           # Contains example service names in comments
)

echo -e "${YELLOW}Checking for hardcoded violations in engine code...${NC}"

# Function: Check if file is exempted
is_exempted() {
    local file="$1"
    for pattern in "${EXEMPTED_PATHS[@]}"; do
        if [[ "$file" == *"$pattern"* ]]; then
            return 0  # File is exempted
        fi
    done
    return 1  # File is NOT exempted
}

# Function: Check single file for violations
check_file() {
    local file="$1"
    
    # Skip if exempted
    if is_exempted "$file"; then
        return 0
    fi
    
    # Skip if file doesn't exist
    if [[ ! -f "$file" ]]; then
        return 0
    fi
    
    local file_violations=0
    
    for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
        # Search for pattern (case-insensitive for metric names)
        if grep -n -i "$pattern" "$file" > /dev/null 2>&1; then
            if [[ $file_violations -eq 0 ]]; then
                echo -e "${RED}✗ Violations found in: $file${NC}"
                VIOLATION_FILES+=("$file")
            fi
            
            # Show matching lines with line numbers
            echo -e "  ${YELLOW}Pattern: $pattern${NC}"
            grep -n -i "$pattern" "$file" | while IFS=: read -r line_num line_content; do
                echo -e "    Line $line_num: ${line_content:0:80}"
            done
            
            file_violations=$((file_violations + 1))
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
        fi
    done
    
    return 0
}

# Check all engine files
for file_pattern in "${ENGINE_FILES[@]}"; do
    # Expand glob pattern
    for file in $file_pattern; do
        check_file "$file"
    done
done

# Special check: defaults.go must only have empty slices with NOTE comments
DEFAULT_CONFIG="internal/config/defaults.go"
if [[ -f "$DEFAULT_CONFIG" ]]; then
    echo -e "\n${YELLOW}Checking $DEFAULT_CONFIG for proper empty slice pattern...${NC}"
    
    # Check that Probes and ServiceCandidates are empty with NOTE comments
    if grep -A 2 "Probes:" "$DEFAULT_CONFIG" | grep -v "NOTE(HCB-001)" | grep -E '\[\]string\{".*"\}' > /dev/null; then
        echo -e "${RED}✗ defaults.go contains non-empty Probes slice${NC}"
        echo -e "  Required pattern: Probes: []string{}, // NOTE(HCB-001): ..."
        VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
        VIOLATION_FILES+=("$DEFAULT_CONFIG")
    fi
    
    if grep -A 2 "ServiceCandidates:" "$DEFAULT_CONFIG" | grep -v "NOTE(HCB-002)" | grep -E '\[\]string\{".*"\}' > /dev/null; then
        echo -e "${RED}✗ defaults.go contains non-empty ServiceCandidates slice${NC}"
        echo -e "  Required pattern: ServiceCandidates: []string{}, // NOTE(HCB-002): ..."
        VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
        if [[ ! " ${VIOLATION_FILES[@]} " =~ " ${DEFAULT_CONFIG} " ]]; then
            VIOLATION_FILES+=("$DEFAULT_CONFIG")
        fi
    fi
fi

# Report results
echo ""
echo "=============================================="
if [[ $VIOLATIONS_FOUND -eq 0 ]]; then
    echo -e "${GREEN}✓ No hardcoded violations detected!${NC}"
    echo "All engine code follows AGENTS.md §3.6 rules."
    exit 0
else
    echo -e "${RED}✗ Found $VIOLATIONS_FOUND violation(s) in ${#VIOLATION_FILES[@]} file(s)${NC}"
    echo ""
    echo "Violating files:"
    for file in "${VIOLATION_FILES[@]}"; do
        echo "  - $file"
    done
    echo ""
    echo "AGENTS.md §3.6 Rules:"
    echo "  1. No hardcoded metric/KPI names in engines"
    echo "  2. No hardcoded service names in engine logic"
    echo "  3. No hardcoded label keys (except canonical semantic keys)"
    echo ""
    echo "Fix by:"
    echo "  - Using EngineConfig for configurable values"
    echo "  - Using KPI/service registries for discovery"
    echo "  - Using Stage-00 metadata for labels"
    echo ""
    echo "See: dev/correlation-RCA-engine/current/01-correlation-rca-code-implementation-final.md"
    exit 1
fi
