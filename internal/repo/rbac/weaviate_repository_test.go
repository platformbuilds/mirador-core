package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// MockWeaviateTransport is a mock implementation of the Weaviate Transport interface for testing
type MockWeaviateTransport struct {
	mock.Mock
}

func (m *MockWeaviateTransport) Ready(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockWeaviateTransport) EnsureClasses(ctx context.Context, classDefs []map[string]any) error {
	args := m.Called(ctx, classDefs)
	return args.Error(0)
}

func (m *MockWeaviateTransport) PutObject(ctx context.Context, class, id string, props map[string]any) error {
	args := m.Called(ctx, class, id, props)
	return args.Error(0)
}

func (m *MockWeaviateTransport) GraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	args := m.Called(ctx, query, variables, out)
	return args.Error(0)
}

func (m *MockWeaviateTransport) GetSchema(ctx context.Context, out any) error {
	args := m.Called(ctx, out)
	return args.Error(0)
}

func (m *MockWeaviateTransport) DeleteObject(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestWeaviateRBACRepository_CreateAPIKey_Success(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

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

	// Mock schema check and object creation
	mockTransport.On("EnsureClasses", ctx, mock.Anything).Return(nil)
	mockTransport.On("PutObject", ctx, "RBACAPIKey", mock.AnythingOfType("string"), mock.MatchedBy(func(props map[string]any) bool {
		// Verify key properties
		return props["userId"] == "test-user" &&
			props["tenantId"] == "test-tenant" &&
			props["keyHash"] == "abcd1234hash" &&
			props["isActive"] == true
	})).Return(nil)

	err := repo.CreateAPIKey(ctx, apiKey)

	require.NoError(t, err)
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_CreateAPIKey_Failure(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

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

	// Mock schema check success, but PutObject failure
	mockTransport.On("EnsureClasses", ctx, mock.Anything).Return(nil)
	mockTransport.On("PutObject", ctx, "RBACAPIKey", mock.AnythingOfType("string"), mock.Anything).Return(errors.New("weaviate error"))

	err := repo.CreateAPIKey(ctx, apiKey)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create API key")
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_GetAPIKeyByHash_Existing(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "abcd1234hash"

	expectedAPIKey := &models.APIKey{
		ID:        "test-key-id",
		UserID:    "test-user",
		TenantID:  tenantID,
		Name:      "Test Key",
		KeyHash:   keyHash,
		Prefix:    "sk_test_",
		IsActive:  true,
		Roles:     []string{"admin"},
		Scopes:    []string{"read", "write"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: "test-user",
		UpdatedBy: "test-user",
	}

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{
					{
						"_additional": map[string]any{"id": "test-key-id"},
						"userId":      "test-user",
						"tenantId":    tenantID,
						"name":        "Test Key",
						"keyHash":     keyHash,
						"prefix":      "sk_test_",
						"isActive":    true,
						"roles":       []interface{}{"admin"},
						"scopes":      []interface{}{"read", "write"},
						"createdAt":   time.Now().Format(time.RFC3339),
						"updatedAt":   time.Now().Format(time.RFC3339),
						"createdBy":   "test-user",
						"updatedBy":   "test-user",
					},
				},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.GetAPIKeyByHash(ctx, tenantID, keyHash)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expectedAPIKey.ID, result.ID)
	assert.Equal(t, expectedAPIKey.KeyHash, result.KeyHash)
	assert.Equal(t, expectedAPIKey.TenantID, result.TenantID)
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_GetAPIKeyByHash_NonExistent(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "nonexistent"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.GetAPIKeyByHash(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API key not found")
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_GetAPIKeyByID_Existing(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyID := "test-key-id"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{
					{
						"_additional": map[string]any{"id": keyID},
						"userId":      "test-user",
						"tenantId":    tenantID,
						"name":        "Test Key",
						"keyHash":     "abcd1234hash",
						"prefix":      "sk_test_",
						"isActive":    true,
						"roles":       []interface{}{"admin"},
						"scopes":      []interface{}{"read", "write"},
						"createdAt":   time.Now().Format(time.RFC3339),
						"updatedAt":   time.Now().Format(time.RFC3339),
						"createdBy":   "test-user",
						"updatedBy":   "test-user",
					},
				},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.GetAPIKeyByID(ctx, tenantID, keyID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, keyID, result.ID)
	assert.Equal(t, tenantID, result.TenantID)
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_GetAPIKeyByID_NonExistent(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyID := "nonexistent"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.GetAPIKeyByID(ctx, tenantID, keyID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API key not found")
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_ListAPIKeys_Multiple(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{
					{
						"_additional": map[string]any{"id": "key1"},
						"userId":      userID,
						"tenantId":    tenantID,
						"name":        "Key 1",
						"keyHash":     "hash1",
						"isActive":    true,
						"roles":       []interface{}{"admin"},
						"createdAt":   time.Now().Format(time.RFC3339),
						"updatedAt":   time.Now().Format(time.RFC3339),
					},
					{
						"_additional": map[string]any{"id": "key2"},
						"userId":      userID,
						"tenantId":    tenantID,
						"name":        "Key 2",
						"keyHash":     "hash2",
						"isActive":    true,
						"roles":       []interface{}{"viewer"},
						"createdAt":   time.Now().Format(time.RFC3339),
						"updatedAt":   time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.ListAPIKeys(ctx, tenantID, userID)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "key1", result[0].ID)
	assert.Equal(t, "key2", result[1].ID)
	assert.Equal(t, userID, result[0].UserID)
	assert.Equal(t, userID, result[1].UserID)
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_ListAPIKeys_NoKeys(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.ListAPIKeys(ctx, tenantID, userID)

	require.NoError(t, err)
	assert.Len(t, result, 0)
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_UpdateAPIKey_Success(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	apiKey := &models.APIKey{
		ID:        "test-key-id",
		UserID:    "test-user",
		TenantID:  "test-tenant",
		Name:      "Updated Test Key",
		KeyHash:   "abcd1234hash",
		IsActive:  true,
		Roles:     []string{"admin", "editor"},
		Scopes:    []string{"read", "write", "delete"},
		UpdatedAt: time.Now(),
		UpdatedBy: "test-user",
	}

	mockTransport.On("PutObject", ctx, "RBACAPIKey", "test-key-id", mock.MatchedBy(func(props map[string]any) bool {
		return props["name"] == "Updated Test Key" &&
			props["isActive"] == true &&
			len(props["roles"].([]string)) == 2
	})).Return(nil)

	err := repo.UpdateAPIKey(ctx, apiKey)

	require.NoError(t, err)
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_RevokeAPIKey_Success(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyID := "test-key-id"

	// First, mock GetAPIKeyByID
	getResponse := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{
					{
						"_additional": map[string]any{"id": keyID},
						"userId":      "test-user",
						"tenantId":    tenantID,
						"keyHash":     "abcd1234hash",
						"isActive":    true,
						"createdAt":   time.Now().Format(time.RFC3339),
						"updatedAt":   time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.MatchedBy(func(query string) bool {
		return strings.Contains(query, "RBACAPIKey(where:")
	}), mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(getResponse)
		json.Unmarshal(jsonData, out)
	}).Return(nil).Once()

	// Then mock UpdateAPIKey
	mockTransport.On("PutObject", ctx, "RBACAPIKey", keyID, mock.Anything).Return(nil)

	err := repo.RevokeAPIKey(ctx, tenantID, keyID)

	require.NoError(t, err)
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_ValidateAPIKey_Active(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "abcd1234hash"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{
					{
						"_additional": map[string]any{"id": "test-key-id"},
						"userId":      "test-user",
						"tenantId":    tenantID,
						"keyHash":     keyHash,
						"isActive":    true,
						"roles":       []interface{}{"admin"},
						"createdAt":   time.Now().Format(time.RFC3339),
						"updatedAt":   time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	mockTransport.On("PutObject", ctx, "RBACAPIKey", "test-key-id", mock.Anything).Return(nil)

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, keyHash, result.KeyHash)
	assert.True(t, result.IsActive)
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_ValidateAPIKey_Revoked(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "abcd1234hash"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{
					{
						"_additional": map[string]any{"id": "test-key-id"},
						"userId":      "test-user",
						"tenantId":    tenantID,
						"keyHash":     keyHash,
						"isActive":    false, // Revoked
						"roles":       []interface{}{"admin"},
						"createdAt":   time.Now().Format(time.RFC3339),
						"updatedAt":   time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API key is revoked")
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_ValidateAPIKey_Expired(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "abcd1234hash"

	expiredTime := time.Now().Add(-1 * time.Hour)
	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{
					{
						"_additional": map[string]any{"id": "test-key-id"},
						"userId":      "test-user",
						"tenantId":    tenantID,
						"keyHash":     keyHash,
						"isActive":    true,
						"expiresAt":   expiredTime.Format(time.RFC3339),
						"roles":       []interface{}{"admin"},
						"createdAt":   time.Now().Format(time.RFC3339),
						"updatedAt":   time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API key is expired")
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_ValidateAPIKey_NonExistent(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	keyHash := "nonexistent"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API key not found")
	mockTransport.AssertExpectations(t)
}

func TestWeaviateRBACRepository_ValidateAPIKey_WrongTenant(t *testing.T) {
	mockTransport := &MockWeaviateTransport{}
	repo := &WeaviateRBACRepository{
		transport: mockTransport,
	}

	ctx := context.Background()
	tenantID := "wrong-tenant"
	keyHash := "abcd1234hash"

	response := map[string]any{
		"data": map[string]any{
			"Get": map[string]any{
				"RBACAPIKey": []map[string]any{},
			},
		},
	}

	mockTransport.On("GraphQL", ctx, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3)
		jsonData, _ := json.Marshal(response)
		json.Unmarshal(jsonData, out)
	}).Return(nil)

	result, err := repo.ValidateAPIKey(ctx, tenantID, keyHash)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API key not found")
	mockTransport.AssertExpectations(t)
}
