package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/platformbuilds/mirador-core/internal/config"
)

func TestExtractToken_Sources(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x?token=qt", http.NoBody)
	if got := extractToken(c); got != "" {
		t.Fatalf("query token should be rejected, got %q", got)
	}

	c.Request = httptest.NewRequest(http.MethodGet, "/x", http.NoBody)
	c.Request.Header.Set("X-Session-Token", "xs")
	if got := extractToken(c); got != "xs" {
		t.Fatalf("x-session got %q", got)
	}

	c.Request = httptest.NewRequest(http.MethodGet, "/x", http.NoBody)
	c.Request.Header.Set("Authorization", "Bearer abcd")
	if got := extractToken(c); got != "abcd" {
		t.Fatalf("auth got %q", got)
	}
}

func TestRequireTenant_Enforces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireTenant())
	r.GET("/t", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/t", http.NoBody))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestValidateJWTToken_OK(t *testing.T) {
	// Create a valid HMAC token compatible with validateJWTToken
	key := []byte("secret123")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":    "u1",
		"tenant": "t1",
		"roles":  []string{"viewer", "admin"},
		"email":  "u1@example.com",
		"name":   "U One",
	})
	s, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	cfg := config.AuthConfig{}
	cfg.OAuth.Enabled = true
	cfg.JWT.Secret = string(key)

	sess, err := validateJWTToken(s, cfg)
	if err != nil {
		t.Fatalf("validate jwt: %v", err)
	}
	if sess.UserID != "u1" || sess.TenantID != "t1" {
		t.Fatalf("unexpected session: %+v", sess)
	}
	if len(sess.Roles) == 0 {
		t.Fatalf("expected roles")
	}
}
