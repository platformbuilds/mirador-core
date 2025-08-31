package alert

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProcessAlertRequest represents an alert processing request
type ProcessAlertRequest struct {
	Alert    *Alert `json:"alert"`
	TenantId string `json:"tenant_id"`
}

// ProcessAlertResponse represents an alert processing response
type ProcessAlertResponse struct {
	ProcessedAlert *ProcessedAlert `json:"processed_alert"`
}

// Alert represents an alert
type Alert struct {
	Id          string            `json:"id"`
	Severity    string            `json:"severity"`
	Component   string            `json:"component"`
	Message     string            `json:"message"`
	Timestamp   int64             `json:"timestamp"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// ProcessedAlert represents a processed alert
type ProcessedAlert struct {
	Id            string   `json:"id"`
	Action        string   `json:"action"`
	ClusterId     string   `json:"cluster_id"`
	Escalation    string   `json:"escalation"`
	Notifications []string `json:"notifications"`
}

// GetAlertRulesRequest represents a request for alert rules
type GetAlertRulesRequest struct {
	TenantId string `json:"tenant_id"`
}

// GetAlertRulesResponse represents response with alert rules
type GetAlertRulesResponse struct {
	Rules []*AlertRule `json:"rules"`
}

// AlertRule represents an alert rule
type AlertRule struct {
	Id          string            `json:"id"`
	Name        string            `json:"name"`
	Query       string            `json:"query"`
	Condition   string            `json:"condition"`
	Severity    string            `json:"severity"`
	Enabled     bool              `json:"enabled"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// CreateAlertRuleRequest represents a request to create an alert rule
type CreateAlertRuleRequest struct {
	Rule     *AlertRule `json:"rule"`
	TenantId string     `json:"tenant_id"`
}

// CreateAlertRuleResponse represents response from creating an alert rule
type CreateAlertRuleResponse struct {
	Rule *AlertRule `json:"rule"`
}

// GetActiveAlertsRequest represents a request for active alerts
type GetActiveAlertsRequest struct {
	TenantId string `json:"tenant_id"`
	Limit    int32  `json:"limit"`
	Severity string `json:"severity"`
}

// GetActiveAlertsResponse represents response with active alerts
type GetActiveAlertsResponse struct {
	Alerts []*Alert `json:"alerts"`
	Total  int32    `json:"total"`
}

// AcknowledgeAlertRequest represents a request to acknowledge an alert
type AcknowledgeAlertRequest struct {
	AlertId        string `json:"alert_id"`
	AcknowledgedBy string `json:"acknowledged_by"`
	Comment        string `json:"comment"`
}

// AcknowledgeAlertResponse represents response from acknowledging an alert
type AcknowledgeAlertResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetHealthRequest represents a health check request
type GetHealthRequest struct{}

// GetHealthResponse represents a health check response
type GetHealthResponse struct {
	Status      string `json:"status"`
	ActiveAlerts int32  `json:"active_alerts"`
	RulesCount   int32  `json:"rules_count"`
	LastUpdate   string `json:"last_update"`
}

// Getter methods for all structs
func (x *ProcessAlertRequest) GetAlert() *Alert {
	if x != nil {
		return x.Alert
	}
	return nil
}

func (x *ProcessAlertRequest) GetTenantId() string {
	if x != nil {
		return x.TenantId
	}
	return ""
}

func (x *ProcessAlertResponse) GetProcessedAlert() *ProcessedAlert {
	if x != nil {
		return x.ProcessedAlert
	}
	return nil
}

func (x *Alert) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Alert) GetSeverity() string {
	if x != nil {
		return x.Severity
	}
	return ""
}

func (x *Alert) GetComponent() string {
	if x != nil {
		return x.Component
	}
	return ""
}

func (x *Alert) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *Alert) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *Alert) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *Alert) GetAnnotations() map[string]string {
	if x != nil {
		return x.Annotations
	}
	return nil
}

func (x *ProcessedAlert) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ProcessedAlert) GetAction() string {
	if x != nil {
		return x.Action
	}
	return ""
}

func (x *ProcessedAlert) GetClusterId() string {
	if x != nil {
		return x.ClusterId
	}
	return ""
}

func (x *ProcessedAlert) GetEscalation() string {
	if x != nil {
		return x.Escalation
	}
	return ""
}

func (x *ProcessedAlert) GetNotifications() []string {
	if x != nil {
		return x.Notifications
	}
	return nil
}

func (x *GetAlertRulesRequest) GetTenantId() string {
	if x != nil {
		return x.TenantId
	}
	return ""
}

func (x *GetAlertRulesResponse) GetRules() []*AlertRule {
	if x != nil {
		return x.Rules
	}
	return nil
}

func (x *AlertRule) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *AlertRule) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *AlertRule) GetQuery() string {
	if x != nil {
		return x.Query
	}
	return ""
}

func (x *AlertRule) GetCondition() string {
	if x != nil {
		return x.Condition
	}
	return ""
}

func (x *AlertRule) GetSeverity() string {
	if x != nil {
		return x.Severity
	}
	return ""
}

func (x *AlertRule) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

func (x *AlertRule) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *AlertRule) GetAnnotations() map[string]string {
	if x != nil {
		return x.Annotations
	}
	return nil
}

func (x *GetHealthResponse) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *GetHealthResponse) GetActiveAlerts() int32 {
	if x != nil {
		return x.ActiveAlerts
	}
	return 0
}

func (x *GetHealthResponse) GetRulesCount() int32 {
	if x != nil {
		return x.RulesCount
	}
	return 0
}

func (x *GetHealthResponse) GetLastUpdate() string {
	if x != nil {
		return x.LastUpdate
	}
	return ""
}

// AlertEngineServiceClient is the client interface for AlertEngineService
type AlertEngineServiceClient interface {
	ProcessAlert(ctx context.Context, in *ProcessAlertRequest, opts ...grpc.CallOption) (*ProcessAlertResponse, error)
	GetAlertRules(ctx context.Context, in *GetAlertRulesRequest, opts ...grpc.CallOption) (*GetAlertRulesResponse, error)
	CreateAlertRule(ctx context.Context, in *CreateAlertRuleRequest, opts ...grpc.CallOption) (*CreateAlertRuleResponse, error)
	GetActiveAlerts(ctx context.Context, in *GetActiveAlertsRequest, opts ...grpc.CallOption) (*GetActiveAlertsResponse, error)
	AcknowledgeAlert(ctx context.Context, in *AcknowledgeAlertRequest, opts ...grpc.CallOption) (*AcknowledgeAlertResponse, error)
	GetHealth(ctx context.Context, in *GetHealthRequest, opts ...grpc.CallOption) (*GetHealthResponse, error)
}

type alertEngineServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewAlertEngineServiceClient creates a new AlertEngineService client
func NewAlertEngineServiceClient(cc grpc.ClientConnInterface) AlertEngineServiceClient {
	return &alertEngineServiceClient{cc}
}

func (c *alertEngineServiceClient) ProcessAlert(ctx context.Context, in *ProcessAlertRequest, opts ...grpc.CallOption) (*ProcessAlertResponse, error) {
	out := new(ProcessAlertResponse)
	err := c.cc.Invoke(ctx, "/mirador.alert.AlertEngineService/ProcessAlert", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *alertEngineServiceClient) GetAlertRules(ctx context.Context, in *GetAlertRulesRequest, opts ...grpc.CallOption) (*GetAlertRulesResponse, error) {
	out := new(GetAlertRulesResponse)
	err := c.cc.Invoke(ctx, "/mirador.alert.AlertEngineService/GetAlertRules", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *alertEngineServiceClient) CreateAlertRule(ctx context.Context, in *CreateAlertRuleRequest, opts ...grpc.CallOption) (*CreateAlertRuleResponse, error) {
	out := new(CreateAlertRuleResponse)
	err := c.cc.Invoke(ctx, "/mirador.alert.AlertEngineService/CreateAlertRule", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *alertEngineServiceClient) GetActiveAlerts(ctx context.Context, in *GetActiveAlertsRequest, opts ...grpc.CallOption) (*GetActiveAlertsResponse, error) {
	out := new(GetActiveAlertsResponse)
	err := c.cc.Invoke(ctx, "/mirador.alert.AlertEngineService/GetActiveAlerts", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *alertEngineServiceClient) AcknowledgeAlert(ctx context.Context, in *AcknowledgeAlertRequest, opts ...grpc.CallOption) (*AcknowledgeAlertResponse, error) {
	out := new(AcknowledgeAlertResponse)
	err := c.cc.Invoke(ctx, "/mirador.alert.AlertEngineService/AcknowledgeAlert", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *alertEngineServiceClient) GetHealth(ctx context.Context, in *GetHealthRequest, opts ...grpc.CallOption) (*GetHealthResponse, error) {
	out := new(GetHealthResponse)
	err := c.cc.Invoke(ctx, "/mirador.alert.AlertEngineService/GetHealth", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
