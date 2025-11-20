package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/platformbuilds/mirador-core/api" // Import generated Swagger docs
	"github.com/platformbuilds/mirador-core/internal/api/handlers"
	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/rca"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/tracing"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/mapping"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/storage"
	"github.com/platformbuilds/mirador-core/internal/utils/search"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type Server struct {
	config                      *config.Config
	logger                      logger.Logger
	cache                       cache.ValkeyCluster
	vmServices                  *services.VictoriaMetricsServices
	schemaRepo                  repo.SchemaStore
	searchRouter                *search.SearchRouter
	searchThrottling            *middleware.SearchQueryThrottlingMiddleware
	metricsMetadataIndexer      services.MetricsMetadataIndexer
	metricsMetadataSynchronizer services.MetricsMetadataSynchronizer
	router                      *gin.Engine
	httpServer                  *http.Server
	tracerProvider              *tracing.TracerProvider
}

func NewServer(
	cfg *config.Config,
	log logger.Logger,
	valkeyCache cache.ValkeyCluster,
	vmServices *services.VictoriaMetricsServices,
	schemaRepo repo.SchemaStore,
) *Server {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Initialize distributed tracing if enabled
	var tracerProvider *tracing.TracerProvider
	if cfg.Monitoring.TracingEnabled {
		var err error
		tracerProvider, err = tracing.NewTracerProvider(
			"mirador-core",
			"v7.0.0", // TODO: Pass version from build info
			cfg.Monitoring.JaegerEndpoint,
		)
		if err != nil {
			log.Error("Failed to initialize tracer provider", "error", err)
		} else {
			log.Info("Distributed tracing initialized", "endpoint", cfg.Monitoring.JaegerEndpoint)
			// Initialize global tracer
			tracing.InitGlobalTracer("mirador-core")
		}
	}

	server := &Server{
		config:         cfg,
		logger:         log,
		cache:          valkeyCache,
		vmServices:     vmServices,
		schemaRepo:     schemaRepo,
		router:         router,
		tracerProvider: tracerProvider,
	}

	// Create search router
	searchConfig := &search.SearchConfig{
		DefaultEngine: cfg.Search.DefaultEngine,
		EnableBleve:   cfg.Search.EnableBleve,
		EnableLucene:  cfg.Search.EnableLucene,
	}
	searchRouter, err := search.NewSearchRouter(searchConfig, log)
	if err != nil {
		log.Error("Failed to create search router", "error", err)
		return nil
	}
	server.searchRouter = searchRouter

	// Initialize metrics metadata components if enabled
	if cfg.Search.Bleve.MetricsEnabled {
		if err := server.initializeMetricsMetadataComponents(); err != nil {
			log.Error("Failed to initialize metrics metadata components", "error", err)
			return nil
		}
	} else {
		// Create stub implementations for development/testing
		log.Info("Metrics metadata components disabled - creating stub implementations for API compatibility")
		server.metricsMetadataIndexer = services.NewStubMetricsMetadataIndexer(log)
		server.metricsMetadataSynchronizer = services.NewStubMetricsMetadataSynchronizer(log)
	}

	server.setupMiddleware()
	server.setupRoutes()

	return server
}

func (s *Server) setupMiddleware() {
	// Recovery middleware
	s.router.Use(gin.Recovery())

	// Error handling middleware (must be early)
	s.router.Use(middleware.ErrorHandler(s.logger))

	// CORS for MIRADOR-UI communication
	s.router.Use(middleware.CORSMiddleware(s.config.CORS))

	// Request logging
	s.router.Use(middleware.RequestLogger(s.logger))

	// Prometheus request metrics
	s.router.Use(middleware.MetricsMiddleware())

	// Rate limiting using Valkey cluster
	s.router.Use(middleware.RateLimiter(s.cache))

	// Search query throttling based on complexity
	s.searchThrottling = middleware.NewSearchQueryThrottlingMiddleware(s.cache, s.logger)

	// OpenAPI specification endpoints
	s.router.StaticFile("/api/openapi.yaml", "api/openapi.yaml")
	s.router.GET("/api/openapi.json", handlers.GetOpenAPISpec)

	// Swagger UI via gin-swagger (configured to use our comprehensive OpenAPI spec)
	// Visit /swagger/index.html
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api/openapi.json")))

	// Prometheus metrics endpoint
	monitoring.SetupPrometheusMetrics(s.router)
}

func (s *Server) setupRoutes() {
	// Create health handler instance
	healthHandler := handlers.NewHealthHandlerWithCache(s.vmServices, s.cache, s.logger)

	// Public health endpoints - now using handler instance methods
	s.router.GET("/health", healthHandler.HealthCheck)
	s.router.GET("/ready", healthHandler.ReadinessCheck)
	s.router.GET("/microservices/status", healthHandler.MicroservicesStatus)

	// Root redirect to Swagger UI for convenience
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/swagger/index.html")
	})

	// API v1 group
	v1 := s.router.Group("/api/v1")

	// Back-compat: expose health under /api/v1 as well
	v1.GET("/health", healthHandler.HealthCheck)
	v1.GET("/ready", healthHandler.ReadinessCheck)
	v1.GET("/microservices/status", healthHandler.MicroservicesStatus)

	// Also expose metrics under /api/v1 for consistency
	monitoring.SetupPrometheusMetrics(v1)

	// MetricsQL endpoints (VictoriaMetrics integration)
	var metricsHandler *handlers.MetricsQLHandler
	if s.schemaRepo != nil {
		metricsHandler = handlers.NewMetricsQLHandlerWithSchema(s.vmServices.Metrics, s.cache, s.logger, s.schemaRepo)
	} else {
		metricsHandler = handlers.NewMetricsQLHandler(s.vmServices.Metrics, s.cache, s.logger)
	}

	// Register metrics endpoints conditionally - retired when unified query is enabled
	if !s.config.UnifiedQuery.Enabled {
		// New metrics endpoints under /metrics/*
		metricsGroup := v1.Group("/metrics")
		{
			metricsGroup.POST("/query", metricsHandler.ExecuteQuery)
			metricsGroup.POST("/query_range", metricsHandler.ExecuteRangeQuery)
			metricsGroup.GET("/names", metricsHandler.GetMetricNames)
			metricsGroup.GET("/series", metricsHandler.GetSeries)
			metricsGroup.POST("/labels", metricsHandler.GetLabels)
			metricsGroup.GET("/label/:name/values", metricsHandler.GetLabelValues)
		}
		// Back-compat (deprecated): keep old routes registered
		v1.POST("/query", metricsHandler.ExecuteQuery)
		v1.POST("/query_range", metricsHandler.ExecuteRangeQuery)
		s.router.POST("/query", metricsHandler.ExecuteQuery)
		s.router.POST("/query_range", metricsHandler.ExecuteRangeQuery)
		s.router.GET("/metrics/names", metricsHandler.GetMetricNames)
		s.router.GET("/metrics/series", metricsHandler.GetSeries)
		s.router.POST("/metrics/labels", metricsHandler.GetLabels)
	}

	// MetricsQL function query endpoints (hierarchical by category)
	queryHandler := handlers.NewMetricsQLQueryHandler(s.vmServices.Query, s.cache, s.logger)
	validationMiddleware := middleware.NewMetricsQLQueryValidationMiddleware(s.logger)

	// Rollup functions
	rollupGroup := v1.Group("/metrics/query/rollup")
	{
		rollupGroup.POST("/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteRollupFunction)
		rollupGroup.POST("/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteRollupRangeFunction)
	}
	// Transform functions
	transformGroup := v1.Group("/metrics/query/transform")
	{
		transformGroup.POST("/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteTransformFunction)
		transformGroup.POST("/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteTransformRangeFunction)
	}
	// Label functions
	labelGroup := v1.Group("/metrics/query/label")
	{
		labelGroup.POST("/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteLabelFunction)
		labelGroup.POST("/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteLabelRangeFunction)
	}
	// Aggregate functions
	aggregateGroup := v1.Group("/metrics/query/aggregate")
	{
		aggregateGroup.POST("/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteAggregateFunction)
		aggregateGroup.POST("/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteAggregateRangeFunction)
	}

	// LogsQL endpoints (VictoriaLogs integration)
	logsHandler := handlers.NewLogsQLHandler(s.vmServices.Logs, s.cache, s.logger, s.searchRouter, s.config)

	// Register logs endpoints conditionally - retired when unified query is enabled
	if !s.config.UnifiedQuery.Enabled {
		logsGroup := v1.Group("/logs")
		{
			logsGroup.POST("/query", s.searchThrottling.ThrottleLogsQuery(), logsHandler.ExecuteQuery)
			logsGroup.GET("/streams", logsHandler.GetStreams)
			logsGroup.GET("/fields", logsHandler.GetFields)
			logsGroup.POST("/export", logsHandler.ExportLogs)
			logsGroup.POST("/store", logsHandler.StoreEvent) // For AI engines to store JSON events
		}
	}

	// D3-friendly log endpoints for the upcoming UI (histogram, facets, search, tail)
	d3Logs := handlers.NewLogsHandler(s.vmServices.Logs, s.logger)
	d3LogsGroup := v1.Group("/logs")
	{
		d3LogsGroup.GET("/histogram", d3Logs.Histogram)
		d3LogsGroup.GET("/facets", d3Logs.Facets)
		d3LogsGroup.POST("/search", d3Logs.Search)
		d3LogsGroup.GET("/tail", d3Logs.TailWS) // WebSocket upgrade for tailing logs
	}

	// VictoriaTraces endpoints (Jaeger-compatible HTTP API)
	tracesHandler := handlers.NewTracesHandler(s.vmServices.Traces, s.cache, s.logger, s.searchRouter, s.config)

	// Register traces endpoints conditionally - retired when unified query is enabled
	if !s.config.UnifiedQuery.Enabled {
		tracesGroup := v1.Group("/traces")
		{
			tracesGroup.GET("/services", tracesHandler.GetServices)
			tracesGroup.GET("/services/:service/operations", tracesHandler.GetOperations)
			tracesGroup.GET("/:traceId", tracesHandler.GetTrace)
			tracesGroup.GET("/:traceId/flamegraph", tracesHandler.GetFlameGraph)
			tracesGroup.POST("/search", s.searchThrottling.ThrottleTracesQuery(), tracesHandler.SearchTraces)
			tracesGroup.POST("/flamegraph/search", tracesHandler.SearchFlameGraph)
		}
	}

	// Phase 4: Create RCA Engine for /rca endpoints
	// Create an empty service graph (will be enhanced from metrics in production)
	rcaServiceGraphForEngine := rca.NewServiceGraph()

	// Create a mock anomaly events provider (no-op for now)
	anomalyProviderForEngine := &noOpAnomalyProvider{}

	// Create anomaly collector and candidate cause service
	incidentAnomalyCollectorForEngine := rca.NewIncidentAnomalyCollector(
		anomalyProviderForEngine,
		rcaServiceGraphForEngine,
		s.logger,
	)
	candidateCauseServiceForEngine := rca.NewCandidateCauseService(incidentAnomalyCollectorForEngine, s.logger)

	// Create RCA engine for endpoints
	rcaEngineForEndpoints := rca.NewRCAEngine(candidateCauseServiceForEngine, rcaServiceGraphForEngine, s.logger)

	// AI RCA-ENGINE endpoints (correlation with red anchors pattern)
	rcaServiceGraph := services.NewServiceGraphService(s.vmServices.Metrics, s.logger)
	rcaHandler := handlers.NewRCAHandler(s.vmServices.Logs, rcaServiceGraph, s.cache, s.logger, rcaEngineForEndpoints)
	rcaGroup := v1.Group("/rca")
	{
		rcaGroup.GET("/correlations", rcaHandler.GetActiveCorrelations)
		rcaGroup.POST("/investigate", rcaHandler.StartInvestigation)
		rcaGroup.GET("/patterns", rcaHandler.GetFailurePatterns)
		rcaGroup.POST("/service-graph", rcaHandler.GetServiceGraph)
		rcaGroup.POST("/store", rcaHandler.StoreCorrelation) // Store correlation back to VictoriaLogs
	}

	// KPI APIs (primary interface for schema definitions)
	if s.schemaRepo != nil {
		kpiHandler := handlers.NewKPIHandler(s.schemaRepo, s.cache, s.logger)
		if kpiHandler != nil {
			// KPI Definitions API
			kpiDefsGroup := v1.Group("/kpi/defs")
			{
				kpiDefsGroup.GET("", kpiHandler.GetKPIDefinitions)
				kpiDefsGroup.POST("", kpiHandler.CreateOrUpdateKPIDefinition)
				kpiDefsGroup.DELETE("/:id", kpiHandler.DeleteKPIDefinition)
			}
		}
	}

	// Unified Query Engine (Phase 1.5: Unified API Implementation)
	if s.config.UnifiedQuery.Enabled {
		s.setupUnifiedQueryEngine(v1, rcaEngineForEndpoints)
	}

	// Metrics Metadata Discovery API (Phase 2: Metrics Metadata Integration)
	if s.metricsMetadataIndexer != nil {
		metricsSearchHandler := handlers.NewMetricsSearchHandler(s.metricsMetadataIndexer, s.logger)
		metricsSearchGroup := v1.Group("/metrics")
		{
			metricsSearchGroup.POST("/search", metricsSearchHandler.HandleMetricsSearch)
			metricsSearchGroup.POST("/sync", metricsSearchHandler.HandleMetricsSync)
			metricsSearchGroup.GET("/health", metricsSearchHandler.HandleMetricsHealth)
		}
	}

	// Metrics Metadata Synchronization API (Phase 2: Metrics Metadata Integration)
	if s.metricsMetadataSynchronizer != nil {
		syncHandler := handlers.NewMetricsSyncHandler(s.metricsMetadataSynchronizer, s.logger)
		metricsSyncGroup := v1.Group("/metrics/sync/control")
		{
			metricsSyncGroup.POST("", syncHandler.HandleSyncNow)
			metricsSyncGroup.GET("/state", syncHandler.HandleGetSyncState)
			metricsSyncGroup.GET("/status", syncHandler.HandleGetSyncStatus)
		}
		v1.PUT("/metrics/sync/control/config", syncHandler.HandleUpdateConfig)
	}
}

// setupUnifiedQueryEngine sets up the unified query engine and registers its routes
func (s *Server) setupUnifiedQueryEngine(router *gin.RouterGroup, rcaEngineForEndpoints rca.RCAEngine) {
	// Create correlation engine
	correlationEngine := services.NewCorrelationEngine(
		s.vmServices.Metrics,
		s.vmServices.Logs,
		s.vmServices.Traces,
		s.cache,
		s.logger,
	)

	// Create unified query engine
	// Note: BleveSearchService is optional, can be nil if not configured
	unifiedEngine := services.NewUnifiedQueryEngine(
		s.vmServices.Metrics,
		s.vmServices.Logs,
		s.vmServices.Traces,
		correlationEngine,
		nil, // BleveSearchService - optional, can be configured later
		s.cache,
		s.logger,
	)

	// Create unified query handler
	unifiedHandler := handlers.NewUnifiedQueryHandler(unifiedEngine, s.logger)

	// Register unified query routes
	unifiedGroup := router.Group("/unified")
	{
		unifiedGroup.POST("/query", unifiedHandler.HandleUnifiedQuery)
		unifiedGroup.POST("/correlation", unifiedHandler.HandleUnifiedCorrelation)
		unifiedGroup.POST("/failures/detect", unifiedHandler.HandleFailureDetection)
		unifiedGroup.POST("/failures/correlate", unifiedHandler.HandleTransactionFailureCorrelation)
		unifiedGroup.GET("/metadata", unifiedHandler.HandleQueryMetadata)
		unifiedGroup.GET("/health", unifiedHandler.HandleHealthCheck)
		unifiedGroup.POST("/search", unifiedHandler.HandleUnifiedSearch)
		unifiedGroup.GET("/stats", unifiedHandler.HandleUnifiedStats)
		// Phase 4: RCA endpoint
		unifiedGroup.POST("/rca", func(c *gin.Context) {
			// Create a temporary RCA handler just for this endpoint
			rcaServiceGraph := services.NewServiceGraphService(s.vmServices.Metrics, s.logger)
			rcaHandler := handlers.NewRCAHandler(s.vmServices.Logs, rcaServiceGraph, s.cache, s.logger, rcaEngineForEndpoints)
			rcaHandler.HandleComputeRCA(c)
		})
	}

	// Register UQL routes
	uqlGroup := router.Group("/uql")
	{
		uqlGroup.POST("/query", unifiedHandler.HandleUQLQuery)
		uqlGroup.POST("/validate", unifiedHandler.HandleUQLValidate)
		uqlGroup.POST("/explain", unifiedHandler.HandleUQLExplain)
	}

	s.logger.Info("Unified query engine initialized and routes registered")
}

// noOpAnomalyProvider is a minimal implementation of AnomalyEventsProvider for Phase 4
type noOpAnomalyProvider struct{}

func (p *noOpAnomalyProvider) GetAnomalies(
	ctx context.Context,
	startTime time.Time,
	endTime time.Time,
	services []string,
) ([]*rca.AnomalyEvent, error) {
	// Return empty list; in production, this would query VictoriaMetrics/VictoriaLogs
	return make([]*rca.AnomalyEvent, 0), nil
}

func (s *Server) Start(ctx context.Context) error {
	// Validate port before starting
	if s.config.Port < 0 || s.config.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", s.config.Port)
	}

	// Start metrics metadata synchronizer
	if s.metricsMetadataSynchronizer != nil {
		s.logger.Info("Starting metrics metadata synchronizer")
		if err := s.metricsMetadataSynchronizer.Start(ctx); err != nil {
			s.logger.Error("Failed to start metrics metadata synchronizer", "error", err)
			return fmt.Errorf("failed to start synchronizer: %w", err)
		}
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("MIRADOR-CORE REST API server starting", "port", s.config.Port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errCh:
		return fmt.Errorf("server failed: %w", err)
	case <-ctx.Done():
		s.logger.Info("Shutting down MIRADOR-CORE gracefully")
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop metrics metadata synchronizer
	if s.metricsMetadataSynchronizer != nil {
		s.logger.Info("Stopping metrics metadata synchronizer")
		if err := s.metricsMetadataSynchronizer.Stop(); err != nil {
			s.logger.Error("Failed to stop metrics metadata synchronizer", "error", err)
		}
	}

	// Shutdown tracer provider
	if s.tracerProvider != nil {
		s.logger.Info("Shutting down tracer provider")
		if err := s.tracerProvider.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("Failed to shutdown tracer provider", "error", err)
		}
	}

	return s.httpServer.Shutdown(shutdownCtx)
}

// initializeMetricsMetadataComponents initializes the metrics metadata indexer and synchronizer
func (s *Server) initializeMetricsMetadataComponents() error {
	// Create Valkey metadata store
	metadataStore := bleve.NewValkeyMetadataStore(s.cache, s.logger)

	// Create document mapper
	documentMapper := mapping.NewBleveDocumentMapper(s.logger)

	// Create tiered storage (for now, use nil disk store - in-memory only)
	// TODO: Implement proper disk storage for production
	storage := storage.NewTieredStore(nil, 1000, 5*time.Minute, s.logger)

	// Create shard manager
	shardManager := bleve.NewShardManager(
		s.config.Search.Bleve.MetricsSync.ShardCount, // configurable shard count
		storage,
		metadataStore,
		documentMapper,
		s.logger,
		s.config.Search.Bleve.IndexPath,
	)

	// Create metrics metadata indexer
	s.metricsMetadataIndexer = services.NewMetricsMetadataIndexer(
		s.vmServices.Metrics,
		shardManager,
		documentMapper,
		s.logger,
	)

	// Create metrics metadata synchronizer with configurable settings
	syncConfig := &models.MetricMetadataSyncConfig{
		Enabled:           s.config.Search.Bleve.MetricsSync.Enabled,
		Strategy:          s.parseSyncStrategy(s.config.Search.Bleve.MetricsSync.Strategy),
		Interval:          s.config.Search.Bleve.MetricsSync.Interval,
		FullSyncInterval:  s.config.Search.Bleve.MetricsSync.FullSyncInterval,
		BatchSize:         s.config.Search.Bleve.MetricsSync.BatchSize,
		MaxRetries:        s.config.Search.Bleve.MetricsSync.MaxRetries,
		RetryDelay:        s.config.Search.Bleve.MetricsSync.RetryDelay,
		TimeRangeLookback: s.config.Search.Bleve.MetricsSync.TimeRangeLookback,
	}

	s.metricsMetadataSynchronizer = services.NewMetricsMetadataSynchronizer(
		s.metricsMetadataIndexer,
		s.cache,
		syncConfig,
		s.logger,
	)

	return nil
}

// parseSyncStrategy converts string strategy to enum
func (s *Server) parseSyncStrategy(strategy string) models.SyncStrategy {
	switch strategy {
	case "full":
		return models.SyncStrategyFull
	case "hybrid":
		return models.SyncStrategyHybrid
	case "incremental":
		fallthrough
	default:
		return models.SyncStrategyIncremental
	}
}

// Handler returns the underlying Gin engine so tests (or embedders) can mount it.
func (s *Server) Handler() http.Handler {
	return s.router
}
