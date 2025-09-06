package services

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "net/url"
    "time"
    "github.com/platformbuilds/mirador-core/internal/utils"

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
}

// VictoriaTracesService handles VictoriaTraces operations
type VictoriaTracesService struct {
    endpoints []string
    timeout   time.Duration
    client    *http.Client
    logger    logger.Logger
    current   int // For round-robin load balancing
    mu        sync.Mutex

    username string
    password string
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
        username: cfg.Username,
        password: cfg.Password,
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

    if utils.IsUint32String(tenantID) { req.Header.Set("AccountID", tenantID) }
    if s.username != "" { req.SetBasicAuth(s.username, s.password) }

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

    if utils.IsUint32String(tenantID) { req.Header.Set("AccountID", tenantID) }
    if s.username != "" { req.SetBasicAuth(s.username, s.password) }
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
    var wrap struct{ Data []models.Trace `json:"data"` }
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
    req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
    if err != nil {
        return nil, err
    }

    if utils.IsUint32String(request.TenantID) { req.Header.Set("AccountID", request.TenantID) }
    if s.username != "" { req.SetBasicAuth(s.username, s.password) }
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
// (moved) tenant ID numeric check lives in utils.IsUint32String

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
}

// ReplaceEndpoints allows dynamic update from discovery
func (s *VictoriaTracesService) ReplaceEndpoints(eps []string) {
    s.mu.Lock()
    s.endpoints = append([]string(nil), eps...)
    s.current = 0
    s.mu.Unlock()
    s.logger.Info("VictoriaTraces endpoints updated", "count", len(eps))
}
