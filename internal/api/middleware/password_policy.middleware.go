package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// PasswordPolicyService handles password policy enforcement
type PasswordPolicyService struct {
	config *config.Config
	cache  cache.ValkeyCluster
	repo   repo.SchemaStore
	logger logger.Logger
}

// NewPasswordPolicyService creates a new password policy service
func NewPasswordPolicyService(cfg *config.Config, cache cache.ValkeyCluster, repo repo.SchemaStore, logger logger.Logger) *PasswordPolicyService {
	return &PasswordPolicyService{
		config: cfg,
		cache:  cache,
		repo:   repo,
		logger: logger,
	}
}

// PasswordPolicyMiddleware enforces password policies on password changes
func (pps *PasswordPolicyService) PasswordPolicyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Password        string `json:"password" binding:"required"`
			CurrentPassword string `json:"current_password,omitempty"`
			UserID          string `json:"user_id,omitempty"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Invalid request format",
			})
			return
		}

		// Get user ID from context if not provided
		if req.UserID == "" {
			if userID, exists := c.Get("user_id"); exists {
				req.UserID = userID.(string)
			}
		}

		// Validate password against policy
		if err := pps.validatePasswordPolicy(req.Password, req.UserID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Password does not meet policy requirements",
				"detail": err.Error(),
			})
			return
		}

		// Check password history (prevent reuse of recent passwords)
		if req.CurrentPassword != "" && req.UserID != "" {
			if err := pps.checkPasswordHistory(req.UserID, req.Password); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status": "error",
					"error":  "Password cannot be reused",
					"detail": err.Error(),
				})
				return
			}
		}

		c.Next()
	}
}

// validatePasswordPolicy validates a password against the current policy
func (pps *PasswordPolicyService) validatePasswordPolicy(password, userID string) error {
	// Get password policy from config
	policy := pps.getPasswordPolicy()

	// Basic length check
	if len(password) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters long", policy.MinLength)
	}

	// Character requirements
	var hasUpper, hasLower, hasNumber, hasSymbol bool

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSymbol = true
		}
	}

	if policy.RequireUppercase && !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if policy.RequireLowercase && !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if policy.RequireNumbers && !hasNumber {
		return fmt.Errorf("password must contain at least one number")
	}
	if policy.RequireSymbols && !hasSymbol {
		return fmt.Errorf("password must contain at least one symbol")
	}

	// Check against common passwords
	if pps.isCommonPassword(password) {
		return fmt.Errorf("password is too common, please choose a more unique password")
	}

	// Check for sequential characters
	if pps.hasSequentialChars(password) {
		return fmt.Errorf("password cannot contain sequential characters")
	}

	// Check for repeated characters
	if pps.hasRepeatedChars(password) {
		return fmt.Errorf("password cannot contain too many repeated characters")
	}

	// Check against personal information
	if userID != "" {
		if err := pps.checkPersonalInfo(password, userID); err != nil {
			return err
		}
	}

	// Check against dictionary words
	if pps.containsDictionaryWord(password) {
		return fmt.Errorf("password cannot contain dictionary words")
	}

	return nil
}

// checkPasswordHistory checks if password was recently used
func (pps *PasswordPolicyService) checkPasswordHistory(userID, newPassword string) error {
	policy := pps.getPasswordPolicy()

	if policy.PreventReuseCount == 0 {
		return nil
	}

	// Get password history from cache/storage
	history, err := pps.getPasswordHistory(userID)
	if err != nil {
		pps.logger.Warn("Failed to get password history", "user_id", userID, "error", err)
		return nil // Don't block on history check failure
	}

	// Check against recent passwords
	for _, oldHash := range history {
		if pps.comparePasswordToHash(newPassword, oldHash) {
			return fmt.Errorf("password was used recently, please choose a different password")
		}
	}

	return nil
}

// getPasswordPolicy returns the current password policy
func (pps *PasswordPolicyService) getPasswordPolicy() models.PasswordPolicy {
	// Return policy from config, with defaults
	policy := models.PasswordPolicy{
		MinLength:              12,
		RequireUppercase:       true,
		RequireLowercase:       true,
		RequireNumbers:         true,
		RequireSymbols:         true,
		MaxAgeDays:             90,
		PreventReuseCount:      5,
		LockoutThreshold:       5,
		LockoutDurationMinutes: 30,
	}

	// Override with config values if set (placeholder - no config field exists yet)
	// if pps.config.Auth.MinPasswordLength > 0 {
	//     policy.MinLength = pps.config.Auth.MinPasswordLength
	// }

	return policy
}

// isCommonPassword checks if password is in common password list
func (pps *PasswordPolicyService) isCommonPassword(password string) bool {
	commonPasswords := []string{
		"password", "123456", "123456789", "qwerty", "abc123",
		"password123", "admin", "letmein", "welcome", "monkey",
		"1234567890", "password1", "qwerty123", "welcome123",
		"admin123", "root", "user", "guest", "test", "demo",
	}

	password = strings.ToLower(password)
	for _, common := range commonPasswords {
		if password == common {
			return true
		}
	}

	return false
}

// hasSequentialChars checks for sequential characters
func (pps *PasswordPolicyService) hasSequentialChars(password string) bool {
	// Check for sequential letters
	for i := 0; i < len(password)-2; i++ {
		if password[i+1] == password[i]+1 && password[i+2] == password[i]+2 {
			return true
		}
		if password[i+1] == password[i]-1 && password[i+2] == password[i]-2 {
			return true
		}
	}

	// Check for sequential numbers
	for i := 0; i < len(password)-2; i++ {
		if password[i] >= '0' && password[i] <= '9' &&
			password[i+1] >= '0' && password[i+1] <= '9' &&
			password[i+2] >= '0' && password[i+2] <= '9' {
			if password[i+1] == password[i]+1 && password[i+2] == password[i]+2 {
				return true
			}
		}
	}

	return false
}

// hasRepeatedChars checks for too many repeated characters
func (pps *PasswordPolicyService) hasRepeatedChars(password string) bool {
	for i := 0; i < len(password)-2; i++ {
		if password[i] == password[i+1] && password[i+1] == password[i+2] {
			return true
		}
	}
	return false
}

// checkPersonalInfo checks password against user's personal information
func (pps *PasswordPolicyService) checkPersonalInfo(password, userID string) error {
	// Get user information
	user, err := pps.getUserByID(userID)
	if err != nil {
		pps.logger.Warn("Failed to get user for personal info check", "user_id", userID, "error", err)
		return nil // Don't block on this check
	}

	// List of personal info to check against
	personalInfo := []string{
		user.Email,
		user.FullName,
		strings.Split(user.Email, "@")[0], // Email username
	}

	// Check for reversed personal info
	for _, info := range personalInfo {
		if len(info) > 2 {
			reversed := reverseString(info)
			personalInfo = append(personalInfo, reversed)
		}
	}

	password = strings.ToLower(password)
	for _, info := range personalInfo {
		if info == "" {
			continue
		}
		info = strings.ToLower(info)
		if strings.Contains(password, info) {
			return fmt.Errorf("password cannot contain personal information")
		}
	}

	return nil
}

// containsDictionaryWord checks if password contains dictionary words
func (pps *PasswordPolicyService) containsDictionaryWord(password string) bool {
	// Simple dictionary check - in production, use a proper word list
	dictionaryWords := []string{
		"password", "secret", "admin", "user", "login", "welcome",
		"system", "access", "account", "security", "secure", "private",
		"public", "database", "server", "client", "service", "api",
		"token", "key", "auth", "session", "cookie", "header",
	}

	password = strings.ToLower(password)
	for _, word := range dictionaryWords {
		if strings.Contains(password, word) && len(word) > 3 {
			return true
		}
	}

	return false
}

// Helper functions
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// Placeholder methods (would be implemented with actual database/cache calls)
func (pps *PasswordPolicyService) getUserByID(userID string) (*models.User, error) {
	// Placeholder - would query Weaviate
	return &models.User{
		ID:       userID,
		Email:    "user@example.com",
		FullName: "John Doe",
	}, nil
}

func (pps *PasswordPolicyService) getPasswordHistory(userID string) ([]string, error) {
	// Placeholder - would query password history from storage
	return []string{}, nil
}

func (pps *PasswordPolicyService) comparePasswordToHash(password, hash string) bool {
	// Placeholder - would use bcrypt comparison
	return false
}
