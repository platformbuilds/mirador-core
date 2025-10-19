package middleware

import (
	"testing"
	"time"
)

func TestSessionManager_Basics(t *testing.T) {
	sm := NewSessionManager()
	s := sm.CreateSession("u1", "t1", []string{"r1"})
	if s.UserID != "u1" || s.TenantID != "t1" || len(s.Roles) != 1 {
		t.Fatalf("unexpected session: %+v", s)
	}
	if !sm.IsSessionValid(s) {
		t.Fatalf("session should be valid")
	}
	sm.RefreshSession(s)
}

func TestExtractHelpers(t *testing.T) {
	roles := ExtractRolesFromJWT(map[string]interface{}{"roles": []interface{}{"a", "b"}})
	if len(roles) != 2 {
		t.Fatalf("roles parse failed: %v", roles)
	}
	roles = ExtractRolesFromJWT(map[string]interface{}{"groups": []string{"x"}})
	if len(roles) != 1 || roles[0] != "x" {
		t.Fatalf("groups parse failed: %v", roles)
	}
	if ExtractTenantFromJWT(map[string]interface{}{"tenant_id": "acme"}) != "acme" {
		t.Fatalf("tenant parse failed")
	}
	if ExtractTenantFromJWT(map[string]interface{}{}) != "default" {
		t.Fatalf("default tenant expected")
	}
}

func TestGenerateSessionID_Unique(t *testing.T) {
	a := generateSessionID()
	b := generateSessionID()
	if a == "" || b == "" || a == b {
		t.Fatalf("expected unique ids got %q %q", a, b)
	}
}

func TestIsSessionValid_Expired(t *testing.T) {
	sm := NewSessionManager()
	s := sm.CreateSession("u", "t", nil)
	// Force old LastActivity
	s.LastActivity = time.Now().Add(-25 * time.Hour)
	if sm.IsSessionValid(s) {
		t.Fatalf("expected invalid due to inactivity")
	}
}
