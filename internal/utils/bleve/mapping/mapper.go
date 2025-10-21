package mapping

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// DocumentMapper defines the interface for mapping Mirador data models to indexable documents
type DocumentMapper interface {
	// MapLogs converts log entries to indexable documents
	MapLogs(logs []map[string]any, tenantID string) ([]IndexableDocument, error)

	// MapTraces converts trace data to indexable documents
	MapTraces(traces []map[string]interface{}, tenantID string) ([]IndexableDocument, error)

	// GetIndexName returns the appropriate index name for the data type
	GetIndexName(dataType string, tenantID string) string
}

// IndexableDocument represents a document that can be indexed by Bleve
type IndexableDocument struct {
	ID   string
	Data interface{}
}

// BleveDocumentMapper implements DocumentMapper for Bleve indexing
type BleveDocumentMapper struct {
	logger logger.Logger
}

// Thread-safe object pools for memory optimization
var (
	// Pool for map objects used during field extraction
	fieldMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 16) // Pre-allocate capacity
		},
	}

	// Pool for string builders used for ID generation
	stringBuilderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}

	// Pool for document slices
	documentSlicePool = sync.Pool{
		New: func() interface{} {
			return make([]IndexableDocument, 0, 100) // Pre-allocate capacity
		},
	}

	// Pool for interface{} slices used during processing
	interfaceSlicePool = sync.Pool{
		New: func() interface{} {
			return make([]interface{}, 0, 50) // Pre-allocate capacity
		},
	}
)

// NewBleveDocumentMapper creates a new document mapper
func NewBleveDocumentMapper(logger logger.Logger) DocumentMapper {
	return &BleveDocumentMapper{
		logger: logger,
	}
}

// LogDocument represents a log entry in indexable format
type LogDocument struct {
	ID        string                 `json:"id"`
	TenantID  string                 `json:"tenant_id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Service   string                 `json:"service,omitempty"`
	Host      string                 `json:"host,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Raw       map[string]interface{} `json:"raw"`
}

// TraceDocument represents a trace in indexable format
type TraceDocument struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	TraceID     string                 `json:"trace_id"`
	ServiceName string                 `json:"service_name,omitempty"`
	Operation   string                 `json:"operation,omitempty"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time,omitempty"`
	Duration    int64                  `json:"duration_ms,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Tags        map[string]interface{} `json:"tags,omitempty"`
	Raw         map[string]interface{} `json:"raw"`
}

// SpanDocument represents a span within a trace
type SpanDocument struct {
	ID        string                 `json:"id"`
	TraceID   string                 `json:"trace_id"`
	SpanID    string                 `json:"span_id"`
	Service   string                 `json:"service"`
	Operation string                 `json:"operation"`
	StartTime time.Time              `json:"start_time"`
	Duration  int64                  `json:"duration_ms"`
	Tags      map[string]interface{} `json:"tags,omitempty"`
}

// MapLogs converts log entries to indexable documents
func (m *BleveDocumentMapper) MapLogs(logs []map[string]any, tenantID string) ([]IndexableDocument, error) {
	// Get pooled slice for documents
	documents := documentSlicePool.Get().([]IndexableDocument)
	defer func() {
		// Reset and return to pool
		documents = documents[:0]
		documentSlicePool.Put(documents)
	}()

	for i, logEntry := range logs {
		doc, err := m.mapLogEntry(logEntry, tenantID, i)
		if err != nil {
			m.logger.Warn("Failed to map log entry", "error", err, "index", i)
			continue
		}
		documents = append(documents, doc)
	}

	// Return a copy to avoid pool contamination
	result := make([]IndexableDocument, len(documents))
	copy(result, documents)
	return result, nil
}

// mapLogEntry converts a single log entry to an indexable document
func (m *BleveDocumentMapper) mapLogEntry(logEntry map[string]any, tenantID string, index int) (IndexableDocument, error) {
	// Extract common log fields
	timestamp := m.extractTimestamp(logEntry)
	level := m.extractStringField(logEntry, "level")
	message := m.extractStringField(logEntry, "message")
	service := m.extractStringField(logEntry, "service")
	host := m.extractStringField(logEntry, "host")

	// Generate unique ID using pooled string builder
	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()
	sb.WriteString("log_")
	sb.WriteString(tenantID)
	sb.WriteByte('_')
	sb.WriteString(fmt.Sprintf("%d_%d", timestamp.UnixNano(), index))
	id := sb.String()

	// Get pooled map for fields
	fields := fieldMapPool.Get().(map[string]interface{})
	defer func() {
		// Clear and return to pool
		for k := range fields {
			delete(fields, k)
		}
		fieldMapPool.Put(fields)
	}()

	// Extract additional fields
	for key, value := range logEntry {
		switch key {
		case "timestamp", "level", "message", "service", "host":
			// Already handled
			continue
		default:
			fields[key] = value
		}
	}

	// Create a copy of the fields map to avoid pool contamination
	fieldsCopy := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		fieldsCopy[k] = v
	}

	// Create new document (don't use pool to avoid data sharing)
	doc := &LogDocument{
		ID:        id,
		TenantID:  tenantID,
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Service:   service,
		Host:      host,
		Fields:    fieldsCopy, // Use copied map
		Raw:       logEntry,
	}

	return IndexableDocument{
		ID:   id,
		Data: doc,
	}, nil
}

// MapTraces converts trace entries to indexable documents
func (m *BleveDocumentMapper) MapTraces(traces []map[string]any, tenantID string) ([]IndexableDocument, error) {
	// Get pooled slice for documents
	documents := documentSlicePool.Get().([]IndexableDocument)
	defer func() {
		// Reset and return to pool
		documents = documents[:0]
		documentSlicePool.Put(documents)
	}()

	for i, traceEntry := range traces {
		doc, err := m.mapTrace(traceEntry, tenantID)
		if err != nil {
			m.logger.Warn("Failed to map trace entry", "error", err, "index", i)
			continue
		}
		documents = append(documents, doc)
	}

	// Return a copy to avoid pool contamination
	result := make([]IndexableDocument, len(documents))
	copy(result, documents)
	return result, nil
}

// mapTrace converts a single trace to an indexable document
func (m *BleveDocumentMapper) mapTrace(traceData map[string]interface{}, tenantID string) (IndexableDocument, error) {
	traceID, _ := traceData["traceID"].(string)
	if traceID == "" {
		return IndexableDocument{}, fmt.Errorf("trace missing traceID")
	}

	// Extract spans
	spans, _ := traceData["spans"].([]interface{})

	var startTime, endTime time.Time
	var totalDuration int64
	serviceName := ""
	operation := ""

	for _, spanInterface := range spans {
		spanData, ok := spanInterface.(map[string]interface{})
		if !ok {
			continue
		}

		spanStart := m.extractTimestamp(spanData)
		spanDuration := m.extractInt64Field(spanData, "duration")

		if startTime.IsZero() || spanStart.Before(startTime) {
			startTime = spanStart
		}
		spanEnd := spanStart.Add(time.Duration(spanDuration) * time.Millisecond)
		if endTime.IsZero() || spanEnd.After(endTime) {
			endTime = spanEnd
		}
		totalDuration += spanDuration

		// Extract service and operation from first span
		if serviceName == "" {
			processData, ok := spanData["process"].(map[string]interface{})
			if ok {
				serviceName = m.extractStringField(processData, "serviceName")
			}
		}
		if operation == "" {
			operation = m.extractStringField(spanData, "operationName")
		}
	}

	id := fmt.Sprintf("trace_%s_%s", tenantID, traceID)

	// Create new document (don't use pool to avoid data sharing)
	doc := &TraceDocument{
		ID:          id,
		TenantID:    tenantID,
		TraceID:     traceID,
		ServiceName: serviceName,
		Operation:   operation,
		StartTime:   startTime,
		EndTime:     endTime,
		Duration:    totalDuration,
		Raw:         traceData,
	}

	return IndexableDocument{
		ID:   id,
		Data: doc,
	}, nil
}

// GetIndexName returns the appropriate index name
func (m *BleveDocumentMapper) GetIndexName(dataType string, tenantID string) string {
	return fmt.Sprintf("mirador_%s_%s", dataType, tenantID)
}

// Helper functions

func (m *BleveDocumentMapper) extractTimestamp(data map[string]interface{}) time.Time {
	if ts, ok := data["timestamp"].(time.Time); ok {
		return ts
	}
	if ts, ok := data["startTime"].(time.Time); ok {
		return ts
	}
	if tsStr, ok := data["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
			return t
		}
	}
	if tsMillis, ok := data["timestamp"].(float64); ok {
		return time.UnixMilli(int64(tsMillis))
	}
	return time.Now() // fallback
}

func (m *BleveDocumentMapper) extractStringField(data map[string]interface{}, field string) string {
	if val, ok := data[field].(string); ok {
		return val
	}
	return ""
}

func (m *BleveDocumentMapper) extractInt64Field(data map[string]interface{}, field string) int64 {
	if val, ok := data[field].(int64); ok {
		return val
	}
	if val, ok := data[field].(float64); ok {
		return int64(val)
	}
	if val, ok := data[field].(int); ok {
		return int64(val)
	}
	return 0
}
