package clients

import (
	"context"
	"time"

	pb "github.com/platformbuilds/miradorstack/internal/grpc/proto/alert"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AlertEngineClient struct {
	client pb.AlertEngineServiceClient
	conn   *grpc.ClientConn
	logger logger.Logger
}

func NewAlertEngineClient(endpoint string, logger logger.Logger) (*AlertEngineClient, error) {
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &AlertEngineClient{
		client: pb.NewAlertEngineServiceClient(conn),
		conn:   conn,
		logger: logger,
	}, nil
}

// ProcessAlert handles intelligent alert processing with noise reduction
func (c *AlertEngineClient) ProcessAlert(ctx context.Context, alert *models.Alert) (*models.ProcessedAlert, error) {
	grpcRequest := &pb.ProcessAlertRequest{
		Alert: &pb.Alert{
			Id:          alert.ID,
			Severity:    alert.Severity,
			Component:   alert.Component,
			Message:     alert.Message,
			Timestamp:   alert.Timestamp.Unix(),
			Labels:      alert.Labels,
			Annotations: alert.Annotations,
		},
		TenantId: alert.TenantID,
	}

	response, err := c.client.ProcessAlert(ctx, grpcRequest)
	if err != nil {
		c.logger.Error("ALERT-ENGINE gRPC call failed", "alertId", alert.ID, "error", err)
		return nil, err
	}

	return &models.ProcessedAlert{
		OriginalID:    alert.ID,
		ProcessedID:   response.ProcessedAlert.Id,
		Action:        response.ProcessedAlert.Action, // fire, suppress, escalate, cluster
		ClusterID:     response.ProcessedAlert.ClusterId,
		Escalation:    response.ProcessedAlert.Escalation,
		Notifications: response.ProcessedAlert.Notifications,
		ProcessedAt:   time.Now(),
	}, nil
}

func (c *AlertEngineClient) GetAlertRules(ctx context.Context, tenantID string) ([]*models.AlertRule, error) {
	response, err := c.client.GetAlertRules(ctx, &pb.GetAlertRulesRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return nil, err
	}

	rules := make([]*models.AlertRule, len(response.Rules))
	for i, rule := range response.Rules {
		rules[i] = &models.AlertRule{
			ID:          rule.Id,
			Name:        rule.Name,
			Query:       rule.Query,
			Condition:   rule.Condition,
			Severity:    rule.Severity,
			Enabled:     rule.Enabled,
			Labels:      rule.Labels,
			Annotations: rule.Annotations,
		}
	}

	return rules, nil
}

func (c *AlertEngineClient) Close() error {
	return c.conn.Close()
}
