package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sync"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type VictoriaMetricsService struct {
	name      string
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

	// Optional child services for multi-source aggregation. When non-empty,
	// this service will fan-out queries to each child (and optionally itself)
	// and aggregate results.
	children []*VictoriaMetricsService
}

func NewVictoriaMetricsService(cfg config.VictoriaMetricsConfig, logger logger.Logger) *VictoriaMetricsService {
	return &VictoriaMetricsService{
		name:      cfg.Name,
		endpoints: cfg.Endpoints,
		timeout:   time.Duration(cfg.Timeout) * time.Millisecond,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
		},
		logger:    logger,
		retries:   3,    // total attempts
		backoffMS: 1000, // 1s, 2s, 4s
		username:  cfg.Username,
		password:  cfg.Password,
	}
}

// SetChildren configures downstream VictoriaMetricsService instances used for
// multi-source aggregation. Passing nil or empty slice disables aggregation.
func (s *VictoriaMetricsService) SetChildren(children []*VictoriaMetricsService) {
	s.mu.Lock()
	s.children = children
	s.mu.Unlock()
	if len(children) > 0 {
		s.logger.Info("VictoriaMetrics multi-source aggregation enabled", "sources", len(children)+boolToInt(len(s.endpoints) > 0))
	}
}

// ReplaceEndpoints swaps the list used for round-robin (used by discovery)
func (s *VictoriaMetricsService) ReplaceEndpoints(eps []string) {
	s.mu.Lock()
	s.endpoints = append([]string(nil), eps...)
	s.current = 0
	s.mu.Unlock()
	s.logger.Info("VictoriaMetrics endpoints updated", "source", s.name, "count", len(eps))
}

func (s *VictoriaMetricsService) ExecuteQuery(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	// Aggregation path when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.executeQueryMultiEndpoint(ctx, request)
	}

	// Aggregation path when multiple sources configured
	if len(s.children) > 0 {
		return s.executeQueryAggregated(ctx, request)
	}

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

	// Prefer cluster path (back-compat with tests); fallback to single-node path if unsupported
	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/query?%s", endpoint, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/query?%s", endpoint, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("VictoriaMetrics request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try single-node path if cluster path is unsupported
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
			if err != nil {
				return nil, fmt.Errorf("VictoriaMetrics request failed (single path): %w", err)
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
		"source", s.name,
		"endpoint", endpoint,
		"took", executionTime,
		"seriesCount", result.SeriesCount,
		"tenant", request.TenantID,
	)
	return result, nil
}

// executeQueryAggregated fans out the query to all configured sources,
// aggregates the results (concatenates series), and returns a combined result.
func (s *VictoriaMetricsService) executeQueryAggregated(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	type out struct {
		res *models.MetricsQLQueryResult
		err error
	}
	// Build list: include self if it has endpoints configured
	services := make([]*VictoriaMetricsService, 0, len(s.children)+1)
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
		services = append(services, &VictoriaMetricsService{ // shallow runner using self endpoints/auth
			endpoints: s.endpoints,
			timeout:   s.timeout,
			client:    s.client,
			logger:    s.logger,
			username:  s.username,
			password:  s.password,
			retries:   s.retries,
			backoffMS: s.backoffMS,
		})
	}
	services = append(services, s.children...)

	ch := make(chan out, len(services))
	for _, svc := range services {
		go func(svc *VictoriaMetricsService) {
			r, e := svc.ExecuteQuery(ctx, request)
			ch <- out{r, e}
		}(svc)
	}
	var firstStatus string
	merged := map[string]any{}
	var seriesCount int
	successes := 0
	for i := 0; i < len(services); i++ {
		o := <-ch
		if o.err != nil || o.res == nil || o.res.Data == nil {
			if o.err != nil {
				s.logger.Warn("metrics source failed", "error", o.err)
			}
			continue
		}
		if firstStatus == "" {
			firstStatus = o.res.Status
		}
		// Merge "data" by concatenating result arrays if present
		merged = mergeVMData(merged, o.res.Data)
		seriesCount += o.res.SeriesCount
		successes++
	}
	if successes == 0 {
		return nil, fmt.Errorf("all metrics sources failed")
	}
	return &models.MetricsQLQueryResult{Status: ifEmpty(firstStatus, "success"), Data: merged, SeriesCount: seriesCount}, nil
}

// executeQueryMultiEndpoint fans out the query to all configured endpoints in this service,
// aggregates the results (concatenates series), and returns a combined result.
func (s *VictoriaMetricsService) executeQueryMultiEndpoint(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	type out struct {
		res *models.MetricsQLQueryResult
		err error
	}

	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaMetrics endpoints configured")
	}

	ch := make(chan out, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			// Create a temporary service instance for this endpoint
			tempSvc := &VictoriaMetricsService{
				name:      s.name,
				endpoints: []string{ep}, // Single endpoint
				timeout:   s.timeout,
				client:    s.client,
				logger:    s.logger,
				username:  s.username,
				password:  s.password,
				retries:   s.retries,
				backoffMS: s.backoffMS,
			}
			r, e := tempSvc.executeQuerySingleEndpoint(ctx, request)
			ch <- out{r, e}
		}(endpoint)
	}

	var firstStatus string
	merged := map[string]any{}
	var seriesCount int
	successes := 0
	for i := 0; i < len(endpoints); i++ {
		o := <-ch
		if o.err != nil || o.res == nil || o.res.Data == nil {
			if o.err != nil {
				s.logger.Warn("metrics endpoint failed", "error", o.err)
			}
			continue
		}
		if firstStatus == "" {
			firstStatus = o.res.Status
		}
		// Merge "data" by concatenating result arrays if present
		merged = mergeVMData(merged, o.res.Data)
		seriesCount += o.res.SeriesCount
		successes++
	}
	if successes == 0 {
		return nil, fmt.Errorf("all metrics endpoints failed")
	}
	return &models.MetricsQLQueryResult{Status: ifEmpty(firstStatus, "success"), Data: merged, SeriesCount: seriesCount}, nil
}

// executeQuerySingleEndpoint executes a query against a single endpoint (used by multi-endpoint fan-out)
func (s *VictoriaMetricsService) executeQuerySingleEndpoint(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
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

	// Prefer cluster path (back-compat with tests); fallback to single-node path if unsupported
	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/query?%s", endpoint, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/query?%s", endpoint, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("VictoriaMetrics request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try single-node path if cluster path is unsupported
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
			if err != nil {
				return nil, fmt.Errorf("VictoriaMetrics request failed (single path): %w", err)
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
		"source", s.name,
		"endpoint", endpoint,
		"took", executionTime,
		"seriesCount", result.SeriesCount,
		"tenant", request.TenantID,
	)
	return result, nil
}

func (s *VictoriaMetricsService) ExecuteRangeQuery(ctx context.Context, request *models.MetricsQLRangeQueryRequest) (*models.MetricsQLRangeQueryResult, error) {
	// Aggregation path when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.executeRangeMultiEndpoint(ctx, request)
	}

	if len(s.children) > 0 {
		return s.executeRangeAggregated(ctx, request)
	}
	endpoint := s.selectEndpoint()
	if endpoint == "" {
		return nil, errors.New("no VictoriaMetrics endpoint configured")
	}

	params := url.Values{}
	params.Set("query", request.Query)
	params.Set("start", request.Start)
	params.Set("end", request.End)
	params.Set("step", request.Step)

	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/query_range?%s", endpoint, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/query_range?%s", endpoint, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
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

func (s *VictoriaMetricsService) executeRangeAggregated(ctx context.Context, request *models.MetricsQLRangeQueryRequest) (*models.MetricsQLRangeQueryResult, error) {
	type out struct {
		res *models.MetricsQLRangeQueryResult
		err error
	}
	services := make([]*VictoriaMetricsService, 0, len(s.children)+1)
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
		services = append(services, &VictoriaMetricsService{
			endpoints: s.endpoints,
			timeout:   s.timeout,
			client:    s.client,
			logger:    s.logger,
			username:  s.username,
			password:  s.password,
			retries:   s.retries,
			backoffMS: s.backoffMS,
		})
	}
	services = append(services, s.children...)
	ch := make(chan out, len(services))
	for _, svc := range services {
		go func(svc *VictoriaMetricsService) {
			r, e := svc.ExecuteRangeQuery(ctx, request)
			ch <- out{r, e}
		}(svc)
	}
	var firstStatus string
	merged := map[string]any{}
	points := 0
	successes := 0
	for i := 0; i < len(services); i++ {
		o := <-ch
		if o.err != nil || o.res == nil || o.res.Data == nil {
			if o.err != nil {
				s.logger.Warn("metrics range source failed", "error", o.err)
			}
			continue
		}
		if firstStatus == "" {
			firstStatus = o.res.Status
		}
		merged = mergeVMData(merged, o.res.Data)
		points += o.res.DataPointCount
		successes++
	}
	if successes == 0 {
		return nil, fmt.Errorf("all metrics sources failed")
	}
	return &models.MetricsQLRangeQueryResult{Status: ifEmpty(firstStatus, "success"), Data: merged, DataPointCount: points}, nil
}

// executeRangeMultiEndpoint fans out the range query to all configured endpoints in this service,
// aggregates the results (concatenates series), and returns a combined result.
func (s *VictoriaMetricsService) executeRangeMultiEndpoint(ctx context.Context, request *models.MetricsQLRangeQueryRequest) (*models.MetricsQLRangeQueryResult, error) {
	type out struct {
		res *models.MetricsQLRangeQueryResult
		err error
	}

	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaMetrics endpoints configured")
	}

	ch := make(chan out, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			// Create a temporary service instance for this endpoint
			tempSvc := &VictoriaMetricsService{
				name:      s.name,
				endpoints: []string{ep}, // Single endpoint
				timeout:   s.timeout,
				client:    s.client,
				logger:    s.logger,
				username:  s.username,
				password:  s.password,
				retries:   s.retries,
				backoffMS: s.backoffMS,
			}
			r, e := tempSvc.executeRangeSingleEndpoint(ctx, request)
			ch <- out{r, e}
		}(endpoint)
	}

	var firstStatus string
	merged := map[string]any{}
	points := 0
	successes := 0
	for i := 0; i < len(endpoints); i++ {
		o := <-ch
		if o.err != nil || o.res == nil || o.res.Data == nil {
			if o.err != nil {
				s.logger.Warn("metrics range endpoint failed", "error", o.err)
			}
			continue
		}
		if firstStatus == "" {
			firstStatus = o.res.Status
		}
		merged = mergeVMData(merged, o.res.Data)
		points += o.res.DataPointCount
		successes++
	}
	if successes == 0 {
		return nil, fmt.Errorf("all metrics range endpoints failed")
	}
	return &models.MetricsQLRangeQueryResult{Status: ifEmpty(firstStatus, "success"), Data: merged, DataPointCount: points}, nil
}

// executeRangeSingleEndpoint executes a range query against a single endpoint (used by multi-endpoint fan-out)
func (s *VictoriaMetricsService) executeRangeSingleEndpoint(ctx context.Context, request *models.MetricsQLRangeQueryRequest) (*models.MetricsQLRangeQueryResult, error) {
	endpoint := s.selectEndpoint()
	if endpoint == "" {
		return nil, errors.New("no VictoriaMetrics endpoint configured")
	}

	params := url.Values{}
	params.Set("query", request.Query)
	params.Set("start", request.Start)
	params.Set("end", request.End)
	params.Set("step", request.Step)

	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/query_range?%s", endpoint, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/query_range?%s", endpoint, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try single-node path if cluster path is unsupported
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
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
		return nil, fmt.Errorf("failed to parse VictoriaMetrics response: %w", err)
	}

	result := &models.MetricsQLRangeQueryResult{
		Status:         vmResponse.Status,
		Data:           vmResponse.Data,
		DataPointCount: countDataPoints(vmResponse.Data),
	}
	return result, nil
}

func (s *VictoriaMetricsService) GetSeries(ctx context.Context, request *models.SeriesRequest) ([]map[string]string, error) {
	// Multi-endpoint aggregation when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.getSeriesMultiEndpoint(ctx, request)
	}

	if len(s.children) > 0 {
		// aggregate: append series from all sources
		services := make([]*VictoriaMetricsService, 0, len(s.children)+1)
		if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
			services = append(services, s)
		}
		services = append(services, s.children...)
		ch := make(chan struct {
			data []map[string]string
			err  error
		}, len(services))
		for _, svc := range services {
			go func(svc *VictoriaMetricsService) {
				d, e := svc.GetSeries(ctx, request)
				ch <- struct {
					data []map[string]string
					err  error
				}{d, e}
			}(svc)
		}
		var out []map[string]string
		successes := 0
		for i := 0; i < len(services); i++ {
			r := <-ch
			if r.err != nil {
				s.logger.Warn("series from source failed", "error", r.err)
				continue
			}
			out = append(out, r.data...)
			successes++
		}
		if successes == 0 {
			return nil, fmt.Errorf("all metrics sources failed")
		}
		return out, nil
	}
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

	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/series?%s", endpoint, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/series?%s", endpoint, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
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

// getSeriesMultiEndpoint aggregates series from all configured endpoints in this service
func (s *VictoriaMetricsService) getSeriesMultiEndpoint(ctx context.Context, request *models.SeriesRequest) ([]map[string]string, error) {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaMetrics endpoints configured")
	}

	ch := make(chan struct {
		data []map[string]string
		err  error
	}, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			// Create a temporary service instance for this endpoint
			tempSvc := &VictoriaMetricsService{
				name:      s.name,
				endpoints: []string{ep}, // Single endpoint
				timeout:   s.timeout,
				client:    s.client,
				logger:    s.logger,
				username:  s.username,
				password:  s.password,
				retries:   s.retries,
				backoffMS: s.backoffMS,
			}
			d, e := tempSvc.getSeriesSingleEndpoint(ctx, request)
			ch <- struct {
				data []map[string]string
				err  error
			}{d, e}
		}(endpoint)
	}

	var out []map[string]string
	successes := 0
	for i := 0; i < len(endpoints); i++ {
		r := <-ch
		if r.err != nil {
			s.logger.Warn("series from endpoint failed", "error", r.err)
			continue
		}
		out = append(out, r.data...)
		successes++
	}
	if successes == 0 {
		return nil, fmt.Errorf("all metrics endpoints failed")
	}
	return out, nil
}

// getSeriesSingleEndpoint gets series from a single endpoint (used by multi-endpoint aggregation)
func (s *VictoriaMetricsService) getSeriesSingleEndpoint(ctx context.Context, request *models.SeriesRequest) ([]map[string]string, error) {
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

	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/series?%s", endpoint, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/series?%s", endpoint, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
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
	// Multi-endpoint aggregation when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.getLabelsMultiEndpoint(ctx, request)
	}

	if len(s.children) > 0 {
		services := make([]*VictoriaMetricsService, 0, len(s.children)+1)
		if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
			services = append(services, s)
		}
		services = append(services, s.children...)
		ch := make(chan struct {
			data []string
			err  error
		}, len(services))
		for _, svc := range services {
			go func(svc *VictoriaMetricsService) {
				d, e := svc.GetLabels(ctx, request)
				ch <- struct {
					data []string
					err  error
				}{d, e}
			}(svc)
		}
		set := map[string]struct{}{}
		successes := 0
		for i := 0; i < len(services); i++ {
			r := <-ch
			if r.err != nil {
				s.logger.Warn("labels from source failed", "error", r.err)
				continue
			}
			for _, v := range r.data {
				set[v] = struct{}{}
			}
			successes++
		}
		if successes == 0 {
			return nil, fmt.Errorf("all metrics sources failed")
		}
		// flatten
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		return out, nil
	}
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

	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/labels?%s", endpoint, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/labels?%s", endpoint, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
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

// getLabelsMultiEndpoint aggregates labels from all configured endpoints in this service
func (s *VictoriaMetricsService) getLabelsMultiEndpoint(ctx context.Context, request *models.LabelsRequest) ([]string, error) {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaMetrics endpoints configured")
	}

	ch := make(chan struct {
		data []string
		err  error
	}, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			// Create a temporary service instance for this endpoint
			tempSvc := &VictoriaMetricsService{
				name:      s.name,
				endpoints: []string{ep}, // Single endpoint
				timeout:   s.timeout,
				client:    s.client,
				logger:    s.logger,
				username:  s.username,
				password:  s.password,
				retries:   s.retries,
				backoffMS: s.backoffMS,
			}
			d, e := tempSvc.getLabelsSingleEndpoint(ctx, request)
			ch <- struct {
				data []string
				err  error
			}{d, e}
		}(endpoint)
	}

	set := map[string]struct{}{}
	successes := 0
	for i := 0; i < len(endpoints); i++ {
		r := <-ch
		if r.err != nil {
			s.logger.Warn("labels from endpoint failed", "error", r.err)
			continue
		}
		for _, v := range r.data {
			set[v] = struct{}{}
		}
		successes++
	}
	if successes == 0 {
		return nil, fmt.Errorf("all metrics endpoints failed")
	}
	// flatten
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out, nil
}

// getLabelsSingleEndpoint gets labels from a single endpoint (used by multi-endpoint aggregation)
func (s *VictoriaMetricsService) getLabelsSingleEndpoint(ctx context.Context, request *models.LabelsRequest) ([]string, error) {
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

	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/labels?%s", endpoint, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/labels?%s", endpoint, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
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
	// Multi-endpoint aggregation when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.getLabelValuesMultiEndpoint(ctx, request)
	}

	if len(s.children) > 0 {
		services := make([]*VictoriaMetricsService, 0, len(s.children)+1)
		if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
			services = append(services, s)
		}
		services = append(services, s.children...)
		ch := make(chan struct {
			data []string
			err  error
		}, len(services))
		for _, svc := range services {
			go func(svc *VictoriaMetricsService) {
				d, e := svc.GetLabelValues(ctx, request)
				ch <- struct {
					data []string
					err  error
				}{d, e}
			}(svc)
		}
		set := map[string]struct{}{}
		successes := 0
		for i := 0; i < len(services); i++ {
			r := <-ch
			if r.err != nil {
				s.logger.Warn("label values from source failed", "error", r.err)
				continue
			}
			for _, v := range r.data {
				set[v] = struct{}{}
			}
			successes++
		}
		if successes == 0 {
			return nil, fmt.Errorf("all metrics sources failed")
		}
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		return out, nil
	}
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

	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/label/%s/values?%s", endpoint, url.PathEscape(request.Label), params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/label/%s/values?%s", endpoint, url.PathEscape(request.Label), params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
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

// getLabelValuesMultiEndpoint aggregates label values from all configured endpoints in this service
func (s *VictoriaMetricsService) getLabelValuesMultiEndpoint(ctx context.Context, request *models.LabelValuesRequest) ([]string, error) {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaMetrics endpoints configured")
	}

	ch := make(chan struct {
		data []string
		err  error
	}, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			// Create a temporary service instance for this endpoint
			tempSvc := &VictoriaMetricsService{
				name:      s.name,
				endpoints: []string{ep}, // Single endpoint
				timeout:   s.timeout,
				client:    s.client,
				logger:    s.logger,
				username:  s.username,
				password:  s.password,
				retries:   s.retries,
				backoffMS: s.backoffMS,
			}
			d, e := tempSvc.getLabelValuesSingleEndpoint(ctx, request)
			ch <- struct {
				data []string
				err  error
			}{d, e}
		}(endpoint)
	}

	set := map[string]struct{}{}
	successes := 0
	for i := 0; i < len(endpoints); i++ {
		r := <-ch
		if r.err != nil {
			s.logger.Warn("label values from endpoint failed", "error", r.err)
			continue
		}
		for _, v := range r.data {
			set[v] = struct{}{}
		}
		successes++
	}
	if successes == 0 {
		return nil, fmt.Errorf("all metrics endpoints failed")
	}
	// flatten
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out, nil
}

// getLabelValuesSingleEndpoint gets label values from a single endpoint (used by multi-endpoint aggregation)
func (s *VictoriaMetricsService) getLabelValuesSingleEndpoint(ctx context.Context, request *models.LabelValuesRequest) ([]string, error) {
	endpoint := s.selectEndpoint()
	if endpoint == "" {
		return nil, errors.New("no VictoriaMetrics endpoint configured")
	}

	params := url.Values{}
	params.Set("label", request.Label)
	if request.Start != "" {
		params.Set("start", request.Start)
	}
	if request.End != "" {
		params.Set("end", request.End)
	}
	for _, match := range request.Match {
		params.Add("match[]", match)
	}

	urlCluster := fmt.Sprintf("%s/select/0/prometheus/api/v1/label/%s/values?%s", endpoint, request.Label, params.Encode())
	urlSingle := fmt.Sprintf("%s/api/v1/label/%s/values?%s", endpoint, request.Label, params.Encode())
	headers := map[string]string{"Accept": "application/json"}
	if request.TenantID != "" {
		headers["AccountID"] = request.TenantID
	}

	resp, err := s.doRequestWithRetry(ctx, http.MethodGet, urlCluster, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBodySnippet(resp.Body)
		if resp.StatusCode == http.StatusNotFound || (resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(body), "unsupported path")) {
			_ = resp.Body.Close()
			resp, err = s.doRequestWithRetry(ctx, http.MethodGet, urlSingle, nil, headers)
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
	// Multi-endpoint health check when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.healthCheckMultiEndpoint(ctx)
	}

	if len(s.children) > 0 {
		// Healthy if any source is healthy
		if err := s.healthCheckSelf(ctx); err == nil {
			return nil
		}
		for _, c := range s.children {
			if c.HealthCheck(ctx) == nil {
				return nil
			}
		}
		return fmt.Errorf("all metrics sources unhealthy")
	}
	return s.healthCheckSelf(ctx)
}

// healthCheckMultiEndpoint checks health across all configured endpoints in this service
func (s *VictoriaMetricsService) healthCheckMultiEndpoint(ctx context.Context) error {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return errors.New("no VictoriaMetrics endpoints configured")
	}

	// Healthy if any endpoint is healthy
	for _, endpoint := range endpoints {
		tempSvc := &VictoriaMetricsService{
			name:      s.name,
			endpoints: []string{endpoint}, // Single endpoint
			timeout:   s.timeout,
			client:    s.client,
			logger:    s.logger,
			username:  s.username,
			password:  s.password,
			retries:   s.retries,
			backoffMS: s.backoffMS,
		}
		if err := tempSvc.healthCheckSelf(ctx); err == nil {
			return nil // At least one endpoint is healthy
		}
	}
	return fmt.Errorf("all metrics endpoints unhealthy")
}

func (s *VictoriaMetricsService) healthCheckSelf(ctx context.Context) error {
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

// mergeVMData merges VictoriaMetrics JSON data blocks by concatenating the
// "result" arrays when present. For non-standard shapes, it prefers the
// left-hand side and falls back to the right if left is empty.
func mergeVMData(dst map[string]any, src any) map[string]any {
	// normalize dst
	if dst == nil {
		dst = map[string]any{}
	}
	sm, ok := src.(map[string]any)
	if !ok {
		// if dst empty, wrap src
		if len(dst) == 0 {
			return map[string]any{"result": src}
		}
		return dst
	}
	// copy resultType if missing
	if _, ok := dst["resultType"]; !ok {
		if rt, ok := sm["resultType"]; ok {
			dst["resultType"] = rt
		}
	}
	// concat result arrays when possible
	if arr, ok := sm["result"].([]any); ok {
		if cur, ok := dst["result"].([]any); ok {
			dst["result"] = append(cur, arr...)
		} else {
			// clone
			cp := make([]any, len(arr))
			copy(cp, arr)
			dst["result"] = cp
		}
		return dst
	}
	// fallback: shallow merge keys (don't overwrite existing)
	for k, v := range sm {
		if _, ok := dst[k]; !ok {
			dst[k] = v
		}
	}
	return dst
}

func ifEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
