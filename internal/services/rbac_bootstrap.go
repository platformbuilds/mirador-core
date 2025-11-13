package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

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

// BootstrapDefaultTenant creates the default 'platformbuilds' tenant if it doesn't exist
func (s *RBACBootstrapService) BootstrapDefaultTenant(ctx context.Context) error {
	const defaultTenantID = "platformbuilds"

	s.logger.Info("Checking for default tenant", "tenant_id", defaultTenantID)

	// Check if tenant already exists
	existingTenant, err := s.rbacService.GetTenant(ctx, "system", defaultTenantID)
	if err == nil && existingTenant != nil {
		s.logger.Info("Default tenant already exists, skipping creation", "tenant_id", defaultTenantID)
		return nil
	}

	// If error is not "not found", it's a real error
	if err != nil && !s.isNotFoundError(err) {
		return fmt.Errorf("failed to check for existing tenant: %w", err)
	}

	// Create default tenant
	tenant := &models.Tenant{
		ID:          defaultTenantID,
		Name:        "Platform Builds",
		Description: "Default tenant for Mirador Core platform",
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = s.rbacService.CreateTenant(ctx, "system", tenant)
	if err != nil {
		// Handle case where tenant was created by another instance during race condition
		if s.isAlreadyExistsError(err) {
			s.logger.Info("Default tenant was created by another instance, skipping", "tenant_id", defaultTenantID)
			return nil
		}
		return fmt.Errorf("failed to create default tenant: %w", err)
	}

	s.logger.Info("Created default tenant", "tenant_id", defaultTenantID)
	return nil
}

// BootstrapGlobalAdmin creates the default global admin user 'aarvee' if it doesn't exist
func (s *RBACBootstrapService) BootstrapGlobalAdmin(ctx context.Context) error {
	const adminUserID = "aarvee"
	const defaultTenantID = "platformbuilds"

	s.logger.Info("Checking for global admin user", "user_id", adminUserID)

	// Check if user already exists
	existingUser, err := s.rbacService.GetUser(ctx, adminUserID)
	if err == nil && existingUser != nil {
		s.logger.Info("Global admin user already exists, checking tenant association", "user_id", adminUserID)

		// Check if tenant-user association exists
		existingTenantUser, err := s.rbacService.GetTenantUser(ctx, defaultTenantID, adminUserID)
		if err == nil && existingTenantUser != nil {
			s.logger.Info("Tenant-user association already exists", "user_id", adminUserID, "tenant_id", defaultTenantID)
			return nil
		}

		// If error is not "not found", it's a real error
		if err != nil && !s.isNotFoundError(err) {
			return fmt.Errorf("failed to check for existing tenant-user association: %w", err)
		}

		// Create tenant-user association if it doesn't exist
		tenantUser := &models.TenantUser{
			TenantID:   defaultTenantID,
			UserID:     adminUserID,
			TenantRole: "tenant_admin",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		_, err = s.rbacService.CreateTenantUser(ctx, tenantUser, "bootstrap")
		if err != nil {
			// Handle case where association was created by another instance during race condition
			if s.isAlreadyExistsError(err) {
				s.logger.Info("Tenant-user association was created by another instance, skipping", "user_id", adminUserID, "tenant_id", defaultTenantID)
				return nil
			}
			return fmt.Errorf("failed to create tenant-user association: %w", err)
		}

		s.logger.Info("Created tenant-user association for existing admin user", "user_id", adminUserID, "tenant_id", defaultTenantID)
		return nil
	}

	// If error is not "not found", it's a real error
	if err != nil && !s.isNotFoundError(err) {
		return fmt.Errorf("failed to check for existing user: %w", err)
	}

	// Create admin user
	user := &models.User{
		ID:         adminUserID,
		Email:      "admin@platformbuilds.com",
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
				return nil
			}

			// Create tenant-user association if it doesn't exist
			tenantUser := &models.TenantUser{
				TenantID:   defaultTenantID,
				UserID:     adminUserID,
				TenantRole: "tenant_admin",
				Status:     "active",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			_, err = s.rbacService.CreateTenantUser(ctx, tenantUser, "bootstrap")
			if err != nil && !s.isAlreadyExistsError(err) {
				return fmt.Errorf("failed to create tenant-user association: %w", err)
			}

			return nil
		}
		return fmt.Errorf("failed to create global admin user: %w", err)
	}

	// Create tenant-user association
	tenantUser := &models.TenantUser{
		TenantID:   defaultTenantID,
		UserID:     adminUserID,
		TenantRole: "tenant_admin",
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err = s.rbacService.CreateTenantUser(ctx, tenantUser, "bootstrap")
	if err != nil {
		// Handle case where association was created by another instance during race condition
		if s.isAlreadyExistsError(err) {
			s.logger.Info("Tenant-user association was created by another instance, skipping", "user_id", adminUserID, "tenant_id", defaultTenantID)
			return nil
		}
		return fmt.Errorf("failed to create tenant-user association: %w", err)
	}

	s.logger.Info("Created global admin user", "user_id", adminUserID, "tenant_id", defaultTenantID)
	return nil
}

// BootstrapAdminAuth creates MiradorAuth credentials for the admin user
func (s *RBACBootstrapService) BootstrapAdminAuth(ctx context.Context) error {
	const adminUserID = "aarvee"

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
	defaultPassword := "ChangeMe123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash default password: %w", err)
	}

	// Generate TOTP secret
	totpSecret, err := s.generateTOTPSecret()
	if err != nil {
		return fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	// Create MiradorAuth record
	auth := &models.MiradorAuth{
		UserID:       adminUserID,
		Username:     adminUserID,
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
func (s *RBACBootstrapService) BootstrapDefaultRoles(ctx context.Context) error {
	const defaultTenantID = "platformbuilds"

	roles := []struct {
		name        string
		description string
		permissions []string
		isSystem    bool
	}{
		{
			name:        "global_admin",
			description: "Global administrator with full system access",
			permissions: []string{"admin", "rbac.admin", "tenant.admin", "user.admin"},
			isSystem:    true,
		},
		{
			name:        "tenant_admin",
			description: "Tenant administrator with tenant-level management",
			permissions: []string{"tenant.admin", "rbac.admin", "user.admin", "dashboard.admin"},
			isSystem:    true,
		},
		{
			name:        "tenant_editor",
			description: "Tenant editor with read/write access",
			permissions: []string{"dashboard.create", "dashboard.update", "dashboard.read", "kpi.create", "kpi.update", "kpi.read"},
			isSystem:    true,
		},
		{
			name:        "tenant_guest",
			description: "Tenant guest with read-only access",
			permissions: []string{"dashboard.read", "kpi.read"},
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

	// Bootstrap in order
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"default tenant", s.BootstrapDefaultTenant},
		{"global admin user", s.BootstrapGlobalAdmin},
		{"admin auth credentials", s.BootstrapAdminAuth},
		{"default roles", s.BootstrapDefaultRoles},
	}

	for _, step := range steps {
		s.logger.Info("Running bootstrap step", "step", step.name)
		if err := step.fn(ctx); err != nil {
			return fmt.Errorf("bootstrap step %s failed: %w", step.name, err)
		}
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
	return errStr == "not found" || errStr == "entity not found" || errStr == "document not found"
}

// isAlreadyExistsError checks if an error indicates that an entity already exists
func (s *RBACBootstrapService) isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "already exists" || errStr == "entity already exists" || errStr == "duplicate key"
}
