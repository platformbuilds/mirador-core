package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_example(t *testing.T) {
	path := filepath.Join(".", "simulator-config.yaml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Telemetry.ServiceNames.APIGateway != "api-gateway" {
		t.Fatalf("unexpected api_gateway name: %v", cfg.Telemetry.ServiceNames.APIGateway)
	}

	if cfg.Failure.Mode != "bursty" {
		t.Fatalf("unexpected failure mode: %v", cfg.Failure.Mode)
	}

	if len(cfg.Failure.Bursts) == 0 {
		t.Fatalf("expected at least one burst in example config")
	}

	// check that burst start/duration parseable by creating scheduler
	_, err = newFailureScheduler(cfg.Failure, time.Now())
	if err != nil {
		t.Fatalf("newFailureScheduler failed: %v", err)
	}
}
