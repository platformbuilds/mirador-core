// internal/api/handlers/auth.handler.go
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type AuthHandler struct {
	authService *middleware.AuthService
	cache       cache.ValkeyCluster
	logger      logger.Logger
}

func NewAuthHandler(cfg *config.Config, cache cache.ValkeyCluster, rbacRepo rbac.RBACRepository, logger logger.Logger) *AuthHandler {
	authService := middleware.NewAuthService(cfg, cache, rbacRepo, logger)
	return &AuthHandler{
		authService: authService,
		cache:       cache,
		logger:      logger,
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

	// Authenticate user using the auth service
	session, err := h.authService.AuthenticateLocalUser(req.Username, req.Password, req.TOTPCode, c)
	if err != nil {
		h.logger.Warn("Local auth failed", "username", req.Username, "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": "error",
			"error":  "Authentication failed",
		})
		return
	}

	// Store session in cache
	if err := h.cache.SetSession(c.Request.Context(), session); err != nil {
		h.logger.Error("Failed to store session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Session creation failed",
		})
		return
	}

	// Generate JWT token for the session
	jwtToken, err := h.authService.GenerateJWTToken(session)
	if err != nil {
		h.logger.Error("Failed to generate JWT token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Token generation failed",
		})
		return
	}

	// Return session token and JWT token
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"session_token": session.ID,
			"jwt_token":     jwtToken,
			"user_id":       session.UserID,
			"tenant_id":     session.TenantID,
			"roles":         session.Roles,
			"expires_at":    session.CreatedAt.Add(24 * time.Hour),
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

// ValidateToken validates a session or JWT token
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
