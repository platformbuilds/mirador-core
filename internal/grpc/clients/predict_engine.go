package clients

import (
	"context"
	"time"

	pb "github.com/mirador/core/internal/grpc/proto/predict"
	"github.com/mirador/core/internal/models"
	"github.com/mirador/core/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PredictEngineClient struct {
	client pb.PredictEngineServiceClient
	conn   *grpc.ClientConn
	logger logger.Logger
}

func NewPredictEngineClient(endpoint string, logger logger.Logger) (*PredictEngineClient, error) {
	conn, err := grpc.Dial(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	client := pb.NewPredictEngineServiceClient(conn)

	return &PredictEngineClient{
		client: client,
		conn:   conn,
		logger: logger,
	}, nil
}

// AnalyzeFractures predicts possible fracture/fatigue in running services
// Returns: 1. Remaining time for fatigue to convert to fracture
//  2. Severity of the fracture
func (c *PredictEngineClient) AnalyzeFractures(ctx context.Context, request *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
	grpcRequest := &pb.AnalyzeFracturesRequest{
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
			ID:                  f.Id,
			Component:           f.Component,
			FractureType:        f.FractureType,
			TimeToFracture:      time.Duration(f.TimeToFractureSeconds) * time.Second,
			Severity:            f.Severity,
			Probability:         f.Probability,
			Confidence:          f.Confidence,
			ContributingFactors: f.ContributingFactors,
			Recommendation:      f.Recommendation,
			PredictedAt:         time.Now(),
		}
	}

	return &models.FractureAnalysisResponse{
		Fractures:        fractures,
		ModelsUsed:       response.ModelsUsed,
		ProcessingTimeMs: response.ProcessingTimeMs,
	}, nil
}

func (c *PredictEngineClient) Close() error {
	return c.conn.Close()
}
