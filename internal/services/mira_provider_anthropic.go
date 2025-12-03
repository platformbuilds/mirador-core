package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
)

// AnthropicProvider implements MIRAService using Anthropic's Claude API.
type AnthropicProvider struct {
	endpoint  string
	apiKey    string
	model     string
	maxTokens int
	timeout   time.Duration
	logger    logging.Logger
	client    *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(cfg config.AnthropicConfig, timeout time.Duration, logger logging.Logger) (*AnthropicProvider, error) {
	apiKey := resolveEnvVar(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}

	return &AnthropicProvider{
		endpoint:  cfg.Endpoint,
		apiKey:    apiKey,
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
		timeout:   timeout,
		logger:    logger,
		client:    &http.Client{Timeout: timeout},
	}, nil
}

// GenerateExplanation generates a MIRA explanation using Anthropic Claude.
func (p *AnthropicProvider) GenerateExplanation(ctx context.Context, prompt string) (*MIRAResponse, error) {
	p.logger.Info("Calling Anthropic API", "model", p.model)

	reqBody := map[string]interface{}{
		"model":      p.model,
		"max_tokens": p.maxTokens,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("Anthropic API call failed", "error", err)
		return nil, fmt.Errorf("Anthropic API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Error("Anthropic API error", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("Anthropic API returned status %d", resp.StatusCode)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("Anthropic returned no content")
	}

	totalTokens := result.Usage.InputTokens + result.Usage.OutputTokens

	p.logger.Info("Anthropic API call successful", "tokens_used", totalTokens)

	return &MIRAResponse{
		Explanation: result.Content[0].Text,
		TokensUsed:  totalTokens,
		Model:       p.model,
		Provider:    "anthropic",
		GeneratedAt: time.Now().UTC(),
		Cached:      false,
	}, nil
}

// GetProviderName returns "anthropic".
func (p *AnthropicProvider) GetProviderName() string {
	return "anthropic"
}

// GetModelName returns the Anthropic model name.
func (p *AnthropicProvider) GetModelName() string {
	return p.model
}
