package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNoAuthMiddleware_DefaultsAndOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoAuthMiddleware())
	r.GET("/ping", func(c *gin.Context) {
		// echo back context values
		c.String(200, c.GetString("tenant_id")+","+c.GetString("user_id"))
	})

	// default tenant
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w.Code != 200 || w.Body.String() != "default,anonymous" {
		t.Fatalf("unexpected resp: %d %q", w.Code, w.Body.String())
	}

	// header override
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Tenant-ID", "t1")
	r.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != "t1,anonymous" {
		t.Fatalf("override resp: %d %q", w.Code, w.Body.String())
	}
}
