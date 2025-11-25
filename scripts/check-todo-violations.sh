#!/bin/bash
# check-todo-violations.sh
#
# Purpose: Detect TODO/FIXME comments without proper tracker references
# Usage: ./scripts/check-todo-violations.sh
# Exit codes:
#   0 = No violations found
#   1 = Anonymous TODOs detected
#   2 = Usage/configuration error
#
# This script enforces AGENTS.md §3.6 rule:
# - No TODO/FIXME stubs in engine logic
# - All TODO/FIXME must reference tracker items (AT-XXX, HCB-XXX) or be NOTEs
#
# Allowed patterns:
#   // NOTE(HCB-007): Explanation...
#   // TODO(AT-009): Implement after X is done
#   // FIXME(HCB-012): Known issue, tracked separately
#
# Forbidden patterns:
#   // TODO: something
#   // FIXME: broken
#   // TODO implement this

set -euo pipefail

# ANSI color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

VIOLATIONS_FOUND=0
VIOLATION_FILES=()

# Regex pattern for anonymous TODOs/FIXMEs
# Matches: "TODO:" or "FIXME:" NOT followed by (TRACKER-ID)
# Valid tracker IDs: AT-XXX, HCB-XXX
ANONYMOUS_TODO_PATTERN='(TODO|FIXME):\s*[^(]'

# Engine files to check (strict enforcement)
ENGINE_PATHS=(
    "internal/services/correlation_engine.go"
    "internal/rca"
    "internal/services/metrics_metadata_indexer.go"
    "internal/services/metrics_metadata_synchronizer.go"
)

# Exempted paths (tests may have TODO comments for incomplete test cases)
EXEMPTED_PATHS=(
    "*_test.go"
    "testdata"
    "cmd/otel-fintrans-simulator"
)

echo -e "${YELLOW}Checking for anonymous TODO/FIXME comments in engine code...${NC}"

# Function: Check if file is exempted
is_exempted() {
    local file="$1"
    for pattern in "${EXEMPTED_PATHS[@]}"; do
        if [[ "$file" == *"$pattern"* ]]; then
            return 0
        fi
    done
    return 1
}

# Function: Check file for anonymous TODOs
check_file() {
    local file="$1"
    
    if is_exempted "$file"; then
        return 0
    fi
    
    if [[ ! -f "$file" ]]; then
        return 0
    fi
    
    # Search for anonymous TODO/FIXME (not followed by tracker reference)
    local matches
    matches=$(grep -n -E "$ANONYMOUS_TODO_PATTERN" "$file" 2>/dev/null || true)
    
    if [[ -n "$matches" ]]; then
        local file_had_violations=0
        echo -e "${RED}✗ Anonymous TODO/FIXME found in: $file${NC}"
        echo "$matches" | while IFS=: read -r line_num line_content; do
            # Filter out NOTE() comments (these are allowed)
            if [[ ! "$line_content" =~ NOTE\( ]]; then
                echo -e "  Line $line_num: ${line_content:0:100}"
                file_had_violations=1
            fi
        done
        
        # Count violations from this file
        local count
        count=$(echo "$matches" | grep -v "NOTE(" | wc -l | tr -d ' ')
        if [[ $count -gt 0 ]]; then
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + count))
            VIOLATION_FILES+=("$file")
        fi
    fi
    
    return 0
}

# Check all engine files
for path_pattern in "${ENGINE_PATHS[@]}"; do
    if [[ -d "$path_pattern" ]]; then
        # Directory - check all .go files
        for file in "$path_pattern"/*.go; do
            check_file "$file"
        done
    elif [[ -f "$path_pattern" ]]; then
        # Single file
        check_file "$path_pattern"
    else
        # Glob pattern
        for file in $path_pattern; do
            check_file "$file"
        done
    fi
done

# Report results
echo ""
echo "=============================================="
if [[ $VIOLATIONS_FOUND -eq 0 ]]; then
    echo -e "${GREEN}✓ No anonymous TODO/FIXME comments detected!${NC}"
    echo "All engine code follows AGENTS.md §3.6 TODO rules."
    exit 0
else
    echo -e "${RED}✗ Found $VIOLATIONS_FOUND anonymous TODO/FIXME comment(s)${NC}"
    echo ""
    echo "Violating files:"
    for file in "${VIOLATION_FILES[@]}"; do
        echo "  - $file"
    done
    echo ""
    echo "AGENTS.md §3.6 Rule:"
    echo "  - No TODO/FIXME stubs without tracker references in engine logic"
    echo ""
    echo "Fix by:"
    echo "  1. Replace with NOTE(TRACKER-ID): Explanation"
    echo "     Example: // NOTE(HCB-007): Caching planned for later optimization"
    echo ""
    echo "  2. Reference tracker item:"
    echo "     Example: // TODO(AT-009): Implement partial correlation"
    echo ""
    echo "  3. Implement the TODO immediately (preferred for engine code)"
    echo ""
    echo "Valid tracker prefixes: AT-XXX (action tracker), HCB-XXX (hardcode blunders)"
    exit 1
fi
