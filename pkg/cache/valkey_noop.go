package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// noopValkeyCache provides an in-memory, process-local fallback that satisfies
// ValkeyCluster when the external cache is unavailable. It is best-effort and
// intended for development and degraded operation; data is not shared across
// replicas and is lost on restart.
type noopValkeyCache struct {
    m      map[string][]byte
    mu     sync.RWMutex
    logger logger.Logger
}

func NewNoopValkeyCache(log logger.Logger) ValkeyCluster {
    log.Warn("Valkey cache unavailable; using in-memory fallback (noop)")
    return &noopValkeyCache{m: make(map[string][]byte), logger: log}
}

func (n *noopValkeyCache) Get(ctx context.Context, key string) ([]byte, error) {
    n.mu.RLock(); defer n.mu.RUnlock()
    b, ok := n.m[key]
    if !ok {
        return nil, fmt.Errorf("key not found: %s", key)
    }
    return b, nil
}

func (n *noopValkeyCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    var b []byte
    switch v := value.(type) {
    case []byte:
        b = v
    case string:
        b = []byte(v)
    default:
        jb, err := json.Marshal(v)
        if err != nil { return err }
        b = jb
    }
    n.mu.Lock(); n.m[key] = b; n.mu.Unlock()
    return nil
}

func (n *noopValkeyCache) Delete(ctx context.Context, key string) error {
    n.mu.Lock(); delete(n.m, key); n.mu.Unlock(); return nil
}

func (n *noopValkeyCache) SetSession(ctx context.Context, session *models.UserSession) error {
    session.LastActivity = time.Now()
    return n.Set(ctx, "session:"+session.ID, session, 24*time.Hour)
}
func (n *noopValkeyCache) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
    b, err := n.Get(ctx, "session:"+sessionID)
    if err != nil { return nil, err }
    var s models.UserSession
    if err := json.Unmarshal(b, &s); err != nil { return nil, err }
    return &s, nil
}
func (n *noopValkeyCache) InvalidateSession(ctx context.Context, sessionID string) error {
    return n.Delete(ctx, "session:"+sessionID)
}
func (n *noopValkeyCache) GetActiveSessions(ctx context.Context, tenantID string) ([]*models.UserSession, error) {
    // Not tracking per-tenant sets in noop; scan map as a best-effort
    n.mu.RLock(); defer n.mu.RUnlock()
    out := []*models.UserSession{}
    for k, v := range n.m {
        if len(k) >= 8 && k[:8] == "session:" {
            var s models.UserSession
            if json.Unmarshal(v, &s) == nil && s.TenantID == tenantID {
                out = append(out, &s)
            }
        }
    }
    return out, nil
}

func (n *noopValkeyCache) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
    return n.Set(ctx, "query_cache:"+queryHash, result, ttl)
}
func (n *noopValkeyCache) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
    return n.Get(ctx, "query_cache:"+queryHash)
}

