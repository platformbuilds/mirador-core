package cache

import (
	"context"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// autoSwapCache wraps a ValkeyCluster implementation and can swap from a
// fallback (e.g., in-memory noop) to a real Valkey client once it becomes
// available. It satisfies the ValkeyCluster interface by delegating all calls
// to the currently active implementation.
type autoSwapCache struct {
	mu      sync.RWMutex
	current ValkeyCluster
	logger  logger.Logger

	// control for background connector
	stopCh chan struct{}
}

// newAutoSwapCache creates an auto-swapping cache that starts with `fallback`
// and keeps trying `dialReal` until it succeeds, then atomically swaps.
func newAutoSwapCache(
	fallback ValkeyCluster,
	logger logger.Logger,
	dialReal func() (ValkeyCluster, error),
) *autoSwapCache {
	a := &autoSwapCache{
		current: fallback,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				real, err := dialReal()
				if err != nil {
					a.logger.Warn("Valkey connection attempt failed; will retry", "error", err)
					continue
				}
				a.mu.Lock()
				a.current = real
				a.mu.Unlock()
				a.logger.Info("Valkey connection established; switched from in-memory to real cache")
				return // stop after first successful swap
			}
		}
	}()

	return a
}

// Stop stops the background connector (used if the parent context is cancelled).
func (a *autoSwapCache) Stop() { close(a.stopCh) }

/* --- Delegate methods to active implementation --- */

func (a *autoSwapCache) withCurrent(f func(ValkeyCluster) error) error {
	a.mu.RLock()
	c := a.current
	a.mu.RUnlock()
	return f(c)
}

func (a *autoSwapCache) Get(ctx context.Context, key string) ([]byte, error) {
	var out []byte
	var retErr error
	_ = a.withCurrent(func(c ValkeyCluster) error {
		b, e := c.Get(ctx, key)
		out, retErr = b, e
		return nil
	})
	return out, retErr
}

func (a *autoSwapCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return a.withCurrent(func(c ValkeyCluster) error { return c.Set(ctx, key, value, ttl) })
}

func (a *autoSwapCache) Delete(ctx context.Context, key string) error {
	return a.withCurrent(func(c ValkeyCluster) error { return c.Delete(ctx, key) })
}

func (a *autoSwapCache) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
	var out *models.UserSession
	var retErr error
	_ = a.withCurrent(func(c ValkeyCluster) error {
		s, e := c.GetSession(ctx, sessionID)
		out, retErr = s, e
		return nil
	})
	return out, retErr
}

func (a *autoSwapCache) SetSession(ctx context.Context, session *models.UserSession) error {
	return a.withCurrent(func(c ValkeyCluster) error { return c.SetSession(ctx, session) })
}

func (a *autoSwapCache) InvalidateSession(ctx context.Context, sessionID string) error {
	return a.withCurrent(func(c ValkeyCluster) error { return c.InvalidateSession(ctx, sessionID) })
}

func (a *autoSwapCache) GetActiveSessions(ctx context.Context) ([]*models.UserSession, error) {
	var out []*models.UserSession
	var retErr error
	_ = a.withCurrent(func(c ValkeyCluster) error {
		s, e := c.GetActiveSessions(ctx)
		out, retErr = s, e
		return nil
	})
	return out, retErr
}

func (a *autoSwapCache) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
	return a.withCurrent(func(c ValkeyCluster) error { return c.CacheQueryResult(ctx, queryHash, result, ttl) })
}

func (a *autoSwapCache) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
	var out []byte
	var retErr error
	_ = a.withCurrent(func(c ValkeyCluster) error {
		b, e := c.GetCachedQueryResult(ctx, queryHash)
		out, retErr = b, e
		return nil
	})
	return out, retErr
}

func (a *autoSwapCache) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	var acquired bool
	var retErr error
	_ = a.withCurrent(func(c ValkeyCluster) error {
		b, e := c.AcquireLock(ctx, key, ttl)
		acquired, retErr = b, e
		return nil
	})
	return acquired, retErr
}

func (a *autoSwapCache) ReleaseLock(ctx context.Context, key string) error {
	var retErr error
	_ = a.withCurrent(func(c ValkeyCluster) error {
		retErr = c.ReleaseLock(ctx, key)
		return nil
	})
	return retErr
}

// HealthCheck delegates to the current underlying cache if it implements HealthCheck.
func (a *autoSwapCache) HealthCheck(ctx context.Context) error {
	a.mu.RLock()
	c := a.current
	a.mu.RUnlock()
	type cacheHealth interface{ HealthCheck(context.Context) error }
	if hc, ok := c.(cacheHealth); ok {
		return hc.HealthCheck(ctx)
	}
	return nil
}

// GetMemoryInfo delegates to the current underlying cache.
func (a *autoSwapCache) GetMemoryInfo(ctx context.Context) (*CacheMemoryInfo, error) {
	var retErr error
	var retInfo *CacheMemoryInfo
	_ = a.withCurrent(func(c ValkeyCluster) error {
		retInfo, retErr = c.GetMemoryInfo(ctx)
		return nil
	})
	return retInfo, retErr
}

// AdjustCacheTTL delegates to the current underlying cache.
func (a *autoSwapCache) AdjustCacheTTL(ctx context.Context, keyPattern string, newTTL time.Duration) error {
	var retErr error
	_ = a.withCurrent(func(c ValkeyCluster) error {
		retErr = c.AdjustCacheTTL(ctx, keyPattern, newTTL)
		return nil
	})
	return retErr
}

// CleanupExpiredEntries delegates to the current underlying cache.
func (a *autoSwapCache) CleanupExpiredEntries(ctx context.Context, keyPattern string) (int64, error) {
	var retErr error
	var retCount int64
	_ = a.withCurrent(func(c ValkeyCluster) error {
		retCount, retErr = c.CleanupExpiredEntries(ctx, keyPattern)
		return nil
	})
	return retCount, retErr
}

/* --------------------------- pattern-based cache invalidation --------------------------- */

func (a *autoSwapCache) AddToPatternIndex(ctx context.Context, patternKey string, cacheKey string) error {
	return a.withCurrent(func(c ValkeyCluster) error { return c.AddToPatternIndex(ctx, patternKey, cacheKey) })
}

func (a *autoSwapCache) GetPatternIndexKeys(ctx context.Context, patternKey string) ([]string, error) {
	var out []string
	var retErr error
	_ = a.withCurrent(func(c ValkeyCluster) error {
		keys, e := c.GetPatternIndexKeys(ctx, patternKey)
		out, retErr = keys, e
		return nil
	})
	return out, retErr
}

func (a *autoSwapCache) DeletePatternIndex(ctx context.Context, patternKey string) error {
	return a.withCurrent(func(c ValkeyCluster) error { return c.DeletePatternIndex(ctx, patternKey) })
}

func (a *autoSwapCache) DeleteMultiple(ctx context.Context, keys []string) error {
	return a.withCurrent(func(c ValkeyCluster) error { return c.DeleteMultiple(ctx, keys) })
}

// NewAutoSwapForSingle creates an auto-swapping cache that upgrades from
// in-memory to a single-node Valkey client when reachable.
func NewAutoSwapForSingle(addr string, db int, password string, ttl time.Duration, log logger.Logger, fallback ValkeyCluster) ValkeyCluster {
	return newAutoSwapCache(fallback, log, func() (ValkeyCluster, error) {
		return NewValkeySingle(addr, db, password, ttl)
	})
}

// NewAutoSwapForCluster creates an auto-swapping cache that upgrades from
// in-memory to a Valkey cluster client when reachable.
func NewAutoSwapForCluster(nodes []string, ttl time.Duration, log logger.Logger, fallback ValkeyCluster) ValkeyCluster {
	return newAutoSwapCache(fallback, log, func() (ValkeyCluster, error) {
		return NewValkeyCluster(nodes, ttl)
	})
}
