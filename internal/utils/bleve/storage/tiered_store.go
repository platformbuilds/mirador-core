package storage

import (
	"fmt"
	"sync"
	"time"

	store "github.com/blevesearch/upsidedown_store_api"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TieredStore implements a two-tiered storage strategy:
// - Tier 1: In-memory cache for frequently accessed data
// - Tier 2: Disk-based persistent storage for all data
type TieredStore struct {
	memoryStore *MemoryStore
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

// MemoryStore is a simple in-memory key-value store with TTL
type MemoryStore struct {
	data   map[string][]byte
	expiry map[string]time.Time
	mu     sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data:   make(map[string][]byte),
		expiry: make(map[string]time.Time),
	}
}

func (m *MemoryStore) Get(key []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := string(key)
	if expiry, exists := m.expiry[k]; exists && time.Now().After(expiry) {
		// Key has expired, remove it
		delete(m.data, k)
		delete(m.expiry, k)
		return nil, fmt.Errorf("key not found")
	}

	if value, exists := m.data[k]; exists {
		return value, nil
	}

	return nil, fmt.Errorf("key not found")
}

func (m *MemoryStore) Set(key []byte, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := string(key)
	m.data[k] = value
	if ttl > 0 {
		m.expiry[k] = time.Now().Add(ttl)
	} else {
		delete(m.expiry, k) // No expiry
	}
	return nil
}

func (m *MemoryStore) Delete(key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := string(key)
	delete(m.data, k)
	delete(m.expiry, k)
	return nil
}

func (m *MemoryStore) Has(key []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := string(key)
	if expiry, exists := m.expiry[k]; exists && time.Now().After(expiry) {
		// Clean up expired key
		delete(m.data, k)
		delete(m.expiry, k)
		return false
	}

	_, exists := m.data[k]
	return exists
}

func (m *MemoryStore) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Clean up expired keys
	now := time.Now()
	for k, expiry := range m.expiry {
		if now.After(expiry) {
			delete(m.data, k)
			delete(m.expiry, k)
		}
	}

	return len(m.data)
}

// NewTieredStore creates a new tiered storage instance
func NewTieredStore(diskStore store.KVStore, maxMemoryItems int, ttl time.Duration, logger logger.Logger) *TieredStore {
	ts := &TieredStore{
		memoryStore:    NewMemoryStore(),
		diskStore:      diskStore,
		logger:         logger,
		maxMemoryItems: maxMemoryItems,
		ttl:            ttl,
	}

	// Start adaptive cache sizing goroutine
	go ts.adaptiveCacheManager()

	return ts
}

// adaptiveCacheManager periodically adjusts cache sizes based on memory pressure
func (t *TieredStore) adaptiveCacheManager() {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		t.optimizeCacheSize()
	}
}

// optimizeCacheSize dynamically adjusts cache sizes based on available memory
func (t *TieredStore) optimizeCacheSize() {
	availableMemory := t.getAvailableMemory()
	memoryPressure := t.calculateMemoryPressure()

	var newMaxMemoryItems int

	switch {
	case memoryPressure > 0.8: // High memory pressure
		// Reduce memory cache to 30% of available memory
		newMaxMemoryItems = int(float64(availableMemory) * 0.3 / float64(t.estimateItemSize()))
		t.logger.Info("High memory pressure detected, reducing cache size",
			"newMaxMemoryItems", newMaxMemoryItems,
			"memoryPressure", memoryPressure)

	case memoryPressure > 0.6: // Moderate memory pressure
		// Maintain 50% of available memory
		newMaxMemoryItems = int(float64(availableMemory) * 0.5 / float64(t.estimateItemSize()))
		t.logger.Info("Moderate memory pressure detected, adjusting cache size",
			"newMaxMemoryItems", newMaxMemoryItems,
			"memoryPressure", memoryPressure)

	default: // Low memory pressure
		// Use 70% of available memory
		newMaxMemoryItems = int(float64(availableMemory) * 0.7 / float64(t.estimateItemSize()))
	}

	// Ensure minimum cache size
	if newMaxMemoryItems < 100 {
		newMaxMemoryItems = 100
	}

	// Apply new cache size
	t.mu.Lock()
	oldSize := t.maxMemoryItems
	t.maxMemoryItems = newMaxMemoryItems
	t.mu.Unlock()

	if oldSize != newMaxMemoryItems {
		t.logger.Info("Cache size adjusted",
			"oldSize", oldSize,
			"newSize", newMaxMemoryItems,
			"memoryPressure", memoryPressure)
	}
}

// getAvailableMemory estimates available memory (simplified implementation)
func (t *TieredStore) getAvailableMemory() int {
	// In a real implementation, this would query system memory
	// For now, use a conservative estimate
	return 100 * 1024 * 1024 // Assume 100MB available (adjust based on system)
}

// calculateMemoryPressure returns a value between 0-1 indicating memory pressure
func (t *TieredStore) calculateMemoryPressure() float64 {
	currentItems := t.memoryStore.Size()
	maxItems := t.maxMemoryItems

	if maxItems == 0 {
		return 0
	}

	pressure := float64(currentItems) / float64(maxItems)

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
		memoryStore:    NewMemoryStore(),
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
func (t *TieredStore) RecordStorageMetrics(tenantID, shardNum string) {
	memoryUsage := t.memoryStore.Size() * t.estimateItemSize()
	// For disk usage, we'd need to implement disk size tracking
	// For now, use a placeholder
	diskUsage := int64(0) // TODO: Implement actual disk usage tracking

	monitoring.RecordBleveStorageUsage(tenantID, shardNum, int64(memoryUsage), diskUsage)
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
	store *MemoryStore
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
