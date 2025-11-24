package services

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type mockMetricsService struct {
	queryResult *models.MetricsQLQueryResult
	queryErr    error
}

func (m *mockMetricsService) ExecuteQuery(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return m.queryResult, nil
}

type mockLogsService struct {
	queryResult *models.LogsQLQueryResult
	queryErr    error
}

func (m *mockLogsService) ExecuteQuery(ctx context.Context, req *models.LogsQLQueryRequest) (*models.LogsQLQueryResult, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return m.queryResult, nil
}

type mockTracesService struct {
	operations    []string
	searchResult  *models.TraceSearchResult
	operationsErr error
	searchErr     error
}

func (m *mockTracesService) GetOperations(ctx context.Context, service string) ([]string, error) {
	if m.operationsErr != nil {
		return nil, m.operationsErr
	}
	return m.operations, nil
}

func (m *mockTracesService) SearchTraces(ctx context.Context, request *models.TraceSearchRequest) (*models.TraceSearchResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResult, nil
}

func createTestFailureSignal(txID string, service string, signalType string, anomalyScore *float64) models.FailureSignal {
	signal := models.FailureSignal{
		Type:      signalType,
		Engine:    models.QueryTypeLogs,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"transaction_id": txID,
			"service.name":   service,
			"severity":       "ERROR",
		},
		AnomalyScore: anomalyScore,
	}

	if service == "kafka-producer" || service == "kafka-consumer" {
		signal.Data["failure_mode"] = "kafka"
	} else if service == "cassandra-client" {
		signal.Data["failure_mode"] = "cassandra"
	} else if service == "keydb-client" {
		signal.Data["failure_mode"] = "keydb"
	} else if service == "api-gateway" {
		signal.Data["failure_mode"] = "api-gateway"
	} else if service == "tps" {
		signal.Data["failure_mode"] = "tps"
	}

	return signal
}

func TestDetectComponentFailures_KafkaFailure(t *testing.T) {
	mockMetrics := &mockMetricsService{
		queryResult: &models.MetricsQLQueryResult{
			Status:      "success",
			SeriesCount: 0,
		},
	}

	now := time.Now()
	logEntries := []map[string]interface{}{
		{
			"transaction_id": "tx-001",
			"service.name":   "tps",
			"severity":       "ERROR",
			"failure_reason": "kafka_produce_failure",
			"timestamp":      now.Unix(),
		},
		{
			"transaction_id": "tx-002",
			"service.name":   "kafka-producer",
			"severity":       "ERROR",
			"failure_reason": "kafka_produce_failed",
			"timestamp":      now.Unix(),
		},
	}

	mockLogs := &mockLogsService{
		queryResult: &models.LogsQLQueryResult{
			Logs: logEntries,
		},
	}

	mockTraces := &mockTracesService{
		searchResult: &models.TraceSearchResult{
			Traces: []map[string]interface{}{},
		},
	}

	log := logger.New("info")
	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, nil, nil, log, config.EngineConfig{})

	ctx := context.Background()
	timeRange := models.TimeRange{
		Start: now.Add(-5 * time.Minute),
		End:   now.Add(5 * time.Minute),
	}

	result, err := engine.DetectComponentFailures(ctx, timeRange, []models.FailureComponent{models.FailureComponentKafka})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	t.Logf("Found %d incidents", len(result.Incidents))
}

func TestFailureSignalGrouping(t *testing.T) {
	log := logger.New("info")

	mockMetrics := &mockMetricsService{
		queryResult: &models.MetricsQLQueryResult{Status: "success"},
	}

	mockLogs := &mockLogsService{
		queryResult: &models.LogsQLQueryResult{Logs: []map[string]interface{}{}},
	}

	mockTraces := &mockTracesService{
		searchResult: &models.TraceSearchResult{Traces: []map[string]interface{}{}},
	}

	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, nil, nil, log, config.EngineConfig{}).(*CorrelationEngineImpl)

	signals := []models.FailureSignal{
		createTestFailureSignal("tx-001", "kafka-producer", "log", nil),
		createTestFailureSignal("tx-001", "tps", "log", nil),
		createTestFailureSignal("tx-002", "cassandra-client", "log", nil),
	}

	groups := engine.groupSignalsByTransactionAndComponent(signals, []models.FailureComponent{})
	if len(groups) != 2 {
		t.Fatalf("Expected 2 transaction groups, got %d", len(groups))
	}

	t.Log("Signal grouping test passed")
}

func TestServiceToComponentMapping(t *testing.T) {
	log := logger.New("info")

	mockMetrics := &mockMetricsService{
		queryResult: &models.MetricsQLQueryResult{Status: "success"},
	}

	mockLogs := &mockLogsService{
		queryResult: &models.LogsQLQueryResult{Logs: []map[string]interface{}{}},
	}

	mockTraces := &mockTracesService{
		searchResult: &models.TraceSearchResult{Traces: []map[string]interface{}{}},
	}

	engine := NewCorrelationEngine(mockMetrics, mockLogs, mockTraces, nil, nil, log, config.EngineConfig{}).(*CorrelationEngineImpl)

	testCases := []struct {
		service  string
		expected string
	}{
		{"api-gateway", "api-gateway"},
		{"kafka-producer", "kafka"},
		{"cassandra-client", "cassandra"},
	}

	for _, tc := range testCases {
		result := engine.mapServiceToComponent(tc.service)
		if result != tc.expected {
			t.Errorf("For service '%s': expected '%s', got '%s'", tc.service, tc.expected, result)
		}
	}

	t.Log("Service to component mapping test passed")
}
