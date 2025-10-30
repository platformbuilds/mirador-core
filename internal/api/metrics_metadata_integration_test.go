package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestMetricsMetadataIntegration(t *testing.T) {
	// Create a test configuration with metrics metadata enabled
	cfg := &config.Config{
		Environment: "test",
		Port:        0, // Use random port
		Search: config.SearchConfig{
			DefaultEngine: "lucene",
			EnableBleve:   true,
			Bleve: config.BleveConfig{
				MetricsEnabled: true,
				IndexPath:      "/tmp/mirador-test-bleve",
			},
		},
		UnifiedQuery: config.UnifiedQueryConfig{
			Enabled: true,
		},
	}

	log := logger.New("error")

	// Create mock services
	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}

	grpcClients := &clients.GRPCClients{}
	valleyCache := cache.NewNoopValkeyCache(log)
	var schemaRepo repo.SchemaStore // nil for this test

	// Create server - this should initialize metrics metadata components
	server := NewServer(cfg, log, valleyCache, grpcClients, vms, schemaRepo)
	if server == nil {
		t.Fatal("Failed to create server")
	}

	// Check that metrics metadata components were initialized
	if server.metricsMetadataIndexer == nil {
		t.Error("MetricsMetadataIndexer should be initialized")
	}
	if server.metricsMetadataSynchronizer == nil {
		t.Error("MetricsMetadataSynchronizer should be initialized")
	}

	// Create a test HTTP request to check if routes are registered
	req := httptest.NewRequest("GET", "/api/v1/metrics/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// The endpoint should exist (even if it returns an error due to no VictoriaMetrics)
	if w.Code == http.StatusNotFound {
		t.Error("Metrics health endpoint should be registered")
	}

	t.Logf("Metrics metadata integration test passed - components initialized and routes registered")
}
