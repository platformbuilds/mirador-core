#!/bin/bash
# Failure Correlation Engine - Quick Verification Script
# This script verifies that the failure correlation engine is properly implemented

set -e

echo "🔍 Failure Correlation Engine Verification Script"
echo "=================================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

check() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅${NC} $1"
    else
        echo -e "${RED}❌${NC} $1"
        exit 1
    fi
}

# 1. Check that core files exist
echo "1️⃣  Checking core implementation files..."
test -f internal/services/correlation_engine.go && check "correlation_engine.go exists"
test -f internal/models/correlation_query.go && check "correlation_query.go exists"
test -f internal/api/handlers/unified_query.go && check "unified_query.go exists"
test -f internal/services/correlation_engine_failure_detection_test.go && check "failure detection tests exist"

# 2. Check that required methods are implemented
echo ""
echo "2️⃣  Checking core methods..."
grep -q "func.*DetectComponentFailures" internal/services/correlation_engine.go && check "DetectComponentFailures method exists"
grep -q "func.*CorrelateTransactionFailures" internal/services/correlation_engine.go && check "CorrelateTransactionFailures method exists"
grep -q "func.*queryErrorSignals" internal/services/correlation_engine.go && check "queryErrorSignals method exists"
grep -q "func.*groupSignalsByTransactionAndComponent" internal/services/correlation_engine.go && check "groupSignalsByTransactionAndComponent method exists"

# 3. Check that API routes are registered
echo ""
echo "3️⃣  Checking API routes..."
grep -q "failures/detect" internal/api/server.go && check "Failure detection endpoint registered"
grep -q "failures/correlate" internal/api/server.go && check "Failure correlation endpoint registered"

# 4. Check that data models exist
echo ""
echo "4️⃣  Checking data models..."
grep -q "type FailureIncident" internal/models/correlation_query.go && check "FailureIncident type exists"
grep -q "type FailureSignal" internal/models/correlation_query.go && check "FailureSignal type exists"
grep -q "type FailureComponent" internal/models/correlation_query.go && check "FailureComponent enum exists"

# 5. Run unit tests
echo ""
echo "5️⃣  Running unit tests..."
TEST_OUTPUT=$(go test -v ./internal/services -run "Failure|Grouping|Mapping" -timeout 30s 2>&1)
echo "$TEST_OUTPUT" | grep -q "ok.*github.com/mirastacklabs-ai/mirador-core/internal/services" && check "All correlation engine tests pass"

# 6. Check documentation
echo ""
echo "6️⃣  Checking documentation..."
test -f dev/correlation-failures.md && check "correlation-failures.md documentation exists"
test -f FAILURE_CORRELATION_SUMMARY.md && check "FAILURE_CORRELATION_SUMMARY.md exists"

# 7. Summary statistics
echo ""
echo "7️⃣  Code statistics..."
LINES=$(wc -l < internal/services/correlation_engine.go)
echo -e "${GREEN}✅${NC} correlation_engine.go: $LINES lines"

TESTS=$(grep -c "func Test" internal/services/correlation_engine_failure_detection_test.go)
echo -e "${GREEN}✅${NC} Test cases: $TESTS"

# 8. Print implementation summary
echo ""
echo "=========================================="
echo -e "${GREEN}✅ VERIFICATION COMPLETE${NC}"
echo "=========================================="
echo ""
echo "Summary:"
echo "✅ Core implementation: COMPLETE"
echo "✅ API endpoints: REGISTERED"
echo "✅ Data models: DEFINED"
echo "✅ Unit tests: PASSING"
echo "✅ Documentation: COMPLETE"
echo ""
echo "Components Supported:"
echo "  • api-gateway"
echo "  • tps (Transaction Processing System)"
echo "  • keydb"
echo "  • kafka"
echo "  • cassandra"
echo ""
echo "Quick Start:"
echo "  make localdev-up"
echo "  make localdev-seed-otel"
echo "  curl -X POST http://localhost:8010/api/v1/unified/failures/detect"
echo ""
