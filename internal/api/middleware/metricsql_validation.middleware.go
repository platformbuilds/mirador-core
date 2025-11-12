package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/utils"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// MetricsQLQueryValidationMiddleware validates MetricsQL function query requests
type MetricsQLQueryValidationMiddleware struct {
	validator *utils.QueryValidator
	logger    logger.Logger
}

// NewMetricsQLQueryValidationMiddleware creates a new MetricsQL query validation middleware
func NewMetricsQLQueryValidationMiddleware(logger logger.Logger) *MetricsQLQueryValidationMiddleware {
	return &MetricsQLQueryValidationMiddleware{
		validator: utils.NewQueryValidator(),
		logger:    logger,
	}
}

// ValidateFunctionQuery validates MetricsQL function query requests
func (m *MetricsQLQueryValidationMiddleware) ValidateFunctionQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.MetricsQLFunctionRequest

		// Bind the JSON request
		if err := c.ShouldBindJSON(&req); err != nil {
			m.logger.Error("Failed to bind MetricsQL function request", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request format",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate the base query
		if err := m.validator.ValidateMetricsQL(req.Query); err != nil {
			m.logger.Error("Invalid MetricsQL query", "query", req.Query, "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid MetricsQL query",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate function-specific parameters
		if err := m.validateFunctionParameters(req.Function, req.Params); err != nil {
			m.logger.Error("Invalid function parameters", "function", req.Function, "params", req.Params, "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid function parameters",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate time parameters if provided
		if req.Time != "" {
			if err := m.validateTimeParameter(req.Time); err != nil {
				m.logger.Error("Invalid time parameter", "time", req.Time, "error", err)
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid time parameter",
					"details": err.Error(),
				})
				c.Abort()
				return
			}
		}

		// Validate timeout parameter if provided
		if req.Timeout != "" {
			if err := m.validateTimeoutParameter(req.Timeout); err != nil {
				m.logger.Error("Invalid timeout parameter", "timeout", req.Timeout, "error", err)
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid timeout parameter",
					"details": err.Error(),
				})
				c.Abort()
				return
			}
		}

		// Store validated request in context for handlers
		c.Set("validated_request", &req)

		m.logger.Info("MetricsQL query validation passed", "function", req.Function, "query_length", len(req.Query))
		c.Next()
	}
}

// ValidateRangeFunctionQuery validates MetricsQL function range query requests
func (m *MetricsQLQueryValidationMiddleware) ValidateRangeFunctionQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.MetricsQLFunctionRangeRequest

		// Bind the JSON request
		if err := c.ShouldBindJSON(&req); err != nil {
			m.logger.Error("Failed to bind MetricsQL range function request", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request format",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate the base query
		if err := m.validator.ValidateMetricsQL(req.Query); err != nil {
			m.logger.Error("Invalid MetricsQL query", "query", req.Query, "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid MetricsQL query",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate function-specific parameters
		if err := m.validateFunctionParameters(req.Function, req.Params); err != nil {
			m.logger.Error("Invalid function parameters", "function", req.Function, "params", req.Params, "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid function parameters",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate time range parameters
		if err := m.validateTimeRangeParameters(req.Start, req.End, req.Step); err != nil {
			m.logger.Error("Invalid time range parameters", "start", req.Start, "end", req.End, "step", req.Step, "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid time range parameters",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Store validated request in context for handlers
		c.Set("validated_range_request", &req)

		m.logger.Info("MetricsQL range query validation passed", "function", req.Function, "query_length", len(req.Query))
		c.Next()
	}
}

// validateFunctionParameters validates function-specific parameters
func (m *MetricsQLQueryValidationMiddleware) validateFunctionParameters(functionName string, params map[string]interface{}) error {
	if params == nil {
		// Some functions don't require parameters, which is fine
		return nil
	}

	switch functionName {
	case "quantile_over_time", "histogram_quantile":
		// These functions require a quantile parameter
		if quantile, ok := params["quantile"]; !ok {
			return fmt.Errorf("function %s requires a 'quantile' parameter", functionName)
		} else {
			if q, ok := quantile.(float64); !ok || q < 0 || q > 1 {
				return fmt.Errorf("quantile parameter must be a number between 0 and 1")
			}
		}

	case "predict_linear":
		// Requires duration parameter
		if duration, ok := params["duration"]; !ok {
			return fmt.Errorf("predict_linear requires a 'duration' parameter")
		} else {
			if d, ok := duration.(string); !ok || d == "" {
				return fmt.Errorf("duration parameter must be a non-empty string")
			}
		}

	case "holt_winters":
		// Requires sf and tf parameters
		if sf, ok := params["sf"]; !ok {
			return fmt.Errorf("holt_winters requires an 'sf' parameter")
		} else {
			if s, ok := sf.(float64); !ok || s <= 0 {
				return fmt.Errorf("sf parameter must be a positive number")
			}
		}
		if tf, ok := params["tf"]; !ok {
			return fmt.Errorf("holt_winters requires a 'tf' parameter")
		} else {
			if t, ok := tf.(float64); !ok || t <= 0 {
				return fmt.Errorf("tf parameter must be a positive number")
			}
		}

	case "alias":
		// Requires label parameter
		if label, ok := params["label"]; !ok {
			return fmt.Errorf("alias requires a 'label' parameter")
		} else {
			if l, ok := label.(string); !ok || l == "" {
				return fmt.Errorf("label parameter must be a non-empty string")
			}
		}

	case "label_set":
		// Requires label and value parameters
		if label, ok := params["label"]; !ok {
			return fmt.Errorf("label_set requires a 'label' parameter")
		} else {
			if l, ok := label.(string); !ok || l == "" {
				return fmt.Errorf("label parameter must be a non-empty string")
			}
		}
		if value, ok := params["value"]; !ok {
			return fmt.Errorf("label_set requires a 'value' parameter")
		} else {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("value parameter must be a string")
			}
		}

	case "label_join":
		// Requires dst, separator, and src_labels parameters
		if dst, ok := params["dst"]; !ok {
			return fmt.Errorf("label_join requires a 'dst' parameter")
		} else {
			if d, ok := dst.(string); !ok || d == "" {
				return fmt.Errorf("dst parameter must be a non-empty string")
			}
		}
		if sep, ok := params["separator"]; !ok {
			return fmt.Errorf("label_join requires a 'separator' parameter")
		} else {
			if _, ok := sep.(string); !ok {
				return fmt.Errorf("separator parameter must be a string")
			}
		}
		if srcLabels, ok := params["src_labels"]; !ok {
			return fmt.Errorf("label_join requires a 'src_labels' parameter")
		} else {
			labels, ok := srcLabels.([]interface{})
			if !ok || len(labels) == 0 {
				return fmt.Errorf("src_labels parameter must be a non-empty array of strings")
			}
			for _, label := range labels {
				if l, ok := label.(string); !ok || l == "" {
					return fmt.Errorf("all src_labels must be non-empty strings")
				}
			}
		}

	case "label_replace":
		// Requires dst, replacement, src, and regex parameters
		requiredParams := []string{"dst", "replacement", "src", "regex"}
		for _, param := range requiredParams {
			if value, ok := params[param]; !ok {
				return fmt.Errorf("label_replace requires a '%s' parameter", param)
			} else {
				if s, ok := value.(string); !ok || s == "" {
					return fmt.Errorf("%s parameter must be a non-empty string", param)
				}
			}
		}

	case "label_match", "label_mismatch":
		// Require label and regex parameters
		requiredParams := []string{"label", "regex"}
		for _, param := range requiredParams {
			if value, ok := params[param]; !ok {
				return fmt.Errorf("%s requires a '%s' parameter", functionName, param)
			} else {
				if s, ok := value.(string); !ok || s == "" {
					return fmt.Errorf("%s parameter must be a non-empty string", param)
				}
			}
		}

	case "labels_equal":
		// Requires label and value parameters
		requiredParams := []string{"label", "value"}
		for _, param := range requiredParams {
			if value, ok := params[param]; !ok {
				return fmt.Errorf("labels_equal requires a '%s' parameter", param)
			} else {
				if s, ok := value.(string); !ok || s == "" {
					return fmt.Errorf("%s parameter must be a non-empty string", param)
				}
			}
		}

	case "label_lowercase", "label_uppercase":
		// Require label parameter
		if label, ok := params["label"]; !ok {
			return fmt.Errorf("%s requires a 'label' parameter", functionName)
		} else {
			if l, ok := label.(string); !ok || l == "" {
				return fmt.Errorf("label parameter must be a non-empty string")
			}
		}

	case "label_copy", "label_move":
		// Require src and dst parameters
		requiredParams := []string{"src", "dst"}
		for _, param := range requiredParams {
			if value, ok := params[param]; !ok {
				return fmt.Errorf("%s requires a '%s' parameter", functionName, param)
			} else {
				if s, ok := value.(string); !ok || s == "" {
					return fmt.Errorf("%s parameter must be a non-empty string", param)
				}
			}
		}

	case "label_del", "label_keep":
		// Require labels parameter
		if labels, ok := params["labels"]; !ok {
			return fmt.Errorf("%s requires a 'labels' parameter", functionName)
		} else {
			labelList, ok := labels.([]interface{})
			if !ok || len(labelList) == 0 {
				return fmt.Errorf("labels parameter must be a non-empty array of strings")
			}
			for _, label := range labelList {
				if l, ok := label.(string); !ok || l == "" {
					return fmt.Errorf("all labels must be non-empty strings")
				}
			}
		}

	case "label_map":
		// Requires label and mapping parameters
		if label, ok := params["label"]; !ok {
			return fmt.Errorf("label_map requires a 'label' parameter")
		} else {
			if l, ok := label.(string); !ok || l == "" {
				return fmt.Errorf("label parameter must be a non-empty string")
			}
		}
		if mapping, ok := params["mapping"]; !ok {
			return fmt.Errorf("label_map requires a 'mapping' parameter")
		} else {
			if m, ok := mapping.(map[string]interface{}); !ok || len(m) == 0 {
				return fmt.Errorf("mapping parameter must be a non-empty object")
			}
		}

	case "label_transform":
		// Requires label, regex, and replacement parameters
		requiredParams := []string{"label", "regex", "replacement"}
		for _, param := range requiredParams {
			if value, ok := params[param]; !ok {
				return fmt.Errorf("label_transform requires a '%s' parameter", param)
			} else {
				if s, ok := value.(string); !ok || s == "" {
					return fmt.Errorf("%s parameter must be a non-empty string", param)
				}
			}
		}
		// Optional separator parameter
		if sep, ok := params["separator"]; ok {
			if _, ok := sep.(string); !ok {
				return fmt.Errorf("separator parameter must be a string")
			}
		}

	case "label_graphite_group":
		// Optional prefix parameter
		if prefix, ok := params["prefix"]; ok {
			if _, ok := prefix.(string); !ok {
				return fmt.Errorf("prefix parameter must be a string")
			}
		}

	case "sort_by_label", "sort_by_label_desc":
		// Optional labels parameter
		if labels, ok := params["labels"]; ok {
			if _, ok := labels.([]interface{}); !ok {
				return fmt.Errorf("labels parameter must be an array of strings")
			}
			if labelList, ok := labels.([]interface{}); ok {
				for _, label := range labelList {
					if l, ok := label.(string); !ok || l == "" {
						return fmt.Errorf("all labels must be non-empty strings")
					}
				}
			}
		}

	// Aggregate function validations
	case "topk", "bottomk":
		// Require k parameter (number of top/bottom results)
		if k, ok := params["k"]; !ok {
			return fmt.Errorf("%s requires a 'k' parameter", functionName)
		} else {
			if kVal, ok := k.(float64); !ok || kVal <= 0 {
				return fmt.Errorf("k parameter must be a positive number")
			}
		}

	case "quantile":
		// Require quantile parameter
		if quantile, ok := params["quantile"]; !ok {
			return fmt.Errorf("quantile requires a 'quantile' parameter")
		} else {
			if q, ok := quantile.(float64); !ok || q < 0 || q > 1 {
				return fmt.Errorf("quantile parameter must be a number between 0 and 1")
			}
		}

	case "quantiles":
		// Require quantiles parameter (array of quantile values)
		if quantiles, ok := params["quantiles"]; !ok {
			return fmt.Errorf("quantiles requires a 'quantiles' parameter")
		} else {
			qList, ok := quantiles.([]interface{})
			if !ok || len(qList) == 0 {
				return fmt.Errorf("quantiles parameter must be a non-empty array of numbers")
			}
			for _, q := range qList {
				if qVal, ok := q.(float64); !ok || qVal < 0 || qVal > 1 {
					return fmt.Errorf("all quantiles must be numbers between 0 and 1")
				}
			}
		}

	case "limitk":
		// Require k parameter
		if k, ok := params["k"]; !ok {
			return fmt.Errorf("limitk requires a 'k' parameter")
		} else {
			if kVal, ok := k.(float64); !ok || kVal <= 0 {
				return fmt.Errorf("k parameter must be a positive number")
			}
		}

	case "outliersk":
		// Require k parameter
		if k, ok := params["k"]; !ok {
			return fmt.Errorf("outliersk requires a 'k' parameter")
		} else {
			if kVal, ok := k.(float64); !ok || kVal <= 0 {
				return fmt.Errorf("k parameter must be a positive number")
			}
		}
	}

	return nil
}

// validateTimeParameter validates time parameter format
func (m *MetricsQLQueryValidationMiddleware) validateTimeParameter(timeStr string) error {
	// Accept Unix timestamps (numeric) or RFC3339 format
	if timeStr == "" {
		return fmt.Errorf("time parameter cannot be empty")
	}
	// Basic validation - could be enhanced with more specific time format validation
	return nil
}

// validateTimeoutParameter validates timeout parameter format
func (m *MetricsQLQueryValidationMiddleware) validateTimeoutParameter(timeoutStr string) error {
	if timeoutStr == "" {
		return fmt.Errorf("timeout parameter cannot be empty")
	}
	// Should be a duration string like "30s", "5m", etc.
	if !strings.Contains(timeoutStr, "s") && !strings.Contains(timeoutStr, "m") && !strings.Contains(timeoutStr, "h") {
		return fmt.Errorf("timeout parameter must include time unit (s, m, h)")
	}
	return nil
}

// validateTimeRangeParameters validates time range parameters for range queries
func (m *MetricsQLQueryValidationMiddleware) validateTimeRangeParameters(start, end, step string) error {
	if start == "" || end == "" || step == "" {
		return fmt.Errorf("start, end, and step parameters are required for range queries")
	}
	// Basic validation - could be enhanced with more specific validation
	return nil
}
