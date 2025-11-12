package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// generateCorrelationID generates a unique correlation ID for audit logging
func generateCorrelationID() string {
	return fmt.Sprintf("tenant-%d", time.Now().UnixNano())
}

type TenantHandler struct {
	rbacService        *rbac.RBACService
	logger             logger.Logger
	featureFlagService *services.RuntimeFeatureFlagService
}

func NewTenantHandler(rbacService *rbac.RBACService, l logger.Logger) *TenantHandler {
	return &TenantHandler{
		rbacService:        rbacService,
		logger:             l,
		featureFlagService: services.NewRuntimeFeatureFlagService(nil, l), // TODO: inject cache
	}
}

// checkFeatureEnabled checks if the RBAC feature is enabled for the current tenant
func (h *TenantHandler) checkFeatureEnabled(c *gin.Context) bool {
	tenantID := c.GetString("tenant_id")
	flags, err := h.featureFlagService.GetFeatureFlags(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to check feature flags", "tenantID", tenantID, "error", err)
		return false
	}
	return flags.RBACEnabled
}

// GET /api/v1/tenants
// List tenants (global admin only)
func (h *TenantHandler) ListTenants(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
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

	// Parse query parameters
	var filters rbac.TenantFilters
	if name := c.Query("name"); name != "" {
		filters.Name = &name
	}
	if status := c.Query("status"); status != "" {
		filters.Status = &status
	}
	if adminEmail := c.Query("adminEmail"); adminEmail != "" {
		filters.AdminEmail = &adminEmail
	}
	if limit := c.Query("limit"); limit != "" {
		// Parse limit (not implemented in this basic version)
	}
	if offset := c.Query("offset"); offset != "" {
		// Parse offset (not implemented in this basic version)
	}

	// Get tenants using service
	tenants, err := h.rbacService.ListTenants(c.Request.Context(), userID, filters)
	if err != nil {
		h.logger.Error("Failed to list tenants", "userID", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve tenants",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenants": tenants,
			"total":   len(tenants),
		},
	})
}

// POST /api/v1/tenants
// Create a new tenant (global admin only)
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
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
		Name        string                    `json:"name" binding:"required"`
		DisplayName string                    `json:"displayName"`
		Description string                    `json:"description"`
		AdminEmail  string                    `json:"adminEmail" binding:"required,email"`
		AdminName   string                    `json:"adminName"`
		Deployments []models.TenantDeployment `json:"deployments"`
		Quotas      models.TenantQuotas       `json:"quotas"`
		Features    []string                  `json:"features"`
		Tags        []string                  `json:"tags"`
		Metadata    map[string]string         `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Create tenant using service
	tenant := &models.Tenant{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		AdminEmail:  req.AdminEmail,
		AdminName:   req.AdminName,
		Deployments: req.Deployments,
		Quotas:      req.Quotas,
		Features:    req.Features,
		Tags:        req.Tags,
		Metadata:    req.Metadata,
		Status:      "active", // Default status
	}

	err := h.rbacService.CreateTenant(c.Request.Context(), userID, tenant)
	if err != nil {
		h.logger.Error("Failed to create tenant", "userID", userID, "tenantName", req.Name, "error", err)

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
			"error":  "Failed to create tenant",
		})
		return
	}

	h.logger.Info("Tenant created", "userID", userID, "tenantName", req.Name, "tenantID", tenant.ID)

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"tenant":    tenant,
			"createdAt": tenant.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}

// GET /api/v1/tenants/:tenantId
// Get tenant details
func (h *TenantHandler) GetTenant(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant ID is required",
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

	// Get tenant using service
	tenant, err := h.rbacService.GetTenant(c.Request.Context(), userID, tenantID)
	if err != nil {
		h.logger.Error("Failed to get tenant", "userID", userID, "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve tenant",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   gin.H{"tenant": tenant},
	})
}

// PUT /api/v1/tenants/:tenantId
// Update tenant
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant ID is required",
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
		DisplayName string                    `json:"displayName"`
		Description string                    `json:"description"`
		AdminEmail  string                    `json:"adminEmail"`
		AdminName   string                    `json:"adminName"`
		Deployments []models.TenantDeployment `json:"deployments"`
		Quotas      models.TenantQuotas       `json:"quotas"`
		Features    []string                  `json:"features"`
		Tags        []string                  `json:"tags"`
		Metadata    map[string]string         `json:"metadata"`
		Status      string                    `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Get existing tenant first
	existingTenant, err := h.rbacService.GetTenant(c.Request.Context(), userID, tenantID)
	if err != nil {
		h.logger.Error("Failed to get existing tenant", "userID", userID, "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve tenant",
		})
		return
	}

	// Update tenant fields
	if req.DisplayName != "" {
		existingTenant.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		existingTenant.Description = req.Description
	}
	if req.AdminEmail != "" {
		existingTenant.AdminEmail = req.AdminEmail
	}
	if req.AdminName != "" {
		existingTenant.AdminName = req.AdminName
	}
	if req.Deployments != nil {
		existingTenant.Deployments = req.Deployments
	}
	if req.Quotas.MaxUsers != 0 || req.Quotas.MaxDashboards != 0 || req.Quotas.MaxKPIs != 0 {
		existingTenant.Quotas = req.Quotas
	}
	if req.Features != nil {
		existingTenant.Features = req.Features
	}
	if req.Tags != nil {
		existingTenant.Tags = req.Tags
	}
	if req.Metadata != nil {
		existingTenant.Metadata = req.Metadata
	}
	if req.Status != "" {
		existingTenant.Status = req.Status
	}

	// Update tenant using service
	err = h.rbacService.UpdateTenant(c.Request.Context(), userID, existingTenant)
	if err != nil {
		h.logger.Error("Failed to update tenant", "userID", userID, "tenantID", tenantID, "error", err)

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
			"error":  "Failed to update tenant",
		})
		return
	}

	h.logger.Info("Tenant updated", "userID", userID, "tenantID", tenantID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenant":    existingTenant,
			"updatedAt": existingTenant.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}

// DELETE /api/v1/tenants/:tenantId
// Delete tenant (global admin only)
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant ID is required",
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

	// Delete tenant using service
	err := h.rbacService.DeleteTenant(c.Request.Context(), userID, tenantID)
	if err != nil {
		h.logger.Error("Failed to delete tenant", "userID", userID, "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to delete tenant",
		})
		return
	}

	h.logger.Info("Tenant deleted", "userID", userID, "tenantID", tenantID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tenant deleted successfully",
	})
}

// POST /api/v1/tenants/:tenantId/users
// Add a user to a tenant
func (h *TenantHandler) CreateTenantUser(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant ID is required",
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
		UserID                string            `json:"userId" binding:"required"`
		TenantRole            string            `json:"tenantRole" binding:"required"`
		Status                string            `json:"status"`
		InvitedBy             string            `json:"invitedBy"`
		AdditionalPermissions []string          `json:"additionalPermissions"`
		Metadata              map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Create tenant-user association using service
	tenantUser := &models.TenantUser{
		TenantID:              tenantID,
		UserID:                req.UserID,
		TenantRole:            req.TenantRole,
		Status:                req.Status,
		InvitedBy:             req.InvitedBy,
		AdditionalPermissions: req.AdditionalPermissions,
		Metadata:              req.Metadata,
		CreatedBy:             userID,
		UpdatedBy:             userID,
	}

	createdTenantUser, err := h.rbacService.CreateTenantUser(c.Request.Context(), tenantUser, generateCorrelationID())
	if err != nil {
		h.logger.Error("Failed to create tenant-user association", "userID", userID, "tenantID", tenantID, "targetUserID", req.UserID, "error", err)

		// Handle specific validation errors
		if strings.Contains(err.Error(), "validation error") || strings.Contains(err.Error(), "already associated") {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to create tenant-user association",
		})
		return
	}

	h.logger.Info("Tenant-user association created", "userID", userID, "tenantID", tenantID, "targetUserID", req.UserID)

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantUser": createdTenantUser,
			"createdAt":  createdTenantUser.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}

// GET /api/v1/tenants/:tenantId/users
// List users in a tenant
func (h *TenantHandler) ListTenantUsers(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant ID is required",
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

	// Parse query parameters
	var filters *rbac.TenantUserFilters
	if status := c.Query("status"); status != "" {
		if filters == nil {
			filters = &rbac.TenantUserFilters{}
		}
		filters.Status = &status
	}
	if tenantRole := c.Query("tenantRole"); tenantRole != "" {
		if filters == nil {
			filters = &rbac.TenantUserFilters{}
		}
		filters.TenantRole = &tenantRole
	}
	if userIDParam := c.Query("userId"); userIDParam != "" {
		if filters == nil {
			filters = &rbac.TenantUserFilters{}
		}
		filters.UserID = &userIDParam
	}

	// Get tenant users using service
	tenantUsers, err := h.rbacService.ListTenantUsers(c.Request.Context(), tenantID, filters)
	if err != nil {
		h.logger.Error("Failed to list tenant users", "userID", userID, "tenantID", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve tenant users",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantUsers": tenantUsers,
			"total":       len(tenantUsers),
		},
	})
}

// GET /api/v1/tenants/:tenantId/users/:userId
// Get tenant-user association details
func (h *TenantHandler) GetTenantUser(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant ID is required",
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

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "User context required",
		})
		return
	}

	// Get tenant-user association using service
	tenantUser, err := h.rbacService.GetTenantUser(c.Request.Context(), tenantID, targetUserID)
	if err != nil {
		h.logger.Error("Failed to get tenant-user association", "userID", userID, "tenantID", tenantID, "targetUserID", targetUserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve tenant-user association",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   gin.H{"tenantUser": tenantUser},
	})
}

// PUT /api/v1/tenants/:tenantId/users/:userId
// Update tenant-user association
func (h *TenantHandler) UpdateTenantUser(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant ID is required",
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

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "User context required",
		})
		return
	}

	var req struct {
		TenantRole            string            `json:"tenantRole"`
		Status                string            `json:"status"`
		InvitedBy             string            `json:"invitedBy"`
		AdditionalPermissions []string          `json:"additionalPermissions"`
		Metadata              map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Create updates object
	updates := &models.TenantUser{
		TenantID:              tenantID,
		UserID:                targetUserID,
		TenantRole:            req.TenantRole,
		Status:                req.Status,
		InvitedBy:             req.InvitedBy,
		AdditionalPermissions: req.AdditionalPermissions,
		Metadata:              req.Metadata,
		UpdatedBy:             userID,
	}

	// Update tenant-user association using service
	updatedTenantUser, err := h.rbacService.UpdateTenantUser(c.Request.Context(), tenantID, targetUserID, updates, generateCorrelationID())
	if err != nil {
		h.logger.Error("Failed to update tenant-user association", "userID", userID, "tenantID", tenantID, "targetUserID", targetUserID, "error", err)

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
			"error":  "Failed to update tenant-user association",
		})
		return
	}

	h.logger.Info("Tenant-user association updated", "userID", userID, "tenantID", tenantID, "targetUserID", targetUserID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"tenantUser": updatedTenantUser,
			"updatedAt":  updatedTenantUser.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}

// DELETE /api/v1/tenants/:tenantId/users/:userId
// Remove user from tenant
func (h *TenantHandler) DeleteTenantUser(c *gin.Context) {
	// Check if RBAC feature is enabled
	if !h.checkFeatureEnabled(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "RBAC feature is disabled",
		})
		return
	}

	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Tenant ID is required",
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

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "User context required",
		})
		return
	}

	// Delete tenant-user association using service
	err := h.rbacService.DeleteTenantUser(c.Request.Context(), tenantID, targetUserID, generateCorrelationID())
	if err != nil {
		h.logger.Error("Failed to delete tenant-user association", "userID", userID, "tenantID", tenantID, "targetUserID", targetUserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to delete tenant-user association",
		})
		return
	}

	h.logger.Info("Tenant-user association deleted", "userID", userID, "tenantID", tenantID, "targetUserID", targetUserID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tenant-user association deleted successfully",
	})
}
