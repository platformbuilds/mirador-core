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

type RCAClient interface {
	InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error)
	ListCorrelations(ctx context.Context, req *models.ListCorrelationsRequest) (*models.ListCorrelationsResponse, error)
	GetPatterns(ctx context.Context, req *models.GetPatternsRequest) (*models.GetPatternsResponse, error)
	SubmitFeedback(ctx context.Context, req *models.FeedbackRequest) (*models.FeedbackResponse, error)
	HealthCheck() error
}

/* ========= Adapters to real concrete clients ========= */

// realRCAAdapter wraps *RCAEngineClient to satisfy RCAClient.
type realRCAAdapter struct{ c *RCAEngineClient }

func (a *realRCAAdapter) InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
	return a.c.InvestigateIncident(ctx, req)
}
func (a *realRCAAdapter) ListCorrelations(ctx context.Context, req *models.ListCorrelationsRequest) (*models.ListCorrelationsResponse, error) {
	return a.c.ListCorrelations(ctx, req)
}
func (a *realRCAAdapter) GetPatterns(ctx context.Context, req *models.GetPatternsRequest) (*models.GetPatternsResponse, error) {
	return a.c.GetPatterns(ctx, req)
}
func (a *realRCAAdapter) SubmitFeedback(ctx context.Context, req *models.FeedbackRequest) (*models.FeedbackResponse, error) {
	return a.c.SubmitFeedback(ctx, req)
}
func (a *realRCAAdapter) HealthCheck() error { return a.c.HealthCheck() }

/* ========= Bundle used by server/handlers ========= */

// GRPCClients holds all gRPC client connections (as interfaces)
type GRPCClients struct {
	RCAEngine   RCAClient
	AlertEngine *AlertEngineClient // unchanged (can be interfaced later)
	logger      logger.Logger
	// enabled flags (useful for health/readiness reporting)
	RCAEnabled   bool
	AlertEnabled bool
	// Dynamic config service for live updates
	dynamicConfig *services.DynamicConfigService
}

// NewGRPCClients creates and initializes all gRPC clients
func NewGRPCClients(cfg *config.Config, log logger.Logger, dynamicConfig *services.DynamicConfigService) (*GRPCClients, error) {
	g := &GRPCClients{logger: log, dynamicConfig: dynamicConfig}

	// Initialize RCA-ENGINE client (now REST)
	if c, err := NewRCAEngineClient(cfg.GRPC.RCAEngine.Endpoint, log); err != nil {
		if cfg.IsDevelopment() {
			log.Warn("RCA-ENGINE unavailable; using no-op client (development)")
			g.RCAEngine = &noopRCAClient{logger: log}
			g.RCAEnabled = false
		} else {
			return nil, fmt.Errorf("failed to create RCA-ENGINE client: %w", err)
		}
	} else {
		g.RCAEngine = &realRCAAdapter{c: c}
		g.RCAEnabled = true
	}

	// Initialize ALERT-ENGINE client
	if c, err := NewAlertEngineClient(cfg.GRPC.AlertEngine.Endpoint, log); err != nil {
		if cfg.IsDevelopment() {
			log.Warn("ALERT-ENGINE unavailable; disabling client (development)")
			g.AlertEngine = nil
			g.AlertEnabled = false
		} else {
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

	// RCA-ENGINE is now REST-based, no close needed
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

// UpdateRCAEndpoint updates the RCA engine endpoint dynamically
func (g *GRPCClients) UpdateRCAEndpoint(ctx context.Context, tenantID, endpoint string) error {
	if g.dynamicConfig == nil {
		return fmt.Errorf("dynamic config service not available")
	}

	// Get current config
	currentConfig, err := g.dynamicConfig.GetGRPCConfig(ctx, tenantID, &config.GRPCConfig{
		RCAEngine:   config.RCAEngineConfig{Endpoint: endpoint},
		AlertEngine: config.AlertEngineConfig{},
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
		RCAEngine:   config.RCAEngineConfig{},
		AlertEngine: config.AlertEngineConfig{Endpoint: endpoint},
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

type noopRCAClient struct{ logger logger.Logger }

func (n *noopRCAClient) InvestigateIncident(ctx context.Context, req *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
	n.logger.Warn("InvestigateIncident called on no-op RCA client")
	return nil, fmt.Errorf("rca engine disabled in development")
}
func (n *noopRCAClient) ListCorrelations(ctx context.Context, req *models.ListCorrelationsRequest) (*models.ListCorrelationsResponse, error) {
	n.logger.Warn("ListCorrelations called on no-op RCA client")
	return &models.ListCorrelationsResponse{Correlations: []models.CorrelationResult{}}, nil
}
func (n *noopRCAClient) GetPatterns(ctx context.Context, req *models.GetPatternsRequest) (*models.GetPatternsResponse, error) {
	n.logger.Warn("GetPatterns called on no-op RCA client")
	return &models.GetPatternsResponse{Patterns: []models.Pattern{}}, nil
}
func (n *noopRCAClient) SubmitFeedback(ctx context.Context, req *models.FeedbackRequest) (*models.FeedbackResponse, error) {
	n.logger.Warn("SubmitFeedback called on no-op RCA client")
	return &models.FeedbackResponse{CorrelationID: req.CorrelationID, Accepted: true}, nil
}
func (n *noopRCAClient) HealthCheck() error { return nil }
