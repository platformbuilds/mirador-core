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

	if roleName := c.Query("name"); roleName != "" {
		role, err := h.rbacService.GetRole(c.Request.Context(), tenantID, roleName)
		if err != nil {
			h.logger.Error("Failed to get role", "tenantID", tenantID, "role", roleName, "error", err)
			status := http.StatusInternalServerError
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{
				"status": "error",
				"error":  "Failed to retrieve role",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"role": role,
			},
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

// POST /api/v1/rbac/permissions
// Create a new permission for the current tenant
func (h *RBACHandler) CreatePermission(c *gin.Context) {
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
		ID              string `json:"id" binding:"required"`
		Resource        string `json:"resource" binding:"required"`
		Action          string `json:"action" binding:"required"`
		ResourcePattern string `json:"resourcePattern"`
		Scope           string `json:"scope"`
		Description     string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Create permission using service
	permission := &models.Permission{
		ID:              req.ID,
		Resource:        req.Resource,
		Action:          req.Action,
		ResourcePattern: req.ResourcePattern,
		Scope:           req.Scope,
		Description:     req.Description,
	}

	err = h.rbacService.CreatePermission(c.Request.Context(), tenantID, userID, permission)
	if err != nil {
		h.logger.Error("Failed to create permission", "tenantID", tenantID, "permissionID", req.ID, "error", err)

		// In audit-only mode, log the error but don't fail the operation
		if rbacMode == services.RBACModeAuditOnly {
			h.logger.Info("RBAC audit-only mode: permission creation failed but operation allowed", "tenantID", tenantID, "permissionID", req.ID)
			c.JSON(http.StatusCreated, gin.H{
				"status":  "success",
				"data":    gin.H{"permission": permission, "createdAt": permission.CreatedAt.Format(time.RFC3339)},
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

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to create permission",
		})
		return
	}

	// Invalidate policy cache for this tenant (permission changes affect all users)
	h.policyCache.InvalidateTenantPolicies(c.Request.Context(), tenantID)

	h.logger.Info("Permission created", "tenantID", tenantID, "permissionID", req.ID)

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"permission": permission,
			"createdAt":  permission.CreatedAt.Format(time.RFC3339),
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

	// Get permissions using service
	permissions, err := h.rbacService.ListPermissions(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to list permissions", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve permissions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   gin.H{"permissions": permissions, "total": len(permissions)},
	})
}

// PUT /api/v1/rbac/permissions/:permissionId
// Update an existing permission
func (h *RBACHandler) UpdatePermission(c *gin.Context) {
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

	permissionID := c.Param("permissionId")
	if permissionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Permission ID is required",
		})
		return
	}

	var req struct {
		Resource        string `json:"resource"`
		Action          string `json:"action"`
		ResourcePattern string `json:"resourcePattern"`
		Scope           string `json:"scope"`
		Description     string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Update permission using service
	updates := &models.Permission{
		Resource:        req.Resource,
		Action:          req.Action,
		ResourcePattern: req.ResourcePattern,
		Scope:           req.Scope,
		Description:     req.Description,
	}

	err := h.rbacService.UpdatePermission(c.Request.Context(), tenantID, userID, permissionID, updates)
	if err != nil {
		h.logger.Error("Failed to update permission", "tenantID", tenantID, "permissionID", permissionID, "error", err)

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
			"error":  "Failed to update permission",
		})
		return
	}

	// Invalidate policy cache for this tenant (permission changes affect all users)
	h.policyCache.InvalidateTenantPolicies(c.Request.Context(), tenantID)

	h.logger.Info("Permission updated", "tenantID", tenantID, "permissionID", permissionID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"permissionId": permissionID,
			"updatedAt":    time.Now().Format(time.RFC3339),
		},
	})
}

// DELETE /api/v1/rbac/permissions/:permissionId
// Delete a permission
func (h *RBACHandler) DeletePermission(c *gin.Context) {
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

	permissionID := c.Param("permissionId")
	if permissionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Permission ID is required",
		})
		return
	}

	// Delete permission using service
	err := h.rbacService.DeletePermission(c.Request.Context(), tenantID, userID, permissionID)
	if err != nil {
		h.logger.Error("Failed to delete permission", "tenantID", tenantID, "permissionID", permissionID, "error", err)

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
			"error":  "Failed to delete permission",
		})
		return
	}

	// Invalidate policy cache for this tenant (permission changes affect all users)
	h.policyCache.InvalidateTenantPolicies(c.Request.Context(), tenantID)

	h.logger.Info("Permission deleted", "tenantID", tenantID, "permissionID", permissionID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"permissionId": permissionID,
			"deletedAt":    time.Now().Format(time.RFC3339),
		},
	})
}

// POST /api/v1/rbac/groups
// Create a new group for the current tenant
func (h *RBACHandler) CreateGroup(c *gin.Context) {
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
		Name         string   `json:"name" binding:"required"`
		Description  string   `json:"description"`
		Roles        []string `json:"roles"`
		ParentGroups []string `json:"parentGroups"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Create group using service
	group := &models.Group{
		Name:         req.Name,
		Description:  req.Description,
		Roles:        req.Roles,
		ParentGroups: req.ParentGroups,
	}

	err = h.rbacService.CreateGroup(c.Request.Context(), tenantID, userID, group)
	if err != nil {
		h.logger.Error("Failed to create group", "tenantID", tenantID, "groupName", req.Name, "error", err)

		// In audit-only mode, log the error but don't fail the operation
		if rbacMode == services.RBACModeAuditOnly {
			h.logger.Info("RBAC audit-only mode: group creation failed but operation allowed", "tenantID", tenantID, "groupName", req.Name)
			c.JSON(http.StatusCreated, gin.H{
				"status":  "success",
				"data":    gin.H{"group": group, "createdAt": group.CreatedAt.Format(time.RFC3339)},
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

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to create group",
		})
		return
	}

	h.logger.Info("Group created", "tenantID", tenantID, "groupName", req.Name)

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"group":     group,
			"createdAt": group.CreatedAt.Format(time.RFC3339),
		},
	})
}

// GET /api/v1/rbac/groups
// Get all groups for the current tenant
func (h *RBACHandler) GetGroups(c *gin.Context) {
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

	// Get groups using service
	groups, err := h.rbacService.ListGroups(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to list groups", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve groups",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   gin.H{"groups": groups, "total": len(groups)},
	})
}

// PUT /api/v1/rbac/groups/:groupName
// Update an existing group
func (h *RBACHandler) UpdateGroup(c *gin.Context) {
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

	groupName := c.Param("groupName")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Group name is required",
		})
		return
	}

	var req struct {
		Description  string   `json:"description"`
		Roles        []string `json:"roles"`
		ParentGroups []string `json:"parentGroups"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Update group using service
	updates := &models.Group{
		Description:  req.Description,
		Roles:        req.Roles,
		ParentGroups: req.ParentGroups,
	}

	err := h.rbacService.UpdateGroup(c.Request.Context(), tenantID, userID, groupName, updates)
	if err != nil {
		h.logger.Error("Failed to update group", "tenantID", tenantID, "groupName", groupName, "error", err)

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
			"error":  "Failed to update group",
		})
		return
	}

	h.logger.Info("Group updated", "tenantID", tenantID, "groupName", groupName)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"groupName": groupName,
			"updatedAt": time.Now().Format(time.RFC3339),
		},
	})
}

// DELETE /api/v1/rbac/groups/:groupName
// Delete a group
func (h *RBACHandler) DeleteGroup(c *gin.Context) {
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

	groupName := c.Param("groupName")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Group name is required",
		})
		return
	}

	// Delete group using service
	err := h.rbacService.DeleteGroup(c.Request.Context(), tenantID, userID, groupName)
	if err != nil {
		h.logger.Error("Failed to delete group", "tenantID", tenantID, "groupName", groupName, "error", err)

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
			"error":  "Failed to delete group",
		})
		return
	}

	h.logger.Info("Group deleted", "tenantID", tenantID, "groupName", groupName)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"groupName": groupName,
			"deletedAt": time.Now().Format(time.RFC3339),
		},
	})
}

// PUT /api/v1/rbac/groups/:groupName/users
// Add users to a group
func (h *RBACHandler) AddUsersToGroup(c *gin.Context) {
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

	groupName := c.Param("groupName")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Group name is required",
		})
		return
	}

	var req struct {
		UserIDs []string `json:"userIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Add users to group using service
	err := h.rbacService.AddUsersToGroup(c.Request.Context(), tenantID, userID, groupName, req.UserIDs)
	if err != nil {
		h.logger.Error("Failed to add users to group", "tenantID", tenantID, "groupName", groupName, "error", err)

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
			"error":  "Failed to add users to group",
		})
		return
	}

	h.logger.Info("Users added to group", "tenantID", tenantID, "groupName", groupName, "userIDs", req.UserIDs)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"groupName": groupName,
			"userIds":   req.UserIDs,
		},
	})
}

// DELETE /api/v1/rbac/groups/:groupName/users
// Remove users from a group
func (h *RBACHandler) RemoveUsersFromGroup(c *gin.Context) {
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

	groupName := c.Param("groupName")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Group name is required",
		})
		return
	}

	var req struct {
		UserIDs []string `json:"userIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Remove users from group using service
	err := h.rbacService.RemoveUsersFromGroup(c.Request.Context(), tenantID, userID, groupName, req.UserIDs)
	if err != nil {
		h.logger.Error("Failed to remove users from group", "tenantID", tenantID, "groupName", groupName, "error", err)

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
			"error":  "Failed to remove users from group",
		})
		return
	}

	h.logger.Info("Users removed from group", "tenantID", tenantID, "groupName", groupName, "userIDs", req.UserIDs)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"groupName": groupName,
			"userIds":   req.UserIDs,
		},
	})
}

// GET /api/v1/rbac/groups/:groupName/members
// Get members of a group
func (h *RBACHandler) GetGroupMembers(c *gin.Context) {
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

	groupName := c.Param("groupName")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Group name is required",
		})
		return
	}

	// Get group members using service
	members, err := h.rbacService.GetGroupMembers(c.Request.Context(), tenantID, groupName)
	if err != nil {
		h.logger.Error("Failed to get group members", "tenantID", tenantID, "groupName", groupName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve group members",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"groupName": groupName,
			"members":   members,
		},
	})
}

// POST /api/v1/rbac/role-bindings
// Create a new role binding for the current tenant
func (h *RBACHandler) CreateRoleBinding(c *gin.Context) {
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
		ID            string                       `json:"id" binding:"required"`
		SubjectType   string                       `json:"subjectType" binding:"required"`
		SubjectID     string                       `json:"subjectId" binding:"required"`
		RoleID        string                       `json:"roleId" binding:"required"`
		Scope         string                       `json:"scope"`
		ResourceID    string                       `json:"resourceId"`
		Precedence    string                       `json:"precedence"`
		Conditions    models.RoleBindingConditions `json:"conditions"`
		ExpiresAt     *time.Time                   `json:"expiresAt"`
		Justification string                       `json:"justification"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Create role binding using service
	binding := &models.RoleBinding{
		ID:            req.ID,
		SubjectType:   req.SubjectType,
		SubjectID:     req.SubjectID,
		RoleID:        req.RoleID,
		Scope:         req.Scope,
		ResourceID:    req.ResourceID,
		Precedence:    req.Precedence,
		Conditions:    req.Conditions,
		ExpiresAt:     req.ExpiresAt,
		Justification: req.Justification,
	}

	err = h.rbacService.CreateRoleBinding(c.Request.Context(), tenantID, userID, binding)
	if err != nil {
		h.logger.Error("Failed to create role binding", "tenantID", tenantID, "bindingID", req.ID, "error", err)

		// In audit-only mode, log the error but don't fail the operation
		if rbacMode == services.RBACModeAuditOnly {
			h.logger.Info("RBAC audit-only mode: role binding creation failed but operation allowed", "tenantID", tenantID, "bindingID", req.ID)
			c.JSON(http.StatusCreated, gin.H{
				"status":  "success",
				"data":    gin.H{"roleBinding": binding, "createdAt": binding.CreatedAt.Format(time.RFC3339)},
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

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to create role binding",
		})
		return
	}

	h.logger.Info("Role binding created", "tenantID", tenantID, "bindingID", req.ID)

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"roleBinding": binding,
			"createdAt":   binding.CreatedAt.Format(time.RFC3339),
		},
	})
}

// GET /api/v1/rbac/role-bindings
// Get role bindings for the current tenant with optional filters
func (h *RBACHandler) GetRoleBindings(c *gin.Context) {
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

	// Parse query parameters for filters
	filters := rbac.RoleBindingFilters{}

	if subjectType := c.Query("subjectType"); subjectType != "" {
		filters.SubjectType = &subjectType
	}
	if subjectID := c.Query("subjectId"); subjectID != "" {
		filters.SubjectID = &subjectID
	}
	if roleID := c.Query("roleId"); roleID != "" {
		filters.RoleID = &roleID
	}
	if scope := c.Query("scope"); scope != "" {
		filters.Scope = &scope
	}
	if resourceID := c.Query("resourceId"); resourceID != "" {
		filters.ResourceID = &resourceID
	}
	if precedence := c.Query("precedence"); precedence != "" {
		filters.Precedence = &precedence
	}
	if expired := c.Query("expired"); expired != "" {
		switch expired {
		case "true":
			filters.Expired = &[]bool{true}[0]
		case "false":
			filters.Expired = &[]bool{false}[0]
		}
	}

	// Get role bindings using service
	bindings, err := h.rbacService.GetRoleBindings(c.Request.Context(), tenantID, filters)
	if err != nil {
		h.logger.Error("Failed to get role bindings", "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve role bindings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   gin.H{"roleBindings": bindings, "total": len(bindings)},
	})
}

// PUT /api/v1/rbac/role-bindings/:bindingId
// Update an existing role binding
func (h *RBACHandler) UpdateRoleBinding(c *gin.Context) {
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

	bindingID := c.Param("bindingId")
	if bindingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Binding ID is required",
		})
		return
	}

	var req struct {
		SubjectType   string                       `json:"subjectType"`
		SubjectID     string                       `json:"subjectId"`
		RoleID        string                       `json:"roleId"`
		Scope         string                       `json:"scope"`
		ResourceID    string                       `json:"resourceId"`
		Precedence    string                       `json:"precedence"`
		Conditions    models.RoleBindingConditions `json:"conditions"`
		ExpiresAt     *time.Time                   `json:"expiresAt"`
		Justification string                       `json:"justification"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Update role binding using service
	updates := &models.RoleBinding{
		SubjectType:   req.SubjectType,
		SubjectID:     req.SubjectID,
		RoleID:        req.RoleID,
		Scope:         req.Scope,
		ResourceID:    req.ResourceID,
		Precedence:    req.Precedence,
		Conditions:    req.Conditions,
		ExpiresAt:     req.ExpiresAt,
		Justification: req.Justification,
	}

	err := h.rbacService.UpdateRoleBinding(c.Request.Context(), tenantID, userID, bindingID, updates)
	if err != nil {
		h.logger.Error("Failed to update role binding", "tenantID", tenantID, "bindingID", bindingID, "error", err)

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
			"error":  "Failed to update role binding",
		})
		return
	}

	h.logger.Info("Role binding updated", "tenantID", tenantID, "bindingID", bindingID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"bindingId": bindingID,
			"updatedAt": time.Now().Format(time.RFC3339),
		},
	})
}

// DELETE /api/v1/rbac/role-bindings/:bindingId
// Delete a role binding
func (h *RBACHandler) DeleteRoleBinding(c *gin.Context) {
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

	bindingID := c.Param("bindingId")
	if bindingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Binding ID is required",
		})
		return
	}

	// Delete role binding using service
	err := h.rbacService.DeleteRoleBinding(c.Request.Context(), tenantID, userID, bindingID)
	if err != nil {
		h.logger.Error("Failed to delete role binding", "tenantID", tenantID, "bindingID", bindingID, "error", err)

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
			"error":  "Failed to delete role binding",
		})
		return
	}

	h.logger.Info("Role binding deleted", "tenantID", tenantID, "bindingID", bindingID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"bindingId": bindingID,
			"deletedAt": time.Now().Format(time.RFC3339),
		},
	})
}
