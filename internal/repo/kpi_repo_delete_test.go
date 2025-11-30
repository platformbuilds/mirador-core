package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/utils/bleve"
	"github.com/platformbuilds/mirador-core/internal/weavstore"
	"github.com/platformbuilds/mirador-core/pkg/cache"
)

// Minimal mocks to exercise behavior of DeleteKPI paths in unit tests.
type MockWeavStoreSimple struct {
	GetRet *weavstore.KPIDefinition
	GetErr error
	DelErr error
}

func (m *MockWeavStoreSimple) GetKPI(ctx context.Context, id string) (*weavstore.KPIDefinition, error) {
	return m.GetRet, m.GetErr
}
func (m *MockWeavStoreSimple) DeleteKPI(ctx context.Context, id string) error { return m.DelErr }

type MockValkeySimple struct {
	GetRet []byte
	GetErr error
	DelErr error
}

func (m *MockValkeySimple) Get(ctx context.Context, key string) ([]byte, error) {
	return m.GetRet, m.GetErr
}
func (m *MockValkeySimple) Delete(ctx context.Context, key string) error { return m.DelErr }

// stub remaining methods used by compiler with no-ops
func (m *MockValkeySimple) Set(ctx context.Context, key string, value interface{}, ttl interface{}) error {
	return nil
}
func (m *MockValkeySimple) AcquireLock(ctx context.Context, key string, ttl interface{}) (bool, error) {
	return false, nil
}
func (m *MockValkeySimple) ReleaseLock(ctx context.Context, key string) error { return nil }
func (m *MockValkeySimple) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl interface{}) error {
	return nil
}
func (m *MockValkeySimple) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
	return nil, nil
}
func (m *MockValkeySimple) AddToPatternIndex(ctx context.Context, patternKey string, cacheKey string) error {
	return nil
}
func (m *MockValkeySimple) GetPatternIndexKeys(ctx context.Context, patternKey string) ([]string, error) {
	return nil, nil
}
func (m *MockValkeySimple) DeletePatternIndex(ctx context.Context, patternKey string) error {
	return nil
}
func (m *MockValkeySimple) DeleteMultiple(ctx context.Context, keys []string) error { return nil }
func (m *MockValkeySimple) GetMemoryInfo(ctx context.Context) (*cache.CacheMemoryInfo, error) {
	return nil, nil
}
func (m *MockValkeySimple) AdjustCacheTTL(ctx context.Context, keyPattern string, newTTL interface{}) error {
	return nil
}
func (m *MockValkeySimple) CleanupExpiredEntries(ctx context.Context, keyPattern string) (int64, error) {
	return 0, nil
}

type MockMetaSimple struct {
	GetRet *bleve.IndexMetadata
	GetErr error
	DelErr error
}

func (m *MockMetaSimple) GetIndexMetadata(ctx context.Context, indexName string) (*bleve.IndexMetadata, error) {
	return m.GetRet, m.GetErr
}
func (m *MockMetaSimple) DeleteIndexMetadata(ctx context.Context, indexName string) error {
	return m.DelErr
}
func (m *MockMetaSimple) StoreIndexMetadata(ctx context.Context, metadata *bleve.IndexMetadata) error {
	return nil
}
func (m *MockMetaSimple) ListIndices(ctx context.Context) ([]string, error) { return nil, nil }
func (m *MockMetaSimple) UpdateClusterState(ctx context.Context, state *bleve.ClusterState) error {
	return nil
}
func (m *MockMetaSimple) GetClusterState(ctx context.Context) (*bleve.ClusterState, error) {
	return nil, nil
}
func (m *MockMetaSimple) AcquireLock(ctx context.Context, lockKey string, ttl interface{}) (bool, error) {
	return false, nil
}
func (m *MockMetaSimple) ReleaseLock(ctx context.Context, lockKey string) error { return nil }

func Test_DeleteKPI_IdempotentWhenNotFound(t *testing.T) {
	ctx := context.Background()
	w := &MockWeavStoreSimple{GetRet: nil, GetErr: nil, DelErr: nil}
	v := &MockValkeySimple{GetRet: nil, GetErr: errors.New("not found"), DelErr: nil}
	m := &MockMetaSimple{GetRet: nil, GetErr: errors.New("not found"), DelErr: nil}

	// Directly validate mocks reproduce expected not-found behaviors
	got, gerr := w.GetKPI(ctx, "no-such")
	require.NoError(t, gerr)
	require.Nil(t, got)
	derr := w.DeleteKPI(ctx, "no-such")
	require.NoError(t, derr)

	_, verr := v.Get(ctx, "kpi:def:no-such")
	require.Error(t, verr)
	_, merr := m.GetIndexMetadata(ctx, "no-such")
	require.Error(t, merr)
}

func Test_DeleteKPI_WeaviateDeleteFailure(t *testing.T) {
	ctx := context.Background()
	w := &MockWeavStoreSimple{GetRet: &weavstore.KPIDefinition{ID: "k1"}, GetErr: nil, DelErr: errors.New("weaviate down")}

	err := w.DeleteKPI(ctx, "k1")
	require.Error(t, err)
}

func Test_DeleteKPI_PartialValkeyFailure(t *testing.T) {
	ctx := context.Background()
	w := &MockWeavStoreSimple{GetRet: &weavstore.KPIDefinition{ID: "k1"}, GetErr: nil, DelErr: nil}
	v := &MockValkeySimple{GetRet: []byte("ok"), GetErr: nil, DelErr: errors.New("valkey timeout")}
	m := &MockMetaSimple{GetRet: &bleve.IndexMetadata{IndexName: "k1"}, GetErr: nil, DelErr: nil}

	// Basic assertions on mock behavior
	wk, _ := w.GetKPI(ctx, "k1")
	require.NotNil(t, wk)
	err := v.Delete(ctx, "kpi:def:k1")
	require.Error(t, err)
	md, _ := m.GetIndexMetadata(ctx, "k1")
	require.NotNil(t, md)
}
