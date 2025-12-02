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

// MockKPIRepo that returns defined KPIs on GetKPI
type MockKPIRepoWithDefs struct {
}

func (m *MockKPIRepoWithDefs) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return nil, "", nil
}
func (m *MockKPIRepoWithDefs) CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	return nil, nil
}
func (m *MockKPIRepoWithDefs) ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	return nil, "", nil
}
func (m *MockKPIRepoWithDefs) ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	return nil, nil
}
func (m *MockKPIRepoWithDefs) DeleteKPI(ctx context.Context, id string) (repo.DeleteResult, error) {
	return repo.DeleteResult{}, nil
}
func (m *MockKPIRepoWithDefs) DeleteKPIBulk(ctx context.Context, ids []string) []error { return nil }

func (m *MockKPIRepoWithDefs) ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
	now := time.Now()
	metricsKPI := &models.KPIDefinition{ID: "kpi_metrics_1", Name: "probe_metric", Formula: "probe_metric", SignalType: "metrics", Layer: "impact", CreatedAt: now, UpdatedAt: now}
	logsKPI := &models.KPIDefinition{ID: "kpi_logs_1", Name: "probe_logs", Formula: "service:checkout", SignalType: "logs", Layer: "cause", CreatedAt: now, UpdatedAt: now}
	return []*models.KPIDefinition{metricsKPI, logsKPI}, 2, nil
}

func (m *MockKPIRepoWithDefs) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	if id == "kpi_metrics_1" {
		now := time.Now()
		return &models.KPIDefinition{ID: "kpi_metrics_1", Name: "probe_metric", Formula: "probe_metric", SignalType: "metrics", Layer: "impact", CreatedAt: now, UpdatedAt: now}, nil
	}
	if id == "kpi_logs_1" {
		now := time.Now()
		return &models.KPIDefinition{ID: "kpi_logs_1", Name: "probe_logs", Formula: "service:checkout", SignalType: "logs", Layer: "cause", CreatedAt: now, UpdatedAt: now}, nil
	}
	return nil, nil
}

func (m *MockKPIRepoWithDefs) EnsureTelemetryStandards(ctx context.Context, cfg *config.EngineConfig) error {
	return nil
}

// Test that Correlate resolves KPI names and preserves original IDs/formula
func TestCorrelationEngine_Correlate_ResolvesKPINames(t *testing.T) {
	mockMetrics := &MockVictoriaMetricsService{}
	mockLogs := &MockVictoriaLogsService{}
	mockTraces := &MockVictoriaTracesService{}
	mockCache := &MockValkeyCluster{}
	mockLogger := logger.New("info")
	mockKPIRepo := &MockKPIRepoWithDefs{}

	// Create engine with mock KPI repo
	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, mockKPIRepo, mockCache, mockLogger, config.EngineConfig{DefaultQueryLimit: 100})

	// Make probes return a single series/log so KPIs are discovered
	metricsRes := &models.MetricsQLQueryResult{Status: "success", SeriesCount: 1, Data: map[string]interface{}{"result": []interface{}{map[string]interface{}{"metric": map[string]string{"service": "checkout"}}}}}
	mockMetrics.On("ExecuteQuery", mock.Anything, mock.Anything).Return(metricsRes, nil)
	mockMetrics.On("ExecuteRangeQuery", mock.Anything, mock.Anything).Return(&models.MetricsQLRangeQueryResult{Status: "success", Data: metricsRes.Data, DataPointCount: 1}, nil)

	logsRes := &models.LogsQLQueryResult{Logs: []map[string]interface{}{{"timestamp": time.Now().Format(time.RFC3339), "message": "oops", "service": "checkout"}}}
	mockLogs.On("ExecuteQuery", mock.Anything, mock.Anything).Return(logsRes, nil)

	ctx := context.Background()
	tr := models.TimeRange{Start: time.Now().Add(-10 * time.Minute), End: time.Now()}

	res, err := engine.Correlate(ctx, tr)
	require.NoError(t, err)
	require.NotNil(t, res)

	// We expect Causes to contain entries where KPI is human name and KPIUUID is original id
	found := false
	for _, c := range res.Causes {
		if c.KPIUUID == "kpi_logs_1" {
			require.Equal(t, "probe_logs", c.KPI)
			require.Equal(t, "service:checkout", c.KPIFormula)
			found = true
			break
		}
	}
	require.True(t, found, "expected to find a resolved candidate KPI with KPIUUID=kpi_logs_1 and KPI=probe_logs")

	// AffectedServices should contain human-readable impact name
	foundSvc := false
	for _, s := range res.AffectedServices {
		if s == "probe_metric" {
			foundSvc = true
			break
		}
	}
	require.True(t, foundSvc, "affected services should contain the resolved impact KPI name probe_metric")

	mockMetrics.AssertExpectations(t)
	mockLogs.AssertExpectations(t)
}
