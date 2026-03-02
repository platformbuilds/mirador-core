// Package cache provides caching abstractions and implementations for
// MIRADOR-CORE services.
//
// # Overview
//
// This package defines the [Cache] interface and implementations for
// various caching backends including Redis/Valkey and in-memory caching.
//
// # Interface
//
// The core [Cache] interface provides standard cache operations:
//
//	type Cache interface {
//	    Get(ctx context.Context, key string) ([]byte, error)
//	    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
//	    Delete(ctx context.Context, key string) error
//	    Exists(ctx context.Context, key string) (bool, error)
//	}
//
// # Usage
//
// Create a cache instance using the factory function:
//
//	cache, err := cache.New(cfg.Cache)
//	if err != nil {
//	    return err
//	}
//	defer cache.Close()
//
//	// Store a value
//	err = cache.Set(ctx, "key", data, 5*time.Minute)
//
//	// Retrieve a value
//	data, err := cache.Get(ctx, "key")
//	if errors.Is(err, cache.ErrNotFound) {
//	    // Handle cache miss
//	}
//
// # Metrics
//
// The cache implementations emit Prometheus metrics for monitoring:
//   - mirador_core_cache_requests_total: Counter by operation and result
//   - mirador_core_cache_request_duration_seconds: Histogram by operation
//
// # Configuration
//
// Cache is configured via the application config file:
//
//	cache:
//	  type: redis          # redis, valkey, or memory
//	  address: localhost:6379
//	  ttl: 300             # default TTL in seconds
//	  maxSize: 1000        # max entries (memory cache only)
//	  password: ""         # optional authentication
//	  db: 0                # database number (redis only)
package cache
