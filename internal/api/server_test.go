package api

import (
    "testing"
    "github.com/platformbuilds/mirador-core/internal/config"
    "github.com/platformbuilds/mirador-core/internal/services"
    grpcclients "github.com/platformbuilds/mirador-core/internal/grpc/clients"
    "github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// Verifies the server can be constructed with minimal config without side effects.
func TestNewServer_Constructs(t *testing.T) {
    cfg := &config.Config{}
    cfg.Environment = "development"
    cfg.Cache.Nodes = []string{"localhost:6379"}
    // No schema store configured (schemaRepo nil)

    log := logger.New("error")
    valley := cache.NewNoopValkeyCache(log)

    // Minimal Victoria services (no network calls during construction)
    vms, err := services.NewVictoriaMetricsServices(cfg.Database, log)
    if err != nil { t.Fatalf("vm services error: %v", err) }

    s := NewServer(cfg, log, valley, &grpcclients.GRPCClients{}, vms, nil)
    if s == nil || s.router == nil {
        t.Fatalf("server or router is nil")
    }
}
