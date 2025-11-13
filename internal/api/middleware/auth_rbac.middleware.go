package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Auth backend types
const (
	AuthBackendJWT   = "jwt"
	AuthBackendSAML  = "saml"
	AuthBackendLocal = "local"
)

// AuthService handles authentication operations
type AuthService struct {
	config     *config.Config
	cache      cache.ValkeyCluster
	repo       rbac.RBACRepository
	logger     logger.Logger
	sessionMgr *SessionManager
}

// NewAuthService creates a new authentication service
func NewAuthService(cfg *config.Config, cache cache.ValkeyCluster, repo rbac.RBACRepository, logger logger.Logger) *AuthService {
	return &AuthService{
		config:     cfg,
		cache:      cache,
		repo:       repo,
		logger:     logger,
		sessionMgr: NewSessionManager(),
	}
}

// JWTMiddleware validates JWT tokens with RBAC integration
func (as *AuthService) JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for public endpoints
		if isPublicEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Extract token
		tokenString := extractToken(c)
		if tokenString == "" {
			as.logger.Warn("JWT middleware: no token provided", "path", c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Authentication required",
			})
			c.Abort()
			return
		}

		// Validate JWT token
		session, err := as.validateJWTToken(tokenString, c)
		if err != nil {
			as.logger.Warn("JWT middleware: token validation failed", "error", err, "path", c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Invalid authentication token",
				"detail": err.Error(),
			})
			c.Abort()
			return
		}

		// Set context for downstream middleware
		as.setContextFromSession(c, session)
		c.Next()
	}
}

// SAMLMiddleware validates SAML assertions
func (as *AuthService) SAMLMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// SAML validation logic would go here
		// For now, this is a placeholder
		as.logger.Info("SAML middleware: SAML validation not yet implemented")
		c.JSON(http.StatusNotImplemented, gin.H{
			"status": "error",
			"error":  "SAML authentication not yet implemented",
		})
		c.Abort()
	}
}

// LocalAuthMiddleware handles local username/password authentication
func (as *AuthService) LocalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
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

		// Authenticate user
		session, err := as.authenticateLocalUser(req.Username, req.Password, req.TOTPCode, c)
		if err != nil {
			as.logger.Warn("Local auth failed", "username", req.Username, "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Authentication failed",
			})
			return
		}

		// Store session in cache
		if err := as.cache.SetSession(c.Request.Context(), session); err != nil {
			as.logger.Error("Failed to store session", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Session creation failed",
			})
			return
		}

		// Return session token
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"session_token": session.ID,
				"user_id":       session.UserID,
				"tenant_id":     session.TenantID,
				"roles":         session.Roles,
				"expires_at":    session.CreatedAt.Add(24 * time.Hour),
			},
		})
	}
}

// TOTPMiddleware validates TOTP codes
func (as *AuthService) TOTPMiddleware() gin.HandlerFunc {
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

		var req struct {
			Code string `json:"code" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Invalid request format",
			})
			return
		}

		// Validate TOTP code
		if err := as.validateTOTPCode(userID.(string), req.Code); err != nil {
			as.logger.Warn("TOTP validation failed", "user_id", userID, "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Invalid TOTP code",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// PasswordPolicyMiddleware enforces password policies
func (as *AuthService) PasswordPolicyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Invalid request format",
			})
			return
		}

		// Validate password against policy
		if err := as.validatePasswordPolicy(req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Password does not meet policy requirements",
				"detail": err.Error(),
			})
			return
		}

		c.Next()
	}
}

// BackendSelectionMiddleware routes authentication to appropriate backend
func (as *AuthService) BackendSelectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Determine auth backend based on request parameters or configuration
		backend := as.determineAuthBackend(c)

		switch backend {
		case AuthBackendJWT:
			as.JWTMiddleware()(c)
		case AuthBackendSAML:
			as.SAMLMiddleware()(c)
		case AuthBackendLocal:
			as.LocalAuthMiddleware()(c)
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Unsupported authentication backend",
			})
			c.Abort()
		}
	}
}

// IdentityNormalizationMiddleware normalizes identity across providers
func (as *AuthService) IdentityNormalizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user identity from context
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		// Normalize identity and create/update Weaviate user record
		if err := as.normalizeAndPersistIdentity(c, userID.(string)); err != nil {
			as.logger.Error("Identity normalization failed", "user_id", userID, "error", err)
			// Don't fail the request, just log the error
		}

		c.Next()
	}
}

// CorrelationMiddleware adds request correlation and tracing
func (as *AuthService) CorrelationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate correlation ID
		correlationID := as.generateCorrelationID()

		// Set correlation headers
		c.Header("X-Correlation-ID", correlationID)
		c.Header("X-Request-ID", correlationID)

		// Add to context for logging
		c.Set("correlation_id", correlationID)
		c.Set("request_id", correlationID)

		// Log request start
		as.logger.Info("Request started",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"correlation_id", correlationID,
			"user_agent", c.Request.UserAgent(),
			"remote_addr", c.ClientIP(),
		)

		// Track request duration
		start := time.Now()
		defer func() {
			duration := time.Since(start)
			as.logger.Info("Request completed",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"correlation_id", correlationID,
				"duration_ms", duration.Milliseconds(),
				"status", c.Writer.Status(),
			)
		}()

		c.Next()
	}
}

// EnhancedCORSMiddleware handles CORS with RBAC support
func (as *AuthService) EnhancedCORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		if as.isOriginAllowed(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-Token, X-Correlation-ID")
			c.Header("Access-Control-Max-Age", "86400") // 24 hours
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// AuthenticateLocalUser is a public wrapper for authenticateLocalUser
func (as *AuthService) AuthenticateLocalUser(username, password, totpCode string, c *gin.Context) (*models.UserSession, error) {
	return as.authenticateLocalUser(username, password, totpCode, c)
}

// GenerateJWTToken is a public wrapper for generateJWTToken
func (as *AuthService) GenerateJWTToken(session *models.UserSession) (string, error) {
	return as.generateJWTToken(session)
}

// ValidateJWTToken is a public wrapper for validateJWTToken
func (as *AuthService) ValidateJWTToken(tokenString string, c *gin.Context) (*models.UserSession, error) {
	return as.validateJWTToken(tokenString, c)
}

// validateJWTToken validates JWT tokens with RBAC claims
func (as *AuthService) validateJWTToken(tokenString string, c *gin.Context) (*models.UserSession, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(as.config.Auth.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Extract user information
	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("missing user ID in token")
	}

	tenantID := ExtractTenantFromJWT(claims)
	roles := ExtractRolesFromJWT(claims)

	// Create session
	session := as.sessionMgr.CreateSession(userID, tenantID, roles)
	session.IPAddress = c.ClientIP()
	session.UserAgent = c.Request.UserAgent()

	return session, nil
}

// authenticateLocalUser authenticates a local user
func (as *AuthService) authenticateLocalUser(username, password, totpCode string, c *gin.Context) (*models.UserSession, error) {
	// Find user by username/email
	user, err := as.findUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get MiradorAuth record
	auth, err := as.findMiradorAuthByUserID(user.ID)
	if err != nil {
		return nil, fmt.Errorf("authentication record not found")
	}

	// Check if account is active
	if !auth.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}

	// Check password lockout
	if auth.LockedUntil != nil && time.Now().Before(*auth.LockedUntil) {
		return nil, fmt.Errorf("account is temporarily locked")
	}

	// Validate password
	if passwordErr := as.validatePassword(password, auth); passwordErr != nil {
		as.incrementFailedLogin(auth)
		return nil, fmt.Errorf("invalid password")
	}

	// Validate TOTP if enabled
	if auth.TOTPEnabled {
		if totpCode == "" {
			return nil, fmt.Errorf("TOTP code required")
		}
		if totpErr := as.validateTOTPCode(user.ID, totpCode); totpErr != nil {
			return nil, fmt.Errorf("invalid TOTP code")
		}
	}

	// Reset failed login count
	as.resetFailedLogin(auth)

	// Get user roles from RBAC system
	roles, err := as.getUserRoles(user.ID, auth.TenantID)
	if err != nil {
		as.logger.Warn("Failed to get user roles", "user_id", user.ID, "error", err)
		roles = []string{"tenant_guest"} // Default role
	}

	// Create session
	session := as.sessionMgr.CreateSession(user.ID, auth.TenantID, roles)
	session.IPAddress = c.ClientIP()
	session.UserAgent = c.Request.UserAgent()

	return session, nil
}

// validateTOTPCode validates a TOTP code
func (as *AuthService) validateTOTPCode(userID, code string) error {
	auth, err := as.findMiradorAuthByUserID(userID)
	if err != nil {
		return fmt.Errorf("TOTP secret not found")
	}

	if auth.TOTPSecret == "" {
		return fmt.Errorf("TOTP not configured")
	}

	// Validate TOTP code directly (TOTPSecret is stored as base64 encoded)
	if !totp.Validate(code, auth.TOTPSecret) {
		return fmt.Errorf("invalid TOTP code")
	}

	return nil
}

// GenerateTOTPSetup generates TOTP setup information for a user
func (as *AuthService) GenerateTOTPSetup(userID string) (*models.TOTPSetup, error) {
	ctx := context.Background()

	// Get user information
	user, err := as.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Mirador Core",
		AccountName: user.Email,
		SecretSize:  32,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	setup := &models.TOTPSetup{
		Secret:    key.Secret(),
		QRCodeURL: key.URL(),
	}

	return setup, nil
}

// EnableTOTP enables TOTP for a user after validating the setup code
func (as *AuthService) EnableTOTP(userID, setupCode string) error {
	ctx := context.Background()

	auth, err := as.repo.GetMiradorAuth(ctx, userID)
	if err != nil {
		return fmt.Errorf("user auth record not found: %w", err)
	}

	// Validate the setup code
	if !totp.Validate(setupCode, auth.TOTPSecret) {
		return fmt.Errorf("invalid TOTP setup code")
	}

	// Enable TOTP
	auth.TOTPEnabled = true
	auth.BackupCodes = as.generateBackupCodes()

	if err := as.repo.UpdateMiradorAuth(ctx, auth); err != nil {
		return fmt.Errorf("failed to enable TOTP: %w", err)
	}

	return nil
}

// generateBackupCodes generates backup codes for TOTP recovery
func (as *AuthService) generateBackupCodes() []string {
	codes := make([]string, 10)
	for i := range codes {
		bytes := make([]byte, 4)
		if _, err := rand.Read(bytes); err != nil {
			// Fallback
			codes[i] = fmt.Sprintf("%08d", time.Now().UnixNano()%100000000)
		} else {
			codes[i] = hex.EncodeToString(bytes)
		}
	}
	return codes
}

// validatePasswordPolicy validates password against policy
func (as *AuthService) validatePasswordPolicy(password string) error {
	// Get default password policy
	policy := models.PasswordPolicy{
		MinLength:        12,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumbers:   true,
		RequireSymbols:   true,
	}

	if len(password) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters long", policy.MinLength)
	}

	hasUpper := strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	hasLower := strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz")
	hasNumber := strings.ContainsAny(password, "0123456789")
	hasSymbol := strings.ContainsAny(password, "!@#$%^&*()_+-=[]{}|;:,.<>?")

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

	return nil
}

// determineAuthBackend determines which auth backend to use
func (as *AuthService) determineAuthBackend(c *gin.Context) string {
	// Check for JWT token
	if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		return AuthBackendJWT
	}

	// Check for SAML assertion
	if c.Query("SAMLResponse") != "" {
		return AuthBackendSAML
	}

	// Default to local auth
	return AuthBackendLocal
}

// normalizeAndPersistIdentity normalizes and persists user identity
func (as *AuthService) normalizeAndPersistIdentity(c *gin.Context, userID string) error {
	// This would integrate with Weaviate to store/update user identity
	// For now, this is a placeholder
	as.logger.Info("Identity normalization", "user_id", userID)
	return nil
}

// generateCorrelationID generates a unique correlation ID
func (as *AuthService) generateCorrelationID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("corr_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("corr_%s", hex.EncodeToString(bytes))
}

// isOriginAllowed checks if CORS origin is allowed
func (as *AuthService) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// Check against configured allowed origins
	for _, allowed := range as.config.CORS.AllowedOrigins {
		if allowed == "*" || allowed == parsedOrigin.Host {
			return true
		}
	}

	return false
}

// generateJWTToken generates a JWT token for authenticated sessions
func (as *AuthService) generateJWTToken(session *models.UserSession) (string, error) {
	// Create JWT claims
	claims := jwt.MapClaims{
		"sub":    session.UserID,
		"tenant": session.TenantID,
		"roles":  session.Roles,
		"iat":    session.CreatedAt.Unix(),
		"exp":    session.CreatedAt.Add(time.Duration(as.config.Auth.JWT.ExpiryMin) * time.Minute).Unix(),
		"iss":    "mirador-core",
		"aud":    []string{"mirador-core"},
	}

	// Add additional claims from session settings
	if email, exists := session.Settings["email"]; exists {
		claims["email"] = email
	}
	if name, exists := session.Settings["full_name"]; exists {
		claims["name"] = name
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	tokenString, err := token.SignedString([]byte(as.config.Auth.JWT.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w", err)
	}

	return tokenString, nil
}

// findUserByUsername finds a user by username (email or username)
func (as *AuthService) findUserByUsername(username string) (*models.User, error) {
	ctx := context.Background()

	// Try to find by email first
	users, err := as.repo.ListUsers(ctx, rbac.UserFilters{
		Email: &username,
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query users by email: %w", err)
	}

	if len(users) > 0 {
		return users[0], nil
	}

	// Try to find by username
	users, err = as.repo.ListUsers(ctx, rbac.UserFilters{
		Username: &username,
		Limit:    1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query users by username: %w", err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return users[0], nil
}

// findMiradorAuthByUserID finds MiradorAuth record by user ID
func (as *AuthService) findMiradorAuthByUserID(userID string) (*models.MiradorAuth, error) {
	ctx := context.Background()
	return as.repo.GetMiradorAuth(ctx, userID)
}

// validatePassword validates password using bcrypt
func (as *AuthService) validatePassword(password string, auth *models.MiradorAuth) error {
	return bcrypt.CompareHashAndPassword([]byte(auth.PasswordHash), []byte(password))
}

// incrementFailedLogin increments failed login count and locks account if needed
func (as *AuthService) incrementFailedLogin(auth *models.MiradorAuth) {
	ctx := context.Background()

	auth.FailedLoginCount++
	now := time.Now()

	// Lock account after 5 failed attempts for 15 minutes
	if auth.FailedLoginCount >= 5 {
		lockUntil := now.Add(15 * time.Minute)
		auth.LockedUntil = &lockUntil
		as.logger.Warn("Account locked due to failed login attempts",
			"user_id", auth.UserID,
			"attempts", auth.FailedLoginCount)
	}

	auth.LastLoginAt = &now

	// Update the record
	if err := as.repo.UpdateMiradorAuth(ctx, auth); err != nil {
		as.logger.Error("Failed to update auth record after failed login", "error", err)
	}
}

// resetFailedLogin resets failed login count after successful login
func (as *AuthService) resetFailedLogin(auth *models.MiradorAuth) {
	ctx := context.Background()

	auth.FailedLoginCount = 0
	now := time.Now()
	auth.LastLoginAt = &now

	// Update the record
	if err := as.repo.UpdateMiradorAuth(ctx, auth); err != nil {
		as.logger.Error("Failed to update auth record after successful login", "error", err)
	}
}

// getUserRoles gets user roles from RBAC system
func (as *AuthService) getUserRoles(userID, tenantID string) ([]string, error) {
	ctx := context.Background()

	var roles []string

	// First, get the user's global role from the User object
	user, err := as.repo.GetUser(ctx, userID)
	if err != nil {
		as.logger.Warn("Failed to get user for role retrieval", "user_id", userID, "error", err)
	} else if user.GlobalRole != "" {
		roles = append(roles, user.GlobalRole)
	}

	// Then get tenant-specific roles from RBAC repository
	tenantRoles, err := as.repo.GetUserRoles(ctx, tenantID, userID)
	if err != nil {
		as.logger.Warn("Failed to get tenant-specific user roles", "user_id", userID, "tenant_id", tenantID, "error", err)
		// Don't return error, just log it and continue
	} else {
		roles = append(roles, tenantRoles...)
	}

	// If no roles assigned, return default role
	if len(roles) == 0 {
		return []string{"tenant_guest"}, nil
	}

	return roles, nil
}

func (as *AuthService) setContextFromSession(c *gin.Context, session *models.UserSession) {
	c.Set("session", session)
	c.Set("user_id", session.UserID)
	c.Set("tenant_id", session.TenantID)
	c.Set("user_roles", session.Roles)
	c.Set("session_id", session.ID)

	// Add security headers
	headers := DefaultSecurityHeaders()
	ApplySecurityHeaders(c, headers)
}
