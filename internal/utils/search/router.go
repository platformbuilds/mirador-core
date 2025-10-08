package search

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/utils/bleve"
	"github.com/platformbuilds/mirador-core/internal/utils/lucene"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// TraceFilters represents extracted filters for Jaeger HTTP API.
type TraceFilters struct {
	Service     string
	Operation   string
	Tags        map[string]string
	MinDuration string
	MaxDuration string
	Since       string // e.g., "15m" → handler converts to Start/End
	StartExpr   string // e.g., "now-15m" or RFC3339
	EndExpr     string // e.g., "now"
}

// Translator defines the interface for query translation
type Translator interface {
	// TranslateToLogsQL translates a query to LogsQL format
	TranslateToLogsQL(query string) (string, error)

	// TranslateToTraces translates a query to trace filters
	TranslateToTraces(query string) (TraceFilters, error)
}

// LuceneTranslator wraps the lucene package functions to implement the Translator interface
type LuceneTranslator struct{}

// NewLuceneTranslator creates a new Lucene translator
func NewLuceneTranslator() *LuceneTranslator {
	return &LuceneTranslator{}
}

// TranslateToLogsQL translates a query to LogsQL format using Lucene translator
func (t *LuceneTranslator) TranslateToLogsQL(query string) (string, error) {
	result, ok := lucene.Translate(query, lucene.TargetLogsQL)
	if !ok {
		return "", fmt.Errorf("failed to translate query to LogsQL")
	}
	return result, nil
}

// TranslateToTraces translates a query to trace filters using Lucene translator
func (t *LuceneTranslator) TranslateToTraces(query string) (TraceFilters, error) {
	filters, ok := lucene.TranslateTraces(query)
	if !ok {
		return TraceFilters{}, fmt.Errorf("failed to translate query to trace filters")
	}
	return TraceFilters{
		Service:     filters.Service,
		Operation:   filters.Operation,
		Tags:        filters.Tags,
		MinDuration: filters.MinDuration,
		MaxDuration: filters.MaxDuration,
		Since:       filters.Since,
		StartExpr:   filters.StartExpr,
		EndExpr:     filters.EndExpr,
	}, nil
}

// BleveTranslatorWrapper wraps the bleve translator to implement the Translator interface
type BleveTranslatorWrapper struct {
	translator *bleve.Translator
}

func NewBleveTranslator() *BleveTranslatorWrapper {
	return &BleveTranslatorWrapper{
		translator: bleve.NewTranslator(),
	}
}

func (w *BleveTranslatorWrapper) TranslateToLogsQL(query string) (string, error) {
	return w.translator.TranslateToLogsQL(query)
}

func (w *BleveTranslatorWrapper) TranslateToTraces(query string) (TraceFilters, error) {
	filters, err := w.translator.TranslateToTraces(query)
	if err != nil {
		return TraceFilters{}, err
	}
	// Convert bleve.TraceFilters to search.TraceFilters
	return TraceFilters{
		Service:     filters.Service,
		Operation:   filters.Operation,
		Tags:        filters.Tags,
		MinDuration: filters.MinDuration,
		MaxDuration: filters.MaxDuration,
		Since:       filters.Since,
		StartExpr:   filters.StartExpr,
		EndExpr:     filters.EndExpr,
	}, nil
}

// SearchRouter routes queries to the appropriate translator
type SearchRouter struct {
	config      *SearchConfig
	logger      logger.Logger
	translators map[string]Translator
	cache       cache.ValkeyCluster
	cacheTTL    time.Duration
}

// SearchConfig holds configuration for search engines
type SearchConfig struct {
	DefaultEngine string
	EnableBleve   bool
	EnableLucene  bool
	Cache         cache.ValkeyCluster
	CacheTTL      time.Duration
}

// NewSearchRouter creates a new search router
func NewSearchRouter(config *SearchConfig, logger logger.Logger) (*SearchRouter, error) {
	router := &SearchRouter{
		config:      config,
		logger:      logger,
		translators: make(map[string]Translator),
		cache:       config.Cache,
		cacheTTL:    config.CacheTTL,
	}

	// Set default cache TTL if not specified
	if router.cacheTTL == 0 {
		router.cacheTTL = 30 * time.Minute // Default 30 minutes
	}

	// Always register Lucene translator (default)
	luceneTranslator := NewLuceneTranslator()
	router.translators["lucene"] = luceneTranslator

	// Register Bleve translator if enabled
	if config.EnableBleve {
		bleveTranslator := NewBleveTranslator()
		router.translators["bleve"] = bleveTranslator
	}

	return router, nil
}

// GetTranslator returns the appropriate translator for the given engine
func (r *SearchRouter) GetTranslator(engine string) (Translator, error) {
	translator, exists := r.translators[engine]
	if !exists {
		return nil, fmt.Errorf("unsupported search engine: %s", engine)
	}
	return translator, nil
}

// IsEngineSupported checks if the given engine is supported
func (r *SearchRouter) IsEngineSupported(engine string) bool {
	_, exists := r.translators[engine]
	return exists
}

// SupportedEngines returns a list of supported search engines
func (r *SearchRouter) SupportedEngines() []string {
	engines := make([]string, 0, len(r.translators))
	for engine := range r.translators {
		engines = append(engines, engine)
	}
	return engines
}

// GetDefaultEngine returns the default search engine
func (r *SearchRouter) GetDefaultEngine() string {
	return r.config.DefaultEngine
}

// generateCacheKey generates a cache key for query results
func (r *SearchRouter) generateCacheKey(engine, queryType, query string) string {
	hash := md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", engine, queryType, query)))
	return fmt.Sprintf("search_cache:%x", hash)
}

// cacheQueryResult caches a query result
func (r *SearchRouter) cacheQueryResult(ctx context.Context, engine, queryType, query string, result interface{}) {
	if r.cache == nil {
		return // No cache configured
	}

	cacheKey := r.generateCacheKey(engine, queryType, query)
	err := r.cache.CacheQueryResult(ctx, cacheKey, result, r.cacheTTL)
	if err != nil {
		r.logger.Warn("Failed to cache query result", "error", err, "key", cacheKey)
	}
}

// getCachedQueryResult retrieves a cached query result
func (r *SearchRouter) getCachedQueryResult(ctx context.Context, engine, queryType, query string) ([]byte, error) {
	if r.cache == nil {
		return nil, fmt.Errorf("cache not available")
	}

	cacheKey := r.generateCacheKey(engine, queryType, query)
	return r.cache.GetCachedQueryResult(ctx, cacheKey)
}

// TranslateToLogsQLCached translates a query to LogsQL with caching
func (r *SearchRouter) TranslateToLogsQLCached(ctx context.Context, engine, query string) (string, error) {
	// Try to get from cache first
	if cachedResult, err := r.getCachedQueryResult(ctx, engine, "logsql", query); err == nil {
		var result string
		if err := json.Unmarshal(cachedResult, &result); err == nil {
			r.logger.Debug("Cache hit for LogsQL translation", "engine", engine, "query", query)
			return result, nil
		}
	}

	// Cache miss - translate and cache
	translator, err := r.GetTranslator(engine)
	if err != nil {
		return "", err
	}

	result, err := translator.TranslateToLogsQL(query)
	if err != nil {
		return "", err
	}

	// Cache the result
	r.cacheQueryResult(ctx, engine, "logsql", query, result)
	r.logger.Debug("Cached LogsQL translation", "engine", engine, "query", query)

	return result, nil
}

// TranslateToTracesCached translates a query to trace filters with caching
func (r *SearchRouter) TranslateToTracesCached(ctx context.Context, engine, query string) (TraceFilters, error) {
	// Try to get from cache first
	if cachedResult, err := r.getCachedQueryResult(ctx, engine, "traces", query); err == nil {
		var result TraceFilters
		if err := json.Unmarshal(cachedResult, &result); err == nil {
			r.logger.Debug("Cache hit for trace filters translation", "engine", engine, "query", query)
			return result, nil
		}
	}

	// Cache miss - translate and cache
	translator, err := r.GetTranslator(engine)
	if err != nil {
		return TraceFilters{}, err
	}

	result, err := translator.TranslateToTraces(query)
	if err != nil {
		return TraceFilters{}, err
	}

	// Cache the result
	r.cacheQueryResult(ctx, engine, "traces", query, result)
	r.logger.Debug("Cached trace filters translation", "engine", engine, "query", query)

	return result, nil
}

// ABTestResult holds the result of an A/B test comparison
type ABTestResult struct {
	Query         string            `json:"query"`
	LuceneResult  TranslationResult `json:"lucene_result"`
	BleveResult   TranslationResult `json:"bleve_result"`
	Comparison    ABTestComparison  `json:"comparison"`
	ExecutionTime time.Duration     `json:"execution_time"`
}

// TranslationResult holds the result of a single translation
type TranslationResult struct {
	LogsQL   string        `json:"logsql,omitempty"`
	Traces   TraceFilters  `json:"traces,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// ABTestComparison holds the comparison between two translation results
type ABTestComparison struct {
	LogsQLEqual     bool     `json:"logsql_equal"`
	TracesEqual     bool     `json:"traces_equal"`
	Differences     []string `json:"differences"`
	Recommendations []string `json:"recommendations"`
}

// RunABTest runs a query against both Lucene and Bleve engines and compares results
func (r *SearchRouter) RunABTest(ctx context.Context, query string) (*ABTestResult, error) {
	startTime := time.Now()

	result := &ABTestResult{
		Query: query,
	}

	// Test Lucene translation
	luceneStart := time.Now()
	luceneLogsQL, luceneLogsErr := r.TranslateToLogsQLCached(ctx, "lucene", query)
	luceneTraces, _ := r.TranslateToTracesCached(ctx, "lucene", query)
	luceneDuration := time.Since(luceneStart)

	result.LuceneResult = TranslationResult{
		Duration: luceneDuration,
	}
	if luceneLogsErr == nil {
		result.LuceneResult.LogsQL = luceneLogsQL
	} else {
		result.LuceneResult.Error = luceneLogsErr.Error()
	}
	result.LuceneResult.Traces = luceneTraces

	// Test Bleve translation
	bleveStart := time.Now()
	bleveLogsQL, bleveLogsErr := r.TranslateToLogsQLCached(ctx, "bleve", query)
	bleveTraces, _ := r.TranslateToTracesCached(ctx, "bleve", query)
	bleveDuration := time.Since(bleveStart)

	result.BleveResult = TranslationResult{
		Duration: bleveDuration,
	}
	if bleveLogsErr == nil {
		result.BleveResult.LogsQL = bleveLogsQL
	} else {
		result.BleveResult.Error = bleveLogsErr.Error()
	}
	result.BleveResult.Traces = bleveTraces

	// Compare results
	result.Comparison = r.compareTranslations(result.LuceneResult, result.BleveResult)
	result.ExecutionTime = time.Since(startTime)

	r.logger.Info("A/B test completed",
		"query", query,
		"lucene_duration", luceneDuration,
		"bleve_duration", bleveDuration,
		"logsql_equal", result.Comparison.LogsQLEqual,
		"traces_equal", result.Comparison.TracesEqual)

	return result, nil
}

// compareTranslations compares two translation results
func (r *SearchRouter) compareTranslations(lucene, bleve TranslationResult) ABTestComparison {
	comparison := ABTestComparison{
		LogsQLEqual:     true,
		TracesEqual:     true,
		Differences:     []string{},
		Recommendations: []string{},
	}

	// Compare LogsQL results
	if lucene.LogsQL != bleve.LogsQL {
		comparison.LogsQLEqual = false
		comparison.Differences = append(comparison.Differences,
			fmt.Sprintf("LogsQL mismatch: Lucene='%s', Bleve='%s'", lucene.LogsQL, bleve.LogsQL))
	}

	// Compare Traces results
	if !r.compareTraceFilters(lucene.Traces, bleve.Traces) {
		comparison.TracesEqual = false
		comparison.Differences = append(comparison.Differences,
			fmt.Sprintf("Traces mismatch: Lucene=%+v, Bleve=%+v", lucene.Traces, bleve.Traces))
	}

	// Check for errors
	if lucene.Error != "" && bleve.Error == "" {
		comparison.Recommendations = append(comparison.Recommendations,
			"Bleve handled query successfully while Lucene failed")
	} else if lucene.Error == "" && bleve.Error != "" {
		comparison.Recommendations = append(comparison.Recommendations,
			"Lucene handled query successfully while Bleve failed")
	}

	// Performance comparison
	if bleve.Duration < lucene.Duration {
		comparison.Recommendations = append(comparison.Recommendations,
			fmt.Sprintf("Bleve is %.1fx faster than Lucene", float64(lucene.Duration)/float64(bleve.Duration)))
	} else if lucene.Duration < bleve.Duration {
		comparison.Recommendations = append(comparison.Recommendations,
			fmt.Sprintf("Lucene is %.1fx faster than Bleve", float64(bleve.Duration)/float64(lucene.Duration)))
	}

	return comparison
}

// compareTraceFilters compares two TraceFilters structs
func (r *SearchRouter) compareTraceFilters(a, b TraceFilters) bool {
	if a.Service != b.Service || a.Operation != b.Operation ||
		a.MinDuration != b.MinDuration || a.MaxDuration != b.MaxDuration ||
		a.Since != b.Since || a.StartExpr != b.StartExpr || a.EndExpr != b.EndExpr {
		return false
	}

	// Compare tags maps
	if len(a.Tags) != len(b.Tags) {
		return false
	}
	for k, v := range a.Tags {
		if bv, ok := b.Tags[k]; !ok || v != bv {
			return false
		}
	}

	return true
}
