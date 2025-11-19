package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// RuntimeFeatureFlags represents the runtime-togglable feature flags
// RBAC and auth-related flags have been removed as authentication is now handled externally
type RuntimeFeatureFlags struct {
	RCAEnabled          bool `json:"rca_enabled" yaml:"rca_enabled"`
	UserSettingsEnabled bool `json:"user_settings_enabled" yaml:"user_settings_enabled"`
}

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

// GetFeatureFlags retrieves the current runtime feature flags
func (s *RuntimeFeatureFlagService) GetFeatureFlags(ctx context.Context) (*RuntimeFeatureFlags, error) {
	cacheKey := "runtime_features:system"

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

// SetFeatureFlags updates the runtime feature flags
func (s *RuntimeFeatureFlagService) SetFeatureFlags(ctx context.Context, flags *RuntimeFeatureFlags) error {
	// Using system-level cache key
	cacheKey := "runtime_features:system"

	// Serialize to JSON
	data, err := json.Marshal(flags)
	if err != nil {
		return fmt.Errorf("failed to marshal feature flags: %w", err)
	}

	// Store in cache with no expiration
	if err := s.cache.Set(ctx, cacheKey, data, 0); err != nil {
		return fmt.Errorf("failed to store feature flags in cache: %w", err)
	}

	s.logger.Info("Runtime feature flags updated", "flags", flags)
	return nil
}

// UpdateFeatureFlag updates a single feature flag
func (s *RuntimeFeatureFlagService) UpdateFeatureFlag(ctx context.Context, flagName string, enabled bool) error {
	flags, err := s.GetFeatureFlags(ctx)
	if err != nil {
		return err
	}

	// Update the specific flag
	switch flagName {
	case "rca_enabled":
		flags.RCAEnabled = enabled
	case "user_settings_enabled":
		flags.UserSettingsEnabled = enabled
	default:
		return fmt.Errorf("unknown feature flag: %s", flagName)
	}

	return s.SetFeatureFlags(ctx, flags)
}

// ResetFeatureFlags resets feature flags to defaults
func (s *RuntimeFeatureFlagService) ResetFeatureFlags(ctx context.Context) error {
	defaults := s.getDefaultFeatureFlags()
	return s.SetFeatureFlags(ctx, defaults)
}

// getDefaultFeatureFlags returns the default runtime feature flags
func (s *RuntimeFeatureFlagService) getDefaultFeatureFlags() *RuntimeFeatureFlags {
	return &RuntimeFeatureFlags{
		RCAEnabled:          true,
		UserSettingsEnabled: true,
	}
}
