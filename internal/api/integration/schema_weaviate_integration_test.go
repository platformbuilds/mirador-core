package integration

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync"
    "testing"
    "time"
    "context"

    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/mirador-core/internal/api/handlers"
    "github.com/platformbuilds/mirador-core/internal/repo"
    storage_weaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
    "github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockWeaviate is a minimal in-memory stand-in for Weaviate used for integration tests
type mockWeaviate struct {
    mu      sync.Mutex
    classes map[string]bool
    // key: id, val: { class, properties }
    objects map[string]map[string]any
}

func newMockWeaviate() *mockWeaviate {
    return &mockWeaviate{classes: map[string]bool{}, objects: map[string]map[string]any{}}
}

func (m *mockWeaviate) handler() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/v1/.well-known/ready", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    mux.HandleFunc("/v1/schema", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            m.mu.Lock(); defer m.mu.Unlock()
            type cls struct{ Class string `json:"class"` }
            out := struct{ Classes []cls `json:"classes"` }{Classes: []cls{}}
            for c := range m.classes { out.Classes = append(out.Classes, cls{Class: c}) }
            _ = json.NewEncoder(w).Encode(out)
        case http.MethodPost:
            var in struct{ Class string `json:"class"` }
            _ = json.NewDecoder(r.Body).Decode(&in)
            m.mu.Lock(); m.classes[in.Class] = true; m.mu.Unlock()
            w.WriteHeader(200)
        default:
            w.WriteHeader(405)
        }
    })
    mux.HandleFunc("/v1/objects/", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPut { w.WriteHeader(405); return }
        var in struct {
            Class      string         `json:"class"`
            ID         string         `json:"id"`
            Properties map[string]any `json:"properties"`
        }
        body, _ := io.ReadAll(r.Body)
        _ = json.Unmarshal(body, &in)
        m.mu.Lock()
        if m.objects == nil { m.objects = map[string]map[string]any{} }
        if in.Properties == nil { in.Properties = map[string]any{} }
        if _, ok := m.objects[in.ID]; !ok { m.objects[in.ID] = map[string]any{} }
        m.objects[in.ID]["class"] = in.Class
        m.objects[in.ID]["properties"] = in.Properties
        m.mu.Unlock()
        w.WriteHeader(200)
    })
    mux.HandleFunc("/v1/graphql", func(w http.ResponseWriter, r *http.Request) {
        // naive handler that responds based on stored objects and simple query matching
        var in struct{ Query string `json:"query"` }
        _ = json.NewDecoder(r.Body).Decode(&in)
        q := in.Query
        m.mu.Lock(); defer m.mu.Unlock()
        // Build responses only for the classes used in tests
        if strings.Contains(q, "MetricDef(") {
            // Find first object with class MetricDef matching tenantId+metric
            var out struct{ Data struct{ Get struct{ MetricDef []map[string]any } } }
            out.Data.Get.MetricDef = []map[string]any{}
            for _, obj := range m.objects {
                if obj["class"] == "MetricDef" {
                    props := obj["properties"].(map[string]any)
                    out.Data.Get.MetricDef = append(out.Data.Get.MetricDef, map[string]any{
                        "tenantId":    props["tenantId"],
                        "metric":      props["metric"],
                        "description": props["description"],
                        "owner":       props["owner"],
                        "tags":        props["tags"],
                        "updatedAt":   props["updatedAt"],
                    })
                }
            }
            _ = json.NewEncoder(w).Encode(out)
            return
        }
        if strings.Contains(q, "MetricLabelDef(") {
            var out struct{ Data struct{ Get struct{ MetricLabelDef []map[string]any } } }
            out.Data.Get.MetricLabelDef = []map[string]any{}
            for _, obj := range m.objects {
                if obj["class"] == "MetricLabelDef" {
                    props := obj["properties"].(map[string]any)
                    out.Data.Get.MetricLabelDef = append(out.Data.Get.MetricLabelDef, map[string]any{
                        "label":         props["label"],
                        "type":          props["type"],
                        "description":   props["description"],
                        "allowedValues": props["allowedValues"],
                        "required":      props["required"],
                    })
                }
            }
            _ = json.NewEncoder(w).Encode(out)
            return
        }
        if strings.Contains(q, "MetricDefVersion(") {
            // Return one version row if exists
            var out struct{ Data struct{ Get struct{ MetricDefVersion []map[string]any } } }
            out.Data.Get.MetricDefVersion = []map[string]any{}
            for _, obj := range m.objects {
                if obj["class"] == "MetricDefVersion" {
                    props := obj["properties"].(map[string]any)
                    out.Data.Get.MetricDefVersion = append(out.Data.Get.MetricDefVersion, map[string]any{
                        "version":   props["version"],
                        "author":    props["author"],
                        "createdAt": props["createdAt"],
                        "payload":   props["payload"],
                    })
                }
            }
            _ = json.NewEncoder(w).Encode(out)
            return
        }
        if strings.Contains(q, "LogFieldDef(") {
            var out struct{ Data struct{ Get struct{ LogFieldDef []map[string]any } } }
            out.Data.Get.LogFieldDef = []map[string]any{}
            for _, obj := range m.objects {
                if obj["class"] == "LogFieldDef" {
                    props := obj["properties"].(map[string]any)
                    out.Data.Get.LogFieldDef = append(out.Data.Get.LogFieldDef, map[string]any{
                        "field":       props["field"],
                        "type":        props["type"],
                        "description": props["description"],
                        "tags":        props["tags"],
                        "examples":    props["examples"],
                        "updatedAt":   props["updatedAt"],
                    })
                }
            }
            _ = json.NewEncoder(w).Encode(out)
            return
        }
        // Default empty
        _ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"Get": map[string]any{}}})
    })
    return mux
}

func TestSchemaRoutes_WithMockWeaviate(t *testing.T) {
    gin.SetMode(gin.TestMode)
    m := newMockWeaviate()
    srv := httptest.NewServer(m.handler())
    defer srv.Close()

    // Client and repo
    c := &storage_weaviate.Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 2 * time.Second}}
    r := repo.NewWeaviateRepo(c)
    if err := r.EnsureSchema(context.Background()); err != nil {
        t.Fatalf("ensure schema failed: %v", err)
    }

    // Wire a minimal router with only schema routes we need
    log := logger.New("error")
    cch := cache.NewNoopValkeyCache(log)
    sh := handlers.NewSchemaHandler(r, nil, nil, cch, log, 1<<20)
    router := gin.New()
    v1 := router.Group("/api/v1")
    // Inject tenant id context for tests
    tenant := "t1"
    v1.Use(func(c *gin.Context){ c.Set("tenant_id", tenant); c.Next() })
    v1.POST("/schema/metrics", sh.UpsertMetric)
    v1.GET("/schema/metrics/:metric", sh.GetMetric)
    v1.POST("/schema/logs/fields", sh.UpsertLogField)
    v1.GET("/schema/logs/fields/:field", sh.GetLogField)

    // Upsert metric
    payload := map[string]any{
        "tenantId":    tenant,
        "metric":      "http_requests_total",
        "description": "http requests",
        "owner":       "team-a",
        "tags":        map[string]any{"app": "web"},
        "author":      "tester",
    }
    body, _ := json.Marshal(payload)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/schema/metrics", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("upsert metric code=%d body=%s", w.Code, w.Body.String())
    }

    // Get metric
    req2 := httptest.NewRequest(http.MethodGet, "/api/v1/schema/metrics/http_requests_total", nil)
    w2 := httptest.NewRecorder()
    router.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK {
        t.Fatalf("get metric code=%d body=%s", w2.Code, w2.Body.String())
    }
    var got map[string]any
    _ = json.Unmarshal(w2.Body.Bytes(), &got)
    if got["metric"] != "http_requests_total" {
        t.Fatalf("unexpected metric payload: %v", got)
    }

    // Upsert log field
    lf := map[string]any{
        "tenantId":    tenant,
        "field":       "trace_id",
        "type":        "string",
        "description": "trace id",
        "author":      "tester",
    }
    lfb, _ := json.Marshal(lf)
    req3 := httptest.NewRequest(http.MethodPost, "/api/v1/schema/logs/fields", bytes.NewReader(lfb))
    req3.Header.Set("Content-Type", "application/json")
    w3 := httptest.NewRecorder()
    router.ServeHTTP(w3, req3)
    if w3.Code != http.StatusOK {
        t.Fatalf("upsert log field code=%d body=%s", w3.Code, w3.Body.String())
    }

    // Get log field
    req4 := httptest.NewRequest(http.MethodGet, "/api/v1/schema/logs/fields/trace_id", nil)
    w4 := httptest.NewRecorder()
    router.ServeHTTP(w4, req4)
    if w4.Code != http.StatusOK {
        t.Fatalf("get log field code=%d body=%s", w4.Code, w4.Body.String())
    }
}
