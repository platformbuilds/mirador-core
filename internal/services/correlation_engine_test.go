package services

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock services for testing
type MockVictoriaMetricsService struct {
	mock.Mock
}

func (m *MockVictoriaMetricsService) ExecuteQuery(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.MetricsQLQueryResult), args.Error(1)
}

type MockVictoriaLogsService struct {
	mock.Mock
}

func (m *MockVictoriaLogsService) ExecuteQuery(ctx context.Context, req *models.LogsQLQueryRequest) (*models.LogsQLQueryResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.LogsQLQueryResult), args.Error(1)
}

type MockVictoriaTracesService struct {
	mock.Mock
}

func (m *MockVictoriaTracesService) GetOperations(ctx context.Context, service, tenantID string) ([]string, error) {
	args := m.Called(ctx, service, tenantID)
	return args.Get(0).([]string), args.Error(1)
}

type MockValkeyCluster struct {
	mock.Mock
}

func (m *MockValkeyCluster) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockValkeyCluster) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockValkeyCluster) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockValkeyCluster) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, key, ttl)
	return args.Bool(0), args.Error(1)
}

func (m *MockValkeyCluster) ReleaseLock(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockValkeyCluster) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(*models.UserSession), args.Error(1)
}

func (m *MockValkeyCluster) SetSession(ctx context.Context, session *models.UserSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockValkeyCluster) InvalidateSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockValkeyCluster) GetActiveSessions(ctx context.Context, tenantID string) ([]*models.UserSession, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.UserSession), args.Error(1)
}

func (m *MockValkeyCluster) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
	args := m.Called(ctx, queryHash, result, ttl)
	return args.Error(0)
}

func (m *MockValkeyCluster) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
	args := m.Called(ctx, queryHash)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockValkeyCluster) AddToPatternIndex(ctx context.Context, patternKey string, cacheKey string) error {
	args := m.Called(ctx, patternKey, cacheKey)
	return args.Error(0)
}

func (m *MockValkeyCluster) GetPatternIndexKeys(ctx context.Context, patternKey string) ([]string, error) {
	args := m.Called(ctx, patternKey)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockValkeyCluster) DeletePatternIndex(ctx context.Context, patternKey string) error {
	args := m.Called(ctx, patternKey)
	return args.Error(0)
}

func (m *MockValkeyCluster) DeleteMultiple(ctx context.Context, keys []string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

func (m *MockValkeyCluster) GetMemoryInfo(ctx context.Context) (*cache.CacheMemoryInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(*cache.CacheMemoryInfo), args.Error(1)
}

func (m *MockValkeyCluster) AdjustCacheTTL(ctx context.Context, keyPattern string, newTTL time.Duration) error {
	args := m.Called(ctx, keyPattern, newTTL)
	return args.Error(0)
}

func (m *MockValkeyCluster) CleanupExpiredEntries(ctx context.Context, keyPattern string) (int64, error) {
	args := m.Called(ctx, keyPattern)
	return args.Get(0).(int64), args.Error(1)
}

func TestCorrelationEngineImpl_ExecuteCorrelation(t *testing.T) {
	// Setup mocks
	mockMetrics := &MockVictoriaMetricsService{}
	mockLogs := &MockVictoriaLogsService{}
	mockTraces := &MockVictoriaTracesService{}
	mockCache := &MockValkeyCluster{}
	mockLogger := logger.New("info")

	// Create correlation engine
	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, mockCache, mockLogger)

	ctx := context.Background()

	t.Run("successful time-window correlation", func(t *testing.T) {
		// Setup test data
		query := &models.CorrelationQuery{
			ID:       "test-correlation",
			RawQuery: "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
			Expressions: []models.CorrelationExpression{
				{Engine: models.QueryTypeLogs, Query: "error"},
				{Engine: models.QueryTypeMetrics, Query: "cpu_usage", Condition: " > 80"},
			},
			TimeWindow: func() *time.Duration { d := 5 * time.Minute; return &d }(),
			Operator:   models.CorrelationOpAND,
		}

		// Mock responses
		now := time.Now()
		logsResponse := &models.LogsQLQueryResult{
			Logs: []map[string]interface{}{
				{"timestamp": now.Format(time.RFC3339), "message": "error occurred", "level": "error"},
			},
		}
		metricsResponse := &models.MetricsQLQueryResult{
			Status: "success",
			Data: map[string]interface{}{
				"result": []interface{}{
					map[string]interface{}{
						"metric": map[string]string{"__name__": "cpu_usage"},
						"values": [][]interface{}{
							{float64(now.Unix()), "85.5"},
						},
					},
				},
			},
			SeriesCount: 1,
		}

		mockLogs.On("ExecuteQuery", mock.Anything, mock.Anything).Return(logsResponse, nil)

		mockMetrics.On("ExecuteQuery", mock.Anything, mock.Anything).Return(metricsResponse, nil)

		// Execute correlation
		result, err := engine.ExecuteCorrelation(ctx, query)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.Correlations)
		assert.NotNil(t, result.Summary)
		assert.Greater(t, result.Summary.TotalCorrelations, 0)

		// Verify mocks
		mockLogs.AssertExpectations(t)
		mockMetrics.AssertExpectations(t)
	})

	t.Run("successful label-based correlation", func(t *testing.T) {
		// Setup test data
		query := &models.CorrelationQuery{
			ID:       "test-label-correlation",
			RawQuery: "logs:service:checkout AND traces:service:checkout",
			Expressions: []models.CorrelationExpression{
				{Engine: models.QueryTypeLogs, Query: "service:checkout"},
				{Engine: models.QueryTypeTraces, Query: "service:checkout"},
			},
			Operator: models.CorrelationOpAND,
		}

		// Mock responses
		logsResponse := &models.LogsQLQueryResult{
			Logs: []map[string]interface{}{
				{"timestamp": time.Now().Format(time.RFC3339), "message": "checkout error", "service": "checkout"},
			},
		}
		tracesResponse := []string{"checkout", "payment"}

		mockLogs.On("ExecuteQuery", mock.Anything, mock.Anything).Return(logsResponse, nil)
		mockTraces.On("GetOperations", mock.Anything, "service:checkout", "").Return(tracesResponse, nil)

		// Execute correlation
		result, err := engine.ExecuteCorrelation(ctx, query)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Summary)

		// Verify mocks
		mockLogs.AssertExpectations(t)
		mockTraces.AssertExpectations(t)
	})

	t.Run("invalid correlation query", func(t *testing.T) {
		query := &models.CorrelationQuery{
			ID:          "invalid-query",
			Expressions: []models.CorrelationExpression{}, // Empty expressions
		}

		result, err := engine.ExecuteCorrelation(ctx, query)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid correlation query")
	})

	t.Run("engine execution failure", func(t *testing.T) {
		query := &models.CorrelationQuery{
			ID:       "test-failure",
			RawQuery: "logs:error AND metrics:cpu",
			Expressions: []models.CorrelationExpression{
				{Engine: models.QueryTypeLogs, Query: "error"},
				{Engine: models.QueryTypeMetrics, Query: "cpu"},
			},
			Operator: models.CorrelationOpAND,
		}

		mockLogs.On("ExecuteQuery", mock.Anything, mock.Anything).Return(nil, assert.AnError)
		mockMetrics.On("ExecuteQuery", mock.Anything, mock.Anything).Return(nil, assert.AnError)

		result, err := engine.ExecuteCorrelation(ctx, query)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.Summary.TotalCorrelations)
		mockLogs.AssertExpectations(t)
		mockMetrics.AssertExpectations(t)
	})
}

func TestCorrelationEngineImpl_ValidateCorrelationQuery(t *testing.T) {
	mockMetrics := &MockVictoriaMetricsService{}
	mockLogs := &MockVictoriaLogsService{}
	mockTraces := &MockVictoriaTracesService{}
	mockCache := &MockValkeyCluster{}
	mockLogger := logger.New("info")

	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, mockCache, mockLogger)

	tests := []struct {
		name        string
		query       *models.CorrelationQuery
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid query",
			query: &models.CorrelationQuery{
				Expressions: []models.CorrelationExpression{
					{Engine: models.QueryTypeLogs, Query: "error"},
					{Engine: models.QueryTypeMetrics, Query: "cpu"},
				},
			},
			expectError: false,
		},
		{
			name: "empty expressions",
			query: &models.CorrelationQuery{
				Expressions: []models.CorrelationExpression{},
			},
			expectError: true,
			errorMsg:    "correlation query must have at least one expression",
		},
		{
			name: "missing engine",
			query: &models.CorrelationQuery{
				Expressions: []models.CorrelationExpression{
					{Query: "error"}, // missing engine
				},
			},
			expectError: true,
			errorMsg:    "missing engine",
		},
		{
			name: "missing query",
			query: &models.CorrelationQuery{
				Expressions: []models.CorrelationExpression{
					{Engine: models.QueryTypeLogs}, // missing query
				},
			},
			expectError: true,
			errorMsg:    "missing query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ValidateCorrelationQuery(tt.query)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCorrelationEngineImpl_GetCorrelationExamples(t *testing.T) {
	mockMetrics := &MockVictoriaMetricsService{}
	mockLogs := &MockVictoriaLogsService{}
	mockTraces := &MockVictoriaTracesService{}
	mockCache := &MockValkeyCluster{}
	mockLogger := logger.New("info")

	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, mockCache, mockLogger)

	examples := engine.GetCorrelationExamples()

	assert.NotEmpty(t, examples)
	assert.Contains(t, examples, "logs:error AND metrics:high_latency")
	assert.Contains(t, examples, "logs:service:checkout AND traces:service:checkout")
}
