package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// UQLOptimizer optimizes UQL queries for efficient execution
type UQLOptimizer interface {
	// Optimize applies optimization passes to a UQL query
	Optimize(uqlQuery *models.UQLQuery) (*models.UQLQuery, error)

	// GetOptimizationStats returns statistics about applied optimizations
	GetOptimizationStats() OptimizationStats

	// GenerateQueryPlan generates an execution plan for a UQL query
	GenerateQueryPlan(uqlQuery *models.UQLQuery) (*QueryPlan, error)

	// ExplainQuery returns a human-readable explanation of the query execution plan
	ExplainQuery(uqlQuery *models.UQLQuery) (string, error)
}

// OptimizationStats contains statistics about query optimizations
type OptimizationStats struct {
	QueryRewrites          int           `json:"query_rewrites"`
	PredicatePushdown      int           `json:"predicate_pushdown"`
	TimeWindowOpt          int           `json:"time_window_optimizations"`
	JoinOptimizations      int           `json:"join_optimizations"`
	IndexSelections        int           `json:"index_selections"`
	CostBasedOptimizations int           `json:"cost_based_optimizations"`
	QueryPlanCaching       int           `json:"query_plan_caching"`
	ExecutionTime          time.Duration `json:"optimization_time"`
}

// QueryPlan represents an execution plan for a UQL query
type QueryPlan struct {
	QueryID       string                `json:"query_id"`
	QueryType     models.UQLQueryType   `json:"query_type"`
	Steps         []QueryPlanStep       `json:"steps"`
	EstimatedCost float64               `json:"estimated_cost"`
	EstimatedRows int64                 `json:"estimated_rows"`
	DataSources   []QueryPlanDataSource `json:"data_sources"`
	Optimizations []string              `json:"optimizations_applied"`
	GeneratedAt   time.Time             `json:"generated_at"`
}

// QueryPlanStep represents a single step in the query execution plan
type QueryPlanStep struct {
	ID          int                    `json:"id"`
	Operation   string                 `json:"operation"`
	Description string                 `json:"description"`
	Cost        float64                `json:"cost"`
	Rows        int64                  `json:"rows"`
	DataSource  *QueryPlanDataSource   `json:"data_source,omitempty"`
	Children    []int                  `json:"children,omitempty"` // IDs of child steps
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// QueryPlanDataSource represents a data source in the query plan
type QueryPlanDataSource struct {
	Name                 string   `json:"name"`
	Type                 string   `json:"type"` // logs, metrics, traces, correlations
	Engine               string   `json:"engine"`
	Filters              []string `json:"filters,omitempty"`
	Indexes              []string `json:"indexes,omitempty"`
	EstimatedCardinality int64    `json:"estimated_cardinality"`
}

// UQLOptimizerImpl implements the UQLOptimizer interface
type UQLOptimizerImpl struct {
	logger      logger.Logger
	enabledOpts map[string]bool
	stats       OptimizationStats
}

// NewUQLOptimizer creates a new UQL optimizer
func NewUQLOptimizer(logger logger.Logger) UQLOptimizer {
	return &UQLOptimizerImpl{
		logger: logger,
		enabledOpts: map[string]bool{
			"predicate_pushdown":      true,
			"query_rewrite":           true,
			"time_window_opt":         true,
			"field_pruning":           true,
			"constant_folding":        true,
			"join_optimization":       true,
			"index_selection":         true,
			"cost_based_opt":          true,
			"query_plan_caching":      true,
			"subquery_optimization":   true,
			"materialized_view_check": true,
		},
		stats: OptimizationStats{},
	}
}

// Optimize applies optimization passes to a UQL query
func (o *UQLOptimizerImpl) Optimize(uqlQuery *models.UQLQuery) (*models.UQLQuery, error) {
	start := time.Now()

	// Create a copy of the query to avoid modifying the original
	optimized := o.deepCopyUQLQuery(uqlQuery)

	// Apply optimization passes
	if err := o.applyOptimizationPasses(optimized); err != nil {
		return nil, fmt.Errorf("optimization failed: %w", err)
	}

	o.stats.ExecutionTime = time.Since(start)

	o.logger.Info("UQL query optimized",
		"original_query", uqlQuery.RawQuery,
		"optimized_query", optimized.RawQuery,
		"optimization_time", o.stats.ExecutionTime)

	return optimized, nil
}

// GetOptimizationStats returns statistics about applied optimizations
func (o *UQLOptimizerImpl) GetOptimizationStats() OptimizationStats {
	return o.stats
}

// GenerateQueryPlan generates an execution plan for a UQL query
func (o *UQLOptimizerImpl) GenerateQueryPlan(uqlQuery *models.UQLQuery) (*QueryPlan, error) {
	plan := &QueryPlan{
		QueryID:       fmt.Sprintf("query_%d", time.Now().UnixNano()),
		QueryType:     uqlQuery.Type,
		Steps:         []QueryPlanStep{},
		DataSources:   []QueryPlanDataSource{},
		Optimizations: []string{},
		GeneratedAt:   time.Now(),
	}

	// Analyze the query structure and build execution steps
	switch uqlQuery.Type {
	case models.UQLQueryTypeSelect:
		if err := o.buildSelectPlan(uqlQuery, plan); err != nil {
			return nil, err
		}
	case models.UQLQueryTypeAggregation:
		if err := o.buildAggregationPlan(uqlQuery, plan); err != nil {
			return nil, err
		}
	case models.UQLQueryTypeCorrelation:
		if err := o.buildCorrelationPlan(uqlQuery, plan); err != nil {
			return nil, err
		}
	case models.UQLQueryTypeJoin:
		if err := o.buildJoinPlan(uqlQuery, plan); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported query type: %s", uqlQuery.Type)
	}

	// Estimate costs and cardinalities
	o.estimatePlanCosts(plan)

	return plan, nil
}

// ExplainQuery returns a human-readable explanation of the query execution plan
func (o *UQLOptimizerImpl) ExplainQuery(uqlQuery *models.UQLQuery) (string, error) {
	plan, err := o.GenerateQueryPlan(uqlQuery)
	if err != nil {
		return "", err
	}

	return o.formatQueryPlan(plan), nil
}

// applyOptimizationPasses applies all enabled optimization passes
func (o *UQLOptimizerImpl) applyOptimizationPasses(query *models.UQLQuery) error {
	passes := []struct {
		name string
		fn   func(*models.UQLQuery) error
	}{
		{"constant_folding", o.constantFolding},
		{"predicate_pushdown", o.predicatePushdown},
		{"query_rewrite", o.queryRewrite},
		{"time_window_opt", o.timeWindowOptimization},
		{"field_pruning", o.fieldPruning},
		{"join_optimization", o.joinOptimization},
		{"index_selection", o.indexSelection},
		{"cost_based_opt", o.costBasedOptimization},
		{"query_plan_caching", o.queryPlanCaching},
		{"subquery_optimization", o.subqueryOptimization},
		{"materialized_view_check", o.materializedViewCheck},
	}

	for _, pass := range passes {
		if o.enabledOpts[pass.name] {
			if err := pass.fn(query); err != nil {
				o.logger.Warn("Optimization pass failed", "pass", pass.name, "error", err)
				// Continue with other passes even if one fails
			}
		}
	}

	return nil
}

// constantFolding evaluates constant expressions and folds them
func (o *UQLOptimizerImpl) constantFolding(query *models.UQLQuery) error {
	// For now, this is a placeholder for constant folding optimizations
	// In a full implementation, this would evaluate expressions like:
	// - Mathematical operations on constants
	// - Boolean logic simplifications
	// - String concatenations

	o.logger.Debug("Applied constant folding optimization")
	return nil
}

// predicatePushdown pushes predicates down the query tree for better performance
func (o *UQLOptimizerImpl) predicatePushdown(query *models.UQLQuery) error {
	switch query.Type {
	case models.UQLQueryTypeSelect:
		return o.predicatePushdownSelect(query)
	case models.UQLQueryTypeCorrelation:
		return o.predicatePushdownCorrelation(query)
	case models.UQLQueryTypeAggregation:
		return o.predicatePushdownAggregation(query)
	}

	return nil
}

// predicatePushdownSelect optimizes SELECT queries
func (o *UQLOptimizerImpl) predicatePushdownSelect(query *models.UQLQuery) error {
	if query.Select == nil {
		return nil
	}

	// Push WHERE conditions that can be evaluated early
	if query.Select.Where != nil {
		// For metrics queries, push label selectors
		if query.Select.DataSource.Engine == models.EngineMetrics {
			o.optimizeMetricsPredicates(query.Select)
		}

		// For logs queries, push field filters
		if query.Select.DataSource.Engine == models.EngineLogs {
			o.optimizeLogsPredicates(query.Select)
		}
	}

	o.stats.PredicatePushdown++
	o.logger.Debug("Applied predicate pushdown to SELECT query")
	return nil
}

// predicatePushdownCorrelation optimizes correlation queries
func (o *UQLOptimizerImpl) predicatePushdownCorrelation(query *models.UQLQuery) error {
	if query.Correlation == nil {
		return nil
	}

	// Optimize time windows for better performance
	if query.Correlation.TimeWindow != nil {
		optimized := o.optimizeTimeWindow(*query.Correlation.TimeWindow)
		query.Correlation.TimeWindow = &optimized
		o.stats.TimeWindowOpt++
	}

	o.logger.Debug("Applied predicate pushdown to correlation query")
	return nil
}

// predicatePushdownAggregation optimizes aggregation queries
func (o *UQLOptimizerImpl) predicatePushdownAggregation(query *models.UQLQuery) error {
	if query.Aggregation == nil {
		return nil
	}

	// Push WHERE conditions before aggregation for better performance
	if query.Aggregation.Where != nil {
		// For metrics aggregations, optimize label selectors
		if query.Aggregation.DataSource.Engine == models.EngineMetrics {
			o.optimizeMetricsAggregationPredicates(query.Aggregation)
		}
	}

	o.logger.Debug("Applied predicate pushdown to aggregation query")
	return nil
}

// queryRewrite rewrites queries for better performance
func (o *UQLOptimizerImpl) queryRewrite(query *models.UQLQuery) error {
	switch query.Type {
	case models.UQLQueryTypeSelect:
		return o.rewriteSelectQuery(query)
	case models.UQLQueryTypeCorrelation:
		return o.rewriteCorrelationQuery(query)
	case models.UQLQueryTypeAggregation:
		return o.rewriteAggregationQuery(query)
	}

	return nil
}

// rewriteSelectQuery rewrites SELECT queries for optimization
func (o *UQLOptimizerImpl) rewriteSelectQuery(query *models.UQLQuery) error {
	if query.Select == nil {
		return nil
	}

	// Optimize field selection
	optimizedFields := o.optimizeFieldSelection(query.Select.Fields)
	query.Select.Fields = optimizedFields

	// Optimize ORDER BY for better indexing
	if len(query.OrderBy) > 0 {
		query.OrderBy = o.optimizeOrderBy(query.OrderBy)
	}

	o.stats.QueryRewrites++
	o.logger.Debug("Applied query rewrite to SELECT query")
	return nil
}

// rewriteCorrelationQuery rewrites correlation queries
func (o *UQLOptimizerImpl) rewriteCorrelationQuery(query *models.UQLQuery) error {
	if query.Correlation == nil {
		return nil
	}

	// Optimize correlation operators
	optimized := o.optimizeCorrelationOperators(query.Correlation)
	query.Correlation = optimized

	o.stats.QueryRewrites++
	o.logger.Debug("Applied query rewrite to correlation query")
	return nil
}

// rewriteAggregationQuery rewrites aggregation queries
func (o *UQLOptimizerImpl) rewriteAggregationQuery(query *models.UQLQuery) error {
	if query.Aggregation == nil {
		return nil
	}

	// Optimize aggregation functions
	optimizedFunc := o.optimizeAggregationFunction(query.Aggregation.Function)
	query.Aggregation.Function = optimizedFunc

	o.stats.QueryRewrites++
	o.logger.Debug("Applied query rewrite to aggregation query")
	return nil
}

// timeWindowOptimization optimizes time window specifications
func (o *UQLOptimizerImpl) timeWindowOptimization(query *models.UQLQuery) error {
	// Optimize time windows across the query
	if query.TimeWindow != nil {
		optimized := o.optimizeTimeWindow(*query.TimeWindow)
		query.TimeWindow = &optimized
		o.stats.TimeWindowOpt++
	}

	// Optimize time windows in sub-components
	switch query.Type {
	case models.UQLQueryTypeCorrelation:
		if query.Correlation != nil && query.Correlation.TimeWindow != nil {
			optimized := o.optimizeTimeWindow(*query.Correlation.TimeWindow)
			query.Correlation.TimeWindow = &optimized
			o.stats.TimeWindowOpt++
		}
	}

	o.logger.Debug("Applied time window optimization")
	return nil
}

// fieldPruning removes unnecessary fields from SELECT clauses
func (o *UQLOptimizerImpl) fieldPruning(query *models.UQLQuery) error {
	if query.Type != models.UQLQueryTypeSelect || query.Select == nil {
		return nil
	}

	// Identify which fields are actually used
	usedFields := o.identifyUsedFields(query)

	// Prune unused fields
	var prunedFields []models.UQLField
	for _, field := range query.Select.Fields {
		if o.isFieldUsed(field, usedFields) {
			prunedFields = append(prunedFields, field)
		}
	}

	if len(prunedFields) < len(query.Select.Fields) {
		query.Select.Fields = prunedFields
		o.logger.Debug("Applied field pruning", "original_fields", len(query.Select.Fields), "pruned_fields", len(prunedFields))
	}

	return nil
}

// joinOptimization optimizes join operations in queries
func (o *UQLOptimizerImpl) joinOptimization(query *models.UQLQuery) error {
	if query.Type != models.UQLQueryTypeJoin || query.Join == nil {
		return nil
	}

	// Optimize join order based on table sizes and selectivity
	optimizedJoin := o.optimizeJoinOrder(query.Join)
	query.Join = optimizedJoin

	// Choose optimal join algorithm
	o.selectJoinAlgorithm(query.Join)

	o.stats.JoinOptimizations++
	o.logger.Debug("Applied join optimization")
	return nil
}

// indexSelection selects optimal indexes for query execution
func (o *UQLOptimizerImpl) indexSelection(query *models.UQLQuery) error {
	// Analyze query predicates to select best indexes
	switch query.Type {
	case models.UQLQueryTypeSelect:
		if query.Select != nil {
			o.selectIndexesForSelect(query.Select)
		}
	case models.UQLQueryTypeAggregation:
		if query.Aggregation != nil {
			o.selectIndexesForAggregation(query.Aggregation)
		}
	}

	o.stats.IndexSelections++
	o.logger.Debug("Applied index selection optimization")
	return nil
}

// costBasedOptimization performs cost-based query optimization
func (o *UQLOptimizerImpl) costBasedOptimization(query *models.UQLQuery) error {
	// Estimate execution costs for different query plans
	// This is a simplified cost-based optimizer

	// For correlation queries, choose between parallel and sequential execution
	if query.Type == models.UQLQueryTypeCorrelation {
		o.optimizeCorrelationExecutionStrategy(query)
	}

	// For complex queries, consider different execution orders
	if o.isComplexQuery(query) {
		o.optimizeExecutionOrder(query)
	}

	o.stats.CostBasedOptimizations++
	o.logger.Debug("Applied cost-based optimization")
	return nil
}

// queryPlanCaching caches optimized query plans for reuse
func (o *UQLOptimizerImpl) queryPlanCaching(query *models.UQLQuery) error {
	// Generate a plan hash for caching
	planHash := o.generateQueryPlanHash(query)

	// Check if we have a cached plan
	if cachedPlan := o.getCachedPlan(planHash); cachedPlan != nil {
		// Apply cached optimizations
		o.applyCachedPlan(query, cachedPlan)
		o.stats.QueryPlanCaching++
		o.logger.Debug("Applied cached query plan")
		return nil
	}

	// Cache the current optimized plan
	o.cacheQueryPlan(planHash, query)

	o.logger.Debug("Cached new query plan")
	return nil
}

// subqueryOptimization optimizes subqueries for better performance
func (o *UQLOptimizerImpl) subqueryOptimization(query *models.UQLQuery) error {
	// Convert subqueries to joins where beneficial
	if query.Type == models.UQLQueryTypeSelect && query.Select != nil {
		o.convertSubqueriesToJoins(query.Select)
	}

	// Optimize correlated subqueries
	o.optimizeCorrelatedSubqueries(query)

	o.logger.Debug("Applied subquery optimization")
	return nil
}

// materializedViewCheck checks if materialized views can be used
func (o *UQLOptimizerImpl) materializedViewCheck(query *models.UQLQuery) error {
	// Check if the query can be satisfied by a materialized view
	if mv := o.findApplicableMaterializedView(query); mv != nil {
		o.rewriteQueryForMaterializedView(query, mv)
		o.logger.Debug("Applied materialized view optimization")
	}

	return nil
}

// Helper methods for optimizations

// optimizeMetricsPredicates optimizes predicates for metrics queries
func (o *UQLOptimizerImpl) optimizeMetricsPredicates(selectClause *models.UQLSelect) {
	// Group label selectors for better PromQL performance
	if selectClause.Where != nil {
		o.groupLabelSelectors(selectClause.Where)
	}
}

// optimizeLogsPredicates optimizes predicates for logs queries
func (o *UQLOptimizerImpl) optimizeLogsPredicates(selectClause *models.UQLSelect) {
	// Order filters by selectivity for better performance
	if selectClause.Where != nil {
		o.orderFiltersBySelectivity(selectClause.Where)
	}
}

// optimizeMetricsAggregationPredicates optimizes aggregation predicates for metrics
func (o *UQLOptimizerImpl) optimizeMetricsAggregationPredicates(aggClause *models.UQLAggregation) {
	// Push label selectors before aggregation
	if aggClause.Where != nil {
		o.groupLabelSelectors(aggClause.Where)
	}
}

// optimizeTimeWindow optimizes time window specifications
func (o *UQLOptimizerImpl) optimizeTimeWindow(window time.Duration) time.Duration {
	// Round time windows to common intervals for better caching
	commonIntervals := []time.Duration{
		time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute,
		time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour,
	}

	var closest time.Duration
	minDiff := window

	for _, interval := range commonIntervals {
		diff := window - interval
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			closest = interval
		}
	}

	if closest != 0 && minDiff < window/10 { // Within 10% tolerance
		return closest
	}

	return window
}

// optimizeFieldSelection optimizes field selection in SELECT clauses
func (o *UQLOptimizerImpl) optimizeFieldSelection(fields []models.UQLField) []models.UQLField {
	// Remove duplicate fields
	seen := make(map[string]bool)
	var unique []models.UQLField

	for _, field := range fields {
		key := field.Name + string(field.Function)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, field)
		}
	}

	return unique
}

// optimizeOrderBy optimizes ORDER BY clauses
func (o *UQLOptimizerImpl) optimizeOrderBy(orderBy []models.UQLOrderBy) []models.UQLOrderBy {
	// Sort by field name for consistent ordering
	sort.Slice(orderBy, func(i, j int) bool {
		return orderBy[i].Field < orderBy[j].Field
	})

	return orderBy
}

// optimizeCorrelationOperators optimizes correlation operators
func (o *UQLOptimizerImpl) optimizeCorrelationOperators(corr *models.UQLCorrelation) *models.UQLCorrelation {
	// For time-window correlations, ensure optimal operator ordering
	if corr.TimeWindow != nil {
		// WITHIN is generally more efficient than NEAR for small windows
		if *corr.TimeWindow < 5*time.Minute && corr.Operator == models.OpNEAR {
			o.logger.Debug("Converting NEAR to WITHIN for small time window")
			corr.Operator = models.OpWITHIN
		}
	}

	return corr
}

// optimizeAggregationFunction optimizes aggregation functions
func (o *UQLOptimizerImpl) optimizeAggregationFunction(fn models.UQLFunction) models.UQLFunction {
	// Some functions can be optimized to more efficient equivalents
	switch fn {
	case models.FuncAVG:
		// AVG can sometimes be optimized to a combination of SUM and COUNT
		// but we keep it as-is for now
		return fn
	default:
		return fn
	}
}

// groupLabelSelectors groups label selectors for PromQL optimization
func (o *UQLOptimizerImpl) groupLabelSelectors(condition *models.UQLCondition) {
	// Recursively group AND conditions for better PromQL label matching
	if condition.And != nil {
		o.groupLabelSelectors(condition.And)
	}
	if condition.Or != nil {
		o.groupLabelSelectors(condition.Or)
	}
}

// orderFiltersBySelectivity orders filters by estimated selectivity
func (o *UQLOptimizerImpl) orderFiltersBySelectivity(condition *models.UQLCondition) {
	// This is a simplified selectivity estimation
	// In a full implementation, this would use statistics about field cardinalities

	selectivityOrder := map[string]int{
		"level":     1, // High selectivity (few distinct values)
		"service":   2,
		"host":      3,
		"timestamp": 4, // Low selectivity (many distinct values)
	}

	// For now, just ensure high-selectivity filters come first
	if condition.And != nil {
		o.reorderBySelectivity(condition, selectivityOrder)
	}
}

// reorderBySelectivity reorders conditions by selectivity
func (o *UQLOptimizerImpl) reorderBySelectivity(condition *models.UQLCondition, selectivityOrder map[string]int) {
	// This is a placeholder for more sophisticated reordering logic
	// In practice, this would reorder the AND conditions based on selectivity estimates
}

// identifyUsedFields identifies which fields are actually used in the query
func (o *UQLOptimizerImpl) identifyUsedFields(query *models.UQLQuery) map[string]bool {
	used := make(map[string]bool)

	// Mark fields used in WHERE clause
	if query.Select != nil && query.Select.Where != nil {
		o.markUsedFieldsInCondition(query.Select.Where, used)
	}

	// Mark fields used in GROUP BY
	for _, groupField := range query.Select.GroupBy {
		used[groupField] = true
	}

	// Mark fields used in ORDER BY
	for _, orderField := range query.OrderBy {
		used[orderField.Field] = true
	}

	return used
}

// markUsedFieldsInCondition marks fields used in conditions
func (o *UQLOptimizerImpl) markUsedFieldsInCondition(condition *models.UQLCondition, used map[string]bool) {
	if condition.Field != "" {
		used[condition.Field] = true
	}

	if condition.And != nil {
		o.markUsedFieldsInCondition(condition.And, used)
	}
	if condition.Or != nil {
		o.markUsedFieldsInCondition(condition.Or, used)
	}
}

// isFieldUsed checks if a field is used
func (o *UQLOptimizerImpl) isFieldUsed(field models.UQLField, used map[string]bool) bool {
	// Always keep wildcard fields
	if field.Name == "*" {
		return true
	}

	// Check if the field name is used
	return used[field.Name]
}

// optimizeJoinOrder optimizes the order of joins for better performance
func (o *UQLOptimizerImpl) optimizeJoinOrder(join *models.UQLJoin) *models.UQLJoin {
	// For now, keep the original order
	// In a full implementation, this would reorder joins based on table sizes and selectivity
	return join
}

// selectJoinAlgorithm chooses the optimal join algorithm
func (o *UQLOptimizerImpl) selectJoinAlgorithm(join *models.UQLJoin) {
	// For now, default to hash join for most cases
	// In a full implementation, this would choose between nested loop, hash, merge joins
	join.JoinType = models.JoinTypeTime // Prefer time-based joins for observability data
}

// selectIndexesForSelect selects optimal indexes for SELECT queries
func (o *UQLOptimizerImpl) selectIndexesForSelect(selectClause *models.UQLSelect) {
	// Analyze WHERE conditions to determine best indexes
	if selectClause.Where != nil {
		o.analyzeIndexUsage(selectClause.Where, selectClause.DataSource.Engine)
	}
}

// selectIndexesForAggregation selects optimal indexes for aggregation queries
func (o *UQLOptimizerImpl) selectIndexesForAggregation(aggClause *models.UQLAggregation) {
	// Analyze WHERE and GROUP BY for index selection
	if aggClause.Where != nil {
		o.analyzeIndexUsage(aggClause.Where, aggClause.DataSource.Engine)
	}
	// Consider indexes for GROUP BY fields
	for _, groupField := range aggClause.GroupBy {
		o.considerIndexForField(groupField, aggClause.DataSource.Engine)
	}
}

// optimizeCorrelationExecutionStrategy optimizes how correlation queries are executed
func (o *UQLOptimizerImpl) optimizeCorrelationExecutionStrategy(query *models.UQLQuery) {
	// For correlations with small time windows, prefer sequential execution
	// For larger time windows, prefer parallel execution
	if query.Correlation != nil && query.Correlation.TimeWindow != nil {
		if *query.Correlation.TimeWindow < 5*time.Minute {
			// Small window - sequential might be better
			o.logger.Debug("Optimizing correlation for small time window - sequential execution")
		} else {
			// Large window - parallel execution beneficial
			o.logger.Debug("Optimizing correlation for large time window - parallel execution")
		}
	}
}

// isComplexQuery determines if a query is complex enough to warrant advanced optimization
func (o *UQLOptimizerImpl) isComplexQuery(query *models.UQLQuery) bool {
	// Consider a query complex if it has multiple conditions, joins, or aggregations
	switch query.Type {
	case models.UQLQueryTypeSelect:
		if query.Select == nil {
			return false
		}
		complexity := len(query.Select.Fields)
		if query.Select.Where != nil {
			complexity += o.calculateConditionComplexity(query.Select.Where)
		}
		complexity += len(query.Select.GroupBy)
		return complexity > 5
	case models.UQLQueryTypeCorrelation:
		return query.Correlation != nil
	case models.UQLQueryTypeAggregation:
		return query.Aggregation != nil
	case models.UQLQueryTypeJoin:
		return true
	default:
		return false
	}
}

// optimizeExecutionOrder optimizes the order of operations in complex queries
func (o *UQLOptimizerImpl) optimizeExecutionOrder(query *models.UQLQuery) {
	// This is a placeholder for execution order optimization
	// In practice, this would reorder operations like filters, joins, aggregations
	o.logger.Debug("Optimizing execution order for complex query")
}

// generateQueryPlanHash generates a hash for query plan caching
func (o *UQLOptimizerImpl) generateQueryPlanHash(query *models.UQLQuery) string {
	// Simple hash based on query structure
	// In practice, this would be a more sophisticated hash
	return fmt.Sprintf("%s_%s", query.Type, query.RawQuery)
}

// getCachedPlan retrieves a cached query plan
func (o *UQLOptimizerImpl) getCachedPlan(planHash string) *models.UQLQuery {
	// Placeholder for plan caching
	// In practice, this would check a cache store
	return nil
}

// applyCachedPlan applies optimizations from a cached plan
func (o *UQLOptimizerImpl) applyCachedPlan(query, cachedPlan *models.UQLQuery) {
	// Placeholder for applying cached optimizations
	o.logger.Debug("Applying cached query plan optimizations")
}

// cacheQueryPlan caches a query plan for future use
func (o *UQLOptimizerImpl) cacheQueryPlan(planHash string, query *models.UQLQuery) {
	// Placeholder for plan caching
	o.logger.Debug("Caching query plan", "hash", planHash)
}

// convertSubqueriesToJoins converts subqueries to joins where beneficial
func (o *UQLOptimizerImpl) convertSubqueriesToJoins(selectClause *models.UQLSelect) {
	// Placeholder for subquery to join conversion
	// This would analyze subqueries and convert them to joins when appropriate
}

// optimizeCorrelatedSubqueries optimizes correlated subqueries
func (o *UQLOptimizerImpl) optimizeCorrelatedSubqueries(query *models.UQLQuery) {
	// Placeholder for correlated subquery optimization
	// This would optimize subqueries that reference outer query variables
}

// findApplicableMaterializedView finds materialized views that can satisfy the query
func (o *UQLOptimizerImpl) findApplicableMaterializedView(query *models.UQLQuery) *MaterializedView {
	// Placeholder for materialized view matching
	// This would check if any materialized views can satisfy the query
	return nil
}

// rewriteQueryForMaterializedView rewrites a query to use a materialized view
func (o *UQLOptimizerImpl) rewriteQueryForMaterializedView(query *models.UQLQuery, mv *MaterializedView) {
	// Placeholder for materialized view rewrite
	o.logger.Debug("Rewriting query to use materialized view")
}

// analyzeIndexUsage analyzes conditions to determine index usage
func (o *UQLOptimizerImpl) analyzeIndexUsage(condition *models.UQLCondition, engine models.UQLEngine) {
	if condition.Field != "" {
		o.considerIndexForField(condition.Field, engine)
	}
	if condition.And != nil {
		o.analyzeIndexUsage(condition.And, engine)
	}
	if condition.Or != nil {
		o.analyzeIndexUsage(condition.Or, engine)
	}
}

// considerIndexForField considers if an index should be used for a field
func (o *UQLOptimizerImpl) considerIndexForField(field string, engine models.UQLEngine) {
	// Placeholder for index consideration logic
	// This would check if indexes exist for the field and engine
	o.logger.Debug("Considering index for field", "field", field, "engine", engine)
}

// calculateConditionComplexity calculates the complexity of a condition tree
func (o *UQLOptimizerImpl) calculateConditionComplexity(condition *models.UQLCondition) int {
	complexity := 1
	if condition.And != nil {
		complexity += o.calculateConditionComplexity(condition.And)
	}
	if condition.Or != nil {
		complexity += o.calculateConditionComplexity(condition.Or)
	}
	return complexity
}

// MaterializedView represents a materialized view
type MaterializedView struct {
	Name        string
	Query       string
	LastRefresh time.Time
}

// buildSelectPlan builds execution plan for SELECT queries
func (o *UQLOptimizerImpl) buildSelectPlan(query *models.UQLQuery, plan *QueryPlan) error {
	if query.Select == nil {
		return fmt.Errorf("missing select clause")
	}

	stepID := 0

	// Add data source
	dataSource := QueryPlanDataSource{
		Name:                 string(query.Select.DataSource.Engine) + ":" + query.Select.DataSource.Query,
		Type:                 string(query.Select.DataSource.Engine),
		Engine:               string(query.Select.DataSource.Engine),
		EstimatedCardinality: 1000, // Default estimate
	}

	// Add filters if present
	if query.Select.Where != nil {
		filters := o.extractFilters(query.Select.Where)
		dataSource.Filters = filters
	}

	plan.DataSources = append(plan.DataSources, dataSource)

	// Data source scan step
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "DataSource Scan",
		Description: fmt.Sprintf("Scan %s data source", dataSource.Type),
		Cost:        10.0,
		Rows:        dataSource.EstimatedCardinality,
		DataSource:  &dataSource,
		Properties: map[string]interface{}{
			"engine": dataSource.Engine,
			"query":  query.Select.DataSource.Query,
		},
	})
	stepID++

	// Filter step if WHERE clause exists
	if query.Select.Where != nil {
		plan.Steps = append(plan.Steps, QueryPlanStep{
			ID:          stepID,
			Operation:   "Filter",
			Description: "Apply WHERE conditions",
			Cost:        5.0,
			Rows:        int64(float64(dataSource.EstimatedCardinality) * 0.1), // Assume 10% selectivity
			Properties: map[string]interface{}{
				"conditions": o.formatCondition(query.Select.Where),
			},
		})
		plan.Steps[stepID-1].Children = []int{stepID}
		stepID++
	}

	// Group by step if present
	if len(query.Select.GroupBy) > 0 {
		plan.Steps = append(plan.Steps, QueryPlanStep{
			ID:          stepID,
			Operation:   "Group By",
			Description: fmt.Sprintf("Group by %v", query.Select.GroupBy),
			Cost:        15.0,
			Rows:        int64(len(query.Select.GroupBy) * 10), // Estimate groups
			Properties: map[string]interface{}{
				"group_fields": query.Select.GroupBy,
			},
		})
		plan.Steps[stepID-1].Children = []int{stepID}
		stepID++
	}

	// Projection step
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "Projection",
		Description: "Select and compute final fields",
		Cost:        2.0,
		Rows:        plan.Steps[stepID-1].Rows,
		Properties: map[string]interface{}{
			"fields": o.formatFields(query.Select.Fields),
		},
	})
	plan.Steps[stepID-1].Children = []int{stepID}
	stepID++

	// Order by step if present
	if len(query.OrderBy) > 0 {
		plan.Steps = append(plan.Steps, QueryPlanStep{
			ID:          stepID,
			Operation:   "Sort",
			Description: "Order results",
			Cost:        20.0,
			Rows:        plan.Steps[stepID-1].Rows,
			Properties: map[string]interface{}{
				"order_fields": o.formatOrderBy(query.OrderBy),
			},
		})
		plan.Steps[stepID-1].Children = []int{stepID}
		stepID++
	}

	// Limit step if present
	if query.Limit != nil {
		plan.Steps = append(plan.Steps, QueryPlanStep{
			ID:          stepID,
			Operation:   "Limit",
			Description: fmt.Sprintf("Limit to %d rows", *query.Limit),
			Cost:        1.0,
			Rows:        int64(*query.Limit),
			Properties: map[string]interface{}{
				"limit": *query.Limit,
			},
		})
		plan.Steps[stepID-1].Children = []int{stepID}
	}

	return nil
}

// buildAggregationPlan builds execution plan for aggregation queries
func (o *UQLOptimizerImpl) buildAggregationPlan(query *models.UQLQuery, plan *QueryPlan) error {
	if query.Aggregation == nil {
		return fmt.Errorf("missing aggregation clause")
	}

	stepID := 0

	// Add data source
	dataSource := QueryPlanDataSource{
		Name:                 string(query.Aggregation.DataSource.Engine) + ":" + query.Aggregation.DataSource.Query,
		Type:                 string(query.Aggregation.DataSource.Engine),
		Engine:               string(query.Aggregation.DataSource.Engine),
		EstimatedCardinality: 10000, // Larger estimate for aggregations
	}

	// Add filters if present
	if query.Aggregation.Where != nil {
		filters := o.extractFilters(query.Aggregation.Where)
		dataSource.Filters = filters
	}

	plan.DataSources = append(plan.DataSources, dataSource)

	// Data source scan step
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "DataSource Scan",
		Description: fmt.Sprintf("Scan %s data source for aggregation", dataSource.Type),
		Cost:        15.0,
		Rows:        dataSource.EstimatedCardinality,
		DataSource:  &dataSource,
		Properties: map[string]interface{}{
			"engine": dataSource.Engine,
			"query":  query.Aggregation.DataSource.Query,
		},
	})
	stepID++

	// Filter step if WHERE clause exists
	if query.Aggregation.Where != nil {
		plan.Steps = append(plan.Steps, QueryPlanStep{
			ID:          stepID,
			Operation:   "Filter",
			Description: "Apply WHERE conditions before aggregation",
			Cost:        5.0,
			Rows:        int64(float64(dataSource.EstimatedCardinality) * 0.1),
			Properties: map[string]interface{}{
				"conditions": o.formatCondition(query.Aggregation.Where),
			},
		})
		plan.Steps[stepID-1].Children = []int{stepID}
		stepID++
	}

	// Aggregation step
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "Aggregation",
		Description: fmt.Sprintf("Compute %s(%s)", query.Aggregation.Function, query.Aggregation.Field),
		Cost:        25.0,
		Rows:        1, // Aggregation typically returns single row
		Properties: map[string]interface{}{
			"function": string(query.Aggregation.Function),
			"field":    query.Aggregation.Field,
			"group_by": query.Aggregation.GroupBy,
		},
	})
	plan.Steps[stepID-1].Children = []int{stepID}

	return nil
}

// buildCorrelationPlan builds execution plan for correlation queries
func (o *UQLOptimizerImpl) buildCorrelationPlan(query *models.UQLQuery, plan *QueryPlan) error {
	if query.Correlation == nil {
		return fmt.Errorf("missing correlation clause")
	}

	stepID := 0

	// Add left data source
	leftDS := QueryPlanDataSource{
		Name:                 o.formatExpression(&query.Correlation.LeftExpression),
		Type:                 string(query.Correlation.LeftExpression.DataSource.Engine),
		Engine:               string(query.Correlation.LeftExpression.DataSource.Engine),
		EstimatedCardinality: 1000,
	}
	plan.DataSources = append(plan.DataSources, leftDS)

	// Add right data source
	rightDS := QueryPlanDataSource{
		Name:                 o.formatExpression(&query.Correlation.RightExpression),
		Type:                 string(query.Correlation.RightExpression.DataSource.Engine),
		Engine:               string(query.Correlation.RightExpression.DataSource.Engine),
		EstimatedCardinality: 1000,
	}
	plan.DataSources = append(plan.DataSources, rightDS)

	// Left data source scan
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "DataSource Scan",
		Description: fmt.Sprintf("Scan left data source: %s", leftDS.Name),
		Cost:        10.0,
		Rows:        leftDS.EstimatedCardinality,
		DataSource:  &leftDS,
	})
	stepID++

	// Right data source scan
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "DataSource Scan",
		Description: fmt.Sprintf("Scan right data source: %s", rightDS.Name),
		Cost:        10.0,
		Rows:        rightDS.EstimatedCardinality,
		DataSource:  &rightDS,
	})
	stepID++

	// Correlation step
	corrDesc := fmt.Sprintf("Correlate using %s", query.Correlation.Operator)
	if query.Correlation.TimeWindow != nil {
		corrDesc += fmt.Sprintf(" within %v", *query.Correlation.TimeWindow)
	}

	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "Correlation",
		Description: corrDesc,
		Cost:        30.0,
		Rows:        int64(float64(leftDS.EstimatedCardinality) * 0.05), // Assume 5% correlation rate
		Properties: map[string]interface{}{
			"operator":     string(query.Correlation.Operator),
			"time_window":  query.Correlation.TimeWindow,
			"left_source":  leftDS.Name,
			"right_source": rightDS.Name,
		},
	})
	plan.Steps[stepID-2].Children = []int{stepID}
	plan.Steps[stepID-1].Children = []int{stepID}

	return nil
}

// buildJoinPlan builds execution plan for join queries
func (o *UQLOptimizerImpl) buildJoinPlan(query *models.UQLQuery, plan *QueryPlan) error {
	if query.Join == nil {
		return fmt.Errorf("missing join clause")
	}

	stepID := 0

	// Add left data source
	leftDS := QueryPlanDataSource{
		Name:                 o.formatExpression(&query.Join.Left),
		Type:                 "join_source",
		Engine:               "join",
		EstimatedCardinality: 1000,
	}
	plan.DataSources = append(plan.DataSources, leftDS)

	// Add right data source
	rightDS := QueryPlanDataSource{
		Name:                 o.formatExpression(&query.Join.Right),
		Type:                 "join_source",
		Engine:               "join",
		EstimatedCardinality: 1000,
	}
	plan.DataSources = append(plan.DataSources, rightDS)

	// Left scan
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "DataSource Scan",
		Description: "Scan left join source",
		Cost:        10.0,
		Rows:        leftDS.EstimatedCardinality,
		DataSource:  &leftDS,
	})
	stepID++

	// Right scan
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "DataSource Scan",
		Description: "Scan right join source",
		Cost:        10.0,
		Rows:        rightDS.EstimatedCardinality,
		DataSource:  &rightDS,
	})
	stepID++

	// Join step
	joinDesc := fmt.Sprintf("%s Join", query.Join.JoinType)
	plan.Steps = append(plan.Steps, QueryPlanStep{
		ID:          stepID,
		Operation:   "Join",
		Description: joinDesc,
		Cost:        50.0,
		Rows:        int64(float64(leftDS.EstimatedCardinality) * float64(rightDS.EstimatedCardinality) * 0.01), // Join cardinality estimate
		Properties: map[string]interface{}{
			"join_type": string(query.Join.JoinType),
			"condition": o.formatCondition(&query.Join.Condition),
		},
	})
	plan.Steps[stepID-2].Children = []int{stepID}
	plan.Steps[stepID-1].Children = []int{stepID}

	return nil
}

// estimatePlanCosts estimates costs and cardinalities for the plan
func (o *UQLOptimizerImpl) estimatePlanCosts(plan *QueryPlan) {
	totalCost := 0.0
	totalRows := int64(0)

	for i := range plan.Steps {
		step := &plan.Steps[i]
		totalCost += step.Cost

		// Apply optimizations that affect cost
		if o.enabledOpts["index_selection"] && len(plan.DataSources) > 0 {
			step.Cost *= 0.7 // Index reduces cost by 30%
			plan.Optimizations = append(plan.Optimizations, "index_selection")
		}

		if o.enabledOpts["query_plan_caching"] {
			step.Cost *= 0.9 // Caching reduces cost by 10%
			plan.Optimizations = append(plan.Optimizations, "query_plan_caching")
		}

		totalRows = step.Rows
	}

	plan.EstimatedCost = totalCost
	plan.EstimatedRows = totalRows
}

// formatQueryPlan formats the query plan as a human-readable string
func (o *UQLOptimizerImpl) formatQueryPlan(plan *QueryPlan) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("Query Plan for %s\n", plan.QueryID))
	result.WriteString(fmt.Sprintf("Query Type: %s\n", plan.QueryType))
	result.WriteString(fmt.Sprintf("Estimated Cost: %.2f\n", plan.EstimatedCost))
	result.WriteString(fmt.Sprintf("Estimated Rows: %d\n", plan.EstimatedRows))
	result.WriteString(fmt.Sprintf("Generated At: %s\n", plan.GeneratedAt.Format(time.RFC3339)))

	if len(plan.Optimizations) > 0 {
		result.WriteString(fmt.Sprintf("Optimizations Applied: %v\n", plan.Optimizations))
	}

	result.WriteString("\nExecution Steps:\n")

	// Sort steps by ID for proper ordering
	sortedSteps := make([]QueryPlanStep, len(plan.Steps))
	copy(sortedSteps, plan.Steps)
	sort.Slice(sortedSteps, func(i, j int) bool {
		return sortedSteps[i].ID < sortedSteps[j].ID
	})

	for _, step := range sortedSteps {
		result.WriteString(o.formatStep(step, 0))
	}

	if len(plan.DataSources) > 0 {
		result.WriteString("\nData Sources:\n")
		for _, ds := range plan.DataSources {
			result.WriteString(fmt.Sprintf("  - %s (%s)\n", ds.Name, ds.Type))
			if len(ds.Filters) > 0 {
				result.WriteString(fmt.Sprintf("    Filters: %v\n", ds.Filters))
			}
			if len(ds.Indexes) > 0 {
				result.WriteString(fmt.Sprintf("    Indexes: %v\n", ds.Indexes))
			}
		}
	}

	return result.String()
}

// formatStep formats a single execution step with indentation
func (o *UQLOptimizerImpl) formatStep(step QueryPlanStep, indent int) string {
	var result strings.Builder

	indentStr := strings.Repeat("  ", indent)
	result.WriteString(fmt.Sprintf("%s%d. %s\n", indentStr, step.ID, step.Operation))
	result.WriteString(fmt.Sprintf("%s   Description: %s\n", indentStr, step.Description))
	result.WriteString(fmt.Sprintf("%s   Cost: %.2f, Rows: %d\n", indentStr, step.Cost, step.Rows))

	if step.DataSource != nil {
		result.WriteString(fmt.Sprintf("%s   Data Source: %s\n", indentStr, step.DataSource.Name))
	}

	if len(step.Properties) > 0 {
		result.WriteString(fmt.Sprintf("%s   Properties: %v\n", indentStr, step.Properties))
	}

	return result.String()
}

// Helper methods for formatting

// extractFilters extracts filter conditions from a condition tree
func (o *UQLOptimizerImpl) extractFilters(condition *models.UQLCondition) []string {
	var filters []string
	o.extractFiltersRecursive(condition, &filters)
	return filters
}

// extractFiltersRecursive recursively extracts filter conditions
func (o *UQLOptimizerImpl) extractFiltersRecursive(condition *models.UQLCondition, filters *[]string) {
	if condition == nil {
		return
	}

	if condition.Field != "" {
		filter := fmt.Sprintf("%s %s %v", condition.Field, condition.Operator, condition.Value)
		*filters = append(*filters, filter)
	}

	o.extractFiltersRecursive(condition.And, filters)
	o.extractFiltersRecursive(condition.Or, filters)
}

// formatCondition formats a condition tree as a string
func (o *UQLOptimizerImpl) formatCondition(condition *models.UQLCondition) string {
	if condition == nil {
		return ""
	}

	var result strings.Builder
	o.formatConditionRecursive(condition, &result)
	return result.String()
}

// formatConditionRecursive recursively formats condition tree
func (o *UQLOptimizerImpl) formatConditionRecursive(condition *models.UQLCondition, result *strings.Builder) {
	if condition.Field != "" {
		fmt.Fprintf(result, "%s %s %v", condition.Field, condition.Operator, condition.Value)
	}

	if condition.And != nil {
		result.WriteString(" AND ")
		o.formatConditionRecursive(condition.And, result)
	}

	if condition.Or != nil {
		result.WriteString(" OR ")
		o.formatConditionRecursive(condition.Or, result)
	}
}

// formatFields formats field list as string
func (o *UQLOptimizerImpl) formatFields(fields []models.UQLField) []string {
	var result []string
	for _, field := range fields {
		if field.Function != "" {
			result = append(result, fmt.Sprintf("%s(%s)", field.Function, field.Name))
		} else {
			result = append(result, field.Name)
		}
	}
	return result
}

// formatOrderBy formats order by clause as string
func (o *UQLOptimizerImpl) formatOrderBy(orderBy []models.UQLOrderBy) []string {
	var result []string
	for _, ob := range orderBy {
		result = append(result, fmt.Sprintf("%s %s", ob.Field, ob.Direction))
	}
	return result
}

// formatExpression formats a UQL expression as string
func (o *UQLOptimizerImpl) formatExpression(expr *models.UQLExpression) string {
	if expr.DataSource != nil {
		return fmt.Sprintf("%s:%s", expr.DataSource.Engine, expr.DataSource.Query)
	}
	return "unknown"
}

// deepCopyUQLQuery creates a deep copy of a UQL query
func (o *UQLOptimizerImpl) deepCopyUQLQuery(query *models.UQLQuery) *models.UQLQuery {
	// Create a new query with copied values
	copied := &models.UQLQuery{
		Type:     query.Type,
		RawQuery: query.RawQuery,
	}

	// Deep copy components based on type
	switch query.Type {
	case models.UQLQueryTypeSelect:
		if query.Select != nil {
			copied.Select = o.deepCopyUQLSelect(query.Select)
		}
	case models.UQLQueryTypeCorrelation:
		if query.Correlation != nil {
			copied.Correlation = o.deepCopyUQLCorrelation(query.Correlation)
		}
	case models.UQLQueryTypeAggregation:
		if query.Aggregation != nil {
			copied.Aggregation = o.deepCopyUQLAggregation(query.Aggregation)
		}
	}

	// Copy common fields
	if query.TimeWindow != nil {
		timeWindow := *query.TimeWindow
		copied.TimeWindow = &timeWindow
	}
	if query.Limit != nil {
		limit := *query.Limit
		copied.Limit = &limit
	}
	copied.OrderBy = make([]models.UQLOrderBy, len(query.OrderBy))
	copy(copied.OrderBy, query.OrderBy)

	return copied
}

// deepCopyUQLSelect creates a deep copy of UQLSelect
func (o *UQLOptimizerImpl) deepCopyUQLSelect(selectClause *models.UQLSelect) *models.UQLSelect {
	copied := &models.UQLSelect{
		DataSource: selectClause.DataSource,
		GroupBy:    make([]string, len(selectClause.GroupBy)),
	}

	copy(copied.GroupBy, selectClause.GroupBy)
	copied.Fields = make([]models.UQLField, len(selectClause.Fields))
	copy(copied.Fields, selectClause.Fields)

	if selectClause.Where != nil {
		copied.Where = o.deepCopyUQLCondition(selectClause.Where)
	}
	if selectClause.Having != nil {
		copied.Having = o.deepCopyUQLCondition(selectClause.Having)
	}

	return copied
}

// deepCopyUQLCorrelation creates a deep copy of UQLCorrelation
func (o *UQLOptimizerImpl) deepCopyUQLCorrelation(corr *models.UQLCorrelation) *models.UQLCorrelation {
	copy := &models.UQLCorrelation{
		LeftExpression:  corr.LeftExpression,
		RightExpression: corr.RightExpression,
		Operator:        corr.Operator,
	}

	if corr.TimeWindow != nil {
		timeWindow := *corr.TimeWindow
		copy.TimeWindow = &timeWindow
	}
	if corr.JoinCondition != nil {
		copy.JoinCondition = o.deepCopyUQLCondition(corr.JoinCondition)
	}

	return copy
}

// deepCopyUQLAggregation creates a deep copy of UQLAggregation
func (o *UQLOptimizerImpl) deepCopyUQLAggregation(agg *models.UQLAggregation) *models.UQLAggregation {
	copied := &models.UQLAggregation{
		Function:   agg.Function,
		Field:      agg.Field,
		DataSource: agg.DataSource,
		GroupBy:    make([]string, len(agg.GroupBy)),
		Arguments:  make([]string, len(agg.Arguments)),
	}

	copy(copied.GroupBy, agg.GroupBy)
	copy(copied.Arguments, agg.Arguments)

	if agg.Where != nil {
		copied.Where = o.deepCopyUQLCondition(agg.Where)
	}

	return copied
}

// deepCopyUQLCondition creates a deep copy of UQLCondition
func (o *UQLOptimizerImpl) deepCopyUQLCondition(cond *models.UQLCondition) *models.UQLCondition {
	copy := &models.UQLCondition{
		Field:    cond.Field,
		Operator: cond.Operator,
		Value:    cond.Value,
	}

	if cond.And != nil {
		copy.And = o.deepCopyUQLCondition(cond.And)
	}
	if cond.Or != nil {
		copy.Or = o.deepCopyUQLCondition(cond.Or)
	}

	return copy
}
