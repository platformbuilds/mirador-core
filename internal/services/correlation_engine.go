package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// CorrelationEngine handles correlation queries across multiple engines
type CorrelationEngine interface {
	// ExecuteCorrelation executes a correlation query
	ExecuteCorrelation(ctx context.Context, query *models.CorrelationQuery) (*models.UnifiedCorrelationResult, error)

	// ValidateCorrelationQuery validates a correlation query
	ValidateCorrelationQuery(query *models.CorrelationQuery) error

	// GetCorrelationExamples returns example correlation queries
	GetCorrelationExamples() []string
}

// MetricsService interface for metrics operations
type MetricsService interface {
	ExecuteQuery(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error)
}

// LogsService interface for logs operations
type LogsService interface {
	ExecuteQuery(ctx context.Context, req *models.LogsQLQueryRequest) (*models.LogsQLQueryResult, error)
}

// TracesService interface for traces operations
type TracesService interface {
	GetOperations(ctx context.Context, service, tenantID string) ([]string, error)
}

// CorrelationEngineImpl implements the CorrelationEngine interface
type CorrelationEngineImpl struct {
	metricsService MetricsService
	logsService    LogsService
	tracesService  TracesService
	cache          cache.ValkeyCluster
	logger         logger.Logger
	parser         *models.CorrelationQueryParser
	resultMerger   *CorrelationResultMerger
}

// NewCorrelationEngine creates a new correlation engine
func NewCorrelationEngine(
	metricsSvc MetricsService,
	logsSvc LogsService,
	tracesSvc TracesService,
	cache cache.ValkeyCluster,
	logger logger.Logger,
) CorrelationEngine {
	return &CorrelationEngineImpl{
		metricsService: metricsSvc,
		logsService:    logsSvc,
		tracesService:  tracesSvc,
		cache:          cache,
		logger:         logger,
		parser:         models.NewCorrelationQueryParser(),
		resultMerger:   NewCorrelationResultMerger(logger),
	}
}

// ExecuteCorrelation executes a correlation query across multiple engines
func (ce *CorrelationEngineImpl) ExecuteCorrelation(ctx context.Context, query *models.CorrelationQuery) (*models.UnifiedCorrelationResult, error) {
	start := time.Now()

	// Validate the query
	if err := ce.ValidateCorrelationQuery(query); err != nil {
		return nil, fmt.Errorf("invalid correlation query: %w", err)
	}

	ce.logger.Info("Executing correlation query",
		"query_id", query.ID,
		"raw_query", query.RawQuery,
		"expressions", len(query.Expressions))

	// Execute expressions in parallel
	results, err := ce.executeExpressionsParallel(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute expressions: %w", err)
	}

	// Correlate results
	correlations := ce.correlateResults(query, results)

	// Merge and deduplicate correlations
	correlations = ce.resultMerger.MergeResults(correlations)

	// Create summary
	summary := ce.createCorrelationSummary(correlations, time.Since(start))

	result := &models.UnifiedCorrelationResult{
		Correlations: correlations,
		Summary:      summary,
	}

	ce.logger.Info("Correlation query completed",
		"query_id", query.ID,
		"correlations_found", len(correlations),
		"execution_time_ms", time.Since(start).Milliseconds())

	return result, nil
}

// executeExpressionsParallel executes all expressions in the correlation query in parallel
func (ce *CorrelationEngineImpl) executeExpressionsParallel(ctx context.Context, query *models.CorrelationQuery) (map[models.QueryType]*models.UnifiedResult, error) {
	results := make(map[models.QueryType]*models.UnifiedResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstError error
	var errorOnce sync.Once

	// Group expressions by engine to avoid duplicate queries
	engineExpressions := make(map[models.QueryType][]models.CorrelationExpression)
	for _, expr := range query.Expressions {
		engineExpressions[expr.Engine] = append(engineExpressions[expr.Engine], expr)
	}

	// Execute queries for each engine in parallel
	for engine, expressions := range engineExpressions {
		wg.Add(1)
		go func(engine models.QueryType, expressions []models.CorrelationExpression) {
			defer wg.Done()

			result, err := ce.executeEngineQuery(ctx, engine, expressions, query)
			if err != nil {
				errorOnce.Do(func() {
					firstError = err
				})
				ce.logger.Error("Failed to execute engine query",
					"engine", engine,
					"error", err)
				return
			}

			mu.Lock()
			results[engine] = result
			mu.Unlock()
		}(engine, expressions)
	}

	wg.Wait()

	if firstError != nil {
		return nil, firstError
	}

	return results, nil
}

// executeEngineQuery executes queries for a specific engine
func (ce *CorrelationEngineImpl) executeEngineQuery(
	ctx context.Context,
	engine models.QueryType,
	expressions []models.CorrelationExpression,
	corrQuery *models.CorrelationQuery,
) (*models.UnifiedResult, error) {
	switch engine {
	case models.QueryTypeMetrics:
		return ce.executeMetricsCorrelationQuery(ctx, expressions, corrQuery)
	case models.QueryTypeLogs:
		return ce.executeLogsCorrelationQuery(ctx, expressions, corrQuery)
	case models.QueryTypeTraces:
		return ce.executeTracesCorrelationQuery(ctx, expressions, corrQuery)
	default:
		return nil, fmt.Errorf("unsupported engine for correlation: %s", engine)
	}
}

// executeMetricsCorrelationQuery executes metrics queries for correlation
func (ce *CorrelationEngineImpl) executeMetricsCorrelationQuery(
	ctx context.Context,
	expressions []models.CorrelationExpression,
	corrQuery *models.CorrelationQuery,
) (*models.UnifiedResult, error) {
	if ce.metricsService == nil {
		return nil, fmt.Errorf("metrics service not configured")
	}

	// For now, execute the first expression (TODO: handle multiple expressions per engine)
	expr := expressions[0]

	// Create unified query for metrics
	query := &models.UnifiedQuery{
		ID:       corrQuery.ID + "_metrics",
		Type:     models.QueryTypeMetrics,
		Query:    expr.Query,
		TenantID: "", // TODO: extract from correlation query
	}

	if corrQuery.TimeWindow != nil {
		// For time-window correlations, extend the time range
		endTime := time.Now()
		startTime := endTime.Add(-*corrQuery.TimeWindow * 2) // Extend window for correlation
		query.StartTime = &startTime
		query.EndTime = &endTime
	}

	// Execute the metrics query
	metricsQuery := &models.MetricsQLQueryRequest{
		Query:    query.Query,
		TenantID: query.TenantID,
	}

	result, err := ce.metricsService.ExecuteQuery(ctx, metricsQuery)
	if err != nil {
		return nil, err
	}

	// Convert to UnifiedResult
	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeMetrics,
		Status:  result.Status,
		Data:    result.Data,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeMetrics: {
					Engine:      models.QueryTypeMetrics,
					Status:      result.Status,
					RecordCount: result.SeriesCount,
					DataSource:  "victoria-metrics",
				},
			},
			TotalRecords: result.SeriesCount,
			DataSources:  []string{"victoria-metrics"},
		},
	}, nil
}

// executeLogsCorrelationQuery executes logs queries for correlation
func (ce *CorrelationEngineImpl) executeLogsCorrelationQuery(
	ctx context.Context,
	expressions []models.CorrelationExpression,
	corrQuery *models.CorrelationQuery,
) (*models.UnifiedResult, error) {
	if ce.logsService == nil {
		return nil, fmt.Errorf("logs service not configured")
	}

	// For now, execute the first expression (TODO: handle multiple expressions per engine)
	expr := expressions[0]

	// Create unified query for logs
	query := &models.UnifiedQuery{
		ID:       corrQuery.ID + "_logs",
		Type:     models.QueryTypeLogs,
		Query:    expr.Query,
		TenantID: "", // TODO: extract from correlation query
	}

	if corrQuery.TimeWindow != nil {
		// For time-window correlations, extend the time range
		endTime := time.Now()
		startTime := endTime.Add(-*corrQuery.TimeWindow * 2)
		query.StartTime = &startTime
		query.EndTime = &endTime
	}

	startTime := int64(0)
	endTime := int64(0)
	if query.StartTime != nil {
		startTime = query.StartTime.UnixMilli()
	}
	if query.EndTime != nil {
		endTime = query.EndTime.UnixMilli()
	}

	logsQuery := &models.LogsQLQueryRequest{
		Query:    query.Query,
		Start:    startTime,
		End:      endTime,
		Limit:    1000, // TODO: make configurable
		TenantID: query.TenantID,
	}

	result, err := ce.logsService.ExecuteQuery(ctx, logsQuery)
	if err != nil {
		return nil, err
	}

	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeLogs,
		Status:  "success",
		Data:    result.Logs,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeLogs: {
					Engine:      models.QueryTypeLogs,
					Status:      "success",
					RecordCount: len(result.Logs),
					DataSource:  "victoria-logs",
				},
			},
			TotalRecords: len(result.Logs),
			DataSources:  []string{"victoria-logs"},
		},
	}, nil
}

// executeTracesCorrelationQuery executes traces queries for correlation
func (ce *CorrelationEngineImpl) executeTracesCorrelationQuery(
	ctx context.Context,
	expressions []models.CorrelationExpression,
	corrQuery *models.CorrelationQuery,
) (*models.UnifiedResult, error) {
	if ce.tracesService == nil {
		return nil, fmt.Errorf("traces service not configured")
	}

	// For now, execute the first expression (TODO: handle multiple expressions per engine)
	expr := expressions[0]

	// Create unified query for traces
	query := &models.UnifiedQuery{
		ID:       corrQuery.ID + "_traces",
		Type:     models.QueryTypeTraces,
		Query:    expr.Query,
		TenantID: "", // TODO: extract from correlation query
	}

	if corrQuery.TimeWindow != nil {
		// For time-window correlations, extend the time range
		endTime := time.Now()
		startTime := endTime.Add(-*corrQuery.TimeWindow * 2)
		query.StartTime = &startTime
		query.EndTime = &endTime
	}

	// For traces, use GetOperations as a basic implementation
	operations, err := ce.tracesService.GetOperations(ctx, expr.Query, query.TenantID)
	if err != nil {
		return nil, err
	}

	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeTraces,
		Status:  "success",
		Data:    operations,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeTraces: {
					Engine:      models.QueryTypeTraces,
					Status:      "success",
					RecordCount: len(operations),
					DataSource:  "victoria-traces",
				},
			},
			TotalRecords: len(operations),
			DataSources:  []string{"victoria-traces"},
		},
	}, nil
}

// correlateResults correlates results from different engines
func (ce *CorrelationEngineImpl) correlateResults(
	query *models.CorrelationQuery,
	results map[models.QueryType]*models.UnifiedResult,
) []models.Correlation {
	var correlations []models.Correlation

	if query.TimeWindow != nil {
		// Time-window correlation
		correlations = ce.correlateByTimeWindow(query, results)
	} else {
		// Label-based correlation
		correlations = ce.correlateByLabels(query, results)
	}

	return correlations
}

// correlateByTimeWindow correlates results within a time window
func (ce *CorrelationEngineImpl) correlateByTimeWindow(
	query *models.CorrelationQuery,
	results map[models.QueryType]*models.UnifiedResult,
) []models.Correlation {
	var correlations []models.Correlation

	// For time-window correlation, we expect exactly 2 expressions
	if len(query.Expressions) != 2 {
		ce.logger.Warn("Time-window correlation requires exactly 2 expressions",
			"expressions_count", len(query.Expressions))
		return correlations
	}

	expr1 := query.Expressions[0]
	expr2 := query.Expressions[1]

	result1, exists1 := results[expr1.Engine]
	result2, exists2 := results[expr2.Engine]

	if !exists1 || !exists2 {
		ce.logger.Warn("Missing results for time-window correlation",
			"expr1_engine", expr1.Engine, "has_result1", exists1,
			"expr2_engine", expr2.Engine, "has_result2", exists2)
		return correlations
	}

	// Extract timestamps and data points from results
	dataPoints1 := ce.extractDataPointsWithTimestamps(result1, expr1.Engine)
	dataPoints2 := ce.extractDataPointsWithTimestamps(result2, expr2.Engine)

	// Find correlations within the time window
	windowCorrelations := ce.findTimeWindowCorrelations(dataPoints1, dataPoints2, *query.TimeWindow)

	// Convert to correlation objects
	for _, wc := range windowCorrelations {
		correlation := models.Correlation{
			ID:         fmt.Sprintf("%s_time_window_%d", query.ID, len(correlations)+1),
			Timestamp:  wc.Timestamp,
			Engines:    make(map[models.QueryType]interface{}),
			Confidence: wc.Confidence,
			Metadata: map[string]interface{}{
				"time_window":      query.TimeWindow.String(),
				"correlation_type": "time_window",
			},
		}

		// Add correlated data points
		if wc.DataPoint1 != nil {
			correlation.Engines[expr1.Engine] = wc.DataPoint1
		}
		if wc.DataPoint2 != nil {
			correlation.Engines[expr2.Engine] = wc.DataPoint2
		}

		correlations = append(correlations, correlation)
	}

	return correlations
}

// correlateByLabels correlates results by shared labels
func (ce *CorrelationEngineImpl) correlateByLabels(
	query *models.CorrelationQuery,
	results map[models.QueryType]*models.UnifiedResult,
) []models.Correlation {
	var correlations []models.Correlation

	// Extract labels from all results
	resultLabels := make(map[models.QueryType][]dataLabels)
	for engine, result := range results {
		resultLabels[engine] = ce.extractLabelsFromResult(result, engine)
	}

	// Find correlations based on label matches
	for i, expr1 := range query.Expressions {
		for j, expr2 := range query.Expressions {
			if i >= j {
				continue // Avoid duplicate correlations
			}

			labels1 := resultLabels[expr1.Engine]
			labels2 := resultLabels[expr2.Engine]

			if len(labels1) == 0 || len(labels2) == 0 {
				continue
			}

			// Find matching labels between the two result sets
			labelMatches := ce.findLabelMatches(labels1, labels2)

			if len(labelMatches) > 0 {
				correlation := models.Correlation{
					ID:         fmt.Sprintf("%s_label_match_%d_%d", query.ID, i, j),
					Timestamp:  time.Now(), // TODO: Use actual timestamps from data
					Engines:    make(map[models.QueryType]interface{}),
					Confidence: ce.calculateLabelMatchConfidence(labelMatches),
					Metadata: map[string]interface{}{
						"correlation_type": "label_based",
						"label_matches":    labelMatches,
					},
				}

				// Add sample data from both engines (first match)
				if len(labels1) > 0 && labels1[0].Data != nil {
					correlation.Engines[expr1.Engine] = labels1[0].Data
				}
				if len(labels2) > 0 && labels2[0].Data != nil {
					correlation.Engines[expr2.Engine] = labels2[0].Data
				}

				correlations = append(correlations, correlation)
			}
		}
	}

	return correlations
} // createCorrelationSummary creates a summary of correlation results
func (ce *CorrelationEngineImpl) createCorrelationSummary(
	correlations []models.Correlation,
	executionTime time.Duration,
) models.CorrelationSummary {
	engines := make(map[models.QueryType]bool)
	totalConfidence := 0.0

	for _, corr := range correlations {
		for engine := range corr.Engines {
			engines[engine] = true
		}
		totalConfidence += corr.Confidence
	}

	var enginesInvolved []models.QueryType
	for engine := range engines {
		enginesInvolved = append(enginesInvolved, engine)
	}

	avgConfidence := 0.0
	if len(correlations) > 0 {
		avgConfidence = totalConfidence / float64(len(correlations))
	}

	return models.CorrelationSummary{
		TotalCorrelations: len(correlations),
		AverageConfidence: avgConfidence,
		TimeRange:         fmt.Sprintf("%v", executionTime),
		EnginesInvolved:   enginesInvolved,
	}
}

// ValidateCorrelationQuery validates a correlation query
func (ce *CorrelationEngineImpl) ValidateCorrelationQuery(query *models.CorrelationQuery) error {
	return query.Validate()
}

// GetCorrelationExamples returns example correlation queries
func (ce *CorrelationEngineImpl) GetCorrelationExamples() []string {
	return models.CorrelationQueryExamples
}

// timeWindowCorrelation represents a correlation found within a time window
type timeWindowCorrelation struct {
	Timestamp  time.Time
	DataPoint1 interface{}
	DataPoint2 interface{}
	Confidence float64
}

// extractDataPointsWithTimestamps extracts data points with their timestamps from unified results
func (ce *CorrelationEngineImpl) extractDataPointsWithTimestamps(result *models.UnifiedResult, engine models.QueryType) []timeWindowDataPoint {
	var dataPoints []timeWindowDataPoint

	switch engine {
	case models.QueryTypeMetrics:
		dataPoints = ce.extractMetricsDataPoints(result)
	case models.QueryTypeLogs:
		dataPoints = ce.extractLogsDataPoints(result)
	case models.QueryTypeTraces:
		dataPoints = ce.extractTracesDataPoints(result)
	default:
		ce.logger.Warn("Unsupported engine for timestamp extraction", "engine", engine)
	}

	return dataPoints
}

// timeWindowDataPoint represents a data point with timestamp
type timeWindowDataPoint struct {
	Timestamp time.Time
	Data      interface{}
}

// extractMetricsDataPoints extracts data points from metrics results
func (ce *CorrelationEngineImpl) extractMetricsDataPoints(result *models.UnifiedResult) []timeWindowDataPoint {
	var dataPoints []timeWindowDataPoint

	// Metrics data structure depends on VictoriaMetrics response format
	// This is a simplified implementation - in practice, we'd parse the actual metrics data
	if result.Data != nil {
		// For now, assume current time for metrics (TODO: parse actual timestamps from metrics data)
		dataPoints = append(dataPoints, timeWindowDataPoint{
			Timestamp: time.Now(),
			Data:      result.Data,
		})
	}

	return dataPoints
}

// extractLogsDataPoints extracts data points from logs results
func (ce *CorrelationEngineImpl) extractLogsDataPoints(result *models.UnifiedResult) []timeWindowDataPoint {
	var dataPoints []timeWindowDataPoint

	if logs, ok := result.Data.([]map[string]interface{}); ok {
		for _, log := range logs {
			// Try to extract timestamp from log entry
			timestamp := ce.extractTimestampFromLog(log)
			dataPoints = append(dataPoints, timeWindowDataPoint{
				Timestamp: timestamp,
				Data:      log,
			})
		}
	}

	return dataPoints
}

// extractTracesDataPoints extracts data points from traces results
func (ce *CorrelationEngineImpl) extractTracesDataPoints(result *models.UnifiedResult) []timeWindowDataPoint {
	var dataPoints []timeWindowDataPoint

	if traces, ok := result.Data.([]map[string]interface{}); ok {
		for _, trace := range traces {
			// Try to extract timestamp from trace
			timestamp := ce.extractTimestampFromTrace(trace)
			dataPoints = append(dataPoints, timeWindowDataPoint{
				Timestamp: timestamp,
				Data:      trace,
			})
		}
	}

	return dataPoints
}

// extractTimestampFromLog attempts to extract timestamp from a log entry
func (ce *CorrelationEngineImpl) extractTimestampFromLog(log map[string]interface{}) time.Time {
	// Try common timestamp fields
	timestampFields := []string{"timestamp", "@timestamp", "time", "ts"}

	for _, field := range timestampFields {
		if ts, exists := log[field]; exists {
			if t, err := ce.parseTimestamp(ts); err == nil {
				return t
			}
		}
	}

	// Default to current time if no timestamp found
	return time.Now()
}

// extractTimestampFromTrace attempts to extract timestamp from a trace
func (ce *CorrelationEngineImpl) extractTimestampFromTrace(trace map[string]interface{}) time.Time {
	// Try to extract from trace start time or spans
	if startTime, exists := trace["startTime"]; exists {
		if t, err := ce.parseTimestamp(startTime); err == nil {
			return t
		}
	}

	// Default to current time
	return time.Now()
}

// parseTimestamp attempts to parse various timestamp formats
func (ce *CorrelationEngineImpl) parseTimestamp(ts interface{}) (time.Time, error) {
	switch v := ts.(type) {
	case string:
		// Try RFC3339 first
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, nil
		}
		// Try Unix timestamp as string
		if t, err := time.Parse(time.UnixDate, v); err == nil {
			return t, nil
		}
	case int64:
		// Assume milliseconds since epoch
		return time.UnixMilli(v), nil
	case int:
		return time.UnixMilli(int64(v)), nil
	case float64:
		return time.UnixMilli(int64(v)), nil
	}

	return time.Time{}, fmt.Errorf("unsupported timestamp format: %T", ts)
}

// findTimeWindowCorrelations finds correlations between two sets of data points within a time window
func (ce *CorrelationEngineImpl) findTimeWindowCorrelations(
	dataPoints1, dataPoints2 []timeWindowDataPoint,
	timeWindow time.Duration,
) []timeWindowCorrelation {
	var correlations []timeWindowCorrelation

	// Sort data points by timestamp for efficient correlation
	// For simplicity, we'll do a nested loop (O(n*m) complexity)
	// In production, this should be optimized with sorting and binary search

	for _, dp1 := range dataPoints1 {
		for _, dp2 := range dataPoints2 {
			timeDiff := dp1.Timestamp.Sub(dp2.Timestamp)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}

			// Check if within time window
			if timeDiff <= timeWindow {
				// Calculate confidence based on time proximity
				confidence := ce.calculateTimeWindowConfidence(timeDiff, timeWindow)

				correlations = append(correlations, timeWindowCorrelation{
					Timestamp:  ce.calculateCorrelationTimestamp(dp1.Timestamp, dp2.Timestamp),
					DataPoint1: dp1.Data,
					DataPoint2: dp2.Data,
					Confidence: confidence,
				})
			}
		}
	}

	return correlations
}

// calculateTimeWindowConfidence calculates confidence based on time proximity
func (ce *CorrelationEngineImpl) calculateTimeWindowConfidence(timeDiff, timeWindow time.Duration) float64 {
	if timeWindow == 0 {
		return 1.0
	}

	// Higher confidence for closer timestamps
	proximityRatio := 1.0 - (timeDiff.Seconds() / timeWindow.Seconds())
	if proximityRatio < 0 {
		proximityRatio = 0
	}

	// Base confidence on proximity, with minimum threshold
	confidence := 0.5 + (proximityRatio * 0.4) // Range: 0.5 to 0.9
	if confidence > 0.95 {
		confidence = 0.95 // Cap at 0.95 to leave room for other factors
	}

	return confidence
}

// calculateCorrelationTimestamp calculates the representative timestamp for a correlation
func (ce *CorrelationEngineImpl) calculateCorrelationTimestamp(t1, t2 time.Time) time.Time {
	// Use the average of the two timestamps
	return t1.Add(t2.Sub(t1) / 2)
}

// dataLabels represents labels extracted from a data point
type dataLabels struct {
	Data   interface{}
	Labels map[string]string
}

// labelMatch represents a matching label between two data points
type labelMatch struct {
	Key    string
	Value  string
	Weight float64 // Importance weight for this label match
}

// extractLabelsFromResult extracts labels from a unified result
func (ce *CorrelationEngineImpl) extractLabelsFromResult(result *models.UnifiedResult, engine models.QueryType) []dataLabels {
	switch engine {
	case models.QueryTypeLogs:
		return ce.extractLabelsFromLogsResult(result)
	case models.QueryTypeTraces:
		return ce.extractLabelsFromTracesResult(result)
	case models.QueryTypeMetrics:
		return ce.extractLabelsFromMetricsResult(result)
	default:
		ce.logger.Warn("Unsupported engine for label extraction", "engine", engine)
		return nil
	}
}

// extractLabelsFromLogsResult extracts labels from logs data
func (ce *CorrelationEngineImpl) extractLabelsFromLogsResult(result *models.UnifiedResult) []dataLabels {
	var labels []dataLabels

	if logs, ok := result.Data.([]map[string]interface{}); ok {
		for _, log := range logs {
			dataLabels := dataLabels{
				Data:   log,
				Labels: make(map[string]string),
			}

			// Extract common label fields from logs
			labelFields := []string{"service", "pod", "namespace", "deployment", "container", "host", "level"}
			for _, field := range labelFields {
				if value, exists := log[field]; exists {
					if strValue, ok := value.(string); ok {
						dataLabels.Labels[field] = strValue
					}
				}
			}

			// Extract kubernetes labels if present
			if k8s, exists := log["kubernetes"].(map[string]interface{}); exists {
				if podName, ok := k8s["pod_name"].(string); ok {
					dataLabels.Labels["pod"] = podName
				}
				if namespace, ok := k8s["namespace_name"].(string); ok {
					dataLabels.Labels["namespace"] = namespace
				}
				if container, ok := k8s["container_name"].(string); ok {
					dataLabels.Labels["container"] = container
				}
			}

			labels = append(labels, dataLabels)
		}
	}

	return labels
}

// extractLabelsFromTracesResult extracts labels from traces data
func (ce *CorrelationEngineImpl) extractLabelsFromTracesResult(result *models.UnifiedResult) []dataLabels {
	var labels []dataLabels

	if traces, ok := result.Data.([]map[string]interface{}); ok {
		for _, trace := range traces {
			dataLabels := dataLabels{
				Data:   trace,
				Labels: make(map[string]string),
			}

			// Extract service and operation
			if service, exists := trace["serviceName"].(string); exists {
				dataLabels.Labels["service"] = service
			}
			if operation, exists := trace["operationName"].(string); exists {
				dataLabels.Labels["operation"] = operation
			}

			// Extract tags
			if tags, exists := trace["tags"].(map[string]interface{}); exists {
				for key, value := range tags {
					if strValue, ok := value.(string); ok {
						dataLabels.Labels[key] = strValue
					}
				}
			}

			labels = append(labels, dataLabels)
		}
	}

	return labels
}

// extractLabelsFromMetricsResult extracts labels from metrics data
func (ce *CorrelationEngineImpl) extractLabelsFromMetricsResult(result *models.UnifiedResult) []dataLabels {
	var labels []dataLabels

	// Metrics data structure is complex - this is a simplified implementation
	// In practice, we'd need to parse the Prometheus/VictoriaMetrics response format
	if result.Data != nil {
		dataLabels := dataLabels{
			Data:   result.Data,
			Labels: make(map[string]string),
		}

		// For now, we'll assume metrics have labels in a specific format
		// TODO: Implement proper metrics label extraction
		// This would typically involve parsing the metrics response and extracting
		// label names and values from the time series data

		labels = append(labels, dataLabels)
	}

	return labels
}

// findLabelMatches finds matching labels between two sets of data labels
func (ce *CorrelationEngineImpl) findLabelMatches(labels1, labels2 []dataLabels) []labelMatch {
	var matches []labelMatch

	// Define label weights (importance for correlation)
	labelWeights := map[string]float64{
		"service":    1.0,
		"pod":        0.9,
		"namespace":  0.8,
		"deployment": 0.8,
		"container":  0.7,
		"operation":  0.8,
		"host":       0.6,
		"level":      0.3, // Less important for correlation
	}

	// For each data point in first set, find matches in second set
	for _, dl1 := range labels1 {
		for _, dl2 := range labels2 {
			for key1, value1 := range dl1.Labels {
				if value2, exists := dl2.Labels[key1]; exists && value1 == value2 {
					weight := labelWeights[key1]
					if weight == 0 {
						weight = 0.5 // Default weight for unknown labels
					}

					matches = append(matches, labelMatch{
						Key:    key1,
						Value:  value1,
						Weight: weight,
					})
				}
			}
		}
	}

	return matches
}

// calculateLabelMatchConfidence calculates confidence based on label matches
func (ce *CorrelationEngineImpl) calculateLabelMatchConfidence(matches []labelMatch) float64 {
	if len(matches) == 0 {
		return 0.0
	}

	// Calculate weighted confidence
	totalWeight := 0.0
	matchedWeight := 0.0

	// Define all possible important labels for normalization
	allImportantLabels := []string{"service", "pod", "namespace", "deployment", "container", "operation"}
	for _, label := range allImportantLabels {
		if weight, exists := map[string]float64{
			"service":    1.0,
			"pod":        0.9,
			"namespace":  0.8,
			"deployment": 0.8,
			"container":  0.7,
			"operation":  0.8,
		}[label]; exists {
			totalWeight += weight
		}
	}

	// Sum weights of matched labels
	for _, match := range matches {
		matchedWeight += match.Weight
	}

	// Calculate confidence as ratio of matched weight to total possible weight
	if totalWeight > 0 {
		confidence := matchedWeight / totalWeight
		// Cap at 0.95 and ensure minimum 0.6 for any matches
		if confidence > 0.95 {
			confidence = 0.95
		}
		if confidence < 0.6 {
			confidence = 0.6
		}
		return confidence
	}

	return 0.5 // Default confidence
}

// CorrelationResultMerger handles merging and deduplicating correlation results
type CorrelationResultMerger struct {
	logger logger.Logger
}

// NewCorrelationResultMerger creates a new result merger
func NewCorrelationResultMerger(logger logger.Logger) *CorrelationResultMerger {
	return &CorrelationResultMerger{
		logger: logger,
	}
}

// MergeResults merges and deduplicates correlation results
func (crm *CorrelationResultMerger) MergeResults(correlations []models.Correlation) []models.Correlation {
	if len(correlations) == 0 {
		return correlations
	}

	// Group correlations by similar characteristics
	groups := crm.groupSimilarCorrelations(correlations)

	// Merge each group into a single correlation
	var merged []models.Correlation
	for _, group := range groups {
		merged = append(merged, crm.mergeCorrelationGroup(group))
	}

	crm.logger.Info("Merged correlation results",
		"original_count", len(correlations),
		"merged_count", len(merged))

	return merged
}

// groupSimilarCorrelations groups correlations that represent the same logical correlation
func (crm *CorrelationResultMerger) groupSimilarCorrelations(correlations []models.Correlation) [][]models.Correlation {
	var groups [][]models.Correlation

	for _, corr := range correlations {
		// Find existing group this correlation belongs to
		found := false
		for i, group := range groups {
			if crm.correlationsAreSimilar(corr, group[0]) {
				groups[i] = append(groups[i], corr)
				found = true
				break
			}
		}

		// Create new group if not found
		if !found {
			groups = append(groups, []models.Correlation{corr})
		}
	}

	return groups
}

// correlationsAreSimilar checks if two correlations represent the same logical event
func (crm *CorrelationResultMerger) correlationsAreSimilar(corr1, corr2 models.Correlation) bool {
	// Check if they involve the same engines
	if len(corr1.Engines) != len(corr2.Engines) {
		return false
	}

	for engine := range corr1.Engines {
		if _, exists := corr2.Engines[engine]; !exists {
			return false
		}
	}

	// Check if timestamps are close (within 1 minute for similarity)
	timeDiff := corr1.Timestamp.Sub(corr2.Timestamp)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > time.Minute {
		return false
	}

	// Check if confidence is similar
	confidenceDiff := corr1.Confidence - corr2.Confidence
	if confidenceDiff < 0 {
		confidenceDiff = -confidenceDiff
	}
	if confidenceDiff > 0.2 { // More than 20% difference
		return false
	}

	return true
}

// mergeCorrelationGroup merges a group of similar correlations into one
func (crm *CorrelationResultMerger) mergeCorrelationGroup(group []models.Correlation) models.Correlation {
	if len(group) == 0 {
		return models.Correlation{}
	}

	if len(group) == 1 {
		return group[0]
	}

	// Use the first correlation as base
	merged := group[0]

	// Merge timestamps (use average)
	totalTime := merged.Timestamp
	for i := 1; i < len(group); i++ {
		totalTime = totalTime.Add(group[i].Timestamp.Sub(merged.Timestamp))
	}
	merged.Timestamp = merged.Timestamp.Add(totalTime.Sub(merged.Timestamp) / time.Duration(len(group)))

	// Merge confidence (use average)
	totalConfidence := merged.Confidence
	for i := 1; i < len(group); i++ {
		totalConfidence += group[i].Confidence
	}
	merged.Confidence = totalConfidence / float64(len(group))

	// Merge data from all engines
	for i := 1; i < len(group); i++ {
		for engine, data := range group[i].Engines {
			if _, exists := merged.Engines[engine]; !exists {
				merged.Engines[engine] = data
			} else {
				// Merge data for same engine (combine into array if different)
				existing := merged.Engines[engine]
				if existingData, ok := existing.([]interface{}); ok {
					merged.Engines[engine] = append(existingData, data)
				} else {
					merged.Engines[engine] = []interface{}{existing, data}
				}
			}
		}
	}

	// Update metadata to indicate merging
	if merged.Metadata == nil {
		merged.Metadata = make(map[string]interface{})
	}
	merged.Metadata["merged_count"] = len(group)
	merged.Metadata["merge_timestamp"] = time.Now()

	return merged
}
