package bleve

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"

	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/mapping"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/storage"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// ShardManager manages a collection of Bleve indexes distributed across shards
type ShardManager struct {
	shards        map[string]*Shard
	numShards     int
	shardMutex    sync.RWMutex
	storage       *storage.TieredStore
	metadata      MetadataStore
	mapper        mapping.DocumentMapper
	logger        logger.Logger
	shardStrategy ShardStrategy
	basePath      string
}

// Shard represents a single Bleve index shard
type Shard struct {
	ID       string
	Index    bleve.Index
	Path     string
	IsActive bool
	mutex    sync.RWMutex
}

// ShardStrategy defines how documents are distributed across shards
type ShardStrategy interface {
	GetShardID(documentID string, numShards int) string
}

// ConsistentHashShardStrategy implements consistent hashing for shard distribution
type ConsistentHashShardStrategy struct{}

// GetShardID returns the shard ID for a document using consistent hashing
func (s *ConsistentHashShardStrategy) GetShardID(documentID string, numShards int) string {
	h := fnv.New32a()
	h.Write([]byte(documentID))
	hash := h.Sum32()
	return fmt.Sprintf("shard_%d", int(hash)%numShards)
}

// NewShardManager creates a new shard manager
func NewShardManager(
	numShards int,
	storage *storage.TieredStore,
	metadata MetadataStore,
	mapper mapping.DocumentMapper,
	logger logger.Logger,
	basePath string,
) *ShardManager {
	return &ShardManager{
		shards:        make(map[string]*Shard),
		numShards:     numShards,
		storage:       storage,
		metadata:      metadata,
		mapper:        mapper,
		logger:        logger,
		shardStrategy: &ConsistentHashShardStrategy{},
		basePath:      basePath,
	}
}

// InitializeShards creates and initializes all shards
func (sm *ShardManager) InitializeShards(tenantID string) error {
	// Acquire distributed lock for shard initialization
	lockKey := fmt.Sprintf("bleve:shards:init:%s", tenantID)
	locked, err := sm.metadata.AcquireLock(context.Background(), lockKey, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to acquire shard initialization lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("shard initialization already in progress for tenant %s", tenantID)
	}
	defer func() {
		if err := sm.metadata.ReleaseLock(context.Background(), lockKey); err != nil {
			sm.logger.Warn("Failed to release shard initialization lock", "tenantID", tenantID, "error", err)
		}
	}()

	sm.shardMutex.Lock()
	defer sm.shardMutex.Unlock()

	for i := 0; i < sm.numShards; i++ {
		shardID := fmt.Sprintf("tenant_%s_shard_%d", tenantID, i)
		shardPath := fmt.Sprintf("%s/tenant_%s_%s", sm.basePath, tenantID, shardID) // Create Bleve index for this shard
		index, err := bleve.New(shardPath, bleve.NewIndexMapping())
		if err != nil {
			sm.logger.Error("Failed to create shard index", "shardID", shardID, "error", err)
			return fmt.Errorf("failed to create shard %s: %w", shardID, err)
		}

		shard := &Shard{
			ID:       shardID,
			Index:    index,
			Path:     shardPath,
			IsActive: true,
		}

		sm.shards[shardID] = shard
		sm.logger.Info("Created shard", "shardID", shardID, "path", shardPath)
	}

	// Store shard metadata
	shardMetadata := &IndexMetadata{
		IndexName:   fmt.Sprintf("tenant_%s_shards", tenantID),
		ShardCount:  sm.numShards,
		ShardNodes:  sm.getShardNodeMap(),
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
		Status:      "active",
	}

	if err := sm.metadata.StoreIndexMetadata(context.Background(), shardMetadata); err != nil {
		return err
	}

	// Record shard count metric
	monitoring.RecordBleveShardCount(tenantID, int64(sm.numShards))
	return nil
}

// IndexDocuments indexes a batch of documents across appropriate shards
func (sm *ShardManager) IndexDocuments(documents []mapping.IndexableDocument, tenantID string) error {
	start := time.Now()

	// Acquire distributed lock for indexing operation
	lockKey := fmt.Sprintf("bleve:indexing:%s", tenantID)
	locked, err := sm.metadata.AcquireLock(context.Background(), lockKey, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to acquire indexing lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("indexing operation already in progress for tenant %s", tenantID)
	}
	defer func() {
		if err := sm.metadata.ReleaseLock(context.Background(), lockKey); err != nil {
			sm.logger.Warn("Failed to release indexing lock", "tenantID", tenantID, "error", err)
		}
	}()

	// Group documents by shard
	shardGroups := make(map[string][]mapping.IndexableDocument)

	for _, doc := range documents {
		shardID := fmt.Sprintf("tenant_%s_%s", tenantID, sm.shardStrategy.GetShardID(doc.ID, sm.numShards))
		shardGroups[shardID] = append(shardGroups[shardID], doc)
	}

	// Index documents in each shard
	for shardID, docs := range shardGroups {
		if err := sm.indexDocumentsInShard(shardID, docs); err != nil {
			sm.logger.Error("Failed to index documents in shard", "shardID", shardID, "error", err)
			monitoring.RecordBleveIndexOperation("batch_index", tenantID, time.Since(start), false)
			return fmt.Errorf("failed to index in shard %s: %w", shardID, err)
		}
	}

	sm.logger.Info("Indexed documents across shards", "totalDocs", len(documents), "shardsUsed", len(shardGroups))
	monitoring.RecordBleveIndexOperation("batch_index", tenantID, time.Since(start), true)
	return nil
}

// indexDocumentsInShard indexes documents in a specific shard
func (sm *ShardManager) indexDocumentsInShard(shardID string, documents []mapping.IndexableDocument) error {
	sm.shardMutex.RLock()
	shard, exists := sm.shards[shardID]
	sm.shardMutex.RUnlock()

	if !exists {
		return fmt.Errorf("shard %s does not exist", shardID)
	}

	shard.mutex.Lock()
	defer shard.mutex.Unlock()

	batch := shard.Index.NewBatch()
	for _, doc := range documents {
		if err := batch.Index(doc.ID, doc.Data); err != nil {
			return fmt.Errorf("failed to add document %s to batch: %w", doc.ID, err)
		}
	}

	if err := shard.Index.Batch(batch); err != nil {
		return fmt.Errorf("failed to execute batch for shard %s: %w", shardID, err)
	}

	return nil
}

// Search executes a search query across all active shards for a specific tenant
func (sm *ShardManager) Search(request *bleve.SearchRequest, tenantID string) (*bleve.SearchResult, error) {
	start := time.Now()
	sm.shardMutex.RLock()
	defer sm.shardMutex.RUnlock()

	// Execute search on all active shards for this tenant
	var allResults []*bleve.SearchResult
	tenantPrefix := fmt.Sprintf("tenant_%s_shard_", tenantID)

	for shardID, shard := range sm.shards {
		// Only search shards that belong to this tenant
		if !strings.HasPrefix(shardID, tenantPrefix) {
			continue
		}

		if !shard.IsActive {
			continue
		}

		result, err := shard.Index.Search(request)
		if err != nil {
			sm.logger.Warn("Search failed on shard", "shardID", shard.ID, "error", err)
			monitoring.RecordBleveSearchOperation(tenantID, time.Since(start), 0, false)
			return nil, fmt.Errorf("search failed on shard %s: %w", shardID, err)
		}
		allResults = append(allResults, result)
	}

	// Merge results from all shards
	if len(allResults) == 0 {
		monitoring.RecordBleveSearchOperation(tenantID, time.Since(start), 0, true)
		return &bleve.SearchResult{}, nil
	}

	// Merge all results properly
	mergedResult := &bleve.SearchResult{
		Status: &bleve.SearchStatus{},
	}

	totalHits := uint64(0)
	var allHits []*search.DocumentMatch

	for _, result := range allResults {
		totalHits += result.Total
		if result.Hits != nil {
			allHits = append(allHits, result.Hits...)
		}
		// Update status and other metadata from the result with most hits
		if result.Total > mergedResult.Total {
			mergedResult.Status = result.Status
			mergedResult.Request = result.Request
			mergedResult.Facets = result.Facets
			mergedResult.Total = result.Total
			mergedResult.Took = result.Took
			mergedResult.MaxScore = result.MaxScore
		}
	}

	// Sort hits by score and take top results
	if len(allHits) > 0 {
		sort.Slice(allHits, func(i, j int) bool {
			return allHits[i].Score > allHits[j].Score
		})
		// Take top 100 hits or configurable limit
		maxHits := 100
		if len(allHits) > maxHits {
			allHits = allHits[:maxHits]
		}
		mergedResult.Hits = allHits
	}

	mergedResult.Total = totalHits

	monitoring.RecordBleveSearchOperation(tenantID, time.Since(start), int(totalHits), true)
	return mergedResult, nil
}

// GetShardStats returns statistics for all shards
func (sm *ShardManager) GetShardStats() map[string]interface{} {
	sm.shardMutex.RLock()
	defer sm.shardMutex.RUnlock()

	stats := make(map[string]interface{})
	tenantShardCounts := make(map[string]int)

	for shardID, shard := range sm.shards {
		shardStats := map[string]interface{}{
			"id":       shard.ID,
			"path":     shard.Path,
			"isActive": shard.IsActive,
		}

		// Get document count from Bleve index
		docCount, err := shard.Index.DocCount()
		if err != nil {
			sm.logger.Warn("Failed to get doc count for shard", "shardID", shardID, "error", err)
			shardStats["docCount"] = 0
		} else {
			shardStats["docCount"] = docCount
			// Record document count metric
			parts := strings.Split(shardID, "_")
			if len(parts) >= 3 {
				tenantID := parts[1] // tenant_{tenantID}_shard_{num}
				shardNum := parts[3]
				monitoring.RecordBleveIndexHealth(tenantID, shardNum, int64(docCount))
				// Record storage metrics if storage is available
				if sm.storage != nil {
					sm.storage.RecordStorageMetrics(tenantID, shardNum)
				}
				tenantShardCounts[tenantID]++
			}
		}

		stats[shardID] = shardStats
	}

	// Record shard counts for each tenant
	for tenantID, count := range tenantShardCounts {
		monitoring.RecordBleveShardCount(tenantID, int64(count))
	}

	return stats
}

// Close closes all shard indexes
func (sm *ShardManager) Close() error {
	sm.shardMutex.Lock()
	defer sm.shardMutex.Unlock()

	var errors []error
	for shardID, shard := range sm.shards {
		if err := shard.Index.Close(); err != nil {
			sm.logger.Error("Failed to close shard", "shardID", shardID, "error", err)
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to close %d shards", len(errors))
	}

	return nil
}

// getShardNodeMap returns a map of shard IDs to node IDs (for now, all shards are on the local node)
func (sm *ShardManager) getShardNodeMap() map[string]string {
	nodeMap := make(map[string]string)
	for id := range sm.shards {
		nodeMap[id] = "local-node" // In a distributed setup, this would be actual node IDs
	}
	return nodeMap
}
