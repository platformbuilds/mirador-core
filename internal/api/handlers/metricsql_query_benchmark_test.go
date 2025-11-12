package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// BenchmarkMetricsQLAggregateFunctions benchmarks the performance of MetricsQL aggregate function endpoints
func BenchmarkMetricsQLAggregateFunctions(b *testing.B) {
	// Set up mock VictoriaMetrics server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		response := models.VictoriaMetricsResponse{
			Status: "success",
			Data: map[string]any{
				"result": []any{
					map[string]any{
						"metric": map[string]string{"__name__": "http_requests_total"},
						"value":  []any{1703123456, "42.5"},
					},
				},
			},
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

	// Test data
	requestBody := models.MetricsQLFunctionRequest{
		Query: "rate(http_requests_total[5m])",
	}
	jsonBody, _ := json.Marshal(requestBody)

	// Benchmark different aggregate functions
	functions := []string{"sum", "avg", "count", "min", "max", "median"}

	for _, function := range functions {
		b.Run("Aggregate_"+function, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics/query/aggregate/"+function, bytes.NewBuffer(jsonBody))
				req.Header.Set("Content-Type", "application/json")

				r.ServeHTTP(w, req)
				if w.Code != http.StatusOK {
					b.Errorf("Expected status 200, got %d", w.Code)
				}
			}
		})
	}
}

// BenchmarkMetricsQLParameterFunctions benchmarks functions that require parameters
func BenchmarkMetricsQLParameterFunctions(b *testing.B) {
	// Set up mock VictoriaMetrics server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		response := models.VictoriaMetricsResponse{
			Status: "success",
			Data: map[string]any{
				"result": []any{
					map[string]any{
						"metric": map[string]string{"__name__": "http_requests_total"},
						"value":  []any{1703123456, "95"},
					},
				},
			},
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

	// Test data with parameters
	requestBody := models.MetricsQLFunctionRequest{
		Query:  "rate(http_requests_total[5m])",
		Params: map[string]interface{}{"quantile": 0.95, "k": 5.0},
	}
	jsonBody, _ := json.Marshal(requestBody)

	// Benchmark parameter-dependent functions
	functions := []string{"quantile", "topk", "bottomk", "outliersk"}

	for _, function := range functions {
		b.Run("Parameter_"+function, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics/query/aggregate/"+function, bytes.NewBuffer(jsonBody))
				req.Header.Set("Content-Type", "application/json")

				r.ServeHTTP(w, req)
				if w.Code != http.StatusOK {
					b.Errorf("Expected status 200, got %d", w.Code)
				}
			}
		})
	}
}
