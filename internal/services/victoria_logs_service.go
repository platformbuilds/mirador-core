// ================================
// internal/services/victoria_logs.service.go - VictoriaLogs Integration
// ================================

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mirador/core/internal/config"
	"github.com/mirador/core/internal/models"
	"github.com/mirador/core/pkg/logger"
)

type VictoriaLogsService struct {
	endpoints []string
	timeout   time.Duration
	client    *http.Client
	logger    logger.Logger
	current   int
}

func NewVictoriaLogsService(cfg config.VictoriaLogsConfig, logger logger.Logger) *VictoriaLogsService {
	return &VictoriaLogsService{
		endpoints: cfg.Endpoints,
		timeout:   time.Duration(cfg.Timeout) * time.Millisecond,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
		},
		logger: logger,
	}
}

// StoreJSONEvent stores JSON events from AI engines (predictions, correlations)
func (s *VictoriaLogsService) StoreJSONEvent(ctx context.Context, event map[string]interface{}, tenantID string) error {
	// Convert to VictoriaLogs JSON format
	logEntry := map[string]interface{}{
		"_time": event["_time"],
		"_msg":  event["_msg"],
		"data":  event,
	}

	jsonData, err := json.Marshal([]map[string]interface{}{logEntry})
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Select endpoint (load balancing)
	endpoint := s.selectEndpoint()
	url := fmt.Sprintf("%s/insert/jsonline", endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if tenantID != "" {
		req.Header.Set("AccountID", tenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("VictoriaLogs returned status %d", resp.StatusCode)
	}

	s.logger.Info("Event stored in VictoriaLogs", "type", event["type"], "tenant", tenantID)
	return nil
}

// QueryPredictionEvents retrieves stored prediction events using LogsQL
func (s *VictoriaLogsService) QueryPredictionEvents(ctx context.Context, query, tenantID string) ([]*models.SystemFracture, error) {
	endpoint := s.selectEndpoint()
	url := fmt.Sprintf("%s/select/logsql/query", endpoint)

	reqBody := map[string]interface{}{
		"query": query,
		"limit": 1000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if tenantID != "" {
		req.Header.Set("AccountID", tenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var queryResponse models.LogsQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResponse); err != nil {
		return nil, err
	}

	// Convert log entries back to fracture predictions
	var fractures []*models.SystemFracture
	for _, entry := range queryResponse.Data {
		if prediction, ok := entry["prediction"].(map[string]interface{}); ok {
			fracture := &models.SystemFracture{
				ID:        prediction["id"].(string),
				Component: prediction["component"].(string),
				// ... convert other fields
			}
			fractures = append(fractures, fracture)
		}
	}

	return fractures, nil
}

func (s *VictoriaLogsService) selectEndpoint() string {
	// Simple round-robin load balancing
	return s.endpoints[time.Now().Unix()%int64(len(s.endpoints))]
}

// HealthCheck checks VictoriaLogs health
func (s *VictoriaLogsService) HealthCheck(ctx context.Context) error {
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
		return fmt.Errorf("VictoriaLogs health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// selectEndpoint implements round-robin load balancing
func (s *VictoriaLogsService) selectEndpoint() string {
	if len(s.endpoints) == 0 {
		return "http://localhost:9428" // Default fallback
	}

	endpoint := s.endpoints[s.current%len(s.endpoints)]
	s.current++
	return endpoint
}
