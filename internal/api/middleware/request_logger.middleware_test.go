package middleware

import (
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestRequestLoggerWithBody_Captures200(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(RequestLoggerWithBody(logger.New("error")))
    r.POST("/echo", func(c *gin.Context){
        b, _ := io.ReadAll(c.Request.Body)
        c.String(200, string(b))
    })

    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/echo", io.NopCloser(strings.NewReader("hi")))
    r.ServeHTTP(w, req)
    if w.Code != 200 || w.Body.String() != "hi" {
        t.Fatalf("unexpected: %d %q", w.Code, w.Body.String())
    }
}

func TestIsSensitiveEndpoint(t *testing.T) {
    cases := map[string]bool{
        "/api/v1/auth/login":          true,
        "/api/v1/users/password":     true,
        "/api/v1/config/secrets":     true,
        "/some/other/path":           false,
    }
    for p, want := range cases {
        if got := isSensitiveEndpoint(p); got != want {
            t.Fatalf("path %s: want %v got %v", p, want, got)
        }
    }
}
