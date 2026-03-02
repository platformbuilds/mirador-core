package mariadb

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GracefulErrorResponse represents an error response for MariaDB failures.
type GracefulErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HandleUnavailable returns a 503 Service Unavailable response with a clear
// message that the config database is unavailable. Use this in handlers that
// require MariaDB data.
func HandleUnavailable(c *gin.Context, operation string) {
	c.JSON(http.StatusServiceUnavailable, GracefulErrorResponse{
		Error: "config_database_unavailable",
		Code:  "MARIADB_UNAVAILABLE",
		Message: "The configuration database is temporarily unavailable. " +
			"Operation '" + operation + "' cannot be completed. " +
			"Please try again later.",
	})
}

// HandleDisabled returns a 501 Not Implemented response when MariaDB is
// disabled in configuration but the endpoint requires it.
func HandleDisabled(c *gin.Context, operation string) {
	c.JSON(http.StatusNotImplemented, GracefulErrorResponse{
		Error: "config_database_disabled",
		Code:  "MARIADB_DISABLED",
		Message: "The configuration database is not enabled for this deployment. " +
			"Operation '" + operation + "' is not available.",
	})
}

// HandleQueryError returns a 500 error for MariaDB query failures.
func HandleQueryError(c *gin.Context, operation string, err error) {
	c.JSON(http.StatusInternalServerError, GracefulErrorResponse{
		Error:   "config_database_error",
		Code:    "MARIADB_QUERY_ERROR",
		Message: "Failed to query configuration database for '" + operation + "': " + err.Error(),
	})
}

// CheckAvailable is a helper that checks if MariaDB is available and returns
// an error response if not. Returns true if MariaDB is available.
func CheckAvailable(c *gin.Context, client *Client, operation string) bool {
	if client == nil {
		HandleDisabled(c, operation)
		return false
	}

	if !client.IsEnabled() {
		HandleDisabled(c, operation)
		return false
	}

	if !client.IsConnected() {
		// Try to reconnect once
		if err := client.Reconnect(); err != nil {
			HandleUnavailable(c, operation)
			return false
		}
	}

	return true
}
