// internal/grpc/clients/grpc_clients.go
package clients

import (
	"context"
	"fmt"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

/* ========= Interfaces used by HTTP handlers ========= */

// PredictClient is the abstraction the HTTP handlers depend on.
// It must include every RPC your handlers call.
type PredictClient interface {
	AnalyzeFractures(ctx context.Context, req *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error)
	GetActiveModels(ctx context.Context, req *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error)
	HealthCheck() error
}

type RCAClient interface {
	InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error)
	HealthCheck() error
}

/* ========= Adapters to real concrete clients ========= */

// realPredictAdapter wraps *PredictEngineClient to satisfy PredictClient.
type realPredictAdapter struct{ c *PredictEngineClient }

func (a *realPredictAdapter) AnalyzeFractures(ctx context.Context, req *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
	return a.c.AnalyzeFractures(ctx, req)
}
func (a *realPredictAdapter) GetActiveModels(ctx context.Context, req *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error) {
	return a.c.GetActiveModels(ctx, req)
}
func (a *realPredictAdapter) HealthCheck() error { return a.c.HealthCheck() }

// realRCAAdapter wraps *RCAEngineClient to satisfy RCAClient.
type realRCAAdapter struct{ c *RCAEngineClient }

func (a *realRCAAdapter) InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
	return a.c.InvestigateIncident(ctx, req)
}
func (a *realRCAAdapter) HealthCheck() error { return a.c.HealthCheck() }

/* ========= Bundle used by server/handlers ========= */

// GRPCClients holds all gRPC client connections (as interfaces)
type GRPCClients struct {
	PredictEngine PredictClient
	RCAEngine     RCAClient
	AlertEngine   *AlertEngineClient // unchanged (can be interfaced later)
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

	// Initialize ALERT-ENGINE client (unchanged)
	alertEngine, err := NewAlertEngineClient(cfg.GRPC.AlertEngine.Endpoint, logger)
	if err != nil {
		predictEngine.Close()
		rcaEngine.Close()
		return nil, fmt.Errorf("failed to create ALERT-ENGINE client: %w", err)
	}

	return &GRPCClients{
		PredictEngine: &realPredictAdapter{c: predictEngine},
		RCAEngine:     &realRCAAdapter{c: rcaEngine},
		AlertEngine:   alertEngine,
		logger:        logger,
	}, nil
}

// Close closes all gRPC connections
func (g *GRPCClients) Close() error {
	var errors []error

	// Close underlying real clients when using adapters
	if a, ok := g.PredictEngine.(*realPredictAdapter); ok && a.c != nil {
		if err := a.c.Close(); err != nil {
			errors = append(errors, fmt.Errorf("PREDICT-ENGINE close error: %w", err))
		}
	}
	if a, ok := g.RCAEngine.(*realRCAAdapter); ok && a.c != nil {
		if err := a.c.Close(); err != nil {
			errors = append(errors, fmt.Errorf("RCA-ENGINE close error: %w", err))
		}
	}
	if g.AlertEngine != nil {
		if err := g.AlertEngine.Close(); err != nil {
			errors = append(errors, fmt.Errorf("ALERT-ENGINE close error: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("gRPC client close errors: %v", errors)
	}
	g.logger.Info("All gRPC clients closed successfully")
	return nil
}

// HealthCheck checks health of all gRPC services
func (g *GRPCClients) HealthCheck() error {
	if g.PredictEngine != nil {
		if err := g.PredictEngine.HealthCheck(); err != nil {
			return fmt.Errorf("PREDICT-ENGINE health check failed: %w", err)
		}
	}
	if g.RCAEngine != nil {
		if err := g.RCAEngine.HealthCheck(); err != nil {
			return fmt.Errorf("RCA-ENGINE health check failed: %w", err)
		}
	}
	if g.AlertEngine != nil {
		if err := g.AlertEngine.HealthCheck(); err != nil {
			return fmt.Errorf("ALERT-ENGINE health check failed: %w", err)
		}
	}
	return nil
}
