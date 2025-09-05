package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/miradorstack/internal/config"
    "github.com/platformbuilds/miradorstack/internal/services"
    "github.com/platformbuilds/miradorstack/pkg/cache"
    "github.com/platformbuilds/miradorstack/pkg/logger"
    "github.com/stretchr/testify/assert"
)

type noCache struct{ cache.ValkeyCluster }

func logsServer() *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch {
        case r.Method == http.MethodGet && r.URL.Path == "/select/logsql/api/v1/export":
            // stream a couple of json lines
            _, _ = w.Write([]byte("{\"a\":1}\n{\"b\":2}\n"))
        case r.Method == http.MethodPost && r.URL.Path == "/insert/jsonline":
            w.WriteHeader(http.StatusOK)
        case r.Method == http.MethodGet && r.URL.Path == "/select/logsql/labels":
            _, _ = w.Write([]byte(`["app","env"]`))
        case r.Method == http.MethodGet && r.URL.Path == "/select/logsql/field_names":
            _, _ = w.Write([]byte(`{"status":"success","data":["_msg","level"]}`))
        default:
            http.NotFound(w, r)
        }
    }))
}

func newLogsHandlerForTest(t *testing.T) (*LogsQLHandler, func()) {
    t.Helper()
    srv := logsServer()
    logs := services.NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{srv.URL}, Timeout: 500}, logger.New("error"))
    h := NewLogsQLHandler(logs, &noCache{}, logger.New("error"))
    return h, srv.Close
}

func TestLogs_ExecuteQuery_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h, closeFn := newLogsHandlerForTest(t)
    defer closeFn()

    body := map[string]any{"query": "_stream:app"}
    b, _ := json.Marshal(body)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/query", bytes.NewReader(b))
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    c.Set("tenant_id", "t1")
    h.ExecuteQuery(c)
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogs_ExecuteQuery_BadRequestOnEmptyQuery(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h, closeFn := newLogsHandlerForTest(t)
    defer closeFn()

    body := map[string]any{"query": ""}
    b, _ := json.Marshal(body)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/query", bytes.NewReader(b))
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    h.ExecuteQuery(c)
    assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogs_StoreEvent_Created(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h, closeFn := newLogsHandlerForTest(t)
    defer closeFn()

    body := map[string]any{"type": "rca_correlation"}
    b, _ := json.Marshal(body)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/store", bytes.NewReader(b))
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    c.Set("tenant_id", "t1")
    h.StoreEvent(c)
    assert.Equal(t, http.StatusCreated, w.Code)
}

func TestLogs_GetStreams_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h, closeFn := newLogsHandlerForTest(t)
    defer closeFn()

    req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/streams?limit=10", nil)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    c.Set("tenant_id", "t1")
    h.GetStreams(c)
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogs_GetFields_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h, closeFn := newLogsHandlerForTest(t)
    defer closeFn()

    req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/fields", nil)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    c.Set("tenant_id", "t1")
    h.GetFields(c)
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogs_Export_OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h, closeFn := newLogsHandlerForTest(t)
    defer closeFn()

    body := map[string]any{"query": "_stream:app", "format": "json"}
    b, _ := json.Marshal(body)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/export", bytes.NewReader(b))
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    c.Set("tenant_id", "t1")
    h.ExportLogs(c)
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogs_Export_BadRequestMissingQuery(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h, closeFn := newLogsHandlerForTest(t)
    defer closeFn()

    body := map[string]any{"format": "json"} // missing required query
    b, _ := json.Marshal(body)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/export", bytes.NewReader(b))
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    h.ExportLogs(c)
    assert.Equal(t, http.StatusBadRequest, w.Code)
}
