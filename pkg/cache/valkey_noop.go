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
	n.mu.RLock()
	defer n.mu.RUnlock()
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
		if err != nil {
			return err
		}
		b = jb
	}
	n.mu.Lock()
	n.m[key] = b
	n.mu.Unlock()
	return nil
}

func (n *noopValkeyCache) Delete(ctx context.Context, key string) error {
	n.mu.Lock()
	delete(n.m, key)
	n.mu.Unlock()
	return nil
}

func (n *noopValkeyCache) SetSession(ctx context.Context, session *models.UserSession) error {
	session.LastActivity = time.Now()
	if err := n.Set(ctx, "session:"+session.ID, session, 24*time.Hour); err != nil {
		return err
	}
	_ = n.AddToPatternIndex(ctx, fmt.Sprintf("active_sessions:%s"), fmt.Sprintf("session:%s", session.ID))
	return nil
}
func (n *noopValkeyCache) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
	b, err := n.Get(ctx, "session:"+sessionID)
	if err != nil {
		return nil, err
	}
	var s models.UserSession
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
func (n *noopValkeyCache) InvalidateSession(ctx context.Context, sessionID string) error {
	return n.Delete(ctx, "session:"+sessionID)
}
func (n *noopValkeyCache) GetActiveSessions(ctx context.Context) ([]*models.UserSession, error) {
	// Scan all sessions from memory
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := []*models.UserSession{}
	// Look up by pattern index if present
	patternKey := fmt.Sprintf("active_sessions:%s")
	keys, _ := n.GetPatternIndexKeys(ctx, patternKey)
	if len(keys) > 0 {
		for _, k := range keys {
			if v, ok := n.m[k]; ok {
				var s models.UserSession
				if json.Unmarshal(v, &s) == nil {
					out = append(out, &s)
				}
			}
		}
	} else {
		// fallback: scan all sessions in-memory
		for k, v := range n.m {
			if len(k) >= 8 && k[:8] == "session:" {
				var s models.UserSession
				if json.Unmarshal(v, &s) == nil {
					out = append(out, &s)
				}
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

func (n *noopValkeyCache) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	// In noop mode, always acquire the lock (no contention)
	return true, nil
}

func (n *noopValkeyCache) ReleaseLock(ctx context.Context, key string) error {
	// In noop mode, nothing to release
	return nil
}

// HealthCheck returns an error to indicate no external Valkey connectivity.
func (n *noopValkeyCache) HealthCheck(ctx context.Context) error {
	return fmt.Errorf("valkey noop cache in use (external cache not connected)")
}

// GetMemoryInfo returns empty memory info for noop cache
func (n *noopValkeyCache) GetMemoryInfo(ctx context.Context) (*CacheMemoryInfo, error) {
	return &CacheMemoryInfo{
		UsedMemory:          0,
		PeakMemory:          0,
		MemoryFragmentation: 1.0,
		TotalKeys:           int64(len(n.m)),
		ExpiredKeys:         0,
		EvictedKeys:         0,
		HitRate:             0.0,
		MissRate:            0.0,
	}, nil
}

// AdjustCacheTTL is a no-op for noop cache
func (n *noopValkeyCache) AdjustCacheTTL(ctx context.Context, keyPattern string, newTTL time.Duration) error {
	// No-op for in-memory cache
	return nil
}

// CleanupExpiredEntries is a no-op for noop cache
func (n *noopValkeyCache) CleanupExpiredEntries(ctx context.Context, keyPattern string) (int64, error) {
	// No-op for in-memory cache (no TTL support)
	return 0, nil
}

/* --------------------------- pattern-based cache invalidation --------------------------- */

func (n *noopValkeyCache) AddToPatternIndex(ctx context.Context, patternKey string, cacheKey string) error {
	// For noop cache, we simulate sets using a simple key format
	setKey := fmt.Sprintf("set:%s", patternKey)

	// Get existing set members
	existingData, _ := n.Get(ctx, setKey)
	var members []string
	if len(existingData) > 0 {
		if err := json.Unmarshal(existingData, &members); err != nil {
			members = []string{}
		}
	}

	// Add new member if not already present
	for _, member := range members {
		if member == cacheKey {
			return nil // Already in set
		}
	}

	members = append(members, cacheKey)
	data, err := json.Marshal(members)
	if err != nil {
		return err
	}

	return n.Set(ctx, setKey, data, 0)
}

func (n *noopValkeyCache) GetPatternIndexKeys(ctx context.Context, patternKey string) ([]string, error) {
	setKey := fmt.Sprintf("set:%s", patternKey)
	data, err := n.Get(ctx, setKey)
	if err != nil {
		return []string{}, nil // Empty set
	}

	var members []string
	if err := json.Unmarshal(data, &members); err != nil {
		return []string{}, nil
	}

	return members, nil
}

func (n *noopValkeyCache) DeletePatternIndex(ctx context.Context, patternKey string) error {
	setKey := fmt.Sprintf("set:%s", patternKey)
	return n.Delete(ctx, setKey)
}

func (n *noopValkeyCache) DeleteMultiple(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := n.Delete(ctx, key); err != nil {
			// Continue deleting other keys even if one fails
			n.logger.Warn("Failed to delete key in noop cache", "key", key, "error", err)
		}
	}
	return nil
}
