//go:build integration

package services

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/platformbuilds/mirador-core/internal/config"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// Integration Test Cases: exercise HTTP client against a mocked VM endpoint,
// including cluster path fallback and happy-path JSON parsing.
func TestVictoriaMetricsService_Integration_QueryAndLabels(t *testing.T) {
    // Mock VM: return 404 for cluster path, 200 for single-node path
    mux := http.NewServeMux()
    mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request){
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(models.VictoriaMetricsResponse{
            Status: "success",
            Data: map[string]any{"result": []any{map[string]any{"metric": map[string]string{"__name__":"up"}, "value": []any{1,2}}}},
        })
    })
    mux.HandleFunc("/api/v1/labels", func(w http.ResponseWriter, r *http.Request){
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(struct{Status string `json:"status"`; Data []string `json:"data"`}{
            Status: "success", Data: []string{"__name__","job"},
        })
    })
    // catch-all returns 404 to trigger fallback
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){ http.NotFound(w, r) })
    ts := httptest.NewServer(mux)
    defer ts.Close()

    log := logger.New("error")
    svc := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{ts.URL}, Timeout: 2000}, log)

    ctx := context.Background()
    res, err := svc.ExecuteQuery(ctx, &models.MetricsQLQueryRequest{Query: "up"})
    if err != nil { t.Fatalf("query: %v", err) }
    if res.SeriesCount == 0 { t.Fatalf("expected series") }

    labels, err := svc.GetLabels(ctx, &models.LabelsRequest{})
    if err != nil { t.Fatalf("labels: %v", err) }
    if len(labels) == 0 { t.Fatalf("expected labels") }
}

