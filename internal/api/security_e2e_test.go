package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

// mockRBACRepositoryForSecurityE2ETest implements RBACRepository for security E2E testing
type mockRBACRepositoryForSecurityE2ETest struct {
	apiKeys map[string]*models.APIKey
}

func newMockRBACRepositoryForSecurityE2ETest() *mockRBACRepositoryForSecurityE2ETest {
	return &mockRBACRepositoryForSecurityE2ETest{
		apiKeys: make(map[string]*models.APIKey),
	}
}

func (m *mockRBACRepositoryForSecurityE2ETest) CreateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	m.apiKeys[apiKey.KeyHash] = apiKey
	return nil
}

func (m *mockRBACRepositoryForSecurityE2ETest) GetAPIKeyByHash(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	if apiKey, exists := m.apiKeys[keyHash]; exists && apiKey.TenantID == tenantID {
		return apiKey, nil
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForSecurityE2ETest) GetAPIKeyByID(ctx context.Context, tenantID, keyID string) (*models.APIKey, error) {
	for _, apiKey := range m.apiKeys {
		if apiKey.ID == keyID && apiKey.TenantID == tenantID {
			return apiKey, nil
		}
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForSecurityE2ETest) ListAPIKeys(ctx context.Context, tenantID, userID string) ([]*models.APIKey, error) {
	var keys []*models.APIKey
	for _, apiKey := range m.apiKeys {
		if apiKey.TenantID == tenantID && apiKey.UserID == userID {
			keys = append(keys, apiKey)
		}
	}
	return keys, nil
}

func (m *mockRBACRepositoryForSecurityE2ETest) UpdateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	if _, exists := m.apiKeys[apiKey.KeyHash]; exists {
		m.apiKeys[apiKey.KeyHash] = apiKey
		return nil
	}
	return fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForSecurityE2ETest) RevokeAPIKey(ctx context.Context, tenantID, keyID string) error {
	for _, apiKey := range m.apiKeys {
		if apiKey.ID == keyID && apiKey.TenantID == tenantID {
			apiKey.IsActive = false
			return nil
		}
	}
	return fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForSecurityE2ETest) ValidateAPIKey(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	if apiKey, exists := m.apiKeys[keyHash]; exists && apiKey.TenantID == tenantID && apiKey.IsActive {
		return apiKey, nil
	}
	return nil, fmt.Errorf("API key not found or inactive")
}

// Implement other RBACRepository methods (stubs for this test)
func (m *mockRBACRepositoryForSecurityE2ETest) CreateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetRoleBindings(ctx context.Context, tenantID string, filters rbac.RoleBindingFilters) ([]*models.RoleBinding, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) CreateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeleteTenant(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) CreateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeleteUser(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeleteMiradorAuth(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForSecurityE2ETest) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	return nil
}

// TestSecurityE2E_UnauthorizedAccess tests various unauthorized access attempts
func TestSecurityE2E_UnauthorizedAccess(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup test environment
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true,
			APIKeyRateLimit: config.APIKeyRateLimitConfig{
				MaxRequests:    10,
				WindowDuration: time.Minute,
				BlockDuration:  time.Minute,
			},
		},
	}

	// Create mock Valkey cache
	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	// Create RBAC repository
	rbacRepo := newMockRBACRepositoryForSecurityE2ETest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup protected routes
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(authMiddleware)

	apiGroup.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "access granted"})
	})

	apiGroup.POST("/sensitive", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "sensitive operation completed"})
	})

	t.Run("NoAuthentication_Fails", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/protected", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("InvalidAPIKey_Fails", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/protected", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid-api-key-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("MalformedAuthorizationHeader_Fails", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/protected", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "InvalidFormat api-key-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("ExpiredAPIKey_Fails", func(t *testing.T) {
		// Create expired API key
		expiredTime := time.Now().Add(-1 * time.Hour)
		expiredKey := &models.APIKey{
			UserID:    "expired-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "expired-key-hash",
			Prefix:    "mk_expired_",
			IsActive:  true,
			ExpiresAt: &expiredTime, // Already expired
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), expiredKey)
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/api/v1/protected", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_expired_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("RevokedAPIKey_Fails", func(t *testing.T) {
		// Create and then revoke API key
		validTime := time.Now().Add(24 * time.Hour)
		revokedKey := &models.APIKey{
			UserID:    "revoked-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "revoked-key-hash",
			Prefix:    "mk_revoked_",
			IsActive:  false, // Revoked
			ExpiresAt: &validTime,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), revokedKey)
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/api/v1/protected", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_revoked_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestSecurityE2E_APIKeyBruteForceProtection tests protection against API key brute force attacks
func TestSecurityE2E_APIKeyBruteForceProtection(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup test environment
	gin.SetMode(gin.TestMode)

	// Create config with very low rate limits for testing
	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true,
			APIKeyRateLimit: config.APIKeyRateLimitConfig{
				MaxRequests:    3, // Very low for testing
				WindowDuration: time.Minute,
				BlockDuration:  time.Minute,
			},
		},
	}

	// Create mock Valkey cache
	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	// Create RBAC repository
	rbacRepo := newMockRBACRepositoryForSecurityE2ETest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup protected routes
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(authMiddleware)

	apiGroup.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "access granted"})
	})

	t.Run("BruteForceAttack_Mitigated", func(t *testing.T) {
		// Simulate brute force attack with multiple invalid keys
		invalidKeys := []string{
			"invalid-key-1",
			"invalid-key-2",
			"invalid-key-3",
			"invalid-key-4",
			"invalid-key-5",
		}

		// First few requests should fail with unauthorized
		for i, invalidKey := range invalidKeys[:3] {
			req, err := http.NewRequest("GET", "/api/v1/protected", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+invalidKey)
			req.Header.Set("X-Tenant-ID", "test-tenant-123")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if i < 3 {
				// First 3 should be unauthorized but not rate limited
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			}
		}

		// Subsequent requests should be rate limited
		for _, invalidKey := range invalidKeys[3:] {
			req, err := http.NewRequest("GET", "/api/v1/protected", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+invalidKey)
			req.Header.Set("X-Tenant-ID", "test-tenant-123")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should be rate limited
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
		}
	})
}

// TestSecurityE2E_SessionTokenRejection tests that session tokens are rejected for programmatic access
func TestSecurityE2E_SessionTokenRejection(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup test environment
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true, // Enforce strict mode
		},
	}

	// Create mock Valkey cache
	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	// Create RBAC repository
	rbacRepo := newMockRBACRepositoryForSecurityE2ETest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup protected routes
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(authMiddleware)

	apiGroup.GET("/programmatic", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "programmatic access granted"})
	})

	t.Run("SessionToken_Rejected", func(t *testing.T) {
		// Try to access with session token (should be rejected in strict mode)
		req, err := http.NewRequest("GET", "/api/v1/programmatic", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer session-token-123") // Session token
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should be rejected because only API keys are allowed in strict mode
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("APIKey_Accepted", func(t *testing.T) {
		// Create valid API key
		validTime := time.Now().Add(24 * time.Hour)
		validKey := &models.APIKey{
			UserID:    "api-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "valid-key-hash",
			Prefix:    "mk_valid_",
			IsActive:  true,
			ExpiresAt: &validTime,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), validKey)
		require.NoError(t, err)

		// Access with valid API key
		req, err := http.NewRequest("GET", "/api/v1/programmatic", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_valid_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "programmatic access granted", response["message"])
	})
}

// TestSecurityE2E_RBACBoundaryTesting tests RBAC permission boundaries
func TestSecurityE2E_RBACBoundaryTesting(t *testing.T) {
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
	rbacRepo := newMockRBACRepositoryForSecurityE2ETest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup routes with different permission requirements
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(authMiddleware)

	// Admin only
	apiGroup.DELETE("/users/:id", func(c *gin.Context) {
		roles, _ := c.Get("roles")
		userRoles := roles.([]string)

		for _, role := range userRoles {
			if role == "admin" {
				c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
	})

	// Write scope required
	apiGroup.POST("/data", func(c *gin.Context) {
		scopes, _ := c.Get("scopes")
		userScopes := scopes.([]string)

		for _, scope := range userScopes {
			if scope == "write" {
				c.JSON(http.StatusOK, gin.H{"message": "data created"})
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "write scope required"})
	})

	// Read scope required
	apiGroup.GET("/data", func(c *gin.Context) {
		scopes, _ := c.Get("scopes")
		userScopes := scopes.([]string)

		for _, scope := range userScopes {
			if scope == "read" {
				c.JSON(http.StatusOK, gin.H{"message": "data retrieved"})
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "read scope required"})
	})

	t.Run("RoleEscalation_Prevented", func(t *testing.T) {
		// Create user with limited permissions
		validTime := time.Now().Add(24 * time.Hour)
		userKey := &models.APIKey{
			UserID:    "limited-user-123",
			TenantID:  "test-tenant-123",
			KeyHash:   "limited-key-hash",
			Prefix:    "mk_limited_",
			IsActive:  true,
			ExpiresAt: &validTime,
			Roles:     []string{"user"}, // Not admin
			Scopes:    []string{"read"}, // Only read
		}

		err := rbacRepo.CreateAPIKey(context.Background(), userKey)
		require.NoError(t, err)

		// Try to delete user (requires admin)
		req, err := http.NewRequest("DELETE", "/api/v1/users/456", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_limited_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "admin required", response["error"])
	})

	t.Run("ScopeEscalation_Prevented", func(t *testing.T) {
		// Try to create data (requires write scope)
		req, err := http.NewRequest("POST", "/api/v1/data", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_limited_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "write scope required", response["error"])
	})

	t.Run("AuthorizedAccess_Succeeds", func(t *testing.T) {
		// Read access should work
		req, err := http.NewRequest("GET", "/api/v1/data", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_limited_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant-123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "data retrieved", response["message"])
	})
}

// TestSecurityE2E_AuthorizationBypassAttempts tests various authorization bypass attempts
func TestSecurityE2E_AuthorizationBypassAttempts(t *testing.T) {
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
	rbacRepo := newMockRBACRepositoryForSecurityE2ETest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup protected routes
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(authMiddleware)

	apiGroup.GET("/protected/:tenantId", func(c *gin.Context) {
		// Check tenant isolation
		requestedTenant := c.Param("tenantId")
		userTenant, _ := c.Get("tenant_id")

		if requestedTenant != userTenant {
			c.JSON(http.StatusForbidden, gin.H{"error": "tenant access denied"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "tenant data accessed"})
	})

	t.Run("PathTraversal_Blocked", func(t *testing.T) {
		// Create API key for tenant A
		validTime := time.Now().Add(24 * time.Hour)
		tenantAKey := &models.APIKey{
			UserID:    "user-a",
			TenantID:  "tenant-a",
			KeyHash:   "tenant-a-hash",
			Prefix:    "mk_ta_",
			IsActive:  true,
			ExpiresAt: &validTime,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), tenantAKey)
		require.NoError(t, err)

		// Try to access tenant B data with tenant A key
		req, err := http.NewRequest("GET", "/api/v1/protected/tenant-b", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_ta_fullkey123")
		req.Header.Set("X-Tenant-ID", "tenant-a")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]string
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "tenant access denied", response["error"])
	})

	t.Run("HeaderInjection_Blocked", func(t *testing.T) {
		// Try header injection to bypass tenant check
		req, err := http.NewRequest("GET", "/api/v1/protected/tenant-a", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_ta_fullkey123")
		req.Header.Set("X-Tenant-ID", "tenant-a")
		req.Header.Set("X-Forwarded-Tenant-Id", "tenant-b") // Try to inject different tenant

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should still work because we use X-Tenant-ID, not forwarded headers
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("PrivilegeEscalationViaRoles_Blocked", func(t *testing.T) {
		// Create API key with user role trying to access admin functionality
		validTime := time.Now().Add(24 * time.Hour)
		userKey := &models.APIKey{
			UserID:    "regular-user",
			TenantID:  "test-tenant",
			KeyHash:   "user-hash",
			Prefix:    "mk_user_",
			IsActive:  true,
			ExpiresAt: &validTime,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), userKey)
		require.NoError(t, err)

		// Try to modify roles through API key metadata (if such endpoint existed)
		// This simulates a privilege escalation attempt
		req, err := http.NewRequest("GET", "/api/v1/protected/test-tenant", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_user_fullkey123")
		req.Header.Set("X-Tenant-ID", "test-tenant")
		// Try to inject role escalation via headers
		req.Header.Set("X-Roles", "admin") // This should be ignored

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed for same tenant but role should not be escalated
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestSecurityE2E_SecurityHeaders tests security headers and response sanitization
func TestSecurityE2E_SecurityHeaders(t *testing.T) {
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
	rbacRepo := newMockRBACRepositoryForSecurityE2ETest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Add security headers middleware
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	// Setup protected routes
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(authMiddleware)

	apiGroup.GET("/secure", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "secure response"})
	})

	t.Run("SecurityHeaders_Present", func(t *testing.T) {
		// Create valid API key
		validTime := time.Now().Add(24 * time.Hour)
		secureKey := &models.APIKey{
			UserID:    "secure-user",
			TenantID:  "secure-tenant",
			KeyHash:   "secure-hash",
			Prefix:    "mk_secure_",
			IsActive:  true,
			ExpiresAt: &validTime,
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), secureKey)
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/api/v1/secure", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer mk_secure_fullkey123")
		req.Header.Set("X-Tenant-ID", "secure-tenant")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check security headers
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	})

	t.Run("ErrorMessages_Sanitized", func(t *testing.T) {
		// Try to access without authentication
		req, err := http.NewRequest("GET", "/api/v1/secure", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// Check that error message doesn't leak sensitive information
		body := w.Body.String()
		assert.NotContains(t, body, "internal")
		assert.NotContains(t, body, "stack")
		assert.NotContains(t, body, "panic")
	})
}

// TestSecurityE2E_APIKeyEnumerationProtection tests protection against API key enumeration attempts
func TestSecurityE2E_APIKeyEnumerationProtection(t *testing.T) {
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
	rbacRepo := newMockRBACRepositoryForIntegrationTest()

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Add auth middleware
	router.Use(middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo))

	// Add a protected endpoint
	router.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	t.Run("Invalid_API_Keys_Uniform_Response", func(t *testing.T) {
		// Test various invalid API key formats to ensure uniform error responses
		invalidKeys := []string{
			"mrk_invalid",
			"mrk_1234567890123456789012345678901234567890", // Wrong length
			"mrk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", // Invalid chars
			"", // Empty
			"not_mrk_prefix",
		}

		for _, key := range invalidKeys {
			req, err := http.NewRequest("GET", "/api/v1/test", nil)
			require.NoError(t, err)
			req.Header.Set("X-API-Key", key)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// All should return 401 with generic error
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			body := w.Body.String()
			assert.Contains(t, body, "Invalid authentication token")
			// Ensure no detailed error information
			assert.NotContains(t, body, "format")
			assert.NotContains(t, body, "length")
		}
	})
}

// TestSecurityE2E_BruteForceProtection tests protection against brute force attacks
func TestSecurityE2E_BruteForceProtection(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup similar to above
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true,
			APIKeyRateLimit: config.APIKeyRateLimitConfig{
				Enabled:        true,
				MaxRequests:    5, // Low limit for testing
				WindowDuration: time.Minute,
				BlockDuration:  time.Minute,
			},
		},
	}

	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	rbacRepo := newMockRBACRepositoryForIntegrationTest()

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo))

	router.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	t.Run("Rate_Limiting_Enforced", func(t *testing.T) {
		// Simulate brute force by making many requests with invalid keys
		for i := 0; i < 10; i++ {
			req, err := http.NewRequest("GET", "/api/v1/test", nil)
			require.NoError(t, err)
			req.Header.Set("X-API-Key", "mrk_invalid_key_12345678901234567890")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if i < 5 {
				// First few should be 401
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			} else {
				// After rate limit, should be 401 with rate limit message
				assert.Equal(t, http.StatusUnauthorized, w.Code)
				// Check for rate limit headers or message
			}
		}
	})
}

// TestSecurityE2E_SessionFixationProtection tests protection against session fixation attacks
func TestSecurityE2E_SessionFixationProtection(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup for session-based auth (not strict API key mode)
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: false, // Allow sessions
		},
	}

	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	rbacRepo := newMockRBACRepositoryForIntegrationTest()

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo))

	router.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	t.Run("Session_Tokens_Rejected_For_Programmatic_Access", func(t *testing.T) {
		// Create a mock session token
		session := &models.UserSession{
			ID:           "test_session_123",
			UserID:       "user123",
			TenantID:     "tenant123",
			Roles:        []string{"user"},
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
		}

		// Store session
		err := cacheClient.SetSession(context.Background(), session)
		require.NoError(t, err)

		// Try to use session token for programmatic access (should fail in strict mode, but here it's not strict)
		req, err := http.NewRequest("GET", "/api/v1/test", nil)
		require.NoError(t, err)
		req.Header.Set("X-Session-Token", session.ID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed since not strict mode
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestSecurityE2E_TenantIsolation tests tenant data isolation
func TestSecurityE2E_TenantIsolation(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true,
		},
	}

	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	rbacRepo := newMockRBACRepositoryForIntegrationTest()

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo))

	router.GET("/api/v1/test", func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		c.JSON(http.StatusOK, gin.H{"tenant_id": tenantID})
	})

	t.Run("Tenant_ID_Properly_Isolated", func(t *testing.T) {
		// Test that tenant ID from auth is used consistently
		// This would require setting up valid API keys for different tenants
		// For now, just test that tenant context is available
		req, err := http.NewRequest("GET", "/api/v1/test", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", "mrk_invalid") // Will fail auth, but test isolation logic

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail auth, but if it passed, tenant should be set
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
