package api

import (
    "bytes"
    "io/ioutil"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/platformbuilds/mirador-core/internal/config"
    "github.com/platformbuilds/mirador-core/internal/services"
    "github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// Ensure endpoints that were intentionally deregistered return 404 (not found)
func Test_DeactivatedEndpoints_Return404(t *testing.T) {
    log := logger.New("error")
    cfg := &config.Config{Environment: "development", Port: 0}
    cch := cache.NewNoopValkeyCache(log)
    vms := &services.VictoriaMetricsServices{
        Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
        Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
        Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
    }

    s := NewServer(cfg, log, cch, vms, nil)
    ts := httptest.NewServer(s.Handler())
    defer ts.Close()

    client := &http.Client{}

    tests := []struct{ method, path string }{
        {"POST", "/query"},
        {"POST", "/query_range"},
        {"POST", "/api/v1/metrics/query/aggregate/sum"},
        {"POST", "/api/v1/logs/query"},
        {"POST", "/api/v1/logs/search"},
        {"GET", "/api/v1/logs/tail"},
        {"GET", "/api/v1/traces/services"},
        {"POST", "/api/v1/metrics/search"},
    }

    for _, tt := range tests {
        req, err := http.NewRequest(tt.method, ts.URL+tt.path, bytes.NewBuffer([]byte("{}")))
        if err != nil {
            t.Fatalf("failed to create request for %s %s: %v", tt.method, tt.path, err)
        }
        resp, err := client.Do(req)
        if err != nil {
            t.Fatalf("request failed for %s %s: %v", tt.method, tt.path, err)
        }
        _, _ = ioutil.ReadAll(resp.Body)
        resp.Body.Close()
        if resp.StatusCode != http.StatusNotFound {
            t.Fatalf("expected 404 for %s %s, got %d", tt.method, tt.path, resp.StatusCode)
        }
    }

    // sanity check: docs endpoint still available
    resp, err := http.Get(ts.URL + "/api/openapi.json")
    if err != nil {
        t.Fatalf("openapi request failed: %v", err)
    }
    resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("openapi expected 200, got %d", resp.StatusCode)
    }
}
