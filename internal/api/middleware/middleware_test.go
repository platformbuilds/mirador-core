package middleware

import (
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/mirador-core/internal/config"
    "github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestCORS_IsOriginAllowed(t *testing.T) {
    allowed := []string{"https://a.example.com", "https://b.example.com"}
    if !isOriginAllowed("https://a.example.com", allowed) {
        t.Fatalf("expected origin allowed")
    }
    if isOriginAllowed("https://x.example.com", allowed) {
        t.Fatalf("unexpected origin allowed")
    }
}

func TestRateLimiter_AppliesHeaders(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    log := logger.New("error")
    cch := cache.NewNoopValkeyCache(log)
    r.Use(RateLimiter(cch))
    r.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
    w := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/ping", nil)
    r.ServeHTTP(w, req)
    if w.Code != 200 { t.Fatalf("unexpected status: %d", w.Code) }
    if w.Header().Get("X-Rate-Limit-Remaining") == "" {
        t.Fatalf("missing rate limit header")
    }
}

func TestAuth_PublicEndpointBypasses(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    log := logger.New("error")
    cch := cache.NewNoopValkeyCache(log)
    r.Use(AuthMiddleware(config.AuthConfig{Enabled: false}, cch))
    r.GET("/health", func(c *gin.Context) { c.String(200, "ok") })
    w := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/health", nil)
    r.ServeHTTP(w, req)
    if w.Code != 200 { t.Fatalf("unexpected status: %d", w.Code) }
}

func TestRBAC_HasAnyRole(t *testing.T) {
    if !hasAnyRole([]string{"user","admin"}, []string{"mirador-admin","admin"}) {
        t.Fatalf("expected role match")
    }
    if hasAnyRole([]string{"user"}, []string{"admin"}) {
        t.Fatalf("unexpected role match")
    }
}
