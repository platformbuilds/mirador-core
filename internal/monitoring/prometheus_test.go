package monitoring

import (
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
)

func TestSetupPrometheusMetrics(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    SetupPrometheusMetrics(r)
    w := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/metrics", nil)
    r.ServeHTTP(w, req)
    if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
}

