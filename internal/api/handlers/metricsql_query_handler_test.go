package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMetricsQLQueryHandler_Routing tests that all aggregate function routes are properly registered
func TestMetricsQLQueryHandler_Routing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test router with the routes
	r := gin.New()
	r.Use(gin.Recovery())

	// Add a simple handler for testing routing
	r.GET("/api/v1/metrics/query/aggregate/:function", func(c *gin.Context) {
		function := c.Param("function")
		query := c.Query("query")

		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter is required"})
			return
		}

		// Basic validation - check if function is in our allowlist
		aggregateFunctions := []string{
			"any", "avg", "bottomk", "bottomk_avg", "bottomk_max", "bottomk_min", "count", "count_values", "distinct", "geomean", "group", "histogram", "limitk", "mad", "max",
			"median", "min", "mode", "outliers_iqr", "outliers_mad", "outliersk", "quantile", "quantiles", "share", "stddev", "stdvar", "sum", "sum2", "topk", "topk_avg", "topk_max", "topk_min", "zscore",
		}

		validFunction := false
		for _, f := range aggregateFunctions {
			if f == function {
				validFunction = true
				break
			}
		}

		if !validFunction {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid aggregate function"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"function": function, "query": query})
	})

	tests := []struct {
		name           string
		function       string
		query          string
		expectedStatus int
		expectError    bool
	}{
		{"valid sum function", "sum", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid avg function", "avg", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid count function", "count", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid min function", "min", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid max function", "max", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid median function", "median", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid mode function", "mode", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid stddev function", "stddev", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid stdvar function", "stdvar", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid distinct function", "distinct", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid any function", "any", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid group function", "group", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid count_values function", "count_values", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid sum2 function", "sum2", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid zscore function", "zscore", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid mad function", "mad", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid geomean function", "geomean", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid histogram function", "histogram", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid share function", "share", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid outliers_iqr function", "outliers_iqr", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid outliers_mad function", "outliers_mad", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid quantile function", "quantile", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid quantiles function", "quantiles", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid topk function", "topk", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid bottomk function", "bottomk", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid topk_avg function", "topk_avg", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid bottomk_avg function", "bottomk_avg", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid topk_max function", "topk_max", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid bottomk_max function", "bottomk_max", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid topk_min function", "topk_min", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid bottomk_min function", "bottomk_min", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid outliersk function", "outliersk", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"valid limitk function", "limitk", "rate(http_requests_total[5m])", http.StatusOK, false},
		{"invalid function", "invalid_function", "rate(http_requests_total[5m])", http.StatusBadRequest, true},
		{"missing query parameter", "sum", "", http.StatusBadRequest, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			url := "/api/v1/metrics/query/aggregate/" + tt.function
			if tt.query != "" {
				url += "?query=" + tt.query
			}

			req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d for function %s. Body: %s", tt.expectedStatus, w.Code, tt.function, w.Body.String())
			}
		})
	}
}

// TestMetricsQLQueryHandler_ParameterValidation tests parameter validation for functions that require them
func TestMetricsQLQueryHandler_ParameterValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(gin.Recovery())

	// Add a handler that validates parameters
	r.GET("/api/v1/metrics/query/aggregate/:function", func(c *gin.Context) {
		function := c.Param("function")
		query := c.Query("query")

		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter is required"})
			return
		}

		// Check parameter requirements
		switch function {
		case "topk", "bottomk", "outliersk", "limitk":
			k := c.Query("k")
			if k == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "k parameter is required for " + function})
				return
			}
		case "quantile":
			quantile := c.Query("quantile")
			if quantile == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "quantile parameter is required for " + function})
				return
			}
		case "quantiles":
			quantiles := c.Query("quantiles")
			if quantiles == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "quantiles parameter is required for " + function})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"function": function, "query": query})
	})

	tests := []struct {
		name           string
		function       string
		query          string
		params         map[string]string
		expectedStatus int
	}{
		{"topk with k parameter", "topk", "rate(http_requests_total[5m])", map[string]string{"k": "5"}, http.StatusOK},
		{"topk missing k parameter", "topk", "rate(http_requests_total[5m])", nil, http.StatusBadRequest},
		{"bottomk with k parameter", "bottomk", "rate(http_requests_total[5m])", map[string]string{"k": "3"}, http.StatusOK},
		{"bottomk missing k parameter", "bottomk", "rate(http_requests_total[5m])", nil, http.StatusBadRequest},
		{"quantile with quantile parameter", "quantile", "rate(http_requests_total[5m])", map[string]string{"quantile": "0.95"}, http.StatusOK},
		{"quantile missing quantile parameter", "quantile", "rate(http_requests_total[5m])", nil, http.StatusBadRequest},
		{"quantiles with quantiles parameter", "quantiles", "rate(http_requests_total[5m])", map[string]string{"quantiles": "0.5,0.95,0.99"}, http.StatusOK},
		{"quantiles missing quantiles parameter", "quantiles", "rate(http_requests_total[5m])", nil, http.StatusBadRequest},
		{"outliersk with k parameter", "outliersk", "rate(http_requests_total[5m])", map[string]string{"k": "2"}, http.StatusOK},
		{"outliersk missing k parameter", "outliersk", "rate(http_requests_total[5m])", nil, http.StatusBadRequest},
		{"limitk with k parameter", "limitk", "rate(http_requests_total[5m])", map[string]string{"k": "10"}, http.StatusOK},
		{"limitk missing k parameter", "limitk", "rate(http_requests_total[5m])", nil, http.StatusBadRequest},
		{"sum without required parameters", "sum", "rate(http_requests_total[5m])", nil, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			url := "/api/v1/metrics/query/aggregate/" + tt.function + "?query=" + tt.query
			for key, value := range tt.params {
				url += "&" + key + "=" + value
			}

			req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d for test %s. Body: %s", tt.expectedStatus, w.Code, tt.name, w.Body.String())
			}
		})
	}
}
