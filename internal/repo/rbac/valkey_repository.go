package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
)

// ValkeyRBACRepository implements CacheRepository using Valkey (Redis)
type ValkeyRBACRepository struct {
	client ValkeyClient
}

// ValkeyClient interface for Redis operations
type ValkeyClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Keys(ctx context.Context, pattern string) ([]string, error)
}

// NewValkeyRBACRepository creates a new Valkey-based RBAC cache repository
func NewValkeyRBACRepository(client ValkeyClient) *ValkeyRBACRepository {
	return &ValkeyRBACRepository{
		client: client,
	}
}

// makeCacheKey generates cache keys for RBAC objects
func makeCacheKey(class, tenantID string, parts ...string) string {
	allParts := append([]string{"rbac", class, tenantID}, parts...)
	return fmt.Sprintf("%s", allParts)
}

// CacheRole caches a role with TTL
func (r *ValkeyRBACRepository) CacheRole(ctx context.Context, role *models.Role, ttl time.Duration) error {
	key := makeCacheKey("role", role.TenantID, role.Name)

	data, err := json.Marshal(role)
	if err != nil {
		return fmt.Errorf("failed to marshal role: %w", err)
	}

	start := time.Now()
	setErr := r.client.Set(ctx, key, string(data), ttl)
	_ = time.Since(start)

	result := "success"
	if setErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("cache_role", result)
		return fmt.Errorf("failed to cache role %s: %w", role.Name, setErr)
	}

	monitoring.RecordCacheOperation("cache_role", result)
	return nil
}

// GetCachedRole retrieves a cached role
func (r *ValkeyRBACRepository) GetCachedRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	key := makeCacheKey("role", tenantID, roleName)

	start := time.Now()
	data, getErr := r.client.Get(ctx, key)
	_ = time.Since(start)

	result := "hit"
	if getErr != nil {
		result = "miss"
		monitoring.RecordCacheOperation("get_cached_role", result)
		return nil, fmt.Errorf("failed to get cached role %s: %w", roleName, getErr)
	}

	monitoring.RecordCacheOperation("get_cached_role", result)

	var role models.Role
	if err := json.Unmarshal([]byte(data), &role); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached role: %w", err)
	}

	return &role, nil
}

// InvalidateRoleCache removes a role from cache
func (r *ValkeyRBACRepository) InvalidateRoleCache(ctx context.Context, tenantID, roleName string) error {
	key := makeCacheKey("role", tenantID, roleName)

	start := time.Now()
	delErr := r.client.Del(ctx, key)
	_ = time.Since(start)

	result := "success"
	if delErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("invalidate_role_cache", result)
		return fmt.Errorf("failed to invalidate role cache %s: %w", roleName, delErr)
	}

	monitoring.RecordCacheOperation("invalidate_role_cache", result)
	return nil
}

// CacheUserRoles caches user role assignments
func (r *ValkeyRBACRepository) CacheUserRoles(ctx context.Context, tenantID, userID string, roles []string, ttl time.Duration) error {
	key := makeCacheKey("user_roles", tenantID, userID)

	data, err := json.Marshal(roles)
	if err != nil {
		return fmt.Errorf("failed to marshal user roles: %w", err)
	}

	start := time.Now()
	setErr := r.client.Set(ctx, key, string(data), ttl)
	_ = time.Since(start)

	result := "success"
	if setErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("cache_user_roles", result)
		return fmt.Errorf("failed to cache user roles for %s: %w", userID, setErr)
	}

	monitoring.RecordCacheOperation("cache_user_roles", result)
	return nil
}

// GetCachedUserRoles retrieves cached user roles
func (r *ValkeyRBACRepository) GetCachedUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	key := makeCacheKey("user_roles", tenantID, userID)

	start := time.Now()
	data, getErr := r.client.Get(ctx, key)
	_ = time.Since(start)

	result := "hit"
	if getErr != nil {
		result = "miss"
		monitoring.RecordCacheOperation("get_cached_user_roles", result)
		return nil, fmt.Errorf("failed to get cached user roles for %s: %w", userID, getErr)
	}

	monitoring.RecordCacheOperation("get_cached_user_roles", result)

	var roles []string
	if err := json.Unmarshal([]byte(data), &roles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached user roles: %w", err)
	}

	return roles, nil
}

// InvalidateUserRolesCache removes user roles from cache
func (r *ValkeyRBACRepository) InvalidateUserRolesCache(ctx context.Context, tenantID, userID string) error {
	key := makeCacheKey("user_roles", tenantID, userID)

	start := time.Now()
	delErr := r.client.Del(ctx, key)
	_ = time.Since(start)

	result := "success"
	if delErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("invalidate_user_roles_cache", result)
		return fmt.Errorf("failed to invalidate user roles cache for %s: %w", userID, delErr)
	}

	monitoring.RecordCacheOperation("invalidate_user_roles_cache", result)
	return nil
}

// CachePermissions caches permissions for a resource
func (r *ValkeyRBACRepository) CachePermissions(ctx context.Context, tenantID, resource string, permissions []*models.Permission, ttl time.Duration) error {
	key := makeCacheKey("permissions", tenantID, resource)

	data, err := json.Marshal(permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	start := time.Now()
	setErr := r.client.Set(ctx, key, string(data), ttl)
	_ = time.Since(start)

	result := "success"
	if setErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("cache_permissions", result)
		return fmt.Errorf("failed to cache permissions for %s: %w", resource, setErr)
	}

	monitoring.RecordCacheOperation("cache_permissions", result)
	return nil
}

// GetCachedPermissions retrieves cached permissions
func (r *ValkeyRBACRepository) GetCachedPermissions(ctx context.Context, tenantID, resource string) ([]*models.Permission, error) {
	key := makeCacheKey("permissions", tenantID, resource)

	start := time.Now()
	data, getErr := r.client.Get(ctx, key)
	_ = time.Since(start)

	result := "hit"
	if getErr != nil {
		result = "miss"
		monitoring.RecordCacheOperation("get_cached_permissions", result)
		return nil, fmt.Errorf("failed to get cached permissions for %s: %w", resource, getErr)
	}

	monitoring.RecordCacheOperation("get_cached_permissions", result)

	var permissions []*models.Permission
	if err := json.Unmarshal([]byte(data), &permissions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached permissions: %w", err)
	}

	return permissions, nil
}

// InvalidatePermissionsCache removes permissions from cache
func (r *ValkeyRBACRepository) InvalidatePermissionsCache(ctx context.Context, tenantID, resource string) error {
	key := makeCacheKey("permissions", tenantID, resource)

	start := time.Now()
	delErr := r.client.Del(ctx, key)
	_ = time.Since(start)

	result := "success"
	if delErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("invalidate_permissions_cache", result)
		return fmt.Errorf("failed to invalidate permissions cache for %s: %w", resource, delErr)
	}

	monitoring.RecordCacheOperation("invalidate_permissions_cache", result)
	return nil
}

// CacheRoleBindings caches role bindings for a subject
func (r *ValkeyRBACRepository) CacheRoleBindings(ctx context.Context, tenantID, subjectType, subjectID string, bindings []*models.RoleBinding, ttl time.Duration) error {
	key := makeCacheKey("role_bindings", tenantID, subjectType, subjectID)

	data, err := json.Marshal(bindings)
	if err != nil {
		return fmt.Errorf("failed to marshal role bindings: %w", err)
	}

	start := time.Now()
	setErr := r.client.Set(ctx, key, string(data), ttl)
	_ = time.Since(start)

	result := "success"
	if setErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("cache_role_bindings", result)
		return fmt.Errorf("failed to cache role bindings for %s %s: %w", subjectType, subjectID, setErr)
	}

	monitoring.RecordCacheOperation("cache_role_bindings", result)
	return nil
}

// GetCachedRoleBindings retrieves cached role bindings
func (r *ValkeyRBACRepository) GetCachedRoleBindings(ctx context.Context, tenantID, subjectType, subjectID string) ([]*models.RoleBinding, error) {
	key := makeCacheKey("role_bindings", tenantID, subjectType, subjectID)

	start := time.Now()
	data, getErr := r.client.Get(ctx, key)
	_ = time.Since(start)

	result := "hit"
	if getErr != nil {
		result = "miss"
		monitoring.RecordCacheOperation("get_cached_role_bindings", result)
		return nil, fmt.Errorf("failed to get cached role bindings for %s %s: %w", subjectType, subjectID, getErr)
	}

	monitoring.RecordCacheOperation("get_cached_role_bindings", result)

	var bindings []*models.RoleBinding
	if err := json.Unmarshal([]byte(data), &bindings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached role bindings: %w", err)
	}

	return bindings, nil
}

// InvalidateRoleBindingsCache removes role bindings from cache
func (r *ValkeyRBACRepository) InvalidateRoleBindingsCache(ctx context.Context, tenantID, subjectType, subjectID string) error {
	key := makeCacheKey("role_bindings", tenantID, subjectType, subjectID)

	start := time.Now()
	delErr := r.client.Del(ctx, key)
	_ = time.Since(start)

	result := "success"
	if delErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("invalidate_role_bindings_cache", result)
		return fmt.Errorf("failed to invalidate role bindings cache for %s %s: %w", subjectType, subjectID, delErr)
	}

	monitoring.RecordCacheOperation("invalidate_role_bindings_cache", result)
	return nil
}

// CacheGroup caches a group with TTL
func (r *ValkeyRBACRepository) CacheGroup(ctx context.Context, group *models.Group, ttl time.Duration) error {
	key := makeCacheKey("group", group.TenantID, group.Name)

	data, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("failed to marshal group: %w", err)
	}

	start := time.Now()
	setErr := r.client.Set(ctx, key, string(data), ttl)
	_ = time.Since(start)

	result := "success"
	if setErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("cache_group", result)
		return fmt.Errorf("failed to cache group %s: %w", group.Name, setErr)
	}

	monitoring.RecordCacheOperation("cache_group", result)
	return nil
}

// GetCachedGroup retrieves a cached group
func (r *ValkeyRBACRepository) GetCachedGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	key := makeCacheKey("group", tenantID, groupName)

	start := time.Now()
	data, getErr := r.client.Get(ctx, key)
	_ = time.Since(start)

	result := "hit"
	if getErr != nil {
		result = "miss"
		monitoring.RecordCacheOperation("get_cached_group", result)
		return nil, fmt.Errorf("failed to get cached group %s: %w", groupName, getErr)
	}

	monitoring.RecordCacheOperation("get_cached_group", result)

	var group models.Group
	if err := json.Unmarshal([]byte(data), &group); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached group: %w", err)
	}

	return &group, nil
}

// InvalidateGroupCache removes a group from cache
func (r *ValkeyRBACRepository) InvalidateGroupCache(ctx context.Context, tenantID, groupName string) error {
	key := makeCacheKey("group", tenantID, groupName)

	start := time.Now()
	delErr := r.client.Del(ctx, key)
	_ = time.Since(start)

	result := "success"
	if delErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("invalidate_group_cache", result)
		return fmt.Errorf("failed to invalidate group cache %s: %w", groupName, delErr)
	}

	monitoring.RecordCacheOperation("invalidate_group_cache", result)
	return nil
}

// CacheGroupMembers caches group membership
func (r *ValkeyRBACRepository) CacheGroupMembers(ctx context.Context, tenantID, groupName string, members []string, ttl time.Duration) error {
	key := makeCacheKey("group_members", tenantID, groupName)

	data, err := json.Marshal(members)
	if err != nil {
		return fmt.Errorf("failed to marshal group members: %w", err)
	}

	start := time.Now()
	setErr := r.client.Set(ctx, key, string(data), ttl)
	_ = time.Since(start)

	result := "success"
	if setErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("cache_group_members", result)
		return fmt.Errorf("failed to cache group members for %s: %w", groupName, setErr)
	}

	monitoring.RecordCacheOperation("cache_group_members", result)
	return nil
}

// GetCachedGroupMembers retrieves cached group members
func (r *ValkeyRBACRepository) GetCachedGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	key := makeCacheKey("group_members", tenantID, groupName)

	start := time.Now()
	data, getErr := r.client.Get(ctx, key)
	_ = time.Since(start)

	result := "hit"
	if getErr != nil {
		result = "miss"
		monitoring.RecordCacheOperation("get_cached_group_members", result)
		return nil, fmt.Errorf("failed to get cached group members for %s: %w", groupName, getErr)
	}

	monitoring.RecordCacheOperation("get_cached_group_members", result)

	var members []string
	if err := json.Unmarshal([]byte(data), &members); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached group members: %w", err)
	}

	return members, nil
}

// InvalidateGroupMembersCache removes group members from cache
func (r *ValkeyRBACRepository) InvalidateGroupMembersCache(ctx context.Context, tenantID, groupName string) error {
	key := makeCacheKey("group_members", tenantID, groupName)

	start := time.Now()
	delErr := r.client.Del(ctx, key)
	_ = time.Since(start)

	result := "success"
	if delErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("invalidate_group_members_cache", result)
		return fmt.Errorf("failed to invalidate group members cache for %s: %w", groupName, delErr)
	}

	monitoring.RecordCacheOperation("invalidate_group_members_cache", result)
	return nil
}

// ClearTenantCache clears all RBAC cache entries for a tenant
func (r *ValkeyRBACRepository) ClearTenantCache(ctx context.Context, tenantID string) error {
	pattern := makeCacheKey("*", tenantID, "*")

	start := time.Now()
	keys, keysErr := r.client.Keys(ctx, pattern)
	_ = time.Since(start)

	if keysErr != nil {
		monitoring.RecordCacheOperation("clear_tenant_cache_keys", "error")
		return fmt.Errorf("failed to get cache keys for tenant %s: %w", tenantID, keysErr)
	}

	if len(keys) == 0 {
		return nil
	}

	start = time.Now()
	delErr := r.client.Del(ctx, keys...)
	_ = time.Since(start)

	result := "success"
	if delErr != nil {
		result = "error"
		monitoring.RecordCacheOperation("clear_tenant_cache_del", result)
		return fmt.Errorf("failed to delete cache keys for tenant %s: %w", tenantID, delErr)
	}

	monitoring.RecordCacheOperation("clear_tenant_cache_del", result)
	return nil
}

// WarmupCache pre-loads frequently accessed RBAC data
func (r *ValkeyRBACRepository) WarmupCache(ctx context.Context, tenantID string, ttl time.Duration) error {
	// This would be called during application startup or tenant activation
	// to pre-load critical RBAC data into cache
	// Implementation would depend on specific warmup requirements

	monitoring.RecordCacheOperation("warmup_cache", "success")
	return nil
}

// CacheRepository interface implementation methods

// SetRole implements CacheRepository.SetRole
func (r *ValkeyRBACRepository) SetRole(ctx context.Context, tenantID, roleName string, role *models.Role, ttl time.Duration) error {
	return r.CacheRole(ctx, role, ttl)
}

// GetRole implements CacheRepository.GetRole
func (r *ValkeyRBACRepository) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return r.GetCachedRole(ctx, tenantID, roleName)
}

// DeleteRole implements CacheRepository.DeleteRole
func (r *ValkeyRBACRepository) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return r.InvalidateRoleCache(ctx, tenantID, roleName)
}

// InvalidateTenantRoles implements CacheRepository.InvalidateTenantRoles
func (r *ValkeyRBACRepository) InvalidateTenantRoles(ctx context.Context, tenantID string) error {
	return r.ClearTenantCache(ctx, tenantID)
}

// SetUserRoles implements CacheRepository.SetUserRoles
func (r *ValkeyRBACRepository) SetUserRoles(ctx context.Context, tenantID, userID string, roles []string, ttl time.Duration) error {
	return r.CacheUserRoles(ctx, tenantID, userID, roles, ttl)
}

// GetUserRoles implements CacheRepository.GetUserRoles
func (r *ValkeyRBACRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return r.GetCachedUserRoles(ctx, tenantID, userID)
}

// DeleteUserRoles implements CacheRepository.DeleteUserRoles
func (r *ValkeyRBACRepository) DeleteUserRoles(ctx context.Context, tenantID, userID string) error {
	return r.InvalidateUserRolesCache(ctx, tenantID, userID)
}

// InvalidateUserRoles implements CacheRepository.InvalidateUserRoles
func (r *ValkeyRBACRepository) InvalidateUserRoles(ctx context.Context, tenantID, userID string) error {
	return r.InvalidateUserRolesCache(ctx, tenantID, userID)
}

// SetPermissions implements CacheRepository.SetPermissions
func (r *ValkeyRBACRepository) SetPermissions(ctx context.Context, tenantID string, permissions []*models.Permission, ttl time.Duration) error {
	// For now, cache permissions under a generic "permissions" key
	// In a real implementation, you might want to cache by resource type
	return r.CachePermissions(ctx, tenantID, "all", permissions, ttl)
}

// GetPermissions implements CacheRepository.GetPermissions
func (r *ValkeyRBACRepository) GetPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return r.GetCachedPermissions(ctx, tenantID, "all")
}

// InvalidatePermissions implements CacheRepository.InvalidatePermissions
func (r *ValkeyRBACRepository) InvalidatePermissions(ctx context.Context, tenantID string) error {
	return r.InvalidatePermissionsCache(ctx, tenantID, "all")
}

// NoOpCacheRepository is a simple no-op implementation of CacheRepository for when caching is disabled
type NoOpCacheRepository struct{}

func NewNoOpCacheRepository() *NoOpCacheRepository {
	return &NoOpCacheRepository{}
}

func (r *NoOpCacheRepository) SetRole(ctx context.Context, tenantID, roleName string, role *models.Role, ttl time.Duration) error {
	return nil
}

func (r *NoOpCacheRepository) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, fmt.Errorf("cache miss")
}

func (r *NoOpCacheRepository) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}

func (r *NoOpCacheRepository) InvalidateTenantRoles(ctx context.Context, tenantID string) error {
	return nil
}

func (r *NoOpCacheRepository) SetUserRoles(ctx context.Context, tenantID, userID string, roles []string, ttl time.Duration) error {
	return nil
}

func (r *NoOpCacheRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, fmt.Errorf("cache miss")
}

func (r *NoOpCacheRepository) DeleteUserRoles(ctx context.Context, tenantID, userID string) error {
	return nil
}

func (r *NoOpCacheRepository) InvalidateUserRoles(ctx context.Context, tenantID, userID string) error {
	return nil
}

func (r *NoOpCacheRepository) SetPermissions(ctx context.Context, tenantID string, permissions []*models.Permission, ttl time.Duration) error {
	return nil
}

func (r *NoOpCacheRepository) GetPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, fmt.Errorf("cache miss")
}

func (r *NoOpCacheRepository) InvalidatePermissions(ctx context.Context, tenantID string) error {
	return nil
}
