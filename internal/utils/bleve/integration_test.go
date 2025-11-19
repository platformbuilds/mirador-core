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
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// IntegrationTestSuite represents a complete distributed indexing system for testing
type IntegrationTestSuite struct {
	tempDir      string
	metadata     MetadataStore
	storage      *storage.TieredStore
	mapper       mapping.DocumentMapper
	shardManager *ShardManager
	coordinator  *ClusterCoordinator
	logger       logger.Logger
}

// SetupIntegrationTest creates a complete test environment
func SetupIntegrationTest(t *testing.T) *IntegrationTestSuite {
	tempDir, err := os.MkdirTemp("", "mirador_integration_test_*")
	require.NoError(t, err)

	logger := logger.New("integration-test")

	// Setup metadata store
	valkey := cache.NewNoopValkeyCache(logger)
	metadata := NewValkeyMetadataStore(valkey, logger)

	// Setup storage
	diskStore := storage.NewTieredStore(nil, 1000, time.Hour, logger)

	// Setup mapper
	mapper := mapping.NewBleveDocumentMapper(logger)

	// Setup shard manager
	shardManager := NewShardManager(3, diskStore, metadata, mapper, logger, tempDir)

	// Setup cluster coordinator
	coordinator := NewClusterCoordinator("test-node-1", metadata, logger)

	return &IntegrationTestSuite{
		tempDir:      tempDir,
		metadata:     metadata,
		storage:      diskStore,
		mapper:       mapper,
		shardManager: shardManager,
		coordinator:  coordinator,
		logger:       logger,
	}
}

// Teardown cleans up the test environment
func (suite *IntegrationTestSuite) Teardown() {
	if suite.coordinator != nil {
		suite.coordinator.Stop()
	}
	if suite.shardManager != nil {
		suite.shardManager.Close()
	}
	os.RemoveAll(suite.tempDir)
}

// TestDistributedIndexingEndToEnd tests the complete indexing pipeline
func TestDistributedIndexingEndToEnd(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start cluster coordinator
	err := suite.coordinator.Start(ctx)
	require.NoError(t, err)

	// Initialize shards
	err = suite.shardManager.InitializeShards()
	require.NoError(t, err)

	// Create test data - mix of logs and traces
	logs := []map[string]any{
		{
			"timestamp": time.Now(),
			"level":     "INFO",
			"message":   "User login successful",
			"service":   "auth-service",
			"host":      "auth-01",
			"user_id":   "user123",
		},
		{
			"timestamp":  time.Now().Add(-time.Minute),
			"level":      "ERROR",
			"message":    "Database connection failed",
			"service":    "db-service",
			"host":       "db-01",
			"error_code": "CONN_TIMEOUT",
		},
	}

	traces := []map[string]interface{}{
		{
			"traceID": "trace-001",
			"spans": []interface{}{
				map[string]interface{}{
					"spanID":        "span-001",
					"operationName": "http_request",
					"process": map[string]interface{}{
						"serviceName": "web-service",
					},
					"startTime": time.Now().Add(-time.Minute),
					"duration":  5000000, // 5ms in microseconds
				},
			},
		},
	}

	// Map and index logs
	logDocuments, err := suite.mapper.MapLogs(logs)
	require.NoError(t, err)
	assert.Len(t, logDocuments, 2)

	err = suite.shardManager.IndexDocuments(logDocuments)
	require.NoError(t, err)

	// Map and index traces
	traceDocuments, err := suite.mapper.MapTraces(traces)
	require.NoError(t, err)
	assert.Len(t, traceDocuments, 1)

	err = suite.shardManager.IndexDocuments(traceDocuments)
	require.NoError(t, err)

	// Verify indexing by checking shard stats
	stats := suite.shardManager.GetShardStats()
	totalDocs := 0
	for _, shardStats := range stats {
		statsMap := shardStats.(map[string]interface{})
		docCount := statsMap["docCount"].(uint64)
		totalDocs += int(docCount)
	}
	assert.Equal(t, 3, totalDocs) // 2 logs + 1 trace

	// Test search functionality
	query := bleve.NewQueryStringQuery("login")
	request := bleve.NewSearchRequest(query)
	result, err := suite.shardManager.Search(request)
	require.NoError(t, err)
	assert.Greater(t, result.Total, uint64(0))

	// Test service-specific search
	query = bleve.NewQueryStringQuery("auth-service")
	request = bleve.NewSearchRequest(query)
	result, err = suite.shardManager.Search(request)
	require.NoError(t, err)
	assert.Greater(t, result.Total, uint64(0))
}

// TestShardDistribution tests that documents are properly distributed across shards
func TestShardDistribution(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.Teardown()

	// Initialize shards
	err := suite.shardManager.InitializeShards()
	require.NoError(t, err)

	// Create many documents to test distribution
	documents := make([]mapping.IndexableDocument, 100)
	for i := 0; i < 100; i++ {
		documents[i] = mapping.IndexableDocument{
			ID: fmt.Sprintf("doc_%d", i),
			Data: mapping.LogDocument{
				Message:   fmt.Sprintf("Test message %d", i),
				Service:   "test-service",
				Timestamp: time.Now(),
			},
		}
	}

	// Index documents
	err = suite.shardManager.IndexDocuments(documents)
	require.NoError(t, err)

	// Check distribution across shards
	stats := suite.shardManager.GetShardStats()
	totalDocs := 0
	shardsUsed := 0

	for _, shardStats := range stats {
		statsMap := shardStats.(map[string]interface{})
		docCount := statsMap["docCount"].(uint64)
		totalDocs += int(docCount)
		if docCount > 0 {
			shardsUsed++
		}
	}

	assert.Equal(t, 100, totalDocs)
	assert.Greater(t, shardsUsed, 1, "Documents should be distributed across multiple shards")
}

// TestClusterCoordination tests multi-node coordination
func TestClusterCoordination(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start coordinator
	err := suite.coordinator.Start(ctx)
	require.NoError(t, err)

	// Trigger leader election
	err = suite.coordinator.PerformLeaderElection(ctx)
	require.NoError(t, err)

	// Verify leadership
	assert.True(t, suite.coordinator.IsLeader())
	assert.Equal(t, "test-node-1", suite.coordinator.GetLeaderID())

	// Check cluster status
	status := suite.coordinator.GetClusterStatus()
	assert.Equal(t, 1, status["member_count"])
	assert.True(t, status["is_leader"].(bool))

	members := status["members"].([]map[string]interface{})
	assert.Len(t, members, 1)
	assert.Equal(t, "test-node-1", members[0]["node_id"])
	assert.True(t, members[0]["is_active"].(bool))
}

// TestFailureRecovery tests system behavior under failures
func TestFailureRecovery(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Start system
	err := suite.coordinator.Start(ctx)
	require.NoError(t, err)

	err = suite.shardManager.InitializeShards()
	require.NoError(t, err)

	// Index some data
	documents := []mapping.IndexableDocument{
		{
			ID: "recovery_test_1",
			Data: mapping.LogDocument{
				Message:   "Recovery test message",
				Service:   "test-service",
				Timestamp: time.Now(),
			},
		},
	}

	err = suite.shardManager.IndexDocuments(documents)
	require.NoError(t, err)

	// Verify data is searchable
	query := bleve.NewQueryStringQuery("recovery")
	request := bleve.NewSearchRequest(query)
	result, err := suite.shardManager.Search(request)
	require.NoError(t, err)
	assert.Greater(t, result.Total, uint64(0))

	// Simulate coordinator restart
	err = suite.coordinator.Stop()
	require.NoError(t, err)

	// Create new coordinator (simulating restart)
	newCoordinator := NewClusterCoordinator("test-node-1", suite.metadata, suite.logger)
	err = newCoordinator.Start(ctx)
	require.NoError(t, err)
	defer newCoordinator.Stop()

	// Verify system still works
	result2, err := suite.shardManager.Search(request)
	require.NoError(t, err)
	assert.Equal(t, result.Total, result2.Total, "Search results should be consistent after restart")
}

// TestConcurrentIndexing tests concurrent indexing operations
func TestConcurrentIndexing(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.Teardown()

	// Initialize shards
	err := suite.shardManager.InitializeShards()
	require.NoError(t, err)

	// Create concurrent indexing goroutines
	numWorkers := 5
	docsPerWorker := 20
	done := make(chan bool, numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			documents := make([]mapping.IndexableDocument, docsPerWorker)
			for j := 0; j < docsPerWorker; j++ {
				documents[j] = mapping.IndexableDocument{
					ID: fmt.Sprintf("concurrent_doc_w%d_%d", workerID, j),
					Data: mapping.LogDocument{
						Message:   fmt.Sprintf("Concurrent message from worker %d", workerID),
						Service:   fmt.Sprintf("worker-service-%d", workerID),
						Timestamp: time.Now(),
					},
				}
			}

			err := suite.shardManager.IndexDocuments(documents)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < numWorkers; i++ {
		select {
		case <-done:
			// Worker completed
		case <-time.After(10 * time.Second):
			t.Fatal("Worker timed out")
		}
	}

	// Verify all documents were indexed
	stats := suite.shardManager.GetShardStats()
	totalDocs := 0
	for _, shardStats := range stats {
		statsMap := shardStats.(map[string]interface{})
		docCount := statsMap["docCount"].(uint64)
		totalDocs += int(docCount)
	}

	expectedDocs := numWorkers * docsPerWorker
	assert.Equal(t, expectedDocs, totalDocs)
}

// TestDataConsistency tests that data remains consistent across operations
func TestDataConsistency(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.Teardown()

	// Initialize shards
	err := suite.shardManager.InitializeShards()
	require.NoError(t, err)

	// Index initial data
	initialDocs := []mapping.IndexableDocument{
		{
			ID: "consistency_test_1",
			Data: mapping.LogDocument{
				Message:   "Initial message",
				Service:   "consistency-service",
				Timestamp: time.Now(),
			},
		},
	}

	err = suite.shardManager.IndexDocuments(initialDocs)
	require.NoError(t, err)

	// Verify initial data
	query := bleve.NewQueryStringQuery("initial")
	request := bleve.NewSearchRequest(query)
	result1, err := suite.shardManager.Search(request)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result1.Total)

	// Add more data
	additionalDocs := []mapping.IndexableDocument{
		{
			ID: "consistency_test_2",
			Data: mapping.LogDocument{
				Message:   "Additional message",
				Service:   "consistency-service",
				Timestamp: time.Now(),
			},
		},
	}

	err = suite.shardManager.IndexDocuments(additionalDocs)
	require.NoError(t, err)

	// Verify both documents are searchable
	result2, err := suite.shardManager.Search(request)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result2.Total) // Only "initial" should match

	query = bleve.NewQueryStringQuery("consistency-service")
	result3, err := suite.shardManager.Search(request)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result3.Total) // Should find one document

	// Test broader search
	query = bleve.NewQueryStringQuery("message")
	request = bleve.NewSearchRequest(query)
	result4, err := suite.shardManager.Search(request)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), result4.Total) // Should find both documents
}
