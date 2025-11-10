package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockRBACRepositoryForServerMoreTest implements RBACRepository for testing
type mockRBACRepositoryForServerMoreTest struct{}

func (m *mockRBACRepositoryForServerMoreTest) CreateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetRoleBindings(ctx context.Context, tenantID string, filters rbac.RoleBindingFilters) ([]*models.RoleBinding, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) CreateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeleteTenant(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) CreateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeleteUser(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeleteMiradorAuth(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerMoreTest) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForServerMoreTest) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	return nil
}

// Covers auth-off branch and openapi.json route
func TestServer_OpenAPI_AndRootRedirect(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: 0}
	cfg.Auth.Enabled = false
	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}
	grpc := &clients.GRPCClients{}
	cch := cache.NewNoopValkeyCache(log)

	mockRBACRepo := &mockRBACRepositoryForServerMoreTest{}

	s := NewServer(cfg, log, cch, grpc, vms, nil, mockRBACRepo)
	ts := httptest.NewServer(s.router)
	defer ts.Close()

	// root should redirect to swagger
	// prevent following redirects so we observe 302
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("root status=%d", resp.StatusCode)
	}

	// openapi.json should return 200
	r2, err := http.Get(ts.URL + "/api/openapi.json")
	if err != nil {
		t.Fatalf("openapi.json: %v", err)
	}
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("openapi.json status=%d", r2.StatusCode)
	}
}

// Cover Start/Stop path (graceful shutdown)
// Note: Start/Stop path is exercised via integration/runtime, not unit tests, to avoid
// closing uninitialized gRPC clients. The server handler is covered via other tests.
