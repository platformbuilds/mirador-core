package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockSchemaStore implements the SchemaStore interface for testing
type mockSchemaStore struct{}

func (m *mockSchemaStore) UpsertMetric(ctx context.Context, metric repo.MetricDef, author string) error {
	return nil
}
func (m *mockSchemaStore) GetMetric(ctx context.Context, tenantID, metric string) (*repo.MetricDef, error) {
	return nil, nil
}
func (m *mockSchemaStore) ListMetricVersions(ctx context.Context, tenantID, metric string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockSchemaStore) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockSchemaStore) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	return nil
}
func (m *mockSchemaStore) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
	return nil, nil
}
func (m *mockSchemaStore) UpsertLogField(ctx context.Context, f repo.LogFieldDef, author string) error {
	return nil
}
func (m *mockSchemaStore) GetLogField(ctx context.Context, tenantID, field string) (*repo.LogFieldDef, error) {
	return nil, nil
}
func (m *mockSchemaStore) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockSchemaStore) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockSchemaStore) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (m *mockSchemaStore) GetTraceService(ctx context.Context, tenantID, service string) (*repo.TraceServiceDef, error) {
	return nil, nil
}
func (m *mockSchemaStore) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockSchemaStore) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockSchemaStore) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (m *mockSchemaStore) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*repo.TraceOperationDef, error) {
	return nil, nil
}
func (m *mockSchemaStore) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockSchemaStore) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockSchemaStore) UpsertLabel(ctx context.Context, tenantID, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	return nil
}
func (m *mockSchemaStore) GetLabel(ctx context.Context, tenantID, name string) (*repo.LabelDef, error) {
	return nil, nil
}
func (m *mockSchemaStore) ListLabelVersions(ctx context.Context, tenantID, name string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (m *mockSchemaStore) GetLabelVersion(ctx context.Context, tenantID, name string, version int64) (map[string]any, repo.VersionInfo, error) {
	return nil, repo.VersionInfo{}, nil
}
func (m *mockSchemaStore) DeleteLabel(ctx context.Context, tenantID, name string) error { return nil }
func (m *mockSchemaStore) DeleteMetric(ctx context.Context, tenantID, metric string) error {
	return nil
}
func (m *mockSchemaStore) DeleteLogField(ctx context.Context, tenantID, field string) error {
	return nil
}
func (m *mockSchemaStore) DeleteTraceService(ctx context.Context, tenantID, service string) error {
	return nil
}
func (m *mockSchemaStore) DeleteTraceOperation(ctx context.Context, tenantID, service, operation string) error {
	return nil
}
func (m *mockSchemaStore) UpsertSchemaAsKPI(ctx context.Context, schemaDef *models.SchemaDefinition, author string) error {
	return nil
}
func (m *mockSchemaStore) GetSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) (*models.SchemaDefinition, error) {
	return &models.SchemaDefinition{ID: id, Type: models.SchemaType(schemaType), TenantID: tenantID}, nil
}
func (m *mockSchemaStore) ListSchemasAsKPIs(ctx context.Context, tenantID, schemaType string, limit, offset int) ([]*models.SchemaDefinition, int, error) {
	return []*models.SchemaDefinition{}, 0, nil
}
func (m *mockSchemaStore) DeleteSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) error {
	return nil
}

// mockRBACRepository implements the RBACRepository interface for testing
type mockRBACRepository struct{}

func (m *mockRBACRepository) CreateRole(ctx context.Context, role *models.Role) error { return nil }
func (m *mockRBACRepository) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	if roleName == "viewer" {
		return &models.Role{
			Name:        "viewer",
			TenantID:    tenantID,
			Description: "Read-only access",
			Permissions: []string{"metrics:read:tenant"},
		}, nil
	}
	return nil, nil
}
func (m *mockRBACRepository) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return []*models.Role{}, nil
}
func (m *mockRBACRepository) UpdateRole(ctx context.Context, role *models.Role) error { return nil }
func (m *mockRBACRepository) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepository) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return []string{}, nil
}
func (m *mockRBACRepository) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepository) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepository) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return []*models.Permission{}, nil
}
func (m *mockRBACRepository) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepository) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	return nil
}
func (m *mockRBACRepository) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepository) GetRoleBindings(ctx context.Context, tenantID string, filters rbac.RoleBindingFilters) ([]*models.RoleBinding, error) {
	return []*models.RoleBinding{}, nil
}
func (m *mockRBACRepository) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepository) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	return nil
}
func (m *mockRBACRepository) CreateGroup(ctx context.Context, group *models.Group) error { return nil }
func (m *mockRBACRepository) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	return []*models.Group{}, nil
}
func (m *mockRBACRepository) UpdateGroup(ctx context.Context, group *models.Group) error { return nil }
func (m *mockRBACRepository) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	return nil
}
func (m *mockRBACRepository) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepository) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepository) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	return []string{}, nil
}
func (m *mockRBACRepository) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepository) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return []string{}, nil
}
func (m *mockRBACRepository) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return []*models.AuditLog{}, nil
}
func (m *mockRBACRepository) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepository) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return []*models.Tenant{}, nil
}
func (m *mockRBACRepository) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepository) DeleteTenant(ctx context.Context, tenantID string) error { return nil }
func (m *mockRBACRepository) CreateUser(ctx context.Context, user *models.User) error { return nil }
func (m *mockRBACRepository) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return []*models.User{}, nil
}
func (m *mockRBACRepository) UpdateUser(ctx context.Context, user *models.User) error { return nil }
func (m *mockRBACRepository) DeleteUser(ctx context.Context, userID string) error     { return nil }
func (m *mockRBACRepository) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepository) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return []*models.TenantUser{}, nil
}
func (m *mockRBACRepository) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepository) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockRBACRepository) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepository) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	return nil, nil
}
func (m *mockRBACRepository) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepository) DeleteMiradorAuth(ctx context.Context, userID string) error { return nil }
func (m *mockRBACRepository) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepository) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepository) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepository) DeleteAuthConfig(ctx context.Context, tenantID string) error { return nil }

// mockCacheRepository implements the CacheRepository interface for testing
type mockCacheRepository struct{}

func (m *mockCacheRepository) SetRole(ctx context.Context, tenantID, roleName string, role *models.Role, ttl time.Duration) error {
	return nil
}
func (m *mockCacheRepository) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockCacheRepository) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockCacheRepository) InvalidateTenantRoles(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockCacheRepository) SetUserRoles(ctx context.Context, tenantID, userID string, roles []string, ttl time.Duration) error {
	return nil
}
func (m *mockCacheRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockCacheRepository) DeleteUserRoles(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockCacheRepository) InvalidateUserRoles(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockCacheRepository) SetPermissions(ctx context.Context, tenantID string, permissions []*models.Permission, ttl time.Duration) error {
	return nil
}
func (m *mockCacheRepository) GetPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return []*models.Permission{}, nil
}
func (m *mockCacheRepository) InvalidatePermissions(ctx context.Context, tenantID string) error {
	return nil
}

func newTestRouterWithContext(cch cache.ValkeyCluster, mw func(*gin.Context)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(mw)
	log := logger.New("error")

	// Create dynamic config service and grpc clients for the config handler
	dynamicConfig := services.NewDynamicConfigService(cch, log)
	grpcClients, _ := clients.NewGRPCClients(&config.Config{Environment: "development"}, log, dynamicConfig)

	// Create RBAC service dependencies
	mockRepo := &mockRBACRepository{}
	mockCacheRepo := &mockCacheRepository{}
	auditService := rbac.NewAuditService(mockRepo)
	rbacService := rbac.NewRBACService(mockRepo, mockCacheRepo, auditService)

	// RBAC
	rbacHandler := NewRBACHandler(rbacService, cch, log)
	v1.GET("/rbac/roles", rbacHandler.GetRoles)
	v1.POST("/rbac/roles", rbacHandler.CreateRole)
	v1.PUT("/rbac/users/:userId/roles", rbacHandler.AssignUserRoles)

	// Sessions
	sess := NewSessionHandler(cch, log)
	v1.GET("/sessions/active", sess.GetActiveSessions)
	v1.POST("/sessions/invalidate", sess.InvalidateSession)
	v1.GET("/sessions/user/:userId", sess.GetUserSessions)

	// Config
	cfg := NewConfigHandler(cch, log, dynamicConfig, grpcClients, &mockSchemaStore{})
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
	_ = cch.SetSession(context.TODO(), sess)

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
