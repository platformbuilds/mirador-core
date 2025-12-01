package models

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CorrelationQuerySyntax defines the grammar for correlation queries
// Supports patterns like:
// - logs:error AND metrics:high_latency
// - logs:error WITHIN 5m OF metrics:response_time > 1000
// - logs:service:checkout AND traces:service:checkout
// - (logs:error OR logs:warn) WITHIN 10m OF metrics:cpu_usage > 80

// CorrelationOperator represents correlation operators
type CorrelationOperator string

const (
	CorrelationOpAND    CorrelationOperator = "AND"
	CorrelationOpOR     CorrelationOperator = "OR"
	CorrelationOpWITHIN CorrelationOperator = "WITHIN"
	CorrelationOpOF     CorrelationOperator = "OF"
)

// CorrelationQuery represents a parsed correlation query
type CorrelationQuery struct {
	ID          string                  `json:"id"`
	RawQuery    string                  `json:"raw_query"`
	Expressions []CorrelationExpression `json:"expressions"`
	TimeWindow  *time.Duration          `json:"time_window,omitempty"`
	Operator    CorrelationOperator     `json:"operator"`
}

// CorrelationExpression represents a single query expression in a correlation
type CorrelationExpression struct {
	Engine     QueryType      `json:"engine"`
	Query      string         `json:"query"`
	Condition  string         `json:"condition,omitempty"` // e.g., "> 1000", "== 'error'"
	TimeWindow *time.Duration `json:"time_window,omitempty"`
	LabelKey   string         `json:"label_key,omitempty"` // for label-based correlation
	LabelValue string         `json:"label_value,omitempty"`
}

// CorrelationQueryParser parses correlation query syntax
type CorrelationQueryParser struct{}

// NewCorrelationQueryParser creates a new correlation query parser
func NewCorrelationQueryParser() *CorrelationQueryParser {
	return &CorrelationQueryParser{}
}

// Parse parses a correlation query string into a CorrelationQuery
func (p *CorrelationQueryParser) Parse(query string) (*CorrelationQuery, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty correlation query")
	}

	corrQuery := &CorrelationQuery{
		RawQuery: query,
	}

	// Check for WITHIN operator (time-window correlation)
	if withinIndex := strings.Index(strings.ToUpper(query), " WITHIN "); withinIndex != -1 {
		beforeWithin := query[:withinIndex]
		afterWithin := query[withinIndex+8:] // Skip " WITHIN "

		// Parse time window
		timeWindow, remainder, err := p.parseTimeWindow(afterWithin)
		if err != nil {
			return nil, fmt.Errorf("invalid time window: %w", err)
		}
		corrQuery.TimeWindow = &timeWindow

		// Parse the "OF" part
		if !strings.HasPrefix(strings.ToUpper(remainder), "OF ") {
			return nil, fmt.Errorf("expected 'OF' after time window")
		}
		afterOf := strings.TrimSpace(remainder[3:])

		// Parse expressions
		expressions, operator, err := p.parseExpressions(beforeWithin)
		if err != nil {
			return nil, err
		}
		corrQuery.Expressions = expressions
		corrQuery.Operator = operator

		// Parse the reference expression after OF
		refExpr, err := p.parseSingleExpression(afterOf)
		if err != nil {
			return nil, err
		}
		corrQuery.Expressions = append(corrQuery.Expressions, *refExpr)

	} else {
		// Simple correlation without time window
		expressions, operator, err := p.parseExpressions(query)
		if err != nil {
			return nil, err
		}
		corrQuery.Expressions = expressions
		corrQuery.Operator = operator
	}

	return corrQuery, nil
}

// parseExpressions parses multiple expressions connected by AND/OR
func (p *CorrelationQueryParser) parseExpressions(query string) ([]CorrelationExpression, CorrelationOperator, error) {
	var expressions []CorrelationExpression
	var operator = CorrelationOpAND

	// Check for OR operator first (higher precedence)
	if orIndex := strings.Index(strings.ToUpper(query), " OR "); orIndex != -1 {
		operator = CorrelationOpOR
		parts := strings.Split(query, " OR ")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			expr, err := p.parseSingleExpression(part)
			if err != nil {
				return nil, operator, err
			}
			expressions = append(expressions, *expr)
		}
	} else if andIndex := strings.Index(strings.ToUpper(query), " AND "); andIndex != -1 {
		operator = CorrelationOpAND
		parts := strings.Split(query, " AND ")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			expr, err := p.parseSingleExpression(part)
			if err != nil {
				return nil, operator, err
			}
			expressions = append(expressions, *expr)
		}
	} else {
		// Single expression
		expr, err := p.parseSingleExpression(query)
		if err != nil {
			return nil, operator, err
		}
		expressions = append(expressions, *expr)
	}

	if len(expressions) == 0 {
		return nil, operator, fmt.Errorf("no valid expressions found")
	}

	return expressions, operator, nil
}

// parseSingleExpression parses a single engine-specific expression
func (p *CorrelationQueryParser) parseSingleExpression(expr string) (*CorrelationExpression, error) {
	expr = strings.TrimSpace(expr)

	// Check for engine prefix (logs:, metrics:, traces:)
	engine, query, found := strings.Cut(expr, ":")
	if !found {
		return nil, fmt.Errorf("missing engine prefix in expression: %s", expr)
	}

	var queryType QueryType
	switch strings.ToLower(engine) {
	case "logs":
		queryType = QueryTypeLogs
	case "metrics":
		queryType = QueryTypeMetrics
	case "traces":
		queryType = QueryTypeTraces
	default:
		return nil, fmt.Errorf("unknown engine: %s", engine)
	}

	// Check for condition (e.g., > 1000, == 'error')
	var condition string
	if condIndex := strings.Index(query, " > "); condIndex != -1 {
		condition = " > " + strings.TrimSpace(query[condIndex+3:])
		query = strings.TrimSpace(query[:condIndex])
	} else if condIndex := strings.Index(query, " < "); condIndex != -1 {
		condition = " < " + strings.TrimSpace(query[condIndex+3:])
		query = strings.TrimSpace(query[:condIndex])
	} else if condIndex := strings.Index(query, " == "); condIndex != -1 {
		condition = " == " + strings.TrimSpace(query[condIndex+4:])
		query = strings.TrimSpace(query[:condIndex])
	} else if condIndex := strings.Index(query, " != "); condIndex != -1 {
		condition = " != " + strings.TrimSpace(query[condIndex+4:])
		query = strings.TrimSpace(query[:condIndex])
	}

	return &CorrelationExpression{
		Engine:    queryType,
		Query:     query,
		Condition: condition,
	}, nil
}

// parseTimeWindow parses time window expressions like "5m", "1h", "30s"
func (p *CorrelationQueryParser) parseTimeWindow(input string) (time.Duration, string, error) {
	input = strings.TrimSpace(input)

	// Find the time unit
	re := regexp.MustCompile(`^(\d+)([smhd])`)
	matches := re.FindStringSubmatch(input)
	if len(matches) != 3 {
		return 0, input, fmt.Errorf("invalid time window format: %s", input)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, input, fmt.Errorf("invalid time value: %s", matches[1])
	}

	var duration time.Duration
	switch matches[2] {
	case "s":
		duration = time.Duration(value) * time.Second
	case "m":
		duration = time.Duration(value) * time.Minute
	case "h":
		duration = time.Duration(value) * time.Hour
	case "d":
		duration = time.Duration(value) * 24 * time.Hour
	default:
		return 0, input, fmt.Errorf("unknown time unit: %s", matches[2])
	}

	// Return duration and remainder
	remainder := strings.TrimSpace(input[len(matches[0]):])
	return duration, remainder, nil
}

// Validate validates a correlation query
func (cq *CorrelationQuery) Validate() error {
	if len(cq.Expressions) == 0 {
		return fmt.Errorf("correlation query must have at least one expression")
	}

	// Check for time-window correlation requirements
	if cq.TimeWindow != nil {
		if len(cq.Expressions) != 2 {
			return fmt.Errorf("time-window correlation requires exactly 2 expressions")
		}
		if cq.Operator != CorrelationOpAND {
			return fmt.Errorf("time-window correlation only supports AND operator")
		}
	}

	// Validate each expression
	for i, expr := range cq.Expressions {
		if expr.Engine == "" {
			return fmt.Errorf("expression %d: missing engine", i+1)
		}
		if expr.Query == "" {
			return fmt.Errorf("expression %d: missing query", i+1)
		}
	}

	return nil
}

// String returns a string representation of the correlation query
func (cq *CorrelationQuery) String() string {
	var parts []string
	for _, expr := range cq.Expressions {
		part := fmt.Sprintf("%s:%s", expr.Engine, expr.Query)
		if expr.Condition != "" {
			part += expr.Condition
		}
		parts = append(parts, part)
	}

	result := strings.Join(parts, fmt.Sprintf(" %s ", cq.Operator))

	if cq.TimeWindow != nil {
		result = fmt.Sprintf("(%s) WITHIN %s OF %s", result, formatDuration(*cq.TimeWindow), parts[len(parts)-1])
	}

	return result
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d.Hours() >= 24 {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	} else if d.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else if d.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

// CorrelationQueryExamples provides example correlation queries
var CorrelationQueryExamples = []string{
	"logs:error AND metrics:high_latency",
	"logs:service:checkout AND traces:service:checkout",
	"(logs:error OR logs:warn) WITHIN 5m OF metrics:cpu_usage > 80",
	"logs:exception WITHIN 10m OF traces:status:error",
	"metrics:http_requests > 1000 AND logs:error",
}

// FailureComponent represents a component that can fail in the financial transaction system
type FailureComponent string

const (
	FailureComponentAPIGateway FailureComponent = "api-gateway"
	FailureComponentTPS        FailureComponent = "tps"
	FailureComponentKeyDB      FailureComponent = "keydb"
	FailureComponentKafka      FailureComponent = "kafka"
	FailureComponentCassandra  FailureComponent = "cassandra"
)

// FailureIncident represents a correlated failure incident
type FailureIncident struct {
	IncidentID             string           `json:"incident_id"`
	FailureID              string           `json:"failure_id"`   // Human-readable identifier (e.g., "kafka-failure-1733022000")
	FailureUUID            string           `json:"failure_uuid"` // Unique UUID v5 identifier
	TimeRange              TimeRange        `json:"time_range"`
	PrimaryComponent       FailureComponent `json:"primary_component"`
	AffectedTransactionIDs []string         `json:"affected_transaction_ids"`
	ServicesInvolved       []string         `json:"services_involved"`
	FailureMode            string           `json:"failure_mode,omitempty"`
	FailureReason          string           `json:"failure_reason,omitempty"`
	Signals                []FailureSignal  `json:"signals"`
	AnomalyScore           float64          `json:"anomaly_score,omitempty"`
	Severity               string           `json:"severity"` // "low", "medium", "high", "critical"
	Confidence             float64          `json:"confidence"`
}

// FailureSignal represents a signal (log, metric, trace) that contributed to the failure detection
type FailureSignal struct {
	Type         string                 `json:"type"` // "log", "metric", "trace"
	Engine       QueryType              `json:"engine"`
	Timestamp    time.Time              `json:"timestamp"`
	Data         map[string]interface{} `json:"data"`
	AnomalyScore *float64               `json:"anomaly_score,omitempty"`
}

// FailureCorrelationResult contains the results of failure correlation
type FailureCorrelationResult struct {
	Incidents []FailureIncident `json:"incidents"`
	Summary   FailureSummary    `json:"summary"`
}

// ServiceComponentSummary provides a summary for a specific service+component combination
type ServiceComponentSummary struct {
	Service              string    `json:"service"`
	Component            string    `json:"component"`
	FailureCount         int       `json:"failure_count"`
	AffectedTransactions int       `json:"affected_transactions"`
	AverageAnomalyScore  float64   `json:"average_anomaly_score"`
	AverageConfidence    float64   `json:"average_confidence"`
	ErrorSpansCount      int       `json:"error_spans_count"`
	ErrorMetricsCount    int       `json:"error_metrics_count"`
	LastFailureTimestamp time.Time `json:"last_failure_timestamp"`
	FailureID            string    `json:"failure_id"`   // User-readable identifier
	FailureUUID          string    `json:"failure_uuid"` // UUID v5 for Weaviate storage
}

// MetricSummaryItem represents a single metric in the error metrics summary
type MetricSummaryItem struct {
	MetricName    string                 `json:"metric_name"`
	Count         int                    `json:"count"`
	Labels        map[string]interface{} `json:"labels"`
	AverageValue  float64                `json:"average_value"`
	LastValue     float64                `json:"last_value"`
	LastTimestamp time.Time              `json:"last_timestamp"`
}

// MetricsErrorSummary summarizes all error metrics (status_code=STATUS_CODE_ERROR)
type MetricsErrorSummary struct {
	TotalErrorMetrics    int                 `json:"total_error_metrics"`
	TotalAnomalyMetrics  int                 `json:"total_anomaly_metrics"`
	ErrorMetricsByName   []MetricSummaryItem `json:"error_metrics_by_name"`
	AnomalyMetricsByName []MetricSummaryItem `json:"anomaly_metrics_by_name"`
}

// FailureSummary provides summary statistics for failure incidents
type FailureSummary struct {
	TotalIncidents            int                       `json:"total_incidents"`
	TimeRange                 TimeRange                 `json:"time_range"`
	ComponentsAffected        map[FailureComponent]int  `json:"components_affected"`
	ServicesInvolved          []string                  `json:"services_involved"`
	FailureModes              map[string]int            `json:"failure_modes"`
	AverageConfidence         float64                   `json:"average_confidence"`
	AnomalyDetected           bool                      `json:"anomaly_detected"`
	ServiceComponentSummaries []ServiceComponentSummary `json:"service_component_summaries"`
	MetricsErrorSummary       *MetricsErrorSummary      `json:"metrics_error_summary,omitempty"`
}

// FailureDetectionRequest represents a request to detect component failures
type FailureDetectionRequest struct {
	TimeRange  TimeRange          `json:"time_range"`
	Components []FailureComponent `json:"components,omitempty"` // Optional: filter by specific components
	Services   []string           `json:"services,omitempty"`   // Optional: list of services to target for detection
}

// TransactionFailureCorrelationRequest represents a request to correlate failures for specific transactions
type TransactionFailureCorrelationRequest struct {
	TransactionIDs []string  `json:"transaction_ids"`
	TimeRange      TimeRange `json:"time_range"`
}
