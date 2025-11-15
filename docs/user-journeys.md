# MIRADOR-CORE User Journeys: Authentication and Unified Query

## User Authentication Journey

### Preconditions
- **Base URL**: The MIRADOR-CORE API server URL (e.g., `http://localhost:8010` for local development, or production URL)
- **Credentials**: Valid username and password from LDAP/AD or SSO system
- **Tenant Information**: Optional tenant ID (defaults to "default" if not provided)
- **Environment**: Ensure the server is running and accessible

### Step-by-Step Flow

#### Step 1 – Obtain Access Token
**Purpose**: Authenticate the user and obtain session and JWT tokens for subsequent API calls.

**HTTP Method + URL Path**: `POST /api/v1/auth/login`

**Required Headers**:
- `Content-Type: application/json`
- `x-tenant-id: <tenant-id>` (optional, defaults to "default")

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
  -H "x-tenant-id: default" \
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
    "jwt_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user_id": "john.doe",
    "tenant_id": "default",
    "roles": ["mirador-user", "mirador-admin"],
    "expires_at": "2025-11-16T10:30:00Z"
  }
}
```

**Notes**:
- The `session_token` is used for session-based authentication and stored in cache
- The `jwt_token` is a JWT token that can be used for stateless authentication
- Tokens expire after 24 hours of inactivity
- RBAC roles are extracted from LDAP group memberships
- Tenant ID is determined from LDAP OU or defaults to "default"

### Error Handling & Best Practices

#### Common Authentication Errors

**Invalid Credentials**:
- **Status Code**: `401 Unauthorized`
- **Response**:
```json
{
  "status": "error",
  "error": "Authentication failed"
}
```
- **Client Action**: Prompt user to re-enter credentials

**Missing or Invalid Tenant**:
- **Status Code**: `400 Bad Request`
- **Response**:
```json
{
  "status": "error",
  "error": "Tenant context required"
}
```
- **Client Action**: Verify tenant ID or use default

**Session/Token Expired**:
- **Status Code**: `401 Unauthorized`
- **Response**:
```json
{
  "status": "error",
  "error": "Invalid authentication token"
}
```
- **Client Action**: Re-authenticate using login endpoint

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
- Implement proper logout by calling `POST /api/v1/auth/logout` with the session token

## Unified Query Journey (after Authentication)

### Preconditions
- **Valid Token**: Session token or JWT token obtained from authentication
- **Tenant Context**: Valid tenant ID from authentication
- **Authorization Header**: `Authorization: Bearer <token>` for all requests
- **Base URL**: Same as authentication endpoint

### Step-by-Step Flow

#### Step 1 – Prepare Unified Query Request
**Purpose**: Construct a query request for metrics, logs, traces, or correlation data.

**HTTP Method + URL Path**: `POST /unified/query`

**Required Headers**:
- `Authorization: Bearer <session_token or jwt_token>`
- `Content-Type: application/json`

**Request Body Structure**:
```json
{
  "query": {
    "id": "unique-query-id",
    "type": "metrics|logs|traces|correlation",
    "query": "query-string-appropriate-for-type",
    "tenant_id": "tenant-from-auth",
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
curl -X POST http://localhost:8010/unified/query \
  -H "Authorization: Bearer sess_abc123def456" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "metrics-query-001",
      "type": "metrics",
      "query": "http_requests_total{job=\"api\"}",
      "tenant_id": "default",
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
curl -X POST http://localhost:8010/unified/query \
  -H "Authorization: Bearer sess_abc123def456" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "id": "logs-query-001",
      "type": "logs",
      "query": "service.name:api AND level:error",
      "tenant_id": "default",
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
- Tenant isolation ensures data separation between tenants
- RBAC permissions are enforced based on user roles

### End-to-End Narrative
1. **Client authenticates** via `POST /api/v1/auth/login` with credentials
2. **Receives tokens** (session_token and jwt_token) in response
3. **Stores token securely** for subsequent requests
4. **Constructs unified query** with appropriate type and parameters
5. **Sends query** via `POST /unified/query` with Authorization header
6. **Receives results** in standardized format with metadata
7. **Handles pagination/limits** if result sets are large
8. **Processes correlations** if correlation query type was used
9. **Implements retry logic** for transient failures
10. **Refreshes tokens** before expiration

### Error Handling for Unified Query

#### Common Query Errors

**Invalid/Expired Token**:
- **Status Code**: `401 Unauthorized`
- **Response**:
```json
{
  "error": "Authentication required"
}
```
- **Client Action**: Re-authenticate and retry

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

**Insufficient Permissions**:
- **Status Code**: `403 Forbidden`
- **Response**:
```json
{
  "error": "Insufficient permissions"
}
```
- **Client Action**: Check user roles or request elevated permissions

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
- Parse metadata for detailed error information per engine</content>
<parameter name="filePath">/Users/aarvee/repos/github/public/miradorstack/mirador-core/docs/user-journeys.md