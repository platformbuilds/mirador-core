# User Journeys: Authentication and Unified Query

## User Authentication Journey

### Preconditions
- **Base URL**: The MIRADOR-CORE API server URL (e.g., `http://localhost:8010` for local development, or production URL)
- **Credentials**: Valid username and password from LDAP/AD or SSO system
- **Environment**: Ensure the server is running and accessible

### Step-by-Step Flow

#### Step 1 – Obtain Access Token
**Purpose**: Authenticate the user and obtain an API key for subsequent API calls. API keys are the primary authentication method for all programmatic access.

**HTTP Method + URL Path**: `POST /api/v1/auth/login`

**Required Headers**:
- `Content-Type: application/json`

**Request Body**:
```json
{
  "username": "your-username",
  "password": "your-password",
  "totp_code": "123456",
  "remember_me": true
}
```

**Example cURL Request**:
```bash
curl -X POST http://localhost:8010/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john.doe",
    "password": "secure-password",
    "totp_code": "123456",
    "remember_me": true
  }'
```

**Example Response**:
```json
{
  "status": "success",
  "data": {
    "session_token": "sess_abc123def456",
    "api_key": "mrk_1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef12",
    "key_prefix": "mrk_1a2b",
    "user_id": "john.doe",
    "roles": ["mirador-user", "mirador-admin"],
    "expires_at": "2025-11-16T10:30:00Z",
    "warning": "Store this API key securely. It will not be shown again."
  }
}
```

**Notes**:
- The `api_key` is the primary authentication method for all programmatic API access
- The `session_token` is provided for UI/web applications that need session-based authentication
- API keys start with the "mrk_" prefix (MIRADOR Key identifier) for easy identification
- Tokens expire after 24 hours of inactivity
- RBAC roles are extracted from LDAP group memberships

### Error Handling & Best Practices

#### Common Authentication Errors

**Invalid Credentials**:
- **Status Code**: `401 Unauthorized`
- **Response**:
```json
{
  "status": "error",
  "error":  "Authentication failed"
}
```
- **Client Action**: Prompt user to re-enter credentials

**API Key/Token Expired**:
- **Status Code**: `401 Unauthorized`
- **Response**:
```json
{
  "status": "error",
  "error":  "Invalid authentication token"
}
```
- **Client Action**: Re-authenticate using login endpoint to obtain a new API key

**RBAC/Permission Issues**:
- **Status Code**: `403 Forbidden`
- **Response**:
```json
{
  "status": "error",
  "error": "Insufficient permissions"
}
```
- **Client Action**: Check user roles or contact administrator

#### Best Practices
- Store tokens securely (e.g., in secure storage, not localStorage for web apps)
- Implement token refresh logic before expiration
- Handle token validation failures by redirecting to login
- Use HTTPS in production environments
- Implement proper logout by calling `POST /api/v1/auth/logout` with the session token (for UI applications)

## API Key Management

**Note**: API keys are the recommended authentication method for all REST API calls (except login). They provide secure, programmatic access without storing user credentials.

### Generate API Key
**Purpose**: Create a new API key for programmatic access to all REST APIs.

**HTTP Method + URL Path**: `POST /api/v1/auth/apikeys`

**Required Headers**:
- `Authorization: Bearer <existing_api_key>` or `X-API-Key: <existing_api_key>`
- `Content-Type: application/json`

**Request Body**:
```json
{
  "name": "Production API Key",
  "description": "Key for production deployments",
  "expires_at": "2026-11-15T00:00:00Z",
  "scopes": ["metrics.read", "logs.read"]
}
```

**Example Response**:
```json
{
  "status": "success",
  "data": {
    "api_key": "mrk_1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef12",
    "key_prefix": "mrk_1a2b",
    "name": "Production API Key",
    "expires_at": "2026-11-15T00:00:00Z",
    "scopes": ["metrics.read", "logs.read"],
    "warning": "Store this API key securely. It will not be shown again."
  }
}
```

### List API Keys
**Purpose**: Retrieve all API keys for the authenticated user.

**HTTP Method + URL Path**: `GET /api/v1/auth/apikeys`

**Required Headers**:
- `Authorization: Bearer <token>` or `X-API-Key: <api_key>`

**Example Response**:
```json
{
  "status": "success",
  "data": {
    "api_keys": [
      {
        "id": "key-1",
        "name": "Production API Key",
        "prefix": "mrk_abcd",
        "expires_at": null,
        "scopes": ["metrics.read", "logs.read"],
        "created_at": "2025-11-15T10:00:00Z",
        "last_used": "2025-11-15T12:00:00Z"
      }
    ],
    "total": 1
  }
}
```

### Revoke API Key
**Purpose**: Deactivate an API key.

**HTTP Method + URL Path**: `DELETE /api/v1/auth/apikeys/{keyId}`

**Required Headers**:
- `Authorization: Bearer <token>` or `X-API-Key: <api_key>`

**Example Response**:
```json
{
  "status": "success",
  "data": {
    "message": "API key revoked successfully"
  }
}
```

### Validate Token
**Purpose**: Validate an API key (primarily for testing API key validity).

**HTTP Method + URL Path**: `POST /api/v1/auth/validate`

**Request Body**:
```json
{
  "token": "mrk_1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef12"
}
```

**Example Response**:
```json
{
  "status": "success",
  "data": {
    "valid": true,
    "type": "api_key",
    "user_id": "john.doe",
    "roles": ["mirador-user"]
  }
}
```

## Unified Query Journey (after Authentication)

### Authentication & Authorization Requirements

**🔐 REQUIRED: Authentication**
All unified query endpoints require valid authentication. For **programmatic API access**, you must use API keys only.

**✅ RECOMMENDED: API Key Authentication**
```bash
# Use API keys for all REST API calls (except login)
-H "Authorization: Bearer <api_key>"
# OR
-H "X-API-Key: <api_key>"
```

**⚠️ IMPORTANT: Authentication Method Guidelines**
- **Login Endpoint Only**: `POST /api/v1/auth/login` accepts username/password
- **All Other APIs**: Use API keys only (not session tokens or JWT for programmatic access)
- **UI Applications**: May use session tokens from login response
- **API Keys**: Start with `mrk_` prefix (MIRADOR Key identifier) and are returned only during creation

**🛡️ REQUIRED: RBAC Permissions**
All unified query endpoints require the `unified.read` permission. Users must have this permission assigned through their roles.

### Preconditions
- **Valid API Key**: API key obtained from authentication (starts with `mrk_` prefix for MIRADOR Key identification)
- **Authorization Header**: `Authorization: Bearer <api_key>` or `X-API-Key: <api_key>`
- **RBAC Permission**: User must have `unified.read` permission
- **Base URL**: Same as authentication endpoint

### Step-by-Step Flow

#### Step 1 – Prepare Unified Query Request
**Purpose**: Construct a query request for metrics, logs, traces, or correlation data.

**HTTP Method + URL Path**: `POST /api/v1/unified/query`

**Required Headers**:
- `Authorization: Bearer <api_key>` OR `X-API-Key: <api_key>`
- `Content-Type: application/json`

**Request Body Structure**:
```json
{
  "query": {
    "id": "unique-query-id",
    "type": "metrics|logs|traces|correlation",
    "query": "query-string-appropriate-for-type",
    "start_time": "2025-11-15T00:00:00Z",
    "end_time": "2025-11-15T01:00:00Z",
    "timeout": "30s",
    "parameters": {},
    "correlation_options": {},
    "cache_options": {
      "enabled": true,
      "ttl": "5m"
    }
  }
}
```

#### Step 2 – Execute Unified Query
**Purpose**: Send the query to MIRADOR-CORE and receive results.

**Example cURL Request (Metrics Query)**:
```bash
# Note: Use API keys for programmatic access (not session tokens)
# Replace 'mrk_...' with your actual API key from login response
# API keys start with 'mrk_' (MIRADOR Key identifier)
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer mrk_1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef12" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "metrics-query-001",
      "type": "metrics",
      "query": "http_requests_total{job=\"api\"}",
      "start_time": "2025-11-15T00:00:00Z",
      "end_time": "2025-11-15T01:00:00Z",
      "timeout": "30s",
      "cache_options": {
        "enabled": true,
        "ttl": "5m"
      }
    }
  }'
```

**Example Response (Metrics)**:
```json
{
  "result": {
    "query_id": "metrics-query-001",
    "type": "metrics",
    "status": "success",
    "data": {
      "resultType": "vector",
      "result": [
        {
          "metric": {
            "__name__": "http_requests_total",
            "job": "api",
            "instance": "pod-1"
          },
          "value": [1690000000, "1234"]
        }
      ]
    },
    "metadata": {
      "engine_results": {
        "metrics": {
          "engine": "metrics",
          "status": "success",
          "record_count": 1,
          "execution_time_ms": 45,
          "data_source": "victoriametrics"
        }
      },
      "total_records": 1,
      "data_sources": ["victoriametrics"]
    },
    "execution_time_ms": 50,
    "cached": false
  }
}
```

**Example cURL Request (Logs Query)**:
```bash
curl -X POST http://localhost:8010/api/v1/unified/query \
  -H "Authorization: Bearer mrk_1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef12" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "logs-query-001",
      "type": "logs",
      "query": "service.name:api AND level:error",
      "start_time": "2025-11-15T00:00:00Z",
      "end_time": "2025-11-15T01:00:00Z",
      "timeout": "30s",
      "parameters": {
        "limit": 1000
      }
    }
  }'
```

**Notes**:
- Query types: `metrics` (MetricsQL/PromQL), `logs` (Lucene), `traces` (Jaeger filters), `correlation` (cross-engine)
- Time ranges are optional but recommended for performance
- Cache options help reduce latency for repeated queries
- RBAC permissions are enforced based on user roles (requires "unified.read" permission)

### End-to-End Narrative
1. **Client authenticates** via `POST /api/v1/auth/login` with username/password (credentials only used here)
2. **Receives API key** (primary auth method) and session token (for UI use) in response
3. **Stores API key securely** for subsequent requests (API key is the primary auth method for all programmatic APIs)
4. **Uses API key** for all subsequent REST API calls (session tokens are for UI applications only)
5. **Constructs unified query** with appropriate type and parameters
6. **Sends query** via `POST /api/v1/unified/query` with Authorization header containing API key
7. **Receives results** in standardized format with metadata
8. **Handles pagination/limits** if result sets are large
9. **Processes correlations** if correlation query type was used
10. **Implements retry logic** for transient failures

### Error Handling for Unified Query

#### Authentication & Authorization Errors

**Missing Authentication**:
- **Status Code**: `401 Unauthorized`
- **Response**:
```json
{
  "error": "Authentication required"
}
```
- **Client Action**: Include valid authentication token in request headers

**Invalid/Expired Token**:
- **Status Code**: `401 Unauthorized`
- **Response**:
```json
{
  "error": "Invalid authentication token"
}
```
- **Client Action**: Re-authenticate using login endpoint

**Insufficient RBAC Permissions**:
- **Status Code**: `403 Forbidden`
- **Response**:
```json
{
  "status": "error",
  "error": "Access denied",
  "reason": "permission_denied",
  "required_permissions": ["unified.read"]
}
```
- **Client Action**: Request `unified.read` permission from administrator

#### Common Query Errors

**Invalid Query Format**:
- **Status Code**: `400 Bad Request`
- **Response**:
```json
{
  "error": "Invalid request format",
  "details": "query.type: must be one of [metrics logs traces correlation]"
}
```
- **Client Action**: Validate query parameters before sending

**Query Execution Failed**:
- **Status Code**: `500 Internal Server Error`
- **Response**:
```json
{
  "error": "Query execution failed",
  "details": "connection timeout",
  "query_id": "query-123"
}
```
- **Client Action**: Implement exponential backoff retry, log for debugging

**Partial Results**:
- **Status Code**: `200 OK` (but with status: "partial")
- **Response**:
```json
{
  "result": {
    "status": "partial",
    "metadata": {
      "engine_results": {
        "metrics": { "status": "success" },
        "logs": { "status": "error", "error": "timeout" }
      }
    }
  }
}
```
- **Client Action**: Process available data, log partial failure

#### Best Practices
- Validate query syntax client-side when possible
- Implement proper timeout handling (respect server timeout)
- Use correlation IDs for request tracing
- Handle rate limiting (429 responses) with backoff
- Cache results appropriately based on cache_options
- Monitor execution_time_ms for performance issues
- Parse metadata for detailed error information per engine

## Additional Unified Query Endpoints

**Note**: All unified query endpoints require authentication and `unified.read` RBAC permission.

### Correlation Queries
**Purpose**: Execute correlation queries across multiple engines.

**HTTP Method + URL Path**: `POST /api/v1/unified/correlation`

**Authentication**: Required (same as unified query)
**RBAC Permission**: `unified.read`
**Request Body**: Same as unified query but with `type: "correlation"` and correlation options.

### Unified Search
**Purpose**: Perform search across all engines.

**HTTP Method + URL Path**: `POST /api/v1/unified/search`

**Authentication**: Required (same as unified query)
**RBAC Permission**: `unified.read`
**Request Body**: Same as unified query.

### Query Metadata
**Purpose**: Get information about supported query capabilities.

**HTTP Method + URL Path**: `GET /api/v1/unified/metadata`

**Authentication**: Required (same as unified query)
**RBAC Permission**: `unified.read`

**Example Response**:
```json
{
  "supported_engines": ["metrics", "logs", "traces", "correlation"],
  "query_capabilities": {
    "metrics": ["promql", "metricsql"],
    "logs": ["lucene"],
    "traces": ["jaeger"],
    "correlation": ["cross-engine"]
  },
  "cache_capabilities": {
    "supported": true,
    "default_ttl": "5m",
    "max_ttl": "1h"
  }
}
```

### Health Check
**Purpose**: Check health status of all engines.

**HTTP Method + URL Path**: `GET /api/v1/unified/health`

**Authentication**: Required (same as unified query)
**RBAC Permission**: `unified.read`

**Example Response**:
```json
{
  "overall_health": "healthy",
  "engine_health": {
    "metrics": "healthy",
    "logs": "healthy",
    "traces": "healthy",
    "correlation": "healthy"
  },
  "last_checked": "2025-11-15T12:00:00Z"
}
```

### UQL (Unified Query Language) Endpoints

**Note**: UQL endpoints also require authentication and `unified.read` permission.

**Execute UQL Query**: `POST /api/v1/uql/query`
**Validate UQL Syntax**: `POST /api/v1/uql/validate`
**Explain UQL Plan**: `POST /api/v1/uql/explain`

### Query Statistics
**Purpose**: Get statistics about unified query operations.

**HTTP Method + URL Path**: `GET /api/v1/unified/stats`

**Authentication**: Required (same as unified query)
**RBAC Permission**: `unified.read`

**Example Response**:
```json
{
  "unified_query_engine": {
    "health": "healthy",
    "supported_engines": ["metrics", "logs", "traces", "correlation"],
    "cache_enabled": true,
    "cache_default_ttl": "5m",
    "cache_max_ttl": "1h",
    "last_health_check": "2025-11-15T12:00:00Z"
  },
  "engines": {
    "metrics": "healthy",
    "logs": "healthy",
    "traces": "healthy",
    "correlation": "healthy"
  }
}
```

## Authentication Guidelines Summary

### For Programmatic API Access (Recommended)
1. **Login once** with username/password to get API key
2. **Use API keys** for all subsequent API calls
3. **Store API keys securely** (never in code or version control)
4. **API keys start with `mrk_`** prefix (MIRADOR Key identifier) for easy identification

### Authentication Methods by Use Case
- **Login Endpoint**: Username + password only
- **REST APIs**: API keys only (Authorization Bearer or X-API-Key header)
- **UI Applications**: Session tokens (from login response)
- **OAuth/OIDC**: JWT tokens (when enabled)

### Security Best Practices
- Use API keys for programmatic access (more secure than storing credentials)
- Rotate API keys regularly
- Use different API keys for different applications/services
- Monitor API key usage and revoke unused keys
- Never share API keys in code, logs, or version control</content>
<parameter name="filePath">/Users/aarvee/repos/github/public/miradorstack/mirador-core/docs/user-journeys.md