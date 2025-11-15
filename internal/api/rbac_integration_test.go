package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRBACRepositoryForRBACIntegrationTest implements RBACRepository for RBAC integration testing
type mockRBACRepositoryForRBACIntegrationTest struct {
	apiKeys map[string]*models.APIKey
}

func newMockRBACRepositoryForRBACIntegrationTest() *mockRBACRepositoryForRBACIntegrationTest {
	return &mockRBACRepositoryForRBACIntegrationTest{
		apiKeys: make(map[string]*models.APIKey),
	}
}

func (m *mockRBACRepositoryForRBACIntegrationTest) CreateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	m.apiKeys[apiKey.KeyHash] = apiKey
	return nil
}

func (m *mockRBACRepositoryForRBACIntegrationTest) GetAPIKeyByHash(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	if apiKey, exists := m.apiKeys[keyHash]; exists && apiKey.TenantID == tenantID {
		return apiKey, nil
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForRBACIntegrationTest) GetAPIKeyByID(ctx context.Context, tenantID, keyID string) (*models.APIKey, error) {
	for _, apiKey := range m.apiKeys {
		if apiKey.ID == keyID && apiKey.TenantID == tenantID {
			return apiKey, nil
		}
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForRBACIntegrationTest) ListAPIKeys(ctx context.Context, tenantID, userID string) ([]*models.APIKey, error) {
	var keys []*models.APIKey
	for _, apiKey := range m.apiKeys {
		if apiKey.TenantID == tenantID && apiKey.UserID == userID {
			keys = append(keys, apiKey)
		}
	}
	return keys, nil
}

func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	if _, exists := m.apiKeys[apiKey.KeyHash]; exists {
		m.apiKeys[apiKey.KeyHash] = apiKey
		return nil
	}
	return fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForRBACIntegrationTest) RevokeAPIKey(ctx context.Context, tenantID, keyID string) error {
	for _, apiKey := range m.apiKeys {
		if apiKey.ID == keyID && apiKey.TenantID == tenantID {
			apiKey.IsActive = false
			return nil
		}
	}
	return fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForRBACIntegrationTest) ValidateAPIKey(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	if apiKey, exists := m.apiKeys[keyHash]; exists && apiKey.TenantID == tenantID && apiKey.IsActive {
		return apiKey, nil
	}
	return nil, fmt.Errorf("API key not found or inactive")
}

// Implement other RBACRepository methods (stubs for this test)
func (m *mockRBACRepositoryForRBACIntegrationTest) CreateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetRoleBindings(ctx context.Context, tenantID string, filters rbac.RoleBindingFilters) ([]*models.RoleBinding, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) CreateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeleteTenant(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) CreateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeleteUser(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeleteMiradorAuth(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForRBACIntegrationTest) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	return nil
}

// TestRBACIntegration_APIKeyRBACEnforcement tests RBAC enforcement with API key authentication
func TestRBACIntegration_APIKeyRBACEnforcement(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup test environment
	gin.SetMode(gin.TestMode)

	// Create test configuration
	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true,
			APIKeyRateLimit: config.APIKeyRateLimitConfig{
				MaxRequests:    1000,
				WindowDuration: time.Hour,
				BlockDuration:  time.Minute,
			},
		},
	}

	// Create mock Valkey cache
	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	// Create RBAC repository
	rbacRepo := newMockRBACRepositoryForRBACIntegrationTest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup protected routes with different permission requirements
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(authMiddleware)

	{
		// Admin only endpoint
		apiGroup.GET("/admin/users", func(c *gin.Context) {
			// Check if user has admin role
			roles, exists := c.Get("roles")
			if !exists {
				c.JSON(http.StatusForbidden, gin.H{"error": "no roles found"})
				return
			}

			userRoles, ok := roles.([]string)
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "invalid roles format"})
				return
			}

			hasAdmin := false
			for _, role := range userRoles {
				if role == "admin" {
					hasAdmin = true
					break
				}
			}

			if !hasAdmin {
				c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "admin access granted"})
		})

		// Read-write endpoint
		apiGroup.POST("/data", func(c *gin.Context) {
			// Check if user has write scope
			scopes, exists := c.Get("scopes")
			if !exists {
				c.JSON(http.StatusForbidden, gin.H{"error": "no scopes found"})
				return
			}

			userScopes, ok := scopes.([]string)
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "invalid scopes format"})
				return
			}

			hasWrite := false
			for _, scope := range userScopes {
				if scope == "write" {
					hasWrite = true
					break
				}
			}

			if !hasWrite {
				c.JSON(http.StatusForbidden, gin.H{"error": "write scope required"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "write access granted"})
		})

		// Read-only endpoint
		apiGroup.GET("/data", func(c *gin.Context) {
			// Check if user has read scope
			scopes, exists := c.Get("scopes")
			if !exists {
				c.JSON(http.StatusForbidden, gin.H{"error": "no scopes found"})
				return
			}

			userScopes, ok := scopes.([]string)
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "invalid scopes format"})
				return
			}

			hasRead := false
			for _, scope := range userScopes {
				if scope == "read" {
					hasRead = true
					break
				}
			}

			if !hasRead {
				c.JSON(http.StatusForbidden, gin.H{"error": "read scope required"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "read access granted"})
		})
	}

	t.Run("AdminRole_AccessGranted", func(t *testing.T) {
		// Create API key with admin role
		expiresAt := time.Now().Add(24 * time.Hour)
		adminKey := &models.APIKey{
			UserID:    "admin-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "admin-key-hash",
			Prefix:    "mk_admin_",
			IsActive:  true,
			ExpiresAt: &expiresAt,
			Roles:     []string{"admin"},
			Scopes:    []string{"read", "write"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), adminKey)
		require.NoError(t, err)

		// Create request with admin API key
		req, err := http.NewRequest("GET", "/api/v1/admin/users", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_admin_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response - should succeed
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "admin access granted", response["message"])
	})

	t.Run("WriteScope_AccessGranted", func(t *testing.T) {
		// Create API key with write scope
		expiresAt := time.Now().Add(24 * time.Hour)
		writeKey := &models.APIKey{
			UserID:    "write-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "write-key-hash",
			Prefix:    "mk_write_",
			IsActive:  true,
			ExpiresAt: &expiresAt,
			Roles:     []string{"user"},
			Scopes:    []string{"read", "write"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), writeKey)
		require.NoError(t, err)

		// Create request with write API key
		req, err := http.NewRequest("POST", "/api/v1/data", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_write_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response - should succeed
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "write access granted", response["message"])
	})

	t.Run("ReadScope_AccessGranted", func(t *testing.T) {
		// Create API key with read scope only
		expiresAt := time.Now().Add(24 * time.Hour)
		readKey := &models.APIKey{
			UserID:    "read-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "read-key-hash",
			Prefix:    "mk_read_",
			IsActive:  true,
			ExpiresAt: &expiresAt,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), readKey)
		require.NoError(t, err)

		// Create request with read API key
		req, err := http.NewRequest("GET", "/api/v1/data", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_read_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response - should succeed
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "read access granted", response["message"])
	})

	t.Run("InsufficientPermissions_AccessDenied", func(t *testing.T) {
		// Create API key with read scope only
		expiresAt := time.Now().Add(24 * time.Hour)
		readOnlyKey := &models.APIKey{
			UserID:    "readonly-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "readonly-key-hash",
			Prefix:    "mk_readonly_",
			IsActive:  true,
			ExpiresAt: &expiresAt,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), readOnlyKey)
		require.NoError(t, err)

		// Try to access admin endpoint with read-only key
		req, err := http.NewRequest("GET", "/api/v1/admin/users", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_readonly_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response - should be forbidden
		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "admin role required")
	})

	t.Run("NoScopes_AccessDenied", func(t *testing.T) {
		// Create API key with no scopes
		expiresAt := time.Now().Add(24 * time.Hour)
		noScopeKey := &models.APIKey{
			UserID:    "noscope-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "noscope-key-hash",
			Prefix:    "mk_noscope_",
			IsActive:  true,
			ExpiresAt: &expiresAt,
			Roles:     []string{"user"},
			Scopes:    []string{}, // No scopes
		}

		err := rbacRepo.CreateAPIKey(context.Background(), noScopeKey)
		require.NoError(t, err)

		// Try to access read endpoint with no-scope key
		req, err := http.NewRequest("GET", "/api/v1/data", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_noscope_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response - should be forbidden
		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "read scope required")
	})
}

// TestRBACIntegration_MultiTenantIsolation tests tenant isolation in RBAC
func TestRBACIntegration_MultiTenantIsolation(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup test environment
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true,
		},
	}

	// Create mock Valkey cache
	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	// Create RBAC repository
	rbacRepo := newMockRBACRepositoryForRBACIntegrationTest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup protected routes
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(authMiddleware)

	apiGroup.GET("/tenant-data", func(c *gin.Context) {
		tenantID, exists := c.Get("tenant_id")
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no tenant"})
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"tenant_id": tenantID,
			"user_id":   userID,
			"message":   "tenant data accessed",
		})
	})

	t.Run("TenantIsolation_Enforced", func(t *testing.T) {
		// Create API key for tenant A
		expiresAt := time.Now().Add(24 * time.Hour)
		tenantAKey := &models.APIKey{
			UserID:    "user-tenant-a",
			TenantID:  "tenant-a",
			KeyHash:   "tenant-a-key-hash",
			Prefix:    "mk_ta_",
			IsActive:  true,
			ExpiresAt: &expiresAt,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), tenantAKey)
		require.NoError(t, err)

		// Create API key for tenant B
		expiresAtB := time.Now().Add(24 * time.Hour)
		tenantBKey := &models.APIKey{
			UserID:    "user-tenant-b",
			TenantID:  "tenant-b",
			KeyHash:   "tenant-b-key-hash",
			Prefix:    "mk_tb_",
			IsActive:  true,
			ExpiresAt: &expiresAtB,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err = rbacRepo.CreateAPIKey(context.Background(), tenantBKey)
		require.NoError(t, err)

		// Test tenant A access
		reqA, err := http.NewRequest("GET", "/api/v1/tenant-data", nil)
		require.NoError(t, err)
		reqA.Header.Set("Authorization", "Bearer mk_ta_fullkey123")
		reqA.Header.Set("X-Tenant-ID", "tenant-a")

		wA := httptest.NewRecorder()
		router.ServeHTTP(wA, reqA)

		assert.Equal(t, http.StatusOK, wA.Code)

		var responseA map[string]interface{}
		err = json.Unmarshal(wA.Body.Bytes(), &responseA)
		require.NoError(t, err)
		assert.Equal(t, "tenant-a", responseA["tenant_id"])
		assert.Equal(t, "user-tenant-a", responseA["user_id"])

		// Test tenant B access
		reqB, err := http.NewRequest("GET", "/api/v1/tenant-data", nil)
		require.NoError(t, err)
		reqB.Header.Set("Authorization", "Bearer mk_tb_fullkey123")
		reqB.Header.Set("X-Tenant-ID", "tenant-b")

		wB := httptest.NewRecorder()
		router.ServeHTTP(wB, reqB)

		assert.Equal(t, http.StatusOK, wB.Code)

		var responseB map[string]interface{}
		err = json.Unmarshal(wB.Body.Bytes(), &responseB)
		require.NoError(t, err)
		assert.Equal(t, "tenant-b", responseB["tenant_id"])
		assert.Equal(t, "user-tenant-b", responseB["user_id"])
	})

	t.Run("CrossTenantAccess_Blocked", func(t *testing.T) {
		// Try to access tenant A data with tenant B key
		req, err := http.NewRequest("GET", "/api/v1/tenant-data", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_tb_fullkey123") // Tenant B key
		req.Header.Set("X-Tenant-ID", "tenant-a")                  // But requesting tenant A data

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail due to tenant mismatch
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestRBACIntegration_UnifiedQueryEndpoint tests the unified query endpoint protection
func TestRBACIntegration_UnifiedQueryEndpoint(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup test environment
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true,
		},
	}

	// Create mock Valkey cache
	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	// Create RBAC repository
	rbacRepo := newMockRBACRepositoryForRBACIntegrationTest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup unified query endpoint
	queryGroup := router.Group("/api/v1")
	queryGroup.Use(authMiddleware)

	queryGroup.POST("/query", func(c *gin.Context) {
		// Simulate unified query processing
		tenantID, _ := c.Get("tenant_id")
		userID, _ := c.Get("user_id")
		scopes, _ := c.Get("scopes")

		c.JSON(http.StatusOK, gin.H{
			"tenant_id": tenantID,
			"user_id":   userID,
			"scopes":    scopes,
			"result":    "query executed successfully",
		})
	})

	t.Run("UnifiedQuery_WithValidAPIKey", func(t *testing.T) {
		// Create API key with query permissions
		expiresAt := time.Now().Add(24 * time.Hour)
		queryKey := &models.APIKey{
			UserID:    "query-user-123",
			TenantID:  "query-tenant-123",
			KeyHash:   "query-key-hash",
			Prefix:    "mk_query_",
			IsActive:  true,
			ExpiresAt: &expiresAt,
			Roles:     []string{"analyst"},
			Scopes:    []string{"read", "query"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), queryKey)
		require.NoError(t, err)

		// Create query request
		queryBody := map[string]string{
			"query": "SELECT * FROM logs LIMIT 10",
		}
		jsonBody, err := json.Marshal(queryBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/api/v1/query", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer mk_query_fullkey123")
		req.Header.Set("X-Tenant-ID", "query-tenant-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assert successful query execution
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "query executed successfully", response["result"])
		assert.Equal(t, "query-tenant-123", response["tenant_id"])
		assert.Equal(t, "query-user-123", response["user_id"])
	})

	t.Run("UnifiedQuery_WithoutAPIKey_Fails", func(t *testing.T) {
		// Try to query without API key
		queryBody := map[string]string{
			"query": "SELECT * FROM logs LIMIT 10",
		}
		jsonBody, err := json.Marshal(queryBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/api/v1/query", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		// No Authorization header

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail authentication
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
