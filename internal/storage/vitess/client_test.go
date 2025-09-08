package vitess

import (
    "strings"
    "testing"
    "github.com/platformbuilds/mirador-core/internal/config"
)

func TestDSNFrom_Config(t *testing.T) {
    cfg := config.VitessConfig{Host: "db.local", Port: 15306, Keyspace: "mirador", User: "u", Password: "p", TLS: true}
    dsn := dsnFrom(cfg)
    if !strings.Contains(dsn, "db.local:15306") || !strings.Contains(dsn, "/mirador?") {
        t.Fatalf("unexpected dsn: %s", dsn)
    }
    if !strings.Contains(dsn, "parseTime=true") { t.Fatalf("missing parseTime param") }
}

