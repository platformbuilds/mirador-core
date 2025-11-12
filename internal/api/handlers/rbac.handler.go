package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type RBACHandler struct {
	rbacService        *rbac.RBACService
	cache              cache.ValkeyCluster
	logger             logger.Logger
	featureFlagService *services.RuntimeFeatureFlagService
	policyCache        *middleware.PolicyCache
}

func NewRBACHandler(rbacService *rbac.RBACService, c cache.ValkeyCluster, l logger.Logger) *RBACHandler {
	policyCache := middleware.NewPolicyCache(c, l, 15*time.Minute)
	return &RBACHandler{
		rbacService:        rbacService,
		cache:              c,
		logger:             l,
		featureFlagService: services.NewRuntimeFeatureFlagService(c, l),
		policyCache:        policyCache,
	}
}

// checkFeatureEnabled checks if the RBAC feature is enabled for the current tenant
func (h *RBACHandler) checkFeatureEnabled(c *gin.Context) bool {
	tenantID := c.GetString("tenant_id")
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to check feature flags", "tenantID", tenantID, "error", err)
		return false
	}
	return flags.RBACEnabled
}

// checkRBACMode checks the RBAC operational mode for the current tenant
func (h *RBACHandler) checkRBACMode(c *gin.Context) (services.RBACMode, error) {
	tenantID := c.GetString("tenant_id")
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to check RBAC mode", "tenantID", tenantID, "error", err)
		return services.RBACModeDisabled, err
	}
	return flags.RBACMode, nil
}

// shouldUseLegacyFallback checks if legacy fallback should be used
func (h *RBACHandler) shouldUseLegacyFallback(c *gin.Context) bool {
	tenantID := c.GetString("tenant_id")
	useFallback, err := h.featureFlagService.ShouldUseRBACLegacyFallback(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to check legacy fallback", "tenantID", tenantID, "error", err)
		return true // Default to fallback on error
	}
	return useFallback
}

// GET /api/v1/rbac/roles
// Get all roles for the current tenant
func (h *RBACHandler) GetRoles(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant context required",
		})
		return
	}

	// Get roles using service
	roles, err := h.rbacService.ListRoles(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to list roles", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve roles",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   gin.H{"roles": roles, "total": len(roles)},
	})
}

// POST /api/v1/rbac/roles
// Create a new role for the current tenant
func (h *RBACHandler) CreateRole(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	rbacMode, err := h.checkRBACMode(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to check RBAC mode",
		})
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant context required",
		})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "User context required",
		})
		return
	}

	var req struct {
		Name        string   `json:"name" binding:"required"`
		Permissions []string `json:"permissions"`
		Description string   `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Create role using service
	role := &models.Role{
		Name:        req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
		IsSystem:    false,
	}

	err = h.rbacService.CreateRole(c.Request.Context(), tenantID, userID, role)
	if err != nil {
		h.logger.Error("Failed to create role", "tenantID", tenantID, "roleName", req.Name, "error", err)

		// In audit-only mode, log the error but don't fail the operation
		if rbacMode == services.RBACModeAuditOnly {
			h.logger.Info("RBAC audit-only mode: role creation failed but operation allowed", "tenantID", tenantID, "roleName", req.Name)
			// Return success with the role that would have been created
			c.JSON(http.StatusCreated, gin.H{
				"status":  "success",
				"data":    gin.H{"role": role, "createdAt": role.CreatedAt.Format(time.RFC3339)},
				"warning": "RBAC validation failed but operation allowed in audit-only mode",
			})
			return
		}

		// Handle specific validation errors
		if strings.Contains(err.Error(), "validation error") {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		// Check if legacy fallback should be used
		if h.shouldUseLegacyFallback(c) {
			h.logger.Warn("Using legacy fallback for role creation", "tenantID", tenantID, "roleName", req.Name, "error", err)
			// Implement legacy cache-based creation here
			c.JSON(http.StatusCreated, gin.H{
				"status":  "success",
				"data":    gin.H{"role": role, "createdAt": role.CreatedAt.Format(time.RFC3339)},
				"warning": "Used legacy fallback due to service error",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to create role",
		})
		return
	}

	// Invalidate policy cache for this tenant (role changes affect all users)
	h.policyCache.InvalidateTenantPolicies(c.Request.Context(), tenantID)

	h.logger.Info("Role created", "tenantID", tenantID, "roleName", req.Name)

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"role":      role,
			"createdAt": role.CreatedAt.Format(time.RFC3339),
		},
	})
}

// PUT /api/v1/rbac/users/:userId/roles
// Assign roles to a user within the current tenant
func (h *RBACHandler) AssignUserRoles(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant context required",
		})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "User context required",
		})
		return
	}

	targetUserID := c.Param("userId")
	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "User ID is required",
		})
		return
	}

	var req struct {
		Roles []string `json:"roles" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Assign roles using service
	if err := h.rbacService.AssignUserRoles(c.Request.Context(), tenantID, userID, targetUserID, req.Roles); err != nil {
		h.logger.Error("Failed to assign user roles", "tenantID", tenantID, "userID", targetUserID, "error", err)

		// Handle specific validation errors
		if strings.Contains(err.Error(), "validation error") {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to assign roles",
		})
		return
	}

	// Invalidate policy cache for this user (role changes affect their permissions)
	h.policyCache.InvalidateUserPolicies(c.Request.Context(), tenantID, targetUserID)

	h.logger.Info("User roles assigned", "tenantID", tenantID, "userID", targetUserID, "roles", req.Roles)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"userId": targetUserID,
			"roles":  req.Roles,
		},
	})
}

// GET /api/v1/rbac/users/:userId/roles
// Get roles assigned to a user
func (h *RBACHandler) GetUserRoles(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant context required",
		})
		return
	}

	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "User ID is required",
		})
		return
	}

	// Get user roles using service
	roles, err := h.rbacService.GetUserRoles(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("Failed to get user roles", "tenantID", tenantID, "userID", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve user roles",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"userId": userID,
			"roles":  roles,
		},
	})
}

// GET /api/v1/rbac/permissions
// Get all permissions for the current tenant
func (h *RBACHandler) GetPermissions(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant context required",
		})
		return
	}

	// Return standard permissions
	permissions := []gin.H{
		{"resource": "dashboard", "action": "create", "scope": "tenant", "description": "Create dashboards"},
		{"resource": "dashboard", "action": "read", "scope": "tenant", "description": "Read dashboards"},
		{"resource": "dashboard", "action": "update", "scope": "tenant", "description": "Update dashboards"},
		{"resource": "dashboard", "action": "delete", "scope": "tenant", "description": "Delete dashboards"},
		{"resource": "kpi_definition", "action": "create", "scope": "tenant", "description": "Create KPI definitions"},
		{"resource": "kpi_definition", "action": "read", "scope": "tenant", "description": "Read KPI definitions"},
		{"resource": "kpi_definition", "action": "update", "scope": "tenant", "description": "Update KPI definitions"},
		{"resource": "kpi_definition", "action": "delete", "scope": "tenant", "description": "Delete KPI definitions"},
		{"resource": "layout", "action": "create", "scope": "tenant", "description": "Create layouts"},
		{"resource": "layout", "action": "read", "scope": "tenant", "description": "Read layouts"},
		{"resource": "layout", "action": "update", "scope": "tenant", "description": "Update layouts"},
		{"resource": "layout", "action": "delete", "scope": "tenant", "description": "Delete layouts"},
		{"resource": "user_prefs", "action": "read", "scope": "user", "description": "Read user preferences"},
		{"resource": "user_prefs", "action": "update", "scope": "user", "description": "Update user preferences"},
		{"resource": "admin", "action": "admin", "scope": "global", "description": "Global admin access"},
		{"resource": "rbac", "action": "admin", "scope": "tenant", "description": "RBAC administration"},
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   gin.H{"permissions": permissions, "total": len(permissions)},
	})
}

// GET /api/v1/rbac/cache/stats
// Get policy cache statistics for monitoring
func (h *RBACHandler) GetCacheStats(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant context required",
		})
		return
	}

	// Get cache statistics
	stats, err := h.policyCache.GetCacheStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get cache stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve cache statistics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   stats,
	})
}

// POST /api/v1/rbac/cache/warm
// Warm up policy cache with common permissions
func (h *RBACHandler) WarmPolicyCache(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant context required",
		})
		return
	}

	// This would typically be called by an admin or during startup
	// For now, we'll create a simple enforcer to warm the cache
	enforcer := middleware.NewRBACEnforcer(h.rbacService, h.cache, h.logger)
	err := enforcer.WarmCommonPolicies(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to warm policy cache", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to warm policy cache",
		})
		return
	}

	h.logger.Info("Policy cache warmed", "tenantID", tenantID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Policy cache warmed successfully",
	})
}
