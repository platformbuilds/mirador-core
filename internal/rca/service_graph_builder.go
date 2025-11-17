package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MetricsQuerier interface for querying servicegraph metrics.
// This abstracts the underlying metrics store (VictoriaMetrics, Prometheus, etc).
type MetricsQuerier interface {
	ExecuteQuery(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error)
}

// ServiceGraphBuilder builds a ServiceGraph from servicegraph connector metrics.
// It queries the configured metrics store for servicegraph metrics and constructs
// the dependency topology.
type ServiceGraphBuilder struct {
	metricsQuerier MetricsQuerier
	logger         logger.Logger
}

// NewServiceGraphBuilder creates a new ServiceGraphBuilder.
func NewServiceGraphBuilder(metricsQuerier MetricsQuerier, logger logger.Logger) *ServiceGraphBuilder {
	return &ServiceGraphBuilder{
		metricsQuerier: metricsQuerier,
		logger:         logger,
	}
}

// BuildGraph constructs a ServiceGraph from servicegraph metrics within a time range.
// It queries multiple servicegraph metrics and aggregates them into a dependency graph.
func (sgb *ServiceGraphBuilder) BuildGraph(ctx context.Context, tenantID string, start, end time.Time) (*ServiceGraph, error) {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return nil, fmt.Errorf("invalid time range: start=%v, end=%v", start, end)
	}

	graph := NewServiceGraph()

	// Calculate time window for rate calculations.
	window := end.Sub(start)
	windowSeconds := window.Seconds()
	if windowSeconds <= 0 {
		windowSeconds = 1
	}

	// Query servicegraph metrics. The servicegraph connector emits:
	// - traces_service_graph_request_total
	// - traces_service_graph_request_failed_total
	// - traces_service_graph_request_server_sum / _count (server latency)
	// - traces_service_graph_request_client_sum / _count (client latency)
	// All with labels: client, server, connection_type

	metricsToQuery := []struct {
		name  string
		apply func(edge *ServiceEdge, value float64)
	}{
		{
			name: "traces_service_graph_request_total",
			apply: func(edge *ServiceEdge, value float64) {
				edge.RequestCount = value
			},
		},
		{
			name: "traces_service_graph_request_failed_total",
			apply: func(edge *ServiceEdge, value float64) {
				edge.FailureCount = value
			},
		},
		{
			name: "traces_service_graph_request_server_sum",
			apply: func(edge *ServiceEdge, value float64) {
				edge.LatencyAvgMs = value // Will be normalized below
			},
		},
		{
			name: "traces_service_graph_request_server_count",
			apply: func(edge *ServiceEdge, value float64) {
				// Used to normalize server latency
				if edge.LatencyAvgMs > 0 && value > 0 {
					edge.LatencyAvgMs = edge.LatencyAvgMs / value
				} else {
					edge.LatencyAvgMs = 0
				}
			},
		},
	}

	// Accumulator for edges keyed by (source, target)
	edgeMap := make(map[string]*ServiceEdge)

	// Query each metric
	for _, metric := range metricsToQuery {
		samples, err := sgb.querySamples(ctx, tenantID, metric.name, start, end)
		if err != nil {
			sgb.logger.Warn("Failed to query metric",
				"metric", metric.name,
				"error", err)
			// Continue with other metrics rather than fail entirely
			continue
		}

		for _, sample := range samples {
			client, ok := sample.labels["client"]
			if !ok {
				continue
			}
			server, ok := sample.labels["server"]
			if !ok {
				continue
			}

			key := fmt.Sprintf("%s->%s", client, server)
			if _, ok := edgeMap[key]; !ok {
				edgeMap[key] = &ServiceEdge{
					Source:     ServiceNode(client),
					Target:     ServiceNode(server),
					Attributes: make(map[string]interface{}),
				}
			}

			metric.apply(edgeMap[key], sample.value)
		}
	}

	// Normalize rates and error rates
	for _, edge := range edgeMap {
		edge.RequestRate = edge.RequestCount / windowSeconds
		edge.FailureRate = edge.FailureCount / windowSeconds

		if edge.RequestCount > 0 {
			edge.ErrorRate = edge.FailureCount / edge.RequestCount
		} else {
			edge.ErrorRate = 0
		}

		// Store labels in attributes
		if edge.Attributes == nil {
			edge.Attributes = make(map[string]interface{})
		}
		edge.Attributes["time_window_seconds"] = windowSeconds

		graph.AddEdge(*edge)
	}

	sgb.logger.Info("Built service graph from metrics",
		"nodes", graph.Size(),
		"edges", graph.EdgeCount(),
		"time_range", fmt.Sprintf("%s to %s", start.Format(time.RFC3339), end.Format(time.RFC3339)))

	return graph, nil
}

// promSample represents a single Prometheus/VictoriaMetrics sample.
type promSample struct {
	labels map[string]string
	value  float64
}

// querySamples executes a query for a single metric and returns all samples.
func (sgb *ServiceGraphBuilder) querySamples(ctx context.Context, tenantID, metricName string, start, end time.Time) ([]promSample, error) {
	// Use the same approach as ServiceGraphService in services/service_graph.service.go:
	// Query metric using increase() to get total count over the time window.

	window := end.Sub(start)
	rangeSeconds := int64(math.Round(window.Seconds()))
	if rangeSeconds <= 0 {
		rangeSeconds = 1
	}

	// Build MetricsQL query using increase() to get cumulative change over the window.
	query := fmt.Sprintf("increase(%s[%ds])", metricName, rangeSeconds)

	req := &models.MetricsQLQueryRequest{
		Query:    query,
		TenantID: tenantID,
	}

	res, err := sgb.metricsQuerier.ExecuteQuery(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute query failed: %w", err)
	}

	if res == nil || res.Data == nil {
		return nil, nil
	}

	// Parse Prometheus instant vector response.
	raw, err := json.Marshal(res.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal response failed: %w", err)
	}

	var payload struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	var samples []promSample
	for _, result := range payload.Result {
		if len(result.Value) < 2 {
			continue
		}

		// result.Value[0] is timestamp, result.Value[1] is value.
		valueStr, ok := result.Value[1].(string)
		if !ok {
			continue
		}

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		samples = append(samples, promSample{
			labels: result.Metric,
			value:  value,
		})
	}

	return samples, nil
}
