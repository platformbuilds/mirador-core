package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GetOperations returns all operations for a specific service from VictoriaTraces
func (s *VictoriaTracesService) GetOperations(ctx context.Context, serviceName, tenantID string) ([]string, error) {
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services/%s/operations", endpoint, serviceName)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if tenantID != "" {
		req.Header.Set("AccountID", tenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VictoriaTraces returned status %d", resp.StatusCode)
	}

	var operations []string
	if err := json.NewDecoder(resp.Body).Decode(&operations); err != nil {
		return nil, err
	}

	s.logger.Debug("Operations retrieved successfully",
		"service", serviceName,
		"endpoint", endpoint,
		"operationCount", len(operations),
		"tenant", tenantID,
	)

	return operations, nil
}
