package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

// MIRAService defines the interface for MIRA-powered RCA explanation generation.
type MIRAService interface {
	// GenerateExplanation takes a prompt and generates a non-technical explanation.
	GenerateExplanation(ctx context.Context, prompt string) (*MIRAResponse, error)

	// GetProviderName returns the name of the AI provider (e.g., "openai", "anthropic").
	GetProviderName() string

	// GetModelName returns the model name being used.
	GetModelName() string
}

// MIRAResponse contains the MIRA-generated explanation and metadata.
type MIRAResponse struct {
	Explanation string    `json:"explanation"`
	TokensUsed  int       `json:"tokensUsed"`
	Model       string    `json:"model"`
	Provider    string    `json:"provider"`
	GeneratedAt time.Time `json:"generatedAt"`
	Cached      bool      `json:"cached"`
}

// NewMIRAService creates a MIRAService based on configuration.
func NewMIRAService(cfg config.MIRAConfig, logger logging.Logger, valkeyCache cache.ValkeyCluster) (MIRAService, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("MIRA service is disabled in configuration")
	}

	switch cfg.Provider {
	case "openai":
		return NewOpenAIProvider(cfg.OpenAI, cfg.Timeout, logger)
	case "anthropic":
		return NewAnthropicProvider(cfg.Anthropic, cfg.Timeout, logger)
	case "vllm":
		return NewVLLMProvider(cfg.VLLM, cfg.Timeout, logger)
	case "ollama":
		return NewOllamaProvider(cfg.Ollama, cfg.Timeout, logger)
	default:
		return nil, fmt.Errorf("unsupported MIRA provider: %s", cfg.Provider)
	}
}

// CachedMIRAService wraps a MIRAService with caching capabilities.
type CachedMIRAService struct {
	underlying   MIRAService
	valkeyCache  cache.ValkeyCluster
	cacheEnabled bool
	cacheTTL     time.Duration
	logger       logging.Logger
}

// NewCachedMIRAService creates a caching wrapper around a MIRAService.
func NewCachedMIRAService(underlying MIRAService, cfg config.CacheStrategyConfig, valkeyCache cache.ValkeyCluster, logger logging.Logger) *CachedMIRAService {
	return &CachedMIRAService{
		underlying:   underlying,
		valkeyCache:  valkeyCache,
		cacheEnabled: cfg.Enabled && cfg.UseValkeyForFastCache,
		cacheTTL:     time.Duration(cfg.TTL) * time.Second,
		logger:       logger,
	}
}

// GenerateExplanation implements MIRAService with caching.
func (c *CachedMIRAService) GenerateExplanation(ctx context.Context, prompt string) (*MIRAResponse, error) {
	// Generate cache key from prompt
	cacheKey := c.generateCacheKey(prompt)

	// Try cache first
	if c.cacheEnabled && c.valkeyCache != nil {
		if cached, err := c.getFromCache(ctx, cacheKey); err == nil && cached != nil {
			c.logger.Info("AI response cache hit", "cache_key", cacheKey)
			cached.Cached = true
			return cached, nil
		}
	}

	// Cache miss - call underlying MIRA service
	c.logger.Info("MIRA response cache miss, calling provider", "provider", c.underlying.GetProviderName())
	response, err := c.underlying.GenerateExplanation(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if c.cacheEnabled && c.valkeyCache != nil {
		if err := c.storeInCache(ctx, cacheKey, response); err != nil {
			c.logger.Warn("Failed to store MIRA response in cache", "error", err)
		}
	}

	response.Cached = false
	return response, nil
}

// GetProviderName returns the underlying provider name.
func (c *CachedMIRAService) GetProviderName() string {
	return c.underlying.GetProviderName()
}

// GetModelName returns the underlying model name.
func (c *CachedMIRAService) GetModelName() string {
	return c.underlying.GetModelName()
}

// generateCacheKey creates a cache key from the prompt.
func (c *CachedMIRAService) generateCacheKey(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("mira:ai:prompt:%x", hash[:16])
}

// getFromCache retrieves a cached MIRA response.
func (c *CachedMIRAService) getFromCache(ctx context.Context, key string) (*MIRAResponse, error) {
	data, err := c.valkeyCache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var response MIRAResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// storeInCache stores a MIRA response in the cache.
func (c *CachedMIRAService) storeInCache(ctx context.Context, key string, response *MIRAResponse) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return c.valkeyCache.Set(ctx, key, string(data), c.cacheTTL)
}

// GenerateRCACacheKey generates a cache key from an RCA response payload.
// This is used for caching MIRA explanations of identical RCA outputs.
func GenerateRCACacheKey(rca *models.RCAResponse) string {
	// Normalize RCA data for consistent caching
	// Remove timestamp fields that change on every request
	normalized := normalizeRCAForCaching(rca)

	jsonBytes, _ := json.Marshal(normalized)
	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("mira:rca:ai:%x", hash[:16])
}

// normalizeRCAForCaching creates a normalized version of RCA data for cache key generation.
// Removes volatile fields like timestamps and generated IDs.
func normalizeRCAForCaching(rca *models.RCAResponse) map[string]interface{} {
	if rca == nil || rca.Data == nil {
		return map[string]interface{}{}
	}

	normalized := make(map[string]interface{})

	// Include only stable fields that represent the actual incident
	if rca.Data.Impact != nil {
		normalized["impact_service"] = rca.Data.Impact.ImpactService
		normalized["metric_name"] = rca.Data.Impact.MetricName
		normalized["severity"] = rca.Data.Impact.Severity
	}

	if rca.Data.RootCause != nil {
		normalized["root_cause_service"] = rca.Data.RootCause.Service
		normalized["root_cause_component"] = rca.Data.RootCause.Component
	}

	// Include chain count and scores (not full chains to avoid over-specificity)
	if len(rca.Data.Chains) > 0 {
		normalized["chain_count"] = len(rca.Data.Chains)
		if len(rca.Data.Chains) > 0 {
			normalized["top_chain_score"] = rca.Data.Chains[0].Score
		}
	}

	return normalized
}
