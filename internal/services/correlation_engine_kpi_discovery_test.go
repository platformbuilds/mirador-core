package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// minimal mock KPIRepo that implements ListKPIs and stubs other methods
type MockKPIRepoForTest struct{}

func (m *MockKPIRepoForTest) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return nil, "", nil
}
func (m *MockKPIRepoForTest) CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	return nil, nil
}
func (m *MockKPIRepoForTest) ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return nil, "", nil
}
func (m *MockKPIRepoForTest) ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	return nil, nil
}
func (m *MockKPIRepoForTest) DeleteKPI(ctx context.Context, id string) (repo.DeleteResult, error) {
	return repo.DeleteResult{}, nil
}
func (m *MockKPIRepoForTest) DeleteKPIBulk(ctx context.Context, ids []string) []error { return nil }
func (m *MockKPIRepoForTest) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	now := time.Now()
	switch id {
	case "kpi_metrics_1":
		return &models.KPIDefinition{ID: "kpi_metrics_1", Name: "probe_metric", SignalType: "metrics", Datastore: "victoria-metrics", Formula: "probe_metric", Layer: "impact", DimensionsHint: []string{"service", "instance"}, CreatedAt: now, UpdatedAt: now}, nil
	case "kpi_logs_1":
		return &models.KPIDefinition{ID: "kpi_logs_1", Name: "probe_logs", SignalType: "logs", Datastore: "victoria-logs", Formula: "service:checkout", Layer: "cause", DimensionsHint: []string{"service"}, CreatedAt: now, UpdatedAt: now}, nil
	default:
		return nil, nil
	}
}
func (m *MockKPIRepoForTest) ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
	// return two KPIs: one metrics, one logs
	now := time.Now()
	metricsKPI := &models.KPIDefinition{
		ID:             "kpi_metrics_1",
		Name:           "probe_metric",
		SignalType:     "metrics",
		Datastore:      "victoria-metrics",
		Formula:        "probe_metric",
		Layer:          "impact",
		DimensionsHint: []string{"service", "instance"},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	logsKPI := &models.KPIDefinition{
		ID:             "kpi_logs_1",
		Name:           "probe_logs",
		SignalType:     "logs",
		Datastore:      "victoria-logs",
		Formula:        "service:checkout",
		Layer:          "cause",
		DimensionsHint: []string{"service"},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return []*models.KPIDefinition{metricsKPI, logsKPI}, 2, nil
}

func (m *MockKPIRepoForTest) EnsureTelemetryStandards(ctx context.Context, cfg *config.EngineConfig) error {
	return nil
}

// SearchKPIs stub to satisfy interface for tests
func (m *MockKPIRepoForTest) SearchKPIs(ctx context.Context, req models.KPISearchRequest) ([]models.KPISearchResult, int64, error) {
	return nil, 0, nil
}

// Tests that Correlate probes KPI registry and fetches labels from backends
func TestCorrelationEngine_KPIDiscoveryAndLabelExtraction(t *testing.T) {
	mockMetrics := &MockVictoriaMetricsService{}
	mockLogs := &MockVictoriaLogsService{}
	mockTraces := &MockVictoriaTracesService{}
	mockCache := &MockValkeyCluster{}
	mockLogger := logger.New("info")
	mockKPIRepo := &MockKPIRepoForTest{}

	// Create engine with mock KPI repo
	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, mockKPIRepo, mockCache, mockLogger, config.EngineConfig{
		DefaultQueryLimit: 100,
	})

	// Setup mocks: metrics query returns one series with labels
	metricsRes := &models.MetricsQLQueryResult{
		Status:      "success",
		SeriesCount: 1,
		Data:        map[string]interface{}{"result": []interface{}{map[string]interface{}{"metric": map[string]string{"service": "checkout", "instance": "i-123"}}}},
	}
	mockMetrics.On("ExecuteQuery", mock.Anything, mock.Anything).Return(metricsRes, nil)
	mockMetrics.On("ExecuteRangeQuery", mock.Anything, mock.Anything).Return(&models.MetricsQLRangeQueryResult{Status: "success", Data: metricsRes.Data, DataPointCount: 1}, nil)

	// Logs query returns one log with service label
	logsRes := &models.LogsQLQueryResult{
		Logs: []map[string]interface{}{{"timestamp": time.Now().Format(time.RFC3339), "message": "oops", "service": "checkout"}},
	}
	mockLogs.On("ExecuteQuery", mock.Anything, mock.Anything).Return(logsRes, nil)

	ctx := context.Background()
	tr := models.TimeRange{Start: time.Now().Add(-10 * time.Minute), End: time.Now()}

	res, err := engine.Correlate(ctx, tr)
	require.NoError(t, err)
	require.NotNil(t, res)

	// impact KPI (layer=impact) should be present in affected services as human-readable name
	found := false
	for _, s := range res.AffectedServices {
		if s == "probe_metric" {
			found = true
			break
		}
	}
	require.True(t, found, "impact KPI name should be in AffectedServices")

	mockMetrics.AssertExpectations(t)
	mockLogs.AssertExpectations(t)
}
