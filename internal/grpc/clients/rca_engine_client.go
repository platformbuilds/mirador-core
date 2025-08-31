package clients

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/platformbuilds/miradorstack/internal/grpc/proto/rca"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

// RCAEngineClient wraps the gRPC client for RCA-ENGINE
type RCAEngineClient struct {
	client rca.RCAEngineServiceClient
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewRCAEngineClient creates a new RCA-ENGINE gRPC client
func NewRCAEngineClient(endpoint string, logger logger.Logger) (*RCAEngineClient, error) {
	conn, err := grpc.Dial(endpoint, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RCA-ENGINE: %w", err)
	}

	client := rca.NewRCAEngineServiceClient(conn)

	return &RCAEngineClient{
		client: client,
		conn:   conn,
		logger: logger,
	}, nil
}

// InvestigateIncident uses Time and Anomaly Score Pattern (red anchors)
func (c *RCAEngineClient) InvestigateIncident(ctx context.Context, request *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
	grpcRequest := &rca.InvestigateRequest{
		IncidentId:       request.IncidentID,
		Symptoms:         request.Symptoms,
		TimeRange:        convertTimeRangeToGRPC(request.TimeRange),
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
			DataType:  anchor.DataType,
		}
	}

	return &models.CorrelationResult{
		CorrelationID:    response.CorrelationId,
		IncidentID:       request.IncidentID,
		RootCause:        response.RootCause,
		Confidence:       response.Confidence,
		AffectedServices: response.AffectedServices,
		Timeline:         convertTimelineFromGRPC(response.Timeline),
		RedAnchors:       redAnchors,
		Recommendations:  response.Recommendations,
		CreatedAt:        time.Now(),
	}, nil
}

// HealthCheck checks the health of the RCA-ENGINE
func (c *RCAEngineClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.GetHealth(ctx, &rca.GetHealthRequest{})
	return err
}

// Close closes the gRPC connection
func (c *RCAEngineClient) Close() error {
	return c.conn.Close()
}

// Helper functions
func convertTimeRangeToGRPC(tr models.TimeRange) *rca.TimeRange {
	return &rca.TimeRange{
		StartUnix: tr.Start.Unix(),
		EndUnix:   tr.End.Unix(),
	}
}

func convertTimelineFromGRPC(timeline []*rca.TimelineEvent) []models.TimelineEvent {
	events := make([]models.TimelineEvent, len(timeline))
	for i, event := range timeline {
		events[i] = models.TimelineEvent{
			Time:         time.Unix(event.TimestampUnix, 0),
			Event:        event.Event,
			Service:      event.Service,
			Severity:     event.Severity,
			AnomalyScore: event.AnomalyScore,
			DataSource:   event.DataType,
		}
	}
	return events
}
