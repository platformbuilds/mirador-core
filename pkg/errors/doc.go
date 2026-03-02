// Package errors provides unified error types for consistent error handling
// across MIRADOR-CORE services.
//
// # Overview
//
// This package defines a structured error type [AppError] that includes:
//   - Error category (NotFound, Validation, Internal, etc.)
//   - HTTP status code mapping
//   - Error wrapping support
//   - Gin framework integration
//
// # Error Categories
//
// Errors are categorized using the [Category] type:
//   - [CategoryNotFound]: Resource does not exist (HTTP 404)
//   - [CategoryValidation]: Invalid input or request (HTTP 400)
//   - [CategoryUnauthorized]: Authentication required (HTTP 401)
//   - [CategoryForbidden]: Permission denied (HTTP 403)
//   - [CategoryConflict]: Resource state conflict (HTTP 409)
//   - [CategoryInternal]: Server-side errors (HTTP 500)
//   - [CategoryUnavailable]: Service temporarily unavailable (HTTP 503)
//   - [CategoryTimeout]: Operation timeout (HTTP 504)
//
// # Creating Errors
//
// Use the constructor functions for type-safe error creation:
//
//	// Not found error
//	err := errors.NotFound("kpi", kpiID)
//
//	// Validation error
//	err := errors.Validation("invalid time range: start must be before end")
//
//	// Internal error with wrapped cause
//	err := errors.InternalWithCause("database query failed", dbErr)
//
//	// Custom error
//	err := errors.New(errors.CategoryConflict, "resource already exists")
//
// # Gin Integration
//
// The [gin.go] file provides helpers for HTTP responses:
//
//	func GetKPI(c *gin.Context) {
//	    kpi, err := service.GetKPI(id)
//	    if err != nil {
//	        errors.RespondError(c, err)
//	        return
//	    }
//	    c.JSON(http.StatusOK, kpi)
//	}
//
// # Error Inspection
//
// Check error types using the helper functions:
//
//	if errors.IsNotFound(err) {
//	    // Handle not found case
//	}
//
//	if errors.IsValidation(err) {
//	    // Handle validation error
//	}
//
//	// Get the underlying cause
//	cause := errors.Unwrap(err)
package errors
