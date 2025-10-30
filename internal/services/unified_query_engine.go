package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// UnifiedQueryEngine provides a unified interface for querying across multiple observability engines
type UnifiedQueryEngine interface {
	// ExecuteQuery executes a unified query across the appropriate engines
	ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

	// ExecuteCorrelationQuery executes a correlation query across multiple engines
	ExecuteCorrelationQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error)

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

// UnifiedQueryEngineImpl implements the UnifiedQueryEngine interface
type UnifiedQueryEngineImpl struct {
	metricsService    *VictoriaMetricsService
	logsService       *VictoriaLogsService
	tracesService     *VictoriaTracesService
	correlationEngine CorrelationEngine
	cache             cache.ValkeyCluster
	logger            logger.Logger
	queryRouter       *QueryRouter
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
	// For now, just log the operation
	// TODO: Implement pattern-based cache invalidation
	u.logger.Info("Cache invalidation requested", "pattern", queryPattern)
	return nil
}

// ExecuteQuery executes a unified query across the appropriate engines
func (u *UnifiedQueryEngineImpl) ExecuteQuery(ctx context.Context, query *models.UnifiedQuery) (*models.UnifiedResult, error) {
	start := time.Now()

	// Use intelligent query router to determine optimal engine
	routedType, routingReason, err := u.queryRouter.RouteQuery(query)
	if err != nil {
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

	// Generate cache key
	cacheKey := u.generateCacheKey(query)

	// Check cache first if enabled
	if query.CacheOptions != nil && query.CacheOptions.Enabled && !query.CacheOptions.BypassCache {
		if cachedResult, err := u.getCachedResult(ctx, cacheKey); err == nil && cachedResult != nil {
			u.logger.Info("Returning cached result", "query_id", query.ID, "cache_key", cacheKey)
			cachedResult.Cached = true
			return cachedResult, nil
		}
	}

	// Route to appropriate engine based on routed query type
	var result *models.UnifiedResult
	var routingErr error

	switch query.Type {
	case models.QueryTypeMetrics:
		result, routingErr = u.executeMetricsQuery(ctx, query)
	case models.QueryTypeLogs:
		result, routingErr = u.executeLogsQuery(ctx, query)
	case models.QueryTypeTraces:
		result, routingErr = u.executeTracesQuery(ctx, query)
	case models.QueryTypeCorrelation:
		result, routingErr = u.ExecuteCorrelationQuery(ctx, query)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", query.Type)
	}

	if routingErr != nil {
		return nil, routingErr
	}

	result.ExecutionTime = time.Since(start).Milliseconds()
	result.Cached = false

	// Cache the result if caching is enabled
	if query.CacheOptions != nil && query.CacheOptions.Enabled {
		ttl := query.CacheOptions.TTL
		if ttl == 0 {
			ttl = 5 * time.Minute // default TTL
		}
		if err := u.cacheResult(ctx, cacheKey, result, ttl); err != nil {
			u.logger.Warn("Failed to cache query result", "error", err, "query_id", query.ID)
		}
	}

	return result, nil
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

	return u.cache.CacheQueryResult(ctx, cacheKey, data, ttl)
}
