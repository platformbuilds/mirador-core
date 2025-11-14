package main

import (
	"context"
	"log"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/repo/rbac"
	"github.com/platformbuilds/mirador-core/internal/services"
	storage_weaviate "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// main runs the RBAC bootstrap process
func main() {
	log.Println("🚀 Starting Mirador Core RBAC Bootstrap")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := logger.New(cfg.LogLevel)
	logger.Info("RBAC Bootstrap initializing", "environment", cfg.Environment)

	// Initialize Valkey cache
	var valkeyCache cache.ValkeyCluster
	if len(cfg.Cache.Nodes) == 1 {
		valkeyCache, err = cache.NewValkeySingle(cfg.Cache.Nodes[0], cfg.Cache.DB, cfg.Cache.Password, time.Duration(cfg.Cache.TTL)*time.Second)
		if err != nil {
			log.Fatalf("Failed to connect to Valkey: %v", err)
		}
		logger.Info("Valkey single-node cache initialized", "addr", cfg.Cache.Nodes[0])
	} else {
		valkeyCache, err = cache.NewValkeyCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second)
		if err != nil {
			log.Fatalf("Failed to connect to Valkey cluster: %v", err)
		}
		logger.Info("Valkey cluster cache initialized", "nodes", len(cfg.Cache.Nodes))
	}

	// Initialize Weaviate transport
	if !cfg.Weaviate.Enabled {
		log.Fatal("Weaviate must be enabled for RBAC bootstrap")
	}

	t, err := storage_weaviate.NewTransportFromConfig(cfg.Weaviate, logger)
	if err != nil {
		log.Fatalf("Failed to init Weaviate transport: %v", err)
	}

	// Check Weaviate readiness
	ctxPing, cancelPing := context.WithTimeout(context.Background(), 10*time.Second)
	if err := storage_weaviate.Ready(ctxPing, t); err != nil {
		cancelPing()
		log.Fatalf("Weaviate not ready: %v", err)
	}
	cancelPing()
	logger.Info("Weaviate ready")

	// Initialize RBAC repository
	rbacRepo := rbac.NewWeaviateRBACRepository(t)
	logger.Info("RBAC repository initialized")

	// Initialize audit service
	auditService := rbac.NewAuditService(rbacRepo)

	// Initialize RBAC cache repository using Valkey
	valkeyAdapter := rbac.NewValkeyClusterAdapter(valkeyCache)
	cacheRepo := rbac.NewValkeyRBACRepository(valkeyAdapter)

	// Initialize RBAC service
	rbacService := rbac.NewRBACService(rbacRepo, cacheRepo, auditService, logger)
	logger.Info("RBAC service initialized")

	// Create bootstrap service
	bootstrapService := services.NewRBACBootstrapService(rbacService, rbacRepo, logger)
	logger.Info("Bootstrap service created")

	// Run bootstrap
	ctx := context.Background()
	if err := bootstrapService.RunBootstrap(ctx); err != nil {
		log.Fatalf("❌ Bootstrap failed: %v", err)
	}

	logger.Info("✅ RBAC Bootstrap completed successfully!")
	log.Println("✅ RBAC Bootstrap completed successfully!")
	log.Println("")
	log.Println("📋 Default credentials:")
	log.Println("   Username: aarvee")
	log.Println("   Password: password123")
	log.Println("   Tenant:   PLATFORMBUILDS")
	log.Println("")
	log.Println("⚠️  IMPORTANT: Change the default password immediately after first login!")
}
