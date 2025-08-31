package predict

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AnalyzeFracturesRequest represents a request to analyze system fractures
type AnalyzeFracturesRequest struct {
	Component  string   `json:"component"`
	TimeRange  string   `json:"time_range"`
	ModelTypes []string `json:"model_types"`
	TenantId   string   `json:"tenant_id"`
}

// AnalyzeFracturesResponse represents the response from fracture analysis
type AnalyzeFracturesResponse struct {
	Fractures        []*SystemFracture `json:"fractures"`
	ModelsUsed       []string          `json:"models_used"`
	ProcessingTimeMs int64             `json:"processing_time_ms"`
}

// SystemFracture represents a predicted system failure
type SystemFracture struct {
	Id                    string   `json:"id"`
	Component             string   `json:"component"`
	FractureType          string   `json:"fracture_type"`
	TimeToFractureSeconds int64    `json:"time_to_fracture_seconds"`
	Severity              string   `json:"severity"`
	Probability           float64  `json:"probability"`
	Confidence            float64  `json:"confidence"`
	ContributingFactors   []string `json:"contributing_factors"`
	Recommendation        string   `json:"recommendation"`
}

// GetFracturesRequest represents a request to get predicted fractures
type GetFracturesRequest struct {
	TimeRange     string  `json:"time_range"`
	MinProbability float64 `json:"min_probability"`
	TenantId      string  `json:"tenant_id"`
}

// GetFracturesResponse represents response with predicted fractures
type GetFracturesResponse struct {
	Fractures []*SystemFracture `json:"fractures"`
}

// GetModelsRequest represents a request for ML models
type GetModelsRequest struct {
	TenantId string `json:"tenant_id"`
}

// GetModelsResponse represents response with ML models
type GetModelsResponse struct {
	Models []*MLModel `json:"models"`
}

// MLModel represents a machine learning model
type MLModel struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Accuracy    float64 `json:"accuracy"`
	LastTrained string  `json:"last_trained"`
}

// GetHealthRequest represents a health check request
type GetHealthRequest struct{}

// GetHealthResponse represents a health check response
type GetHealthResponse struct {
	Status             string  `json:"status"`
	ModelsActive       int32   `json:"models_active"`
	PredictionsPerHour int32   `json:"predictions_per_hour"`
	Accuracy           float64 `json:"accuracy"`
	LastUpdate         string  `json:"last_update"`
}

// PredictEngineServiceClient is the client interface for PredictEngineService
type PredictEngineServiceClient interface {
	AnalyzeFractures(ctx context.Context, in *AnalyzeFracturesRequest, opts ...grpc.CallOption) (*AnalyzeFracturesResponse, error)
	GetPredictedFractures(ctx context.Context, in *GetFracturesRequest, opts ...grpc.CallOption) (*GetFracturesResponse, error)
	GetModels(ctx context.Context, in *GetModelsRequest, opts ...grpc.CallOption) (*GetModelsResponse, error)
	GetHealth(ctx context.Context, in *GetHealthRequest, opts ...grpc.CallOption) (*GetHealthResponse, error)
}

type predictEngineServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewPredictEngineServiceClient creates a new PredictEngineService client
func NewPredictEngineServiceClient(cc grpc.ClientConnInterface) PredictEngineServiceClient {
	return &predictEngineServiceClient{cc}
}

func (c *predictEngineServiceClient) AnalyzeFractures(ctx context.Context, in *AnalyzeFracturesRequest, opts ...grpc.CallOption) (*AnalyzeFracturesResponse, error) {
	out := new(AnalyzeFracturesResponse)
	err := c.cc.Invoke(ctx, "/mirador.predict.PredictEngineService/AnalyzeFractures", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictEngineServiceClient) GetPredictedFractures(ctx context.Context, in *GetFracturesRequest, opts ...grpc.CallOption) (*GetFracturesResponse, error) {
	out := new(GetFracturesResponse)
	err := c.cc.Invoke(ctx, "/mirador.predict.PredictEngineService/GetPredictedFractures", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictEngineServiceClient) GetModels(ctx context.Context, in *GetModelsRequest, opts ...grpc.CallOption) (*GetModelsResponse, error) {
	out := new(GetModelsResponse)
	err := c.cc.Invoke(ctx, "/mirador.predict.PredictEngineService/GetModels", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictEngineServiceClient) GetHealth(ctx context.Context, in *GetHealthRequest, opts ...grpc.CallOption) (*GetHealthResponse, error) {
	out := new(GetHealthResponse)
	err := c.cc.Invoke(ctx, "/mirador.predict.PredictEngineService/GetHealth", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PredictEngineServiceServer is the server interface for PredictEngineService
type PredictEngineServiceServer interface {
	AnalyzeFractures(context.Context, *AnalyzeFracturesRequest) (*AnalyzeFracturesResponse, error)
	GetPredictedFractures(context.Context, *GetFracturesRequest) (*GetFracturesResponse, error)
	GetModels(context.Context, *GetModelsRequest) (*GetModelsResponse, error)
	GetHealth(context.Context, *GetHealthRequest) (*GetHealthResponse, error)
}

// UnimplementedPredictEngineServiceServer should be embedded for forward compatibility
type UnimplementedPredictEngineServiceServer struct{}

func (UnimplementedPredictEngineServiceServer) AnalyzeFractures(context.Context, *AnalyzeFracturesRequest) (*AnalyzeFracturesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AnalyzeFractures not implemented")
}

func (UnimplementedPredictEngineServiceServer) GetPredictedFractures(context.Context, *GetFracturesRequest) (*GetFracturesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPredictedFractures not implemented")
}

func (UnimplementedPredictEngineServiceServer) GetModels(context.Context, *GetModelsRequest) (*GetModelsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetModels not implemented")
}

func (UnimplementedPredictEngineServiceServer) GetHealth(context.Context, *GetHealthRequest) (*GetHealthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetHealth not implemented")
}
