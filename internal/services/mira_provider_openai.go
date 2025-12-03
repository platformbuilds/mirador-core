package services

import (
	"context"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements MIRAService using OpenAI's API.
type OpenAIProvider struct {
	client      *openai.Client
	model       string
	maxTokens   int
	temperature float32
	timeout     time.Duration
	logger      logging.Logger
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(cfg config.OpenAIConfig, timeout time.Duration, logger logging.Logger) (*OpenAIProvider, error) {
	// Resolve API key from environment variable if needed
	apiKey := resolveEnvVar(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(apiKey)

	return &OpenAIProvider{
		client:      client,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		timeout:     timeout,
		logger:      logger,
	}, nil
}

// GenerateExplanation generates a MIRA explanation using OpenAI.
func (p *OpenAIProvider) GenerateExplanation(ctx context.Context, prompt string) (*MIRAResponse, error) {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	p.logger.Info("Calling OpenAI API", "model", p.model, "max_tokens", p.maxTokens)

	req := openai.ChatCompletionRequest{
		Model:       p.model,
		MaxTokens:   p.maxTokens,
		Temperature: p.temperature,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	resp, err := p.client.CreateChatCompletion(timeoutCtx, req)
	if err != nil {
		p.logger.Error("OpenAI API call failed", "error", err)
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	explanation := resp.Choices[0].Message.Content

	p.logger.Info("OpenAI API call successful",
		"tokens_used", resp.Usage.TotalTokens,
		"explanation_length", len(explanation))

	return &MIRAResponse{
		Explanation: explanation,
		TokensUsed:  resp.Usage.TotalTokens,
		Model:       p.model,
		Provider:    "openai",
		GeneratedAt: time.Now().UTC(),
		Cached:      false,
	}, nil
}

// GetProviderName returns "openai".
func (p *OpenAIProvider) GetProviderName() string {
	return "openai"
}

// GetModelName returns the OpenAI model name.
func (p *OpenAIProvider) GetModelName() string {
	return p.model
}
