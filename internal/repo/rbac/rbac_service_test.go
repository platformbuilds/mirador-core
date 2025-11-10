package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type MockRBACRepository struct {
	mock.Mock
}

func (m *MockRBACRepository) CreateRole(ctx context.Context, role *models.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRBACRepository) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	args := m.Called(ctx, tenantID, roleName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}

func (m *MockRBACRepository) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.Role), args.Error(1)
}

func (m *MockRBACRepository) UpdateRole(ctx context.Context, role *models.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRBACRepository) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	args := m.Called(ctx, tenantID, roleName)
	return args.Error(0)
}

func (m *MockRBACRepository) AssignUserRoles(ctx context.Context, tenantID, userID string, roleNames []string) error {
	args := m.Called(ctx, tenantID, userID, roleNames)
	return args.Error(0)
}

func (m *MockRBACRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	args := m.Called(ctx, tenantID, userID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRBACRepository) RemoveUserRoles(ctx context.Context, tenantID, userID string, roleNames []string) error {
	args := m.Called(ctx, tenantID, userID, roleNames)
	return args.Error(0)
}

func (m *MockRBACRepository) CreatePermission(ctx context.Context, permission *models.Permission) error {
	args := m.Called(ctx, permission)
	return args.Error(0)
}

func (m *MockRBACRepository) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	args := m.Called(ctx, tenantID, permissionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Permission), args.Error(1)
}

func (m *MockRBACRepository) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.Permission), args.Error(1)
}

func (m *MockRBACRepository) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	args := m.Called(ctx, permission)
	return args.Error(0)
}

func (m *MockRBACRepository) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	args := m.Called(ctx, tenantID, permissionID)
	return args.Error(0)
}

func (m *MockRBACRepository) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	args := m.Called(ctx, binding)
	return args.Error(0)
}

func (m *MockRBACRepository) GetRoleBindings(ctx context.Context, tenantID string, filters RoleBindingFilters) ([]*models.RoleBinding, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]*models.RoleBinding), args.Error(1)
}

func (m *MockRBACRepository) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	args := m.Called(ctx, binding)
	return args.Error(0)
}

func (m *MockRBACRepository) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	args := m.Called(ctx, tenantID, bindingID)
	return args.Error(0)
}

func (m *MockRBACRepository) CreateGroup(ctx context.Context, group *models.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockRBACRepository) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	args := m.Called(ctx, tenantID, groupName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *MockRBACRepository) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.Group), args.Error(1)
}

func (m *MockRBACRepository) UpdateGroup(ctx context.Context, group *models.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockRBACRepository) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	args := m.Called(ctx, tenantID, groupName)
	return args.Error(0)
}

func (m *MockRBACRepository) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	args := m.Called(ctx, tenantID, groupName, userIDs)
	return args.Error(0)
}

func (m *MockRBACRepository) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	args := m.Called(ctx, tenantID, groupName, userIDs)
	return args.Error(0)
}

func (m *MockRBACRepository) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	args := m.Called(ctx, tenantID, userID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRBACRepository) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	args := m.Called(ctx, tenantID, groupName)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRBACRepository) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockRBACRepository) GetAuditEvents(ctx context.Context, tenantID string, filters AuditFilters) ([]*models.AuditLog, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]*models.AuditLog), args.Error(1)
}

func (m *MockRBACRepository) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockRBACRepository) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Tenant), args.Error(1)
}

func (m *MockRBACRepository) ListTenants(ctx context.Context, filters TenantFilters) ([]*models.Tenant, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*models.Tenant), args.Error(1)
}

func (m *MockRBACRepository) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockRBACRepository) DeleteTenant(ctx context.Context, tenantID string) error {
	args := m.Called(ctx, tenantID)
	return args.Error(0)
}

func (m *MockRBACRepository) CreateUser(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRBACRepository) GetUser(ctx context.Context, userID string) (*models.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockRBACRepository) ListUsers(ctx context.Context, filters UserFilters) ([]*models.User, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *MockRBACRepository) UpdateUser(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRBACRepository) DeleteUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRBACRepository) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	args := m.Called(ctx, tenantUser)
	return args.Error(0)
}

func (m *MockRBACRepository) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TenantUser), args.Error(1)
}

func (m *MockRBACRepository) ListTenantUsers(ctx context.Context, tenantID string, filters TenantUserFilters) ([]*models.TenantUser, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]*models.TenantUser), args.Error(1)
}

func (m *MockRBACRepository) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	args := m.Called(ctx, tenantUser)
	return args.Error(0)
}

func (m *MockRBACRepository) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	args := m.Called(ctx, tenantID, userID)
	return args.Error(0)
}

func (m *MockRBACRepository) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	args := m.Called(ctx, auth)
	return args.Error(0)
}

func (m *MockRBACRepository) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MiradorAuth), args.Error(1)
}

func (m *MockRBACRepository) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	args := m.Called(ctx, auth)
	return args.Error(0)
}

func (m *MockRBACRepository) DeleteMiradorAuth(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRBACRepository) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockRBACRepository) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AuthConfig), args.Error(1)
}

func (m *MockRBACRepository) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockRBACRepository) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	args := m.Called(ctx, tenantID)
	return args.Error(0)
}

type MockCacheRepository struct {
	mock.Mock
}

func (m *MockCacheRepository) SetRole(ctx context.Context, tenantID, roleName string, role *models.Role, ttl time.Duration) error {
	args := m.Called(ctx, tenantID, roleName, role, ttl)
	return args.Error(0)
}

func (m *MockCacheRepository) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	args := m.Called(ctx, tenantID, roleName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}

func (m *MockCacheRepository) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	args := m.Called(ctx, tenantID, roleName)
	return args.Error(0)
}

func (m *MockCacheRepository) InvalidateTenantRoles(ctx context.Context, tenantID string) error {
	args := m.Called(ctx, tenantID)
	return args.Error(0)
}

func (m *MockCacheRepository) SetUserRoles(ctx context.Context, tenantID, userID string, roles []string, ttl time.Duration) error {
	args := m.Called(ctx, tenantID, userID, roles, ttl)
	return args.Error(0)
}

func (m *MockCacheRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	args := m.Called(ctx, tenantID, userID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCacheRepository) DeleteUserRoles(ctx context.Context, tenantID, userID string) error {
	args := m.Called(ctx, tenantID, userID)
	return args.Error(0)
}

func (m *MockCacheRepository) InvalidateUserRoles(ctx context.Context, tenantID, userID string) error {
	args := m.Called(ctx, tenantID, userID)
	return args.Error(0)
}

func (m *MockCacheRepository) SetPermissions(ctx context.Context, tenantID string, permissions []*models.Permission, ttl time.Duration) error {
	args := m.Called(ctx, tenantID, permissions, ttl)
	return args.Error(0)
}

func (m *MockCacheRepository) GetPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.Permission), args.Error(1)
}

func (m *MockCacheRepository) InvalidatePermissions(ctx context.Context, tenantID string) error {
	args := m.Called(ctx, tenantID)
	return args.Error(0)
}

func TestRBACService_CreateRole(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	role := &models.Role{
		Name:        "test-role",
		Description: "Test role",
		Permissions: []string{"read:resource", "write:resource"},
	}

	// Setup expectations
	mockRepo.On("GetRole", ctx, tenantID, role.Name).Return(nil, nil) // Role doesn't exist
	mockRepo.On("CreateRole", ctx, role).Return(nil)
	mockCache.On("SetRole", ctx, tenantID, role.Name, role, mock.AnythingOfType("time.Duration")).Return(nil)
	mockRepo.On("LogAuditEvent", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil)

	// Execute
	err := service.CreateRole(ctx, tenantID, userID, role)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, tenantID, role.TenantID)
	assert.Equal(t, userID, role.CreatedBy)
	assert.Equal(t, userID, role.UpdatedBy)
	assert.NotZero(t, role.CreatedAt)
	assert.NotZero(t, role.UpdatedAt)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestRBACService_CreateRole_ValidationError(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	// Empty role name should fail validation
	role := &models.Role{
		Name:        "",
		Description: "Test role",
		Permissions: []string{"read:resource"},
	}

	// Setup expectations for error logging
	mockRepo.On("LogAuditEvent", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil)

	// Execute
	err := service.CreateRole(ctx, tenantID, userID, role)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "role name cannot be empty")

	mockRepo.AssertExpectations(t)
}

func TestRBACService_GetRole_CacheHit(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	roleName := "test-role"

	cachedRole := &models.Role{
		Name:        roleName,
		Description: "Cached role",
		Permissions: []string{"read:resource"},
	}

	// Setup expectations - cache hit
	mockCache.On("GetRole", ctx, tenantID, roleName).Return(cachedRole, nil)

	// Execute
	result, err := service.GetRole(ctx, tenantID, roleName)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, cachedRole, result)

	mockCache.AssertExpectations(t)
	// Repository should not be called on cache hit
	mockRepo.AssertNotCalled(t, "GetRole", mock.Anything, mock.Anything, mock.Anything)
}

func TestRBACService_GetRole_CacheMiss(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	roleName := "test-role"

	repoRole := &models.Role{
		Name:        roleName,
		Description: "Repository role",
		Permissions: []string{"read:resource", "write:resource"},
		TenantID:    tenantID, // Add tenant ID
	}

	// Setup expectations - cache miss, then repository hit
	mockCache.On("GetRole", ctx, tenantID, roleName).Return(nil, nil) // Cache miss
	mockRepo.On("GetRole", ctx, tenantID, roleName).Return(repoRole, nil)
	mockCache.On("SetRole", ctx, tenantID, roleName, repoRole, mock.AnythingOfType("time.Duration")).Return(nil)

	// Execute
	result, err := service.GetRole(ctx, tenantID, roleName)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, repoRole, result)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestRBACService_AssignUserRoles(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "admin-user"
	targetUserID := "target-user"
	roleNames := []string{"admin", "editor"}

	// Mock roles exist
	adminRole := &models.Role{Name: "admin", Permissions: []string{"*"}, TenantID: tenantID}
	editorRole := &models.Role{Name: "editor", Permissions: []string{"read:*", "write:*"}, TenantID: tenantID}

	// Cache misses for role validation
	mockCache.On("GetRole", ctx, tenantID, "admin").Return(nil, nil)
	mockCache.On("GetRole", ctx, tenantID, "editor").Return(nil, nil)
	mockRepo.On("GetRole", ctx, tenantID, "admin").Return(adminRole, nil)
	mockRepo.On("GetRole", ctx, tenantID, "editor").Return(editorRole, nil)
	mockCache.On("SetRole", ctx, tenantID, "admin", adminRole, mock.AnythingOfType("time.Duration")).Return(nil)
	mockCache.On("SetRole", ctx, tenantID, "editor", editorRole, mock.AnythingOfType("time.Duration")).Return(nil)
	mockRepo.On("AssignUserRoles", ctx, tenantID, targetUserID, roleNames).Return(nil)
	mockCache.On("SetUserRoles", ctx, tenantID, targetUserID, roleNames, mock.AnythingOfType("time.Duration")).Return(nil)
	mockRepo.On("LogAuditEvent", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil)

	// Execute
	err := service.AssignUserRoles(ctx, tenantID, userID, targetUserID, roleNames)

	// Assert
	require.NoError(t, err)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestRBACService_CheckPermission_Allowed(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"
	resource := "dashboard"
	action := "read"

	userRoles := []string{"viewer"}
	viewerRole := &models.Role{
		Name:        "viewer",
		Permissions: []string{"dashboard:read"},
		TenantID:    tenantID,
	}

	dashboardReadPerm := &models.Permission{
		ID:       "dashboard:read",
		Resource: "dashboard",
		Action:   "read",
		Scope:    "tenant",
	}

	// Setup expectations
	mockCache.On("GetUserRoles", ctx, tenantID, userID).Return(userRoles, nil)
	mockCache.On("GetRole", ctx, tenantID, "viewer").Return(nil, nil) // Cache miss
	mockRepo.On("GetRole", ctx, tenantID, "viewer").Return(viewerRole, nil)
	mockCache.On("SetRole", ctx, tenantID, "viewer", viewerRole, mock.AnythingOfType("time.Duration")).Return(nil)
	mockRepo.On("GetPermission", ctx, tenantID, "dashboard:read").Return(dashboardReadPerm, nil)
	mockRepo.On("GetUserGroups", ctx, tenantID, userID).Return([]string{}, nil) // No groups for this user
	mockRepo.On("LogAuditEvent", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil)

	// Execute
	allowed, err := service.CheckPermission(ctx, tenantID, userID, resource, action)

	// Assert
	require.NoError(t, err)
	assert.True(t, allowed)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestRBACService_CheckPermission_Denied(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"
	resource := "dashboard"
	action := "delete"

	userRoles := []string{"viewer"}
	viewerRole := &models.Role{
		Name:        "viewer",
		Permissions: []string{"dashboard:read"}, // No delete permission
		TenantID:    tenantID,
	}

	dashboardReadPerm := &models.Permission{
		ID:       "dashboard:read",
		Resource: "dashboard",
		Action:   "read",
		Scope:    "tenant",
	}

	// Setup expectations
	mockCache.On("GetUserRoles", ctx, tenantID, userID).Return(userRoles, nil)
	mockCache.On("GetRole", ctx, tenantID, "viewer").Return(nil, nil) // Cache miss
	mockRepo.On("GetRole", ctx, tenantID, "viewer").Return(viewerRole, nil)
	mockCache.On("SetRole", ctx, tenantID, "viewer", viewerRole, mock.AnythingOfType("time.Duration")).Return(nil)
	mockRepo.On("GetPermission", ctx, tenantID, "dashboard:read").Return(dashboardReadPerm, nil)
	mockRepo.On("GetUserGroups", ctx, tenantID, userID).Return([]string{}, nil) // No groups for this user
	mockRepo.On("LogAuditEvent", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil)

	// Execute
	allowed, err := service.CheckPermission(ctx, tenantID, userID, resource, action)

	// Assert
	require.NoError(t, err)
	assert.False(t, allowed)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestRBACService_ValidateRoleAssignments_CircularDependency(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	// Create a role with circular dependency
	roleA := &models.Role{
		Name:        "role-a",
		ParentRoles: []string{"role-b"},
		TenantID:    tenantID,
	}
	roleB := &models.Role{
		Name:        "role-b",
		ParentRoles: []string{"role-a"}, // Creates circular dependency
		TenantID:    tenantID,
	}

	// Setup expectations
	mockCache.On("GetRole", ctx, tenantID, "role-a").Return(nil, nil) // Cache miss
	mockCache.On("GetRole", ctx, tenantID, "role-b").Return(nil, nil) // Cache miss
	mockRepo.On("GetRole", ctx, tenantID, "role-a").Return(roleA, nil)
	mockRepo.On("GetRole", ctx, tenantID, "role-b").Return(roleB, nil)
	mockCache.On("SetRole", ctx, tenantID, "role-a", roleA, mock.AnythingOfType("time.Duration")).Return(nil)
	mockCache.On("SetRole", ctx, tenantID, "role-b", roleB, mock.AnythingOfType("time.Duration")).Return(nil)

	// Execute validation
	err := service.validateRoleAssignments(ctx, tenantID, userID, []string{"role-a"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")

	mockRepo.AssertExpectations(t)
}

func TestRBACService_ValidateRoleAssignments_RoleNotFound(t *testing.T) {
	mockRepo := &MockRBACRepository{}
	mockCache := &MockCacheRepository{}

	auditService := NewAuditService(mockRepo)

	service := NewRBACService(mockRepo, mockCache, auditService)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	// Setup expectations - role doesn't exist
	mockCache.On("GetRole", ctx, tenantID, "nonexistent-role").Return(nil, nil) // Cache miss
	mockRepo.On("GetRole", ctx, tenantID, "nonexistent-role").Return(nil, nil)

	// Execute validation
	err := service.validateRoleAssignments(ctx, tenantID, userID, []string{"nonexistent-role"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "role does not exist")

	mockRepo.AssertExpectations(t)
}
