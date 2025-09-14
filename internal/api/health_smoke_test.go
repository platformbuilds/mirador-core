package api

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/platformbuilds/mirador-core/internal/config"
    "github.com/platformbuilds/mirador-core/internal/grpc/clients"
    "github.com/platformbuilds/mirador-core/internal/services"
    "github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// Smoke test for /health route with minimal dependencies.
func TestHealth_OK(t *testing.T) {
    log := logger.New("error")

    cfg := &config.Config{}
    cfg.Environment = "test"
    cfg.Port = 0
    cfg.Auth.Enabled = false

    // Minimal services and clients (no endpoints required for /health)
    vms := &services.VictoriaMetricsServices{
        Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
        Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
        Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
    }
    grpc := &clients.GRPCClients{}
    cch := cache.NewNoopValkeyCache(log)

    s := NewServer(cfg, log, cch, grpc, vms, nil)
    ts := httptest.NewServer(s.router)
    defer ts.Close()

    resp, err := http.Get(ts.URL + "/health")
    if err != nil {
        t.Fatalf("GET /health failed: %v", err)
    }
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("unexpected status: %s", resp.Status)
    }
}

