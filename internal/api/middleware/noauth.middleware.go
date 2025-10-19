package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// NoAuthMiddleware injects a default user/tenant context when auth is disabled.
// It honors X-Tenant-ID header if provided, otherwise falls back to "default".
func NoAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant := c.GetHeader("X-Tenant-ID")
		if strings.TrimSpace(tenant) == "" {
			tenant = "default"
		}
		c.Set("tenant_id", tenant)
		c.Set("user_id", "anonymous")
		c.Set("user_roles", []string{"viewer"})
		c.Set("session_id", "")
		c.Next()
	}
}
