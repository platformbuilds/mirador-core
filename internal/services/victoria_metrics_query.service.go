package services

import (
	"context"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// VictoriaMetricsQueryService handles MetricsQL function-specific queries
type VictoriaMetricsQueryService struct {
	metricsService *VictoriaMetricsService
	logger         logger.Logger
}

// NewVictoriaMetricsQueryService creates a new VictoriaMetrics query service
func NewVictoriaMetricsQueryService(metricsService *VictoriaMetricsService, logger logger.Logger) *VictoriaMetricsQueryService {
	return &VictoriaMetricsQueryService{
		metricsService: metricsService,
		logger:         logger,
	}
}

// ExecuteFunctionQuery executes a MetricsQL function query
func (s *VictoriaMetricsQueryService) ExecuteFunctionQuery(ctx context.Context, req *models.MetricsQLFunctionRequest) (*models.MetricsQLFunctionResponse, error) {
	start := time.Now()

	// Create a MetricsQLQueryRequest for the underlying service
	queryReq := &models.MetricsQLQueryRequest{
		Query:    req.Query,
		Time:     req.Time, // Use Time field for instant queries
		Timeout:  req.Timeout,
		TenantID: req.TenantID,
	}

	// Execute the query using the underlying metrics service
	result, err := s.metricsService.ExecuteQuery(ctx, queryReq)
	if err != nil {
		s.logger.Error("Failed to execute MetricsQL function query", "error", err, "function", req.Function)
		return &models.MetricsQLFunctionResponse{
			Status:        "error",
			Error:         err.Error(),
			ExecutionTime: time.Since(start).Milliseconds(),
			Function:      req.Function,
		}, nil
	}

	return &models.MetricsQLFunctionResponse{
		Status:        result.Status,
		Data:          result.Data,
		ExecutionTime: time.Since(start).Milliseconds(),
		Function:      req.Function,
	}, nil
}

// ExecuteRangeFunctionQuery executes a MetricsQL function range query
func (s *VictoriaMetricsQueryService) ExecuteRangeFunctionQuery(ctx context.Context, req *models.MetricsQLFunctionRangeRequest) (*models.MetricsQLFunctionResponse, error) {
	start := time.Now()

	// For range queries, we need to use ExecuteRangeQuery
	queryReq := &models.MetricsQLRangeQueryRequest{
		Query:    req.Query,
		Start:    req.Start,
		End:      req.End,
		Step:     req.Step,
		TenantID: req.TenantID,
	}

	// Execute the range query using the underlying service
	result, err := s.metricsService.ExecuteRangeQuery(ctx, queryReq)
	if err != nil {
		s.logger.Error("MetricsQL function range query failed",
			"function", req.Function,
			"query", req.Query,
			"error", err)
		return &models.MetricsQLFunctionResponse{
			Status:        "error",
			Error:         err.Error(),
			ExecutionTime: time.Since(start).Milliseconds(),
			Function:      req.Function,
		}, nil
	}

	return &models.MetricsQLFunctionResponse{
		Status:        result.Status,
		Data:          result.Data,
		ExecutionTime: time.Since(start).Milliseconds(),
		Function:      req.Function,
	}, nil
}

// ValidateFunctionParameters validates function-specific parameters
func (s *VictoriaMetricsQueryService) ValidateFunctionParameters(function string, params map[string]interface{}) error {
	// Basic validation - can be extended for specific function requirements
	switch function {
	case "histogram_quantile":
		if phi, ok := params["phi"]; ok {
			if phiVal, ok := phi.(float64); !ok || phiVal < 0 || phiVal > 1 {
				return fmt.Errorf("phi must be a number between 0 and 1")
			}
		}
	case "quantile":
		if phi, ok := params["phi"]; ok {
			if phiVal, ok := phi.(float64); !ok || phiVal < 0 || phiVal > 1 {
				return fmt.Errorf("phi must be a number between 0 and 1")
			}
		}
		// Add more function-specific validations as needed
	}
	return nil
}
