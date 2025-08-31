package clients

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/mirador/core/internal/grpc/proto/alert"
	"github.com/mirador/core/internal/models"
	"github.com/mirador/core/pkg/logger"
)

// AlertEngineClient wraps the gRPC client for ALERT-ENGINE
type AlertEngineClient struct {
	client alert.AlertEngineServiceClient
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewAlertEngineClient creates a new ALERT-ENGINE gRPC client
func NewAlertEngineClient(endpoint string, logger logger.Logger) (*AlertEngineClient, error) {
	conn, err := grpc.Dial(endpoint, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ALERT-ENGINE: %w", err)
	}

	client := alert.NewAlertEngineServiceClient(conn)

	return &AlertEngineClient{
		client: client,
		conn:   conn,
		logger: logger,
	}, nil
}

// ProcessAlert handles intelligent alert processing with noise reduction
func (c *AlertEngineClient) ProcessAlert(ctx context.Context, alertModel *models.Alert) (*models.ProcessedAlert, error) {
	grpcRequest := &alert.ProcessAlertRequest{
		Alert: &alert.Alert{
			Id:          alertModel.ID,
			Severity:    alertModel.Severity,
			Component:   alertModel.Component,
			Message:     alertModel.Message,
			Timestamp:   alertModel.Timestamp.Unix(),
			Labels:      alertModel.Labels,
			Annotations: alertModel.Annotations,
		},
		TenantId: alertModel.TenantID,
	}

	response, err := c.client.ProcessAlert(ctx, grpcRequest)
	if err != nil {
		c.logger.Error("ALERT-ENGINE gRPC call failed", "alertId", alertModel.ID, "error", err)
		return nil, err
	}

	return &models.ProcessedAlert{
		OriginalID:     alertModel.ID,
		ProcessedID:    response.ProcessedAlert.Id,
		Action:         response.ProcessedAlert.Action,
		ClusterID:      response.ProcessedAlert.ClusterId,
		Escalation:     response.ProcessedAlert.Escalation,
		Notifications:  response.ProcessedAlert.Notifications,
		ProcessedAt:    time.Now(),
	}, nil
}

// GetAlertRules retrieves alert rules for a tenant
func (c *AlertEngineClient) GetAlertRules(ctx context.Context, tenantID string) ([]*models.AlertRule, error) {
	response, err := c.client.GetAlertRules(ctx, &alert.GetAlertRulesRequest{
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
			TenantID:    tenantID,
		}
	}

	return rules, nil
}

// CreateAlertRule creates a new alert rule
func (c *AlertEngineClient) CreateAlertRule(ctx context.Context, rule *models.AlertRule) (*models.AlertRule, error) {
	grpcRequest := &alert.CreateAlertRuleRequest{
		Rule: &alert.AlertRule{
			Name:        rule.Name,
			Query:       rule.Query,
			Condition:   rule.Condition,
			Severity:    rule.Severity,
			Enabled:     rule.Enabled,
			Labels:      rule.Labels,
			Annotations: rule.Annotations,
		},
		TenantId: rule.TenantID,
	}

	response, err := c.client.CreateAlertRule(ctx, grpcRequest)
	if err != nil {
		return nil, err
	}

	return &models.AlertRule{
		ID:          response.Rule.Id,
		Name:        response.Rule.Name,
		Query:       response.Rule.Query,
		Condition:   response.Rule.Condition,
		Severity:    response.Rule.Severity,
		Enabled:     response.Rule.Enabled,
		Labels:      response.Rule.Labels,
		Annotations: response.Rule.Annotations,
		TenantID:    rule.TenantID,
		CreatedAt:   time.Now(),
	}, nil
}

// GetActiveAlerts retrieves active alerts for a tenant
func (c *AlertEngineClient) GetActiveAlerts(ctx context.Context, query *models.AlertQuery) ([]*models.Alert, error) {
	grpcRequest := &alert.GetActiveAlertsRequest{
		TenantId: query.TenantID,
		Limit:    int32(query.Limit),
		Severity: query.Severity,
	}

	response, err := c.client.GetActiveAlerts(ctx, grpcRequest)
	if err != nil {
		return nil, err
	}

	alerts := make([]*models.Alert, len(response.Alerts))
	for i, a := range response.Alerts {
		alerts[i] = &models.Alert{
			ID:          a.Id,
			Severity:    a.Severity,
			Component:   a.Component,
			Message:     a.Message,
			Timestamp:   time.Unix(a.Timestamp, 0),
			Labels:      a.Labels,
			Annotations: a.Annotations,
			TenantID:    query.TenantID,
		}
	}

	return alerts, nil
}

// AcknowledgeAlert acknowledges an alert
func (c *AlertEngineClient) AcknowledgeAlert(ctx context.Context, ack *models.AlertAcknowledgment) error {
	grpcRequest := &alert.AcknowledgeAlertRequest{
		AlertId:        ack.AlertID,
		AcknowledgedBy: ack.AcknowledgedBy,
		Comment:        ack.Comment,
	}

	response, err := c.client.AcknowledgeAlert(ctx, grpcRequest)
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("failed to acknowledge alert: %s", response.Message)
	}

	return nil
}

// HealthCheck checks the health of the ALERT-ENGINE
func (c *AlertEngineClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.GetHealth(ctx, &alert.GetHealthRequest{})
	return err
}

// Close closes the gRPC connection
func (c *AlertEngineClient) Close() error {
	return c.conn.Close()
}
