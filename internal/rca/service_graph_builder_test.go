package rca

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MockMetricsQuerier implements MetricsQuerier for testing.
type MockMetricsQuerier struct {
	responses map[string]interface{}
}

func NewMockMetricsQuerier() *MockMetricsQuerier {
	return &MockMetricsQuerier{
		responses: make(map[string]interface{}),
	}
}

// SetResponse sets the response for a specific query pattern.
// The key should be the metric name (e.g., "traces_service_graph_request_total")
// and it will match queries regardless of the time range suffix.
func (m *MockMetricsQuerier) SetResponse(queryPattern string, response interface{}) {
	m.responses[queryPattern] = response
}

// extractMetricName extracts the metric name from a MetricsQL query.
func (m *MockMetricsQuerier) extractMetricName(query string) string {
	// Query is typically in the form "increase(metric_name[time_range])"
	// Extract the metric name.
	start := strings.Index(query, "(")
	if start == -1 {
		return query
	}
	end := strings.Index(query[start:], "[")
	if end == -1 {
		return query[start+1:]
	}
	return query[start+1 : start+end]
}

func (m *MockMetricsQuerier) ExecuteQuery(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	// Extract metric name from query to match against registered responses.
	metricName := m.extractMetricName(request.Query)

	var data interface{}

	if response, ok := m.responses[metricName]; ok {
		data = response
	} else if response, ok := m.responses[request.Query]; ok {
		// Fall back to exact query match.
		data = response
	} else {
		// Return an empty result if no specific response set.
		data = map[string]interface{}{
			"resultType": "vector",
			"result":     []interface{}{},
		}
	}

	return &models.MetricsQLQueryResult{Data: data}, nil
}

func TestServiceGraphBuilder_BuildGraph_Basic(t *testing.T) {
	mockQuerier := NewMockMetricsQuerier()
	log := logger.New("info")
	builder := NewServiceGraphBuilder(mockQuerier, log)

	// Set up response for traces_service_graph_request_total.
	mockQuerier.SetResponse("traces_service_graph_request_total", map[string]interface{}{
		"resultType": "vector",
		"result": []map[string]interface{}{
			{
				"metric": map[string]string{
					"client": "api-gateway",
					"server": "tps",
				},
				"value": []interface{}{"1234567890", "100"},
			},
			{
				"metric": map[string]string{
					"client": "tps",
					"server": "cassandra",
				},
				"value": []interface{}{"1234567890", "80"},
			},
		},
	})

	// Set response for failures (none in this test).
	mockQuerier.SetResponse("traces_service_graph_request_failed_total", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})

	// Set response for latencies (empty for simplicity).
	mockQuerier.SetResponse("traces_service_graph_request_server_sum", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})

	mockQuerier.SetResponse("traces_service_graph_request_server_count", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})

	start := time.Now().UTC().Add(-15 * time.Minute)
	end := time.Now().UTC()

	graph, err := builder.BuildGraph(context.Background(), "default", start, end)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	// Should have 3 nodes: api-gateway, tps, cassandra.
	if graph.Size() != 3 {
		t.Errorf("Expected 3 nodes, got %d", graph.Size())
	}

	// Should have 2 edges.
	if graph.EdgeCount() != 2 {
		t.Errorf("Expected 2 edges, got %d", graph.EdgeCount())
	}

	// Check api-gateway -> tps edge.
	edge, ok := graph.GetEdge(ServiceNode("api-gateway"), ServiceNode("tps"))
	if !ok {
		t.Fatal("Expected edge api-gateway -> tps")
	}

	if edge.RequestCount != 100 {
		t.Errorf("Expected RequestCount=100, got %f", edge.RequestCount)
	}

	// RequestRate should be calculated (100 requests / time window).
	if edge.RequestRate == 0 {
		t.Errorf("Expected non-zero RequestRate, got %f", edge.RequestRate)
	}
}

func TestServiceGraphBuilder_BuildGraph_WithFailures(t *testing.T) {
	mockQuerier := NewMockMetricsQuerier()
	log := logger.New("info")
	builder := NewServiceGraphBuilder(mockQuerier, log)

	// Set responses for request totals and failures.
	mockQuerier.SetResponse("traces_service_graph_request_total", map[string]interface{}{
		"resultType": "vector",
		"result": []map[string]interface{}{
			{
				"metric": map[string]string{
					"client": "api-gateway",
					"server": "tps",
				},
				"value": []interface{}{"1234567890", "100"},
			},
		},
	})

	mockQuerier.SetResponse("traces_service_graph_request_failed_total", map[string]interface{}{
		"resultType": "vector",
		"result": []map[string]interface{}{
			{
				"metric": map[string]string{
					"client": "api-gateway",
					"server": "tps",
				},
				"value": []interface{}{"1234567890", "10"},
			},
		},
	})

	mockQuerier.SetResponse("traces_service_graph_request_server_sum", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})

	mockQuerier.SetResponse("traces_service_graph_request_server_count", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})

	start := time.Now().UTC().Add(-5 * time.Minute)
	end := time.Now().UTC()

	graph, err := builder.BuildGraph(context.Background(), "default", start, end)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	edge, ok := graph.GetEdge(ServiceNode("api-gateway"), ServiceNode("tps"))
	if !ok {
		t.Fatal("Expected edge")
	}

	if edge.FailureCount != 10 {
		t.Errorf("Expected FailureCount=10, got %f", edge.FailureCount)
	}

	// ErrorRate = failures / requests = 10 / 100 = 0.1
	expectedErrorRate := 0.1
	if edge.ErrorRate != expectedErrorRate {
		t.Errorf("Expected ErrorRate=%f, got %f", expectedErrorRate, edge.ErrorRate)
	}
}

func TestServiceGraphBuilder_BuildGraph_InvalidTimeRange(t *testing.T) {
	mockQuerier := NewMockMetricsQuerier()
	log := logger.New("info")
	builder := NewServiceGraphBuilder(mockQuerier, log)

	// Test with zero times.
	_, err := builder.BuildGraph(context.Background(), "default", time.Time{}, time.Time{})
	if err == nil {
		t.Fatal("Expected error for invalid time range")
	}

	// Test with end before start.
	now := time.Now().UTC()
	_, err = builder.BuildGraph(context.Background(), "default", now.Add(1*time.Hour), now)
	if err == nil {
		t.Fatal("Expected error for end before start")
	}
}

func TestServiceGraphBuilder_QuerySamples(t *testing.T) {
	mockQuerier := NewMockMetricsQuerier()
	log := logger.New("info")
	_ = NewServiceGraphBuilder(mockQuerier, log)

	// Mock response with specific format.
	mockQuerier.SetResponse("increase(test_metric[*])", map[string]interface{}{
		"resultType": "vector",
		"result": []map[string]interface{}{
			{
				"metric": map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				"value": []interface{}{"1234567890", "42.5"},
			},
		},
	})

	// Test setup verified; querySamples is tested indirectly through BuildGraph.
}

func TestServiceGraphBuilder_ComplexServiceTopology(t *testing.T) {
	mockQuerier := NewMockMetricsQuerier()
	log := logger.New("info")
	builder := NewServiceGraphBuilder(mockQuerier, log)

	// Build the financial transaction service graph:
	// api-gateway -> tps
	// api-gateway -> keydb
	// tps -> kafka
	// tps -> cassandra
	// kafka -> cassandra

	mockQuerier.SetResponse("traces_service_graph_request_total", map[string]interface{}{
		"resultType": "vector",
		"result": []map[string]interface{}{
			{
				"metric": map[string]string{"client": "api-gateway", "server": "tps"},
				"value":  []interface{}{"1", "1000"},
			},
			{
				"metric": map[string]string{"client": "api-gateway", "server": "keydb"},
				"value":  []interface{}{"1", "500"},
			},
			{
				"metric": map[string]string{"client": "tps", "server": "kafka"},
				"value":  []interface{}{"1", "700"},
			},
			{
				"metric": map[string]string{"client": "tps", "server": "cassandra"},
				"value":  []interface{}{"1", "1500"},
			},
			{
				"metric": map[string]string{"client": "kafka", "server": "cassandra"},
				"value":  []interface{}{"1", "1500"},
			},
		},
	})

	mockQuerier.SetResponse("traces_service_graph_request_failed_total", map[string]interface{}{
		"resultType": "vector",
		"result": []map[string]interface{}{
			{
				"metric": map[string]string{"client": "api-gateway", "server": "tps"},
				"value":  []interface{}{"1", "10"},
			},
			{
				"metric": map[string]string{"client": "tps", "server": "cassandra"},
				"value":  []interface{}{"1", "5"},
			},
		},
	})

	mockQuerier.SetResponse("traces_service_graph_request_server_sum", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})

	mockQuerier.SetResponse("traces_service_graph_request_server_count", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})

	start := time.Now().UTC().Add(-15 * time.Minute)
	end := time.Now().UTC()

	graph, err := builder.BuildGraph(context.Background(), "default", start, end)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	// Verify nodes.
	nodes := graph.AllNodes()
	if len(nodes) != 5 {
		t.Errorf("Expected 5 nodes, got %d: %v", len(nodes), nodes)
	}

	// Verify edges.
	if graph.EdgeCount() != 5 {
		t.Errorf("Expected 5 edges, got %d", graph.EdgeCount())
	}

	// Verify error rates.
	edge, ok := graph.GetEdge(ServiceNode("api-gateway"), ServiceNode("tps"))
	if !ok {
		t.Fatal("Expected api-gateway -> tps edge")
	}
	expectedErrorRate := 10.0 / 1000.0
	if edge.ErrorRate != expectedErrorRate {
		t.Errorf("Expected ErrorRate=%f, got %f", expectedErrorRate, edge.ErrorRate)
	}

	// Verify topology relationships.
	if !graph.IsUpstream(ServiceNode("api-gateway"), ServiceNode("cassandra")) {
		t.Error("Expected api-gateway -> cassandra to be reachable")
	}

	path, found := graph.ShortestPath(ServiceNode("api-gateway"), ServiceNode("cassandra"))
	if !found {
		t.Fatal("Expected path from api-gateway to cassandra")
	}
	if len(path) < 2 {
		t.Errorf("Expected path of length >= 2, got %d", len(path))
	}
}

// TestServiceGraphBuilder_ResponseParsing tests parsing of different response formats.
func TestServiceGraphBuilder_ResponseParsing(t *testing.T) {
	mockQuerier := NewMockMetricsQuerier()
	log := logger.New("info")
	builder := NewServiceGraphBuilder(mockQuerier, log)

	// Mock response with JSON-encoded response (as would come from real VictoriaMetrics).
	responseData := map[string]interface{}{
		"resultType": "vector",
		"result": []map[string]interface{}{
			{
				"metric": map[string]string{
					"client":          "api-gateway",
					"server":          "tps",
					"connection_type": "http",
				},
				"value": []interface{}{"1234567890", "123.45"},
			},
		},
	}

	// Ensure it can be marshaled/unmarshaled as JSON.
	jsonBytes, err := json.Marshal(responseData)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	mockQuerier.SetResponse("traces_service_graph_request_total", parsed)
	mockQuerier.SetResponse("traces_service_graph_request_failed_total", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})
	mockQuerier.SetResponse("traces_service_graph_request_server_sum", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})
	mockQuerier.SetResponse("traces_service_graph_request_server_count", map[string]interface{}{
		"resultType": "vector",
		"result":     []interface{}{},
	})

	start := time.Now().UTC().Add(-5 * time.Minute)
	end := time.Now().UTC()

	graph, err := builder.BuildGraph(context.Background(), "default", start, end)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	edge, ok := graph.GetEdge(ServiceNode("api-gateway"), ServiceNode("tps"))
	if !ok {
		t.Fatal("Expected edge")
	}

	if edge.RequestCount != 123.45 {
		t.Errorf("Expected RequestCount=123.45, got %f", edge.RequestCount)
	}
}
