package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// SearchQueryThrottlingMiddleware implements throttling for search queries based on complexity
type SearchQueryThrottlingMiddleware struct {
	valkeyCache cache.ValkeyCluster
	logger      logger.Logger
}

// NewSearchQueryThrottlingMiddleware creates a new search query throttling middleware
func NewSearchQueryThrottlingMiddleware(valkeyCache cache.ValkeyCluster, logger logger.Logger) *SearchQueryThrottlingMiddleware {
	return &SearchQueryThrottlingMiddleware{
		valkeyCache: valkeyCache,
		logger:      logger,
	}
}

// ThrottleLogsQuery implements throttling for logs search queries
func (m *SearchQueryThrottlingMiddleware) ThrottleLogsQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read the request body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			m.logger.Error("Failed to read request body", "error", err)
			c.Next()
			return
		}

		var req models.LogsQLQueryRequest

		// Unmarshal the JSON request
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			// If unmarshaling fails, reset the body and let the handler deal with it
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			c.Next()
			return
		}

		// Reset the request body for the handler
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = "anonymous"
		}

		// Calculate query complexity score
		complexity := m.calculateLogsQueryComplexity(&req)

		// Apply throttling based on complexity
		if !m.checkThrottleLimit(c, tenantID, "logs", complexity) {
			m.logger.Warn("Logs query throttled due to complexity",
				"tenant", tenantID,
				"complexity", complexity,
				"query", req.Query)

			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":      "error",
				"error":       "Query too complex, please simplify and try again",
				"retry_after": 60,
			})
			c.Abort()
			return
		}

		// Add complexity score to context for monitoring
		c.Set("query_complexity", complexity)
		c.Next()
	}
}

// ThrottleTracesQuery implements throttling for traces search queries
func (m *SearchQueryThrottlingMiddleware) ThrottleTracesQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read the request body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			m.logger.Error("Failed to read request body", "error", err)
			c.Next()
			return
		}

		var req models.TraceSearchRequest

		// Unmarshal the JSON request
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			// If unmarshaling fails, reset the body and let the handler deal with it
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			c.Next()
			return
		}

		// Reset the request body for the handler
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = "anonymous"
		}

		// Calculate query complexity score
		complexity := m.calculateTracesQueryComplexity(&req)

		// Apply throttling based on complexity
		if !m.checkThrottleLimit(c, tenantID, "traces", complexity) {
			m.logger.Warn("Traces query throttled due to complexity",
				"tenant", tenantID,
				"complexity", complexity,
				"query", req.Query)

			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":      "error",
				"error":       "Query too complex, please simplify and try again",
				"retry_after": 60,
			})
			c.Abort()
			return
		}

		// Add complexity score to context for monitoring
		c.Set("query_complexity", complexity)
		c.Next()
	}
}

// calculateLogsQueryComplexity calculates a complexity score for logs queries
func (m *SearchQueryThrottlingMiddleware) calculateLogsQueryComplexity(req *models.LogsQLQueryRequest) int {
	complexity := 0

	// Base complexity from query length
	queryLen := len(req.Query)
	if queryLen > 1000 {
		complexity += 50
	} else if queryLen > 500 {
		complexity += 25
	} else if queryLen > 100 {
		complexity += 10
	}

	// Operators increase complexity
	operators := []string{" AND ", " OR ", " NOT ", "&&", "||", "!"}
	for _, op := range operators {
		count := strings.Count(strings.ToUpper(req.Query), op)
		complexity += count * 5
	}

	// Wildcards and regex patterns
	if strings.Contains(req.Query, "*") || strings.Contains(req.Query, "~") {
		complexity += 15
	}

	// Field-specific searches are more complex
	if strings.Contains(req.Query, ":") {
		colonCount := strings.Count(req.Query, ":")
		complexity += colonCount * 3
	}

	// Bleve queries are generally more complex to process
	if req.SearchEngine == "bleve" {
		complexity += 20
	}

	// Large time ranges increase complexity
	if req.End > req.Start {
		timeRangeHours := (req.End - req.Start) / (1000 * 60 * 60) // milliseconds to hours
		if timeRangeHours > 24*30 {                                // More than 30 days
			complexity += 30
		} else if timeRangeHours > 24*7 { // More than 7 days
			complexity += 15
		} else if timeRangeHours > 24 { // More than 1 day
			complexity += 5
		}
	}

	// Limit affects complexity (higher limits = more work)
	if req.Limit > 10000 {
		complexity += 40
	} else if req.Limit > 1000 {
		complexity += 20
	} else if req.Limit > 100 {
		complexity += 5
	}

	return complexity
}

// calculateTracesQueryComplexity calculates a complexity score for traces queries
func (m *SearchQueryThrottlingMiddleware) calculateTracesQueryComplexity(req *models.TraceSearchRequest) int {
	complexity := 0

	// Base complexity from query string
	if req.Query != "" {
		queryLen := len(req.Query)
		if queryLen > 1000 {
			complexity += 50
		} else if queryLen > 500 {
			complexity += 25
		} else if queryLen > 100 {
			complexity += 10
		}

		// Operators in trace queries
		operators := []string{" AND ", " OR ", " NOT ", "&&", "||", "!"}
		for _, op := range operators {
			count := strings.Count(strings.ToUpper(req.Query), op)
			complexity += count * 5
		}

		// Bleve queries are more complex
		if req.SearchEngine == "bleve" {
			complexity += 20
		}
	}

	// Tags increase complexity
	if req.Tags != "" {
		tagCount := strings.Count(req.Tags, ",")
		complexity += tagCount * 2
	}

	// Duration filters
	if req.MinDuration != "" || req.MaxDuration != "" {
		complexity += 10
	}

	// Large time ranges
	if !req.End.IsZero() && !req.Start.IsZero() {
		timeRange := req.End.Time.Sub(req.Start.Time)
		hours := timeRange.Hours()
		if hours > 24*30 { // More than 30 days
			complexity += 30
		} else if hours > 24*7 { // More than 7 days
			complexity += 15
		} else if hours > 24 { // More than 1 day
			complexity += 5
		}
	}

	// High limits
	if req.Limit > 10000 {
		complexity += 40
	} else if req.Limit > 1000 {
		complexity += 20
	} else if req.Limit > 100 {
		complexity += 5
	}

	return complexity
}

// checkThrottleLimit checks if the request should be throttled based on complexity
func (m *SearchQueryThrottlingMiddleware) checkThrottleLimit(c *gin.Context, tenantID, queryType string, complexity int) bool {
	// Define complexity thresholds
	var maxComplexity int
	switch queryType {
	case "logs":
		maxComplexity = 100 // Allow up to 100 complexity points for logs
	case "traces":
		maxComplexity = 80 // Slightly lower for traces due to different processing
	default:
		maxComplexity = 50 // Conservative default
	}

	// If complexity is within limits, allow the request
	if complexity <= maxComplexity {
		return true
	}

	// For very high complexity queries, implement rate limiting
	if complexity > maxComplexity*2 {
		return m.checkRateLimit(c, tenantID, queryType, "high_complexity", 1, 5*time.Minute) // 1 request per 5 minutes
	}

	// For moderately high complexity, allow but with reduced rate
	return m.checkRateLimit(c, tenantID, queryType, "moderate_complexity", 5, time.Minute) // 5 requests per minute
}

// checkRateLimit implements token bucket rate limiting using Valkey
func (m *SearchQueryThrottlingMiddleware) checkRateLimit(c *gin.Context, tenantID, queryType, complexityLevel string, maxRequests int64, window time.Duration) bool {
	// Create a unique key for this tenant, query type, and complexity level
	key := fmt.Sprintf("throttle:%s:%s:%s:%d", tenantID, queryType, complexityLevel, time.Now().Unix()/int64(window.Seconds()))

	// Get current request count
	countBytes, err := m.valkeyCache.Get(c.Request.Context(), key)
	var currentCount int64 = 0

	if err == nil {
		if count, err := parseInt64(string(countBytes)); err == nil {
			currentCount = count
		}
	}

	// Check if limit exceeded
	if currentCount >= maxRequests {
		return false
	}

	// Increment counter
	newCount := currentCount + 1
	m.valkeyCache.Set(c.Request.Context(), key, fmt.Sprintf("%d", newCount), window*2) // Keep for 2 windows

	return true
}

// parseInt64 is a helper to parse int64 from string
func parseInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
