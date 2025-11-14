#!/bin/bash

API_BASE="http://localhost:8010/api/v1"
TENANT_ID="PLATFORMBUILDS"
username="aarvee"
password="password123"

echo "Testing authentication..."

# First authentication
echo "First auth:"
login_data=$(printf '{"username": "%s", "password": "%s"}' "$username" "$password")
echo "login_data: $login_data"

response1=$(curl -s -X POST "$API_BASE/auth/login" \
    -H "Content-Type: application/json" \
    -H "x-tenant-id: $TENANT_ID" \
    -d "$login_data")
echo "Response 1: $response1"

# Extract token if successful
if echo "$response1" | jq -e '.status == "success"' >/dev/null 2>&1; then
    AUTH_TOKEN=$(echo "$response1" | jq -r '.data.jwt_token')
    echo "Got token: ${AUTH_TOKEN:0:50}..."
else
    echo "First auth failed"
    exit 1
fi

# Second authentication
echo "Second auth:"
response2=$(curl -s -X POST "$API_BASE/auth/login" \
    -H "Content-Type: application/json" \
    -H "x-tenant-id: $TENANT_ID" \
    -d "$login_data")
echo "Response 2: $response2"

if echo "$response2" | jq -e '.status == "success"' >/dev/null 2>&1; then
    echo "Second auth succeeded"
else
    echo "Second auth failed"
fi
