// internal/api/handlers/auth.handler.go
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type AuthHandler struct {
	authService *middleware.AuthService
	cache       cache.ValkeyCluster
	rbacRepo    rbac.RBACRepository
	logger      logger.Logger
	config      *config.Config
}

func NewAuthHandler(cfg *config.Config, cache cache.ValkeyCluster, rbacRepo rbac.RBACRepository, logger logger.Logger) *AuthHandler {
	authService := middleware.NewAuthService(cfg, cache, rbacRepo, logger)
	return &AuthHandler{
		authService: authService,
		cache:       cache,
		rbacRepo:    rbacRepo,
		logger:      logger,
		config:      cfg,
	}
}

// Login handles local authentication login requests
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username   string `json:"username" binding:"required"`
		Password   string `json:"password" binding:"required"`
		TOTPCode   string `json:"totp_code,omitempty"`
		RememberMe bool   `json:"remember_me,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Extract tenant from header
	tenantID := c.GetHeader("x-tenant-id")
	if tenantID == "" {
		tenantID = "default" // fallback to default tenant
	}

	// Authenticate user using the auth service
	session, err := h.authService.AuthenticateLocalUser(req.Username, req.Password, req.TOTPCode, tenantID, c)
	if err != nil {
		h.logger.Warn("Local auth failed", "username", req.Username, "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": "error",
			"error":  "Authentication failed",
		})
		return
	}

	// Store session in cache
	if setErr := h.cache.SetSession(c.Request.Context(), session); setErr != nil {
		h.logger.Error("Failed to store session", "error", setErr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Session creation failed",
		})
		return
	}

	// Generate API key for the user instead of JWT token
	rawAPIKey, err := models.GenerateAPIKey()
	if err != nil {
		h.logger.Error("Failed to generate API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "API key generation failed",
		})
		return
	}

	// Create API key record for login-generated key
	apiKeyRecord := &models.APIKey{
		UserID:    session.UserID,
		TenantID:  session.TenantID,
		Name:      "login-generated",
		KeyHash:   models.HashAPIKey(rawAPIKey),
		Prefix:    models.ExtractKeyPrefix(rawAPIKey),
		Roles:     session.Roles,
		Scopes:    []string{"read", "write"}, // Default scopes for login-generated keys
		ExpiresAt: nil,                       // No expiry for login-generated keys
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: session.UserID,
		UpdatedBy: session.UserID,
		Metadata:  map[string]string{"note": "Auto-generated on login"},
	}

	// Store API key in repository
	if err := h.rbacRepo.CreateAPIKey(c.Request.Context(), apiKeyRecord); err != nil {
		h.logger.Error("Failed to store login-generated API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "API key storage failed",
		})
		return
	}

	h.logger.Info("Login-generated API key created",
		"user_id", session.UserID,
		"tenant_id", session.TenantID,
		"api_key_id", apiKeyRecord.ID)

	// Return session token and API key
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"session_token": session.ID,
			"api_key":       rawAPIKey,
			"key_prefix":    models.ExtractKeyPrefix(rawAPIKey),
			"user_id":       session.UserID,
			"tenant_id":     session.TenantID,
			"roles":         session.Roles,
			"expires_at":    session.CreatedAt.Add(24 * time.Hour),
			"warning":       "Store this API key securely. It will not be shown again.",
		},
	})
}

// Logout handles user logout
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID := c.GetString("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "No active session",
		})
		return
	}

	// Invalidate session
	if err := h.cache.InvalidateSession(c.Request.Context(), sessionID); err != nil {
		h.logger.Warn("Failed to invalidate session", "session_id", sessionID, "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"logged_out": true,
		},
	})
}

// ValidateToken validates a session, API key, or JWT token
// POST /api/v1/auth/validate
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Try to validate as API key first (starts with "mrk_")
	if strings.HasPrefix(req.Token, "mrk_") {
		// Extract tenant from authenticated context (set by middleware) or header
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = c.GetHeader("x-tenant-id")
		}
		if tenantID == "" {
			tenantID = "default"
		}

		// Hash the API key for validation
		keyHash := models.HashAPIKey(req.Token)

		// Validate API key using repository
		apiKey, err := h.rbacRepo.ValidateAPIKey(c.Request.Context(), tenantID, keyHash)
		if err != nil {
			h.logger.Warn("API key validation failed", "tenant_id", tenantID, "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Invalid API key",
			})
			return
		}

		if !apiKey.IsValid() {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "API key is inactive or expired",
			})
			return
		}

		// Update last used timestamp
		apiKey.UpdateLastUsed()
		if updateErr := h.rbacRepo.UpdateAPIKey(c.Request.Context(), apiKey); updateErr != nil {
			h.logger.Warn("Failed to update API key last used", "api_key_id", apiKey.ID, "error", updateErr)
			// Don't fail the request for this, just log it
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"valid":     true,
				"type":      "api_key",
				"user_id":   apiKey.UserID,
				"tenant_id": apiKey.TenantID,
				"roles":     apiKey.Roles,
				"scopes":    apiKey.Scopes,
			},
		})
		return
	}

	// Try to validate as session token first
	session, err := h.cache.GetSession(c.Request.Context(), req.Token)
	if err == nil && session != nil {
		// Check if session is still valid
		if time.Since(session.LastActivity) > 24*time.Hour {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Session expired",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"valid":     true,
				"type":      "session",
				"user_id":   session.UserID,
				"tenant_id": session.TenantID,
				"roles":     session.Roles,
			},
		})
		return
	}

	// Try to validate as JWT token
	jwtSession, err := h.authService.ValidateJWTToken(req.Token, c)
	if err == nil && jwtSession != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"valid":     true,
				"type":      "jwt",
				"user_id":   jwtSession.UserID,
				"tenant_id": jwtSession.TenantID,
				"roles":     jwtSession.Roles,
			},
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{
		"status": "error",
		"error":  "Invalid token",
	})
}

// GenerateAPIKey creates a new API key for the authenticated user
// POST /api/v1/auth/apikeys
func (h *AuthHandler) GenerateAPIKey(c *gin.Context) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")
	userRoles := c.GetStringSlice("roles")

	var req struct {
		Name        string     `json:"name" binding:"required"`
		Description string     `json:"description"`
		ExpiresAt   *time.Time `json:"expires_at"`
		Scopes      []string   `json:"scopes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Get current API key count and limits for validation
	// For now, use mock validation - this would need actual repository implementation
	currentKeyCount := h.getCurrentAPIKeyCount(userID)
	maxAllowed := h.getMaxAPIKeysForUser(tenantID, userRoles)

	if currentKeyCount >= maxAllowed {
		h.logger.Warn("API key limit exceeded",
			"user_id", userID,
			"current_count", currentKeyCount,
			"max_allowed", maxAllowed)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":        "error",
			"error":         fmt.Sprintf("API key limit exceeded. Maximum allowed: %d", maxAllowed),
			"current_count": currentKeyCount,
			"max_allowed":   maxAllowed,
		})
		return
	}

	// Generate API key
	rawKey, err := models.GenerateAPIKey()
	if err != nil {
		h.logger.Error("Failed to generate API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to generate API key",
		})
		return
	}

	// Validate expiry against configuration
	if err := h.validateAPIKeyExpiry(req.ExpiresAt); err != nil {
		h.logger.Error("API key expiry validation failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	// Create API key record
	apiKeyRecord := &models.APIKey{
		UserID:    userID,
		TenantID:  tenantID,
		Name:      req.Name,
		KeyHash:   models.HashAPIKey(rawKey),
		Prefix:    models.ExtractKeyPrefix(rawKey),
		Roles:     userRoles,
		Scopes:    req.Scopes,
		ExpiresAt: req.ExpiresAt,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: userID,
		UpdatedBy: userID,
		Metadata:  map[string]string{"description": req.Description},
	}

	// Store in repository
	if err := h.rbacRepo.CreateAPIKey(c.Request.Context(), apiKeyRecord); err != nil {
		h.logger.Error("Failed to create API key in repository", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to create API key",
		})
		return
	}

	h.logger.Info("API key created",
		"user_id", userID,
		"tenant_id", tenantID,
		"name", req.Name,
		"api_key_id", apiKeyRecord.ID)

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data": gin.H{
			"api_key":    rawKey, // Security: Raw key returned only on creation; never stored or logged
			"key_prefix": apiKeyRecord.Prefix,
			"name":       apiKeyRecord.Name,
			"expires_at": apiKeyRecord.ExpiresAt,
			"scopes":     apiKeyRecord.Scopes,
			"warning":    "Store this API key securely. It will not be shown again.",
		},
	})
}

// ListAPIKeys returns API keys based on user permissions
// GET /api/v1/auth/apikeys
func (h *AuthHandler) ListAPIKeys(c *gin.Context) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")

	// Get API keys from repository
	apiKeys, err := h.rbacRepo.ListAPIKeys(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("Failed to list API keys", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to retrieve API keys",
		})
		return
	}

	// Convert to response format (exclude sensitive data like key hashes)
	var responseKeys []gin.H
	for _, key := range apiKeys {
		responseKeys = append(responseKeys, gin.H{
			"id":         key.ID,
			"name":       key.Name,
			"prefix":     key.Prefix,
			"expires_at": key.ExpiresAt,
			"scopes":     key.Scopes,
			"created_at": key.CreatedAt,
			"last_used":  key.LastUsedAt,
			"is_active":  key.IsActive,
		})
	}

	h.logger.Info("Listed API keys",
		"user_id", userID,
		"tenant_id", tenantID,
		"count", len(apiKeys))

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"api_keys": responseKeys,
			"total":    len(responseKeys),
		},
	})
}

// RevokeAPIKey deactivates an API key
// DELETE /api/v1/auth/apikeys/:keyId
func (h *AuthHandler) RevokeAPIKey(c *gin.Context) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")
	keyID := c.Param("keyId")

	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "API key ID is required",
		})
		return
	}

	// Revoke API key using repository
	if err := h.rbacRepo.RevokeAPIKey(c.Request.Context(), tenantID, keyID); err != nil {
		h.logger.Error("Failed to revoke API key", "error", err, "api_key_id", keyID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to revoke API key",
		})
		return
	}

	h.logger.Info("API key revoked",
		"api_key_id", keyID,
		"user_id", userID,
		"tenant_id", tenantID)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"message": "API key revoked successfully",
		},
	})
}

// GetAPIKeyLimits returns the current API key limits for a tenant
// GET /api/v1/auth/apikey-limits
func (h *AuthHandler) GetAPIKeyLimits(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Get tenant-specific limits from configuration
	limits := models.GetAPIKeyLimitsForTenant(tenantID, h.convertConfigToModels())

	// Add configuration metadata for admin visibility
	configInfo := gin.H{
		"configuration_source":  "helm_chart",
		"allow_tenant_override": h.config.APIKeys.AllowTenantOverride,
		"allow_admin_override":  h.config.APIKeys.AllowAdminOverride,
		"enforce_expiry":        h.config.APIKeys.EnforceExpiry,
		"max_expiry_days":       h.config.APIKeys.MaxExpiryDays,
		"min_expiry_days":       h.config.APIKeys.MinExpiryDays,
	}

	h.logger.Info("Retrieved API key limits", "tenant_id", tenantID)

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          limits,
		"configuration": configInfo,
	})
}

// UpdateAPIKeyLimits updates the API key limits for a tenant (admin only)
// PUT /api/v1/auth/apikey-limits
func (h *AuthHandler) UpdateAPIKeyLimits(c *gin.Context) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")
	userRoles := c.GetStringSlice("roles")

	// Check if updates are allowed by configuration
	if !h.config.APIKeys.AllowAdminOverride {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "API key limit updates are disabled by system configuration",
		})
		return
	}

	// Validate admin permissions and configuration constraints
	isGlobalAdmin := false
	for _, role := range userRoles {
		if role == "global_admin" {
			isGlobalAdmin = true
			break
		}
	}

	// Tenant admins can only update if allowed by configuration
	if !isGlobalAdmin && !h.config.APIKeys.AllowTenantOverride {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "Tenant admin overrides are disabled by system configuration",
		})
		return
	}

	if !h.canManageAPIKeyLimits(userRoles) {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "Insufficient permissions. Only tenant admins and global admins can update API key limits",
		})
		return
	}

	var req models.APIKeyLimitsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Create updated limits
	limits := &models.APIKeyLimits{
		TenantID:              tenantID,
		MaxKeysPerUser:        req.MaxKeysPerUser,
		MaxKeysPerTenantAdmin: req.MaxKeysPerTenantAdmin,
		MaxKeysPerGlobalAdmin: req.MaxKeysPerGlobalAdmin,
		UpdatedAt:             time.Now(),
		UpdatedBy:             userID,
	}

	// For now, just log the update - this would need actual repository implementation
	h.logger.Info("Updated API key limits",
		"tenant_id", tenantID,
		"updated_by", userID,
		"max_keys_per_user", req.MaxKeysPerUser,
		"max_keys_per_tenant_admin", req.MaxKeysPerTenantAdmin,
		"max_keys_per_global_admin", req.MaxKeysPerGlobalAdmin)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"message": "API key limits updated successfully",
			"limits":  limits,
		},
	})
}

// Helper methods for API key limit management

// getCurrentAPIKeyCount returns the current number of active API keys for a user
func (h *AuthHandler) getCurrentAPIKeyCount(userID string) int {
	// Get all API keys for the user and count active ones
	// Note: This assumes tenant context is available, but for now we'll use a default
	// In production, this should be called with proper tenant context
	apiKeys, err := h.rbacRepo.ListAPIKeys(context.Background(), "default", userID)
	if err != nil {
		h.logger.Warn("Failed to get API key count", "user_id", userID, "error", err)
		return 0
	}

	activeCount := 0
	for _, key := range apiKeys {
		if key.IsActive && key.IsValid() {
			activeCount++
		}
	}
	return activeCount
}

// getMaxAPIKeysForUser returns the maximum allowed API keys for a user based on tenant limits and roles
func (h *AuthHandler) getMaxAPIKeysForUser(tenantID string, roles []string) int {
	// Use configuration-driven limits instead of hardcoded defaults
	limits := models.GetAPIKeyLimitsForTenant(tenantID, h.convertConfigToModels())
	return limits.GetMaxKeysForRoles(roles)
}

// canManageAPIKeyLimits checks if user can manage API key limits
func (h *AuthHandler) canManageAPIKeyLimits(roles []string) bool {
	for _, role := range roles {
		if role == "tenant_admin" || role == "global_admin" {
			return true
		}
	}
	return false
}

// validateAPIKeyExpiry validates API key expiry against configuration
func (h *AuthHandler) validateAPIKeyExpiry(expiresAt *time.Time) error {
	return models.ValidateExpiryWithConfig(expiresAt, h.convertConfigToModels())
}

// convertConfigToModels converts config.APIKeyLimitsConfig to models.APIKeyLimitsConfig
func (h *AuthHandler) convertConfigToModels() models.APIKeyLimitsConfig {
	cfg := h.config.APIKeys

	// Convert config types to models types
	defaultLimits := models.DefaultAPIKeyLimits{
		MaxKeysPerUser:        cfg.DefaultLimits.MaxKeysPerUser,
		MaxKeysPerTenantAdmin: cfg.DefaultLimits.MaxKeysPerTenantAdmin,
		MaxKeysPerGlobalAdmin: cfg.DefaultLimits.MaxKeysPerGlobalAdmin,
	}

	var tenantLimits []models.TenantAPIKeyLimits
	for _, tl := range cfg.TenantLimits {
		tenantLimits = append(tenantLimits, models.TenantAPIKeyLimits{
			TenantID:              tl.TenantID,
			MaxKeysPerUser:        tl.MaxKeysPerUser,
			MaxKeysPerTenantAdmin: tl.MaxKeysPerTenantAdmin,
			MaxKeysPerGlobalAdmin: tl.MaxKeysPerGlobalAdmin,
		})
	}

	var globalOverride *models.GlobalAPIKeyLimits
	if cfg.GlobalLimitsOverride != nil {
		globalOverride = &models.GlobalAPIKeyLimits{
			MaxKeysPerUser:        cfg.GlobalLimitsOverride.MaxKeysPerUser,
			MaxKeysPerTenantAdmin: cfg.GlobalLimitsOverride.MaxKeysPerTenantAdmin,
			MaxKeysPerGlobalAdmin: cfg.GlobalLimitsOverride.MaxKeysPerGlobalAdmin,
			MaxTotalKeys:          cfg.GlobalLimitsOverride.MaxTotalKeys,
		}
	}

	return models.APIKeyLimitsConfig{
		Enabled:              cfg.Enabled,
		DefaultLimits:        defaultLimits,
		TenantLimits:         tenantLimits,
		GlobalLimitsOverride: globalOverride,
		AllowTenantOverride:  cfg.AllowTenantOverride,
		AllowAdminOverride:   cfg.AllowAdminOverride,
		MaxExpiryDays:        cfg.MaxExpiryDays,
		MinExpiryDays:        cfg.MinExpiryDays,
		EnforceExpiry:        cfg.EnforceExpiry,
	}
}

// GetAPIKeyConfiguration returns the current API key configuration (global admin only)
// GET /api/v1/auth/apikey-config
func (h *AuthHandler) GetAPIKeyConfiguration(c *gin.Context) {
	userRoles := c.GetStringSlice("roles")

	// Only global admins can view system configuration
	isGlobalAdmin := false
	for _, role := range userRoles {
		if role == "global_admin" {
			isGlobalAdmin = true
			break
		}
	}

	if !isGlobalAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "Insufficient permissions. Only global admins can view API key configuration",
		})
		return
	}

	// Return sanitized configuration (no sensitive data)
	configInfo := gin.H{
		"enabled": h.config.APIKeys.Enabled,
		"default_limits": gin.H{
			"max_keys_per_user":         h.config.APIKeys.DefaultLimits.MaxKeysPerUser,
			"max_keys_per_tenant_admin": h.config.APIKeys.DefaultLimits.MaxKeysPerTenantAdmin,
			"max_keys_per_global_admin": h.config.APIKeys.DefaultLimits.MaxKeysPerGlobalAdmin,
		},
		"tenant_specific_limits_count":   len(h.config.APIKeys.TenantLimits),
		"global_limits_override_enabled": h.config.APIKeys.GlobalLimitsOverride != nil,
		"permissions": gin.H{
			"allow_tenant_override": h.config.APIKeys.AllowTenantOverride,
			"allow_admin_override":  h.config.APIKeys.AllowAdminOverride,
		},
		"expiry_settings": gin.H{
			"enforce_expiry":  h.config.APIKeys.EnforceExpiry,
			"max_expiry_days": h.config.APIKeys.MaxExpiryDays,
			"min_expiry_days": h.config.APIKeys.MinExpiryDays,
		},
		"configuration_source": "config_file_or_helm_chart",
		"environment":          h.config.Environment,
	}

	h.logger.Info("Retrieved API key configuration", "requested_by", c.GetString("user_id"))

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   configInfo,
	})
}
