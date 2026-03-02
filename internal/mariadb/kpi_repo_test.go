package mariadb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/mirastacklabs-ai/mirador-core/internal/config"
)

func TestNewKPIRepo(t *testing.T) {
	client := &Client{logger: zap.NewNop()}
	logger := zap.NewNop()

	repo := NewKPIRepo(client, logger)

	assert.NotNil(t, repo)
	assert.Equal(t, client, repo.client)
	assert.Equal(t, logger, repo.logger)
}

func TestKPIRepo_GetByID_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, err := repo.GetByID(context.Background(), "test-id")

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestKPIRepo_GetByID_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil,
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, err := repo.GetByID(context.Background(), "test-id")

	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestKPIRepo_List_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, _, err := repo.List(context.Background(), nil)

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestKPIRepo_List_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil,
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, _, err := repo.List(context.Background(), nil)

	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestKPIRepo_ListAll_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, err := repo.ListAll(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestKPIRepo_ListAll_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil,
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, err := repo.ListAll(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestKPIRepo_ListUpdatedSince_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, err := repo.ListUpdatedSince(context.Background(), time.Now())

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestKPIRepo_ListUpdatedSince_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil,
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, err := repo.ListUpdatedSince(context.Background(), time.Now())

	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestKPIRepo_Count_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, err := repo.Count(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestKPIRepo_Count_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil,
		logger: zap.NewNop(),
	}
	repo := NewKPIRepo(client, zap.NewNop())

	_, err := repo.Count(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestKPIDataType_Constants(t *testing.T) {
	// Verify KPI data type constants
	assert.Equal(t, KPIDataType("timeseries"), KPIDataTypeTimeseries)
	assert.Equal(t, KPIDataType("value"), KPIDataTypeValue)
	assert.Equal(t, KPIDataType("categorical"), KPIDataTypeCategorical)
}

func TestKPIErrors(t *testing.T) {
	// Verify error messages are meaningful
	assert.Contains(t, ErrKPINotFound.Error(), "not found")
	assert.Contains(t, ErrKPIQueryError.Error(), "query")
}

func TestKPIRepo_ImplementsInterface(t *testing.T) {
	// Compile-time check that KPIRepo implements KPIReader
	var _ KPIReader = (*KPIRepo)(nil)
}

func TestKPI_Fields(t *testing.T) {
	// Test KPI struct field access
	kpi := KPI{
		ID:           "kpi-1",
		Name:         "Test KPI",
		Description:  "A test KPI",
		DataType:     KPIDataTypeTimeseries,
		DataSourceID: "ds-1",
		Namespace:    "test-ns",
		Kind:         "gauge",
		Layer:        "application",
		SignalType:   "metric",
		IsShared:     true,
	}

	assert.Equal(t, "kpi-1", kpi.ID)
	assert.Equal(t, "Test KPI", kpi.Name)
	assert.Equal(t, KPIDataTypeTimeseries, kpi.DataType)
	assert.True(t, kpi.IsShared)
}

func TestKPIListOptions_Fields(t *testing.T) {
	// Test KPIListOptions struct
	opts := KPIListOptions{
		Limit:      10,
		Offset:     20,
		Namespace:  "production",
		Kind:       "counter",
		Layer:      "infrastructure",
		SignalType: "metric",
		UserID:     "user-123",
	}

	assert.Equal(t, 10, opts.Limit)
	assert.Equal(t, 20, opts.Offset)
	assert.Equal(t, "production", opts.Namespace)
	assert.Equal(t, "counter", opts.Kind)
}

// MockKPIReader is a mock implementation for testing consumers.
type MockKPIReader struct {
	KPIs         []*KPI
	GetByIDError error
	ListError    error
	TotalCount   int64
}

func (m *MockKPIReader) GetByID(_ context.Context, id string) (*KPI, error) {
	if m.GetByIDError != nil {
		return nil, m.GetByIDError
	}
	for _, kpi := range m.KPIs {
		if kpi.ID == id {
			return kpi, nil
		}
	}
	return nil, ErrKPINotFound
}

func (m *MockKPIReader) List(_ context.Context, _ *KPIListOptions) ([]*KPI, int64, error) {
	if m.ListError != nil {
		return nil, 0, m.ListError
	}
	return m.KPIs, m.TotalCount, nil
}

func (m *MockKPIReader) ListAll(_ context.Context) ([]*KPI, error) {
	if m.ListError != nil {
		return nil, m.ListError
	}
	return m.KPIs, nil
}

func (m *MockKPIReader) ListUpdatedSince(_ context.Context, _ time.Time) ([]*KPI, error) {
	if m.ListError != nil {
		return nil, m.ListError
	}
	return m.KPIs, nil
}

func (m *MockKPIReader) Count(_ context.Context) (int64, error) {
	if m.ListError != nil {
		return 0, m.ListError
	}
	return m.TotalCount, nil
}

// Verify MockKPIReader implements the interface
var _ KPIReader = (*MockKPIReader)(nil)

func TestMockKPIReader(t *testing.T) {
	mock := &MockKPIReader{
		KPIs: []*KPI{
			{ID: "kpi-1", Name: "Test KPI", DataType: KPIDataTypeTimeseries},
		},
		TotalCount: 1,
	}

	// Test GetByID
	kpi, err := mock.GetByID(context.Background(), "kpi-1")
	require.NoError(t, err)
	assert.Equal(t, "Test KPI", kpi.Name)

	// Test not found
	_, err = mock.GetByID(context.Background(), "not-exists")
	assert.ErrorIs(t, err, ErrKPINotFound)

	// Test List
	list, total, err := mock.List(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(1), total)

	// Test ListAll
	all, err := mock.ListAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, all, 1)

	// Test Count
	count, err := mock.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
