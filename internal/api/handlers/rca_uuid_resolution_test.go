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

func (m *mockKPIRepo) DeleteKPI(ctx context.Context, id string) error {
	return nil
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
