package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type metricsQuerier interface {
	ExecuteQuery(ctx context.Context, request *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error)
}

// ServiceGraphFetcher exposes the behaviour required by HTTP handlers.
type ServiceGraphFetcher interface {
	FetchServiceGraph(ctx context.Context, req *models.ServiceGraphRequest) (*models.ServiceGraphData, error)
}

// ServiceGraphService aggregates OpenTelemetry service graph metrics emitted by
// the servicegraph connector and stored in VictoriaMetrics.
type ServiceGraphService struct {
	metrics metricsQuerier
	logger  logger.Logger
}

func NewServiceGraphService(metrics *VictoriaMetricsService, logger logger.Logger) *ServiceGraphService {
	if metrics == nil {
		return nil
	}
	return &ServiceGraphService{metrics: metrics, logger: logger}
}

// FetchServiceGraph returns the directed service dependency edges observed
// within the provided time window. It aggregates metrics from all configured
// VictoriaMetrics sources (leveraging the underlying metrics service).
func (s *ServiceGraphService) FetchServiceGraph(ctx context.Context, req *models.ServiceGraphRequest) (*models.ServiceGraphData, error) {
	if s == nil || s.metrics == nil {
		return nil, errors.New("service graph metrics client not configured")
	}
	if req == nil {
		return nil, errors.New("service graph request missing")
	}

	start := req.Start.AsTime()
	end := req.End.AsTime()
	if start.IsZero() && !end.IsZero() {
		start = end.Add(-15 * time.Minute)
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if start.IsZero() {
		start = end.Add(-15 * time.Minute)
	}
	if !end.After(start) {
		return nil, fmt.Errorf("invalid time range: end %s must be after start %s", end.Format(time.RFC3339), start.Format(time.RFC3339))
	}

	window := end.Sub(start)
	rangeSeconds := int64(math.Round(window.Seconds()))
	if rangeSeconds <= 0 {
		rangeSeconds = 1
	}
	rangeSelector := fmt.Sprintf("%ds", rangeSeconds)
	evalTime := fmt.Sprintf("%d", end.Unix())

	selector := buildLabelSelector(req)

	agg := newEdgeAggregator(window)

	queries := []struct {
		metric string
		apply  func(key edgeKey, value float64)
	}{
		{metric: "traces_service_graph_request_total", apply: func(k edgeKey, v float64) { agg.edge(k).callCount += v }},
		{metric: "traces_service_graph_request_failed_total", apply: func(k edgeKey, v float64) { agg.edge(k).errorCount += v }},
		{metric: "traces_service_graph_unpaired_spans_total", apply: func(k edgeKey, v float64) { agg.edge(k).unpaired += v }},
		{metric: "traces_service_graph_dropped_spans_total", apply: func(k edgeKey, v float64) { agg.edge(k).dropped += v }},
		{metric: "traces_service_graph_request_server_sum", apply: func(k edgeKey, v float64) { agg.edge(k).serverSum += v }},
		{metric: "traces_service_graph_request_server_count", apply: func(k edgeKey, v float64) { agg.edge(k).serverCount += v }},
		{metric: "traces_service_graph_request_client_sum", apply: func(k edgeKey, v float64) { agg.edge(k).clientSum += v }},
		{metric: "traces_service_graph_request_client_count", apply: func(k edgeKey, v float64) { agg.edge(k).clientCount += v }},
	}

	for _, q := range queries {
		query := fmt.Sprintf("increase(%s%s[%s])", q.metric, selector, rangeSelector)
		samples, err := s.runInstantVector(ctx, query, evalTime)
		if err != nil {
			return nil, fmt.Errorf("query %s failed: %w", q.metric, err)
		}
		for _, sample := range samples {
			key := makeEdgeKey(sample.labels)
			if key.isEmpty() {
				continue
			}
			q.apply(key, sample.value)
		}
	}

	result := agg.build(start, end)
	sort.Slice(result.Edges, func(i, j int) bool {
		if result.Edges[i].CallRate == result.Edges[j].CallRate {
			return result.Edges[i].Source < result.Edges[j].Source
		}
		return result.Edges[i].CallRate > result.Edges[j].CallRate
	})

	return result, nil
}

type promSample struct {
	labels map[string]string
	value  float64
}

func (s *ServiceGraphService) runInstantVector(ctx context.Context, query, evalTime string) ([]promSample, error) {
	req := &models.MetricsQLQueryRequest{Query: query}
	if evalTime != "" {
		req.Time = evalTime
	}

	res, err := s.metrics.ExecuteQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	if res == nil || res.Data == nil {
		return nil, nil
	}

	raw, err := json.Marshal(res.Data)
	if err != nil {
		return nil, err
	}
	var payload struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.ResultType != "vector" {
		return nil, fmt.Errorf("unexpected result type %q", payload.ResultType)
	}

	samples := make([]promSample, 0, len(payload.Result))
	for _, series := range payload.Result {
		if len(series.Value) != 2 {
			continue
		}
		vStr, ok := series.Value[1].(string)
		if !ok {
			// Value may already be numeric after marshal/unmarshal.
			if f, ok := series.Value[1].(float64); ok {
				samples = append(samples, promSample{labels: series.Metric, value: f})
				continue
			}
			continue
		}
		f, err := strconv.ParseFloat(vStr, 64)
		if err != nil {
			continue
		}
		samples = append(samples, promSample{labels: series.Metric, value: f})
	}
	return samples, nil
}

type edgeKey struct {
	client         string
	server         string
	connectionType string
}

func makeEdgeKey(labels map[string]string) edgeKey {
	return edgeKey{
		client:         labels["client"],
		server:         labels["server"],
		connectionType: labels["connection_type"],
	}
}

func (k edgeKey) isEmpty() bool {
	return k.client == "" && k.server == ""
}

type edgeAccumulator struct {
	callCount   float64
	errorCount  float64
	serverSum   float64
	serverCount float64
	clientSum   float64
	clientCount float64
	unpaired    float64
	dropped     float64
}

type edgeAggregator struct {
	edges  map[edgeKey]*edgeAccumulator
	window time.Duration
}

func newEdgeAggregator(window time.Duration) *edgeAggregator {
	if window <= 0 {
		window = time.Minute
	}
	return &edgeAggregator{edges: make(map[edgeKey]*edgeAccumulator), window: window}
}

func (a *edgeAggregator) edge(key edgeKey) *edgeAccumulator {
	if acc, ok := a.edges[key]; ok {
		return acc
	}
	acc := &edgeAccumulator{}
	a.edges[key] = acc
	return acc
}

func (a *edgeAggregator) build(start, end time.Time) *models.ServiceGraphData {
	windowMinutes := a.window.Minutes()
	windowSeconds := a.window.Seconds()
	if windowMinutes <= 0 {
		windowMinutes = math.Max(windowSeconds/60, 1)
	}

	edges := make([]models.ServiceGraphEdge, 0, len(a.edges))
	for key, acc := range a.edges {
		if key.isEmpty() {
			continue
		}
		callRate := 0.0
		if windowMinutes > 0 {
			callRate = acc.callCount / windowMinutes
		}
		errorRate := 0.0
		if acc.callCount > 0 {
			errorRate = (acc.errorCount / acc.callCount) * 100
		}
		serverAvg := 0.0
		if acc.serverCount > 0 {
			serverAvg = (acc.serverSum / acc.serverCount) * 1000 // seconds → ms
		}
		clientAvg := 0.0
		if acc.clientCount > 0 {
			clientAvg = (acc.clientSum / acc.clientCount) * 1000
		}
		edges = append(edges, models.ServiceGraphEdge{
			Source:         key.client,
			Target:         key.server,
			ConnectionType: key.connectionType,
			CallCount:      acc.callCount,
			CallRate:       callRate,
			ErrorCount:     acc.errorCount,
			ErrorRate:      errorRate,
			ServerLatency:  models.ServiceGraphLatency{AverageMs: serverAvg},
			ClientLatency:  models.ServiceGraphLatency{AverageMs: clientAvg},
			UnpairedSpans:  acc.unpaired,
			DroppedSpans:   acc.dropped,
		})
	}

	return &models.ServiceGraphData{
		Window: models.ServiceGraphWindow{
			Start:           start,
			End:             end,
			DurationSeconds: int64(math.Round(windowSeconds)),
		},
		Edges: edges,
	}
}

func buildLabelSelector(req *models.ServiceGraphRequest) string {
	var parts []string
	if req.Client != "" {
		parts = append(parts, fmt.Sprintf("client=%q", req.Client))
	}
	if req.Server != "" {
		parts = append(parts, fmt.Sprintf("server=%q", req.Server))
	}
	if req.ConnectionType != "" {
		parts = append(parts, fmt.Sprintf("connection_type=%q", req.ConnectionType))
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ",") + "}"
}
