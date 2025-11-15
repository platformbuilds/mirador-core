package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/platformbuilds/mirador-core/internal/api"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/services"
	storage_weaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockRBACRepository implements RBACRepository for basic functionality
type mockRBACRepository struct{}

func (m *mockRBACRepository) CreateRole(ctx context.Context, role *models.Role) error { return nil }
func (m *mockRBACRepository) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepository) UpdateRole(ctx context.Context, role *models.Role) error { return nil }
func (m *mockRBACRepository) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepository) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepository) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepository) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepository) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepository) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, nil
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
	return nil, nil
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
	return nil, nil
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
	return nil, nil
}
func (m *mockRBACRepository) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepository) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return nil, nil
}
func (m *mockRBACRepository) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepository) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepository) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepository) DeleteTenant(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepository) CreateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepository) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepository) UpdateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepository) DeleteUser(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepository) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepository) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return nil, nil
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
func (m *mockRBACRepository) DeleteMiradorAuth(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepository) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepository) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepository) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepository) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepository) CreateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	return nil
}
func (m *mockRBACRepository) GetAPIKeyByHash(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRBACRepository) GetAPIKeyByID(ctx context.Context, tenantID, apiKeyID string) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRBACRepository) ListAPIKeys(ctx context.Context, tenantID, userID string) ([]*models.APIKey, error) {
	return nil, nil
}
func (m *mockRBACRepository) UpdateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	return nil
}
func (m *mockRBACRepository) RevokeAPIKey(ctx context.Context, tenantID, apiKeyID string) error {
	return nil
}
func (m *mockRBACRepository) ValidateAPIKey(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	return nil, nil
}

// @title Mirador Core API
// @version 8.0.0
// @description Mirador Core is a comprehensive observability and analytics platform that provides KPI definitions, layouts, dashboards, and user preferences for monitoring and analyzing system metrics.
// @termsOfService http://swagger.io/terms/

// @contact.name Platform Builds Team
// @contact.url https://github.com/platformbuilds/mirador-core
// @contact.email support@platformbuilds.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8010
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API key for authentication

// @externalDocs.description OpenAPI
// @externalDocs.url https://swagger.io/resources/open-api/

// These are set via -ldflags at build time (see Makefile)
var (
	version    = "dev"
	commitHash = "unknown"
	buildTime  = ""
)

func main() {
	// Check for healthcheck command
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		// Load configuration to verify it's valid
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Configuration load failed: %v", err)
		}

		// Make HTTP request to health endpoint
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/health", cfg.Port))
		if err != nil {
			log.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			log.Fatalf("Health check failed: status %d", resp.StatusCode)
		}

		// Parse response
		var healthResp struct {
			Service   string `json:"service"`
			Status    string `json:"status"`
			Version   string `json:"version"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
			log.Fatalf("Failed to parse health response: %v", err)
		}

		if healthResp.Service != "mirador-core" || healthResp.Status != "healthy" {
			log.Fatalf("Health check failed: invalid response %+v", healthResp)
		}

		log.Println("healthy")
		return
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := logger.New(cfg.LogLevel)
	logger.Info("Starting MIRADOR-CORE", "version", version, "commit", commitHash, "built", buildTime, "environment", cfg.Environment)

	// Initialize Valkey cache: single-node when one address is provided; cluster otherwise
	var valkeyCache cache.ValkeyCluster
	if len(cfg.Cache.Nodes) == 1 {
		// Try immediate single-node connect; on failure, start with noop and auto-swap in background
		valkeyCache, err = cache.NewValkeySingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second)
		if err != nil {
			logger.Warn("Valkey single-node unavailable; starting with in-memory cache (auto-reconnect enabled)", "error", err)
			fallback := cache.NewNoopValkeyCache(logger)
			valkeyCache = cache.NewAutoSwapForSingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
		} else {
			logger.Info("Valkey single-node cache initialized", "addr", cfg.Cache.Nodes[0])
		}
	} else {
		// Prefer cluster when multiple nodes provided; if the target is a standalone instance
		// (common in development), detect the specific error and fall back to single-node.
		valkeyCache, err = cache.NewValkeyCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "cluster support disabled") {
				logger.Warn("Valkey reports cluster support disabled; falling back to single-node mode", "nodes", cfg.Cache.Nodes)
				// Try single-node on the first address; if that fails, use noop with auto-swap-to-single
				if len(cfg.Cache.Nodes) > 0 {
					if single, sErr := cache.NewValkeySingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second); sErr == nil {
						valkeyCache = single
						logger.Info("Valkey single-node cache initialized via fallback", "addr", cfg.Cache.Nodes[0])
					} else {
						logger.Warn("Valkey single-node fallback unavailable; starting with in-memory cache (auto-reconnect to single)", "error", sErr)
						fallback := cache.NewNoopValkeyCache(logger)
						valkeyCache = cache.NewAutoSwapForSingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
					}
				}
			} else {
				logger.Warn("Valkey cluster unavailable; starting with in-memory cache (auto-reconnect to cluster)", "error", err)
				fallback := cache.NewNoopValkeyCache(logger)
				valkeyCache = cache.NewAutoSwapForCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
			}
		} else {
			logger.Info("Valkey cluster cache initialized", "nodes", len(cfg.Cache.Nodes))
		}
	}

	// Initialize gRPC clients for AI engines
	dynamicConfigService := services.NewDynamicConfigService(valkeyCache, logger)
	grpcClients, err := clients.NewGRPCClients(cfg, logger, dynamicConfigService)
	if err != nil {
		logger.Fatal("Failed to initialize gRPC clients", "error", err)
	}
	logger.Info("gRPC clients initialized for AI engines")

	// Initialize VictoriaMetrics services
	vmServices, err := services.NewVictoriaMetricsServices(cfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to initialize VictoriaMetrics services", "error", err)
	}

	// Initialize schema store (Weaviate)
	var schemaStore repo.SchemaStore
	var wrepo *repo.WeaviateRepo
	if cfg.Weaviate.Enabled {
		// Construct transport (HTTP by default; official client when built with tags)
		t, terr := storage_weaviate.NewTransportFromConfig(cfg.Weaviate, logger)
		if terr != nil {
			logger.Fatal("Failed to init Weaviate transport", "error", terr)
		}
		ctxPing, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
		if err := storage_weaviate.Ready(ctxPing, t); err != nil {
			cancelPing()
			logger.Fatal("Weaviate not ready", "error", err)
		}
		cancelPing()
		logger.Info("Weaviate ready")
		wrepo = repo.NewWeaviateRepoFromTransport(t)
		if err := wrepo.EnsureSchema(context.Background()); err != nil {
			logger.Warn("Weaviate schema ensure failed", "error", err)
		}
		schemaStore = wrepo
	}

	// No legacy DB fallback; expect Weaviate

	// Initialize RBAC repository
	var rbacRepo rbac.RBACRepository
	if cfg.Weaviate.Enabled && wrepo != nil {
		rbacRepo = rbac.NewWeaviateRBACRepository(wrepo.Transport())
	} else {
		rbacRepo = &mockRBACRepository{}
	}

	// Initialize API server
	apiServer := api.NewServer(cfg, logger, valkeyCache, grpcClients, vmServices, schemaStore, rbacRepo)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// If cache supports Stop (auto-swap connector), tie it to lifecycle
	if stopper, ok := interface{}(valkeyCache).(interface{ Stop() }); ok {
		go func() { <-ctx.Done(); stopper.Stop() }()
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Info("Shutdown signal received")
		cancel()
	}()

	// Start dynamic endpoint discovery (DNS-based) if configured
	vmServices.StartDiscovery(ctx, cfg.Database, logger)

	// Start server
	if err := apiServer.Start(ctx); err != nil {
		logger.Fatal("Server failed to start", "error", err)
	}

	logger.Info("MIRADOR-CORE shutdown complete")
}
