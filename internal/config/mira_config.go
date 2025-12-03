package config

import "time"

// MIRAConfig contains configuration for the MIRA (Mirador Intelligent Research Assistant) RCA analysis feature.
type MIRAConfig struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled"`
	Provider string `mapstructure:"provider" yaml:"provider"` // "openai" | "anthropic" | "vllm" | "ollama"

	// Provider-specific configurations
	OpenAI    OpenAIConfig    `mapstructure:"openai" yaml:"openai"`
	Anthropic AnthropicConfig `mapstructure:"anthropic" yaml:"anthropic"`
	VLLM      VLLMConfig      `mapstructure:"vllm" yaml:"vllm"`
	Ollama    OllamaConfig    `mapstructure:"ollama" yaml:"ollama"`

	// General MIRA configuration
	Timeout    time.Duration `mapstructure:"timeout" yaml:"timeout"`
	Retries    int           `mapstructure:"retries" yaml:"retries"`
	RetryDelay time.Duration `mapstructure:"retry_delay" yaml:"retry_delay"`

	// Caching configuration
	Cache CacheStrategyConfig `mapstructure:"cache_strategy" yaml:"cache_strategy"`

	// Rate limiting (overrides default for MIRA endpoints)
	RateLimit RateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit"`

	// Prompt template for generating explanations
	PromptTemplate string `mapstructure:"prompt_template" yaml:"prompt_template"`
}

// OpenAIConfig contains OpenAI-specific configuration.
type OpenAIConfig struct {
	Endpoint    string  `mapstructure:"endpoint" yaml:"endpoint"`
	APIKey      string  `mapstructure:"api_key" yaml:"api_key"` // Can use ${ENV_VAR} syntax
	Model       string  `mapstructure:"model" yaml:"model"`
	MaxTokens   int     `mapstructure:"max_tokens" yaml:"max_tokens"`
	Temperature float32 `mapstructure:"temperature" yaml:"temperature"`
}

// AnthropicConfig contains Anthropic-specific configuration.
type AnthropicConfig struct {
	Endpoint  string `mapstructure:"endpoint" yaml:"endpoint"`
	APIKey    string `mapstructure:"api_key" yaml:"api_key"` // Can use ${ENV_VAR} syntax
	Model     string `mapstructure:"model" yaml:"model"`
	MaxTokens int    `mapstructure:"max_tokens" yaml:"max_tokens"`
}

// VLLMConfig contains vLLM-specific configuration.
type VLLMConfig struct {
	Endpoint  string `mapstructure:"endpoint" yaml:"endpoint"`
	Model     string `mapstructure:"model" yaml:"model"`
	MaxTokens int    `mapstructure:"max_tokens" yaml:"max_tokens"`
}

// OllamaConfig contains Ollama-specific configuration.
type OllamaConfig struct {
	Endpoint  string `mapstructure:"endpoint" yaml:"endpoint"`
	Model     string `mapstructure:"model" yaml:"model"`
	MaxTokens int    `mapstructure:"max_tokens" yaml:"max_tokens"`
}

// CacheStrategyConfig defines caching behavior for MIRA responses.
type CacheStrategyConfig struct {
	Enabled                      bool `mapstructure:"enabled" yaml:"enabled"`
	TTL                          int  `mapstructure:"ttl" yaml:"ttl"` // seconds
	UseValkeyForFastCache        bool `mapstructure:"use_valkey_for_fast_cache" yaml:"use_valkey_for_fast_cache"`
	UseWeaviateForSemanticSearch bool `mapstructure:"use_weaviate_for_semantic_search" yaml:"use_weaviate_for_semantic_search"`
}

// RateLimitConfig defines rate limiting for MIRA endpoints.
type RateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled" yaml:"enabled"`
	RequestsPerMinute int  `mapstructure:"requests_per_minute" yaml:"requests_per_minute"`
	BurstSize         int  `mapstructure:"burst_size" yaml:"burst_size"`
}

// GetMIRAConfig returns the MIRA configuration from the main Config struct.
// Returns default configuration if MIRA section is not present.
func (c *Config) GetMIRAConfig() MIRAConfig {
	// Return MIRA config if present, otherwise return defaults
	return c.MIRA
}

// DefaultMIRAConfig returns sensible defaults for MIRA configuration.
func DefaultMIRAConfig() MIRAConfig {
	return MIRAConfig{
		Enabled:  false, // Disabled by default, must be explicitly enabled
		Provider: "ollama",
		OpenAI: OpenAIConfig{
			Endpoint:    "https://api.openai.com/v1/chat/completions",
			APIKey:      "${OPENAI_API_KEY}",
			Model:       "gpt-4",
			MaxTokens:   2000,
			Temperature: 0.7,
		},
		Anthropic: AnthropicConfig{
			Endpoint:  "https://api.anthropic.com/v1/messages",
			APIKey:    "${ANTHROPIC_API_KEY}",
			Model:     "claude-3-5-sonnet-20241022",
			MaxTokens: 2000,
		},
		VLLM: VLLMConfig{
			Endpoint:  "http://localhost:8000/v1/completions",
			Model:     "meta-llama/Llama-3.1-70B-Instruct",
			MaxTokens: 2000,
		},
		Ollama: OllamaConfig{
			Endpoint:  "http://localhost:11434/api/generate",
			Model:     "llama3.1:70b",
			MaxTokens: 2000,
		},
		Timeout:    30 * time.Second,
		Retries:    3,
		RetryDelay: 2 * time.Second,
		Cache: CacheStrategyConfig{
			Enabled:                      true,
			TTL:                          3600, // 1 hour
			UseValkeyForFastCache:        true,
			UseWeaviateForSemanticSearch: false, // Disabled by default (requires setup)
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 10,
			BurstSize:         5,
		},
		PromptTemplate: defaultPromptTemplate(),
	}
}

func defaultPromptTemplate() string {
	return `You are MIRA (Mirador Intelligent Research Assistant), an expert at translating complex technical incidents into comprehensive, detailed narratives.

Your task: Provide a THOROUGH, DETAILED analysis that preserves ALL technical information while making it understandable.
{{- if .TimeRings}}
Timing context: {{.TimeRings}}
{{- end}}

{{.TOONData}}

IMPORTANT INSTRUCTIONS:
- Be VERBOSE and COMPREHENSIVE - this is a detailed technical report, not a brief summary
- Mention ALL service names, KPI names, metric names explicitly by name
- Include ALL scores, correlation values, and data points mentioned in the data
- Explain EVERY step and relationship thoroughly
- Use plain business language, but DO NOT omit technical details - translate them instead
- Provide specific examples and concrete data points wherever available
- Explain the "why" and "how" behind each relationship

Required sections:
1. DETAILED INCIDENT DESCRIPTION: Comprehensive explanation of what happened, including all services and metrics involved
2. USER IMPACT ANALYSIS: Thorough description of business consequences with specific data
3. ROOT CAUSE EXPLANATION: Complete technical explanation in plain language, naming all components
4. CAUSAL RELATIONSHIPS: Detailed explanation of how the root cause led to the impact
5. EVIDENCE & DATA: All supporting metrics, scores, and correlation data
6. PREVENTION RECOMMENDATIONS: Specific, actionable steps

Do not summarize - provide the FULL, DETAILED analysis.`
}
