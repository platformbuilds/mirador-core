//go:build db

package cache

import (
    "context"
    "os"
    "strings"
    "testing"
    "time"
)

// Database Test Cases: live Valkey cluster if VALKEY_NODES is set (comma-separated)
func TestValkeyCluster_DB(t *testing.T) {
    nodesEnv := os.Getenv("VALKEY_NODES")
    if strings.TrimSpace(nodesEnv) == "" {
        t.Skip("VALKEY_NODES not set; skipping DB test")
    }
    nodes := strings.Split(nodesEnv, ",")
    cch, err := NewValkeyCluster(nodes, 2*time.Second)
    if err != nil { t.Fatalf("connect cluster: %v", err) }
    ctx := context.Background()
    if err := cch.Set(ctx, "dbk2", "dbv2", time.Second); err != nil { t.Fatalf("set: %v", err) }
    b, err := cch.Get(ctx, "dbk2")
    if err != nil || string(b) != "dbv2" { t.Fatalf("get: %v %q", err, string(b)) }
}

