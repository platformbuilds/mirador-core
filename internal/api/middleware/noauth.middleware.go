package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// NoAuthMiddleware injects a default user/tenant context when auth is disabled.
// It honors X-Tenant-ID header if provided, otherwise falls back to "default".
func NoAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant := c.GetHeader("X-Tenant-ID")
		if strings.TrimSpace(tenant) == "" {
			tenant = models.DefaultTenantID
		}

		// Create a dummy session for consistency with AuthMiddleware
		// For single-tenant, give admin-like permissions for testing
		session := &models.UserSession{
			ID:           "anonymous-session",
			UserID:       "anonymous",
			TenantID:     tenant,
			Roles:        []string{"global_admin"}, // Give full permissions for testing
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Settings:     make(map[string]interface{}),
		}

		c.Set("session", session)
		c.Set("tenant_id", tenant)
		c.Set("user_id", "anonymous")
		c.Set("user_roles", []string{"global_admin"}) // Give full permissions for testing
		c.Set("session_id", "anonymous-session")
		c.Next()
	}
}
