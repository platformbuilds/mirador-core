package integration

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/mirador-core/internal/api/handlers"
    "github.com/platformbuilds/mirador-core/internal/repo"
    storage_weaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
    "github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestSchema_VersionsAndLabels_WithMockWeaviate(t *testing.T) {
    gin.SetMode(gin.TestMode)
    m := newMockWeaviate()
    srv := httptest.NewServer(m.handler())
    defer srv.Close()

    c := &storage_weaviate.Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 2 * time.Second}}
    r := repo.NewWeaviateRepo(c)
    if err := r.EnsureSchema(context.Background()); err != nil {
        t.Fatalf("ensure schema failed: %v", err)
    }

    log := logger.New("error")
    cch := cache.NewNoopValkeyCache(log)
    sh := handlers.NewSchemaHandler(r, nil, nil, cch, log, 1<<20)
    router := gin.New()
    v1 := router.Group("/api/v1")
    v1.Use(func(c *gin.Context){ c.Set("tenant_id", "t1"); c.Next() })
    v1.POST("/schema/metrics", sh.UpsertMetric)
    v1.GET("/schema/metrics/:metric/versions", sh.ListMetricVersions)
    v1.GET("/schema/metrics/:metric/versions/:version", sh.GetMetricVersion)

    // Upsert metric twice to create two versions
    up := func(desc string) {
        payload := map[string]any{"tenantId":"t1","metric":"cpu_usage","description":desc,"owner":"team","tags":map[string]any{"env":"dev"},"author":"tester"}
        b,_ := json.Marshal(payload)
        req := httptest.NewRequest(http.MethodPost, "/api/v1/schema/metrics", bytes.NewReader(b))
        req.Header.Set("Content-Type","application/json")
        w := httptest.NewRecorder(); router.ServeHTTP(w, req)
        if w.Code != http.StatusOK { t.Fatalf("upsert metric failed: %d %s", w.Code, w.Body.String()) }
    }
    up("v1"); up("v2")

    // List versions
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/v1/schema/metrics/cpu_usage/versions", nil)
    router.ServeHTTP(w, req)
    if w.Code != http.StatusOK { t.Fatalf("list versions failed: %d %s", w.Code, w.Body.String()) }

    // Get version 1
    w2 := httptest.NewRecorder()
    req2 := httptest.NewRequest(http.MethodGet, "/api/v1/schema/metrics/cpu_usage/versions/1", nil)
    router.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK { t.Fatalf("get version failed: %d %s", w2.Code, w2.Body.String()) }

    // Labels: upsert one label and fetch definitions
    if err := r.UpsertMetricLabel(context.Background(), "t1", "cpu_usage", "host", "string", true, map[string]any{"enum": []string{"h1","h2"}}, "hostname"); err != nil {
        t.Fatalf("upsert label failed: %v", err)
    }
    defs, err := r.GetMetricLabelDefs(context.Background(), "t1", "cpu_usage", []string{"host"})
    if err != nil || defs["host"] == nil { t.Fatalf("get label defs failed: %v", err) }
}

