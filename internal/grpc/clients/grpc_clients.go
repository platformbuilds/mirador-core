package clients

import (
	"fmt"

	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

// GRPCClients holds all gRPC client connections
type GRPCClients struct {
	PredictEngine *PredictEngineClient
	RCAEngine     *RCAEngineClient
	AlertEngine   *AlertEngineClient
	logger        logger.Logger
}

// NewGRPCClients creates and initializes all gRPC clients
func NewGRPCClients(cfg *config.Config, logger logger.Logger) (*GRPCClients, error) {
	// Initialize PREDICT-ENGINE client
	predictEngine, err := NewPredictEngineClient(cfg.GRPC.PredictEngine.Endpoint, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create PREDICT-ENGINE client: %w", err)
	}

	// Initialize RCA-ENGINE client
	rcaEngine, err := NewRCAEngineClient(cfg.GRPC.RCAEngine.Endpoint, logger)
	if err != nil {
		predictEngine.Close() // Cleanup on failure
		return nil, fmt.Errorf("failed to create RCA-ENGINE client: %w", err)
	}

	// Initialize ALERT-ENGINE client
	alertEngine, err := NewAlertEngineClient(cfg.GRPC.AlertEngine.Endpoint, logger)
	if err != nil {
		predictEngine.Close()
		rcaEngine.Close()
		return nil, fmt.Errorf("failed to create ALERT-ENGINE client: %w", err)
	}

	return &GRPCClients{
		PredictEngine: predictEngine,
		RCAEngine:     rcaEngine,
		AlertEngine:   alertEngine,
		logger:        logger,
	}, nil
}

// Close closes all gRPC connections
func (g *GRPCClients) Close() error {
	var errors []error

	if err := g.PredictEngine.Close(); err != nil {
		errors = append(errors, fmt.Errorf("PREDICT-ENGINE close error: %w", err))
	}

	if err := g.RCAEngine.Close(); err != nil {
		errors = append(errors, fmt.Errorf("RCA-ENGINE close error: %w", err))
	}

	if err := g.AlertEngine.Close(); err != nil {
		errors = append(errors, fmt.Errorf("ALERT-ENGINE close error: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("gRPC client close errors: %v", errors)
	}

	g.logger.Info("All gRPC clients closed successfully")
	return nil
}

// HealthCheck checks health of all gRPC services
func (g *GRPCClients) HealthCheck() error {
	// Check PREDICT-ENGINE
	if err := g.PredictEngine.HealthCheck(); err != nil {
		return fmt.Errorf("PREDICT-ENGINE health check failed: %w", err)
	}

	// Check RCA-ENGINE
	if err := g.RCAEngine.HealthCheck(); err != nil {
		return fmt.Errorf("RCA-ENGINE health check failed: %w", err)
	}

	// Check ALERT-ENGINE
	if err := g.AlertEngine.HealthCheck(); err != nil {
		return fmt.Errorf("ALERT-ENGINE health check failed: %w", err)
	}

	return nil
}
