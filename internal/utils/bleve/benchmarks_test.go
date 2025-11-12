package bleve

import (
	"fmt"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"

	"github.com/platformbuilds/mirador-core/internal/utils/bleve/mapping"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/storage"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Benchmark data structures
type BenchmarkDocument struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Service   string                 `json:"service"`
	Tags      map[string]string      `json:"tags"`
	Data      map[string]interface{} `json:"data"`
}

type BenchmarkResult struct {
	Operation       string
	Engine          string
	Duration        time.Duration
	DocsProcessed   int64
	QueriesExecuted int64
	AvgQueryTime    time.Duration
	MemoryUsage     int64
	DiskUsage       int64
}

// generateBenchmarkDocuments creates test documents for benchmarking
func generateBenchmarkDocuments(count int) []mapping.IndexableDocument {
	docs := make([]mapping.IndexableDocument, count)
	levels := []string{"info", "warn", "error", "debug"}
	services := []string{"api-gateway", "user-service", "order-service", "payment-service", "notification-service"}

	for i := 0; i < count; i++ {
		doc := &BenchmarkDocument{
			ID:        fmt.Sprintf("doc-%d", i),
			Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
			Level:     levels[i%len(levels)],
			Message:   fmt.Sprintf("This is a test log message number %d with some additional context", i),
			Service:   services[i%len(services)],
			Tags: map[string]string{
				"env":     "production",
				"version": "1.2.3",
				"region":  "us-east-1",
				"cluster": "main",
			},
			Data: map[string]interface{}{
				"user_id":       i % 1000,
				"request_id":    fmt.Sprintf("req-%d", i),
				"response_time": i % 5000,
				"status_code":   200 + (i % 5),
			},
		}
		docs[i] = mapping.IndexableDocument{
			ID:   doc.ID,
			Data: doc,
		}
	}

	return docs
}

// BenchmarkBleveIndexing benchmarks document indexing performance
func BenchmarkBleveIndexing(b *testing.B) {
	// Setup
	logger := logger.New("info")
	storage := storage.NewTieredStore(nil, 1000, time.Hour, logger)
	metadata := &mockMetadataStore{}
	mapper := mapping.NewBleveDocumentMapper(logger)
	tempDir := b.TempDir()
	shardManager := NewShardManager(3, storage, metadata, mapper, logger, tempDir)

	// Initialize shards and index documents
	err := shardManager.InitializeShards("bench-tenant")
	if err != nil {
		b.Fatalf("Failed to initialize shards: %v", err)
	}

	docs := generateBenchmarkDocuments(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := shardManager.IndexDocuments(docs, "bench-tenant")
		if err != nil {
			b.Fatalf("Failed to index documents: %v", err)
		}
	}
}

// BenchmarkBleveSearch benchmarks Bleve search performance
func BenchmarkBleveSearch(b *testing.B) {
	// Setup
	logger := logger.New("info")
	storage := storage.NewTieredStore(nil, 1000, time.Hour, logger)
	metadata := &mockMetadataStore{}
	mapper := mapping.NewBleveDocumentMapper(logger)
	tempDir := b.TempDir()
	shardManager := NewShardManager(3, storage, metadata, mapper, logger, tempDir)

	// Initialize shards and index documents
	err := shardManager.InitializeShards("bench-tenant")
	if err != nil {
		b.Fatalf("Failed to initialize shards: %v", err)
	}

	docs := generateBenchmarkDocuments(10000)
	err = shardManager.IndexDocuments(docs, "bench-tenant")
	if err != nil {
		b.Fatalf("Failed to index documents: %v", err)
	}

	// Prepare search requests
	searchRequests := []*bleve.SearchRequest{
		bleve.NewSearchRequest(bleve.NewQueryStringQuery("error")),
		bleve.NewSearchRequest(bleve.NewQueryStringQuery("service:api-gateway")),
		bleve.NewSearchRequest(bleve.NewQueryStringQuery("level:info AND service:user-service")),
		bleve.NewSearchRequest(bleve.NewQueryStringQuery("response_time:>1000")),
		bleve.NewSearchRequest(bleve.NewQueryStringQuery("user_id:123")),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		request := searchRequests[i%len(searchRequests)]
		_, err := shardManager.Search(request, "bench-tenant")
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// BenchmarkBleveConcurrentIndexing benchmarks concurrent indexing performance
func BenchmarkBleveConcurrentIndexing(b *testing.B) {
	// Setup
	logger := logger.New("info")
	storage := storage.NewTieredStore(nil, 1000, time.Hour, logger)
	metadata := &mockMetadataStore{}
	mapper := mapping.NewBleveDocumentMapper(logger)
	tempDir := b.TempDir()
	shardManager := NewShardManager(3, storage, metadata, mapper, logger, tempDir)

	// Initialize shards
	err := shardManager.InitializeShards("bench-tenant")
	if err != nil {
		b.Fatalf("Failed to initialize shards: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		localDocs := generateBenchmarkDocuments(100)
		docIndex := 0

		for pb.Next() {
			// Create unique document for this iteration
			doc := localDocs[docIndex%len(localDocs)]
			uniqueDoc := mapping.IndexableDocument{
				ID:   fmt.Sprintf("%s-%d", doc.ID, docIndex),
				Data: doc.Data,
			}

			err := shardManager.IndexDocuments([]mapping.IndexableDocument{uniqueDoc}, "bench-tenant")
			if err != nil {
				b.Fatalf("Failed to index document: %v", err)
			}
			docIndex++
		}
	})
}

// BenchmarkBleveMemoryUsage benchmarks memory usage patterns
func BenchmarkBleveMemoryUsage(b *testing.B) {
	logger := logger.New("info")

	b.Run("SmallIndex", func(b *testing.B) {
		storage := storage.NewTieredStore(nil, 100, time.Hour, logger)
		metadata := &mockMetadataStore{}
		mapper := mapping.NewBleveDocumentMapper(logger)
		tempDir := b.TempDir()
		shardManager := NewShardManager(1, storage, metadata, mapper, logger, tempDir)

		err := shardManager.InitializeShards("bench-tenant")
		if err != nil {
			b.Fatalf("Failed to initialize shards: %v", err)
		}

		docs := generateBenchmarkDocuments(100)
		for i := 0; i < b.N; i++ {
			err := shardManager.IndexDocuments(docs, "bench-tenant")
			if err != nil {
				b.Fatalf("Failed to index documents: %v", err)
			}
		}
	})

	b.Run("LargeIndex", func(b *testing.B) {
		storage := storage.NewTieredStore(nil, 1000, time.Hour, logger)
		metadata := &mockMetadataStore{}
		mapper := mapping.NewBleveDocumentMapper(logger)
		tempDir := b.TempDir()
		shardManager := NewShardManager(3, storage, metadata, mapper, logger, tempDir)

		err := shardManager.InitializeShards("bench-tenant")
		if err != nil {
			b.Fatalf("Failed to initialize shards: %v", err)
		}

		docs := generateBenchmarkDocuments(1000)
		for i := 0; i < b.N; i++ {
			err := shardManager.IndexDocuments(docs, "bench-tenant")
			if err != nil {
				b.Fatalf("Failed to index documents: %v", err)
			}
		}
	})
}

// BenchmarkBleveQueryComplexity benchmarks different query complexity levels
func BenchmarkBleveQueryComplexity(b *testing.B) {
	// Setup
	logger := logger.New("info")
	storage := storage.NewTieredStore(nil, 1000, time.Hour, logger)
	metadata := &mockMetadataStore{}
	mapper := mapping.NewBleveDocumentMapper(logger)
	tempDir := b.TempDir()
	shardManager := NewShardManager(3, storage, metadata, mapper, logger, tempDir)

	// Initialize shards and index documents
	err := shardManager.InitializeShards("bench-tenant")
	if err != nil {
		b.Fatalf("Failed to initialize shards: %v", err)
	}

	docs := generateBenchmarkDocuments(50000) // Large dataset for meaningful benchmarks
	err = shardManager.IndexDocuments(docs, "bench-tenant")
	if err != nil {
		b.Fatalf("Failed to index documents: %v", err)
	}

	queries := map[string]string{
		"SimpleTerm":    "error",
		"FieldQuery":    "level:error",
		"BooleanQuery":  "level:error AND service:api-gateway",
		"RangeQuery":    "response_time:>2000",
		"ComplexQuery":  "level:(info OR warn) AND service:(user-service OR order-service) AND response_time:[1000 TO 3000]",
		"WildcardQuery": "message:test*",
		"PhraseQuery":   `"test log message"`,
	}

	for name, query := range queries {
		b.Run(name, func(b *testing.B) {
			request := bleve.NewSearchRequest(bleve.NewQueryStringQuery(query))

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := shardManager.Search(request, "bench-tenant")
				if err != nil {
					b.Fatalf("Search failed for query %s: %v", query, err)
				}
			}
		})
	}
}
