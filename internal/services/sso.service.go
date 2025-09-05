package services

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/auth"
	"github.com/platformbuilds/miradorstack/pkg/cache"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type SSOService struct {
	config   config.AuthConfig
	cache    cache.ValkeyCluster
	logger   logger.Logger
	ldapConn *ldap.Conn
}

func NewSSOService(cfg config.AuthConfig, cache cache.ValkeyCluster, logger logger.Logger) (*SSOService, error) {
	service := &SSOService{
		config: cfg,
		cache:  cache,
		logger: logger,
	}
	// Initialize LDAP connection (tolerate disabled LDAP in config if needed)
	if cfg.LDAP.URL != "" {
		if err := service.initLDAPConnection(); err != nil {
			return nil, fmt.Errorf("failed to initialize LDAP: %w", err)
		}
	}
	return service, nil
}

func (s *SSOService) initLDAPConnection() error {
	conn, err := ldap.DialURL(s.config.LDAP.URL, ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		return err
	}
	s.ldapConn = conn
	return nil
}

// AuthenticateUser handles LDAP/AD authentication and creates session
func (s *SSOService) AuthenticateUser(ctx context.Context, username, password string) (*models.UserSession, error) {
	if s.ldapConn == nil {
		return nil, fmt.Errorf("ldap is not configured")
	}

	// LDAP authentication
	userDN := fmt.Sprintf("uid=%s,%s", username, s.config.LDAP.BaseDN)
	if err := s.ldapConn.Bind(userDN, password); err != nil {
		s.logger.Error("LDAP authentication failed", "username", username, "error", err)
		return nil, fmt.Errorf("authentication failed")
	}

	// Get user details and roles from LDAP
	userInfo, err := s.getUserInfoFromLDAP(username)
	if err != nil {
		return nil, err
	}

	// Create user session with Valkey cluster caching
	session := &models.UserSession{
		ID:           generateSessionID(),
		UserID:       userInfo.UserID,
		TenantID:     userInfo.TenantID,
		Roles:        userInfo.Roles,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Settings:     make(map[string]interface{}),
		IPAddress:    "", // Set by middleware
		UserAgent:    "", // Set by middleware
	}

	// Store session in Valkey cluster
	if err := s.cache.SetSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.logger.Info("User authenticated successfully", "username", username, "sessionId", session.ID)
	return session, nil
}

// ValidateOAuthToken handles OAuth 2.0/OIDC token validation
func (s *SSOService) ValidateOAuthToken(ctx context.Context, tokenString string) (*models.UserSession, error) {
	// Parse and validate JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method and return key
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.OAuth.ClientSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid OAuth token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Extract user information from token, safely
	userID, _ := claims["sub"].(string)
	if userID == "" {
		return nil, fmt.Errorf("token missing sub")
	}
	tenantID, _ := claims["tenant"].(string)
	if tenantID == "" {
		tenantID = "default"
	}

	// roles can be []string, []interface{}, or comma-separated string
	var userRoles []string
	switch v := claims["roles"].(type) {
	case []interface{}:
		for _, r := range v {
			if s, ok := r.(string); ok && s != "" {
				userRoles = append(userRoles, strings.ToLower(s))
			}
		}
	case []string:
		for _, r := range v {
			if r != "" {
				userRoles = append(userRoles, strings.ToLower(r))
			}
		}
	case string:
		for _, r := range strings.Split(v, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				userRoles = append(userRoles, strings.ToLower(r))
			}
		}
	}

	// Create session
	session := &models.UserSession{
		ID:           generateSessionID(),
		UserID:       userID,
		TenantID:     tenantID,
		Roles:        userRoles,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Settings:     make(map[string]interface{}),
	}

	// Store in Valkey cluster
	if err := s.cache.SetSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *SSOService) getUserInfoFromLDAP(username string) (*auth.UserInfo, error) {
	// LDAP search for user details
	searchRequest := ldap.NewSearchRequest(
		s.config.LDAP.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(uid=%s)", username),
		[]string{"uid", "cn", "mail", "ou", "memberOf"},
		nil,
	)

	searchResult, err := s.ldapConn.Search(searchRequest)
	if err != nil {
		return nil, err
	}
	if len(searchResult.Entries) == 0 {
		return nil, fmt.Errorf("user not found")
	}
	entry := searchResult.Entries[0]

	// Extract roles from LDAP groups
	roles := extractRolesFromLDAP(entry.GetAttributeValues("memberOf"))

	return &auth.UserInfo{
		UserID:   entry.GetAttributeValue("uid"),
		TenantID: extractTenantFromLDAP(entry.GetAttributeValue("ou")),
		Email:    entry.GetAttributeValue("mail"),
		FullName: entry.GetAttributeValue("cn"),
		Roles:    roles,
	}, nil
}

/* ------------------------- helpers (private) ------------------------- */

// generateSessionID returns a 32-byte random hex string (64 chars).
func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// fallback (dev only) – still unique enough
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// extractRolesFromLDAP converts memberOf DNs to simple role names.
// Example DN: "CN=ops,OU=groups,DC=corp,DC=local" -> "ops"
func extractRolesFromLDAP(memberOf []string) []string {
	roles := make([]string, 0, len(memberOf))
	for _, dn := range memberOf {
		parts := strings.Split(dn, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(strings.ToUpper(p), "CN=") {
				role := strings.TrimSpace(p[3:])
				if role != "" {
					roles = append(roles, strings.ToLower(role))
				}
				break
			}
		}
	}
	// de-dup
	seen := map[string]struct{}{}
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

// extractTenantFromLDAP tries OU or returns a default.
func extractTenantFromLDAP(ou string) string {
	ou = strings.TrimSpace(ou)
	if ou == "" {
		return "default"
	}
	// If OU is a DN fragment like "OU=payments", keep the right side.
	if strings.Contains(ou, "=") {
		kv := strings.SplitN(ou, "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[1]) != "" {
			return strings.ToLower(strings.TrimSpace(kv[1]))
		}
	}
	return strings.ToLower(ou)
}

// Close releases LDAP connection if open.
func (s *SSOService) Close() error {
	if s.ldapConn != nil {
		s.ldapConn.Close()
	}
	return nil
}
