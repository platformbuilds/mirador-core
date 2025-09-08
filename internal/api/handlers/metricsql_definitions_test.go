package handlers

import (
    "context"
    "testing"
    "time"
    "github.com/platformbuilds/mirador-core/internal/repo"
)

// Test buildDefinitionsFiltered returns per-metric label definitions and placeholders
func TestBuildDefinitionsFiltered_PerMetricLabels(t *testing.T) {
    // Fake schema provider
    fake := &fakeSchema{
        metrics: map[string]*repo.MetricDef{
            "http_requests_total": {TenantID: "t1", Metric: "http_requests_total", Description: "Total HTTP requests", Owner: "obs", UpdatedAt: time.Now()},
        },
        labels: map[string]map[string]*repo.MetricLabelDef{
            "http_requests_total": {
                "instance": {TenantID: "t1", Metric: "http_requests_total", Label: "instance", Type: "string", Description: "The pod or host instance"},
            },
        },
    }
    h := &MetricsQLHandler{ schemaRepo: fake }

    // fake VM data with two metrics and labels
    data := map[string]any{
        "result": []any{
            map[string]any{"metric": map[string]any{"__name__": "http_requests_total", "job": "api", "instance": "pod-1"}},
            map[string]any{"metric": map[string]any{"__name__": "process_cpu_seconds_total", "namespace": "ns"}},
        },
    }

    defs := h.buildDefinitionsFiltered(context.Background(), "t1", data, nil)
    if defs == nil { t.Fatalf("defs nil") }
    metricsDefs, ok := defs["metrics"].(map[string]any)
    if !ok { t.Fatalf("metrics defs missing") }
    if _, ok := metricsDefs["http_requests_total"]; !ok { t.Errorf("http_requests_total def missing") }
    if _, ok := metricsDefs["process_cpu_seconds_total"]; !ok { t.Errorf("process_cpu_seconds_total placeholder missing") }

    labelsPerMetric, ok := defs["labels"].(map[string]any)
    if !ok { t.Fatalf("labels defs missing") }
    // http_requests_total labels
    lhttp, ok := labelsPerMetric["http_requests_total"].(map[string]any)
    if !ok { t.Fatalf("labels for http_requests_total missing") }
    if _, ok := lhttp["instance"]; !ok { t.Errorf("instance label def missing for http_requests_total") }
    if _, ok := lhttp["job"]; !ok { t.Errorf("job placeholder missing for http_requests_total") }
    // process_cpu_seconds_total labels
    lproc, ok := labelsPerMetric["process_cpu_seconds_total"].(map[string]any)
    if !ok { t.Fatalf("labels for process_cpu_seconds_total missing") }
    if _, ok := lproc["namespace"]; !ok { t.Errorf("namespace placeholder missing for process_cpu_seconds_total") }
}

type fakeSchema struct{
    metrics map[string]*repo.MetricDef
    labels map[string]map[string]*repo.MetricLabelDef // metric -> label -> def
}

func (f *fakeSchema) GetMetric(ctx context.Context, tenantID, metric string) (*repo.MetricDef, error) {
    if m, ok := f.metrics[metric]; ok { return m, nil }
    return nil, context.Canceled // non-nil error indicates not found
}
func (f *fakeSchema) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
    out := map[string]*repo.MetricLabelDef{}
    if m, ok := f.labels[metric]; ok {
        for _, l := range labels {
            if d, ok := m[l]; ok { out[l] = d }
        }
    }
    return out, nil
}
