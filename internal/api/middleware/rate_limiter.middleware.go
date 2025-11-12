package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/pkg/cache"
)

// Anonymous tenant ID for unauthenticated requests
const AnonymousTenantID = "anonymous"

// RateLimiter implements per-tenant rate limiting using Valkey cluster
func RateLimiter(valkeyCache cache.ValkeyCluster) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = AnonymousTenantID
		}

		// Rate limiting key
		window := time.Now().Unix() / 60 // 1-minute windows
		key := fmt.Sprintf("rate_limit:%s:%d", tenantID, window)

		// Get current request count
		countBytes, err := valkeyCache.Get(c.Request.Context(), key)
		var currentCount int64 = 0

		if err == nil {
			if count, err := strconv.ParseInt(string(countBytes), 10, 64); err == nil {
				currentCount = count
			}
		}

		// Rate limit: 1000 requests per minute per tenant
		maxRequests := int64(1000)
		if currentCount >= maxRequests {
			c.Header("X-Rate-Limit-Limit", strconv.FormatInt(maxRequests, 10))
			c.Header("X-Rate-Limit-Remaining", "0")
			c.Header("X-Rate-Limit-Reset", strconv.FormatInt((window+1)*60, 10))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":      "error",
				"error":       "Rate limit exceeded",
				"retry_after": 60,
			})
			c.Abort()
			return
		}

		// Increment counter
		newCount := currentCount + 1
		valkeyCache.Set(c.Request.Context(), key, newCount, 2*time.Minute)

		// Set rate limit headers
		c.Header("X-Rate-Limit-Limit", strconv.FormatInt(maxRequests, 10))
		c.Header("X-Rate-Limit-Remaining", strconv.FormatInt(maxRequests-newCount, 10))
		c.Header("X-Rate-Limit-Reset", strconv.FormatInt((window+1)*60, 10))

		c.Next()
	}
}
