package mariadb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/platformbuilds/mirador-core/internal/config"
)

func TestNewDataSourceRepo(t *testing.T) {
	client := &Client{logger: zap.NewNop()}
	logger := zap.NewNop()

	repo := NewDataSourceRepo(client, logger)

	assert.NotNil(t, repo)
	assert.Equal(t, client, repo.client)
	assert.Equal(t, logger, repo.logger)
}

func TestDataSourceRepo_GetByID_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.GetByID(context.Background(), "test-id")

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestDataSourceRepo_GetByID_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil,
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.GetByID(context.Background(), "test-id")

	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestDataSourceRepo_ListByType_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.ListByType(context.Background(), DataSourceTypePrometheus)

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestDataSourceRepo_ListByType_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil,
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.ListByType(context.Background(), DataSourceTypePrometheus)

	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestDataSourceRepo_ListAll_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.ListAll(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestDataSourceRepo_ListAll_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil,
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.ListAll(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestDataSourceRepo_GetMetricsEndpoints_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.GetMetricsEndpoints(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestDataSourceRepo_GetLogsEndpoints_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.GetLogsEndpoints(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestDataSourceRepo_GetTracesEndpoints_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.GetTracesEndpoints(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestDataSourceRepo_GetMetricsSourcesWithCreds_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	repo := NewDataSourceRepo(client, zap.NewNop())

	_, err := repo.GetMetricsSourcesWithCreds(context.Background())

	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestDataSourceType_Constants(t *testing.T) {
	// Verify data source type constants
	assert.Equal(t, DataSourceType("prometheus"), DataSourceTypePrometheus)
	assert.Equal(t, DataSourceType("victorialogs"), DataSourceTypeVictoriaLogs)
	assert.Equal(t, DataSourceType("jaeger"), DataSourceTypeJaeger)
	assert.Equal(t, DataSourceType("miradorcore"), DataSourceTypeMiradorCore)
	assert.Equal(t, DataSourceType("miradorsecurity"), DataSourceTypeMiradorSecurity)
	assert.Equal(t, DataSourceType("aiengine"), DataSourceTypeAIEngine)
	assert.Equal(t, DataSourceType("victoriatraces"), DataSourceTypeVictoriaTraces)
}

func TestDataSourceErrors(t *testing.T) {
	// Verify error messages are meaningful
	assert.Contains(t, ErrDataSourceNotFound.Error(), "not found")
	assert.Contains(t, ErrNoActiveDataSources.Error(), "no active")
	assert.Contains(t, ErrDataSourceQueryError.Error(), "query")
}

func TestDataSourceRepo_ImplementsInterface(t *testing.T) {
	// Compile-time check that DataSourceRepo implements DataSourceReader
	var _ DataSourceReader = (*DataSourceRepo)(nil)
}

func TestDataSource_Fields(t *testing.T) {
	// Test DataSource struct field access
	ds := DataSource{
		ID:                "test-id",
		Name:              "Test Source",
		Type:              DataSourceTypePrometheus,
		URL:               "http://localhost:9090",
		ProjectIdentifier: "project-1",
		IsActive:          true,
	}

	assert.Equal(t, "test-id", ds.ID)
	assert.Equal(t, "Test Source", ds.Name)
	assert.Equal(t, DataSourceTypePrometheus, ds.Type)
	assert.Equal(t, "http://localhost:9090", ds.URL)
	assert.True(t, ds.IsActive)
}

func TestDataSourceWithCredentials_Fields(t *testing.T) {
	// Test DataSourceWithCredentials struct
	creds := DataSourceWithCredentials{
		URL:      "http://localhost:9090",
		Username: "admin",
		Password: "secret",
		APIKey:   "api-key-123",
	}

	assert.Equal(t, "http://localhost:9090", creds.URL)
	assert.Equal(t, "admin", creds.Username)
	assert.Equal(t, "secret", creds.Password)
	assert.Equal(t, "api-key-123", creds.APIKey)
}

// MockDataSourceReader is a mock implementation for testing consumers.
type MockDataSourceReader struct {
	DataSources      []*DataSource
	GetByIDError     error
	ListError        error
	MetricsEndpoints []string
	EndpointsError   error
}

func (m *MockDataSourceReader) GetByID(_ context.Context, id string) (*DataSource, error) {
	if m.GetByIDError != nil {
		return nil, m.GetByIDError
	}
	for _, ds := range m.DataSources {
		if ds.ID == id {
			return ds, nil
		}
	}
	return nil, ErrDataSourceNotFound
}

func (m *MockDataSourceReader) ListByType(_ context.Context, dsType DataSourceType) ([]*DataSource, error) {
	if m.ListError != nil {
		return nil, m.ListError
	}
	var result []*DataSource
	for _, ds := range m.DataSources {
		if ds.Type == dsType {
			result = append(result, ds)
		}
	}
	return result, nil
}

func (m *MockDataSourceReader) ListAll(_ context.Context) ([]*DataSource, error) {
	if m.ListError != nil {
		return nil, m.ListError
	}
	return m.DataSources, nil
}

func (m *MockDataSourceReader) GetMetricsEndpoints(_ context.Context) ([]string, error) {
	if m.EndpointsError != nil {
		return nil, m.EndpointsError
	}
	return m.MetricsEndpoints, nil
}

func (m *MockDataSourceReader) GetLogsEndpoints(_ context.Context) ([]string, error) {
	if m.EndpointsError != nil {
		return nil, m.EndpointsError
	}
	return nil, nil
}

func (m *MockDataSourceReader) GetTracesEndpoints(_ context.Context) ([]string, error) {
	if m.EndpointsError != nil {
		return nil, m.EndpointsError
	}
	return nil, nil
}

func (m *MockDataSourceReader) GetMetricsSourcesWithCreds(_ context.Context) ([]DataSourceWithCredentials, error) {
	if m.EndpointsError != nil {
		return nil, m.EndpointsError
	}
	return nil, nil
}

// Verify MockDataSourceReader implements the interface
var _ DataSourceReader = (*MockDataSourceReader)(nil)

func TestMockDataSourceReader(t *testing.T) {
	mock := &MockDataSourceReader{
		DataSources: []*DataSource{
			{ID: "ds-1", Name: "Test", Type: DataSourceTypePrometheus, URL: "http://vm:8428"},
		},
		MetricsEndpoints: []string{"http://vm:8428"},
	}

	// Test GetByID
	ds, err := mock.GetByID(context.Background(), "ds-1")
	require.NoError(t, err)
	assert.Equal(t, "Test", ds.Name)

	// Test not found
	_, err = mock.GetByID(context.Background(), "not-exists")
	assert.ErrorIs(t, err, ErrDataSourceNotFound)

	// Test ListByType
	list, err := mock.ListByType(context.Background(), DataSourceTypePrometheus)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Test GetMetricsEndpoints
	endpoints, err := mock.GetMetricsEndpoints(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"http://vm:8428"}, endpoints)
}
