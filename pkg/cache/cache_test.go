package cache

import (
    "context"
    "testing"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestNoopCache_SetGet(t *testing.T) {
    log := logger.New("error")
    c := NewNoopValkeyCache(log)
    if err := c.Set(context.Background(), "k", "v", 0); err != nil { t.Fatalf("set: %v", err) }
    b, err := c.Get(context.Background(), "k")
    if err != nil { t.Fatalf("get: %v", err) }
    if string(b) != "v" { t.Fatalf("unexpected value: %s", string(b)) }
}

