package api

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestServer_Start_And_Handler(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "development", Port: 0}
	cch := cache.NewNoopValkeyCache(log)
	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}

	s := NewServer(cfg, log, cch, vms, nil)

	// call Handler() to cover that method
	if s.Handler() == nil {
		t.Fatalf("handler should not be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- s.Start(ctx) }()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("start/shutdown error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server did not shut down in time")
	}
}

func TestServer_Start_Fails(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: -1}
	cch := cache.NewNoopValkeyCache(log)
	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}

	s := NewServer(cfg, log, cch, vms, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	// Expect immediate error due to invalid port
	if err := s.Start(ctx); err == nil {
		t.Fatalf("expected start error with invalid port")
	}
}
