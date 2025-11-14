package middleware

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TenantIsolationConfig configures tenant isolation behavior
type TenantIsolationConfig struct {
	// GlobalAdminRole defines the role that can access all tenants
	GlobalAdminRole string
	// TenantParamName defines the URL parameter name for tenant ID
	TenantParamName string
	// TenantHeaderName defines the header name for tenant override (admin only)
	TenantHeaderName string
	// AllowTenantOverride allows admins to override tenant context via header
	AllowTenantOverride bool
}

// DefaultTenantIsolationConfig returns default configuration
func DefaultTenantIsolationConfig() *TenantIsolationConfig {
	return &TenantIsolationConfig{
		GlobalAdminRole:     "global_admin",
		TenantParamName:     "tenantId",
		TenantHeaderName:    "X-Tenant-ID",
		AllowTenantOverride: true,
	}
}

// TenantIsolationMiddleware provides comprehensive tenant isolation
type TenantIsolationMiddleware struct {
	config      *TenantIsolationConfig
	rbacService *rbac.RBACService
	logger      logger.Logger
}

// NewTenantIsolationMiddleware creates a new tenant isolation middleware
func NewTenantIsolationMiddleware(config *TenantIsolationConfig, rbacService *rbac.RBACService, logger logger.Logger) *TenantIsolationMiddleware {
	if config == nil {
		config = DefaultTenantIsolationConfig()
	}
	return &TenantIsolationMiddleware{
		config:      config,
		rbacService: rbacService,
		logger:      logger,
	}
}

// TenantIsolation enforces tenant isolation for all requests
func (m *TenantIsolationMiddleware) TenantIsolation() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for public endpoints
		if m.isPublicEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Extract tenant ID from various sources
		tenantID, source := m.extractTenantID(c)
		if tenantID == "" {
			m.logger.Warn("No tenant ID found in request",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"user_id", c.GetString("user_id"))
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Tenant context required",
			})
			c.Abort()
			return
		}

		// Resolve tenant identifier to canonical ID
		resolvedTenantID, err := m.resolveTenantID(c.Request.Context(), tenantID)
		if err != nil {
			m.logger.Warn("Failed to resolve tenant identifier",
				"tenant_identifier", tenantID,
				"path", c.Request.URL.Path,
				"error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Invalid tenant context",
			})
			c.Abort()
			return
		}
		tenantID = resolvedTenantID

		// Validate tenant access
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Authentication required",
			})
			c.Abort()
			return
		}

		// Check if user has access to this tenant
		if err := m.validateTenantAccess(c, userID, tenantID); err != nil {
			m.logger.Warn("Tenant access denied",
				"user_id", userID,
				"tenant_id", tenantID,
				"path", c.Request.URL.Path,
				"error", err)
			c.JSON(http.StatusForbidden, gin.H{
				"status": "error",
				"error":  "Access denied to tenant",
			})
			c.Abort()
			return
		}

		// Set tenant context
		c.Set("tenant_id", tenantID)
		c.Set("tenant_source", source)

		// Add tenant header to response for debugging
		c.Header("X-Tenant-ID", tenantID)
		c.Header("X-Tenant-Source", source)

		m.logger.Debug("Tenant context established",
			"tenant_id", tenantID,
			"source", source,
			"user_id", userID,
			"path", c.Request.URL.Path)

		c.Next()
	}
}

// GlobalAdminOnly restricts access to global admin users only
func (m *TenantIsolationMiddleware) GlobalAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles := c.GetStringSlice("user_roles")
		userID := c.GetString("user_id")

		// Check if user has global admin role
		hasGlobalAdmin := false
		for _, role := range userRoles {
			if role == m.config.GlobalAdminRole {
				hasGlobalAdmin = true
				break
			}
		}

		if !hasGlobalAdmin {
			m.logger.Warn("Global admin access denied",
				"user_id", userID,
				"roles", userRoles,
				"path", c.Request.URL.Path)
			c.JSON(http.StatusForbidden, gin.H{
				"status": "error",
				"error":  "Global admin access required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// TenantAdminOnly restricts access to tenant admins for tenant-scoped operations
func (m *TenantIsolationMiddleware) TenantAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		tenantID := c.GetString("tenant_id")
		userRoles := c.GetStringSlice("user_roles")

		// Check if user is global admin (can access all tenants)
		hasGlobalAdmin := false
		for _, role := range userRoles {
			if role == m.config.GlobalAdminRole {
				hasGlobalAdmin = true
				break
			}
		}

		if hasGlobalAdmin {
			c.Next()
			return
		}

		// Check if user is tenant admin for this specific tenant
		if err := m.validateTenantAdminAccess(c, userID, tenantID); err != nil {
			m.logger.Warn("Tenant admin access denied",
				"user_id", userID,
				"tenant_id", tenantID,
				"path", c.Request.URL.Path,
				"error", err)
			c.JSON(http.StatusForbidden, gin.H{
				"status": "error",
				"error":  "Tenant admin access required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractTenantID extracts tenant ID from various sources with priority order
func (m *TenantIsolationMiddleware) extractTenantID(c *gin.Context) (string, string) {
	// 1. URL parameter (highest priority for tenant-scoped endpoints)
	if tenantID := c.Param(m.config.TenantParamName); tenantID != "" {
		return tenantID, "url_param"
	}

	// 2. Header override (admin only)
	if m.config.AllowTenantOverride {
		if tenantID := c.GetHeader(m.config.TenantHeaderName); tenantID != "" {
			// Validate that user is admin before allowing header override
			userRoles := c.GetStringSlice("user_roles")
			for _, role := range userRoles {
				if role == m.config.GlobalAdminRole {
					return tenantID, "header_override"
				}
			}
		}
	}

	// 3. Session/JWT tenant (from authentication)
	if tenantID := c.GetString("tenant_id"); tenantID != "" && tenantID != "unknown" {
		return tenantID, "session"
	}

	// 4. Extract from URL path pattern (for tenant-scoped routes)
	if tenantID := m.extractTenantFromPath(c.Request.URL.Path); tenantID != "" {
		return tenantID, "url_path"
	}

	return "", ""
}

// resolveTenantID resolves tenant identifier to canonical tenant ID
func (m *TenantIsolationMiddleware) resolveTenantID(ctx context.Context, tenantIdentifier string) (string, error) {
	return m.rbacService.ResolveTenantID(ctx, tenantIdentifier)
}

// extractTenantFromPath extracts tenant ID from URL path patterns
func (m *TenantIsolationMiddleware) extractTenantFromPath(path string) string {
	// Pattern: /api/v1/tenants/{tenantId}/...
	tenantRegex := regexp.MustCompile(`/api/v1/tenants/([^/]+)/`)
	matches := tenantRegex.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}

	// Pattern: /api/v1/{tenantId}/... (alternative pattern)
	tenantRegex2 := regexp.MustCompile(`/api/v1/([^/]+)/`)
	matches2 := tenantRegex2.FindStringSubmatch(path)
	if len(matches2) > 1 {
		tenantID := matches2[1]
		// Exclude common non-tenant prefixes
		excludedPrefixes := []string{"auth", "health", "metrics", "openapi", "swagger"}
		for _, prefix := range excludedPrefixes {
			if tenantID == prefix {
				return ""
			}
		}
		return tenantID
	}

	return ""
}

// validateTenantAccess checks if user has access to the specified tenant
func (m *TenantIsolationMiddleware) validateTenantAccess(c *gin.Context, userID, tenantID string) error {
	// Check if user has global admin role
	userRoles := c.GetStringSlice("user_roles")
	for _, role := range userRoles {
		if role == m.config.GlobalAdminRole {
			return nil // Global admins can access all tenants
		}
	}

	// Check if user is associated with this tenant
	tenantUser, err := m.rbacService.GetTenantUser(c.Request.Context(), tenantID, userID)
	if err != nil {
		return fmt.Errorf("failed to get tenant-user association: %w", err)
	}

	if tenantUser == nil {
		return fmt.Errorf("user not associated with tenant")
	}

	// Check if user status allows access
	if tenantUser.Status != "active" {
		return fmt.Errorf("user status '%s' does not allow access", tenantUser.Status)
	}

	return nil
}

// validateTenantAdminAccess checks if user is a tenant admin
func (m *TenantIsolationMiddleware) validateTenantAdminAccess(c *gin.Context, userID, tenantID string) error {
	// Get tenant-user association
	tenantUser, err := m.rbacService.GetTenantUser(c.Request.Context(), tenantID, userID)
	if err != nil {
		return fmt.Errorf("failed to get tenant-user association: %w", err)
	}

	if tenantUser == nil {
		return fmt.Errorf("user not associated with tenant")
	}

	// Check if user has tenant admin role
	if tenantUser.TenantRole != "tenant_admin" {
		return fmt.Errorf("user does not have tenant admin role")
	}

	// Check if user status allows access
	if tenantUser.Status != "active" {
		return fmt.Errorf("user status '%s' does not allow admin access", tenantUser.Status)
	}

	return nil
}

// isPublicEndpoint checks if endpoint should skip tenant isolation
func (m *TenantIsolationMiddleware) isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/",
		"/health",
		"/ready",
		"/metrics",
		"/api/openapi.json",
		"/api/openapi.yaml",
		"/swagger/",
		"/api/v1/auth/",
	}

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}

// TenantScopedRoute wraps a handler to ensure tenant context
func (m *TenantIsolationMiddleware) TenantScopedRoute(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ensure tenant context is set
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Tenant context required for this operation",
			})
			c.Abort()
			return
		}

		// Add tenant ID to request context for database operations
		ctx := c.Request.Context()
		// Note: In a real implementation, you'd add tenant ID to context
		// ctx = context.WithValue(ctx, "tenant_id", tenantID)
		c.Request = c.Request.WithContext(ctx)

		handler(c)
	}
}
