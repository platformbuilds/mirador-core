package rbac

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	storageweaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
)

// WeaviateRBACRepository implements RBACRepository using Weaviate
type WeaviateRBACRepository struct {
	transport storageweaviate.Transport
}

// UUID v5 (SHA-1) namespace for deterministic IDs
var nsMirador = mustParseUUID("6ba7b811-9dad-11d1-80b4-00c04fd430c8") // URL namespace (stable)

func makeID(parts ...string) string {
	name := strings.Join(parts, "|")
	return uuidV5(nsMirador, name)
}

func mustParseUUID(s string) [16]byte {
	b, ok := parseUUID(s)
	if !ok {
		panic("invalid UUID namespace: " + s)
	}
	return b
}

func parseUUID(s string) ([16]byte, bool) {
	var out [16]byte
	// remove hyphens
	hex := make([]byte, 0, 32)
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			continue
		}
		hex = append(hex, s[i])
	}
	if len(hex) != 32 {
		return out, false
	}
	// convert hex to bytes
	for i := 0; i < 16; i++ {
		hi := fromHex(hex[2*i])
		lo := fromHex(hex[2*i+1])
		if hi < 0 || lo < 0 {
			return out, false
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, true
}

func fromHex(b byte) int {
	switch {
	case '0' <= b && b <= '9':
		return int(b - '0')
	case 'a' <= b && b <= 'f':
		return int(b - 'a' + 10)
	case 'A' <= b && b <= 'F':
		return int(b - 'A' + 10)
	default:
		return -1
	}
}

func uuidV5(ns [16]byte, name string) string {
	// RFC 4122, version 5: SHA-1 of namespace + name
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	sum := h.Sum(nil) // 20 bytes
	var u [16]byte
	copy(u[:], sum[:16])
	// Set version (5) in high nibble of byte 6
	u[6] = (u[6] & 0x0f) | (5 << 4)
	// Set variant (RFC4122) in the two most significant bits of byte 8
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(u[0])<<24|uint32(u[1])<<16|uint32(u[2])<<8|uint32(u[3]),
		uint16(u[4])<<8|uint16(u[5]),
		uint16(u[6])<<8|uint16(u[7]),
		uint16(u[8])<<8|uint16(u[9]),
		(uint64(u[10])<<40)|(uint64(u[11])<<32)|(uint64(u[12])<<24)|(uint64(u[13])<<16)|(uint64(u[14])<<8)|uint64(u[15]),
	)
}

// NewWeaviateRBACRepository creates a new Weaviate-based RBAC repository
func NewWeaviateRBACRepository(transport storageweaviate.Transport) *WeaviateRBACRepository {
	return &WeaviateRBACRepository{
		transport: transport,
	}
}

// ensureRBACSchema ensures RBAC classes exist in Weaviate
func (r *WeaviateRBACRepository) ensureRBACSchema(ctx context.Context) error {
	// Define RBAC class schemas
	classes := []map[string]any{
		// Role class
		map[string]any{
			"class":      "RBACRole",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "tenantId", "dataType": []string{"text"}},
				{"name": "name", "dataType": []string{"text"}},
				{"name": "description", "dataType": []string{"text"}},
				{"name": "permissions", "dataType": []string{"text[]"}},
				{"name": "isSystem", "dataType": []string{"boolean"}},
				{"name": "parentRoles", "dataType": []string{"text[]"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
		// Permission class
		map[string]any{
			"class":      "RBACPermission",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "tenantId", "dataType": []string{"text"}},
				{"name": "resource", "dataType": []string{"text"}},
				{"name": "action", "dataType": []string{"text"}},
				{"name": "scope", "dataType": []string{"text"}},
				{"name": "description", "dataType": []string{"text"}},
				{"name": "resourcePattern", "dataType": []string{"text"}},
				{"name": "conditions", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "isSystem", "dataType": []string{"boolean"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
		// RoleBinding class
		map[string]any{
			"class":      "RBACRoleBinding",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "tenantId", "dataType": []string{"text"}},
				{"name": "subjectType", "dataType": []string{"text"}},
				{"name": "subjectId", "dataType": []string{"text"}},
				{"name": "roleId", "dataType": []string{"text"}},
				{"name": "scope", "dataType": []string{"text"}},
				{"name": "resourceId", "dataType": []string{"text"}},
				{"name": "expiresAt", "dataType": []string{"date"}},
				{"name": "notBefore", "dataType": []string{"date"}},
				{"name": "precedence", "dataType": []string{"text"}},
				{"name": "conditions", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "justification", "dataType": []string{"text"}},
				{"name": "approvedBy", "dataType": []string{"text"}},
				{"name": "approvedAt", "dataType": []string{"date"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
		// Group class
		map[string]any{
			"class":      "RBACGroup",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "tenantId", "dataType": []string{"text"}},
				{"name": "name", "dataType": []string{"text"}},
				{"name": "description", "dataType": []string{"text"}},
				{"name": "members", "dataType": []string{"text[]"}},
				{"name": "roles", "dataType": []string{"text[]"}},
				{"name": "parentGroups", "dataType": []string{"text[]"}},
				{"name": "isSystem", "dataType": []string{"boolean"}},
				{"name": "maxMembers", "dataType": []string{"int"}},
				{"name": "memberSyncEnabled", "dataType": []string{"boolean"}},
				{"name": "externalId", "dataType": []string{"text"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
		// AuditLog class
		map[string]any{
			"class":      "RBACAuditLog",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "tenantId", "dataType": []string{"text"}},
				{"name": "timestamp", "dataType": []string{"date"}},
				{"name": "subjectId", "dataType": []string{"text"}},
				{"name": "subjectType", "dataType": []string{"text"}},
				{"name": "action", "dataType": []string{"text"}},
				{"name": "resource", "dataType": []string{"text"}},
				{"name": "resourceId", "dataType": []string{"text"}},
				{"name": "result", "dataType": []string{"text"}},
				{"name": "details", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "severity", "dataType": []string{"text"}},
				{"name": "source", "dataType": []string{"text"}},
				{"name": "correlationId", "dataType": []string{"text"}},
				{"name": "retentionClass", "dataType": []string{"text"}},
			},
		},
		// Tenant class
		map[string]any{
			"class":      "RBACTenant",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "name", "dataType": []string{"text"}},
				{"name": "displayName", "dataType": []string{"text"}},
				{"name": "description", "dataType": []string{"text"}},
				{"name": "deployments", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "environment", "dataType": []string{"text"}},
						{"name": "metricsEndpoint", "dataType": []string{"text"}},
						{"name": "logsEndpoint", "dataType": []string{"text"}},
						{"name": "tracesEndpoint", "dataType": []string{"text"}},
						{"name": "priority", "dataType": []string{"int"}},
						{"name": "tags", "dataType": []string{"text[]"}},
					}},
				{"name": "status", "dataType": []string{"text"}},
				{"name": "adminEmail", "dataType": []string{"text"}},
				{"name": "adminName", "dataType": []string{"text"}},
				{"name": "quotas", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "maxUsers", "dataType": []string{"int"}},
						{"name": "maxDashboards", "dataType": []string{"int"}},
						{"name": "maxKpis", "dataType": []string{"int"}},
						{"name": "storageLimitGb", "dataType": []string{"int"}},
						{"name": "apiRateLimit", "dataType": []string{"int"}},
					}},
				{"name": "features", "dataType": []string{"text[]"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "tags", "dataType": []string{"text[]"}},
				{"name": "isSystem", "dataType": []string{"boolean"}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
		// User class
		map[string]any{
			"class":      "RBACUser",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "email", "dataType": []string{"text"}},
				{"name": "username", "dataType": []string{"text"}},
				{"name": "fullName", "dataType": []string{"text"}},
				{"name": "globalRole", "dataType": []string{"text"}},
				{"name": "passwordHash", "dataType": []string{"text"}},
				{"name": "mfaEnabled", "dataType": []string{"boolean"}},
				{"name": "mfaSecret", "dataType": []string{"text"}},
				{"name": "status", "dataType": []string{"text"}},
				{"name": "emailVerified", "dataType": []string{"boolean"}},
				{"name": "avatar", "dataType": []string{"text"}},
				{"name": "phone", "dataType": []string{"text"}},
				{"name": "timezone", "dataType": []string{"text"}},
				{"name": "language", "dataType": []string{"text"}},
				{"name": "lastLoginAt", "dataType": []string{"date"}},
				{"name": "loginCount", "dataType": []string{"int"}},
				{"name": "failedLoginCount", "dataType": []string{"int"}},
				{"name": "lockedUntil", "dataType": []string{"date"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "tags", "dataType": []string{"text[]"}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
		// TenantUser class
		map[string]any{
			"class":      "RBACTenantUser",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "tenantId", "dataType": []string{"text"}},
				{"name": "userId", "dataType": []string{"text"}},
				{"name": "tenantRole", "dataType": []string{"text"}},
				{"name": "status", "dataType": []string{"text"}},
				{"name": "invitedBy", "dataType": []string{"text"}},
				{"name": "invitedAt", "dataType": []string{"date"}},
				{"name": "acceptedAt", "dataType": []string{"date"}},
				{"name": "additionalPermissions", "dataType": []string{"text[]"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
		// MiradorAuth class
		map[string]any{
			"class":      "RBACMiradorAuth",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "userId", "dataType": []string{"text"}},
				{"name": "username", "dataType": []string{"text"}},
				{"name": "email", "dataType": []string{"text"}},
				{"name": "passwordHash", "dataType": []string{"text"}},
				{"name": "salt", "dataType": []string{"text"}},
				{"name": "totpSecret", "dataType": []string{"text"}},
				{"name": "totpEnabled", "dataType": []string{"boolean"}},
				{"name": "backupCodes", "dataType": []string{"text[]"}},
				{"name": "tenantId", "dataType": []string{"text"}},
				{"name": "roles", "dataType": []string{"text[]"}},
				{"name": "groups", "dataType": []string{"text[]"}},
				{"name": "isActive", "dataType": []string{"boolean"}},
				{"name": "passwordChangedAt", "dataType": []string{"date"}},
				{"name": "passwordExpiresAt", "dataType": []string{"date"}},
				{"name": "lastLoginAt", "dataType": []string{"date"}},
				{"name": "failedLoginCount", "dataType": []string{"int"}},
				{"name": "lockedUntil", "dataType": []string{"date"}},
				{"name": "requirePasswordChange", "dataType": []string{"boolean"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
		// AuthConfig class
		map[string]any{
			"class":      "RBACAuthConfig",
			"vectorizer": "none",
			"properties": []map[string]any{
				{"name": "tenantId", "dataType": []string{"text"}},
				{"name": "defaultBackend", "dataType": []string{"text"}},
				{"name": "enabledBackends", "dataType": []string{"text[]"}},
				{"name": "backendConfigs", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "passwordPolicy", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "require2fa", "dataType": []string{"boolean"}},
				{"name": "totpIssuer", "dataType": []string{"text"}},
				{"name": "sessionTimeoutMinutes", "dataType": []string{"int"}},
				{"name": "maxConcurrentSessions", "dataType": []string{"int"}},
				{"name": "allowRememberMe", "dataType": []string{"boolean"}},
				{"name": "rememberMeDays", "dataType": []string{"int"}},
				{"name": "metadata", "dataType": []string{"object"},
					"nestedProperties": []map[string]any{
						{"name": "note", "dataType": []string{"text"}},
					}},
				{"name": "createdAt", "dataType": []string{"date"}},
				{"name": "updatedAt", "dataType": []string{"date"}},
				{"name": "createdBy", "dataType": []string{"text"}},
				{"name": "updatedBy", "dataType": []string{"text"}},
			},
		},
	}

	start := time.Now()
	err := r.transport.EnsureClasses(ctx, classes)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("ensure_rbac_classes", "rbac_schema", duration, false)
		return fmt.Errorf("failed to ensure RBAC classes: %w", err)
	}

	monitoring.RecordWeaviateOperation("ensure_rbac_classes", "rbac_schema", duration, true)
	return nil
}

// makeRBACID generates deterministic IDs for RBAC objects
func makeRBACID(class, tenantID string, parts ...string) string {
	allParts := append([]string{class, tenantID}, parts...)
	return makeID(allParts...)
}

// CreateRole creates a new role
func (r *WeaviateRBACRepository) CreateRole(ctx context.Context, role *models.Role) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	role.ID = makeRBACID("Role", role.TenantID, role.Name)
	now := time.Now()

	props := map[string]any{
		"tenantId":    role.TenantID,
		"name":        role.Name,
		"description": role.Description,
		"permissions": role.Permissions,
		"isSystem":    role.IsSystem,
		"parentRoles": role.ParentRoles,
		"metadata":    role.Metadata,
		"createdAt":   now.Format(time.RFC3339),
		"updatedAt":   now.Format(time.RFC3339),
		"createdBy":   role.CreatedBy,
		"updatedBy":   role.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACRole", role.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_role", role.Name, duration, false)
		return fmt.Errorf("failed to create role %s: %w", role.Name, err)
	}

	monitoring.RecordWeaviateOperation("create_role", role.Name, duration, true)
	return nil
}

// GetRole retrieves a role by tenant and name
func (r *WeaviateRBACRepository) GetRole(ctx context.Context, tenantID, roleName string) (*models.Role, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACRole(
				where: {
					operator: And,
					operands: [
						{ path: ["tenantId"], operator: Equal, valueString: "%s" },
						{ path: ["name"], operator: Equal, valueString: "%s" }
					]
				}
			) {
				tenantId name description permissions isSystem parentRoles metadata
				createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, tenantID, roleName)

	var resp struct {
		Data struct {
			Get struct {
				RBACRole []map[string]any `json:"RBACRole"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_role", roleName, duration, false)
		return nil, fmt.Errorf("failed to get role %s: %w", roleName, err)
	}

	monitoring.RecordWeaviateOperation("get_role", roleName, duration, true)

	roles := resp.Data.Get.RBACRole
	if len(roles) == 0 {
		return nil, fmt.Errorf("role not found")
	}

	return r.mapToRole(roles[0])
}

// ListRoles retrieves all roles for a tenant
func (r *WeaviateRBACRepository) ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACRole(
				where: { path: ["tenantId"], operator: Equal, valueString: "%s" }
			) {
				tenantId name description permissions isSystem parentRoles metadata
				createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, tenantID)

	var resp struct {
		Data struct {
			Get struct {
				RBACRole []map[string]any `json:"RBACRole"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("list_roles", tenantID, duration, false)
		return nil, fmt.Errorf("failed to list roles for tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("list_roles", tenantID, duration, true)

	roles := resp.Data.Get.RBACRole
	result := make([]*models.Role, len(roles))
	for i, roleData := range roles {
		role, err := r.mapToRole(roleData)
		if err != nil {
			return nil, err
		}
		result[i] = role
	}

	return result, nil
}

// mapToRole converts Weaviate response to Role model
func (r *WeaviateRBACRepository) mapToRole(data map[string]any) (*models.Role, error) {
	role := &models.Role{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		role.ID = id
	}
	if v, ok := data["tenantId"].(string); ok {
		role.TenantID = v
	}
	if v, ok := data["name"].(string); ok {
		role.Name = v
	}
	if v, ok := data["description"].(string); ok {
		role.Description = v
	}
	if v, ok := data["permissions"].([]interface{}); ok {
		role.Permissions = make([]string, len(v))
		for i, p := range v {
			if s, ok := p.(string); ok {
				role.Permissions[i] = s
			}
		}
	}
	if v, ok := data["isSystem"].(bool); ok {
		role.IsSystem = v
	}
	if v, ok := data["parentRoles"].([]interface{}); ok {
		role.ParentRoles = make([]string, len(v))
		for i, p := range v {
			if s, ok := p.(string); ok {
				role.ParentRoles[i] = s
			}
		}
	}
	if v, ok := data["metadata"].(map[string]any); ok {
		role.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				role.Metadata[k] = s
			}
		}
	}
	if v, ok := data["createdBy"].(string); ok {
		role.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		role.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			role.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			role.UpdatedAt = t
		}
	}

	return role, nil
}

// UpdateRole updates an existing role
func (r *WeaviateRBACRepository) UpdateRole(ctx context.Context, role *models.Role) error {
	role.UpdatedAt = time.Now()

	props := map[string]any{
		"description": role.Description,
		"permissions": role.Permissions,
		"parentRoles": role.ParentRoles,
		"metadata":    role.Metadata,
		"updatedAt":   role.UpdatedAt.Format(time.RFC3339),
		"updatedBy":   role.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACRole", role.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_role", role.Name, duration, false)
		return fmt.Errorf("failed to update role %s: %w", role.Name, err)
	}

	monitoring.RecordWeaviateOperation("update_role", role.Name, duration, true)
	return nil
}

// DeleteRole deletes a role
func (r *WeaviateRBACRepository) DeleteRole(ctx context.Context, tenantID, roleName string) error {
	roleID := makeRBACID("Role", tenantID, roleName)

	start := time.Now()
	err := r.transport.DeleteObject(ctx, roleID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_role", roleName, duration, false)
		return fmt.Errorf("failed to delete role %s: %w", roleName, err)
	}

	monitoring.RecordWeaviateOperation("delete_role", roleName, duration, true)
	return nil
}

// CreatePermission creates a new permission
func (r *WeaviateRBACRepository) CreatePermission(ctx context.Context, permission *models.Permission) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	permission.ID = makeRBACID("Permission", permission.TenantID, permission.Resource, permission.Action)
	now := time.Now()

	props := map[string]any{
		"tenantId":        permission.TenantID,
		"resource":        permission.Resource,
		"action":          permission.Action,
		"scope":           permission.Scope,
		"description":     permission.Description,
		"resourcePattern": permission.ResourcePattern,
		"conditions":      permission.Conditions,
		"isSystem":        permission.IsSystem,
		"metadata":        permission.Metadata,
		"createdAt":       now.Format(time.RFC3339),
		"updatedAt":       now.Format(time.RFC3339),
		"createdBy":       permission.CreatedBy,
		"updatedBy":       permission.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACPermission", permission.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_permission", permission.Resource+"."+permission.Action, duration, false)
		return fmt.Errorf("failed to create permission %s.%s: %w", permission.Resource, permission.Action, err)
	}

	monitoring.RecordWeaviateOperation("create_permission", permission.Resource+"."+permission.Action, duration, true)
	return nil
}

// GetPermission retrieves a permission by tenant and ID
func (r *WeaviateRBACRepository) GetPermission(ctx context.Context, tenantID, permissionID string) (*models.Permission, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACPermission(where: { path: ["id"], operator: Equal, valueString: "%s" }) {
				tenantId resource action scope description resourcePattern conditions
				isSystem metadata createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, permissionID)

	var resp struct {
		Data struct {
			Get struct {
				RBACPermission []map[string]any `json:"RBACPermission"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_permission", permissionID, duration, false)
		return nil, fmt.Errorf("failed to get permission %s: %w", permissionID, err)
	}

	monitoring.RecordWeaviateOperation("get_permission", permissionID, duration, true)

	permissions := resp.Data.Get.RBACPermission
	if len(permissions) == 0 {
		return nil, fmt.Errorf("permission not found")
	}

	return r.mapToPermission(permissions[0])
}

// ListPermissions retrieves all permissions for a tenant
func (r *WeaviateRBACRepository) ListPermissions(ctx context.Context, tenantID string) ([]*models.Permission, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACPermission(where: { path: ["tenantId"], operator: Equal, valueString: "%s" }) {
				tenantId resource action scope description resourcePattern conditions
				isSystem metadata createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, tenantID)

	var resp struct {
		Data struct {
			Get struct {
				RBACPermission []map[string]any `json:"RBACPermission"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("list_permissions", tenantID, duration, false)
		return nil, fmt.Errorf("failed to list permissions for tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("list_permissions", tenantID, duration, true)

	permissions := resp.Data.Get.RBACPermission
	result := make([]*models.Permission, len(permissions))
	for i, permissionData := range permissions {
		permission, err := r.mapToPermission(permissionData)
		if err != nil {
			return nil, err
		}
		result[i] = permission
	}

	return result, nil
}

// UpdatePermission updates an existing permission
func (r *WeaviateRBACRepository) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	permission.UpdatedAt = time.Now()

	props := map[string]any{
		"scope":           permission.Scope,
		"description":     permission.Description,
		"resourcePattern": permission.ResourcePattern,
		"conditions":      permission.Conditions,
		"metadata":        permission.Metadata,
		"updatedAt":       permission.UpdatedAt.Format(time.RFC3339),
		"updatedBy":       permission.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACPermission", permission.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_permission", permission.Resource+"."+permission.Action, duration, false)
		return fmt.Errorf("failed to update permission %s.%s: %w", permission.Resource, permission.Action, err)
	}

	monitoring.RecordWeaviateOperation("update_permission", permission.Resource+"."+permission.Action, duration, true)
	return nil
}

// DeletePermission deletes a permission
func (r *WeaviateRBACRepository) DeletePermission(ctx context.Context, tenantID, permissionID string) error {
	start := time.Now()
	err := r.transport.DeleteObject(ctx, permissionID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_permission", permissionID, duration, false)
		return fmt.Errorf("failed to delete permission %s: %w", permissionID, err)
	}

	monitoring.RecordWeaviateOperation("delete_permission", permissionID, duration, true)
	return nil
}

// mapToPermission converts Weaviate response to Permission model
func (r *WeaviateRBACRepository) mapToPermission(data map[string]any) (*models.Permission, error) {
	permission := &models.Permission{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		permission.ID = id
	}
	if v, ok := data["tenantId"].(string); ok {
		permission.TenantID = v
	}
	if v, ok := data["resource"].(string); ok {
		permission.Resource = v
	}
	if v, ok := data["action"].(string); ok {
		permission.Action = v
	}
	if v, ok := data["scope"].(string); ok {
		permission.Scope = v
	}
	if v, ok := data["description"].(string); ok {
		permission.Description = v
	}
	if v, ok := data["resourcePattern"].(string); ok {
		permission.ResourcePattern = v
	}
	if v, ok := data["conditions"].(map[string]any); ok {
		// Convert the conditions map to PermissionConditions struct
		conditionsJSON, err := json.Marshal(v)
		if err == nil {
			var conditions models.PermissionConditions
			if err := json.Unmarshal(conditionsJSON, &conditions); err == nil {
				permission.Conditions = conditions
			}
		}
	}
	if v, ok := data["isSystem"].(bool); ok {
		permission.IsSystem = v
	}
	if v, ok := data["metadata"].(map[string]any); ok {
		permission.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				permission.Metadata[k] = s
			}
		}
	}
	if v, ok := data["createdBy"].(string); ok {
		permission.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		permission.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			permission.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			permission.UpdatedAt = t
		}
	}

	return permission, nil
}

// CreateGroup creates a new group
func (r *WeaviateRBACRepository) CreateGroup(ctx context.Context, group *models.Group) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	group.ID = makeRBACID("Group", group.TenantID, group.Name)
	now := time.Now()

	props := map[string]any{
		"tenantId":          group.TenantID,
		"name":              group.Name,
		"description":       group.Description,
		"members":           group.Members,
		"roles":             group.Roles,
		"parentGroups":      group.ParentGroups,
		"isSystem":          group.IsSystem,
		"maxMembers":        group.MaxMembers,
		"memberSyncEnabled": group.MemberSyncEnabled,
		"externalId":        group.ExternalID,
		"metadata":          group.Metadata,
		"createdAt":         now.Format(time.RFC3339),
		"updatedAt":         now.Format(time.RFC3339),
		"createdBy":         group.CreatedBy,
		"updatedBy":         group.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACGroup", group.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_group", group.Name, duration, false)
		return fmt.Errorf("failed to create group %s: %w", group.Name, err)
	}

	monitoring.RecordWeaviateOperation("create_group", group.Name, duration, true)
	return nil
}

// GetGroup retrieves a group by tenant and name
func (r *WeaviateRBACRepository) GetGroup(ctx context.Context, tenantID, groupName string) (*models.Group, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACGroup(
				where: {
					operator: And,
					operands: [
						{ path: ["tenantId"], operator: Equal, valueString: "%s" },
						{ path: ["name"], operator: Equal, valueString: "%s" }
					]
				}
			) {
				tenantId name description members roles parentGroups isSystem
				maxMembers memberSyncEnabled externalId metadata
				createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, tenantID, groupName)

	var resp struct {
		Data struct {
			Get struct {
				RBACGroup []map[string]any `json:"RBACGroup"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_group", groupName, duration, false)
		return nil, fmt.Errorf("failed to get group %s: %w", groupName, err)
	}

	monitoring.RecordWeaviateOperation("get_group", groupName, duration, true)

	groups := resp.Data.Get.RBACGroup
	if len(groups) == 0 {
		return nil, fmt.Errorf("group not found")
	}

	return r.mapToGroup(groups[0])
}

// ListGroups retrieves all groups for a tenant
func (r *WeaviateRBACRepository) ListGroups(ctx context.Context, tenantID string) ([]*models.Group, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACGroup(where: { path: ["tenantId"], operator: Equal, valueString: "%s" }) {
				tenantId name description members roles parentGroups isSystem
				maxMembers memberSyncEnabled externalId metadata
				createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, tenantID)

	var resp struct {
		Data struct {
			Get struct {
				RBACGroup []map[string]any `json:"RBACGroup"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("list_groups", tenantID, duration, false)
		return nil, fmt.Errorf("failed to list groups for tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("list_groups", tenantID, duration, true)

	groups := resp.Data.Get.RBACGroup
	result := make([]*models.Group, len(groups))
	for i, groupData := range groups {
		group, err := r.mapToGroup(groupData)
		if err != nil {
			return nil, err
		}
		result[i] = group
	}

	return result, nil
}

// UpdateGroup updates an existing group
func (r *WeaviateRBACRepository) UpdateGroup(ctx context.Context, group *models.Group) error {
	group.UpdatedAt = time.Now()

	props := map[string]any{
		"description":       group.Description,
		"members":           group.Members,
		"roles":             group.Roles,
		"parentGroups":      group.ParentGroups,
		"maxMembers":        group.MaxMembers,
		"memberSyncEnabled": group.MemberSyncEnabled,
		"externalId":        group.ExternalID,
		"metadata":          group.Metadata,
		"updatedAt":         group.UpdatedAt.Format(time.RFC3339),
		"updatedBy":         group.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACGroup", group.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_group", group.Name, duration, false)
		return fmt.Errorf("failed to update group %s: %w", group.Name, err)
	}

	monitoring.RecordWeaviateOperation("update_group", group.Name, duration, true)
	return nil
}

// DeleteGroup deletes a group
func (r *WeaviateRBACRepository) DeleteGroup(ctx context.Context, tenantID, groupName string) error {
	groupID := makeRBACID("Group", tenantID, groupName)

	start := time.Now()
	err := r.transport.DeleteObject(ctx, groupID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_group", groupName, duration, false)
		return fmt.Errorf("failed to delete group %s: %w", groupName, err)
	}

	monitoring.RecordWeaviateOperation("delete_group", groupName, duration, true)
	return nil
}

// mapToGroup converts Weaviate response to Group model
func (r *WeaviateRBACRepository) mapToGroup(data map[string]any) (*models.Group, error) {
	group := &models.Group{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		group.ID = id
	}
	if v, ok := data["tenantId"].(string); ok {
		group.TenantID = v
	}
	if v, ok := data["name"].(string); ok {
		group.Name = v
	}
	if v, ok := data["description"].(string); ok {
		group.Description = v
	}
	if v, ok := data["members"].([]interface{}); ok {
		group.Members = make([]string, len(v))
		for i, m := range v {
			if s, ok := m.(string); ok {
				group.Members[i] = s
			}
		}
	}
	if v, ok := data["roles"].([]interface{}); ok {
		group.Roles = make([]string, len(v))
		for i, r := range v {
			if s, ok := r.(string); ok {
				group.Roles[i] = s
			}
		}
	}
	if v, ok := data["parentGroups"].([]interface{}); ok {
		group.ParentGroups = make([]string, len(v))
		for i, p := range v {
			if s, ok := p.(string); ok {
				group.ParentGroups[i] = s
			}
		}
	}
	if v, ok := data["isSystem"].(bool); ok {
		group.IsSystem = v
	}
	if v, ok := data["maxMembers"].(float64); ok {
		group.MaxMembers = int(v)
	}
	if v, ok := data["memberSyncEnabled"].(bool); ok {
		group.MemberSyncEnabled = v
	}
	if v, ok := data["externalId"].(string); ok {
		group.ExternalID = v
	}
	if v, ok := data["metadata"].(map[string]any); ok {
		group.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				group.Metadata[k] = s
			}
		}
	}
	if v, ok := data["createdBy"].(string); ok {
		group.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		group.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			group.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			group.UpdatedAt = t
		}
	}

	return group, nil
}

// AddUsersToGroup adds users to a group
func (r *WeaviateRBACRepository) AddUsersToGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	// Get the current group
	group, err := r.GetGroup(ctx, tenantID, groupName)
	if err != nil {
		return fmt.Errorf("failed to get group %s: %w", groupName, err)
	}

	// Add users to members list (avoid duplicates)
	membersSet := make(map[string]bool)
	for _, member := range group.Members {
		membersSet[member] = true
	}
	for _, userID := range userIDs {
		membersSet[userID] = true
	}

	// Convert back to slice
	newMembers := make([]string, 0, len(membersSet))
	for member := range membersSet {
		newMembers = append(newMembers, member)
	}
	group.Members = newMembers

	// Update the group
	return r.UpdateGroup(ctx, group)
}

// RemoveUsersFromGroup removes users from a group
func (r *WeaviateRBACRepository) RemoveUsersFromGroup(ctx context.Context, tenantID, groupName string, userIDs []string) error {
	// Get the current group
	group, err := r.GetGroup(ctx, tenantID, groupName)
	if err != nil {
		return fmt.Errorf("failed to get group %s: %w", groupName, err)
	}

	// Create set of users to remove
	removeSet := make(map[string]bool)
	for _, userID := range userIDs {
		removeSet[userID] = true
	}

	// Filter out users to remove
	newMembers := make([]string, 0, len(group.Members))
	for _, member := range group.Members {
		if !removeSet[member] {
			newMembers = append(newMembers, member)
		}
	}
	group.Members = newMembers

	// Update the group
	return r.UpdateGroup(ctx, group)
}

// GetGroupMembers retrieves all members of a group
func (r *WeaviateRBACRepository) GetGroupMembers(ctx context.Context, tenantID, groupName string) ([]string, error) {
	group, err := r.GetGroup(ctx, tenantID, groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get group %s: %w", groupName, err)
	}

	return group.Members, nil
}

// GetUserGroups retrieves all groups that a user belongs to
func (r *WeaviateRBACRepository) GetUserGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	// Get all groups for the tenant
	groups, err := r.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups for tenant %s: %w", tenantID, err)
	}

	// Filter groups where user is a member
	userGroups := make([]string, 0)
	for _, group := range groups {
		for _, member := range group.Members {
			if member == userID {
				userGroups = append(userGroups, group.Name)
				break
			}
		}
	}

	return userGroups, nil
}

// CreateRoleBinding creates a new role binding
func (r *WeaviateRBACRepository) CreateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	binding.ID = makeRBACID("RoleBinding", binding.SubjectID, binding.RoleID, binding.ResourceID)
	now := time.Now()

	conditionsJSON, _ := json.Marshal(binding.Conditions)

	props := map[string]any{
		"subjectType":   binding.SubjectType,
		"subjectId":     binding.SubjectID,
		"roleId":        binding.RoleID,
		"scope":         binding.Scope,
		"resourceId":    binding.ResourceID,
		"precedence":    binding.Precedence,
		"conditions":    string(conditionsJSON),
		"justification": binding.Justification,
		"metadata":      binding.Metadata,
		"createdAt":     now.Format(time.RFC3339),
		"updatedAt":     now.Format(time.RFC3339),
		"createdBy":     binding.CreatedBy,
		"updatedBy":     binding.UpdatedBy,
	}

	// Handle optional timestamp fields
	if binding.ExpiresAt != nil {
		props["expiresAt"] = binding.ExpiresAt.Format(time.RFC3339)
	}
	if binding.NotBefore != nil {
		props["notBefore"] = binding.NotBefore.Format(time.RFC3339)
	}
	if binding.ApprovedAt != nil {
		props["approvedAt"] = binding.ApprovedAt.Format(time.RFC3339)
	}
	if binding.ApprovedBy != "" {
		props["approvedBy"] = binding.ApprovedBy
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACRoleBinding", binding.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_role_binding", binding.SubjectID, duration, false)
		return fmt.Errorf("failed to create role binding for subject %s: %w", binding.SubjectID, err)
	}

	monitoring.RecordWeaviateOperation("create_role_binding", binding.SubjectID, duration, true)
	return nil
}

// GetRoleBindings retrieves role bindings with filters
func (r *WeaviateRBACRepository) GetRoleBindings(ctx context.Context, tenantID string, filters RoleBindingFilters) ([]*models.RoleBinding, error) {
	// Build where clause based on filters
	operands := []map[string]any{}

	if filters.SubjectType != nil {
		operands = append(operands, map[string]any{
			"path": []string{"subjectType"}, "operator": "Equal", "valueString": *filters.SubjectType,
		})
	}
	if filters.SubjectID != nil {
		operands = append(operands, map[string]any{
			"path": []string{"subjectId"}, "operator": "Equal", "valueString": *filters.SubjectID,
		})
	}
	if filters.RoleID != nil {
		operands = append(operands, map[string]any{
			"path": []string{"roleId"}, "operator": "Equal", "valueString": *filters.RoleID,
		})
	}
	if filters.Scope != nil {
		operands = append(operands, map[string]any{
			"path": []string{"scope"}, "operator": "Equal", "valueString": *filters.Scope,
		})
	}
	if filters.ResourceID != nil {
		operands = append(operands, map[string]any{
			"path": []string{"resourceId"}, "operator": "Equal", "valueString": *filters.ResourceID,
		})
	}
	if filters.Precedence != nil {
		operands = append(operands, map[string]any{
			"path": []string{"precedence"}, "operator": "Equal", "valueString": *filters.Precedence,
		})
	}
	if filters.Expired != nil {
		now := time.Now().Format(time.RFC3339)
		if *filters.Expired {
			// Include expired bindings
			operands = append(operands, map[string]any{
				"path": []string{"expiresAt"}, "operator": "LessThan", "valueDate": now,
			})
		} else {
			// Exclude expired bindings
			operands = append(operands, map[string]any{
				"operator": "Or",
				"operands": []map[string]any{
					{"path": []string{"expiresAt"}, "operator": "GreaterThan", "valueDate": now},
					{"path": []string{"expiresAt"}, "operator": "IsNull"},
				},
			})
		}
	}

	whereClause := ""
	if len(operands) > 0 {
		whereJSON, _ := json.Marshal(map[string]any{
			"operator": "And",
			"operands": operands,
		})
		whereClause = fmt.Sprintf(`, where: %s`, string(whereJSON))
	}

	query := fmt.Sprintf(`{
		Get {
			RBACRoleBinding%s {
				subjectType subjectId roleId scope resourceId expiresAt notBefore
				precedence conditions justification approvedBy approvedAt
				metadata createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, whereClause)

	var resp struct {
		Data struct {
			Get struct {
				RBACRoleBinding []map[string]any `json:"RBACRoleBinding"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_role_bindings", tenantID, duration, false)
		return nil, fmt.Errorf("failed to get role bindings for tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("get_role_bindings", tenantID, duration, true)

	bindings := resp.Data.Get.RBACRoleBinding
	result := make([]*models.RoleBinding, len(bindings))
	for i, bindingData := range bindings {
		binding, err := r.mapToRoleBinding(bindingData)
		if err != nil {
			return nil, err
		}
		result[i] = binding
	}

	return result, nil
}

// UpdateRoleBinding updates an existing role binding
func (r *WeaviateRBACRepository) UpdateRoleBinding(ctx context.Context, binding *models.RoleBinding) error {
	binding.UpdatedAt = time.Now()

	conditionsJSON, _ := json.Marshal(binding.Conditions)

	props := map[string]any{
		"scope":         binding.Scope,
		"resourceId":    binding.ResourceID,
		"precedence":    binding.Precedence,
		"conditions":    string(conditionsJSON),
		"justification": binding.Justification,
		"metadata":      binding.Metadata,
		"updatedAt":     binding.UpdatedAt.Format(time.RFC3339),
		"updatedBy":     binding.UpdatedBy,
	}

	// Handle optional timestamp fields
	if binding.ExpiresAt != nil {
		props["expiresAt"] = binding.ExpiresAt.Format(time.RFC3339)
	}
	if binding.NotBefore != nil {
		props["notBefore"] = binding.NotBefore.Format(time.RFC3339)
	}
	if binding.ApprovedAt != nil {
		props["approvedAt"] = binding.ApprovedAt.Format(time.RFC3339)
	}
	if binding.ApprovedBy != "" {
		props["approvedBy"] = binding.ApprovedBy
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACRoleBinding", binding.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_role_binding", binding.SubjectID, duration, false)
		return fmt.Errorf("failed to update role binding for subject %s: %w", binding.SubjectID, err)
	}

	monitoring.RecordWeaviateOperation("update_role_binding", binding.SubjectID, duration, true)
	return nil
}

// DeleteRoleBinding deletes a role binding
func (r *WeaviateRBACRepository) DeleteRoleBinding(ctx context.Context, tenantID, bindingID string) error {
	start := time.Now()
	err := r.transport.DeleteObject(ctx, bindingID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_role_binding", bindingID, duration, false)
		return fmt.Errorf("failed to delete role binding %s: %w", bindingID, err)
	}

	monitoring.RecordWeaviateOperation("delete_role_binding", bindingID, duration, true)
	return nil
}

// mapToRoleBinding converts Weaviate response to RoleBinding model
func (r *WeaviateRBACRepository) mapToRoleBinding(data map[string]any) (*models.RoleBinding, error) {
	binding := &models.RoleBinding{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		binding.ID = id
	}
	if v, ok := data["subjectType"].(string); ok {
		binding.SubjectType = v
	}
	if v, ok := data["subjectId"].(string); ok {
		binding.SubjectID = v
	}
	if v, ok := data["roleId"].(string); ok {
		binding.RoleID = v
	}
	if v, ok := data["scope"].(string); ok {
		binding.Scope = v
	}
	if v, ok := data["resourceId"].(string); ok {
		binding.ResourceID = v
	}
	if v, ok := data["precedence"].(string); ok {
		binding.Precedence = v
	}
	if v, ok := data["justification"].(string); ok {
		binding.Justification = v
	}
	if v, ok := data["approvedBy"].(string); ok {
		binding.ApprovedBy = v
	}
	if v, ok := data["metadata"].(map[string]any); ok {
		binding.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				binding.Metadata[k] = s
			}
		}
	}
	if v, ok := data["createdBy"].(string); ok {
		binding.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		binding.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			binding.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			binding.UpdatedAt = t
		}
	}
	if v, ok := data["expiresAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			binding.ExpiresAt = &t
		}
	}
	if v, ok := data["notBefore"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			binding.NotBefore = &t
		}
	}
	if v, ok := data["approvedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			binding.ApprovedAt = &t
		}
	}

	// Parse conditions JSON
	if v, ok := data["conditions"].(string); ok {
		var conditions models.RoleBindingConditions
		if err := json.Unmarshal([]byte(v), &conditions); err == nil {
			binding.Conditions = conditions
		}
	}

	return binding, nil
}

// AssignUserRoles assigns roles to a user (stored as RoleBindings)
func (r *WeaviateRBACRepository) AssignUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	now := time.Now()

	// Remove existing role bindings for this user
	if err := r.removeUserRoleBindings(ctx, tenantID, userID); err != nil {
		return fmt.Errorf("failed to remove existing role bindings: %w", err)
	}

	// Create new role bindings
	for _, roleName := range roles {
		binding := &models.RoleBinding{
			ID:          makeRBACID("RoleBinding", tenantID, userID, roleName),
			SubjectType: "user",
			SubjectID:   userID,
			RoleID:      roleName,
			Scope:       "tenant",
			Precedence:  "allow",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		props := map[string]any{
			"tenantId":    tenantID,
			"subjectType": binding.SubjectType,
			"subjectId":   binding.SubjectID,
			"roleId":      binding.RoleID,
			"scope":       binding.Scope,
			"precedence":  binding.Precedence,
			"createdAt":   now.Format(time.RFC3339),
			"updatedAt":   now.Format(time.RFC3339),
		}

		if err := r.transport.PutObject(ctx, "RBACRoleBinding", binding.ID, props); err != nil {
			return fmt.Errorf("failed to create role binding for role %s: %w", roleName, err)
		}
	}

	return nil
}

// GetUserRoles retrieves roles assigned to a user
func (r *WeaviateRBACRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACRoleBinding(
				where: {
					operator: And,
					operands: [
						{ path: ["tenantId"], operator: Equal, valueString: "%s" },
						{ path: ["subjectType"], operator: Equal, valueString: "user" },
						{ path: ["subjectId"], operator: Equal, valueString: "%s" }
					]
				}
			) {
				roleId
			}
		}
	}`, tenantID, userID)

	var resp struct {
		Data struct {
			Get struct {
				RBACRoleBinding []map[string]any `json:"RBACRoleBinding"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_user_roles", userID, duration, false)
		return nil, fmt.Errorf("failed to get user roles for %s: %w", userID, err)
	}

	monitoring.RecordWeaviateOperation("get_user_roles", userID, duration, true)

	bindings := resp.Data.Get.RBACRoleBinding
	roles := make([]string, len(bindings))
	for i, binding := range bindings {
		if roleID, ok := binding["roleId"].(string); ok {
			roles[i] = roleID
		}
	}

	return roles, nil
}

// removeUserRoleBindings removes all role bindings for a user
func (r *WeaviateRBACRepository) removeUserRoleBindings(ctx context.Context, tenantID, userID string) error {
	// Get all binding IDs for this user
	query := fmt.Sprintf(`{
		Get {
			RBACRoleBinding(
				where: {
					operator: And,
					operands: [
						{ path: ["tenantId"], operator: Equal, valueString: "%s" },
						{ path: ["subjectType"], operator: Equal, valueString: "user" },
						{ path: ["subjectId"], operator: Equal, valueString: "%s" }
					]
				}
			) {
				_additional { id }
			}
		}
	}`, tenantID, userID)

	var resp struct {
		Data struct {
			Get struct {
				RBACRoleBinding []map[string]any `json:"RBACRoleBinding"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := r.transport.GraphQL(ctx, query, nil, &resp); err != nil {
		return err
	}

	// Delete each binding
	for _, binding := range resp.Data.Get.RBACRoleBinding {
		if additional, ok := binding["_additional"].(map[string]any); ok {
			if id, ok := additional["id"].(string); ok {
				if err := r.transport.DeleteObject(ctx, id); err != nil {
					return fmt.Errorf("failed to delete role binding %s: %w", id, err)
				}
			}
		}
	}

	return nil
}

// RemoveUserRoles removes specific roles from a user
func (r *WeaviateRBACRepository) RemoveUserRoles(ctx context.Context, tenantID, userID string, roles []string) error {
	for _, roleName := range roles {
		bindingID := makeRBACID("RoleBinding", tenantID, userID, roleName)
		if err := r.transport.DeleteObject(ctx, bindingID); err != nil {
			return fmt.Errorf("failed to remove role %s from user %s: %w", roleName, userID, err)
		}
	}
	return nil
}

// LogAuditEvent logs an audit event
func (r *WeaviateRBACRepository) LogAuditEvent(ctx context.Context, event *models.AuditLog) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	event.ID = makeRBACID("AuditLog", event.TenantID, event.SubjectID, event.Action, fmt.Sprintf("%d", event.Timestamp.Unix()))

	detailsJSON, _ := json.Marshal(event.Details)

	props := map[string]any{
		"tenantId":       event.TenantID,
		"timestamp":      event.Timestamp.Format(time.RFC3339),
		"subjectId":      event.SubjectID,
		"subjectType":    event.SubjectType,
		"action":         event.Action,
		"resource":       event.Resource,
		"resourceId":     event.ResourceID,
		"result":         event.Result,
		"details":        string(detailsJSON),
		"severity":       event.Severity,
		"source":         event.Source,
		"correlationId":  event.CorrelationID,
		"retentionClass": event.RetentionClass,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACAuditLog", event.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("log_audit_event", event.Action, duration, false)
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	monitoring.RecordWeaviateOperation("log_audit_event", event.Action, duration, true)
	return nil
}

// GetAuditEvents retrieves audit events with filters
func (r *WeaviateRBACRepository) GetAuditEvents(ctx context.Context, tenantID string, filters AuditFilters) ([]*models.AuditLog, error) {
	// Build where clause based on filters
	operands := []map[string]any{
		{"path": []string{"tenantId"}, "operator": "Equal", "valueString": tenantID},
	}

	if filters.SubjectID != nil {
		operands = append(operands, map[string]any{
			"path": []string{"subjectId"}, "operator": "Equal", "valueString": *filters.SubjectID,
		})
	}
	if filters.Action != nil {
		operands = append(operands, map[string]any{
			"path": []string{"action"}, "operator": "Equal", "valueString": *filters.Action,
		})
	}
	if filters.StartTime != nil {
		operands = append(operands, map[string]any{
			"path": []string{"timestamp"}, "operator": "GreaterThanEqual", "valueDate": filters.StartTime.Format(time.RFC3339),
		})
	}
	if filters.EndTime != nil {
		operands = append(operands, map[string]any{
			"path": []string{"timestamp"}, "operator": "LessThanEqual", "valueDate": filters.EndTime.Format(time.RFC3339),
		})
	}

	whereClause := map[string]any{
		"operator": "And",
		"operands": operands,
	}

	limit := 100
	if filters.Limit > 0 && filters.Limit <= 1000 {
		limit = filters.Limit
	}

	query := map[string]any{
		"query": fmt.Sprintf(`{
			Get {
				RBACAuditLog(where: $where, limit: %d, offset: %d) {
					tenantId timestamp subjectId subjectType action resource resourceId
					result details severity source correlationId retentionClass
				}
			}
		}`, limit, filters.Offset),
		"variables": map[string]any{"where": whereClause},
	}

	var resp struct {
		Data struct {
			Get struct {
				RBACAuditLog []map[string]any `json:"RBACAuditLog"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query["query"].(string), query["variables"].(map[string]any), &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_audit_events", tenantID, duration, false)
		return nil, fmt.Errorf("failed to get audit events: %w", err)
	}

	monitoring.RecordWeaviateOperation("get_audit_events", tenantID, duration, true)

	events := resp.Data.Get.RBACAuditLog
	result := make([]*models.AuditLog, len(events))
	for i, eventData := range events {
		event, err := r.mapToAuditLog(eventData)
		if err != nil {
			return nil, err
		}
		result[i] = event
	}

	return result, nil
}

// mapToAuditLog converts Weaviate response to AuditLog model
func (r *WeaviateRBACRepository) mapToAuditLog(data map[string]any) (*models.AuditLog, error) {
	event := &models.AuditLog{}

	if v, ok := data["tenantId"].(string); ok {
		event.TenantID = v
	}
	if v, ok := data["subjectId"].(string); ok {
		event.SubjectID = v
	}
	if v, ok := data["subjectType"].(string); ok {
		event.SubjectType = v
	}
	if v, ok := data["action"].(string); ok {
		event.Action = v
	}
	if v, ok := data["resource"].(string); ok {
		event.Resource = v
	}
	if v, ok := data["resourceId"].(string); ok {
		event.ResourceID = v
	}
	if v, ok := data["result"].(string); ok {
		event.Result = v
	}
	if v, ok := data["severity"].(string); ok {
		event.Severity = v
	}
	if v, ok := data["source"].(string); ok {
		event.Source = v
	}
	if v, ok := data["correlationId"].(string); ok {
		event.CorrelationID = v
	}
	if v, ok := data["retentionClass"].(string); ok {
		event.RetentionClass = v
	}

	// Parse timestamp
	if v, ok := data["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			event.Timestamp = t
		}
	}

	// Parse details JSON
	if v, ok := data["details"].(string); ok {
		var details models.AuditLogDetails
		if err := json.Unmarshal([]byte(v), &details); err == nil {
			event.Details = details
		}
	}

	return event, nil
}

// Placeholder implementations for new interface methods
// These will be implemented as needed

func (r *WeaviateRBACRepository) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	// Always generate a UUID for the tenant ID (required by Weaviate)
	tenant.ID = makeRBACID("Tenant", "", tenant.Name)
	now := time.Now()

	props := map[string]any{
		"name":        tenant.Name,
		"displayName": tenant.DisplayName,
		"description": tenant.Description,
		"deployments": tenant.Deployments,
		"status":      tenant.Status,
		"adminEmail":  tenant.AdminEmail,
		"adminName":   tenant.AdminName,
		"quotas":      tenant.Quotas,
		"features":    tenant.Features,
		"metadata":    tenant.Metadata,
		"tags":        tenant.Tags,
		"isSystem":    tenant.IsSystem,
		"createdAt":   now.Format(time.RFC3339),
		"updatedAt":   now.Format(time.RFC3339),
		"createdBy":   tenant.CreatedBy,
		"updatedBy":   tenant.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACTenant", tenant.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_tenant", tenant.Name, duration, false)
		return fmt.Errorf("failed to create tenant %s: %w", tenant.Name, err)
	}

	monitoring.RecordWeaviateOperation("create_tenant", tenant.Name, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACTenant(where: { path: ["id"], operator: Equal, valueString: "%s" }) {
				name displayName description deployments status adminEmail adminName
				quotas features metadata tags isSystem createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, tenantID)

	var resp struct {
		Data struct {
			Get struct {
				RBACTenant []map[string]any `json:"RBACTenant"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_tenant", tenantID, duration, false)
		return nil, fmt.Errorf("failed to get tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("get_tenant", tenantID, duration, true)

	tenants := resp.Data.Get.RBACTenant
	if len(tenants) == 0 {
		return nil, fmt.Errorf("tenant not found")
	}

	return r.mapToTenant(tenants[0])
}

func (r *WeaviateRBACRepository) ListTenants(ctx context.Context, filters TenantFilters) ([]*models.Tenant, error) {
	// Build where clause based on filters
	operands := []map[string]any{}

	if filters.Name != nil {
		operands = append(operands, map[string]any{
			"path": []string{"name"}, "operator": "Equal", "valueString": *filters.Name,
		})
	}
	if filters.Status != nil {
		operands = append(operands, map[string]any{
			"path": []string{"status"}, "operator": "Equal", "valueString": *filters.Status,
		})
	}
	if filters.AdminEmail != nil {
		operands = append(operands, map[string]any{
			"path": []string{"adminEmail"}, "operator": "Equal", "valueString": *filters.AdminEmail,
		})
	}

	whereClause := map[string]any{}
	if len(operands) > 0 {
		whereClause = map[string]any{
			"operator": "And",
			"operands": operands,
		}
	}

	limit := 100
	if filters.Limit > 0 && filters.Limit <= 1000 {
		limit = filters.Limit
	}

	query := map[string]any{
		"query": fmt.Sprintf(`{
			Get {
				RBACTenant(limit: %d, offset: %d%s) {
					name displayName description deployments status adminEmail adminName
					quotas features metadata tags isSystem createdAt updatedAt createdBy updatedBy
					_additional { id }
				}
			}
		}`, limit, filters.Offset, func() string {
			if len(whereClause) > 0 {
				return ", where: $where"
			}
			return ""
		}()),
		"variables": func() map[string]any {
			if len(whereClause) > 0 {
				return map[string]any{"where": whereClause}
			}
			return nil
		}(),
	}

	var resp struct {
		Data struct {
			Get struct {
				RBACTenant []map[string]any `json:"RBACTenant"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query["query"].(string), query["variables"].(map[string]any), &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("list_tenants", "all", duration, false)
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	monitoring.RecordWeaviateOperation("list_tenants", "all", duration, true)

	tenants := resp.Data.Get.RBACTenant
	result := make([]*models.Tenant, len(tenants))
	for i, tenantData := range tenants {
		tenant, err := r.mapToTenant(tenantData)
		if err != nil {
			return nil, err
		}
		result[i] = tenant
	}

	return result, nil
}

func (r *WeaviateRBACRepository) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	tenant.UpdatedAt = time.Now()

	props := map[string]any{
		"displayName": tenant.DisplayName,
		"description": tenant.Description,
		"deployments": tenant.Deployments,
		"status":      tenant.Status,
		"adminEmail":  tenant.AdminEmail,
		"adminName":   tenant.AdminName,
		"quotas":      tenant.Quotas,
		"features":    tenant.Features,
		"metadata":    tenant.Metadata,
		"tags":        tenant.Tags,
		"isSystem":    tenant.IsSystem,
		"updatedAt":   tenant.UpdatedAt.Format(time.RFC3339),
		"updatedBy":   tenant.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACTenant", tenant.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_tenant", tenant.Name, duration, false)
		return fmt.Errorf("failed to update tenant %s: %w", tenant.Name, err)
	}

	monitoring.RecordWeaviateOperation("update_tenant", tenant.Name, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) DeleteTenant(ctx context.Context, tenantID string) error {
	start := time.Now()
	err := r.transport.DeleteObject(ctx, tenantID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_tenant", tenantID, duration, false)
		return fmt.Errorf("failed to delete tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("delete_tenant", tenantID, duration, true)
	return nil
}

// mapToTenant converts Weaviate response to Tenant model
func (r *WeaviateRBACRepository) mapToTenant(data map[string]any) (*models.Tenant, error) {
	tenant := &models.Tenant{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		tenant.ID = id
	}
	if v, ok := data["name"].(string); ok {
		tenant.Name = v
	}
	if v, ok := data["displayName"].(string); ok {
		tenant.DisplayName = v
	}
	if v, ok := data["description"].(string); ok {
		tenant.Description = v
	}
	if v, ok := data["deployments"].([]interface{}); ok {
		tenant.Deployments = make([]models.TenantDeployment, len(v))
		for i, d := range v {
			if deploymentMap, ok := d.(map[string]interface{}); ok {
				deployment := models.TenantDeployment{}
				if env, ok := deploymentMap["environment"].(string); ok {
					deployment.Environment = env
				}
				if metrics, ok := deploymentMap["metricsEndpoint"].(string); ok {
					deployment.MetricsEndpoint = metrics
				}
				if logs, ok := deploymentMap["logsEndpoint"].(string); ok {
					deployment.LogsEndpoint = logs
				}
				if traces, ok := deploymentMap["tracesEndpoint"].(string); ok {
					deployment.TracesEndpoint = traces
				}
				if priority, ok := deploymentMap["priority"].(float64); ok {
					deployment.Priority = int(priority)
				}
				if tags, ok := deploymentMap["tags"].([]interface{}); ok {
					deployment.Tags = make([]string, len(tags))
					for j, t := range tags {
						if s, ok := t.(string); ok {
							deployment.Tags[j] = s
						}
					}
				}
				tenant.Deployments[i] = deployment
			}
		}
	}
	if v, ok := data["status"].(string); ok {
		tenant.Status = v
	}
	if v, ok := data["adminEmail"].(string); ok {
		tenant.AdminEmail = v
	}
	if v, ok := data["adminName"].(string); ok {
		tenant.AdminName = v
	}
	if v, ok := data["quotas"].(map[string]interface{}); ok {
		quotas := models.TenantQuotas{}
		if maxUsers, ok := v["maxUsers"].(float64); ok {
			quotas.MaxUsers = int(maxUsers)
		}
		if maxDashboards, ok := v["maxDashboards"].(float64); ok {
			quotas.MaxDashboards = int(maxDashboards)
		}
		if maxKPIs, ok := v["maxKpis"].(float64); ok {
			quotas.MaxKPIs = int(maxKPIs)
		}
		if storageLimit, ok := v["storageLimitGb"].(float64); ok {
			quotas.StorageLimitGB = int(storageLimit)
		}
		if apiRateLimit, ok := v["apiRateLimit"].(float64); ok {
			quotas.APIRateLimit = int(apiRateLimit)
		}
		tenant.Quotas = quotas
	}
	if v, ok := data["features"].([]interface{}); ok {
		tenant.Features = make([]string, len(v))
		for i, f := range v {
			if s, ok := f.(string); ok {
				tenant.Features[i] = s
			}
		}
	}
	if v, ok := data["metadata"].(map[string]interface{}); ok {
		tenant.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				tenant.Metadata[k] = s
			}
		}
	}
	if v, ok := data["tags"].([]interface{}); ok {
		tenant.Tags = make([]string, len(v))
		for i, t := range v {
			if s, ok := t.(string); ok {
				tenant.Tags[i] = s
			}
		}
	}
	if v, ok := data["isSystem"].(bool); ok {
		tenant.IsSystem = v
	}
	if v, ok := data["createdBy"].(string); ok {
		tenant.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		tenant.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			tenant.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			tenant.UpdatedAt = t
		}
	}

	return tenant, nil
}

// mapToTenantUser converts Weaviate response to TenantUser model
func (r *WeaviateRBACRepository) mapToTenantUser(data map[string]any) (*models.TenantUser, error) {
	tenantUser := &models.TenantUser{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		tenantUser.ID = id
	}
	if v, ok := data["tenantId"].(string); ok {
		tenantUser.TenantID = v
	}
	if v, ok := data["userId"].(string); ok {
		tenantUser.UserID = v
	}
	if v, ok := data["tenantRole"].(string); ok {
		tenantUser.TenantRole = v
	}
	if v, ok := data["status"].(string); ok {
		tenantUser.Status = v
	}
	if v, ok := data["invitedBy"].(string); ok {
		tenantUser.InvitedBy = v
	}
	if v, ok := data["additionalPermissions"].([]interface{}); ok {
		tenantUser.AdditionalPermissions = make([]string, len(v))
		for i, p := range v {
			if s, ok := p.(string); ok {
				tenantUser.AdditionalPermissions[i] = s
			}
		}
	}
	if v, ok := data["metadata"].(map[string]interface{}); ok {
		tenantUser.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				tenantUser.Metadata[k] = s
			}
		}
	}
	if v, ok := data["createdBy"].(string); ok {
		tenantUser.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		tenantUser.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			tenantUser.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			tenantUser.UpdatedAt = t
		}
	}
	if v, ok := data["invitedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			tenantUser.InvitedAt = &t
		}
	}
	if v, ok := data["acceptedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			tenantUser.AcceptedAt = &t
		}
	}

	return tenantUser, nil
}

func (r *WeaviateRBACRepository) CreateUser(ctx context.Context, user *models.User) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	// Only generate ID if not already set
	if user.ID == "" {
		user.ID = makeRBACID("User", "", user.Email)
	}
	now := time.Now()

	props := map[string]any{
		"email":            user.Email,
		"username":         user.Username,
		"fullName":         user.FullName,
		"globalRole":       user.GlobalRole,
		"passwordHash":     user.PasswordHash,
		"mfaEnabled":       user.MFAEnabled,
		"mfaSecret":        user.MFASecret,
		"status":           user.Status,
		"emailVerified":    user.EmailVerified,
		"avatar":           user.Avatar,
		"phone":            user.Phone,
		"timezone":         user.Timezone,
		"language":         user.Language,
		"loginCount":       user.LoginCount,
		"failedLoginCount": user.FailedLoginCount,
		"metadata":         user.Metadata,
		"tags":             user.Tags,
		"createdAt":        now.Format(time.RFC3339),
		"updatedAt":        now.Format(time.RFC3339),
		"createdBy":        user.CreatedBy,
		"updatedBy":        user.UpdatedBy,
	}

	// Handle optional timestamp fields
	if user.LastLoginAt != nil {
		props["lastLoginAt"] = user.LastLoginAt.Format(time.RFC3339)
	}
	if user.LockedUntil != nil {
		props["lockedUntil"] = user.LockedUntil.Format(time.RFC3339)
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACUser", user.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_user", user.Email, duration, false)
		return fmt.Errorf("failed to create user %s: %w", user.Email, err)
	}

	monitoring.RecordWeaviateOperation("create_user", user.Email, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) GetUser(ctx context.Context, userID string) (*models.User, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACUser(where: { path: ["id"], operator: Equal, valueString: "%s" }) {
				email username fullName globalRole mfaEnabled status emailVerified
				avatar phone timezone language lastLoginAt loginCount failedLoginCount
				lockedUntil tags createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, userID)

	var resp struct {
		Data struct {
			Get struct {
				RBACUser []map[string]any `json:"RBACUser"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_user", userID, duration, false)
		return nil, fmt.Errorf("failed to get user %s: %w", userID, err)
	}

	monitoring.RecordWeaviateOperation("get_user", userID, duration, true)

	users := resp.Data.Get.RBACUser
	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return r.mapToUser(users[0])
}

func (r *WeaviateRBACRepository) ListUsers(ctx context.Context, filters UserFilters) ([]*models.User, error) {
	// Build where clause based on filters
	operands := []map[string]any{}

	if filters.Email != nil {
		operands = append(operands, map[string]any{
			"path": []string{"email"}, "operator": "Equal", "valueString": *filters.Email,
		})
	}
	if filters.Username != nil {
		operands = append(operands, map[string]any{
			"path": []string{"username"}, "operator": "Equal", "valueString": *filters.Username,
		})
	}
	if filters.GlobalRole != nil {
		operands = append(operands, map[string]any{
			"path": []string{"globalRole"}, "operator": "Equal", "valueString": *filters.GlobalRole,
		})
	}
	if filters.Status != nil {
		operands = append(operands, map[string]any{
			"path": []string{"status"}, "operator": "Equal", "valueString": *filters.Status,
		})
	}

	limit := 100
	if filters.Limit > 0 && filters.Limit <= 1000 {
		limit = filters.Limit
	}

	// Build where clause as GraphQL syntax (unquoted keys, valueString for strings)
	whereStr := ""
	if len(operands) == 1 {
		// Single operand - use it directly
		op := operands[0]
		whereStr = fmt.Sprintf(`, where: { path: ["%s"], operator: Equal, valueString: "%s" }`,
			op["path"].([]string)[0], op["valueString"])
	} else if len(operands) > 1 {
		// Multiple operands - use And operator
		operandStrs := make([]string, len(operands))
		for i, op := range operands {
			operandStrs[i] = fmt.Sprintf(`{ path: ["%s"], operator: Equal, valueString: "%s" }`,
				op["path"].([]string)[0], op["valueString"])
		}
		whereStr = fmt.Sprintf(`, where: { operator: And, operands: [%s] }`,
			strings.Join(operandStrs, ", "))
	}

	query := fmt.Sprintf(`{
		Get {
			RBACUser(limit: %d, offset: %d%s) {
				email username fullName globalRole mfaEnabled status emailVerified
				avatar phone timezone language lastLoginAt loginCount failedLoginCount
				lockedUntil tags createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, limit, filters.Offset, whereStr)

	var resp struct {
		Data struct {
			Get struct {
				RBACUser []map[string]any `json:"RBACUser"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("list_users", "all", duration, false)
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	monitoring.RecordWeaviateOperation("list_users", "all", duration, true)

	users := resp.Data.Get.RBACUser
	result := make([]*models.User, len(users))
	for i, userData := range users {
		user, err := r.mapToUser(userData)
		if err != nil {
			return nil, err
		}
		result[i] = user
	}

	return result, nil
}

func (r *WeaviateRBACRepository) UpdateUser(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()

	props := map[string]any{
		"username":         user.Username,
		"fullName":         user.FullName,
		"globalRole":       user.GlobalRole,
		"passwordHash":     user.PasswordHash,
		"mfaEnabled":       user.MFAEnabled,
		"mfaSecret":        user.MFASecret,
		"status":           user.Status,
		"emailVerified":    user.EmailVerified,
		"avatar":           user.Avatar,
		"phone":            user.Phone,
		"timezone":         user.Timezone,
		"language":         user.Language,
		"loginCount":       user.LoginCount,
		"failedLoginCount": user.FailedLoginCount,
		"metadata":         user.Metadata,
		"tags":             user.Tags,
		"updatedAt":        user.UpdatedAt.Format(time.RFC3339),
		"updatedBy":        user.UpdatedBy,
	}

	// Handle optional timestamp fields
	if user.LastLoginAt != nil {
		props["lastLoginAt"] = user.LastLoginAt.Format(time.RFC3339)
	}
	if user.LockedUntil != nil {
		props["lockedUntil"] = user.LockedUntil.Format(time.RFC3339)
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACUser", user.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_user", user.Email, duration, false)
		return fmt.Errorf("failed to update user %s: %w", user.Email, err)
	}

	monitoring.RecordWeaviateOperation("update_user", user.Email, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) DeleteUser(ctx context.Context, userID string) error {
	start := time.Now()
	err := r.transport.DeleteObject(ctx, userID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_user", userID, duration, false)
		return fmt.Errorf("failed to delete user %s: %w", userID, err)
	}

	monitoring.RecordWeaviateOperation("delete_user", userID, duration, true)
	return nil
}

// mapToUser converts Weaviate response to User model
func (r *WeaviateRBACRepository) mapToUser(data map[string]any) (*models.User, error) {
	user := &models.User{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		user.ID = id
	}
	if v, ok := data["email"].(string); ok {
		user.Email = v
	}
	if v, ok := data["username"].(string); ok {
		user.Username = v
	}
	if v, ok := data["fullName"].(string); ok {
		user.FullName = v
	}
	if v, ok := data["globalRole"].(string); ok {
		user.GlobalRole = v
	}
	if v, ok := data["passwordHash"].(string); ok {
		user.PasswordHash = v
	}
	if v, ok := data["mfaEnabled"].(bool); ok {
		user.MFAEnabled = v
	}
	if v, ok := data["mfaSecret"].(string); ok {
		user.MFASecret = v
	}
	if v, ok := data["status"].(string); ok {
		user.Status = v
	}
	if v, ok := data["emailVerified"].(bool); ok {
		user.EmailVerified = v
	}
	if v, ok := data["avatar"].(string); ok {
		user.Avatar = v
	}
	if v, ok := data["phone"].(string); ok {
		user.Phone = v
	}
	if v, ok := data["timezone"].(string); ok {
		user.Timezone = v
	}
	if v, ok := data["language"].(string); ok {
		user.Language = v
	}
	if v, ok := data["loginCount"].(float64); ok {
		user.LoginCount = int(v)
	}
	if v, ok := data["failedLoginCount"].(float64); ok {
		user.FailedLoginCount = int(v)
	}
	if v, ok := data["metadata"].(map[string]any); ok {
		user.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				user.Metadata[k] = s
			}
		}
	}
	if v, ok := data["tags"].([]interface{}); ok {
		user.Tags = make([]string, len(v))
		for i, t := range v {
			if s, ok := t.(string); ok {
				user.Tags[i] = s
			}
		}
	}
	if v, ok := data["createdBy"].(string); ok {
		user.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		user.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			user.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			user.UpdatedAt = t
		}
	}
	if v, ok := data["lastLoginAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			user.LastLoginAt = &t
		}
	}
	if v, ok := data["lockedUntil"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			user.LockedUntil = &t
		}
	}

	return user, nil
}

func (r *WeaviateRBACRepository) CreateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	tenantUser.ID = makeRBACID("TenantUser", tenantUser.TenantID, tenantUser.UserID)
	now := time.Now()

	props := map[string]any{
		"tenantId":              tenantUser.TenantID,
		"userId":                tenantUser.UserID,
		"tenantRole":            tenantUser.TenantRole,
		"status":                tenantUser.Status,
		"invitedBy":             tenantUser.InvitedBy,
		"additionalPermissions": tenantUser.AdditionalPermissions,
		"metadata":              tenantUser.Metadata,
		"createdAt":             now.Format(time.RFC3339),
		"updatedAt":             now.Format(time.RFC3339),
		"createdBy":             tenantUser.CreatedBy,
		"updatedBy":             tenantUser.UpdatedBy,
	}

	// Handle optional timestamp fields
	if tenantUser.InvitedAt != nil {
		props["invitedAt"] = tenantUser.InvitedAt.Format(time.RFC3339)
	}
	if tenantUser.AcceptedAt != nil {
		props["acceptedAt"] = tenantUser.AcceptedAt.Format(time.RFC3339)
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACTenantUser", tenantUser.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_tenant_user", tenantUser.UserID, duration, false)
		return fmt.Errorf("failed to create tenant-user association for user %s in tenant %s: %w", tenantUser.UserID, tenantUser.TenantID, err)
	}

	monitoring.RecordWeaviateOperation("create_tenant_user", tenantUser.UserID, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) GetTenantUser(ctx context.Context, tenantID, userID string) (*models.TenantUser, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACTenantUser(
				where: {
					operator: And,
					operands: [
						{ path: ["tenantId"], operator: Equal, valueString: "%s" },
						{ path: ["userId"], operator: Equal, valueString: "%s" }
					]
				}
			) {
				tenantId userId tenantRole status invitedBy invitedAt acceptedAt
				additionalPermissions metadata createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, tenantID, userID)

	var resp struct {
		Data struct {
			Get struct {
				RBACTenantUser []map[string]any `json:"RBACTenantUser"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_tenant_user", userID, duration, false)
		return nil, fmt.Errorf("failed to get tenant-user association for user %s in tenant %s: %w", userID, tenantID, err)
	}

	monitoring.RecordWeaviateOperation("get_tenant_user", userID, duration, true)

	associations := resp.Data.Get.RBACTenantUser
	if len(associations) == 0 {
		return nil, fmt.Errorf("tenant-user association not found")
	}

	return r.mapToTenantUser(associations[0])
}

func (r *WeaviateRBACRepository) ListTenantUsers(ctx context.Context, tenantID string, filters TenantUserFilters) ([]*models.TenantUser, error) {
	// Build where clause based on filters
	operands := []map[string]any{
		{"path": []string{"tenantId"}, "operator": "Equal", "valueString": tenantID},
	}

	if filters.UserID != nil {
		operands = append(operands, map[string]any{
			"path": []string{"userId"}, "operator": "Equal", "valueString": *filters.UserID,
		})
	}
	if filters.TenantRole != nil {
		operands = append(operands, map[string]any{
			"path": []string{"tenantRole"}, "operator": "Equal", "valueString": *filters.TenantRole,
		})
	}
	if filters.Status != nil {
		operands = append(operands, map[string]any{
			"path": []string{"status"}, "operator": "Equal", "valueString": *filters.Status,
		})
	}

	whereClause := map[string]any{
		"operator": "And",
		"operands": operands,
	}

	limit := 100
	if filters.Limit > 0 && filters.Limit <= 1000 {
		limit = filters.Limit
	}

	query := map[string]any{
		"query": fmt.Sprintf(`{
			Get {
				RBACTenantUser(where: $where, limit: %d, offset: %d) {
					tenantId userId tenantRole status invitedBy invitedAt acceptedAt
					additionalPermissions metadata createdAt updatedAt createdBy updatedBy
					_additional { id }
				}
			}
		}`, limit, filters.Offset),
		"variables": map[string]any{"where": whereClause},
	}

	var resp struct {
		Data struct {
			Get struct {
				RBACTenantUser []map[string]any `json:"RBACTenantUser"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query["query"].(string), query["variables"].(map[string]any), &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("list_tenant_users", tenantID, duration, false)
		return nil, fmt.Errorf("failed to list tenant users for tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("list_tenant_users", tenantID, duration, true)

	associations := resp.Data.Get.RBACTenantUser
	result := make([]*models.TenantUser, len(associations))
	for i, associationData := range associations {
		association, err := r.mapToTenantUser(associationData)
		if err != nil {
			return nil, err
		}
		result[i] = association
	}

	return result, nil
}

func (r *WeaviateRBACRepository) UpdateTenantUser(ctx context.Context, tenantUser *models.TenantUser) error {
	tenantUser.UpdatedAt = time.Now()

	props := map[string]any{
		"tenantRole":            tenantUser.TenantRole,
		"status":                tenantUser.Status,
		"invitedBy":             tenantUser.InvitedBy,
		"additionalPermissions": tenantUser.AdditionalPermissions,
		"metadata":              tenantUser.Metadata,
		"updatedAt":             tenantUser.UpdatedAt.Format(time.RFC3339),
		"updatedBy":             tenantUser.UpdatedBy,
	}

	// Handle optional timestamp fields
	if tenantUser.InvitedAt != nil {
		props["invitedAt"] = tenantUser.InvitedAt.Format(time.RFC3339)
	}
	if tenantUser.AcceptedAt != nil {
		props["acceptedAt"] = tenantUser.AcceptedAt.Format(time.RFC3339)
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACTenantUser", tenantUser.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_tenant_user", tenantUser.UserID, duration, false)
		return fmt.Errorf("failed to update tenant-user association for user %s in tenant %s: %w", tenantUser.UserID, tenantUser.TenantID, err)
	}

	monitoring.RecordWeaviateOperation("update_tenant_user", tenantUser.UserID, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) DeleteTenantUser(ctx context.Context, tenantID, userID string) error {
	tenantUserID := makeRBACID("TenantUser", tenantID, userID)

	start := time.Now()
	err := r.transport.DeleteObject(ctx, tenantUserID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_tenant_user", userID, duration, false)
		return fmt.Errorf("failed to delete tenant-user association for user %s in tenant %s: %w", userID, tenantID, err)
	}

	monitoring.RecordWeaviateOperation("delete_tenant_user", userID, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) CreateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	auth.ID = makeRBACID("MiradorAuth", auth.TenantID, auth.UserID)
	now := time.Now()

	props := map[string]any{
		"userId":                auth.UserID,
		"username":              auth.Username,
		"email":                 auth.Email,
		"passwordHash":          auth.PasswordHash,
		"salt":                  auth.Salt,
		"totpSecret":            auth.TOTPSecret,
		"totpEnabled":           auth.TOTPEnabled,
		"backupCodes":           auth.BackupCodes,
		"tenantId":              auth.TenantID,
		"roles":                 auth.Roles,
		"groups":                auth.Groups,
		"isActive":              auth.IsActive,
		"failedLoginCount":      auth.FailedLoginCount,
		"requirePasswordChange": auth.RequirePasswordChange,
		"metadata":              auth.Metadata,
		"createdAt":             now.Format(time.RFC3339),
		"updatedAt":             now.Format(time.RFC3339),
		"createdBy":             auth.CreatedBy,
		"updatedBy":             auth.UpdatedBy,
	}

	// Handle optional timestamp fields
	if auth.PasswordChangedAt != nil {
		props["passwordChangedAt"] = auth.PasswordChangedAt.Format(time.RFC3339)
	}
	if auth.PasswordExpiresAt != nil {
		props["passwordExpiresAt"] = auth.PasswordExpiresAt.Format(time.RFC3339)
	}
	if auth.LastLoginAt != nil {
		props["lastLoginAt"] = auth.LastLoginAt.Format(time.RFC3339)
	}
	if auth.LockedUntil != nil {
		props["lockedUntil"] = auth.LockedUntil.Format(time.RFC3339)
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACMiradorAuth", auth.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_mirador_auth", auth.UserID, duration, false)
		return fmt.Errorf("failed to create mirador auth for user %s: %w", auth.UserID, err)
	}

	monitoring.RecordWeaviateOperation("create_mirador_auth", auth.UserID, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) GetMiradorAuth(ctx context.Context, userID string) (*models.MiradorAuth, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACMiradorAuth(where: { path: ["userId"], operator: Equal, valueString: "%s" }) {
				userId username email passwordHash salt totpSecret totpEnabled
				backupCodes tenantId roles groups isActive passwordChangedAt
				passwordExpiresAt lastLoginAt failedLoginCount lockedUntil
				requirePasswordChange createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, userID)

	var resp struct {
		Data struct {
			Get struct {
				RBACMiradorAuth []map[string]any `json:"RBACMiradorAuth"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_mirador_auth", userID, duration, false)
		return nil, fmt.Errorf("failed to get mirador auth for user %s: %w", userID, err)
	}

	monitoring.RecordWeaviateOperation("get_mirador_auth", userID, duration, true)

	auths := resp.Data.Get.RBACMiradorAuth
	if len(auths) == 0 {
		return nil, fmt.Errorf("mirador auth not found")
	}

	return r.mapToMiradorAuth(auths[0])
}

func (r *WeaviateRBACRepository) UpdateMiradorAuth(ctx context.Context, auth *models.MiradorAuth) error {
	auth.UpdatedAt = time.Now()

	props := map[string]any{
		"username":              auth.Username,
		"email":                 auth.Email,
		"passwordHash":          auth.PasswordHash,
		"salt":                  auth.Salt,
		"totpSecret":            auth.TOTPSecret,
		"totpEnabled":           auth.TOTPEnabled,
		"backupCodes":           auth.BackupCodes,
		"roles":                 auth.Roles,
		"groups":                auth.Groups,
		"isActive":              auth.IsActive,
		"failedLoginCount":      auth.FailedLoginCount,
		"requirePasswordChange": auth.RequirePasswordChange,
		"metadata":              auth.Metadata,
		"updatedAt":             auth.UpdatedAt.Format(time.RFC3339),
		"updatedBy":             auth.UpdatedBy,
	}

	// Handle optional timestamp fields
	if auth.PasswordChangedAt != nil {
		props["passwordChangedAt"] = auth.PasswordChangedAt.Format(time.RFC3339)
	}
	if auth.PasswordExpiresAt != nil {
		props["passwordExpiresAt"] = auth.PasswordExpiresAt.Format(time.RFC3339)
	}
	if auth.LastLoginAt != nil {
		props["lastLoginAt"] = auth.LastLoginAt.Format(time.RFC3339)
	}
	if auth.LockedUntil != nil {
		props["lockedUntil"] = auth.LockedUntil.Format(time.RFC3339)
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACMiradorAuth", auth.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_mirador_auth", auth.UserID, duration, false)
		return fmt.Errorf("failed to update mirador auth for user %s: %w", auth.UserID, err)
	}

	monitoring.RecordWeaviateOperation("update_mirador_auth", auth.UserID, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) DeleteMiradorAuth(ctx context.Context, userID string) error {
	// First get the auth record to find its ID
	auth, err := r.GetMiradorAuth(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get mirador auth for deletion: %w", err)
	}

	start := time.Now()
	err = r.transport.DeleteObject(ctx, auth.ID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_mirador_auth", userID, duration, false)
		return fmt.Errorf("failed to delete mirador auth for user %s: %w", userID, err)
	}

	monitoring.RecordWeaviateOperation("delete_mirador_auth", userID, duration, true)
	return nil
}

// mapToMiradorAuth converts Weaviate response to MiradorAuth model
func (r *WeaviateRBACRepository) mapToMiradorAuth(data map[string]any) (*models.MiradorAuth, error) {
	auth := &models.MiradorAuth{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		auth.ID = id
	}
	if v, ok := data["userId"].(string); ok {
		auth.UserID = v
	}
	if v, ok := data["username"].(string); ok {
		auth.Username = v
	}
	if v, ok := data["email"].(string); ok {
		auth.Email = v
	}
	if v, ok := data["passwordHash"].(string); ok {
		auth.PasswordHash = v
	}
	if v, ok := data["salt"].(string); ok {
		auth.Salt = v
	}
	if v, ok := data["totpSecret"].(string); ok {
		auth.TOTPSecret = v
	}
	if v, ok := data["totpEnabled"].(bool); ok {
		auth.TOTPEnabled = v
	}
	if v, ok := data["backupCodes"].([]interface{}); ok {
		auth.BackupCodes = make([]string, len(v))
		for i, b := range v {
			if s, ok := b.(string); ok {
				auth.BackupCodes[i] = s
			}
		}
	}
	if v, ok := data["tenantId"].(string); ok {
		auth.TenantID = v
	}
	if v, ok := data["roles"].([]interface{}); ok {
		auth.Roles = make([]string, len(v))
		for i, r := range v {
			if s, ok := r.(string); ok {
				auth.Roles[i] = s
			}
		}
	}
	if v, ok := data["groups"].([]interface{}); ok {
		auth.Groups = make([]string, len(v))
		for i, g := range v {
			if s, ok := g.(string); ok {
				auth.Groups[i] = s
			}
		}
	}
	if v, ok := data["isActive"].(bool); ok {
		auth.IsActive = v
	}
	if v, ok := data["failedLoginCount"].(float64); ok {
		auth.FailedLoginCount = int(v)
	}
	if v, ok := data["requirePasswordChange"].(bool); ok {
		auth.RequirePasswordChange = v
	}
	if v, ok := data["metadata"].(map[string]any); ok {
		auth.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				auth.Metadata[k] = s
			}
		}
	}
	if v, ok := data["createdBy"].(string); ok {
		auth.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		auth.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			auth.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			auth.UpdatedAt = t
		}
	}
	if v, ok := data["passwordChangedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			auth.PasswordChangedAt = &t
		}
	}
	if v, ok := data["passwordExpiresAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			auth.PasswordExpiresAt = &t
		}
	}
	if v, ok := data["lastLoginAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			auth.LastLoginAt = &t
		}
	}
	if v, ok := data["lockedUntil"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			auth.LockedUntil = &t
		}
	}

	return auth, nil
}

func (r *WeaviateRBACRepository) CreateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	if err := r.ensureRBACSchema(ctx); err != nil {
		return err
	}

	config.ID = makeRBACID("AuthConfig", config.TenantID, "")
	now := time.Now()

	props := map[string]any{
		"tenantId":              config.TenantID,
		"defaultBackend":        config.DefaultBackend,
		"enabledBackends":       config.EnabledBackends,
		"backendConfigs":        config.BackendConfigs,
		"passwordPolicy":        config.PasswordPolicy,
		"require2fa":            config.Require2FA,
		"totpIssuer":            config.TOTPIssuer,
		"sessionTimeoutMinutes": config.SessionTimeoutMinutes,
		"maxConcurrentSessions": config.MaxConcurrentSessions,
		"allowRememberMe":       config.AllowRememberMe,
		"rememberMeDays":        config.RememberMeDays,
		"metadata":              config.Metadata,
		"createdAt":             now.Format(time.RFC3339),
		"updatedAt":             now.Format(time.RFC3339),
		"createdBy":             config.CreatedBy,
		"updatedBy":             config.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACAuthConfig", config.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("create_auth_config", config.TenantID, duration, false)
		return fmt.Errorf("failed to create auth config for tenant %s: %w", config.TenantID, err)
	}

	monitoring.RecordWeaviateOperation("create_auth_config", config.TenantID, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) GetAuthConfig(ctx context.Context, tenantID string) (*models.AuthConfig, error) {
	query := fmt.Sprintf(`{
		Get {
			RBACAuthConfig(where: { path: ["tenantId"], operator: Equal, valueString: "%s" }) {
				tenantId defaultBackend enabledBackends backendConfigs passwordPolicy
				require2fa totpIssuer sessionTimeoutMinutes maxConcurrentSessions
				allowRememberMe rememberMeDays metadata createdAt updatedAt createdBy updatedBy
				_additional { id }
			}
		}
	}`, tenantID)

	var resp struct {
		Data struct {
			Get struct {
				RBACAuthConfig []map[string]any `json:"RBACAuthConfig"`
			} `json:"Get"`
		} `json:"data"`
	}

	start := time.Now()
	err := r.transport.GraphQL(ctx, query, nil, &resp)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("get_auth_config", tenantID, duration, false)
		return nil, fmt.Errorf("failed to get auth config for tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("get_auth_config", tenantID, duration, true)

	configs := resp.Data.Get.RBACAuthConfig
	if len(configs) == 0 {
		return nil, fmt.Errorf("auth config not found")
	}

	return r.mapToAuthConfig(configs[0])
}

func (r *WeaviateRBACRepository) UpdateAuthConfig(ctx context.Context, config *models.AuthConfig) error {
	config.UpdatedAt = time.Now()

	props := map[string]any{
		"defaultBackend":        config.DefaultBackend,
		"enabledBackends":       config.EnabledBackends,
		"backendConfigs":        config.BackendConfigs,
		"passwordPolicy":        config.PasswordPolicy,
		"require2fa":            config.Require2FA,
		"totpIssuer":            config.TOTPIssuer,
		"sessionTimeoutMinutes": config.SessionTimeoutMinutes,
		"maxConcurrentSessions": config.MaxConcurrentSessions,
		"allowRememberMe":       config.AllowRememberMe,
		"rememberMeDays":        config.RememberMeDays,
		"metadata":              config.Metadata,
		"updatedAt":             config.UpdatedAt.Format(time.RFC3339),
		"updatedBy":             config.UpdatedBy,
	}

	start := time.Now()
	err := r.transport.PutObject(ctx, "RBACAuthConfig", config.ID, props)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("update_auth_config", config.TenantID, duration, false)
		return fmt.Errorf("failed to update auth config for tenant %s: %w", config.TenantID, err)
	}

	monitoring.RecordWeaviateOperation("update_auth_config", config.TenantID, duration, true)
	return nil
}

func (r *WeaviateRBACRepository) DeleteAuthConfig(ctx context.Context, tenantID string) error {
	// First get the config to find its ID
	config, err := r.GetAuthConfig(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get auth config for deletion: %w", err)
	}

	start := time.Now()
	err = r.transport.DeleteObject(ctx, config.ID)
	duration := time.Since(start)

	if err != nil {
		monitoring.RecordWeaviateOperation("delete_auth_config", tenantID, duration, false)
		return fmt.Errorf("failed to delete auth config for tenant %s: %w", tenantID, err)
	}

	monitoring.RecordWeaviateOperation("delete_auth_config", tenantID, duration, true)
	return nil
}

// mapToAuthConfig converts Weaviate response to AuthConfig model
func (r *WeaviateRBACRepository) mapToAuthConfig(data map[string]any) (*models.AuthConfig, error) {
	config := &models.AuthConfig{}

	if id, ok := data["_additional"].(map[string]any)["id"].(string); ok {
		config.ID = id
	}
	if v, ok := data["tenantId"].(string); ok {
		config.TenantID = v
	}
	if v, ok := data["defaultBackend"].(string); ok {
		config.DefaultBackend = v
	}
	if v, ok := data["enabledBackends"].([]interface{}); ok {
		config.EnabledBackends = make([]string, len(v))
		for i, b := range v {
			if s, ok := b.(string); ok {
				config.EnabledBackends[i] = s
			}
		}
	}
	if v, ok := data["backendConfigs"].(map[string]any); ok {
		backendConfigs := models.AuthBackendConfigs{}
		if saml, ok := v["saml"].(map[string]any); ok {
			samlConfig := models.SAMLConfig{}
			if entityID, ok := saml["entityId"].(string); ok {
				samlConfig.EntityID = entityID
			}
			if acsURL, ok := saml["acsUrl"].(string); ok {
				samlConfig.ACSURL = acsURL
			}
			if metadataURL, ok := saml["metadataUrl"].(string); ok {
				samlConfig.MetadataURL = metadataURL
			}
			if signingCert, ok := saml["signingCert"].(string); ok {
				samlConfig.SigningCert = signingCert
			}
			if encryptionCert, ok := saml["encryptionCert"].(string); ok {
				samlConfig.EncryptionCert = encryptionCert
			}
			if nameIDFormat, ok := saml["nameIdFormat"].(string); ok {
				samlConfig.NameIDFormat = nameIDFormat
			}
			if attributeMapping, ok := saml["attributeMapping"].(map[string]any); ok {
				samlConfig.AttributeMapping = make(map[string]string)
				for k, val := range attributeMapping {
					if s, ok := val.(string); ok {
						samlConfig.AttributeMapping[k] = s
					}
				}
			}
			backendConfigs.SAML = samlConfig
		}
		if oidc, ok := v["oidc"].(map[string]any); ok {
			oidcConfig := models.OIDCConfig{}
			if clientID, ok := oidc["clientId"].(string); ok {
				oidcConfig.ClientID = clientID
			}
			if clientSecret, ok := oidc["clientSecret"].(string); ok {
				oidcConfig.ClientSecret = clientSecret
			}
			if issuerURL, ok := oidc["issuerUrl"].(string); ok {
				oidcConfig.IssuerURL = issuerURL
			}
			if redirectURL, ok := oidc["redirectUrl"].(string); ok {
				oidcConfig.RedirectURL = redirectURL
			}
			if scopes, ok := oidc["scopes"].([]interface{}); ok {
				oidcConfig.Scopes = make([]string, len(scopes))
				for i, s := range scopes {
					if str, ok := s.(string); ok {
						oidcConfig.Scopes[i] = str
					}
				}
			}
			if attributeMapping, ok := oidc["attributeMapping"].(map[string]any); ok {
				oidcConfig.AttributeMapping = make(map[string]string)
				for k, val := range attributeMapping {
					if s, ok := val.(string); ok {
						oidcConfig.AttributeMapping[k] = s
					}
				}
			}
			backendConfigs.OIDC = oidcConfig
		}
		if ldap, ok := v["ldap"].(map[string]any); ok {
			ldapConfig := models.LDAPConfig{}
			if host, ok := ldap["host"].(string); ok {
				ldapConfig.Host = host
			}
			if port, ok := ldap["port"].(float64); ok {
				ldapConfig.Port = int(port)
			}
			if useTLS, ok := ldap["useTls"].(bool); ok {
				ldapConfig.UseTLS = useTLS
			}
			if bindDN, ok := ldap["bindDn"].(string); ok {
				ldapConfig.BindDN = bindDN
			}
			if bindPassword, ok := ldap["bindPassword"].(string); ok {
				ldapConfig.BindPassword = bindPassword
			}
			if baseDN, ok := ldap["baseDn"].(string); ok {
				ldapConfig.BaseDN = baseDN
			}
			if userFilter, ok := ldap["userFilter"].(string); ok {
				ldapConfig.UserFilter = userFilter
			}
			if groupFilter, ok := ldap["groupFilter"].(string); ok {
				ldapConfig.GroupFilter = groupFilter
			}
			if attributeMapping, ok := ldap["attributeMapping"].(map[string]any); ok {
				ldapConfig.AttributeMapping = make(map[string]string)
				for k, val := range attributeMapping {
					if s, ok := val.(string); ok {
						ldapConfig.AttributeMapping[k] = s
					}
				}
			}
			backendConfigs.LDAP = ldapConfig
		}
		config.BackendConfigs = backendConfigs
	}
	if v, ok := data["passwordPolicy"].(map[string]any); ok {
		policy := models.PasswordPolicy{}
		if minLength, ok := v["minLength"].(float64); ok {
			policy.MinLength = int(minLength)
		}
		if requireUppercase, ok := v["requireUppercase"].(bool); ok {
			policy.RequireUppercase = requireUppercase
		}
		if requireLowercase, ok := v["requireLowercase"].(bool); ok {
			policy.RequireLowercase = requireLowercase
		}
		if requireNumbers, ok := v["requireNumbers"].(bool); ok {
			policy.RequireNumbers = requireNumbers
		}
		if requireSymbols, ok := v["requireSymbols"].(bool); ok {
			policy.RequireSymbols = requireSymbols
		}
		if maxAgeDays, ok := v["maxAgeDays"].(float64); ok {
			policy.MaxAgeDays = int(maxAgeDays)
		}
		if preventReuseCount, ok := v["preventReuseCount"].(float64); ok {
			policy.PreventReuseCount = int(preventReuseCount)
		}
		if lockoutThreshold, ok := v["lockoutThreshold"].(float64); ok {
			policy.LockoutThreshold = int(lockoutThreshold)
		}
		if lockoutDurationMinutes, ok := v["lockoutDurationMinutes"].(float64); ok {
			policy.LockoutDurationMinutes = int(lockoutDurationMinutes)
		}
		config.PasswordPolicy = policy
	}
	if v, ok := data["require2fa"].(bool); ok {
		config.Require2FA = v
	}
	if v, ok := data["totpIssuer"].(string); ok {
		config.TOTPIssuer = v
	}
	if v, ok := data["sessionTimeoutMinutes"].(float64); ok {
		config.SessionTimeoutMinutes = int(v)
	}
	if v, ok := data["maxConcurrentSessions"].(float64); ok {
		config.MaxConcurrentSessions = int(v)
	}
	if v, ok := data["allowRememberMe"].(bool); ok {
		config.AllowRememberMe = v
	}
	if v, ok := data["rememberMeDays"].(float64); ok {
		config.RememberMeDays = int(v)
	}
	if v, ok := data["metadata"].(map[string]any); ok {
		config.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				config.Metadata[k] = s
			}
		}
	}
	if v, ok := data["createdBy"].(string); ok {
		config.CreatedBy = v
	}
	if v, ok := data["updatedBy"].(string); ok {
		config.UpdatedBy = v
	}

	// Parse timestamps
	if v, ok := data["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			config.CreatedAt = t
		}
	}
	if v, ok := data["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			config.UpdatedAt = t
		}
	}

	return config, nil
}
