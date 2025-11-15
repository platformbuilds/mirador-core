package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID         string     `json:"id" weaviate:"id"`
	UserID     string     `json:"userId" weaviate:"userId"`
	TenantID   string     `json:"tenantId" weaviate:"tenantId"`
	Name       string     `json:"name" weaviate:"name"`                     // User-friendly name/label
	KeyHash    string     `json:"-" weaviate:"keyHash"`                     // SHA256 hash of the API key
	Prefix     string     `json:"prefix" weaviate:"prefix"`                 // First 8 chars for identification
	IsActive   bool       `json:"isActive" weaviate:"isActive"`             // Revocation flag
	ExpiresAt  *time.Time `json:"expiresAt,omitempty" weaviate:"expiresAt"` // nil means no expiry
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty" weaviate:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy  string     `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy  string     `json:"updatedBy" weaviate:"updatedBy"`

	// Permissions and scopes
	Roles    []string          `json:"roles,omitempty" weaviate:"roles"`       // Inherited user roles
	Scopes   []string          `json:"scopes,omitempty" weaviate:"scopes"`     // API access scopes
	Metadata map[string]string `json:"metadata,omitempty" weaviate:"metadata"` // Additional metadata
}

// APIKeyGenerateRequest represents a request to generate an API key
type APIKeyGenerateRequest struct {
	Name      string     `json:"name" binding:"required"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Scopes    []string   `json:"scopes,omitempty"`
}

// APIKeyGenerateResponse contains the generated API key (only returned once)
type APIKeyGenerateResponse struct {
	APIKey  *APIKey `json:"apiKey"`
	RawKey  string  `json:"rawKey"`  // Only returned on creation
	Warning string  `json:"warning"` // Security warning about storing the key
}

// APIKeyListResponse represents the response for listing API keys
type APIKeyListResponse struct {
	Keys  []*APIKey `json:"keys"`
	Total int       `json:"total"`
}

// APIKeyLimits represents API key limits configuration
type APIKeyLimits struct {
	ID                    string    `json:"id" weaviate:"id"`
	TenantID              string    `json:"tenantId" weaviate:"tenantId"`
	MaxKeysPerUser        int       `json:"maxKeysPerUser" weaviate:"maxKeysPerUser"`
	MaxKeysPerTenantAdmin int       `json:"maxKeysPerTenantAdmin" weaviate:"maxKeysPerTenantAdmin"`
	MaxKeysPerGlobalAdmin int       `json:"maxKeysPerGlobalAdmin" weaviate:"maxKeysPerGlobalAdmin"`
	CreatedAt             time.Time `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy             string    `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy             string    `json:"updatedBy" weaviate:"updatedBy"`
}

// APIKeyLimitsRequest represents a request to update API key limits
type APIKeyLimitsRequest struct {
	MaxKeysPerUser        int `json:"maxKeysPerUser" binding:"min=1,max=100"`
	MaxKeysPerTenantAdmin int `json:"maxKeysPerTenantAdmin" binding:"min=1,max=200"`
	MaxKeysPerGlobalAdmin int `json:"maxKeysPerGlobalAdmin" binding:"min=1,max=500"`
}

// GetDefaultAPIKeyLimits returns default API key limits from configuration
// If config is nil, returns hardcoded fallback values
func GetDefaultAPIKeyLimits(config interface{}) *APIKeyLimits {
	// Type assertion for config interface
	if cfg, ok := config.(APIKeyLimitsConfig); ok && cfg.Enabled {
		return &APIKeyLimits{
			MaxKeysPerUser:        cfg.DefaultLimits.MaxKeysPerUser,
			MaxKeysPerTenantAdmin: cfg.DefaultLimits.MaxKeysPerTenantAdmin,
			MaxKeysPerGlobalAdmin: cfg.DefaultLimits.MaxKeysPerGlobalAdmin,
			CreatedAt:             time.Now(),
			UpdatedAt:             time.Now(),
		}
	}

	// Fallback to hardcoded defaults if no config provided
	return &APIKeyLimits{
		MaxKeysPerUser:        10,  // Regular users
		MaxKeysPerTenantAdmin: 25,  // Tenant admins
		MaxKeysPerGlobalAdmin: 100, // Global admins
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}

// APIKeyLimitsConfig represents configuration structure for API key limits
// This is imported from config package to avoid circular imports
type APIKeyLimitsConfig struct {
	Enabled              bool
	DefaultLimits        DefaultAPIKeyLimits
	TenantLimits         []TenantAPIKeyLimits
	GlobalLimitsOverride *GlobalAPIKeyLimits
	AllowTenantOverride  bool
	AllowAdminOverride   bool
	MaxExpiryDays        int
	MinExpiryDays        int
	EnforceExpiry        bool
}

type DefaultAPIKeyLimits struct {
	MaxKeysPerUser        int
	MaxKeysPerTenantAdmin int
	MaxKeysPerGlobalAdmin int
}

type TenantAPIKeyLimits struct {
	TenantID              string
	MaxKeysPerUser        int
	MaxKeysPerTenantAdmin int
	MaxKeysPerGlobalAdmin int
}

type GlobalAPIKeyLimits struct {
	MaxKeysPerUser        int
	MaxKeysPerTenantAdmin int
	MaxKeysPerGlobalAdmin int
	MaxTotalKeys          int
}

// GetMaxKeysForRoles determines the maximum API keys allowed based on user roles
func (limits *APIKeyLimits) GetMaxKeysForRoles(roles []string) int {
	// Check for global admin first (highest privilege)
	for _, role := range roles {
		if role == "global_admin" {
			return limits.MaxKeysPerGlobalAdmin
		}
	}

	// Check for tenant admin
	for _, role := range roles {
		if role == "tenant_admin" {
			return limits.MaxKeysPerTenantAdmin
		}
	}

	// Default to regular user limit
	return limits.MaxKeysPerUser
}

// GenerateAPIKey creates a new API key with secure random generation
func GenerateAPIKey() (string, error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}

	// Create a readable format: mrk_<base64url>
	key := "mrk_" + hex.EncodeToString(bytes)
	return key, nil
}

// HashAPIKey creates a SHA256 hash of the API key for secure storage
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// ExtractKeyPrefix returns the first 8 characters for identification
func ExtractKeyPrefix(key string) string {
	if len(key) < 8 {
		return key
	}
	return key[:8]
}

// IsExpired checks if the API key has expired
func (ak *APIKey) IsExpired() bool {
	if ak.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*ak.ExpiresAt)
}

// IsValid checks if the API key is active and not expired
func (ak *APIKey) IsValid() bool {
	return ak.IsActive && !ak.IsExpired()
}

// UpdateLastUsed updates the last used timestamp
func (ak *APIKey) UpdateLastUsed() {
	now := time.Now()
	ak.LastUsedAt = &now
	ak.UpdatedAt = now
}

// ValidateExpiryWithConfig validates API key expiry against configuration constraints
func ValidateExpiryWithConfig(expiresAt *time.Time, config APIKeyLimitsConfig) error {
	if !config.Enabled {
		return nil // Skip validation if API keys are disabled
	}

	// Check if expiry is required
	if config.EnforceExpiry && expiresAt == nil {
		return fmt.Errorf("API key expiry is required by configuration")
	}

	// Skip further validation if no expiry set
	if expiresAt == nil {
		return nil
	}

	daysFromNow := int(time.Until(*expiresAt).Hours() / 24)

	// Check minimum expiry
	if config.MinExpiryDays > 0 && daysFromNow < config.MinExpiryDays {
		return fmt.Errorf("API key expiry must be at least %d days from now", config.MinExpiryDays)
	}

	// Check maximum expiry
	if config.MaxExpiryDays > 0 && daysFromNow > config.MaxExpiryDays {
		return fmt.Errorf("API key expiry cannot be more than %d days from now", config.MaxExpiryDays)
	}

	return nil
}

// ValidateRequest validates the API key generation request against configuration
func (req *APIKeyGenerateRequest) ValidateRequest(config APIKeyLimitsConfig) error {
	return ValidateExpiryWithConfig(req.ExpiresAt, config)
}

// GetAPIKeyLimitsForTenant returns limits for a specific tenant, considering configuration overrides
func GetAPIKeyLimitsForTenant(tenantID string, config APIKeyLimitsConfig) *APIKeyLimits {
	// Start with default limits
	limits := GetDefaultAPIKeyLimits(config)
	limits.TenantID = tenantID

	// Check for tenant-specific overrides
	for _, tenantLimit := range config.TenantLimits {
		if tenantLimit.TenantID == tenantID {
			limits.MaxKeysPerUser = tenantLimit.MaxKeysPerUser
			limits.MaxKeysPerTenantAdmin = tenantLimit.MaxKeysPerTenantAdmin
			limits.MaxKeysPerGlobalAdmin = tenantLimit.MaxKeysPerGlobalAdmin
			break
		}
	}

	// Apply global overrides if configured
	if config.GlobalLimitsOverride != nil {
		if config.GlobalLimitsOverride.MaxKeysPerUser > 0 {
			limits.MaxKeysPerUser = config.GlobalLimitsOverride.MaxKeysPerUser
		}
		if config.GlobalLimitsOverride.MaxKeysPerTenantAdmin > 0 {
			limits.MaxKeysPerTenantAdmin = config.GlobalLimitsOverride.MaxKeysPerTenantAdmin
		}
		if config.GlobalLimitsOverride.MaxKeysPerGlobalAdmin > 0 {
			limits.MaxKeysPerGlobalAdmin = config.GlobalLimitsOverride.MaxKeysPerGlobalAdmin
		}
	}

	return limits
}
