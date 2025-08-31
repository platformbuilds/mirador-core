package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirador/core/internal/api/handlers"
	"github.com/mirador/core/internal/api/middleware"
	"github.com/mirador/core/internal/config"
	"github.com/mirador/core/internal/grpc/clients"
	"github.com/mirador/core/internal/services"
	"github.com/mirador/core/pkg/cache"
	"github.com/mirador/core/pkg/logger"
)

type Server struct {
	config      *config.Config
	logger      logger.Logger
	cache       cache.ValkeyCluster
	grpcClients *clients.GRPCClients
	vmServices  *services.VictoriaMetricsServices
	router      *gin.Engine
	httpServer  *http.Server
}

func NewServer(
	cfg *config.Config,
	log logger.Logger,
	valleyCache cache.ValkeyCluster,
	grpcClients *clients.GRPCClients,
	vmServices *services.VictoriaMetricsServices,
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

	// Rate limiting using Valkey cluster
	s.router.Use(middleware.RateLimiter(s.cache))

	// RBAC authentication (LDAP/AD + SSO integration)
	s.router.Use(middleware.AuthMiddleware(s.config.Auth, s.cache))
}

func (s *Server) setupRoutes() {
	// Public health endpoints
	s.router.GET("/health", handlers.HealthCheck)
	s.router.GET("/ready", handlers.ReadinessCheck)

	// OpenAPI specification endpoint
	s.router.GET("/api/openapi.json", handlers.GetOpenAPISpec)

	// API v1 group (protected by RBAC)
	v1 := s.router.Group("/api/v1")

	// MetricsQL endpoints (VictoriaMetrics integration)
	metricsHandler := handlers.NewMetricsQLHandler(s.vmServices.Metrics, s.cache, s.logger)
	v1.POST("/query", metricsHandler.ExecuteQuery)
	v1.POST("/query_range", metricsHandler.ExecuteRangeQuery)
	v1.GET("/series", metricsHandler.GetSeries)
	v1.GET("/labels", metricsHandler.GetLabels)
	v1.GET("/label/:name/values", metricsHandler.GetLabelValues)

	// LogsQL endpoints (VictoriaLogs integration)
	logsHandler := handlers.NewLogsQLHandler(s.vmServices.Logs, s.cache, s.logger)
	v1.POST("/logs/query", logsHandler.ExecuteQuery)
	v1.GET("/logs/streams", logsHandler.GetStreams)
	v1.GET("/logs/fields", logsHandler.GetFields)
	v1.POST("/logs/export", logsHandler.ExportLogs)
	v1.POST("/logs/store", logsHandler.StoreEvent) // For AI engines to store JSON events

	// VictoriaTraces endpoints (Jaeger-compatible HTTP API)
	tracesHandler := handlers.NewTracesHandler(s.vmServices.Traces, s.cache, s.logger)
	v1.GET("/traces/services", tracesHandler.GetServices)
	v1.GET("/traces/services/:service/operations", tracesHandler.GetOperations)
	v1.GET("/traces/:traceId", tracesHandler.GetTrace)
	v1.POST("/traces/search", tracesHandler.SearchTraces)

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
