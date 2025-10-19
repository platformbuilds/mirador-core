package cache

import (
	"context"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestNoopValkey_BasicOps(t *testing.T) {
	log := logger.New("error")
	cch := NewNoopValkeyCache(log)
	ctx := context.Background()

	if err := cch.Set(ctx, "k1", "v1", time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	b, err := cch.Get(ctx, "k1")
	if err != nil || string(b) != "v1" {
		t.Fatalf("get: %v %q", err, string(b))
	}
	if err := cch.Delete(ctx, "k1"); err != nil {
		t.Fatalf("del: %v", err)
	}

	// session helpers
	s := &models.UserSession{ID: "tok", TenantID: "t1", UserID: "u1"}
	if err := cch.SetSession(ctx, s); err != nil {
		t.Fatalf("set session: %v", err)
	}
	got, err := cch.GetSession(ctx, "tok")
	if err != nil || got.UserID != "u1" {
		t.Fatalf("get session: %v %+v", err, got)
	}
	act, _ := cch.GetActiveSessions(ctx, "t1")
	if len(act) == 0 {
		t.Fatalf("active sessions empty")
	}
	if err := cch.InvalidateSession(ctx, "tok"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	// query cache
	if err := cch.CacheQueryResult(ctx, "h", map[string]int{"a": 1}, time.Second); err != nil {
		t.Fatalf("cache: %v", err)
	}
	if _, err := cch.GetCachedQueryResult(ctx, "h"); err != nil {
		t.Fatalf("get cached: %v", err)
	}

	// if available, health check on noop returns error indicating noop
	if nc, ok := cch.(*noopValkeyCache); ok {
		if err := nc.HealthCheck(ctx); err == nil {
			t.Fatalf("expected health error for noop cache")
		}
	}
}
