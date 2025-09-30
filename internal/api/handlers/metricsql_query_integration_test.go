//go:build integration

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/require"
)

// Integration test for MetricsQL query handlers with mocked VictoriaMetrics backend
func TestMetricsQLQueryHandler_Integration_AggregateFunctions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock VictoriaMetrics server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse the query to determine what response to return
		query := r.URL.Query().Get("query")
		var response models.VictoriaMetricsResponse

		if strings.Contains(query, "sum(") {
			response = models.VictoriaMetricsResponse{
				Status: "success",
				Data: map[string]any{
					"result": []any{
						map[string]any{
							"metric": map[string]string{"__name__": "http_requests_total"},
							"value": []any{1703123456, "100"},
						},
					},
				},
			}
		} else if strings.Contains(query, "avg(") {
			response = models.VictoriaMetricsResponse{
				Status: "success",
				Data: map[string]any{
					"result": []any{
						map[string]any{
							"metric": map[string]string{"__name__": "http_requests_total"},
							"value": []any{1703123456, "25.5"},
						},
					},
				},
			}
		} else if strings.Contains(query, "count(") {
			response = models.VictoriaMetricsResponse{
				Status: "success",
				Data: map[string]any{
					"result": []any{
						map[string]any{
							"metric": map[string]string{"__name__": "http_requests_total"},
							"value": []any{1703123456, "4"},
						},
					},
				},
			}
		} else if strings.Contains(query, "quantile(") {
			response = models.VictoriaMetricsResponse{
				Status: "success",
				Data: map[string]any{
					"result": []any{
						map[string]any{
							"metric": map[string]string{"__name__": "http_requests_total"},
							"value": []any{1703123456, "95"},
						},
					},
				},
			}
		} else if strings.Contains(query, "topk(") {
			response = models.VictoriaMetricsResponse{
				Status: "success",
				Data: map[string]any{
					"result": []any{
						map[string]any{
							"metric": map[string]string{"__name__": "http_requests_total", "instance": "app1"},
							"value": []any{1703123456, "50"},
						},
						map[string]any{
							"metric": map[string]string{"__name__": "http_requests_total", "instance": "app2"},
							"value": []any{1703123456, "30"},
						},
					},
				},
			}
		} else {
			// Default response for other functions
			response = models.VictoriaMetricsResponse{
				Status: "success",
				Data: map[string]any{
					"result": []any{
						map[string]any{
							"metric": map[string]string{"__name__": "http_requests_total"},
							"value": []any{1703123456, "42"},
						},
					},
				},
			}
		}

		json.NewEncoder(w).Encode(response)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Set up services
	log := logger.New("error")
	vmConfig := config.VictoriaMetricsConfig{
		Endpoints: []string{ts.URL},
		Timeout:   5000,
	}
	vmService := services.NewVictoriaMetricsService(vmConfig, log)
	queryService := services.NewVictoriaMetricsQueryService(vmService, log)
	cacheService := cache.NewNoopValkeyCache(log)

	// Create handler
	handler := NewMetricsQLQueryHandler(queryService, cacheService, log)

	// Create validation middleware
	validationMiddleware := middleware.NewMetricsQLQueryValidationMiddleware(log)

	// Set up router
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/api/v1/metrics/query/aggregate/:function", validationMiddleware.ValidateFunctionQuery(), handler.ExecuteAggregateFunction)

	tests := []struct {
		name         string
		function     string
		requestBody  models.MetricsQLFunctionRequest
		expectSeries bool
	}{
		{"sum function integration", "sum", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"avg function integration", "avg", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"count function integration", "count", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"min function integration", "min", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"max function integration", "max", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"median function integration", "median", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"quantile function integration", "quantile", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])", Params: map[string]interface{}{"quantile": 0.95}}, true},
		{"topk function integration", "topk", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])", Params: map[string]interface{}{"k": 2.0}}, true},
		{"bottomk function integration", "bottomk", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])", Params: map[string]interface{}{"k": 2.0}}, true},
		{"distinct function integration", "distinct", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"histogram function integration", "histogram", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"outliers_iqr function integration", "outliers_iqr", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])"}, true},
		{"outliersk function integration", "outliersk", models.MetricsQLFunctionRequest{Query: "rate(http_requests_total[5m])", Params: map[string]interface{}{"k": 2.0}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			// Create request body
			requestBody := tt.requestBody
			jsonBody, err := json.Marshal(requestBody)
			require.NoError(t, err)

			// Create POST request
			req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics/query/aggregate/"+tt.function, bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status code 200, got %d. Body: %s", w.Code, w.Body.String())
				return
			}

			// Parse response
			var response models.VictoriaMetricsResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse response JSON: %v", err)
				return
			}

			if response.Status != "success" {
				t.Errorf("Expected response status 'success', got '%s'", response.Status)
				return
			}

			if tt.expectSeries {
				data, ok := response.Data.(map[string]any)
				if !ok {
					t.Errorf("Expected data to be a map")
					return
				}

				result, ok := data["result"].([]any)
				if !ok {
					t.Errorf("Expected result to be an array")
					return
				}

				if len(result) == 0 {
					t.Errorf("Expected at least one result series")
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}