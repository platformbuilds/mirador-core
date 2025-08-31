// ================================
// internal/services/victoria_services.go - VictoriaMetrics Services Container
// ================================

package services

import (
	"bytes"
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

// VictoriaMetricsServices contains all VictoriaMetrics ecosystem services
type VictoriaMetricsServices struct {
	Metrics *VictoriaMetricsService
	Logs    *VictoriaLogsService
	Traces  *VictoriaTracesService
}

// VictoriaTracesService handles VictoriaTraces operations
type VictoriaTracesService struct {
	endpoints []string
	timeout   time.Duration
	client    *http.Client
	logger    logger.Logger
	current   int // For round-robin load balancing
}

// NewVictoriaTracesService creates a new VictoriaTraces service
func NewVictoriaTracesService(cfg config.VictoriaTracesConfig, logger logger.Logger) *VictoriaTracesService {
	return &VictoriaTracesService{
		endpoints: cfg.Endpoints,
		timeout:   time.Duration(cfg.Timeout) * time.Millisecond,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
		},
		logger: logger,
	}
}

// GetServices returns all services from VictoriaTraces
func (s *VictoriaTracesService) GetServices(ctx context.Context, tenantID string) ([]string, error) {
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services", endpoint)
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if tenantID != "" {
		req.Header.Set("AccountID", tenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VictoriaTraces request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VictoriaTraces returned status %d", resp.StatusCode)
	}

	var services struct {
		Data []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return nil, fmt.Errorf("failed to parse VictoriaTraces response: %w", err)
	}

	return services.Data, nil
}

// GetTrace retrieves a specific trace by ID
func (s *VictoriaTracesService) GetTrace(ctx context.Context, traceID, tenantID string) (*models.Trace, error) {
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/traces/%s", endpoint, traceID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if tenantID != "" {
		req.Header.Set("AccountID", tenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VictoriaTraces request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("trace not found")
	}

	var trace models.Trace
	if err := json.NewDecoder(resp.Body).Decode(&trace); err != nil {
		return nil, fmt.Errorf("failed to parse trace response: %w", err)
	}

	return &trace, nil
}

// SearchTraces searches for traces with filters
func (s *VictoriaTracesService) SearchTraces(ctx context.Context, request *models.TraceSearchRequest) (*models.TraceSearchResult, error) {
	endpoint := s.selectEndpoint()
	
	params := url.Values{}
	if request.Service != "" {
		params.Set("service", request.Service)
	}
	if request.Operation != "" {
		params.Set("operation", request.Operation)
	}
	if request.Tags != "" {
		params.Set("tags", request.Tags)
	}
	if request.MinDuration != "" {
		params.Set("minDuration", request.MinDuration)
	}
	if request.MaxDuration != "" {
		params.Set("maxDuration", request.MaxDuration)
	}
	if !request.Start.IsZero() {
		params.Set("start", fmt.Sprintf("%d", request.Start.Unix()))
	}
	if !request.End.IsZero() {
		params.Set("end", fmt.Sprintf("%d", request.End.Unix()))
	}
	if request.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", request.Limit))
	}

	fullURL := fmt.Sprintf("%s/select/jaeger/api/traces?%s", endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if request.TenantID != "" {
		req.Header.Set("AccountID", request.TenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VictoriaTraces request failed: %w", err)
	}
	defer resp.Body.Close()

	var response struct {
		Data []map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse VictoriaTraces response: %w", err)
	}

	return &models.TraceSearchResult{
		Traces:     response.Data,
		Total:      len(response.Data),
		SearchTime: 0, // Would be calculated from response time
	}, nil
}

// HealthCheck checks VictoriaTraces health
func (s *VictoriaTracesService) HealthCheck(ctx context.Context) error {
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
		return fmt.Errorf("VictoriaTraces health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// selectEndpoint implements round-robin load balancing
func (s *VictoriaTracesService) selectEndpoint() string {
	endpoint := s.endpoints[s.current%len(s.endpoints)]
	s.current++
	return endpoint
}

// NewVictoriaMetricsServices initializes all VictoriaMetrics services
func NewVictoriaMetricsServices(dbConfig config.DatabaseConfig, logger logger.Logger) (*VictoriaMetricsServices, error) {
	// Initialize VictoriaMetrics service
	metricsService := NewVictoriaMetricsService(dbConfig.VictoriaMetrics, logger)
	
	// Initialize VictoriaLogs service
	logsService := NewVictoriaLogsService(dbConfig.VictoriaLogs, logger)
	
	// Initialize VictoriaTraces service
	tracesService := NewVictoriaTracesService(dbConfig.VictoriaTraces, logger)

	logger.Info("VictoriaMetrics services initialized successfully",
		"metrics_endpoints", len(dbConfig.VictoriaMetrics.Endpoints),
		"logs_endpoints", len(dbConfig.VictoriaLogs.Endpoints),
		"traces_endpoints", len(dbConfig.VictoriaTraces.Endpoints),
	)

	return &VictoriaMetricsServices{
		Metrics: metricsService,
		Logs:    logsService,
		Traces:  tracesService,
	}, nil
}
