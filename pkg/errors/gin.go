package errors

import (
	"github.com/gin-gonic/gin"
)

// RespondError sends a JSON error response using the appropriate HTTP status.
// For AppError types, it uses the error's HTTP status and structured response.
// For other error types, it returns a 500 Internal Server Error.
func RespondError(c *gin.Context, err error) {
	status := GetHTTPStatus(err)
	resp := ToErrorResponse(err)
	c.JSON(status, resp)
}

// RespondValidationError is a convenience function for validation errors.
func RespondValidationError(c *gin.Context, message string) {
	err := InvalidRequest(message)
	c.JSON(err.HTTPStatus(), err.ToResponse())
}

// RespondNotFound is a convenience function for not found errors.
func RespondNotFound(c *gin.Context, resource, identifier string) {
	err := NotFound(resource, identifier)
	c.JSON(err.HTTPStatus(), err.ToResponse())
}

// RespondUnauthorized is a convenience function for unauthorized errors.
func RespondUnauthorized(c *gin.Context, message string) {
	err := Unauthorized(message)
	c.JSON(err.HTTPStatus(), err.ToResponse())
}

// RespondForbidden is a convenience function for forbidden errors.
func RespondForbidden(c *gin.Context, message string) {
	err := Forbidden(message)
	c.JSON(err.HTTPStatus(), err.ToResponse())
}

// RespondInternal is a convenience function for internal server errors.
// It logs the actual error but returns a generic message to the client.
func RespondInternal(c *gin.Context, logMessage string) {
	err := Internal("an internal error occurred")
	err.Details = logMessage // Store for logging, not sent to client
	resp := ErrorResponse{
		Error: err.Message,
		Code:  err.Code,
	}
	c.JSON(err.HTTPStatus(), resp)
}

// RespondUnavailable is a convenience function for service unavailable errors.
func RespondUnavailable(c *gin.Context, service string) {
	err := Unavailable(service)
	c.JSON(err.HTTPStatus(), err.ToResponse())
}

// RespondTimeout is a convenience function for timeout errors.
func RespondTimeout(c *gin.Context, operation string) {
	err := Timeout(operation)
	c.JSON(err.HTTPStatus(), err.ToResponse())
}

// AbortWithError aborts the request with an error response.
// This is useful in middleware where you want to stop the request chain.
func AbortWithError(c *gin.Context, err error) {
	status := GetHTTPStatus(err)
	resp := ToErrorResponse(err)
	c.AbortWithStatusJSON(status, resp)
}
