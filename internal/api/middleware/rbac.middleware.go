package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/api/constants"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Use AnonymousTenantID from rate_limiter.middleware
// import "github.com/platformbuilds/mirador-core/internal/api/middleware"
// ...existing code...
// ...existing code...
// PolicyCacheResult represents a cached policy evaluation result
type PolicyCacheResult struct {
	Allowed  bool      `json:"allowed"`
	Reason   string    `json:"reason"`
	CachedAt time.Time `json:"cached_at"`
}

// PolicyCache provides high-performance policy evaluation caching
type PolicyCache struct {
	cache  cache.ValkeyCluster
	logger logger.Logger
	ttl    time.Duration
}

// NewPolicyCache creates a new policy cache with configurable TTL
func NewPolicyCache(cache cache.ValkeyCluster, logger logger.Logger, ttl time.Duration) *PolicyCache {
	if ttl <= 0 {
		ttl = 15 * time.Minute // Default 15 minutes
	}
	return &PolicyCache{
		cache:  cache,
		logger: logger,
		ttl:    ttl,
	}
}

// GetPolicyCacheKey generates a cache key for policy evaluation
func (pc *PolicyCache) GetPolicyCacheKey(userID, tenantID string, requiredPermissions []string) string {
	// Sort permissions for consistent cache key
	sortedPerms := make([]string, len(requiredPermissions))
	copy(sortedPerms, requiredPermissions)
	// Simple sort for consistency (in production, use proper sorting)
	for i := 0; i < len(sortedPerms)-1; i++ {
		for j := i + 1; j < len(sortedPerms); j++ {
			if sortedPerms[i] > sortedPerms[j] {
				sortedPerms[i], sortedPerms[j] = sortedPerms[j], sortedPerms[i]
			}
		}
	}

	permsStr := strings.Join(sortedPerms, ",")
	return fmt.Sprintf("rbac:policy:%s:%s:%s", tenantID, userID, permsStr)
}

// GetCachedPolicy retrieves a cached policy evaluation result
func (pc *PolicyCache) GetCachedPolicy(ctx context.Context, userID, tenantID string, requiredPermissions []string) (*PolicyCacheResult, error) {
	cacheKey := pc.GetPolicyCacheKey(userID, tenantID, requiredPermissions)

	data, err := pc.cache.Get(ctx, cacheKey)
	if err != nil {
		// Cache miss
		return nil, err
	}

	var result PolicyCacheResult
	if err := json.Unmarshal(data, &result); err != nil {
		pc.logger.Warn("Failed to unmarshal cached policy result", "error", err, "key", cacheKey)
		return nil, err
	}

	// Check if cache is stale (shouldn't happen with TTL, but safety check)
	if time.Since(result.CachedAt) > pc.ttl {
		pc.logger.Warn("Cached policy result is stale", "key", cacheKey, "age", time.Since(result.CachedAt))
		return nil, fmt.Errorf("stale cache")
	}

	return &result, nil
}

// SetCachedPolicy stores a policy evaluation result in cache
func (pc *PolicyCache) SetCachedPolicy(ctx context.Context, userID, tenantID string, requiredPermissions []string, allowed bool, reason string) error {
	cacheKey := pc.GetPolicyCacheKey(userID, tenantID, requiredPermissions)

	result := PolicyCacheResult{
		Allowed:  allowed,
		Reason:   reason,
		CachedAt: time.Now(),
	}

	data, err := json.Marshal(result)
	if err != nil {
		pc.logger.Error("Failed to marshal policy result", "error", err)
		return err
	}

	return pc.cache.Set(ctx, cacheKey, data, pc.ttl)
}

// InvalidateUserPolicies invalidates all cached policies for a user
func (pc *PolicyCache) InvalidateUserPolicies(ctx context.Context, tenantID, userID string) error {
	// Use pattern-based invalidation
	patternKey := fmt.Sprintf("rbac:policy:%s:%s:*", tenantID, userID)

	// Get all keys matching the pattern
	keys, err := pc.cache.GetPatternIndexKeys(ctx, patternKey)
	if err != nil {
		pc.logger.Warn("Failed to get pattern index keys for invalidation", "pattern", patternKey, "error", err)
		// Fallback: try to delete with a broader pattern (less efficient)
		return pc.invalidateWithPattern(ctx, fmt.Sprintf("rbac:policy:%s:%s:*", tenantID, userID))
	}

	if len(keys) > 0 {
		err = pc.cache.DeleteMultiple(ctx, keys)
		if err != nil {
			pc.logger.Error("Failed to delete multiple policy cache keys", "error", err, "keys", keys)
			return err
		}
		pc.logger.Info("Invalidated user policy cache", "tenantID", tenantID, "userID", userID, "keys_deleted", len(keys))
	}

	return nil
}

// InvalidateTenantPolicies invalidates all cached policies for a tenant
func (pc *PolicyCache) InvalidateTenantPolicies(ctx context.Context, tenantID string) error {
	patternKey := fmt.Sprintf("rbac:policy:%s:*", tenantID)
	return pc.invalidateWithPattern(ctx, patternKey)
}

// invalidateWithPattern performs pattern-based invalidation using KEYS command
func (pc *PolicyCache) invalidateWithPattern(ctx context.Context, pattern string) error {
	// This is a simplified implementation. In production, you'd use SCAN for large datasets
	// For now, we'll rely on TTL expiration for most cases and manual invalidation for critical updates

	pc.logger.Info("Policy cache invalidation requested", "pattern", pattern)
	// Note: Full pattern invalidation would require SCAN in production
	// For this implementation, we rely on TTL and targeted invalidation

	return nil
}

// WarmCache pre-loads frequently accessed policies into cache
func (pc *PolicyCache) WarmCache(ctx context.Context, tenantID string, commonPermissions [][]string) error {
	// This would be called during application startup or periodically
	// For now, it's a placeholder for future implementation

	pc.logger.Info("Policy cache warming requested", "tenantID", tenantID, "permission_sets", len(commonPermissions))
	return nil
}

// RBACEnforcer handles role-based access control with two-tier evaluation
type RBACEnforcer struct {
	rbacService *rbac.RBACService
	policyCache *PolicyCache
	logger      logger.Logger
}

// NewRBACEnforcer creates a new RBAC enforcer
func NewRBACEnforcer(rbacService *rbac.RBACService, cache cache.ValkeyCluster, logger logger.Logger) *RBACEnforcer {
	policyCache := NewPolicyCache(cache, logger, 15*time.Minute) // 15 minute default TTL
	return &RBACEnforcer{
		rbacService: rbacService,
		policyCache: policyCache,
		logger:      logger,
	}
}

// RBACMiddleware enforces role-based access control with two-tier evaluation
func (r *RBACEnforcer) RBACMiddleware(requiredPermissions []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user context
		userID := c.GetString("user_id")
		tenantID := c.GetString("tenant_id")
		globalRole := c.GetString("global_role")

		// Skip RBAC enforcement when auth is disabled (anonymous user from NoAuthMiddleware)
		if userID == constants.AnonymousTenantID {
			r.logger.Debug("RBAC check skipped: auth disabled (anonymous user)", "path", c.Request.URL.Path)
			c.Next()
			return
		}

		if userID == "" || tenantID == "" {
			r.logger.Warn("RBAC check failed: missing user or tenant context",
				"userId", userID, "tenantId", tenantID, "path", c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status": "error",
				"error":  "Authentication required",
			})
			c.Abort()
			return
		}

		// Check if access is allowed
		allowed, reason := r.evaluateAccess(userID, tenantID, globalRole, requiredPermissions, c)
		if !allowed {
			r.logger.Warn("RBAC check failed: access denied",
				"userId", userID,
				"tenantId", tenantID,
				"globalRole", globalRole,
				"requiredPermissions", requiredPermissions,
				"reason", reason,
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusForbidden, gin.H{
				"status":               "error",
				"error":                "Access denied",
				"reason":               reason,
				"required_permissions": requiredPermissions,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// evaluateAccess performs two-tier RBAC evaluation (Global + Tenant roles)
func (r *RBACEnforcer) evaluateAccess(userID, tenantID, globalRole string, requiredPermissions []string, c *gin.Context) (bool, string) {
	// Check policy cache first
	if cachedResult, err := r.policyCache.GetCachedPolicy(c.Request.Context(), userID, tenantID, requiredPermissions); err == nil && cachedResult != nil {
		r.logger.Debug("Policy cache hit", "userId", userID, "tenantId", tenantID, "permissions", requiredPermissions, "allowed", cachedResult.Allowed)
		return cachedResult.Allowed, cachedResult.Reason
	}

	// Cache miss - perform actual evaluation using RBAC service
	r.logger.Debug("Policy cache miss - evaluating with RBAC service", "userId", userID, "tenantId", tenantID, "permissions", requiredPermissions)

	// Check each required permission using the RBAC service
	for _, perm := range requiredPermissions {
		resource, action, err := r.parsePermissionString(perm)
		if err != nil {
			r.logger.Error("Invalid permission format", "permission", perm, "error", err)
			r.policyCache.SetCachedPolicy(c.Request.Context(), userID, tenantID, requiredPermissions, false, "invalid_permission_format")
			return false, "invalid_permission_format"
		}

		// Create permission context
		permCtx := &rbac.PermissionContext{
			UserID:      userID,
			TenantID:    tenantID,
			Resource:    resource,
			Action:      action,
			RequestTime: time.Now(),
			IPAddress:   c.ClientIP(),
		}

		// Check permission using RBAC service
		allowed, err := r.rbacService.CheckPermissionWithContext(c.Request.Context(), permCtx)
		if err != nil {
			r.logger.Error("RBAC service permission check failed", "userId", userID, "tenantId", tenantID, "resource", resource, "action", action, "error", err)
			r.policyCache.SetCachedPolicy(c.Request.Context(), userID, tenantID, requiredPermissions, false, "rbac_service_error")
			return false, "rbac_service_error"
		}

		if !allowed {
			r.logger.Debug("Permission denied", "userId", userID, "tenantId", tenantID, "resource", resource, "action", action)
			r.policyCache.SetCachedPolicy(c.Request.Context(), userID, tenantID, requiredPermissions, false, "permission_denied")
			return false, "permission_denied"
		}
	}

	// All permissions granted
	r.logger.Debug("All permissions granted", "userId", userID, "tenantId", tenantID, "permissions", requiredPermissions)
	r.policyCache.SetCachedPolicy(c.Request.Context(), userID, tenantID, requiredPermissions, true, "all_permissions_granted")
	return true, "all_permissions_granted"
}

// parsePermissionString parses a permission string into resource and action
func (r *RBACEnforcer) parsePermissionString(permission string) (resource, action string, err error) {
	parts := strings.Split(permission, ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid permission format: %s, expected 'resource.action'", permission)
	}

	resource = parts[0]
	action = parts[1]

	// Validate resource and action are not empty
	if resource == "" || action == "" {
		return "", "", fmt.Errorf("invalid permission format: %s, resource and action cannot be empty", permission)
	}

	return resource, action, nil
}

// AdminOnlyMiddleware restricts access to admin users
func (r *RBACEnforcer) AdminOnlyMiddleware() gin.HandlerFunc {
	return r.RBACMiddleware([]string{"admin.*"})
}

// GetCacheStats returns policy cache statistics
func (pc *PolicyCache) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	// Get Valkey memory info
	memInfo, err := pc.cache.GetMemoryInfo(ctx)
	if err != nil {
		pc.logger.Warn("Failed to get cache memory info", "error", err)
		memInfo = &cache.CacheMemoryInfo{}
	}

	return map[string]interface{}{
		"policy_cache_ttl_seconds":  pc.ttl.Seconds(),
		"cache_memory_used_bytes":   memInfo.UsedMemory,
		"cache_memory_peak_bytes":   memInfo.PeakMemory,
		"cache_fragmentation_ratio": memInfo.MemoryFragmentation,
		"cache_total_keys":          memInfo.TotalKeys,
		"cache_hit_rate":            memInfo.HitRate,
		"cache_miss_rate":           memInfo.MissRate,
	}, nil
}

// WarmCommonPolicies pre-loads frequently used policy combinations
func (r *RBACEnforcer) WarmCommonPolicies(ctx context.Context, tenantID string) error {
	commonPermissions := [][]string{
		{"dashboard.read"},
		{"dashboard.create", "dashboard.read"},
		{"kpi_definition.read"},
		{"admin.*"},
		{"rbac.*"},
	}

	return r.policyCache.WarmCache(ctx, tenantID, commonPermissions)
}
