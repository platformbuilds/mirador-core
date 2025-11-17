package services

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorrelationEngine_DetectComponentFailures(t *testing.T) {
	// Create a correlation engine instance
	engine := &CorrelationEngineImpl{}

	// Create test time range
	timeRange := models.TimeRange{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}

	// Test with no services (should return empty result)
	result, err := engine.DetectComponentFailures(context.Background(), timeRange, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Incidents)
}

func TestCorrelationEngine_CorrelateTransactionFailures(t *testing.T) {
	// Create a correlation engine instance
	engine := &CorrelationEngineImpl{}

	// Create test time range
	timeRange := models.TimeRange{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}

	// Test with empty transaction IDs
	result, err := engine.CorrelateTransactionFailures(context.Background(), []string{}, timeRange)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Incidents)
}

func TestCorrelationEngine_FailureSignalProcessing(t *testing.T) {
	engine := &CorrelationEngineImpl{}

	// Test signal creation and processing
	signals := []models.FailureSignal{
		{
			Type:      "log",
			Engine:    models.QueryTypeLogs,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"transaction_id": "tx-123",
				"service":        "api-gateway",
				"level":          "error",
				"message":        "connection failed",
			},
		},
		{
			Type:      "metric",
			Engine:    models.QueryTypeMetrics,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"transaction_id": "tx-123",
				"service":        "keydb",
				"metric":         "up",
				"value":          0,
			},
		},
	}

	// Test transaction grouping
	grouped := engine.groupSignalsByTransaction(signals)
	assert.Len(t, grouped, 1)
	assert.Contains(t, grouped, "tx-123")
	assert.Len(t, grouped["tx-123"], 2)

	// Test component grouping
	componentGrouped := engine.groupSignalsByTransactionAndComponent(signals, []models.FailureComponent{models.FailureComponentAPIGateway, models.FailureComponentKeyDB})
	assert.Len(t, componentGrouped, 1)
	assert.Contains(t, componentGrouped, "tx-123")
	assert.Len(t, componentGrouped["tx-123"], 2) // api-gateway and keydb
}

func TestCorrelationEngine_FailureIncidentCreation(t *testing.T) {
	engine := &CorrelationEngineImpl{}

	timeRange := models.TimeRange{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}

	// Create test signals
	signals := []models.FailureSignal{
		{
			Type:      "log",
			Engine:    models.QueryTypeLogs,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"transaction_id": "tx-123",
				"service":        "api-gateway",
				"level":          "error",
				"failure_mode":   "connection_error",
			},
		},
		{
			Type:      "log",
			Engine:    models.QueryTypeLogs,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"transaction_id": "tx-123",
				"service":        "keydb",
				"level":          "error",
				"failure_mode":   "timeout",
			},
		},
	}

	// Test incident creation for transaction
	incident := engine.createFailureIncidentForTransaction(signals, timeRange)
	require.NotNil(t, incident)
	assert.Equal(t, models.FailureComponentAPIGateway, incident.PrimaryComponent) // First component alphabetically
	assert.Contains(t, incident.AffectedTransactionIDs, "tx-123")
	assert.Len(t, incident.ServicesInvolved, 2)
	assert.Contains(t, incident.ServicesInvolved, "api-gateway")
	assert.Contains(t, incident.ServicesInvolved, "keydb")
	assert.Equal(t, "connection_error", incident.FailureMode)
	assert.Greater(t, incident.Confidence, 0.0)
}

func TestCorrelationEngine_ComponentMapping(t *testing.T) {
	engine := &CorrelationEngineImpl{}

	testCases := []struct {
		service  string
		expected string
	}{
		{"api-gateway", "api-gateway"},
		{"tps", "tps"},
		{"keydb", "keydb"},
		{"keydb-client", "keydb"},
		{"kafka", "kafka"},
		{"kafka-producer", "kafka"},
		{"kafka-consumer", "kafka"},
		{"cassandra", "cassandra"},
		{"cassandra-client", "cassandra"},
		{"unknown-service", "unknown-service"},
	}

	for _, tc := range testCases {
		result := engine.mapServiceToComponent(tc.service)
		assert.Equal(t, tc.expected, result, "Service %s should map to %s", tc.service, tc.expected)
	}
}

func TestCorrelationEngine_SeverityCalculation(t *testing.T) {
	engine := &CorrelationEngineImpl{}

	testCases := []struct {
		signalCount   int
		anomalyScore  float64
		expectedLevel string
	}{
		{1, 0.3, "low"},
		{3, 0.5, "medium"},
		{6, 0.7, "high"},
		{15, 0.9, "critical"},
	}

	for _, tc := range testCases {
		severity := engine.calculateSeverity(tc.signalCount, tc.anomalyScore)
		assert.Equal(t, tc.expectedLevel, severity, "Signal count %d, anomaly score %.1f should be %s", tc.signalCount, tc.anomalyScore, tc.expectedLevel)
	}
}

func TestCorrelationEngine_ConfidenceCalculation(t *testing.T) {
	engine := &CorrelationEngineImpl{}

	// Test with empty signals
	confidence := engine.calculateFailureConfidence([]models.FailureSignal{}, models.FailureComponentAPIGateway)
	assert.Equal(t, 0.0, confidence)

	// Test with signals
	signals := []models.FailureSignal{
		{
			Type:         "log",
			Engine:       models.QueryTypeLogs,
			Timestamp:    time.Now(),
			AnomalyScore: &[]float64{0.8}[0],
			Data: map[string]interface{}{
				"service": "api-gateway",
			},
		},
		{
			Type:         "metric",
			Engine:       models.QueryTypeMetrics,
			Timestamp:    time.Now(),
			AnomalyScore: &[]float64{0.6}[0],
			Data: map[string]interface{}{
				"service": "api-gateway",
			},
		},
	}

	confidence = engine.calculateFailureConfidence(signals, models.FailureComponentAPIGateway)
	assert.Greater(t, confidence, 0.0)
	assert.LessOrEqual(t, confidence, 0.95)
}
