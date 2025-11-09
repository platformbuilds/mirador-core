package models

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// Validation errors
var (
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrInvalidUsername    = errors.New("invalid username format")
	ErrInvalidRole        = errors.New("invalid role")
	ErrInvalidStatus      = errors.New("invalid status")
	ErrInvalidTenantID    = errors.New("invalid tenant ID")
	ErrInvalidUserID      = errors.New("invalid user ID")
	ErrInvalidPermission  = errors.New("invalid permission")
	ErrInvalidGroupID     = errors.New("invalid group ID")
	ErrInvalidResource    = errors.New("invalid resource")
	ErrInvalidAction      = errors.New("invalid action")
	ErrInvalidScope       = errors.New("invalid scope")
	ErrInvalidSubjectType = errors.New("invalid subject type")
	ErrInvalidPrecedence  = errors.New("invalid precedence")
	ErrInvalidSeverity    = errors.New("invalid severity")
	ErrInvalidRetention   = errors.New("invalid retention class")
	ErrRequiredField      = errors.New("required field is missing")
	ErrInvalidTimeRange   = errors.New("invalid time range")
	ErrInvalidIPRange     = errors.New("invalid IP range")
)

// Valid values for enums
var (
	validGlobalRoles = map[string]bool{
		"global_admin":        true,
		"global_tenant_admin": true,
		"tenant_user":         true,
	}

	validTenantRoles = map[string]bool{
		"tenant_admin":  true,
		"tenant_editor": true,
		"tenant_guest":  true,
	}

	validTenantStatuses = map[string]bool{
		"active":           true,
		"suspended":        true,
		"pending_deletion": true,
	}

	validUserStatuses = map[string]bool{
		"active":               true,
		"suspended":            true,
		"pending_verification": true,
		"deactivated":          true,
	}

	validTenantUserStatuses = map[string]bool{
		"active":    true,
		"invited":   true,
		"suspended": true,
		"removed":   true,
	}

	validResources = map[string]bool{
		"dashboard":      true,
		"kpi_definition": true,
		"layout":         true,
		"user_prefs":     true,
		"admin":          true,
		"rbac":           true,
	}

	validActions = map[string]bool{
		"create": true,
		"read":   true,
		"update": true,
		"delete": true,
		"list":   true,
		"admin":  true,
	}

	validScopes = map[string]bool{
		"global":   true,
		"tenant":   true,
		"resource": true,
	}

	validSubjectTypes = map[string]bool{
		"user":  true,
		"group": true,
	}

	validPrecedences = map[string]bool{
		"allow": true,
		"deny":  true,
	}

	validSeverities = map[string]bool{
		"low":      true,
		"medium":   true,
		"high":     true,
		"critical": true,
	}

	validRetentionClasses = map[string]bool{
		"standard":  true,
		"extended":  true,
		"permanent": true,
	}

	validAuditResults = map[string]bool{
		"success": true,
		"failure": true,
		"denied":  true,
		"error":   true,
	}

	validAuditSources = map[string]bool{
		"api":    true,
		"auth":   true,
		"rbac":   true,
		"system": true,
	}

	validSubjectTypesAudit = map[string]bool{
		"user":            true,
		"service_account": true,
		"system":          true,
	}
)

// Email validation regex
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Username validation regex (alphanumeric, underscore, dash, 3-50 chars)
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`)

// UUID validation regex
var uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// ValidateTenant validates a Tenant struct
func (t *Tenant) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("%w: tenant ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(t.ID) {
		return fmt.Errorf("%w: tenant ID must be a valid UUID", ErrInvalidTenantID)
	}
	if t.Name == "" {
		return fmt.Errorf("%w: tenant name", ErrRequiredField)
	}
	if len(t.Name) < 3 || len(t.Name) > 100 {
		return errors.New("tenant name must be between 3 and 100 characters")
	}
	if t.DisplayName == "" {
		return fmt.Errorf("%w: tenant display name", ErrRequiredField)
	}
	if t.AdminEmail == "" {
		return fmt.Errorf("%w: admin email", ErrRequiredField)
	}
	if !emailRegex.MatchString(t.AdminEmail) {
		return ErrInvalidEmail
	}
	if !validTenantStatuses[t.Status] {
		return ErrInvalidStatus
	}
	if t.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if t.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateTenantUser validates a TenantUser struct
func (tu *TenantUser) Validate() error {
	if tu.ID == "" {
		return fmt.Errorf("%w: tenant user ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(tu.ID) {
		return errors.New("tenant user ID must be a valid UUID")
	}
	if tu.TenantID == "" {
		return fmt.Errorf("%w: tenant ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(tu.TenantID) {
		return ErrInvalidTenantID
	}
	if tu.UserID == "" {
		return fmt.Errorf("%w: user ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(tu.UserID) {
		return ErrInvalidUserID
	}
	if !validTenantRoles[tu.TenantRole] {
		return ErrInvalidRole
	}
	if !validTenantUserStatuses[tu.Status] {
		return ErrInvalidStatus
	}
	if tu.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if tu.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateUser validates a User struct
func (u *User) Validate() error {
	if u.ID == "" {
		return fmt.Errorf("%w: user ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(u.ID) {
		return errors.New("user ID must be a valid UUID")
	}
	if u.Email == "" {
		return fmt.Errorf("%w: email", ErrRequiredField)
	}
	if !emailRegex.MatchString(u.Email) {
		return ErrInvalidEmail
	}
	if u.Username != "" && !usernameRegex.MatchString(u.Username) {
		return ErrInvalidUsername
	}
	if !validGlobalRoles[u.GlobalRole] {
		return ErrInvalidRole
	}
	if !validUserStatuses[u.Status] {
		return ErrInvalidStatus
	}
	if u.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if u.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateMiradorAuth validates a MiradorAuth struct
func (ma *MiradorAuth) Validate() error {
	if ma.ID == "" {
		return fmt.Errorf("%w: mirador auth ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(ma.ID) {
		return errors.New("mirador auth ID must be a valid UUID")
	}
	if ma.UserID == "" {
		return fmt.Errorf("%w: user ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(ma.UserID) {
		return ErrInvalidUserID
	}
	if ma.Username != "" && !usernameRegex.MatchString(ma.Username) {
		return ErrInvalidUsername
	}
	if ma.Email == "" {
		return fmt.Errorf("%w: email", ErrRequiredField)
	}
	if !emailRegex.MatchString(ma.Email) {
		return ErrInvalidEmail
	}
	if ma.TenantID == "" {
		return fmt.Errorf("%w: tenant ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(ma.TenantID) {
		return ErrInvalidTenantID
	}
	if ma.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if ma.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateAuthConfig validates an AuthConfig struct
func (ac *AuthConfig) Validate() error {
	if ac.ID == "" {
		return fmt.Errorf("%w: auth config ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(ac.ID) {
		return errors.New("auth config ID must be a valid UUID")
	}
	if ac.TenantID == "" {
		return fmt.Errorf("%w: tenant ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(ac.TenantID) {
		return ErrInvalidTenantID
	}
	validBackends := map[string]bool{
		"local": true,
		"saml":  true,
		"oidc":  true,
		"ldap":  true,
	}
	if !validBackends[ac.DefaultBackend] {
		return errors.New("invalid default backend")
	}
	for _, backend := range ac.EnabledBackends {
		if !validBackends[backend] {
			return errors.New("invalid enabled backend")
		}
	}
	if ac.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if ac.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateRole validates a Role struct
func (r *Role) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("%w: role ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(r.ID) {
		return errors.New("role ID must be a valid UUID")
	}
	if r.Name == "" {
		return fmt.Errorf("%w: role name", ErrRequiredField)
	}
	if len(r.Name) < 2 || len(r.Name) > 100 {
		return errors.New("role name must be between 2 and 100 characters")
	}
	if r.TenantID == "" {
		return fmt.Errorf("%w: tenant ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(r.TenantID) {
		return ErrInvalidTenantID
	}
	for _, permID := range r.Permissions {
		if !uuidRegex.MatchString(permID) {
			return ErrInvalidPermission
		}
	}
	for _, parentID := range r.ParentRoles {
		if !uuidRegex.MatchString(parentID) {
			return errors.New("parent role ID must be a valid UUID")
		}
	}
	if r.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if r.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidatePermission validates a Permission struct
func (p *Permission) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("%w: permission ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(p.ID) {
		return errors.New("permission ID must be a valid UUID")
	}
	if !validResources[p.Resource] {
		return ErrInvalidResource
	}
	if !validActions[p.Action] {
		return ErrInvalidAction
	}
	if !validScopes[p.Scope] {
		return ErrInvalidScope
	}
	if p.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if p.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateGroup validates a Group struct
func (g *Group) Validate() error {
	if g.ID == "" {
		return fmt.Errorf("%w: group ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(g.ID) {
		return errors.New("group ID must be a valid UUID")
	}
	if g.Name == "" {
		return fmt.Errorf("%w: group name", ErrRequiredField)
	}
	if len(g.Name) < 2 || len(g.Name) > 100 {
		return errors.New("group name must be between 2 and 100 characters")
	}
	if g.TenantID == "" {
		return fmt.Errorf("%w: tenant ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(g.TenantID) {
		return ErrInvalidTenantID
	}
	for _, memberID := range g.Members {
		if !uuidRegex.MatchString(memberID) {
			return ErrInvalidUserID
		}
	}
	for _, roleID := range g.Roles {
		if !uuidRegex.MatchString(roleID) {
			return errors.New("role ID must be a valid UUID")
		}
	}
	for _, parentID := range g.ParentGroups {
		if !uuidRegex.MatchString(parentID) {
			return ErrInvalidGroupID
		}
	}
	if g.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if g.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateRoleBinding validates a RoleBinding struct
func (rb *RoleBinding) Validate() error {
	if rb.ID == "" {
		return fmt.Errorf("%w: role binding ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(rb.ID) {
		return errors.New("role binding ID must be a valid UUID")
	}
	if !validSubjectTypes[rb.SubjectType] {
		return ErrInvalidSubjectType
	}
	if rb.SubjectID == "" {
		return fmt.Errorf("%w: subject ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(rb.SubjectID) {
		return errors.New("subject ID must be a valid UUID")
	}
	if rb.RoleID == "" {
		return fmt.Errorf("%w: role ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(rb.RoleID) {
		return errors.New("role ID must be a valid UUID")
	}
	if rb.Scope != "" && !validScopes[rb.Scope] {
		return ErrInvalidScope
	}
	if !validPrecedences[rb.Precedence] {
		return ErrInvalidPrecedence
	}
	if rb.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if rb.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateGroupBinding validates a GroupBinding struct
func (gb *GroupBinding) Validate() error {
	if gb.ID == "" {
		return fmt.Errorf("%w: group binding ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(gb.ID) {
		return errors.New("group binding ID must be a valid UUID")
	}
	if gb.UserID == "" {
		return fmt.Errorf("%w: user ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(gb.UserID) {
		return ErrInvalidUserID
	}
	if gb.GroupID == "" {
		return fmt.Errorf("%w: group ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(gb.GroupID) {
		return ErrInvalidGroupID
	}
	if gb.TenantID == "" {
		return fmt.Errorf("%w: tenant ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(gb.TenantID) {
		return ErrInvalidTenantID
	}
	if gb.CreatedAt.IsZero() {
		return errors.New("createdAt must be set")
	}
	if gb.UpdatedAt.IsZero() {
		return errors.New("updatedAt must be set")
	}
	return nil
}

// ValidateAuditLog validates an AuditLog struct
func (al *AuditLog) Validate() error {
	if al.ID == "" {
		return fmt.Errorf("%w: audit log ID", ErrRequiredField)
	}
	if !uuidRegex.MatchString(al.ID) {
		return errors.New("audit log ID must be a valid UUID")
	}
	if al.Timestamp.IsZero() {
		return errors.New("timestamp must be set")
	}
	if al.TenantID != "" && !uuidRegex.MatchString(al.TenantID) {
		return ErrInvalidTenantID
	}
	if al.SubjectID != "" && !uuidRegex.MatchString(al.SubjectID) {
		return errors.New("subject ID must be a valid UUID")
	}
	if !validSubjectTypesAudit[al.SubjectType] {
		return errors.New("invalid subject type")
	}
	if al.Action == "" {
		return fmt.Errorf("%w: action", ErrRequiredField)
	}
	if al.Resource == "" {
		return fmt.Errorf("%w: resource", ErrRequiredField)
	}
	if !validAuditResults[al.Result] {
		return errors.New("invalid result")
	}
	if !validSeverities[al.Severity] {
		return ErrInvalidSeverity
	}
	if !validAuditSources[al.Source] {
		return errors.New("invalid source")
	}
	if !validRetentionClasses[al.RetentionClass] {
		return ErrInvalidRetention
	}
	return nil
}

// ValidateIPRanges validates a list of IP ranges
func ValidateIPRanges(ipRanges []string) error {
	for _, ipRange := range ipRanges {
		if _, _, err := net.ParseCIDR(ipRange); err != nil {
			if net.ParseIP(ipRange) == nil {
				return fmt.Errorf("%w: %s", ErrInvalidIPRange, ipRange)
			}
		}
	}
	return nil
}

// ValidateTimeWindows validates time window conditions
func ValidateTimeWindows(timeWindows []TimeWindowCondition) error {
	for _, tw := range timeWindows {
		for _, day := range tw.DaysOfWeek {
			validDays := map[string]bool{
				"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
				"friday": true, "saturday": true, "sunday": true,
			}
			if !validDays[strings.ToLower(day)] {
				return errors.New("invalid day of week: " + day)
			}
		}
		if tw.StartTime != "" {
			if _, err := time.Parse("15:04", tw.StartTime); err != nil {
				return fmt.Errorf("invalid start time format: %s", tw.StartTime)
			}
		}
		if tw.EndTime != "" {
			if _, err := time.Parse("15:04", tw.EndTime); err != nil {
				return fmt.Errorf("invalid end time format: %s", tw.EndTime)
			}
		}
	}
	return nil
}

// IsExpired checks if a time-based entity is expired
func IsExpired(expiresAt *time.Time) bool {
	if expiresAt == nil {
		return false
	}
	return time.Now().After(*expiresAt)
}

// IsActive checks if a time-based entity is currently active
func IsActive(notBefore, expiresAt *time.Time) bool {
	now := time.Now()
	if notBefore != nil && now.Before(*notBefore) {
		return false
	}
	if expiresAt != nil && now.After(*expiresAt) {
		return false
	}
	return true
}
