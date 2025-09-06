package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

	"github.com/platformbuilds/mirador-core/internal/api"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// These are set via -ldflags at build time (see Makefile)
var (
    version    = "dev"
    commitHash = "unknown"
    buildTime  = ""
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
    logger := logger.New(cfg.LogLevel)
    logger.Info("Starting MIRADOR-CORE", "version", version, "commit", commitHash, "built", buildTime, "environment", cfg.Environment)

    // Initialize Valkey cache: single-node when one address is provided; cluster otherwise
    var valleyCache cache.ValkeyCluster
    if len(cfg.Cache.Nodes) == 1 {
        // Try immediate single-node connect; on failure, start with noop and auto-swap in background
        valleyCache, err = cache.NewValkeySingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second)
        if err != nil {
            logger.Warn("Valkey single-node unavailable; starting with in-memory cache (auto-reconnect enabled)", "error", err)
            fallback := cache.NewNoopValkeyCache(logger)
            valleyCache = cache.NewAutoSwapForSingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
        } else {
            logger.Info("Valkey single-node cache initialized", "addr", cfg.Cache.Nodes[0])
        }
    } else {
        // Try immediate cluster connect; on failure, start with noop and auto-swap in background
        valleyCache, err = cache.NewValkeyCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second)
        if err != nil {
            logger.Warn("Valkey cluster unavailable; starting with in-memory cache (auto-reconnect enabled)", "error", err)
            fallback := cache.NewNoopValkeyCache(logger)
            valleyCache = cache.NewAutoSwapForCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
        } else {
            logger.Info("Valkey cluster cache initialized", "nodes", len(cfg.Cache.Nodes))
        }
    }

	// Initialize gRPC clients for AI engines
	grpcClients, err := clients.NewGRPCClients(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize gRPC clients", "error", err)
	}
	logger.Info("gRPC clients initialized for AI engines")

	// Initialize VictoriaMetrics services
	vmServices, err := services.NewVictoriaMetricsServices(cfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to initialize VictoriaMetrics services", "error", err)
	}

    // Initialize API server
    apiServer := api.NewServer(cfg, logger, valleyCache, grpcClients, vmServices)

    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // If cache supports Stop (auto-swap connector), tie it to lifecycle
    if stopper, ok := interface{}(valleyCache).(interface{ Stop() }); ok {
        go func() { <-ctx.Done(); stopper.Stop() }()
    }

    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan
        logger.Info("Shutdown signal received")
        cancel()
    }()

    // Start dynamic endpoint discovery (DNS-based) if configured
    vmServices.StartDiscovery(ctx, cfg.Database, logger)

    // Start server
    if err := apiServer.Start(ctx); err != nil {
        logger.Fatal("Server failed to start", "error", err)
    }

    logger.Info("MIRADOR-CORE shutdown complete")
}
