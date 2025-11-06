package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/api/handlers"
	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/tracing"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/mapping"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve/storage"
	"github.com/platformbuilds/mirador-core/internal/utils/search"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Server struct {
	config                      *config.Config
	logger                      logger.Logger
	cache                       cache.ValkeyCluster
	grpcClients                 *clients.GRPCClients
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
	valleyCache cache.ValkeyCluster,
	grpcClients *clients.GRPCClients,
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
		cache:          valleyCache,
		grpcClients:    grpcClients,
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

	// Authentication (can be disabled via config.auth.enabled)
	if s.config.Auth.Enabled {
		s.router.Use(middleware.AuthMiddleware(s.config.Auth, s.cache))
	} else {
		s.router.Use(middleware.NoAuthMiddleware())
		s.logger.Warn("Authentication is DISABLED by configuration; requests will use anonymous/default context")
	}

	// OpenAPI specification endpoints
	s.router.StaticFile("/api/openapi.yaml", "api/openapi.yaml")
	s.router.GET("/api/openapi.json", handlers.GetOpenAPISpec)

	// Swagger UI via gin-swagger (serves Swagger UI using external openapi.yaml)
	// Visit /swagger/index.html
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api/openapi.yaml")))

	// Prometheus metrics endpoint
	monitoring.SetupPrometheusMetrics(s.router)
}

func (s *Server) setupRoutes() {
	// Create health handler instance
	healthHandler := handlers.NewHealthHandlerWithCache(s.grpcClients, s.vmServices, s.cache, s.logger)

	// Public health endpoints - now using handler instance methods
	s.router.GET("/health", healthHandler.HealthCheck)
	s.router.GET("/ready", healthHandler.ReadinessCheck)
	s.router.GET("/microservices/status", healthHandler.MicroservicesStatus)

	// Root redirect to Swagger UI for convenience
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/swagger/index.html")
	})

	// API v1 group (protected by RBAC)
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
	// New metrics endpoints under /metrics/*
	v1.POST("/metrics/query", metricsHandler.ExecuteQuery)
	v1.POST("/metrics/query_range", metricsHandler.ExecuteRangeQuery)
	// Back-compat (deprecated): keep old routes registered
	v1.POST("/query", metricsHandler.ExecuteQuery)
	v1.POST("/query_range", metricsHandler.ExecuteRangeQuery)
	v1.GET("/series", metricsHandler.GetSeries)
	v1.GET("/labels", metricsHandler.GetLabels)
	v1.GET("/metrics/names", metricsHandler.GetMetricNames)
	v1.GET("/metrics/series", metricsHandler.GetSeries)
	v1.POST("/metrics/labels", metricsHandler.GetLabels)
	// Back-compat aliases at root so Swagger with base "/" also works
	// Back-compat aliases at root so Swagger with base "/" also works
	s.router.POST("/query", metricsHandler.ExecuteQuery)
	s.router.POST("/query_range", metricsHandler.ExecuteRangeQuery)
	s.router.GET("/series", metricsHandler.GetSeries)
	s.router.GET("/labels", metricsHandler.GetLabels)
	s.router.GET("/metrics/names", metricsHandler.GetMetricNames)
	s.router.GET("/metrics/series", metricsHandler.GetSeries)
	s.router.POST("/metrics/labels", metricsHandler.GetLabels)
	v1.GET("/label/:name/values", metricsHandler.GetLabelValues)

	// MetricsQL function query endpoints (hierarchical by category)
	queryHandler := handlers.NewMetricsQLQueryHandler(s.vmServices.Query, s.cache, s.logger)
	validationMiddleware := middleware.NewMetricsQLQueryValidationMiddleware(s.logger)

	// Rollup functions
	v1.POST("/metrics/query/rollup/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteRollupFunction)
	v1.POST("/metrics/query/rollup/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteRollupRangeFunction)
	// Transform functions
	v1.POST("/metrics/query/transform/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteTransformFunction)
	v1.POST("/metrics/query/transform/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteTransformRangeFunction)
	// Label functions
	v1.POST("/metrics/query/label/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteLabelFunction)
	v1.POST("/metrics/query/label/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteLabelRangeFunction)
	// Aggregate functions
	v1.POST("/metrics/query/aggregate/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteAggregateFunction)
	v1.POST("/metrics/query/aggregate/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteAggregateRangeFunction)

	// LogsQL endpoints (VictoriaLogs integration)
	logsHandler := handlers.NewLogsQLHandler(s.vmServices.Logs, s.cache, s.logger, s.searchRouter, s.config)
	v1.POST("/logs/query", s.searchThrottling.ThrottleLogsQuery(), logsHandler.ExecuteQuery)
	v1.GET("/logs/streams", logsHandler.GetStreams)
	v1.GET("/logs/fields", logsHandler.GetFields)
	v1.POST("/logs/export", logsHandler.ExportLogs)
	v1.POST("/logs/store", logsHandler.StoreEvent) // For AI engines to store JSON events

	// D3-friendly log endpoints for the upcoming UI (histogram, facets, search, tail)
	d3Logs := handlers.NewLogsHandler(s.vmServices.Logs, s.logger)
	v1.GET("/logs/histogram", d3Logs.Histogram)
	v1.GET("/logs/facets", d3Logs.Facets)
	v1.POST("/logs/search", d3Logs.Search)
	v1.GET("/logs/tail", d3Logs.TailWS) // WebSocket upgrade for tailing logs

	// VictoriaTraces endpoints (Jaeger-compatible HTTP API)
	tracesHandler := handlers.NewTracesHandler(s.vmServices.Traces, s.cache, s.logger, s.searchRouter, s.config)
	v1.GET("/traces/services", tracesHandler.GetServices)
	v1.GET("/traces/services/:service/operations", tracesHandler.GetOperations)
	v1.GET("/traces/:traceId", tracesHandler.GetTrace)
	v1.GET("/traces/:traceId/flamegraph", tracesHandler.GetFlameGraph)
	v1.POST("/traces/search", s.searchThrottling.ThrottleTracesQuery(), tracesHandler.SearchTraces)
	v1.POST("/traces/flamegraph/search", tracesHandler.SearchFlameGraph)

	// AI RCA-ENGINE endpoints (correlation with red anchors pattern)
	rcaServiceGraph := services.NewServiceGraphService(s.vmServices.Metrics, s.logger)
	rcaHandler := handlers.NewRCAHandler(s.grpcClients.RCAEngine, s.vmServices.Logs, rcaServiceGraph, s.cache, s.logger)
	v1.GET("/rca/correlations", rcaHandler.GetActiveCorrelations)
	v1.POST("/rca/investigate", rcaHandler.StartInvestigation)
	v1.GET("/rca/patterns", rcaHandler.GetFailurePatterns)
	v1.POST("/rca/service-graph", rcaHandler.GetServiceGraph)
	v1.POST("/rca/store", rcaHandler.StoreCorrelation) // Store correlation back to VictoriaLogs

	// Configuration endpoints (user-driven settings storage)
	dynamicConfigService := services.NewDynamicConfigService(s.cache, s.logger)
	configHandler := handlers.NewConfigHandler(s.cache, s.logger, dynamicConfigService, s.grpcClients)
	v1.GET("/config/datasources", configHandler.GetDataSources)
	v1.POST("/config/datasources", configHandler.AddDataSource)
	v1.GET("/config/user-settings", configHandler.GetUserSettings)
	v1.PUT("/config/user-settings", configHandler.UpdateUserSettings)
	v1.GET("/config/integrations", configHandler.GetIntegrations)

	// Runtime feature flag endpoints
	v1.GET("/config/features", configHandler.GetFeatureFlags)
	v1.PUT("/config/features", configHandler.UpdateFeatureFlags)
	v1.POST("/config/features/reset", configHandler.ResetFeatureFlags)

	// Dynamic gRPC endpoint configuration
	v1.GET("/config/grpc/endpoints", configHandler.GetGRPCEndpoints)
	v1.PUT("/config/grpc/endpoints", configHandler.UpdateGRPCEndpoints)
	v1.POST("/config/grpc/endpoints/reset", configHandler.ResetGRPCEndpoints)

	// Session management (Valkey cluster caching)
	sessionHandler := handlers.NewSessionHandler(s.cache, s.logger)
	v1.GET("/sessions/active", sessionHandler.GetActiveSessions)
	v1.POST("/sessions/invalidate", sessionHandler.InvalidateSession)
	v1.GET("/sessions/user/:userId", sessionHandler.GetUserSessions)

	// RBAC endpoints
	rbacHandler := handlers.NewRBACHandler(s.cache, s.logger)
	v1.GET("/rbac/roles", rbacHandler.GetRoles)
	v1.POST("/rbac/roles", rbacHandler.CreateRole)
	v1.PUT("/rbac/users/:userId/roles", rbacHandler.AssignUserRoles)

	// WebSocket streams (metrics, alerts)
	ws := handlers.NewWebSocketHandler(s.logger)
	v1.GET("/ws/metrics", ws.HandleMetricsStream)
	v1.GET("/ws/alerts", ws.HandleAlertsStream)

	// Schema definitions (Weaviate-backed once enabled)
	if s.schemaRepo != nil {
		schemaHandler := handlers.NewSchemaHandler(s.schemaRepo, s.vmServices.Metrics, s.vmServices.Logs, s.cache, s.logger, s.config.Uploads.BulkMaxBytes)
		v1.POST("/schema/metrics/:metric/labels", schemaHandler.UpsertMetricLabel)
		v1.GET("/schema/metrics/:metric", schemaHandler.GetMetric)
		v1.GET("/schema/metrics/:metric/versions", schemaHandler.ListMetricVersions)
		v1.DELETE("/schema/metrics/:metric", schemaHandler.DeleteMetric)
		v1.POST("/schema/metrics", schemaHandler.UpsertMetric)
		v1.POST("/schema/metrics/bulk", schemaHandler.BulkUpsertMetricsCSV)
		v1.GET("/schema/metrics/bulk/sample", schemaHandler.SampleCSV)
		v1.POST("/schema/logs/fields", schemaHandler.UpsertLogField)
		v1.POST("/schema/logs/fields/bulk", schemaHandler.BulkUpsertLogFieldsCSV)
		v1.GET("/schema/logs/fields/bulk/sample", schemaHandler.SampleCSVLogFields)
		v1.GET("/schema/logs/fields/:field", schemaHandler.GetLogField)
		v1.GET("/schema/logs/fields/:field/versions", schemaHandler.ListLogFieldVersions)
		v1.GET("/schema/logs/fields/:field/versions/:version", schemaHandler.GetLogFieldVersion)
		v1.DELETE("/schema/logs/fields/:field", schemaHandler.DeleteLogField)
		v1.GET("/schema/metrics/:metric/versions/:version", schemaHandler.GetMetricVersion)
		// Independent label schema
		v1.POST("/schema/labels", schemaHandler.UpsertLabel)
		v1.GET("/schema/labels/:name", schemaHandler.GetLabel)
		v1.GET("/schema/labels/:name/versions", schemaHandler.ListLabelVersions)
		v1.GET("/schema/labels/:name/versions/:version", schemaHandler.GetLabelVersion)
		v1.DELETE("/schema/labels/:name", schemaHandler.DeleteLabel)
		v1.POST("/schema/labels/bulk", schemaHandler.BulkUpsertLabelsCSV)
		v1.GET("/schema/labels/bulk/sample", schemaHandler.SampleCSVLabels)
		// Traces: services and operations schema
		v1.POST("/schema/traces/services", schemaHandler.UpsertTraceService)
		v1.POST("/schema/traces/services/bulk", schemaHandler.BulkUpsertTraceServicesCSV)
		v1.GET("/schema/traces/services/bulk/sample", schemaHandler.SampleCSVTraceServices)
		v1.GET("/schema/traces/services/:service", schemaHandler.GetTraceService)
		v1.GET("/schema/traces/services/:service/versions", schemaHandler.ListTraceServiceVersions)
		v1.GET("/schema/traces/services/:service/versions/:version", schemaHandler.GetTraceServiceVersion)
		v1.DELETE("/schema/traces/services/:service", schemaHandler.DeleteTraceService)
		v1.POST("/schema/traces/operations", schemaHandler.UpsertTraceOperation)
		v1.POST("/schema/traces/operations/bulk", schemaHandler.BulkUpsertTraceOperationsCSV)
		v1.GET("/schema/traces/operations/bulk/sample", schemaHandler.SampleCSVTraceOperations)
		v1.GET("/schema/traces/services/:service/operations/:operation", schemaHandler.GetTraceOperation)
		v1.GET("/schema/traces/services/:service/operations/:operation/versions", schemaHandler.ListTraceOperationVersions)
		v1.GET("/schema/traces/services/:service/operations/:operation/versions/:version", schemaHandler.GetTraceOperationVersion)
		v1.DELETE("/schema/traces/services/:service/operations/:operation", schemaHandler.DeleteTraceOperation)
	}

	// Unified Query Engine (Phase 1.5: Unified API Implementation)
	if s.config.UnifiedQuery.Enabled {
		s.setupUnifiedQueryEngine(v1)
	}

	// Metrics Metadata Discovery API (Phase 2: Metrics Metadata Integration)
	if s.metricsMetadataIndexer != nil {
		metricsSearchHandler := handlers.NewMetricsSearchHandler(s.metricsMetadataIndexer, s.logger)
		v1.POST("/metrics/search", metricsSearchHandler.HandleMetricsSearch)
		v1.POST("/metrics/sync", metricsSearchHandler.HandleMetricsSync)
		v1.GET("/metrics/health", metricsSearchHandler.HandleMetricsHealth)
	}

	// Metrics Metadata Synchronization API (Phase 2: Metrics Metadata Integration)
	if s.metricsMetadataSynchronizer != nil {
		syncHandler := handlers.NewMetricsSyncHandler(s.metricsMetadataSynchronizer, s.logger)
		v1.POST("/metrics/sync/:tenantId", syncHandler.HandleSyncNow)
		v1.GET("/metrics/sync/:tenantId/state", syncHandler.HandleGetSyncState)
		v1.GET("/metrics/sync/:tenantId/status", syncHandler.HandleGetSyncStatus)
		v1.PUT("/metrics/sync/config", syncHandler.HandleUpdateConfig)
	}
}

// setupUnifiedQueryEngine sets up the unified query engine and registers its routes
func (s *Server) setupUnifiedQueryEngine(router *gin.RouterGroup) {
	// Create correlation engine
	correlationEngine := services.NewCorrelationEngine(
		s.vmServices.Metrics,
		s.vmServices.Logs,
		s.vmServices.Traces,
		s.cache,
		s.logger,
	)

	// Create unified query engine
	unifiedEngine := services.NewUnifiedQueryEngine(
		s.vmServices.Metrics,
		s.vmServices.Logs,
		s.vmServices.Traces,
		correlationEngine,
		s.cache,
		s.logger,
	)

	// Create unified query handler
	unifiedHandler := handlers.NewUnifiedQueryHandler(unifiedEngine, s.logger)

	// Register unified query routes
	unifiedHandler.RegisterRoutes(router)

	s.logger.Info("Unified query engine initialized and routes registered")
}

func (s *Server) Start(ctx context.Context) error {
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
		s.metricsMetadataSynchronizer.Stop()
	}

	// Shutdown tracer provider
	if s.tracerProvider != nil {
		s.logger.Info("Shutting down tracer provider")
		if err := s.tracerProvider.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("Failed to shutdown tracer provider", "error", err)
		}
	}

	// Close gRPC connections
	if err := s.grpcClients.Close(); err != nil {
		s.logger.Error("Failed to close gRPC clients", "error", err)
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
