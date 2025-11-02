package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// DynamicGRPCConfig represents the configurable gRPC endpoint settings
type DynamicGRPCConfig struct {
	RCAEngine   RCAEngineConfig   `json:"rca_engine"`
	AlertEngine AlertEngineConfig `json:"alert_engine"`
}

// RCAEngineConfig represents the RCA engine configuration
type RCAEngineConfig struct {
	Endpoint             string  `json:"endpoint"`
	CorrelationThreshold float64 `json:"correlation_threshold"`
	Timeout              int     `json:"timeout"`
}

// AlertEngineConfig represents the alert engine configuration
type AlertEngineConfig struct {
	Endpoint  string `json:"endpoint"`
	RulesPath string `json:"rules_path"`
	Timeout   int    `json:"timeout"`
}

// DynamicConfigService manages dynamic configuration updates stored in cache
type DynamicConfigService struct {
	cache  cache.ValkeyCluster
	logger logger.Logger
}

// NewDynamicConfigService creates a new dynamic configuration service
func NewDynamicConfigService(cache cache.ValkeyCluster, logger logger.Logger) *DynamicConfigService {
	return &DynamicConfigService{
		cache:  cache,
		logger: logger,
	}
}

// GetGRPCConfig retrieves the current gRPC endpoint configuration from cache
// If not found, returns the default configuration from static config
func (s *DynamicConfigService) GetGRPCConfig(ctx context.Context, tenantID string, defaultConfig *config.GRPCConfig) (*DynamicGRPCConfig, error) {
	key := s.getConfigKey(tenantID, "grpc_endpoints")

	// Try to get from cache first
	data, err := s.cache.Get(ctx, key)
	if err != nil {
		s.logger.Warn("Failed to get gRPC config from cache, using defaults", "tenantID", tenantID, "error", err)
		return s.convertToDynamicConfig(defaultConfig), nil
	}

	if data == nil || len(data) == 0 {
		// Not in cache, return defaults
		return s.convertToDynamicConfig(defaultConfig), nil
	}

	// Parse from cache
	var cfg DynamicGRPCConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		s.logger.Error("Failed to unmarshal gRPC config from cache", "tenantID", tenantID, "error", err)
		return s.convertToDynamicConfig(defaultConfig), nil
	}

	return &cfg, nil
}

// SetGRPCConfig updates the gRPC endpoint configuration in cache
func (s *DynamicConfigService) SetGRPCConfig(ctx context.Context, tenantID string, cfg *DynamicGRPCConfig) error {
	key := s.getConfigKey(tenantID, "grpc_endpoints")

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal gRPC config: %w", err)
	}

	// Store with a reasonable TTL (24 hours)
	if err := s.cache.Set(ctx, key, string(data), 24*time.Hour); err != nil {
		return fmt.Errorf("failed to store gRPC config in cache: %w", err)
	}

	s.logger.Info("Updated gRPC endpoint configuration", "tenantID", tenantID)
	return nil
}

// ResetGRPCConfig resets the gRPC configuration to defaults
func (s *DynamicConfigService) ResetGRPCConfig(ctx context.Context, tenantID string, defaultConfig *config.GRPCConfig) error {
	cfg := s.convertToDynamicConfig(defaultConfig)
	return s.SetGRPCConfig(ctx, tenantID, cfg)
}

// convertToDynamicConfig converts static config to dynamic config format
func (s *DynamicConfigService) convertToDynamicConfig(staticConfig *config.GRPCConfig) *DynamicGRPCConfig {
	return &DynamicGRPCConfig{
		RCAEngine: RCAEngineConfig{
			Endpoint:             staticConfig.RCAEngine.Endpoint,
			CorrelationThreshold: staticConfig.RCAEngine.CorrelationThreshold,
			Timeout:              staticConfig.RCAEngine.Timeout,
		},
		AlertEngine: AlertEngineConfig{
			Endpoint:  staticConfig.AlertEngine.Endpoint,
			RulesPath: staticConfig.AlertEngine.RulesPath,
			Timeout:   staticConfig.AlertEngine.Timeout,
		},
	}
}

// getConfigKey generates the cache key for configuration
func (s *DynamicConfigService) getConfigKey(tenantID, configType string) string {
	return fmt.Sprintf("cfg:%s:%s", tenantID, configType)
}
