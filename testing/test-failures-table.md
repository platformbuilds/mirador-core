# API Test Failures Report

**Report Generated:** 19 October 2025, 09:04:49 UTC

## Failed Tests Summary

The following table provides detailed information about failed API tests, including the specific endpoints, error reasons, and suggested fixes.

| Test Name | API Endpoint | Expected Status | Actual Status | Error Reason | Suggested Fix |
|-----------|--------------|-----------------|---------------|--------------|---------------|
| Search Traces | `http://localhost:8010/api/v1/traces/search` | 200 | 400 | Expected status 200, got 400 | Provide required query parameters (start/end or _time in query) |
| Predict Engine Health | `http://localhost:8010/api/v1/predict/health` | 200 | 503 | Expected status 200, got 503 | Predict engine unhealthy. Check predict microservice status and dependencies |
| Get Active Models | `http://localhost:8010/api/v1/predict/models` | 200 | 500 | Expected status 200, got 500 | Predict engine not running. Start predict microservice or disable predict tests |
| Get Predicted Fractures | `http://localhost:8010/api/v1/predict/fractures` | 200 | 500 | Expected status 200, got 500 | Predict engine not running. Start predict microservice or disable predict tests |
| Get User Settings | `http://localhost:8010/api/v1/config/user-settings` | 200 | 500 | Expected status 200, got 500 | User management not configured. Check authentication and user store configuration |
| Get Metrics Schema | `http://localhost:8010/api/v1/schema/metrics` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Get Logs Fields Schema | `http://localhost:8010/api/v1/schema/logs/fields` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Get Traces Services Schema | `http://localhost:8010/api/v1/schema/traces/services` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Get Labels Schema | `http://localhost:8010/api/v1/schema/labels` | 200 | 404 | Expected status 200, got 404 | Schema endpoints may be disabled. Check feature flags or enable schema store |
| Legacy: Get Labels | `http://localhost:8010/labels` | 200 | 400 | Expected status 200, got 400 | Check request format and required parameters |
| Invalid Method | `http://localhost:8010/api/v1/health` | 405 | 404 | Expected status 405, got 404 | Endpoint may not support DELETE. Check OpenAPI spec for allowed methods |

## Common Issues and Solutions

### Microservice Dependencies
- **Predict Engine (503/500)**: Predict microservice not running. This is optional for basic functionality.
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

