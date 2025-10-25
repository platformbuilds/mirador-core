package api

import (
	"time"

	"github.com/platformbuilds/mirador-core/internal/api/handlers"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Configuration for Unified Query Engine
type UnifiedQueryConfig struct {
	Enabled           bool          `yaml:"enabled" default:"true"`
	CacheTTL          time.Duration `yaml:"cache_ttl" default:"5m"`
	MaxCacheTTL       time.Duration `yaml:"max_cache_ttl" default:"1h"`
	DefaultLimit      int           `yaml:"default_limit" default:"1000"`
	EnableCorrelation bool          `yaml:"enable_correlation" default:"false"`
}

// Example of how to integrate UnifiedQueryEngine into the server
func (s *Server) setupUnifiedQueryEngine() error {
	// Create unified query engine instance
	unifiedEngine := services.NewUnifiedQueryEngine(
		s.vmServices.Metrics, // VictoriaMetrics service
		s.vmServices.Logs,    // VictoriaLogs service
		s.vmServices.Traces,  // VictoriaTraces service
		s.cache,              // Valkey cache
		s.logger,             // Logger
	)

	// Create unified query handler
	unifiedHandler := handlers.NewUnifiedQueryHandler(unifiedEngine, s.logger)

	// Register unified query routes
	unifiedHandler.RegisterRoutes(s.router.Group("/api/v1"))

	s.logger.Info("Unified Query Engine initialized and routes registered")
	return nil
}

// Example of how to call this from server initialization
func NewServerWithUnifiedQuery(
	cfg *config.Config,
	log logger.Logger,
	valleyCache cache.ValkeyCluster,
	grpcClients *clients.GRPCClients,
	vmServices *services.VictoriaMetricsServices,
	schemaRepo repo.SchemaStore,
) *Server {
	server := NewServer(cfg, log, valleyCache, grpcClients, vmServices, schemaRepo)

	// Initialize unified query engine if enabled in config
	if cfg.UnifiedQuery.Enabled {
		if err := server.setupUnifiedQueryEngine(); err != nil {
			log.Error("Failed to setup unified query engine", "error", err)
			return nil
		}
	}

	return server
}
