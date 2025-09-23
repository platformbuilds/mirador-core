package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type stubMetrics struct {
	responses map[string]*models.MetricsQLQueryResult
}

func (s *stubMetrics) ExecuteQuery(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error) {
	if req == nil {
		return nil, fmt.Errorf("missing request")
	}
	if res, ok := s.responses[req.Query]; ok {
		return res, nil
	}
	return &models.MetricsQLQueryResult{Status: "success", Data: map[string]any{"resultType": "vector", "result": []any{}}}, nil
}

func vectorResult(labels map[string]string, value float64) map[string]any {
	return map[string]any{
		"resultType": "vector",
		"result": []any{
			map[string]any{
				"metric": labels,
				"value":  []any{float64(0), fmt.Sprintf("%f", value)},
			},
		},
	}
}

func TestServiceGraphService_AggregatesMetrics(t *testing.T) {
	labels := map[string]string{"client": "checkout", "server": "payments", "connection_type": "http"}
	responses := map[string]*models.MetricsQLQueryResult{
		"increase(traces_service_graph_request_total[300s])":        {Status: "success", Data: vectorResult(labels, 600)},
		"increase(traces_service_graph_request_failed_total[300s])": {Status: "success", Data: vectorResult(labels, 30)},
		"increase(traces_service_graph_unpaired_spans_total[300s])": {Status: "success", Data: vectorResult(labels, 10)},
		"increase(traces_service_graph_dropped_spans_total[300s])":  {Status: "success", Data: vectorResult(labels, 5)},
		"increase(traces_service_graph_request_server_sum[300s])":   {Status: "success", Data: vectorResult(labels, 120)},
		"increase(traces_service_graph_request_server_count[300s])": {Status: "success", Data: vectorResult(labels, 600)},
		"increase(traces_service_graph_request_client_sum[300s])":   {Status: "success", Data: vectorResult(labels, 90)},
		"increase(traces_service_graph_request_client_count[300s])": {Status: "success", Data: vectorResult(labels, 600)},
	}

	svc := &ServiceGraphService{metrics: &stubMetrics{responses: responses}, logger: logger.New("error")}
	start := time.Unix(0, 0).UTC()
	end := start.Add(5 * time.Minute)
	req := &models.ServiceGraphRequest{Start: models.FlexibleTime{Time: start}, End: models.FlexibleTime{Time: end}}

	data, err := svc.FetchServiceGraph(context.Background(), "", req)
	if err != nil {
		t.Fatalf("FetchServiceGraph error: %v", err)
	}
	if data == nil {
		t.Fatalf("expected data")
	}
	if data.Window.DurationSeconds != 300 {
		t.Fatalf("expected duration 300s, got %d", data.Window.DurationSeconds)
	}
	if len(data.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(data.Edges))
	}
	edge := data.Edges[0]
	if edge.Source != "checkout" || edge.Target != "payments" || edge.ConnectionType != "http" {
		t.Fatalf("unexpected edge labels: %+v", edge)
	}
	if edge.CallCount != 600 {
		t.Fatalf("expected call count 600, got %f", edge.CallCount)
	}
	if diff := edge.CallRate - 120; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("expected call rate 120, got %f", edge.CallRate)
	}
	if diff := edge.ErrorRate - 5; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("expected error rate 5, got %f", edge.ErrorRate)
	}
	if diff := edge.ServerLatency.AverageMs - 200; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("expected server latency 200ms, got %f", edge.ServerLatency.AverageMs)
	}
	if diff := edge.ClientLatency.AverageMs - 150; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("expected client latency 150ms, got %f", edge.ClientLatency.AverageMs)
	}
	if edge.UnpairedSpans != 10 || edge.DroppedSpans != 5 {
		t.Fatalf("unexpected span counters: %+v", edge)
	}
}

func TestServiceGraphService_InvalidRange(t *testing.T) {
	svc := &ServiceGraphService{metrics: &stubMetrics{responses: map[string]*models.MetricsQLQueryResult{}}, logger: logger.New("error")}
	end := time.Unix(0, 0).UTC()
	start := end.Add(5 * time.Minute)
	req := &models.ServiceGraphRequest{Start: models.FlexibleTime{Time: start}, End: models.FlexibleTime{Time: end}}

	_, err := svc.FetchServiceGraph(context.Background(), "", req)
	if err == nil {
		t.Fatalf("expected error for invalid range")
	}
}
