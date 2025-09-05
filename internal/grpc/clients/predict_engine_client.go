package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/grpc/proto/predict"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// PredictEngineClient wraps the gRPC client for PREDICT-ENGINE
type PredictEngineClient struct {
	client predict.PredictEngineServiceClient
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewPredictEngineClient creates a new PREDICT-ENGINE gRPC client
func NewPredictEngineClient(endpoint string, log logger.Logger) (*PredictEngineClient, error) {
	// Prefer ctx timeout + WithBlock over deprecated grpc.WithTimeout
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  200 * time.Millisecond,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   3 * time.Second,
			},
			MinConnectTimeout: 3 * time.Second,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to PREDICT-ENGINE: %w", err)
	}

	return &PredictEngineClient{
		client: predict.NewPredictEngineServiceClient(conn),
		conn:   conn,
		logger: log,
	}, nil
}

// AnalyzeFractures predicts possible fracture/fatigue in running services
// Returns: (1) ETA to fracture, (2) severity, (3) metadata
func (c *PredictEngineClient) AnalyzeFractures(ctx context.Context, req *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
	grpcReq := &predict.AnalyzeFracturesRequest{
		Component:  req.Component,
		TimeRange:  req.TimeRange,
		ModelTypes: req.ModelTypes,
		TenantId:   req.TenantID,
	}

	resp, err := c.client.AnalyzeFractures(ctx, grpcReq)
	if err != nil {
		c.logger.Error("PREDICT-ENGINE AnalyzeFractures failed", "component", req.Component, "error", err)
		return nil, err
	}

	fractures := make([]*models.SystemFracture, len(resp.Fractures))
	now := time.Now()
	for i, f := range resp.Fractures {
		fractures[i] = &models.SystemFracture{
			ID:                  f.Id,
			Component:           f.Component,
			FractureType:        f.FractureType,
			TimeToFracture:      time.Duration(f.TimeToFractureSeconds) * time.Second,
			Severity:            f.Severity,
			Probability:         f.Probability,
			Confidence:          f.Confidence,
			ContributingFactors: f.ContributingFactors,
			Recommendation:      f.Recommendation,
			PredictedAt:         now,
		}
	}

	return &models.FractureAnalysisResponse{
		Fractures:        fractures,
		ModelsUsed:       resp.ModelsUsed,
		ProcessingTimeMs: resp.ProcessingTimeMs,
	}, nil
}

// (Optional) Convenience: get fractures listing
func (c *PredictEngineClient) GetPredictedFractures(ctx context.Context, timeRange string, minProb float64) ([]*models.SystemFracture, error) {
	resp, err := c.client.GetPredictedFractures(ctx, &predict.GetFracturesRequest{
		TimeRange:      timeRange,
		MinProbability: minProb,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*models.SystemFracture, len(resp.Fractures))
	now := time.Now()
	for i, f := range resp.Fractures {
		out[i] = &models.SystemFracture{
			ID:                  f.Id,
			Component:           f.Component,
			FractureType:        f.FractureType,
			TimeToFracture:      time.Duration(f.TimeToFractureSeconds) * time.Second,
			Severity:            f.Severity,
			Probability:         f.Probability,
			Confidence:          f.Confidence,
			ContributingFactors: f.ContributingFactors,
			Recommendation:      f.Recommendation,
			PredictedAt:         now,
		}
	}
	return out, nil
}

// (Optional) Convenience: get model inventory
func (c *PredictEngineClient) GetModels(ctx context.Context) ([]*predict.MLModel, error) {
	resp, err := c.client.GetModels(ctx, &predict.GetModelsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// HealthCheck checks the health of the PREDICT-ENGINE
func (c *PredictEngineClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.client.GetHealth(ctx, &predict.GetHealthRequest{})
	return err
}

func (c *PredictEngineClient) Close() error {
	return c.conn.Close()
}

// GetActiveModels retrieves active prediction models from PREDICT-ENGINE
func (c *PredictEngineClient) GetActiveModels(ctx context.Context, request *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error) {
	// Call gRPC directly so we can send tenant_id from the request
	resp, err := c.client.GetModels(ctx, &predict.GetModelsRequest{
		TenantId: request.TenantID,
	})
	if err != nil {
		return nil, err
	}

	active := make([]models.PredictionModel, 0, len(resp.Models))
	for _, m := range resp.Models {
		// Treat empty status as active (safe default for early engine builds)
		if m.Status != "" && m.Status != "active" {
			continue
		}

		// NOTE: MLModel has no Id/Version in proto.
		// If your internal model requires them, fill sensible placeholders.
		pm := models.PredictionModel{
			ID:          "", // proto has no id; leave empty or derive from name+type
			Name:        m.Name,
			Version:     "", // proto has no version
			Type:        m.Type,
			Status:      "active",
			Accuracy:    m.Accuracy,
			CreatedAt:   m.LastTrained, // proto gives string; your struct uses string in current code
			UpdatedAt:   m.LastTrained,
			Description: "",
			Parameters:  map[string]interface{}{},
			Metrics: models.ModelMetrics{
				Precision: 0, Recall: 0, F1Score: 0, MAE: 0, RMSE: 0,
			},
		}
		active = append(active, pm)
	}

	return &models.ActiveModelsResponse{
		Models:      active,
		LastUpdated: time.Now().Format(time.RFC3339),
	}, nil
}
