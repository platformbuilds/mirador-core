package handlers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/miradorstack/internal/config"
    "github.com/platformbuilds/miradorstack/internal/models"
    "github.com/platformbuilds/miradorstack/internal/services"
    "github.com/platformbuilds/miradorstack/pkg/cache"
    "github.com/platformbuilds/miradorstack/pkg/logger"
    "github.com/stretchr/testify/assert"
    "context"
)

type memCache struct{ m map[string][]byte }

func (c *memCache) Get(ctx context.Context, key string) ([]byte, error)                                  { return nil, fmt.Errorf("miss") }
func (c *memCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error       { return nil }
func (c *memCache) Delete(ctx context.Context, key string) error                                          { return nil }
func (c *memCache) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error)         { return nil, nil }
func (c *memCache) SetSession(ctx context.Context, session *models.UserSession) error                     { return nil }
func (c *memCache) InvalidateSession(ctx context.Context, sessionID string) error                         { return nil }
func (c *memCache) GetActiveSessions(ctx context.Context, tenantID string) ([]*models.UserSession, error) { return nil, nil }
func (c *memCache) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
    if c.m == nil { c.m = map[string][]byte{} }
    b, _ := json.Marshal(result)
    c.m[queryHash] = b
    return nil
}
func (c *memCache) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
    if c.m == nil { return nil, fmt.Errorf("empty") }
    b, ok := c.m[queryHash]
    if !ok { return nil, fmt.Errorf("miss") }
    return b, nil
}

func metricsServer() *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch {
        case r.URL.Path == "/select/0/prometheus/api/v1/query":
            w.Header().Set("Content-Type", "application/json")
            _, _ = w.Write([]byte(`{"status":"success","data":{"result":[]}}`))
        case r.URL.Path == "/select/0/prometheus/api/v1/query_range":
            w.Header().Set("Content-Type", "application/json")
            _, _ = w.Write([]byte(`{"status":"success","data":{"result":[]}}`))
        case r.URL.Path == "/select/0/prometheus/api/v1/series":
            _, _ = w.Write([]byte(`{"status":"success","data":[{"__name__":"up"}]}`))
        case r.URL.Path == "/select/0/prometheus/api/v1/labels":
            _, _ = w.Write([]byte(`{"status":"success","data":["job","instance"]}`))
        default:
            if len(r.URL.Path) > 0 && r.URL.Path[:21] == "/select/0/prometheus/" {
                // label values; accept anything
                _, _ = w.Write([]byte(`{"status":"success","data":["v1","v2"]}`))
                return
            }
            http.NotFound(w, r)
        }
    }))
}

func TestMetricsQL_ExecuteQuery_CacheFlow(t *testing.T) {
    gin.SetMode(gin.TestMode)
    srv := metricsServer(); defer srv.Close()
    svc := services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{srv.URL}, Timeout: 500}, logger.New("error"))
    h := NewMetricsQLHandler(svc, cache.ValkeyCluster(&memCache{}), logger.New("error"))

    payload := map[string]any{"query": "up"}
    b, _ := json.Marshal(payload)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/query", bytes.NewReader(b))
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    c.Set("tenant_id", "t1")
    h.ExecuteQuery(c)
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsQL_ExecuteQuery_BadRequestOnEmptyQuery(t *testing.T) {
    gin.SetMode(gin.TestMode)
    srv := metricsServer(); defer srv.Close()
    svc := services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{srv.URL}, Timeout: 500}, logger.New("error"))
    h := NewMetricsQLHandler(svc, cache.ValkeyCluster(&memCache{}), logger.New("error"))

    payload := map[string]any{"query": ""}
    b, _ := json.Marshal(payload)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/query", bytes.NewReader(b))
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    h.ExecuteQuery(c)
    assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMetricsQL_ExecuteRangeQuery_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    srv := metricsServer(); defer srv.Close()
    svc := services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{srv.URL}, Timeout: 500}, logger.New("error"))
    h := NewMetricsQLHandler(svc, cache.ValkeyCluster(&memCache{}), logger.New("error"))

    payload := map[string]any{"query": "up", "start": "1690000000", "end": "1690003600", "step": "30"}
    b, _ := json.Marshal(payload)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/query_range", bytes.NewReader(b))
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    h.ExecuteRangeQuery(c)
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsQL_GetSeries_BadRequestNoMatch(t *testing.T) {
    gin.SetMode(gin.TestMode)
    srv := metricsServer(); defer srv.Close()
    svc := services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{srv.URL}, Timeout: 500}, logger.New("error"))
    h := NewMetricsQLHandler(svc, cache.ValkeyCluster(&memCache{}), logger.New("error"))

    req := httptest.NewRequest(http.MethodGet, "/api/v1/series", nil)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    h.GetSeries(c)
    assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMetricsQL_GetLabels_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    srv := metricsServer(); defer srv.Close()
    svc := services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{srv.URL}, Timeout: 500}, logger.New("error"))
    h := NewMetricsQLHandler(svc, cache.ValkeyCluster(&memCache{}), logger.New("error"))

    req := httptest.NewRequest(http.MethodGet, "/api/v1/labels?start=1&end=2&match[]=up", nil)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    h.GetLabels(c)
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsQL_GetLabelValues_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    srv := metricsServer(); defer srv.Close()
    svc := services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{srv.URL}, Timeout: 500}, logger.New("error"))
    h := NewMetricsQLHandler(svc, cache.ValkeyCluster(&memCache{}), logger.New("error"))

    req := httptest.NewRequest(http.MethodGet, "/api/v1/label/job/values?start=1&end=2&match[]=up", nil)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Params = append(c.Params, gin.Param{Key: "name", Value: "job"})
    c.Request = req
    h.GetLabelValues(c)
    assert.Equal(t, http.StatusOK, w.Code)
}
