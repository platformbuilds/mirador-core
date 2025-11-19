package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"

	"github.com/platformbuilds/mirador-core/internal/models"
	bleveUtils "github.com/platformbuilds/mirador-core/internal/utils/bleve"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/mapping"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MetricsMetadataIndexer provides functionality to index metrics metadata in Bleve
type MetricsMetadataIndexer interface {
	// SyncMetadata synchronizes metrics metadata from VictoriaMetrics to Bleve
	SyncMetadata(ctx context.Context, request *models.MetricMetadataSyncRequest) (*models.MetricMetadataSyncResult, error)

	// SearchMetrics searches for metrics using Bleve
	SearchMetrics(ctx context.Context, request *models.MetricMetadataSearchRequest) (*models.MetricMetadataSearchResult, error)

	// GetHealthStatus returns the health status of the metadata indexer
	GetHealthStatus(ctx context.Context) (*models.MetricMetadataHealthStatus, error)

	// InvalidateCache invalidates cached metadata
	InvalidateCache(ctx context.Context) error
}

// MetricsMetadataIndexerImpl implements the MetricsMetadataIndexer interface
type MetricsMetadataIndexerImpl struct {
	victoriaMetricsSvc *VictoriaMetricsService
	shardManager       *bleveUtils.ShardManager
	documentMapper     mapping.DocumentMapper
	logger             logger.Logger
}

// NewStubMetricsMetadataIndexer creates a stub implementation for development/testing
func NewStubMetricsMetadataIndexer(logger logger.Logger) MetricsMetadataIndexer {
	return &StubMetricsMetadataIndexer{logger: logger}
}

// StubMetricsMetadataIndexer provides stub implementations for metrics metadata operations
type StubMetricsMetadataIndexer struct {
	logger logger.Logger
}

// SyncMetadata returns a stub sync result
func (s *StubMetricsMetadataIndexer) SyncMetadata(ctx context.Context, request *models.MetricMetadataSyncRequest) (*models.MetricMetadataSyncResult, error) {
	s.logger.Info("StubMetricsMetadataIndexer.SyncMetadata called")

	return &models.MetricMetadataSyncResult{
		MetricsProcessed: 0,
		MetricsAdded:     0,
		MetricsUpdated:   0,
		MetricsRemoved:   0,
		Duration:         0,
		LastSyncTime:     time.Now(),
		Errors:           []string{"Metrics metadata indexing is disabled in this environment"},
	}, nil
}

// SearchMetrics returns a stub search result
func (s *StubMetricsMetadataIndexer) SearchMetrics(ctx context.Context, request *models.MetricMetadataSearchRequest) (*models.MetricMetadataSearchResult, error) {
	s.logger.Info("StubMetricsMetadataIndexer.SearchMetrics called", "query", request.Query)

	return &models.MetricMetadataSearchResult{
		Metrics:    []*models.MetricMetadataDocument{},
		TotalCount: 0,
		QueryTime:  0,
	}, nil
}

// GetHealthStatus returns a stub health status
func (s *StubMetricsMetadataIndexer) GetHealthStatus(ctx context.Context) (*models.MetricMetadataHealthStatus, error) {
	return &models.MetricMetadataHealthStatus{
		IsHealthy:      false,
		LastSyncTime:   time.Now(),
		TotalMetrics:   0,
		ActiveMetrics:  0,
		IndexSizeBytes: 0,
		SyncErrors:     []string{"Metrics metadata indexing is disabled in this environment"},
	}, nil
}

// InvalidateCache returns nil (no-op)
func (s *StubMetricsMetadataIndexer) InvalidateCache(ctx context.Context) error {
	s.logger.Info("StubMetricsMetadataIndexer.InvalidateCache called")
	return nil
}

// NewMetricsMetadataIndexer creates a new metrics metadata indexer
func NewMetricsMetadataIndexer(
	victoriaSvc *VictoriaMetricsService,
	shardManager *bleveUtils.ShardManager,
	documentMapper mapping.DocumentMapper,
	logger logger.Logger,
) MetricsMetadataIndexer {
	return &MetricsMetadataIndexerImpl{
		victoriaMetricsSvc: victoriaSvc,
		shardManager:       shardManager,
		documentMapper:     documentMapper,
		logger:             logger,
	}
}
func (m *MetricsMetadataIndexerImpl) SyncMetadata(ctx context.Context, request *models.MetricMetadataSyncRequest) (*models.MetricMetadataSyncResult, error) {
	start := time.Now()
	result := &models.MetricMetadataSyncResult{}

	// Check if ShardManager is available
	if m.shardManager == nil {
		result.Errors = append(result.Errors, "Bleve ShardManager not available")
		return result, fmt.Errorf("Bleve ShardManager not configured")
	}

	m.logger.Info("Starting metrics metadata sync", "forceFullSync", request.ForceFullSync)

	// Extract metrics metadata from VictoriaMetrics
	metadataDocs, err := m.extractMetricsMetadata(ctx, request)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to extract metadata: %v", err))
		return result, fmt.Errorf("failed to extract metrics metadata: %w", err)
	}

	result.MetricsProcessed = len(metadataDocs)

	// Convert to indexable documents
	indexableDocs := make([]mapping.IndexableDocument, len(metadataDocs))
	for i, doc := range metadataDocs {
		indexableDocs[i] = mapping.IndexableDocument{
			ID:   doc.ID,
			Data: doc,
		}
	}

	// Index documents in Bleve
	if err := m.shardManager.IndexDocuments(indexableDocs); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to index documents: %v", err))
		return result, fmt.Errorf("failed to index metrics metadata: %w", err)
	}

	result.MetricsAdded = len(metadataDocs) // For now, assume all are new
	result.Duration = time.Since(start).Milliseconds()
	result.LastSyncTime = time.Now()

	m.logger.Info("Completed metrics metadata sync",
		"metricsProcessed", result.MetricsProcessed,
		"duration", result.Duration)

	return result, nil
}

// extractMetricsMetadata extracts metrics metadata from VictoriaMetrics
func (m *MetricsMetadataIndexerImpl) extractMetricsMetadata(ctx context.Context, request *models.MetricMetadataSyncRequest) ([]*models.MetricMetadataDocument, error) {
	var allMetadata []*models.MetricMetadataDocument

	// Get all series to extract metric names and labels
	seriesRequest := &models.SeriesRequest{
		Start: request.TimeRange.Start.Format(time.RFC3339),
		End:   request.TimeRange.End.Format(time.RFC3339),
		// Use a match that selects all metrics for metadata extraction
		Match: []string{"{__name__=~\".+\"}"},
	}

	series, err := m.victoriaMetricsSvc.GetSeries(ctx, seriesRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to get series from VictoriaMetrics: %w", err)
	}

	// Group series by metric name
	metricsMap := make(map[string]*models.MetricMetadataDocument)

	for _, serie := range series {
		metricName, ok := serie["__name__"]
		if !ok {
			continue
		}

		nameStr := metricName // Already a string

		// Get or create metadata document for this metric
		doc, exists := metricsMap[nameStr]
		if !exists {
			doc = models.NewMetricMetadataDocument(nameStr)
			metricsMap[nameStr] = doc
		}

		// Extract labels from this series
		labels := make(map[string][]string)
		for key, value := range serie {
			if key == "__name__" {
				continue
			}
			labels[key] = append(labels[key], value) // Already a string
		}

		// Update document with labels
		doc.UpdateLabels(labels)
		doc.MarkSeen()
	}

	// Convert map to slice
	for _, doc := range metricsMap {
		allMetadata = append(allMetadata, doc)
	}

	m.logger.Info("Extracted metrics metadata",
		"totalSeries", len(series),
		"uniqueMetrics", len(allMetadata))

	return allMetadata, nil
}

// SearchMetrics searches for metrics using Bleve
func (m *MetricsMetadataIndexerImpl) SearchMetrics(ctx context.Context, request *models.MetricMetadataSearchRequest) (*models.MetricMetadataSearchResult, error) {
	start := time.Now()

	// Check if ShardManager is available
	if m.shardManager == nil {
		return nil, fmt.Errorf("Bleve ShardManager not configured")
	}

	// Build Bleve search request
	query := bleve.NewQueryStringQuery(request.Query)
	searchRequest := bleve.NewSearchRequest(query)

	// Set pagination
	searchRequest.Size = request.Limit
	searchRequest.From = request.Offset

	// Set sorting (by relevance by default)
	searchRequest.SortBy([]string{"-_score", "metric_name"})

	// Execute search
	searchResult, err := m.shardManager.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("Bleve search failed: %w", err)
	}

	// Convert results
	metrics := make([]*models.MetricMetadataDocument, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		doc := &models.MetricMetadataDocument{
			ID:         hit.ID,
			MetricName: extractMetricNameFromID(hit.ID),
		}
		metrics = append(metrics, doc)
	}

	result := &models.MetricMetadataSearchResult{
		Metrics:    metrics,
		TotalCount: int(searchResult.Total),
		QueryTime:  time.Since(start).Milliseconds(),
	}

	m.logger.Info("Metrics search completed",
		"query", request.Query,
		"results", len(metrics),
		"total", searchResult.Total,
		"queryTime", result.QueryTime)

	return result, nil
}

// extractMetricNameFromID extracts the metric name from a document ID
func extractMetricNameFromID(id string) string {
	parts := strings.Split(id, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return id
}

// GetHealthStatus returns the health status of the metadata indexer
func (m *MetricsMetadataIndexerImpl) GetHealthStatus(ctx context.Context) (*models.MetricMetadataHealthStatus, error) {
	// Check VictoriaMetrics health
	vmHealthy := true
	if err := m.victoriaMetricsSvc.HealthCheck(ctx); err != nil {
		vmHealthy = false
		m.logger.Warn("VictoriaMetrics health check failed", "error", err)
	}

	// Get shard statistics if available
	var totalMetrics, activeMetrics int64
	var indexSizeBytes int64
	if m.shardManager != nil {
		shardStats := m.shardManager.GetShardStats()

		// Calculate total metrics and active metrics
		for _, stat := range shardStats {
			if stats, ok := stat.(map[string]interface{}); ok {
				if docCount, ok := stats["docCount"].(uint64); ok {
					totalMetrics += int64(docCount)
					activeMetrics += int64(docCount) // Assume all are active for now
				}
			}
		}
	}

	// Determine overall health
	isHealthy := vmHealthy && (m.shardManager != nil)
	var errors []string
	if !vmHealthy {
		errors = append(errors, "VictoriaMetrics is unhealthy")
	}
	if m.shardManager == nil {
		errors = append(errors, "Bleve ShardManager not configured")
	}

	return &models.MetricMetadataHealthStatus{
		IsHealthy:      isHealthy,
		LastSyncTime:   time.Now(), // This should be tracked properly
		TotalMetrics:   totalMetrics,
		ActiveMetrics:  activeMetrics,
		IndexSizeBytes: indexSizeBytes,
		SyncErrors:     errors,
	}, nil
}

// InvalidateCache invalidates cached metadata
func (m *MetricsMetadataIndexerImpl) InvalidateCache(ctx context.Context) error {
	// For now, just log the operation
	// TODO: Implement actual cache invalidation
	m.logger.Info("Cache invalidation requested")
	return nil
}
