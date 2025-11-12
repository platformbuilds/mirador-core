package bleve

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/utils/bleve/mapping"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/storage"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func setupTestShardManager(t *testing.T, numShards int) (*ShardManager, func()) {
	// Create temporary directory for indexes
	tempDir, err := os.MkdirTemp("", "bleve_test_*")
	require.NoError(t, err)

	// Create mock dependencies
	mockStorage := storage.NewTieredStore(nil, 1000, time.Hour, logger.New("test"))
	mockMetadata := &mockMetadataStore{}
	mockMapper := mapping.NewBleveDocumentMapper(logger.New("test"))

	manager := NewShardManager(numShards, mockStorage, mockMetadata, mockMapper, logger.New("test"), tempDir)

	// Cleanup function
	cleanup := func() {
		manager.Close()
		os.RemoveAll(tempDir)
	}

	return manager, cleanup
}

func TestConsistentHashShardStrategy_GetShardID(t *testing.T) {
	strategy := &ConsistentHashShardStrategy{}

	// Test that same document ID always gets same shard
	docID := "test_doc_123"
	shard1 := strategy.GetShardID(docID, 4)
	shard2 := strategy.GetShardID(docID, 4)
	assert.Equal(t, shard1, shard2)

	// Test distribution across shards
	shardIDs := make(map[string]bool)
	for i := 0; i < 100; i++ {
		shardID := strategy.GetShardID(fmt.Sprintf("doc_%d", i), 4)
		shardIDs[shardID] = true
	}
	// Should use multiple shards
	assert.Greater(t, len(shardIDs), 1)
}

func TestShardManager_InitializeShards(t *testing.T) {
	manager, cleanup := setupTestShardManager(t, 3)
	defer cleanup()

	err := manager.InitializeShards("test-tenant")
	require.NoError(t, err)

	// Check that shards were created
	stats := manager.GetShardStats()
	assert.Len(t, stats, 3)

	for i := 0; i < 3; i++ {
		shardID := fmt.Sprintf("tenant_test-tenant_shard_%d", i)
		shardStats, exists := stats[shardID]
		assert.True(t, exists, "Shard %s should exist", shardID)

		statsMap := shardStats.(map[string]interface{})
		assert.Equal(t, shardID, statsMap["id"])
		assert.True(t, statsMap["isActive"].(bool))
	}
}

func TestShardManager_IndexDocuments(t *testing.T) {
	manager, cleanup := setupTestShardManager(t, 2)
	defer cleanup()

	err := manager.InitializeShards("test-tenant")
	require.NoError(t, err)

	// Create test documents
	documents := []mapping.IndexableDocument{
		{
			ID:   "doc1",
			Data: mapping.LogDocument{TenantID: "test-tenant", Message: "test message 1"},
		},
		{
			ID:   "doc2",
			Data: mapping.LogDocument{TenantID: "test-tenant", Message: "test message 2"},
		},
		{
			ID:   "doc3",
			Data: mapping.LogDocument{TenantID: "test-tenant", Message: "test message 3"},
		},
	}

	err = manager.IndexDocuments(documents, "test-tenant")
	require.NoError(t, err)

	// Check that documents were distributed across shards
	stats := manager.GetShardStats()
	totalDocs := 0
	for _, shardStats := range stats {
		statsMap := shardStats.(map[string]interface{})
		docCount := statsMap["docCount"].(uint64)
		totalDocs += int(docCount)
	}
	assert.Equal(t, len(documents), totalDocs)
}

func TestShardManager_Search(t *testing.T) {
	manager, cleanup := setupTestShardManager(t, 2)
	defer cleanup()

	err := manager.InitializeShards("test-tenant")
	require.NoError(t, err)

	// Index a test document
	documents := []mapping.IndexableDocument{
		{
			ID:   "search_test_doc",
			Data: mapping.LogDocument{TenantID: "test-tenant", Message: "searchable content"},
		},
	}

	err = manager.IndexDocuments(documents, "test-tenant")
	require.NoError(t, err)

	// Perform a search
	query := bleve.NewQueryStringQuery("searchable")
	request := bleve.NewSearchRequest(query)
	result, err := manager.Search(request, "test-tenant")
	require.NoError(t, err)

	// Should find the document
	assert.Greater(t, result.Total, uint64(0))
}

func TestShardManager_GetShardStats(t *testing.T) {
	manager, cleanup := setupTestShardManager(t, 2)
	defer cleanup()

	err := manager.InitializeShards("test-tenant")
	require.NoError(t, err)

	stats := manager.GetShardStats()
	assert.Len(t, stats, 2)

	for _, shardStats := range stats {
		statsMap := shardStats.(map[string]interface{})
		assert.Contains(t, statsMap, "id")
		assert.Contains(t, statsMap, "path")
		assert.Contains(t, statsMap, "isActive")
		assert.Contains(t, statsMap, "docCount")
		assert.True(t, statsMap["isActive"].(bool))
	}
}

func TestShardManager_Close(t *testing.T) {
	manager, _ := setupTestShardManager(t, 2)
	// Don't use cleanup since we're testing Close() explicitly

	err := manager.InitializeShards("test-tenant")
	require.NoError(t, err)

	// Close should not error
	err = manager.Close()
	assert.NoError(t, err)

	// Clean up temp directory manually
	if manager.basePath != "" {
		os.RemoveAll(manager.basePath)
	}
}

// mockMetadataStore implements MetadataStore for testing
type mockMetadataStore struct{}

func (m *mockMetadataStore) StoreIndexMetadata(ctx context.Context, metadata *IndexMetadata) error {
	return nil
}

func (m *mockMetadataStore) GetIndexMetadata(ctx context.Context, indexName string) (*IndexMetadata, error) {
	return &IndexMetadata{}, nil
}

func (m *mockMetadataStore) DeleteIndexMetadata(ctx context.Context, indexName string) error {
	return nil
}

func (m *mockMetadataStore) ListIndices(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (m *mockMetadataStore) UpdateClusterState(ctx context.Context, state *ClusterState) error {
	return nil
}

func (m *mockMetadataStore) GetClusterState(ctx context.Context) (*ClusterState, error) {
	return &ClusterState{}, nil
}

func (m *mockMetadataStore) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	return true, nil
}

func (m *mockMetadataStore) ReleaseLock(ctx context.Context, lockKey string) error {
	return nil
}
