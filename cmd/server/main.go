package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/platformbuilds/mirador-core/internal/api"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/grpc/clients"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	storage_weaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// These are set via -ldflags at build time (see Makefile)
var (
	version    = "dev"
	commitHash = "unknown"
	buildTime  = ""
)

// healthcheck performs an HTTP health check against the running service
func healthcheck() {
	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8010"
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Make health check request
	url := fmt.Sprintf("http://localhost:%s/health", port)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check returned non-OK status: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	// Health check passed
	os.Exit(0)
}

func main() {
	// Handle healthcheck subcommand
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		healthcheck()
		return
	}

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
		// Prefer cluster when multiple nodes provided; if the target is a standalone instance
		// (common in development), detect the specific error and fall back to single-node.
		valleyCache, err = cache.NewValkeyCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "cluster support disabled") {
				logger.Warn("Valkey reports cluster support disabled; falling back to single-node mode", "nodes", cfg.Cache.Nodes)
				// Try single-node on the first address; if that fails, use noop with auto-swap-to-single
				if len(cfg.Cache.Nodes) > 0 {
					if single, sErr := cache.NewValkeySingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second); sErr == nil {
						valleyCache = single
						logger.Info("Valkey single-node cache initialized via fallback", "addr", cfg.Cache.Nodes[0])
					} else {
						logger.Warn("Valkey single-node fallback unavailable; starting with in-memory cache (auto-reconnect to single)", "error", sErr)
						fallback := cache.NewNoopValkeyCache(logger)
						valleyCache = cache.NewAutoSwapForSingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
					}
				}
			} else {
				logger.Warn("Valkey cluster unavailable; starting with in-memory cache (auto-reconnect to cluster)", "error", err)
				fallback := cache.NewNoopValkeyCache(logger)
				valleyCache = cache.NewAutoSwapForCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
			}
		} else {
			logger.Info("Valkey cluster cache initialized", "nodes", len(cfg.Cache.Nodes))
		}
	}

	// Initialize gRPC clients for AI engines
	dynamicConfigService := services.NewDynamicConfigService(valleyCache, logger)
	grpcClients, err := clients.NewGRPCClients(cfg, logger, dynamicConfigService)
	if err != nil {
		logger.Fatal("Failed to initialize gRPC clients", "error", err)
	}
	logger.Info("gRPC clients initialized for AI engines")

	// Initialize VictoriaMetrics services
	vmServices, err := services.NewVictoriaMetricsServices(cfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to initialize VictoriaMetrics services", "error", err)
	}

	// Initialize schema store (Weaviate)
	var schemaStore repo.SchemaStore
	if cfg.Weaviate.Enabled {
		// Construct transport (HTTP by default; official client when built with tags)
		t, terr := storage_weaviate.NewTransportFromConfig(cfg.Weaviate)
		if terr != nil {
			logger.Fatal("Failed to init Weaviate transport", "error", terr)
		}
		ctxPing, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
		if err := storage_weaviate.Ready(ctxPing, t); err != nil {
			cancelPing()
			logger.Fatal("Weaviate not ready", "error", err)
		}
		cancelPing()
		logger.Info("Weaviate ready")
		wrepo := repo.NewWeaviateRepoFromTransport(t)
		if err := wrepo.EnsureSchema(context.Background()); err != nil {
			logger.Warn("Weaviate schema ensure failed", "error", err)
		}
		schemaStore = wrepo
	}

	// No legacy DB fallback; expect Weaviate

	// Initialize API server
	apiServer := api.NewServer(cfg, logger, valleyCache, grpcClients, vmServices, schemaStore)

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
