// internal/api/middleware/auth.middleware.go
package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

const (
	// DefaultTenantID is the fallback tenant ID when none is specified
	DefaultTenantID = "default"
	// UnknownTenantID represents an unknown/unset tenant
	UnknownTenantID = "unknown"
)

// AuthMiddleware handles LDAP/AD + SSO authentication for MIRADOR-CORE
func AuthMiddleware(authConfig config.AuthConfig, cache cache.ValkeyCluster) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for public endpoints
		if isPublicEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

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

		// Validate session token or JWT token
		session, err := validateToken(c, token, authConfig, cache)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Invalid authentication token",
				"detail": err.Error(),
			})
			c.Abort()
			return
		}

		// Update last activity
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

	// Try X-Session-Token header (for MIRADOR-UI)
	sessionToken := c.GetHeader("X-Session-Token")
	if sessionToken != "" {
		return sessionToken
	}

	// Try cookie (fallback for browser sessions)
	if cookie, err := c.Cookie("mirador_session"); err == nil {
		return cookie
	}

	// Try query parameter (for WebSocket upgrades)
	if queryToken := c.Query("token"); queryToken != "" {
		return queryToken
	}

	return ""
}

// validateToken validates session or JWT tokens
func validateToken(c *gin.Context, token string, authConfig config.AuthConfig, cache cache.ValkeyCluster) (*models.UserSession, error) {
	// First, try to validate as session token (from cache)
	if session, err := validateSessionToken(c, token, cache); err == nil {
		return session, nil
	}

	// If session validation fails, try JWT validation (OAuth/OIDC)
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
	}

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}

// randomString generates a random string of specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// RequireAuth is a helper middleware that ensures authentication
func RequireAuth(authConfig config.AuthConfig, cache cache.ValkeyCluster) gin.HandlerFunc {
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
