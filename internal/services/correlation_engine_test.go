package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Mock services for testing
type MockVictoriaMetricsService struct {
	mock.Mock
}

func (m *MockVictoriaMetricsService) ExecuteQuery(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.MetricsQLQueryResult), args.Error(1)
}

func (m *MockVictoriaMetricsService) ExecuteRangeQuery(ctx context.Context, req *models.MetricsQLRangeQueryRequest) (*models.MetricsQLRangeQueryResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MetricsQLRangeQueryResult), args.Error(1)
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

func (m *MockVictoriaTracesService) GetOperations(ctx context.Context, service string) ([]string, error) {
	args := m.Called(ctx, service)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockVictoriaTracesService) SearchTraces(ctx context.Context, request *models.TraceSearchRequest) (*models.TraceSearchResult, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*models.TraceSearchResult), args.Error(1)
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
	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, nil, mockCache, mockLogger, config.EngineConfig{})

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
		mockTraces.On("GetOperations", mock.Anything, "service:checkout").Return(tracesResponse, nil)

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

	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, nil, mockCache, mockLogger, config.EngineConfig{})

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

	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, nil, mockCache, mockLogger, config.EngineConfig{})

	examples := engine.GetCorrelationExamples()

	assert.NotEmpty(t, examples)
	assert.Contains(t, examples, "logs:error AND metrics:high_latency")
	assert.Contains(t, examples, "logs:service:checkout AND traces:service:checkout")
}

func TestCorrelationEngineImpl_Correlate(t *testing.T) {
	// Setup mocks
	mockMetrics := &MockVictoriaMetricsService{}
	mockLogs := &MockVictoriaLogsService{}
	mockTraces := &MockVictoriaTracesService{}
	mockCache := &MockValkeyCluster{}
	mockLogger := logger.New("info")

	// NOTE(HCB-001): Since we removed hardcoded probes, tests must explicitly provide
	// probe config or use a mock KPI repo. Here we provide test probes in EngineConfig.
	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, nil, mockCache, mockLogger, config.EngineConfig{
		MinAnomalyScore: 0.5,
		Buckets: config.BucketConfig{
			CoreWindowSize: 5 * time.Minute,
			PreRings:       1,
			PostRings:      1,
			RingStep:       2 * time.Minute,
		},
		// Provide test probes for this test (registry-driven discovery would replace this in production)
		Probes: []string{"test_probe_metric"},
	})

	ctx := context.Background()
	now := time.Now()

	// Prepare a generic metrics response with one series so probes detect KPIs
	metricsResponse := &models.MetricsQLQueryResult{
		Status:      "success",
		SeriesCount: 1,
		Data:        map[string]interface{}{"result": []interface{}{map[string]interface{}{"metric": map[string]string{"__name__": "probe_metric"}}}},
	}

	// Any probe should return the same mocked response
	mockMetrics.On("ExecuteQuery", mock.Anything, mock.Anything).Return(metricsResponse, nil)

	tr := models.TimeRange{Start: now.Add(-15 * time.Minute), End: now}

	res, err := engine.Correlate(ctx, tr)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.GreaterOrEqual(t, len(res.AffectedServices), 0)
	// Confidence should be between 0 and 1
	assert.True(t, res.Confidence >= 0.0 && res.Confidence <= 1.0)

	mockMetrics.AssertExpectations(t)
}
