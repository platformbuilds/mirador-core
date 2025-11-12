package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	rbacService *rbac.RBACService
	logger      logger.Logger
}

func NewUserHandler(rbacService *rbac.RBACService, logger logger.Logger) *UserHandler {
	return &UserHandler{
		rbacService: rbacService,
		logger:      logger,
	}
}

// getIntQueryParam extracts an integer query parameter with a default value
func getIntQueryParam(c *gin.Context, key string, defaultValue int) int {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}
	if intValue, err := strconv.Atoi(value); err == nil {
		return intValue
	}
	return defaultValue
} // ListUsers handles GET /api/v1/users
func (h *UserHandler) ListUsers(c *gin.Context) {
	correlationID := fmt.Sprintf("user-list-%d", time.Now().UnixNano())
	h.logger.Info("Listing users", "correlation_id", correlationID)

	filters := rbac.UserFilters{
		Limit:  getIntQueryParam(c, "limit", 50),
		Offset: getIntQueryParam(c, "offset", 0),
	}

	users, err := h.rbacService.ListUsers(c.Request.Context(), filters)
	if err != nil {
		h.logger.Error("Failed to list users", "error", err, "correlation_id", correlationID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	h.logger.Info("Users listed successfully", "count", len(users), "correlation_id", correlationID)
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// CreateUser handles POST /api/v1/users
func (h *UserHandler) CreateUser(c *gin.Context) {
	correlationID := fmt.Sprintf("user-create-%d", time.Now().UnixNano())
	h.logger.Info("Creating user", "correlation_id", correlationID)

	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err, "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if strings.TrimSpace(req.Email) == "" {
		h.logger.Error("Email is required", "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}

	// Generate user ID
	userID := fmt.Sprintf("user-%d", time.Now().UnixNano())

	// Create user
	user := &models.User{
		ID:    userID,
		Email: strings.TrimSpace(req.Email),
	}

	err := h.rbacService.CreateUser(c.Request.Context(), userID, user)
	if err != nil {
		h.logger.Error("Failed to create user", "error", err, "correlation_id", correlationID, "email", req.Email)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	h.logger.Info("User created successfully", "user_id", userID, "correlation_id", correlationID)
	c.JSON(http.StatusCreated, user)
}

// GetUser handles GET /api/v1/users/{id}
func (h *UserHandler) GetUser(c *gin.Context) {
	correlationID := fmt.Sprintf("user-get-%d", time.Now().UnixNano())
	userID := c.Param("id")

	h.logger.Info("Getting user", "user_id", userID, "correlation_id", correlationID)

	// Get user
	user, err := h.rbacService.GetUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user", "error", err, "correlation_id", correlationID, "user_id", userID)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		}
		return
	}

	h.logger.Info("User retrieved successfully", "user_id", userID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, user)
}

// UpdateUser handles PUT /api/v1/users/{id}
func (h *UserHandler) UpdateUser(c *gin.Context) {
	correlationID := fmt.Sprintf("user-update-%d", time.Now().UnixNano())
	userID := c.Param("id")

	h.logger.Info("Updating user", "user_id", userID, "correlation_id", correlationID)

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err, "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if strings.TrimSpace(req.Email) == "" {
		h.logger.Error("Email is required", "correlation_id", correlationID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}

	// Update user
	user := &models.User{
		ID:    userID,
		Email: strings.TrimSpace(req.Email),
	}

	err := h.rbacService.UpdateUser(c.Request.Context(), userID, user)
	if err != nil {
		h.logger.Error("Failed to update user", "error", err, "correlation_id", correlationID, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	h.logger.Info("User updated successfully", "user_id", userID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, user)
}

// DeleteUser handles DELETE /api/v1/users/{id}
func (h *UserHandler) DeleteUser(c *gin.Context) {
	correlationID := fmt.Sprintf("user-delete-%d", time.Now().UnixNano())
	userID := c.Param("id")

	h.logger.Info("Deleting user", "user_id", userID, "correlation_id", correlationID)

	// Delete user
	err := h.rbacService.DeleteUser(c.Request.Context(), userID, userID)
	if err != nil {
		h.logger.Error("Failed to delete user", "error", err, "correlation_id", correlationID, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	h.logger.Info("User deleted successfully", "user_id", userID, "correlation_id", correlationID)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}
