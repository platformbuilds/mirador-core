package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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

// mockRBACRepositoryForServerCoverTest implements RBACRepository for testing
type mockRBACRepositoryForServerCoverTest struct{}

func (m *mockRBACRepositoryForServerCoverTest) CreateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetRoleBindings(ctx context.Context, tenantID string, filters rbac.RoleBindingFilters) ([]*models.RoleBinding, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) CreateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeleteTenant(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) CreateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeleteUser(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeleteMiradorAuth(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerCoverTest) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForServerCoverTest) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	return nil
}

// stubSchemaRepo is a minimal implementation to make schema routes register.
type stubSchemaRepo struct{}

func (stubSchemaRepo) UpsertMetric(ctx context.Context, m repo.MetricDef, author string) error {
	return nil
}
func (stubSchemaRepo) GetMetric(ctx context.Context, tenantID, metric string) (*repo.MetricDef, error) {
	return &repo.MetricDef{TenantID: tenantID, Metric: metric}, nil
}
func (stubSchemaRepo) ListMetricVersions(ctx context.Context, tenantID, metric string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetMetricVersion(ctx context.Context, tenantID, metric string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) UpsertMetricLabel(ctx context.Context, tenantID, metric, label, typ string, required bool, allowed map[string]any, description string) error {
	return nil
}
func (stubSchemaRepo) GetMetricLabelDefs(ctx context.Context, tenantID, metric string, labels []string) (map[string]*repo.MetricLabelDef, error) {
	return map[string]*repo.MetricLabelDef{}, nil
}
func (stubSchemaRepo) UpsertLogField(ctx context.Context, f repo.LogFieldDef, author string) error {
	return nil
}
func (stubSchemaRepo) GetLogField(ctx context.Context, tenantID, field string) (*repo.LogFieldDef, error) {
	return &repo.LogFieldDef{TenantID: tenantID, Field: field}, nil
}
func (stubSchemaRepo) ListLogFieldVersions(ctx context.Context, tenantID, field string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetLogFieldVersion(ctx context.Context, tenantID, field string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) UpsertLabel(ctx context.Context, tenantID, name, typ string, required bool, allowed map[string]any, description, category, sentiment, author string) error {
	return nil
}
func (stubSchemaRepo) GetLabel(ctx context.Context, tenantID, name string) (*repo.LabelDef, error) {
	return &repo.LabelDef{Name: name, TenantID: tenantID}, nil
}
func (stubSchemaRepo) ListLabelVersions(ctx context.Context, tenantID, name string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetLabelVersion(ctx context.Context, tenantID, name string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) DeleteLabel(ctx context.Context, tenantID, name string) error { return nil }
func (stubSchemaRepo) UpsertTraceServiceWithAuthor(ctx context.Context, tenantID, service, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (stubSchemaRepo) GetTraceService(ctx context.Context, tenantID, service string) (*repo.TraceServiceDef, error) {
	return &repo.TraceServiceDef{TenantID: tenantID, Service: service}, nil
}
func (stubSchemaRepo) ListTraceServiceVersions(ctx context.Context, tenantID, service string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetTraceServiceVersion(ctx context.Context, tenantID, service string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) UpsertTraceOperationWithAuthor(ctx context.Context, tenantID, service, operation, servicePurpose, owner, category, sentiment string, tags []string, author string) error {
	return nil
}
func (stubSchemaRepo) GetTraceOperation(ctx context.Context, tenantID, service, operation string) (*repo.TraceOperationDef, error) {
	return &repo.TraceOperationDef{TenantID: tenantID, Service: service, Operation: operation}, nil
}
func (stubSchemaRepo) ListTraceOperationVersions(ctx context.Context, tenantID, service, operation string) ([]repo.VersionInfo, error) {
	return nil, nil
}
func (stubSchemaRepo) GetTraceOperationVersion(ctx context.Context, tenantID, service, operation string, version int64) (map[string]any, repo.VersionInfo, error) {
	return map[string]any{}, repo.VersionInfo{Version: version}, nil
}
func (stubSchemaRepo) DeleteMetric(ctx context.Context, tenantID, metric string) error  { return nil }
func (stubSchemaRepo) DeleteLogField(ctx context.Context, tenantID, field string) error { return nil }
func (stubSchemaRepo) DeleteTraceService(ctx context.Context, tenantID, service string) error {
	return nil
}
func (stubSchemaRepo) DeleteTraceOperation(ctx context.Context, tenantID, service, operation string) error {
	return nil
}
func (stubSchemaRepo) UpsertSchemaAsKPI(ctx context.Context, schemaDef *models.SchemaDefinition, author string) error {
	return nil
}
func (stubSchemaRepo) GetSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) (*models.SchemaDefinition, error) {
	return &models.SchemaDefinition{ID: id, Type: models.SchemaType(schemaType), TenantID: tenantID}, nil
}
func (stubSchemaRepo) ListSchemasAsKPIs(ctx context.Context, tenantID, schemaType string, limit, offset int) ([]*models.SchemaDefinition, int, error) {
	return []*models.SchemaDefinition{}, 0, nil
}
func (stubSchemaRepo) DeleteSchemaAsKPI(ctx context.Context, tenantID, schemaType, id string) error {
	return nil
}

// KPI methods
func (stubSchemaRepo) UpsertKPI(kpi *models.KPIDefinition) error {
	return nil
}
func (stubSchemaRepo) GetKPI(tenantID, id string) (*models.KPIDefinition, error) {
	return &models.KPIDefinition{ID: id, TenantID: tenantID}, nil
}
func (stubSchemaRepo) ListKPIs(tenantID string, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error) {
	return []*models.KPIDefinition{}, 0, nil
}
func (stubSchemaRepo) DeleteKPI(tenantID, id string) error {
	return nil
}
func (stubSchemaRepo) GetKPILayoutsForDashboard(tenantID, dashboardID string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (stubSchemaRepo) BatchUpsertKPILayouts(tenantID, dashboardID string, layouts map[string]interface{}) error {
	return nil
}
func (stubSchemaRepo) UpsertDashboard(dashboard *models.Dashboard) error {
	return nil
}
func (stubSchemaRepo) GetDashboard(tenantID, id string) (*models.Dashboard, error) {
	return &models.Dashboard{ID: id, TenantID: tenantID}, nil
}
func (stubSchemaRepo) ListDashboards(tenantID string, limit, offset int) ([]*models.Dashboard, int, error) {
	return []*models.Dashboard{}, 0, nil
}
func (stubSchemaRepo) DeleteDashboard(tenantID, id string) error {
	return nil
}

func TestServer_AuthOn_And_SchemaRegistered(t *testing.T) {
	// Ensure switching gin mode for production path does not leak globally
	prev := gin.Mode()
	defer gin.SetMode(prev)

	log := logger.New("error")
	cfg := &config.Config{Environment: "production", Port: 0}
	cfg.Auth.Enabled = true
	cfg.UnifiedQuery.Enabled = false

	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	// mockRBACRepositoryForServerCoverTest implements RBACRepository for testing
	mockRBACRepo := &mockRBACRepositoryForServerCoverTest{}

	s := NewServer(cfg, log, cch, grpc, vms, stubSchemaRepo{}, mockRBACRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// Public health (non-versioned) should work with auth enabled
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", resp.StatusCode)
	}

	// Protected endpoint should be unauthorized without token
	r2, err := http.Get(ts.URL + "/api/v1/logs/streams")
	if err != nil {
		t.Fatalf("logs streams: %v", err)
	}
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", r2.StatusCode)
	}

	// Schema routes registered (not invoked here due to auth)
}
