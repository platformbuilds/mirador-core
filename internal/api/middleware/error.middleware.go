package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error   string      `json:"error"`
	Code    string      `json:"code,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// ErrorHandler provides centralized error handling middleware
func ErrorHandler(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			// Get the last error
			err := c.Errors.Last()

			// Determine HTTP status code based on error type
			statusCode := determineStatusCode(err.Err)

			// Create standardized error response
			errorResp := ErrorResponse{
				Error: err.Err.Error(),
				Code:  determineErrorCode(err.Err, statusCode),
			}

			// Add additional details for validation errors
			if details := extractValidationDetails(err.Err); details != nil {
				errorResp.Details = details
			}

			// Log the error
			logError(log, statusCode, err.Err, c)

			// Send error response
			c.JSON(statusCode, errorResp)
			return
		}

		// If no errors but status indicates error, ensure proper error format
		if c.Writer.Status() >= 400 && !c.Writer.Written() {
			statusCode := c.Writer.Status()
			errorResp := ErrorResponse{
				Error: http.StatusText(statusCode),
				Code:  determineErrorCodeFromStatus(statusCode),
			}

			// Try to get error message from context
			if errorMsg, exists := c.Get("error_message"); exists {
				if msg, ok := errorMsg.(string); ok {
					errorResp.Error = msg
				}
			}

			log.Warn("HTTP Error Response",
				"status", statusCode,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"client_ip", c.ClientIP(),
				"error", errorResp.Error,
			)

			c.JSON(statusCode, errorResp)
		}
	}
}

// determineStatusCode determines HTTP status code from error type
func determineStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Check for specific error types or messages
	errMsg := err.Error()

	// Validation errors
	if containsAny(errMsg, "invalid", "required", "cannot be empty", "must be") {
		return http.StatusBadRequest
	}

	// Not found errors
	if containsAny(errMsg, "not found", "does not exist") {
		return http.StatusNotFound
	}

	// Forbidden/Unauthorized
	if containsAny(errMsg, "forbidden", "unauthorized", "permission denied") {
		return http.StatusForbidden
	}

	// Conflict
	if containsAny(errMsg, "already exists", "conflict", "duplicate") {
		return http.StatusConflict
	}

	// Unprocessable Entity
	if containsAny(errMsg, "cannot delete", "invalid format", "malformed") {
		return http.StatusUnprocessableEntity
	}

	// Default to internal server error
	return http.StatusInternalServerError
}

// determineErrorCode creates a machine-readable error code
func determineErrorCode(err error, statusCode int) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// Map common error patterns to codes
	switch {
	case containsAny(errMsg, "invalid", "bad request"):
		return "INVALID_REQUEST"
	case containsAny(errMsg, "not found"):
		return "NOT_FOUND"
	case containsAny(errMsg, "unauthorized", "forbidden"):
		return "ACCESS_DENIED"
	case containsAny(errMsg, "already exists", "conflict"):
		return "CONFLICT"
	case containsAny(errMsg, "cannot delete"):
		return "OPERATION_NOT_ALLOWED"
	case containsAny(errMsg, "timeout"):
		return "TIMEOUT"
	case containsAny(errMsg, "connection", "network"):
		return "CONNECTION_ERROR"
	default:
		return "INTERNAL_ERROR"
	}
}

// determineErrorCodeFromStatus creates error code from HTTP status
func determineErrorCodeFromStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "INVALID_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "ACCESS_DENIED"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusUnprocessableEntity:
		return "VALIDATION_ERROR"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	default:
		return "UNKNOWN_ERROR"
	}
}

// extractValidationDetails extracts validation error details if available
func extractValidationDetails(err error) interface{} {
	// For now, return nil - can be enhanced to parse structured validation errors
	// from libraries like go-playground/validator
	return nil
}

// logError logs errors with appropriate level
func logError(log logger.Logger, statusCode int, err error, c *gin.Context) {
	fields := []interface{}{
		"status", statusCode,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"client_ip", c.ClientIP(),
		"error", err.Error(),
	}

	// Add request ID if available
	if requestID := c.Request.Header.Get("X-Request-ID"); requestID != "" {
		fields = append(fields, "request_id", requestID)
	}

	// Add user/tenant context if available
	if tenantID := c.GetString("tenant_id"); tenantID != "" {
		fields = append(fields, "tenant_id", tenantID)
	}
	if userID := c.GetString("user_id"); userID != "" {
		fields = append(fields, "user_id", userID)
	}

	// Log based on severity
	if statusCode >= 500 {
		log.Error("HTTP Error", fields...)
	} else if statusCode >= 400 {
		log.Warn("HTTP Error", fields...)
	} else {
		log.Info("HTTP Error", fields...)
	}
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrings ...string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsIgnoreCase(s, substr))
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}
