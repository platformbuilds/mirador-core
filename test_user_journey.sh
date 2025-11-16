#!/bin/bash

# Test script for Mirador Core User Journey
# This script tests the authentication and unified query flow

set -e

BASE_URL="http://localhost:8010"
USERNAME="aarvee"
PASSWORD="password123"
TENANT="PLATFORMBUILDS"
TOTP_CODE="123456"

echo "🚀 Starting Mirador Core User Journey Test"
echo "=========================================="

# Step 1: Authenticate and get API key
echo "Step 1: Authenticating user and obtaining API key..."
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -H "x-tenant-id: $TENANT" \
  -d "{
    \"username\": \"$USERNAME\",
    \"password\": \"$PASSWORD\",
    \"totp_code\": \"$TOTP_CODE\",
    \"remember_me\": true
  }")

# Check if login was successful
if echo "$LOGIN_RESPONSE" | jq -e '.status == "success"' > /dev/null; then
    echo "✅ Login successful!"
    API_KEY=$(echo "$LOGIN_RESPONSE" | jq -r '.data.api_key')
    SESSION_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.session_token')
    echo "   API Key: ${API_KEY:0:10}..."
    echo "   Session Token: ${SESSION_TOKEN:0:10}..."
else
    echo "❌ Login failed!"
    echo "Response: $LOGIN_RESPONSE"
    exit 1
fi

echo ""

# Step 2: Test unified query (Metrics)
echo "Step 2: Testing unified metrics query..."
METRICS_QUERY_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/unified/query" \
  -H "X-API-Key: $API_KEY" \
  -H "x-tenant-id: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "test-metrics-query",
      "type": "metrics",
      "query": "gen",
      "tenant_id": "'$TENANT'",
      "start_time": "2025-11-15T19:00:00Z",
      "end_time": "2025-11-15T21:00:00Z",
      "timeout": "30s",
      "cache_options": {
        "enabled": true
      }
    }
  }')

# Check if query was successful
if echo "$METRICS_QUERY_RESPONSE" | jq -e '.result.status == "success"' > /dev/null; then
    echo "✅ Metrics query successful!"
    RECORD_COUNT=$(echo "$METRICS_QUERY_RESPONSE" | jq -r '.result.metadata.engine_results.metrics.record_count')
    EXEC_TIME=$(echo "$METRICS_QUERY_RESPONSE" | jq -r '.result.execution_time_ms')
    echo "   Records found: $RECORD_COUNT"
    echo "   Execution time: ${EXEC_TIME}ms"
else
    echo "❌ Metrics query failed!"
    echo "Response: $METRICS_QUERY_RESPONSE"
fi

echo ""

# Step 3: Test unified query (Logs)
echo "Step 3: Testing unified logs query..."
LOGS_QUERY_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/unified/query" \
  -H "X-API-Key: $API_KEY" \
  -H "x-tenant-id: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "test-logs-query",
      "type": "logs",
      "query": "telemetrygen",
      "tenant_id": "'$TENANT'",
      "start_time": "2025-11-15T19:00:00Z",
      "end_time": "2025-11-15T21:00:00Z",
      "timeout": "30s",
      "parameters": {
        "limit": 10
      }
    }
  }')

# Check if query was successful
if echo "$LOGS_QUERY_RESPONSE" | jq -e '.result.status == "success"' > /dev/null; then
    echo "✅ Logs query successful!"
    RECORD_COUNT=$(echo "$LOGS_QUERY_RESPONSE" | jq -r '.result.metadata.engine_results.logs.record_count')
    EXEC_TIME=$(echo "$LOGS_QUERY_RESPONSE" | jq -r '.result.execution_time_ms')
    echo "   Records found: $RECORD_COUNT"
    echo "   Execution time: ${EXEC_TIME}ms"
else
    echo "❌ Logs query failed!"
    echo "Response: $LOGS_QUERY_RESPONSE"
fi

echo ""

# Step 4: Test API key validation
echo "Step 4: Validating API key..."
VALIDATE_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/validate" \
  -H "X-API-Key: $API_KEY" \
  -H "x-tenant-id: $TENANT" \
  -H "Authorization: Bearer $SESSION_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "token": "'$API_KEY'"
  }')

if echo "$VALIDATE_RESPONSE" | jq -e '.data.valid == true' > /dev/null; then
    echo "✅ API key validation successful!"
    USER_ID=$(echo "$VALIDATE_RESPONSE" | jq -r '.data.user_id')
    ROLES=$(echo "$VALIDATE_RESPONSE" | jq -r '.data.roles | join(", ")')
    echo "   User ID: $USER_ID"
    echo "   Roles: $ROLES"
else
    echo "❌ API key validation failed!"
    echo "Response: $VALIDATE_RESPONSE"
fi

echo ""

# Step 5: Test unified health check
echo "Step 5: Checking unified query health..."
HEALTH_RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/unified/health" \
  -H "X-API-Key: $API_KEY" \
  -H "x-tenant-id: $TENANT")

if echo "$HEALTH_RESPONSE" | jq -e '.overall_health == "healthy"' > /dev/null; then
    echo "✅ Unified query health check passed!"
    ENGINES=$(echo "$HEALTH_RESPONSE" | jq -r '.engine_health // {} | keys | join(", ")')
    echo "   Engines: $ENGINES"
else
    echo "❌ Health check failed!"
    echo "Response: $HEALTH_RESPONSE"
fi

echo ""
echo "🎉 User journey test completed successfully!"
echo "=========================================="