package bleve

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestValkeyMetadataStore(t *testing.T) {
	// Use noop cache for testing
	valkey := cache.NewNoopValkeyCache(logger.New("test"))
	store := NewValkeyMetadataStore(valkey, logger.New("test"))

	ctx := context.Background()

	t.Run("Store and Get Index Metadata", func(t *testing.T) {
		metadata := &IndexMetadata{
			IndexName:  "test-index",
			ShardCount: 3,
			ShardNodes: map[string]string{
				"shard-0": "node-1",
				"shard-1": "node-2",
				"shard-2": "node-3",
			},
			Status: "active",
		}

		err := store.StoreIndexMetadata(ctx, metadata)
		require.NoError(t, err)

		retrieved, err := store.GetIndexMetadata(ctx, "test-index")
		require.NoError(t, err)
		assert.Equal(t, metadata.IndexName, retrieved.IndexName)
		assert.Equal(t, metadata.ShardCount, retrieved.ShardCount)
		assert.Equal(t, metadata.Status, retrieved.Status)
	})

	t.Run("Delete Index Metadata", func(t *testing.T) {
		metadata := &IndexMetadata{IndexName: "delete-test"}
		err := store.StoreIndexMetadata(ctx, metadata)
		require.NoError(t, err)

		err = store.DeleteIndexMetadata(ctx, "delete-test")
		require.NoError(t, err)

		_, err = store.GetIndexMetadata(ctx, "delete-test")
		assert.Error(t, err)
	})

	t.Run("Cluster State", func(t *testing.T) {
		state := &ClusterState{
			Nodes: map[string]NodeInfo{
				"node-1": {
					NodeID:     "node-1",
					Address:    "localhost:8080",
					Status:     "active",
					ShardCount: 2,
				},
			},
		}

		err := store.UpdateClusterState(ctx, state)
		require.NoError(t, err)

		retrieved, err := store.GetClusterState(ctx)
		require.NoError(t, err)
		assert.Equal(t, state.Nodes["node-1"].NodeID, retrieved.Nodes["node-1"].NodeID)
	})

	t.Run("Distributed Locks", func(t *testing.T) {
		lockKey := "test-lock"

		// Acquire lock
		acquired, err := store.AcquireLock(ctx, lockKey, time.Minute)
		require.NoError(t, err)
		assert.True(t, acquired)

		// Try to acquire again (should fail in real Valkey, but noop allows it)
		_, err = store.AcquireLock(ctx, lockKey, time.Minute)
		require.NoError(t, err)
		// In noop mode, it always returns true

		// Release lock
		err = store.ReleaseLock(ctx, lockKey)
		require.NoError(t, err)
	})

	t.Run("List Indices", func(t *testing.T) {
		// Currently returns empty list as not implemented
		indices, err := store.ListIndices(ctx)
		require.NoError(t, err)
		assert.Empty(t, indices)
	})
}
