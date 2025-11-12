package middleware

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// SAMLService handles SAML authentication operations
type SAMLService struct {
	config     *config.Config
	cache      cache.ValkeyCluster
	repo       repo.SchemaStore
	logger     logger.Logger
	sessionMgr *SessionManager
}

// NewSAMLService creates a new SAML service
func NewSAMLService(cfg *config.Config, cache cache.ValkeyCluster, repo repo.SchemaStore, logger logger.Logger) *SAMLService {
	return &SAMLService{
		config:     cfg,
		cache:      cache,
		repo:       repo,
		logger:     logger,
		sessionMgr: NewSessionManager(),
	}
}

// SAMLAssertion represents a SAML assertion (placeholder)
type SAMLAssertion struct {
	XMLName            xml.Name `xml:"Assertion"`
	Version            string   `xml:"Version,attr"`
	ID                 string   `xml:"ID,attr"`
	IssueInstant       string   `xml:"IssueInstant,attr"`
	Issuer             string   `xml:"Issuer"`
	Subject            SAMLSubject
	Conditions         SAMLConditions
	AttributeStatement SAMLAttributeStatement `xml:"AttributeStatement"`
}

type SAMLSubject struct {
	NameID              SAMLNameID
	SubjectConfirmation SAMLSubjectConfirmation
}

type SAMLNameID struct {
	Format string `xml:"Format,attr"`
	Value  string `xml:",chardata"`
}

type SAMLSubjectConfirmation struct {
	Method                  string `xml:"Method,attr"`
	SubjectConfirmationData SAMLSubjectConfirmationData
}

type SAMLSubjectConfirmationData struct {
	NotOnOrAfter string `xml:"NotOnOrAfter,attr"`
	Recipient    string `xml:"Recipient,attr"`
}

type SAMLConditions struct {
	NotBefore           string `xml:"NotBefore,attr"`
	NotOnOrAfter        string `xml:"NotOnOrAfter,attr"`
	AudienceRestriction SAMLAudienceRestriction
}

type SAMLAudienceRestriction struct {
	Audience string `xml:"Audience"`
}

type SAMLAttributeStatement struct {
	Attributes []SAMLAttribute `xml:"Attribute"`
}

type SAMLAttribute struct {
	Name         string               `xml:"Name,attr"`
	NameFormat   string               `xml:"NameFormat,attr"`
	FriendlyName string               `xml:"FriendlyName,attr"`
	Values       []SAMLAttributeValue `xml:"AttributeValue"`
}

type SAMLAttributeValue struct {
	Value string `xml:",chardata"`
}

// SAMLResponse represents a SAML response
type SAMLResponse struct {
	XMLName      xml.Name `xml:"Response"`
	Version      string   `xml:"Version,attr"`
	ID           string   `xml:"ID,attr"`
	IssueInstant string   `xml:"IssueInstant,attr"`
	Destination  string   `xml:"Destination,attr"`
	Issuer       string   `xml:"Issuer"`
	Status       SAMLStatus
	Assertion    SAMLAssertion `xml:"Assertion"`
}

type SAMLStatus struct {
	StatusCode SAMLStatusCode
}

type SAMLStatusCode struct {
	Value string `xml:"Value,attr"`
}

// SAMLAuthMiddleware handles SAML authentication
func (ss *SAMLService) SAMLAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Handle SAML response
		if c.Request.Method == "POST" && c.Request.URL.Path == "/auth/saml/acs" {
			ss.handleSAMLResponse(c)
			return
		}

		// Handle SAML request initiation
		if c.Request.Method == "GET" && c.Request.URL.Path == "/auth/saml/login" {
			ss.initiateSAMLLogin(c)
			return
		}

		// For other requests, check for SAML session
		ss.validateSAMLSession(c)
	}
}

// handleSAMLResponse processes SAML response from IdP
func (ss *SAMLService) handleSAMLResponse(c *gin.Context) {
	// Get SAMLResponse from form data
	samlResponse := c.PostForm("SAMLResponse")
	if samlResponse == "" {
		ss.logger.Error("SAML response missing")
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "SAML response missing",
		})
		return
	}

	// Decode base64 SAML response
	responseXML, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		ss.logger.Error("Failed to decode SAML response", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid SAML response format",
		})
		return
	}

	// Parse SAML response
	var samlResp SAMLResponse
	if err := xml.Unmarshal(responseXML, &samlResp); err != nil {
		ss.logger.Error("Failed to parse SAML response", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid SAML response XML",
		})
		return
	}

	// Validate SAML response (placeholder)
	if err := ss.validateSAMLResponse(&samlResp, c); err != nil {
		ss.logger.Error("SAML response validation failed", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": "error",
			"error":  "SAML authentication failed",
			"detail": err.Error(),
		})
		return
	}

	// Extract user information from assertion
	userInfo, err := ss.extractUserInfoFromAssertion(&samlResp.Assertion)
	if err != nil {
		ss.logger.Error("Failed to extract user info from SAML assertion", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to process SAML assertion",
		})
		return
	}

	// Create or update user in system
	session, err := ss.createSAMLUserSession(userInfo, c)
	if err != nil {
		ss.logger.Error("Failed to create SAML user session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Session creation failed",
		})
		return
	}

	// Store session
	if err := ss.cache.SetSession(c.Request.Context(), session); err != nil {
		ss.logger.Error("Failed to store SAML session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Session storage failed",
		})
		return
	}

	// Redirect to application with session token
	redirectURL := "/" // Placeholder - would use config.Auth.SAML.SuccessRedirectURL
	redirectURL += "?session_token=" + session.ID
	c.Redirect(http.StatusFound, redirectURL)
}

// initiateSAMLLogin initiates SAML authentication flow
func (ss *SAMLService) initiateSAMLLogin(c *gin.Context) {
	// Generate SAML request (placeholder)
	samlRequest := "placeholder_saml_request"

	// Encode SAML request (placeholder)
	encodedRequest := base64.StdEncoding.EncodeToString([]byte(samlRequest))

	// Build IdP URL (placeholder)
	idpURL := "https://idp.example.com/saml?SAMLRequest=" + url.QueryEscape(encodedRequest)

	// Redirect to IdP
	c.Redirect(http.StatusFound, idpURL)
}

// validateSAMLSession validates existing SAML session
func (ss *SAMLService) validateSAMLSession(c *gin.Context) {
	// Check for session token
	sessionToken := extractToken(c)
	if sessionToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": "error",
			"error":  "Authentication required",
		})
		c.Abort()
		return
	}

	// Validate session
	session, err := ss.cache.GetSession(c.Request.Context(), sessionToken)
	if err != nil {
		ss.logger.Warn("SAML session validation failed", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": "error",
			"error":  "Invalid session",
		})
		c.Abort()
		return
	}

	// Check if session is SAML-based (placeholder - always assume SAML for now)
	// Placeholder - would check session.AuthMethod

	// Validate SAML session expiry
	if time.Now().After(session.CreatedAt.Add(8 * time.Hour)) { // SAML sessions typically 8 hours
		// Placeholder - would call ss.cache.DeleteSession(c.Request.Context(), sessionToken)
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": "error",
			"error":  "SAML session expired",
		})
		c.Abort()
		return
	}

	// Set context
	c.Set("session", session)
	c.Set("user_id", session.UserID)
	c.Set("tenant_id", session.TenantID)
	c.Set("user_roles", session.Roles)
	c.Set("session_id", session.ID)

	c.Next()
}

// validateSAMLResponse validates the SAML response
func (ss *SAMLService) validateSAMLResponse(resp *SAMLResponse, c *gin.Context) error {
	// Check status
	if resp.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:2.0:status:Success" {
		return fmt.Errorf("SAML authentication failed: %s", resp.Status.StatusCode.Value)
	}

	// Placeholder validation - would check destination, issuer, etc.
	ss.logger.Info("SAML response validation placeholder - implement actual validation")

	// Validate assertion
	return ss.validateSAMLAssertion(&resp.Assertion, c)
}

// validateSAMLAssertion validates the SAML assertion
func (ss *SAMLService) validateSAMLAssertion(assertion *SAMLAssertion, c *gin.Context) error {
	// Placeholder validation - would check issuer, conditions, audience, etc.
	ss.logger.Info("SAML assertion validation placeholder - implement actual validation")
	return nil
}

// extractUserInfoFromAssertion extracts user information from SAML assertion
func (ss *SAMLService) extractUserInfoFromAssertion(assertion *SAMLAssertion) (*SAMLUserInfo, error) {
	userInfo := &SAMLUserInfo{}

	// Extract NameID
	userInfo.NameID = assertion.Subject.NameID.Value
	userInfo.NameIDFormat = assertion.Subject.NameID.Format

	// Extract attributes
	for _, attr := range assertion.AttributeStatement.Attributes {
		switch attr.Name {
		case "email", "mail", "urn:oid:0.9.2342.19200300.100.1.3":
			if len(attr.Values) > 0 {
				userInfo.Email = attr.Values[0].Value
			}
		case "givenName", "firstName", "urn:oid:2.5.4.42":
			if len(attr.Values) > 0 {
				userInfo.FirstName = attr.Values[0].Value
			}
		case "sn", "surname", "lastName", "urn:oid:2.5.4.4":
			if len(attr.Values) > 0 {
				userInfo.LastName = attr.Values[0].Value
			}
		case "displayName", "cn", "urn:oid:2.5.4.3":
			if len(attr.Values) > 0 {
				userInfo.DisplayName = attr.Values[0].Value
			}
		case "memberOf", "groups", "urn:oid:1.3.6.1.4.1.5923.1.5.1.1":
			for _, value := range attr.Values {
				userInfo.Groups = append(userInfo.Groups, value.Value)
			}
		}
	}

	if userInfo.NameID == "" {
		return nil, fmt.Errorf("no NameID found in SAML assertion")
	}

	return userInfo, nil
}

// SAMLUserInfo contains user information extracted from SAML assertion
type SAMLUserInfo struct {
	NameID       string
	NameIDFormat string
	Email        string
	FirstName    string
	LastName     string
	DisplayName  string
	Groups       []string
}

// createSAMLUserSession creates a user session from SAML user info
func (ss *SAMLService) createSAMLUserSession(userInfo *SAMLUserInfo, c *gin.Context) (*models.UserSession, error) {
	// Normalize user identity
	userID := ss.normalizeSAMLUserID(userInfo)
	tenantID := ss.determineTenantFromSAMLGroups(userInfo.Groups)

	// Get roles from SAML groups
	roles := ss.mapSAMLGroupsToRoles(userInfo.Groups)

	// Create session
	session := ss.sessionMgr.CreateSession(userID, tenantID, roles)
	// Placeholder - would set session.AuthMethod = "saml"
	session.IPAddress = c.ClientIP()
	session.UserAgent = c.Request.UserAgent()

	return session, nil
}

// normalizeSAMLUserID creates a normalized user ID from SAML info
func (ss *SAMLService) normalizeSAMLUserID(userInfo *SAMLUserInfo) string {
	// Use email as primary identifier if available
	if userInfo.Email != "" {
		return "saml_" + userInfo.Email
	}

	// Fallback to NameID
	return "saml_" + userInfo.NameID
}

// determineTenantFromSAMLGroups determines tenant from SAML groups
func (ss *SAMLService) determineTenantFromSAMLGroups(groups []string) string {
	// Look for tenant-specific groups
	for _, group := range groups {
		if strings.HasPrefix(group, "tenant_") {
			return strings.TrimPrefix(group, "tenant_")
		}
	}

	// Default tenant
	return DefaultTenantID
}

// mapSAMLGroupsToRoles maps SAML groups to RBAC roles
func (ss *SAMLService) mapSAMLGroupsToRoles(groups []string) []string {
	roles := []string{"tenant_user"} // Default role

	roleMappings := map[string]string{
		"admin":     "tenant_admin",
		"manager":   "tenant_manager",
		"developer": "tenant_developer",
		"viewer":    "tenant_viewer",
	}

	for _, group := range groups {
		group = strings.ToLower(group)
		if role, exists := roleMappings[group]; exists {
			roles = append(roles, role)
		}
	}

	return roles
}
