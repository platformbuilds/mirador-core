//go:build db

package cache

import (
    "context"
    "os"
    "testing"
    "time"
)

// Database Test Cases: live Valkey/Redis single-node if VALKEY_ADDR is set.
func TestValkeySingle_DB(t *testing.T) {
    addr := os.Getenv("VALKEY_ADDR")
    if addr == "" {
        t.Skip("VALKEY_ADDR not set; skipping DB test")
    }
    ttl := 2 * time.Second
    cch, err := NewValkeySingle(addr, 0, os.Getenv("VALKEY_PASSWORD"), ttl)
    if err != nil { t.Fatalf("connect: %v", err) }

    ctx := context.Background()
    if err := cch.Set(ctx, "dbk", "dbv", ttl); err != nil { t.Fatalf("set: %v", err) }
    b, err := cch.Get(ctx, "dbk")
    if err != nil || string(b) != "dbv" { t.Fatalf("get: %v %q", err, string(b)) }
}

