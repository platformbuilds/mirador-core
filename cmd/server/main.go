package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mirador/core/internal/api"
	"github.com/mirador/core/internal/config"
	"github.com/mirador/core/internal/grpc/clients"
	"github.com/mirador/core/internal/services"
	"github.com/mirador/core/pkg/cache"
	"github.com/mirador/core/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := logger.New(cfg.LogLevel)
	logger.Info("Starting MIRADOR-CORE", "version", "v2.1.3", "environment", cfg.Environment)

	// Initialize Valley Cluster caching (as shown in diagram)
	valleyCache, err := cache.NewValleyCluster(cfg.Cache.Nodes, time.Duration(cfg.Cache.TTL)*time.Second)
	if err != nil {
		logger.Fatal("Failed to initialize Valley cluster cache", "error", err)
	}
	logger.Info("Valley cluster cache initialized", "nodes", len(cfg.Cache.Nodes))

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

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Info("Shutdown signal received")
		cancel()
	}()

	// Start server
	if err := apiServer.Start(ctx); err != nil {
		logger.Fatal("Server failed to start", "error", err)
	}

	logger.Info("MIRADOR-CORE shutdown complete")
}
