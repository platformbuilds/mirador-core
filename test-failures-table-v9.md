# API Test Failures Report - Mirador Core v9.0.0

## Failed Tests Summary

The following table provides detailed information about failed API tests, including the specific endpoints, error reasons, and suggested fixes for the RBAC-enabled Mirador Core.

| Test Name | API Endpoint | Expected Status | Actual Status | Error Reason | Suggested Fix |
|-----------|--------------|-----------------|---------------|--------------|---------------|
| Validate Default Tenant Exists | `http://localhost:8010/api/v1/tenants/PLATFORMBUILDS` | 200 | 403 | Expected status 200, got 403 | Access forbidden. Check user permissions and RBAC roles. |
| Validate Default Admin User Exists | `http://localhost:8010/api/v1/users/aarvee` | 200 | 403 | Expected status 200, got 403 | Access forbidden. Check user permissions and RBAC roles. |
| Validate Global Admin Role Exists | `http://localhost:8010/api/v1/rbac/roles/global_admin` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Validate Tenant Admin Role Exists | `http://localhost:8010/api/v1/rbac/roles/tenant_admin` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Validate Tenant Editor Role Exists | `http://localhost:8010/api/v1/rbac/roles/tenant_editor` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Validate Tenant Guest Role Exists | `http://localhost:8010/api/v1/rbac/roles/tenant_guest` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Validate Admin Tenant-User Association | `http://localhost:8010/api/v1/tenants/PLATFORMBUILDS/users/aarvee` | 200 | 403 | Expected status 200, got 403 | Access forbidden. Check user permissions and RBAC roles. |

## Common Issues and Solutions

### Authentication & Authorization
- **401 Unauthorized**: Ensure valid JWT token is provided in Authorization header
- **403 Forbidden**: Check user permissions and RBAC roles for the requested operation
- **Bootstrap Required**: Run bootstrap to create default tenant, admin user, and roles

### Multi-Tenant Issues
- **Tenant Isolation**: Users cannot access data from other tenants
- **Tenant Context**: Ensure correct `x-tenant-id` header is set
- **Tenant Permissions**: Verify user has appropriate tenant-level roles

### RBAC Configuration
- **Role Not Found**: Ensure required roles exist (global_admin, tenant_admin, etc.)
- **Permission Denied**: Check role bindings and permission assignments
- **Policy Evaluation**: Verify RBAC policies are correctly configured

### Service Dependencies
- **Weaviate Down**: Ensure Weaviate vector database is running and accessible
- **Valkey Down**: Ensure Valkey in-memory database is running for sessions
- **Schema Missing**: Run bootstrap or schemactl to deploy Weaviate schemas

### Federation (Not Implemented in v9.0.0)
- **501 Not Implemented**: SAML/OIDC endpoints are placeholders in current version
- **Federation Disabled**: Enterprise federation features require additional implementation

