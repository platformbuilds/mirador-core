package weaviate

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTP       *http.Client
	logger     logger.Logger
	healthMu   sync.RWMutex
	lastHealth time.Time
	healthTTL  time.Duration
}

func New(cfg config.WeaviateConfig) *Client {
	scheme := cfg.Scheme
	if scheme == "" {
		scheme = "http"
	}
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	base := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	// Configure HTTP client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     20,
		IdleConnTimeout:     90 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // Increased timeout for complex queries
	}

	return &Client{
		BaseURL:   base,
		APIKey:    cfg.APIKey,
		HTTP:      httpClient,
		healthTTL: 30 * time.Second, // Cache health checks for 30 seconds
	}
}

// SetLogger sets the logger for health check logging
func (c *Client) SetLogger(l logger.Logger) {
	c.logger = l
}

// Ready probes the readiness endpoint and returns error if not 200 OK
// Uses caching to avoid excessive health checks
func (c *Client) Ready(ctx context.Context) error {
	c.healthMu.RLock()
	if time.Since(c.lastHealth) < c.healthTTL {
		c.healthMu.RUnlock()
		return nil // Cached healthy response
	}
	c.healthMu.RUnlock()

	c.healthMu.Lock()
	defer c.healthMu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(c.lastHealth) < c.healthTTL {
		return nil
	}

	err := c.checkHealth(ctx)
	if err != nil {
		if c.logger != nil {
			c.logger.Warn("Weaviate health check failed", "error", err, "url", c.BaseURL)
		}
		return err
	}

	c.lastHealth = time.Now()
	if c.logger != nil {
		c.logger.Debug("Weaviate health check passed", "url", c.BaseURL)
	}
	return nil
}

// checkHealth performs the actual health check
func (c *Client) checkHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/.well-known/ready", http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("weaviate not ready: %s", resp.Status)
	}
	return nil
}

// Health returns detailed health information
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/meta", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create meta request: %w", err)
	}

	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meta request failed: %w", err)
	}
	defer resp.Body.Close()

	health := &HealthStatus{
		Available: resp.StatusCode == http.StatusOK,
		Timestamp: time.Now(),
	}

	if resp.StatusCode != http.StatusOK {
		health.Message = fmt.Sprintf("weaviate meta endpoint returned: %s", resp.Status)
	} else {
		health.Message = "weaviate is healthy"
	}

	return health, nil
}

// HealthStatus represents the health status of Weaviate
type HealthStatus struct {
	Available bool      `json:"available"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Close gracefully closes the HTTP client connections
func (c *Client) Close() error {
	// Close idle connections in the transport
	if transport, ok := c.HTTP.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}
