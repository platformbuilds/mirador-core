package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	wv "github.com/weaviate/weaviate-go-client/v5/weaviate"
	"go.uber.org/zap"

	_ "github.com/platformbuilds/mirador-core/api" // Import generated Swagger docs
	"github.com/platformbuilds/mirador-core/internal/api/handlers"
	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/bootstrap"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
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
	"github.com/platformbuilds/mirador-core/internal/weavstore"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type Server struct {
	config                      *config.Config
	logger                      logger.Logger
	internalLogger              logging.Logger
	cache                       cache.ValkeyCluster
	vmServices                  *services.VictoriaMetricsServices
	schemaRepo                  repo.SchemaStore
	kpiRepo                     repo.KPIRepo
	searchRouter                *search.SearchRouter
	searchThrottling            *middleware.SearchQueryThrottlingMiddleware
	metricsMetadataIndexer      services.MetricsMetadataIndexer
	metricsMetadataSynchronizer services.MetricsMetadataSynchronizer
	router                      *gin.Engine
	httpServer                  *http.Server
	tracerProvider              *tracing.TracerProvider
	weaviateClient              *wv.Client
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

	server := &Server{
		config:         cfg,
		logger:         log,
		internalLogger: logging.FromCoreLogger(log),
		cache:          valkeyCache,
		vmServices:     vmServices,
		schemaRepo:     schemaRepo,
		router:         router,
	}

	// Initialize subsystems using helper methods to keep NewServer simple and
	// reduce cyclomatic complexity.
	server.tracerProvider = server.initTracing(cfg, log)

	kpiStore, zapLogger := server.initWeaviateStore(cfg, log)
	// Pass Valkey cache to repo wiring; metadata store may be wired later.
	server.initKPIRepo(schemaRepo, kpiStore, zapLogger)

	// Bootstrap telemetry via the repo layer (keeps models out of bootstrap)
	if err := bootstrap.BootstrapTelemetryStandards(context.Background(), &cfg.Engine, server.kpiRepo, server.internalLogger); err != nil {
		log.Warn("failed to bootstrap telemetry standards", "error", err)
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

// initTracing sets up distributed tracing if enabled in config.
func (s *Server) initTracing(cfg *config.Config, log logger.Logger) *tracing.TracerProvider {
	if !cfg.Monitoring.TracingEnabled {
		return nil
	}
	tp, err := tracing.NewTracerProvider(
		"mirador-core",
		"v7.0.0",
		cfg.Monitoring.JaegerEndpoint,
	)
	if err != nil {
		log.Error("Failed to initialize tracer provider", "error", err)
		return nil
	}
	log.Info("Distributed tracing initialized", "endpoint", cfg.Monitoring.JaegerEndpoint)
	tracing.InitGlobalTracer("mirador-core")
	return tp
}

// initWeaviateStore initializes a Weaviate client/store if enabled and returns
// the store and the zap logger used for it. It also stores the client in the Server
// for later use (e.g., in failure store initialization).
func (s *Server) initWeaviateStore(cfg *config.Config, log logger.Logger) (*weavstore.WeaviateKPIStore, *zap.Logger) {
	if !cfg.Weaviate.Enabled {
		return nil, zap.NewNop()
	}
	hostPort := cfg.Weaviate.Host
	if cfg.Weaviate.Port != 0 {
		hostPort = fmt.Sprintf("%s:%d", cfg.Weaviate.Host, cfg.Weaviate.Port)
	}
	conf := wv.Config{Scheme: cfg.Weaviate.Scheme, Host: hostPort}
	if client, err := wv.NewClient(conf); err == nil {
		s.weaviateClient = client
		zapLogger := logging.ExtractZapLogger(log)
		store := weavstore.NewWeaviateKPIStore(client, zapLogger)
		return store, zapLogger
	}
	log.Error("Failed to create Weaviate v5 client", "error", fmt.Errorf("weaviate client init failed"))
	return nil, zap.NewNop()
}

// initKPIRepo wires the KPIRepo: prefer schemaRepo if it implements KPIRepo,
// otherwise construct DefaultKPIRepo using the provided weaviate store and zap logger.
func (s *Server) initKPIRepo(schemaRepo repo.SchemaStore, kpiStore *weavstore.WeaviateKPIStore, zapLogger *zap.Logger) {
	var kpiRepo repo.KPIRepo
	if schemaRepo != nil {
		if kp, ok := schemaRepo.(repo.KPIRepo); ok {
			kpiRepo = kp
		}
	}
	if kpiRepo == nil {
		if zapLogger == nil {
			zapLogger = zap.NewNop()
		}
		// Pass Valkey cache to repo. Metadata store will be wired later
		kpiRepo = repo.NewDefaultKPIRepo(kpiStore, zapLogger, s.cache, nil)
	}
	s.kpiRepo = kpiRepo
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
	// var metricsHandler *handlers.MetricsQLHandler
	// if s.schemaRepo != nil {
	//	metricsHandler = handlers.NewMetricsQLHandlerWithSchema(s.vmServices.Metrics, s.cache, s.logger, s.schemaRepo)
	// } else {
	//	metricsHandler = handlers.NewMetricsQLHandler(s.vmServices.Metrics, s.cache, s.logger)
	// }

	// Legacy standalone Metrics endpoints are deregistered.
	// Metrics queries should be performed via the Unified Query API (UQL) or
	// via the metrics metadata/search endpoints when enabled.

	// Metrics function endpoints (rollup/transform/label/aggregate) are deregistered.

	// Logs/LogsQL endpoints are deregistered. Log queries should use Unified UQL instead.

	// D3-specific log endpoints and WebSocket tail are deregistered.

	// Traces (Jaeger-compatible) endpoints are deregistered.

	// Phase 4: Create RCA Engine for /rca endpoints
	// Create an empty service graph (will be enhanced from metrics in production)
	rcaServiceGraphForEngine := rca.NewServiceGraph()

	// Ensure KPI repo is wired so the correlation engine can operate KPI-first.
	if s.kpiRepo == nil {
		s.logger.Warn("KPI repo is not configured; correlation engine will fall back to config probes")
	}

	// Create a correlation engine-backed anomaly provider so RCA endpoints
	// can fetch real failure signals from the unified engines.
	correlationEngineForProvider := services.NewCorrelationEngine(
		s.vmServices.Metrics,
		s.vmServices.Logs,
		s.vmServices.Traces,
		s.kpiRepo,
		s.cache,
		s.logger,
		s.config.Engine,
	)

	anomalyProviderForEngine := &correlationAnomalyProvider{ce: correlationEngineForProvider, logger: s.logger}

	// Create anomaly collector and candidate cause service
	incidentAnomalyCollectorForEngine := rca.NewIncidentAnomalyCollector(
		anomalyProviderForEngine,
		rcaServiceGraphForEngine,
		s.logger,
	)
	candidateCauseServiceForEngine := rca.NewCandidateCauseService(incidentAnomalyCollectorForEngine, s.logger)

	// Create RCA engine for endpoints (wire correlation engine for TimeRange API)
	rcaEngineForEndpoints := rca.NewRCAEngine(candidateCauseServiceForEngine, rcaServiceGraphForEngine, s.logger, s.config.Engine, correlationEngineForProvider)

	// KPI APIs (primary interface for schema definitions)
	if s.kpiRepo != nil {
		kpiHandler := handlers.NewKPIHandler(s.config, s.kpiRepo, s.cache, s.logger)
		if kpiHandler != nil {
			// KPI Definitions API
			kpiDefsGroup := v1.Group("/kpi/defs")
			{
				kpiDefsGroup.GET("", kpiHandler.GetKPIDefinitions)
				kpiDefsGroup.POST("", kpiHandler.CreateOrUpdateKPIDefinition)
				kpiDefsGroup.POST("/bulk-json", kpiHandler.BulkIngestJSON)
				kpiDefsGroup.POST("/bulk-csv", kpiHandler.BulkIngestCSV)
				kpiDefsGroup.DELETE("/:id", kpiHandler.DeleteKPIDefinition)
			}
		}
	}

	// Unified Query Engine (Phase 1.5: Unified API Implementation)
	if s.config.UnifiedQuery.Enabled {
		s.setupUnifiedQueryEngine(v1, rcaEngineForEndpoints)
	}

	// Metrics metadata indexing/search/sync endpoints are deregistered.
}

// setupUnifiedQueryEngine sets up the unified query engine and registers its routes
func (s *Server) setupUnifiedQueryEngine(router *gin.RouterGroup, rcaEngineForEndpoints rca.RCAEngine) {
	// Create correlation engine
	correlationEngine := services.NewCorrelationEngine(
		s.vmServices.Metrics,
		s.vmServices.Logs,
		s.vmServices.Traces,
		s.kpiRepo,
		s.cache,
		s.logger,
		s.config.Engine,
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
	unifiedHandler := handlers.NewUnifiedQueryHandler(unifiedEngine, s.logger, s.kpiRepo, s.config.Engine)

	// Initialize failure store if Weaviate is enabled and client is available
	if s.config.Weaviate.Enabled && s.weaviateClient != nil {
		zapLogger := logging.ExtractZapLogger(s.logger)
		failureStore := weavstore.NewWeaviateFailureStore(s.weaviateClient, zapLogger)
		unifiedHandler.SetFailureStore(failureStore)
	}

	// Create RCA handler for unified RCA endpoints
	rcaServiceGraph := services.NewServiceGraphService(s.vmServices.Metrics, s.logger)
	rcaHandler := handlers.NewRCAHandler(s.vmServices.Logs, rcaServiceGraph, s.cache, s.logger, rcaEngineForEndpoints, s.config.Engine, s.kpiRepo)

	// Create MIRA handler for AI-powered RCA explanations (if enabled)
	var miraHandler *handlers.MIRARCAHandler
	var miraServiceForAsync services.MIRAService // Store for async handler
	if s.config.MIRA.Enabled {
		miraService, err := services.NewMIRAService(s.config.MIRA, s.logger, s.cache)
		if err != nil {
			s.logger.Warn("Failed to initialize MIRA service", "error", err)
		} else {
			// Wrap with caching layer
			cachedMIRAService := services.NewCachedMIRAService(miraService, s.config.MIRA.Cache, s.cache, s.logger)
			miraHandler = handlers.NewMIRARCAHandler(cachedMIRAService, s.config.MIRA, s.logger)
			miraServiceForAsync = cachedMIRAService // Store for async handler
			s.logger.Info("MIRA service initialized", "provider", s.config.MIRA.Provider, "model", miraService.GetModelName())
		}
	}

	// Register unified query routes
	unifiedGroup := router.Group("/unified")
	{
		unifiedGroup.POST("/query", unifiedHandler.HandleUnifiedQuery)
		unifiedGroup.POST("/correlation", unifiedHandler.HandleUnifiedCorrelation)
		unifiedGroup.POST("/failures/detect", unifiedHandler.HandleFailureDetection)
		unifiedGroup.POST("/failures/correlate", unifiedHandler.HandleTransactionFailureCorrelation)
		unifiedGroup.POST("/failures/list", unifiedHandler.HandleGetFailures)
		unifiedGroup.POST("/failures/get", unifiedHandler.HandleGetFailureDetail)
		unifiedGroup.POST("/failures/delete", unifiedHandler.HandleDeleteFailure)
		unifiedGroup.GET("/metadata", unifiedHandler.HandleQueryMetadata)
		unifiedGroup.GET("/health", unifiedHandler.HandleHealthCheck)
		unifiedGroup.POST("/search", unifiedHandler.HandleUnifiedSearch)
		unifiedGroup.GET("/stats", unifiedHandler.HandleUnifiedStats)
		// Phase 4: RCA endpoint
		unifiedGroup.POST("/rca", func(c *gin.Context) {
			rcaHandler.HandleComputeRCA(c)
		})
		// Migrated service-graph endpoint
		unifiedGroup.POST("/service-graph", func(c *gin.Context) {
			rcaHandler.GetServiceGraph(c)
		})
	}

	// Register UQL routes
	uqlGroup := router.Group("/uql")
	{
		uqlGroup.POST("/query", unifiedHandler.HandleUQLQuery)
		uqlGroup.POST("/validate", unifiedHandler.HandleUQLValidate)
		uqlGroup.POST("/explain", unifiedHandler.HandleUQLExplain)
	}

	// Register MIRA routes (AI-powered RCA explanations)
	if miraHandler != nil {
		miraGroup := router.Group("/mira")
		// Apply MIRA-specific rate limiting (stricter than default)
		miraGroup.Use(middleware.MIRARateLimiter(s.cache, s.config.MIRA.RateLimit))
		{
			// Sync API (blocks until completion, may timeout for large RCA)
			miraGroup.POST("/rca_analyze", miraHandler.HandleMIRARCAAnalyze)

			// Async API with dual persistence: Valkey (hot cache) + Weaviate (long-term storage)
			if miraServiceForAsync != nil {
				// Initialize Weaviate MIRA RCA store if Weaviate is enabled
				var mirarcaStore *weavstore.WeaviateMIRARCAStore
				if s.weaviateClient != nil {
					mirarcaStore = weavstore.NewWeaviateMIRARCAStore(s.weaviateClient, zap.L())
					s.logger.Info("MIRA RCA Weaviate store initialized for long-term task persistence")
				}

				miraAsyncHandler := handlers.NewMIRARCAAsyncHandler(miraServiceForAsync, s.config.MIRA, s.cache, mirarcaStore, s.logger)
				miraGroup.POST("/rca_analyze_async", miraAsyncHandler.HandleMIRARCAAnalyzeAsync)
				miraGroup.GET("/rca_analyze/:taskId", miraAsyncHandler.HandleGetTaskStatus)
			}
		}
		storageBackends := "valkey"
		if s.weaviateClient != nil {
			storageBackends = "valkey+weaviate"
		}
		s.logger.Info("MIRA routes registered with rate limiting",
			"enabled", s.config.MIRA.RateLimit.Enabled,
			"requests_per_minute", s.config.MIRA.RateLimit.RequestsPerMinute,
			"async_api", "enabled",
			"task_storage", storageBackends)
	}

	s.logger.Info("Unified query engine initialized and routes registered")
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

	// If repo is DefaultKPIRepo, wire the metadata store so repo-level
	// delete operations can remove Bleve metadata as part of cleanup.
	if dr, ok := s.kpiRepo.(*repo.DefaultKPIRepo); ok {
		dr.SetMetadataStore(metadataStore)
	}

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

	// Initialize shards immediately so the indexer can write to them.
	// If shard initialization fails (commonly due to disk permissions),
	// fall back to stub implementations so the server remains available
	// and doesn't return nil (which would cause a panic on Start()).
	if err := shardManager.InitializeShards(); err != nil {
		s.logger.Error("Failed to initialize Bleve shards; falling back to stub indexer/synchronizer", "error", err, "indexPath", s.config.Search.Bleve.IndexPath)
		// Use stub implementations to keep API surface available
		s.metricsMetadataIndexer = services.NewStubMetricsMetadataIndexer(s.logger)
		s.metricsMetadataSynchronizer = services.NewStubMetricsMetadataSynchronizer(s.logger)
		// Record a monitoring hint so operators can surface this condition.
		// TODO: emit a metric here if monitoring is configured.
		return nil
	}

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
