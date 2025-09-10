package api

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/platformbuilds/mirador-core/internal/config"
    "github.com/platformbuilds/mirador-core/internal/grpc/clients"
    "github.com/platformbuilds/mirador-core/internal/repo"
    storage_weaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
    "github.com/platformbuilds/mirador-core/internal/services"
    "github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

type miniWeav struct{ objects map[string]map[string]any }

func (m *miniWeav) handler() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/v1/.well-known/ready", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    mux.HandleFunc("/v1/schema", func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(map[string]any{"classes": []any{}}) })
    mux.HandleFunc("/v1/objects/", func(w http.ResponseWriter, r *http.Request) {
        var in struct{ Class string `json:"class"`; ID string `json:"id"`; Properties map[string]any `json:"properties"` }
        _ = json.NewDecoder(r.Body).Decode(&in)
        if m.objects == nil { m.objects = map[string]map[string]any{} }
        if in.Properties == nil { in.Properties = map[string]any{} }
        m.objects[in.ID] = map[string]any{"class": in.Class, "properties": in.Properties}
        w.WriteHeader(200)
    })
    mux.HandleFunc("/v1/graphql", func(w http.ResponseWriter, r *http.Request) {
        var in struct{ Query string `json:"query"` }
        _ = json.NewDecoder(r.Body).Decode(&in)
        if bytes.Contains([]byte(in.Query), []byte("MetricDef(")) {
            arr := []map[string]any{}
            for _, o := range m.objects {
                if o["class"] == "MetricDef" { arr = append(arr, o["properties"].(map[string]any)) }
            }
            _ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"Get": map[string]any{"MetricDef": arr}}})
            return
        }
        _ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"Get": map[string]any{}}})
    })
    return mux
}

func TestServerE2E_SchemaMetric(t *testing.T) {
    m := &miniWeav{}
    ts := httptest.NewServer(m.handler())
    defer ts.Close()

    logg := logger.New("error")
    cch := cache.NewNoopValkeyCache(logg)
    vm := &services.VictoriaMetricsServices{}
    grpc := &clients.GRPCClients{}

    cfg := &config.Config{}
    cfg.Port = 0
    cfg.Weaviate.Enabled = true
    cfg.Weaviate.Scheme = "http"
    cfg.Weaviate.Host = ts.URL[len("http://"):]
    cfg.Weaviate.Port = 80

    cli := &storage_weaviate.Client{BaseURL: ts.URL, HTTP: &http.Client{Timeout: 2 * time.Second}}
    rep := repo.NewWeaviateRepo(cli)
    _ = rep.EnsureSchema(context.Background())

    s := NewServer(cfg, logg, cch, grpc, vm, rep)

    httpSrv := httptest.NewServer(s.router)
    defer httpSrv.Close()

    payload := map[string]any{"tenantId": "t1", "metric": "requests_total", "description": "desc", "owner": "team", "tags": map[string]any{"app": "api"}, "author": "tester"}
    b, _ := json.Marshal(payload)
    resp, err := http.Post(httpSrv.URL+"/api/v1/schema/metrics", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatalf("post failed: %v", err) }
    if resp.StatusCode != 200 { t.Fatalf("unexpected status: %s", resp.Status) }

    r2, err := http.Get(httpSrv.URL+"/api/v1/schema/metrics/requests_total")
    if err != nil { t.Fatalf("get failed: %v", err) }
    if r2.StatusCode != 200 { t.Fatalf("unexpected status: %s", r2.Status) }
}

