// internal/api/middleware/session.go
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/miradorstack/internal/models"
)

// generateSessionID creates a cryptographically secure session identifier
func generateSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("sess_%d_%d", time.Now().UnixNano(), time.Now().Unix())
	}
	return "sess_" + hex.EncodeToString(bytes)
}

// SessionManager provides session management utilities
type SessionManager struct {
	defaultExpiry time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		defaultExpiry: 24 * time.Hour, // Default 24 hour expiry
	}
}

// CreateSession creates a new user session
func (sm *SessionManager) CreateSession(userID, tenantID string, roles []string) *models.UserSession {
	now := time.Now()

	return &models.UserSession{
		ID:           generateSessionID(),
		UserID:       userID,
		TenantID:     tenantID,
		Roles:        roles,
		CreatedAt:    now,
		LastActivity: now,
		Settings:     make(map[string]interface{}),
	}
}

// IsSessionValid checks if a session is still valid
func (sm *SessionManager) IsSessionValid(session *models.UserSession) bool {
	if session == nil {
		return false
	}

	now := time.Now()

	// Check activity timeout (24 hours of inactivity)
	if now.Sub(session.LastActivity) > 24*time.Hour {
		return false
	}

	return true
}

// RefreshSession updates session activity
func (sm *SessionManager) RefreshSession(session *models.UserSession) {
	session.LastActivity = time.Now()
}

// ExtractRolesFromJWT extracts roles from JWT claims
func ExtractRolesFromJWT(claims map[string]interface{}) []string {
	var roles []string

	// Try different claim names for roles
	roleClaims := []string{"roles", "groups", "authorities", "permissions"}

	for _, claimName := range roleClaims {
		if rolesClaim, exists := claims[claimName]; exists {
			switch v := rolesClaim.(type) {
			case []interface{}:
				for _, role := range v {
					if roleStr, ok := role.(string); ok {
						roles = append(roles, roleStr)
					}
				}
			case []string:
				roles = append(roles, v...)
			case string:
				// Single role as string
				roles = append(roles, v)
			}
		}
	}

	// Default role if none found
	if len(roles) == 0 {
		roles = append(roles, "user")
	}

	return roles
}

// ExtractTenantFromJWT extracts tenant ID from JWT claims
func ExtractTenantFromJWT(claims map[string]interface{}) string {
	// Try different claim names for tenant
	tenantClaims := []string{"tenant", "tenant_id", "organization", "org", "company"}

	for _, claimName := range tenantClaims {
		if tenantClaim, exists := claims[claimName]; exists {
			if tenantStr, ok := tenantClaim.(string); ok && tenantStr != "" {
				return tenantStr
			}
		}
	}

	// Default tenant
	return "default"
}

// SecurityHeaders contains common security headers for MIRADOR-CORE
type SecurityHeaders struct {
	ContentTypeOptions    string
	FrameOptions          string
	XSSProtection         string
	ReferrerPolicy        string
	ContentSecurityPolicy string
}

// DefaultSecurityHeaders returns secure defaults for MIRADOR-CORE
func DefaultSecurityHeaders() SecurityHeaders {
	return SecurityHeaders{
		ContentTypeOptions:    "nosniff",
		FrameOptions:          "DENY",
		XSSProtection:         "1; mode=block",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:",
	}
}

// ApplySecurityHeaders applies security headers to response
func ApplySecurityHeaders(c *gin.Context, headers SecurityHeaders) {
	c.Header("X-Content-Type-Options", headers.ContentTypeOptions)
	c.Header("X-Frame-Options", headers.FrameOptions)
	c.Header("X-XSS-Protection", headers.XSSProtection)
	c.Header("Referrer-Policy", headers.ReferrerPolicy)
	if headers.ContentSecurityPolicy != "" {
		c.Header("Content-Security-Policy", headers.ContentSecurityPolicy)
	}
}
