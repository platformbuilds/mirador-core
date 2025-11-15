package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// MockValkeyClient is a mock implementation of ValkeyClient for testing
type MockValkeyClient struct {
	mock.Mock
}

func (m *MockValkeyClient) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockValkeyClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func (m *MockValkeyClient) Del(ctx context.Context, keys ...string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

func (m *MockValkeyClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	args := m.Called(ctx, keys)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockValkeyClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	args := m.Called(ctx, key, expiration)
	return args.Error(0)
}

func (m *MockValkeyClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	args := m.Called(ctx, pattern)
	return args.Get(0).([]string), args.Error(1)
}

func TestValkeyRBACRepository_CreateAPIKey_Success(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	apiKey := &models.APIKey{
		ID:        "test-key-id",
		UserID:    "test-user",
		TenantID:  "test-tenant",
		Name:      "Test Key",
		KeyHash:   "abcd1234hash",
		Prefix:    "sk_test_",
		IsActive:  true,
		Roles:     []string{"admin"},
		Scopes:    []string{"read", "write"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: "test-user",
		UpdatedBy: "test-user",
	}

	hashKey := "apikey:test-tenant:abcd1234hash"
	idKey := "apikey_id:test-tenant:test-key-id"

	// Mock successful operations
	mockClient.On("Set", ctx, hashKey, mock.AnythingOfType("string"), time.Duration(0)).Return(nil)
	mockClient.On("Set", ctx, idKey, "abcd1234hash", time.Duration(0)).Return(nil)

	err := repo.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_CreateAPIKey_Failure(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	apiKey := &models.APIKey{
		ID:        "test-key-id",
		UserID:    "test-user",
		TenantID:  "test-tenant",
		Name:      "Test Key",
		KeyHash:   "abcd1234hash",
		Prefix:    "sk_test_",
		IsActive:  true,
		Roles:     []string{"admin"},
		Scopes:    []string{"read", "write"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: "test-user",
		UpdatedBy: "test-user",
	}

	hashKey := "apikey:test-tenant:abcd1234hash"

	// Mock failure on hash key set
	mockClient.On("Set", ctx, hashKey, mock.AnythingOfType("string"), time.Duration(0)).Return(errors.New("valkey error"))

	err := repo.CreateAPIKey(ctx, apiKey)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create API key")
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_GetAPIKeyByHash_Existing(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "abcd1234hash"

	hashKey := "apikey:test-tenant:abcd1234hash"
	jsonData := `{"id":"test-key-id","userId":"test-user","tenantId":"test-tenant","name":"Test Key","keyHash":"abcd1234hash","prefix":"sk_test_","isActive":true,"roles":["admin"],"scopes":["read","write"],"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z","createdBy":"test-user","updatedBy":"test-user"}`

	mockClient.On("Get", ctx, hashKey).Return(jsonData, nil)

	result, err := repo.GetAPIKeyByHash(ctx, tenantID, keyHash)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-key-id", result.ID)
	assert.Equal(t, "abcd1234hash", result.KeyHash)
	assert.Equal(t, tenantID, result.TenantID)
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_GetAPIKeyByHash_NonExistent(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "nonexistent"

	hashKey := "apikey:test-tenant:nonexistent"

	mockClient.On("Get", ctx, hashKey).Return("", errors.New("key not found"))

	result, err := repo.GetAPIKeyByHash(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get API key")
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_GetAPIKeyByID_Existing(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyID := "test-key-id"

	expectedAPIKey := &models.APIKey{
		ID:        keyID,
		UserID:    "test-user",
		TenantID:  tenantID,
		Name:      "Test Key",
		KeyHash:   "abcd1234hash",
		Prefix:    "sk_test_",
		IsActive:  true,
		Roles:     []string{"admin"},
		Scopes:    []string{"read", "write"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: "test-user",
		UpdatedBy: "test-user",
	}

	idKey := "apikey_id:test-tenant:test-key-id"
	hashKey := "apikey:test-tenant:abcd1234hash"
	data, _ := json.Marshal(expectedAPIKey)

	mockClient.On("Get", ctx, idKey).Return("abcd1234hash", nil)
	mockClient.On("Get", ctx, hashKey).Return(string(data), nil)

	result, err := repo.GetAPIKeyByID(ctx, tenantID, keyID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, keyID, result.ID)
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_GetAPIKeyByID_NonExistent(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyID := "nonexistent"

	idKey := "apikey_id:test-tenant:nonexistent"

	mockClient.On("Get", ctx, idKey).Return("", errors.New("key not found"))

	result, err := repo.GetAPIKeyByID(ctx, tenantID, keyID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get API key hash")
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_ListAPIKeys_Multiple(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	// Mock keys found
	keys := []string{"apikey_id:test-tenant:key1", "apikey_id:test-tenant:key2"}
	mockClient.On("Keys", ctx, "apikey_id:test-tenant:*").Return(keys, nil)

	// Mock API key data
	jsonData1 := `{"id":"key1","userId":"test-user","tenantId":"test-tenant","name":"Key 1","keyHash":"hash1","isActive":true,"roles":["admin"],"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}`
	jsonData2 := `{"id":"key2","userId":"test-user","tenantId":"test-tenant","name":"Key 2","keyHash":"hash2","isActive":true,"roles":["viewer"],"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}`

	mockClient.On("Get", ctx, "apikey_id:test-tenant:key1").Return("hash1", nil)
	mockClient.On("Get", ctx, "apikey:test-tenant:hash1").Return(jsonData1, nil)
	mockClient.On("Get", ctx, "apikey_id:test-tenant:key2").Return("hash2", nil)
	mockClient.On("Get", ctx, "apikey:test-tenant:hash2").Return(jsonData2, nil)

	result, err := repo.ListAPIKeys(ctx, tenantID, userID)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "key1", result[0].ID)
	assert.Equal(t, "key2", result[1].ID)
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_ListAPIKeys_NoKeys(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	mockClient.On("Keys", ctx, "apikey_id:test-tenant:*").Return([]string{}, nil)

	result, err := repo.ListAPIKeys(ctx, tenantID, userID)

	require.NoError(t, err)
	assert.Len(t, result, 0)
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_UpdateAPIKey_Success(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	apiKey := &models.APIKey{
		ID:        "test-key-id",
		UserID:    "test-user",
		TenantID:  "test-tenant",
		Name:      "Updated Test Key",
		KeyHash:   "abcd1234hash",
		Prefix:    "sk_test_",
		IsActive:  true,
		Roles:     []string{"admin", "editor"},
		Scopes:    []string{"read", "write", "delete"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: "test-user",
		UpdatedBy: "test-user",
	}

	hashKey := "apikey:test-tenant:abcd1234hash"

	mockClient.On("Set", ctx, hashKey, mock.AnythingOfType("string"), time.Duration(0)).Return(nil)

	err := repo.UpdateAPIKey(ctx, apiKey)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_RevokeAPIKey_Success(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyID := "test-key-id"

	idKey := "apikey_id:test-tenant:test-key-id"
	hashKey := "apikey:test-tenant:abcd1234hash"
	jsonData := `{"id":"test-key-id","userId":"test-user","tenantId":"test-tenant","keyHash":"abcd1234hash","isActive":true,"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}`

	mockClient.On("Get", ctx, idKey).Return("abcd1234hash", nil)
	mockClient.On("Get", ctx, hashKey).Return(jsonData, nil)
	mockClient.On("Set", ctx, hashKey, mock.AnythingOfType("string"), time.Duration(0)).Return(nil)

	err := repo.RevokeAPIKey(ctx, tenantID, keyID)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_ValidateAPIKey_Active(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "abcd1234hash"

	jsonData := `{"id":"test-key-id","userId":"test-user","tenantId":"test-tenant","keyHash":"abcd1234hash","isActive":true,"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}`

	hashKey := "apikey:test-tenant:abcd1234hash"

	mockClient.On("Get", ctx, hashKey).Return(jsonData, nil)
	mockClient.On("Set", ctx, hashKey, mock.AnythingOfType("string"), time.Duration(0)).Return(nil)

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, keyHash, result.KeyHash)
	assert.True(t, result.IsActive)
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_ValidateAPIKey_Revoked(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "abcd1234hash"

	jsonData := `{"id":"test-key-id","userId":"test-user","tenantId":"test-tenant","keyHash":"abcd1234hash","isActive":false,"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}`

	hashKey := "apikey:test-tenant:abcd1234hash"

	mockClient.On("Get", ctx, hashKey).Return(jsonData, nil)

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API key is revoked")
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_ValidateAPIKey_Expired(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "abcd1234hash"

	expiredAt := time.Now().Add(-1 * time.Hour)
	jsonData := fmt.Sprintf(`{"id":"test-key-id","userId":"test-user","tenantId":"test-tenant","keyHash":"abcd1234hash","isActive":true,"expiresAt":"%s","createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}`, expiredAt.Format(time.RFC3339))

	hashKey := "apikey:test-tenant:abcd1234hash"

	mockClient.On("Get", ctx, hashKey).Return(jsonData, nil)

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API key is expired")
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_ValidateAPIKey_NonExistent(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "nonexistent"

	hashKey := "apikey:test-tenant:nonexistent"

	mockClient.On("Get", ctx, hashKey).Return("", errors.New("key not found"))

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get API key")
	mockClient.AssertExpectations(t)
}

func TestValkeyRBACRepository_ValidateAPIKey_WrongTenant(t *testing.T) {
	mockClient := &MockValkeyClient{}
	repo := NewValkeyRBACRepository(mockClient)

	ctx := context.Background()
	tenantID := "wrong-tenant"
	keyHash := "abcd1234hash"

	hashKey := "apikey:wrong-tenant:abcd1234hash"

	mockClient.On("Get", ctx, hashKey).Return("", errors.New("key not found"))

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get API key")
	mockClient.AssertExpectations(t)
}
