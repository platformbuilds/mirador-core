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

func fakeVT(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	// services list
	if h, ok := handlers["/select/jaeger/api/services"]; ok {
		mux.HandleFunc("/select/jaeger/api/services", h)
	} else {
		mux.HandleFunc("/select/jaeger/api/services", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(struct {
				Data []string `json:"data"`
			}{Data: []string{}})
		})
	}
	// search
	if h, ok := handlers["/select/jaeger/api/traces"]; ok {
		mux.HandleFunc("/select/jaeger/api/traces", h)
	} else {
		mux.HandleFunc("/select/jaeger/api/traces", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(struct {
				Data []map[string]any `json:"data"`
			}{Data: []map[string]any{}})
		})
	}
	// get trace by id: match prefix
	mux.HandleFunc("/select/jaeger/api/traces/", func(w http.ResponseWriter, r *http.Request) {
		if h, ok := handlers["/select/jaeger/api/traces/"]; ok {
			h(w, r)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	return httptest.NewServer(mux)
}

func TestTraces_Aggregated_GetServices_Union(t *testing.T) {
	a := fakeVT(t, map[string]http.HandlerFunc{"/select/jaeger/api/services": func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []string `json:"data"`
		}{Data: []string{"svc-a", "svc-common"}})
	}})
	defer a.Close()
	b := fakeVT(t, map[string]http.HandlerFunc{"/select/jaeger/api/services": func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []string `json:"data"`
		}{Data: []string{"svc-b", "svc-common"}})
	}})
	defer b.Close()
	log := logger.New("error")
	parent := NewVictoriaTracesService(config.VictoriaTracesConfig{Name: "parent"}, log)
	parent.SetChildren([]*VictoriaTracesService{
		NewVictoriaTracesService(config.VictoriaTracesConfig{Name: "A", Endpoints: []string{a.URL}, Timeout: 2000}, log),
		NewVictoriaTracesService(config.VictoriaTracesConfig{Name: "B", Endpoints: []string{b.URL}, Timeout: 2000}, log),
	})
	svcs, err := parent.GetServices(context.Background(), "")
	if err != nil {
		t.Fatalf("GetServices: %v", err)
	}
	if len(svcs) != 3 {
		t.Fatalf("expected union size 3, got %d (%v)", len(svcs), svcs)
	}
}

func TestTraces_Aggregated_Search_Concat(t *testing.T) {
	a := fakeVT(t, map[string]http.HandlerFunc{"/select/jaeger/api/traces": func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []map[string]any `json:"data"`
		}{Data: []map[string]any{{"traceID": "1"}}})
	}})
	defer a.Close()
	b := fakeVT(t, map[string]http.HandlerFunc{"/select/jaeger/api/traces": func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []map[string]any `json:"data"`
		}{Data: []map[string]any{{"traceID": "2"}}})
	}})
	defer b.Close()
	log := logger.New("error")
	parent := NewVictoriaTracesService(config.VictoriaTracesConfig{Name: "parent"}, log)
	parent.SetChildren([]*VictoriaTracesService{
		NewVictoriaTracesService(config.VictoriaTracesConfig{Endpoints: []string{a.URL}, Timeout: 2000}, log),
		NewVictoriaTracesService(config.VictoriaTracesConfig{Endpoints: []string{b.URL}, Timeout: 2000}, log),
	})
	res, err := parent.SearchTraces(context.Background(), &models.TraceSearchRequest{Limit: 10})
	if err != nil {
		t.Fatalf("SearchTraces: %v", err)
	}
	if res.Total != 2 || len(res.Traces) != 2 {
		t.Fatalf("expected 2 traces, got total=%d len=%d", res.Total, len(res.Traces))
	}
}

func TestTraces_Aggregated_GetTrace_FirstFound(t *testing.T) {
	// A returns 404 (not found), B returns a valid trace
	a := fakeVT(t, map[string]http.HandlerFunc{"/select/jaeger/api/traces/": func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }})
	defer a.Close()
	b := fakeVT(t, map[string]http.HandlerFunc{"/select/jaeger/api/traces/": func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Data []models.Trace `json:"data"`
		}{Data: []models.Trace{{TraceID: "abc"}}})
	}})
	defer b.Close()
	log := logger.New("error")
	parent := NewVictoriaTracesService(config.VictoriaTracesConfig{Name: "parent"}, log)
	parent.SetChildren([]*VictoriaTracesService{
		NewVictoriaTracesService(config.VictoriaTracesConfig{Endpoints: []string{a.URL}, Timeout: 2000}, log),
		NewVictoriaTracesService(config.VictoriaTracesConfig{Endpoints: []string{b.URL}, Timeout: 2000}, log),
	})
	tr, err := parent.GetTrace(context.Background(), "abc", "")
	if err != nil {
		t.Fatalf("GetTrace: %v", err)
	}
	if tr == nil || tr.TraceID != "abc" {
		t.Fatalf("unexpected trace: %#v", tr)
	}
}
