package weaviate

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/platformbuilds/mirador-core/internal/config"
)

type Client struct {
    BaseURL string
    APIKey  string
    HTTP    *http.Client
}

func New(cfg config.WeaviateConfig) *Client {
    scheme := cfg.Scheme
    if scheme == "" { scheme = "http" }
    host := cfg.Host
    if host == "" { host = "localhost" }
    port := cfg.Port
    if port == 0 { port = 8080 }
    base := fmt.Sprintf("%s://%s:%d", scheme, host, port)
    return &Client{
        BaseURL: base,
        APIKey:  cfg.APIKey,
        HTTP:    &http.Client{ Timeout: 10 * time.Second },
    }
}

// Ready probes the readiness endpoint and returns error if not 200 OK
func (c *Client) Ready(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/.well-known/ready", nil)
    if c.APIKey != "" {
        req.Header.Set("Authorization", "Bearer "+c.APIKey)
    }
    resp, err := c.HTTP.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("weaviate not ready: %s", resp.Status)
    }
    return nil
}

