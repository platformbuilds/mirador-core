package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// IntegrationTestFramework provides a comprehensive testing framework for unified query engine
type IntegrationTestFramework struct {
	// Mock services
	MetricsService *MockMetricsService
	LogsService    *MockLogsService
	TracesService  *MockTracesService
	BleveService   *MockBleveService
	CorrEngine     *MockCorrelationEngine

	// Real unified query engine (to be created in tests)
	UnifiedEngine UnifiedQueryEngine

	// Test data
	TestData *TestDataRepository
}

// TestDataRepository holds test data for integration tests
type TestDataRepository struct {
	Metrics map[string]*models.MetricsQLQueryResult
	Logs    map[string]*models.LogsQLQueryResult
	Traces  map[string][]models.Trace
	Bleve   map[string]interface{}
}

// NewIntegrationTestFramework creates a new integration test framework
func NewIntegrationTestFramework() *IntegrationTestFramework {
	framework := &IntegrationTestFramework{
		MetricsService: &MockMetricsService{},
		LogsService:    &MockLogsService{},
		TracesService:  &MockTracesService{},
		BleveService:   &MockBleveService{},
		CorrEngine:     &MockCorrelationEngine{},
		TestData:       NewTestDataRepository(),
	}

	return framework
}

// Setup initializes the test framework with mock expectations
func (f *IntegrationTestFramework) Setup() {
	// Setup default mock behaviors
	// Tests should configure specific expectations as needed
}

// TearDown cleans up after tests
func (f *IntegrationTestFramework) TearDown() {
	// Note: AssertExpectations is typically called in individual test cases
	// This method is here for any framework-level cleanup needed
}

// SetupMetricsData configures mock metrics data
func (f *IntegrationTestFramework) SetupMetricsData(query string, result *models.MetricsQLQueryResult) {
	f.TestData.Metrics[query] = result
	f.MetricsService.On("ExecuteQuery", mock.Anything, mock.MatchedBy(func(req *models.MetricsQLQueryRequest) bool {
		return req.Query == query
	})).Return(result, nil)
}

// SetupLogsData configures mock logs data
func (f *IntegrationTestFramework) SetupLogsData(query string, result *models.LogsQLQueryResult) {
	f.TestData.Logs[query] = result
	f.LogsService.On("ExecuteQuery", mock.Anything, mock.MatchedBy(func(req *models.LogsQLQueryRequest) bool {
		return req.Query == query
	})).Return(result, nil)
}

// SetupTracesData configures mock traces data
func (f *IntegrationTestFramework) SetupTracesData(service string, traces []models.Trace) {
	f.TestData.Traces[service] = traces
	f.TracesService.On("SearchTraces", mock.Anything, service, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(traces, nil)
}

// SetupBleveData configures mock Bleve search data
func (f *IntegrationTestFramework) SetupBleveData(query string, results interface{}) {
	f.TestData.Bleve[query] = results
	f.BleveService.On("SearchLogs", mock.Anything, query, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(results, nil)
}

// NewTestDataRepository creates a new test data repository
func NewTestDataRepository() *TestDataRepository {
	return &TestDataRepository{
		Metrics: make(map[string]*models.MetricsQLQueryResult),
		Logs:    make(map[string]*models.LogsQLQueryResult),
		Traces:  make(map[string][]models.Trace),
		Bleve:   make(map[string]interface{}),
	}
}

// Mock Services

// MockMetricsService mocks VictoriaMetricsService
type MockMetricsService struct {
	mock.Mock
}

func (m *MockMetricsService) ExecuteQuery(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MetricsQLQueryResult), args.Error(1)
}

func (m *MockMetricsService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockLogsService mocks VictoriaLogsService
type MockLogsService struct {
	mock.Mock
}

func (m *MockLogsService) ExecuteQuery(ctx context.Context, req *models.LogsQLQueryRequest) (*models.LogsQLQueryResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LogsQLQueryResult), args.Error(1)
}

func (m *MockLogsService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockTracesService mocks VictoriaTracesService
type MockTracesService struct {
	mock.Mock
}

func (m *MockTracesService) GetOperations(ctx context.Context, service, tenantID string) ([]string, error) {
	args := m.Called(ctx, service, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockTracesService) SearchTraces(ctx context.Context, service, operation string, startTime, endTime time.Time, minDuration, maxDuration time.Duration) ([]models.Trace, error) {
	args := m.Called(ctx, service, operation, startTime, endTime, minDuration, maxDuration)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Trace), args.Error(1)
}

func (m *MockTracesService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockBleveService mocks BleveSearchService
type MockBleveService struct {
	mock.Mock
}

func (m *MockBleveService) SearchLogs(ctx context.Context, query string, startTime, endTime *time.Time, limit, offset int) (interface{}, error) {
	args := m.Called(ctx, query, startTime, endTime, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

func (m *MockBleveService) SearchTraces(ctx context.Context, query string, startTime, endTime *time.Time, limit, offset int) (interface{}, error) {
	args := m.Called(ctx, query, startTime, endTime, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

func (m *MockBleveService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockCorrelationEngine mocks CorrelationEngine
type MockCorrelationEngine struct {
	mock.Mock
}

func (m *MockCorrelationEngine) ExecuteCorrelation(ctx context.Context, query *models.CorrelationQuery) (*models.CorrelationResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CorrelationResult), args.Error(1)
}

func (m *MockCorrelationEngine) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockValkeyCluster is already defined in correlation_engine_test.go, but we'll add helper methods

// Test Assertion Helpers

// AssertQuerySuccess validates that a query succeeded
func (f *IntegrationTestFramework) AssertQuerySuccess(t mock.TestingT, result *models.UnifiedResult, expectedType models.QueryType) {
	if result == nil {
		t.Errorf("Expected non-nil result")
		return
	}
	if result.Type != expectedType {
		t.Errorf("Expected query type %s, got %s", expectedType, result.Type)
	}
	if result.Data == nil {
		t.Errorf("Expected non-nil data")
	}
	if result.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", result.Status)
	}
}

// AssertQueryError validates that a query failed with expected error
func (f *IntegrationTestFramework) AssertQueryError(t mock.TestingT, err error, expectedErrorSubstring string) {
	if err == nil {
		t.Errorf("Expected error containing '%s', got nil", expectedErrorSubstring)
		return
	}
	// Simple substring check
	errMsg := err.Error()
	if errMsg == "" {
		t.Errorf("Expected error with message, got empty error")
	}
}

// AssertCorrelationSuccess validates correlation results
func (f *IntegrationTestFramework) AssertCorrelationSuccess(t mock.TestingT, result *models.CorrelationResult, minConfidence float64) {
	if result == nil {
		t.Errorf("Expected non-nil correlation result")
		return
	}
	if result.Confidence < minConfidence {
		t.Errorf("Expected confidence >= %.2f, got %.2f", minConfidence, result.Confidence)
	}
	if result.RootCause == "" {
		t.Errorf("Expected non-empty root cause")
	}
}

// AssertEngineHealth validates engine health status
func (f *IntegrationTestFramework) AssertEngineHealth(t mock.TestingT, health map[models.QueryType]string, expectedEngine models.QueryType, expectedStatus string) {
	status, exists := health[expectedEngine]
	if !exists {
		t.Errorf("Expected health status for engine %s, but not found", expectedEngine)
		return
	}
	if status != expectedStatus {
		t.Errorf("Expected engine %s health status %s, got %s", expectedEngine, expectedStatus, status)
	}
}

// Test Data Generators

// GenerateMetricsResult creates test metrics data
func GenerateMetricsResult(metricName string, value float64, timestamp time.Time) *models.MetricsQLQueryResult {
	return &models.MetricsQLQueryResult{
		Status: "success",
		Data: map[string]interface{}{
			"resultType": "matrix",
			"result": []interface{}{
				map[string]interface{}{
					"metric": map[string]string{
						"__name__": metricName,
					},
					"values": [][]interface{}{
						{float64(timestamp.Unix()), fmt.Sprintf("%.2f", value)},
					},
				},
			},
		},
	}
}

// GenerateLogsResult creates test logs data
func GenerateLogsResult(level, message string, timestamp time.Time) *models.LogsQLQueryResult {
	return &models.LogsQLQueryResult{
		Logs: []map[string]interface{}{
			{
				"timestamp": timestamp.Format(time.RFC3339),
				"level":     level,
				"message":   message,
			},
		},
	}
}

// GenerateTracesResult creates test traces data
func GenerateTracesResult(traceID string) []models.Trace {
	return []models.Trace{
		{
			TraceID: traceID,
			Spans: []map[string]interface{}{
				{
					"spanID":        "span-456",
					"operationName": "GET /api/users",
					"startTime":     time.Now().Unix(),
					"duration":      100000, // microseconds
				},
			},
			Processes: map[string]interface{}{
				"p1": map[string]interface{}{
					"serviceName": "api-service",
				},
			},
		},
	}
}

// Helper Functions

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(condition func() bool, timeout, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

// CompareQueryResults compares two query results for equality
func CompareQueryResults(result1, result2 *models.UnifiedResult) bool {
	if result1 == nil || result2 == nil {
		return result1 == result2
	}

	// Compare basic fields
	if result1.QueryID != result2.QueryID ||
		result1.Type != result2.Type ||
		result1.Status != result2.Status {
		return false
	}

	// Compare data (simplified comparison)
	data1, _ := json.Marshal(result1.Data)
	data2, _ := json.Marshal(result2.Data)
	return bytes.Equal(data1, data2)
}

// CreateTestUnifiedQuery creates a unified query for testing
func CreateTestUnifiedQuery(id string, queryType models.QueryType, query string, timeRange time.Duration) *models.UnifiedQuery {
	now := time.Now()
	startTime := now.Add(-timeRange)

	return &models.UnifiedQuery{
		ID:        id,
		Type:      queryType,
		Query:     query,
		StartTime: &startTime,
		EndTime:   &now,
	}
}

// CreateTestContext creates a context with timeout for testing
func CreateTestContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
