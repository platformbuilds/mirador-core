package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

// RBACMiddleware enforces role-based access control
func RBACMiddleware(requiredRoles []string, logger logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user roles from context (set by auth middleware)
		userRoles, exists := c.Get("user_roles")
		if !exists {
			logger.Warn("RBAC check failed: no roles found", "path", c.Request.URL.Path)
			c.JSON(http.StatusForbidden, gin.H{
				"status": "error",
				"error":  "Access denied: no roles assigned",
			})
			c.Abort()
			return
		}

		roles := userRoles.([]string)

		// Check if user has any of the required roles
		if !hasAnyRole(roles, requiredRoles) {
			logger.Warn("RBAC check failed: insufficient permissions",
				"userId", c.GetString("user_id"),
				"userRoles", roles,
				"requiredRoles", requiredRoles,
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{
				"status":         "error",
				"error":          "Access denied: insufficient permissions",
				"required_roles": requiredRoles,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminOnlyMiddleware restricts access to admin users
func AdminOnlyMiddleware(logger logger.Logger) gin.HandlerFunc {
	return RBACMiddleware([]string{"mirador-admin", "system-admin"}, logger)
}

// TenantIsolationMiddleware ensures tenant data isolation
func TenantIsolationMiddleware(logger logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			logger.Error("Tenant isolation failed: no tenant ID", "userId", c.GetString("user_id"))
			c.JSON(http.StatusForbidden, gin.H{
				"status": "error",
				"error":  "Tenant context required",
			})
			c.Abort()
			return
		}

		// Add tenant ID to all outgoing requests
		c.Header("X-Tenant-ID", tenantID)
		c.Next()
	}
}

func hasAnyRole(userRoles, requiredRoles []string) bool {
	roleMap := make(map[string]bool)
	for _, role := range userRoles {
		roleMap[role] = true
	}

	for _, required := range requiredRoles {
		if roleMap[required] {
			return true
		}
	}
	return false
}
