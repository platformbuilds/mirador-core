# API Versioning Strategy

This document describes the API versioning strategy for Mirador Core.

## Version Format

Mirador Core uses **URL path versioning** with the format `/api/v{major}`:

```
/api/v1/kpis
/api/v1/unified/query
/api/v1/health
```

## Current Version

**Current API Version: v1**

All production endpoints are currently served under `/api/v1/`.

## Versioning Rules

### Semantic Versioning

The API follows semantic versioning principles:

- **Major version (v1, v2, ...)**: Incremented for breaking changes
- **Minor/Patch versions**: Not exposed in URL; tracked in response headers

### What Constitutes a Breaking Change

A **breaking change** requires a major version bump:

- Removing an endpoint
- Removing or renaming request/response fields
- Changing field types (e.g., `string` → `number`)
- Changing required/optional status of request fields
- Changing authentication/authorization requirements
- Changing error response structure
- Changing default behavior in backward-incompatible ways

### Non-Breaking Changes (No Version Bump)

These changes can be made without versioning:

- Adding new endpoints
- Adding optional request fields (with defaults)
- Adding new response fields
- Adding new enum values (when clients handle unknown values)
- Deprecating (not removing) fields
- Performance improvements
- Bug fixes that don't change documented behavior

## Implementation Details

### URL Structure

```
Base URL: /api/v1
├── /health              # Health check (no auth)
├── /kpis                # KPI CRUD operations
├── /datasources         # Data source management
├── /unified/query       # Unified query endpoint
├── /unified/correlate   # Correlation analysis
├── /unified/rca         # Root cause analysis
└── /mira/chat          # AI chat interface
```

### Response Headers

All API responses include version information:

```http
X-API-Version: 1.0.0
X-Deprecation-Notice: (optional, for deprecated endpoints)
```

### Deprecation Process

When deprecating an endpoint or field:

1. **Announce**: Document deprecation in CHANGELOG.md
2. **Warn**: Add `X-Deprecation-Notice` header with removal date
3. **Maintain**: Keep deprecated functionality for minimum 3 months
4. **Remove**: Remove in next major version

Example deprecation header:
```http
X-Deprecation-Notice: This endpoint is deprecated and will be removed in v2. Use /api/v1/unified/query instead.
```

## Multi-Version Support

### During Transition Periods

When releasing a new major version:

1. Both versions run concurrently for a transition period
2. Old version enters maintenance mode (security fixes only)
3. Documentation clearly indicates version status
4. Minimum 6-month overlap before old version removal

### Version Negotiation

Clients should:
1. Explicitly specify version in URL path
2. Handle unknown response fields gracefully (forward compatibility)
3. Check `X-API-Version` header for exact version

## Error Responses

All versions use consistent error response format:

```json
{
  "error": "Human-readable error message",
  "code": "MACHINE_READABLE_CODE",
  "details": "Optional additional context"
}
```

HTTP status codes are consistent across versions:

| Status | Meaning |
|--------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request (validation error) |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 409 | Conflict |
| 500 | Internal Server Error |
| 503 | Service Unavailable |
| 504 | Gateway Timeout |

## Client Guidelines

### Recommended Practices

1. **Pin to specific version**: Always use explicit version in URL
2. **Handle new fields**: Ignore unknown response fields
3. **Check deprecation headers**: Log and alert on deprecation notices
4. **Use Content-Type**: Always send `Content-Type: application/json`
5. **Accept header**: Include `Accept: application/json`

### Example Request

```bash
curl -X POST https://api.example.com/api/v1/unified/query \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{"query": "SELECT * FROM metrics WHERE time > now() - 1h"}'
```

## Future Versions

### v2 Planning

When v2 is released, it may include:

- Consolidated endpoint structure
- GraphQL support (alongside REST)
- Streaming response support
- Enhanced pagination

### Migration Path

Migration guides will be provided when new major versions are released, including:

- Field mapping tables
- Code examples
- Automated migration tools where applicable
- Timeline for v1 deprecation

## Related Documentation

- [API Reference](api-docs.md)
- [Getting Started](getting-started.md)
- [Configuration](configuration.md)
