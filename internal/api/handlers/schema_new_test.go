package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// fakeSchemaRepo is an in-memory minimal SchemaStore used for handler tests.
type fakeSchemaRepo struct {
	mu      sync.Mutex
	metrics map[string]repo.MetricDef // key: tenant+"/"+metric
}

func (f *fakeSchemaRepo) key(tid, metric string) string { return tid + "/" + metric }

func (f *fakeSchemaRepo) UpsertMetric(_ context.Context, m repo.MetricDef, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.metrics == nil {
		f.metrics = map[string]repo.MetricDef{}
	}
	f.metrics[f.key(m.TenantID, m.Metric)] = m
	return nil
}
func (f *fakeSchemaRepo) GetMetric(_ context.Context, tenantID, metric string) (*repo.MetricDef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m, ok := f.metrics[f.key(tenantID, metric)]; ok {
		cp := m
		return &cp, nil
	}
	return nil, errors.New("not found")
}

// Unused methods for this test suite
func (*fakeSchemaRepo) ListMetricVersions(_ context.Context, _, _ string) ([]repo.VersionInfo, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) GetMetricVersion(_ context.Context, _, _ string, _ int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, errors.New("not implemented")
}
func (*fakeSchemaRepo) UpsertMetricLabel(_ context.Context, _, _, _, _ string, _ bool, _ map[string]any, _ string) error {
	return errors.New("not implemented")
}
func (*fakeSchemaRepo) GetMetricLabelDefs(_ context.Context, _, _ string, _ []string) (map[string]*repo.MetricLabelDef, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) UpsertLogField(_ context.Context, _ repo.LogFieldDef, _ string) error {
	return errors.New("not implemented")
}
func (*fakeSchemaRepo) GetLogField(_ context.Context, _, _ string) (*repo.LogFieldDef, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) ListLogFieldVersions(_ context.Context, _, _ string) ([]repo.VersionInfo, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) GetLogFieldVersion(_ context.Context, _, _ string, _ int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, errors.New("not implemented")
}
func (*fakeSchemaRepo) UpsertLabel(_ context.Context, _, _, _ string, _ bool, _ map[string]any, _, _, _, _ string) error {
	return errors.New("not implemented")
}
func (*fakeSchemaRepo) GetLabel(_ context.Context, _, _ string) (*repo.LabelDef, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) ListLabelVersions(_ context.Context, _, _ string) ([]repo.VersionInfo, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) GetLabelVersion(_ context.Context, _, _ string, _ int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, errors.New("not implemented")
}
func (*fakeSchemaRepo) DeleteLabel(_ context.Context, _, _ string) error { return nil }
func (*fakeSchemaRepo) UpsertTraceServiceWithAuthor(_ context.Context, _, _, _, _, _, _ string, _ []string, _ string) error {
	return errors.New("not implemented")
}
func (*fakeSchemaRepo) GetTraceService(_ context.Context, _, _ string) (*repo.TraceServiceDef, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) ListTraceServiceVersions(_ context.Context, _, _ string) ([]repo.VersionInfo, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) GetTraceServiceVersion(_ context.Context, _, _ string, _ int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, errors.New("not implemented")
}
func (*fakeSchemaRepo) UpsertTraceOperationWithAuthor(_ context.Context, _, _, _, _, _, _, _ string, _ []string, _ string) error {
	return errors.New("not implemented")
}
func (*fakeSchemaRepo) GetTraceOperation(_ context.Context, _, _, _ string) (*repo.TraceOperationDef, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) ListTraceOperationVersions(_ context.Context, _, _, _ string) ([]repo.VersionInfo, error) {
	return nil, errors.New("not implemented")
}
func (*fakeSchemaRepo) GetTraceOperationVersion(_ context.Context, _, _, _ string, _ int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, errors.New("not implemented")
}
func (*fakeSchemaRepo) DeleteMetric(_ context.Context, _, _ string) error            { return nil }
func (*fakeSchemaRepo) DeleteLogField(_ context.Context, _, _ string) error          { return nil }
func (*fakeSchemaRepo) DeleteTraceService(_ context.Context, _, _ string) error      { return nil }
func (*fakeSchemaRepo) DeleteTraceOperation(_ context.Context, _, _, _ string) error { return nil }

// Test that posting a metric with tags as array works and it can be retrieved.
func TestSchema_MetricUpsertAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")
	rep := &fakeSchemaRepo{}
	cch := cache.NewNoopValkeyCache(log)
	h := NewSchemaHandler(rep, nil, nil, cch, log, 1<<20)

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(func(c *gin.Context) { c.Set("tenant_id", "t1"); c.Next() })
	v1.POST("/schema/metrics", h.UpsertMetric)
	v1.GET("/schema/metrics/:metric", h.GetMetric)

	payload := map[string]any{
		"tenantId":    "t1",
		"metric":      "http_requests_total",
		"description": "requests",
		"owner":       "team",
		"tags":        []string{"app=web", "env=dev"},
		"author":      "tester",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schema/metrics", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upsert metric failed: %d %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/schema/metrics/http_requests_total", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("get metric failed: %d %s", w2.Code, w2.Body.String())
	}
}
