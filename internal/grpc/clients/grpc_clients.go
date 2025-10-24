// internal/grpc/clients/grpc_clients.go
package clients

import (
	"context"
	"fmt"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
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
	// enabled flags (useful for health/readiness reporting)
	PredictEnabled bool
	RCAEnabled     bool
	AlertEnabled   bool
	// Dynamic config service for live updates
	dynamicConfig *services.DynamicConfigService
}

// NewGRPCClients creates and initializes all gRPC clients
func NewGRPCClients(cfg *config.Config, logger logger.Logger, dynamicConfig *services.DynamicConfigService) (*GRPCClients, error) {
	g := &GRPCClients{logger: logger, dynamicConfig: dynamicConfig}

	// Initialize PREDICT-ENGINE client
	if c, err := NewPredictEngineClient(cfg.GRPC.PredictEngine.Endpoint, logger); err != nil {
		if cfg.IsDevelopment() {
			logger.Warn("PREDICT-ENGINE unavailable; using no-op client (development)")
			g.PredictEngine = &noopPredictClient{logger: logger}
			g.PredictEnabled = false
		} else {
			return nil, fmt.Errorf("failed to create PREDICT-ENGINE client: %w", err)
		}
	} else {
		g.PredictEngine = &realPredictAdapter{c: c}
		g.PredictEnabled = true
	}

	// Initialize RCA-ENGINE client
	if c, err := NewRCAEngineClient(cfg.GRPC.RCAEngine.Endpoint, logger); err != nil {
		if cfg.IsDevelopment() {
			logger.Warn("RCA-ENGINE unavailable; using no-op client (development)")
			g.RCAEngine = &noopRCAClient{logger: logger}
			g.RCAEnabled = false
		} else {
			// close predict if it was real
			if a, ok := g.PredictEngine.(*realPredictAdapter); ok && a.c != nil {
				_ = a.c.Close()
			}
			return nil, fmt.Errorf("failed to create RCA-ENGINE client: %w", err)
		}
	} else {
		g.RCAEngine = &realRCAAdapter{c: c}
		g.RCAEnabled = true
	}

	// Initialize ALERT-ENGINE client
	if c, err := NewAlertEngineClient(cfg.GRPC.AlertEngine.Endpoint, logger); err != nil {
		if cfg.IsDevelopment() {
			logger.Warn("ALERT-ENGINE unavailable; disabling client (development)")
			g.AlertEngine = nil
			g.AlertEnabled = false
		} else {
			if a, ok := g.PredictEngine.(*realPredictAdapter); ok && a.c != nil {
				_ = a.c.Close()
			}
			if a, ok := g.RCAEngine.(*realRCAAdapter); ok && a.c != nil {
				_ = a.c.Close()
			}
			return nil, fmt.Errorf("failed to create ALERT-ENGINE client: %w", err)
		}
	} else {
		g.AlertEngine = c
		g.AlertEnabled = true
	}

	return g, nil
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

// UpdatePredictEndpoint updates the predict engine endpoint dynamically
func (g *GRPCClients) UpdatePredictEndpoint(ctx context.Context, tenantID, endpoint string) error {
	if g.dynamicConfig == nil {
		return fmt.Errorf("dynamic config service not available")
	}

	// Get current config
	currentConfig, err := g.dynamicConfig.GetGRPCConfig(ctx, tenantID, &config.GRPCConfig{
		PredictEngine: config.PredictEngineConfig{Endpoint: endpoint},
		RCAEngine:     config.RCAEngineConfig{},
		AlertEngine:   config.AlertEngineConfig{},
	})
	if err != nil {
		return fmt.Errorf("failed to get current gRPC config: %w", err)
	}

	// Update the endpoint
	currentConfig.PredictEngine.Endpoint = endpoint

	// Save updated config
	if err := g.dynamicConfig.SetGRPCConfig(ctx, tenantID, currentConfig); err != nil {
		return fmt.Errorf("failed to save updated gRPC config: %w", err)
	}

	// Update the actual client connection
	if a, ok := g.PredictEngine.(*realPredictAdapter); ok && a.c != nil {
		if err := a.c.UpdateEndpoint(endpoint); err != nil {
			return fmt.Errorf("failed to update predict engine client endpoint: %w", err)
		}
	}

	g.logger.Info("Successfully updated predict engine endpoint", "tenantID", tenantID, "endpoint", endpoint)
	return nil
}

// UpdateRCAEndpoint updates the RCA engine endpoint dynamically
func (g *GRPCClients) UpdateRCAEndpoint(ctx context.Context, tenantID, endpoint string) error {
	if g.dynamicConfig == nil {
		return fmt.Errorf("dynamic config service not available")
	}

	// Get current config
	currentConfig, err := g.dynamicConfig.GetGRPCConfig(ctx, tenantID, &config.GRPCConfig{
		PredictEngine: config.PredictEngineConfig{},
		RCAEngine:     config.RCAEngineConfig{Endpoint: endpoint},
		AlertEngine:   config.AlertEngineConfig{},
	})
	if err != nil {
		return fmt.Errorf("failed to get current gRPC config: %w", err)
	}

	// Update the endpoint
	currentConfig.RCAEngine.Endpoint = endpoint

	// Save updated config
	if err := g.dynamicConfig.SetGRPCConfig(ctx, tenantID, currentConfig); err != nil {
		return fmt.Errorf("failed to save updated gRPC config: %w", err)
	}

	// Update the actual client connection
	if a, ok := g.RCAEngine.(*realRCAAdapter); ok && a.c != nil {
		if err := a.c.UpdateEndpoint(endpoint); err != nil {
			return fmt.Errorf("failed to update RCA engine client endpoint: %w", err)
		}
	}

	g.logger.Info("Successfully updated RCA engine endpoint", "tenantID", tenantID, "endpoint", endpoint)
	return nil
}

// UpdateAlertEndpoint updates the alert engine endpoint dynamically
func (g *GRPCClients) UpdateAlertEndpoint(ctx context.Context, tenantID, endpoint string) error {
	if g.dynamicConfig == nil {
		return fmt.Errorf("dynamic config service not available")
	}

	// Get current config
	currentConfig, err := g.dynamicConfig.GetGRPCConfig(ctx, tenantID, &config.GRPCConfig{
		PredictEngine: config.PredictEngineConfig{},
		RCAEngine:     config.RCAEngineConfig{},
		AlertEngine:   config.AlertEngineConfig{Endpoint: endpoint},
	})
	if err != nil {
		return fmt.Errorf("failed to get current gRPC config: %w", err)
	}

	// Update the endpoint
	currentConfig.AlertEngine.Endpoint = endpoint

	// Save updated config
	if err := g.dynamicConfig.SetGRPCConfig(ctx, tenantID, currentConfig); err != nil {
		return fmt.Errorf("failed to save updated gRPC config: %w", err)
	}

	// Update the actual client connection
	if g.AlertEngine != nil {
		if err := g.AlertEngine.UpdateEndpoint(endpoint); err != nil {
			return fmt.Errorf("failed to update alert engine client endpoint: %w", err)
		}
	}

	g.logger.Info("Successfully updated alert engine endpoint", "tenantID", tenantID, "endpoint", endpoint)
	return nil
}

/* ========= No-op clients for development ========= */

type noopPredictClient struct{ logger logger.Logger }

func (n *noopPredictClient) AnalyzeFractures(ctx context.Context, req *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
	n.logger.Warn("AnalyzeFractures called on no-op PREDICT client")
	return nil, fmt.Errorf("predict engine disabled in development")
}
func (n *noopPredictClient) GetActiveModels(ctx context.Context, req *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error) {
	n.logger.Warn("GetActiveModels called on no-op PREDICT client")
	return &models.ActiveModelsResponse{Models: []models.PredictionModel{}}, nil
}
func (n *noopPredictClient) HealthCheck() error { return nil }

type noopRCAClient struct{ logger logger.Logger }

func (n *noopRCAClient) InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
	n.logger.Warn("InvestigateIncident called on no-op RCA client")
	return nil, fmt.Errorf("rca engine disabled in development")
}
func (n *noopRCAClient) HealthCheck() error { return nil }
