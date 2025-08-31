package rca

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InvestigateRequest represents an RCA investigation request
type InvestigateRequest struct {
	IncidentId       string     `json:"incident_id"`
	Symptoms         []string   `json:"symptoms"`
	TimeRange        *TimeRange `json:"time_range"`
	AffectedServices []string   `json:"affected_services"`
	AnomalyThreshold float64    `json:"anomaly_threshold"`
}

// InvestigateResponse represents an RCA investigation response
type InvestigateResponse struct {
	CorrelationId    string           `json:"correlation_id"`
	RootCause        string           `json:"root_cause"`
	Confidence       float64          `json:"confidence"`
	AffectedServices []string         `json:"affected_services"`
	Timeline         []*TimelineEvent `json:"timeline"`
	RedAnchors       []*RedAnchor     `json:"red_anchors"`
	Recommendations  []string         `json:"recommendations"`
}

// RedAnchor represents anomaly score pattern for RCA analysis
type RedAnchor struct {
	Service       string  `json:"service"`
	Metric        string  `json:"metric"`
	AnomalyScore  float64 `json:"anomaly_score"`
	Threshold     float64 `json:"threshold"`
	TimestampUnix int64   `json:"timestamp_unix"`
	DataType      string  `json:"data_type"`
}

// TimeRange represents a time range for queries
type TimeRange struct {
	StartUnix int64 `json:"start_unix"`
	EndUnix   int64 `json:"end_unix"`
}

// TimelineEvent represents an event in the correlation timeline
type TimelineEvent struct {
	TimestampUnix int64   `json:"timestamp_unix"`
	Event         string  `json:"event"`
	Service       string  `json:"service"`
	Severity      string  `json:"severity"`
	AnomalyScore  float64 `json:"anomaly_score"`
}

// GetCorrelationsRequest represents a request for correlations
type GetCorrelationsRequest struct {
	TenantId string `json:"tenant_id"`
}

// GetCorrelationsResponse represents response with correlations
type GetCorrelationsResponse struct {
	Correlations []*Correlation `json:"correlations"`
}

// Correlation represents a correlation result
type Correlation struct {
	CorrelationId    string           `json:"correlation_id"`
	RootCause        string           `json:"root_cause"`
	Confidence       float64          `json:"confidence"`
	AffectedServices []string         `json:"affected_services"`
	Timeline         []*TimelineEvent `json:"timeline"`
	RedAnchors       []*RedAnchor     `json:"red_anchors"`
	Recommendations  []string         `json:"recommendations"`
}

// GetPatternsRequest represents a request for failure patterns
type GetPatternsRequest struct {
	TenantId string `json:"tenant_id"`
}

// GetPatternsResponse represents response with failure patterns
type GetPatternsResponse struct {
	Patterns []*FailurePattern `json:"patterns"`
}

// FailurePattern represents a failure pattern
type FailurePattern struct {
	PatternId   string   `json:"pattern_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Indicators  []string `json:"indicators"`
	Frequency   float64  `json:"frequency"`
}

// GetHealthRequest represents a health check request
type GetHealthRequest struct{}

// GetHealthResponse represents a health check response
type GetHealthResponse struct {
	Status              string  `json:"status"`
	ActiveCorrelations  int32   `json:"active_correlations"`
	AvgResolutionTime   float64 `json:"avg_resolution_time"`
	LastUpdate          string  `json:"last_update"`
}

// Getter methods for all structs
func (x *InvestigateRequest) GetIncidentId() string {
	if x != nil {
		return x.IncidentId
	}
	return ""
}

func (x *InvestigateRequest) GetSymptoms() []string {
	if x != nil {
		return x.Symptoms
	}
	return nil
}

func (x *InvestigateRequest) GetTimeRange() *TimeRange {
	if x != nil {
		return x.TimeRange
	}
	return nil
}

func (x *InvestigateRequest) GetAffectedServices() []string {
	if x != nil {
		return x.AffectedServices
	}
	return nil
}

func (x *InvestigateRequest) GetAnomalyThreshold() float64 {
	if x != nil {
		return x.AnomalyThreshold
	}
	return 0
}

func (x *InvestigateResponse) GetCorrelationId() string {
	if x != nil {
		return x.CorrelationId
	}
	return ""
}

func (x *InvestigateResponse) GetRootCause() string {
	if x != nil {
		return x.RootCause
	}
	return ""
}

func (x *InvestigateResponse) GetConfidence() float64 {
	if x != nil {
		return x.Confidence
	}
	return 0
}

func (x *InvestigateResponse) GetAffectedServices() []string {
	if x != nil {
		return x.AffectedServices
	}
	return nil
}

func (x *InvestigateResponse) GetTimeline() []*TimelineEvent {
	if x != nil {
		return x.Timeline
	}
	return nil
}

func (x *InvestigateResponse) GetRedAnchors() []*RedAnchor {
	if x != nil {
		return x.RedAnchors
	}
	return nil
}

func (x *InvestigateResponse) GetRecommendations() []string {
	if x != nil {
		return x.Recommendations
	}
	return nil
}

func (x *RedAnchor) GetService() string {
	if x != nil {
		return x.Service
	}
	return ""
}

func (x *RedAnchor) GetMetric() string {
	if x != nil {
		return x.Metric
	}
	return ""
}

func (x *RedAnchor) GetAnomalyScore() float64 {
	if x != nil {
		return x.AnomalyScore
	}
	return 0
}

func (x *RedAnchor) GetThreshold() float64 {
	if x != nil {
		return x.Threshold
	}
	return 0
}

func (x *RedAnchor) GetTimestampUnix() int64 {
	if x != nil {
		return x.TimestampUnix
	}
	return 0
}

func (x *RedAnchor) GetDataType() string {
	if x != nil {
		return x.DataType
	}
	return ""
}

func (x *TimeRange) GetStartUnix() int64 {
	if x != nil {
		return x.StartUnix
	}
	return 0
}

func (x *TimeRange) GetEndUnix() int64 {
	if x != nil {
		return x.EndUnix
	}
	return 0
}

func (x *TimelineEvent) GetTimestampUnix() int64 {
	if x != nil {
		return x.TimestampUnix
	}
	return 0
}

func (x *TimelineEvent) GetEvent() string {
	if x != nil {
		return x.Event
	}
	return ""
}

func (x *TimelineEvent) GetService() string {
	if x != nil {
		return x.Service
	}
	return ""
}

func (x *TimelineEvent) GetSeverity() string {
	if x != nil {
		return x.Severity
	}
	return ""
}

func (x *TimelineEvent) GetAnomalyScore() float64 {
	if x != nil {
		return x.AnomalyScore
	}
	return 0
}

func (x *GetHealthResponse) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *GetHealthResponse) GetActiveCorrelations() int32 {
	if x != nil {
		return x.ActiveCorrelations
	}
	return 0
}

func (x *GetHealthResponse) GetAvgResolutionTime() float64 {
	if x != nil {
		return x.AvgResolutionTime
	}
	return 0
}

func (x *GetHealthResponse) GetLastUpdate() string {
	if x != nil {
		return x.LastUpdate
	}
	return ""
}

// RCAEngineServiceClient is the client interface for RCAEngineService
type RCAEngineServiceClient interface {
	InvestigateIncident(ctx context.Context, in *InvestigateRequest, opts ...grpc.CallOption) (*InvestigateResponse, error)
	GetActiveCorrelations(ctx context.Context, in *GetCorrelationsRequest, opts ...grpc.CallOption) (*GetCorrelationsResponse, error)
	GetFailurePatterns(ctx context.Context, in *GetPatternsRequest, opts ...grpc.CallOption) (*GetPatternsResponse, error)
	GetHealth(ctx context.Context, in *GetHealthRequest, opts ...grpc.CallOption) (*GetHealthResponse, error)
}

type rcaEngineServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewRCAEngineServiceClient creates a new RCAEngineService client
func NewRCAEngineServiceClient(cc grpc.ClientConnInterface) RCAEngineServiceClient {
	return &rcaEngineServiceClient{cc}
}

func (c *rcaEngineServiceClient) InvestigateIncident(ctx context.Context, in *InvestigateRequest, opts ...grpc.CallOption) (*InvestigateResponse, error) {
	out := new(InvestigateResponse)
	err := c.cc.Invoke(ctx, "/mirador.rca.RCAEngineService/InvestigateIncident", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rcaEngineServiceClient) GetActiveCorrelations(ctx context.Context, in *GetCorrelationsRequest, opts ...grpc.CallOption) (*GetCorrelationsResponse, error) {
	out := new(GetCorrelationsResponse)
	err := c.cc.Invoke(ctx, "/mirador.rca.RCAEngineService/GetActiveCorrelations", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rcaEngineServiceClient) GetFailurePatterns(ctx context.Context, in *GetPatternsRequest, opts ...grpc.CallOption) (*GetPatternsResponse, error) {
	out := new(GetPatternsResponse)
	err := c.cc.Invoke(ctx, "/mirador.rca.RCAEngineService/GetFailurePatterns", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rcaEngineServiceClient) GetHealth(ctx context.Context, in *GetHealthRequest, opts ...grpc.CallOption) (*GetHealthResponse, error) {
	out := new(GetHealthResponse)
	err := c.cc.Invoke(ctx, "/mirador.rca.RCAEngineService/GetHealth", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
