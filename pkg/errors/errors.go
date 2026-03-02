// Package errors provides unified error types and utilities for Mirador Core.
// It offers typed errors with categories, HTTP status mapping, and error wrapping.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Category represents the type/category of an error for classification.
type Category string

const (
	CategoryValidation   Category = "VALIDATION"
	CategoryNotFound     Category = "NOT_FOUND"
	CategoryConflict     Category = "CONFLICT"
	CategoryUnauthorized Category = "UNAUTHORIZED"
	CategoryForbidden    Category = "FORBIDDEN"
	CategoryInternal     Category = "INTERNAL"
	CategoryTimeout      Category = "TIMEOUT"
	CategoryUnavailable  Category = "UNAVAILABLE"
	CategoryBadGateway   Category = "BAD_GATEWAY"
)

// AppError is the base application error type with category, code, and context.
type AppError struct {
	Category Category // Error category for classification
	Code     string   // Machine-readable error code (e.g., "KPI_NOT_FOUND")
	Message  string   // Human-readable error message
	Details  string   // Optional additional details
	Cause    error    // Underlying error (for wrapping)
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Cause.Error())
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is checks if the target error matches this error's code.
func (e *AppError) Is(target error) bool {
	var appErr *AppError
	if errors.As(target, &appErr) {
		return e.Code == appErr.Code
	}
	return false
}

// HTTPStatus returns the appropriate HTTP status code for this error.
func (e *AppError) HTTPStatus() int {
	switch e.Category {
	case CategoryValidation:
		return http.StatusBadRequest
	case CategoryNotFound:
		return http.StatusNotFound
	case CategoryConflict:
		return http.StatusConflict
	case CategoryUnauthorized:
		return http.StatusUnauthorized
	case CategoryForbidden:
		return http.StatusForbidden
	case CategoryTimeout:
		return http.StatusGatewayTimeout
	case CategoryUnavailable:
		return http.StatusServiceUnavailable
	case CategoryBadGateway:
		return http.StatusBadGateway
	case CategoryInternal:
		fallthrough
	default:
		return http.StatusInternalServerError
	}
}

// WithDetails adds additional details to the error.
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithCause wraps an underlying error.
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// ErrorResponse is the standard JSON error response structure.
type ErrorResponse struct {
	Error   string `json:"error"`             // Human-readable message
	Code    string `json:"code,omitempty"`    // Machine-readable code
	Details string `json:"details,omitempty"` // Optional details
}

// ToResponse converts an AppError to an ErrorResponse.
func (e *AppError) ToResponse() ErrorResponse {
	return ErrorResponse{
		Error:   e.Message,
		Code:    e.Code,
		Details: e.Details,
	}
}

// ---------- Constructors ----------

// New creates a new AppError with the given category, code, and message.
func New(category Category, code, message string) *AppError {
	return &AppError{
		Category: category,
		Code:     code,
		Message:  message,
	}
}

// Wrap wraps an existing error with context.
func Wrap(cause error, code, message string) *AppError {
	return &AppError{
		Category: CategoryInternal,
		Code:     code,
		Message:  message,
		Cause:    cause,
	}
}

// ---------- Validation Errors ----------

// Validation creates a validation error.
func Validation(code, message string) *AppError {
	return New(CategoryValidation, code, message)
}

// InvalidField creates a validation error for an invalid field.
func InvalidField(field, reason string) *AppError {
	return &AppError{
		Category: CategoryValidation,
		Code:     "INVALID_FIELD",
		Message:  fmt.Sprintf("invalid field '%s': %s", field, reason),
		Details:  field,
	}
}

// MissingField creates a validation error for a missing required field.
func MissingField(field string) *AppError {
	return &AppError{
		Category: CategoryValidation,
		Code:     "MISSING_FIELD",
		Message:  fmt.Sprintf("missing required field: %s", field),
		Details:  field,
	}
}

// InvalidRequest creates a validation error for an invalid request.
func InvalidRequest(message string) *AppError {
	return &AppError{
		Category: CategoryValidation,
		Code:     "INVALID_REQUEST",
		Message:  message,
	}
}

// ---------- Not Found Errors ----------

// NotFound creates a not found error.
func NotFound(resource, identifier string) *AppError {
	return &AppError{
		Category: CategoryNotFound,
		Code:     fmt.Sprintf("%s_NOT_FOUND", resource),
		Message:  fmt.Sprintf("%s not found: %s", resource, identifier),
		Details:  identifier,
	}
}

// KPINotFound creates a not found error for a KPI.
func KPINotFound(id string) *AppError {
	return NotFound("KPI", id)
}

// DataSourceNotFound creates a not found error for a data source.
func DataSourceNotFound(id string) *AppError {
	return NotFound("DATA_SOURCE", id)
}

// ---------- Conflict Errors ----------

// Conflict creates a conflict error.
func Conflict(resource, message string) *AppError {
	return &AppError{
		Category: CategoryConflict,
		Code:     fmt.Sprintf("%s_CONFLICT", resource),
		Message:  message,
	}
}

// AlreadyExists creates a conflict error for an existing resource.
func AlreadyExists(resource, identifier string) *AppError {
	return &AppError{
		Category: CategoryConflict,
		Code:     fmt.Sprintf("%s_EXISTS", resource),
		Message:  fmt.Sprintf("%s already exists: %s", resource, identifier),
		Details:  identifier,
	}
}

// ---------- Auth Errors ----------

// Unauthorized creates an unauthorized error.
func Unauthorized(message string) *AppError {
	return &AppError{
		Category: CategoryUnauthorized,
		Code:     "UNAUTHORIZED",
		Message:  message,
	}
}

// Forbidden creates a forbidden error.
func Forbidden(message string) *AppError {
	return &AppError{
		Category: CategoryForbidden,
		Code:     "FORBIDDEN",
		Message:  message,
	}
}

// ---------- Internal Errors ----------

// Internal creates an internal server error.
func Internal(message string) *AppError {
	return &AppError{
		Category: CategoryInternal,
		Code:     "INTERNAL_ERROR",
		Message:  message,
	}
}

// InternalWithCause creates an internal error wrapping another error.
func InternalWithCause(message string, cause error) *AppError {
	return &AppError{
		Category: CategoryInternal,
		Code:     "INTERNAL_ERROR",
		Message:  message,
		Cause:    cause,
	}
}

// ---------- Availability Errors ----------

// Timeout creates a timeout error.
func Timeout(operation string) *AppError {
	return &AppError{
		Category: CategoryTimeout,
		Code:     "TIMEOUT",
		Message:  fmt.Sprintf("operation timed out: %s", operation),
		Details:  operation,
	}
}

// Unavailable creates a service unavailable error.
func Unavailable(service string) *AppError {
	return &AppError{
		Category: CategoryUnavailable,
		Code:     "SERVICE_UNAVAILABLE",
		Message:  fmt.Sprintf("service unavailable: %s", service),
		Details:  service,
	}
}

// DatabaseUnavailable creates a database unavailable error.
func DatabaseUnavailable(dbType string) *AppError {
	return &AppError{
		Category: CategoryUnavailable,
		Code:     "DATABASE_UNAVAILABLE",
		Message:  fmt.Sprintf("database unavailable: %s", dbType),
		Details:  dbType,
	}
}

// ---------- Helpers ----------

// IsNotFound returns true if the error is a not found error.
func IsNotFound(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Category == CategoryNotFound
	}
	return false
}

// IsValidation returns true if the error is a validation error.
func IsValidation(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Category == CategoryValidation
	}
	return false
}

// IsUnavailable returns true if the error is an unavailable error.
func IsUnavailable(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Category == CategoryUnavailable
	}
	return false
}

// GetHTTPStatus returns the HTTP status code for an error.
// For non-AppError types, returns 500.
func GetHTTPStatus(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus()
	}
	return http.StatusInternalServerError
}

// ToErrorResponse converts any error to an ErrorResponse.
// For non-AppError types, returns a generic internal error response.
func ToErrorResponse(err error) ErrorResponse {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.ToResponse()
	}
	return ErrorResponse{
		Error: err.Error(),
		Code:  "INTERNAL_ERROR",
	}
}
