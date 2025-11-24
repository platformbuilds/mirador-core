package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// errorResponse is a helper for decoding error JSON responses.
type errorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func setupRCAHandlerRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &RCAHandler{} // All methods return 501 Not Implemented
	r.GET("/api/v1/rca/correlations", h.GetActiveCorrelations)
	r.GET("/api/v1/rca/patterns", h.GetFailurePatterns)
	r.POST("/api/v1/rca/store", h.StoreCorrelation)
	return r
}

func decodeErrorResponse(t *testing.T, body string) errorResponse {
	var resp errorResponse
	err := json.Unmarshal([]byte(body), &resp)
	if err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	return resp
}

func TestRCAHandlerEndpoints(t *testing.T) {
	router := setupRCAHandlerRouter()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "GET /api/v1/rca/correlations returns 501",
			method:     http.MethodGet,
			path:       "/api/v1/rca/correlations",
			wantStatus: http.StatusNotImplemented,
			wantError:  "not_implemented",
		},
		{
			name:       "GET /api/v1/rca/patterns returns 501",
			method:     http.MethodGet,
			path:       "/api/v1/rca/patterns",
			wantStatus: http.StatusNotImplemented,
			wantError:  "not_implemented",
		},
		{
			name:       "POST /api/v1/rca/store returns 501",
			method:     http.MethodPost,
			path:       "/api/v1/rca/store",
			body:       `{"dummy":"value"}`,
			wantStatus: http.StatusNotImplemented,
			wantError:  "not_implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			resp := decodeErrorResponse(t, w.Body.String())
			if resp.Error != tt.wantError {
				t.Errorf("expected error '%s', got '%s'", tt.wantError, resp.Error)
			}
			if resp.Status != "error" {
				t.Errorf("expected status 'error', got '%s'", resp.Status)
			}
		})
	}
}
