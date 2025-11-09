package models

import (
	"time"
)

// RBAC Models for Multi-Tenant Role-Based Access Control

// Tenant represents a tenant in the multi-tenant architecture
type Tenant struct {
	ID          string             `json:"id" weaviate:"id"`
	Name        string             `json:"name" weaviate:"name"`
	DisplayName string             `json:"displayName" weaviate:"displayName"`
	Description string             `json:"description" weaviate:"description"`
	Deployments []TenantDeployment `json:"deployments" weaviate:"deployments"`
	Status      string             `json:"status" weaviate:"status"` // active, suspended, pending_deletion
	AdminEmail  string             `json:"adminEmail" weaviate:"adminEmail"`
	AdminName   string             `json:"adminName" weaviate:"adminName"`
	Quotas      TenantQuotas       `json:"quotas" weaviate:"quotas"`
	Features    []string           `json:"features" weaviate:"features"`
	Metadata    map[string]string  `json:"metadata" weaviate:"metadata"`
	Tags        []string           `json:"tags" weaviate:"tags"`
	CreatedAt   time.Time          `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy   string             `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy   string             `json:"updatedBy" weaviate:"updatedBy"`
}

// TenantDeployment represents Victoria* deployment endpoints
type TenantDeployment struct {
	Environment     string   `json:"environment"`
	MetricsEndpoint string   `json:"metricsEndpoint"`
	LogsEndpoint    string   `json:"logsEndpoint"`
	TracesEndpoint  string   `json:"tracesEndpoint"`
	Priority        int      `json:"priority"`
	Tags            []string `json:"tags"`
}

// TenantQuotas represents tenant resource limits
type TenantQuotas struct {
	MaxUsers       int `json:"maxUsers"`
	MaxDashboards  int `json:"maxDashboards"`
	MaxKPIs        int `json:"maxKpis"`
	StorageLimitGB int `json:"storageLimitGb"`
	APIRateLimit   int `json:"apiRateLimit"`
}

// TenantUser represents the association between a user and a tenant
type TenantUser struct {
	ID                    string            `json:"id" weaviate:"id"`
	TenantID              string            `json:"tenantId" weaviate:"tenant"`
	UserID                string            `json:"userId" weaviate:"user"`
	TenantRole            string            `json:"tenantRole" weaviate:"tenantRole"` // tenant_admin, tenant_editor, tenant_guest
	Status                string            `json:"status" weaviate:"status"`         // active, invited, suspended, removed
	InvitedBy             string            `json:"invitedBy" weaviate:"invitedBy"`
	InvitedAt             *time.Time        `json:"invitedAt" weaviate:"invitedAt"`
	AcceptedAt            *time.Time        `json:"acceptedAt" weaviate:"acceptedAt"`
	AdditionalPermissions []string          `json:"additionalPermissions" weaviate:"additionalPermissions"`
	Metadata              map[string]string `json:"metadata" weaviate:"metadata"`
	CreatedAt             time.Time         `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt             time.Time         `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy             string            `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy             string            `json:"updatedBy" weaviate:"updatedBy"`
}

// User represents a global user entity
type User struct {
	ID               string            `json:"id" weaviate:"id"`
	Email            string            `json:"email" weaviate:"email"`
	Username         string            `json:"username" weaviate:"username"`
	FullName         string            `json:"fullName" weaviate:"fullName"`
	GlobalRole       string            `json:"globalRole" weaviate:"globalRole"` // global_admin, global_tenant_admin, tenant_user
	PasswordHash     string            `json:"-" weaviate:"passwordHash"`        // Never serialize in JSON
	MFAEnabled       bool              `json:"mfaEnabled" weaviate:"mfaEnabled"`
	MFASecret        string            `json:"-" weaviate:"mfaSecret"`   // Never serialize in JSON
	Status           string            `json:"status" weaviate:"status"` // active, suspended, pending_verification, deactivated
	EmailVerified    bool              `json:"emailVerified" weaviate:"emailVerified"`
	Avatar           string            `json:"avatar" weaviate:"avatar"`
	Phone            string            `json:"phone" weaviate:"phone"`
	Timezone         string            `json:"timezone" weaviate:"timezone"`
	Language         string            `json:"language" weaviate:"language"`
	LastLoginAt      *time.Time        `json:"lastLoginAt" weaviate:"lastLoginAt"`
	LoginCount       int               `json:"loginCount" weaviate:"loginCount"`
	FailedLoginCount int               `json:"failedLoginCount" weaviate:"failedLoginCount"`
	LockedUntil      *time.Time        `json:"lockedUntil" weaviate:"lockedUntil"`
	Metadata         map[string]string `json:"metadata" weaviate:"metadata"`
	Tags             []string          `json:"tags" weaviate:"tags"`
	CreatedAt        time.Time         `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy        string            `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy        string            `json:"updatedBy" weaviate:"updatedBy"`
}

// MiradorAuth represents local authentication credentials
type MiradorAuth struct {
	ID                    string            `json:"id" weaviate:"id"`
	UserID                string            `json:"userId" weaviate:"user"`
	Username              string            `json:"username" weaviate:"username"`
	Email                 string            `json:"email" weaviate:"email"`
	PasswordHash          string            `json:"-" weaviate:"passwordHash"` // Never serialize in JSON
	Salt                  string            `json:"-" weaviate:"salt"`         // Never serialize in JSON
	TOTPSecret            string            `json:"-" weaviate:"totpSecret"`   // Never serialize in JSON
	TOTPEnabled           bool              `json:"totpEnabled" weaviate:"totpEnabled"`
	BackupCodes           []string          `json:"-" weaviate:"backupCodes"` // Never serialize in JSON
	TenantID              string            `json:"tenantId" weaviate:"tenant"`
	Roles                 []string          `json:"roles" weaviate:"roles"`   // Role IDs
	Groups                []string          `json:"groups" weaviate:"groups"` // Group IDs
	IsActive              bool              `json:"isActive" weaviate:"isActive"`
	PasswordChangedAt     *time.Time        `json:"passwordChangedAt" weaviate:"passwordChangedAt"`
	PasswordExpiresAt     *time.Time        `json:"passwordExpiresAt" weaviate:"passwordExpiresAt"`
	LastLoginAt           *time.Time        `json:"lastLoginAt" weaviate:"lastLoginAt"`
	FailedLoginCount      int               `json:"failedLoginCount" weaviate:"failedLoginCount"`
	LockedUntil           *time.Time        `json:"lockedUntil" weaviate:"lockedUntil"`
	RequirePasswordChange bool              `json:"requirePasswordChange" weaviate:"requirePasswordChange"`
	Metadata              map[string]string `json:"metadata" weaviate:"metadata"`
	CreatedAt             time.Time         `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt             time.Time         `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy             string            `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy             string            `json:"updatedBy" weaviate:"updatedBy"`
}

// AuthConfig represents authentication configuration for a tenant
type AuthConfig struct {
	ID                    string             `json:"id" weaviate:"id"`
	TenantID              string             `json:"tenantId" weaviate:"tenant"`
	DefaultBackend        string             `json:"defaultBackend" weaviate:"defaultBackend"` // local, saml, oidc, ldap
	EnabledBackends       []string           `json:"enabledBackends" weaviate:"enabledBackends"`
	BackendConfigs        AuthBackendConfigs `json:"backendConfigs" weaviate:"backendConfigs"`
	PasswordPolicy        PasswordPolicy     `json:"passwordPolicy" weaviate:"passwordPolicy"`
	Require2FA            bool               `json:"require2fa" weaviate:"require2fa"`
	TOTPIssuer            string             `json:"totpIssuer" weaviate:"totpIssuer"`
	SessionTimeoutMinutes int                `json:"sessionTimeoutMinutes" weaviate:"sessionTimeoutMinutes"`
	MaxConcurrentSessions int                `json:"maxConcurrentSessions" weaviate:"maxConcurrentSessions"`
	AllowRememberMe       bool               `json:"allowRememberMe" weaviate:"allowRememberMe"`
	RememberMeDays        int                `json:"rememberMeDays" weaviate:"rememberMeDays"`
	Metadata              map[string]string  `json:"metadata" weaviate:"metadata"`
	CreatedAt             time.Time          `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt             time.Time          `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy             string             `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy             string             `json:"updatedBy" weaviate:"updatedBy"`
}

// AuthBackendConfigs contains configuration for different auth backends
type AuthBackendConfigs struct {
	SAML SAMLConfig `json:"saml"`
	OIDC OIDCConfig `json:"oidc"`
	LDAP LDAPConfig `json:"ldap"`
}

// SAMLConfig represents SAML authentication configuration
type SAMLConfig struct {
	EntityID         string            `json:"entityId"`
	ACSURL           string            `json:"acsUrl"`
	MetadataURL      string            `json:"metadataUrl"`
	SigningCert      string            `json:"signingCert"`
	EncryptionCert   string            `json:"encryptionCert"`
	NameIDFormat     string            `json:"nameIdFormat"`
	AttributeMapping map[string]string `json:"attributeMapping"`
}

// OIDCConfig represents OIDC authentication configuration
type OIDCConfig struct {
	ClientID         string            `json:"clientId"`
	ClientSecret     string            `json:"clientSecret"`
	IssuerURL        string            `json:"issuerUrl"`
	RedirectURL      string            `json:"redirectUrl"`
	Scopes           []string          `json:"scopes"`
	AttributeMapping map[string]string `json:"attributeMapping"`
}

// LDAPConfig represents LDAP authentication configuration
type LDAPConfig struct {
	Host             string            `json:"host"`
	Port             int               `json:"port"`
	UseTLS           bool              `json:"useTls"`
	BindDN           string            `json:"bindDn"`
	BindPassword     string            `json:"bindPassword"`
	BaseDN           string            `json:"baseDn"`
	UserFilter       string            `json:"userFilter"`
	GroupFilter      string            `json:"groupFilter"`
	AttributeMapping map[string]string `json:"attributeMapping"`
}

// PasswordPolicy represents password policy configuration
type PasswordPolicy struct {
	MinLength              int  `json:"minLength"`
	RequireUppercase       bool `json:"requireUppercase"`
	RequireLowercase       bool `json:"requireLowercase"`
	RequireNumbers         bool `json:"requireNumbers"`
	RequireSymbols         bool `json:"requireSymbols"`
	MaxAgeDays             int  `json:"maxAgeDays"`
	PreventReuseCount      int  `json:"preventReuseCount"`
	LockoutThreshold       int  `json:"lockoutThreshold"`
	LockoutDurationMinutes int  `json:"lockoutDurationMinutes"`
}

// Role represents a tenant-scoped role
type Role struct {
	ID          string            `json:"id" weaviate:"id"`
	Name        string            `json:"name" weaviate:"name"`
	Description string            `json:"description" weaviate:"description"`
	TenantID    string            `json:"tenantId" weaviate:"tenant"`
	Permissions []string          `json:"permissions" weaviate:"permissions"` // Permission IDs
	IsSystem    bool              `json:"isSystem" weaviate:"isSystem"`
	ParentRoles []string          `json:"parentRoles" weaviate:"parentRoles"` // Role IDs
	Metadata    map[string]string `json:"metadata" weaviate:"metadata"`
	CreatedAt   time.Time         `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy   string            `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy   string            `json:"updatedBy" weaviate:"updatedBy"`
}

// Permission represents granular permissions
type Permission struct {
	ID              string               `json:"id" weaviate:"id"`
	Resource        string               `json:"resource" weaviate:"resource"` // dashboard, kpi_definition, layout, user_prefs, admin, rbac
	Action          string               `json:"action" weaviate:"action"`     // create, read, update, delete, list, admin
	Scope           string               `json:"scope" weaviate:"scope"`       // global, tenant, resource
	Description     string               `json:"description" weaviate:"description"`
	ResourcePattern string               `json:"resourcePattern" weaviate:"resourcePattern"`
	Conditions      PermissionConditions `json:"conditions" weaviate:"conditions"`
	IsSystem        bool                 `json:"isSystem" weaviate:"isSystem"`
	Metadata        map[string]string    `json:"metadata" weaviate:"metadata"`
	CreatedAt       time.Time            `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy       string               `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy       string               `json:"updatedBy" weaviate:"updatedBy"`
}

// PermissionConditions represents ABAC conditions
type PermissionConditions struct {
	TimeBased      TimeBasedCondition      `json:"timeBased"`
	IPBased        []string                `json:"ipBased"`
	AttributeBased AttributeBasedCondition `json:"attributeBased"`
}

// TimeBasedCondition represents time-based access conditions
type TimeBasedCondition struct {
	AllowedHours []string `json:"allowedHours"` // "09:00-17:00"
	AllowedDays  []string `json:"allowedDays"`  // "monday", "tuesday", etc.
}

// AttributeBasedCondition represents user attribute requirements
type AttributeBasedCondition struct {
	Department     []string `json:"department"`
	ClearanceLevel string   `json:"clearanceLevel"`
}

// Group represents user groups for role assignment
type Group struct {
	ID                string            `json:"id" weaviate:"id"`
	Name              string            `json:"name" weaviate:"name"`
	Description       string            `json:"description" weaviate:"description"`
	TenantID          string            `json:"tenantId" weaviate:"tenant"`
	Members           []string          `json:"members" weaviate:"members"`           // User IDs
	Roles             []string          `json:"roles" weaviate:"roles"`               // Role IDs
	ParentGroups      []string          `json:"parentGroups" weaviate:"parentGroups"` // Group IDs
	IsSystem          bool              `json:"isSystem" weaviate:"isSystem"`
	MaxMembers        int               `json:"maxMembers" weaviate:"maxMembers"`
	MemberSyncEnabled bool              `json:"memberSyncEnabled" weaviate:"memberSyncEnabled"`
	ExternalID        string            `json:"externalId" weaviate:"externalId"`
	Metadata          map[string]string `json:"metadata" weaviate:"metadata"`
	CreatedAt         time.Time         `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy         string            `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy         string            `json:"updatedBy" weaviate:"updatedBy"`
}

// RoleBinding represents role assignments to users/groups
type RoleBinding struct {
	ID            string                `json:"id" weaviate:"id"`
	SubjectType   string                `json:"subjectType" weaviate:"subjectType"` // user, group
	SubjectID     string                `json:"subjectId" weaviate:"subjectId"`
	RoleID        string                `json:"roleId" weaviate:"role"`
	Scope         string                `json:"scope" weaviate:"scope"` // tenant, resource
	ResourceID    string                `json:"resourceId" weaviate:"resourceId"`
	ExpiresAt     *time.Time            `json:"expiresAt" weaviate:"expiresAt"`
	NotBefore     *time.Time            `json:"notBefore" weaviate:"notBefore"`
	Precedence    string                `json:"precedence" weaviate:"precedence"` // allow, deny
	Conditions    RoleBindingConditions `json:"conditions" weaviate:"conditions"`
	Justification string                `json:"justification" weaviate:"justification"`
	ApprovedBy    string                `json:"approvedBy" weaviate:"approvedBy"`
	ApprovedAt    *time.Time            `json:"approvedAt" weaviate:"approvedAt"`
	Metadata      map[string]string     `json:"metadata" weaviate:"metadata"`
	CreatedAt     time.Time             `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy     string                `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy     string                `json:"updatedBy" weaviate:"updatedBy"`
}

// RoleBindingConditions represents binding conditions
type RoleBindingConditions struct {
	IPRanges    []string              `json:"ipRanges"`
	TimeWindows []TimeWindowCondition `json:"timeWindows"`
	DeviceTypes []string              `json:"deviceTypes"`
	RiskLevels  []string              `json:"riskLevels"`
}

// TimeWindowCondition represents time window restrictions
type TimeWindowCondition struct {
	DaysOfWeek []string `json:"daysOfWeek"`
	StartTime  string   `json:"startTime"` // HH:MM
	EndTime    string   `json:"endTime"`   // HH:MM
}

// GroupBinding represents user membership in groups
type GroupBinding struct {
	ID            string            `json:"id" weaviate:"id"`
	UserID        string            `json:"userId" weaviate:"user"`
	GroupID       string            `json:"groupId" weaviate:"group"`
	TenantID      string            `json:"tenantId" weaviate:"tenant"`
	ExpiresAt     *time.Time        `json:"expiresAt" weaviate:"expiresAt"`
	NotBefore     *time.Time        `json:"notBefore" weaviate:"notBefore"`
	AddedBy       string            `json:"addedBy" weaviate:"addedBy"`
	AddedAt       *time.Time        `json:"addedAt" weaviate:"addedAt"`
	Justification string            `json:"justification" weaviate:"justification"`
	SyncSource    string            `json:"syncSource" weaviate:"syncSource"` // manual, ldap_sync, scim
	Metadata      map[string]string `json:"metadata" weaviate:"metadata"`
	CreatedAt     time.Time         `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy     string            `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy     string            `json:"updatedBy" weaviate:"updatedBy"`
}

// AuditLog represents audit log entries
type AuditLog struct {
	ID             string          `json:"id" weaviate:"id"`
	Timestamp      time.Time       `json:"timestamp" weaviate:"timestamp"`
	TenantID       string          `json:"tenantId" weaviate:"tenant"`
	SubjectID      string          `json:"subjectId" weaviate:"subject"`
	SubjectType    string          `json:"subjectType" weaviate:"subjectType"` // user, service_account, system
	Action         string          `json:"action" weaviate:"action"`
	Resource       string          `json:"resource" weaviate:"resource"`
	ResourceID     string          `json:"resourceId" weaviate:"resourceId"`
	Result         string          `json:"result" weaviate:"result"` // success, failure, denied, error
	Details        AuditLogDetails `json:"details" weaviate:"details"`
	Severity       string          `json:"severity" weaviate:"severity"` // low, medium, high, critical
	Source         string          `json:"source" weaviate:"source"`     // api, auth, rbac, system
	CorrelationID  string          `json:"correlationId" weaviate:"correlationId"`
	RetentionClass string          `json:"retentionClass" weaviate:"retentionClass"` // standard, extended, permanent
}

// AuditLogDetails contains structured audit details
type AuditLogDetails struct {
	UserAgent    string                 `json:"userAgent"`
	IPAddress    string                 `json:"ipAddress"`
	SessionID    string                 `json:"sessionId"`
	RequestID    string                 `json:"requestId"`
	Method       string                 `json:"method"`
	Endpoint     string                 `json:"endpoint"`
	OldValues    map[string]interface{} `json:"oldValues"`
	NewValues    map[string]interface{} `json:"newValues"`
	ErrorMessage string                 `json:"errorMessage"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// IdentityMapping represents identity normalization across authentication providers
type IdentityMapping struct {
	ID                   string            `json:"id" weaviate:"id"`
	NormalizedID         string            `json:"normalizedId" weaviate:"normalizedId"`
	ProviderUserID       string            `json:"providerUserId" weaviate:"providerUserId"`
	AuthProvider         string            `json:"authProvider" weaviate:"authProvider"` // local, saml, jwt, oidc, ldap
	User                 *User             `json:"user" weaviate:"user"`
	TenantID             string            `json:"tenantId" weaviate:"tenant"`
	ProviderAttributes   map[string]string `json:"providerAttributes" weaviate:"providerAttributes"`
	LastLoginAt          *time.Time        `json:"lastLoginAt" weaviate:"lastLoginAt"`
	LoginCount           int               `json:"loginCount" weaviate:"loginCount"`
	FirstLoginAt         time.Time         `json:"firstLoginAt" weaviate:"firstLoginAt"`
	AccountStatus        string            `json:"accountStatus" weaviate:"accountStatus"`               // active, suspended, deactivated
	IdentityVerification string            `json:"identityVerification" weaviate:"identityVerification"` // verified, unverified, pending
	RiskScore            float64           `json:"riskScore" weaviate:"riskScore"`
	Metadata             map[string]string `json:"metadata" weaviate:"metadata"`
	CreatedAt            time.Time         `json:"createdAt" weaviate:"createdAt"`
	UpdatedAt            time.Time         `json:"updatedAt" weaviate:"updatedAt"`
	CreatedBy            string            `json:"createdBy" weaviate:"createdBy"`
	UpdatedBy            string            `json:"updatedBy" weaviate:"updatedBy"`
}
