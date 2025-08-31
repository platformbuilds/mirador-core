// ================================
// pkg/auth/ldap.go - LDAP/AD Integration
// ================================

package auth

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type LDAPAuthenticator struct {
	config config.LDAPConfig
	logger logger.Logger
}

func NewLDAPAuthenticator(cfg config.LDAPConfig, logger logger.Logger) *LDAPAuthenticator {
	return &LDAPAuthenticator{
		config: cfg,
		logger: logger,
	}
}

func (l *LDAPAuthenticator) Authenticate(username, password string) (*UserInfo, error) {
	conn, err := ldap.DialURL(l.config.URL)
	if err != nil {
		return nil, fmt.Errorf("LDAP connection failed: %w", err)
	}
	defer conn.Close()

	// Bind with user credentials
	userDN := fmt.Sprintf("uid=%s,%s", username, l.config.BaseDN)
	if err := conn.Bind(userDN, password); err != nil {
		l.logger.Warn("LDAP authentication failed", "username", username)
		return nil, fmt.Errorf("authentication failed")
	}

	// Search for user details
	searchRequest := ldap.NewSearchRequest(
		l.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(uid=%s)", username),
		[]string{"uid", "cn", "mail", "ou", "memberOf", "department"},
		nil,
	)

	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %w", err)
	}

	if len(searchResult.Entries) == 0 {
		return nil, fmt.Errorf("user not found in LDAP")
	}

	entry := searchResult.Entries[0]

	return &UserInfo{
		UserID:     entry.GetAttributeValue("uid"),
		Email:      entry.GetAttributeValue("mail"),
		FullName:   entry.GetAttributeValue("cn"),
		Department: entry.GetAttributeValue("department"),
		TenantID:   extractTenantFromOU(entry.GetAttributeValue("ou")),
		Roles:      extractRolesFromMemberOf(entry.GetAttributeValues("memberOf")),
	}, nil
}

type UserInfo struct {
	UserID     string   `json:"user_id"`
	Email      string   `json:"email"`
	FullName   string   `json:"full_name"`
	Department string   `json:"department"`
	TenantID   string   `json:"tenant_id"`
	Roles      []string `json:"roles"`
}

func extractTenantFromOU(ou string) string {
	// Extract tenant from organizational unit
	// Example: "ou=engineering,ou=company" -> "engineering"
	parts := strings.Split(ou, ",")
	if len(parts) > 0 {
		ouPart := strings.TrimSpace(parts[0])
		if strings.HasPrefix(ouPart, "ou=") {
			return strings.TrimPrefix(ouPart, "ou=")
		}
	}
	return "default"
}

func extractRolesFromMemberOf(memberOf []string) []string {
	var roles []string
	for _, group := range memberOf {
		// Extract role from group DN
		// Example: "cn=mirador-admin,ou=groups,dc=company,dc=com" -> "mirador-admin"
		parts := strings.Split(group, ",")
		if len(parts) > 0 {
			cnPart := strings.TrimSpace(parts[0])
			if strings.HasPrefix(cnPart, "cn=") {
				role := strings.TrimPrefix(cnPart, "cn=")
				if strings.Contains(role, "mirador") || strings.Contains(role, "admin") {
					roles = append(roles, role)
				}
			}
		}
	}

	// Default role if no specific roles found
	if len(roles) == 0 {
		roles = append(roles, "mirador-user")
	}

	return roles
}
