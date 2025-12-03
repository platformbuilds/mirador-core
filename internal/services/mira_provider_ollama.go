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

// OllamaProvider implements MIRAService using Ollama's local API.
type OllamaProvider struct {
	endpoint  string
	model     string
	maxTokens int
	timeout   time.Duration
	logger    logging.Logger
	client    *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(cfg config.OllamaConfig, timeout time.Duration, logger logging.Logger) (*OllamaProvider, error) {
	endpoint := resolveEnvVar(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "http://localhost:11434/api/generate" // Default fallback
	}

	return &OllamaProvider{
		endpoint:  endpoint,
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
		timeout:   timeout,
		logger:    logger,
		client:    &http.Client{Timeout: timeout},
	}, nil
}

// GenerateExplanation generates a MIRA explanation using Ollama.
func (p *OllamaProvider) GenerateExplanation(ctx context.Context, prompt string) (*MIRAResponse, error) {
	p.logger.Info("Calling Ollama API", "model", p.model, "endpoint", p.endpoint)

	// Ollama generate API
	reqBody := map[string]interface{}{
		"model":  p.model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"num_predict": p.maxTokens,
			"temperature": 0.7,
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

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("Ollama API call failed", "error", err)
		return nil, fmt.Errorf("Ollama API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Error("Ollama API error", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	var result struct {
		Response string `json:"response"`
		Context  []int  `json:"context"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Response == "" {
		return nil, fmt.Errorf("Ollama returned empty response")
	}

	// Ollama doesn't always return token counts, estimate from context length
	tokensUsed := len(result.Context)
	if tokensUsed == 0 {
		tokensUsed = len(prompt)/4 + len(result.Response)/4 // rough estimate
	}

	p.logger.Info("Ollama API call successful", "estimated_tokens", tokensUsed)

	return &MIRAResponse{
		Explanation: result.Response,
		TokensUsed:  tokensUsed,
		Model:       p.model,
		Provider:    "ollama",
		GeneratedAt: time.Now().UTC(),
		Cached:      false,
	}, nil
}

// GetProviderName returns "ollama".
func (p *OllamaProvider) GetProviderName() string {
	return "ollama"
}

// GetModelName returns the Ollama model name.
func (p *OllamaProvider) GetModelName() string {
	return p.model
}
