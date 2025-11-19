package services

import (
	"context"
	"fmt"
	"strings"
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
		Query:   fullQuery,
		Time:    req.Time,
		Timeout: req.Timeout,
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
		Query: fullQuery,
		Start: req.Start,
		End:   req.End,
		Step:  req.Step,
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

	// Mathematical transform functions
	case "abs":
		return fmt.Sprintf("abs(%s)", baseQuery), nil
	case "ceil":
		return fmt.Sprintf("ceil(%s)", baseQuery), nil
	case "floor":
		return fmt.Sprintf("floor(%s)", baseQuery), nil
	case "round":
		return fmt.Sprintf("round(%s)", baseQuery), nil
	case "exp":
		return fmt.Sprintf("exp(%s)", baseQuery), nil
	case "ln":
		return fmt.Sprintf("ln(%s)", baseQuery), nil
	case "log2":
		return fmt.Sprintf("log2(%s)", baseQuery), nil
	case "log10":
		return fmt.Sprintf("log10(%s)", baseQuery), nil
	case "sqrt":
		return fmt.Sprintf("sqrt(%s)", baseQuery), nil
	// Trigonometric functions
	case "acos":
		return fmt.Sprintf("acos(%s)", baseQuery), nil
	case "acosh":
		return fmt.Sprintf("acosh(%s)", baseQuery), nil
	case "asin":
		return fmt.Sprintf("asin(%s)", baseQuery), nil
	case "asinh":
		return fmt.Sprintf("asinh(%s)", baseQuery), nil
	case "atan":
		return fmt.Sprintf("atan(%s)", baseQuery), nil
	case "atanh":
		return fmt.Sprintf("atanh(%s)", baseQuery), nil
	case "cos":
		return fmt.Sprintf("cos(%s)", baseQuery), nil
	case "cosh":
		return fmt.Sprintf("cosh(%s)", baseQuery), nil
	case "sin":
		return fmt.Sprintf("sin(%s)", baseQuery), nil
	case "sinh":
		return fmt.Sprintf("sinh(%s)", baseQuery), nil
	case "tan":
		return fmt.Sprintf("tan(%s)", baseQuery), nil
	case "tanh":
		return fmt.Sprintf("tanh(%s)", baseQuery), nil

	// Time-related transform functions
	case "day_of_month":
		return fmt.Sprintf("day_of_month(%s)", baseQuery), nil
	case "day_of_week":
		return fmt.Sprintf("day_of_week(%s)", baseQuery), nil
	case "day_of_year":
		return fmt.Sprintf("day_of_year(%s)", baseQuery), nil
	case "hour":
		return fmt.Sprintf("hour(%s)", baseQuery), nil
	case "minute":
		return fmt.Sprintf("minute(%s)", baseQuery), nil
	case "month":
		return fmt.Sprintf("month(%s)", baseQuery), nil
	case "year":
		return fmt.Sprintf("year(%s)", baseQuery), nil
	case "time":
		return fmt.Sprintf("time(%s)", baseQuery), nil
	case "now":
		return fmt.Sprintf("now(%s)", baseQuery), nil
	case "timezone_offset":
		return fmt.Sprintf("timezone_offset(%s)", baseQuery), nil

	// Data manipulation functions
	case "clamp":
		return fmt.Sprintf("clamp(%s)", baseQuery), nil
	case "clamp_max":
		return fmt.Sprintf("clamp_max(%s)", baseQuery), nil
	case "clamp_min":
		return fmt.Sprintf("clamp_min(%s)", baseQuery), nil
	case "interpolate":
		return fmt.Sprintf("interpolate(%s)", baseQuery), nil
	case "keep_last_value":
		return fmt.Sprintf("keep_last_value(%s)", baseQuery), nil
	case "keep_next_value":
		return fmt.Sprintf("keep_next_value(%s)", baseQuery), nil
	case "remove_resets":
		return fmt.Sprintf("remove_resets(%s)", baseQuery), nil
	case "scalar":
		return fmt.Sprintf("scalar(%s)", baseQuery), nil
	case "union":
		return fmt.Sprintf("union(%s)", baseQuery), nil
	case "vector":
		return fmt.Sprintf("vector(%s)", baseQuery), nil

	// Histogram transform functions
	case "histogram_quantile":
		// histogram_quantile needs a quantile parameter
		if quantile, ok := params["quantile"]; ok {
			if q, ok := quantile.(float64); ok {
				return fmt.Sprintf("histogram_quantile(%.2f, %s)", q, baseQuery), nil
			}
		}
		return "", fmt.Errorf("histogram_quantile requires a 'quantile' parameter")
	case "histogram_avg":
		return fmt.Sprintf("histogram_avg(%s)", baseQuery), nil
	case "histogram_stddev":
		return fmt.Sprintf("histogram_stddev(%s)", baseQuery), nil
	case "prometheus_buckets":
		return fmt.Sprintf("prometheus_buckets(%s)", baseQuery), nil

	// Additional transform functions
	case "pi":
		return "pi()", nil
	case "rad":
		return fmt.Sprintf("rad(%s)", baseQuery), nil
	case "deg":
		return fmt.Sprintf("deg(%s)", baseQuery), nil
	case "sgn":
		return fmt.Sprintf("sgn(%s)", baseQuery), nil
	case "range_linear":
		return fmt.Sprintf("range_linear(%s)", baseQuery), nil
	case "range_vector":
		return fmt.Sprintf("range_vector(%s)", baseQuery), nil
	case "running_sum":
		return fmt.Sprintf("running_sum(%s)", baseQuery), nil
	case "running_avg":
		return fmt.Sprintf("running_avg(%s)", baseQuery), nil
	case "running_min":
		return fmt.Sprintf("running_min(%s)", baseQuery), nil
	case "running_max":
		return fmt.Sprintf("running_max(%s)", baseQuery), nil
	case "rand":
		return "rand()", nil
	case "rand_normal":
		return "rand_normal()", nil
	case "sort":
		return fmt.Sprintf("sort(%s)", baseQuery), nil
	case "smooth_exponential":
		return fmt.Sprintf("smooth_exponential(%s)", baseQuery), nil

	// Label manipulation functions
	case "alias":
		// alias needs a new label name parameter
		if newLabel, ok := params["label"]; ok {
			if label, ok := newLabel.(string); ok {
				return fmt.Sprintf("alias(%s, %s)", baseQuery, label), nil
			}
		}
		return "", fmt.Errorf("alias requires a 'label' parameter")
	case "label_set":
		// label_set needs label and value parameters
		label, hasLabel := params["label"]
		value, hasValue := params["value"]
		if hasLabel && hasValue {
			if l, ok := label.(string); ok {
				if v, ok := value.(string); ok {
					return fmt.Sprintf("label_set(%s, %s, %s)", l, v, baseQuery), nil
				}
			}
		}
		return "", fmt.Errorf("label_set requires 'label' and 'value' parameters")
	case "label_del":
		// label_del can take multiple label names
		if labels, ok := params["labels"]; ok {
			if labelList, ok := labels.([]string); ok && len(labelList) > 0 {
				labelStr := strings.Join(labelList, ",")
				return fmt.Sprintf("label_del(%s, %s)", baseQuery, labelStr), nil
			}
		}
		return "", fmt.Errorf("label_del requires a 'labels' parameter with label names")
	case "label_keep":
		// label_keep can take multiple label names
		if labels, ok := params["labels"]; ok {
			if labelList, ok := labels.([]string); ok && len(labelList) > 0 {
				labelStr := strings.Join(labelList, ",")
				return fmt.Sprintf("label_keep(%s, %s)", baseQuery, labelStr), nil
			}
		}
		return "", fmt.Errorf("label_keep requires a 'labels' parameter with label names")
	case "label_copy":
		// label_copy needs src and dst label parameters
		src, hasSrc := params["src"]
		dst, hasDst := params["dst"]
		if hasSrc && hasDst {
			if s, ok := src.(string); ok {
				if d, ok := dst.(string); ok {
					return fmt.Sprintf("label_copy(%s, %s, %s)", s, d, baseQuery), nil
				}
			}
		}
		return "", fmt.Errorf("label_copy requires 'src' and 'dst' parameters")
	case "label_move":
		// label_move needs src and dst label parameters
		src, hasSrc := params["src"]
		dst, hasDst := params["dst"]
		if hasSrc && hasDst {
			if s, ok := src.(string); ok {
				if d, ok := dst.(string); ok {
					return fmt.Sprintf("label_move(%s, %s, %s)", s, d, baseQuery), nil
				}
			}
		}
		return "", fmt.Errorf("label_move requires 'src' and 'dst' parameters")
	case "label_join":
		// label_join needs dst_label, separator, and src_labels parameters
		dst, hasDst := params["dst"]
		separator, hasSep := params["separator"]
		srcLabels, hasSrc := params["src_labels"]
		if hasDst && hasSep && hasSrc {
			if d, ok := dst.(string); ok {
				if sep, ok := separator.(string); ok {
					if srcList, ok := srcLabels.([]string); ok && len(srcList) > 0 {
						srcStr := strings.Join(srcList, ",")
						return fmt.Sprintf("label_join(%s, %s, %s, %s)", d, sep, srcStr, baseQuery), nil
					}
				}
			}
		}
		return "", fmt.Errorf("label_join requires 'dst', 'separator', and 'src_labels' parameters")
	case "label_replace":
		// label_replace needs dst_label, replacement, src_label, and regex parameters
		dst, hasDst := params["dst"]
		replacement, hasRep := params["replacement"]
		src, hasSrc := params["src"]
		regex, hasRegex := params["regex"]
		if hasDst && hasRep && hasSrc && hasRegex {
			if d, ok := dst.(string); ok {
				if rep, ok := replacement.(string); ok {
					if s, ok := src.(string); ok {
						if reg, ok := regex.(string); ok {
							return fmt.Sprintf("label_replace(%s, %s, %s, %s, %s)", baseQuery, d, rep, s, reg), nil
						}
					}
				}
			}
		}
		return "", fmt.Errorf("label_replace requires 'dst', 'replacement', 'src', and 'regex' parameters")
	case "label_map":
		// label_map needs label and mapping parameters
		label, hasLabel := params["label"]
		mapping, hasMapping := params["mapping"]
		if hasLabel && hasMapping {
			if l, ok := label.(string); ok {
				if m, ok := mapping.(map[string]string); ok {
					// Convert mapping to string format: key1=value1,key2=value2
					var mappingPairs []string
					for k, v := range m {
						mappingPairs = append(mappingPairs, fmt.Sprintf("%s=%s", k, v))
					}
					mappingStr := strings.Join(mappingPairs, ",")
					return fmt.Sprintf("label_map(%s, %s, %s)", baseQuery, l, mappingStr), nil
				}
			}
		}
		return "", fmt.Errorf("label_map requires 'label' and 'mapping' parameters")
	case "label_transform":
		// label_transform needs label, regex, replacement, and optionally separator
		label, hasLabel := params["label"]
		regex, hasRegex := params["regex"]
		replacement, hasRep := params["replacement"]
		if hasLabel && hasRegex && hasRep {
			if l, ok := label.(string); ok {
				if reg, ok := regex.(string); ok {
					if rep, ok := replacement.(string); ok {
						if sep, hasSep := params["separator"]; hasSep {
							if s, ok := sep.(string); ok {
								return fmt.Sprintf("label_transform(%s, %s, %s, %s, %s)", baseQuery, l, reg, rep, s), nil
							}
						}
						return fmt.Sprintf("label_transform(%s, %s, %s, %s)", baseQuery, l, reg, rep), nil
					}
				}
			}
		}
		return "", fmt.Errorf("label_transform requires 'label', 'regex', and 'replacement' parameters")
	case "label_lowercase":
		// label_lowercase needs label parameter
		if label, ok := params["label"]; ok {
			if l, ok := label.(string); ok {
				return fmt.Sprintf("label_lowercase(%s, %s)", baseQuery, l), nil
			}
		}
		return "", fmt.Errorf("label_lowercase requires a 'label' parameter")
	case "label_uppercase":
		// label_uppercase needs label parameter
		if label, ok := params["label"]; ok {
			if l, ok := label.(string); ok {
				return fmt.Sprintf("label_uppercase(%s, %s)", baseQuery, l), nil
			}
		}
		return "", fmt.Errorf("label_uppercase requires a 'label' parameter")
	case "label_match":
		// label_match needs label and regex parameters
		label, hasLabel := params["label"]
		regex, hasRegex := params["regex"]
		if hasLabel && hasRegex {
			if l, ok := label.(string); ok {
				if reg, ok := regex.(string); ok {
					return fmt.Sprintf("label_match(%s, %s, %s)", baseQuery, l, reg), nil
				}
			}
		}
		return "", fmt.Errorf("label_match requires 'label' and 'regex' parameters")
	case "label_mismatch":
		// label_mismatch needs label and regex parameters
		label, hasLabel := params["label"]
		regex, hasRegex := params["regex"]
		if hasLabel && hasRegex {
			if l, ok := label.(string); ok {
				if reg, ok := regex.(string); ok {
					return fmt.Sprintf("label_mismatch(%s, %s, %s)", baseQuery, l, reg), nil
				}
			}
		}
		return "", fmt.Errorf("label_mismatch requires 'label' and 'regex' parameters")
	case "labels_equal":
		// labels_equal needs label and value parameters
		label, hasLabel := params["label"]
		value, hasValue := params["value"]
		if hasLabel && hasValue {
			if l, ok := label.(string); ok {
				if v, ok := value.(string); ok {
					return fmt.Sprintf("labels_equal(%s, %s, %s)", baseQuery, l, v), nil
				}
			}
		}
		return "", fmt.Errorf("labels_equal requires 'label' and 'value' parameters")
	case "sort_by_label":
		// sort_by_label can optionally take label names
		if labels, ok := params["labels"]; ok {
			if labelList, ok := labels.([]string); ok && len(labelList) > 0 {
				labelStr := strings.Join(labelList, ",")
				return fmt.Sprintf("sort_by_label(%s, %s)", baseQuery, labelStr), nil
			}
		}
		return fmt.Sprintf("sort_by_label(%s)", baseQuery), nil
	case "sort_by_label_desc":
		// sort_by_label_desc can optionally take label names
		if labels, ok := params["labels"]; ok {
			if labelList, ok := labels.([]string); ok && len(labelList) > 0 {
				labelStr := strings.Join(labelList, ",")
				return fmt.Sprintf("sort_by_label_desc(%s, %s)", baseQuery, labelStr), nil
			}
		}
		return fmt.Sprintf("sort_by_label_desc(%s)", baseQuery), nil
	case "label_graphite_group":
		// label_graphite_group can optionally take a prefix parameter
		if prefix, ok := params["prefix"]; ok {
			if p, ok := prefix.(string); ok {
				return fmt.Sprintf("label_graphite_group(%s, %s)", baseQuery, p), nil
			}
		}
		return fmt.Sprintf("label_graphite_group(%s)", baseQuery), nil
	case "drop_common_labels":
		return fmt.Sprintf("drop_common_labels(%s)", baseQuery), nil

	// Aggregate functions
	case "sum":
		return fmt.Sprintf("sum(%s)", baseQuery), nil
	case "avg":
		return fmt.Sprintf("avg(%s)", baseQuery), nil
	case "min":
		return fmt.Sprintf("min(%s)", baseQuery), nil
	case "max":
		return fmt.Sprintf("max(%s)", baseQuery), nil
	case "count":
		return fmt.Sprintf("count(%s)", baseQuery), nil
	case "stddev":
		return fmt.Sprintf("stddev(%s)", baseQuery), nil
	case "stdvar":
		return fmt.Sprintf("stdvar(%s)", baseQuery), nil
	case "median":
		return fmt.Sprintf("median(%s)", baseQuery), nil
	case "mode":
		return fmt.Sprintf("mode(%s)", baseQuery), nil
	case "topk":
		// topk needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("topk(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("topk requires a 'k' parameter")
	case "bottomk":
		// bottomk needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("bottomk(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("bottomk requires a 'k' parameter")
	case "topk_avg":
		// topk_avg needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("topk_avg(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("topk_avg requires a 'k' parameter")
	case "topk_max":
		// topk_max needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("topk_max(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("topk_max requires a 'k' parameter")
	case "topk_min":
		// topk_min needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("topk_min(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("topk_min requires a 'k' parameter")
	case "bottomk_avg":
		// bottomk_avg needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("bottomk_avg(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("bottomk_avg requires a 'k' parameter")
	case "bottomk_max":
		// bottomk_max needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("bottomk_max(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("bottomk_max requires a 'k' parameter")
	case "bottomk_min":
		// bottomk_min needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("bottomk_min(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("bottomk_min requires a 'k' parameter")
	case "quantile":
		// quantile needs a quantile parameter
		if quantile, ok := params["quantile"]; ok {
			if q, ok := quantile.(float64); ok {
				return fmt.Sprintf("quantile(%.2f, %s)", q, baseQuery), nil
			}
		}
		return "", fmt.Errorf("quantile requires a 'quantile' parameter")
	case "quantiles":
		// quantiles needs a quantiles parameter (array)
		if quantiles, ok := params["quantiles"]; ok {
			if qList, ok := quantiles.([]interface{}); ok && len(qList) > 0 {
				// Convert quantiles to string format
				var quantileStrs []string
				for _, q := range qList {
					if qVal, ok := q.(float64); ok {
						quantileStrs = append(quantileStrs, fmt.Sprintf("%.2f", qVal))
					}
				}
				quantilesStr := strings.Join(quantileStrs, ",")
				return fmt.Sprintf("quantiles(%s, %s)", quantilesStr, baseQuery), nil
			}
		}
		return "", fmt.Errorf("quantiles requires a 'quantiles' parameter with quantile values")
	case "mad":
		return fmt.Sprintf("mad(%s)", baseQuery), nil
	case "geomean":
		return fmt.Sprintf("geomean(%s)", baseQuery), nil
	case "distinct":
		return fmt.Sprintf("distinct(%s)", baseQuery), nil
	case "histogram":
		return fmt.Sprintf("histogram(%s)", baseQuery), nil
	case "share":
		return fmt.Sprintf("share(%s)", baseQuery), nil
	case "outliers_iqr":
		return fmt.Sprintf("outliers_iqr(%s)", baseQuery), nil
	case "outliers_mad":
		return fmt.Sprintf("outliers_mad(%s)", baseQuery), nil
	case "outliersk":
		// outliersk needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("outliersk(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("outliersk requires a 'k' parameter")
	case "any":
		return fmt.Sprintf("any(%s)", baseQuery), nil
	case "group":
		return fmt.Sprintf("group(%s)", baseQuery), nil
	case "limitk":
		// limitk needs a k parameter
		if k, ok := params["k"]; ok {
			if kVal, ok := k.(float64); ok {
				return fmt.Sprintf("limitk(%.0f, %s)", kVal, baseQuery), nil
			}
		}
		return "", fmt.Errorf("limitk requires a 'k' parameter")
	case "count_values":
		return fmt.Sprintf("count_values(%s)", baseQuery), nil
	case "sum2":
		return fmt.Sprintf("sum2(%s)", baseQuery), nil
	case "zscore":
		return fmt.Sprintf("zscore(%s)", baseQuery), nil

	default:
		return "", fmt.Errorf("unsupported function: %s", functionName)
	}
}
