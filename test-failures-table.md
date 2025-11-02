# API Test Failures Report

## Failed Tests Summary

The following table provides detailed information about failed API tests, including the specific endpoints, error reasons, and suggested fixes.

| Test Name | API Endpoint | Expected Status | Actual Status | Error Reason | Suggested Fix |
|-----------|--------------|-----------------|---------------|--------------|---------------|
| Unified Search | `http://localhost:8010/api/v1/unified/search` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Unified Query with Filters | `http://localhost:8010/api/v1/unified/query` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |
| Unified Statistics | `http://localhost:8010/api/v1/unified/stats` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Search Metrics Metadata | `http://localhost:8010/api/v1/metrics/search` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Sync Metrics Metadata | `http://localhost:8010/api/v1/metrics/sync` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Metrics Metadata Health | `http://localhost:8010/api/v1/metrics/health` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Trigger Sync for Default Tenant | `http://localhost:8010/api/v1/metrics/sync/default` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Get Sync State for Default Tenant | `http://localhost:8010/api/v1/metrics/sync/default/state` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Get Sync Status for Default Tenant | `http://localhost:8010/api/v1/metrics/sync/default/status` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Update Sync Configuration | `http://localhost:8010/api/v1/metrics/sync/config` | 200 | 404 | Expected status 200, got 404 | Endpoint not found. Verify API version and endpoint path |
| Create Metric Schema | `http://localhost:8010/api/v1/schema/metrics` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Create Log Field Schema | `http://localhost:8010/api/v1/schema/logs/fields` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |
| Create Trace Service Schema | `http://localhost:8010/api/v1/schema/traces/services` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |
| Create Trace Operation Schema | `http://localhost:8010/api/v1/schema/traces/operations` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |
| Get Metric Schema | `http://localhost:8010/api/v1/schema/metrics/e2e_metric_1761396628` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Get Log Field Schema | `http://localhost:8010/api/v1/schema/logs/fields/e2e_field_1761396628` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Get Trace Service Schema | `http://localhost:8010/api/v1/schema/traces/services/e2e_service_1761396628` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Legacy: Get Labels | `http://localhost:8010/labels` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |

## Common Issues and Solutions

### Microservice Dependencies
- **RCA Engine**: Root Cause Analysis features require additional microservices.

### Configuration Issues
- **Schema Endpoints (404)**: Schema management may be disabled. Check feature flags.
- **User Settings (500)**: User management requires proper authentication configuration.

### API Usage
- **Series Endpoint (400)**: Requires `match[]` parameter, e.g., `?match[]=up`
- **Traces Search (400)**: Needs proper time range parameters

### Infrastructure
- **Network Issues**: Check service connectivity and firewall rules
- **Service Health**: Verify all required services are running and healthy

