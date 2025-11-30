package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/rca"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockKPIRepo is a simple mock for KPI repository
type mockKPIRepo struct {
	kpis map[string]*models.KPIDefinition
}

func (m *mockKPIRepo) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	if m.kpis == nil {
		return nil, nil
	}
	kpi, ok := m.kpis[id]
	if !ok {
		return nil, nil
	}
	return kpi, nil
}

func (m *mockKPIRepo) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return nil, "", nil
}

func (m *mockKPIRepo) CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	return nil, nil
}

func (m *mockKPIRepo) ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return nil, "", nil
}

func (m *mockKPIRepo) ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	return nil, nil
}

func (m *mockKPIRepo) DeleteKPI(ctx context.Context, id string) (repo.DeleteResult, error) {
	return repo.DeleteResult{}, nil
}

func (m *mockKPIRepo) DeleteKPIBulk(ctx context.Context, ids []string) []error {
	return nil
}

func (m *mockKPIRepo) ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
	return nil, 0, nil
}

func (m *mockKPIRepo) EnsureTelemetryStandards(ctx context.Context, cfg *config.EngineConfig) error {
	return nil
}

// mockRCAEngineWithUUIDs returns UUIDs in service/component fields
type mockRCAEngineWithUUIDs struct{}

func (m *mockRCAEngineWithUUIDs) ComputeRCA(
	ctx context.Context,
	incident *rca.IncidentContext,
	opts rca.RCAOptions,
) (*rca.RCAIncident, error) {
	return nil, nil
}

func (m *mockRCAEngineWithUUIDs) ComputeRCAByTimeRange(ctx context.Context, tr rca.TimeRange) (*rca.RCAIncident, error) {
	// Create an incident with UUID-based identifiers
	incident := &rca.IncidentContext{
		ID:            "corr_test_123",
		ImpactService: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", // KPI UUID
		ImpactSignal: rca.ImpactSignal{
			ServiceName: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
			MetricName:  "864c82d3-e941-5020-9dbc-99b4dcb0318d", // Different KPI UUID
			Direction:   "higher_is_worse",
		},
		TimeBounds:    rca.IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End},
		ImpactSummary: "Test incident with UUIDs",
		Severity:      0.85,
		CreatedAt:     time.Now().UTC(),
	}

	result := rca.NewRCAIncident(incident)

	// Create a chain with UUID-based steps
	chain := rca.NewRCAChain()

	step1 := rca.NewRCAStep(1, "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", "864c82d3-e941-5020-9dbc-99b4dcb0318d")
	step1.TimeRange = tr
	step1.Ring = rca.RingImmediate
	step1.Direction = rca.DirectionSame
	step1.Score = 0.9
	chain.AddStep(step1)

	step2 := rca.NewRCAStep(2, "864c82d3-e941-5020-9dbc-99b4dcb0318d", "component-x")
	step2.TimeRange = tr
	step2.Ring = rca.RingImmediate
	step2.Direction = rca.DirectionUpstream
	step2.Score = 0.85
	chain.AddStep(step2)

	chain.Score = 0.875
	result.AddChain(chain)
	result.SetRootCauseFromBestChain()

	return result, nil
}

// stubInlineEngine returns an incident with inline UUID tokens in ImpactSummary
type stubInlineEngine struct{}

func (s *stubInlineEngine) ComputeRCA(ctx context.Context, incident *rca.IncidentContext, opts rca.RCAOptions) (*rca.RCAIncident, error) {
	return nil, nil
}

// stubStepSummaryEngine returns an incident with RCAStep.Summary fields that
// contain inline KPI UUID tokens to exercise summary replacement logic.
type stubStepSummaryEngine struct{}

func (s *stubStepSummaryEngine) ComputeRCA(ctx context.Context, incident *rca.IncidentContext, opts rca.RCAOptions) (*rca.RCAIncident, error) {
	return nil, nil
}

func (s *stubStepSummaryEngine) ComputeRCAByTimeRange(ctx context.Context, tr rca.TimeRange) (*rca.RCAIncident, error) {
	ic := &rca.IncidentContext{
		ID:            "corr_test_step_summary",
		ImpactService: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
		ImpactSignal:  rca.ImpactSignal{ServiceName: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", MetricName: "864c82d3-e941-5020-9dbc-99b4dcb0318d", Direction: "higher_is_worse"},
		TimeBounds:    rca.IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End},
		ImpactSummary: "impact summary",
		Severity:      0.5,
		CreatedAt:     time.Now().UTC(),
	}

	inc := rca.NewRCAIncident(ic)
	chain := rca.NewRCAChain()
	step1 := rca.NewRCAStep(1, "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", "864c82d3-e941-5020-9dbc-99b4dcb0318d")
	step1.Summary = "Found suspicious KPI 47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3 affecting downstream 864c82d3-e941-5020-9dbc-99b4dcb0318d-dep"
	step1.TimeRange = tr
	step1.Ring = rca.RingImmediate
	step1.Score = 0.8
	chain.AddStep(step1)

	inc.AddChain(chain)
	return inc, nil
}

// stubCompEngine returns an incident with a step whose Component contains a
// uuid with an appended suffix (e.g. <uuid>-dep) so we can test suffix
// resolution logic.
type stubCompEngine struct{}

func (s *stubCompEngine) ComputeRCA(ctx context.Context, incident *rca.IncidentContext, opts rca.RCAOptions) (*rca.RCAIncident, error) {
	return nil, nil
}

func (s *stubCompEngine) ComputeRCAByTimeRange(ctx context.Context, tr rca.TimeRange) (*rca.RCAIncident, error) {
	ic := &rca.IncidentContext{ID: "corr_test_comp_suffix", ImpactService: "unknown", ImpactSignal: rca.ImpactSignal{ServiceName: "unknown", MetricName: "unknown", Direction: "higher_is_worse"}, TimeBounds: rca.IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End}, ImpactSummary: "impact summary", Severity: 0.5, CreatedAt: time.Now().UTC()}
	inc := rca.NewRCAIncident(ic)
	chain := rca.NewRCAChain()
	// component has suffix -dep
	step := rca.NewRCAStep(1, "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", "864c82d3-e941-5020-9dbc-99b4dcb0318d-dep")
	step.TimeRange = tr
	step.Ring = rca.RingImmediate
	step.Score = 0.7
	chain.AddStep(step)
	inc.AddChain(chain)
	return inc, nil
}

// stubServiceEngine returns an incident chain with whyIndex 1/2/3 patterns used
// by TestHandleComputeRCA_ServiceAndSummarySuffixResolution.
type stubServiceEngine struct{}

func (s *stubServiceEngine) ComputeRCA(ctx context.Context, incident *rca.IncidentContext, opts rca.RCAOptions) (*rca.RCAIncident, error) {
	return nil, nil
}

func (s *stubServiceEngine) ComputeRCAByTimeRange(ctx context.Context, tr rca.TimeRange) (*rca.RCAIncident, error) {
	ic := &rca.IncidentContext{ID: "corr_test_service_suffix", ImpactService: "Service Request Failure Count (Impact)", ImpactSignal: rca.ImpactSignal{ServiceName: "Service Request Failure Count (Impact)", MetricName: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", Direction: "higher_is_worse"}, TimeBounds: rca.IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End}, ImpactSummary: "impact summary", Severity: 0.5, CreatedAt: time.Now().UTC()}
	inc := rca.NewRCAIncident(ic)
	chain := rca.NewRCAChain()

	// whyIndex 1: already resolved
	step1 := rca.NewRCAStep(1, "Service Request Failure Count (Impact)", "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3")
	step1.TimeRange = tr
	step1.Ring = rca.RingImmediate
	step1.Score = 0.9
	chain.AddStep(step1)

	// whyIndex 2: service is plain uuid
	step2 := rca.NewRCAStep(2, "864c82d3-e941-5020-9dbc-99b4dcb0318d", "864c82d3-e941-5020-9dbc-99b4dcb0318d")
	step2.Summary = "Why 2: 864c82d3-e941-5020-9dbc-99b4dcb0318d anomalies detected"
	step2.TimeRange = tr
	step2.Ring = rca.RingImmediate
	step2.Score = 0.9
	chain.AddStep(step2)

	// whyIndex 3: service contains suffix -dep
	step3 := rca.NewRCAStep(3, "864c82d3-e941-5020-9dbc-99b4dcb0318d-dep", "dependency")
	step3.Summary = "Why 3: 864c82d3-e941-5020-9dbc-99b4dcb0318d-dep (dependency) at 03:40:55"
	step3.TimeRange = tr
	step3.Ring = rca.RingShort
	step3.Score = 0.63
	chain.AddStep(step3)

	inc.AddChain(chain)
	return inc, nil
}

func (s *stubInlineEngine) ComputeRCAByTimeRange(ctx context.Context, tr rca.TimeRange) (*rca.RCAIncident, error) {
	incident := &rca.IncidentContext{
		ID:            "corr_test_inline_123",
		ImpactService: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
		ImpactSignal: rca.ImpactSignal{
			ServiceName: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
			MetricName:  "864c82d3-e941-5020-9dbc-99b4dcb0318d-dep",
			Direction:   "higher_is_worse",
		},
		TimeBounds:    rca.IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End},
		ImpactSummary: "Observed spike in 47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3 affecting 864c82d3-e941-5020-9dbc-99b4dcb0318d-dep",
		Severity:      0.85,
		CreatedAt:     time.Now().UTC(),
	}
	result := rca.NewRCAIncident(incident)
	return result, nil
}

func TestHandleComputeRCA_UUIDResolution(t *testing.T) {
	// Create mock KPI repository with test KPIs
	mockRepo := &mockKPIRepo{
		kpis: map[string]*models.KPIDefinition{
			"47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3": {
				ID:      "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
				Name:    "Transaction Success Rate",
				Formula: "sum(rate(transactions_total{status=\"success\"}[5m]))",
				Kind:    "kpi",
				Layer:   "impact",
			},
			"864c82d3-e941-5020-9dbc-99b4dcb0318d": {
				ID:      "864c82d3-e941-5020-9dbc-99b4dcb0318d",
				Name:    "Error Rate Percentage",
				Formula: "sum(rate(errors_total[5m])) / sum(rate(requests_total[5m])) * 100",
				Kind:    "kpi",
				Layer:   "cause",
			},
		},
	}

	// Create handler with mock engine and repo
	engine := &mockRCAEngineWithUUIDs{}
	lg := logger.NewMockLogger(nil)
	cfg := config.EngineConfig{
		StrictTimeWindowPayload: true,
	}
	handler := &RCAHandler{
		rcaEngine:               engine,
		logger:                  logging.FromCoreLogger(lg),
		engineCfg:               cfg,
		strictTimeWindowPayload: true,
		kpiRepo:                 mockRepo,
	}

	// Create test request
	reqBody := map[string]interface{}{
		"startTime": "2025-11-25T15:33:16Z",
		"endTime":   "2025-11-25T15:48:16Z",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Setup Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute handler
	handler.HandleComputeRCA(c)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.RCAResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.NotNil(t, response.Data)

	// Verify UUIDs were resolved to names
	assert.Equal(t, "Transaction Success Rate", response.Data.Impact.ImpactService,
		"ImpactService should be resolved to KPI name")
	assert.Equal(t, "Error Rate Percentage", response.Data.Impact.MetricName,
		"MetricName should be resolved to KPI name")

	// Verify UUID fields are populated
	assert.Equal(t, "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", response.Data.Impact.ImpactServiceUUID,
		"ImpactServiceUUID should contain original UUID")
	assert.Equal(t, "864c82d3-e941-5020-9dbc-99b4dcb0318d", response.Data.Impact.MetricNameUUID,
		"MetricNameUUID should contain original UUID")

	// Verify steps were resolved
	assert.NotEmpty(t, response.Data.Chains)
	assert.NotEmpty(t, response.Data.Chains[0].Steps)

	step1 := response.Data.Chains[0].Steps[0]
	assert.Equal(t, "Transaction Success Rate", step1.Service,
		"Step 1 Service should be resolved to KPI name")
	assert.Equal(t, "Error Rate Percentage", step1.Component,
		"Step 1 Component should be resolved to KPI name")
	assert.Equal(t, "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", step1.ServiceUUID,
		"Step 1 ServiceUUID should contain original UUID")
	assert.Equal(t, "864c82d3-e941-5020-9dbc-99b4dcb0318d", step1.ComponentUUID,
		"Step 1 ComponentUUID should contain original UUID")

	step2 := response.Data.Chains[0].Steps[1]
	assert.Equal(t, "Error Rate Percentage", step2.Service,
		"Step 2 Service should be resolved to KPI name")
	assert.Equal(t, "component-x", step2.Component,
		"Step 2 Component should remain unchanged (not a UUID)")
	assert.Equal(t, "864c82d3-e941-5020-9dbc-99b4dcb0318d", step2.ServiceUUID,
		"Step 2 ServiceUUID should contain original UUID")
	assert.Empty(t, step2.ComponentUUID,
		"Step 2 ComponentUUID should be empty (not a KPI)")

	// Verify ImpactPath was resolved
	assert.NotEmpty(t, response.Data.Chains[0].ImpactPath)
	assert.Equal(t, "Transaction Success Rate", response.Data.Chains[0].ImpactPath[0],
		"ImpactPath[0] should be resolved to KPI name")
	assert.Equal(t, "Error Rate Percentage", response.Data.Chains[0].ImpactPath[1],
		"ImpactPath[1] should be resolved to KPI name")

	t.Logf("✓ UUID resolution test passed successfully")
	t.Logf("  - ImpactService: %s (UUID: %s)", response.Data.Impact.ImpactService, response.Data.Impact.ImpactServiceUUID)
	t.Logf("  - MetricName: %s (UUID: %s)", response.Data.Impact.MetricName, response.Data.Impact.MetricNameUUID)
	t.Logf("  - Step1 Service: %s (UUID: %s)", step1.Service, step1.ServiceUUID)
	t.Logf("  - Step1 Component: %s (UUID: %s)", step1.Component, step1.ComponentUUID)
}

func TestHandleComputeRCA_InlineImpactSummaryResolution(t *testing.T) {
	// Create mock KPI repository with test KPIs (same as previous test)
	mockRepo := &mockKPIRepo{
		kpis: map[string]*models.KPIDefinition{
			"47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3": {
				ID:      "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
				Name:    "Transaction Success Rate",
				Formula: "sum(rate(transactions_total{status=\"success\"}[5m]))",
				Kind:    "kpi",
				Layer:   "impact",
			},
			"864c82d3-e941-5020-9dbc-99b4dcb0318d": {
				ID:      "864c82d3-e941-5020-9dbc-99b4dcb0318d",
				Name:    "Error Rate Percentage",
				Formula: "sum(rate(errors_total[5m])) / sum(rate(requests_total[5m])) * 100",
				Kind:    "kpi",
				Layer:   "cause",
			},
		},
	}

	// Engine returns an IncidentContext with UUID tokens embedded inside ImpactSummary
	engine := &mockRCAEngineWithUUIDs{}
	lg := logger.NewMockLogger(nil)
	cfg := config.EngineConfig{
		StrictTimeWindowPayload: true,
	}
	handler := &RCAHandler{
		rcaEngine:               engine,
		logger:                  logging.FromCoreLogger(lg),
		engineCfg:               cfg,
		strictTimeWindowPayload: true,
		kpiRepo:                 mockRepo,
	}

	// Use a stub engine that returns inline UUID tokens inside ImpactSummary
	handler.rcaEngine = &stubInlineEngine{}

	// send the request like previous test
	reqBody := map[string]interface{}{
		"startTime": "2025-11-25T15:33:16Z",
		"endTime":   "2025-11-25T15:48:16Z",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	// stubInlineEngine is already used to return an ImpactSummary containing inline tokens

	// Execute handler
	handler.HandleComputeRCA(c)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	var response models.RCAResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.NotNil(t, response.Data)

	// Inspect ImpactSummary to ensure inline tokens were replaced with names
	// Expect the UUID and the uuid-with-suffix to be replaced with the KPI names
	expected := "Observed spike in Transaction Success Rate affecting Error Rate Percentage-dep"
	assert.Equal(t, expected, response.Data.Impact.ImpactSummary)
}

func TestHandleComputeRCA_StepSummaryInlineResolution(t *testing.T) {
	mockRepo := &mockKPIRepo{
		kpis: map[string]*models.KPIDefinition{
			"47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3": {ID: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", Name: "Transaction Success Rate", Kind: "kpi", Layer: "impact"},
			"864c82d3-e941-5020-9dbc-99b4dcb0318d": {ID: "864c82d3-e941-5020-9dbc-99b4dcb0318d", Name: "Error Rate Percentage", Kind: "kpi", Layer: "cause"},
		},
	}

	// Engine returns a chain with step summaries containing inline UUID tokens (stubStepSummaryEngine defined at package scope)

	lg := logger.NewMockLogger(nil)
	cfg := config.EngineConfig{StrictTimeWindowPayload: true}
	handler := &RCAHandler{rcaEngine: &stubStepSummaryEngine{}, logger: logging.FromCoreLogger(lg), engineCfg: cfg, strictTimeWindowPayload: true, kpiRepo: mockRepo}

	// send request
	reqBody := map[string]interface{}{"startTime": "2025-11-25T15:33:16Z", "endTime": "2025-11-25T15:48:16Z"}
	bodyBytes, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	// Quick sanity: make sure mock repo can resolve the base uuid we expect.
	k, err := mockRepo.GetKPI(context.Background(), "864c82d3-e941-5020-9dbc-99b4dcb0318d")
	assert.NoError(t, err)
	assert.NotNil(t, k, "sanity: mock repo should find base KPI for suffix test")

	// Also exercise convertRCAStep directly to check enrichment path
	step3Direct := rca.NewRCAStep(3, "864c82d3-e941-5020-9dbc-99b4dcb0318d-dep", "dependency")
	step3Direct.Summary = "Why 3: 864c82d3-e941-5020-9dbc-99b4dcb0318d-dep (dependency) at 03:40:55"
	dtoDirect := handler.convertRCAStep(step3Direct)
	t.Logf("direct conversion -> service=%s serviceUUID=%s summary=%s", dtoDirect.Service, dtoDirect.ServiceUUID, dtoDirect.Summary)

	handler.HandleComputeRCA(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.RCAResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp.Data)
	assert.NotEmpty(t, resp.Data.Chains)
	step := resp.Data.Chains[0].Steps[0]
	// Summary should have the UUID tokens replaced (suffix preserved)
	assert.Contains(t, step.Summary, "Transaction Success Rate")
	assert.Contains(t, step.Summary, "Error Rate Percentage-dep")
}

func TestHandleComputeRCA_ComponentSuffixResolution(t *testing.T) {
	// Setup mock repo with KFIs
	mockRepo := &mockKPIRepo{
		kpis: map[string]*models.KPIDefinition{
			"47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3": {ID: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", Name: "Transaction Success Rate", Kind: "kpi", Layer: "impact"},
			"864c82d3-e941-5020-9dbc-99b4dcb0318d": {ID: "864c82d3-e941-5020-9dbc-99b4dcb0318d", Name: "Error Rate Percentage", Kind: "kpi", Layer: "cause"},
		},
	}

	// Engine returns a step where the component contains a suffix (uuid-dep)

	lg := logger.NewMockLogger(nil)
	cfg := config.EngineConfig{StrictTimeWindowPayload: true}
	handler := &RCAHandler{rcaEngine: &stubCompEngine{}, logger: logging.FromCoreLogger(lg), engineCfg: cfg, strictTimeWindowPayload: true, kpiRepo: mockRepo}

	reqBody := map[string]interface{}{"startTime": "2025-11-25T15:33:16Z", "endTime": "2025-11-25T15:48:16Z"}
	bodyBytes, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)
	assert.Equal(t, http.StatusOK, w.Code)
	t.Logf("response body: %s", w.Body.String())

	var resp models.RCAResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp.Data)
	assert.NotEmpty(t, resp.Data.Chains)
	st := resp.Data.Chains[0].Steps[0]

	// Expect component resolved and suffix preserved
	assert.Equal(t, "Error Rate Percentage-dep", st.Component)
	assert.Equal(t, "864c82d3-e941-5020-9dbc-99b4dcb0318d", st.ComponentUUID)
}

func TestHandleComputeRCA_ServiceAndSummarySuffixResolution(t *testing.T) {
	// This test covers whyIndex 2 and 3 style cases where the step.service
	// may be a KPI UUID or a KPI UUID with a suffix ("-dep"). We validate
	// that service and summary are resolved to KPI names and UUIDs are
	// preserved in ServiceUUID when applicable.

	mockRepo := &mockKPIRepo{
		kpis: map[string]*models.KPIDefinition{
			"47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3": {ID: "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3", Name: "Service Request Failure Count (Impact)", Kind: "kpi", Layer: "impact"},
			"864c82d3-e941-5020-9dbc-99b4dcb0318d": {ID: "864c82d3-e941-5020-9dbc-99b4dcb0318d", Name: "Error Rate Percentage", Kind: "kpi", Layer: "cause"},
		},
	}

	// stubServiceEngine is defined at package scope; it returns the chain used
	// to exercise whyIndex 2/3 service and summary resolution.

	lg := logger.NewMockLogger(nil)
	cfg := config.EngineConfig{StrictTimeWindowPayload: true}
	handler := &RCAHandler{rcaEngine: &stubServiceEngine{}, logger: logging.FromCoreLogger(lg), engineCfg: cfg, strictTimeWindowPayload: true, kpiRepo: mockRepo}

	reqBody := map[string]interface{}{"startTime": "2025-11-26T03:40:55Z", "endTime": "2025-11-26T04:25:55Z"}
	bodyBytes, _ := json.Marshal(reqBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/unified/rca", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleComputeRCA(c)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.RCAResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp.Data)
	assert.NotEmpty(t, resp.Data.Chains)
	ch := resp.Data.Chains[0]

	// whyIndex 2 should have service resolved
	s2 := ch.Steps[1]
	assert.Equal(t, "Error Rate Percentage", s2.Service)
	assert.Equal(t, "864c82d3-e941-5020-9dbc-99b4dcb0318d", s2.ServiceUUID)
	assert.Contains(t, s2.Summary, "Error Rate Percentage")

	// whyIndex 3 should have suffix resolved and preserved
	s3 := ch.Steps[2]
	assert.Equal(t, "Error Rate Percentage-dep", s3.Service)
	assert.Equal(t, "864c82d3-e941-5020-9dbc-99b4dcb0318d", s3.ServiceUUID)
	assert.Contains(t, s3.Summary, "Error Rate Percentage-dep")
}
