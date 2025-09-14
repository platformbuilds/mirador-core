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
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Server struct {
	config      *config.Config
	logger      logger.Logger
	cache       cache.ValkeyCluster
	grpcClients *clients.GRPCClients
	vmServices  *services.VictoriaMetricsServices
	schemaRepo  repo.SchemaStore
	router      *gin.Engine
	httpServer  *http.Server
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

	server := &Server{
		config:      cfg,
		logger:      log,
		cache:       valleyCache,
		grpcClients: grpcClients,
		vmServices:  vmServices,
		schemaRepo:  schemaRepo,
		router:      router,
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

	// MetricsQL endpoints (VictoriaMetrics integration)
	var metricsHandler *handlers.MetricsQLHandler
	if s.schemaRepo != nil {
		metricsHandler = handlers.NewMetricsQLHandlerWithSchema(s.vmServices.Metrics, s.cache, s.logger, s.schemaRepo)
	} else {
		metricsHandler = handlers.NewMetricsQLHandler(s.vmServices.Metrics, s.cache, s.logger)
	}
	v1.POST("/query", metricsHandler.ExecuteQuery)
	v1.POST("/query_range", metricsHandler.ExecuteRangeQuery)
	v1.GET("/series", metricsHandler.GetSeries)
	v1.GET("/labels", metricsHandler.GetLabels)
	v1.GET("/metrics/names", metricsHandler.GetMetricNames)
	// Back-compat aliases at root so Swagger with base "/" also works
	s.router.POST("/query", metricsHandler.ExecuteQuery)
	s.router.POST("/query_range", metricsHandler.ExecuteRangeQuery)
	s.router.GET("/series", metricsHandler.GetSeries)
	s.router.GET("/labels", metricsHandler.GetLabels)
	s.router.GET("/metrics/names", metricsHandler.GetMetricNames)
	v1.GET("/label/:name/values", metricsHandler.GetLabelValues)

	// LogsQL endpoints (VictoriaLogs integration)
	logsHandler := handlers.NewLogsQLHandler(s.vmServices.Logs, s.cache, s.logger)
	v1.POST("/logs/query", logsHandler.ExecuteQuery)
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
	tracesHandler := handlers.NewTracesHandler(s.vmServices.Traces, s.cache, s.logger)
	v1.GET("/traces/services", tracesHandler.GetServices)
	v1.GET("/traces/services/:service/operations", tracesHandler.GetOperations)
	v1.GET("/traces/:traceId", tracesHandler.GetTrace)
	v1.GET("/traces/:traceId/flamegraph", tracesHandler.GetFlameGraph)
	v1.POST("/traces/search", tracesHandler.SearchTraces)
	v1.POST("/traces/flamegraph/search", tracesHandler.SearchFlameGraph)

	// AI PREDICT-ENGINE endpoints (gRPC + protobuf communication)
	predictHandler := handlers.NewPredictHandler(s.grpcClients.PredictEngine, s.vmServices.Logs, s.cache, s.logger)
	v1.GET("/predict/health", predictHandler.GetHealth)
	v1.POST("/predict/analyze", predictHandler.AnalyzeFractures) // Predicts fracture/fatigue
	v1.GET("/predict/fractures", predictHandler.GetPredictedFractures)
	v1.GET("/predict/models", predictHandler.GetActiveModels)

	// AI RCA-ENGINE endpoints (correlation with red anchors pattern)
	rcaHandler := handlers.NewRCAHandler(s.grpcClients.RCAEngine, s.vmServices.Logs, s.cache, s.logger)
	v1.GET("/rca/correlations", rcaHandler.GetActiveCorrelations)
	v1.POST("/rca/investigate", rcaHandler.StartInvestigation)
	v1.GET("/rca/patterns", rcaHandler.GetFailurePatterns)
	v1.POST("/rca/store", rcaHandler.StoreCorrelation) // Store correlation back to VictoriaLogs

	// Configuration endpoints (user-driven settings storage)
	configHandler := handlers.NewConfigHandler(s.cache, s.logger)
	v1.GET("/config/datasources", configHandler.GetDataSources)
	v1.POST("/config/datasources", configHandler.AddDataSource)
	v1.GET("/config/user-settings", configHandler.GetUserSettings)
	v1.PUT("/config/user-settings", configHandler.UpdateUserSettings)
	v1.GET("/config/integrations", configHandler.GetIntegrations)

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

	// WebSocket streams (metrics, alerts, predictions)
	ws := handlers.NewWebSocketHandler(s.logger)
	v1.GET("/ws/metrics", ws.HandleMetricsStream)
	v1.GET("/ws/alerts", ws.HandleAlertsStream)
	v1.GET("/ws/predictions", ws.HandlePredictionsStream)

	// Schema definitions (Weaviate-backed once enabled)
	if s.schemaRepo != nil {
		schemaHandler := handlers.NewSchemaHandler(s.schemaRepo, s.vmServices.Metrics, s.vmServices.Logs, s.cache, s.logger, s.config.Uploads.BulkMaxBytes)
		v1.POST("/schema/metrics", schemaHandler.UpsertMetric)
		v1.POST("/schema/metrics/bulk", schemaHandler.BulkUpsertMetricsCSV)
		v1.GET("/schema/metrics/bulk/sample", schemaHandler.SampleCSV)
		v1.GET("/schema/metrics/:metric", schemaHandler.GetMetric)
		v1.GET("/schema/metrics/:metric/versions", schemaHandler.ListMetricVersions)
		v1.POST("/schema/logs/fields", schemaHandler.UpsertLogField)
		v1.POST("/schema/logs/fields/bulk", schemaHandler.BulkUpsertLogFieldsCSV)
		v1.GET("/schema/logs/fields/bulk/sample", schemaHandler.SampleCSVLogFields)
		v1.GET("/schema/logs/fields/:field", schemaHandler.GetLogField)
		v1.GET("/schema/logs/fields/:field/versions", schemaHandler.ListLogFieldVersions)
		v1.GET("/schema/logs/fields/:field/versions/:version", schemaHandler.GetLogFieldVersion)
		v1.GET("/schema/metrics/:metric/versions/:version", schemaHandler.GetMetricVersion)
		// Traces: services and operations schema
		v1.POST("/schema/traces/services", schemaHandler.UpsertTraceService)
		v1.POST("/schema/traces/services/bulk", schemaHandler.BulkUpsertTraceServicesCSV)
		v1.GET("/schema/traces/services/:service", schemaHandler.GetTraceService)
		v1.GET("/schema/traces/services/:service/versions", schemaHandler.ListTraceServiceVersions)
		v1.GET("/schema/traces/services/:service/versions/:version", schemaHandler.GetTraceServiceVersion)
		v1.POST("/schema/traces/operations", schemaHandler.UpsertTraceOperation)
		v1.POST("/schema/traces/operations/bulk", schemaHandler.BulkUpsertTraceOperationsCSV)
		v1.GET("/schema/traces/services/:service/operations/:operation", schemaHandler.GetTraceOperation)
		v1.GET("/schema/traces/services/:service/operations/:operation/versions", schemaHandler.ListTraceOperationVersions)
		v1.GET("/schema/traces/services/:service/operations/:operation/versions/:version", schemaHandler.GetTraceOperationVersion)
	}
}

func (s *Server) Start(ctx context.Context) error {
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

	// Close gRPC connections
	if err := s.grpcClients.Close(); err != nil {
		s.logger.Error("Failed to close gRPC clients", "error", err)
	}

	return s.httpServer.Shutdown(shutdownCtx)
}

// Handler returns the underlying Gin engine so tests (or embedders) can mount it.
func (s *Server) Handler() http.Handler {
	return s.router
}
