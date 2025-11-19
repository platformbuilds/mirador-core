package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/discovery"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// VictoriaMetricsServices contains all VictoriaMetrics ecosystem services
type VictoriaMetricsServices struct {
	Metrics *VictoriaMetricsService
	Logs    *VictoriaLogsService
	Traces  *VictoriaTracesService
	Query   *VictoriaMetricsQueryService
}

// VictoriaTracesService handles VictoriaTraces operations
type VictoriaTracesService struct {
	name      string
	endpoints []string
	timeout   time.Duration
	client    *http.Client
	logger    logger.Logger
	current   int // For round-robin load balancing
	mu        sync.Mutex

	username string
	password string

	// Retry configuration
	retries   int
	backoffMS int

	// Optional child services for multi-source aggregation
	children []*VictoriaTracesService
}

// NewVictoriaTracesService creates a new VictoriaTraces service
func NewVictoriaTracesService(cfg config.VictoriaTracesConfig, logger logger.Logger) *VictoriaTracesService {
	return &VictoriaTracesService{
		name:      cfg.Name,
		endpoints: cfg.Endpoints,
		timeout:   time.Duration(cfg.Timeout) * time.Millisecond,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
		},
		logger:    logger,
		username:  cfg.Username,
		password:  cfg.Password,
		retries:   3,    // total attempts
		backoffMS: 1000, // 1s, 2s, 4s
	}
}

// SetChildren configures downstream services for aggregation
func (s *VictoriaTracesService) SetChildren(children []*VictoriaTracesService) {
	s.mu.Lock()
	s.children = children
	s.mu.Unlock()
	if len(children) > 0 {
		s.logger.Info("VictoriaTraces multi-source aggregation enabled", "sources", len(children)+boolToInt(len(s.endpoints) > 0))
	}
}

// GetOperations returns all operations for a specific service from VictoriaTraces
func (s *VictoriaTracesService) GetOperations(ctx context.Context, serviceName string) ([]string, error) {
	// Multi-endpoint aggregation when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.getOperationsMultiEndpoint(ctx, serviceName)
	}

	if len(s.children) > 0 {
		services := make([]*VictoriaTracesService, 0, len(s.children)+1)
		if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
			services = append(services, s)
		}
		services = append(services, s.children...)
		ch := make(chan struct {
			out []string
			err error
		}, len(services))
		for _, svc := range services {
			go func(svc *VictoriaTracesService) {
				o, e := svc.GetOperations(ctx, serviceName)
				ch <- struct {
					out []string
					err error
				}{o, e}
			}(svc)
		}
		set := map[string]struct{}{}
		okCount := 0
		for i := 0; i < len(services); i++ {
			r := <-ch
			if r.err != nil {
				continue
			}
			for _, v := range r.out {
				set[v] = struct{}{}
			}
			okCount++
		}
		if okCount == 0 {
			return nil, fmt.Errorf("all traces sources failed")
		}
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		return out, nil
	}
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services/%s/operations", endpoint, serviceName)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VictoriaTraces request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VictoriaTraces returned status %d", resp.StatusCode)
	}

	var operations struct {
		Data []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&operations); err != nil {
		return nil, fmt.Errorf("failed to parse VictoriaTraces response: %w", err)
	}

	return operations.Data, nil
}

// getOperationsMultiEndpoint aggregates operations from all configured endpoints in this service
func (s *VictoriaTracesService) getOperationsMultiEndpoint(ctx context.Context, serviceName string) ([]string, error) {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaTraces endpoints configured")
	}

	type out struct {
		operations []string
		err        error
	}
	ch := make(chan out, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			tempSvc := &VictoriaTracesService{
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
			ops, e := tempSvc.getOperationsSingleEndpoint(ctx, serviceName)
			ch <- out{ops, e}
		}(endpoint)
	}

	// Aggregate results - collect all unique operations
	opSet := make(map[string]struct{})
	for i := 0; i < len(endpoints); i++ {
		r := <-ch
		if r.err != nil {
			s.logger.Warn("operations from endpoint failed", "error", r.err)
			continue
		}
		for _, op := range r.operations {
			opSet[op] = struct{}{}
		}
	}

	operations := make([]string, 0, len(opSet))
	for op := range opSet {
		operations = append(operations, op)
	}
	return operations, nil
}

// getOperationsSingleEndpoint gets operations from a single endpoint (used by multi-endpoint aggregation)
func (s *VictoriaTracesService) getOperationsSingleEndpoint(ctx context.Context, serviceName string) ([]string, error) {
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services/%s/operations", endpoint, serviceName)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VictoriaTraces request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VictoriaTraces returned status %d", resp.StatusCode)
	}

	var operations struct {
		Data []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&operations); err != nil {
		return nil, fmt.Errorf("failed to parse VictoriaTraces response: %w", err)
	}

	return operations.Data, nil
}
func (s *VictoriaTracesService) GetServices(ctx context.Context) ([]string, error) {
	// Multi-endpoint aggregation when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.getServicesMultiEndpoint(ctx)
	}

	if len(s.children) > 0 {
		services := make([]*VictoriaTracesService, 0, len(s.children)+1)
		if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
			services = append(services, s)
		}
		services = append(services, s.children...)
		ch := make(chan struct {
			out []string
			err error
		}, len(services))
		for _, svc := range services {
			go func(svc *VictoriaTracesService) {
				o, e := svc.GetServices(ctx)
				ch <- struct {
					out []string
					err error
				}{o, e}
			}(svc)
		}
		set := map[string]struct{}{}
		okCount := 0
		for i := 0; i < len(services); i++ {
			r := <-ch
			if r.err != nil {
				continue
			}
			for _, v := range r.out {
				set[v] = struct{}{}
			}
			okCount++
		}
		if okCount == 0 {
			return nil, fmt.Errorf("all traces sources failed")
		}
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		return out, nil
	}
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services", endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
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

// getServicesMultiEndpoint aggregates services from all configured endpoints in this service
func (s *VictoriaTracesService) getServicesMultiEndpoint(ctx context.Context) ([]string, error) {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaTraces endpoints configured")
	}

	type out struct {
		services []string
		err      error
	}
	ch := make(chan out, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			tempSvc := &VictoriaTracesService{
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
			svcs, e := tempSvc.getServicesSingleEndpoint(ctx)
			ch <- out{svcs, e}
		}(endpoint)
	}

	// Aggregate results - collect all unique services
	svcSet := make(map[string]struct{})
	for i := 0; i < len(endpoints); i++ {
		r := <-ch
		if r.err != nil {
			s.logger.Warn("services from endpoint failed", "error", r.err)
			continue
		}
		for _, svc := range r.services {
			svcSet[svc] = struct{}{}
		}
	}

	services := make([]string, 0, len(svcSet))
	for svc := range svcSet {
		services = append(services, svc)
	}
	return services, nil
}

// getServicesSingleEndpoint gets services from a single endpoint (used by multi-endpoint aggregation)
func (s *VictoriaTracesService) getServicesSingleEndpoint(ctx context.Context) ([]string, error) {
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services", endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
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
func (s *VictoriaTracesService) GetTrace(ctx context.Context, traceID) (*models.Trace, error) {
	if len(s.children) > 0 {
		services := make([]*VictoriaTracesService, 0, len(s.children)+1)
		if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
			services = append(services, s)
		}
		services = append(services, s.children...)
		ch := make(chan struct {
			tr  *models.Trace
			err error
		}, len(services))
		for _, svc := range services {
			go func(svc *VictoriaTracesService) {
				t, e := svc.GetTrace(ctx, traceID)
				ch <- struct {
					tr  *models.Trace
					err error
				}{t, e}
			}(svc)
		}
		var firstErr error
		for i := 0; i < len(services); i++ {
			r := <-ch
			if r.err == nil && r.tr != nil {
				return r.tr, nil
			}
			if firstErr == nil {
				firstErr = r.err
			}
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("trace not found")
		}
		return nil, firstErr
	}
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/traces/%s", endpoint, traceID)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("Accept", "application/json, */*")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VictoriaTraces request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("trace not found")
	}

	// VictoriaTraces (Jaeger API) returns {"data":[{ trace }]}
	var wrap struct {
		Data []models.Trace `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrap); err != nil {
		return nil, fmt.Errorf("failed to parse trace response: %w", err)
	}
	if len(wrap.Data) == 0 {
		return nil, fmt.Errorf("trace not found in response")
	}
	return &wrap.Data[0], nil
}

// SearchTraces searches for traces with filters
func (s *VictoriaTracesService) SearchTraces(ctx context.Context, request *models.TraceSearchRequest) (*models.TraceSearchResult, error) {
	// Multi-endpoint aggregation when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.searchTracesMultiEndpoint(ctx, request)
	}

	if len(s.children) > 0 {
		services := make([]*VictoriaTracesService, 0, len(s.children)+1)
		if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 0 }() {
			services = append(services, s)
		}
		services = append(services, s.children...)
		ch := make(chan struct {
			res *models.TraceSearchResult
			err error
		}, len(services))
		for _, svc := range services {
			go func(svc *VictoriaTracesService) {
				r, e := svc.SearchTraces(ctx, request)
				ch <- struct {
					res *models.TraceSearchResult
					err error
				}{r, e}
			}(svc)
		}
		var merged []map[string]interface{}
		total := 0
		okCount := 0
		for i := 0; i < len(services); i++ {
			r := <-ch
			if r.err != nil || r.res == nil {
				continue
			}
			if len(r.res.Traces) > 0 {
				merged = append(merged, r.res.Traces...)
			}
			total += r.res.Total
			okCount++
		}
		if okCount == 0 {
			return nil, fmt.Errorf("all traces sources failed")
		}
		return &models.TraceSearchResult{Traces: merged, Total: total, SearchTime: 0}, nil
	}
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
	// Jaeger HTTP API expects start/end in microseconds since epoch
	if !request.Start.IsZero() {
		params.Set("start", fmt.Sprintf("%d", request.Start.AsTime().UnixMicro()))
	}
	if !request.End.IsZero() {
		params.Set("end", fmt.Sprintf("%d", request.End.AsTime().UnixMicro()))
	}
	if request.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", request.Limit))
	}

	fullURL := fmt.Sprintf("%s/select/jaeger/api/traces?%s", endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("Accept", "application/json, */*")

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

// searchTracesMultiEndpoint aggregates trace search results from all configured endpoints in this service
func (s *VictoriaTracesService) searchTracesMultiEndpoint(ctx context.Context, request *models.TraceSearchRequest) (*models.TraceSearchResult, error) {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaTraces endpoints configured")
	}

	type out struct {
		result *models.TraceSearchResult
		err    error
	}
	ch := make(chan out, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			tempSvc := &VictoriaTracesService{
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
			res, e := tempSvc.searchTracesSingleEndpoint(ctx, request)
			ch <- out{res, e}
		}(endpoint)
	}

	// Aggregate results - merge all traces
	var merged []map[string]interface{}
	total := 0
	okCount := 0
	for i := 0; i < len(endpoints); i++ {
		r := <-ch
		if r.err != nil {
			s.logger.Warn("trace search from endpoint failed", "error", r.err)
			continue
		}
		if len(r.result.Traces) > 0 {
			merged = append(merged, r.result.Traces...)
		}
		total += r.result.Total
		okCount++
	}
	if okCount == 0 {
		return nil, fmt.Errorf("all traces endpoints failed")
	}
	return &models.TraceSearchResult{Traces: merged, Total: total, SearchTime: 0}, nil
}

// searchTracesSingleEndpoint searches traces from a single endpoint (used by multi-endpoint aggregation)
func (s *VictoriaTracesService) searchTracesSingleEndpoint(ctx context.Context, request *models.TraceSearchRequest) (*models.TraceSearchResult, error) {
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
	if request.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", request.Limit))
	}
	// Jaeger HTTP API expects start/end in microseconds since epoch
	if !request.Start.IsZero() {
		params.Set("start", fmt.Sprintf("%d", request.Start.AsTime().UnixMicro()))
	}
	if !request.End.IsZero() {
		params.Set("end", fmt.Sprintf("%d", request.End.AsTime().UnixMicro()))
	}

	fullURL := fmt.Sprintf("%s/select/jaeger/api/traces?%s", endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("Accept", "application/json, */*")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VictoriaTraces request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VictoriaTraces returned status %d", resp.StatusCode)
	}

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
	// Multi-endpoint health check when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.healthCheckMultiEndpoint(ctx)
	}

	if len(s.children) > 0 {
		if err := s.healthCheckSelf(ctx); err == nil {
			return nil
		}
		for _, c := range s.children {
			if c.HealthCheck(ctx) == nil {
				return nil
			}
		}
		return fmt.Errorf("all traces sources unhealthy")
	}
	return s.healthCheckSelf(ctx)
}

// healthCheckMultiEndpoint checks health across all configured endpoints in this service
func (s *VictoriaTracesService) healthCheckMultiEndpoint(ctx context.Context) error {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return errors.New("no VictoriaTraces endpoints configured")
	}

	// Healthy if any endpoint is healthy
	for _, endpoint := range endpoints {
		tempSvc := &VictoriaTracesService{
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
	return fmt.Errorf("all traces endpoints unhealthy")
}

func (s *VictoriaTracesService) healthCheckSelf(ctx context.Context) error {
	endpoint := s.selectEndpoint()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/health", http.NoBody)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.endpoints) == 0 {
		return ""
	}
	endpoint := s.endpoints[s.current%len(s.endpoints)]
	s.current++
	return endpoint
}

// NewVictoriaMetricsServices initializes all VictoriaMetrics services
func NewVictoriaMetricsServices(dbConfig config.DatabaseConfig, logger logger.Logger) (*VictoriaMetricsServices, error) {
	// Initialize VictoriaMetrics service
	metricsService := NewVictoriaMetricsService(dbConfig.VictoriaMetrics, logger)
	// If multiple sources are configured, create child services and enable aggregation
	if len(dbConfig.MetricsSources) > 0 {
		children := make([]*VictoriaMetricsService, 0, len(dbConfig.MetricsSources))
		for _, mc := range dbConfig.MetricsSources {
			child := NewVictoriaMetricsService(mc, logger)
			children = append(children, child)
		}
		metricsService.SetChildren(children)
	}

	// Initialize VictoriaLogs service
	logsService := NewVictoriaLogsService(dbConfig.VictoriaLogs, logger)
	if len(dbConfig.LogsSources) > 0 {
		children := make([]*VictoriaLogsService, 0, len(dbConfig.LogsSources))
		for _, lc := range dbConfig.LogsSources {
			child := NewVictoriaLogsService(lc, logger)
			children = append(children, child)
		}
		logsService.SetChildren(children)
	}

	// Initialize VictoriaTraces service
	tracesService := NewVictoriaTracesService(dbConfig.VictoriaTraces, logger)
	if len(dbConfig.TracesSources) > 0 {
		children := make([]*VictoriaTracesService, 0, len(dbConfig.TracesSources))
		for _, tc := range dbConfig.TracesSources {
			child := NewVictoriaTracesService(tc, logger)
			children = append(children, child)
		}
		tracesService.SetChildren(children)
	}

	// Initialize VictoriaMetrics Query service
	queryService := NewVictoriaMetricsQueryService(metricsService, logger)

	logger.Info("VictoriaMetrics services initialized successfully",
		"metrics_endpoints", len(dbConfig.VictoriaMetrics.Endpoints),
		"logs_endpoints", len(dbConfig.VictoriaLogs.Endpoints),
		"traces_endpoints", len(dbConfig.VictoriaTraces.Endpoints),
	)

	return &VictoriaMetricsServices{
		Metrics: metricsService,
		Logs:    logsService,
		Traces:  tracesService,
		Query:   queryService,
	}, nil
}

// StartDiscovery enables periodic DNS discovery for Victoria* services when configured.
// It relies on headless Services (A/AAAA records per pod) or SRV records.
func (s *VictoriaMetricsServices) StartDiscovery(ctx context.Context, dbConfig config.DatabaseConfig, log logger.Logger) {
	// VictoriaMetrics
	if dbConfig.VictoriaMetrics.Discovery.Enabled && s.Metrics != nil {
		cfg := dbConfig.VictoriaMetrics.Discovery
		if len(dbConfig.VictoriaMetrics.Endpoints) > 0 {
			s.Metrics.ReplaceEndpoints(dbConfig.VictoriaMetrics.Endpoints)
		}
		discovery.StartDNSDiscovery(ctx, discovery.DNSConfig{
			Enabled:        true,
			Service:        cfg.Service,
			Port:           cfg.Port,
			Scheme:         cfg.Scheme,
			RefreshSeconds: cfg.RefreshSeconds,
			UseSRV:         cfg.UseSRV,
		}, s.Metrics, log)
	}
	// Discovery for additional metrics sources
	if len(dbConfig.MetricsSources) > 0 && s.Metrics != nil && s.Metrics.children != nil {
		for i, mc := range dbConfig.MetricsSources {
			if mc.Discovery.Enabled && i < len(s.Metrics.children) {
				child := s.Metrics.children[i]
				if len(mc.Endpoints) > 0 {
					child.ReplaceEndpoints(mc.Endpoints)
				}
				cfg := mc.Discovery
				discovery.StartDNSDiscovery(ctx, discovery.DNSConfig{
					Enabled:        true,
					Service:        cfg.Service,
					Port:           cfg.Port,
					Scheme:         cfg.Scheme,
					RefreshSeconds: cfg.RefreshSeconds,
					UseSRV:         cfg.UseSRV,
				}, child, log)
			}
		}
	}

	// VictoriaLogs
	if dbConfig.VictoriaLogs.Discovery.Enabled && s.Logs != nil {
		cfg := dbConfig.VictoriaLogs.Discovery
		if len(dbConfig.VictoriaLogs.Endpoints) > 0 {
			s.Logs.ReplaceEndpoints(dbConfig.VictoriaLogs.Endpoints)
		}
		discovery.StartDNSDiscovery(ctx, discovery.DNSConfig{
			Enabled:        true,
			Service:        cfg.Service,
			Port:           cfg.Port,
			Scheme:         cfg.Scheme,
			RefreshSeconds: cfg.RefreshSeconds,
			UseSRV:         cfg.UseSRV,
		}, s.Logs, log)
	}
	// Discovery for additional VictoriaLogs sources
	if len(dbConfig.LogsSources) > 0 && s.Logs != nil && s.Logs.children != nil {
		for i, lc := range dbConfig.LogsSources {
			if lc.Discovery.Enabled && i < len(s.Logs.children) {
				child := s.Logs.children[i]
				if len(lc.Endpoints) > 0 {
					child.ReplaceEndpoints(lc.Endpoints)
				}
				cfg := lc.Discovery
				discovery.StartDNSDiscovery(ctx, discovery.DNSConfig{
					Enabled:        true,
					Service:        cfg.Service,
					Port:           cfg.Port,
					Scheme:         cfg.Scheme,
					RefreshSeconds: cfg.RefreshSeconds,
					UseSRV:         cfg.UseSRV,
				}, child, log)
			}
		}
	}

	// VictoriaTraces
	if dbConfig.VictoriaTraces.Discovery.Enabled && s.Traces != nil {
		cfg := dbConfig.VictoriaTraces.Discovery
		if len(dbConfig.VictoriaTraces.Endpoints) > 0 {
			s.Traces.ReplaceEndpoints(dbConfig.VictoriaTraces.Endpoints)
		}
		discovery.StartDNSDiscovery(ctx, discovery.DNSConfig{
			Enabled:        true,
			Service:        cfg.Service,
			Port:           cfg.Port,
			Scheme:         cfg.Scheme,
			RefreshSeconds: cfg.RefreshSeconds,
			UseSRV:         cfg.UseSRV,
		}, s.Traces, log)
	}
	// Discovery for additional VictoriaTraces sources
	if len(dbConfig.TracesSources) > 0 && s.Traces != nil && s.Traces.children != nil {
		for i, tc := range dbConfig.TracesSources {
			if tc.Discovery.Enabled && i < len(s.Traces.children) {
				child := s.Traces.children[i]
				if len(tc.Endpoints) > 0 {
					child.ReplaceEndpoints(tc.Endpoints)
				}
				cfg := tc.Discovery
				discovery.StartDNSDiscovery(ctx, discovery.DNSConfig{
					Enabled:        true,
					Service:        cfg.Service,
					Port:           cfg.Port,
					Scheme:         cfg.Scheme,
					RefreshSeconds: cfg.RefreshSeconds,
					UseSRV:         cfg.UseSRV,
				}, child, log)
			}
		}
	}
}

// ReplaceEndpoints allows dynamic update from discovery
func (s *VictoriaTracesService) ReplaceEndpoints(eps []string) {
	s.mu.Lock()
	s.endpoints = append([]string(nil), eps...)
	s.current = 0
	s.mu.Unlock()
	s.logger.Info("VictoriaTraces endpoints updated", "count", len(eps))
}
