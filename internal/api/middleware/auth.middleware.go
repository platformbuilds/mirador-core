// internal/api/middleware/auth.middleware.go
package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

const (
	// DefaultTenantID is the fallback tenant ID when none is specified
	DefaultTenantID = "PLATFORMBUILDS"
	// UnknownTenantID represents an unknown/unset tenant
	UnknownTenantID = "unknown"
)

// AuthMiddleware handles LDAP/AD + SSO authentication for MIRADOR-CORE
func AuthMiddleware(authConfig config.AuthConfig, cache cache.ValkeyCluster, rbacRepo rbac.RBACRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for public endpoints
		if isPublicEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Set auth config in context for rate limiting
		c.Set("auth_config", authConfig)

		// Extract token from request
		token := extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Authentication required",
			})
			c.Abort()
			return
		}

		// Validate token based on strict mode
		var session *models.UserSession
		var err error

		if authConfig.StrictAPIKeyMode && strings.HasPrefix(token, "mrk_") {
			// Strict mode: only API keys allowed for programmatic access
			session, err = validateAPIKeyToken(c, token, cache, rbacRepo)
		} else {
			// Normal mode: try all validation methods
			session, err = validateToken(c, token, authConfig, cache, rbacRepo)
		}

		if err != nil {
			// Record failed auth attempt for monitoring
			// monitoring.RecordAuthFailure(c.Request.Context(), "invalid_token")
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Invalid authentication token",
			})
			c.Abort()
			return
		}

		// Record successful auth for monitoring
		// monitoring.RecordAuthSuccess(c.Request.Context(), session.AuthType)		// Update last activity
		session.LastActivity = time.Now()
		session.IPAddress = c.ClientIP()
		session.UserAgent = c.Request.UserAgent()

		// Store updated session back to cache
		if err := cache.SetSession(c.Request.Context(), session); err != nil {
			// Log error but don't fail the request
			// log.Warn("Failed to update session", "error", err)
		}

		// Set context for downstream middleware and handlers
		c.Set("session", session)
		c.Set("user_id", session.UserID)
		c.Set("tenant_id", session.TenantID)
		c.Set("user_roles", session.Roles)
		c.Set("session_id", session.ID)

		// Add security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")

		c.Next()
	}
}

// extractToken gets authentication token from various sources
func extractToken(c *gin.Context) string {
	// Try Authorization header first (Bearer token)
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return parts[1]
		}
	}

	// Try X-API-Key header for API key authentication
	apiKey := c.GetHeader("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	// Try X-Session-Token header (for MIRADOR-UI)
	sessionToken := c.GetHeader("X-Session-Token")
	if sessionToken != "" {
		return sessionToken
	}

	// Try cookie (fallback for browser sessions)
	if cookie, err := c.Cookie("mirador_session"); err == nil {
		return cookie
	}

	return ""
}

// validateToken validates session, API key, or JWT tokens
func validateToken(c *gin.Context, token string, authConfig config.AuthConfig, cache cache.ValkeyCluster, rbacRepo rbac.RBACRepository) (*models.UserSession, error) {
	// For programmatic access (API calls), only allow API key authentication
	// Check if this looks like an API key (starts with "mrk_" prefix)
	if strings.HasPrefix(token, "mrk_") {
		// This is an API key - validate it
		if session, err := validateAPIKeyToken(c, token, cache, rbacRepo); err == nil {
			return session, nil
		}
		return nil, fmt.Errorf("invalid API key")
	}

	// For non-API key tokens, try session token validation (for UI/web access)
	if session, err := validateSessionToken(c, token, cache); err == nil {
		return session, nil
	}

	// Finally, try JWT validation if OAuth is enabled (for SSO/OIDC)
	if authConfig.OAuth.Enabled {
		if session, err := validateJWTToken(token, authConfig); err == nil {
			// Store JWT-derived session in cache for future requests
			if err := cache.SetSession(c.Request.Context(), session); err != nil {
				// Log but don't fail
			}
			return session, nil
		}
	}

	return nil, fmt.Errorf("token validation failed")
}

// validateSessionToken validates session tokens stored in Valkey cluster
func validateSessionToken(c *gin.Context, token string, cache cache.ValkeyCluster) (*models.UserSession, error) {
	session, err := cache.GetSession(c.Request.Context(), token)
	if err != nil {
		return nil, err
	}

	// Check session expiration (24 hours)
	if time.Since(session.LastActivity) > 24*time.Hour {
		// Remove expired session
		cache.InvalidateSession(c.Request.Context(), token)
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// validateJWTToken validates OAuth 2.0/OIDC JWT tokens
func validateJWTToken(tokenString string, authConfig config.AuthConfig) (*models.UserSession, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(authConfig.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Extract user information from JWT claims
	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("missing user ID in token")
	}

	tenantID, ok := claims["tenant"].(string)
	if !ok {
		tenantID = DefaultTenantID // Fallback tenant
	}

	// Extract roles
	var userRoles []string
	if rolesInterface, exists := claims["roles"]; exists {
		if rolesList, ok := rolesInterface.([]interface{}); ok {
			for _, role := range rolesList {
				if roleStr, ok := role.(string); ok {
					userRoles = append(userRoles, roleStr)
				}
			}
		}
	}

	// Create session from JWT
	session := &models.UserSession{
		ID:           generateSessionID(),
		UserID:       userID,
		TenantID:     tenantID,
		Roles:        userRoles,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Settings:     make(map[string]interface{}),
	}

	// Add additional claims as settings
	if email, exists := claims["email"]; exists {
		session.Settings["email"] = email
	}
	if name, exists := claims["name"]; exists {
		session.Settings["full_name"] = name
	}

	return session, nil
}

// validateAPIKeyToken validates API key tokens
func validateAPIKeyToken(c *gin.Context, token string, cache cache.ValkeyCluster, rbacRepo rbac.RBACRepository) (*models.UserSession, error) {
	// API keys start with "mrk_" prefix
	if !strings.HasPrefix(token, "mrk_") {
		return nil, fmt.Errorf("invalid API key format")
	}

	// Hash the API key for lookup (same logic as in models.APIKey)
	// Security: Never store or compare plaintext API keys; always use secure hashes
	keyHash := models.HashAPIKey(token)

	// Extract tenant ID from context or use default
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = DefaultTenantID
	}

	// Validate API key using repository
	// Performance: Repository implements caching to avoid repeated DB lookups for valid keys
	apiKey, err := rbacRepo.ValidateAPIKey(c.Request.Context(), tenantID, keyHash)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Check if API key is valid
	if !apiKey.IsValid() {
		return nil, fmt.Errorf("invalid API key")
	}

	// Check rate limiting for API key
	if err := checkAPIKeyRateLimit(c, cache, apiKey.ID, tenantID); err != nil {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Create session from API key
	session := &models.UserSession{
		ID:           generateSessionID(),
		UserID:       apiKey.UserID,
		TenantID:     apiKey.TenantID,
		Roles:        apiKey.Roles,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Settings:     make(map[string]interface{}),
	}

	// Mark this as an API key session
	session.Settings["auth_type"] = "api_key"
	session.Settings["api_key_id"] = apiKey.ID
	session.Settings["api_key_prefix"] = models.ExtractKeyPrefix(token)

	return session, nil
}

// checkAPIKeyRateLimit implements per-API-key rate limiting using Valkey cluster
func checkAPIKeyRateLimit(c *gin.Context, cache cache.ValkeyCluster, apiKeyID, tenantID string) error {
	// Get rate limit config from context (set by AuthMiddleware)
	authConfig, exists := c.Get("auth_config")
	if !exists {
		// If no config, skip rate limiting
		return nil
	}

	config, ok := authConfig.(config.AuthConfig)
	if !ok || !config.APIKeyRateLimit.Enabled {
		return nil
	}

	// Create rate limit key: apikey_ratelimit:{tenantID}:{apiKeyID}
	rateLimitKey := fmt.Sprintf("apikey_ratelimit:%s:%s", tenantID, apiKeyID)

	// Check if key is currently blocked
	blockKey := fmt.Sprintf("apikey_blocked:%s:%s", tenantID, apiKeyID)
	_, err := cache.Get(c.Request.Context(), blockKey)
	if err == nil {
		// Key exists, so it's blocked
		return fmt.Errorf("API key is temporarily blocked due to rate limit violations")
	}
	// If error is "key not found", it's not blocked, which is fine

	// Use token bucket algorithm for rate limiting
	maxRequests := int64(config.APIKeyRateLimit.MaxRequests)
	windowDuration := config.APIKeyRateLimit.WindowDuration

	// Get current request count
	currentCountBytes, err := cache.Get(c.Request.Context(), rateLimitKey)
	count := int64(0)
	if err == nil && len(currentCountBytes) > 0 {
		// Successfully got the count
		currentCountStr := string(currentCountBytes)
		if parsed, err := strconv.ParseInt(currentCountStr, 10, 64); err == nil {
			count = parsed
		}
	}
	// If error (key not found), count remains 0, which is fine

	// Check if limit exceeded
	if count >= maxRequests {
		// Block the key
		if err := cache.Set(c.Request.Context(), blockKey, "1", config.APIKeyRateLimit.BlockDuration); err == nil {
			// Reset the counter when blocking
			cache.Set(c.Request.Context(), rateLimitKey, "0", windowDuration)
		}
		return fmt.Errorf("API key rate limit exceeded (%d/%d requests per %v)", count, maxRequests, windowDuration)
	}

	// Increment counter
	newCount := count + 1
	if err := cache.Set(c.Request.Context(), rateLimitKey, strconv.FormatInt(newCount, 10), windowDuration); err != nil {
		// Log error but allow request
		return nil
	}

	// Add rate limit headers to response
	remaining := maxRequests - newCount
	if remaining < 0 {
		remaining = 0
	}
	c.Header("X-RateLimit-Limit", strconv.FormatInt(maxRequests, 10))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(windowDuration).Unix(), 10))

	return nil
}

// isPublicEndpoint checks if an endpoint requires authentication
func isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/health",
		"/ready",
		"/api/openapi.json",
		"/api/openapi.yaml",
		"/swagger/", // Swagger UI
		"/metrics",  // Prometheus metrics endpoint
		"/api/v1/auth/login",
		"/api/v1/auth/oauth/callback",
		"/api/v1/auth/oauth/login",
		"/api/v1/health", // Back-compat health endpoint
	}

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}

// RequireAuth is a helper middleware that ensures authentication
func RequireAuth(authConfig config.AuthConfig, cache cache.ValkeyCluster, rbacRepo rbac.RBACRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get("user_id"); !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Authentication required",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireTenant ensures tenant context is available
func RequireTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" || tenantID == UnknownTenantID {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "Tenant context required",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
