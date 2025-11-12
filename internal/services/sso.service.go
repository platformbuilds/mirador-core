package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// User represents an authenticated user in the SSO system
type User struct {
	ID          string                 `json:"id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	DisplayName string                 `json:"display_name"`
	Groups      []string               `json:"groups"`
	Attributes  map[string]interface{} `json:"attributes"`
}

type SSOService struct {
	config      config.AuthConfig
	cache       cache.ValkeyCluster
	logger      logger.Logger
	ldapConn    *ldap.Conn
	ldapSyncSvc *LDAPSyncService
}

func NewSSOService(cfg config.AuthConfig, cache cache.ValkeyCluster, logger logger.Logger, ldapSyncSvc *LDAPSyncService) (*SSOService, error) {
	service := &SSOService{
		config:      cfg,
		cache:       cache,
		logger:      logger,
		ldapSyncSvc: ldapSyncSvc,
	}
	// Initialize LDAP connection (tolerate disabled LDAP in config if needed)
	if cfg.LDAP.Enabled {
		if err := service.initLDAPConnection(); err != nil {
			return nil, fmt.Errorf("failed to initialize LDAP: %w", err)
		}
	}
	return service, nil
}

// initLDAPConnection initializes the LDAP connection for authentication
func (s *SSOService) initLDAPConnection() error {
	// LDAP initialization is handled by the LDAPSyncService
	// This method can be used for additional SSO-specific LDAP setup if needed
	return nil
}

// AuthenticateUser authenticates a user against LDAP directory
func (s *SSOService) AuthenticateUser(username, password string) (*User, error) {
	if !s.config.LDAP.Enabled {
		return nil, fmt.Errorf("LDAP authentication is disabled")
	}

	// Use LDAP sync service to authenticate and get user details
	ldapUser, err := s.ldapSyncSvc.AuthenticateUser(username, password)
	if err != nil {
		s.logger.Error("LDAP authentication failed", "username", username, "error", err)
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Convert LDAP user to SSO user
	attributes := make(map[string]interface{})
	for k, v := range ldapUser.Attributes {
		if len(v) == 1 {
			attributes[k] = v[0]
		} else {
			attributes[k] = v
		}
	}

	user := &User{
		ID:          ldapUser.DN, // Use DN as unique identifier
		Username:    ldapUser.Username,
		Email:       ldapUser.Email,
		DisplayName: ldapUser.FullName,
		Groups:      ldapUser.Groups,
		Attributes:  attributes,
	}

	// Cache user session
	if err := s.cacheUserSession(user); err != nil {
		s.logger.Warn("Failed to cache user session", "user", user.Username, "error", err)
		// Don't fail authentication due to cache error
	}

	s.logger.Info("User authenticated successfully", "username", username, "groups", len(user.Groups))
	return user, nil
}

// GetUserGroups retrieves user groups from LDAP
func (s *SSOService) GetUserGroups(username string) ([]string, error) {
	if !s.config.LDAP.Enabled {
		return nil, fmt.Errorf("LDAP is disabled")
	}

	// Use LDAP sync service to get user groups
	ldapUser, err := s.ldapSyncSvc.GetUserByUsername(username)
	if err != nil {
		s.logger.Error("Failed to get user from LDAP", "username", username, "error", err)
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}

	return ldapUser.Groups, nil
}

// ValidateToken validates a user session token
func (s *SSOService) ValidateToken(token string) (*User, error) {
	// Check cache for session
	user, err := s.getUserFromCache(token)
	if err != nil {
		s.logger.Debug("Token not found in cache", "token", token[:8]+"...")
		return nil, fmt.Errorf("invalid token")
	}

	// For now, skip real-time LDAP validation to avoid complexity
	// In production, this could be added as a configurable option

	return user, nil
}

// Logout invalidates a user session
func (s *SSOService) Logout(token string) error {
	return s.invalidateUserSession(token)
}

// cacheUserSession caches user session information
func (s *SSOService) cacheUserSession(user *User) error {
	token := generateSessionToken()
	key := fmt.Sprintf("sso:session:%s", token)

	sessionData := map[string]interface{}{
		"user_id":      user.ID,
		"username":     user.Username,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"groups":       user.Groups,
		"attributes":   user.Attributes,
		"created_at":   time.Now().Unix(),
	}

	data, err := json.Marshal(sessionData)
	if err != nil {
		return err
	}

	// Cache for session timeout duration (use JWT expiry as session timeout)
	ctx := context.Background()
	return s.cache.Set(ctx, key, string(data), time.Duration(s.config.JWT.ExpiryMin)*time.Minute)
}

// getUserFromCache retrieves user from session cache
func (s *SSOService) getUserFromCache(token string) (*User, error) {
	key := fmt.Sprintf("sso:session:%s", token)

	ctx := context.Background()
	data, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return nil, err
	}

	// Check session expiry
	createdAt := int64(sessionData["created_at"].(float64))
	if time.Now().Unix()-createdAt > int64(s.config.JWT.ExpiryMin)*60 {
		s.invalidateUserSession(token)
		return nil, fmt.Errorf("session expired")
	}

	user := &User{
		ID:          sessionData["user_id"].(string),
		Username:    sessionData["username"].(string),
		Email:       sessionData["email"].(string),
		DisplayName: sessionData["display_name"].(string),
		Groups:      make([]string, 0),
		Attributes:  make(map[string]interface{}),
	}

	// Parse groups
	if groups, ok := sessionData["groups"].([]interface{}); ok {
		for _, g := range groups {
			user.Groups = append(user.Groups, g.(string))
		}
	}

	// Parse attributes
	if attrs, ok := sessionData["attributes"].(map[string]interface{}); ok {
		user.Attributes = attrs
	}

	return user, nil
}

// invalidateUserSession removes user session from cache
func (s *SSOService) invalidateUserSession(token string) error {
	key := fmt.Sprintf("sso:session:%s", token)
	ctx := context.Background()
	return s.cache.Delete(ctx, key)
}

// generateSessionToken generates a random session token
func generateSessionToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based token if crypto fails
		return fmt.Sprintf("%d-%d", time.Now().Unix(), time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
