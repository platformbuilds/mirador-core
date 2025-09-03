// ================================
// internal/services/victoria_metrics.service.go - VictoriaMetrics Integration
// ================================

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type VictoriaMetricsService struct {
	endpoints []string
	timeout   time.Duration
	client    *http.Client
	logger    logger.Logger
	current   int // For round-robin load balancing
}

func NewVictoriaMetricsService(cfg config.VictoriaMetricsConfig, logger logger.Logger) *VictoriaMetricsService {
	return &VictoriaMetricsService{
		endpoints: cfg.Endpoints,
		timeout:   time.Duration(cfg.Timeout) * time.Millisecond,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
		},
		logger: logger,
	}
}

func (s *VictoriaMetricsService) ExecuteQuery(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	start := time.Now()

	// Select endpoint with load balancing
	endpoint := s.selectEndpoint()

	// Build query parameters
	params := url.Values{}
	params.Set("query", request.Query)
	if request.Time != "" {
		params.Set("time", request.Time)
	}
	if request.Timeout != "" {
		params.Set("timeout", request.Timeout)
	}

	// Create HTTP request
	fullURL := fmt.Sprintf("%s/select/0/prometheus/api/v1/query?%s", endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	// Add tenant header for multi-tenancy
	if request.TenantID != "" {
		req.Header.Set("AccountID", request.TenantID)
	}
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VictoriaMetrics request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VictoriaMetrics returned status %d", resp.StatusCode)
	}

	// Parse response
	var vmResponse models.VictoriaMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&vmResponse); err != nil {
		return nil, fmt.Errorf("failed to parse VictoriaMetrics response: %w", err)
	}

	executionTime := time.Since(start)

	result := &models.MetricsQLQueryResult{
		Status:        vmResponse.Status,
		Data:          vmResponse.Data,
		SeriesCount:   countSeries(vmResponse.Data),
		ExecutionTime: executionTime.Milliseconds(),
	}

	s.logger.Info("MetricsQL query executed successfully",
		"query", request.Query,
		"endpoint", endpoint,
		"executionTime", executionTime,
		"seriesCount", result.SeriesCount,
		"tenant", request.TenantID,
	)

	return result, nil
}

func (s *VictoriaMetricsService) ExecuteRangeQuery(ctx context.Context, request *models.MetricsQLRangeQueryRequest) (*models.MetricsQLRangeQueryResult, error) {
	endpoint := s.selectEndpoint()

	params := url.Values{}
	params.Set("query", request.Query)
	params.Set("start", request.Start)
	params.Set("end", request.End)
	params.Set("step", request.Step)

	fullURL := fmt.Sprintf("%s/select/0/prometheus/api/v1/query_range?%s", endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if request.TenantID != "" {
		req.Header.Set("AccountID", request.TenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var vmResponse models.VictoriaMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&vmResponse); err != nil {
		return nil, err
	}

	return &models.MetricsQLRangeQueryResult{
		Status:         vmResponse.Status,
		Data:           vmResponse.Data,
		DataPointCount: countDataPoints(vmResponse.Data),
	}, nil
}

func (s *VictoriaMetricsService) GetSeries(ctx context.Context, request *models.SeriesRequest) ([]map[string]string, error) {
	endpoint := s.selectEndpoint()

	params := url.Values{}
	for _, match := range request.Match {
		params.Add("match[]", match)
	}
	if request.Start != "" {
		params.Set("start", request.Start)
	}
	if request.End != "" {
		params.Set("end", request.End)
	}

	fullURL := fmt.Sprintf("%s/select/0/prometheus/api/v1/series?%s", endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if request.TenantID != "" {
		req.Header.Set("AccountID", request.TenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var vmResponse struct {
		Status string              `json:"status"`
		Data   []map[string]string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&vmResponse); err != nil {
		return nil, err
	}

	return vmResponse.Data, nil
}

func (s *VictoriaMetricsService) HealthCheck(ctx context.Context) error {
	endpoint := s.selectEndpoint()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("VictoriaMetrics health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// selectEndpoint implements round-robin load balancing
func (s *VictoriaMetricsService) selectEndpoint() string {
	endpoint := s.endpoints[s.current%len(s.endpoints)]
	s.current++
	return endpoint
}

func (s *VictoriaMetricsService) GetLabels(ctx context.Context, request *models.LabelsRequest) ([]string, error) {
	endpoint := s.selectEndpoint()

	params := url.Values{}
	if request.Start != "" {
		params.Set("start", request.Start)
	}
	if request.End != "" {
		params.Set("end", request.End)
	}
	for _, match := range request.Match {
		params.Add("match[]", match)
	}

	fullURL := fmt.Sprintf("%s/select/0/prometheus/api/v1/labels?%s", endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if request.TenantID != "" {
		req.Header.Set("AccountID", request.TenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var vmResponse struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&vmResponse); err != nil {
		return nil, err
	}

	return vmResponse.Data, nil
}

func (s *VictoriaMetricsService) GetLabelValues(ctx context.Context, request *models.LabelValuesRequest) ([]string, error) {
	endpoint := s.selectEndpoint()

	params := url.Values{}
	if request.Start != "" {
		params.Set("start", request.Start)
	}
	if request.End != "" {
		params.Set("end", request.End)
	}
	for _, match := range request.Match {
		params.Add("match[]", match)
	}

	fullURL := fmt.Sprintf("%s/select/0/prometheus/api/v1/label/%s/values?%s", endpoint, request.Label, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if request.TenantID != "" {
		req.Header.Set("AccountID", request.TenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var vmResponse struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&vmResponse); err != nil {
		return nil, err
	}

	return vmResponse.Data, nil
}
