package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"

	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	bleveUtils "github.com/platformbuilds/mirador-core/internal/utils/bleve"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/mapping"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
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
	logger             logging.Logger
}

// NewStubMetricsMetadataIndexer creates a stub implementation for development/testing
func NewStubMetricsMetadataIndexer(logger corelogger.Logger) MetricsMetadataIndexer {
	return &StubMetricsMetadataIndexer{logger: logging.FromCoreLogger(logger)}
}

// StubMetricsMetadataIndexer provides stub implementations for metrics metadata operations
type StubMetricsMetadataIndexer struct {
	logger logging.Logger
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
	logger corelogger.Logger,
) MetricsMetadataIndexer {
	return &MetricsMetadataIndexerImpl{
		victoriaMetricsSvc: victoriaSvc,
		shardManager:       shardManager,
		documentMapper:     documentMapper,
		logger:             logging.FromCoreLogger(logger),
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
	// Instead of fetching all series (which can exceed VM limits), fetch
	// metric names via the label values API for __name__, then for each
	// metric fetch label names and a small sample of label values. This
	// reduces the number of series scanned and avoids VM 30k+ unique
	// timeseries limits.

	// Determine per-request limits
	batchLimit := request.BatchSize
	if batchLimit <= 0 {
		batchLimit = 10000 // reasonable default cap
	}

	// Fetch metric names using label values for __name__
	nameReq := &models.LabelValuesRequest{
		Label: "__name__",
		Start: request.TimeRange.Start.Format(time.RFC3339),
		End:   request.TimeRange.End.Format(time.RFC3339),
		Limit: batchLimit,
	}

	metricNames, err := m.victoriaMetricsSvc.GetLabelValues(ctx, nameReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric names from VictoriaMetrics: %w", err)
	}

	m.logger.Info("Fetched metric names for metadata sync", "count", len(metricNames))

	metricsMap := make(map[string]*models.MetricMetadataDocument)

	// Process metric names in batches to avoid overwhelming VM or local resources
	for i := 0; i < len(metricNames); i += batchLimit {
		end := i + batchLimit
		if end > len(metricNames) {
			end = len(metricNames)
		}
		batch := metricNames[i:end]

		for _, nameStr := range batch {
			if nameStr == "" {
				continue
			}
			// Create or get doc
			doc, exists := metricsMap[nameStr]
			if !exists {
				doc = models.NewMetricMetadataDocument(nameStr)
				metricsMap[nameStr] = doc
			}

			// Get label names for this metric by querying series filtered to the metric
			match := fmt.Sprintf("{__name__=\"%s\"}", nameStr)
			labelsReq := &models.LabelsRequest{
				Start: request.TimeRange.Start.Format(time.RFC3339),
				End:   request.TimeRange.End.Format(time.RFC3339),
				Match: []string{match},
			}
			labelNames, err := m.victoriaMetricsSvc.GetLabels(ctx, labelsReq)
			if err != nil {
				// Log and continue; don't fail the whole sync for a single metric
				m.logger.Warn("Failed to get labels for metric", "metric", nameStr, "error", err)
				doc.MarkSeen()
				continue
			}

			// For each label, fetch a small set of values to populate metadata (limit values)
			labelsMap := make(map[string][]string)
			for _, label := range labelNames {
				if label == "__name__" {
					continue
				}
				lvReq := &models.LabelValuesRequest{
					Label: label,
					Start: request.TimeRange.Start.Format(time.RFC3339),
					End:   request.TimeRange.End.Format(time.RFC3339),
					Match: []string{match},
					Limit: 50, // sample up to 50 values per label
				}
				vals, err := m.victoriaMetricsSvc.GetLabelValues(ctx, lvReq)
				if err != nil {
					m.logger.Warn("Failed to get label values", "metric", nameStr, "label", label, "error", err)
					continue
				}
				labelsMap[label] = vals
			}

			doc.UpdateLabels(labelsMap)
			doc.MarkSeen()
		}
	}

	// Convert map to slice
	for _, doc := range metricsMap {
		allMetadata = append(allMetadata, doc)
	}

	m.logger.Info("Extracted metrics metadata",
		"metricNames", len(metricNames),
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
