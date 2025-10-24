package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func newTestRouterWithContext(cch cache.ValkeyCluster, mw func(*gin.Context)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(mw)
	log := logger.New("error")

	// Create dynamic config service and grpc clients for the config handler
	dynamicConfig := services.NewDynamicConfigService(cch, log)
	grpcClients, _ := clients.NewGRPCClients(&config.Config{Environment: "development"}, log, dynamicConfig)

	// RBAC
	rbac := NewRBACHandler(cch, log)
	v1.GET("/rbac/roles", rbac.GetRoles)
	v1.POST("/rbac/roles", rbac.CreateRole)
	v1.PUT("/rbac/users/:userId/roles", rbac.AssignUserRoles)

	// Sessions
	sess := NewSessionHandler(cch, log)
	v1.GET("/sessions/active", sess.GetActiveSessions)
	v1.POST("/sessions/invalidate", sess.InvalidateSession)
	v1.GET("/sessions/user/:userId", sess.GetUserSessions)

	// Config
	cfg := NewConfigHandler(cch, log, dynamicConfig, grpcClients)
	v1.GET("/config/user-settings", cfg.GetUserSettings)
	v1.PUT("/config/user-settings", cfg.UpdateUserSettings)
	v1.GET("/config/datasources", cfg.GetDataSources)
	v1.POST("/config/datasources", cfg.AddDataSource)
	v1.GET("/config/integrations", cfg.GetIntegrations)

	return r
}

func TestRBAC_Session_Config(t *testing.T) {
	log := logger.New("error")
	cch := cache.NewNoopValkeyCache(log)
	// Preseed a session
	sess := &models.UserSession{ID: "tok1", TenantID: "t1", UserID: "u1", Settings: map[string]any{"theme": "light"}}
	_ = cch.SetSession(nil, sess)

	r := newTestRouterWithContext(cch, func(c *gin.Context) {
		c.Set("tenant_id", "t1")
		c.Set("user_id", "u1")
		c.Set("session_id", "tok1")
		c.Next()
	})

	// RBAC list
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/rbac/roles", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("rbac roles status=%d", w.Code)
	}

	// RBAC create invalid
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/rbac/roles", bytes.NewReader([]byte("{}"))))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("rbac create invalid=%d", w.Code)
	}

	// RBAC assign
	body, _ := json.Marshal(map[string]any{"roles": []string{"viewer"}})
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/rbac/users/u1/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("rbac assign=%d body=%s", w.Code, w.Body.String())
	}

	// Sessions active
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/sessions/active", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("sessions active=%d", w.Code)
	}

	// Sessions user
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/sessions/user/u1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("sessions user=%d", w.Code)
	}

	// Sessions invalidate with session in context should succeed
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/invalidate", bytes.NewReader([]byte("{}"))))
	if w.Code != http.StatusOK {
		t.Fatalf("invalidate with session should 200, got=%d", w.Code)
	}

	// Sessions invalidate truly missing token (no session_id in context)
	r2 := newTestRouterWithContext(cache.NewNoopValkeyCache(log), func(c *gin.Context) { c.Set("tenant_id", "t1"); c.Set("user_id", "u1"); c.Next() })
	w = httptest.NewRecorder()
	r2.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/invalidate", bytes.NewReader([]byte("{}"))))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalidate missing should 400, got=%d", w.Code)
	}

	// Config get user settings (uses session)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config/user-settings", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("get settings=%d", w.Code)
	}

	// Config update settings
	upd, _ := json.Marshal(map[string]any{"theme": "dark"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/config/user-settings", bytes.NewReader(upd))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update settings=%d body=%s", w.Code, w.Body.String())
	}

	// Config list datasources
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config/datasources", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("datasources=%d", w.Code)
	}

	// Config add datasource
	ds, _ := json.Marshal(map[string]any{"name": "vm2", "type": "metrics", "url": "http://vm2"})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/datasources", bytes.NewReader(ds))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("add datasource=%d", w.Code)
	}

	// Config integrations
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config/integrations", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("integrations=%d", w.Code)
	}
}
