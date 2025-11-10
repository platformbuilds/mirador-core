package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// RuntimeFeatureFlags represents the runtime-togglable feature flags
type RuntimeFeatureFlags struct {
	RCAEnabled          bool `json:"rca_enabled" yaml:"rca_enabled"`
	UserSettingsEnabled bool `json:"user_settings_enabled" yaml:"user_settings_enabled"`
	RBACEnabled         bool `json:"rbac_enabled" yaml:"rbac_enabled"`

	// RBAC-specific feature flags for gradual rollout
	RBACMode           RBACMode `json:"rbac_mode" yaml:"rbac_mode"`                       // "disabled", "audit_only", "enforced"
	RBACLegacyFallback bool     `json:"rbac_legacy_fallback" yaml:"rbac_legacy_fallback"` // Fall back to cache-only if service fails
	RBACAuditOnly      bool     `json:"rbac_audit_only" yaml:"rbac_audit_only"`           // Deprecated: use RBACMode instead
}

// RBACMode defines the operational mode for RBAC
type RBACMode string

const (
	RBACModeDisabled  RBACMode = "disabled"   // RBAC completely disabled
	RBACModeAuditOnly RBACMode = "audit_only" // RBAC operations logged but not enforced
	RBACModeEnforced  RBACMode = "enforced"   // RBAC fully enforced
)

// RuntimeFeatureFlagService manages runtime feature flags stored in cache
type RuntimeFeatureFlagService struct {
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewRuntimeFeatureFlagService creates a new runtime feature flag service
func NewRuntimeFeatureFlagService(cache cache.ValkeyCluster, logger logger.Logger) *RuntimeFeatureFlagService {
	return &RuntimeFeatureFlagService{
		cache:  cache,
		logger: logger,
	}
}

// GetFeatureFlags retrieves the current runtime feature flags for a tenant
func (s *RuntimeFeatureFlagService) GetFeatureFlags(ctx context.Context, tenantID string) (*RuntimeFeatureFlags, error) {
	cacheKey := fmt.Sprintf("runtime_features:%s", tenantID)

	// Try to get from cache first
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && len(cached) > 0 {
		var flags RuntimeFeatureFlags
		if err := json.Unmarshal(cached, &flags); err == nil {
			return &flags, nil
		}
		s.logger.Warn("Failed to unmarshal cached feature flags, using defaults", "error", err)
	}

	// Return defaults if not found in cache
	defaults := s.getDefaultFeatureFlags()
	return defaults, nil
}

// SetFeatureFlags updates the runtime feature flags for a tenant
func (s *RuntimeFeatureFlagService) SetFeatureFlags(ctx context.Context, tenantID string, flags *RuntimeFeatureFlags) error {
	cacheKey := fmt.Sprintf("runtime_features:%s", tenantID)

	// Serialize to JSON
	data, err := json.Marshal(flags)
	if err != nil {
		return fmt.Errorf("failed to marshal feature flags: %w", err)
	}

	// Store in cache with no expiration (runtime flags persist until explicitly changed)
	if err := s.cache.Set(ctx, cacheKey, data, 0); err != nil {
		return fmt.Errorf("failed to store feature flags in cache: %w", err)
	}

	s.logger.Info("Runtime feature flags updated", "tenantID", tenantID, "flags", flags)
	return nil
}

// UpdateFeatureFlag updates a single feature flag for a tenant
func (s *RuntimeFeatureFlagService) UpdateFeatureFlag(ctx context.Context, tenantID, flagName string, enabled bool) error {
	flags, err := s.GetFeatureFlags(ctx, tenantID)
	if err != nil {
		return err
	}

	// Update the specific flag
	switch flagName {
	case "rca_enabled":
		flags.RCAEnabled = enabled
	case "user_settings_enabled":
		flags.UserSettingsEnabled = enabled
	case "rbac_enabled":
		flags.RBACEnabled = enabled
	case "rbac_legacy_fallback":
		flags.RBACLegacyFallback = enabled
	case "rbac_audit_only":
		if enabled {
			flags.RBACMode = RBACModeAuditOnly
		} else {
			flags.RBACMode = RBACModeEnforced
		}
	default:
		return fmt.Errorf("unknown feature flag: %s", flagName)
	}

	return s.SetFeatureFlags(ctx, tenantID, flags)
}

// UpdateRBACMode updates the RBAC operational mode for a tenant
func (s *RuntimeFeatureFlagService) UpdateRBACMode(ctx context.Context, tenantID string, mode RBACMode) error {
	flags, err := s.GetFeatureFlags(ctx, tenantID)
	if err != nil {
		return err
	}

	flags.RBACMode = mode
	return s.SetFeatureFlags(ctx, tenantID, flags)
}

// IsRBACEnforced checks if RBAC should be enforced for the tenant
func (s *RuntimeFeatureFlagService) IsRBACEnforced(ctx context.Context, tenantID string) (bool, error) {
	flags, err := s.GetFeatureFlags(ctx, tenantID)
	if err != nil {
		return false, err
	}

	return flags.RBACMode == RBACModeEnforced, nil
}

// IsRBACAuditOnly checks if RBAC is in audit-only mode for the tenant
func (s *RuntimeFeatureFlagService) IsRBACAuditOnly(ctx context.Context, tenantID string) (bool, error) {
	flags, err := s.GetFeatureFlags(ctx, tenantID)
	if err != nil {
		return false, err
	}

	return flags.RBACMode == RBACModeAuditOnly, nil
}

// ShouldUseRBACLegacyFallback checks if legacy fallback should be used
func (s *RuntimeFeatureFlagService) ShouldUseRBACLegacyFallback(ctx context.Context, tenantID string) (bool, error) {
	flags, err := s.GetFeatureFlags(ctx, tenantID)
	if err != nil {
		return false, err
	}

	return flags.RBACLegacyFallback, nil
}

// ResetFeatureFlags resets feature flags to defaults for a tenant
func (s *RuntimeFeatureFlagService) ResetFeatureFlags(ctx context.Context, tenantID string) error {
	defaults := s.getDefaultFeatureFlags()
	return s.SetFeatureFlags(ctx, tenantID, defaults)
}

// getDefaultFeatureFlags returns the default runtime feature flags
func (s *RuntimeFeatureFlagService) getDefaultFeatureFlags() *RuntimeFeatureFlags {
	// All features enabled by default in development
	return &RuntimeFeatureFlags{
		RCAEnabled:          true,
		UserSettingsEnabled: true,
		RBACEnabled:         true,
		RBACMode:            RBACModeEnforced, // Default to enforced in production
		RBACLegacyFallback:  true,             // Enable legacy fallback by default during rollout
		RBACAuditOnly:       false,            // Deprecated field
	}
}
