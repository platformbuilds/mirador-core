package services

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

// UUID generation functions (copied from weaviate_repository.go)
var nsMirador = mustParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

func mustParseUUID(s string) [16]byte {
	b, ok := parseUUID(s)
	if !ok {
		panic("invalid UUID namespace: " + s)
	}
	return b
}

func parseUUID(s string) ([16]byte, bool) {
	var out [16]byte
	// remove hyphens
	hex := make([]byte, 0, 32)
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			continue
		}
		hex = append(hex, s[i])
	}
	if len(hex) != 32 {
		return out, false
	}
	// convert hex to bytes
	for i := 0; i < 16; i++ {
		hi := fromHex(hex[2*i])
		lo := fromHex(hex[2*i+1])
		if hi < 0 || lo < 0 {
			return out, false
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, true
}

func fromHex(b byte) int {
	switch {
	case '0' <= b && b <= '9':
		return int(b - '0')
	case 'a' <= b && b <= 'f':
		return int(b - 'a' + 10)
	case 'A' <= b && b <= 'F':
		return int(b - 'A' + 10)
	default:
		return -1
	}
}

func uuidV5(ns [16]byte, name string) string {
	// RFC 4122, version 5: SHA-1 of namespace + name
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	sum := h.Sum(nil) // 20 bytes
	var u [16]byte
	copy(u[:], sum[:16])
	// Set version (5) in high nibble of byte 6
	u[6] = (u[6] & 0x0f) | (5 << 4)
	// Set variant (RFC4122) in the two most significant bits of byte 8
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(u[0])<<24|uint32(u[1])<<16|uint32(u[2])<<8|uint32(u[3]),
		uint16(u[4])<<8|uint16(u[5]),
		uint16(u[6])<<8|uint16(u[7]),
		uint16(u[8])<<8|uint16(u[9]),
		(uint64(u[10])<<40)|(uint64(u[11])<<32)|(uint64(u[12])<<24)|(uint64(u[13])<<16)|(uint64(u[14])<<8)|uint64(u[15]),
	)
}

func makeID(parts ...string) string {
	name := strings.Join(parts, "|")
	return uuidV5(nsMirador, name)
}

func makeRBACID(class, tenantID string, parts ...string) string {
	allParts := append([]string{class, tenantID}, parts...)
	return makeID(allParts...)
}

// RBACBootstrapService handles initial RBAC setup and seeding
type RBACBootstrapService struct {
	rbacService *rbac.RBACService
	repository  rbac.RBACRepository
	logger      logger.Logger
}

// NewRBACBootstrapService creates a new RBAC bootstrap service
func NewRBACBootstrapService(rbacService *rbac.RBACService, repository rbac.RBACRepository, logger logger.Logger) *RBACBootstrapService {
	return &RBACBootstrapService{
		rbacService: rbacService,
		repository:  repository,
		logger:      logger,
	}
}

// BootstrapDefaultTenant creates the default 'PLATFORMBUILDS' tenant if it doesn't exist
func (s *RBACBootstrapService) BootstrapDefaultTenant(ctx context.Context) (string, error) {
	const defaultTenantName = "PLATFORMBUILDS"
	tenantNameFilter := defaultTenantName
	const maxRetries = 3
	const retryDelay = 100 * time.Millisecond

	s.logger.Info("Checking for default tenant", "tenant_name", defaultTenantName)

	// Check if tenant already exists by name with retry logic
	var existingTenants []*models.Tenant
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		existingTenants, err = s.rbacService.ListTenants(ctx, "system", rbac.TenantFilters{Name: &tenantNameFilter})
		if err == nil && len(existingTenants) > 0 {
			s.logger.Info("Default tenant already exists, skipping creation", "tenant_id", existingTenants[0].ID, "tenant_name", defaultTenantName)
			return existingTenants[0].ID, nil
		}
		if attempt < maxRetries-1 {
			s.logger.Debug("Tenant not found, retrying", "attempt", attempt+1, "max_retries", maxRetries)
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to check for existing tenant: %w", err)
	}

	// Create default system tenant (cannot be deleted, only renamed)
	tenant := &models.Tenant{
		Name:        defaultTenantName,
		Description: "Default tenant for Mirador Core platform (system tenant - cannot be deleted)",
		AdminEmail:  "admin@platformbuilds.com",
		Status:      "active",
		IsSystem:    true, // Mark as system tenant - cannot be deleted
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = s.rbacService.CreateTenant(ctx, "system", tenant)
	if err != nil {
		return "", fmt.Errorf("failed to create default tenant: %w", err)
	}

	s.logger.Info("Created default system tenant", "tenant_id", tenant.ID, "tenant_name", defaultTenantName, "is_system", true)

	if tenant.ID == "" {
		return "", fmt.Errorf("tenant ID is empty after creation - this should not happen")
	}

	return tenant.ID, nil
}

// BootstrapGlobalAdmin creates the default global admin user 'aarvee' if it doesn't exist
func (s *RBACBootstrapService) BootstrapGlobalAdmin(ctx context.Context, defaultTenantID string) (string, error) {
	const adminEmail = "admin@platformbuilds.com"

	// Generate deterministic UUID for admin user based on email
	adminUserID := makeRBACID("User", "", adminEmail)

	s.logger.Info("Checking for global admin user", "user_id", adminUserID, "default_tenant_id", defaultTenantID)

	// Check if user already exists
	existingUser, err := s.rbacService.GetUser(ctx, adminUserID)
	if err == nil && existingUser != nil {
		s.logger.Info("Global admin user already exists, checking tenant association", "user_id", adminUserID)

		// Check if tenant-user association exists
		existingTenantUser, err := s.rbacService.GetTenantUser(ctx, defaultTenantID, adminUserID)
		if err == nil && existingTenantUser != nil {
			s.logger.Info("Tenant-user association already exists", "user_id", adminUserID, "tenant_id", defaultTenantID)
			return adminUserID, nil
		}

		// If error is not "not found", it's a real error
		if err != nil && !s.isNotFoundError(err) {
			return "", fmt.Errorf("failed to check for existing tenant-user association: %w", err)
		}

		// Create tenant-user association if it doesn't exist using repository directly
		// to bypass tenant existence validation during bootstrap
		tenantUserID := makeRBACID("TenantUser", defaultTenantID, adminUserID)
		tenantUser := &models.TenantUser{
			ID:         tenantUserID,
			TenantID:   defaultTenantID,
			UserID:     adminUserID,
			TenantRole: "tenant_admin",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			CreatedBy:  "system",
			UpdatedBy:  "system",
		}

		err = s.repository.CreateTenantUser(ctx, tenantUser)
		if err != nil {
			if s.isAlreadyExistsError(err) {
				s.logger.Info("Tenant-user association was created by another instance, skipping", "user_id", adminUserID, "tenant_id", defaultTenantID)
				return adminUserID, nil
			}
			return "", fmt.Errorf("failed to create tenant-user association: %w", err)
		}

		s.logger.Info("Created tenant-user association for existing admin user", "user_id", adminUserID, "tenant_id", defaultTenantID)
		return adminUserID, nil
	}

	// If error is not "not found", it's a real error
	if err != nil && !s.isNotFoundError(err) {
		return "", fmt.Errorf("failed to check for existing user: %w", err)
	}

	// Create admin user
	user := &models.User{
		ID:         adminUserID,
		Email:      adminEmail,
		Username:   "aarvee", // Human-readable username
		GlobalRole: "global_admin",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = s.rbacService.CreateUser(ctx, "system", user)
	if err != nil {
		// Handle case where user was created by another instance during race condition
		if s.isAlreadyExistsError(err) {
			s.logger.Info("Global admin user was created by another instance, checking tenant association", "user_id", adminUserID)

			// Check if tenant-user association exists
			existingTenantUser, err := s.rbacService.GetTenantUser(ctx, defaultTenantID, adminUserID)
			if err == nil && existingTenantUser != nil {
				s.logger.Info("Tenant-user association already exists", "user_id", adminUserID, "tenant_id", defaultTenantID)
				return adminUserID, nil
			}

			// Create tenant-user association if it doesn't exist using repository directly
			// to bypass tenant existence validation during bootstrap
			tenantUserID := makeRBACID("TenantUser", defaultTenantID, adminUserID)
			tenantUser := &models.TenantUser{
				ID:         tenantUserID,
				TenantID:   defaultTenantID,
				UserID:     adminUserID,
				TenantRole: "tenant_admin",
				Status:     "active",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				CreatedBy:  "system",
				UpdatedBy:  "system",
			}

			err = s.repository.CreateTenantUser(ctx, tenantUser)
			if err != nil && !s.isAlreadyExistsError(err) {
				return "", fmt.Errorf("failed to create tenant-user association: %w", err)
			}

			return adminUserID, nil
		}
		return "", fmt.Errorf("failed to create global admin user: %w", err)
	}

	// User created successfully with predetermined ID

	// Create tenant-user association using repository directly to bypass tenant existence validation
	// During bootstrap, we know the tenant was just created, so service validation would fail
	tenantUserID := makeRBACID("TenantUser", defaultTenantID, adminUserID)
	tenantUser := &models.TenantUser{
		ID:         tenantUserID,
		TenantID:   defaultTenantID,
		UserID:     adminUserID,
		TenantRole: "tenant_admin",
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		CreatedBy:  "system",
		UpdatedBy:  "system",
	}

	err = s.repository.CreateTenantUser(ctx, tenantUser)
	if err != nil {
		// Handle case where association was created by another instance during race condition
		if s.isAlreadyExistsError(err) {
			s.logger.Info("Tenant-user association was created by another instance, skipping", "user_id", adminUserID, "tenant_id", defaultTenantID)
			return adminUserID, nil
		}
		return "", fmt.Errorf("failed to create tenant-user association: %w", err)
	}

	s.logger.Info("Created tenant-user association", "user_id", adminUserID, "tenant_id", defaultTenantID)
	return adminUserID, nil
}

// BootstrapAdminAuth creates MiradorAuth credentials for the admin user
func (s *RBACBootstrapService) BootstrapAdminAuth(ctx context.Context, adminUserID string) error {
	s.logger.Info("Checking for admin auth credentials", "user_id", adminUserID)

	// Check if auth already exists
	existingAuth, err := s.repository.GetMiradorAuth(ctx, adminUserID)
	if err == nil && existingAuth != nil {
		s.logger.Info("Admin auth credentials already exist, skipping creation", "user_id", adminUserID)
		return nil
	}

	// If error is not "not found", it's a real error
	if err != nil && !s.isNotFoundError(err) {
		return fmt.Errorf("failed to check for existing auth credentials: %w", err)
	}

	// Generate secure password (this should be changed after first login)
	defaultPassword := "password123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash default password: %w", err)
	}

	// Generate TOTP secret
	totpSecret, err := s.generateTOTPSecret()
	if err != nil {
		return fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	// Get user to retrieve username
	user, err := s.repository.GetUser(ctx, adminUserID)
	if err != nil {
		return fmt.Errorf("failed to get user for auth creation: %w", err)
	}

	// Create MiradorAuth record
	auth := &models.MiradorAuth{
		UserID:       adminUserID,
		Username:     user.Username,
		TenantID:     "", // MiradorAuth is tenant-agnostic
		PasswordHash: string(hashedPassword),
		TOTPSecret:   totpSecret,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.repository.CreateMiradorAuth(ctx, auth)
	if err != nil {
		// Handle case where auth was created by another instance during race condition
		if s.isAlreadyExistsError(err) {
			s.logger.Info("Admin auth credentials were created by another instance, skipping", "user_id", adminUserID)
			return nil
		}
		return fmt.Errorf("failed to create admin auth credentials: %w", err)
	}

	s.logger.Info("Created admin auth credentials", "user_id", adminUserID)
	s.logger.Warn("Default admin password set - CHANGE IMMEDIATELY", "username", adminUserID, "default_password", defaultPassword)
	return nil
}

// BootstrapDefaultRoles creates essential RBAC roles
func (s *RBACBootstrapService) BootstrapDefaultRoles(ctx context.Context, defaultTenantID string) error {
	s.logger.Info("Found default tenant for roles", "tenant_id", defaultTenantID)

	roles := []struct {
		name        string
		description string
		permissions []string
		isSystem    bool
	}{
		{
			name:        "global_admin",
			description: "Global administrator with full system access",
			permissions: []string{"*:admin", "rbac:admin", "tenant:admin", "user:admin"},
			isSystem:    true,
		},
		{
			name:        "tenant_admin",
			description: "Tenant administrator with tenant-level management",
			permissions: []string{"tenant:admin", "rbac:admin", "user:admin", "dashboard:admin"},
			isSystem:    true,
		},
		{
			name:        "tenant_editor",
			description: "Tenant editor with read/write access",
			permissions: []string{"dashboard:create", "dashboard:update", "dashboard:read", "kpi:create", "kpi:update", "kpi:read"},
			isSystem:    true,
		},
		{
			name:        "tenant_guest",
			description: "Tenant guest with read-only access",
			permissions: []string{"dashboard:read", "kpi:read"},
			isSystem:    true,
		},
	}

	for _, roleDef := range roles {
		// Check if role already exists by trying to get it
		existingRole, err := s.rbacService.GetRole(ctx, defaultTenantID, roleDef.name)
		if err == nil && existingRole != nil {
			s.logger.Info("Role already exists, skipping creation", "role", roleDef.name)
			continue
		}

		// If error is not "not found", it's a real error
		if err != nil && !s.isNotFoundError(err) {
			return fmt.Errorf("failed to check for existing role %s: %w", roleDef.name, err)
		}

		// Create role
		role := &models.Role{
			Name:        roleDef.name,
			Description: roleDef.description,
			TenantID:    defaultTenantID,
			Permissions: roleDef.permissions,
			IsSystem:    roleDef.isSystem,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		err = s.rbacService.CreateRole(ctx, defaultTenantID, "system", role)
		if err != nil {
			// Handle case where role was created by another instance during race condition
			if s.isAlreadyExistsError(err) {
				s.logger.Info("Role was created by another instance, skipping", "role", roleDef.name)
				continue
			}
			return fmt.Errorf("failed to create role %s: %w", roleDef.name, err)
		}

		s.logger.Info("Created default role", "role", roleDef.name)
	}

	return nil
}

// RunBootstrap executes the complete RBAC bootstrap process
func (s *RBACBootstrapService) RunBootstrap(ctx context.Context) error {
	s.logger.Info("Starting RBAC bootstrap process")

	// Bootstrap default tenant first
	s.logger.Info("Running bootstrap step", "step", "default tenant")
	defaultTenantID, err := s.BootstrapDefaultTenant(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap step default tenant failed: %w", err)
	}

	// Bootstrap global admin user
	s.logger.Info("Running bootstrap step", "step", "global admin user")
	adminUserID, err := s.BootstrapGlobalAdmin(ctx, defaultTenantID)
	if err != nil {
		return fmt.Errorf("bootstrap step global admin user failed: %w", err)
	}

	// Bootstrap admin auth credentials
	s.logger.Info("Running bootstrap step", "step", "admin auth credentials")
	if err := s.BootstrapAdminAuth(ctx, adminUserID); err != nil {
		return fmt.Errorf("bootstrap step admin auth credentials failed: %w", err)
	}

	// Bootstrap default roles
	s.logger.Info("Running bootstrap step", "step", "default roles")
	if err := s.BootstrapDefaultRoles(ctx, defaultTenantID); err != nil {
		return fmt.Errorf("bootstrap step default roles failed: %w", err)
	}

	s.logger.Info("RBAC bootstrap completed successfully")
	return nil
}

// generateTOTPSecret generates a random TOTP secret
func (s *RBACBootstrapService) generateTOTPSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// isNotFoundError checks if an error indicates that an entity was not found
func (s *RBACBootstrapService) isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "not found") || strings.Contains(errStr, "entity not found") || strings.Contains(errStr, "document not found")
}

// isAlreadyExistsError checks if an error indicates that an entity already exists
func (s *RBACBootstrapService) isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "already exists" || errStr == "entity already exists" || errStr == "duplicate key"
}

// isTenantNotFoundError checks if an error indicates that a tenant was not found
func (s *RBACBootstrapService) isTenantNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "tenant does not exist") || strings.Contains(errStr, "tenant not found")
}
