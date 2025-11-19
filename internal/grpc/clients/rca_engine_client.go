package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// RCAEngineClient wraps the REST client for RCA-ENGINE
type RCAEngineClient struct {
	baseURL string
	client  *http.Client
	logger  logger.Logger
}

// NewRCAEngineClient creates a new RCA-ENGINE REST client
func NewRCAEngineClient(endpoint string, log logger.Logger) (*RCAEngineClient, error) {
	return &RCAEngineClient{
		baseURL: endpoint,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: log,
	}, nil
}

// InvestigateIncident starts an RCA investigation via REST API
func (c *RCAEngineClient) InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
	// Convert request to JSON
	requestBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal investigation request: %w", err)
	}

	// Create HTTP request
	endpointURL := fmt.Sprintf("%s/api/v1/investigate", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpointURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call RCA-ENGINE investigate endpoint: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RCA-ENGINE investigate failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result models.CorrelationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode investigation response: %w", err)
	}

	c.logger.Info("RCA investigation completed successfully", "incident", req.IncidentID, "correlation", result.CorrelationID)
	return &result, nil
}

// ListCorrelations retrieves active correlations via REST API
func (c *RCAEngineClient) ListCorrelations(ctx context.Context, req *models.ListCorrelationsRequest) (*models.ListCorrelationsResponse, error) {
	// Build query parameters
	params := url.Values{}
	if req.Service != "" {
		params.Add("service", req.Service)
	}
	if req.StartTime != nil {
		params.Add("start", req.StartTime.Format(time.RFC3339))
	}
	if req.EndTime != nil {
		params.Add("end", req.EndTime.Format(time.RFC3339))
	}
	if req.PageSize > 0 {
		params.Add("page_size", strconv.Itoa(int(req.PageSize)))
	}
	if req.PageToken != "" {
		params.Add("page_token", req.PageToken)
	}

	// Create HTTP request
	endpointURL := fmt.Sprintf("%s/api/v1/correlations?%s", c.baseURL, params.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpointURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Make the request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call RCA-ENGINE correlations endpoint: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RCA-ENGINE correlations failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response models.ListCorrelationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode correlations response: %w", err)
	}

	c.logger.Info("Retrieved correlations successfully", "count", len(response.Correlations))
	return &response, nil
}

// GetPatterns retrieves known failure patterns via REST API
func (c *RCAEngineClient) GetPatterns(ctx context.Context, req *models.GetPatternsRequest) (*models.GetPatternsResponse, error) {
	// Build query parameters
	params := url.Values{}
	if req.Service != "" {
		params.Add("service", req.Service)
	}

	// Create HTTP request
	endpointURL := fmt.Sprintf("%s/api/v1/patterns?%s", c.baseURL, params.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpointURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Make the request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call RCA-ENGINE patterns endpoint: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RCA-ENGINE patterns failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response - mirador-rca returns {"patterns": [...]}
	var response struct {
		Patterns []models.Pattern `json:"patterns"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode patterns response: %w", err)
	}

	c.logger.Info("Retrieved patterns successfully", "count", len(response.Patterns))
	return &models.GetPatternsResponse{Patterns: response.Patterns}, nil
}

// SubmitFeedback submits feedback on correlation accuracy via REST API
func (c *RCAEngineClient) SubmitFeedback(ctx context.Context, req *models.FeedbackRequest) (*models.FeedbackResponse, error) {
	// Convert request to JSON
	feedback := map[string]interface{}{
		"correlation_id": req.CorrelationID,
		"correct":        req.Correct,
		"notes":          req.Notes,
		"submitted_at":   time.Now(),
	}

	requestBody, err := json.Marshal(feedback)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal feedback request: %w", err)
	}

	// Create HTTP request
	endpointURL := fmt.Sprintf("%s/api/v1/feedback", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpointURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call RCA-ENGINE feedback endpoint: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RCA-ENGINE feedback failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response - mirador-rca returns {"correlation_id": "...", "accepted": true}
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode feedback response: %w", err)
	}

	c.logger.Info("Feedback submitted successfully", "correlation", req.CorrelationID)
	return &models.FeedbackResponse{
		CorrelationID: req.CorrelationID,
		Accepted:      true,
	}, nil
}

// HealthCheck checks the health of the RCA-ENGINE via REST API
func (c *RCAEngineClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create HTTP request
	endpointURL := fmt.Sprintf("%s/health", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpointURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Make the request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to call RCA-ENGINE health endpoint: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("RCA-ENGINE health check failed with status %d", resp.StatusCode)
	}

	// Parse response - mirador-rca returns {"status": "healthy"}
	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode health response: %w", err)
	}

	if response["status"] != "healthy" {
		return fmt.Errorf("RCA-ENGINE reported unhealthy status: %s", response["status"])
	}

	return nil
}

// Close closes the HTTP client (no-op for REST client)
func (c *RCAEngineClient) Close() error {
	return nil
}

// UpdateEndpoint updates the REST endpoint
func (c *RCAEngineClient) UpdateEndpoint(endpoint string) error {
	c.baseURL = endpoint
	c.logger.Info("Successfully updated RCA-ENGINE endpoint", "endpoint", endpoint)
	return nil
}
