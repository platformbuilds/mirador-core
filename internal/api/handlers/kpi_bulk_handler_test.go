package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockRepo implements repo.KPIRepo with minimal stubs for tests
type mockRepo struct {
	upserted []*models.KPIDefinition
}

// Implement new KPIRepo interface methods
func (m *mockRepo) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	_ = m.UpsertKPI(ctx, k)
	return k, "created", nil
}
func (m *mockRepo) ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return k, "updated", nil
}
func (m *mockRepo) CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	created := make([]*models.KPIDefinition, 0, len(items))
	errs := make([]error, len(items))
	for i, k := range items {
		_, _, err := m.CreateKPI(ctx, k)
		errs[i] = err
		if err == nil {
			created = append(created, k)
		}
	}
	return created, errs
}
func (m *mockRepo) ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	modified := make([]*models.KPIDefinition, 0, len(items))
	errs := make([]error, len(items))
	for i, k := range items {
		_, _, err := m.ModifyKPI(ctx, k)
		errs[i] = err
		if err == nil {
			modified = append(modified, k)
		}
	}
	return modified, errs
}
func (m *mockRepo) DeleteKPIBulk(ctx context.Context, ids []string) []error {
	errs := make([]error, len(ids))
	for i := range ids {
		errs[i] = nil
	}
	return errs
}

func (m *mockRepo) UpsertKPI(ctx context.Context, kpi *models.KPIDefinition) error {
	m.upserted = append(m.upserted, kpi)
	return nil
}
func (m *mockRepo) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	return nil, nil
}

// removed: old ListKPIs with int count

// Correct signature for KPIRepo interface
func (m *mockRepo) ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
	return nil, 0, nil
}
func (m *mockRepo) DeleteKPI(ctx context.Context, id string) error { return nil }

// EnsureTelemetryStandards satisfies the KPIRepo interface for tests.
func (m *mockRepo) EnsureTelemetryStandards(ctx context.Context, cfg *config.EngineConfig) error {
	return nil
}

// --- SchemaStore stubs (many methods) ---
func (m *mockRepo) UpsertMetric(ctx context.Context, def repo.MetricDef, author string) error {
	return nil
}
func (m *mockRepo) GetMetric(ctx context.Context, metric string) (*repo.MetricDef, error) {
	return nil, nil
}
func (m *mockRepo) ListMetricVersions(ctx context.Context, metric string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepo) GetMetricVersion(ctx context.Context, metric string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepo) UpsertMetricLabel(ctx context.Context, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	return nil
}
func (m *mockRepo) GetMetricLabelDefs(ctx context.Context, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
	return nil, nil
}
func (m *mockRepo) UpsertLogField(ctx context.Context, f repo.LogFieldDef, author string) error {
	return nil
}
func (m *mockRepo) GetLogField(ctx context.Context, field string) (*repo.LogFieldDef, error) {
	return nil, nil
}
func (m *mockRepo) ListLogFieldVersions(ctx context.Context, field string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepo) GetLogFieldVersion(ctx context.Context, field string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepo) UpsertTraceServiceWithAuthor(ctx context.Context, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (m *mockRepo) GetTraceService(ctx context.Context, service string) (*repo.TraceServiceDef, error) {
	return nil, nil
}
func (m *mockRepo) ListTraceServiceVersions(ctx context.Context, service string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepo) GetTraceServiceVersion(ctx context.Context, service string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepo) UpsertTraceOperationWithAuthor(ctx context.Context, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (m *mockRepo) GetTraceOperation(ctx context.Context, service, operation string) (*repo.TraceOperationDef, error) {
	return nil, nil
}
func (m *mockRepo) ListTraceOperationVersions(ctx context.Context, service, operation string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepo) GetTraceOperationVersion(ctx context.Context, service, operation string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepo) UpsertLabel(ctx context.Context, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	return nil
}
func (m *mockRepo) GetLabel(ctx context.Context, name string) (*repo.LabelDef, error) {
	return nil, nil
}
func (m *mockRepo) ListLabelVersions(ctx context.Context, name string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepo) GetLabelVersion(ctx context.Context, name string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepo) DeleteLabel(ctx context.Context, name string) error           { return nil }
func (m *mockRepo) DeleteMetric(ctx context.Context, metric string) error        { return nil }
func (m *mockRepo) DeleteLogField(ctx context.Context, field string) error       { return nil }
func (m *mockRepo) DeleteTraceService(ctx context.Context, service string) error { return nil }
func (m *mockRepo) DeleteTraceOperation(ctx context.Context, service, operation string) error {
	return nil
}

func setupHandlerForTest() (*KPIHandler, *mockRepo) {
	mr := &mockRepo{upserted: []*models.KPIDefinition{}}
	// build handler directly
	l := logger.NewMockLogger(&strings.Builder{})
	cfg := &config.Config{}
	h := &KPIHandler{repo: mr, cache: nil, logger: l, cfg: cfg}
	return h, mr
}

func TestBulkIngestJSON_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mr := setupHandlerForTest()

	// two valid KPI items
	a := &models.KPIDefinition{Name: "kpi-a", Layer: "impact", SignalType: "metrics", Sentiment: "negative", Definition: "something broke"}
	b := &models.KPIDefinition{Name: "kpi-b", Layer: "impact", SignalType: "metrics", Sentiment: "negative", Definition: "something else"}

	payload := map[string]any{"items": []*models.KPIDefinition{a, b}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs/bulk-json", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BulkIngestJSON(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var summary BulkSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if summary.SuccessCount != 2 || summary.FailureCount != 0 || summary.Total != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(mr.upserted) != 2 {
		t.Fatalf("expected 2 upserted, got %d", len(mr.upserted))
	}
}

func TestBulkIngestJSON_WithInvalidItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mr := setupHandlerForTest()

	// one valid, one invalid (missing name)
	a := &models.KPIDefinition{Name: "kpi-a", Layer: "impact", SignalType: "metrics", Sentiment: "negative", Definition: "ok"}
	b := &models.KPIDefinition{Layer: "cause", SignalType: "metrics", Sentiment: "negative"} // missing name and classifier for cause

	payload := map[string]any{"items": []*models.KPIDefinition{a, b}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs/bulk-json", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BulkIngestJSON(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var summary BulkSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if summary.SuccessCount != 1 || summary.FailureCount != 1 || summary.Total != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(mr.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(mr.upserted))
	}
}

func TestBulkIngestCSV_Minimal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mr := setupHandlerForTest()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "kpis.csv")
	csvData := "name,layer,signalType,sentiment,definition\n" +
		"csv-kpi,impact,metrics,negative,definition one\n" +
		",cause,metrics,negative,missing name row\n"
	io.Copy(part, strings.NewReader(csvData))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs/bulk-csv", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BulkIngestCSV(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var summary BulkSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if summary.SuccessCount != 1 || summary.FailureCount != 1 || summary.Total != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(mr.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(mr.upserted))
	}
}

func TestBulkIngestCSV_TagsAndExamples(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mr := setupHandlerForTest()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "kpis.csv")
	// header with mixed/uppercase to test case-insensitivity and an unknown column (FooColumn)
	csvData := "Name,Layer,SignalType,Sentiment,Tags,FooColumn,Examples,Definition\n" +
		"csv-kpi,impact,metrics,negative,\"a,b;c\",ignored,\"[{\"\"example\"\":true}]\",an example definition\n" +
		"csv-kpi-2,impact,metrics,negative,tag1,ignored,invalid-json,another definition\n"
	io.Copy(part, strings.NewReader(csvData))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs/bulk-csv", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BulkIngestCSV(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var summary BulkSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if summary.SuccessCount != 1 || summary.FailureCount != 1 || summary.Total != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(mr.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(mr.upserted))
	}

	// Validate tags for first row - should be split into parts by comma/semicolon
	if len(mr.upserted[0].Tags) == 0 {
		t.Fatalf("expected tags on first row, got none")
	}
	// Validate examples: first row had valid JSON, second had invalid JSON and should be a failure
	if len(mr.upserted[0].Examples) == 0 {
		t.Fatalf("expected examples for first row")
	}
	if len(mr.upserted) != 1 {
		t.Fatalf("expected only one upserted entry due to invalid examples in second row, got %d", len(mr.upserted))
	}

	// Assert failure points to second CSV row
	if len(summary.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(summary.Failures))
	}
	if summary.Failures[0].Row != 3 {
		t.Fatalf("expected failure row 3 (header=1, data rows=2,3), got %d", summary.Failures[0].Row)
	}
}

func TestBulkIngestJSON_DetGeneratedIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mr := setupHandlerForTest()

	a := &models.KPIDefinition{Name: "kpi-a", Layer: "impact", SignalType: "metrics", Sentiment: "negative", Definition: "something"}
	b := &models.KPIDefinition{Name: "kpi-b", Layer: "impact", SignalType: "metrics", Sentiment: "negative", Definition: "something else"}

	payload := map[string]any{"items": []*models.KPIDefinition{a, b}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs/bulk-json", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BulkIngestJSON(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}

	// All upserted items should have deterministic IDs set
	if len(mr.upserted) != 2 {
		t.Fatalf("expected 2 upserted, got %d", len(mr.upserted))
	}
	for i, k := range mr.upserted {
		if k.ID == "" {
			t.Fatalf("expected id to be generated for %+v", k)
		}
		// Also assert it matches the deterministic algorithm
		// Note: req.Items were a and b and they may be mutated in-place
		expected, err := services.GenerateDeterministicKPIID(mr.upserted[i])
		if err != nil {
			t.Fatalf("failed to generate deterministic id: %v", err)
		}
		if k.ID != expected {
			t.Fatalf("expected id %s got %s", expected, k.ID)
		}
	}
}

func TestBulkIngestJSON_MultipleFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, mr := setupHandlerForTest()

	a := &models.KPIDefinition{Name: "kpi-a", Layer: "impact", SignalType: "metrics", Sentiment: "negative", Definition: "ok"}
	b := &models.KPIDefinition{Layer: "cause", SignalType: "metrics", Sentiment: "negative"} // missing name and classifier
	c := &models.KPIDefinition{Name: "kpi-c", Layer: "impact"}                               // missing signalType & sentiment

	payload := map[string]any{"items": []*models.KPIDefinition{a, b, c}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs/bulk-json", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	cctx, _ := gin.CreateTestContext(w)
	cctx.Request = req

	h.BulkIngestJSON(cctx)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var summary BulkSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if summary.SuccessCount != 1 || summary.FailureCount != 2 || summary.Total != 3 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(mr.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(mr.upserted))
	}
}

// mockRepoFail returns an error on upsert when the KPI Name matches a value.
type mockRepoFail struct {
	upserted []*models.KPIDefinition
	failName string
}

// Implement new KPIRepo interface methods
func (m *mockRepoFail) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	if err := m.UpsertKPI(ctx, k); err != nil {
		return nil, "", err
	}
	return k, "created", nil
}
func (m *mockRepoFail) ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return k, "updated", nil
}
func (m *mockRepoFail) CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	created := make([]*models.KPIDefinition, 0, len(items))
	errs := make([]error, len(items))
	for i, k := range items {
		_, _, err := m.CreateKPI(ctx, k)
		errs[i] = err
		if err == nil {
			created = append(created, k)
		}
	}
	return created, errs
}
func (m *mockRepoFail) ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	modified := make([]*models.KPIDefinition, 0, len(items))
	errs := make([]error, len(items))
	for i, k := range items {
		_, _, err := m.ModifyKPI(ctx, k)
		errs[i] = err
		if err == nil {
			modified = append(modified, k)
		}
	}
	return modified, errs
}
func (m *mockRepoFail) DeleteKPIBulk(ctx context.Context, ids []string) []error {
	errs := make([]error, len(ids))
	for i := range ids {
		errs[i] = nil
	}
	return errs
}

func (m *mockRepoFail) UpsertKPI(ctx context.Context, kpi *models.KPIDefinition) error {
	if kpi != nil && kpi.Name == m.failName {
		return errors.New("fail upsert")
	}
	m.upserted = append(m.upserted, kpi)
	return nil
}

// implement required repo methods
func (m *mockRepoFail) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	return nil, nil
}

// removed: old ListKPIs with int count

// Correct signature for KPIRepo interface
func (m *mockRepoFail) ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
	return nil, 0, nil
}
func (m *mockRepoFail) DeleteKPI(ctx context.Context, id string) error { return nil }
func (m *mockRepoFail) UpsertMetric(ctx context.Context, def repo.MetricDef, author string) error {
	return nil
}
func (m *mockRepoFail) GetMetric(ctx context.Context, metric string) (*repo.MetricDef, error) {
	return nil, nil
}
func (m *mockRepoFail) ListMetricVersions(ctx context.Context, metric string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepoFail) GetMetricVersion(ctx context.Context, metric string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepoFail) UpsertMetricLabel(ctx context.Context, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	return nil
}
func (m *mockRepoFail) GetMetricLabelDefs(ctx context.Context, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
	return nil, nil
}
func (m *mockRepoFail) UpsertLogField(ctx context.Context, f repo.LogFieldDef, author string) error {
	return nil
}
func (m *mockRepoFail) GetLogField(ctx context.Context, field string) (*repo.LogFieldDef, error) {
	return nil, nil
}
func (m *mockRepoFail) ListLogFieldVersions(ctx context.Context, field string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepoFail) GetLogFieldVersion(ctx context.Context, field string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepoFail) UpsertTraceServiceWithAuthor(ctx context.Context, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (m *mockRepoFail) GetTraceService(ctx context.Context, service string) (*repo.TraceServiceDef, error) {
	return nil, nil
}
func (m *mockRepoFail) ListTraceServiceVersions(ctx context.Context, service string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepoFail) GetTraceServiceVersion(ctx context.Context, service string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepoFail) UpsertTraceOperationWithAuthor(ctx context.Context, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (m *mockRepoFail) GetTraceOperation(ctx context.Context, service, operation string) (*repo.TraceOperationDef, error) {
	return nil, nil
}
func (m *mockRepoFail) ListTraceOperationVersions(ctx context.Context, service, operation string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepoFail) GetTraceOperationVersion(ctx context.Context, service, operation string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepoFail) UpsertLabel(ctx context.Context, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	return nil
}
func (m *mockRepoFail) GetLabel(ctx context.Context, name string) (*repo.LabelDef, error) {
	return nil, nil
}
func (m *mockRepoFail) ListLabelVersions(ctx context.Context, name string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockRepoFail) GetLabelVersion(ctx context.Context, name string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockRepoFail) DeleteLabel(ctx context.Context, name string) error           { return nil }
func (m *mockRepoFail) DeleteMetric(ctx context.Context, metric string) error        { return nil }
func (m *mockRepoFail) DeleteLogField(ctx context.Context, field string) error       { return nil }
func (m *mockRepoFail) DeleteTraceService(ctx context.Context, service string) error { return nil }
func (m *mockRepoFail) DeleteTraceOperation(ctx context.Context, service, operation string) error {
	return nil
}

// EnsureTelemetryStandards satisfies the KPIRepo interface for tests.
func (m *mockRepoFail) EnsureTelemetryStandards(ctx context.Context, cfg *config.EngineConfig) error {
	return nil
}

func TestBulkIngestJSON_UpsertErrorDoesNotStopBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr := &mockRepoFail{upserted: []*models.KPIDefinition{}, failName: "bad"}
	l := logger.NewMockLogger(&strings.Builder{})
	cfg := &config.Config{}
	h := &KPIHandler{repo: mr, cache: nil, logger: l, cfg: cfg}

	a := &models.KPIDefinition{Name: "bad", Layer: "impact", SignalType: "metrics", Sentiment: "negative", Definition: "fail this"}
	b := &models.KPIDefinition{Name: "good", Layer: "impact", SignalType: "metrics", Sentiment: "negative", Definition: "ok"}
	payload := map[string]any{"items": []*models.KPIDefinition{a, b}}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/kpi/defs/bulk-json", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BulkIngestJSON(c)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
	var summary BulkSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if summary.SuccessCount != 1 || summary.FailureCount != 1 || summary.Total != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(mr.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(mr.upserted))
	}
}
