package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	valkey "github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeycompat"
)

// CacheMemoryInfo contains memory usage information for adaptive cache sizing
type CacheMemoryInfo struct {
	UsedMemory          int64   `json:"used_memory_bytes"`
	PeakMemory          int64   `json:"peak_memory_bytes"`
	MemoryFragmentation float64 `json:"memory_fragmentation_ratio"`
	TotalKeys           int64   `json:"total_keys"`
	ExpiredKeys         int64   `json:"expired_keys"`
	EvictedKeys         int64   `json:"evicted_keys"`
	HitRate             float64 `json:"hit_rate"`
	MissRate            float64 `json:"miss_rate"`
}

type ValkeyCluster interface {
	// General caching
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error

	// Distributed locks
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error

	// Query result caching for faster fetch
	CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error
	GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error)

	// Pattern-based cache invalidation (index-based for efficiency)
	AddToPatternIndex(ctx context.Context, patternKey string, cacheKey string) error
	GetPatternIndexKeys(ctx context.Context, patternKey string) ([]string, error)
	DeletePatternIndex(ctx context.Context, patternKey string) error
	DeleteMultiple(ctx context.Context, keys []string) error

	// Adaptive cache sizing
	GetMemoryInfo(ctx context.Context) (*CacheMemoryInfo, error)
	AdjustCacheTTL(ctx context.Context, keyPattern string, newTTL time.Duration) error
	CleanupExpiredEntries(ctx context.Context, keyPattern string) (int64, error)
}

type valkeyClusterImpl struct {
	client valkeycompat.Cmdable
	logger logger.Logger
	ttl    time.Duration
}

func NewValkeyCluster(nodes []string, defaultTTL time.Duration) (ValkeyCluster, error) {
	// Create underlying valkey client and wrap with compatibility adapter.
	cli, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: nodes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey cluster client: %w", err)
	}

	adapter := valkeycompat.NewAdapter(cli)

	// Test connection to Valkey cluster
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := adapter.Ping(ctx).Err(); err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to connect to Valkey cluster: %w", err)
	}

	return &valkeyClusterImpl{
		client: adapter,
		logger: logger.New("info"),
		ttl:    defaultTTL,
	}, nil
}

// HealthCheck pings the Valkey cluster.
func (v *valkeyClusterImpl) HealthCheck(ctx context.Context) error {
	if ctx == nil {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx = c
	}
	return v.client.Ping(ctx).Err()
}

/* ---------------------------- generic cache ---------------------------- */

func (v *valkeyClusterImpl) Get(ctx context.Context, key string) ([]byte, error) {
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

func (v *valkeyClusterImpl) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
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

func (v *valkeyClusterImpl) Delete(ctx context.Context, key string) error {
	err := v.client.Del(ctx, key).Err()
	if err != nil {
		monitoring.RecordCacheOperation("delete", "error")
		return err
	}
	monitoring.RecordCacheOperation("delete", "success")
	return nil
}

/* --------------------------- query result cache --------------------------- */

func (v *valkeyClusterImpl) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("query_cache:%s", queryHash)
	return v.Set(ctx, key, result, ttl)
}

func (v *valkeyClusterImpl) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
	key := fmt.Sprintf("query_cache:%s", queryHash)
	return v.Get(ctx, key)
}

/* --------------------------- pattern-based cache invalidation --------------------------- */

func (v *valkeyClusterImpl) AddToPatternIndex(ctx context.Context, patternKey string, cacheKey string) error {
	err := v.client.SAdd(ctx, patternKey, cacheKey).Err()
	if err != nil {
		monitoring.RecordCacheOperation("add_pattern_index", "error")
		return fmt.Errorf("failed to add to pattern index %s: %w", patternKey, err)
	}
	monitoring.RecordCacheOperation("add_pattern_index", "success")
	return nil
}

func (v *valkeyClusterImpl) GetPatternIndexKeys(ctx context.Context, patternKey string) ([]string, error) {
	keys, err := v.client.SMembers(ctx, patternKey).Result()
	if err != nil {
		monitoring.RecordCacheOperation("get_pattern_index", "error")
		return nil, fmt.Errorf("failed to get pattern index keys %s: %w", patternKey, err)
	}
	monitoring.RecordCacheOperation("get_pattern_index", "success")
	return keys, nil
}

func (v *valkeyClusterImpl) DeletePatternIndex(ctx context.Context, patternKey string) error {
	err := v.client.Del(ctx, patternKey).Err()
	if err != nil {
		monitoring.RecordCacheOperation("delete_pattern_index", "error")
		return fmt.Errorf("failed to delete pattern index %s: %w", patternKey, err)
	}
	monitoring.RecordCacheOperation("delete_pattern_index", "success")
	return nil
}

func (v *valkeyClusterImpl) DeleteMultiple(ctx context.Context, keys []string) error {
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

/* --------------------------- distributed locks --------------------------- */

func (v *valkeyClusterImpl) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
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

func (v *valkeyClusterImpl) ReleaseLock(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("lock:%s", key)

	err := v.client.Del(ctx, lockKey).Err()
	if err != nil {
		monitoring.RecordCacheOperation("release_lock", "error")
		return err
	}

	monitoring.RecordCacheOperation("release_lock", "success")
	return nil
}

/* --------------------------- adaptive cache sizing --------------------------- */

func (v *valkeyClusterImpl) GetMemoryInfo(ctx context.Context) (*CacheMemoryInfo, error) {
	info, err := v.client.Info(ctx, "memory", "stats", "keyspace").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	// Parse Redis INFO output
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

	// Calculate hit/miss rates (simplified)
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

func (v *valkeyClusterImpl) AdjustCacheTTL(ctx context.Context, keyPattern string, newTTL time.Duration) error {
	// For Redis/Valkey, we need to find keys matching the pattern and update their TTL
	// This is a simplified implementation - in production, you'd use SCAN for large datasets

	// Get all keys matching the pattern (limited implementation)
	keys, err := v.client.Keys(ctx, keyPattern).Result()
	if err != nil {
		return fmt.Errorf("failed to find keys for pattern %s: %w", keyPattern, err)
	}

	// Update TTL for each key
	for _, key := range keys {
		if err := v.client.Expire(ctx, key, newTTL).Err(); err != nil {
			v.logger.Warn("Failed to update TTL for key", "key", key, "error", err)
		}
	}

	v.logger.Info("Adjusted cache TTL", "pattern", keyPattern, "new_ttl", newTTL, "keys_updated", len(keys))
	return nil
}

func (v *valkeyClusterImpl) CleanupExpiredEntries(ctx context.Context, keyPattern string) (int64, error) {
	// For Redis/Valkey, expired entries are automatically cleaned up
	// This method provides manual cleanup for specific patterns if needed

	keys, err := v.client.Keys(ctx, keyPattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to find keys for pattern %s: %w", keyPattern, err)
	}

	var deletedCount int64
	for _, key := range keys {
		// Check if key is expired (TTL <= 0)
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

	v.logger.Info("Cleaned up expired entries", "pattern", keyPattern, "deleted", deletedCount)
	return deletedCount, nil
}

// Helper functions for parsing Redis INFO output
func extractInfoValue(info, key string) string {
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
