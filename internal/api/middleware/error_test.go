package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestErrorHandler(t *testing.T) {
	// Create a buffer to capture logs
	var logOutput strings.Builder
	testLogger := logger.NewMockLogger(&logOutput)

	tests := []struct {
		name           string
		setupError     func(*gin.Context)
		expectedStatus int
		expectedBody   string
		expectLog      bool
	}{
		{
			name: "no error - should not modify response",
			setupError: func(c *gin.Context) {
				// No error set
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "",
			expectLog:      false,
		},
		{
			name: "validation error - bad request",
			setupError: func(c *gin.Context) {
				c.Error(errors.New("invalid input: name is required"))
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"invalid input: name is required","code":"INVALID_REQUEST"}`,
			expectLog:      true,
		},
		{
			name: "forbidden error",
			setupError: func(c *gin.Context) {
				c.Error(errors.New("access forbidden"))
			},
			expectedStatus: http.StatusForbidden,
			expectedBody:   `{"error":"access forbidden","code":"ACCESS_DENIED"}`,
			expectLog:      true,
		},
		{
			name: "internal server error",
			setupError: func(c *gin.Context) {
				c.Error(errors.New("database connection failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"database connection failed","code":"CONNECTION_ERROR"}`,
			expectLog:      true,
		},
		{
			name: "error with status set - should use error handler format",
			setupError: func(c *gin.Context) {
				c.Writer.WriteHeader(http.StatusBadRequest)
				c.Error(errors.New("custom validation error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"custom validation error","code":"INTERNAL_ERROR"}`,
			expectLog:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset log output
			logOutput.Reset()

			// Create test router
			router := gin.New()
			router.Use(ErrorHandler(testLogger))

			// Add test route
			router.GET("/test", func(c *gin.Context) {
				tt.setupError(c)
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			w := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(w, req)

			// Check status
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check response body
			if tt.expectedBody != "" {
				body := strings.TrimSpace(w.Body.String())
				if body != tt.expectedBody {
					t.Errorf("Expected body %q, got %q", tt.expectedBody, body)
				}
			}

			// Check logging
			if tt.expectLog && logOutput.Len() == 0 {
				t.Error("Expected log output, but got none")
			}
			if !tt.expectLog && logOutput.Len() > 0 {
				t.Errorf("Expected no log output, but got: %s", logOutput.String())
			}
		})
	}
}

func TestDetermineStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"nil error", nil, http.StatusOK},
		{"empty error", errors.New(""), http.StatusInternalServerError},
		{"invalid input", errors.New("invalid input"), http.StatusBadRequest},
		{"required field", errors.New("name is required"), http.StatusBadRequest},
		{"not found", errors.New("user not found"), http.StatusNotFound},
		{"does not exist", errors.New("resource does not exist"), http.StatusNotFound},
		{"forbidden", errors.New("access forbidden"), http.StatusForbidden},
		{"unauthorized", errors.New("unauthorized access"), http.StatusForbidden},
		{"already exists", errors.New("user already exists"), http.StatusConflict},
		{"conflict", errors.New("version conflict"), http.StatusConflict},
		{"malformed", errors.New("malformed JSON"), http.StatusUnprocessableEntity},
		{"timeout", errors.New("request timeout"), http.StatusInternalServerError},
		{"connection error", errors.New("connection failed"), http.StatusInternalServerError},
		{"unknown error", errors.New("some random error"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineStatusCode(tt.err)
			if result != tt.expected {
				t.Errorf("determineStatusCode(%v) = %d, expected %d", tt.err, result, tt.expected)
			}
		})
	}
}

func TestDetermineErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		status   int
		expected string
	}{
		{"nil error", nil, http.StatusOK, ""},
		{"invalid request", errors.New("invalid input"), http.StatusBadRequest, "INVALID_REQUEST"},
		{"not found", errors.New("not found"), http.StatusNotFound, "NOT_FOUND"},
		{"access denied", errors.New("forbidden"), http.StatusForbidden, "ACCESS_DENIED"},
		{"conflict", errors.New("already exists"), http.StatusConflict, "CONFLICT"},
		{"operation not allowed", errors.New("cannot delete"), http.StatusUnprocessableEntity, "OPERATION_NOT_ALLOWED"},
		{"timeout", errors.New("timeout"), http.StatusInternalServerError, "TIMEOUT"},
		{"connection error", errors.New("connection"), http.StatusInternalServerError, "CONNECTION_ERROR"},
		{"unknown error", errors.New("random error"), http.StatusInternalServerError, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineErrorCode(tt.err, tt.status)
			if result != tt.expected {
				t.Errorf("determineErrorCode(%v, %d) = %s, expected %s", tt.err, tt.status, result, tt.expected)
			}
		})
	}
}

func TestDetermineErrorCodeFromStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected string
	}{
		{"bad request", http.StatusBadRequest, "INVALID_REQUEST"},
		{"unauthorized", http.StatusUnauthorized, "UNAUTHORIZED"},
		{"forbidden", http.StatusForbidden, "ACCESS_DENIED"},
		{"not found", http.StatusNotFound, "NOT_FOUND"},
		{"conflict", http.StatusConflict, "CONFLICT"},
		{"unprocessable entity", http.StatusUnprocessableEntity, "VALIDATION_ERROR"},
		{"too many requests", http.StatusTooManyRequests, "RATE_LIMITED"},
		{"internal server error", http.StatusInternalServerError, "INTERNAL_ERROR"},
		{"service unavailable", http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{"unknown status", 999, "UNKNOWN_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineErrorCodeFromStatus(tt.status)
			if result != tt.expected {
				t.Errorf("determineErrorCodeFromStatus(%d) = %s, expected %s", tt.status, result, tt.expected)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substrs  []string
		expected bool
	}{
		{"contains first", "hello world", []string{"hello", "goodbye"}, true},
		{"contains second", "goodbye world", []string{"hello", "goodbye"}, true},
		{"contains none", "hi world", []string{"hello", "goodbye"}, false},
		{"empty substrs", "hello", []string{}, false},
		{"empty string", "", []string{"hello"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.s, tt.substrs...)
			if result != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, expected %v", tt.s, tt.substrs, result, tt.expected)
			}
		})
	}
}
