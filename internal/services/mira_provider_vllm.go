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

// VLLMProvider implements MIRAService using vLLM (OpenAI-compatible API).
type VLLMProvider struct {
	endpoint  string
	model     string
	maxTokens int
	timeout   time.Duration
	logger    logging.Logger
	client    *http.Client
}

// NewVLLMProvider creates a new vLLM provider.
func NewVLLMProvider(cfg config.VLLMConfig, timeout time.Duration, logger logging.Logger) (*VLLMProvider, error) {
	return &VLLMProvider{
		endpoint:  cfg.Endpoint,
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
		timeout:   timeout,
		logger:    logger,
		client:    &http.Client{Timeout: timeout},
	}, nil
}

// GenerateExplanation generates a MIRA explanation using vLLM.
func (p *VLLMProvider) GenerateExplanation(ctx context.Context, prompt string) (*MIRAResponse, error) {
	p.logger.Info("Calling vLLM API", "model", p.model, "endpoint", p.endpoint)

	// vLLM uses OpenAI-compatible completions API
	reqBody := map[string]interface{}{
		"model":       p.model,
		"prompt":      prompt,
		"max_tokens":  p.maxTokens,
		"temperature": 0.7,
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

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("vLLM API call failed", "error", err)
		return nil, fmt.Errorf("vLLM API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Error("vLLM API error", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("vLLM API returned status %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Text string `json:"text"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("vLLM returned no choices")
	}

	explanation := result.Choices[0].Text

	p.logger.Info("vLLM API call successful", "tokens_used", result.Usage.TotalTokens)

	return &MIRAResponse{
		Explanation: explanation,
		TokensUsed:  result.Usage.TotalTokens,
		Model:       p.model,
		Provider:    "vllm",
		GeneratedAt: time.Now().UTC(),
		Cached:      false,
	}, nil
}

// GetProviderName returns "vllm".
func (p *VLLMProvider) GetProviderName() string {
	return "vllm"
}

// GetModelName returns the vLLM model name.
func (p *VLLMProvider) GetModelName() string {
	return p.model
}
