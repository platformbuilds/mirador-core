package api

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockRBACRepositoryForServerTest implements RBACRepository for testing
type mockRBACRepositoryForServerTest struct{}

func (m *mockRBACRepositoryForServerTest) CreateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetRoleBindings(ctx context.Context, tenantID string, filters rbac.RoleBindingFilters) ([]*models.RoleBinding, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) CreateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	tenant.ID = "test-tenant-id"
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeleteTenant(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) CreateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return &models.User{
		ID:       userID,
		Username: "admin",
		Email:    "admin@platformbuilds.com",
	}, nil
}
func (m *mockRBACRepositoryForServerTest) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeleteUser(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeleteMiradorAuth(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) CreateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) GetAPIKeyByHash(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) GetAPIKeyByID(ctx context.Context, tenantID, keyID string) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForServerTest) ListAPIKeys(ctx context.Context, tenantID, userID string) ([]*models.APIKey, error) {
	return []*models.APIKey{}, nil
}
func (m *mockRBACRepositoryForServerTest) UpdateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) RevokeAPIKey(ctx context.Context, tenantID, keyID string) error {
	return nil
}
func (m *mockRBACRepositoryForServerTest) ValidateAPIKey(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	return nil, nil
}

func TestServer_Start_And_Handler(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "development", Port: 0}
	cfg.Auth.Enabled = false
	cch := cache.NewNoopValkeyCache(log)
	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}
	grpc, err := clients.NewGRPCClients(cfg, log, services.NewDynamicConfigService(cch, log))
	if err != nil {
		t.Fatalf("grpc clients: %v", err)
	}

	// Create a mock RBAC repository for testing
	mockRBACRepo := &mockRBACRepositoryForServerTest{}

	s := NewServer(cfg, log, cch, grpc, vms, nil, mockRBACRepo)

	// call Handler() to cover that method
	if s.Handler() == nil {
		t.Fatalf("handler should not be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- s.Start(ctx) }()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("start/shutdown error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server did not shut down in time")
	}
}

func TestServer_Start_Fails(t *testing.T) {
	log := logger.New("error")
	cfg := &config.Config{Environment: "test", Port: -1}
	cfg.Auth.Enabled = false
	cch := cache.NewNoopValkeyCache(log)
	vms := &services.VictoriaMetricsServices{
		Metrics: services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{}, log),
		Logs:    services.NewVictoriaLogsService(config.VictoriaLogsConfig{}, log),
		Traces:  services.NewVictoriaTracesService(config.VictoriaTracesConfig{}, log),
	}
	// Provide grpc clients with non-nil logger to avoid nil deref during Close (not reached here)
	grpc, _ := clients.NewGRPCClients(&config.Config{Environment: "development"}, log, services.NewDynamicConfigService(cch, log))

	mockRBACRepo := &mockRBACRepositoryForServerTest{}

	s := NewServer(cfg, log, cch, grpc, vms, nil, mockRBACRepo)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	// Expect immediate error due to invalid port
	if err := s.Start(ctx); err == nil {
		t.Fatalf("expected start error with invalid port")
	}
}
