package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// stubSchemaRepo is a minimal implementation to make schema routes register.
type stubSchemaRepo struct{}

func (stubSchemaRepo) UpsertMetric(ctx context.Context, m repo.MetricDef, author string) error {
	return nil
}
func (stubSchemaRepo) GetMetric(ctx context.Context, tenantID, metric string) (*repo.MetricDef, error) {
	return &repo.MetricDef{TenantID: tenantID, Metric: metric}, nil
}
func (stubSchemaRepo) ListMetricVersions(ctx context.Context, tenantID, metric string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	return nil
}
func (stubSchemaRepo) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
	return map[string]*repo.MetricLabelDef{}, nil
}
func (stubSchemaRepo) UpsertLogField(ctx context.Context, f repo.LogFieldDef, author string) error {
	return nil
}
func (stubSchemaRepo) GetLogField(ctx context.Context, tenantID, field string) (*repo.LogFieldDef, error) {
	return &repo.LogFieldDef{TenantID: tenantID, Field: field}, nil
}
func (stubSchemaRepo) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) UpsertLabel(ctx context.Context, tenantID, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	return nil
}
func (stubSchemaRepo) GetLabel(ctx context.Context, tenantID, name string) (*repo.LabelDef, error) {
	return &repo.LabelDef{Name: name, TenantID: tenantID}, nil
}
func (stubSchemaRepo) ListLabelVersions(ctx context.Context, tenantID, name string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetLabelVersion(ctx context.Context, tenantID, name string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) DeleteLabel(ctx context.Context, tenantID, name string) error { return nil }
func (stubSchemaRepo) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (stubSchemaRepo) GetTraceService(ctx context.Context, tenantID, service string) (*repo.TraceServiceDef, error) {
	return &repo.TraceServiceDef{TenantID: tenantID, Service: service}, nil
}
func (stubSchemaRepo) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (stubSchemaRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*repo.TraceOperationDef, error) {
	return &repo.TraceOperationDef{TenantID: tenantID, Service: service, Operation: operation}, nil
}
func (stubSchemaRepo) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) DeleteMetric(ctx context.Context, tenantID, metric string) error  { return nil }
func (stubSchemaRepo) DeleteLogField(ctx context.Context, tenantID, field string) error { return nil }
func (stubSchemaRepo) DeleteTraceService(ctx context.Context, tenantID, service string) error {
	return nil
}
func (stubSchemaRepo) DeleteTraceOperation(ctx context.Context, tenantID, service, operation string) error {
	return nil
}

func TestServer_AuthOn_And_SchemaRegistered(t *testing.T) {
	// Ensure switching gin mode for production path does not leak globally
	prev := gin.Mode()
	defer gin.SetMode(prev)

	log := logger.New("error")
	cfg := &config.Config{Environment: "production", Port: 0}
	cfg.Auth.Enabled = true

	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	s := NewServer(cfg, log, cch, grpc, vms, stubSchemaRepo{})
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// Public health (non-versioned) should work with auth enabled
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", resp.StatusCode)
	}

	// Protected endpoint should be unauthorized without token
	r2, err := http.Get(ts.URL + "/api/v1/logs/streams")
	if err != nil {
		t.Fatalf("logs streams: %v", err)
	}
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", r2.StatusCode)
	}

	// Schema routes registered (not invoked here due to auth)
}
