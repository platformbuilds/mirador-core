package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
    "time"
    "strings"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"sync"
)

type VictoriaMetricsService struct {
    endpoints []string
    timeout   time.Duration
    client    *http.Client
    logger    logger.Logger
    current   int // round-robin cursor

    // guards updates/selection when discovery refreshes endpoints
    mu sync.Mutex

    username string
    password string

	// retry knobs
	retries   int
	backoffMS int // base backoff (ms) for attempt 1; then doubles
}

func NewVictoriaMetricsService(cfg config.VictoriaMetricsConfig, logger logger.Logger) *VictoriaMetricsService {
    return &VictoriaMetricsService{
        endpoints: cfg.Endpoints,
        timeout:   time.Duration(cfg.Timeout) * time.Millisecond,
        client: &http.Client{
            Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
        },
        logger:    logger,
        retries:   3,   // total attempts
        backoffMS: 200, // 200ms, 400ms, 800ms
        username:  cfg.Username,
        password:  cfg.Password,
    }
}

// ReplaceEndpoints swaps the list used for round-robin (used by discovery)
func (s *VictoriaMetricsService) ReplaceEndpoints(eps []string) {
    s.mu.Lock()
    s.endpoints = append([]string(nil), eps...)
    s.current = 0
    s.mu.Unlock()
    s.logger.Info("VictoriaMetrics endpoints updated", "count", len(eps))
}

func (s *VictoriaMetricsService) ExecuteQuery(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	start := time.Now()

	endpoint := s.selectEndpoint()
	if endpoint == "" {
		return nil, errors.New("no VictoriaMetrics endpoint configured")
	}

    params := url.Values{}
    params.Set("query", request.Query)
    if request.Time != "" {
        params.Set("time", request.Time)
    }
    if request.Timeout != "" {
        params.Set("timeout", request.Timeout)
    }

    // Prefer single-node path; fallback to cluster path if unsupported
    urlSingle := fmt.Sprintf("%s/api/v1/query?%s", endpoint, params.Encode())
    urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/query?%s", endpoint, params.Encode())
    headers := map[string]string{"Accept": "application/json"}
    if request.TenantID != "" {
        headers["AccountID"] = request.TenantID
    }

    resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
    if err != nil {
        return nil, fmt.Errorf("VictoriaMetrics request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        // Try cluster path if single-node path is unsupported
        body := readBodySnippet(resp.Body)
        if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
            _ = resp.Body.Close()
            resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
            if err != nil {
                return nil, fmt.Errorf("VictoriaMetrics request failed (cluster path): %w", err)
            }
            defer resp.Body.Close()
        }
        if resp.StatusCode != http.StatusOK {
            return nil, fmt.Errorf("VictoriaMetrics returned status %d: %s", resp.StatusCode, readBodySnippet(resp.Body))
        }
    }

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

	s.logger.Info("MetricsQL query executed",
		"query", request.Query,
		"endpoint", endpoint,
		"took", executionTime,
		"seriesCount", result.SeriesCount,
		"tenant", request.TenantID,
	)
	return result, nil
}

func (s *VictoriaMetricsService) ExecuteRangeQuery(ctx context.Context, request *models.MetricsQLRangeQueryRequest) (*models.MetricsQLRangeQueryResult, error) {
	endpoint := s.selectEndpoint()
	if endpoint == "" {
		return nil, errors.New("no VictoriaMetrics endpoint configured")
	}

	params := url.Values{}
	params.Set("query", request.Query)
	params.Set("start", request.Start)
	params.Set("end", request.End)
	params.Set("step", request.Step)

    urlSingle := fmt.Sprintf("%s/api/v1/query_range?%s", endpoint, params.Encode())
    urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/query_range?%s", endpoint, params.Encode())
    headers := map[string]string{"Accept": "application/json"}
    if request.TenantID != "" {
        headers["AccountID"] = request.TenantID
    }

    resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body := readBodySnippet(resp.Body)
        if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
            _ = resp.Body.Close()
            resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
            if err != nil {
                return nil, err
            }
            defer resp.Body.Close()
        }
        if resp.StatusCode != http.StatusOK {
            return nil, fmt.Errorf("VictoriaMetrics returned status %d: %s", resp.StatusCode, readBodySnippet(resp.Body))
        }
    }

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
	if endpoint == "" {
		return nil, errors.New("no VictoriaMetrics endpoint configured")
	}

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

    urlSingle := fmt.Sprintf("%s/api/v1/series?%s", endpoint, params.Encode())
    urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/series?%s", endpoint, params.Encode())
    headers := map[string]string{"Accept": "application/json"}
    if request.TenantID != "" {
        headers["AccountID"] = request.TenantID
    }

    resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body := readBodySnippet(resp.Body)
        if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
            _ = resp.Body.Close()
            resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
            if err != nil {
                return nil, err
            }
            defer resp.Body.Close()
        }
        if resp.StatusCode != http.StatusOK {
            return nil, fmt.Errorf("VictoriaMetrics returned status %d: %s", resp.StatusCode, readBodySnippet(resp.Body))
        }
    }

	var vmResponse struct {
		Status string              `json:"status"`
		Data   []map[string]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vmResponse); err != nil {
		return nil, err
	}
	return vmResponse.Data, nil
}

func (s *VictoriaMetricsService) GetLabels(ctx context.Context, request *models.LabelsRequest) ([]string, error) {
	endpoint := s.selectEndpoint()
	if endpoint == "" {
		return nil, errors.New("no VictoriaMetrics endpoint configured")
	}

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

    urlSingle := fmt.Sprintf("%s/api/v1/labels?%s", endpoint, params.Encode())
    urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/labels?%s", endpoint, params.Encode())
    headers := map[string]string{"Accept": "application/json"}
    if request.TenantID != "" {
        headers["AccountID"] = request.TenantID
    }

    resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body := readBodySnippet(resp.Body)
        if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
            _ = resp.Body.Close()
            resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
            if err != nil {
                return nil, err
            }
            defer resp.Body.Close()
        }
        if resp.StatusCode != http.StatusOK {
            return nil, fmt.Errorf("VictoriaMetrics returned status %d: %s", resp.StatusCode, readBodySnippet(resp.Body))
        }
    }

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
	if endpoint == "" {
		return nil, errors.New("no VictoriaMetrics endpoint configured")
	}

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

    urlSingle := fmt.Sprintf("%s/api/v1/label/%s/values?%s", endpoint, url.PathEscape(request.Label), params.Encode())
    urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/label/%s/values?%s", endpoint, url.PathEscape(request.Label), params.Encode())
    headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

    resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body := readBodySnippet(resp.Body)
        if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
            _ = resp.Body.Close()
            resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
            if err != nil {
                return nil, err
            }
            defer resp.Body.Close()
        }
        if resp.StatusCode != http.StatusOK {
            return nil, fmt.Errorf("VictoriaMetrics returned status %d: %s", resp.StatusCode, readBodySnippet(resp.Body))
        }
    }

	var vmResponse struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vmResponse); err != nil {
		return nil, err
	}
	return vmResponse.Data, nil
}

func (s *VictoriaMetricsService) HealthCheck(ctx context.Context) error {
	endpoint := s.selectEndpoint()
	if endpoint == "" {
		return errors.New("no VictoriaMetrics endpoint configured")
	}

	headers := map[string]string{"Accept": "application/json"}
	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, endpoint+"/health", nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("VictoriaMetrics health check failed: status %d: %s", resp.StatusCode, readBodySnippet(resp.Body))
	}
	return nil
}

// selectEndpoint implements round-robin load balancing (safe for empty slice).
func (s *VictoriaMetricsService) selectEndpoint() string {
    s.mu.Lock()
    defer s.mu.Unlock()
    if len(s.endpoints) == 0 {
        return ""
    }
    ep := s.endpoints[s.current%len(s.endpoints)]
    s.current++
    return ep
}

/* ----------------------------- retry + helpers ----------------------------- */

// doRequestWithRetry sends an HTTP request and retries on 5xx or transport errors.
// It logs every retry attempt to stdout via s.logger so operators can see timeouts/errors.
func (s *VictoriaMetricsService) doRequestWithRetry(
    ctx context.Context,
    method, urlStr string,
    body io.Reader,
    headers map[string]string,
) (*http.Response, error) {

	var lastErr error
	backoff := time.Duration(s.backoffMS) * time.Millisecond

	for attempt := 1; attempt <= s.retries; attempt++ {
        req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
        if err != nil {
            return nil, err
        }
        for k, v := range headers {
            req.Header.Set(k, v)
        }
        if s.username != "" {
            req.SetBasicAuth(s.username, s.password)
        }

		resp, err := s.client.Do(req)
		// transport error (timeout, connection refused, etc.)
		if err != nil {
			lastErr = err
			s.logger.Warn("VictoriaMetrics request failed (transport)",
				"attempt", attempt, "method", method, "url", urlStr, "error", err)
		} else if resp.StatusCode >= 500 {
			// server error -> retry
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, readBodySnippet(resp.Body))
			_ = resp.Body.Close()
			s.logger.Warn("VictoriaMetrics 5xx response — retrying",
				"attempt", attempt, "method", method, "url", urlStr, "status", resp.StatusCode)
		} else {
			// success or non-retryable status
			return resp, nil
		}

		// no more retries?
		if attempt == s.retries || ctx.Err() != nil {
			break
		}

		// backoff (exponential)
		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Final log with summary so it's visible in stdout
	s.logger.Error("VictoriaMetrics request exhausted retries",
		"method", method, "url", urlStr, "retries", s.retries, "error", lastErr)
	return nil, lastErr
}

// readBodySnippet returns a short text excerpt from an HTTP body for error messages.
func readBodySnippet(r io.Reader) string {
	const max = 8 << 10 // 8KB
	b, _ := io.ReadAll(io.LimitReader(r, max))
	return string(b)
}

// countSeries tries to estimate the number of series in a VM/Prometheus response.
func countSeries(data any) int {
	// Expecting: { "result": [ {...}, {...} ] }
	m, ok := data.(map[string]any)
	if !ok {
		return 0
	}
	arr, _ := m["result"].([]any)
	return len(arr)
}

// countDataPoints tries to estimate datapoint count in a range query.
func countDataPoints(data any) int {
	m, ok := data.(map[string]any)
	if !ok {
		return 0
	}
	result, _ := m["result"].([]any)
	count := 0
	for _, it := range result {
		series, _ := it.(map[string]any)
		if series == nil {
			continue
		}
		if vals, ok := series["values"].([]any); ok {
			count += len(vals)
			continue
		}
		if _, ok := series["value"].([]any); ok {
			count++
		}
	}
	return count
}
