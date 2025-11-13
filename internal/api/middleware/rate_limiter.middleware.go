package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/api/constants"
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

// RateLimiter implements per-tenant rate limiting using Valkey cluster
func RateLimiter(valkeyCache cache.ValkeyCluster) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = constants.AnonymousTenantID
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
			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":  "error",
				"error":   "Rate limit exceeded",
				"message": "Too many requests",
			})
			c.Abort()
			return
		}

		// Increment request count
		currentCount++
		valkeyCache.Set(c.Request.Context(), key, []byte(strconv.FormatInt(currentCount, 10)), time.Minute)

		c.Header("X-Rate-Limit-Limit", strconv.FormatInt(maxRequests, 10))
		c.Header("X-Rate-Limit-Remaining", strconv.FormatInt(maxRequests-currentCount, 10))
		c.Next()
	}
}
