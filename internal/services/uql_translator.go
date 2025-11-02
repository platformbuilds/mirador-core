package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// UQLTranslator translates UQL queries to engine-specific query languages
type UQLTranslator interface {
	// TranslateQuery translates a UQL query to the appropriate engine-specific query
	TranslateQuery(uqlQuery *models.UQLQuery) (*TranslatedQuery, error)

	// CanTranslate checks if this translator can handle the given UQL query
	CanTranslate(uqlQuery *models.UQLQuery) bool

	// GetSupportedEngines returns the engines this translator supports
	GetSupportedEngines() []models.UQLEngine
}

// TranslatedQuery represents a query translated to engine-specific format
type TranslatedQuery struct {
	Engine      models.UQLEngine       `json:"engine"`
	Query       string                 `json:"query"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	StartTime   *time.Time             `json:"start_time,omitempty"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	TimeWindow  *time.Duration         `json:"time_window,omitempty"`
	Limit       *int                   `json:"limit,omitempty"`
	OrderBy     []string               `json:"order_by,omitempty"`
	GroupBy     []string               `json:"group_by,omitempty"`
	Aggregation *AggregationSpec       `json:"aggregation,omitempty"`
}

// AggregationSpec defines aggregation parameters
type AggregationSpec struct {
	Function   string   `json:"function"`
	Field      string   `json:"field"`
	Parameters []string `json:"parameters,omitempty"`
}

// UQLTranslatorRegistry manages multiple translators
type UQLTranslatorRegistry struct {
	translators []UQLTranslator
	logger      logger.Logger
}

// NewUQLTranslatorRegistry creates a new translator registry
func NewUQLTranslatorRegistry(logger logger.Logger) *UQLTranslatorRegistry {
	registry := &UQLTranslatorRegistry{
		translators: make([]UQLTranslator, 0),
		logger:      logger,
	}

	// Register built-in translators
	registry.RegisterTranslator(NewPromQLTranslator(logger))
	registry.RegisterTranslator(NewLogsQLTranslator(logger))
	registry.RegisterTranslator(NewTracesQLTranslator(logger))
	registry.RegisterTranslator(NewCorrelationTranslator(logger))

	return registry
}

// RegisterTranslator registers a new translator
func (r *UQLTranslatorRegistry) RegisterTranslator(translator UQLTranslator) {
	r.translators = append(r.translators, translator)
	r.logger.Info("Registered UQL translator", "engines", translator.GetSupportedEngines())
}

// TranslateQuery translates a UQL query using the appropriate translator
func (r *UQLTranslatorRegistry) TranslateQuery(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	for _, translator := range r.translators {
		if translator.CanTranslate(uqlQuery) {
			r.logger.Debug("Translating UQL query",
				"query_type", uqlQuery.Type,
				"translator_engines", translator.GetSupportedEngines())

			translated, err := translator.TranslateQuery(uqlQuery)
			if err != nil {
				r.logger.Warn("Translation failed", "error", err)
				return nil, fmt.Errorf("translation failed: %w", err)
			}

			r.logger.Info("Successfully translated UQL query",
				"from_type", uqlQuery.Type,
				"to_engine", translated.Engine,
				"translated_query", translated.Query)

			return translated, nil
		}
	}

	return nil, fmt.Errorf("no translator found for UQL query type: %s", uqlQuery.Type)
}

// PromQLTranslator translates UQL queries to PromQL for VictoriaMetrics
type PromQLTranslator struct {
	logger logger.Logger
}

// NewPromQLTranslator creates a new PromQL translator
func NewPromQLTranslator(logger logger.Logger) *PromQLTranslator {
	return &PromQLTranslator{logger: logger}
}

// CanTranslate checks if this translator can handle the given UQL query
func (t *PromQLTranslator) CanTranslate(uqlQuery *models.UQLQuery) bool {
	// Can translate SELECT queries with metrics data source
	if uqlQuery.Type == models.UQLQueryTypeSelect && uqlQuery.Select != nil {
		return uqlQuery.Select.DataSource.Engine == models.EngineMetrics
	}

	// Can translate aggregation queries with metrics data source
	if uqlQuery.Type == models.UQLQueryTypeAggregation && uqlQuery.Aggregation != nil {
		return uqlQuery.Aggregation.DataSource.Engine == models.EngineMetrics
	}

	return false
}

// GetSupportedEngines returns the engines this translator supports
func (t *PromQLTranslator) GetSupportedEngines() []models.UQLEngine {
	return []models.UQLEngine{models.EngineMetrics}
}

// TranslateQuery translates a UQL query to PromQL
func (t *PromQLTranslator) TranslateQuery(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	switch uqlQuery.Type {
	case models.UQLQueryTypeSelect:
		return t.translateSelectToPromQL(uqlQuery)
	case models.UQLQueryTypeAggregation:
		return t.translateAggregationToPromQL(uqlQuery)
	default:
		return nil, fmt.Errorf("unsupported UQL query type for PromQL translation: %s", uqlQuery.Type)
	}
}

// translateSelectToPromQL translates a SELECT query to PromQL
func (t *PromQLTranslator) translateSelectToPromQL(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	selectClause := uqlQuery.Select

	// Build metric selector
	var promQL strings.Builder

	// Handle aggregation functions in SELECT fields
	for i, field := range selectClause.Fields {
		if i > 0 {
			promQL.WriteString(", ")
		}

		if field.Function != "" {
			// Convert UQL aggregation functions to PromQL
			promQLFunc := t.convertUQLFunctionToPromQL(field.Function)
			if field.Name == "*" {
				// For wildcard, use the data source query as the metric name
				promQL.WriteString(fmt.Sprintf("%s(%s)", promQLFunc, selectClause.DataSource.Query))
			} else {
				promQL.WriteString(fmt.Sprintf("%s(%s)", promQLFunc, field.Name))
			}
		} else {
			// Direct metric reference - use data source query if field is "*"
			if field.Name == "*" {
				promQL.WriteString(selectClause.DataSource.Query)
			} else {
				promQL.WriteString(field.Name)
			}
		}
	}

	// Add label selectors from WHERE clause
	if selectClause.Where != nil {
		labelSelectors := t.buildPromQLLabelSelectors(selectClause.Where)
		if labelSelectors != "" {
			promQL.WriteString(fmt.Sprintf("{%s}", labelSelectors))
		}
	}

	translated := &TranslatedQuery{
		Engine: models.EngineMetrics,
		Query:  promQL.String(),
	}

	// Add time window if specified
	if uqlQuery.TimeWindow != nil {
		translated.TimeWindow = uqlQuery.TimeWindow
	}

	return translated, nil
}

// translateAggregationToPromQL translates an aggregation query to PromQL
func (t *PromQLTranslator) translateAggregationToPromQL(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	aggClause := uqlQuery.Aggregation

	// Convert UQL function to PromQL
	promQLFunc := t.convertUQLFunctionToPromQL(aggClause.Function)

	var promQL strings.Builder
	promQL.WriteString(fmt.Sprintf("%s(%s)", promQLFunc, aggClause.Field))

	// Add label selectors from WHERE clause
	if aggClause.Where != nil {
		labelSelectors := t.buildPromQLLabelSelectors(aggClause.Where)
		if labelSelectors != "" {
			promQL.WriteString(fmt.Sprintf("{%s}", labelSelectors))
		}
	}

	translated := &TranslatedQuery{
		Engine: models.EngineMetrics,
		Query:  promQL.String(),
		Aggregation: &AggregationSpec{
			Function:   promQLFunc,
			Field:      aggClause.Field,
			Parameters: aggClause.Arguments,
		},
	}

	return translated, nil
}

// convertUQLFunctionToPromQL converts UQL aggregation functions to PromQL equivalents
func (t *PromQLTranslator) convertUQLFunctionToPromQL(uqlFunc models.UQLFunction) string {
	switch uqlFunc {
	case models.FuncCOUNT:
		return "count"
	case models.FuncSUM:
		return "sum"
	case models.FuncAVG:
		return "avg"
	case models.FuncMIN:
		return "min"
	case models.FuncMAX:
		return "max"
	case models.FuncRATE:
		return "rate"
	case models.FuncINCREASE:
		return "increase"
	case models.FuncPERCENTILE:
		return "quantile"
	case models.FuncHISTOGRAM:
		return "histogram_quantile"
	default:
		return string(uqlFunc)
	}
}

// buildPromQLLabelSelectors builds PromQL label selectors from UQL conditions
func (t *PromQLTranslator) buildPromQLLabelSelectors(condition *models.UQLCondition) string {
	var selectors []string

	// Handle the main condition
	if condition.Field != "" {
		selector := t.buildSingleLabelSelector(condition)
		if selector != "" {
			selectors = append(selectors, selector)
		}
	}

	// Handle AND conditions recursively
	if condition.And != nil {
		andSelectors := t.buildPromQLLabelSelectors(condition.And)
		if andSelectors != "" {
			selectors = append(selectors, andSelectors)
		}
	}

	// Handle OR conditions (PromQL doesn't support OR in label selectors, so we log a warning)
	if condition.Or != nil {
		t.logger.Warn("PromQL does not support OR conditions in label selectors, ignoring OR clause")
	}

	return strings.Join(selectors, ",")
}

// buildSingleLabelSelector builds a single PromQL label selector
func (t *PromQLTranslator) buildSingleLabelSelector(condition *models.UQLCondition) string {
	field := condition.Field
	operator := condition.Operator
	value := condition.Value

	// Convert UQL operators to PromQL label matching operators
	var promQLOp string
	switch operator {
	case models.OpEQ:
		promQLOp = "="
	case models.OpNEQ:
		promQLOp = "!="
	case models.OpLIKE:
		// Convert LIKE to regex match
		promQLOp = "=~"
		if strVal, ok := value.(string); ok {
			// Convert SQL LIKE patterns to regex
			value = strings.ReplaceAll(strVal, "%", ".*")
		}
	case models.OpMATCH:
		promQLOp = "=~"
	default:
		t.logger.Warn("Unsupported operator in PromQL label selector", "operator", operator)
		return ""
	}

	// Format the label selector
	return fmt.Sprintf(`%s%s"%v"`, field, promQLOp, value)
}

// LogsQLTranslator translates UQL queries to LogsQL for VictoriaLogs
type LogsQLTranslator struct {
	logger logger.Logger
}

// NewLogsQLTranslator creates a new LogsQL translator
func NewLogsQLTranslator(logger logger.Logger) *LogsQLTranslator {
	return &LogsQLTranslator{logger: logger}
}

// CanTranslate checks if this translator can handle the given UQL query
func (t *LogsQLTranslator) CanTranslate(uqlQuery *models.UQLQuery) bool {
	// Can translate SELECT queries with logs data source
	if uqlQuery.Type == models.UQLQueryTypeSelect && uqlQuery.Select != nil {
		return uqlQuery.Select.DataSource.Engine == models.EngineLogs
	}

	// Can translate aggregation queries with logs data source
	if uqlQuery.Type == models.UQLQueryTypeAggregation && uqlQuery.Aggregation != nil {
		return uqlQuery.Aggregation.DataSource.Engine == models.EngineLogs
	}

	return false
}

// GetSupportedEngines returns the engines this translator supports
func (t *LogsQLTranslator) GetSupportedEngines() []models.UQLEngine {
	return []models.UQLEngine{models.EngineLogs}
}

// TranslateQuery translates a UQL query to LogsQL
func (t *LogsQLTranslator) TranslateQuery(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	switch uqlQuery.Type {
	case models.UQLQueryTypeSelect:
		return t.translateSelectToLogsQL(uqlQuery)
	case models.UQLQueryTypeAggregation:
		return t.translateAggregationToLogsQL(uqlQuery)
	default:
		return nil, fmt.Errorf("unsupported UQL query type for LogsQL translation: %s", uqlQuery.Type)
	}
}

// translateSelectToLogsQL translates a SELECT query to LogsQL
func (t *LogsQLTranslator) translateSelectToLogsQL(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	selectClause := uqlQuery.Select

	var logsQL strings.Builder

	// Build the base query from data source
	logsQL.WriteString(selectClause.DataSource.Query)

	// Add WHERE conditions as filter expressions
	if selectClause.Where != nil {
		filterExpr := t.buildLogsQLFilterExpression(selectClause.Where)
		if filterExpr != "" {
			logsQL.WriteString(fmt.Sprintf(" | %s", filterExpr))
		}
	}

	translated := &TranslatedQuery{
		Engine: models.EngineLogs,
		Query:  logsQL.String(),
	}

	// Add limit if specified
	if uqlQuery.Limit != nil {
		translated.Limit = uqlQuery.Limit
	}

	return translated, nil
}

// translateAggregationToLogsQL translates an aggregation query to LogsQL
func (t *LogsQLTranslator) translateAggregationToLogsQL(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	aggClause := uqlQuery.Aggregation

	var logsQL strings.Builder

	// Start with the data source query
	logsQL.WriteString(aggClause.DataSource.Query)

	// Add WHERE conditions
	if aggClause.Where != nil {
		filterExpr := t.buildLogsQLFilterExpression(aggClause.Where)
		if filterExpr != "" {
			logsQL.WriteString(fmt.Sprintf(" | %s", filterExpr))
		}
	}

	// Add aggregation
	funcName := t.convertUQLFunctionToLogsQL(aggClause.Function)
	logsQL.WriteString(fmt.Sprintf(" | %s(%s)", funcName, aggClause.Field))

	translated := &TranslatedQuery{
		Engine: models.EngineLogs,
		Query:  logsQL.String(),
		Aggregation: &AggregationSpec{
			Function:   funcName,
			Field:      aggClause.Field,
			Parameters: aggClause.Arguments,
		},
	}

	return translated, nil
}

// buildLogsQLFilterExpression builds LogsQL filter expressions from UQL conditions
func (t *LogsQLTranslator) buildLogsQLFilterExpression(condition *models.UQLCondition) string {
	var filters []string

	// Handle the main condition
	if condition.Field != "" {
		filter := t.buildSingleLogsQLFilter(condition)
		if filter != "" {
			filters = append(filters, filter)
		}
	}

	// Handle AND conditions
	if condition.And != nil {
		andFilter := t.buildLogsQLFilterExpression(condition.And)
		if andFilter != "" {
			filters = append(filters, fmt.Sprintf("(%s)", andFilter))
		}
	}

	// Handle OR conditions
	if condition.Or != nil {
		orFilter := t.buildLogsQLFilterExpression(condition.Or)
		if orFilter != "" {
			filters = append(filters, fmt.Sprintf("(%s)", orFilter))
		}
	}

	if len(filters) == 0 {
		return ""
	}

	// Join with AND operator (LogsQL default)
	return strings.Join(filters, " AND ")
}

// buildSingleLogsQLFilter builds a single LogsQL filter
func (t *LogsQLTranslator) buildSingleLogsQLFilter(condition *models.UQLCondition) string {
	field := condition.Field
	operator := condition.Operator
	value := condition.Value

	switch operator {
	case models.OpEQ:
		return fmt.Sprintf(`%s:"%v"`, field, value)
	case models.OpNEQ:
		return fmt.Sprintf(`%s!:"%v"`, field, value)
	case models.OpLIKE:
		// Convert LIKE to regex
		if strVal, ok := value.(string); ok {
			regex := strings.ReplaceAll(strVal, "%", ".*")
			return fmt.Sprintf(`%s:~"%s"`, field, regex)
		}
	case models.OpMATCH:
		if strVal, ok := value.(string); ok {
			regex := strings.ReplaceAll(strVal, "%", ".*")
			return fmt.Sprintf(`%s:~"%s"`, field, regex)
		}
	case models.OpGT:
		return fmt.Sprintf(`%s > %v`, field, value)
	case models.OpLT:
		return fmt.Sprintf(`%s < %v`, field, value)
	case models.OpGE:
		return fmt.Sprintf(`%s >= %v`, field, value)
	case models.OpLE:
		return fmt.Sprintf(`%s <= %v`, field, value)
	}

	t.logger.Warn("Unsupported operator in LogsQL filter", "operator", operator)
	return ""
}

// convertUQLFunctionToLogsQL converts UQL functions to LogsQL equivalents
func (t *LogsQLTranslator) convertUQLFunctionToLogsQL(uqlFunc models.UQLFunction) string {
	switch uqlFunc {
	case models.FuncCOUNT:
		return "count"
	case models.FuncSUM:
		return "sum"
	case models.FuncAVG:
		return "avg"
	case models.FuncMIN:
		return "min"
	case models.FuncMAX:
		return "max"
	case models.FuncRATE:
		return "rate"
	case models.FuncINCREASE:
		return "increase"
	default:
		return string(uqlFunc)
	}
}

// TracesQLTranslator translates UQL queries to TracesQL for VictoriaTraces
type TracesQLTranslator struct {
	logger logger.Logger
}

// NewTracesQLTranslator creates a new TracesQL translator
func NewTracesQLTranslator(logger logger.Logger) *TracesQLTranslator {
	return &TracesQLTranslator{logger: logger}
}

// CanTranslate checks if this translator can handle the given UQL query
func (t *TracesQLTranslator) CanTranslate(uqlQuery *models.UQLQuery) bool {
	// Can translate SELECT queries with traces data source
	if uqlQuery.Type == models.UQLQueryTypeSelect && uqlQuery.Select != nil {
		return uqlQuery.Select.DataSource.Engine == models.EngineTraces
	}

	// Can translate aggregation queries with traces data source
	if uqlQuery.Type == models.UQLQueryTypeAggregation && uqlQuery.Aggregation != nil {
		return uqlQuery.Aggregation.DataSource.Engine == models.EngineTraces
	}

	return false
}

// GetSupportedEngines returns the engines this translator supports
func (t *TracesQLTranslator) GetSupportedEngines() []models.UQLEngine {
	return []models.UQLEngine{models.EngineTraces}
}

// TranslateQuery translates a UQL query to TracesQL
func (t *TracesQLTranslator) TranslateQuery(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	switch uqlQuery.Type {
	case models.UQLQueryTypeSelect:
		return t.translateSelectToTracesQL(uqlQuery)
	case models.UQLQueryTypeAggregation:
		return t.translateAggregationToTracesQL(uqlQuery)
	default:
		return nil, fmt.Errorf("unsupported UQL query type for TracesQL translation: %s", uqlQuery.Type)
	}
}

// translateSelectToTracesQL translates a SELECT query to TracesQL
func (t *TracesQLTranslator) translateSelectToTracesQL(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	selectClause := uqlQuery.Select

	var tracesQL strings.Builder

	// Build service selector
	if strings.Contains(selectClause.DataSource.Query, "service:") {
		tracesQL.WriteString(selectClause.DataSource.Query)
	} else {
		// Default to service search
		tracesQL.WriteString(fmt.Sprintf(`{service.name="%s"}`, selectClause.DataSource.Query))
	}

	// Add WHERE conditions as tag filters
	if selectClause.Where != nil {
		tagFilters := t.buildTracesQLTagFilters(selectClause.Where)
		if tagFilters != "" {
			tracesQL.WriteString(fmt.Sprintf(" && %s", tagFilters))
		}
	}

	translated := &TranslatedQuery{
		Engine: models.EngineTraces,
		Query:  tracesQL.String(),
	}

	return translated, nil
}

// translateAggregationToTracesQL translates an aggregation query to TracesQL
func (t *TracesQLTranslator) translateAggregationToTracesQL(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	aggClause := uqlQuery.Aggregation

	var tracesQL strings.Builder

	// Build base query
	if strings.Contains(aggClause.DataSource.Query, "service:") {
		tracesQL.WriteString(aggClause.DataSource.Query)
	} else {
		tracesQL.WriteString(fmt.Sprintf(`{service.name="%s"}`, aggClause.DataSource.Query))
	}

	// Add WHERE conditions
	if aggClause.Where != nil {
		tagFilters := t.buildTracesQLTagFilters(aggClause.Where)
		if tagFilters != "" {
			tracesQL.WriteString(fmt.Sprintf(" && %s", tagFilters))
		}
	}

	// Add aggregation (TracesQL has limited aggregation support)
	// For now, we'll just return the filtered traces
	translated := &TranslatedQuery{
		Engine: models.EngineTraces,
		Query:  tracesQL.String(),
		Aggregation: &AggregationSpec{
			Function:   string(aggClause.Function),
			Field:      aggClause.Field,
			Parameters: aggClause.Arguments,
		},
	}

	return translated, nil
}

// buildTracesQLTagFilters builds TracesQL tag filters from UQL conditions
func (t *TracesQLTranslator) buildTracesQLTagFilters(condition *models.UQLCondition) string {
	var filters []string

	// Handle the main condition
	if condition.Field != "" {
		filter := t.buildSingleTracesQLTagFilter(condition)
		if filter != "" {
			filters = append(filters, filter)
		}
	}

	// Handle AND conditions
	if condition.And != nil {
		andFilter := t.buildTracesQLTagFilters(condition.And)
		if andFilter != "" {
			filters = append(filters, andFilter)
		}
	}

	// Handle OR conditions
	if condition.Or != nil {
		orFilter := t.buildTracesQLTagFilters(condition.Or)
		if orFilter != "" {
			filters = append(filters, orFilter)
		}
	}

	return strings.Join(filters, " && ")
}

// buildSingleTracesQLTagFilter builds a single TracesQL tag filter
func (t *TracesQLTranslator) buildSingleTracesQLTagFilter(condition *models.UQLCondition) string {
	field := condition.Field
	operator := condition.Operator
	value := condition.Value

	switch operator {
	case models.OpEQ:
		return fmt.Sprintf(`%s="%v"`, field, value)
	case models.OpNEQ:
		return fmt.Sprintf(`%s!="%v"`, field, value)
	case models.OpLIKE:
		// TracesQL doesn't have direct LIKE support, convert to regex
		if strVal, ok := value.(string); ok {
			regex := strings.ReplaceAll(strVal, "%", ".*")
			return fmt.Sprintf(`%s=~"%s"`, field, regex)
		}
	case models.OpMATCH:
		if strVal, ok := value.(string); ok {
			regex := strings.ReplaceAll(strVal, "%", ".*")
			return fmt.Sprintf(`%s=~"%s"`, field, regex)
		}
	}

	t.logger.Warn("Unsupported operator in TracesQL tag filter", "operator", operator)
	return ""
}

// CorrelationTranslator handles correlation queries
type CorrelationTranslator struct {
	logger logger.Logger
}

// NewCorrelationTranslator creates a new correlation translator
func NewCorrelationTranslator(logger logger.Logger) *CorrelationTranslator {
	return &CorrelationTranslator{logger: logger}
}

// CanTranslate checks if this translator can handle correlation queries
func (t *CorrelationTranslator) CanTranslate(uqlQuery *models.UQLQuery) bool {
	return uqlQuery.Type == models.UQLQueryTypeCorrelation
}

// GetSupportedEngines returns the engines this translator supports
func (t *CorrelationTranslator) GetSupportedEngines() []models.UQLEngine {
	return []models.UQLEngine{models.EngineCorrelation}
}

// TranslateQuery translates a correlation query
func (t *CorrelationTranslator) TranslateQuery(uqlQuery *models.UQLQuery) (*TranslatedQuery, error) {
	if uqlQuery.Correlation == nil {
		return nil, fmt.Errorf("missing correlation clause")
	}

	// For correlation queries, we keep the original UQL format
	// as the correlation engine expects this format
	translated := &TranslatedQuery{
		Engine: models.EngineCorrelation,
		Query:  uqlQuery.RawQuery,
	}

	// Add time window if specified
	if uqlQuery.Correlation.TimeWindow != nil {
		translated.TimeWindow = uqlQuery.Correlation.TimeWindow
	}

	return translated, nil
}
