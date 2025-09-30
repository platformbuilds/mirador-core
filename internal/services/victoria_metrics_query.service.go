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

	// Construct the full MetricsQL query by wrapping the provided query with the function
	fullQuery, err := s.constructFunctionQuery(req.Function, req.Query, req.Params)
	if err != nil {
		s.logger.Error("Failed to construct function query", "error", err, "function", req.Function)
		return &models.MetricsQLFunctionResponse{
			Status:        "error",
			Error:         err.Error(),
			ExecutionTime: time.Since(start).Milliseconds(),
			Function:      req.Function,
		}, nil
	}

	// Create a MetricsQLQueryRequest for the underlying service
	queryReq := &models.MetricsQLQueryRequest{
		Query:    fullQuery,
		Time:     req.Time,
		Timeout:  req.Timeout,
		TenantID: req.TenantID,
	}

	// Execute the query using the underlying metrics service
	result, err := s.metricsService.ExecuteQuery(ctx, queryReq)
	if err != nil {
		s.logger.Error("Failed to execute MetricsQL function query", "error", err, "function", req.Function, "constructed_query", fullQuery)
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

	// Construct the full MetricsQL query by wrapping the provided query with the function
	fullQuery, err := s.constructFunctionQuery(req.Function, req.Query, req.Params)
	if err != nil {
		s.logger.Error("Failed to construct function range query", "error", err, "function", req.Function)
		return &models.MetricsQLFunctionResponse{
			Status:        "error",
			Error:         err.Error(),
			ExecutionTime: time.Since(start).Milliseconds(),
			Function:      req.Function,
		}, nil
	}

	// For range queries, we need to use ExecuteRangeQuery
	queryReq := &models.MetricsQLRangeQueryRequest{
		Query:    fullQuery,
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
			"constructed_query", fullQuery,
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

// constructFunctionQuery constructs a full MetricsQL query by wrapping the base query with the specified function
func (s *VictoriaMetricsQueryService) constructFunctionQuery(functionName, baseQuery string, params map[string]interface{}) (string, error) {
	// Validate that we have a base query
	if baseQuery == "" {
		return "", fmt.Errorf("base query cannot be empty")
	}

	// Construct the query based on the function type
	switch functionName {
	// Core rollup functions
	case "rate":
		return fmt.Sprintf("rate(%s)", baseQuery), nil
	case "increase":
		return fmt.Sprintf("increase(%s)", baseQuery), nil
	case "delta":
		return fmt.Sprintf("delta(%s)", baseQuery), nil
	case "irate":
		return fmt.Sprintf("irate(%s)", baseQuery), nil
	case "deriv":
		return fmt.Sprintf("deriv(%s)", baseQuery), nil
	case "idelta":
		return fmt.Sprintf("idelta(%s)", baseQuery), nil
	case "ideriv":
		return fmt.Sprintf("ideriv(%s)", baseQuery), nil
	case "absent_over_time":
		return fmt.Sprintf("absent_over_time(%s)", baseQuery), nil

	// Aggregation over time functions
	case "avg_over_time":
		return fmt.Sprintf("avg_over_time(%s)", baseQuery), nil
	case "min_over_time":
		return fmt.Sprintf("min_over_time(%s)", baseQuery), nil
	case "max_over_time":
		return fmt.Sprintf("max_over_time(%s)", baseQuery), nil
	case "sum_over_time":
		return fmt.Sprintf("sum_over_time(%s)", baseQuery), nil
	case "count_over_time":
		return fmt.Sprintf("count_over_time(%s)", baseQuery), nil
	case "quantile_over_time":
		// quantile_over_time needs a quantile parameter, check if it's provided
		if quantile, ok := params["quantile"]; ok {
			if q, ok := quantile.(float64); ok {
				return fmt.Sprintf("quantile_over_time(%.2f, %s)", q, baseQuery), nil
			}
		}
		return "", fmt.Errorf("quantile_over_time requires a 'quantile' parameter")

	// Statistical rollup functions
	case "stddev_over_time":
		return fmt.Sprintf("stddev_over_time(%s)", baseQuery), nil
	case "stdvar_over_time":
		return fmt.Sprintf("stdvar_over_time(%s)", baseQuery), nil
	case "mad_over_time":
		return fmt.Sprintf("mad_over_time(%s)", baseQuery), nil
	case "zscore_over_time":
		return fmt.Sprintf("zscore_over_time(%s)", baseQuery), nil
	case "distinct_over_time":
		return fmt.Sprintf("distinct_over_time(%s)", baseQuery), nil

	// Specialized rollup functions
	case "changes":
		return fmt.Sprintf("changes(%s)", baseQuery), nil
	case "resets":
		return fmt.Sprintf("resets(%s)", baseQuery), nil
	case "timestamp":
		return fmt.Sprintf("timestamp(%s)", baseQuery), nil
	case "histogram_over_time":
		return fmt.Sprintf("histogram_over_time(%s)", baseQuery), nil
	case "lifetime":
		return fmt.Sprintf("lifetime(%s)", baseQuery), nil
	case "lag":
		// lag might need a duration parameter, but for now implement without
		return fmt.Sprintf("lag(%s)", baseQuery), nil
	case "predict_linear":
		// predict_linear needs a duration parameter
		if duration, ok := params["duration"]; ok {
			if d, ok := duration.(string); ok {
				return fmt.Sprintf("predict_linear(%s, %s)", baseQuery, d), nil
			}
		}
		return "", fmt.Errorf("predict_linear requires a 'duration' parameter")
	case "holt_winters":
		// holt_winters needs sf (smoothing factor) and tf (trend factor) parameters
		sf, hasSF := params["sf"]
		tf, hasTF := params["tf"]
		if hasSF && hasTF {
			if sfVal, ok := sf.(float64); ok {
				if tfVal, ok := tf.(float64); ok {
					return fmt.Sprintf("holt_winters(%s, %.2f, %.2f)", baseQuery, sfVal, tfVal), nil
				}
			}
		}
		return "", fmt.Errorf("holt_winters requires 'sf' and 'tf' parameters")
	case "present_over_time":
		return fmt.Sprintf("present_over_time(%s)", baseQuery), nil
	case "last_over_time":
		return fmt.Sprintf("last_over_time(%s)", baseQuery), nil
	case "tmin_over_time":
		return fmt.Sprintf("tmin_over_time(%s)", baseQuery), nil
	case "tmax_over_time":
		return fmt.Sprintf("tmax_over_time(%s)", baseQuery), nil
	case "rollup":
		return fmt.Sprintf("rollup(%s)", baseQuery), nil
	case "rollup_rate":
		return fmt.Sprintf("rollup_rate(%s)", baseQuery), nil
	case "rollup_increase":
		return fmt.Sprintf("rollup_increase(%s)", baseQuery), nil
	case "rollup_delta":
		return fmt.Sprintf("rollup_delta(%s)", baseQuery), nil

	default:
		return "", fmt.Errorf("unsupported function: %s", functionName)
	}
}
