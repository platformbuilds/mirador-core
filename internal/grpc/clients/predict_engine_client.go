package clients

import (
	"context"
	"fmt"
	"time"

	pb "github.com/platformbuilds/miradorstack/internal/grpc/proto/predict"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// PredictEngineClient wraps the gRPC client for PREDICT-ENGINE
type PredictEngineClient struct {
	client pb.PredictEngineServiceClient
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
		client: pb.NewPredictEngineServiceClient(conn),
		conn:   conn,
		logger: log,
	}, nil
}

// AnalyzeFractures predicts possible fracture/fatigue in running services
// Returns: (1) ETA to fracture, (2) severity, (3) metadata
func (c *PredictEngineClient) AnalyzeFractures(ctx context.Context, req *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
	grpcReq := &pb.AnalyzeFracturesRequest{
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
	resp, err := c.client.GetPredictedFractures(ctx, &pb.GetFracturesRequest{
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
func (c *PredictEngineClient) GetModels(ctx context.Context) ([]*pb.MLModel, error) {
	resp, err := c.client.GetModels(ctx, &pb.GetModelsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// HealthCheck checks the health of the PREDICT-ENGINE
func (c *PredictEngineClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.client.GetHealth(ctx, &pb.GetHealthRequest{})
	return err
}

func (c *PredictEngineClient) Close() error {
	return c.conn.Close()
}
