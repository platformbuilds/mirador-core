#!/usr/bin/env bash
# Deploy RBAC Schema to Weaviate
#
# This script deploys the RBAC schema classes to a running Weaviate instance.
# It creates all necessary classes for the Multi-Tenant RBAC system.

set -e

# Configuration
WEAVIATE_URL="${WEAVIATE_URL:-http://localhost:8080}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🚀 Deploying RBAC Schema to Weaviate"
echo "   Weaviate URL: $WEAVIATE_URL"
echo ""

# Check if Weaviate is accessible
echo "📡 Checking Weaviate connectivity..."
if ! curl -s -f "$WEAVIATE_URL/v1/meta" > /dev/null; then
    echo "❌ Error: Cannot connect to Weaviate at $WEAVIATE_URL"
    echo "   Please ensure Weaviate is running:"
    echo "   docker-compose -f deployments/localdev/docker-compose.yaml up -d weaviate"
    exit 1
fi
echo "✅ Weaviate is accessible"
echo ""

# Function to create a schema class
create_class() {
    local class_name=$1
    local schema_json=$2
    
    echo "📝 Creating class: $class_name"
    
    # Check if class already exists
    if curl -s "$WEAVIATE_URL/v1/schema/$class_name" | grep -q "\"class\":\"$class_name\""; then
        echo "   ⚠️  Class $class_name already exists, skipping..."
        return 0
    fi
    
    # Create the class
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$schema_json" \
        "$WEAVIATE_URL/v1/schema")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        echo "   ✅ Created successfully"
    else
        echo "   ❌ Failed to create (HTTP $http_code)"
        echo "   Response: $body"
        return 1
    fi
}

# RBAC Schema Classes

# 1. RBACTenant
echo "Creating RBAC Schema Classes..."
echo ""

create_class "RBACTenant" '{
  "class": "RBACTenant",
  "vectorizer": "none",
  "properties": [
    {"name": "name", "dataType": ["text"]},
    {"name": "displayName", "dataType": ["text"]},
    {"name": "description", "dataType": ["text"]},
    {"name": "status", "dataType": ["text"]},
    {"name": "adminEmail", "dataType": ["text"]},
    {"name": "adminName", "dataType": ["text"]},
    {"name": "features", "dataType": ["text[]"]},
    {"name": "tags", "dataType": ["text[]"]},
    {"name": "isSystem", "dataType": ["boolean"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 2. RBACUser
create_class "RBACUser" '{
  "class": "RBACUser",
  "vectorizer": "none",
  "properties": [
    {"name": "email", "dataType": ["text"]},
    {"name": "username", "dataType": ["text"]},
    {"name": "fullName", "dataType": ["text"]},
    {"name": "globalRole", "dataType": ["text"]},
    {"name": "status", "dataType": ["text"]},
    {"name": "emailVerified", "dataType": ["boolean"]},
    {"name": "avatar", "dataType": ["text"]},
    {"name": "phone", "dataType": ["text"]},
    {"name": "timezone", "dataType": ["text"]},
    {"name": "language", "dataType": ["text"]},
    {"name": "lastLoginAt", "dataType": ["date"]},
    {"name": "tags", "dataType": ["text[]"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 3. RBACTenantUser
create_class "RBACTenantUser" '{
  "class": "RBACTenantUser",
  "vectorizer": "none",
  "properties": [
    {"name": "tenantId", "dataType": ["text"]},
    {"name": "userId", "dataType": ["text"]},
    {"name": "tenantRole", "dataType": ["text"]},
    {"name": "status", "dataType": ["text"]},
    {"name": "invitedBy", "dataType": ["text"]},
    {"name": "invitedAt", "dataType": ["date"]},
    {"name": "acceptedAt", "dataType": ["date"]},
    {"name": "additionalPermissions", "dataType": ["text[]"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 4. RBACMiradorAuth
create_class "RBACMiradorAuth" '{
  "class": "RBACMiradorAuth",
  "vectorizer": "none",
  "properties": [
    {"name": "userId", "dataType": ["text"]},
    {"name": "username", "dataType": ["text"]},
    {"name": "email", "dataType": ["text"]},
    {"name": "passwordHash", "dataType": ["text"]},
    {"name": "salt", "dataType": ["text"]},
    {"name": "totpSecret", "dataType": ["text"]},
    {"name": "totpEnabled", "dataType": ["boolean"]},
    {"name": "backupCodes", "dataType": ["text[]"]},
    {"name": "isActive", "dataType": ["boolean"]},
    {"name": "passwordChangedAt", "dataType": ["date"]},
    {"name": "lastLoginAt", "dataType": ["date"]},
    {"name": "failedLoginCount", "dataType": ["int"]},
    {"name": "lockedUntil", "dataType": ["date"]},
    {"name": "requirePasswordChange", "dataType": ["boolean"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 5. RBACAuthConfig
create_class "RBACAuthConfig" '{
  "class": "RBACAuthConfig",
  "vectorizer": "none",
  "properties": [
    {"name": "tenantId", "dataType": ["text"]},
    {"name": "defaultBackend", "dataType": ["text"]},
    {"name": "enabledBackends", "dataType": ["text[]"]},
    {"name": "require2fa", "dataType": ["boolean"]},
    {"name": "totpIssuer", "dataType": ["text"]},
    {"name": "sessionTimeoutMinutes", "dataType": ["int"]},
    {"name": "maxConcurrentSessions", "dataType": ["int"]},
    {"name": "allowRememberMe", "dataType": ["boolean"]},
    {"name": "rememberMeDays", "dataType": ["int"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 6. RBACRole
create_class "RBACRole" '{
  "class": "RBACRole",
  "vectorizer": "none",
  "properties": [
    {"name": "tenantId", "dataType": ["text"]},
    {"name": "name", "dataType": ["text"]},
    {"name": "description", "dataType": ["text"]},
    {"name": "permissions", "dataType": ["text[]"]},
    {"name": "isSystem", "dataType": ["boolean"]},
    {"name": "parentRoles", "dataType": ["text[]"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 7. RBACPermission
create_class "RBACPermission" '{
  "class": "RBACPermission",
  "vectorizer": "none",
  "properties": [
    {"name": "tenantId", "dataType": ["text"]},
    {"name": "resource", "dataType": ["text"]},
    {"name": "action", "dataType": ["text"]},
    {"name": "scope", "dataType": ["text"]},
    {"name": "description", "dataType": ["text"]},
    {"name": "resourcePattern", "dataType": ["text"]},
    {"name": "isSystem", "dataType": ["boolean"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 8. RBACGroup
create_class "RBACGroup" '{
  "class": "RBACGroup",
  "vectorizer": "none",
  "properties": [
    {"name": "tenantId", "dataType": ["text"]},
    {"name": "name", "dataType": ["text"]},
    {"name": "description", "dataType": ["text"]},
    {"name": "members", "dataType": ["text[]"]},
    {"name": "roles", "dataType": ["text[]"]},
    {"name": "parentGroups", "dataType": ["text[]"]},
    {"name": "isSystem", "dataType": ["boolean"]},
    {"name": "maxMembers", "dataType": ["int"]},
    {"name": "memberSyncEnabled", "dataType": ["boolean"]},
    {"name": "externalId", "dataType": ["text"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 9. RBACRoleBinding
create_class "RBACRoleBinding" '{
  "class": "RBACRoleBinding",
  "vectorizer": "none",
  "properties": [
    {"name": "tenantId", "dataType": ["text"]},
    {"name": "subjectType", "dataType": ["text"]},
    {"name": "subjectId", "dataType": ["text"]},
    {"name": "roleId", "dataType": ["text"]},
    {"name": "scope", "dataType": ["text"]},
    {"name": "resourceId", "dataType": ["text"]},
    {"name": "expiresAt", "dataType": ["date"]},
    {"name": "notBefore", "dataType": ["date"]},
    {"name": "precedence", "dataType": ["text"]},
    {"name": "justification", "dataType": ["text"]},
    {"name": "approvedBy", "dataType": ["text"]},
    {"name": "approvedAt", "dataType": ["date"]},
    {"name": "createdAt", "dataType": ["date"]},
    {"name": "updatedAt", "dataType": ["date"]},
    {"name": "createdBy", "dataType": ["text"]},
    {"name": "updatedBy", "dataType": ["text"]}
  ]
}'

# 10. RBACAuditLog
create_class "RBACAuditLog" '{
  "class": "RBACAuditLog",
  "vectorizer": "none",
  "properties": [
    {"name": "tenantId", "dataType": ["text"]},
    {"name": "timestamp", "dataType": ["date"]},
    {"name": "subjectId", "dataType": ["text"]},
    {"name": "subjectType", "dataType": ["text"]},
    {"name": "action", "dataType": ["text"]},
    {"name": "resource", "dataType": ["text"]},
    {"name": "resourceId", "dataType": ["text"]},
    {"name": "result", "dataType": ["text"]},
    {"name": "severity", "dataType": ["text"]},
    {"name": "source", "dataType": ["text"]},
    {"name": "correlationId", "dataType": ["text"]},
    {"name": "retentionClass", "dataType": ["text"]}
  ]
}'

echo ""
echo "✅ RBAC Schema Deployment Complete!"
echo ""
echo "📊 Schema Summary:"
echo "   - RBACTenant: Multi-tenant isolation"
echo "   - RBACUser: Global user entity"
echo "   - RBACTenantUser: Tenant-user associations"
echo "   - RBACMiradorAuth: Local authentication credentials"
echo "   - RBACAuthConfig: Authentication configuration"
echo "   - RBACRole: Tenant-scoped roles"
echo "   - RBACPermission: Granular permissions"
echo "   - RBACGroup: User groups"
echo "   - RBACRoleBinding: Role assignments"
echo "   - RBACAuditLog: Audit trail"
echo ""
echo "🔍 Verify deployment:"
echo "   curl -s $WEAVIATE_URL/v1/schema | jq '.classes[] | select(.class | startswith(\"RBAC\")) | .class'"
echo ""
