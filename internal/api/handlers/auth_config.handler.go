package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// AuthConfigHandler handles AuthConfig-related HTTP requests
type AuthConfigHandler struct {
	rbacRepo rbac.RBACRepository
	logger   logger.Logger
}

func NewAuthConfigHandler(rbacRepo rbac.RBACRepository, logger logger.Logger) *AuthConfigHandler {
	return &AuthConfigHandler{
		rbacRepo: rbacRepo,
		logger:   logger,
	}
}

// CreateAuthConfigRequest represents the request payload for creating AuthConfig
type CreateAuthConfigRequest struct {
	TenantID              string                    `json:"tenantId" binding:"required"`
	DefaultBackend        string                    `json:"defaultBackend"`
	EnabledBackends       []string                  `json:"enabledBackends"`
	BackendConfigs        models.AuthBackendConfigs `json:"backendConfigs"`
	PasswordPolicy        models.PasswordPolicy     `json:"passwordPolicy"`
	Require2FA            bool                      `json:"require2fa"`
	TOTPIssuer            string                    `json:"totpIssuer"`
	SessionTimeoutMinutes int                       `json:"sessionTimeoutMinutes"`
	MaxConcurrentSessions int                       `json:"maxConcurrentSessions"`
	AllowRememberMe       bool                      `json:"allowRememberMe"`
	RememberMeDays        int                       `json:"rememberMeDays"`
	Metadata              map[string]string         `json:"metadata"`
}

// UpdateAuthConfigRequest represents the request payload for updating AuthConfig
type UpdateAuthConfigRequest struct {
	DefaultBackend        *string                    `json:"defaultBackend,omitempty"`
	EnabledBackends       []string                   `json:"enabledBackends,omitempty"`
	BackendConfigs        *models.AuthBackendConfigs `json:"backendConfigs,omitempty"`
	PasswordPolicy        *models.PasswordPolicy     `json:"passwordPolicy,omitempty"`
	Require2FA            *bool                      `json:"require2fa,omitempty"`
	TOTPIssuer            *string                    `json:"totpIssuer,omitempty"`
	SessionTimeoutMinutes *int                       `json:"sessionTimeoutMinutes,omitempty"`
	MaxConcurrentSessions *int                       `json:"maxConcurrentSessions,omitempty"`
	AllowRememberMe       *bool                      `json:"allowRememberMe,omitempty"`
	RememberMeDays        *int                       `json:"rememberMeDays,omitempty"`
	Metadata              map[string]string          `json:"metadata,omitempty"`
}

// CreateAuthConfig handles POST /api/v1/auth/config
// Create authentication configuration for a tenant
func (h *AuthConfigHandler) CreateAuthConfig(c *gin.Context) {
	correlationID := fmt.Sprintf("auth-config-create-%d", time.Now().UnixNano())
	h.logger.Info("Creating AuthConfig", "correlation_id", correlationID)

	var req CreateAuthConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err, "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.TenantID) == "" {
		h.logger.Error("Missing tenantId", "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantId is required"})
		return
	}

	// Set defaults if not provided
	defaultBackend := req.DefaultBackend
	if defaultBackend == "" {
		defaultBackend = "local"
	}
	enabledBackends := req.EnabledBackends
	if len(enabledBackends) == 0 {
		enabledBackends = []string{"local"}
	}
	sessionTimeout := req.SessionTimeoutMinutes
	if sessionTimeout == 0 {
		sessionTimeout = 480 // 8 hours default
	}
	maxConcurrent := req.MaxConcurrentSessions
	if maxConcurrent == 0 {
		maxConcurrent = 5 // default
	}
	rememberMeDays := req.RememberMeDays
	if rememberMeDays == 0 {
		rememberMeDays = 30 // 30 days default
	}

	// Create AuthConfig record
	config := &models.AuthConfig{
		TenantID:              strings.TrimSpace(req.TenantID),
		DefaultBackend:        defaultBackend,
		EnabledBackends:       enabledBackends,
		BackendConfigs:        req.BackendConfigs,
		PasswordPolicy:        req.PasswordPolicy,
		Require2FA:            req.Require2FA,
		TOTPIssuer:            req.TOTPIssuer,
		SessionTimeoutMinutes: sessionTimeout,
		MaxConcurrentSessions: maxConcurrent,
		AllowRememberMe:       req.AllowRememberMe,
		RememberMeDays:        rememberMeDays,
		Metadata:              req.Metadata,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
		CreatedBy:             c.GetString("user_id"),
		UpdatedBy:             c.GetString("user_id"),
	}

	err := h.rbacRepo.CreateAuthConfig(c.Request.Context(), config)
	if err != nil {
		h.logger.Error("Failed to create AuthConfig", "error", err, "correlation_id", correlationID, "tenantId", req.TenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create authentication configuration"})
		return
	}

	h.logger.Info("AuthConfig created successfully", "tenantId", req.TenantID, "correlation_id", correlationID)
	c.JSON(http.StatusCreated, config)
}

// GetAuthConfig handles GET /api/v1/auth/config/{tenantId}
// Get authentication configuration for a tenant
func (h *AuthConfigHandler) GetAuthConfig(c *gin.Context) {
	correlationID := fmt.Sprintf("auth-config-get-%d", time.Now().UnixNano())
	tenantID := c.Param("tenantId")

	h.logger.Info("Getting AuthConfig", "tenantId", tenantID, "correlation_id", correlationID)

	// Get AuthConfig record
	config, err := h.rbacRepo.GetAuthConfig(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to get AuthConfig", "error", err, "correlation_id", correlationID, "tenantId", tenantID)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Authentication configuration not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get authentication configuration"})
		}
		return
	}

	h.logger.Info("AuthConfig retrieved successfully", "tenantId", tenantID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, config)
}

// UpdateAuthConfig handles PUT /api/v1/auth/config/{tenantId}
// Update authentication configuration for a tenant
func (h *AuthConfigHandler) UpdateAuthConfig(c *gin.Context) {
	correlationID := fmt.Sprintf("auth-config-update-%d", time.Now().UnixNano())
	tenantID := c.Param("tenantId")

	h.logger.Info("Updating AuthConfig", "tenantId", tenantID, "correlation_id", correlationID)

	var req UpdateAuthConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err, "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get existing record first
	existing, err := h.rbacRepo.GetAuthConfig(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to get existing AuthConfig", "error", err, "correlation_id", correlationID, "tenantId", tenantID)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Authentication configuration not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get authentication configuration"})
		}
		return
	}

	// Update fields if provided
	if req.DefaultBackend != nil {
		existing.DefaultBackend = *req.DefaultBackend
	}
	if req.EnabledBackends != nil {
		existing.EnabledBackends = req.EnabledBackends
	}
	if req.BackendConfigs != nil {
		existing.BackendConfigs = *req.BackendConfigs
	}
	if req.PasswordPolicy != nil {
		existing.PasswordPolicy = *req.PasswordPolicy
	}
	if req.Require2FA != nil {
		existing.Require2FA = *req.Require2FA
	}
	if req.TOTPIssuer != nil {
		existing.TOTPIssuer = *req.TOTPIssuer
	}
	if req.SessionTimeoutMinutes != nil {
		existing.SessionTimeoutMinutes = *req.SessionTimeoutMinutes
	}
	if req.MaxConcurrentSessions != nil {
		existing.MaxConcurrentSessions = *req.MaxConcurrentSessions
	}
	if req.AllowRememberMe != nil {
		existing.AllowRememberMe = *req.AllowRememberMe
	}
	if req.RememberMeDays != nil {
		existing.RememberMeDays = *req.RememberMeDays
	}
	if req.Metadata != nil {
		existing.Metadata = req.Metadata
	}

	existing.UpdatedAt = time.Now()
	existing.UpdatedBy = c.GetString("user_id")

	err = h.rbacRepo.UpdateAuthConfig(c.Request.Context(), existing)
	if err != nil {
		h.logger.Error("Failed to update AuthConfig", "error", err, "correlation_id", correlationID, "tenantId", tenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update authentication configuration"})
		return
	}

	h.logger.Info("AuthConfig updated successfully", "tenantId", tenantID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, existing)
}

// DeleteAuthConfig handles DELETE /api/v1/auth/config/{tenantId}
// Delete authentication configuration for a tenant
func (h *AuthConfigHandler) DeleteAuthConfig(c *gin.Context) {
	correlationID := fmt.Sprintf("auth-config-delete-%d", time.Now().UnixNano())
	tenantID := c.Param("tenantId")

	h.logger.Info("Deleting AuthConfig", "tenantId", tenantID, "correlation_id", correlationID)

	// Delete AuthConfig record
	err := h.rbacRepo.DeleteAuthConfig(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("Failed to delete AuthConfig", "error", err, "correlation_id", correlationID, "tenantId", tenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete authentication configuration"})
		return
	}

	h.logger.Info("AuthConfig deleted successfully", "tenantId", tenantID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, gin.H{"message": "Authentication configuration deleted successfully"})
}
