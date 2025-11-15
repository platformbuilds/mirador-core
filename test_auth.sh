#!/bin/bash
API_BASE="http://localhost:8010/api/v1"
TENANT_ID="PLATFORMBUILDS"
login_data='{"username": "aarvee", "password": "password123"}'
echo "Running: curl -s -X POST \"$API_BASE/auth/login\" -H \"Content-Type: application/json\" -H \"x-tenant-id: $TENANT_ID\" -d \"$login_data\""
response=$(curl -s -X POST "$API_BASE/auth/login" -H "Content-Type: application/json" -H "x-tenant-id: $TENANT_ID" -d "$login_data")
echo "Response: $response"
if echo "$response" | jq -e '.status == "success"' >/dev/null 2>&1; then
    echo "Success"
else
    echo "Failed"
fi
