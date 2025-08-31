package clients

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/platformbuilds/miradorstack/internal/grpc/proto/predict"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

// PredictEngineClient wraps the gRPC client for PREDICT-ENGINE
type PredictEngineClient struct {
	client predict.PredictEngineServiceClient
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewPredictEngineClient creates a new PREDICT-ENGINE gRPC client
func NewPredictEngineClient(endpoint string, logger logger.Logger) (*PredictEngineClient, error) {
	conn, err := grpc.Dial(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PREDICT-ENGINE: %w", err)
	}

	client := predict.NewPredictEngineServiceClient(conn)

	return &PredictEngineClient{
		client: client,
		conn:   conn,
		logger: logger,
	}, nil
}

// AnalyzeFractures predicts possible fracture/fatigue in running services
func (c *PredictEngineClient) AnalyzeFractures(ctx context.Context, request *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
	grpcRequest := &predict.AnalyzeFracturesRequest{
		Component:  request.Component,
		TimeRange:  request.TimeRange,
		ModelTypes: request.ModelTypes,
		TenantId:   request.TenantID,
	}

	response, err := c.client.AnalyzeFractures(ctx, grpcRequest)
	if err != nil {
		c.logger.Error("PREDICT-ENGINE gRPC call failed", "component", request.Component, "error", err)
		return nil, err
	}

	// Convert gRPC response to internal model
	fractures := make([]*models.SystemFracture, len(response.Fractures))
	for i, f := range response.Fractures {
		fractures[i] = &models.SystemFracture{
			ID:                f.Id,
			Component:         f.Component,
			FractureType:      f.FractureType,
			TimeToFracture:    time.Duration(f.TimeToFractureSeconds) * time.Second,
			Severity:          f.Severity,
			Probability:       f.Probability,
			Confidence:        f.Confidence,
			ContributingFactors: f.ContributingFactors,
			Recommendation:    f.Recommendation,
			PredictedAt:       time.Now(),
		}
	}

	return &models.FractureAnalysisResponse{
		Fractures:        fractures,
		ModelsUsed:       response.ModelsUsed,
		ProcessingTimeMs: response.ProcessingTimeMs,
	}, nil
}

// HealthCheck checks the health of the PREDICT-ENGINE
func (c *PredictEngineClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.GetHealth(ctx, &predict.GetHealthRequest{})
	return err
}

// Close closes the gRPC connection
func (c *PredictEngineClient) Close() error {
	return c.conn.Close()
}
