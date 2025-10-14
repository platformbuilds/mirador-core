package bleve

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// IndexMetadata represents metadata for a Bleve index
type IndexMetadata struct {
	IndexName   string            `json:"index_name"`
	ShardCount  int               `json:"shard_count"`
	ShardNodes  map[string]string `json:"shard_nodes"` // shard_id -> node_id
	CreatedAt   time.Time         `json:"created_at"`
	LastUpdated time.Time         `json:"last_updated"`
	Status      string            `json:"status"` // active, rebalancing, etc.
}

// ClusterState represents the overall cluster state
type ClusterState struct {
	Nodes       map[string]NodeInfo `json:"nodes"`
	LastUpdated time.Time           `json:"last_updated"`
}

// NodeInfo represents information about a cluster node
type NodeInfo struct {
	NodeID     string    `json:"node_id"`
	Address    string    `json:"address"`
	Status     string    `json:"status"` // active, inactive
	LastSeen   time.Time `json:"last_seen"`
	ShardCount int       `json:"shard_count"`
}

// MetadataStore interface for Bleve index coordination
type MetadataStore interface {
	// Index metadata operations
	StoreIndexMetadata(ctx context.Context, metadata *IndexMetadata) error
	GetIndexMetadata(ctx context.Context, indexName string) (*IndexMetadata, error)
	DeleteIndexMetadata(ctx context.Context, indexName string) error
	ListIndices(ctx context.Context) ([]string, error)

	// Cluster state operations
	UpdateClusterState(ctx context.Context, state *ClusterState) error
	GetClusterState(ctx context.Context) (*ClusterState, error)

	// Distributed locks
	AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, lockKey string) error
}

// ValkeyMetadataStore implements MetadataStore using Valkey
type ValkeyMetadataStore struct {
	valkey cache.ValkeyCluster
	logger logger.Logger
}

// NewValkeyMetadataStore creates a new metadata store using Valkey
func NewValkeyMetadataStore(valkey cache.ValkeyCluster, logger logger.Logger) MetadataStore {
	return &ValkeyMetadataStore{
		valkey: valkey,
		logger: logger,
	}
}

// StoreIndexMetadata stores index metadata in Valkey
func (m *ValkeyMetadataStore) StoreIndexMetadata(ctx context.Context, metadata *IndexMetadata) error {
	key := fmt.Sprintf("bleve:index:%s", metadata.IndexName)
	metadata.LastUpdated = time.Now()

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal index metadata: %w", err)
	}

	err = m.valkey.Set(ctx, key, data, 0) // No TTL for metadata
	if err != nil {
		return fmt.Errorf("failed to store index metadata: %w", err)
	}

	m.logger.Info("Stored index metadata", "index", metadata.IndexName)
	return nil
}

// GetIndexMetadata retrieves index metadata from Valkey
func (m *ValkeyMetadataStore) GetIndexMetadata(ctx context.Context, indexName string) (*IndexMetadata, error) {
	key := fmt.Sprintf("bleve:index:%s", indexName)

	data, err := m.valkey.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get index metadata: %w", err)
	}

	var metadata IndexMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal index metadata: %w", err)
	}

	return &metadata, nil
}

// DeleteIndexMetadata removes index metadata from Valkey
func (m *ValkeyMetadataStore) DeleteIndexMetadata(ctx context.Context, indexName string) error {
	key := fmt.Sprintf("bleve:index:%s", indexName)

	err := m.valkey.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete index metadata: %w", err)
	}

	m.logger.Info("Deleted index metadata", "index", indexName)
	return nil
}

// ListIndices returns a list of all index names
func (m *ValkeyMetadataStore) ListIndices(ctx context.Context) ([]string, error) {
	// This is a simplified implementation. In a real scenario, you might use SCAN or maintain a set.
	// For now, we'll return an empty list as we don't have a way to list keys efficiently.
	// TODO: Implement proper index listing, perhaps using a separate set key.
	return []string{}, nil
}

// UpdateClusterState updates the cluster state in Valkey
func (m *ValkeyMetadataStore) UpdateClusterState(ctx context.Context, state *ClusterState) error {
	key := "bleve:cluster:state"
	state.LastUpdated = time.Now()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster state: %w", err)
	}

	err = m.valkey.Set(ctx, key, data, 0)
	if err != nil {
		return fmt.Errorf("failed to store cluster state: %w", err)
	}

	m.logger.Info("Updated cluster state")
	return nil
}

// GetClusterState retrieves the cluster state from Valkey
func (m *ValkeyMetadataStore) GetClusterState(ctx context.Context) (*ClusterState, error) {
	key := "bleve:cluster:state"

	data, err := m.valkey.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster state: %w", err)
	}

	var state ClusterState
	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster state: %w", err)
	}

	return &state, nil
}

// AcquireLock attempts to acquire a distributed lock
func (m *ValkeyMetadataStore) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	return m.valkey.AcquireLock(ctx, lockKey, ttl)
}

// ReleaseLock releases a distributed lock
func (m *ValkeyMetadataStore) ReleaseLock(ctx context.Context, lockKey string) error {
	return m.valkey.ReleaseLock(ctx, lockKey)
}
