package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TOTPService handles TOTP (Time-based One-Time Password) operations
type TOTPService struct {
	config *config.Config
	cache  cache.ValkeyCluster
	repo   repo.SchemaStore
	logger logger.Logger
}

// NewTOTPService creates a new TOTP service
func NewTOTPService(cfg *config.Config, cache cache.ValkeyCluster, repo repo.SchemaStore, logger logger.Logger) *TOTPService {
	return &TOTPService{
		config: cfg,
		cache:  cache,
		repo:   repo,
		logger: logger,
	}
}

// TOTPSetupRequest represents a TOTP setup request
type TOTPSetupRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// TOTPSetupResponse represents a TOTP setup response
type TOTPSetupResponse struct {
	Secret      string   `json:"secret"`
	QRCodeURL   string   `json:"qr_code_url"`
	BackupCodes []string `json:"backup_codes"`
}

// TOTPVerifyRequest represents a TOTP verification request
type TOTPVerifyRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

// TOTPBackupCodeRequest represents a backup code verification request
type TOTPBackupCodeRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	BackupCode string `json:"backup_code" binding:"required"`
}

// TOTPMiddleware validates TOTP codes for authenticated requests
func (ts *TOTPService) TOTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "User not authenticated",
			})
			c.Abort()
			return
		}

		// Check if TOTP is required for this user
		auth, err := ts.findMiradorAuthByUserID(userID.(string))
		if err != nil {
			ts.logger.Error("Failed to get auth record for TOTP check", "user_id", userID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Authentication check failed",
			})
			c.Abort()
			return
		}

		// If TOTP is not enabled, skip validation
		if !auth.TOTPEnabled {
			c.Next()
			return
		}

		// Check for TOTP code in header or form data
		totpCode := c.GetHeader("X-TOTP-Code")
		if totpCode == "" {
			totpCode = c.PostForm("totp_code")
		}
		if totpCode == "" {
			totpCode = c.Query("totp_code")
		}

		if totpCode == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "TOTP code required for this user",
			})
			c.Abort()
			return
		}

		// Validate TOTP code
		if err := ts.validateTOTPCode(userID.(string), totpCode); err != nil {
			ts.logger.Warn("TOTP validation failed", "user_id", userID, "error", err)

			// Check if backup code was provided
			if strings.HasPrefix(totpCode, "backup_") {
				if err := ts.validateBackupCode(userID.(string), strings.TrimPrefix(totpCode, "backup_")); err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{
						"status": "error",
						"error":  "Invalid TOTP or backup code",
					})
					c.Abort()
					return
				}
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"status": "error",
					"error":  "Invalid TOTP code",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// SetupTOTP sets up TOTP for a user
func (ts *TOTPService) SetupTOTP(c *gin.Context) {
	var req TOTPSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MiradorCore", // Placeholder - would use ts.config.Auth.TOTP.Issuer
		AccountName: req.UserID,
		SecretSize:  32,
	})
	if err != nil {
		ts.logger.Error("Failed to generate TOTP secret", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to generate TOTP secret",
		})
		return
	}

	// Generate backup codes
	backupCodes := ts.generateBackupCodes(10)

	// Store TOTP secret and backup codes
	if err := ts.storeTOTPSecret(req.UserID, key.Secret(), backupCodes); err != nil {
		ts.logger.Error("Failed to store TOTP secret", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to store TOTP configuration",
		})
		return
	}

	// Generate QR code URL
	qrCodeURL := key.URL()

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": TOTPSetupResponse{
			Secret:      key.Secret(),
			QRCodeURL:   qrCodeURL,
			BackupCodes: backupCodes,
		},
	})
}

// VerifyTOTPSetup verifies TOTP setup with a test code
func (ts *TOTPService) VerifyTOTPSetup(c *gin.Context) {
	var req TOTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Validate the TOTP code
	if err := ts.validateTOTPCode(req.UserID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid TOTP code",
		})
		return
	}

	// Enable TOTP for the user
	if err := ts.enableTOTPForUser(req.UserID); err != nil {
		ts.logger.Error("Failed to enable TOTP", "user_id", req.UserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to enable TOTP",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "TOTP has been successfully enabled",
	})
}

// DisableTOTP disables TOTP for a user
func (ts *TOTPService) DisableTOTP(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Code   string `json:"code" binding:"required"` // Require current TOTP code to disable
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Validate the TOTP code before disabling
	if err := ts.validateTOTPCode(req.UserID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid TOTP code",
		})
		return
	}

	// Disable TOTP
	if err := ts.disableTOTPForUser(req.UserID); err != nil {
		ts.logger.Error("Failed to disable TOTP", "user_id", req.UserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to disable TOTP",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "TOTP has been disabled",
	})
}

// RegenerateBackupCodes regenerates backup codes for a user
func (ts *TOTPService) RegenerateBackupCodes(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Code   string `json:"code" binding:"required"` // Require TOTP code
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid request format",
		})
		return
	}

	// Validate TOTP code
	if err := ts.validateTOTPCode(req.UserID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid TOTP code",
		})
		return
	}

	// Generate new backup codes
	backupCodes := ts.generateBackupCodes(10)

	// Store new backup codes
	if err := ts.updateBackupCodes(req.UserID, backupCodes); err != nil {
		ts.logger.Error("Failed to update backup codes", "user_id", req.UserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to regenerate backup codes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"backup_codes": backupCodes,
		},
	})
}

// validateTOTPCode validates a TOTP code for a user
func (ts *TOTPService) validateTOTPCode(userID, code string) error {
	auth, err := ts.findMiradorAuthByUserID(userID)
	if err != nil {
		return fmt.Errorf("TOTP secret not found")
	}

	if auth.TOTPSecret == "" {
		return fmt.Errorf("TOTP not configured")
	}

	// Decode base64 secret
	secret, err := base64.StdEncoding.DecodeString(auth.TOTPSecret)
	if err != nil {
		return fmt.Errorf("invalid TOTP secret")
	}

	// Validate TOTP code with 30-second window
	if !totp.Validate(code, string(secret)) {
		return fmt.Errorf("invalid TOTP code")
	}

	return nil
}

// validateBackupCode validates a backup code
func (ts *TOTPService) validateBackupCode(userID, backupCode string) error {
	auth, err := ts.findMiradorAuthByUserID(userID)
	if err != nil {
		return fmt.Errorf("backup codes not found")
	}

	// Check if backup code exists and hasn't been used
	for i, code := range auth.BackupCodes {
		if code == backupCode && code != "" {
			// Mark backup code as used
			auth.BackupCodes[i] = ""
			// Update in storage
			ts.updateMiradorAuth(auth)
			return nil
		}
	}

	return fmt.Errorf("invalid backup code")
}

// generateBackupCodes generates random backup codes
func (ts *TOTPService) generateBackupCodes(count int) []string {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		bytes := make([]byte, 8)
		if _, err := rand.Read(bytes); err != nil {
			ts.logger.Error("Failed to generate backup code", "error", err)
			continue
		}
		codes[i] = fmt.Sprintf("%08x", bytes)
	}
	return codes
}

// storeTOTPSecret stores TOTP secret and backup codes
func (ts *TOTPService) storeTOTPSecret(userID, secret string, backupCodes []string) error {
	// Encode secret in base64
	encodedSecret := base64.StdEncoding.EncodeToString([]byte(secret))

	auth := &models.MiradorAuth{
		UserID:      userID,
		TOTPSecret:  encodedSecret,
		BackupCodes: backupCodes,
		TOTPEnabled: false, // Will be enabled after verification
	}

	return ts.updateMiradorAuth(auth)
}

// enableTOTPForUser enables TOTP for a user
func (ts *TOTPService) enableTOTPForUser(userID string) error {
	auth, err := ts.findMiradorAuthByUserID(userID)
	if err != nil {
		return err
	}

	auth.TOTPEnabled = true
	return ts.updateMiradorAuth(auth)
}

// disableTOTPForUser disables TOTP for a user
func (ts *TOTPService) disableTOTPForUser(userID string) error {
	auth, err := ts.findMiradorAuthByUserID(userID)
	if err != nil {
		return err
	}

	auth.TOTPEnabled = false
	auth.TOTPSecret = ""
	auth.BackupCodes = nil

	return ts.updateMiradorAuth(auth)
}

// updateBackupCodes updates backup codes for a user
func (ts *TOTPService) updateBackupCodes(userID string, backupCodes []string) error {
	auth, err := ts.findMiradorAuthByUserID(userID)
	if err != nil {
		return err
	}

	auth.BackupCodes = backupCodes
	return ts.updateMiradorAuth(auth)
}

// Helper methods (would be implemented with actual database calls)
func (ts *TOTPService) findMiradorAuthByUserID(userID string) (*models.MiradorAuth, error) {
	// Placeholder - would query Weaviate
	return &models.MiradorAuth{
		UserID:      userID,
		TOTPEnabled: false,
	}, nil
}

func (ts *TOTPService) updateMiradorAuth(auth *models.MiradorAuth) error {
	// Placeholder - would update Weaviate
	return nil
}
