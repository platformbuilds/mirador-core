package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/platformbuilds/mirador-core/internal/api/handlers"
	"github.com/platformbuilds/mirador-core/internal/api/middleware"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
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
	grpcClients                 *clients.GRPCClients
	vmServices                  *services.VictoriaMetricsServices
	schemaRepo                  repo.SchemaStore
	rbacRepo                    rbac.RBACRepository
	searchRouter                *search.SearchRouter
	searchThrottling            *middleware.SearchQueryThrottlingMiddleware
	metricsMetadataIndexer      services.MetricsMetadataIndexer
	metricsMetadataSynchronizer services.MetricsMetadataSynchronizer
	rbacService                 *rbac.RBACService
	rbacEnforcer                *middleware.RBACEnforcer
	tenantIsolationMiddleware   *middleware.TenantIsolationMiddleware
	rbacBootstrap               *services.RBACBootstrapService
	router                      *gin.Engine
	httpServer                  *http.Server
	tracerProvider              *tracing.TracerProvider
}

func NewServer(
	cfg *config.Config,
	log logger.Logger,
	valkeyCache cache.ValkeyCluster,
	grpcClients *clients.GRPCClients,
	vmServices *services.VictoriaMetricsServices,
	schemaRepo repo.SchemaStore,
	rbacRepo rbac.RBACRepository,
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

	// Initialize RBAC components
	auditService := rbac.NewAuditService(rbacRepo)

	// Initialize RBAC cache repository using Valkey
	valkeyAdapter := rbac.NewValkeyClusterAdapter(valkeyCache)
	cacheRepo := rbac.NewValkeyRBACRepository(valkeyAdapter)

	rbacService := rbac.NewRBACService(rbacRepo, cacheRepo, auditService)

	// Initialize RBAC bootstrap service
	rbacBootstrap := services.NewRBACBootstrapService(rbacService, rbacRepo, log)

	// Initialize RBAC enforcer
	rbacEnforcer := middleware.NewRBACEnforcer(rbacService, valkeyCache, log)

	server := &Server{
		config:         cfg,
		logger:         log,
		cache:          valkeyCache,
		grpcClients:    grpcClients,
		vmServices:     vmServices,
		schemaRepo:     schemaRepo,
		rbacRepo:       rbacRepo,
		router:         router,
		tracerProvider: tracerProvider,
		rbacService:    rbacService,
		rbacEnforcer:   rbacEnforcer,
		rbacBootstrap:  rbacBootstrap,
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

	// Authentication (can be disabled via config.auth.enabled)
	if s.config.Auth.Enabled {
		s.router.Use(middleware.AuthMiddleware(s.config.Auth, s.cache))
	} else {
		s.router.Use(middleware.NoAuthMiddleware())
		s.logger.Warn("Authentication is DISABLED by configuration; requests will use anonymous/default context")
	}

	// Tenant isolation middleware (enforces multi-tenant access control)
	s.tenantIsolationMiddleware = middleware.NewTenantIsolationMiddleware(
		middleware.DefaultTenantIsolationConfig(),
		s.rbacService,
		s.logger,
	)
	s.router.Use(s.tenantIsolationMiddleware.TenantIsolation())

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
	// New metrics endpoints under /metrics/* - require metrics.read permission
	metricsGroup := v1.Group("/metrics")
	metricsGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}))
	{
		metricsGroup.POST("/query", metricsHandler.ExecuteQuery)
		metricsGroup.POST("/query_range", metricsHandler.ExecuteRangeQuery)
		metricsGroup.GET("/names", metricsHandler.GetMetricNames)
		metricsGroup.GET("/series", metricsHandler.GetSeries)
		metricsGroup.POST("/labels", metricsHandler.GetLabels)
		metricsGroup.GET("/label/:name/values", metricsHandler.GetLabelValues)
	}
	// Back-compat (deprecated): keep old routes registered with RBAC protection
	v1.POST("/query", s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}), metricsHandler.ExecuteQuery)
	v1.POST("/query_range", s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}), metricsHandler.ExecuteRangeQuery)
	s.router.POST("/query", s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}), metricsHandler.ExecuteQuery)
	s.router.POST("/query_range", s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}), metricsHandler.ExecuteRangeQuery)
	s.router.GET("/metrics/names", s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}), metricsHandler.GetMetricNames)
	s.router.GET("/metrics/series", s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}), metricsHandler.GetSeries)
	s.router.POST("/metrics/labels", s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}), metricsHandler.GetLabels)

	// MetricsQL function query endpoints (hierarchical by category)
	queryHandler := handlers.NewMetricsQLQueryHandler(s.vmServices.Query, s.cache, s.logger)
	validationMiddleware := middleware.NewMetricsQLQueryValidationMiddleware(s.logger)

	// Rollup functions - require metrics.read permission
	rollupGroup := v1.Group("/metrics/query/rollup")
	rollupGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}))
	{
		rollupGroup.POST("/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteRollupFunction)
		rollupGroup.POST("/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteRollupRangeFunction)
	}
	// Transform functions - require metrics.read permission
	transformGroup := v1.Group("/metrics/query/transform")
	transformGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}))
	{
		transformGroup.POST("/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteTransformFunction)
		transformGroup.POST("/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteTransformRangeFunction)
	}
	// Label functions - require metrics.read permission
	labelGroup := v1.Group("/metrics/query/label")
	labelGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}))
	{
		labelGroup.POST("/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteLabelFunction)
		labelGroup.POST("/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteLabelRangeFunction)
	}
	// Aggregate functions - require metrics.read permission
	aggregateGroup := v1.Group("/metrics/query/aggregate")
	aggregateGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}))
	{
		aggregateGroup.POST("/:function", validationMiddleware.ValidateFunctionQuery(), queryHandler.ExecuteAggregateFunction)
		aggregateGroup.POST("/:function/range", validationMiddleware.ValidateRangeFunctionQuery(), queryHandler.ExecuteAggregateRangeFunction)
	}

	// LogsQL endpoints (VictoriaLogs integration)
	logsHandler := handlers.NewLogsQLHandler(s.vmServices.Logs, s.cache, s.logger, s.searchRouter, s.config)
	logsGroup := v1.Group("/logs")
	logsGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"logs.read"}))
	{
		logsGroup.POST("/query", s.searchThrottling.ThrottleLogsQuery(), logsHandler.ExecuteQuery)
		logsGroup.GET("/streams", logsHandler.GetStreams)
		logsGroup.GET("/fields", logsHandler.GetFields)
		logsGroup.POST("/export", logsHandler.ExportLogs)
		logsGroup.POST("/store", logsHandler.StoreEvent) // For AI engines to store JSON events
	}

	// D3-friendly log endpoints for the upcoming UI (histogram, facets, search, tail)
	d3Logs := handlers.NewLogsHandler(s.vmServices.Logs, s.logger)
	d3LogsGroup := v1.Group("/logs")
	d3LogsGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"logs.read"}))
	{
		d3LogsGroup.GET("/histogram", d3Logs.Histogram)
		d3LogsGroup.GET("/facets", d3Logs.Facets)
		d3LogsGroup.POST("/search", d3Logs.Search)
		d3LogsGroup.GET("/tail", d3Logs.TailWS) // WebSocket upgrade for tailing logs
	}

	// VictoriaTraces endpoints (Jaeger-compatible HTTP API)
	tracesHandler := handlers.NewTracesHandler(s.vmServices.Traces, s.cache, s.logger, s.searchRouter, s.config)
	tracesGroup := v1.Group("/traces")
	tracesGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"traces.read"}))
	{
		tracesGroup.GET("/services", tracesHandler.GetServices)
		tracesGroup.GET("/services/:service/operations", tracesHandler.GetOperations)
		tracesGroup.GET("/:traceId", tracesHandler.GetTrace)
		tracesGroup.GET("/:traceId/flamegraph", tracesHandler.GetFlameGraph)
		tracesGroup.POST("/search", s.searchThrottling.ThrottleTracesQuery(), tracesHandler.SearchTraces)
		tracesGroup.POST("/flamegraph/search", tracesHandler.SearchFlameGraph)
	}

	// AI RCA-ENGINE endpoints (correlation with red anchors pattern)
	rcaServiceGraph := services.NewServiceGraphService(s.vmServices.Metrics, s.logger)
	rcaHandler := handlers.NewRCAHandler(s.grpcClients.RCAEngine, s.vmServices.Logs, rcaServiceGraph, s.cache, s.logger)
	rcaGroup := v1.Group("/rca")
	rcaGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"rca.read"}))
	{
		rcaGroup.GET("/correlations", rcaHandler.GetActiveCorrelations)
		rcaGroup.POST("/investigate", rcaHandler.StartInvestigation)
		rcaGroup.GET("/patterns", rcaHandler.GetFailurePatterns)
		rcaGroup.POST("/service-graph", rcaHandler.GetServiceGraph)
		rcaGroup.POST("/store", rcaHandler.StoreCorrelation) // Store correlation back to VictoriaLogs
	}

	// Configuration endpoints (user-driven settings storage)
	dynamicConfigService := services.NewDynamicConfigService(s.cache, s.logger)
	configHandler := handlers.NewConfigHandler(s.cache, s.logger, dynamicConfigService, s.grpcClients, s.schemaRepo)
	configGroup := v1.Group("/config")
	configGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"config.read"}))
	{
		configGroup.GET("/datasources", configHandler.GetDataSources)
		configGroup.POST("/datasources", configHandler.AddDataSource)
		configGroup.GET("/integrations", configHandler.GetIntegrations)

		// User Preferences API (moved from KPI handler)
		configGroup.GET("/user-preferences", configHandler.GetUserPreferences)
		configGroup.POST("/user-preferences", configHandler.CreateUserPreferences)
		configGroup.PUT("/user-preferences", configHandler.UpdateUserPreferences)
		configGroup.DELETE("/user-preferences", configHandler.DeleteUserPreferences)

		// Runtime feature flag endpoints
		configGroup.GET("/features", configHandler.GetFeatureFlags)
		configGroup.PUT("/features", configHandler.UpdateFeatureFlags)
		configGroup.POST("/features/reset", configHandler.ResetFeatureFlags)

		// Dynamic gRPC endpoint configuration
		configGroup.GET("/grpc/endpoints", configHandler.GetGRPCEndpoints)
		configGroup.PUT("/grpc/endpoints", configHandler.UpdateGRPCEndpoints)
		configGroup.POST("/grpc/endpoints/reset", configHandler.ResetGRPCEndpoints)
	}

	// Session management (Valkey cluster caching)
	sessionHandler := handlers.NewSessionHandler(s.cache, s.logger)
	sessionGroup := v1.Group("/sessions")
	sessionGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"session.admin"}))
	{
		sessionGroup.GET("/active", sessionHandler.GetActiveSessions)
		sessionGroup.POST("/invalidate", sessionHandler.InvalidateSession)
		sessionGroup.GET("/user/:userId", sessionHandler.GetUserSessions)
	}

	// Authentication endpoints
	authHandler := handlers.NewAuthHandler(s.config, s.cache, s.rbacRepo, s.logger)
	v1.POST("/auth/login", authHandler.Login)
	v1.POST("/auth/logout", authHandler.Logout)
	v1.POST("/auth/validate", authHandler.ValidateToken)

	// RBAC endpoints
	rbacHandler := handlers.NewRBACHandler(s.rbacService, s.cache, s.logger)
	rbacGroup := v1.Group("/rbac")
	rbacGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"rbac.admin"}))
	{
		// Role management
		rbacGroup.GET("/roles", rbacHandler.GetRoles)
		rbacGroup.POST("/roles", rbacHandler.CreateRole)
		rbacGroup.GET("/users/:userId/roles", rbacHandler.GetUserRoles)

		// Permission management
		rbacGroup.GET("/permissions", rbacHandler.GetPermissions)
		rbacGroup.POST("/permissions", rbacHandler.CreatePermission)
		rbacGroup.PUT("/permissions/:permissionId", rbacHandler.UpdatePermission)
		rbacGroup.DELETE("/permissions/:permissionId", rbacHandler.DeletePermission)

		// Group management
		rbacGroup.GET("/groups", rbacHandler.GetGroups)
		rbacGroup.POST("/groups", rbacHandler.CreateGroup)
		rbacGroup.PUT("/groups/:groupName", rbacHandler.UpdateGroup)
		rbacGroup.DELETE("/groups/:groupName", rbacHandler.DeleteGroup)
		rbacGroup.PUT("/groups/:groupName/users", rbacHandler.AddUsersToGroup)
		rbacGroup.DELETE("/groups/:groupName/users", rbacHandler.RemoveUsersFromGroup)
		rbacGroup.GET("/groups/:groupName/members", rbacHandler.GetGroupMembers)

		// Role binding management
		rbacGroup.GET("/role-bindings", rbacHandler.GetRoleBindings)
		rbacGroup.POST("/role-bindings", rbacHandler.CreateRoleBinding)
		rbacGroup.PUT("/role-bindings/:bindingId", rbacHandler.UpdateRoleBinding)
		rbacGroup.DELETE("/role-bindings/:bindingId", rbacHandler.DeleteRoleBinding)
	}

	// MiradorAuth endpoints (global admin only for local user management)
	miradorAuthHandler := handlers.NewMiradorAuthHandler(s.rbacRepo, s.logger)
	miradorAuthGroup := v1.Group("/auth/users")
	miradorAuthGroup.Use(s.tenantIsolationMiddleware.GlobalAdminOnly())
	{
		miradorAuthGroup.POST("", miradorAuthHandler.CreateMiradorAuth)
		miradorAuthGroup.GET("/:userId", miradorAuthHandler.GetMiradorAuth)
		miradorAuthGroup.PUT("/:userId", miradorAuthHandler.UpdateMiradorAuth)
		miradorAuthGroup.DELETE("/:userId", miradorAuthHandler.DeleteMiradorAuth)
	}

	// AuthConfig endpoints (tenant admin only for per-tenant auth configuration)
	authConfigHandler := handlers.NewAuthConfigHandler(s.rbacRepo, s.logger)
	authConfigGroup := v1.Group("/auth/config")
	authConfigGroup.Use(s.tenantIsolationMiddleware.TenantAdminOnly())
	{
		authConfigGroup.POST("", authConfigHandler.CreateAuthConfig)
		authConfigGroup.GET("/:tenantId", authConfigHandler.GetAuthConfig)
		authConfigGroup.PUT("/:tenantId", authConfigHandler.UpdateAuthConfig)
		authConfigGroup.DELETE("/:tenantId", authConfigHandler.DeleteAuthConfig)
	}

	// RBAC Audit endpoints (tenant admin only for security audit logs)
	rbacAuditHandler := handlers.NewRBACAuditHandler(s.rbacRepo, s.logger)
	rbacAuditGroup := v1.Group("/rbac/audit")
	rbacAuditGroup.Use(s.tenantIsolationMiddleware.TenantAdminOnly())
	{
		rbacAuditGroup.GET("", rbacAuditHandler.GetAuditEvents)
		rbacAuditGroup.GET("/:eventId", rbacAuditHandler.GetAuditEvent)
		rbacAuditGroup.GET("/summary", rbacAuditHandler.GetAuditSummary)
		rbacAuditGroup.GET("/subject/:subjectId", rbacAuditHandler.GetAuditEventsBySubject)
	}

	// Tenant endpoints
	tenantHandler := handlers.NewTenantHandler(s.rbacService, s.logger)

	// Global admin only routes
	tenantGlobalAdmin := v1.Group("/tenants")
	tenantGlobalAdmin.Use(s.tenantIsolationMiddleware.GlobalAdminOnly())
	{
		tenantGlobalAdmin.GET("", tenantHandler.ListTenants)
		tenantGlobalAdmin.POST("", tenantHandler.CreateTenant)
		tenantGlobalAdmin.DELETE("/:tenantId", tenantHandler.DeleteTenant)
	}

	// Tenant admin routes (can manage their own tenant)
	tenantAdmin := v1.Group("/tenants")
	tenantAdmin.Use(s.tenantIsolationMiddleware.TenantAdminOnly())
	{
		tenantAdmin.GET("/:tenantId", tenantHandler.GetTenant)
		tenantAdmin.PUT("/:tenantId", tenantHandler.UpdateTenant)
	}

	// Tenant-user association endpoints (tenant admins can manage)
	tenantUserAdmin := v1.Group("/tenants/:tenantId/users")
	tenantUserAdmin.Use(s.tenantIsolationMiddleware.TenantAdminOnly())
	{
		tenantUserAdmin.POST("", tenantHandler.CreateTenantUser)
		tenantUserAdmin.GET("", tenantHandler.ListTenantUsers)
		tenantUserAdmin.GET("/:userId", tenantHandler.GetTenantUser)
		tenantUserAdmin.PUT("/:userId", tenantHandler.UpdateTenantUser)
		tenantUserAdmin.DELETE("/:userId", tenantHandler.DeleteTenantUser)
	}

	// User endpoints (global admin only for global user management)
	userHandler := handlers.NewUserHandler(s.rbacService, s.logger)
	userGlobalAdmin := v1.Group("/users")
	userGlobalAdmin.Use(s.tenantIsolationMiddleware.GlobalAdminOnly())
	{
		userGlobalAdmin.GET("", userHandler.ListUsers)
		userGlobalAdmin.POST("", userHandler.CreateUser)
		userGlobalAdmin.GET("/:id", userHandler.GetUser)
		userGlobalAdmin.PUT("/:id", userHandler.UpdateUser)
		userGlobalAdmin.DELETE("/:id", userHandler.DeleteUser)
	}

	// WebSocket streams (metrics, alerts)
	ws := handlers.NewWebSocketHandler(s.logger)
	wsGroup := v1.Group("/ws")
	wsGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"metrics.read"}))
	{
		wsGroup.GET("/metrics", ws.HandleMetricsStream)
		wsGroup.GET("/alerts", ws.HandleAlertsStream)
	}

	// KPI APIs (primary interface for schema definitions)
	if s.schemaRepo != nil {
		kpiHandler := handlers.NewKPIHandler(s.schemaRepo, s.cache, s.logger)
		if kpiHandler != nil {
			// KPI Definitions API
			kpiDefsGroup := v1.Group("/kpi/defs")
			kpiDefsGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"kpi.read"}))
			{
				kpiDefsGroup.GET("", kpiHandler.GetKPIDefinitions)
				kpiDefsGroup.POST("", kpiHandler.CreateOrUpdateKPIDefinition)
				kpiDefsGroup.DELETE("/:id", kpiHandler.DeleteKPIDefinition)
			}

			// KPI Layouts API
			kpiLayoutsGroup := v1.Group("/kpi/layouts")
			kpiLayoutsGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"kpi.read"}))
			{
				kpiLayoutsGroup.GET("", kpiHandler.GetKPILayouts)
				kpiLayoutsGroup.POST("/batch", kpiHandler.BatchUpdateKPILayouts)
			}

			// Dashboard API
			dashboardGroup := v1.Group("/kpi/dashboards")
			dashboardGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"dashboard.read"}))
			{
				dashboardGroup.GET("", kpiHandler.GetDashboards)
				dashboardGroup.POST("", kpiHandler.CreateDashboard)
				dashboardGroup.PUT("/:id", kpiHandler.UpdateDashboard)
				dashboardGroup.DELETE("/:id", kpiHandler.DeleteDashboard)
			}

			// User Preferences API moved to /config/user-preferences
		}
	}

	// Unified Query Engine (Phase 1.5: Unified API Implementation)
	if s.config.UnifiedQuery.Enabled {
		s.setupUnifiedQueryEngine(v1)
	}

	// Metrics Metadata Discovery API (Phase 2: Metrics Metadata Integration)
	if s.metricsMetadataIndexer != nil {
		metricsSearchHandler := handlers.NewMetricsSearchHandler(s.metricsMetadataIndexer, s.logger)
		metricsSearchGroup := v1.Group("/metrics")
		metricsSearchGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"metrics.admin"}))
		{
			metricsSearchGroup.POST("/search", metricsSearchHandler.HandleMetricsSearch)
			metricsSearchGroup.POST("/sync", metricsSearchHandler.HandleMetricsSync)
			metricsSearchGroup.GET("/health", metricsSearchHandler.HandleMetricsHealth)
		}
	}

	// Metrics Metadata Synchronization API (Phase 2: Metrics Metadata Integration)
	if s.metricsMetadataSynchronizer != nil {
		syncHandler := handlers.NewMetricsSyncHandler(s.metricsMetadataSynchronizer, s.logger)
		metricsSyncGroup := v1.Group("/metrics/sync")
		metricsSyncGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"metrics.admin"}))
		{
			metricsSyncGroup.POST("/:tenantId", syncHandler.HandleSyncNow)
			metricsSyncGroup.GET("/:tenantId/state", syncHandler.HandleGetSyncState)
			metricsSyncGroup.GET("/:tenantId/status", syncHandler.HandleGetSyncStatus)
		}
		v1.PUT("/metrics/sync/config", s.rbacEnforcer.RBACMiddleware([]string{"metrics.admin"}), syncHandler.HandleUpdateConfig)
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

	// Register unified query routes with RBAC protection
	unifiedGroup := router.Group("/unified")
	unifiedGroup.Use(s.rbacEnforcer.RBACMiddleware([]string{"unified.read"}))
	{
		unifiedGroup.POST("/query", unifiedHandler.HandleUnifiedQuery)
		unifiedGroup.POST("/correlation", unifiedHandler.HandleUnifiedCorrelation)
		unifiedGroup.GET("/metadata", unifiedHandler.HandleQueryMetadata)
		unifiedGroup.GET("/health", unifiedHandler.HandleHealthCheck)
		unifiedGroup.POST("/search", unifiedHandler.HandleUnifiedSearch)
		unifiedGroup.GET("/stats", unifiedHandler.HandleUnifiedStats)
	}

	s.logger.Info("Unified query engine initialized and routes registered")
}

func (s *Server) Start(ctx context.Context) error {
	// Run RBAC bootstrap on startup
	s.logger.Info("Running RBAC bootstrap on server startup")
	if err := s.rbacBootstrap.RunBootstrap(ctx); err != nil {
		s.logger.Error("RBAC bootstrap failed", "error", err)
		// Don't fail server startup for bootstrap errors - log and continue
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
