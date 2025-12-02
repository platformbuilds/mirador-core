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

// helper to create a fake VM server responding on single-node API paths
func newFakeVM(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	// default handlers
	// instant query
	if handlers["/api/v1/query"] == nil {
		mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models.VictoriaMetricsResponse{
				Status: "success",
				Data:   map[string]any{"result": []any{}},
			})
		})
	} else {
		mux.HandleFunc("/api/v1/query", handlers["/api/v1/query"])
	}

	// range query
	if handlers["/api/v1/query_range"] == nil {
		mux.HandleFunc("/api/v1/query_range", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models.VictoriaMetricsResponse{Status: "success", Data: map[string]any{"result": []any{}}})
		})
	} else {
		mux.HandleFunc("/api/v1/query_range", handlers["/api/v1/query_range"])
	}

	// series
	if handlers["/api/v1/series"] == nil {
		mux.HandleFunc("/api/v1/series", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(struct {
				Status string              `json:"status"`
				Data   []map[string]string `json:"data"`
			}{Status: "success", Data: []map[string]string{}})
		})
	} else {
		mux.HandleFunc("/api/v1/series", handlers["/api/v1/series"])
	}

	// labels - now uses /api/v1/series endpoint
	// (already handled above in series section)

	// specific label values override if provided
	if h, ok := handlers["/api/v1/label/app/values"]; ok {
		mux.HandleFunc("/api/v1/label/app/values", h)
	}
	// label values: catch-all to return empty
	mux.HandleFunc("/api/v1/label/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Status string   `json:"status"`
			Data   []string `json:"data"`
		}{Status: "success", Data: []string{}})
	})

	// health
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	// cluster paths → 404 to trigger fallback
	mux.HandleFunc("/select/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	return httptest.NewServer(mux)
}

func TestMetrics_Aggregated_ExecuteQuery_ConcatsResults(t *testing.T) {
	srvA := newFakeVM(t, map[string]http.HandlerFunc{
		"/api/v1/query": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models.VictoriaMetricsResponse{
				Status: "success",
				Data: map[string]any{"result": []any{
					map[string]any{"metric": map[string]any{"__name__": "up", "src": "A"}, "value": []any{1, 2}},
				}},
			})
		},
	})
	defer srvA.Close()
	srvB := newFakeVM(t, map[string]http.HandlerFunc{
		"/api/v1/query": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models.VictoriaMetricsResponse{
				Status: "success",
				Data: map[string]any{"result": []any{
					map[string]any{"metric": map[string]any{"__name__": "up", "src": "B"}, "value": []any{1, 2}},
				}},
			})
		},
	})
	defer srvB.Close()

	log := logger.New("error")
	parent := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "parent", Endpoints: []string{}, Timeout: 2000}, log)
	childA := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "fin_metrics", Endpoints: []string{srvA.URL}, Timeout: 2000}, log)
	childB := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "os_metrics", Endpoints: []string{srvB.URL}, Timeout: 2000}, log)
	parent.SetChildren([]*VictoriaMetricsService{childA, childB})

	res, err := parent.ExecuteQuery(context.Background(), &models.MetricsQLQueryRequest{Query: "up"})
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	if res == nil || res.Data == nil {
		t.Fatalf("no data")
	}
	// ensure both series are present
	m, _ := res.Data.(map[string]any)
	arr, _ := m["result"].([]any)
	if len(arr) != 2 {
		t.Fatalf("expected 2 series, got %d", len(arr))
	}
}

func TestMetrics_Aggregated_ExecuteRange_SumsPoints(t *testing.T) {
	mk := func(points int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			series := map[string]any{"metric": map[string]any{"__name__": "reqs_total"}}
			vals := make([]any, 0, points)
			for i := 0; i < points; i++ {
				vals = append(vals, []any{float64(i), float64(i)})
			}
			series["values"] = vals
			_ = json.NewEncoder(w).Encode(models.VictoriaMetricsResponse{Status: "success", Data: map[string]any{"result": []any{series}}})
		}
	}
	srvA := newFakeVM(t, map[string]http.HandlerFunc{"/api/v1/query_range": mk(3)})
	defer srvA.Close()
	srvB := newFakeVM(t, map[string]http.HandlerFunc{"/api/v1/query_range": mk(5)})
	defer srvB.Close()

	log := logger.New("error")
	parent := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "parent", Timeout: 2000}, log)
	childA := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "fin_metrics", Endpoints: []string{srvA.URL}, Timeout: 2000}, log)
	childB := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "os_metrics", Endpoints: []string{srvB.URL}, Timeout: 2000}, log)
	parent.SetChildren([]*VictoriaMetricsService{childA, childB})

	res, err := parent.ExecuteRangeQuery(context.Background(), &models.MetricsQLRangeQueryRequest{Query: "sum(rate(reqs_total[1m]))", Start: "0", End: "10", Step: "1"})
	if err != nil {
		t.Fatalf("ExecuteRangeQuery: %v", err)
	}
	if res.DataPointCount != 8 {
		t.Fatalf("expected 8 points, got %d", res.DataPointCount)
	}
}

func TestMetrics_Aggregated_Labels_Union(t *testing.T) {
	srvA := newFakeVM(t, map[string]http.HandlerFunc{
		"/api/v1/series": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(struct {
				Status string                   `json:"status"`
				Data   []map[string]interface{} `json:"data"`
			}{Status: "success", Data: []map[string]interface{}{
				{"__name__": "metric1", "job": "job1"},
				{"__name__": "metric1", "job": "job2"},
			}})
		},
	})
	defer srvA.Close()
	srvB := newFakeVM(t, map[string]http.HandlerFunc{
		"/api/v1/series": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(struct {
				Status string                   `json:"status"`
				Data   []map[string]interface{} `json:"data"`
			}{Status: "success", Data: []map[string]interface{}{
				{"__name__": "metric2", "instance": "instance1", "job": "job3"},
				{"__name__": "metric2", "instance": "instance2", "job": "job4"},
			}})
		},
	})
	defer srvB.Close()

	log := logger.New("error")
	parent := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "parent", Timeout: 2000}, log)
	parent.SetChildren([]*VictoriaMetricsService{
		NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "A", Endpoints: []string{srvA.URL}, Timeout: 2000}, log),
		NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "B", Endpoints: []string{srvB.URL}, Timeout: 2000}, log),
	})

	labels, err := parent.GetLabels(context.Background(), &models.LabelsRequest{})
	if err != nil {
		t.Fatalf("GetLabels: %v", err)
	}
	if len(labels) != 3 {
		t.Fatalf("expected union size 3, got %d (%v)", len(labels), labels)
	}
}

func TestMetrics_Aggregated_LabelValues_Union(t *testing.T) {
	// override label values for /api/v1/label/app/values
	srvA := newFakeVM(t, map[string]http.HandlerFunc{"/api/v1/label/app/values": func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Status string   `json:"status"`
			Data   []string `json:"data"`
		}{Status: "success", Data: []string{"api", "worker"}})
	}})
	defer srvA.Close()
	srvB := newFakeVM(t, map[string]http.HandlerFunc{"/api/v1/label/app/values": func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Status string   `json:"status"`
			Data   []string `json:"data"`
		}{Status: "success", Data: []string{"worker", "cron"}})
	}})
	defer srvB.Close()

	log := logger.New("error")
	parent := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "parent", Timeout: 2000}, log)
	parent.SetChildren([]*VictoriaMetricsService{
		NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "A", Endpoints: []string{srvA.URL}, Timeout: 2000}, log),
		NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "B", Endpoints: []string{srvB.URL}, Timeout: 2000}, log),
	})

	vals, err := parent.GetLabelValues(context.Background(), &models.LabelValuesRequest{Label: "app"})
	if err != nil {
		t.Fatalf("GetLabelValues: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("expected union size 3, got %d (%v)", len(vals), vals)
	}
}

func TestMetrics_Aggregated_AllSourcesFail_Error(t *testing.T) {
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }))
	defer srvB.Close()

	log := logger.New("error")
	parent := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "parent", Timeout: 2000}, log)
	parent.SetChildren([]*VictoriaMetricsService{
		NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "A", Endpoints: []string{srvA.URL}, Timeout: 2000}, log),
		NewVictoriaMetricsService(config.VictoriaMetricsConfig{Name: "B", Endpoints: []string{srvB.URL}, Timeout: 2000}, log),
	})

	if _, err := parent.ExecuteQuery(context.Background(), &models.MetricsQLQueryRequest{Query: "up"}); err == nil {
		t.Fatalf("expected error when all sources fail")
	}
}
