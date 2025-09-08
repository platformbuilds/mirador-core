package auth

import "testing"

func TestExtractTenantFromOU(t *testing.T) {
    if got := extractTenantFromOU("ou=ten-123,dc=example"); got != "ten-123" {
        t.Fatalf("expected ten-123, got %s", got)
    }
    if got := extractTenantFromOU(""); got != "default" { t.Fatalf("expected default, got %s", got) }
}

func TestExtractRolesFromMemberOf(t *testing.T) {
    roles := extractRolesFromMemberOf([]string{"cn=admin,ou=roles", "cn=user,ou=roles"})
    if len(roles) != 1 { t.Fatalf("expected 1 role, got %d", len(roles)) }
}
