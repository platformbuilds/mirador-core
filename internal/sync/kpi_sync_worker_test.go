package sync

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/mariadb"
	"github.com/platformbuilds/mirador-core/internal/weavstore"
)

func TestNewKPISyncWorker(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled:   true,
		Interval:  5 * time.Minute,
		BatchSize: 100,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)

	assert.NotNil(t, worker)
	assert.Equal(t, cfg, worker.cfg)
	assert.Equal(t, logger, worker.logger)
	assert.NotNil(t, worker.stopCh)
	assert.False(t, worker.running)
}

func TestKPISyncWorker_Start_Disabled(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled: false,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)

	// Should return immediately when disabled
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	worker.Start(ctx)

	// Verify it's not running
	assert.False(t, worker.running)
}

func TestKPISyncWorker_Stop(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled:  true,
		Interval: 1 * time.Hour, // Long interval so it doesn't tick during test
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)
	worker.running = true
	worker.stopCh = make(chan struct{})

	worker.Stop()

	assert.False(t, worker.running)
}

func TestKPISyncWorker_Stop_AlreadyStopped(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled: true,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)
	worker.running = false

	// Should not panic
	worker.Stop()
	assert.False(t, worker.running)
}

func TestKPISyncWorker_GetStatus(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled:   true,
		Interval:  5 * time.Minute,
		BatchSize: 100,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)

	status := worker.GetStatus()

	assert.True(t, status.Enabled)
	assert.False(t, status.Running)
	assert.Zero(t, status.LastSyncCount)
	assert.Empty(t, status.LastError)
	assert.Equal(t, "5m0s", status.Interval)
}

func TestKPISyncWorker_GetStatus_WithError(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled:  true,
		Interval: 5 * time.Minute,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)
	worker.lastSyncError = mariadb.ErrMariaDBNotConnected

	status := worker.GetStatus()

	assert.Contains(t, status.LastError, "not connected")
}

func TestKPISyncWorker_GetStatus_WithSyncData(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled:  true,
		Interval: 5 * time.Minute,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)
	worker.running = true
	worker.lastSyncTime = time.Now()
	worker.lastSyncCount = 42

	status := worker.GetStatus()

	assert.True(t, status.Running)
	assert.Equal(t, 42, status.LastSyncCount)
	assert.False(t, status.LastSyncTime.IsZero())
}

func TestKPISyncWorker_TriggerSync_Disabled(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled: false,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)

	err := worker.TriggerSync(context.Background())

	assert.ErrorIs(t, err, mariadb.ErrMariaDBDisabled)
}

func TestKPISyncWorker_recordSuccess(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled: true,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)
	worker.lastSyncError = mariadb.ErrMariaDBNotConnected // Set an error first

	worker.recordSuccess(25)

	assert.Equal(t, 25, worker.lastSyncCount)
	assert.Nil(t, worker.lastSyncError)
	assert.False(t, worker.lastSyncTime.IsZero())
}

func TestKPISyncWorker_recordError(t *testing.T) {
	cfg := config.MariaDBSyncConfig{
		Enabled: true,
	}
	logger := zap.NewNop()

	worker := NewKPISyncWorker(nil, nil, cfg, logger)

	worker.recordError(mariadb.ErrKPINotFound)

	assert.Equal(t, mariadb.ErrKPINotFound, worker.lastSyncError)
}

func TestStatus_Fields(t *testing.T) {
	now := time.Now()
	status := Status{
		Enabled:       true,
		Running:       true,
		LastSyncTime:  now,
		LastSyncCount: 100,
		LastError:     "test error",
		Interval:      "5m0s",
	}

	assert.True(t, status.Enabled)
	assert.True(t, status.Running)
	assert.Equal(t, now, status.LastSyncTime)
	assert.Equal(t, 100, status.LastSyncCount)
	assert.Equal(t, "test error", status.LastError)
	assert.Equal(t, "5m0s", status.Interval)
}

func TestStatus_JSONSerialization(t *testing.T) {
	status := Status{
		Enabled:       true,
		Running:       false,
		LastSyncCount: 50,
		Interval:      "10m0s",
	}

	data, err := json.Marshal(status)
	require.NoError(t, err)

	var decoded Status
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, status.Enabled, decoded.Enabled)
	assert.Equal(t, status.Running, decoded.Running)
	assert.Equal(t, status.LastSyncCount, decoded.LastSyncCount)
	assert.Equal(t, status.Interval, decoded.Interval)
}

func TestConvertMariaDBToWeaviate(t *testing.T) {
	now := time.Now()
	mariaKPI := &mariadb.KPI{
		ID:              "kpi-123",
		Name:            "Test KPI",
		Description:     "A test KPI",
		DataType:        mariadb.KPIDataTypeTimeseries,
		Definition:      "Test definition",
		Formula:         "sum(rate(requests[5m]))",
		DataSourceID:    "ds-456",
		Unit:            "requests/sec",
		RefreshInterval: 60,
		IsShared:        true,
		UserID:          "user-789",
		Namespace:       "production",
		Kind:            "counter",
		Layer:           "application",
		Classifier:      "http",
		SignalType:      "metric",
		Sentiment:       "neutral",
		ComponentType:   "api",
		Examples:        "example query",
		QueryType:       "promql",
		Datastore:       "victoria_metrics",
		ServiceFamily:   "backend",
		CreatedAt:       now.Add(-24 * time.Hour),
		UpdatedAt:       now,
	}

	result := convertMariaDBToWeaviate(mariaKPI)

	assert.Equal(t, "kpi-123", result.ID)
	assert.Equal(t, "Test KPI", result.Name)
	assert.Equal(t, "A test KPI", result.Description)
	assert.Equal(t, "timeseries", result.DataType)
	assert.Equal(t, "Test definition", result.Definition)
	assert.Equal(t, "sum(rate(requests[5m]))", result.Formula)
	assert.Equal(t, "ds-456", result.DataSourceID)
	assert.Equal(t, "requests/sec", result.Unit)
	assert.Equal(t, 60, result.RefreshInterval)
	assert.True(t, result.IsShared)
	assert.Equal(t, "user-789", result.UserID)
	assert.Equal(t, "production", result.Namespace)
	assert.Equal(t, "counter", result.Kind)
	assert.Equal(t, "application", result.Layer)
	assert.Equal(t, "http", result.Classifier)
	assert.Equal(t, "metric", result.SignalType)
	assert.Equal(t, "neutral", result.Sentiment)
	assert.Equal(t, "api", result.ComponentType)
	assert.Equal(t, "example query", result.Examples)
	assert.Equal(t, "promql", result.QueryType)
	assert.Equal(t, "victoria_metrics", result.Datastore)
	assert.Equal(t, "backend", result.ServiceFamily)
	assert.Equal(t, "mariadb", result.Source) // Verify source is set
}

func TestConvertMariaDBToWeaviate_WithJSON(t *testing.T) {
	mariaKPI := &mariadb.KPI{
		ID:         "kpi-json",
		Name:       "KPI with JSON",
		DataType:   mariadb.KPIDataTypeTimeseries,
		Query:      []byte(`{"promql": "up"}`),
		Thresholds: []byte(`[{"level": "critical", "value": 90}]`),
	}

	result := convertMariaDBToWeaviate(mariaKPI)

	assert.NotNil(t, result.Query)
	assert.Equal(t, "up", result.Query["promql"])

	require.Len(t, result.Thresholds, 1)
	assert.Equal(t, "critical", result.Thresholds[0].Level)
}

func TestConvertMariaDBToWeaviate_WithInvalidJSON(t *testing.T) {
	mariaKPI := &mariadb.KPI{
		ID:         "kpi-invalid",
		Name:       "KPI with invalid JSON",
		DataType:   mariadb.KPIDataTypeTimeseries,
		Query:      []byte(`{invalid json`),
		Thresholds: []byte(`not json`),
	}

	// Should not panic, just skip the invalid JSON
	result := convertMariaDBToWeaviate(mariaKPI)

	assert.Nil(t, result.Query)
	assert.Nil(t, result.Thresholds)
}

func TestConvertMariaDBToWeaviate_EmptyJSON(t *testing.T) {
	mariaKPI := &mariadb.KPI{
		ID:         "kpi-empty",
		Name:       "KPI with empty JSON",
		DataType:   mariadb.KPIDataTypeTimeseries,
		Query:      nil,
		Thresholds: nil,
	}

	result := convertMariaDBToWeaviate(mariaKPI)

	assert.Nil(t, result.Query)
	assert.Nil(t, result.Thresholds)
}

// MockKPIStore is a mock implementation of weavstore.KPIStore
type MockKPIStore struct {
	KPIs        map[string]*weavstore.KPIDefinition
	CreateError error
	CallCount   int
}

func NewMockKPIStore() *MockKPIStore {
	return &MockKPIStore{
		KPIs: make(map[string]*weavstore.KPIDefinition),
	}
}

func (m *MockKPIStore) CreateOrUpdateKPI(_ context.Context, kpi *weavstore.KPIDefinition) (*weavstore.KPIDefinition, string, error) {
	m.CallCount++
	if m.CreateError != nil {
		return nil, "", m.CreateError
	}
	m.KPIs[kpi.ID] = kpi
	return kpi, "created", nil
}

// Implement other KPIStore methods as needed for the interface
func (m *MockKPIStore) GetKPI(_ context.Context, id string) (*weavstore.KPIDefinition, error) {
	if kpi, ok := m.KPIs[id]; ok {
		return kpi, nil
	}
	return nil, nil
}

func (m *MockKPIStore) ListKPIs(_ context.Context, _ *weavstore.KPIListRequest) ([]*weavstore.KPIDefinition, int64, error) {
	result := make([]*weavstore.KPIDefinition, 0, len(m.KPIs))
	for _, kpi := range m.KPIs {
		result = append(result, kpi)
	}
	return result, int64(len(result)), nil
}

func (m *MockKPIStore) SearchKPIs(_ context.Context, _ *weavstore.KPISearchRequest) ([]*weavstore.KPISearchResult, int64, error) {
	return nil, 0, nil
}

func (m *MockKPIStore) DeleteKPI(_ context.Context, _ string) error {
	return nil
}

// Verify MockKPIStore implements the interface
var _ weavstore.KPIStore = (*MockKPIStore)(nil)
