package mapping

import (
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBleveDocumentMapper_MapLogs(t *testing.T) {
	mapper := NewBleveDocumentMapper(logger.New("test"))

	logs := []map[string]any{
		{
			"timestamp": time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			"level":     "INFO",
			"message":   "Test log message",
			"service":   "test-service",
			"host":      "test-host",
			"user_id":   "12345",
		},
		{
			"timestamp":    "2024-01-01T13:00:00Z",
			"level":        "ERROR",
			"message":      "Another test message",
			"service":      "another-service",
			"custom_field": "custom_value",
		},
	}

	documents, err := mapper.MapLogs(logs, "tenant-123")
	require.NoError(t, err)
	assert.Len(t, documents, 2)

	// Check first document
	doc1 := documents[0]
	assert.Contains(t, doc1.ID, "log_tenant-123_")
	logDoc1 := doc1.Data.(*LogDocument)
	assert.Equal(t, "tenant-123", logDoc1.TenantID)
	assert.Equal(t, "INFO", logDoc1.Level)
	assert.Equal(t, "Test log message", logDoc1.Message)
	assert.Equal(t, "test-service", logDoc1.Service)
	assert.Equal(t, "test-host", logDoc1.Host)
	assert.Equal(t, "12345", logDoc1.Fields["user_id"])

	// Check second document
	doc2 := documents[1]
	assert.Contains(t, doc2.ID, "log_tenant-123_")
	logDoc2 := doc2.Data.(*LogDocument)
	assert.Equal(t, "tenant-123", logDoc2.TenantID)
	assert.Equal(t, "ERROR", logDoc2.Level)
	assert.Equal(t, "Another test message", logDoc2.Message)
	assert.Equal(t, "another-service", logDoc2.Service)
	assert.Equal(t, "custom_value", logDoc2.Fields["custom_field"])
}

func TestBleveDocumentMapper_MapTraces(t *testing.T) {
	mapper := NewBleveDocumentMapper(logger.New("test"))

	traces := []map[string]interface{}{
		{
			"traceID": "trace-123",
			"spans": []interface{}{
				map[string]interface{}{
					"spanID":        "span-1",
					"operationName": "http_request",
					"process": map[string]interface{}{
						"serviceName": "web-service",
					},
					"startTime": time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					"duration":  150000, // 150ms in microseconds
				},
				map[string]interface{}{
					"spanID":        "span-2",
					"operationName": "database_query",
					"process": map[string]interface{}{
						"serviceName": "db-service",
					},
					"startTime": time.Date(2024, 1, 1, 12, 0, 0, 100000000, time.UTC), // 100ms later
					"duration":  50000,                                                // 50ms
				},
			},
		},
	}

	documents, err := mapper.MapTraces(traces, "tenant-456")
	require.NoError(t, err)
	assert.Len(t, documents, 1)

	doc := documents[0]
	assert.Contains(t, doc.ID, "trace_tenant-456_trace-123")
	traceDoc := doc.Data.(*TraceDocument)
	assert.Equal(t, "tenant-456", traceDoc.TenantID)
	assert.Equal(t, "trace-123", traceDoc.TraceID)
	assert.Equal(t, "web-service", traceDoc.ServiceName) // First span's service
	assert.Equal(t, "http_request", traceDoc.Operation)  // First span's operation
	assert.Equal(t, int64(200000), traceDoc.Duration)    // 150ms + 50ms
}

func TestBleveDocumentMapper_GetIndexName(t *testing.T) {
	mapper := NewBleveDocumentMapper(logger.New("test"))

	assert.Equal(t, "mirador_logs_tenant-123", mapper.GetIndexName("logs", "tenant-123"))
	assert.Equal(t, "mirador_traces_tenant-456", mapper.GetIndexName("traces", "tenant-456"))
}

func TestBleveDocumentMapper_ExtractTimestamp(t *testing.T) {
	mapper := &BleveDocumentMapper{}

	// Test time.Time
	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	data := map[string]interface{}{"timestamp": testTime}
	assert.Equal(t, testTime, mapper.extractTimestamp(data))

	// Test string
	data = map[string]interface{}{"timestamp": "2024-01-01T12:00:00Z"}
	parsed := mapper.extractTimestamp(data)
	expected, _ := time.Parse(time.RFC3339, "2024-01-01T12:00:00Z")
	assert.Equal(t, expected, parsed)

	// Test fallback
	data = map[string]interface{}{}
	timestamp := mapper.extractTimestamp(data)
	assert.True(t, timestamp.After(time.Now().Add(-time.Second)))
}

func TestBleveDocumentMapper_ExtractStringField(t *testing.T) {
	mapper := &BleveDocumentMapper{}

	data := map[string]interface{}{"test_field": "test_value"}
	assert.Equal(t, "test_value", mapper.extractStringField(data, "test_field"))

	data = map[string]interface{}{"test_field": 123}
	assert.Equal(t, "", mapper.extractStringField(data, "test_field"))

	data = map[string]interface{}{}
	assert.Equal(t, "", mapper.extractStringField(data, "missing_field"))
}

func TestBleveDocumentMapper_ExtractInt64Field(t *testing.T) {
	mapper := &BleveDocumentMapper{}

	data := map[string]interface{}{"duration": int64(150000)}
	assert.Equal(t, int64(150000), mapper.extractInt64Field(data, "duration"))

	data = map[string]interface{}{"duration": float64(150000.5)}
	assert.Equal(t, int64(150000), mapper.extractInt64Field(data, "duration"))

	data = map[string]interface{}{"duration": "not_a_number"}
	assert.Equal(t, int64(0), mapper.extractInt64Field(data, "duration"))
}
