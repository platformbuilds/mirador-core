package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/security/cabundle"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// LDAPSyncService handles LDAP/AD directory synchronization
type LDAPSyncService struct {
	config    config.LDAPConfig
	repo      rbac.RBACRepository
	logger    logger.Logger
	conn      *ldap.Conn
	connMu    sync.RWMutex
	caBundle  *cabundle.Manager
	syncMutex sync.Mutex
	lastSync  time.Time
	isRunning bool
	stopChan  chan struct{}
}

// LDAPUser represents a user from LDAP
type LDAPUser struct {
	DN         string
	Username   string
	Email      string
	FullName   string
	UserID     string
	Groups     []string
	Attributes map[string][]string
	LastSync   time.Time
}

// LDAPGroup represents a group from LDAP
type LDAPGroup struct {
	DN         string
	Name       string
	Members    []string
	ParentDN   string
	Attributes map[string][]string
	LastSync   time.Time
}

// NewLDAPSyncService creates a new LDAP synchronization service
func NewLDAPSyncService(cfg config.LDAPConfig, rbacRepo rbac.RBACRepository, logger logger.Logger) (*LDAPSyncService, error) {
	service := &LDAPSyncService{
		config:   cfg,
		repo:     rbacRepo,
		logger:   logger,
		stopChan: make(chan struct{}),
	}

	if cfg.TLSCABundlePath != "" {
		manager, err := cabundle.NewManager(cfg.TLSCABundlePath, logger, service.handleCABundleReload)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize LDAP CA bundle: %w", err)
		}
		service.caBundle = manager
	}

	if cfg.TLSSkipVerify {
		logger.Warn("LDAP TLS certificate verification is disabled; prefer providing tls_ca_bundle_path")
	}

	if cfg.Enabled && cfg.Sync.Enabled {
		if err := service.connect(); err != nil {
			return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
		}

		// Start background sync if enabled
		go service.startPeriodicSync()
	}

	return service, nil
}

func (s *LDAPSyncService) buildTLSConfig() *tls.Config {
	if s.caBundle != nil {
		cfg := s.caBundle.TLSConfig(s.config.TLSSkipVerify)
		s.applyServerName(cfg)
		return cfg
	}

	cfg := &tls.Config{InsecureSkipVerify: s.config.TLSSkipVerify}
	s.applyServerName(cfg)
	return cfg
}

func (s *LDAPSyncService) applyServerName(cfg *tls.Config) {
	if cfg == nil || cfg.InsecureSkipVerify || cfg.ServerName != "" {
		return
	}
	if host := s.serverHostname(); host != "" {
		cfg.ServerName = host
	}
}

func (s *LDAPSyncService) serverHostname() string {
	if !strings.Contains(s.config.URL, "://") {
		return s.config.URL
	}
	parsed, err := url.Parse(s.config.URL)
	if err != nil {
		return ""
	}
	if host := parsed.Hostname(); host != "" {
		return host
	}
	return parsed.Host
}

func (s *LDAPSyncService) handleCABundleReload() {
	if !s.config.Enabled {
		return
	}

	s.logger.Info("LDAP CA bundle updated; refreshing connection")
	if err := s.connect(); err != nil {
		s.logger.Warn("Failed to refresh LDAP connection after CA bundle update", "error", err)
	}
}

func (s *LDAPSyncService) currentConn() (*ldap.Conn, error) {
	s.connMu.RLock()
	conn := s.conn
	s.connMu.RUnlock()
	if conn == nil {
		return nil, fmt.Errorf("LDAP connection not established")
	}
	return conn, nil
}

// connect establishes LDAP connection with proper TLS configuration
func (s *LDAPSyncService) connect() error {
	var conn *ldap.Conn
	var err error

	// Configure TLS using optional CA bundle
	tlsConfig := s.buildTLSConfig()

	// Connect to LDAP server
	if strings.HasPrefix(s.config.URL, "ldaps://") {
		conn, err = ldap.DialTLS("tcp", strings.TrimPrefix(s.config.URL, "ldaps://"), tlsConfig)
	} else {
		conn, err = ldap.DialURL(s.config.URL)
		if err != nil {
			return err
		}

		// Start TLS if configured
		if s.config.StartTLS {
			err = conn.StartTLS(tlsConfig)
			if err != nil {
				conn.Close()
				return fmt.Errorf("failed to start TLS: %w", err)
			}
		}
	}

	if err != nil {
		return err
	}

	// Bind with service account
	if s.config.BindDN != "" {
		if err := conn.Bind(s.config.BindDN, s.config.BindPassword); err != nil {
			conn.Close()
			return fmt.Errorf("failed to bind: %w", err)
		}
	}

	s.connMu.Lock()
	if s.conn != nil {
		s.conn.Close()
	}
	s.conn = conn
	s.connMu.Unlock()

	s.logger.Info("Successfully connected to LDAP server", "url", s.config.URL)
	return nil
}

// SyncUsers synchronizes users from LDAP to the system (group memberships only)
func (s *LDAPSyncService) SyncUsers(ctx context.Context, tenantID string) error {
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()

	if !s.config.Enabled || !s.config.Sync.UserSyncEnabled {
		return nil
	}

	s.logger.Info("Starting LDAP user synchronization", "tenant", tenantID)

	users, err := s.searchUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to search users: %w", err)
	}

	syncedCount := 0
	for _, ldapUser := range users {
		if err := s.syncUserGroupsToSystem(ctx, tenantID, ldapUser); err != nil {
			s.logger.Error("Failed to sync user groups", "user", ldapUser.Username, "error", err)
			continue
		}
		syncedCount++
	}

	s.logger.Info("LDAP user synchronization completed", "tenant", tenantID, "synced", syncedCount)
	return nil
}

// SyncGroups synchronizes groups from LDAP to the system
func (s *LDAPSyncService) SyncGroups(ctx context.Context, tenantID string) error {
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()

	if !s.config.Enabled || !s.config.Sync.GroupSyncEnabled {
		return nil
	}

	s.logger.Info("Starting LDAP group synchronization", "tenant", tenantID)

	groups, err := s.searchGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to search groups: %w", err)
	}

	syncedCount := 0
	for _, ldapGroup := range groups {
		if err := s.syncGroupToSystem(ctx, tenantID, ldapGroup); err != nil {
			s.logger.Error("Failed to sync group", "group", ldapGroup.Name, "error", err)
			continue
		}
		syncedCount++
	}

	s.logger.Info("LDAP group synchronization completed", "tenant", tenantID, "synced", syncedCount)
	return nil
}

// SyncAll performs full synchronization of users and groups
func (s *LDAPSyncService) SyncAll(ctx context.Context, tenantID string) error {
	if err := s.SyncUsers(ctx, tenantID); err != nil {
		return fmt.Errorf("user sync failed: %w", err)
	}

	if err := s.SyncGroups(ctx, tenantID); err != nil {
		return fmt.Errorf("group sync failed: %w", err)
	}

	s.lastSync = time.Now()
	return nil
}

// searchUsers searches for users in LDAP with paging support
func (s *LDAPSyncService) searchUsers(ctx context.Context) ([]*LDAPUser, error) {
	conn, err := s.currentConn()
	if err != nil {
		return nil, err
	}

	searchBase := s.config.UserSearchBase
	if searchBase == "" {
		searchBase = s.config.BaseDN
	}

	searchFilter := s.config.UserSearchFilter
	if searchFilter == "" {
		searchFilter = "(&(objectClass=user)(!(objectClass=computer)))"
	}

	attributes := []string{
		s.config.Attributes.Username,
		s.config.Attributes.Email,
		s.config.Attributes.FullName,
		s.config.Attributes.MemberOf,
		s.config.Attributes.UserID,
		"dn",
	}

	var allUsers []*LDAPUser
	pageSize := s.config.Sync.PageSize
	if pageSize <= 0 {
		pageSize = 1000
	}

	var pagingControl *ldap.ControlPaging
	for {
		searchRequest := ldap.NewSearchRequest(
			searchBase,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			0, 0, false,
			searchFilter,
			attributes,
			nil,
		)

		if pagingControl != nil {
			searchRequest.Controls = append(searchRequest.Controls, pagingControl)
		}

		searchResult, err := conn.SearchWithPaging(searchRequest, uint32(pageSize))
		if err != nil {
			return nil, fmt.Errorf("LDAP search failed: %w", err)
		}

		for _, entry := range searchResult.Entries {
			user := s.parseLDAPUser(entry)
			if user != nil {
				allUsers = append(allUsers, user)
			}
		}

		// Check for paging control
		pagingControl = s.getPagingControl(searchResult.Controls)
		if pagingControl == nil || len(pagingControl.Cookie) == 0 {
			break
		}
	}

	return allUsers, nil
}

// searchGroups searches for groups in LDAP with paging support
func (s *LDAPSyncService) searchGroups(ctx context.Context) ([]*LDAPGroup, error) {
	conn, err := s.currentConn()
	if err != nil {
		return nil, err
	}

	searchBase := s.config.GroupSearchBase
	if searchBase == "" {
		searchBase = s.config.BaseDN
	}

	searchFilter := s.config.GroupSearchFilter
	if searchFilter == "" {
		searchFilter = "(objectClass=group)"
	}

	attributes := []string{
		s.config.Attributes.GroupName,
		s.config.Attributes.GroupMembers,
		"dn",
		"memberOf", // For nested groups
	}

	var allGroups []*LDAPGroup
	pageSize := s.config.Sync.PageSize
	if pageSize <= 0 {
		pageSize = 1000
	}

	var pagingControl *ldap.ControlPaging
	for {
		searchRequest := ldap.NewSearchRequest(
			searchBase,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			0, 0, false,
			searchFilter,
			attributes,
			nil,
		)

		if pagingControl != nil {
			searchRequest.Controls = append(searchRequest.Controls, pagingControl)
		}

		searchResult, err := conn.SearchWithPaging(searchRequest, uint32(pageSize))
		if err != nil {
			return nil, fmt.Errorf("LDAP group search failed: %w", err)
		}

		for _, entry := range searchResult.Entries {
			group := s.parseLDAPGroup(entry)
			if group != nil {
				allGroups = append(allGroups, group)
			}
		}

		// Check for paging control
		pagingControl = s.getPagingControl(searchResult.Controls)
		if pagingControl == nil || len(pagingControl.Cookie) == 0 {
			break
		}
	}

	return allGroups, nil
}

// parseLDAPUser converts LDAP entry to LDAPUser struct
func (s *LDAPSyncService) parseLDAPUser(entry *ldap.Entry) *LDAPUser {
	username := entry.GetAttributeValue(s.config.Attributes.Username)
	if username == "" {
		return nil
	}

	user := &LDAPUser{
		DN:         entry.DN,
		Username:   username,
		Email:      entry.GetAttributeValue(s.config.Attributes.Email),
		FullName:   entry.GetAttributeValue(s.config.Attributes.FullName),
		UserID:     entry.GetAttributeValue(s.config.Attributes.UserID),
		Groups:     entry.GetAttributeValues(s.config.Attributes.MemberOf),
		Attributes: make(map[string][]string),
		LastSync:   time.Now(),
	}

	// Store all attributes
	for _, attr := range entry.Attributes {
		user.Attributes[attr.Name] = attr.Values
	}

	return user
}

// parseLDAPGroup converts LDAP entry to LDAPGroup struct
func (s *LDAPSyncService) parseLDAPGroup(entry *ldap.Entry) *LDAPGroup {
	name := entry.GetAttributeValue(s.config.Attributes.GroupName)
	if name == "" {
		return nil
	}

	group := &LDAPGroup{
		DN:         entry.DN,
		Name:       name,
		Members:    entry.GetAttributeValues(s.config.Attributes.GroupMembers),
		Attributes: make(map[string][]string),
		LastSync:   time.Now(),
	}

	// Check for parent groups (nested groups)
	parentGroups := entry.GetAttributeValues("memberOf")
	if len(parentGroups) > 0 {
		group.ParentDN = parentGroups[0] // Primary parent
	}

	// Store all attributes
	for _, attr := range entry.Attributes {
		group.Attributes[attr.Name] = attr.Values
	}

	return group
}

// syncUserGroupsToSystem synchronizes user group memberships to the system
func (s *LDAPSyncService) syncUserGroupsToSystem(ctx context.Context, tenantID string, ldapUser *LDAPUser) error {
	// For now, we'll just log the user groups. In a full implementation,
	// this would sync user attributes and group memberships to a user store
	// Since the RBAC system doesn't have users, we focus on group sync
	s.logger.Debug("LDAP user found", "username", ldapUser.Username, "groups", ldapUser.Groups, "email", ldapUser.Email)

	// TODO: Store user attributes in a separate user attributes store
	// For now, we just ensure the user exists in groups
	return nil
}

// syncGroupToSystem synchronizes a single group to the system
func (s *LDAPSyncService) syncGroupToSystem(ctx context.Context, tenantID string, ldapGroup *LDAPGroup) error {
	// Check if group exists
	existingGroup, err := s.repo.GetGroup(ctx, tenantID, ldapGroup.Name)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}

	if existingGroup == nil {
		// Create new group
		group := &models.Group{
			ID:          generateGroupID(ldapGroup.DN),
			Name:        ldapGroup.Name,
			Description: fmt.Sprintf("LDAP Group: %s", ldapGroup.Name),
			TenantID:    tenantID,
			Members:     ldapGroup.Members,
			Roles:       []string{}, // Will be assigned separately
			IsSystem:    false,
			ExternalID:  ldapGroup.DN,
			Metadata: map[string]string{
				"ldap_sync":  "true",
				"last_sync":  ldapGroup.LastSync.Format(time.RFC3339),
				"ldap_attrs": fmt.Sprintf("%v", ldapGroup.Attributes),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Handle nested groups
		if ldapGroup.ParentDN != "" {
			parentName := s.extractGroupNameFromDN(ldapGroup.ParentDN)
			if parentName != "" {
				group.ParentGroups = []string{parentName}
			}
		}

		if err := s.repo.CreateGroup(ctx, group); err != nil {
			return fmt.Errorf("failed to create group: %w", err)
		}
	} else {
		// Update existing group
		existingGroup.Members = ldapGroup.Members
		existingGroup.ExternalID = ldapGroup.DN
		existingGroup.UpdatedAt = time.Now()

		// Update metadata
		if existingGroup.Metadata == nil {
			existingGroup.Metadata = make(map[string]string)
		}
		existingGroup.Metadata["ldap_sync"] = "true"
		existingGroup.Metadata["last_sync"] = ldapGroup.LastSync.Format(time.RFC3339)
		existingGroup.Metadata["ldap_attrs"] = fmt.Sprintf("%v", ldapGroup.Attributes)

		// Handle nested groups
		if ldapGroup.ParentDN != "" {
			parentName := s.extractGroupNameFromDN(ldapGroup.ParentDN)
			if parentName != "" {
				existingGroup.ParentGroups = []string{parentName}
			}
		}

		if err := s.repo.UpdateGroup(ctx, existingGroup); err != nil {
			return fmt.Errorf("failed to update group: %w", err)
		}
	}

	return nil
}

// getPagingControl extracts paging control from LDAP response
func (s *LDAPSyncService) getPagingControl(controls []ldap.Control) *ldap.ControlPaging {
	for _, control := range controls {
		if pagingControl, ok := control.(*ldap.ControlPaging); ok {
			return pagingControl
		}
	}
	return nil
}

// startPeriodicSync starts the periodic synchronization goroutine
func (s *LDAPSyncService) startPeriodicSync() {
	s.isRunning = true
	ticker := time.NewTicker(s.config.Sync.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			// Sync for default tenant - in multi-tenant setup, this would sync for all tenants
			if err := s.SyncAll(ctx, "default"); err != nil {
				s.logger.Error("Periodic LDAP sync failed", "error", err)
			}
		case <-s.stopChan:
			s.isRunning = false
			return
		}
	}
}

// Stop stops the periodic synchronization
func (s *LDAPSyncService) Stop() {
	if s.isRunning {
		close(s.stopChan)
	}
	s.connMu.Lock()
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	s.connMu.Unlock()
	if s.caBundle != nil {
		if err := s.caBundle.Close(); err != nil {
			s.logger.Warn("Failed to stop LDAP CA bundle watcher", "error", err)
		}
	}
}

// GetLastSync returns the timestamp of the last successful sync
func (s *LDAPSyncService) GetLastSync() time.Time {
	return s.lastSync
}

// IsHealthy checks if the LDAP connection is healthy
func (s *LDAPSyncService) IsHealthy() bool {
	conn, err := s.currentConn()
	if err != nil {
		return false
	}

	if s.config.BindDN != "" {
		return conn.Bind(s.config.BindDN, s.config.BindPassword) == nil
	}

	return true
}

// AuthenticateUser authenticates a user against LDAP directory
func (s *LDAPSyncService) AuthenticateUser(username, password string) (*LDAPUser, error) {
	if _, err := s.currentConn(); err != nil {
		return nil, err
	}

	// Find user DN first
	userDN, err := s.findUserDN(username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user DN: %w", err)
	}

	// Create a new connection for user authentication
	userConn, err := s.createAuthConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to create auth connection: %w", err)
	}
	defer userConn.Close()

	err = userConn.Bind(userDN, password)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Get user details
	user, err := s.getUserByDN(userDN)
	if err != nil {
		return nil, fmt.Errorf("failed to get user details: %w", err)
	}

	return user, nil
}

// GetUserByUsername retrieves user details by username
func (s *LDAPSyncService) GetUserByUsername(username string) (*LDAPUser, error) {
	if _, err := s.currentConn(); err != nil {
		return nil, err
	}

	userDN, err := s.findUserDN(username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user DN: %w", err)
	}

	return s.getUserByDN(userDN)
}

// findUserDN finds the DN for a given username
func (s *LDAPSyncService) findUserDN(username string) (string, error) {
	conn, err := s.currentConn()
	if err != nil {
		return "", err
	}

	searchBase := s.config.UserSearchBase
	if searchBase == "" {
		searchBase = s.config.BaseDN
	}

	searchFilter := fmt.Sprintf("(%s=%s)", s.config.Attributes.Username, username)

	searchRequest := ldap.NewSearchRequest(
		searchBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, 0, false,
		searchFilter,
		[]string{"dn"},
		nil,
	)

	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return "", fmt.Errorf("LDAP search failed: %w", err)
	}

	if len(searchResult.Entries) == 0 {
		return "", fmt.Errorf("user not found: %s", username)
	}

	return searchResult.Entries[0].DN, nil
}

// getUserByDN retrieves user details by DN
func (s *LDAPSyncService) getUserByDN(userDN string) (*LDAPUser, error) {
	conn, err := s.currentConn()
	if err != nil {
		return nil, err
	}

	attributes := []string{
		s.config.Attributes.Username,
		s.config.Attributes.Email,
		s.config.Attributes.FullName,
		s.config.Attributes.MemberOf,
		s.config.Attributes.UserID,
		"dn",
	}

	searchRequest := ldap.NewSearchRequest(
		userDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		attributes,
		nil,
	)

	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %w", err)
	}

	if len(searchResult.Entries) == 0 {
		return nil, fmt.Errorf("user not found: %s", userDN)
	}

	return s.parseLDAPUser(searchResult.Entries[0]), nil
}

// createAuthConnection creates a new LDAP connection for authentication
func (s *LDAPSyncService) createAuthConnection() (*ldap.Conn, error) {
	var conn *ldap.Conn
	var err error

	// Configure TLS
	tlsConfig := s.buildTLSConfig()

	// Connect to LDAP server
	if strings.HasPrefix(s.config.URL, "ldaps://") {
		conn, err = ldap.DialTLS("tcp", strings.TrimPrefix(s.config.URL, "ldaps://"), tlsConfig)
	} else {
		conn, err = ldap.DialURL(s.config.URL)
		if err != nil {
			return nil, err
		}

		// Start TLS if configured
		if s.config.StartTLS {
			err = conn.StartTLS(tlsConfig)
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to start TLS: %w", err)
			}
		}
	}

	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *LDAPSyncService) extractGroupNameFromDN(dn string) string {
	// Extract CN from DN like "CN=GroupName,OU=Groups,DC=domain,DC=com"
	parts := strings.Split(dn, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToUpper(part), "CN=") {
			return strings.TrimSpace(part[3:])
		}
	}
	return ""
}

// Helper functions

func generateGroupID(dn string) string {
	// Generate a consistent ID from DN
	return fmt.Sprintf("ldap_%d", len(dn))
}
