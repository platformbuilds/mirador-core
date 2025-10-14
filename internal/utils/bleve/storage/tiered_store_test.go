package storage

import (
	"testing"
	"time"

	store "github.com/blevesearch/upsidedown_store_api"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDiskStore is a simple in-memory mock for testing
type MockDiskStore struct {
	data map[string][]byte
}

func NewMockDiskStore() *MockDiskStore {
	return &MockDiskStore{data: make(map[string][]byte)}
}

func (m *MockDiskStore) Reader() (store.KVReader, error) {
	return &MockReader{store: m}, nil
}

func (m *MockDiskStore) Writer() (store.KVWriter, error) {
	return &MockWriter{store: m}, nil
}

func (m *MockDiskStore) Close() error {
	return nil
}

type MockReader struct {
	store *MockDiskStore
}

func (m *MockReader) Get(key []byte) ([]byte, error) {
	value, exists := m.store.data[string(key)]
	if !exists {
		return nil, nil
	}
	return value, nil
}

func (m *MockReader) MultiGet(keys [][]byte) ([][]byte, error) {
	results := make([][]byte, len(keys))
	for i, key := range keys {
		results[i], _ = m.Get(key)
	}
	return results, nil
}

func (m *MockReader) PrefixIterator(prefix []byte) store.KVIterator {
	return &MockIterator{}
}

func (m *MockReader) RangeIterator(start, end []byte) store.KVIterator {
	return &MockIterator{}
}

func (m *MockReader) Close() error {
	return nil
}

type MockWriter struct {
	store *MockDiskStore
}

func (m *MockWriter) NewBatch() store.KVBatch {
	return &MockBatch{store: m.store}
}

func (m *MockWriter) NewBatchEx(opts store.KVBatchOptions) ([]byte, store.KVBatch, error) {
	buf := make([]byte, opts.TotalBytes)
	return buf, &MockBatch{store: m.store}, nil
}

func (m *MockWriter) ExecuteBatch(batch store.KVBatch) error {
	mockBatch := batch.(*MockBatch)
	for k, v := range mockBatch.data {
		m.store.data[k] = v
	}
	return nil
}

func (m *MockWriter) Close() error {
	return nil
}

type MockBatch struct {
	store *MockDiskStore
	data  map[string][]byte
}

func (m *MockBatch) Set(key, value []byte) {
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[string(key)] = value
}

func (m *MockBatch) Delete(key []byte) {
	delete(m.data, string(key))
}

func (m *MockBatch) Merge(key, value []byte) {
	m.Set(key, value)
}

func (m *MockBatch) Reset() {
	m.data = make(map[string][]byte)
}

func (m *MockBatch) Close() error {
	return nil
}

type MockIterator struct{}

func (m *MockIterator) SeekFirst()                      {}
func (m *MockIterator) Seek(key []byte)                 {}
func (m *MockIterator) Next()                           {}
func (m *MockIterator) Current() ([]byte, []byte, bool) { return nil, nil, false }
func (m *MockIterator) Key() []byte                     { return nil }
func (m *MockIterator) Value() []byte                   { return nil }
func (m *MockIterator) Valid() bool                     { return false }
func (m *MockIterator) Close() error                    { return nil }

func TestTieredStore_BasicOperations(t *testing.T) {
	diskStore := NewMockDiskStore()
	logger := logger.New("test")

	store := NewTieredStoreWithDisk(diskStore, 100, time.Hour, logger)
	defer store.Close()

	// Test writer
	writer, err := store.Writer()
	require.NoError(t, err)
	defer writer.Close()

	batch := writer.NewBatch()
	batch.Set([]byte("key1"), []byte("value1"))
	batch.Set([]byte("key2"), []byte("value2"))
	err = writer.ExecuteBatch(batch)
	require.NoError(t, err)

	// Test reader
	reader, err := store.Reader()
	require.NoError(t, err)
	defer reader.Close()

	value, err := reader.Get([]byte("key1"))
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)

	value, err = reader.Get([]byte("key2"))
	require.NoError(t, err)
	assert.Equal(t, []byte("value2"), value)

	// Test non-existent key
	value, err = reader.Get([]byte("nonexistent"))
	assert.NoError(t, err)
	assert.Nil(t, value)
}

func TestTieredStore_MemoryCaching(t *testing.T) {
	diskStore := NewMockDiskStore()
	logger := logger.New("test")

	store := NewTieredStoreWithDisk(diskStore, 100, time.Hour, logger)
	defer store.Close()

	// First access should hit disk
	writer, err := store.Writer()
	require.NoError(t, err)
	defer writer.Close()

	batch := writer.NewBatch()
	batch.Set([]byte("cached_key"), []byte("cached_value"))
	err = writer.ExecuteBatch(batch)
	require.NoError(t, err)

	reader, err := store.Reader()
	require.NoError(t, err)
	defer reader.Close()

	// First read - should cache in memory
	value1, err := reader.Get([]byte("cached_key"))
	require.NoError(t, err)
	assert.Equal(t, []byte("cached_value"), value1)

	// Second read - should hit memory cache
	value2, err := reader.Get([]byte("cached_key"))
	require.NoError(t, err)
	assert.Equal(t, []byte("cached_value"), value2)

	// Check stats
	stats := store.Stats()
	assert.True(t, stats["memory_hits"].(int64) > 0)
}

func TestMemoryStore_BasicOperations(t *testing.T) {
	store := NewMemoryStore()

	// Test Set and Get
	err := store.Set([]byte("key1"), []byte("value1"), 0)
	require.NoError(t, err)

	value, err := store.Get([]byte("key1"))
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), value)

	// Test Delete
	err = store.Delete([]byte("key1"))
	require.NoError(t, err)

	_, err = store.Get([]byte("key1"))
	assert.Error(t, err)
}

func TestMemoryStore_TTL(t *testing.T) {
	store := NewMemoryStore()

	// Set with short TTL
	err := store.Set([]byte("ttl_key"), []byte("ttl_value"), 10*time.Millisecond)
	require.NoError(t, err)

	// Should exist immediately
	value, err := store.Get([]byte("ttl_key"))
	require.NoError(t, err)
	assert.Equal(t, []byte("ttl_value"), value)

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	// Should be gone
	_, err = store.Get([]byte("ttl_key"))
	assert.Error(t, err)
}
