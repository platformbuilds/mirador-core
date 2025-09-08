package clients

import (
    "testing"
    "github.com/platformbuilds/mirador-core/internal/config"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestNewGRPCClients_DevelopmentNoop(t *testing.T) {
    cfg := &config.Config{Environment: "development"}
    cfg.GRPC.PredictEngine.Endpoint = "localhost:0"
    cfg.GRPC.RCAEngine.Endpoint = "localhost:0"
    cfg.GRPC.AlertEngine.Endpoint = "localhost:0"
    log := logger.New("error")
    g, err := NewGRPCClients(cfg, log)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if g == nil { t.Fatalf("nil clients") }
    if err := g.Close(); err != nil { t.Fatalf("close error: %v", err) }
}

