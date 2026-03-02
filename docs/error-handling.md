# Error Handling Guide

This document describes the error handling patterns and best practices for MIRADOR-CORE.

## Overview

MIRADOR-CORE uses a unified error handling system built on the `pkg/errors` package. This system provides:

- Structured error types with categories
- Automatic HTTP status code mapping
- Error wrapping and unwrapping
- Consistent JSON error responses
- Gin framework integration

## Error Categories

All errors in MIRADOR-CORE are categorized using the `Category` type. Each category maps to a specific HTTP status code:

| Category | HTTP Status | Use Case |
|----------|-------------|----------|
| `CategoryValidation` | 400 Bad Request | Invalid input, malformed requests |
| `CategoryNotFound` | 404 Not Found | Resource does not exist |
| `CategoryUnauthorized` | 401 Unauthorized | Missing or invalid authentication |
| `CategoryForbidden` | 403 Forbidden | Insufficient permissions |
| `CategoryConflict` | 409 Conflict | Resource state conflicts |
| `CategoryInternal` | 500 Internal Server Error | Server-side errors |
| `CategoryUnavailable` | 503 Service Unavailable | Dependency unavailable |
| `CategoryTimeout` | 504 Gateway Timeout | Operation timeout |
| `CategoryBadGateway` | 502 Bad Gateway | Upstream service error |

## Creating Errors

### Using Constructor Functions

Use the provided constructor functions for type-safe error creation:

```go
import "github.com/mirastacklabs-ai/mirador-core/pkg/errors"

// Validation errors
err := errors.InvalidField("startTime", "must be before endTime")
err := errors.MissingField("kpiId")
err := errors.InvalidRequest("time window exceeds maximum of 1 hour")

// Not found errors
err := errors.NotFound("kpi", "kpi-123")
err := errors.KPINotFound("kpi-123")  // Convenience function
err := errors.DataSourceNotFound("ds-456")

// Conflict errors
err := errors.Conflict("KPI", "KPI with this name already exists")
err := errors.AlreadyExists("KPI", "kpi-123")

// Authentication/Authorization errors
err := errors.Unauthorized("invalid API key")
err := errors.Forbidden("access denied to this resource")

// Server errors
err := errors.Internal("database connection failed")
err := errors.InternalWithCause("query execution failed", dbErr)
err := errors.Unavailable("weaviate")
err := errors.Timeout("correlation analysis")
```

### Wrapping Errors

Wrap underlying errors to preserve context:

```go
// Wrap with additional context
result, err := database.Query(query)
if err != nil {
    return nil, errors.Wrap(err, "QUERY_FAILED", "failed to execute query")
}

// Add cause to existing error
appErr := errors.NotFound("kpi", id)
appErr.WithCause(originalErr)

// Add details
appErr := errors.InvalidRequest("validation failed")
appErr.WithDetails("time window must be positive")
```

## Handling Errors in HTTP Handlers

### Using Gin Helpers

The `pkg/errors` package provides Gin-specific helpers:

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/mirastacklabs-ai/mirador-core/pkg/errors"
)

func GetKPI(c *gin.Context) {
    id := c.Param("id")
    
    kpi, err := service.GetKPI(ctx, id)
    if err != nil {
        // Automatically determines HTTP status from error type
        errors.RespondError(c, err)
        return
    }
    
    c.JSON(http.StatusOK, kpi)
}

// Convenience functions for common cases
func CreateKPI(c *gin.Context) {
    var req CreateKPIRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        errors.RespondValidationError(c, "invalid request body")
        return
    }
    
    if req.Name == "" {
        errors.RespondValidationError(c, "name is required")
        return
    }
    
    // ... create KPI
}

func DeleteKPI(c *gin.Context) {
    id := c.Param("id")
    
    if !service.Exists(ctx, id) {
        errors.RespondNotFound(c, "KPI", id)
        return
    }
    
    // ... delete KPI
}
```

### Aborting in Middleware

Use `AbortWithError` in middleware to stop the request chain:

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            errors.AbortWithError(c, errors.Unauthorized("missing authorization header"))
            return
        }
        
        claims, err := validateToken(token)
        if err != nil {
            errors.AbortWithError(c, errors.Unauthorized("invalid token"))
            return
        }
        
        c.Set("claims", claims)
        c.Next()
    }
}
```

## Error Response Format

All error responses follow this JSON structure:

```json
{
    "error": "Human-readable error message",
    "code": "MACHINE_READABLE_CODE",
    "details": "Optional additional context"
}
```

### Example Responses

**Validation Error (400):**
```json
{
    "error": "invalid field 'startTime': must be before endTime",
    "code": "INVALID_FIELD",
    "details": "startTime"
}
```

**Not Found Error (404):**
```json
{
    "error": "KPI not found: kpi-123",
    "code": "KPI_NOT_FOUND",
    "details": "kpi-123"
}
```

**Internal Error (500):**
```json
{
    "error": "an internal error occurred",
    "code": "INTERNAL_ERROR"
}
```

## Error Inspection

Use standard Go error handling with `errors.Is` and `errors.As`:

```go
import (
    "errors"
    apperrs "github.com/mirastacklabs-ai/mirador-core/pkg/errors"
)

// Check error type
if apperrs.IsNotFound(err) {
    // Handle not found case
    return getDefault()
}

if apperrs.IsValidation(err) {
    // Handle validation error
    log.Warn("validation failed", "error", err)
}

if apperrs.IsTimeout(err) {
    // Retry or fail gracefully
}

// Get HTTP status code
status := apperrs.GetHTTPStatus(err)

// Convert to response
resp := apperrs.ToErrorResponse(err)

// Unwrap to get cause
var appErr *apperrs.AppError
if errors.As(err, &appErr) {
    if appErr.Cause != nil {
        log.Error("underlying cause", "cause", appErr.Cause)
    }
}
```

## Best Practices

### 1. Use Specific Error Types

Prefer specific constructor functions over generic ones:

```go
// Good - specific and informative
err := errors.KPINotFound(id)
err := errors.InvalidField("timeWindow", "must be positive")

// Avoid - too generic
err := errors.New(errors.CategoryNotFound, "NOT_FOUND", "not found")
```

### 2. Preserve Error Context

Always wrap errors to preserve the call chain:

```go
// Good - preserves context
result, err := repo.Query(ctx, query)
if err != nil {
    return nil, errors.InternalWithCause("failed to query KPIs", err)
}

// Avoid - loses context
if err != nil {
    return nil, errors.Internal("query failed")
}
```

### 3. Don't Expose Internal Details

For internal errors, log the details but return a generic message:

```go
func handler(c *gin.Context) {
    result, err := riskyOperation()
    if err != nil {
        // Log the actual error
        log.Error("risky operation failed", "error", err)
        
        // Return generic message to client
        errors.RespondInternal(c, err.Error())
        return
    }
}
```

### 4. Consistent Error Codes

Use consistent, uppercase error codes:

```go
// Good - consistent format
"KPI_NOT_FOUND"
"INVALID_FIELD"
"DATABASE_ERROR"

// Avoid - inconsistent
"kpi-not-found"
"InvalidField"
"dbError"
```

### 5. Handle Errors at Appropriate Levels

Let errors propagate to appropriate handlers:

```go
// Repository layer - return domain error
func (r *KPIRepo) Get(ctx context.Context, id string) (*KPI, error) {
    kpi, err := r.db.Query(ctx, id)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, errors.KPINotFound(id)
    }
    if err != nil {
        return nil, errors.InternalWithCause("database query failed", err)
    }
    return kpi, nil
}

// Service layer - may transform or wrap errors
func (s *KPIService) Get(ctx context.Context, id string) (*KPI, error) {
    return s.repo.Get(ctx, id)  // Let errors propagate
}

// Handler layer - convert to HTTP response
func GetKPI(c *gin.Context) {
    kpi, err := service.Get(ctx, id)
    if err != nil {
        errors.RespondError(c, err)
        return
    }
    c.JSON(http.StatusOK, kpi)
}
```

## Testing with Errors

When testing error cases, use the type assertion helpers:

```go
func TestGetKPI_NotFound(t *testing.T) {
    service := NewKPIService(mockRepo)
    mockRepo.EXPECT().Get(ctx, "invalid-id").Return(nil, errors.KPINotFound("invalid-id"))
    
    _, err := service.Get(ctx, "invalid-id")
    
    require.Error(t, err)
    assert.True(t, errors.IsNotFound(err))
    
    var appErr *errors.AppError
    require.True(t, errors.As(err, &appErr))
    assert.Equal(t, "KPI_NOT_FOUND", appErr.Code)
}
```

## Related Documentation

- [API Versioning](api-versioning.md) - API versioning and error compatibility
- [Service Recovery Procedures](service-recovery-procedures.md) - Error recovery strategies
- [Monitoring and Observability](monitoring-observability.md) - Error metrics and alerting
