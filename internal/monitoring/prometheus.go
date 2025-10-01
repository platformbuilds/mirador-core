// Packa// 2. HTTP metrics are automatically collected by the existing middleware
//    (no need to add HTTPMetricsMiddleware) monitoring provides comprehensive Prometheus metrics for MIRADOR-CORE API.
//
// Usage:
//
//  1. Setup metrics in your main function:
//     router := gin.New()
//     monitoring.SetupPrometheusMetrics(router)
//
//  2. Add HTTP metrics middleware:
//     router.Use(monitoring.HTTPMetricsMiddleware())
//
// 3. Record custom metrics in your handlers:
//
//	// Database operations
//	start := time.Now()
//	// ... your DB code ...
//	monitoring.RecordDBOperation("select", "users", time.Since(start), true)
//
//	// Cache operations
//	monitoring.RecordCacheOperation("get", "hit")
//
//	// API operations
//	monitoring.RecordAPIOperation("create_user", "users", time.Since(start), true)
//
//	// Victoria Metrics queries
//	monitoring.RecordVictoriaMetricsQuery("instant", time.Since(start), true)
//
//	// Weaviate operations
//	monitoring.RecordWeaviateOperation("search", "documents", time.Since(start), true)
//
//	// Authentication attempts
//	monitoring.RecordAuthAttempt("ldap", "success")
//
// Available Metrics:
//
// HTTP Metrics (from existing middleware):
//   - mirador_core_http_requests_total{method, endpoint, status_code, tenant_id}
//   - mirador_core_http_request_duration_seconds{method, endpoint, tenant_id}
//   - mirador_core_active_connections (from this package)
//
// Database Metrics:
//   - mirador_core_db_operations_total{operation, table, status}
//   - mirador_core_db_operation_duration_seconds{operation, table}
//
// Cache Metrics:
//   - mirador_core_cache_operations_total{operation, result}
//
// Authentication Metrics:
//   - mirador_core_auth_attempts_total{method, result}
//
// API Operation Metrics:
//   - mirador_core_api_operations_total{operation, resource, status}
//   - mirador_core_api_operation_duration_seconds{operation, resource}
//
// Victoria Metrics:
//   - mirador_core_victoria_metrics_queries_total{query_type, status}
//   - mirador_core_victoria_metrics_query_duration_seconds{query_type}
//
// Weaviate Metrics:
//   - mirador_core_weaviate_operations_total{operation, collection, status}
//   - mirador_core_weaviate_operation_duration_seconds{operation, collection}
//
// Error Metrics:
//   - mirador_core_errors_total{type, component}
//
// Build Info:
//   - mirador_core_build_info{version, component, go_version}
package monitoring

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP request metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code", "tenant_id"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "tenant_id"},
	)

	// Database operation metrics
	dbOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_db_operations_total",
			Help: "Total number of database operations",
		},
		[]string{"operation", "table", "status"},
	)

	dbOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_db_operation_duration_seconds",
			Help:    "Database operation duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "table"},
	)

	// Cache metrics
	cacheOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_cache_operations_total",
			Help: "Total number of cache operations",
		},
		[]string{"operation", "result"}, // result: hit, miss, error
	)

	// Authentication metrics
	authAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"method", "result"}, // result: success, failure
	)

	// API operation metrics
	apiOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_api_operations_total",
			Help: "Total number of API operations",
		},
		[]string{"operation", "resource", "status"},
	)

	apiOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_api_operation_duration_seconds",
			Help:    "API operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "resource"},
	)

	// Victoria Metrics operations
	victoriaMetricsQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_victoria_metrics_queries_total",
			Help: "Total number of Victoria Metrics queries",
		},
		[]string{"query_type", "status"},
	)

	victoriaMetricsQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_victoria_metrics_query_duration_seconds",
			Help:    "Victoria Metrics query duration in seconds",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 25, 50, 100},
		},
		[]string{"query_type"},
	)

	// Weaviate operations
	weaviateOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_weaviate_operations_total",
			Help: "Total number of Weaviate operations",
		},
		[]string{"operation", "collection", "status"},
	)

	weaviateOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mirador_core_weaviate_operation_duration_seconds",
			Help:    "Weaviate operation duration in seconds",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 25, 50, 100},
		},
		[]string{"operation", "collection"},
	)

	// Active connections gauge
	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mirador_core_active_connections",
			Help: "Number of active connections",
		},
	)

	// Error rate metrics
	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_errors_total",
			Help: "Total number of errors",
		},
		[]string{"type", "component"}, // type: api, db, cache, auth, etc.
	)
)

// SetupPrometheusMetrics configures Prometheus metrics endpoint for MIRADOR-CORE
func SetupPrometheusMetrics(router gin.IRoutes) {
	// Use the default Prometheus registry to combine with existing metrics

	// Register build info (ignore if already registered)
	_ = prometheus.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "mirador_core_build_info",
		Help: "Build information for MIRADOR-CORE",
		ConstLabels: prometheus.Labels{
			"version":    "v2.1.3",
			"component":  "mirador-core",
			"go_version": "1.21",
		},
	}, func() float64 { return 1 }))

	// Register HTTP metrics (these might conflict with existing ones, so ignore errors)
	_ = prometheus.Register(httpRequestsTotal)
	_ = prometheus.Register(httpRequestDuration)

	// Register additional metrics (ignore if already registered)
	_ = prometheus.Register(dbOperationsTotal)
	_ = prometheus.Register(dbOperationDuration)
	_ = prometheus.Register(cacheOperationsTotal)
	_ = prometheus.Register(authAttemptsTotal)
	_ = prometheus.Register(apiOperationsTotal)
	_ = prometheus.Register(apiOperationDuration)
	_ = prometheus.Register(victoriaMetricsQueriesTotal)
	_ = prometheus.Register(victoriaMetricsQueryDuration)
	_ = prometheus.Register(weaviateOperationsTotal)
	_ = prometheus.Register(weaviateOperationDuration)
	_ = prometheus.Register(activeConnections)
	_ = prometheus.Register(errorsTotal)

	// Expose metrics endpoint using default registry
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// HTTPMetricsMiddleware collects HTTP request metrics
func HTTPMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		method := c.Request.Method
		path := c.Request.URL.Path

		// Normalize path for metrics (remove IDs, etc.)
		endpoint := normalizeEndpoint(path)

		// Get tenant_id from context (set by auth middleware)
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = "unknown"
		}

		// Increment active connections
		activeConnections.Inc()
		defer activeConnections.Dec()

		c.Next()

		// Record metrics
		statusCode := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(method, endpoint, statusCode, tenantID).Inc()
		httpRequestDuration.WithLabelValues(method, endpoint, tenantID).Observe(duration)

		// Record errors
		if c.Writer.Status() >= 400 {
			errorsTotal.WithLabelValues("http", endpoint).Inc()
		}
	}
}

// RecordDBOperation records database operation metrics
func RecordDBOperation(operation, table string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
		errorsTotal.WithLabelValues("db", table).Inc()
	}

	dbOperationsTotal.WithLabelValues(operation, table, status).Inc()
	dbOperationDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// RecordCacheOperation records cache operation metrics
func RecordCacheOperation(operation, result string) {
	cacheOperationsTotal.WithLabelValues(operation, result).Inc()
	if result == "error" {
		errorsTotal.WithLabelValues("cache", operation).Inc()
	}
}

// RecordAuthAttempt records authentication attempt metrics
func RecordAuthAttempt(method, result string) {
	authAttemptsTotal.WithLabelValues(method, result).Inc()
	if result == "failure" {
		errorsTotal.WithLabelValues("auth", method).Inc()
	}
}

// RecordAPIOperation records API operation metrics
func RecordAPIOperation(operation, resource string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
		errorsTotal.WithLabelValues("api", resource).Inc()
	}

	apiOperationsTotal.WithLabelValues(operation, resource, status).Inc()
	apiOperationDuration.WithLabelValues(operation, resource).Observe(duration.Seconds())
}

// RecordVictoriaMetricsQuery records Victoria Metrics query metrics
func RecordVictoriaMetricsQuery(queryType string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
		errorsTotal.WithLabelValues("victoria_metrics", queryType).Inc()
	}

	victoriaMetricsQueriesTotal.WithLabelValues(queryType, status).Inc()
	victoriaMetricsQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// RecordWeaviateOperation records Weaviate operation metrics
func RecordWeaviateOperation(operation, collection string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
		errorsTotal.WithLabelValues("weaviate", collection).Inc()
	}

	weaviateOperationsTotal.WithLabelValues(operation, collection, status).Inc()
	weaviateOperationDuration.WithLabelValues(operation, collection).Observe(duration.Seconds())
}

// normalizeEndpoint normalizes API endpoints for consistent metrics
func normalizeEndpoint(path string) string {
	// Remove numeric IDs from paths like /api/v1/users/123 -> /api/v1/users/:id
	// This is a simple implementation - you might want to make it more sophisticated
	if len(path) > 0 && path[len(path)-1] != '/' {
		path += "/"
	}

	// Simple normalization - replace numeric segments with :id
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if isNumeric(part) && i > 0 {
			parts[i] = ":id"
		}
	}

	return strings.Join(parts, "/")
}

// isNumeric checks if a string is numeric
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
