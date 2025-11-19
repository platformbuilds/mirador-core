package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Covers auth-off branch and openapi.json route
func TestServer_OpenAPI_AndRootRedirect(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}
	// cfg.Auth.Enabled = false // Auth removed, so this line is commented out
	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, vms, nil)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// root should redirect to swagger
	// prevent following redirects so we observe 302
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("root status=%d", resp.StatusCode)
	}

	// openapi.json should return 200
	r2, err := http.Get(ts.URL + "/api/openapi.json")
	if err != nil {
		t.Fatalf("openapi.json: %v", err)
	}
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("openapi.json status=%d", r2.StatusCode)
	}
}

// Cover Start/Stop path (graceful shutdown)
// Note: Start/Stop path is exercised via integration/runtime, not unit tests, to avoid
// closing uninitialized gRPC clients. The server handler is covered via other tests.
