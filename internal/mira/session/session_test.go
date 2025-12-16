package session

import (
	"testing"
	"time"
)

func TestStore_SetGetReset(t *testing.T) {
	s := NewStore(2 * time.Second)
	id := "test-session"
	sd := SessionData{Scope: "default"}
	s.Set(id, &sd)
	got, ok := s.Get(id)
	if !ok {
		t.Fatalf("expected session present")
	}
	if got.Scope != "default" {
		t.Fatalf("unexpected scope: %v", got.Scope)
	}
	// Wait to expire
	time.Sleep(3 * time.Second)
	_, ok = s.Get(id)
	if ok {
		t.Fatalf("expected session expired")
	}
}
