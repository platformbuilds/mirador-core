package rbac

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
)

// RBACService provides business logic for RBAC operations
type RBACService struct {
	repository   RBACRepository
	cache        CacheRepository
	auditService *AuditService
}

// NewRBACService creates a new RBAC service
func NewRBACService(repository RBACRepository, cache CacheRepository, auditService *AuditService) *RBACService {
	return &RBACService{
		repository:   repository,
		cache:        cache,
		auditService: auditService,
	}
}

// RoleValidationError represents validation errors for roles
type RoleValidationError struct {
	Field   string
	Message string
}

func (e RoleValidationError) Error() string {
	return fmt.Sprintf("role validation error [%s]: %s", e.Field, e.Message)
}

// PermissionValidationError represents validation errors for permissions
type PermissionValidationError struct {
	Field   string
	Message string
}

func (e PermissionValidationError) Error() string {
	return fmt.Sprintf("permission validation error [%s]: %s", e.Field, e.Message)
}

// AssignmentValidationError represents validation errors for role assignments
type AssignmentValidationError struct {
	UserID string
	Role   string
	Reason string
}

func (e AssignmentValidationError) Error() string {
	return fmt.Sprintf("assignment validation error for user %s role %s: %s", e.UserID, e.Role, e.Reason)
}

// TenantValidationError represents validation errors for tenants
type TenantValidationError struct {
	Field   string
	Message string
}

func (e TenantValidationError) Error() string {
	return fmt.Sprintf("tenant validation error [%s]: %s", e.Field, e.Message)
}

// PermissionContext represents the context for permission evaluation
type PermissionContext struct {
	UserID         string
	TenantID       string
	Resource       string
	Action         string
	RequestTime    time.Time
	IPAddress      string
	UserAttributes map[string]interface{}
}

// CreateRole creates a new role with validation
func (s *RBACService) CreateRole(ctx context.Context, tenantID, userID string, role *models.Role) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("create_role", "rbac.role", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Validate role
	if err := s.validateRole(role); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role.create", "rbac.role", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Set metadata
	role.TenantID = tenantID
	role.CreatedBy = userID
	role.UpdatedBy = userID
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()

	// Check for duplicate role name
	existingRole, err := s.repository.GetRole(ctx, tenantID, role.Name)
	if err == nil && existingRole != nil {
		return RoleValidationError{Field: "name", Message: "role with this name already exists"}
	}

	// Create role in repository
	if err := s.repository.CreateRole(ctx, role); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role.create", "rbac.role", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to create role: %w", err)
	}

	// Cache the role
	cacheTTL := 30 * time.Minute
	if err := s.cache.SetRole(ctx, role.TenantID, role.Name, role, cacheTTL); err != nil {
		// Log cache failure but don't fail the operation
		monitoring.RecordCacheOperation("cache_role_failure", "error")
	}

	// Audit log the creation
	if err := s.auditService.LogRoleCreated(ctx, tenantID, userID, role, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// GetRole retrieves a role with caching
func (s *RBACService) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("get_role", "rbac.role", time.Since(start), true) }()
	// Try cache first
	cachedRole, err := s.cache.GetRole(ctx, tenantID, roleName)
	if err == nil && cachedRole != nil {
		monitoring.RecordCacheOperation("get_role", "hit")
		return cachedRole, nil
	}

	monitoring.RecordCacheOperation("get_role", "miss")

	// Get from repository
	role, err := s.repository.GetRole(ctx, tenantID, roleName)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}
	if role == nil {
		return nil, fmt.Errorf("role not found: %s", roleName)
	}

	// Cache for future requests
	cacheTTL := 30 * time.Minute
	if cacheErr := s.cache.SetRole(ctx, role.TenantID, role.Name, role, cacheTTL); cacheErr != nil {
		monitoring.RecordCacheOperation("cache_role_failure", "error")
	}

	return role, nil
}

// UpdateRole updates an existing role with validation
func (s *RBACService) UpdateRole(ctx context.Context, tenantID, userID, roleName string, updates *models.Role) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("update_role", "rbac.role", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Get existing role
	existingRole, err := s.GetRole(ctx, tenantID, roleName)
	if err != nil {
		return fmt.Errorf("failed to get existing role: %w", err)
	}

	// Validate updates
	if err := s.validateRoleUpdates(existingRole, updates); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role.update", "rbac.role", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Apply updates
	existingRole.Description = updates.Description
	existingRole.Permissions = updates.Permissions
	existingRole.ParentRoles = updates.ParentRoles
	existingRole.Metadata = updates.Metadata
	existingRole.UpdatedBy = userID
	existingRole.UpdatedAt = time.Now()

	// Update in repository
	if err := s.repository.UpdateRole(ctx, existingRole); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role.update", "rbac.role", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to update role: %w", err)
	}

	// Update cache
	cacheTTL := 30 * time.Minute
	if err := s.cache.SetRole(ctx, existingRole.TenantID, existingRole.Name, existingRole, cacheTTL); err != nil {
		monitoring.RecordCacheOperation("cache_role_failure", "error")
	}

	// Audit log the update
	if err := s.auditService.LogRoleUpdated(ctx, tenantID, userID, existingRole, existingRole, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// DeleteRole deletes a role with safety checks
func (s *RBACService) DeleteRole(ctx context.Context, tenantID, userID, roleName string) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("delete_role", "rbac.role", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Check if role is system role
	role, err := s.GetRole(ctx, tenantID, roleName)
	if err != nil {
		return fmt.Errorf("failed to get role: %w", err)
	}

	if role.IsSystem {
		return RoleValidationError{Field: "isSystem", Message: "cannot delete system roles"}
	}

	// Check if role is assigned to any users
	// This is a simplified check - in production, you'd want to check all role bindings
	users, err := s.repository.GetUserRoles(ctx, tenantID, "") // This would need to be implemented properly
	if err == nil && len(users) > 0 {
		return RoleValidationError{Field: "assignments", Message: "cannot delete role that is assigned to users"}
	}

	// Delete from repository
	if err := s.repository.DeleteRole(ctx, tenantID, roleName); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role.delete", "rbac.role", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to delete role: %w", err)
	}

	// Invalidate cache
	if err := s.cache.DeleteRole(ctx, tenantID, roleName); err != nil {
		monitoring.RecordCacheOperation("invalidate_role_cache_failure", "error")
	}

	// Audit log the deletion
	if err := s.auditService.LogRoleDeleted(ctx, tenantID, userID, roleName, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// AssignUserRoles assigns roles to a user with validation
func (s *RBACService) AssignUserRoles(ctx context.Context, tenantID, userID, targetUserID string, roleNames []string) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("assign_user_roles", "rbac.user_role", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Validate role assignments
	if err := s.validateRoleAssignments(ctx, tenantID, targetUserID, roleNames); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "user.role.assign", "rbac.user_role", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Assign roles in repository
	if err := s.repository.AssignUserRoles(ctx, tenantID, targetUserID, roleNames); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "user.role.assign", "rbac.user_role", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to assign user roles: %w", err)
	}

	// Update cache
	cacheTTL := 15 * time.Minute
	if err := s.cache.SetUserRoles(ctx, tenantID, targetUserID, roleNames, cacheTTL); err != nil {
		monitoring.RecordCacheOperation("cache_user_roles_failure", "error")
	}

	// Audit log the assignment
	if err := s.auditService.LogUserRoleAssigned(ctx, tenantID, userID, targetUserID, roleNames, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// RemoveUserRoles removes roles from a user
func (s *RBACService) RemoveUserRoles(ctx context.Context, tenantID, userID, targetUserID string, roleNames []string) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("remove_user_roles", "rbac.user_role", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Remove roles from repository
	if err := s.repository.RemoveUserRoles(ctx, tenantID, targetUserID, roleNames); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "user.role.remove", "rbac.user_role", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to remove user roles: %w", err)
	}

	// Get remaining roles and update cache
	remainingRoles, err := s.repository.GetUserRoles(ctx, tenantID, targetUserID)
	if err == nil {
		cacheTTL := 15 * time.Minute
		if cacheErr := s.cache.SetUserRoles(ctx, tenantID, targetUserID, remainingRoles, cacheTTL); cacheErr != nil {
			monitoring.RecordCacheOperation("cache_user_roles_failure", "error")
		}
	}

	// Audit log the removal
	if err := s.auditService.LogUserRoleRemoved(ctx, tenantID, userID, targetUserID, roleNames, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// GetUserRoles retrieves user roles with caching
func (s *RBACService) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("get_user_roles", "rbac.user_role", time.Since(start), true) }()
	// Try cache first
	cachedRoles, err := s.cache.GetUserRoles(ctx, tenantID, userID)
	if err == nil && cachedRoles != nil {
		monitoring.RecordCacheOperation("get_user_roles", "hit")
		return cachedRoles, nil
	}

	monitoring.RecordCacheOperation("get_user_roles", "miss")

	// Get from repository
	roles, err := s.repository.GetUserRoles(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	// Cache for future requests
	cacheTTL := 15 * time.Minute
	if cacheErr := s.cache.SetUserRoles(ctx, tenantID, userID, roles, cacheTTL); cacheErr != nil {
		monitoring.RecordCacheOperation("cache_user_roles_failure", "error")
	}

	return roles, nil
}

// CreateTenant creates a new tenant with validation
func (s *RBACService) CreateTenant(ctx context.Context, userID string, tenant *models.Tenant) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("create_tenant", "rbac.tenant", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Validate tenant
	if err := s.validateTenant(tenant); err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "tenant.create", "rbac.tenant", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Set metadata
	tenant.CreatedBy = userID
	tenant.UpdatedBy = userID
	tenant.CreatedAt = time.Now()
	tenant.UpdatedAt = time.Now()

	// Check for duplicate tenant name
	existingTenants, err := s.repository.ListTenants(ctx, TenantFilters{Name: &tenant.Name})
	if err == nil && len(existingTenants) > 0 {
		return TenantValidationError{Field: "name", Message: "tenant with this name already exists"}
	}

	// Create tenant in repository
	if err := s.repository.CreateTenant(ctx, tenant); err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "tenant.create", "rbac.tenant", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	// Audit log the creation
	details := map[string]interface{}{
		"tenant_name": tenant.Name,
		"admin_email": tenant.AdminEmail,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenant.ID, "create", "tenant", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// GetTenant retrieves a tenant by ID
func (s *RBACService) GetTenant(ctx context.Context, userID, tenantID string) (*models.Tenant, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("get_tenant", "rbac.tenant", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	tenant, err := s.repository.GetTenant(ctx, tenantID)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "tenant.read", "rbac.tenant", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Audit log the read
	details := map[string]interface{}{
		"tenant_id": tenantID,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "read", "tenant", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return tenant, nil
}

// ListTenants lists tenants with filters
func (s *RBACService) ListTenants(ctx context.Context, userID string, filters TenantFilters) ([]*models.Tenant, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("list_tenants", "rbac.tenant", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	tenants, err := s.repository.ListTenants(ctx, filters)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "tenant.list", "rbac.tenant", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	// Audit log the list operation
	details := map[string]interface{}{
		"filters": filters,
	}
	if err := s.auditService.LogSystemEvent(ctx, "", "list", "tenant", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return tenants, nil
}

// UpdateTenant updates an existing tenant
func (s *RBACService) UpdateTenant(ctx context.Context, userID string, tenant *models.Tenant) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("update_tenant", "rbac.tenant", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Validate tenant
	if err := s.validateTenant(tenant); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenant.ID, userID, "tenant.update", "rbac.tenant", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Set update metadata
	tenant.UpdatedBy = userID
	tenant.UpdatedAt = time.Now()

	// Update tenant in repository
	if err := s.repository.UpdateTenant(ctx, tenant); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenant.ID, userID, "tenant.update", "rbac.tenant", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	// Audit log the update
	details := map[string]interface{}{
		"tenant_id":   tenant.ID,
		"tenant_name": tenant.Name,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenant.ID, "update", "tenant", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// DeleteTenant deletes a tenant
func (s *RBACService) DeleteTenant(ctx context.Context, userID, tenantID string) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("delete_tenant", "rbac.tenant", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Check if tenant exists and is a system tenant
	tenant, err := s.repository.GetTenant(ctx, tenantID)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "tenant.delete", "rbac.tenant", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	// Prevent deletion of system tenants (e.g., default PLATFORMBUILDS tenant)
	if tenant.IsSystem {
		return TenantValidationError{Field: "isSystem", Message: "system tenants cannot be deleted, only renamed"}
	}

	// Delete tenant from repository
	if err := s.repository.DeleteTenant(ctx, tenantID); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "tenant.delete", "rbac.tenant", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	// Audit log the deletion
	details := map[string]interface{}{
		"tenant_id": tenantID,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "delete", "tenant", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// CreateTenantUser creates a new tenant-user association
func (s *RBACService) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser, correlationID string) (*models.TenantUser, error) {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("create_tenant_user", "rbac.tenant_user", time.Since(start), true)
	}()

	// Validate tenant-user data
	if err := s.validateTenantUser(tenantUser); err != nil {
		return nil, err
	}

	// Check if tenant exists
	_, err := s.repository.GetTenant(ctx, tenantUser.TenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant does not exist: %w", err)
	}

	// Check if user exists (this would typically be checked against a user service)
	// For now, we'll assume the user exists if provided

	// Check if association already exists
	existing, err := s.repository.GetTenantUser(ctx, tenantUser.TenantID, tenantUser.UserID)
	if err == nil && existing != nil {
		return nil, TenantUserValidationError{Field: "userId", Message: "user is already associated with this tenant"}
	}

	// Set timestamps
	now := time.Now()
	tenantUser.CreatedAt = now
	tenantUser.UpdatedAt = now

	// Create the association
	err = s.repository.CreateTenantUser(ctx, tenantUser)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantUser.TenantID, tenantUser.CreatedBy, "create", "tenant_user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return nil, fmt.Errorf("failed to create tenant-user association: %w", err)
	}

	// Audit log the creation
	details := map[string]interface{}{
		"tenant_id":   tenantUser.TenantID,
		"user_id":     tenantUser.UserID,
		"tenant_role": tenantUser.TenantRole,
		"status":      tenantUser.Status,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantUser.TenantID, "create", "tenant_user", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return tenantUser, nil
}

// GetTenantUser retrieves a tenant-user association
func (s *RBACService) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("get_tenant_user", "rbac.tenant_user", time.Since(start), true) }()

	tenantUser, err := s.repository.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant-user association: %w", err)
	}

	return tenantUser, nil
}

// ListTenantUsers retrieves all users for a tenant with optional filtering
func (s *RBACService) ListTenantUsers(ctx context.Context, tenantID string, filters *TenantUserFilters) ([]*models.TenantUser, error) {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("list_tenant_users", "rbac.tenant_user", time.Since(start), true)
	}()

	// Check if tenant exists
	_, err := s.repository.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant does not exist: %w", err)
	}

	// Convert filters to the interface type
	var repoFilters TenantUserFilters
	if filters != nil {
		repoFilters = TenantUserFilters{
			UserID:     filters.UserID,
			TenantRole: filters.TenantRole,
			Status:     filters.Status,
			Limit:      filters.Limit,
			Offset:     filters.Offset,
		}
	}

	tenantUsers, err := s.repository.ListTenantUsers(ctx, tenantID, repoFilters)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenant users: %w", err)
	}

	return tenantUsers, nil
}

// UpdateTenantUser updates a tenant-user association
func (s *RBACService) UpdateTenantUser(ctx context.Context, tenantID, userID string, updates *models.TenantUser, correlationID string) (*models.TenantUser, error) {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("update_tenant_user", "rbac.tenant_user", time.Since(start), true)
	}()

	// Get existing association
	existing, err := s.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("tenant-user association not found: %w", err)
	}

	// Validate updates
	if err := s.validateTenantUserUpdates(existing, updates); err != nil {
		return nil, err
	}

	// Merge updates with existing data
	updatedTenantUser := *existing
	if updates.TenantRole != "" {
		updatedTenantUser.TenantRole = updates.TenantRole
	}
	if updates.Status != "" {
		updatedTenantUser.Status = updates.Status
	}
	if updates.AdditionalPermissions != nil {
		updatedTenantUser.AdditionalPermissions = updates.AdditionalPermissions
	}
	if updates.Metadata != nil {
		updatedTenantUser.Metadata = updates.Metadata
	}
	if updates.UpdatedBy != "" {
		updatedTenantUser.UpdatedBy = updates.UpdatedBy
	}
	updatedTenantUser.UpdatedAt = time.Now()

	// Update the association
	err = s.repository.UpdateTenantUser(ctx, &updatedTenantUser)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, updates.UpdatedBy, "update", "tenant_user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return nil, fmt.Errorf("failed to update tenant-user association: %w", err)
	}

	// Audit log the update
	details := map[string]interface{}{
		"tenant_id":  tenantID,
		"user_id":    userID,
		"old_status": existing.Status,
		"new_status": updatedTenantUser.Status,
		"old_role":   existing.TenantRole,
		"new_role":   updatedTenantUser.TenantRole,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "update", "tenant_user", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return &updatedTenantUser, nil
}

// DeleteTenantUser removes a tenant-user association
func (s *RBACService) DeleteTenantUser(ctx context.Context, tenantID, userID string, correlationID string) error {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("delete_tenant_user", "rbac.tenant_user", time.Since(start), true)
	}()

	// Get existing association for audit logging
	existing, err := s.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		return fmt.Errorf("tenant-user association not found: %w", err)
	}

	// Delete the association
	err = s.repository.DeleteTenantUser(ctx, tenantID, userID)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, existing.UpdatedBy, "delete", "tenant_user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to delete tenant-user association: %w", err)
	}

	// Audit log the deletion
	details := map[string]interface{}{
		"tenant_id":   tenantID,
		"user_id":     userID,
		"tenant_role": existing.TenantRole,
		"status":      existing.Status,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "delete", "tenant_user", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// validateRole validates role data
func (s *RBACService) validateRole(role *models.Role) error {
	if strings.TrimSpace(role.Name) == "" {
		return RoleValidationError{Field: "name", Message: "role name cannot be empty"}
	}

	if len(role.Name) > 100 {
		return RoleValidationError{Field: "name", Message: "role name cannot exceed 100 characters"}
	}

	if len(role.Description) > 500 {
		return RoleValidationError{Field: "description", Message: "role description cannot exceed 500 characters"}
	}

	// Validate permissions format
	for _, perm := range role.Permissions {
		if !isValidPermissionFormat(perm) {
			return RoleValidationError{Field: "permissions", Message: fmt.Sprintf("invalid permission format: %s", perm)}
		}
	}

	return nil
}

// validateRoleUpdates validates role update data
func (s *RBACService) validateRoleUpdates(existing, updates *models.Role) error {
	// System roles cannot be modified
	if existing.IsSystem {
		return RoleValidationError{Field: "isSystem", Message: "system roles cannot be modified"}
	}

	return s.validateRole(updates)
}

// validateRoleAssignments validates role assignments
func (s *RBACService) validateRoleAssignments(ctx context.Context, tenantID, userID string, roleNames []string) error {
	for _, roleName := range roleNames {
		// Check if role exists
		role, err := s.GetRole(ctx, tenantID, roleName)
		if err != nil {
			return AssignmentValidationError{UserID: userID, Role: roleName, Reason: "role does not exist"}
		}

		// Check for circular dependencies in parent roles
		if s.hasCircularDependency(ctx, tenantID, role, make(map[string]bool)) {
			return AssignmentValidationError{UserID: userID, Role: roleName, Reason: "role has circular dependency"}
		}
	}

	return nil
}

// validateTenant validates tenant data
func (s *RBACService) validateTenant(tenant *models.Tenant) error {
	if tenant.Name == "" {
		return TenantValidationError{Field: "name", Message: "tenant name is required"}
	}

	if len(tenant.Name) < 3 || len(tenant.Name) > 50 {
		return TenantValidationError{Field: "name", Message: "tenant name must be between 3 and 50 characters"}
	}

	// Validate name format (alphanumeric, spaces, hyphens, underscores)
	validName := regexp.MustCompile(`^[a-zA-Z0-9 _-]+$`)
	if !validName.MatchString(tenant.Name) {
		return TenantValidationError{Field: "name", Message: "tenant name can only contain letters, numbers, spaces, hyphens, and underscores"}
	}

	if tenant.AdminEmail == "" {
		return TenantValidationError{Field: "adminEmail", Message: "admin email is required"}
	}

	// Basic email validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(tenant.AdminEmail) {
		return TenantValidationError{Field: "adminEmail", Message: "invalid email format"}
	}

	if tenant.Status != "" && tenant.Status != "active" && tenant.Status != "suspended" && tenant.Status != "pending_deletion" {
		return TenantValidationError{Field: "status", Message: "status must be one of: active, suspended, pending_deletion"}
	}

	return nil
}

// CheckPermission evaluates if a user has permission for an action on a resource with constraints
func (s *RBACService) CheckPermission(ctx context.Context, tenantID, userID, resource, action string) (bool, error) {
	return s.CheckPermissionWithContext(ctx, &PermissionContext{
		UserID:      userID,
		TenantID:    tenantID,
		Resource:    resource,
		Action:      action,
		RequestTime: time.Now(),
	})
}

// CheckPermissionWithContext evaluates permission with full context for constraint-based access control
func (s *RBACService) CheckPermissionWithContext(ctx context.Context, permCtx *PermissionContext) (bool, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("check_permission", "rbac.permission", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Step 1: Check Global Role (highest precedence)
	user, err := s.repository.GetUser(ctx, permCtx.UserID)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, permCtx.TenantID, permCtx.UserID, "permission.check", permCtx.Resource, err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		if auditErr := s.auditService.LogAccessDenied(ctx, permCtx.TenantID, permCtx.UserID, permCtx.Resource, permCtx.Action, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return false, fmt.Errorf("user not found")
	}

	// Global admin has all permissions
	if user.GlobalRole == "global_admin" {
		if err := s.auditService.LogPermissionCheck(ctx, permCtx.TenantID, permCtx.UserID, permCtx.Resource, permCtx.Action, true, correlationID); err != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return true, nil
	}

	// Global tenant admin has admin permissions within their managed tenants
	if user.GlobalRole == "global_tenant_admin" {
		if isAdminAction(permCtx.Resource, permCtx.Action) {
			if err := s.auditService.LogPermissionCheck(ctx, permCtx.TenantID, permCtx.UserID, permCtx.Resource, permCtx.Action, true, correlationID); err != nil {
				monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
			}
			return true, nil
		}
	}

	// Step 2: Check Tenant Role permissions
	tenantUser, err := s.repository.GetTenantUser(ctx, permCtx.TenantID, permCtx.UserID)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, permCtx.TenantID, permCtx.UserID, "permission.check", permCtx.Resource, err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return false, fmt.Errorf("failed to get tenant user association: %w", err)
	}

	if tenantUser == nil || tenantUser.Status != "active" {
		if auditErr := s.auditService.LogAccessDenied(ctx, permCtx.TenantID, permCtx.UserID, permCtx.Resource, permCtx.Action, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return false, nil // User not associated with tenant or not active
	}

	// Evaluate permissions with constraints based on tenant role
	allowed, denied := s.evaluatePermissionsWithConstraints(ctx, permCtx, []string{tenantUser.TenantRole})

	// Audit log the check
	if allowed && !denied {
		if err := s.auditService.LogPermissionCheck(ctx, permCtx.TenantID, permCtx.UserID, permCtx.Resource, permCtx.Action, true, correlationID); err != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
	} else {
		if err := s.auditService.LogAccessDenied(ctx, permCtx.TenantID, permCtx.UserID, permCtx.Resource, permCtx.Action, correlationID); err != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
	}

	return allowed && !denied, nil
}

// ListRoles retrieves all roles for a tenant with caching
func (s *RBACService) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("list_roles", "rbac.role", time.Since(start), true) }()

	// Get from repository
	roles, err := s.repository.ListRoles(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	return roles, nil
}

// evaluatePermissionsWithConstraints evaluates permissions with ABAC constraints
func (s *RBACService) evaluatePermissionsWithConstraints(ctx context.Context, permCtx *PermissionContext, userRoles []string) (allowed, denied bool) {
	// Get all permissions from user's roles (including group memberships)
	allPermissions := s.collectAllPermissions(ctx, permCtx)

	// Evaluate each permission against the requested action
	for _, perm := range allPermissions {
		if s.permissionMatches(permCtx, perm) {
			// Check constraints
			if s.evaluateConstraints(permCtx, perm.Conditions) {
				allowed = true
				break // Found a matching permission, allow access
			}
		}
	}

	return allowed, false // No deny logic implemented yet
}

// collectAllPermissions collects all permissions from roles and groups
func (s *RBACService) collectAllPermissions(ctx context.Context, permCtx *PermissionContext) []*models.Permission {
	allPermissions := make([]*models.Permission, 0)

	// Get user roles
	userRoles, err := s.GetUserRoles(ctx, permCtx.TenantID, permCtx.UserID)
	if err != nil {
		return allPermissions
	}

	// Get permissions from roles
	for _, roleName := range userRoles {
		role, err := s.GetRole(ctx, permCtx.TenantID, roleName)
		if err != nil {
			continue
		}

		// Add role permissions
		rolePerms := s.getRolePermissions(ctx, permCtx.TenantID, role)
		allPermissions = append(allPermissions, rolePerms...)
	}

	// Add group permissions
	userGroups, err := s.repository.GetUserGroups(ctx, permCtx.TenantID, permCtx.UserID)
	if err == nil {
		for _, groupName := range userGroups {
			group, err := s.repository.GetGroup(ctx, permCtx.TenantID, groupName)
			if err != nil {
				continue
			}
			groupPerms := s.getGroupPermissions(ctx, permCtx.TenantID, group)
			allPermissions = append(allPermissions, groupPerms...)
		}
	}

	return allPermissions
}

// getRolePermissions recursively gets all permissions from a role and its parents
func (s *RBACService) getRolePermissions(ctx context.Context, tenantID string, role *models.Role) []*models.Permission {
	permissions := make([]*models.Permission, 0)

	// Add direct permissions
	for _, permID := range role.Permissions {
		perm, err := s.repository.GetPermission(ctx, tenantID, permID)
		if err == nil {
			permissions = append(permissions, perm)
		}
	}

	// Add permissions from parent roles
	for _, parentRoleName := range role.ParentRoles {
		parentRole, err := s.GetRole(ctx, tenantID, parentRoleName)
		if err != nil {
			continue
		}
		parentPerms := s.getRolePermissions(ctx, tenantID, parentRole)
		permissions = append(permissions, parentPerms...)
	}

	return permissions
}

// getGroupPermissions recursively gets all permissions from a group and its parents
func (s *RBACService) getGroupPermissions(ctx context.Context, tenantID string, group *models.Group) []*models.Permission {
	permissions := make([]*models.Permission, 0)

	// Add permissions from roles assigned to this group
	for _, roleID := range group.Roles {
		role, err := s.GetRole(ctx, tenantID, roleID)
		if err != nil {
			continue
		}
		rolePerms := s.getRolePermissions(ctx, tenantID, role)
		permissions = append(permissions, rolePerms...)
	}

	// Add permissions from parent groups
	for _, parentGroupName := range group.ParentGroups {
		parentGroup, err := s.repository.GetGroup(ctx, tenantID, parentGroupName)
		if err != nil {
			continue
		}
		parentPerms := s.getGroupPermissions(ctx, tenantID, parentGroup)
		permissions = append(permissions, parentPerms...)
	}

	return permissions
}

// permissionMatches checks if a permission applies to the requested resource and action
func (s *RBACService) permissionMatches(permCtx *PermissionContext, perm *models.Permission) bool {
	// Check action match
	if perm.Action != "*" && perm.Action != permCtx.Action {
		return false
	}

	// Check resource match using pattern matching
	if !s.resourceMatches(permCtx.Resource, perm.Resource, perm.ResourcePattern) {
		return false
	}

	// Check scope
	if !s.scopeMatches(permCtx.TenantID, perm.Scope) {
		return false
	}

	return true
}

// scopeMatches checks if the permission scope allows access for the given tenant
func (s *RBACService) scopeMatches(tenantID, permScope string) bool {
	switch permScope {
	case "global":
		// Global permissions apply to all tenants
		return true
	case "tenant":
		// Tenant-scoped permissions apply to any specific tenant
		return tenantID != ""
	default:
		// Resource-specific scope - check if it matches the tenant
		return permScope == tenantID
	}
}

// resourceMatches checks if a resource matches the permission's resource pattern
func (s *RBACService) resourceMatches(requestResource, permResource, resourcePattern string) bool {
	// Exact match
	if permResource == requestResource {
		return true
	}

	// Pattern matching (simple wildcard support)
	if resourcePattern != "" {
		return s.matchResourcePattern(requestResource, resourcePattern)
	}

	// Prefix matching for hierarchical resources
	if strings.HasPrefix(requestResource, permResource+"/") {
		return true
	}

	return false
}

// matchResourcePattern implements simple pattern matching for resources
func (s *RBACService) matchResourcePattern(resource, pattern string) bool {
	// Simple wildcard matching (*)
	// Convert pattern to regex
	regexPattern := strings.ReplaceAll(pattern, "*", ".*")
	matched, _ := regexp.MatchString("^"+regexPattern+"$", resource)
	return matched
}

// evaluateConstraints evaluates ABAC conditions for a permission
func (s *RBACService) evaluateConstraints(permCtx *PermissionContext, conditions models.PermissionConditions) bool {
	// Time-based conditions
	if !s.evaluateTimeConditions(permCtx.RequestTime, conditions.TimeBased) {
		return false
	}

	// IP-based conditions
	if !s.evaluateIPConditions(permCtx.IPAddress, conditions.IPBased) {
		return false
	}

	// Attribute-based conditions
	if !s.evaluateAttributeConditions(permCtx.UserAttributes, conditions.AttributeBased) {
		return false
	}

	return true
}

// evaluateTimeConditions checks time-based access restrictions
func (s *RBACService) evaluateTimeConditions(requestTime time.Time, timeCond models.TimeBasedCondition) bool {
	// If no time conditions, allow
	if len(timeCond.AllowedHours) == 0 && len(timeCond.AllowedDays) == 0 {
		return true
	}

	// Check allowed days
	if len(timeCond.AllowedDays) > 0 {
		dayName := strings.ToLower(requestTime.Weekday().String())
		allowed := false
		for _, allowedDay := range timeCond.AllowedDays {
			if strings.EqualFold(allowedDay, dayName) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Check allowed hours
	if len(timeCond.AllowedHours) > 0 {
		currentTime := requestTime.Format("15:04")
		allowed := false
		for _, timeRange := range timeCond.AllowedHours {
			if s.timeInRange(currentTime, timeRange) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	return true
}

// evaluateIPConditions checks IP-based access restrictions
func (s *RBACService) evaluateIPConditions(clientIP string, allowedIPs []string) bool {
	// If no IP restrictions, allow
	if len(allowedIPs) == 0 {
		return true
	}

	// Check if client IP is in allowed list (supports CIDR notation)
	for _, allowedIP := range allowedIPs {
		if s.ipMatches(clientIP, allowedIP) {
			return true
		}
	}

	return false
}

// evaluateAttributeConditions checks user attribute requirements
func (s *RBACService) evaluateAttributeConditions(userAttrs map[string]interface{}, attrCond models.AttributeBasedCondition) bool {
	// Department check
	if len(attrCond.Department) > 0 {
		userDept, exists := userAttrs["department"]
		if !exists {
			return false
		}
		deptStr, ok := userDept.(string)
		if !ok {
			return false
		}
		allowed := false
		for _, allowedDept := range attrCond.Department {
			if allowedDept == deptStr {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Clearance level check
	if attrCond.ClearanceLevel != "" {
		userClearance, exists := userAttrs["clearance_level"]
		if !exists {
			return false
		}
		clearanceStr, ok := userClearance.(string)
		if !ok {
			return false
		}
		if !s.clearanceLevelSufficient(clearanceStr, attrCond.ClearanceLevel) {
			return false
		}
	}

	return true
}

// timeInRange checks if a time string is within a time range (HH:MM-HH:MM)
func (s *RBACService) timeInRange(timeStr, timeRange string) bool {
	parts := strings.Split(timeRange, "-")
	if len(parts) != 2 {
		return false
	}

	startTime := strings.TrimSpace(parts[0])
	endTime := strings.TrimSpace(parts[1])

	// Simple string comparison for HH:MM format
	return timeStr >= startTime && timeStr <= endTime
}

// ipMatches checks if an IP address matches a pattern (supports CIDR)
func (s *RBACService) ipMatches(clientIP, pattern string) bool {
	// Simple implementation - in production, use proper IP parsing
	if strings.Contains(pattern, "/") {
		// CIDR notation - simplified check
		parts := strings.Split(pattern, "/")
		if len(parts) == 2 {
			// For now, just check if IP starts with network prefix
			return strings.HasPrefix(clientIP, strings.TrimSuffix(parts[0], ".0"))
		}
	}
	return clientIP == pattern
}

// clearanceLevelSufficient checks if user clearance meets required level
func (s *RBACService) clearanceLevelSufficient(userLevel, requiredLevel string) bool {
	// Define clearance hierarchy (higher number = higher clearance)
	clearanceLevels := map[string]int{
		"public":       1,
		"internal":     2,
		"confidential": 3,
		"secret":       4,
		"top_secret":   5,
	}

	userLvl, userExists := clearanceLevels[strings.ToLower(userLevel)]
	reqLvl, reqExists := clearanceLevels[strings.ToLower(requiredLevel)]

	if !userExists || !reqExists {
		return false
	}

	return userLvl >= reqLvl
}

// hasCircularDependency checks for circular dependencies in role hierarchy
func (s *RBACService) hasCircularDependency(ctx context.Context, tenantID string, role *models.Role, visited map[string]bool) bool {
	if visited[role.Name] {
		return true
	}

	visited[role.Name] = true
	defer delete(visited, role.Name)

	for _, parentName := range role.ParentRoles {
		parentRole, err := s.GetRole(ctx, tenantID, parentName)
		if err != nil {
			continue
		}

		if s.hasCircularDependency(ctx, tenantID, parentRole, visited) {
			return true
		}
	}

	return false
}

// isValidPermissionFormat validates permission string format
func isValidPermissionFormat(permission string) bool {
	// Expected format: "resource:action" or "resource:action:scope"
	parts := strings.Split(permission, ":")
	return len(parts) >= 2 && len(parts) <= 3
}

// isAdminAction checks if the requested action is an administrative action
func isAdminAction(resource, action string) bool {
	// Admin actions that global_tenant_admin can perform
	adminActions := map[string][]string{
		"admin":          {"*"},
		"rbac":           {"*"},
		"tenant":         {"admin", "update"},
		"user":           {"admin", "create", "update", "delete", "list"},
		"dashboard":      {"admin"},
		"kpi_definition": {"admin"},
		"layout":         {"admin"},
	}

	if actions, exists := adminActions[resource]; exists {
		for _, adminAction := range actions {
			if adminAction == "*" || adminAction == action {
				return true
			}
		}
	}

	return false
}

// generateCorrelationID generates a unique correlation ID for audit logging
func generateCorrelationID() string {
	return fmt.Sprintf("rbac-%d", time.Now().UnixNano())
}

// validateTenantUser validates tenant-user data
func (s *RBACService) validateTenantUser(tenantUser *models.TenantUser) error {
	if tenantUser.TenantID == "" {
		return TenantUserValidationError{Field: "tenantId", Message: "tenant ID is required"}
	}

	if tenantUser.UserID == "" {
		return TenantUserValidationError{Field: "userId", Message: "user ID is required"}
	}

	if tenantUser.TenantRole == "" {
		return TenantUserValidationError{Field: "tenantRole", Message: "tenant role is required"}
	}

	// Validate tenant role
	validRoles := []string{"tenant_admin", "tenant_editor", "tenant_guest"}
	validRole := false
	for _, role := range validRoles {
		if tenantUser.TenantRole == role {
			validRole = true
			break
		}
	}
	if !validRole {
		return TenantUserValidationError{Field: "tenantRole", Message: "tenant role must be one of: tenant_admin, tenant_editor, tenant_guest"}
	}

	if tenantUser.Status == "" {
		tenantUser.Status = "active"
	}

	// Validate status
	validStatuses := []string{"active", "invited", "suspended", "removed"}
	validStatus := false
	for _, status := range validStatuses {
		if tenantUser.Status == status {
			validStatus = true
			break
		}
	}
	if !validStatus {
		return TenantUserValidationError{Field: "status", Message: "status must be one of: active, invited, suspended, removed"}
	}

	return nil
}

// validateTenantUserUpdates validates tenant-user update data
func (s *RBACService) validateTenantUserUpdates(existing, updates *models.TenantUser) error {
	// Tenant and user IDs cannot be changed
	if updates.TenantID != "" && updates.TenantID != existing.TenantID {
		return TenantUserValidationError{Field: "tenantId", Message: "tenant ID cannot be changed"}
	}

	if updates.UserID != "" && updates.UserID != existing.UserID {
		return TenantUserValidationError{Field: "userId", Message: "user ID cannot be changed"}
	}

	return s.validateTenantUser(updates)
}

// CountGlobalAdmins returns the number of active global admin users
func (s *RBACService) CountGlobalAdmins(ctx context.Context) (int, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("count_global_admins", "rbac.user", time.Since(start), true) }()

	globalAdminRole := "global_admin"
	activeStatus := "active"
	users, err := s.repository.ListUsers(ctx, UserFilters{GlobalRole: &globalAdminRole, Status: &activeStatus})
	if err != nil {
		return 0, fmt.Errorf("failed to count global admins: %w", err)
	}

	return len(users), nil
}

// CreateUser creates a new global user
func (s *RBACService) CreateUser(ctx context.Context, userID string, user *models.User) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("create_user", "rbac.user", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Validate user
	if err := s.validateUser(user); err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "user.create", "rbac.user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Set metadata
	user.CreatedBy = userID
	user.UpdatedBy = userID
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	// Check for duplicate user email
	existingUsers, err := s.repository.ListUsers(ctx, UserFilters{Email: &user.Email})
	if err == nil && len(existingUsers) > 0 {
		return UserValidationError{Field: "email", Message: "user with this email already exists"}
	}

	// Create user in repository
	if err := s.repository.CreateUser(ctx, user); err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "user.create", "rbac.user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Audit log the creation
	if err := s.auditService.LogSystemEvent(ctx, "", "create", "user", map[string]interface{}{
		"user_id": user.ID,
		"email":   user.Email,
	}, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	// Automatically associate new user with default tenant as guest
	// Skip for system-created users (bootstrap process handles this explicitly)
	if userID != "system" {
		defaultTenantName := "platformbuilds"
		tenants, err := s.repository.ListTenants(ctx, TenantFilters{Name: &defaultTenantName})
		if err == nil && len(tenants) > 0 {
			defaultTenantID := tenants[0].ID

			// Create tenant-user association with guest role
			tenantUserID := fmt.Sprintf("tenant_user_%s_%s", defaultTenantID, user.ID)
			tenantUser := &models.TenantUser{
				ID:         tenantUserID,
				TenantID:   defaultTenantID,
				UserID:     user.ID,
				TenantRole: "tenant_guest",
				Status:     "active",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				CreatedBy:  userID,
				UpdatedBy:  userID,
			}

			// Use repository directly to avoid service validation during user creation
			if err := s.repository.CreateTenantUser(ctx, tenantUser); err != nil {
				// Log error but don't fail user creation
				if auditErr := s.auditService.LogError(ctx, defaultTenantID, userID, "tenant_user.create", "rbac.tenant_user", err, correlationID); auditErr != nil {
					monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
				}
				// Continue - user creation succeeded even if tenant association failed
			} else {
				// Audit log the tenant-user association creation
				if err := s.auditService.LogSystemEvent(ctx, defaultTenantID, "create", "tenant_user", map[string]interface{}{
					"tenant_id":   defaultTenantID,
					"user_id":     user.ID,
					"tenant_role": "tenant_guest",
					"status":      "active",
				}, correlationID); err != nil {
					monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
				}
			}
		} else {
			// Log warning if default tenant not found, but don't fail user creation
			if auditErr := s.auditService.LogError(ctx, "", userID, "default_tenant.not_found", "rbac.tenant", fmt.Errorf("default tenant platformbuilds not found"), correlationID); auditErr != nil {
				monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
			}
		}
	}

	return nil
}

// GetUser retrieves a user by ID
func (s *RBACService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("get_user", "rbac.user", time.Since(start), true) }()

	user, err := s.repository.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// ListUsers lists users with filters
func (s *RBACService) ListUsers(ctx context.Context, filters UserFilters) ([]*models.User, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("list_users", "rbac.user", time.Since(start), true) }()

	users, err := s.repository.ListUsers(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// UpdateUser updates an existing user
func (s *RBACService) UpdateUser(ctx context.Context, userID string, user *models.User) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("update_user", "rbac.user", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Get existing user to check current global role
	existingUser, err := s.GetUser(ctx, user.ID)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "user.update", "rbac.user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to get existing user: %w", err)
	}

	// Prevent demoting the last global admin
	if existingUser.GlobalRole == "global_admin" && user.GlobalRole != "global_admin" {
		globalAdminCount, err := s.CountGlobalAdmins(ctx)
		if err != nil {
			if auditErr := s.auditService.LogError(ctx, "", userID, "user.update", "rbac.user", err, correlationID); auditErr != nil {
				monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
			}
			return fmt.Errorf("failed to count global admins: %w", err)
		}

		if globalAdminCount <= 1 {
			return UserValidationError{Field: "globalRole", Message: "cannot demote the last global admin user"}
		}
	}

	// Validate user
	if err := s.validateUser(user); err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "user.update", "rbac.user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Set update metadata
	user.UpdatedBy = userID
	user.UpdatedAt = time.Now()

	// Update user in repository
	if err := s.repository.UpdateUser(ctx, user); err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "user.update", "rbac.user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Audit log the update
	if err := s.auditService.LogSystemEvent(ctx, "", "update", "user", map[string]interface{}{
		"user_id": user.ID,
		"email":   user.Email,
	}, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// DeleteUser deletes a user
func (s *RBACService) DeleteUser(ctx context.Context, userID, targetUserID string) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("delete_user", "rbac.user", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Get the target user to check if they are a global admin
	targetUser, err := s.GetUser(ctx, targetUserID)
	if err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "user.delete", "rbac.user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to get target user: %w", err)
	}

	// Prevent deleting the last global admin
	if targetUser.GlobalRole == "global_admin" {
		globalAdminCount, err := s.CountGlobalAdmins(ctx)
		if err != nil {
			if auditErr := s.auditService.LogError(ctx, "", userID, "user.delete", "rbac.user", err, correlationID); auditErr != nil {
				monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
			}
			return fmt.Errorf("failed to count global admins: %w", err)
		}

		if globalAdminCount <= 1 {
			return UserValidationError{Field: "globalRole", Message: "cannot delete the last global admin user"}
		}
	}

	// Delete user from repository
	if err := s.repository.DeleteUser(ctx, targetUserID); err != nil {
		if auditErr := s.auditService.LogError(ctx, "", userID, "user.delete", "rbac.user", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Audit log the deletion
	if err := s.auditService.LogSystemEvent(ctx, "", "delete", "user", map[string]interface{}{
		"user_id": targetUserID,
	}, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// UserValidationError represents validation errors for users
type UserValidationError struct {
	Field   string
	Message string
}

func (e UserValidationError) Error() string {
	return fmt.Sprintf("user validation error [%s]: %s", e.Field, e.Message)
}

// TenantUserValidationError represents tenant-user validation errors
type TenantUserValidationError struct {
	Field   string
	Message string
}

func (e TenantUserValidationError) Error() string {
	return fmt.Sprintf("tenant-user validation error on field '%s': %s", e.Field, e.Message)
}

// validateUser validates user data
func (s *RBACService) validateUser(user *models.User) error {
	if strings.TrimSpace(user.ID) == "" {
		return UserValidationError{Field: "id", Message: "user ID cannot be empty"}
	}

	if strings.TrimSpace(user.Email) == "" {
		return UserValidationError{Field: "email", Message: "email cannot be empty"}
	}

	// Basic email validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(user.Email) {
		return UserValidationError{Field: "email", Message: "invalid email format"}
	}

	if len(user.Email) > 254 {
		return UserValidationError{Field: "email", Message: "email cannot exceed 254 characters"}
	}

	return nil
}

// CreatePermission creates a new permission with validation
func (s *RBACService) CreatePermission(ctx context.Context, tenantID, userID string, permission *models.Permission) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("create_permission", "rbac.permission", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Validate permission
	if err := s.validatePermission(permission); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "permission.create", "rbac.permission", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Set metadata
	permission.TenantID = tenantID
	permission.CreatedBy = userID
	permission.UpdatedBy = userID
	permission.CreatedAt = time.Now()
	permission.UpdatedAt = time.Now()

	// Check for duplicate permission ID
	existingPermission, err := s.repository.GetPermission(ctx, tenantID, permission.ID)
	if err == nil && existingPermission != nil {
		return PermissionValidationError{Field: "id", Message: "permission with this ID already exists"}
	}

	// Create permission in repository
	if err := s.repository.CreatePermission(ctx, permission); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "permission.create", "rbac.permission", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to create permission: %w", err)
	}

	// Invalidate permissions cache
	if err := s.cache.InvalidatePermissions(ctx, tenantID); err != nil {
		monitoring.RecordCacheOperation("invalidate_permissions_cache_failure", "error")
	}

	// Audit log the creation
	details := map[string]interface{}{
		"permission_id": permission.ID,
		"resource":      permission.Resource,
		"action":        permission.Action,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "create", "permission", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// GetPermission retrieves a permission with caching
func (s *RBACService) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("get_permission", "rbac.permission", time.Since(start), true) }()

	// Try cache first
	cachedPermissions, err := s.cache.GetPermissions(ctx, tenantID)
	if err == nil && cachedPermissions != nil {
		monitoring.RecordCacheOperation("get_permissions", "hit")
		for _, perm := range cachedPermissions {
			if perm.ID == permissionID {
				return perm, nil
			}
		}
	}

	monitoring.RecordCacheOperation("get_permissions", "miss")

	// Get from repository
	permission, err := s.repository.GetPermission(ctx, tenantID, permissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permission: %w", err)
	}
	if permission == nil {
		return nil, fmt.Errorf("permission not found: %s", permissionID)
	}

	return permission, nil
}

// ListPermissions retrieves all permissions for a tenant with caching
func (s *RBACService) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("list_permissions", "rbac.permission", time.Since(start), true) }()

	// Try cache first
	cachedPermissions, err := s.cache.GetPermissions(ctx, tenantID)
	if err == nil && cachedPermissions != nil {
		monitoring.RecordCacheOperation("list_permissions", "hit")
		return cachedPermissions, nil
	}

	monitoring.RecordCacheOperation("list_permissions", "miss")

	// Get from repository
	permissions, err := s.repository.ListPermissions(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	// Cache for future requests
	cacheTTL := 30 * time.Minute
	if cacheErr := s.cache.SetPermissions(ctx, tenantID, permissions, cacheTTL); cacheErr != nil {
		monitoring.RecordCacheOperation("cache_permissions_failure", "error")
	}

	return permissions, nil
}

// UpdatePermission updates an existing permission with validation
func (s *RBACService) UpdatePermission(ctx context.Context, tenantID, userID, permissionID string, updates *models.Permission) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("update_permission", "rbac.permission", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Get existing permission
	existingPermission, err := s.GetPermission(ctx, tenantID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to get existing permission: %w", err)
	}

	// Validate updates
	if err := s.validatePermissionUpdates(existingPermission, updates); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "permission.update", "rbac.permission", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Apply updates
	existingPermission.Resource = updates.Resource
	existingPermission.Action = updates.Action
	existingPermission.ResourcePattern = updates.ResourcePattern
	existingPermission.Scope = updates.Scope
	existingPermission.Conditions = updates.Conditions
	existingPermission.Description = updates.Description
	existingPermission.Metadata = updates.Metadata
	existingPermission.UpdatedBy = userID
	existingPermission.UpdatedAt = time.Now()

	// Update in repository
	if err := s.repository.UpdatePermission(ctx, existingPermission); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "permission.update", "rbac.permission", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to update permission: %w", err)
	}

	// Invalidate permissions cache
	if err := s.cache.InvalidatePermissions(ctx, tenantID); err != nil {
		monitoring.RecordCacheOperation("invalidate_permissions_cache_failure", "error")
	}

	// Audit log the update
	details := map[string]interface{}{
		"permission_id": existingPermission.ID,
		"resource":      existingPermission.Resource,
		"action":        existingPermission.Action,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "update", "permission", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// DeletePermission deletes a permission with safety checks
func (s *RBACService) DeletePermission(ctx context.Context, tenantID, userID, permissionID string) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("delete_permission", "rbac.permission", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Get existing permission
	_, err := s.GetPermission(ctx, tenantID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to get permission: %w", err)
	}

	// Check if permission is assigned to any roles
	// This is a simplified check - in production, you'd want to check all roles
	roles, err := s.repository.ListRoles(ctx, tenantID)
	if err == nil {
		for _, role := range roles {
			for _, permID := range role.Permissions {
				if permID == permissionID {
					return PermissionValidationError{Field: "assignments", Message: "cannot delete permission that is assigned to roles"}
				}
			}
		}
	}

	// Delete from repository
	if err := s.repository.DeletePermission(ctx, tenantID, permissionID); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "permission.delete", "rbac.permission", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	// Invalidate permissions cache
	if err := s.cache.InvalidatePermissions(ctx, tenantID); err != nil {
		monitoring.RecordCacheOperation("invalidate_permissions_cache_failure", "error")
	}

	// Audit log the deletion
	details := map[string]interface{}{
		"permission_id": permissionID,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "delete", "permission", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// CreateGroup creates a new group with validation
func (s *RBACService) CreateGroup(ctx context.Context, tenantID, userID string, group *models.Group) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("create_group", "rbac.group", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Validate group
	if err := s.validateGroup(group); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "group.create", "rbac.group", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Set metadata
	group.TenantID = tenantID
	group.CreatedBy = userID
	group.UpdatedBy = userID
	group.CreatedAt = time.Now()
	group.UpdatedAt = time.Now()

	// Check for duplicate group name
	existingGroup, err := s.repository.GetGroup(ctx, tenantID, group.Name)
	if err == nil && existingGroup != nil {
		return GroupValidationError{Field: "name", Message: "group with this name already exists"}
	}

	// Create group in repository
	if err := s.repository.CreateGroup(ctx, group); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "group.create", "rbac.group", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to create group: %w", err)
	}

	// Audit log the creation
	details := map[string]interface{}{
		"group_name": group.Name,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "create", "group", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// GetGroup retrieves a group with caching
func (s *RBACService) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("get_group", "rbac.group", time.Since(start), true) }()

	// Get from repository
	group, err := s.repository.GetGroup(ctx, tenantID, groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}
	if group == nil {
		return nil, fmt.Errorf("group not found: %s", groupName)
	}

	return group, nil
}

// ListGroups retrieves all groups for a tenant
func (s *RBACService) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("list_groups", "rbac.group", time.Since(start), true) }()

	// Get from repository
	groups, err := s.repository.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	return groups, nil
}

// UpdateGroup updates an existing group with validation
func (s *RBACService) UpdateGroup(ctx context.Context, tenantID, userID, groupName string, updates *models.Group) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("update_group", "rbac.group", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Get existing group
	existingGroup, err := s.GetGroup(ctx, tenantID, groupName)
	if err != nil {
		return fmt.Errorf("failed to get existing group: %w", err)
	}

	// Validate updates
	if err := s.validateGroupUpdates(existingGroup, updates); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "group.update", "rbac.group", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Apply updates
	existingGroup.Description = updates.Description
	existingGroup.Roles = updates.Roles
	existingGroup.ParentGroups = updates.ParentGroups
	existingGroup.Metadata = updates.Metadata
	existingGroup.UpdatedBy = userID
	existingGroup.UpdatedAt = time.Now()

	// Update in repository
	if err := s.repository.UpdateGroup(ctx, existingGroup); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "group.update", "rbac.group", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to update group: %w", err)
	}

	// Audit log the update
	details := map[string]interface{}{
		"group_name": existingGroup.Name,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "update", "group", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// DeleteGroup deletes a group with safety checks
func (s *RBACService) DeleteGroup(ctx context.Context, tenantID, userID, groupName string) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("delete_group", "rbac.group", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Get existing group
	_, err := s.GetGroup(ctx, tenantID, groupName)
	if err != nil {
		return fmt.Errorf("failed to get group: %w", err)
	}

	// Check if group has members
	members, err := s.repository.GetGroupMembers(ctx, tenantID, groupName)
	if err == nil && len(members) > 0 {
		return GroupValidationError{Field: "members", Message: "cannot delete group that has members"}
	}

	// Delete from repository
	if err := s.repository.DeleteGroup(ctx, tenantID, groupName); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "group.delete", "rbac.group", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to delete group: %w", err)
	}

	// Audit log the deletion
	details := map[string]interface{}{
		"group_name": groupName,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "delete", "group", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// AddUsersToGroup adds users to a group
func (s *RBACService) AddUsersToGroup(ctx context.Context, tenantID, userID, groupName string, userIDs []string) error {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("add_users_to_group", "rbac.group", time.Since(start), true) }()

	correlationID := generateCorrelationID()

	// Validate group exists
	_, err := s.GetGroup(ctx, tenantID, groupName)
	if err != nil {
		return fmt.Errorf("failed to get group: %w", err)
	}

	// Add users to group
	if err := s.repository.AddUsersToGroup(ctx, tenantID, groupName, userIDs); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "group.add_users", "rbac.group", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to add users to group: %w", err)
	}

	// Audit log the addition
	details := map[string]interface{}{
		"group_name": groupName,
		"user_ids":   userIDs,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "add_users", "group", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// RemoveUsersFromGroup removes users from a group
func (s *RBACService) RemoveUsersFromGroup(ctx context.Context, tenantID, userID, groupName string, userIDs []string) error {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("remove_users_from_group", "rbac.group", time.Since(start), true)
	}()

	correlationID := generateCorrelationID()

	// Remove users from group
	if err := s.repository.RemoveUsersFromGroup(ctx, tenantID, groupName, userIDs); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "group.remove_users", "rbac.group", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to remove users from group: %w", err)
	}

	// Audit log the removal
	details := map[string]interface{}{
		"group_name": groupName,
		"user_ids":   userIDs,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "remove_users", "group", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// GetGroupMembers retrieves members of a group
func (s *RBACService) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	start := time.Now()
	defer func() { monitoring.RecordAPIOperation("get_group_members", "rbac.group", time.Since(start), true) }()

	members, err := s.repository.GetGroupMembers(ctx, tenantID, groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	return members, nil
}

// CreateRoleBinding creates a new role binding with validation
func (s *RBACService) CreateRoleBinding(ctx context.Context, tenantID, userID string, binding *models.RoleBinding) error {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("create_role_binding", "rbac.role_binding", time.Since(start), true)
	}()

	correlationID := generateCorrelationID()

	// Validate role binding
	if err := s.validateRoleBinding(binding); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role_binding.create", "rbac.role_binding", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Set metadata
	binding.CreatedBy = userID
	binding.UpdatedBy = userID
	binding.CreatedAt = time.Now()
	binding.UpdatedAt = time.Now()

	// Create role binding in repository
	if err := s.repository.CreateRoleBinding(ctx, binding); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role_binding.create", "rbac.role_binding", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to create role binding: %w", err)
	}

	// Audit log the creation
	details := map[string]interface{}{
		"binding_id":   binding.ID,
		"subject_type": binding.SubjectType,
		"subject_id":   binding.SubjectID,
		"role_id":      binding.RoleID,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "create", "role_binding", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// GetRoleBindings retrieves role bindings with filters
func (s *RBACService) GetRoleBindings(ctx context.Context, tenantID string, filters RoleBindingFilters) ([]*models.RoleBinding, error) {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("get_role_bindings", "rbac.role_binding", time.Since(start), true)
	}()

	bindings, err := s.repository.GetRoleBindings(ctx, tenantID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get role bindings: %w", err)
	}

	return bindings, nil
}

// UpdateRoleBinding updates an existing role binding with validation
func (s *RBACService) UpdateRoleBinding(ctx context.Context, tenantID, userID, bindingID string, updates *models.RoleBinding) error {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("update_role_binding", "rbac.role_binding", time.Since(start), true)
	}()

	correlationID := generateCorrelationID()

	// Get existing binding
	existingBindings, err := s.repository.GetRoleBindings(ctx, tenantID, RoleBindingFilters{})
	if err != nil {
		return fmt.Errorf("failed to get existing role bindings: %w", err)
	}

	var existingBinding *models.RoleBinding
	for _, binding := range existingBindings {
		if binding.ID == bindingID {
			existingBinding = binding
			break
		}
	}

	if existingBinding == nil {
		return fmt.Errorf("role binding not found: %s", bindingID)
	}

	// Validate updates
	if err := s.validateRoleBindingUpdates(existingBinding, updates); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role_binding.update", "rbac.role_binding", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return err
	}

	// Apply updates
	existingBinding.SubjectType = updates.SubjectType
	existingBinding.SubjectID = updates.SubjectID
	existingBinding.RoleID = updates.RoleID
	existingBinding.Scope = updates.Scope
	existingBinding.ResourceID = updates.ResourceID
	existingBinding.Precedence = updates.Precedence
	existingBinding.Conditions = updates.Conditions
	existingBinding.ExpiresAt = updates.ExpiresAt
	existingBinding.Metadata = updates.Metadata
	existingBinding.UpdatedBy = userID
	existingBinding.UpdatedAt = time.Now()

	// Update in repository
	if err := s.repository.UpdateRoleBinding(ctx, existingBinding); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role_binding.update", "rbac.role_binding", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to update role binding: %w", err)
	}

	// Audit log the update
	details := map[string]interface{}{
		"binding_id":   existingBinding.ID,
		"subject_type": existingBinding.SubjectType,
		"subject_id":   existingBinding.SubjectID,
		"role_id":      existingBinding.RoleID,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "update", "role_binding", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// DeleteRoleBinding deletes a role binding
func (s *RBACService) DeleteRoleBinding(ctx context.Context, tenantID, userID, bindingID string) error {
	start := time.Now()
	defer func() {
		monitoring.RecordAPIOperation("delete_role_binding", "rbac.role_binding", time.Since(start), true)
	}()

	correlationID := generateCorrelationID()

	// Delete from repository
	if err := s.repository.DeleteRoleBinding(ctx, tenantID, bindingID); err != nil {
		if auditErr := s.auditService.LogError(ctx, tenantID, userID, "role_binding.delete", "rbac.role_binding", err, correlationID); auditErr != nil {
			monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
		}
		return fmt.Errorf("failed to delete role binding: %w", err)
	}

	// Audit log the deletion
	details := map[string]interface{}{
		"binding_id": bindingID,
	}
	if err := s.auditService.LogSystemEvent(ctx, tenantID, "delete", "role_binding", details, correlationID); err != nil {
		monitoring.RecordAPIOperation("audit_log_failure", "rbac.audit", time.Since(start), false)
	}

	return nil
}

// validatePermission validates permission data
func (s *RBACService) validatePermission(permission *models.Permission) error {
	if strings.TrimSpace(permission.ID) == "" {
		return PermissionValidationError{Field: "id", Message: "permission ID cannot be empty"}
	}

	if strings.TrimSpace(permission.Resource) == "" {
		return PermissionValidationError{Field: "resource", Message: "resource cannot be empty"}
	}

	if strings.TrimSpace(permission.Action) == "" {
		return PermissionValidationError{Field: "action", Message: "action cannot be empty"}
	}

	if len(permission.Description) > 500 {
		return PermissionValidationError{Field: "description", Message: "description cannot exceed 500 characters"}
	}

	return nil
}

// validatePermissionUpdates validates permission update data
func (s *RBACService) validatePermissionUpdates(existing, updates *models.Permission) error {
	// ID cannot be changed
	if updates.ID != existing.ID {
		return PermissionValidationError{Field: "id", Message: "permission ID cannot be changed"}
	}

	return s.validatePermission(updates)
}

// validateGroup validates group data
func (s *RBACService) validateGroup(group *models.Group) error {
	if strings.TrimSpace(group.Name) == "" {
		return GroupValidationError{Field: "name", Message: "group name cannot be empty"}
	}

	if len(group.Name) > 100 {
		return GroupValidationError{Field: "name", Message: "group name cannot exceed 100 characters"}
	}

	if len(group.Description) > 500 {
		return GroupValidationError{Field: "description", Message: "group description cannot exceed 500 characters"}
	}

	return nil
}

// validateGroupUpdates validates group update data
func (s *RBACService) validateGroupUpdates(existing, updates *models.Group) error {
	// Name cannot be changed
	if updates.Name != existing.Name {
		return GroupValidationError{Field: "name", Message: "group name cannot be changed"}
	}

	return s.validateGroup(updates)
}

// validateRoleBinding validates role binding data
func (s *RBACService) validateRoleBinding(binding *models.RoleBinding) error {
	if strings.TrimSpace(binding.SubjectType) == "" {
		return RoleBindingValidationError{Field: "subjectType", Message: "subject type cannot be empty"}
	}

	if strings.TrimSpace(binding.SubjectID) == "" {
		return RoleBindingValidationError{Field: "subjectId", Message: "subject ID cannot be empty"}
	}

	if strings.TrimSpace(binding.RoleID) == "" {
		return RoleBindingValidationError{Field: "roleId", Message: "role ID cannot be empty"}
	}

	// Validate subject type
	validSubjectTypes := []string{"user", "group", "service"}
	validSubjectType := false
	for _, st := range validSubjectTypes {
		if binding.SubjectType == st {
			validSubjectType = true
			break
		}
	}
	if !validSubjectType {
		return RoleBindingValidationError{Field: "subjectType", Message: "subject type must be one of: user, group, service"}
	}

	return nil
}

// validateRoleBindingUpdates validates role binding update data
func (s *RBACService) validateRoleBindingUpdates(existing, updates *models.RoleBinding) error {
	// ID cannot be changed
	if updates.ID != existing.ID {
		return RoleBindingValidationError{Field: "id", Message: "role binding ID cannot be changed"}
	}

	return s.validateRoleBinding(updates)
}

// GroupValidationError represents validation errors for groups
type GroupValidationError struct {
	Field   string
	Message string
}

func (e GroupValidationError) Error() string {
	return fmt.Sprintf("group validation error [%s]: %s", e.Field, e.Message)
}

// RoleBindingValidationError represents validation errors for role bindings
type RoleBindingValidationError struct {
	Field   string
	Message string
}

func (e RoleBindingValidationError) Error() string {
	return fmt.Sprintf("role binding validation error [%s]: %s", e.Field, e.Message)
}
