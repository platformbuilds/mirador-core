package rbac

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TestTenantUserBasicOperations tests basic tenant-user operations with mocks
func TestTenantUserBasicOperations(t *testing.T) {
	ctx := context.Background()
	tenantID := "test-tenant"
	userID := "test-user"

	t.Run("CreateTenantUser", func(t *testing.T) {
		mockRepo := &MockRBACRepository{}
		mockCache := &MockCacheRepository{}
		auditService := NewAuditService(mockRepo)
		mockLogger := logger.NewMockLogger(&strings.Builder{})
		service := NewRBACService(mockRepo, mockCache, auditService, mockLogger)

		tenantUser := &models.TenantUser{
			TenantID:   tenantID,
			UserID:     userID,
			TenantRole: "tenant_editor",
			Status:     "active",
			CreatedBy:  "system",
		}

		// Mock expectations
		mockRepo.On("GetTenant", ctx, tenantID).Return(&models.Tenant{ID: tenantID}, nil)
		mockRepo.On("GetTenantUser", ctx, tenantID, userID).Return(nil, nil) // No existing association
		mockRepo.On("CreateTenantUser", ctx, tenantUser).Return(nil).Run(func(args mock.Arguments) {
			created := args.Get(1).(*models.TenantUser)
			created.CreatedAt = time.Now()
			created.UpdatedAt = time.Now()
		})
		mockRepo.On("LogAuditEvent", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil).Maybe()

		result, err := service.CreateTenantUser(ctx, tenantUser, "test-correlation-id")

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, tenantID, result.TenantID)
		assert.Equal(t, userID, result.UserID)
		assert.Equal(t, "tenant_editor", result.TenantRole)

		mockRepo.AssertExpectations(t)
	})

	t.Run("GetTenantUser", func(t *testing.T) {
		mockRepo := &MockRBACRepository{}
		mockCache := &MockCacheRepository{}
		auditService := NewAuditService(mockRepo)
		mockLogger := logger.NewMockLogger(&strings.Builder{})
		service := NewRBACService(mockRepo, mockCache, auditService, mockLogger)

		expectedTenantUser := &models.TenantUser{
			TenantID:   tenantID,
			UserID:     userID,
			TenantRole: "tenant_editor",
			Status:     "active",
		}

		mockRepo.On("GetTenantUser", ctx, tenantID, userID).Return(expectedTenantUser, nil)

		result, err := service.GetTenantUser(ctx, tenantID, userID)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, tenantID, result.TenantID)
		assert.Equal(t, userID, result.UserID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("ListTenantUsers", func(t *testing.T) {
		mockRepo := &MockRBACRepository{}
		mockCache := &MockCacheRepository{}
		auditService := NewAuditService(mockRepo)
		mockLogger := logger.NewMockLogger(&strings.Builder{})
		service := NewRBACService(mockRepo, mockCache, auditService, mockLogger)

		expectedUsers := []*models.TenantUser{
			{
				TenantID:   tenantID,
				UserID:     userID,
				TenantRole: "tenant_editor",
				Status:     "active",
			},
		}

		mockRepo.On("GetTenant", ctx, tenantID).Return(&models.Tenant{ID: tenantID}, nil)
		mockRepo.On("ListTenantUsers", ctx, tenantID, mock.AnythingOfType("TenantUserFilters")).Return(expectedUsers, nil)

		result, err := service.ListTenantUsers(ctx, tenantID, nil)

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, userID, result[0].UserID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("UpdateTenantUser", func(t *testing.T) {
		mockRepo := &MockRBACRepository{}
		mockCache := &MockCacheRepository{}
		auditService := NewAuditService(mockRepo)
		mockLogger := logger.NewMockLogger(&strings.Builder{})
		service := NewRBACService(mockRepo, mockCache, auditService, mockLogger)

		existing := &models.TenantUser{
			TenantID:   tenantID,
			UserID:     userID,
			TenantRole: "tenant_editor",
			Status:     "active",
			CreatedBy:  "system",
			UpdatedBy:  "system",
		}

		updateData := &models.TenantUser{
			TenantID:   tenantID, // Required for validation
			UserID:     userID,   // Required for validation
			TenantRole: "tenant_admin",
			Status:     "active",
			UpdatedBy:  "system",
		}

		mockRepo.On("GetTenantUser", ctx, tenantID, userID).Return(existing, nil)
		mockRepo.On("UpdateTenantUser", ctx, mock.AnythingOfType("*models.TenantUser")).Return(nil).Run(func(args mock.Arguments) {
			updated := args.Get(1).(*models.TenantUser)
			updated.TenantRole = updateData.TenantRole
			updated.UpdatedBy = updateData.UpdatedBy
			updated.UpdatedAt = time.Now()
		})
		mockRepo.On("LogAuditEvent", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil).Maybe()

		result, err := service.UpdateTenantUser(ctx, tenantID, userID, updateData, "update-test")

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "tenant_admin", result.TenantRole)

		mockRepo.AssertExpectations(t)
	})

	t.Run("DeleteTenantUser", func(t *testing.T) {
		mockRepo := &MockRBACRepository{}
		mockCache := &MockCacheRepository{}
		auditService := NewAuditService(mockRepo)
		mockLogger := logger.NewMockLogger(&strings.Builder{})
		service := NewRBACService(mockRepo, mockCache, auditService, mockLogger)

		existing := &models.TenantUser{
			TenantID:   tenantID,
			UserID:     userID,
			TenantRole: "tenant_editor",
			Status:     "active",
		}

		mockRepo.On("GetTenantUser", ctx, tenantID, userID).Return(existing, nil)
		mockRepo.On("DeleteTenantUser", ctx, tenantID, userID).Return(nil)
		mockRepo.On("LogAuditEvent", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil).Maybe()

		err := service.DeleteTenantUser(ctx, tenantID, userID, "delete-test")

		require.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})
}
