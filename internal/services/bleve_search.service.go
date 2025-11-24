package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"

	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/mapping"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// BleveSearchService provides full-text search capabilities using Bleve
type BleveSearchService struct {
	logsIndex   bleve.Index
	tracesIndex bleve.Index
	mapper      mapping.DocumentMapper
	logger      logging.Logger
	dataPath    string
	cache       cache.ValkeyCluster
	mu          sync.RWMutex
}

// BleveSearchConfig holds configuration for Bleve search service
type BleveSearchConfig struct {
	DataPath string
	Cache    cache.ValkeyCluster
}

// NewBleveSearchService creates a new Bleve search service
func NewBleveSearchService(config BleveSearchConfig, logger corelogger.Logger) (*BleveSearchService, error) {
	// Create document mapper using the core logger (mapper expects pkg/logger.Logger)
	mapper := mapping.NewBleveDocumentMapper(logger)

	service := &BleveSearchService{
		mapper:   mapper,
		logger:   logging.FromCoreLogger(logger),
		dataPath: config.DataPath,
		cache:    config.Cache,
	}

	service.logger.Info("BleveSearchService initialized", "data_path", config.DataPath)

	return service, nil
}

// Start initializes and starts the Bleve search service
func (s *BleveSearchService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure data path exists
	if err := os.MkdirAll(s.dataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data path: %w", err)
	}

	// Initialize logs index
	logsIndexPath := filepath.Join(s.dataPath, "logs.bleve")
	var err error

	// Try to open existing index or create new one
	s.logsIndex, err = bleve.Open(logsIndexPath)
	if err == bleve.ErrorIndexPathDoesNotExist {
		// Create new index
		indexMapping := bleve.NewIndexMapping()
		s.logsIndex, err = bleve.New(logsIndexPath, indexMapping)
		if err != nil {
			return fmt.Errorf("failed to create logs index: %w", err)
		}
		s.logger.Info("Created new logs index", "path", logsIndexPath)
	} else if err != nil {
		return fmt.Errorf("failed to open logs index: %w", err)
	} else {
		s.logger.Info("Opened existing logs index", "path", logsIndexPath)
	}

	// Initialize traces index
	tracesIndexPath := filepath.Join(s.dataPath, "traces.bleve")
	s.tracesIndex, err = bleve.Open(tracesIndexPath)
	if err == bleve.ErrorIndexPathDoesNotExist {
		// Create new index
		indexMapping := bleve.NewIndexMapping()
		s.tracesIndex, err = bleve.New(tracesIndexPath, indexMapping)
		if err != nil {
			return fmt.Errorf("failed to create traces index: %w", err)
		}
		s.logger.Info("Created new traces index", "path", tracesIndexPath)
	} else if err != nil {
		return fmt.Errorf("failed to open traces index: %w", err)
	} else {
		s.logger.Info("Opened existing traces index", "path", tracesIndexPath)
	}

	s.logger.Info("BleveSearchService started successfully")
	return nil
}

// Stop gracefully shuts down the Bleve search service
func (s *BleveSearchService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Stopping BleveSearchService")

	var errs []error

	if s.logsIndex != nil {
		if err := s.logsIndex.Close(); err != nil {
			s.logger.Error("Error closing logs index", "error", err)
			errs = append(errs, err)
		}
	}

	if s.tracesIndex != nil {
		if err := s.tracesIndex.Close(); err != nil {
			s.logger.Error("Error closing traces index", "error", err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	s.logger.Info("BleveSearchService stopped")
	return nil
}

// SearchLogs performs full-text search on log documents
func (s *BleveSearchService) SearchLogs(ctx context.Context, req *models.LogsSearchRequest) (*models.LogsSearchResponse, error) {
	start := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.logsIndex == nil {
		return nil, fmt.Errorf("logs index not initialized")
	}

	s.logger.Debug("Searching logs", "query", req.Query)

	// Build Bleve query
	bleveQuery := query.NewQueryStringQuery(req.Query)

	// Create search request
	searchRequest := bleve.NewSearchRequestOptions(bleveQuery, req.Limit, 0, false)

	// Execute search
	searchResult, err := s.logsIndex.SearchInContext(ctx, searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert results to log documents
	logs := make([]map[string]any, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		// Use hit fields directly rather than retrieving full document
		logMap := make(map[string]any)
		logMap["_id"] = hit.ID
		logMap["_score"] = hit.Score

		// Add stored fields from hit
		for key, value := range hit.Fields {
			logMap[key] = value
		}

		logs = append(logs, logMap)
	}

	return &models.LogsSearchResponse{
		Rows: logs,
		Stats: map[string]any{
			"total": searchResult.Total,
			"took":  time.Since(start).Milliseconds(),
		},
	}, nil
}

// SearchTraces performs full-text search on trace documents
func (s *BleveSearchService) SearchTraces(ctx context.Context, req *models.TraceSearchRequest) (*models.TraceSearchResult, error) {
	start := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.tracesIndex == nil {
		return nil, fmt.Errorf("traces index not initialized")
	}

	s.logger.Debug("Searching traces", "query", req.Query)

	// Build Bleve query
	bleveQuery := query.NewQueryStringQuery(req.Query)

	// Create search request with limit
	limit := req.Limit
	if limit == 0 {
		limit = 100 // default limit
	}
	searchRequest := bleve.NewSearchRequestOptions(bleveQuery, limit, 0, false)

	// Execute search
	searchResult, err := s.tracesIndex.SearchInContext(ctx, searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert results to trace documents
	traces := make([]map[string]interface{}, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		// Use hit fields directly rather than retrieving full document
		traceMap := make(map[string]interface{})
		traceMap["_id"] = hit.ID
		traceMap["_score"] = hit.Score

		// Add stored fields from hit
		for key, value := range hit.Fields {
			traceMap[key] = value
		}

		traces = append(traces, traceMap)
	}

	return &models.TraceSearchResult{
		Traces:     traces,
		Total:      int(searchResult.Total),
		SearchTime: time.Since(start).Milliseconds(),
	}, nil
}

// IndexLog indexes a log document for search
func (s *BleveSearchService) IndexLog(ctx context.Context, log map[string]any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.logsIndex == nil {
		return fmt.Errorf("logs index not initialized")
	}

	// Map log to indexable document
	docs, err := s.mapper.MapLogs([]map[string]any{log})
	if err != nil {
		return fmt.Errorf("failed to map log: %w", err)
	}

	if len(docs) == 0 {
		return fmt.Errorf("no documents produced from log")
	}

	// Index document
	doc := docs[0]
	if err := s.logsIndex.Index(doc.ID, doc.Data); err != nil {
		return fmt.Errorf("failed to index log: %w", err)
	}

	return nil
}

// IndexTrace indexes a trace document for search
func (s *BleveSearchService) IndexTrace(ctx context.Context, trace map[string]interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.tracesIndex == nil {
		return fmt.Errorf("traces index not initialized")
	}

	// Map trace to indexable document
	docs, err := s.mapper.MapTraces([]map[string]interface{}{trace})
	if err != nil {
		return fmt.Errorf("failed to map trace: %w", err)
	}

	if len(docs) == 0 {
		return fmt.Errorf("no documents produced from trace")
	}

	// Index document
	doc := docs[0]
	if err := s.tracesIndex.Index(doc.ID, doc.Data); err != nil {
		return fmt.Errorf("failed to index trace: %w", err)
	}

	return nil
}

// BatchIndexLogs indexes multiple log documents in a batch
func (s *BleveSearchService) BatchIndexLogs(ctx context.Context, logs []map[string]any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.logsIndex == nil {
		return fmt.Errorf("logs index not initialized")
	}

	// Map logs to indexable documents
	docs, err := s.mapper.MapLogs(logs)
	if err != nil {
		return fmt.Errorf("failed to map logs: %w", err)
	}

	// Create batch
	batch := s.logsIndex.NewBatch()
	for _, doc := range docs {
		if err := batch.Index(doc.ID, doc.Data); err != nil {
			return fmt.Errorf("failed to add document to batch: %w", err)
		}
	}

	// Execute batch
	if err := s.logsIndex.Batch(batch); err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	s.logger.Debug("Batch indexed logs", "count", len(docs))
	return nil
}

// BatchIndexTraces indexes multiple trace documents in a batch
func (s *BleveSearchService) BatchIndexTraces(ctx context.Context, traces []map[string]interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.tracesIndex == nil {
		return fmt.Errorf("traces index not initialized")
	}

	// Map traces to indexable documents
	docs, err := s.mapper.MapTraces(traces)
	if err != nil {
		return fmt.Errorf("failed to map traces: %w", err)
	}

	// Create batch
	batch := s.tracesIndex.NewBatch()
	for _, doc := range docs {
		if err := batch.Index(doc.ID, doc.Data); err != nil {
			return fmt.Errorf("failed to add document to batch: %w", err)
		}
	}

	// Execute batch
	if err := s.tracesIndex.Batch(batch); err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	s.logger.Debug("Batch indexed traces", "count", len(docs))
	return nil
}

// DeleteLog deletes a log document from the index
func (s *BleveSearchService) DeleteLog(ctx context.Context, logID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.logsIndex == nil {
		return fmt.Errorf("logs index not initialized")
	}

	return s.logsIndex.Delete(logID)
}

// DeleteTrace deletes a trace document from the index
func (s *BleveSearchService) DeleteTrace(ctx context.Context, traceID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.tracesIndex == nil {
		return fmt.Errorf("traces index not initialized")
	}

	return s.tracesIndex.Delete(traceID)
}

// HealthCheck checks the health of the Bleve search service
func (s *BleveSearchService) HealthCheck(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.logsIndex == nil {
		return fmt.Errorf("logs index not initialized")
	}

	if s.tracesIndex == nil {
		return fmt.Errorf("traces index not initialized")
	}

	// Check if indexes are accessible by getting doc count
	_, err := s.logsIndex.DocCount()
	if err != nil {
		return fmt.Errorf("logs index health check failed: %w", err)
	}

	_, err = s.tracesIndex.DocCount()
	if err != nil {
		return fmt.Errorf("traces index health check failed: %w", err)
	}

	return nil
}

// GetStats returns statistics about the Bleve search service
func (s *BleveSearchService) GetStats(ctx context.Context) (*BleveSearchStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &BleveSearchStats{}

	if s.logsIndex != nil {
		logsCount, err := s.logsIndex.DocCount()
		if err == nil {
			stats.LogsDocumentCount = logsCount
		}
	}

	if s.tracesIndex != nil {
		tracesCount, err := s.tracesIndex.DocCount()
		if err == nil {
			stats.TracesDocumentCount = tracesCount
		}
	}

	stats.TotalDocuments = stats.LogsDocumentCount + stats.TracesDocumentCount

	return stats, nil
}

// BleveSearchStats holds statistics about the search service
type BleveSearchStats struct {
	TotalDocuments      uint64 `json:"total_documents"`
	LogsDocumentCount   uint64 `json:"logs_document_count"`
	TracesDocumentCount uint64 `json:"traces_document_count"`
}
