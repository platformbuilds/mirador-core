package rbac

import (
	"context"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// RBACRepository defines the interface for RBAC data persistence operations
type RBACRepository interface {
	// Role operations
	CreateRole(ctx context.Context, role *models.Role) error
	GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error)
	ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error)
	UpdateRole(ctx context.Context, role *models.Role) error
	DeleteRole(ctx context.Context, tenantID, roleName string) error

	// Permission operations
	CreatePermission(ctx context.Context, permission *models.Permission) error
	GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error)
	ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error)
	UpdatePermission(ctx context.Context, permission *models.Permission) error
	DeletePermission(ctx context.Context, tenantID, permissionID string) error

	// User role operations
	AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error
	GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error)
	RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error

	// User group operations
	GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error)

	// Role binding operations
	CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error
	GetRoleBindings(ctx context.Context, tenantID string, filters RoleBindingFilters) ([]*models.RoleBinding, error)
	UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error
	DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error

	// Group operations
	CreateGroup(ctx context.Context, group *models.Group) error
	GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error)
	ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error)
	UpdateGroup(ctx context.Context, group *models.Group) error
	DeleteGroup(ctx context.Context, tenantID, groupName string) error

	// Group membership operations
	AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error
	RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error
	GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error)

	// Audit logging
	LogAuditEvent(ctx context.Context, event *models.AuditLog) error
	GetAuditEvents(ctx context.Context, tenantID string, filters AuditFilters) ([]*models.AuditLog, error)

	// Tenant operations
	CreateTenant(ctx context.Context, tenant *models.Tenant) error
	GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error)
	ListTenants(ctx context.Context, filters TenantFilters) ([]*models.Tenant, error)
	UpdateTenant(ctx context.Context, tenant *models.Tenant) error
	DeleteTenant(ctx context.Context, tenantID string) error

	// User operations
	CreateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, userID string) (*models.User, error)
	ListUsers(ctx context.Context, filters UserFilters) ([]*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, userID string) error

	// Tenant-User association operations
	CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error
	GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error)
	ListTenantUsers(ctx context.Context, tenantID string, filters TenantUserFilters) ([]*models.TenantUser, error)
	UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error
	DeleteTenantUser(ctx context.Context, tenantID, userID string) error

	// MiradorAuth operations
	CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error
	GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error)
	UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error
	DeleteMiradorAuth(ctx context.Context, userID string) error

	// AuthConfig operations
	CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error
	GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error)
	UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error
	DeleteAuthConfig(ctx context.Context, tenantID string) error

	// API Key operations
	CreateAPIKey(ctx context.Context, apiKey *models.APIKey) error
	GetAPIKeyByHash(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error)
	GetAPIKeyByID(ctx context.Context, tenantID, keyID string) (*models.APIKey, error)
	ListAPIKeys(ctx context.Context, tenantID, userID string) ([]*models.APIKey, error)
	UpdateAPIKey(ctx context.Context, apiKey *models.APIKey) error
	RevokeAPIKey(ctx context.Context, tenantID, keyID string) error
	ValidateAPIKey(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error)
}

// RoleBindingFilters defines filters for role binding queries
type RoleBindingFilters struct {
	SubjectType *string
	SubjectID   *string
	RoleID      *string
	Scope       *string
	ResourceID  *string
	Precedence  *string
	Expired     *bool // true = include expired, false = exclude expired, nil = all
}

// AuditFilters defines filters for audit log queries
type AuditFilters struct {
	SubjectID   *string
	SubjectType *string
	Action      *string
	Resource    *string
	ResourceID  *string
	Result      *string
	Severity    *string
	Source      *string
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int
	Offset      int
}

// TenantFilters defines filters for tenant queries
type TenantFilters struct {
	Name       *string
	Status     *string
	AdminEmail *string
	Limit      int
	Offset     int
}

// UserFilters defines filters for user queries
type UserFilters struct {
	Email      *string
	Username   *string
	GlobalRole *string
	Status     *string
	Limit      int
	Offset     int
}

// TenantUserFilters defines filters for tenant-user association queries
type TenantUserFilters struct {
	UserID     *string
	TenantRole *string
	Status     *string
	Limit      int
	Offset     int
}

// CacheRepository defines the interface for RBAC caching operations
type CacheRepository interface {
	// Role cache operations
	SetRole(ctx context.Context, tenantID, roleName string, role *models.Role, ttl time.Duration) error
	GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error)
	DeleteRole(ctx context.Context, tenantID, roleName string) error
	InvalidateTenantRoles(ctx context.Context, tenantID string) error

	// User roles cache operations
	SetUserRoles(ctx context.Context, tenantID, userID string, roles []string, ttl time.Duration) error
	GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error)
	DeleteUserRoles(ctx context.Context, tenantID, userID string) error
	InvalidateUserRoles(ctx context.Context, tenantID, userID string) error

	// Permission cache operations
	SetPermissions(ctx context.Context, tenantID string, permissions []*models.Permission, ttl time.Duration) error
	GetPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error)
	InvalidatePermissions(ctx context.Context, tenantID string) error
}
