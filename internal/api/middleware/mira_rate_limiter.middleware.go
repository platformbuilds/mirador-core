package middleware

import "github.com/gin-gonic/gin"

// MIRARateLimiter is a no-op stub. The MIRA-specific rate limiting
// implementation has been migrated out of the core repository.
func MIRARateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
