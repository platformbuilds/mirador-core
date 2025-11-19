// internal/api/middleware/request_logger.middleware.go
package middleware

import (
	"bytes"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Unknown identifiers for logging when context is not available
const (
	UnknownSessionID = "unknown"
)

// RequestLogger logs HTTP requests for MIRADOR-CORE observability
func RequestLogger(log logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Extract additional context
		sessionID := UnknownSessionID

		// Try to get context from Gin context if available
		if param.Keys != nil {
			if sid, exists := param.Keys["session_id"]; exists {
				if sidStr, ok := sid.(string); ok {
					sessionID = sidStr
				}
			}
		}

		// Log level based on status code
		statusCode := param.StatusCode
		logLevel := "info"
		if statusCode >= 400 && statusCode < 500 {
			logLevel = "warn"
		} else if statusCode >= 500 {
			logLevel = "error"
		}

		// Structure log fields for MIRADOR-CORE observability
		fields := []interface{}{
			"method", param.Method,
			"path", param.Path,
			"status", statusCode,
			"latency", param.Latency,
			"client_ip", param.ClientIP,
			"user_agent", param.Request.UserAgent(),
			"session_id", sessionID,
			"request_id", param.Request.Header.Get("X-Request-ID"),
			"content_length", param.Request.ContentLength,
			"referer", param.Request.Referer(),
		}

		// Add error context if present
		if param.ErrorMessage != "" {
			fields = append(fields, "error", param.ErrorMessage)
		}

		// Log the request
		switch logLevel {
		case "warn":
			log.Warn("HTTP Request", fields...)
		case "error":
			log.Error("HTTP Request", fields...)
		default:
			log.Info("HTTP Request", fields...)
		}

		return ""
	})
}

// RequestLoggerWithBody logs HTTP requests including request/response bodies for debugging
func RequestLoggerWithBody(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Read request body
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Capture response body
		responseWriter := &responseBodyWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		sessionID := c.GetString("session_id")
		if sessionID == "" {
			sessionID = UnknownSessionID
		}

		// Prepare log fields
		fields := []interface{}{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"status", c.Writer.Status(),
			"latency", latency,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"session_id", sessionID,
			"request_id", c.Request.Header.Get("X-Request-ID"),
			"content_length", c.Request.ContentLength,
		}

		// Add request body for debugging (only for non-sensitive endpoints)
		if len(requestBody) > 0 && len(requestBody) < 1024 && !isSensitiveEndpoint(c.Request.URL.Path) {
			fields = append(fields, "request_body", string(requestBody))
		}

		// Add response body for debugging (only for errors or debugging mode)
		if c.Writer.Status() >= 400 || gin.Mode() == gin.DebugMode {
			responseBody := responseWriter.body.String()
			if len(responseBody) < 1024 {
				fields = append(fields, "response_body", responseBody)
			}
		}

		// Log based on status code
		switch {
		case c.Writer.Status() >= 500:
			log.Error("HTTP Request", fields...)
		case c.Writer.Status() >= 400:
			log.Warn("HTTP Request", fields...)
		default:
			log.Info("HTTP Request", fields...)
		}
	}
}

// responseBodyWriter captures response body for logging
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// isSensitiveEndpoint checks if an endpoint contains sensitive data that shouldn't be logged
func isSensitiveEndpoint(path string) bool {
	sensitiveEndpoints := []string{
		// Authentication endpoints removed from MIRADOR-CORE core;
		// keep only server-side secret endpoints as sensitive.
		"/api/v1/users/password",
		"/api/v1/config/secrets",
	}

	for _, endpoint := range sensitiveEndpoints {
		if strings.Contains(path, endpoint) {
			return true
		}
	}

	return false
}
