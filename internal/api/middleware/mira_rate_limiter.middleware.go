package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

// MIRARateLimiter implements configurable rate limiting for MIRA endpoints using Valkey cluster.
// This is more restrictive than the default rate limiter to control AI API costs.
func MIRARateLimiter(valkeyCache cache.ValkeyCluster, cfg config.RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if rate limiting is disabled
		if !cfg.Enabled {
			c.Next()
			return
		}

		// Rate limiting key with MIRA prefix
		window := time.Now().Unix() / 60 // 1-minute windows
		key := fmt.Sprintf("rate_limit:mira:%s:%d", c.ClientIP(), int(window))

		// Get current request count
		countBytes, err := valkeyCache.Get(c.Request.Context(), key)
		var currentCount int64 = 0

		if err == nil {
			if count, err := strconv.ParseInt(string(countBytes), 10, 64); err == nil {
				currentCount = count
			}
		}

		// Apply configured rate limit
		maxRequests := int64(cfg.RequestsPerMinute)
		if currentCount >= maxRequests {
			c.Header("X-RateLimit-Limit", strconv.FormatInt(maxRequests, 10))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(window+60, 10))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":     "error",
				"error":      "mira_rate_limit_exceeded",
				"message":    fmt.Sprintf("MIRA rate limit exceeded. Maximum %d requests per minute allowed.", maxRequests),
				"retryAfter": 60,
			})
			c.Abort()
			return
		}

		// Increment request count
		currentCount++
		valkeyCache.Set(c.Request.Context(), key, strconv.FormatInt(currentCount, 10), time.Minute)

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.FormatInt(maxRequests, 10))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(maxRequests-currentCount, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(window+60, 10))

		c.Next()
	}
}
