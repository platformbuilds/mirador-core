package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// CorrelationService handles request correlation and distributed tracing
type CorrelationService struct {
	config *config.Config
	cache  cache.ValkeyCluster
	repo   repo.SchemaStore
	logger logger.Logger
}

// NewCorrelationService creates a new correlation service
func NewCorrelationService(cfg *config.Config, cache cache.ValkeyCluster, repo repo.SchemaStore, logger logger.Logger) *CorrelationService {
	return &CorrelationService{
		config: cfg,
		cache:  cache,
		repo:   repo,
		logger: logger,
	}
}

// CorrelationMiddleware adds correlation ID and request tracing
func (cs *CorrelationService) CorrelationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate or extract correlation ID
		correlationID := cs.extractOrGenerateCorrelationID(c)

		// Generate request ID
		requestID := cs.generateRequestID()

		// Set correlation headers
		c.Header("X-Correlation-ID", correlationID)
		c.Header("X-Request-ID", requestID)

		// Add to context for logging and downstream use
		ctx := context.WithValue(c.Request.Context(), "correlation_id", correlationID)
		ctx = context.WithValue(ctx, "request_id", requestID)
		c.Request = c.Request.WithContext(ctx)

		// Set in Gin context for easy access
		c.Set("correlation_id", correlationID)
		c.Set("request_id", requestID)

		// Start request tracing
		startTime := time.Now()
		cs.logRequestStart(c, correlationID, requestID, startTime)

		// Capture request body for logging if needed
		if cs.shouldCaptureRequestBody(c) {
			cs.captureRequestBody(c)
		}

		// Wrap response writer to capture response
		responseWriter := &responseCaptureWriter{ResponseWriter: c.Writer, statusCode: 200}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Log request completion
		duration := time.Since(startTime)
		cs.logRequestCompletion(c, correlationID, requestID, duration, responseWriter.statusCode)

		// Store request trace if configured
		if cs.shouldStoreTrace(c) {
			cs.storeRequestTrace(c, correlationID, requestID, startTime, duration, responseWriter.statusCode)
		}
	}
}

// extractOrGenerateCorrelationID extracts correlation ID from headers or generates new one
func (cs *CorrelationService) extractOrGenerateCorrelationID(c *gin.Context) string {
	// Check for correlation ID in headers
	if correlationID := c.GetHeader("X-Correlation-ID"); correlationID != "" {
		return correlationID
	}

	// Check for correlation ID in query parameters
	if correlationID := c.Query("correlation_id"); correlationID != "" {
		return correlationID
	}

	// Generate new correlation ID
	return cs.generateCorrelationID()
}

// generateCorrelationID generates a unique correlation ID
func (cs *CorrelationService) generateCorrelationID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("corr_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("corr_%s", hex.EncodeToString(bytes))
}

// generateRequestID generates a unique request ID
func (cs *CorrelationService) generateRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("req_%s", hex.EncodeToString(bytes))
}

// logRequestStart logs the start of a request
func (cs *CorrelationService) logRequestStart(c *gin.Context, correlationID, requestID string, startTime time.Time) {
	cs.logger.Info("Request started",
		"correlation_id", correlationID,
		"request_id", requestID,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"query", c.Request.URL.RawQuery,
		"user_agent", c.Request.UserAgent(),
		"remote_addr", c.ClientIP(),
		"content_length", c.Request.ContentLength,
		"content_type", c.Request.Header.Get("Content-Type"),
		"start_time", startTime.Format(time.RFC3339Nano),
	)
}

// logRequestCompletion logs the completion of a request
func (cs *CorrelationService) logRequestCompletion(c *gin.Context, correlationID, requestID string, duration time.Duration, statusCode int) {
	// Determine log level based on status code
	fields := []interface{}{
		"correlation_id", correlationID,
		"request_id", requestID,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
		"duration", duration.String(),
	}

	if statusCode >= 400 && statusCode < 500 {
		cs.logger.Warn("Request completed", fields...)
	} else if statusCode >= 500 {
		cs.logger.Error("Request completed", fields...)
	} else {
		cs.logger.Info("Request completed", fields...)
	}
}

// shouldCaptureRequestBody determines if request body should be captured
func (cs *CorrelationService) shouldCaptureRequestBody(c *gin.Context) bool {
	// Only capture for certain content types and methods
	contentType := c.Request.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") &&
		!strings.Contains(contentType, "application/xml") &&
		!strings.Contains(contentType, "text/") {
		return false
	}

	// Only capture for POST, PUT, PATCH methods
	switch c.Request.Method {
	case "POST", "PUT", "PATCH":
		return cs.config.LogLevel == "debug" // Use debug level as proxy for detailed logging
	default:
		return false
	}
}

// captureRequestBody captures the request body for logging
func (cs *CorrelationService) captureRequestBody(c *gin.Context) {
	// Read request body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		cs.logger.Warn("Failed to read request body", "error", err)
		return
	}

	// Restore request body for further processing
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Log request body (truncated if too long)
	bodyStr := string(bodyBytes)
	if len(bodyStr) > 1000 {
		bodyStr = bodyStr[:1000] + "..."
	}

	correlationID, _ := c.Get("correlation_id")
	cs.logger.Debug("Request body",
		"correlation_id", correlationID,
		"body", bodyStr,
	)
}

// shouldStoreTrace determines if request trace should be stored
func (cs *CorrelationService) shouldStoreTrace(c *gin.Context) bool {
	// Store traces based on configuration
	if !cs.config.Monitoring.TracingEnabled {
		return false
	}

	// Store traces for errors
	if c.Writer.Status() >= 400 {
		return true
	}

	// Store traces for slow requests (using 1 second as default threshold)
	duration := time.Since(c.GetTime("start_time"))
	if duration > time.Second {
		return true
	}

	// Store sample of requests based on sampling rate
	// For simplicity, store 10% of requests
	return true // Simplified - always store for now
}

// storeRequestTrace stores request trace information
func (cs *CorrelationService) storeRequestTrace(c *gin.Context, correlationID, requestID string, startTime time.Time, duration time.Duration, statusCode int) {
	trace := map[string]interface{}{
		"correlation_id": correlationID,
		"request_id":     requestID,
		"method":         c.Request.Method,
		"path":           c.Request.URL.Path,
		"query":          c.Request.URL.RawQuery,
		"status_code":    statusCode,
		"duration_ms":    duration.Milliseconds(),
		"user_agent":     c.Request.UserAgent(),
		"remote_addr":    c.ClientIP(),
		"content_length": c.Request.ContentLength,
		"start_time":     startTime,
		"end_time":       startTime.Add(duration),
		"headers":        cs.sanitizeHeaders(c.Request.Header),
	}

	// Add user context if available
	if userID, exists := c.Get("user_id"); exists {
		trace["user_id"] = userID
	}
	if tenantID, exists := c.Get("tenant_id"); exists {
		trace["tenant_id"] = tenantID
	}

	// Store in cache with TTL (1 hour default)
	cacheKey := fmt.Sprintf("trace:%s", requestID)
	ctx := c.Request.Context()
	if err := cs.cache.Set(ctx, cacheKey, trace, time.Hour); err != nil {
		cs.logger.Warn("Failed to store request trace", "error", err, "request_id", requestID)
	}
}

// sanitizeHeaders removes sensitive headers from logging
func (cs *CorrelationService) sanitizeHeaders(headers http.Header) map[string]string {
	sanitized := make(map[string]string)

	sensitiveHeaders := map[string]bool{
		"authorization":   true,
		"x-api-key":       true,
		"x-auth-token":    true,
		"cookie":          true,
		"set-cookie":      true,
		"x-session-token": true,
		"x-csrf-token":    true,
	}

	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			sanitized[key] = "[REDACTED]"
		} else {
			sanitized[key] = strings.Join(values, ", ")
		}
	}

	return sanitized
}

// GetRequestTrace retrieves a stored request trace
func (cs *CorrelationService) GetRequestTrace(ctx context.Context, requestID string) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("trace:%s", requestID)
	data, err := cs.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	var trace map[string]interface{}
	if err := json.Unmarshal(data, &trace); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trace: %w", err)
	}

	return trace, nil
}

// GetTracesByCorrelationID retrieves all traces for a correlation ID
func (cs *CorrelationService) GetTracesByCorrelationID(ctx context.Context, correlationID string) ([]map[string]interface{}, error) {
	// This would require a more sophisticated storage mechanism
	// For now, return empty slice
	return []map[string]interface{}{}, nil
}

// responseCaptureWriter captures response status code
type responseCaptureWriter struct {
	gin.ResponseWriter
	statusCode int
}

func (w *responseCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// DistributedTracingMiddleware adds distributed tracing headers
func (cs *CorrelationService) DistributedTracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tracing headers from incoming request
		traceID := c.GetHeader("X-B3-TraceId")
		spanID := c.GetHeader("X-B3-SpanId")
		parentSpanID := c.GetHeader("X-B3-ParentSpanId")
		sampled := c.GetHeader("X-B3-Sampled")

		// Generate trace ID if not present
		if traceID == "" {
			traceID = cs.generateTraceID()
			sampled = "1" // Sample this request
		}

		// Generate span ID if not present
		if spanID == "" {
			spanID = cs.generateSpanID()
		}

		// Set tracing headers for outgoing requests
		c.Header("X-B3-TraceId", traceID)
		c.Header("X-B3-SpanId", spanID)
		if parentSpanID != "" {
			c.Header("X-B3-ParentSpanId", parentSpanID)
		}
		c.Header("X-B3-Sampled", sampled)

		// Add to context
		ctx := context.WithValue(c.Request.Context(), "trace_id", traceID)
		ctx = context.WithValue(ctx, "span_id", spanID)
		c.Request = c.Request.WithContext(ctx)

		c.Set("trace_id", traceID)
		c.Set("span_id", spanID)

		// Create child span for this request
		childSpanID := cs.generateSpanID()
		c.Set("child_span_id", childSpanID)

		c.Next()

		// Log span completion
		cs.logSpanCompletion(c, traceID, spanID, childSpanID)
	}
}

// generateTraceID generates a distributed tracing trace ID
func (cs *CorrelationService) generateTraceID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// generateSpanID generates a distributed tracing span ID
func (cs *CorrelationService) generateSpanID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// logSpanCompletion logs the completion of a tracing span
func (cs *CorrelationService) logSpanCompletion(c *gin.Context, traceID, spanID, childSpanID string) {
	duration := time.Since(c.GetTime("start_time"))

	cs.logger.Debug("Span completed",
		"trace_id", traceID,
		"span_id", spanID,
		"child_span_id", childSpanID,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status_code", c.Writer.Status(),
		"duration_ms", duration.Milliseconds(),
	)
}

// HealthCheckMiddleware provides health check endpoint with correlation
func (cs *CorrelationService) HealthCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" && c.Request.URL.Path == "/health" {
			correlationID, _ := c.Get("correlation_id")
			cs.logger.Info("Health check requested", "correlation_id", correlationID)

			c.JSON(http.StatusOK, gin.H{
				"status":         "healthy",
				"timestamp":      time.Now().Format(time.RFC3339),
				"correlation_id": correlationID,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
