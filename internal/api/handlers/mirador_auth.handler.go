package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MiradorAuthHandler handles MiradorAuth-related HTTP requests
type MiradorAuthHandler struct {
	rbacRepo rbac.RBACRepository
	logger   logger.Logger
}

func NewMiradorAuthHandler(rbacRepo rbac.RBACRepository, logger logger.Logger) *MiradorAuthHandler {
	return &MiradorAuthHandler{
		rbacRepo: rbacRepo,
		logger:   logger,
	}
}

// CreateMiradorAuthRequest represents the request payload for creating MiradorAuth
type CreateMiradorAuthRequest struct {
	UserID                string            `json:"userId" binding:"required"`
	Username              string            `json:"username" binding:"required"`
	Email                 string            `json:"email" binding:"required"`
	Password              string            `json:"password" binding:"required"`
	TenantID              string            `json:"tenantId" binding:"required"`
	Roles                 []string          `json:"roles"`
	Groups                []string          `json:"groups"`
	RequirePasswordChange bool              `json:"requirePasswordChange"`
	Metadata              map[string]string `json:"metadata"`
}

// UpdateMiradorAuthRequest represents the request payload for updating MiradorAuth
type UpdateMiradorAuthRequest struct {
	Username              *string           `json:"username,omitempty"`
	Email                 *string           `json:"email,omitempty"`
	Password              *string           `json:"password,omitempty"`
	Roles                 []string          `json:"roles,omitempty"`
	Groups                []string          `json:"groups,omitempty"`
	IsActive              *bool             `json:"isActive,omitempty"`
	RequirePasswordChange *bool             `json:"requirePasswordChange,omitempty"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}

// CreateMiradorAuth handles POST /api/v1/auth/users
// Create a new MiradorAuth record for local authentication
func (h *MiradorAuthHandler) CreateMiradorAuth(c *gin.Context) {
	correlationID := fmt.Sprintf("mirador-auth-create-%d", time.Now().UnixNano())
	h.logger.Info("Creating MiradorAuth", "correlation_id", correlationID)

	var req CreateMiradorAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err, "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.Username) == "" ||
		strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" ||
		strings.TrimSpace(req.TenantID) == "" {
		h.logger.Error("Missing required fields", "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields: userId, username, email, password, tenantId"})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("Failed to hash password", "error", err, "correlation_id", correlationID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Generate salt (using bcrypt, salt is included in the hash)
	salt := ""

	// Create MiradorAuth record
	auth := &models.MiradorAuth{
		UserID:                strings.TrimSpace(req.UserID),
		Username:              strings.TrimSpace(req.Username),
		Email:                 strings.TrimSpace(req.Email),
		PasswordHash:          string(hashedPassword),
		Salt:                  salt,
		TenantID:              strings.TrimSpace(req.TenantID),
		Roles:                 req.Roles,
		Groups:                req.Groups,
		IsActive:              true,
		RequirePasswordChange: req.RequirePasswordChange,
		Metadata:              req.Metadata,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
		CreatedBy:             c.GetString("user_id"),
		UpdatedBy:             c.GetString("user_id"),
	}

	err = h.rbacRepo.CreateMiradorAuth(c.Request.Context(), auth)
	if err != nil {
		h.logger.Error("Failed to create MiradorAuth", "error", err, "correlation_id", correlationID, "userId", req.UserID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create authentication record"})
		return
	}

	h.logger.Info("MiradorAuth created successfully", "userId", req.UserID, "correlation_id", correlationID)
	c.JSON(http.StatusCreated, auth)
}

// GetMiradorAuth handles GET /api/v1/auth/users/{userId}
// Get MiradorAuth record by user ID
func (h *MiradorAuthHandler) GetMiradorAuth(c *gin.Context) {
	correlationID := fmt.Sprintf("mirador-auth-get-%d", time.Now().UnixNano())
	userID := c.Param("userId")

	h.logger.Info("Getting MiradorAuth", "userId", userID, "correlation_id", correlationID)

	// Get MiradorAuth record
	auth, err := h.rbacRepo.GetMiradorAuth(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get MiradorAuth", "error", err, "correlation_id", correlationID, "userId", userID)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Authentication record not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get authentication record"})
		}
		return
	}

	h.logger.Info("MiradorAuth retrieved successfully", "userId", userID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, auth)
}

// UpdateMiradorAuth handles PUT /api/v1/auth/users/{userId}
// Update MiradorAuth record
func (h *MiradorAuthHandler) UpdateMiradorAuth(c *gin.Context) {
	correlationID := fmt.Sprintf("mirador-auth-update-%d", time.Now().UnixNano())
	userID := c.Param("userId")

	h.logger.Info("Updating MiradorAuth", "userId", userID, "correlation_id", correlationID)

	var req UpdateMiradorAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err, "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get existing record first
	existing, err := h.rbacRepo.GetMiradorAuth(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get existing MiradorAuth", "error", err, "correlation_id", correlationID, "userId", userID)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Authentication record not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get authentication record"})
		}
		return
	}

	// Update fields if provided
	if req.Username != nil {
		existing.Username = strings.TrimSpace(*req.Username)
	}
	if req.Email != nil {
		existing.Email = strings.TrimSpace(*req.Email)
	}
	if req.Password != nil && *req.Password != "" {
		// Hash the new password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			h.logger.Error("Failed to hash password", "error", err, "correlation_id", correlationID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
			return
		}
		existing.PasswordHash = string(hashedPassword)
		existing.PasswordChangedAt = &time.Time{}
		*existing.PasswordChangedAt = time.Now()
	}
	if req.Roles != nil {
		existing.Roles = req.Roles
	}
	if req.Groups != nil {
		existing.Groups = req.Groups
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	if req.RequirePasswordChange != nil {
		existing.RequirePasswordChange = *req.RequirePasswordChange
	}
	if req.Metadata != nil {
		existing.Metadata = req.Metadata
	}

	existing.UpdatedAt = time.Now()
	existing.UpdatedBy = c.GetString("user_id")

	err = h.rbacRepo.UpdateMiradorAuth(c.Request.Context(), existing)
	if err != nil {
		h.logger.Error("Failed to update MiradorAuth", "error", err, "correlation_id", correlationID, "userId", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update authentication record"})
		return
	}

	h.logger.Info("MiradorAuth updated successfully", "userId", userID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, existing)
}

// DeleteMiradorAuth handles DELETE /api/v1/auth/users/{userId}
// Delete MiradorAuth record
func (h *MiradorAuthHandler) DeleteMiradorAuth(c *gin.Context) {
	correlationID := fmt.Sprintf("mirador-auth-delete-%d", time.Now().UnixNano())
	userID := c.Param("userId")

	h.logger.Info("Deleting MiradorAuth", "userId", userID, "correlation_id", correlationID)

	// Delete MiradorAuth record
	err := h.rbacRepo.DeleteMiradorAuth(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to delete MiradorAuth", "error", err, "correlation_id", correlationID, "userId", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete authentication record"})
		return
	}

	h.logger.Info("MiradorAuth deleted successfully", "userId", userID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, gin.H{"message": "Authentication record deleted successfully"})
}
