package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockRBACRepositoryForMetricsTest implements RBACRepository for testing
type mockRBACRepositoryForMetricsTest struct{}

func (m *mockRBACRepositoryForMetricsTest) CreateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetRoleBindings(ctx context.Context, tenantID string, filters rbac.RoleBindingFilters) ([]*models.RoleBinding, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) CreateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeleteTenant(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) CreateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeleteUser(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeleteMiradorAuth(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForMetricsTest) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForMetricsTest) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	return nil
}

func TestMetricsMetadataIntegration(t *testing.T) {
	// Create a test configuration with metrics metadata enabled
	cfg := &config.Config{
		Environment: "test",
		Port:        0, // Use random port
		Search: config.SearchConfig{
			DefaultEngine: "lucene",
			EnableBleve:   true,
			Bleve: config.BleveConfig{
				MetricsEnabled: true,
				IndexPath:      "/tmp/mirador-test-bleve",
			},
		},
		UnifiedQuery: config.UnifiedQueryConfig{
			Enabled: true,
		},
	}

	log := logger.New("error")

	// Create mock services
	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}

	grpcClients := &clients.GRPCClients{}
	valkeyCache := cache.NewNoopValkeyCache(log)
	var schemaRepo repo.SchemaStore // nil for this test

	mockRBACRepo := &mockRBACRepositoryForMetricsTest{}

	// Create server - this should initialize metrics metadata components
	server := NewServer(cfg, log, valkeyCache, grpcClients, vms, schemaRepo, mockRBACRepo)
	if server == nil {
		t.Fatal("Failed to create server")
	}

	// Check that metrics metadata components were initialized
	if server.metricsMetadataIndexer == nil {
		t.Error("MetricsMetadataIndexer should be initialized")
	}
	if server.metricsMetadataSynchronizer == nil {
		t.Error("MetricsMetadataSynchronizer should be initialized")
	}

	// Create a test HTTP request to check if routes are registered
	req := httptest.NewRequest("GET", "/api/v1/metrics/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// The endpoint should exist (even if it returns an error due to no VictoriaMetrics)
	if w.Code == http.StatusNotFound {
		t.Error("Metrics health endpoint should be registered")
	}

	t.Logf("Metrics metadata integration test passed - components initialized and routes registered")
}
