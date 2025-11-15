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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/api/handlers"
	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// mockRBACRepositoryForIntegrationTest implements RBACRepository for integration testing
type mockRBACRepositoryForIntegrationTest struct {
	apiKeys map[string]*models.APIKey
}

func newMockRBACRepositoryForIntegrationTest() *mockRBACRepositoryForIntegrationTest {
	return &mockRBACRepositoryForIntegrationTest{
		apiKeys: make(map[string]*models.APIKey),
	}
}

func (m *mockRBACRepositoryForIntegrationTest) CreateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	m.apiKeys[apiKey.KeyHash] = apiKey
	return nil
}

func (m *mockRBACRepositoryForIntegrationTest) GetAPIKeyByHash(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	if apiKey, exists := m.apiKeys[keyHash]; exists && apiKey.TenantID == tenantID {
		return apiKey, nil
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForIntegrationTest) GetAPIKeyByID(ctx context.Context, tenantID, keyID string) (*models.APIKey, error) {
	for _, apiKey := range m.apiKeys {
		if apiKey.ID == keyID && apiKey.TenantID == tenantID {
			return apiKey, nil
		}
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForIntegrationTest) ListAPIKeys(ctx context.Context, tenantID, userID string) ([]*models.APIKey, error) {
	var result []*models.APIKey
	for _, apiKey := range m.apiKeys {
		if apiKey.TenantID == tenantID && apiKey.UserID == userID {
			result = append(result, apiKey)
		}
	}
	return result, nil
}

func (m *mockRBACRepositoryForIntegrationTest) UpdateAPIKey(ctx context.Context, apiKey *models.APIKey) error {
	m.apiKeys[apiKey.KeyHash] = apiKey
	return nil
}

func (m *mockRBACRepositoryForIntegrationTest) RevokeAPIKey(ctx context.Context, tenantID, keyID string) error {
	for hash, apiKey := range m.apiKeys {
		if apiKey.ID == keyID && apiKey.TenantID == tenantID {
			apiKey.IsActive = false
			m.apiKeys[hash] = apiKey
			return nil
		}
	}
	return fmt.Errorf("API key not found")
}

func (m *mockRBACRepositoryForIntegrationTest) ValidateAPIKey(ctx context.Context, tenantID, keyHash string) (*models.APIKey, error) {
	if apiKey, exists := m.apiKeys[keyHash]; exists && apiKey.TenantID == tenantID {
		if !apiKey.IsValid() {
			return nil, fmt.Errorf("API key is invalid")
		}
		return apiKey, nil
	}
	return nil, fmt.Errorf("API key not found")
}

// Stub implementations for other RBACRepository methods (not used in auth integration tests)
func (m *mockRBACRepositoryForIntegrationTest) CreateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdateRole(ctx context.Context, role *models.Role) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetRoleBindings(ctx context.Context, tenantID string, filters rbac.RoleBindingFilters) ([]*models.RoleBinding, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) CreateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetAuditEvents(ctx context.Context, tenantID string, filters rbac.AuditFilters) ([]*models.AuditLog, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) ListTenants(ctx context.Context, filters rbac.TenantFilters) ([]*models.Tenant, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeleteTenant(ctx context.Context, tenantID string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) CreateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) ListUsers(ctx context.Context, filters rbac.UserFilters) ([]*models.User, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdateUser(ctx context.Context, user *models.User) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeleteUser(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) ListTenantUsers(ctx context.Context, tenantID string, filters rbac.TenantUserFilters) ([]*models.TenantUser, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeleteMiradorAuth(ctx context.Context, userID string) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	return nil, nil
}
func (m *mockRBACRepositoryForIntegrationTest) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	return nil
}
func (m *mockRBACRepositoryForIntegrationTest) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	return nil
}

// TestAuthIntegration_AuthenticationFlow tests the complete authentication flow
// from API key generation to validation and usage
func TestAuthIntegration_AuthenticationFlow(t *testing.T) {
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
		APIKeys: config.APIKeyLimitsConfig{
			Enabled: true,
			DefaultLimits: config.DefaultAPIKeyLimits{
				MaxKeysPerUser:        10,
				MaxKeysPerTenantAdmin: 50,
				MaxKeysPerGlobalAdmin: 100,
			},
		},
	}

	// Create mock Valkey cache for testing
	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	// Create RBAC repository
	rbacRepo := newMockRBACRepositoryForIntegrationTest()

	// Create auth handler
	authHandler := handlers.NewAuthHandler(cfg, cacheClient, rbacRepo, logger.New("info"))

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Add middleware to set user context for testing
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user-123")
		c.Set("tenant_id", "test-tenant-123")
		c.Set("roles", []string{"user"})
		c.Next()
	})

	// Setup auth routes
	authGroup := router.Group("/api/v1/auth")
	{
		authGroup.POST("/apikeys", authHandler.GenerateAPIKey)
		authGroup.GET("/apikeys", authHandler.ListAPIKeys)
		authGroup.DELETE("/apikeys/:keyId", authHandler.RevokeAPIKey)
		authGroup.POST("/validate", authHandler.ValidateToken)
	}

	t.Run("GenerateAPIKey_Success", func(t *testing.T) {
		// Create API key generation request
		reqBody := map[string]interface{}{
			"name":        "Test API Key",
			"description": "Test key for integration testing",
			"scopes":      []string{"read", "write"},
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		// Create request
		req, err := http.NewRequest("POST", "/api/v1/auth/apikeys", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response
		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check status
		assert.Equal(t, "success", response["status"])

		// Check data
		data := response["data"].(map[string]interface{})
		assert.NotEmpty(t, data["api_key"])
		assert.NotEmpty(t, data["key_prefix"])
		assert.Equal(t, "Test API Key", data["name"])
		assert.Contains(t, data["scopes"], "read")
		assert.Contains(t, data["scopes"], "write")
		assert.NotEmpty(t, data["warning"])
	})

	t.Run("ListAPIKeys_Success", func(t *testing.T) {
		// Create request to list API keys
		req, err := http.NewRequest("GET", "/api/v1/auth/apikeys", nil)
		require.NoError(t, err)

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check status
		assert.Equal(t, "success", response["status"])

		// Check data
		data := response["data"].(map[string]interface{})
		apiKeys := data["api_keys"].([]interface{})
		assert.Greater(t, len(apiKeys), 0)

		// Verify API key structure
		apiKey := apiKeys[0].(map[string]interface{})
		assert.NotEmpty(t, apiKey["id"])
		assert.NotEmpty(t, apiKey["prefix"])
		assert.True(t, apiKey["is_active"].(bool))
		assert.NotEmpty(t, apiKey["created_at"])
	})

	t.Run("ValidateToken_Success", func(t *testing.T) {
		// First, get the full API key from repository for validation
		apiKeys, err := rbacRepo.ListAPIKeys(context.Background(), "test-tenant-123", "test-user-123")
		require.NoError(t, err)
		require.Greater(t, len(apiKeys), 0)

		apiKey := apiKeys[0]
		fullKey := fmt.Sprintf("%s%s", apiKey.Prefix, "test-key-suffix") // Mock full key for testing

		// Create token validation request
		reqBody := map[string]interface{}{
			"token": fullKey,
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		// Create request
		req, err := http.NewRequest("POST", "/api/v1/auth/validate", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "success", response["status"])
		data := response["data"].(map[string]interface{})
		assert.True(t, data["valid"].(bool))
		assert.Equal(t, "api_key", data["type"])
		assert.NotEmpty(t, data["user_id"])
		assert.NotEmpty(t, data["tenant_id"])
	})

	t.Run("RevokeAPIKey_Success", func(t *testing.T) {
		// Get API key ID to revoke
		apiKeys, err := rbacRepo.ListAPIKeys(context.Background(), "test-tenant-123", "test-user-123")
		require.NoError(t, err)
		require.Greater(t, len(apiKeys), 0)

		apiKeyID := apiKeys[0].ID

		// Create revoke request
		req, err := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/auth/apikeys/%s", apiKeyID), nil)
		require.NoError(t, err)

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "success", response["status"])
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "API key revoked successfully", data["message"])
	})

	t.Run("ValidateToken_RevokedKey_Failure", func(t *testing.T) {
		// Try to validate the revoked key
		apiKeys, err := rbacRepo.ListAPIKeys(context.Background(), "test-tenant-123", "test-user-123")
		require.NoError(t, err)
		require.Greater(t, len(apiKeys), 0)

		apiKey := apiKeys[0]
		fullKey := fmt.Sprintf("%s%s", apiKey.Prefix, "test-key-suffix") // Mock full key

		// Create token validation request
		reqBody := map[string]interface{}{
			"token": fullKey,
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		// Create request
		req, err := http.NewRequest("POST", "/api/v1/auth/validate", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response - should fail for revoked key
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "error", response["status"])
		assert.Contains(t, response["error"], "Invalid API key")
	})
}

// TestAuthIntegration_RateLimiting tests API key rate limiting functionality
func TestAuthIntegration_RateLimiting(t *testing.T) {
	if os.Getenv("MIRADOR_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration test requires external services")
	}
	// Setup test environment
	gin.SetMode(gin.TestMode)

	// Create test configuration with low rate limits for testing
	cfg := &config.Config{
		Auth: config.AuthConfig{
			StrictAPIKeyMode: true,
			APIKeyRateLimit: config.APIKeyRateLimitConfig{
				MaxRequests:    3, // Low limit for testing
				WindowDuration: time.Minute,
				BlockDuration:  time.Minute,
			},
		},
	}

	// Create mock Valkey cache
	cacheClient, err := cache.NewValkeyCluster([]string{"localhost:6379"}, time.Hour)
	require.NoError(t, err)

	// Create RBAC repository
	rbacRepo := newMockRBACRepositoryForIntegrationTest()

	// Create auth middleware
	authMiddleware := middleware.AuthMiddleware(cfg.Auth, cacheClient, rbacRepo)

	// Create test router with auth middleware
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(authMiddleware)

	// Setup protected route
	router.GET("/api/v1/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	t.Run("RateLimit_Enforced", func(t *testing.T) {
		apiKey := &models.APIKey{
			UserID:    "rate-limit-user",
			TenantID:  "rate-limit-tenant",
			KeyHash:   models.HashAPIKey("mrk_ratelimit_testkey123"),
			Prefix:    "mrk_ratelimit_",
			IsActive:  true,
			ExpiresAt: func() *time.Time { t := time.Now().Add(24 * time.Hour); return &t }(),
			Roles:     []string{"user"},
			Scopes:    []string{"read"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), apiKey)
		require.NoError(t, err)

		// Make requests up to the limit
		for i := 0; i < 3; i++ {
			req, reqErr := http.NewRequest("GET", "/api/v1/protected", nil)
			require.NoError(t, reqErr)
			req.Header.Set("Authorization", "Bearer mrk_ratelimit_testkey123")
			req.Header.Set("X-Tenant-ID", "rate-limit-tenant")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// First 3 requests should succeed
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// 4th request should be rate limited
		req, reqErr := http.NewRequest("GET", "/api/v1/protected", nil)
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer mrk_ratelimit_testkey123")
		req.Header.Set("X-Tenant-ID", "rate-limit-tenant")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should be rate limited
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})
}

// TestAuthIntegration_SessionCreation tests session creation from API key validation
func TestAuthIntegration_SessionCreation(t *testing.T) {
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

	// Create auth handler
	authHandler := handlers.NewAuthHandler(cfg, cacheClient, rbacRepo, logger.New("info"))

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup auth routes
	authGroup := router.Group("/api/v1/auth")
	{
		authGroup.POST("/validate", authHandler.ValidateToken)
	}

	t.Run("SessionCreation_FromAPIKey", func(t *testing.T) {
		apiKey := &models.APIKey{
			UserID:    "session-user-123",
			TenantID:  "session-tenant-123",
			KeyHash:   models.HashAPIKey("mrk_session_testkey123"),
			Prefix:    "mrk_session_",
			IsActive:  true,
			ExpiresAt: func() *time.Time { t := time.Now().Add(24 * time.Hour); return &t }(),
			Roles:     []string{"admin"},
			Scopes:    []string{"read", "write"},
		}

		err := rbacRepo.CreateAPIKey(context.Background(), apiKey)
		require.NoError(t, err)

		// Create token validation request
		reqBody := map[string]interface{}{
			"token": "mrk_session_testkey123",
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		// Create request
		req, err := http.NewRequest("POST", "/api/v1/auth/validate", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Create response recorder
		w := httptest.NewRecorder()

		// Perform request
		router.ServeHTTP(w, req)

		// Assert response
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "success", response["status"])
		data := response["data"].(map[string]interface{})
		assert.True(t, data["valid"].(bool))
		assert.Equal(t, "api_key", data["type"])
		assert.Equal(t, "session-user-123", data["user_id"])
		assert.Equal(t, "session-tenant-123", data["tenant_id"])
		assert.Contains(t, data["roles"], "admin")
		assert.Contains(t, data["scopes"], "read")
		assert.Contains(t, data["scopes"], "write")
	})
}
