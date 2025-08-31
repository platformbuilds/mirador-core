package clients

import (
	"context"
	"time"

	pb "github.com/platformbuilds/miradorstack/internal/grpc/proto/rca"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RCAEngineClient struct {
	client pb.RCAEngineServiceClient
	conn   *grpc.ClientConn
	logger logger.Logger
}

func NewRCAEngineClient(endpoint string, logger logger.Logger) (*RCAEngineClient, error) {
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &RCAEngineClient{
		client: pb.NewRCAEngineServiceClient(conn),
		conn:   conn,
		logger: logger,
	}, nil
}

// InvestigateIncident uses Time and Anomaly Score Pattern (red anchors)
// across data from multiple data stores and types (metrics, logs, traces)
func (c *RCAEngineClient) InvestigateIncident(ctx context.Context, request *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
	grpcRequest := &pb.InvestigateRequest{
		IncidentId:       request.IncidentID,
		Symptoms:         request.Symptoms,
		TimeRange:        convertTimeRange(request.TimeRange),
		AffectedServices: request.AffectedServices,
		AnomalyThreshold: request.AnomalyThreshold,
	}

	response, err := c.client.InvestigateIncident(ctx, grpcRequest)
	if err != nil {
		c.logger.Error("RCA-ENGINE gRPC call failed", "incident", request.IncidentID, "error", err)
		return nil, err
	}

	// Convert red anchors (anomaly scores) from gRPC response
	redAnchors := make([]*models.RedAnchor, len(response.RedAnchors))
	for i, anchor := range response.RedAnchors {
		redAnchors[i] = &models.RedAnchor{
			Service:   anchor.Service,
			Metric:    anchor.Metric,
			Score:     anchor.AnomalyScore,
			Threshold: anchor.Threshold,
			Timestamp: time.Unix(anchor.TimestampUnix, 0),
			DataType:  anchor.DataType, // metrics, logs, or traces
		}
	}

	return &models.CorrelationResult{
		CorrelationID:    response.CorrelationId,
		IncidentID:       request.IncidentID,
		RootCause:        response.RootCause,
		Confidence:       response.Confidence,
		AffectedServices: response.AffectedServices,
		Timeline:         convertTimeline(response.Timeline),
		RedAnchors:       redAnchors,
		Recommendations:  response.Recommendations,
		CreatedAt:        time.Now(),
	}, nil
}

func (c *RCAEngineClient) Close() error {
	return c.conn.Close()
}
