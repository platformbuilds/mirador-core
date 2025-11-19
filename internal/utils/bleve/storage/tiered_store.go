package storage

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	store "github.com/blevesearch/upsidedown_store_api"

	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TieredStore implements a two-tiered storage strategy:
// - Tier 1: In-memory cache for frequently accessed data
// - Tier 2: Disk-based persistent storage for all data
type TieredStore struct {
	memoryStore *AdaptiveMemoryStore
	diskStore   store.KVStore
	logger      logger.Logger

	// Configuration
	maxMemoryItems int
	ttl            time.Duration

	// Statistics
	memoryHits   int64
	memoryMisses int64
	diskHits     int64
	diskMisses   int64

	mu sync.RWMutex
}

// AdaptiveMemoryStore is an in-memory key-value store with TTL and LRU eviction
type AdaptiveMemoryStore struct {
	data   map[string][]byte
	expiry map[string]time.Time
	access map[string]time.Time // Track access times for LRU
	mu     sync.RWMutex

	// Adaptive sizing
	currentSize    int64
	maxSize        int64
	targetHitRate  float64
	hitRateSamples []float64
	sampleIndex    int
}

// NewAdaptiveMemoryStore creates a new adaptive memory store
func NewAdaptiveMemoryStore(initialMaxSize int64) *AdaptiveMemoryStore {
	return &AdaptiveMemoryStore{
		data:           make(map[string][]byte),
		expiry:         make(map[string]time.Time),
		access:         make(map[string]time.Time),
		maxSize:        initialMaxSize,
		targetHitRate:  0.8,                 // Target 80% hit rate
		hitRateSamples: make([]float64, 10), // Keep 10 samples
	}
}

func (m *AdaptiveMemoryStore) Get(key []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := string(key)
	now := time.Now()

	// Check expiry
	if expiry, exists := m.expiry[k]; exists && now.After(expiry) {
		delete(m.data, k)
		delete(m.expiry, k)
		delete(m.access, k)
		atomic.AddInt64(&m.currentSize, -1)
		return nil, fmt.Errorf("key not found")
	}

	if value, exists := m.data[k]; exists {
		// Update access time for LRU
		m.access[k] = now
		return value, nil
	}

	return nil, fmt.Errorf("key not found")
}

func (m *AdaptiveMemoryStore) Set(key []byte, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := string(key)
	now := time.Now()

	// Check if we need to evict before adding
	if _, exists := m.data[k]; !exists {
		// New item, check if we exceed max size
		for atomic.LoadInt64(&m.currentSize) >= m.maxSize {
			if !m.evictLRU() {
				break // Couldn't evict, allow growth
			}
		}
		atomic.AddInt64(&m.currentSize, 1)
	}

	m.data[k] = value
	m.access[k] = now
	if ttl > 0 {
		m.expiry[k] = now.Add(ttl)
	} else {
		delete(m.expiry, k)
	}
	return nil
}

func (m *AdaptiveMemoryStore) Delete(key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := string(key)
	if _, exists := m.data[k]; exists {
		delete(m.data, k)
		delete(m.expiry, k)
		delete(m.access, k)
		atomic.AddInt64(&m.currentSize, -1)
	}
	return nil
}

func (m *AdaptiveMemoryStore) Has(key []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := string(key)
	if expiry, exists := m.expiry[k]; exists && time.Now().After(expiry) {
		return false
	}
	_, exists := m.data[k]
	return exists
}

func (m *AdaptiveMemoryStore) Size() int64 {
	return atomic.LoadInt64(&m.currentSize)
}

// evictLRU removes the least recently used item
func (m *AdaptiveMemoryStore) evictLRU() bool {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for k, accessTime := range m.access {
		if first || accessTime.Before(oldestTime) {
			oldestKey = k
			oldestTime = accessTime
			first = false
		}
	}

	if oldestKey != "" {
		delete(m.data, oldestKey)
		delete(m.expiry, oldestKey)
		delete(m.access, oldestKey)
		atomic.AddInt64(&m.currentSize, -1)
		return true
	}

	return false
}

// updateHitRateSample adds a new hit rate sample for adaptive sizing
func (m *AdaptiveMemoryStore) updateHitRateSample(hitRate float64) {
	m.hitRateSamples[m.sampleIndex] = hitRate
	m.sampleIndex = (m.sampleIndex + 1) % len(m.hitRateSamples)
}

// getAverageHitRate calculates the average hit rate from recent samples
func (m *AdaptiveMemoryStore) getAverageHitRate() float64 {
	var sum float64
	var count int
	for _, rate := range m.hitRateSamples {
		if rate > 0 { // Only count non-zero samples
			sum += rate
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// adjustMaxSize dynamically adjusts the maximum cache size based on hit rate
func (m *AdaptiveMemoryStore) adjustMaxSize(currentHitRate float64) {
	m.updateHitRateSample(currentHitRate)
	avgHitRate := m.getAverageHitRate()

	var adjustment float64
	if avgHitRate < m.targetHitRate-0.1 {
		// Hit rate too low, increase cache size
		adjustment = 1.2 // Increase by 20%
	} else if avgHitRate > m.targetHitRate+0.1 {
		// Hit rate good, can reduce cache size
		adjustment = 0.9 // Decrease by 10%
	} else {
		// Hit rate in acceptable range, minor adjustment
		adjustment = 1.05 // Slight increase
	}

	newMaxSize := int64(float64(m.maxSize) * adjustment)

	// Set bounds
	minSize := int64(100)
	maxSize := int64(100000) // Max 100K items

	if newMaxSize < minSize {
		newMaxSize = minSize
	} else if newMaxSize > maxSize {
		newMaxSize = maxSize
	}

	atomic.StoreInt64(&m.maxSize, newMaxSize)
}

// NewTieredStore creates a new tiered storage instance
func NewTieredStore(diskStore store.KVStore, maxMemoryItems int, ttl time.Duration, logger logger.Logger) *TieredStore {
	ts := &TieredStore{
		memoryStore:    NewAdaptiveMemoryStore(int64(maxMemoryItems)),
		diskStore:      diskStore,
		logger:         logger,
		maxMemoryItems: maxMemoryItems,
		ttl:            ttl,
	}

	// Start adaptive cache sizing goroutine
	go ts.adaptiveCacheManager()

	return ts
}

// adaptiveCacheManager periodically adjusts cache sizes based on memory pressure and hit rates
func (t *TieredStore) adaptiveCacheManager() {
	ticker := time.NewTicker(2 * time.Minute) // Check every 2 minutes for more responsive adaptation
	defer ticker.Stop()

	for range ticker.C {
		t.optimizeCacheSize()
	}
}

// optimizeCacheSize dynamically adjusts cache sizes based on hit rates and memory pressure
func (t *TieredStore) optimizeCacheSize() {
	// Calculate current hit rate
	totalMemoryRequests := atomic.LoadInt64(&t.memoryHits) + atomic.LoadInt64(&t.memoryMisses)
	var currentHitRate float64
	if totalMemoryRequests > 0 {
		currentHitRate = float64(atomic.LoadInt64(&t.memoryHits)) / float64(totalMemoryRequests)
	}

	// Get current memory pressure
	availableMemory := t.getAvailableMemory()
	memoryPressure := t.calculateMemoryPressure()

	// Adjust cache size based on hit rate and memory pressure
	t.memoryStore.adjustMaxSize(currentHitRate)

	// Apply memory pressure constraints
	currentMaxSize := atomic.LoadInt64(&t.memoryStore.maxSize)
	var newMaxSize int64

	switch {
	case memoryPressure > 0.9: // Critical memory pressure
		// Reduce cache to 20% of available memory
		newMaxSize = int64(float64(availableMemory) * 0.2 / float64(t.estimateItemSize()))
		if newMaxSize < currentMaxSize {
			atomic.StoreInt64(&t.memoryStore.maxSize, newMaxSize)
		}

	case memoryPressure > 0.7: // High memory pressure
		// Reduce cache to 40% of available memory
		newMaxSize = int64(float64(availableMemory) * 0.4 / float64(t.estimateItemSize()))
		if newMaxSize < currentMaxSize {
			atomic.StoreInt64(&t.memoryStore.maxSize, newMaxSize)
		}

	case memoryPressure > 0.5: // Moderate memory pressure
		// Maintain current adaptive size but cap at 60% of available memory
		newMaxSize = int64(float64(availableMemory) * 0.6 / float64(t.estimateItemSize()))
		if currentMaxSize > newMaxSize {
			atomic.StoreInt64(&t.memoryStore.maxSize, newMaxSize)
		}

	default: // Low memory pressure
		// Allow adaptive sizing up to 80% of available memory
		newMaxSize = int64(float64(availableMemory) * 0.8 / float64(t.estimateItemSize()))
		if currentMaxSize > newMaxSize {
			atomic.StoreInt64(&t.memoryStore.maxSize, newMaxSize)
		}
	}

	// Ensure minimum cache size
	minSize := int64(100)
	if atomic.LoadInt64(&t.memoryStore.maxSize) < minSize {
		atomic.StoreInt64(&t.memoryStore.maxSize, minSize)
	}

	t.logger.Info("Adaptive cache optimization completed",
		"currentHitRate", currentHitRate,
		"memoryPressure", memoryPressure,
		"newMaxSize", atomic.LoadInt64(&t.memoryStore.maxSize),
		"currentSize", t.memoryStore.Size())
}

// getAvailableMemory estimates available memory using Go runtime stats
func (t *TieredStore) getAvailableMemory() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Use a conservative estimate: available memory = total system memory - current heap usage
	// In a real implementation, this would query OS-specific memory APIs
	// For now, assume the process can use up to 512MB for caching
	const maxCacheMemory = 512 * 1024 * 1024 // 512MB

	// Estimate based on current heap usage - leave some headroom
	availableForCache := maxCacheMemory - int64(m.HeapAlloc)

	if availableForCache < 50*1024*1024 { // Minimum 50MB
		availableForCache = 50 * 1024 * 1024
	}

	return availableForCache
}

// calculateMemoryPressure returns a value between 0-1 indicating memory pressure
func (t *TieredStore) calculateMemoryPressure() float64 {
	currentItems := float64(t.memoryStore.Size())
	maxItems := float64(atomic.LoadInt64(&t.memoryStore.maxSize))

	if maxItems == 0 {
		return 0
	}

	pressure := currentItems / maxItems

	// Cap at 1.0
	if pressure > 1.0 {
		pressure = 1.0
	}

	return pressure
}

// estimateItemSize provides a rough estimate of memory per cached item
func (t *TieredStore) estimateItemSize() int {
	// Average size based on typical key-value pairs
	// Key: ~50 bytes, Value: ~1KB, Overhead: ~100 bytes
	return 1200 // bytes per item
}

// NewTieredStoreWithDisk creates a new tiered storage instance with disk backend
func NewTieredStoreWithDisk(diskStore store.KVStore, maxMemoryItems int, ttl time.Duration, logger logger.Logger) *TieredStore {
	return &TieredStore{
		memoryStore:    NewAdaptiveMemoryStore(int64(maxMemoryItems)),
		diskStore:      diskStore,
		logger:         logger,
		maxMemoryItems: maxMemoryItems,
		ttl:            ttl,
	}
}

// Close closes the tiered store and underlying resources
func (t *TieredStore) Close() error {
	// Close disk store if it supports closing
	if closer, ok := t.diskStore.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// RecordStorageMetrics records storage usage metrics for monitoring
func (t *TieredStore) RecordStorageMetrics(shardNum string) {
	memoryUsage := t.memoryStore.Size() * int64(t.estimateItemSize())
	// For disk usage, we'd need to implement disk size tracking
	// For now, use a placeholder
	diskUsage := int64(0) // TODO: Implement actual disk usage tracking

	monitoring.RecordBleveStorageUsage(shardNum, memoryUsage, diskUsage)
}

// Stats returns statistics about the tiered store performance
func (t *TieredStore) Stats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	totalRequests := t.memoryHits + t.memoryMisses + t.diskHits + t.diskMisses
	memoryHitRate := float64(0)
	if totalRequests > 0 {
		memoryHitRate = float64(t.memoryHits) / float64(t.memoryHits+t.memoryMisses)
	}

	return map[string]interface{}{
		"memory_hits":     t.memoryHits,
		"memory_misses":   t.memoryMisses,
		"disk_hits":       t.diskHits,
		"disk_misses":     t.diskMisses,
		"memory_hit_rate": memoryHitRate,
		"total_requests":  totalRequests,
	}
}

// Reader returns a KVReader for the tiered store
func (t *TieredStore) Reader() (store.KVReader, error) {
	diskReader, err := t.diskStore.Reader()
	if err != nil {
		return nil, err
	}

	return &TieredReader{
		memoryReader: &MemoryReader{store: t.memoryStore},
		diskReader:   diskReader,
		store:        t,
	}, nil
}

// Writer returns a KVWriter for the tiered store
func (t *TieredStore) Writer() (store.KVWriter, error) {
	diskWriter, err := t.diskStore.Writer()
	if err != nil {
		return nil, err
	}

	return &TieredWriter{
		memoryWriter: &MemoryWriter{store: t},
		diskWriter:   diskWriter,
		store:        t,
	}, nil
}

// TieredReader implements KVReader for tiered storage
type TieredReader struct {
	memoryReader *MemoryReader
	diskReader   store.KVReader
	store        *TieredStore
}

func (t *TieredReader) Get(key []byte) ([]byte, error) {
	// First try memory
	if value, err := t.memoryReader.Get(key); err == nil {
		t.store.mu.Lock()
		t.store.memoryHits++
		t.store.mu.Unlock()
		return value, nil
	}

	t.store.mu.Lock()
	t.store.diskMisses++
	t.store.mu.Unlock()

	// Fallback to disk
	return t.diskReader.Get(key)
}

func (t *TieredReader) MultiGet(keys [][]byte) ([][]byte, error) {
	// For simplicity, delegate to disk reader
	// In a more sophisticated implementation, this could check memory cache first
	return t.diskReader.MultiGet(keys)
}

func (t *TieredReader) PrefixIterator(prefix []byte) store.KVIterator {
	// Delegate to disk reader
	return t.diskReader.PrefixIterator(prefix)
}

func (t *TieredReader) RangeIterator(start, end []byte) store.KVIterator {
	// Delegate to disk reader
	return t.diskReader.RangeIterator(start, end)
}

func (t *TieredReader) Close() error {
	return t.diskReader.Close()
}

// MemoryReader implements KVReader for in-memory storage
type MemoryReader struct {
	store *AdaptiveMemoryStore
}

func (m *MemoryReader) Get(key []byte) ([]byte, error) {
	return m.store.Get(key)
}

func (m *MemoryReader) Close() error {
	return nil
}

// TieredBatch implements KVBatch for tiered storage operations
type TieredBatch struct {
	memoryBatch store.KVBatch
	diskBatch   store.KVBatch
}

func (b *TieredBatch) Set(key, val []byte) {
	b.memoryBatch.Set(key, val)
	b.diskBatch.Set(key, val)
}

func (b *TieredBatch) Delete(key []byte) {
	b.memoryBatch.Delete(key)
	b.diskBatch.Delete(key)
}

func (b *TieredBatch) Merge(key []byte, val []byte) {
	b.memoryBatch.Merge(key, val)
	b.diskBatch.Merge(key, val)
}

func (b *TieredBatch) Reset() {
	b.memoryBatch.Reset()
	b.diskBatch.Reset()
}

func (b *TieredBatch) Close() error {
	if err := b.memoryBatch.Close(); err != nil {
		return err
	}
	return b.diskBatch.Close()
}

// MemoryBatch implements KVBatch for in-memory operations
type MemoryBatch struct {
	operations []batchOperation
}

type batchOperation struct {
	opType string // "set", "delete", "merge"
	key    []byte
	value  []byte
}

func (b *MemoryBatch) Set(key, val []byte) {
	b.operations = append(b.operations, batchOperation{
		opType: "set",
		key:    key,
		value:  val,
	})
}

func (b *MemoryBatch) Delete(key []byte) {
	b.operations = append(b.operations, batchOperation{
		opType: "delete",
		key:    key,
	})
}

func (b *MemoryBatch) Merge(key []byte, val []byte) {
	b.operations = append(b.operations, batchOperation{
		opType: "merge",
		key:    key,
		value:  val,
	})
}

func (b *MemoryBatch) Reset() {
	b.operations = nil
}

func (b *MemoryBatch) Close() error {
	b.operations = nil
	return nil
}

// TieredWriter implements KVWriter for tiered storage
type TieredWriter struct {
	memoryWriter *MemoryWriter
	diskWriter   store.KVWriter
	store        *TieredStore
}

func (t *TieredWriter) NewBatch() store.KVBatch {
	return &TieredBatch{
		memoryBatch: t.memoryWriter.NewBatch(),
		diskBatch:   t.diskWriter.NewBatch(),
	}
}

func (t *TieredWriter) NewBatchEx(opts store.KVBatchOptions) ([]byte, store.KVBatch, error) {
	buf, diskBatch, err := t.diskWriter.NewBatchEx(opts)
	if err != nil {
		return nil, nil, err
	}

	return buf, &TieredBatch{
		memoryBatch: t.memoryWriter.NewBatch(),
		diskBatch:   diskBatch,
	}, nil
}

func (t *TieredWriter) ExecuteBatch(batch store.KVBatch) error {
	tieredBatch := batch.(*TieredBatch)

	// Execute memory batch
	if err := t.memoryWriter.ExecuteBatch(tieredBatch.memoryBatch); err != nil {
		return err
	}

	// Execute disk batch
	return t.diskWriter.ExecuteBatch(tieredBatch.diskBatch)
}

func (t *TieredWriter) Close() error {
	return t.diskWriter.Close()
}

// MemoryWriter implements KVWriter for in-memory storage
type MemoryWriter struct {
	store *TieredStore
}

func (m *MemoryWriter) NewBatch() store.KVBatch {
	return &MemoryBatch{}
}

func (m *MemoryWriter) NewBatchEx(opts store.KVBatchOptions) ([]byte, store.KVBatch, error) {
	return nil, &MemoryBatch{}, nil
}

func (m *MemoryWriter) ExecuteBatch(batch store.KVBatch) error {
	memBatch := batch.(*MemoryBatch)

	for _, op := range memBatch.operations {
		switch op.opType {
		case "set":
			m.store.memoryStore.Set(op.key, op.value, m.store.ttl)
		case "delete":
			m.store.memoryStore.Delete(op.key)
		case "merge":
			// For simplicity, treat merge as set
			m.store.memoryStore.Set(op.key, op.value, m.store.ttl)
		}
	}

	return nil
}

func (m *MemoryWriter) Set(key, value []byte, ttl time.Duration) error {
	return m.store.memoryStore.Set(key, value, ttl)
}

func (m *MemoryWriter) Delete(key []byte) error {
	m.store.memoryStore.Delete(key)
	return nil
}

func (m *MemoryWriter) Close() error {
	return nil
}
