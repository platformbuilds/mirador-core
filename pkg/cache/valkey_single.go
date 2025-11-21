package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	valkey "github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeycompat"
)

// valkeySingleImpl implements ValkeyCluster against a single-node Valkey instance.
type valkeySingleImpl struct {
	client valkeycompat.Cmdable
	logger logger.Logger
	ttl    time.Duration
}

func NewValkeySingle(addr string, db int, password string, defaultTTL time.Duration) (ValkeyCluster, error) {
	// Create underlying valkey client and wrap with compatibility adapter so existing
	// go-redis style calls continue to work via valkeycompat.Cmdable.
	cli, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{addr},
		Password:    password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	adapter := valkeycompat.NewAdapter(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := adapter.Ping(ctx).Err(); err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to connect to Valkey single-node: %w", err)
	}

	return &valkeySingleImpl{
		client: adapter,
		logger: logger.New("info"),
		ttl:    defaultTTL,
	}, nil
}

func (v *valkeySingleImpl) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := v.client.Get(ctx, key).Bytes()
	if err == valkeycompat.Nil {
		monitoring.RecordCacheOperation("get", "miss")
		return nil, fmt.Errorf("key not found: %s", key)
	}

	if err != nil {
		monitoring.RecordCacheOperation("get", "error")
		return nil, err
	}

	monitoring.RecordCacheOperation("get", "hit")
	return b, nil
}

func (v *valkeySingleImpl) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var data []byte
	switch x := value.(type) {
	case []byte:
		data = x
	case string:
		data = []byte(x)
	default:
		j, err := json.Marshal(x)
		if err != nil {
			monitoring.RecordCacheOperation("set", "error")
			return fmt.Errorf("marshal value for key %s: %w", key, err)
		}
		data = j
	}
	if ttl <= 0 {
		ttl = v.ttl
	}
	err := v.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		monitoring.RecordCacheOperation("set", "error")
		return err
	}
	monitoring.RecordCacheOperation("set", "success")
	return nil
}

func (v *valkeySingleImpl) Delete(ctx context.Context, key string) error {
	err := v.client.Del(ctx, key).Err()
	if err != nil {
		monitoring.RecordCacheOperation("delete", "error")
		return err
	}
	monitoring.RecordCacheOperation("delete", "success")
	return nil
}

func (v *valkeySingleImpl) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("query_cache:%s", queryHash)
	return v.Set(ctx, key, result, ttl)
}

func (v *valkeySingleImpl) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
	key := fmt.Sprintf("query_cache:%s", queryHash)
	return v.Get(ctx, key)
}

/* --------------------------- distributed locks --------------------------- */

func (v *valkeySingleImpl) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("lock:%s", key)

	// Use SET with NX (not exists) and PX (milliseconds TTL) for atomic locking
	set, err := v.client.SetNX(ctx, lockKey, "locked", ttl).Result()
	if err != nil {
		monitoring.RecordCacheOperation("acquire_lock", "error")
		return false, err
	}

	if set {
		monitoring.RecordCacheOperation("acquire_lock", "success")
	} else {
		monitoring.RecordCacheOperation("acquire_lock", "conflict")
	}

	return set, nil
}

func (v *valkeySingleImpl) ReleaseLock(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("lock:%s", key)

	err := v.client.Del(ctx, lockKey).Err()
	if err != nil {
		monitoring.RecordCacheOperation("release_lock", "error")
		return err
	}

	monitoring.RecordCacheOperation("release_lock", "success")
	return nil
}

// HealthCheck pings the Valkey single-node instance.
func (v *valkeySingleImpl) HealthCheck(ctx context.Context) error {
	if ctx == nil {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx = c
	}
	return v.client.Ping(ctx).Err()
}

// GetMemoryInfo retrieves memory information from Valkey
func (v *valkeySingleImpl) GetMemoryInfo(ctx context.Context) (*CacheMemoryInfo, error) {
	info, err := v.client.Info(ctx, "memory", "stats", "keyspace").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	memInfo := &CacheMemoryInfo{}

	// Parse memory section
	if usedMemory := extractInfoValue(info, "used_memory"); usedMemory != "" {
		if val, err := parseInt64(usedMemory); err == nil {
			memInfo.UsedMemory = val
		}
	}

	if peakMemory := extractInfoValue(info, "used_memory_peak"); peakMemory != "" {
		if val, err := parseInt64(peakMemory); err == nil {
			memInfo.PeakMemory = val
		}
	}

	if fragmentation := extractInfoValue(info, "mem_fragmentation_ratio"); fragmentation != "" {
		if val, err := parseFloat64(fragmentation); err == nil {
			memInfo.MemoryFragmentation = val
		}
	}

	// Parse stats section
	if totalKeys := extractInfoValue(info, "total_keys"); totalKeys != "" {
		if val, err := parseInt64(totalKeys); err == nil {
			memInfo.TotalKeys = val
		}
	}

	if expiredKeys := extractInfoValue(info, "expired_keys"); expiredKeys != "" {
		if val, err := parseInt64(expiredKeys); err == nil {
			memInfo.ExpiredKeys = val
		}
	}

	if evictedKeys := extractInfoValue(info, "evicted_keys"); evictedKeys != "" {
		if val, err := parseInt64(evictedKeys); err == nil {
			memInfo.EvictedKeys = val
		}
	}

	// Calculate hit/miss rates
	if keyspaceHits := extractInfoValue(info, "keyspace_hits"); keyspaceHits != "" {
		if hits, err := parseInt64(keyspaceHits); err == nil {
			if keyspaceMisses := extractInfoValue(info, "keyspace_misses"); keyspaceMisses != "" {
				if misses, err := parseInt64(keyspaceMisses); err == nil {
					total := hits + misses
					if total > 0 {
						memInfo.HitRate = float64(hits) / float64(total)
						memInfo.MissRate = float64(misses) / float64(total)
					}
				}
			}
		}
	}

	return memInfo, nil
}

// AdjustCacheTTL adjusts TTL for keys matching a pattern
func (v *valkeySingleImpl) AdjustCacheTTL(ctx context.Context, keyPattern string, newTTL time.Duration) error {
	keys, err := v.client.Keys(ctx, keyPattern).Result()
	if err != nil {
		return fmt.Errorf("failed to find keys for pattern %s: %w", keyPattern, err)
	}

	for _, key := range keys {
		if err := v.client.Expire(ctx, key, newTTL).Err(); err != nil {
			v.logger.Warn("Failed to update TTL for key", "key", key, "error", err)
		}
	}

	return nil
}

// CleanupExpiredEntries cleans up expired entries for a pattern
func (v *valkeySingleImpl) CleanupExpiredEntries(ctx context.Context, keyPattern string) (int64, error) {
	keys, err := v.client.Keys(ctx, keyPattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to find keys for pattern %s: %w", keyPattern, err)
	}

	var deletedCount int64
	for _, key := range keys {
		ttl, err := v.client.TTL(ctx, key).Result()
		if err != nil {
			continue
		}

		if ttl <= 0 {
			if err := v.client.Del(ctx, key).Err(); err == nil {
				deletedCount++
			}
		}
	}

	return deletedCount, nil
}

/* --------------------------- pattern-based cache invalidation --------------------------- */

func (v *valkeySingleImpl) AddToPatternIndex(ctx context.Context, patternKey string, cacheKey string) error {
	err := v.client.SAdd(ctx, patternKey, cacheKey).Err()
	if err != nil {
		monitoring.RecordCacheOperation("add_pattern_index", "error")
		return fmt.Errorf("failed to add to pattern index %s: %w", patternKey, err)
	}
	monitoring.RecordCacheOperation("add_pattern_index", "success")
	return nil
}

func (v *valkeySingleImpl) GetPatternIndexKeys(ctx context.Context, patternKey string) ([]string, error) {
	keys, err := v.client.SMembers(ctx, patternKey).Result()
	if err != nil {
		monitoring.RecordCacheOperation("get_pattern_index", "error")
		return nil, fmt.Errorf("failed to get pattern index keys %s: %w", patternKey, err)
	}
	monitoring.RecordCacheOperation("get_pattern_index", "success")
	return keys, nil
}

func (v *valkeySingleImpl) DeletePatternIndex(ctx context.Context, patternKey string) error {
	err := v.client.Del(ctx, patternKey).Err()
	if err != nil {
		monitoring.RecordCacheOperation("delete_pattern_index", "error")
		return fmt.Errorf("failed to delete pattern index %s: %w", patternKey, err)
	}
	monitoring.RecordCacheOperation("delete_pattern_index", "success")
	return nil
}

func (v *valkeySingleImpl) DeleteMultiple(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	// Use pipeline for efficient batch deletion
	pipe := v.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		monitoring.RecordCacheOperation("delete_multiple", "error")
		return fmt.Errorf("failed to delete multiple keys: %w", err)
	}

	monitoring.RecordCacheOperation("delete_multiple", "success")
	return nil
}
