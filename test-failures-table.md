# API Test Failures Report

## Failed Tests Summary

The following table provides detailed information about failed API tests, including the specific endpoints, error reasons, and suggested fixes.

| Test Name | API Endpoint | Expected Status | Actual Status | Error Reason | Suggested Fix |
|-----------|--------------|-----------------|---------------|--------------|---------------|
| Get __name__ Label Values | `http://localhost:8010/api/v1/label/__name__/values` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Get Series | `http://localhost:8010/api/v1/series?match[]=up` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Unified Search | `http://localhost:8010/api/v1/unified/search` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |
| Unified Query with Filters | `http://localhost:8010/api/v1/unified/query` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |
| Sync Metrics Metadata | `http://localhost:8010/api/v1/metrics/sync` | 200 | 500 | Expected status 200, got 500 | Internal server error. Check service logs and microservice dependencies |
| Trigger Sync for Default Tenant | `http://localhost:8010/api/v1/metrics/sync/default` | 200 | 0 | Request failed: 000 | Check network connectivity and service availability |
| Update Sync Configuration | `http://localhost:8010/api/v1/metrics/sync/config` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |
| Get RCA Correlations | `http://localhost:8010/api/v1/rca/correlations` | 200 | 500 | Expected status 200, got 500 | Internal server error. Check service logs and microservice dependencies |
| Get Failure Patterns | `http://localhost:8010/api/v1/rca/patterns` | 200 | 500 | Expected status 200, got 500 | Internal server error. Check service logs and microservice dependencies |
| Predict Engine Health | `http://localhost:8010/api/v1/predict/health` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Get Active Models | `http://localhost:8010/api/v1/predict/models` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Get Predicted Fractures | `http://localhost:8010/api/v1/predict/fractures` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Download Metrics Sample CSV | `http://localhost:8010/api/v1/schema/metrics/bulk/sample` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Download Log Fields Sample CSV | `http://localhost:8010/api/v1/schema/logs/fields/bulk/sample` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Download Trace Services Sample CSV | `http://localhost:8010/api/v1/schema/traces/services/bulk/sample` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Download Labels Sample CSV | `http://localhost:8010/api/v1/schema/labels/bulk/sample` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Create Metric Schema | `http://localhost:8010/api/v1/schema/metrics` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Create Log Field Schema | `http://localhost:8010/api/v1/schema/logs/fields` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Create Trace Service Schema | `http://localhost:8010/api/v1/schema/traces/services` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Create Trace Operation Schema | `http://localhost:8010/api/v1/schema/traces/operations` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Get Metric Schema | `http://localhost:8010/api/v1/schema/metrics/e2e_metric_1763101329` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Get Log Field Schema | `http://localhost:8010/api/v1/schema/logs/fields/e2e_field_1763101329` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Get Trace Service Schema | `http://localhost:8010/api/v1/schema/traces/services/e2e_service_1763101329` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Create Test Tenant | `http://localhost:8010/api/v1/tenants` | 201 | 403 | Expected status 201, got 403 | Unexpected status code. Check service logs and API documentation |
| List Tenants | `http://localhost:8010/api/v1/tenants` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Get Test Tenant | `http://localhost:8010/api/v1/tenants/e2e-test-tenant-1763101331` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Create Tenant-User Association | `http://localhost:8010/api/v1/tenants/e2e-test-tenant-1763101331/users` | 201 | 403 | Expected status 201, got 403 | Unexpected status code. Check service logs and API documentation |
| List Tenant Users | `http://localhost:8010/api/v1/tenants/e2e-test-tenant-1763101331/users` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Get Tenant-User Association | `http://localhost:8010/api/v1/tenants/e2e-test-tenant-1763101331/users/e2e-test-user-1763101332` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Update Tenant-User Association | `http://localhost:8010/api/v1/tenants/e2e-test-tenant-1763101331/users/e2e-test-user-1763101332` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Delete Tenant-User Association | `http://localhost:8010/api/v1/tenants/e2e-test-tenant-1763101331/users/e2e-test-user-1763101332` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Update Test Tenant | `http://localhost:8010/api/v1/tenants/e2e-test-tenant-1763101331` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Delete Test Tenant | `http://localhost:8010/api/v1/tenants/e2e-test-tenant-1763101331` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Create Invalid Tenant (should fail) | `http://localhost:8010/api/v1/tenants` | 400 | 403 | Expected status 400, got 403 | Unexpected status code. Check service logs and API documentation |
| Get Non-Existent Tenant (should fail) | `http://localhost:8010/api/v1/tenants/non-existent-tenant` | 404 | 403 | Expected status 404, got 403 | Unexpected status code. Check service logs and API documentation |
| Create First Tenant-User Association | `http://localhost:8010/api/v1/tenants/default/users` | 201 | 403 | Expected status 201, got 403 | Unexpected status code. Check service logs and API documentation |
| Create Duplicate Tenant-User Association (should fail) | `http://localhost:8010/api/v1/tenants/default/users` | 400 | 403 | Expected status 400, got 403 | Unexpected status code. Check service logs and API documentation |
| Cleanup Duplicate Association | `http://localhost:8010/api/v1/tenants/default/users/duplicate-user` | 200 | 403 | Expected status 200, got 403 | Unexpected status code. Check service logs and API documentation |
| Legacy: Get Series | `http://localhost:8010/series?match[]=up` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Legacy: Get Labels | `http://localhost:8010/labels` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |

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

