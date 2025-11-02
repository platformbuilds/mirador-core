package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/tracing"
	"github.com/platformbuilds/mirador-core/internal/utils"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// UnifiedQueryEngine provides a unified interface for querying across multiple observability engines
type UnifiedQueryEngine interface {
	// ExecuteQuery executes a unified query across the appropriate engines
	ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

	// ExecuteCorrelationQuery executes a correlation query across multiple engines
	ExecuteCorrelationQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

	// ExecuteUQLQuery executes a UQL query by parsing, optimizing, translating, and executing it
	ExecuteUQLQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

	// GetQueryMetadata returns metadata about supported query types and capabilities
	GetQueryMetadata(ctx context.Context) (*models.QueryMetadata, error)

	// HealthCheck checks the health of all underlying engines
	HealthCheck(ctx context.Context) (*models.EngineHealthStatus, error)

	// InvalidateCache invalidates cached results for a query pattern
	InvalidateCache(ctx context.Context, queryPattern string) error
}

// QueryMetadata contains information about supported query capabilities
type QueryMetadata struct {
	SupportedEngines  []models.QueryType            `json:"supported_engines"`
	QueryCapabilities map[models.QueryType][]string `json:"query_capabilities"`
	CacheCapabilities CacheCapabilities             `json:"cache_capabilities"`
}

// CacheCapabilities describes caching capabilities
type CacheCapabilities struct {
	Supported  bool          `json:"supported"`
	DefaultTTL time.Duration `json:"default_ttl"`
	MaxTTL     time.Duration `json:"max_ttl"`
}

// EngineHealthStatus represents the health status of all engines
type EngineHealthStatus struct {
	OverallHealth string                      `json:"overall_health"`
	EngineHealth  map[models.QueryType]string `json:"engine_health"`
	LastChecked   time.Time                   `json:"last_checked"`
}

// QueryRouter analyzes query patterns and routes to optimal engines
type QueryRouter struct {
	logger logger.Logger
}

// NewQueryRouter creates a new query router
func NewQueryRouter(logger logger.Logger) *QueryRouter {
	return &QueryRouter{
		logger: logger,
	}
}

// RouteQuery analyzes the query and determines the optimal routing
func (r *QueryRouter) RouteQuery(query *models.UnifiedQuery) (models.QueryType, string, error) {
	// If query type is explicitly set and not empty, use it
	if query.Type != "" {
		return query.Type, "", nil
	}

	// Analyze query pattern to determine optimal engine
	queryType, reason := r.analyzeQueryPattern(query.Query)

	r.logger.Debug("Query routed",
		"query", query.Query,
		"routed_to", queryType,
		"reason", reason)

	return queryType, reason, nil
}

// analyzeQueryPattern analyzes the query string to determine the best engine
func (r *QueryRouter) analyzeQueryPattern(query string) (models.QueryType, string) {
	if query == "" {
		return models.QueryTypeLogs, "empty query defaults to logs"
	}

	query = strings.ToLower(strings.TrimSpace(query))

	// Check for UQL SELECT queries
	if r.isUQLQuery(query) {
		return models.QueryTypeMetrics, "detected UQL query syntax" // Default to metrics, will be handled by ExecuteUQLQuery
	}

	// Check for metrics patterns
	if r.isMetricsQuery(query) {
		return models.QueryTypeMetrics, "contains metrics patterns (rate, increase, histogram, etc.)"
	}

	// Check for traces patterns
	if r.isTracesQuery(query) {
		return models.QueryTypeTraces, "contains traces patterns (service, operation, span, trace)"
	}

	// Check for search/text patterns (route to Bleve for search)
	if r.isSearchQuery(query) {
		// For now, route search queries to logs since Bleve integration isn't complete
		// TODO: Route to Bleve search engine when available
		return models.QueryTypeLogs, "contains search patterns, routing to logs (Bleve integration pending)"
	}

	// Default to logs for general queries
	return models.QueryTypeLogs, "general query pattern, defaulting to logs"
}

// isMetricsQuery checks if query contains metrics-specific patterns
func (r *QueryRouter) isMetricsQuery(query string) bool {
	metricsPatterns := []string{
		"rate(", "increase(", "histogram", "summary", "counter",
		"gauge", "avg(", "sum(", "min(", "max(", "count(",
		"quantile", "percentile", "by ", "without ",
		"up", "scrape", "cpu", "memory", "disk", "network",
		"latency", "duration", "requests", "errors", "success",
		"http_", "prometheus", "node_", "process_",
	}

	for _, pattern := range metricsPatterns {
		if strings.Contains(query, pattern) {
			return true
		}
	}

	return false
}

// isTracesQuery checks if query contains traces-specific patterns
func (r *QueryRouter) isTracesQuery(query string) bool {
	tracesPatterns := []string{
		"service:", "operation:", "span", "trace", "jaeger",
		"zipkin", "opentracing", "duration:", "tags:",
		"status:", "error", "http.status_code", "db.statement",
		"rpc.method", "messaging.operation",
	}

	for _, pattern := range tracesPatterns {
		if strings.Contains(query, pattern) {
			return true
		}
	}

	return false
}

// isSearchQuery checks if query contains search/text patterns
func (r *QueryRouter) isSearchQuery(query string) bool {
	searchPatterns := []string{
		"error", "warn", "info", "debug", "fatal",
		"exception", "stacktrace", "log", "message",
		"field:", "text:", "content:", "body:",
		"level:", "timestamp:", "host:", "source:",
		"kubernetes", "docker", "container", "pod",
		"namespace", "deployment", "service", "ingress",
	}

	// Count search patterns
	searchScore := 0
	for _, pattern := range searchPatterns {
		if strings.Contains(query, pattern) {
			searchScore++
		}
	}

	// If we have multiple search patterns or structured search syntax, it's likely a search query
	if searchScore >= 2 || strings.Contains(query, ":") || strings.Contains(query, " AND ") || strings.Contains(query, " OR ") {
		return true
	}

	return false
}

// isUQLQuery checks if query contains UQL syntax
func (r *QueryRouter) isUQLQuery(query string) bool {
	// Check for SELECT statements
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") {
		return true
	}

	// Check for other UQL keywords
	uqlKeywords := []string{"FROM", "WHERE", "GROUP BY", "ORDER BY", "LIMIT", "HAVING"}
	for _, keyword := range uqlKeywords {
		if strings.Contains(strings.ToUpper(query), keyword) {
			return true
		}
	}

	return false
}

// UnifiedQueryEngineImpl implements the UnifiedQueryEngine interface
type UnifiedQueryEngineImpl struct {
	metricsService    *VictoriaMetricsService
	logsService       *VictoriaLogsService
	tracesService     *VictoriaTracesService
	correlationEngine CorrelationEngine
	cache             cache.ValkeyCluster
	logger            logger.Logger
	queryRouter       *QueryRouter
	queryPool         *utils.QueryPoolManager
	uqlParser         *models.UQLParser
	uqlTranslator     *UQLTranslatorRegistry
	uqlOptimizer      UQLOptimizer
	tracer            *tracing.QueryTracer
}

// NewUnifiedQueryEngine creates a new UnifiedQueryEngine instance
func NewUnifiedQueryEngine(
	metricsSvc *VictoriaMetricsService,
	logsSvc *VictoriaLogsService,
	tracesSvc *VictoriaTracesService,
	correlationEngine CorrelationEngine,
	cache cache.ValkeyCluster,
	logger logger.Logger,
) UnifiedQueryEngine {
	return &UnifiedQueryEngineImpl{
		metricsService:    metricsSvc,
		logsService:       logsSvc,
		tracesService:     tracesSvc,
		correlationEngine: correlationEngine,
		cache:             cache,
		logger:            logger,
		queryRouter:       NewQueryRouter(logger),
		queryPool:         utils.NewQueryPoolManager(),
		uqlParser:         models.NewUQLParser(),
		uqlTranslator:     NewUQLTranslatorRegistry(logger),
		uqlOptimizer:      NewUQLOptimizer(logger),
		tracer:            tracing.GetGlobalTracer(),
	}
}

// ExecuteCorrelationQuery executes a correlation query across multiple engines
func (u *UnifiedQueryEngineImpl) ExecuteCorrelationQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	start := time.Now()

	if u.correlationEngine == nil {
		return nil, fmt.Errorf("correlation engine not configured")
	}

	// Parse the correlation query from the unified query
	corrQuery, err := u.parseCorrelationQuery(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse correlation query: %w", err)
	}

	// Execute the correlation
	corrResult, err := u.correlationEngine.ExecuteCorrelation(ctx, corrQuery)
	if err != nil {
		return nil, fmt.Errorf("correlation execution failed: %w", err)
	}

	// Convert to unified result
	return &models.UnifiedResult{
		QueryID:       query.ID,
		Type:          models.QueryTypeCorrelation,
		Status:        "success",
		Data:          corrResult,
		Correlations:  corrResult,
		ExecutionTime: time.Since(start).Milliseconds(),
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeCorrelation: {
					Engine:        models.QueryTypeCorrelation,
					Status:        "success",
					RecordCount:   corrResult.Summary.TotalCorrelations,
					ExecutionTime: int64(time.Since(start).Milliseconds()),
					DataSource:    "correlation-engine",
				},
			},
			TotalRecords: corrResult.Summary.TotalCorrelations,
			DataSources:  []string{"correlation-engine"},
		},
	}, nil
}

// ExecuteUQLQuery executes a UQL query by parsing, optimizing, translating, and executing it
func (u *UnifiedQueryEngineImpl) ExecuteUQLQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	start := time.Now()

	// Parse the UQL query
	uqlQuery, err := u.uqlParser.Parse(query.Query)
	if err != nil {
		return nil, fmt.Errorf("UQL parsing failed: %w", err)
	}

	u.logger.Info("UQL query parsed",
		"query_id", query.ID,
		"query_type", uqlQuery.Type,
		"original_query", query.Query)

	// Apply optimizations
	optimizedQuery, err := u.uqlOptimizer.Optimize(uqlQuery)
	if err != nil {
		return nil, fmt.Errorf("UQL optimization failed: %w", err)
	}

	u.logger.Info("UQL query optimized",
		"query_id", query.ID,
		"optimizations_applied", "multiple passes")

	// Translate the UQL query to engine-specific format
	translatedQuery, err := u.uqlTranslator.TranslateQuery(optimizedQuery)
	if err != nil {
		return nil, fmt.Errorf("UQL translation failed: %w", err)
	}

	u.logger.Info("UQL query translated",
		"from_type", optimizedQuery.Type,
		"to_engine", translatedQuery.Engine,
		"translated_query", translatedQuery.Query)

	// Convert translated query to unified query format
	unifiedQuery := &models.UnifiedQuery{
		ID:         fmt.Sprintf("uql_%d", time.Now().UnixNano()),
		Type:       u.mapUQLEngineToQueryType(translatedQuery.Engine),
		Query:      translatedQuery.Query,
		Parameters: translatedQuery.Parameters,
	}

	// Set time parameters
	if translatedQuery.StartTime != nil {
		unifiedQuery.StartTime = translatedQuery.StartTime
	}
	if translatedQuery.EndTime != nil {
		unifiedQuery.EndTime = translatedQuery.EndTime
	}

	// Set time window if specified
	if translatedQuery.TimeWindow != nil {
		unifiedQuery.CorrelationOptions = &models.CorrelationOptions{
			TimeWindow: *translatedQuery.TimeWindow,
		}
	}

	// Execute using the existing unified query execution logic
	result, err := u.ExecuteQuery(ctx, unifiedQuery)
	if err != nil {
		return nil, fmt.Errorf("UQL execution failed: %w", err)
	}

	// Add UQL-specific metadata
	result.Metadata.DataSources = append(result.Metadata.DataSources, "uql-engine")
	result.ExecutionTime = time.Since(start).Milliseconds()

	u.logger.Info("UQL query executed successfully",
		"query_id", result.QueryID,
		"execution_time_ms", result.ExecutionTime)

	return result, nil
}

// mapUQLEngineToQueryType maps UQL engine types to unified query types
func (u *UnifiedQueryEngineImpl) mapUQLEngineToQueryType(engine models.UQLEngine) models.QueryType {
	switch engine {
	case models.EngineMetrics:
		return models.QueryTypeMetrics
	case models.EngineLogs:
		return models.QueryTypeLogs
	case models.EngineTraces:
		return models.QueryTypeTraces
	case models.EngineCorrelation:
		return models.QueryTypeCorrelation
	default:
		u.logger.Warn("Unknown UQL engine type, defaulting to logs", "engine", engine)
		return models.QueryTypeLogs
	}
}

// GetQueryMetadata returns metadata about supported query types and capabilities
func (u *UnifiedQueryEngineImpl) GetQueryMetadata(ctx context.Context) (*models.QueryMetadata, error) {
	return &models.QueryMetadata{
		SupportedEngines: []models.QueryType{
			models.QueryTypeMetrics,
			models.QueryTypeLogs,
			models.QueryTypeTraces,
		},
		QueryCapabilities: map[models.QueryType][]string{
			models.QueryTypeMetrics: {"instant", "range", "series", "labels", "label_values"},
			models.QueryTypeLogs:    {"query", "stream"},
			models.QueryTypeTraces:  {"query", "services", "operations", "traces"},
		},
		CacheCapabilities: models.CacheCapabilities{
			Supported:  true,
			DefaultTTL: 5 * time.Minute,
			MaxTTL:     1 * time.Hour,
		},
	}, nil
}

// HealthCheck checks the health of all underlying engines
func (u *UnifiedQueryEngineImpl) HealthCheck(ctx context.Context) (*models.EngineHealthStatus, error) {
	engineHealth := make(map[models.QueryType]string)

	// Check metrics engine
	if u.metricsService != nil {
		if err := u.metricsService.HealthCheck(ctx); err != nil {
			engineHealth[models.QueryTypeMetrics] = "unhealthy"
		} else {
			engineHealth[models.QueryTypeMetrics] = "healthy"
		}
	} else {
		engineHealth[models.QueryTypeMetrics] = "not_configured"
	}

	// Check logs engine
	if u.logsService != nil {
		if err := u.logsService.HealthCheck(ctx); err != nil {
			engineHealth[models.QueryTypeLogs] = "unhealthy"
		} else {
			engineHealth[models.QueryTypeLogs] = "healthy"
		}
	} else {
		engineHealth[models.QueryTypeLogs] = "not_configured"
	}

	// Check traces engine
	if u.tracesService != nil {
		if err := u.tracesService.HealthCheck(ctx); err != nil {
			engineHealth[models.QueryTypeTraces] = "unhealthy"
		} else {
			engineHealth[models.QueryTypeTraces] = "healthy"
		}
	} else {
		engineHealth[models.QueryTypeTraces] = "not_configured"
	}

	// Determine overall health
	overallHealth := "healthy"
	for _, status := range engineHealth {
		if status == "unhealthy" {
			overallHealth = "unhealthy"
			break
		}
		if status == "not_configured" && overallHealth == "healthy" {
			overallHealth = "partial"
		}
	}

	return &models.EngineHealthStatus{
		OverallHealth: overallHealth,
		EngineHealth:  engineHealth,
		LastChecked:   time.Now(),
	}, nil
}

// InvalidateCache invalidates cached results for a query pattern
func (u *UnifiedQueryEngineImpl) InvalidateCache(ctx context.Context, queryPattern string) error {
	u.logger.Info("Invalidating cache for pattern", "pattern", queryPattern)

	// Generate pattern-based cache keys to invalidate
	patterns := u.generateCacheInvalidationPatterns(queryPattern)

	// Use a background goroutine to avoid blocking the caller
	go func() {
		backgroundCtx := context.Background()
		for _, pattern := range patterns {
			if err := u.invalidateCachePattern(backgroundCtx, pattern); err != nil {
				u.logger.Warn("Failed to invalidate cache pattern", "pattern", pattern, "error", err)
			}
		}
	}()

	return nil
}

// generateCacheInvalidationPatterns generates cache key patterns for invalidation
func (u *UnifiedQueryEngineImpl) generateCacheInvalidationPatterns(queryPattern string) []string {
	patterns := []string{
		fmt.Sprintf("query_cache:*%s*", queryPattern), // Contains pattern
	}

	// Add engine-specific patterns
	engines := []models.QueryType{models.QueryTypeLogs, models.QueryTypeMetrics, models.QueryTypeTraces}
	for _, engine := range engines {
		patterns = append(patterns, fmt.Sprintf("query_cache:%s:*%s*", engine, queryPattern))
	}

	// Add correlation patterns if the pattern contains correlation keywords
	if strings.Contains(strings.ToLower(queryPattern), "and") ||
		strings.Contains(strings.ToLower(queryPattern), "within") ||
		strings.Contains(strings.ToLower(queryPattern), "correlation") {
		patterns = append(patterns, "query_cache:correlation:*")
	}

	return patterns
}

// invalidateCachePattern invalidates all cache keys matching a pattern
func (u *UnifiedQueryEngineImpl) invalidateCachePattern(ctx context.Context, pattern string) error {
	u.logger.Debug("Invalidating cache pattern using indexes", "pattern", pattern)

	// Use pattern indexes for efficient invalidation instead of SCAN
	// This approach maintains sets of cache keys organized by patterns

	patternSetKey := fmt.Sprintf("pattern_index:%s", pattern)

	// Get all cache keys in this pattern set
	cacheKeys, err := u.cache.GetPatternIndexKeys(ctx, patternSetKey)
	if err != nil {
		u.logger.Warn("Failed to get pattern index keys", "pattern", pattern, "error", err)
		return err
	}

	if len(cacheKeys) == 0 {
		u.logger.Debug("No cache keys found for pattern", "pattern", pattern)
		return nil
	}

	// Delete all cache keys in batches to avoid blocking
	batchSize := 100
	for i := 0; i < len(cacheKeys); i += batchSize {
		end := i + batchSize
		if end > len(cacheKeys) {
			end = len(cacheKeys)
		}

		batch := cacheKeys[i:end]
		if err := u.cache.DeleteMultiple(ctx, batch); err != nil {
			u.logger.Warn("Failed to delete cache key batch", "pattern", pattern, "batch_size", len(batch), "error", err)
			// Continue with other batches even if one fails
		}
	}

	// Clean up the pattern index set
	if err := u.cache.DeletePatternIndex(ctx, patternSetKey); err != nil {
		u.logger.Warn("Failed to clean up pattern index", "pattern", patternSetKey, "error", err)
		// Don't return error for cleanup failure
	}

	u.logger.Info("Invalidated cache pattern", "pattern", pattern, "keys_deleted", len(cacheKeys))
	return nil
}

// ExecuteQuery executes a unified query across the appropriate engines
func (u *UnifiedQueryEngineImpl) ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	start := time.Now()

	// Start distributed tracing span for the unified query
	queryCtx, querySpan := u.startQuerySpan(ctx, query)
	defer querySpan.End()

	// Check if query can be parallelized across multiple engines
	if u.canParallelizeQuery(query) {
		u.logger.Info("Executing query in parallel across multiple engines", "query_id", query.ID)
		return u.executeParallelQuery(queryCtx, query, start)
	}

	// Use intelligent query router to determine optimal engine
	routedType, routingReason, err := u.queryRouter.RouteQuery(query)
	if err != nil {
		u.tracer.RecordError(querySpan, err)
		return nil, fmt.Errorf("query routing failed: %w", err)
	}

	// Update query type with routed type
	originalType := query.Type
	query.Type = routedType

	u.logger.Info("Query routed by intelligent router",
		"query_id", query.ID,
		"original_type", originalType,
		"routed_type", routedType,
		"reason", routingReason)

	// Add routing information to span
	u.tracer.AddQueryAttributes(querySpan,
		attribute.String("query.routing.original_type", string(originalType)),
		attribute.String("query.routing.final_type", string(routedType)),
		attribute.String("query.routing.reason", routingReason),
	)

	// Generate cache key
	cacheKey := u.generateCacheKey(query)

	// Check cache first if enabled
	var cacheHit bool
	if query.CacheOptions != nil && query.CacheOptions.Enabled && !query.CacheOptions.BypassCache {
		cacheStart := time.Now()
		if cachedResult, err := u.getCachedResult(queryCtx, cacheKey); err == nil && cachedResult != nil {
			cacheDuration := time.Since(cacheStart)
			u.logger.Info("Returning cached result", "query_id", query.ID, "cache_key", cacheKey)
			cachedResult.Cached = true
			cacheHit = true

			// Record cache hit metrics and tracing
			monitoring.RecordUnifiedQueryCacheOperation("get", "hit")
			u.tracer.RecordCacheMetrics(querySpan, true, cacheDuration)
			u.tracer.RecordQueryMetrics(querySpan, time.Since(start), int64(cachedResult.Metadata.TotalRecords), true)
			monitoring.RecordUnifiedQueryOperation(string(query.Type), string(routedType), true, time.Since(start), true)

			return cachedResult, nil
		}
		// Record cache miss
		cacheDuration := time.Since(cacheStart)
		monitoring.RecordUnifiedQueryCacheOperation("get", "miss")
		u.tracer.RecordCacheMetrics(querySpan, false, cacheDuration)
	}

	// Check if this is a UQL query that needs special processing
	if u.queryRouter.isUQLQuery(query.Query) {
		u.logger.Info("Detected UQL query, using ExecuteUQLQuery", "query_id", query.ID)
		return u.ExecuteUQLQuery(queryCtx, query)
	}

	// Route to appropriate engine based on routed query type
	var result *models.UnifiedResult
	var routingErr error

	switch query.Type {
	case models.QueryTypeMetrics:
		result, routingErr = u.executeMetricsQuery(queryCtx, query)
	case models.QueryTypeLogs:
		result, routingErr = u.executeLogsQuery(queryCtx, query)
	case models.QueryTypeTraces:
		result, routingErr = u.executeTracesQuery(queryCtx, query)
	case models.QueryTypeCorrelation:
		result, routingErr = u.ExecuteCorrelationQuery(queryCtx, query)
	default:
		err := fmt.Errorf("unsupported query type: %s", query.Type)
		u.tracer.RecordError(querySpan, err)
		return nil, err
	}

	if routingErr != nil {
		// Record failed operation metrics and tracing
		u.tracer.RecordError(querySpan, routingErr)
		u.tracer.RecordQueryMetrics(querySpan, time.Since(start), 0, false)
		monitoring.RecordUnifiedQueryOperation(string(query.Type), string(routedType), cacheHit, time.Since(start), false)
		return nil, routingErr
	}

	// Use pooled result object for better performance
	if result == nil {
		result = u.queryPool.GetUnifiedResult()
	}
	result.ExecutionTime = time.Since(start).Milliseconds()
	result.Cached = false

	// Cache the result if caching is enabled
	if query.CacheOptions != nil && query.CacheOptions.Enabled {
		cacheStart := time.Now()
		ttl := query.CacheOptions.TTL
		if ttl == 0 {
			ttl = 5 * time.Minute // default TTL
		}
		if err := u.cacheResult(queryCtx, cacheKey, result, ttl); err != nil {
			cacheDuration := time.Since(cacheStart)
			u.logger.Warn("Failed to cache query result", "error", err, "query_id", query.ID)
			monitoring.RecordUnifiedQueryCacheOperation("set", "error")
			u.tracer.RecordCacheMetrics(querySpan, false, cacheDuration)
		} else {
			cacheDuration := time.Since(cacheStart)
			monitoring.RecordUnifiedQueryCacheOperation("set", "success")
			u.tracer.RecordCacheMetrics(querySpan, true, cacheDuration)
		}
	}

	// Record successful operation metrics and tracing
	u.tracer.RecordQueryMetrics(querySpan, time.Since(start), int64(result.Metadata.TotalRecords), true)
	monitoring.RecordUnifiedQueryOperation(string(query.Type), string(routedType), cacheHit, time.Since(start), true)

	return result, nil
}

// startQuerySpan starts a tracing span for a unified query
func (u *UnifiedQueryEngineImpl) startQuerySpan(ctx context.Context, query *models.UnifiedQuery) (context.Context, trace.Span) {
	if u.tracer == nil {
		// Return no-op span if tracer is not configured
		return ctx, trace.SpanFromContext(ctx)
	}
	return u.tracer.StartQuerySpan(ctx, query.ID, string(query.Type), query.Query)
}

// executeMetricsQuery executes a metrics query using VictoriaMetricsService
func (u *UnifiedQueryEngineImpl) executeMetricsQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	if u.metricsService == nil {
		return nil, fmt.Errorf("metrics service not configured")
	}

	if query.StartTime != nil && query.EndTime != nil {
		// Range query
		rangeQuery := &models.MetricsQLRangeQueryRequest{
			Query:    query.Query,
			Start:    query.StartTime.Format(time.RFC3339),
			End:      query.EndTime.Format(time.RFC3339),
			Step:     "15s", // default step
			TenantID: query.TenantID,
		}
		result, err := u.metricsService.ExecuteRangeQuery(ctx, rangeQuery)
		if err != nil {
			return nil, fmt.Errorf("metrics range query failed: %w", err)
		}

		return &models.UnifiedResult{
			QueryID: query.ID,
			Type:    models.QueryTypeMetrics,
			Status:  result.Status,
			Data:    result.Data,
			Metadata: &models.ResultMetadata{
				EngineResults: map[models.QueryType]*models.EngineResult{
					models.QueryTypeMetrics: {
						Engine:      models.QueryTypeMetrics,
						Status:      result.Status,
						RecordCount: result.DataPointCount,
						DataSource:  "victoria-metrics",
					},
				},
				TotalRecords: result.DataPointCount,
				DataSources:  []string{"victoria-metrics"},
			},
		}, nil
	}

	// Instant query
	metricsQuery := &models.MetricsQLQueryRequest{
		Query:    query.Query,
		TenantID: query.TenantID,
		Timeout:  query.Timeout,
	}
	result, err := u.metricsService.ExecuteQuery(ctx, metricsQuery)
	if err != nil {
		return nil, fmt.Errorf("metrics query failed: %w", err)
	}

	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeMetrics,
		Status:  result.Status,
		Data:    result.Data,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeMetrics: {
					Engine:      models.QueryTypeMetrics,
					Status:      result.Status,
					RecordCount: result.SeriesCount,
					DataSource:  "victoria-metrics",
				},
			},
			TotalRecords: result.SeriesCount,
			DataSources:  []string{"victoria-metrics"},
		},
	}, nil
}

// executeLogsQuery executes a logs query using VictoriaLogsService
func (u *UnifiedQueryEngineImpl) executeLogsQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	if u.logsService == nil {
		return nil, fmt.Errorf("logs service not configured")
	}

	// Convert unified query to LogsQL query
	var startTime, endTime int64
	if query.StartTime != nil {
		startTime = query.StartTime.UnixMilli()
	}
	if query.EndTime != nil {
		endTime = query.EndTime.UnixMilli()
	}

	logsQuery := &models.LogsQLQueryRequest{
		Query:    query.Query,
		Start:    startTime,
		End:      endTime,
		Limit:    1000, // default limit
		TenantID: query.TenantID,
	}

	result, err := u.logsService.ExecuteQuery(ctx, logsQuery)
	if err != nil {
		return nil, fmt.Errorf("logs query failed: %w", err)
	}

	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeLogs,
		Status:  "success",
		Data:    result.Logs,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeLogs: {
					Engine:      models.QueryTypeLogs,
					Status:      "success",
					RecordCount: len(result.Logs),
					DataSource:  "victoria-logs",
				},
			},
			TotalRecords: len(result.Logs),
			DataSources:  []string{"victoria-logs"},
		},
	}, nil
}

// executeTracesQuery executes a traces query using VictoriaTracesService
func (u *UnifiedQueryEngineImpl) executeTracesQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	if u.tracesService == nil {
		return nil, fmt.Errorf("traces service not configured")
	}

	// For now, use GetOperations as a basic traces query
	// TODO: Implement full traces search functionality
	operations, err := u.tracesService.GetOperations(ctx, query.Query, query.TenantID)
	if err != nil {
		return nil, fmt.Errorf("traces query failed: %w", err)
	}

	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeTraces,
		Status:  "success",
		Data:    operations,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeTraces: {
					Engine:      models.QueryTypeTraces,
					Status:      "success",
					RecordCount: len(operations),
					DataSource:  "victoria-traces",
				},
			},
			TotalRecords: len(operations),
			DataSources:  []string{"victoria-traces"},
		},
	}, nil
}

// canParallelizeQuery determines if a query can be executed in parallel across multiple engines
func (u *UnifiedQueryEngineImpl) canParallelizeQuery(query *models.UnifiedQuery) bool {
	queryStr := strings.ToLower(query.Query)

	// Check for explicit parallel keywords
	if strings.Contains(queryStr, "parallel") || strings.Contains(queryStr, "concurrent") {
		return true
	}

	// Check for correlation patterns that can be parallelized
	if strings.Contains(queryStr, " and ") || strings.Contains(queryStr, " within ") {
		return true
	}

	// Check for multi-engine patterns (contains patterns from multiple engines)
	enginePatterns := map[string][]string{
		"metrics": {"rate(", "increase(", "cpu", "memory", "latency", "requests"},
		"logs":    {"error", "warn", "info", "debug", "exception", "stacktrace"},
		"traces":  {"service:", "operation:", "span", "trace", "duration:"},
	}

	enginesFound := 0
	for _, patterns := range enginePatterns {
		for _, pattern := range patterns {
			if strings.Contains(queryStr, pattern) {
				enginesFound++
				break // Count each engine only once
			}
		}
	}

	// If query spans multiple engines, parallelize
	return enginesFound >= 2
}

// executeParallelQuery executes a query across multiple engines in parallel
func (u *UnifiedQueryEngineImpl) executeParallelQuery(ctx context.Context, query *models.UnifiedQuery, start time.Time) (*models.UnifiedResult, error) {
	// Parse the query into sub-queries for different engines
	subQueries := u.parseSubQueries(query)

	if len(subQueries) == 0 {
		// Fallback to single engine execution
		query.Type = models.QueryTypeLogs // Default fallback
		return u.ExecuteQuery(ctx, query)
	}

	u.logger.Info("Executing parallel sub-queries",
		"query_id", query.ID,
		"sub_queries", len(subQueries))

	// Execute sub-queries in parallel
	type subQueryResult struct {
		engine models.QueryType
		result *models.UnifiedResult
		err    error
	}

	resultsChan := make(chan subQueryResult, len(subQueries))

	// Launch goroutines for each sub-query
	for _, subQuery := range subQueries {
		go func(sq *models.UnifiedQuery) {
			result, err := u.ExecuteQuery(ctx, sq)
			resultsChan <- subQueryResult{
				engine: sq.Type,
				result: result,
				err:    err,
			}
		}(subQuery)
	}

	// Collect results
	engineResults := make(map[models.QueryType]*models.EngineResult)
	var allData []interface{}
	var allCorrelations []models.Correlation
	totalRecords := 0
	var firstError error

	for i := 0; i < len(subQueries); i++ {
		subResult := <-resultsChan

		if subResult.err != nil {
			u.logger.Warn("Sub-query failed",
				"engine", subResult.engine,
				"error", subResult.err)
			if firstError == nil {
				firstError = subResult.err
			}
			continue
		}

		// Merge results
		if subResult.result.Metadata != nil {
			for engine, engineResult := range subResult.result.Metadata.EngineResults {
				engineResults[engine] = engineResult
				totalRecords += engineResult.RecordCount
			}
		}

		// Collect data based on type
		switch subResult.result.Type {
		case models.QueryTypeMetrics:
			if subResult.result.Data != nil {
				allData = append(allData, subResult.result.Data)
			}
		case models.QueryTypeLogs:
			if logs, ok := subResult.result.Data.([]map[string]any); ok {
				for _, log := range logs {
					allData = append(allData, log)
				}
			}
		case models.QueryTypeTraces:
			if traces, ok := subResult.result.Data.([]map[string]interface{}); ok {
				for _, trace := range traces {
					allData = append(allData, trace)
				}
			}
		case models.QueryTypeCorrelation:
			if subResult.result.Correlations != nil {
				allCorrelations = append(allCorrelations, subResult.result.Correlations.Correlations...)
			}
		}
	}

	// If all sub-queries failed, return the first error
	if len(engineResults) == 0 && firstError != nil {
		monitoring.RecordUnifiedQueryOperation("parallel", "multiple", false, time.Since(start), false)
		return nil, fmt.Errorf("all parallel sub-queries failed: %w", firstError)
	}

	// Determine primary result type
	resultType := models.QueryTypeLogs
	if len(subQueries) > 0 {
		resultType = subQueries[0].Type
	}

	// Create merged result
	result := &models.UnifiedResult{
		QueryID:       query.ID,
		Type:          resultType,
		Status:        "success",
		Data:          allData,
		Correlations:  &models.UnifiedCorrelationResult{Correlations: allCorrelations},
		ExecutionTime: time.Since(start).Milliseconds(),
		Cached:        false,
		Metadata: &models.ResultMetadata{
			EngineResults: engineResults,
			TotalRecords:  totalRecords,
			DataSources:   u.getDataSourcesFromEngines(engineResults),
		},
	}

	// Record successful parallel operation metrics
	monitoring.RecordUnifiedQueryOperation("parallel", "multiple", false, time.Since(start), true)

	u.logger.Info("Parallel query execution completed",
		"query_id", query.ID,
		"engines_used", len(engineResults),
		"total_records", totalRecords,
		"execution_time_ms", time.Since(start).Milliseconds())

	return result, nil
}

// parseSubQueries parses a complex query into sub-queries for different engines
func (u *UnifiedQueryEngineImpl) parseSubQueries(query *models.UnifiedQuery) []*models.UnifiedQuery {
	subQueries := []*models.UnifiedQuery{}

	// Simple implementation: split on "AND" for correlation-like queries
	queryParts := strings.Split(query.Query, " AND ")

	if len(queryParts) <= 1 {
		// Try other patterns or fallback to single query
		return subQueries
	}

	for i, part := range queryParts {
		part = strings.TrimSpace(part)

		// Determine engine type for this part
		subQuery := &models.UnifiedQuery{
			ID:           fmt.Sprintf("%s_sub_%d", query.ID, i),
			Query:        part,
			TenantID:     query.TenantID,
			StartTime:    query.StartTime,
			EndTime:      query.EndTime,
			Timeout:      query.Timeout,
			Parameters:   query.Parameters,
			CacheOptions: query.CacheOptions,
		}

		// Route each part to appropriate engine
		if routedType, _, err := u.queryRouter.RouteQuery(subQuery); err == nil {
			subQuery.Type = routedType
			subQueries = append(subQueries, subQuery)
		}
	}

	return subQueries
}

// parseCorrelationQuery parses a correlation query from a unified query
func (u *UnifiedQueryEngineImpl) parseCorrelationQuery(query *models.UnifiedQuery) (*models.CorrelationQuery, error) {
	parser := models.NewCorrelationQueryParser()
	corrQuery, err := parser.Parse(query.Query)
	if err != nil {
		return nil, err
	}

	// Set the ID from the unified query
	corrQuery.ID = query.ID

	return corrQuery, nil
}

// getDataSourcesFromEngines extracts data source names from engine results
func (u *UnifiedQueryEngineImpl) getDataSourcesFromEngines(engineResults map[models.QueryType]*models.EngineResult) []string {
	dataSources := []string{}
	for _, result := range engineResults {
		if result.DataSource != "" {
			dataSources = append(dataSources, result.DataSource)
		}
	}
	return dataSources
}

// generateCacheKey generates a cache key for the query
func (u *UnifiedQueryEngineImpl) generateCacheKey(query *models.UnifiedQuery) string {
	// Create a deterministic key based on query content
	keyData := fmt.Sprintf("%s:%s:%s:%s",
		query.Type,
		query.Query,
		query.TenantID,
		query.Parameters,
	)

	if query.StartTime != nil {
		keyData += query.StartTime.Format(time.RFC3339)
	}
	if query.EndTime != nil {
		keyData += query.EndTime.Format(time.RFC3339)
	}

	// Use MD5 hash for consistent key length
	hash := md5.Sum([]byte(keyData))
	return fmt.Sprintf("unified_query:%x", hash)
}

// getCachedResult retrieves a cached result
func (u *UnifiedQueryEngineImpl) getCachedResult(ctx context.Context, cacheKey string) (*models.UnifiedResult, error) {
	data, err := u.cache.GetCachedQueryResult(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	var result models.UnifiedResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached result: %w", err)
	}

	return &result, nil
}

// cacheResult caches a query result
func (u *UnifiedQueryEngineImpl) cacheResult(ctx context.Context, cacheKey string, result *models.UnifiedResult, ttl time.Duration) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result for caching: %w", err)
	}

	// Cache the result
	if err := u.cache.CacheQueryResult(ctx, cacheKey, data, ttl); err != nil {
		return err
	}

	// Maintain pattern indexes for efficient invalidation
	// Add this cache key to relevant pattern sets
	if err := u.maintainPatternIndexes(ctx, cacheKey, result); err != nil {
		u.logger.Warn("Failed to maintain pattern indexes", "cache_key", cacheKey, "error", err)
	}

	return nil
}

// maintainPatternIndexes maintains pattern-based indexes for efficient cache invalidation
func (u *UnifiedQueryEngineImpl) maintainPatternIndexes(ctx context.Context, cacheKey string, result *models.UnifiedResult) error {
	// Generate patterns that this cache key should be indexed under
	patterns := u.generateCachePatternsForResult(result)

	// Add cache key to each pattern index
	for _, pattern := range patterns {
		patternSetKey := fmt.Sprintf("pattern_index:%s", pattern)
		if err := u.cache.AddToPatternIndex(ctx, patternSetKey, cacheKey); err != nil {
			u.logger.Warn("Failed to add cache key to pattern index", "pattern", pattern, "cache_key", cacheKey, "error", err)
			// Continue with other patterns even if one fails
		}
	}

	return nil
}

// generateCachePatternsForResult generates cache patterns for a query result
func (u *UnifiedQueryEngineImpl) generateCachePatternsForResult(result *models.UnifiedResult) []string {
	patterns := []string{
		"query_cache:*", // All query cache keys
		fmt.Sprintf("query_cache:%s:*", result.Type), // Engine-specific patterns
	}

	// Add correlation patterns if this is a correlation result
	if result.Type == models.QueryTypeCorrelation || result.Correlations != nil {
		patterns = append(patterns, "query_cache:correlation:*")
	}

	// Add tenant-specific patterns if tenant info is available
	// Note: We don't have tenant info in the result, but this could be extended

	return patterns
}
