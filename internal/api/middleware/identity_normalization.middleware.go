package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// IdentityNormalizationService handles identity normalization across providers
type IdentityNormalizationService struct {
	config *config.Config
	cache  cache.ValkeyCluster
	repo   repo.SchemaStore
	logger logger.Logger
}

// NewIdentityNormalizationService creates a new identity normalization service
func NewIdentityNormalizationService(cfg *config.Config, cache cache.ValkeyCluster, repo repo.SchemaStore, logger logger.Logger) *IdentityNormalizationService {
	return &IdentityNormalizationService{
		config: cfg,
		cache:  cache,
		repo:   repo,
		logger: logger,
	}
}

// IdentityNormalizationMiddleware normalizes user identities across authentication providers
func (ins *IdentityNormalizationService) IdentityNormalizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user identity from context
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		// Get auth method from context
		authMethod, _ := c.Get("auth_method")

		// Normalize and persist identity
		if err := ins.normalizeAndPersistIdentity(c.Request.Context(), userID.(string), authMethod.(string)); err != nil {
			ins.logger.Error("Identity normalization failed", "user_id", userID, "error", err)
			// Don't fail the request, just log the error
		}

		c.Next()
	}
}

// normalizeAndPersistIdentity normalizes user identity and stores in system
func (ins *IdentityNormalizationService) normalizeAndPersistIdentity(ctx context.Context, userID, authMethod string) error {
	// Get existing user identity
	user, err := ins.getUserByID(ctx, userID)
	if err != nil {
		ins.logger.Warn("User not found for normalization", "user_id", userID, "error", err)
		return err
	}

	// Normalize identity based on auth method
	normalizedID, err := ins.normalizeIdentity(user, authMethod)
	if err != nil {
		return fmt.Errorf("identity normalization failed: %w", err)
	}

	// Check if identity mapping already exists (placeholder)
	existingMapping, err := ins.getIdentityMapping(ctx, normalizedID)
	if err == nil && existingMapping != nil {
		// Update existing mapping
		if err := ins.updateIdentityMapping(ctx, existingMapping, user, authMethod); err != nil {
			return fmt.Errorf("identity mapping update failed: %w", err)
		}
	} else {
		// Create new identity mapping
		now := time.Now()
		mapping := &models.IdentityMapping{
			NormalizedID:   normalizedID,
			ProviderUserID: userID,
			AuthProvider:   authMethod,
			User:           user,
			CreatedAt:      now,
			LastLoginAt:    &now,
			LoginCount:     1,
		}

		if err := ins.createIdentityMapping(ctx, mapping); err != nil {
			return fmt.Errorf("identity mapping creation failed: %w", err)
		}
	}

	// Update user last login
	if err := ins.updateUserLastLogin(ctx, userID); err != nil {
		ins.logger.Warn("Failed to update user last login", "user_id", userID, "error", err)
	}

	return nil
}

// normalizeIdentity creates a normalized identity from user information
func (ins *IdentityNormalizationService) normalizeIdentity(user *models.User, authMethod string) (string, error) {
	var identityComponents []string

	// Use email as primary identifier if available
	if user.Email != "" {
		identityComponents = append(identityComponents, strings.ToLower(user.Email))
	}

	// Add provider-specific normalization
	switch authMethod {
	case "saml":
		// For SAML, use email or username
		if user.Email != "" {
			identityComponents = append(identityComponents, "saml_"+strings.ToLower(user.Email))
		}
	case "jwt":
		// For JWT, use email or username
		if user.Email != "" {
			identityComponents = append(identityComponents, "jwt_"+strings.ToLower(user.Email))
		}
	case "local":
		// For local auth, use email
		if user.Email != "" {
			identityComponents = append(identityComponents, "local_"+strings.ToLower(user.Email))
		}
	}

	if len(identityComponents) == 0 {
		return "", fmt.Errorf("insufficient identity information for normalization")
	}

	// Create normalized ID using SHA256 hash of components
	h := sha256.New()
	for _, component := range identityComponents {
		h.Write([]byte(component))
		h.Write([]byte("|"))
	}

	normalizedID := fmt.Sprintf("%x", h.Sum(nil))
	return normalizedID, nil
}

// Placeholder methods (would be implemented with actual storage calls)
func (ins *IdentityNormalizationService) getIdentityMapping(ctx context.Context, normalizedID string) (*models.IdentityMapping, error) {
	// Placeholder - would query storage
	return nil, fmt.Errorf("not implemented")
}

func (ins *IdentityNormalizationService) createIdentityMapping(ctx context.Context, mapping *models.IdentityMapping) error {
	// Placeholder - would create identity mapping in storage
	ins.logger.Info("Identity mapping created", "normalized_id", mapping.NormalizedID, "provider", mapping.AuthProvider)
	return nil
}

func (ins *IdentityNormalizationService) updateIdentityMapping(ctx context.Context, mapping *models.IdentityMapping, user *models.User, authMethod string) error {
	// Placeholder - would update identity mapping in storage
	ins.logger.Info("Identity mapping updated", "normalized_id", mapping.NormalizedID, "provider", authMethod)
	return nil
}

func (ins *IdentityNormalizationService) updateUserLastLogin(ctx context.Context, userID string) error {
	// Placeholder - would update user last login in storage
	return nil
}

// LinkIdentities links multiple identities belonging to the same user
func (ins *IdentityNormalizationService) LinkIdentities(ctx context.Context, primaryUserID string, linkedUserIDs []string) error {
	// Placeholder implementation
	ins.logger.Info("Identity linking requested", "primary_user", primaryUserID, "linked_users", linkedUserIDs)
	return nil
}

// GetLinkedIdentities returns all identities linked to a user
func (ins *IdentityNormalizationService) GetLinkedIdentities(ctx context.Context, userID string) ([]*models.IdentityMapping, error) {
	// Placeholder implementation
	return []*models.IdentityMapping{}, nil
}

// ResolveUserID resolves a user ID from any linked identity
func (ins *IdentityNormalizationService) ResolveUserID(ctx context.Context, providerUserID, authProvider string) (string, error) {
	// Placeholder implementation
	return "", fmt.Errorf("identity resolution not implemented")
}

// getUserByID retrieves a user by ID
func (ins *IdentityNormalizationService) getUserByID(ctx context.Context, userID string) (*models.User, error) {
	// Placeholder - would query storage
	return &models.User{
		ID:       userID,
		Email:    "user@example.com",
		FullName: "John Doe",
	}, nil
}
