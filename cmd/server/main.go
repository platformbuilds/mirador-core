package main

import (
	"context"
	"encoding/json"
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

// @title Mirador Core API
// @version 8.0.0
// @description Mirador Core is a comprehensive observability and analytics platform that provides KPI definitions, layouts, dashboards, and user preferences for monitoring and analyzing system metrics.
// @termsOfService http://swagger.io/terms/

// @contact.name Platform Builds Team
// @contact.url https://github.com/platformbuilds/mirador-core
// @contact.email support@platformbuilds.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8010
// @BasePath /api/v1

// @externalDocs.description OpenAPI
// @externalDocs.url https://swagger.io/resources/open-api/

// These are set via -ldflags at build time (see Makefile)
var (
	version    = "dev"
	commitHash = "unknown"
	buildTime  = ""
)

func main() {
	// Check for healthcheck command
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		// Load configuration to verify it's valid
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Configuration load failed: %v", err)
		}

		// Make HTTP request to health endpoint
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/health", cfg.Port))
		if err != nil {
			log.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			log.Fatalf("Health check failed: status %d", resp.StatusCode)
		}

		// Parse response
		var healthResp struct {
			Service   string `json:"service"`
			Status    string `json:"status"`
			Version   string `json:"version"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
			log.Fatalf("Failed to parse health response: %v", err)
		}

		if healthResp.Service != "mirador-core" || healthResp.Status != "healthy" {
			log.Fatalf("Health check failed: invalid response %+v", healthResp)
		}

		log.Println("healthy")
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
	var valkeyCache cache.ValkeyCluster
	if len(cfg.Cache.Nodes) == 1 {
		// Try immediate single-node connect; on failure, start with noop and auto-swap in background
		valkeyCache, err = cache.NewValkeySingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second)
		if err != nil {
			logger.Warn("Valkey single-node unavailable; starting with in-memory cache (auto-reconnect enabled)", "error", err)
			fallback := cache.NewNoopValkeyCache(logger)
			valkeyCache = cache.NewAutoSwapForSingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
		} else {
			logger.Info("Valkey single-node cache initialized", "addr", cfg.Cache.Nodes[0])
		}
	} else {
		// Prefer cluster when multiple nodes provided; if the target is a standalone instance
		// (common in development), detect the specific error and fall back to single-node.
		valkeyCache, err = cache.NewValkeyCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "cluster support disabled") {
				logger.Warn("Valkey reports cluster support disabled; falling back to single-node mode", "nodes", cfg.Cache.Nodes)
				// Try single-node on the first address; if that fails, use noop with auto-swap-to-single
				if len(cfg.Cache.Nodes) > 0 {
					if single, sErr := cache.NewValkeySingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second); sErr == nil {
						valkeyCache = single
						logger.Info("Valkey single-node cache initialized via fallback", "addr", cfg.Cache.Nodes[0])
					} else {
						logger.Warn("Valkey single-node fallback unavailable; starting with in-memory cache (auto-reconnect to single)", "error", sErr)
						fallback := cache.NewNoopValkeyCache(logger)
						valkeyCache = cache.NewAutoSwapForSingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
					}
				}
			} else {
				logger.Warn("Valkey cluster unavailable; starting with in-memory cache (auto-reconnect to cluster)", "error", err)
				fallback := cache.NewNoopValkeyCache(logger)
				valkeyCache = cache.NewAutoSwapForCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second, logger, fallback)
			}
		} else {
			logger.Info("Valkey cluster cache initialized", "nodes", len(cfg.Cache.Nodes))
		}
	}

	// Initialize gRPC clients for AI engines
	dynamicConfigService := services.NewDynamicConfigService(valkeyCache, logger)
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
	var wrepo *repo.WeaviateRepo
	if cfg.Weaviate.Enabled {
		// Construct transport (HTTP by default; official client when built with tags)
		t, terr := storage_weaviate.NewTransportFromConfig(cfg.Weaviate, logger)
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
		wrepo = repo.NewWeaviateRepoFromTransport(t)
		if err := wrepo.EnsureSchema(context.Background()); err != nil {
			logger.Warn("Weaviate schema ensure failed", "error", err)
		}
		schemaStore = wrepo
	}

	// No legacy DB fallback; expect Weaviate

	// Initialize API server
	apiServer := api.NewServer(cfg, logger, valkeyCache, grpcClients, vmServices, schemaStore)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// If cache supports Stop (auto-swap connector), tie it to lifecycle
	if stopper, ok := interface{}(valkeyCache).(interface{ Stop() }); ok {
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
