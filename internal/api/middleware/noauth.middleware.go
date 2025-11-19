package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// NoAuthMiddleware injects a default context when running without authentication.
// Mirador-core is designed to run behind an external auth/gateway layer.
// This middleware provides a minimal context for backward compatibility and local testing.
func NoAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a minimal session context
		session := &models.UserSession{
			ID:           "system-session",
			UserID:       "system",
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Settings:     make(map[string]interface{}),
		}

		c.Set("session", session)
		c.Set("user_id", "system")
		c.Set("session_id", "system-session")
		// Provide an internal default tenant id for single-tenant mode
		// Respect X-Tenant-ID header for backwards compatibility but default to the single-tenant id.
		tenant := c.GetHeader("X-Tenant-ID")
		if tenant == "" {
			tenant = models.DefaultTenantID
		}
		c.Set("default", tenant)
		c.Set("tenant_id", tenant)
		c.Next()
	}
}
