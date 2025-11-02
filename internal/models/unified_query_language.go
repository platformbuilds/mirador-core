// Package unifiedquery provides a comprehensive query language for Mirador Core v7.0.0
// that spans metrics, logs, traces, and correlations with advanced operators.
//
// Grammar Design for Unified Query Language (UQL)
//
// UQL supports queries across all observability data types with advanced correlation,
// aggregation, and transformation capabilities.
//
// Basic Syntax:
//
//	SELECT <fields> FROM <engine>:<query> [WHERE <conditions>] [WITHIN <time_window> OF <reference_query>]
//
// Advanced Syntax:
//
//	<query> ::= <select_clause> | <correlation_query> | <aggregation_query> | <join_query>
//
// Select Clause:
//
//	SELECT <field_list> FROM <data_source> [WHERE <condition>] [GROUP BY <group_list>] [ORDER BY <order_list>] [LIMIT <n>]
//
// Correlation Query:
//
//	<correlation_query> ::= <query_expression> <correlation_operator> <query_expression> [WITHIN <time_window>]
//
// Aggregation Query:
//
//	<aggregation_query> ::= <aggregate_function>(<field>) FROM <data_source> [WHERE <condition>] [GROUP BY <group_list>]
//
// Join Query:
//
//	<join_query> ::= <query_expression> JOIN <query_expression> ON <join_condition>
//
// Data Sources:
//
//	<data_source> ::= <engine>:<query> | (<subquery>)
//
// Engines:
//
//	<engine> ::= metrics | logs | traces | correlation
//
// Correlation Operators:
//
//	<correlation_operator> ::= AND | OR | WITHIN <time_window> OF | NEAR <time_window> | BEFORE | AFTER
//
// Time Windows:
//
//	<time_window> ::= <number><unit> (e.g., 5m, 1h, 30s, 2d)
//
// Conditions:
//
//	<condition> ::= <field> <operator> <value> [AND|OR <condition>]*
//
// Aggregate Functions:
//
//	<aggregate_function> ::= COUNT | SUM | AVG | MIN | MAX | PERCENTILE_<n> | RATE | INCREASE
//
// Examples:
//   - SELECT service, count(*) FROM logs:error WHERE level='error' GROUP BY service
//   - logs:error WITHIN 5m OF metrics:cpu_usage > 80
//   - SELECT service FROM logs:error JOIN traces:service:error ON service
//   - AVG(response_time) FROM metrics:http_requests WHERE status='200' GROUP BY service
//   - logs:exception NEAR 1m OF traces:status:error
//
// This grammar extends the existing correlation syntax with:
// - SELECT statements for structured queries
// - JOIN operations across engines
// - Aggregation functions
// - Advanced correlation operators (NEAR, BEFORE, AFTER)
// - Subqueries and complex expressions
package models

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// UnifiedQueryLanguage defines the grammar and parser for UQL
type UnifiedQueryLanguage struct {
	grammar *UQLGrammar
	parser  *UQLParser
}

// UQLGrammar defines the grammar rules for Unified Query Language
type UQLGrammar struct {
	// Keywords
	keywords map[string]bool

	// Operators
	operators map[string]UQLOperator

	// Functions
	functions map[string]UQLFunction

	// Time units
	timeUnits map[string]time.Duration
}

// UQLOperator represents query operators
type UQLOperator string

const (
	OpAND    UQLOperator = "AND"
	OpOR     UQLOperator = "OR"
	OpNOT    UQLOperator = "NOT"
	OpEQ     UQLOperator = "="
	OpNEQ    UQLOperator = "!="
	OpLT     UQLOperator = "<"
	OpLE     UQLOperator = "<="
	OpGT     UQLOperator = ">"
	OpGE     UQLOperator = ">="
	OpLIKE   UQLOperator = "LIKE"
	OpMATCH  UQLOperator = "MATCH"
	OpWITHIN UQLOperator = "WITHIN"
	OpNEAR   UQLOperator = "NEAR"
	OpBEFORE UQLOperator = "BEFORE"
	OpAFTER  UQLOperator = "AFTER"
	OpJOIN   UQLOperator = "JOIN"
	OpON     UQLOperator = "ON"
)

// UQLFunction represents built-in functions
type UQLFunction string

const (
	FuncCOUNT      UQLFunction = "COUNT"
	FuncSUM        UQLFunction = "SUM"
	FuncAVG        UQLFunction = "AVG"
	FuncMIN        UQLFunction = "MIN"
	FuncMAX        UQLFunction = "MAX"
	FuncRATE       UQLFunction = "RATE"
	FuncINCREASE   UQLFunction = "INCREASE"
	FuncPERCENTILE UQLFunction = "PERCENTILE"
	FuncHISTOGRAM  UQLFunction = "HISTOGRAM"
	FuncQUANTILE   UQLFunction = "QUANTILE"
)

// UQLEngine represents supported query engines
type UQLEngine string

const (
	EngineMetrics     UQLEngine = "metrics"
	EngineLogs        UQLEngine = "logs"
	EngineTraces      UQLEngine = "traces"
	EngineCorrelation UQLEngine = "correlation"
)

// UQLQuery represents a parsed UQL query
type UQLQuery struct {
	Type        UQLQueryType    `json:"type"`
	RawQuery    string          `json:"raw_query"`
	Select      *UQLSelect      `json:"select,omitempty"`
	Correlation *UQLCorrelation `json:"correlation,omitempty"`
	Aggregation *UQLAggregation `json:"aggregation,omitempty"`
	Join        *UQLJoin        `json:"join,omitempty"`
	TimeWindow  *time.Duration  `json:"time_window,omitempty"`
	Limit       *int            `json:"limit,omitempty"`
	OrderBy     []UQLOrderBy    `json:"order_by,omitempty"`
}

// UQLQueryType represents the type of UQL query
type UQLQueryType string

const (
	UQLQueryTypeSelect      UQLQueryType = "select"
	UQLQueryTypeCorrelation UQLQueryType = "correlation"
	UQLQueryTypeAggregation UQLQueryType = "aggregation"
	UQLQueryTypeJoin        UQLQueryType = "join"
)

// UQLSelect represents a SELECT statement
type UQLSelect struct {
	Fields     []UQLField    `json:"fields"`
	DataSource UQLDataSource `json:"data_source"`
	Where      *UQLCondition `json:"where,omitempty"`
	GroupBy    []string      `json:"group_by,omitempty"`
	Having     *UQLCondition `json:"having,omitempty"`
}

// UQLField represents a field in a SELECT statement
type UQLField struct {
	Name      string      `json:"name"`
	Function  UQLFunction `json:"function,omitempty"`
	Alias     string      `json:"alias,omitempty"`
	Arguments []string    `json:"arguments,omitempty"`
}

// UQLDataSource represents a data source (engine:query)
type UQLDataSource struct {
	Engine UQLEngine `json:"engine"`
	Query  string    `json:"query"`
}

// UQLCorrelation represents a correlation query
type UQLCorrelation struct {
	LeftExpression  UQLExpression  `json:"left_expression"`
	RightExpression UQLExpression  `json:"right_expression"`
	Operator        UQLOperator    `json:"operator"`
	TimeWindow      *time.Duration `json:"time_window,omitempty"`
	JoinCondition   *UQLCondition  `json:"join_condition,omitempty"`
}

// UQLAggregation represents an aggregation query
type UQLAggregation struct {
	Function   UQLFunction   `json:"function"`
	Field      string        `json:"field"`
	DataSource UQLDataSource `json:"data_source"`
	Where      *UQLCondition `json:"where,omitempty"`
	GroupBy    []string      `json:"group_by,omitempty"`
	Arguments  []string      `json:"arguments,omitempty"`
}

// UQLJoin represents a join query
type UQLJoin struct {
	Left       UQLExpression  `json:"left"`
	Right      UQLExpression  `json:"right"`
	JoinType   UQLJoinType    `json:"join_type"`
	Condition  UQLCondition   `json:"condition"`
	TimeWindow *time.Duration `json:"time_window,omitempty"`
}

// UQLJoinType represents join types
type UQLJoinType string

const (
	JoinTypeInner UQLJoinType = "INNER"
	JoinTypeLeft  UQLJoinType = "LEFT"
	JoinTypeRight UQLJoinType = "RIGHT"
	JoinTypeFull  UQLJoinType = "FULL"
	JoinTypeTime  UQLJoinType = "TIME" // Time-based join
)

// UQLExpression represents a query expression
type UQLExpression struct {
	Type        UQLExpressionType `json:"type"`
	DataSource  *UQLDataSource    `json:"data_source,omitempty"`
	Subquery    *UQLQuery         `json:"subquery,omitempty"`
	Correlation *UQLCorrelation   `json:"correlation,omitempty"`
}

// UQLExpressionType represents expression types
type UQLExpressionType string

const (
	ExprTypeDataSource  UQLExpressionType = "data_source"
	ExprTypeSubquery    UQLExpressionType = "subquery"
	ExprTypeCorrelation UQLExpressionType = "correlation"
)

// UQLCondition represents a WHERE/HAVING condition
type UQLCondition struct {
	Field    string        `json:"field"`
	Operator UQLOperator   `json:"operator"`
	Value    interface{}   `json:"value"`
	And      *UQLCondition `json:"and,omitempty"`
	Or       *UQLCondition `json:"or,omitempty"`
}

// UQLOrderBy represents ORDER BY clause
type UQLOrderBy struct {
	Field     string       `json:"field"`
	Direction UQLDirection `json:"direction"`
}

// UQLDirection represents sort direction
type UQLDirection string

const (
	DirASC  UQLDirection = "ASC"
	DirDESC UQLDirection = "DESC"
)

// NewUnifiedQueryLanguage creates a new UQL instance
func NewUnifiedQueryLanguage() *UnifiedQueryLanguage {
	return &UnifiedQueryLanguage{
		grammar: newUQLGrammar(),
		parser:  &UQLParser{},
	}
}

// newUQLGrammar creates the UQL grammar definition
func newUQLGrammar() *UQLGrammar {
	return &UQLGrammar{
		keywords: map[string]bool{
			"SELECT": true, "FROM": true, "WHERE": true, "GROUP": true, "BY": true,
			"ORDER": true, "LIMIT": true, "HAVING": true, "JOIN": true, "ON": true,
			"INNER": true, "LEFT": true, "RIGHT": true, "FULL": true, "WITHIN": true,
			"NEAR": true, "BEFORE": true, "AFTER": true, "AND": true, "OR": true, "NOT": true,
		},
		operators: map[string]UQLOperator{
			"=": OpEQ, "!=": OpNEQ, "<": OpLT, "<=": OpLE, ">": OpGT, ">=": OpGE,
			"LIKE": OpLIKE, "MATCH": OpMATCH, "AND": OpAND, "OR": OpOR, "NOT": OpNOT,
			"WITHIN": OpWITHIN, "NEAR": OpNEAR, "BEFORE": OpBEFORE, "AFTER": OpAFTER,
			"JOIN": OpJOIN, "ON": OpON,
		},
		functions: map[string]UQLFunction{
			"COUNT": FuncCOUNT, "SUM": FuncSUM, "AVG": FuncAVG, "MIN": FuncMIN, "MAX": FuncMAX,
			"RATE": FuncRATE, "INCREASE": FuncINCREASE, "PERCENTILE": FuncPERCENTILE,
			"HISTOGRAM": FuncHISTOGRAM, "QUANTILE": FuncQUANTILE,
		},
		timeUnits: map[string]time.Duration{
			"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour,
		},
	}
}

// UQLParser parses UQL queries
type UQLParser struct{}

// Parse parses a UQL query string
func (p *UQLParser) Parse(query string) (*UQLQuery, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// Determine query type and parse accordingly
	if strings.HasPrefix(strings.ToUpper(query), "SELECT") {
		return p.parseSelectQuery(query)
	} else if p.isCorrelationQuery(query) {
		return p.parseCorrelationQuery(query)
	} else if p.isAggregationQuery(query) {
		return p.parseAggregationQuery(query)
	} else {
		// Default to correlation query for backward compatibility
		return p.parseCorrelationQuery(query)
	}
}

// parseSelectQuery parses a SELECT statement
func (p *UQLParser) parseSelectQuery(query string) (*UQLQuery, error) {
	// Parse SELECT fields FROM datasource WHERE conditions GROUP BY fields ORDER BY fields LIMIT n
	uqlQuery := &UQLQuery{
		Type:     UQLQueryTypeSelect,
		RawQuery: query,
		Select:   &UQLSelect{},
	}

	upperQuery := strings.ToUpper(query)

	// Parse SELECT clause
	selectIndex := strings.Index(upperQuery, "SELECT ")
	if selectIndex == -1 {
		return nil, fmt.Errorf("missing SELECT keyword")
	}

	fromIndex := strings.Index(upperQuery, " FROM ")
	if fromIndex == -1 {
		return nil, fmt.Errorf("missing FROM keyword")
	}

	// Ensure we have valid bounds for field extraction
	fieldStart := selectIndex + 7
	if fieldStart >= fromIndex {
		return nil, fmt.Errorf("missing fields in SELECT clause")
	}

	fieldStr := strings.TrimSpace(query[fieldStart:fromIndex])
	fields, err := p.parseSelectFields(fieldStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SELECT fields: %w", err)
	}
	uqlQuery.Select.Fields = fields

	// Parse FROM clause
	fromEndIndex := p.findClauseEnd(query, fromIndex+6, []string{"WHERE", "GROUP", "ORDER", "LIMIT", "HAVING"})
	fromStr := strings.TrimSpace(query[fromIndex+6 : fromEndIndex])

	dataSource, err := p.parseDataSource(fromStr)
	if err != nil {
		return nil, fmt.Errorf("invalid FROM clause: %w", err)
	}
	uqlQuery.Select.DataSource = *dataSource

	remaining := query[fromEndIndex:]

	// Parse WHERE clause
	if whereIndex := strings.Index(strings.ToUpper(remaining), " WHERE "); whereIndex != -1 {
		whereStart := whereIndex + 7
		if whereStart > len(remaining) {
			return nil, fmt.Errorf("incomplete WHERE clause")
		}
		whereEndIndex := p.findClauseEnd(remaining, whereStart, []string{"GROUP", "ORDER", "LIMIT", "HAVING"})
		whereStr := strings.TrimSpace(remaining[whereStart:whereEndIndex])
		if whereStr == "" {
			return nil, fmt.Errorf("empty WHERE clause")
		}
		condition, err := p.parseCondition(whereStr)
		if err != nil {
			return nil, fmt.Errorf("invalid WHERE clause: %w", err)
		}
		uqlQuery.Select.Where = condition
		remaining = remaining[whereEndIndex:]
	}

	// Parse GROUP BY clause
	if groupIndex := strings.Index(strings.ToUpper(remaining), " GROUP BY "); groupIndex != -1 {
		groupStart := groupIndex + 10
		if groupStart > len(remaining) {
			return nil, fmt.Errorf("incomplete GROUP BY clause")
		}
		groupEndIndex := p.findClauseEnd(remaining, groupStart, []string{"ORDER", "LIMIT", "HAVING"})
		groupStr := strings.TrimSpace(remaining[groupStart:groupEndIndex])
		if groupStr == "" {
			return nil, fmt.Errorf("empty GROUP BY clause")
		}
		groupBy, err := p.parseGroupBy(groupStr)
		if err != nil {
			return nil, fmt.Errorf("invalid GROUP BY clause: %w", err)
		}
		uqlQuery.Select.GroupBy = groupBy
		remaining = remaining[groupEndIndex:]
	}

	// Parse HAVING clause
	if havingIndex := strings.Index(strings.ToUpper(remaining), " HAVING "); havingIndex != -1 {
		havingEndIndex := p.findClauseEnd(remaining, havingIndex+8, []string{"ORDER", "LIMIT"})
		havingStr := strings.TrimSpace(remaining[havingIndex+8 : havingEndIndex])
		having, err := p.parseCondition(havingStr)
		if err != nil {
			return nil, fmt.Errorf("invalid HAVING clause: %w", err)
		}
		uqlQuery.Select.Having = having
		remaining = remaining[havingEndIndex:]
	}

	// Parse ORDER BY clause
	if orderIndex := strings.Index(strings.ToUpper(remaining), " ORDER BY "); orderIndex != -1 {
		orderStart := orderIndex + 10
		if orderStart > len(remaining) {
			return nil, fmt.Errorf("incomplete ORDER BY clause")
		}
		orderEndIndex := p.findClauseEnd(remaining, orderStart, []string{"LIMIT"})
		orderStr := strings.TrimSpace(remaining[orderStart:orderEndIndex])
		if orderStr == "" {
			return nil, fmt.Errorf("empty ORDER BY clause")
		}
		orderBy, err := p.parseOrderBy(orderStr)
		if err != nil {
			return nil, fmt.Errorf("invalid ORDER BY clause: %w", err)
		}
		uqlQuery.OrderBy = orderBy
		remaining = remaining[orderEndIndex:]
	}

	// Parse LIMIT clause
	if limitIndex := strings.Index(strings.ToUpper(remaining), " LIMIT "); limitIndex != -1 {
		limitStart := limitIndex + 7
		if limitStart >= len(remaining) {
			return nil, fmt.Errorf("incomplete LIMIT clause")
		}
		limitStr := strings.TrimSpace(remaining[limitStart:])
		if limitStr == "" {
			return nil, fmt.Errorf("empty LIMIT clause")
		}
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid LIMIT value: %w", err)
		}
		uqlQuery.Limit = &limit
	}

	// Check for incomplete clauses (trailing keywords without proper syntax)
	remaining = strings.TrimSpace(remaining)
	if remaining != "" {
		upperRemaining := strings.ToUpper(remaining)
		if upperRemaining == "WHERE" || upperRemaining == "GROUP" || upperRemaining == "ORDER" || upperRemaining == "LIMIT" || upperRemaining == "HAVING" {
			return nil, fmt.Errorf("incomplete %s clause", upperRemaining)
		}
		// Check for incomplete compound keywords
		if strings.HasPrefix(upperRemaining, "GROUP ") && !strings.Contains(upperRemaining, " BY ") {
			return nil, fmt.Errorf("incomplete GROUP BY clause")
		}
		if strings.HasPrefix(upperRemaining, "ORDER ") && !strings.Contains(upperRemaining, " BY ") {
			return nil, fmt.Errorf("incomplete ORDER BY clause")
		}
		if strings.HasPrefix(upperRemaining, "HAVING ") && len(strings.TrimSpace(strings.TrimPrefix(upperRemaining, "HAVING "))) == 0 {
			return nil, fmt.Errorf("incomplete HAVING clause")
		}
	}

	return uqlQuery, nil
}

// parseSelectFields parses field list in SELECT clause
func (p *UQLParser) parseSelectFields(fieldStr string) ([]UQLField, error) {
	fieldStr = strings.TrimSpace(fieldStr)
	if fieldStr == "" {
		return nil, fmt.Errorf("missing fields in SELECT clause")
	}

	if fieldStr == "*" {
		return []UQLField{{Name: "*"}}, nil
	}

	fields := []UQLField{}
	fieldParts := strings.Split(fieldStr, ",")

	for _, part := range fieldParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		field, err := p.parseField(part)
		if err != nil {
			return nil, err
		}
		fields = append(fields, *field)
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("no valid fields found in SELECT clause")
	}

	return fields, nil
}

// parseField parses a single field with optional function and alias
func (p *UQLParser) parseField(fieldStr string) (*UQLField, error) {
	fieldStr = strings.TrimSpace(fieldStr)

	// Check for alias (field AS alias)
	var alias string
	if asIndex := strings.LastIndex(strings.ToUpper(fieldStr), " AS "); asIndex != -1 {
		alias = strings.TrimSpace(fieldStr[asIndex+4:])
		fieldStr = strings.TrimSpace(fieldStr[:asIndex])
	}

	// Check for function calls
	re := regexp.MustCompile(`^(\w+)\(([^)]*)\)`)
	if matches := re.FindStringSubmatch(fieldStr); len(matches) == 3 {
		function := UQLFunction(strings.ToUpper(matches[1]))
		args := strings.Split(matches[2], ",")
		for i, arg := range args {
			args[i] = strings.TrimSpace(arg)
		}

		return &UQLField{
			Name:      matches[2], // The field inside the function
			Function:  function,
			Alias:     alias,
			Arguments: args,
		}, nil
	}

	// Regular field
	return &UQLField{
		Name:  fieldStr,
		Alias: alias,
	}, nil
}

// parseCondition parses WHERE/HAVING conditions
func (p *UQLParser) parseCondition(condStr string) (*UQLCondition, error) {
	condStr = strings.TrimSpace(condStr)

	// Handle AND/OR logic
	if andIndex := strings.LastIndex(strings.ToUpper(condStr), " AND "); andIndex != -1 {
		leftStr := strings.TrimSpace(condStr[:andIndex])
		rightStr := strings.TrimSpace(condStr[andIndex+5:])

		_, err := p.parseSimpleCondition(leftStr)
		if err != nil {
			return nil, err
		}
		right, err := p.parseCondition(rightStr)
		if err != nil {
			return nil, err
		}

		return &UQLCondition{
			And: right,
			// Left condition is embedded in the And field
		}, nil
	}

	if orIndex := strings.LastIndex(strings.ToUpper(condStr), " OR "); orIndex != -1 {
		leftStr := strings.TrimSpace(condStr[:orIndex])
		rightStr := strings.TrimSpace(condStr[orIndex+4:])

		_, err := p.parseSimpleCondition(leftStr)
		if err != nil {
			return nil, err
		}
		right, err := p.parseCondition(rightStr)
		if err != nil {
			return nil, err
		}

		return &UQLCondition{
			Or: right,
			// Left condition is embedded in the Or field
		}, nil
	}

	// Simple condition
	return p.parseSimpleCondition(condStr)
}

// parseSimpleCondition parses a single condition like "field = value"
func (p *UQLParser) parseSimpleCondition(condStr string) (*UQLCondition, error) {
	// Supported operators in order of specificity (longest first)
	operators := []string{">=", "<=", "!=", "=", ">", "<"}

	for _, op := range operators {
		if opIndex := strings.Index(condStr, op); opIndex != -1 {
			field := strings.TrimSpace(condStr[:opIndex])
			valueStr := strings.TrimSpace(condStr[opIndex+len(op):])

			operator := UQLOperator(op)
			value, err := p.parseValue(valueStr)
			if err != nil {
				return nil, err
			}

			return &UQLCondition{
				Field:    field,
				Operator: operator,
				Value:    value,
			}, nil
		}
	}

	// Check for LIKE and MATCH operators (with spaces)
	if likeIndex := strings.Index(strings.ToUpper(condStr), " LIKE "); likeIndex != -1 {
		field := strings.TrimSpace(condStr[:likeIndex])
		valueStr := strings.TrimSpace(condStr[likeIndex+6:])

		value, err := p.parseValue(valueStr)
		if err != nil {
			return nil, err
		}

		return &UQLCondition{
			Field:    field,
			Operator: OpLIKE,
			Value:    value,
		}, nil
	}

	if matchIndex := strings.Index(strings.ToUpper(condStr), " MATCH "); matchIndex != -1 {
		field := strings.TrimSpace(condStr[:matchIndex])
		valueStr := strings.TrimSpace(condStr[matchIndex+7:])

		value, err := p.parseValue(valueStr)
		if err != nil {
			return nil, err
		}

		return &UQLCondition{
			Field:    field,
			Operator: OpMATCH,
			Value:    value,
		}, nil
	}

	return nil, fmt.Errorf("no valid operator found in condition: %s", condStr)
}

// parseValue parses condition values (strings, numbers, booleans)
func (p *UQLParser) parseValue(valueStr string) (interface{}, error) {
	valueStr = strings.TrimSpace(valueStr)

	// String literals
	if strings.HasPrefix(valueStr, "'") && strings.HasSuffix(valueStr, "'") {
		return valueStr[1 : len(valueStr)-1], nil
	}
	if strings.HasPrefix(valueStr, "\"") && strings.HasSuffix(valueStr, "\"") {
		return valueStr[1 : len(valueStr)-1], nil
	}

	// Numbers
	if intVal, err := strconv.Atoi(valueStr); err == nil {
		return intVal, nil
	}
	if floatVal, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return floatVal, nil
	}

	// Booleans
	if strings.ToLower(valueStr) == "true" {
		return true, nil
	}
	if strings.ToLower(valueStr) == "false" {
		return false, nil
	}

	// Default to string
	return valueStr, nil
}

// parseGroupBy parses GROUP BY clause
func (p *UQLParser) parseGroupBy(groupStr string) ([]string, error) {
	fields := []string{}
	fieldParts := strings.Split(groupStr, ",")

	for _, part := range fieldParts {
		field := strings.TrimSpace(part)
		if field != "" {
			fields = append(fields, field)
		}
	}

	return fields, nil
}

// parseOrderBy parses ORDER BY clause
func (p *UQLParser) parseOrderBy(orderStr string) ([]UQLOrderBy, error) {
	orderBy := []UQLOrderBy{}
	parts := strings.Split(orderStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for direction
		var field, direction string
		if strings.HasSuffix(strings.ToUpper(part), " DESC") {
			field = strings.TrimSpace(part[:len(part)-5])
			direction = "DESC"
		} else if strings.HasSuffix(strings.ToUpper(part), " ASC") {
			field = strings.TrimSpace(part[:len(part)-4])
			direction = "ASC"
		} else {
			field = part
			direction = "ASC" // Default
		}

		orderBy = append(orderBy, UQLOrderBy{
			Field:     field,
			Direction: UQLDirection(direction),
		})
	}

	return orderBy, nil
}

// findClauseEnd finds the end of a clause before the next keyword
func (p *UQLParser) findClauseEnd(query string, start int, keywords []string) int {
	query = query[start:]
	upperQuery := strings.ToUpper(query)

	minIndex := len(query)
	for _, keyword := range keywords {
		// Check for keyword with spaces around it
		if idx := strings.Index(upperQuery, " "+keyword+" "); idx != -1 {
			if idx < minIndex {
				minIndex = idx
			}
		}
		// Also check for keyword at the end (followed by end of string)
		keywordWithSpace := " " + keyword
		if strings.HasSuffix(upperQuery, keywordWithSpace) {
			idx := len(upperQuery) - len(keywordWithSpace)
			if idx < minIndex {
				minIndex = idx
			}
		}
	}

	if minIndex == len(query) {
		return start + len(query)
	}

	return start + minIndex
}

// parseCorrelationExpressions parses left and right expressions for correlation
func (p *UQLParser) parseCorrelationExpressions(left, right string) (*UQLExpression, *UQLExpression, error) {
	leftExpr, err := p.parseSingleExpression(left)
	if err != nil {
		return nil, nil, err
	}
	rightExpr, err := p.parseSingleExpression(right)
	if err != nil {
		return nil, nil, err
	}
	return leftExpr, rightExpr, nil
}

// parseSingleExpression parses a single engine:query expression
func (p *UQLParser) parseSingleExpression(expr string) (*UQLExpression, error) {
	expr = strings.TrimSpace(expr)

	// Check for engine prefix
	engine, query, found := strings.Cut(expr, ":")
	if !found {
		return nil, fmt.Errorf("missing engine prefix in expression: %s", expr)
	}

	var uqlEngine UQLEngine
	switch strings.ToLower(engine) {
	case "metrics":
		uqlEngine = EngineMetrics
	case "logs":
		uqlEngine = EngineLogs
	case "traces":
		uqlEngine = EngineTraces
	case "correlation":
		uqlEngine = EngineCorrelation
	default:
		return nil, fmt.Errorf("unknown engine: %s", engine)
	}

	return &UQLExpression{
		Type: ExprTypeDataSource,
		DataSource: &UQLDataSource{
			Engine: uqlEngine,
			Query:  query,
		},
	}, nil
}

// parseDataSource parses engine:query format
func (p *UQLParser) parseDataSource(ds string) (*UQLDataSource, error) {
	parts := strings.SplitN(ds, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid data source format: %s", ds)
	}

	if strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("empty query part in data source: %s", ds)
	}

	var uqlEngine UQLEngine
	switch strings.ToLower(parts[0]) {
	case "metrics":
		uqlEngine = EngineMetrics
	case "logs":
		uqlEngine = EngineLogs
	case "traces":
		uqlEngine = EngineTraces
	case "correlation":
		uqlEngine = EngineCorrelation
	default:
		return nil, fmt.Errorf("unknown engine: %s", parts[0])
	}

	return &UQLDataSource{
		Engine: uqlEngine,
		Query:  parts[1],
	}, nil
}

// parseTimeWindow parses time window expressions like "5m", "1h", "30s"
func (p *UQLParser) parseTimeWindow(input string) (time.Duration, string, error) {
	input = strings.TrimSpace(input)

	re := regexp.MustCompile(`^(\d+)([smhd])\b`)
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

	remainder := strings.TrimSpace(input[len(matches[0]):])
	return duration, remainder, nil
}

// isCorrelationQuery checks if query is a correlation query
func (p *UQLParser) isCorrelationQuery(query string) bool {
	query = strings.ToUpper(query)
	return strings.Contains(query, " AND ") ||
		strings.Contains(query, " OR ") ||
		strings.Contains(query, " WITHIN ") ||
		strings.Contains(query, " NEAR ") ||
		strings.Contains(query, " BEFORE ") ||
		strings.Contains(query, " AFTER ")
}

// isAggregationQuery checks if query is an aggregation query
func (p *UQLParser) isAggregationQuery(query string) bool {
	re := regexp.MustCompile(`^\w+\([^)]*\)\s+FROM\s+`)
	return re.MatchString(query)
}

// parseCorrelationQuery parses a correlation query
func (p *UQLParser) parseCorrelationQuery(query string) (*UQLQuery, error) {
	// Enhanced correlation parsing with new operators
	// Parse correlation operators: WITHIN, NEAR, BEFORE, AFTER
	if withinIndex := strings.Index(strings.ToUpper(query), " WITHIN "); withinIndex != -1 {
		return p.parseTimeWindowCorrelation(query, withinIndex, OpWITHIN)
	} else if nearIndex := strings.Index(strings.ToUpper(query), " NEAR "); nearIndex != -1 {
		return p.parseTimeWindowCorrelation(query, nearIndex, OpNEAR)
	} else if beforeIndex := strings.Index(strings.ToUpper(query), " BEFORE "); beforeIndex != -1 {
		return p.parseTimeWindowCorrelation(query, beforeIndex, OpBEFORE)
	} else if afterIndex := strings.Index(strings.ToUpper(query), " AFTER "); afterIndex != -1 {
		return p.parseTimeWindowCorrelation(query, afterIndex, OpAFTER)
	}

	// Parse simple AND/OR correlations
	return p.parseSimpleCorrelation(query)
}

// parseTimeWindowCorrelation parses time-window based correlations
func (p *UQLParser) parseTimeWindowCorrelation(query string, opIndex int, operator UQLOperator) (*UQLQuery, error) {
	beforeOp := query[:opIndex]
	afterOp := query[opIndex+len(" "+string(operator)+" "):]

	// Parse time window
	timeWindow, remainder, err := p.parseTimeWindow(afterOp)
	if err != nil {
		return nil, fmt.Errorf("invalid time window: %w", err)
	}

	// Parse expressions
	leftExpr, rightExpr, err := p.parseCorrelationExpressions(beforeOp, remainder)
	if err != nil {
		return nil, err
	}

	return &UQLQuery{
		Type:     UQLQueryTypeCorrelation,
		RawQuery: query,
		Correlation: &UQLCorrelation{
			LeftExpression:  *leftExpr,
			RightExpression: *rightExpr,
			Operator:        operator,
			TimeWindow:      &timeWindow,
		},
	}, nil
}

// parseSimpleCorrelation parses AND/OR correlations
func (p *UQLParser) parseSimpleCorrelation(query string) (*UQLQuery, error) {
	// Check for OR operator
	if orIndex := strings.Index(strings.ToUpper(query), " OR "); orIndex != -1 {
		leftPart := query[:orIndex]
		rightPart := query[orIndex+4:]

		leftExpr, err := p.parseSingleExpression(leftPart)
		if err != nil {
			return nil, err
		}
		rightExpr, err := p.parseSingleExpression(rightPart)
		if err != nil {
			return nil, err
		}

		return &UQLQuery{
			Type:     UQLQueryTypeCorrelation,
			RawQuery: query,
			Correlation: &UQLCorrelation{
				LeftExpression:  *leftExpr,
				RightExpression: *rightExpr,
				Operator:        OpOR,
			},
		}, nil
	}

	// Check for AND operator
	if andIndex := strings.Index(strings.ToUpper(query), " AND "); andIndex != -1 {
		leftPart := query[:andIndex]
		rightPart := query[andIndex+5:]

		leftExpr, err := p.parseSingleExpression(leftPart)
		if err != nil {
			return nil, err
		}
		rightExpr, err := p.parseSingleExpression(rightPart)
		if err != nil {
			return nil, err
		}

		return &UQLQuery{
			Type:     UQLQueryTypeCorrelation,
			RawQuery: query,
			Correlation: &UQLCorrelation{
				LeftExpression:  *leftExpr,
				RightExpression: *rightExpr,
				Operator:        OpAND,
			},
		}, nil
	}

	// Single expression
	expr, err := p.parseSingleExpression(query)
	if err != nil {
		return nil, err
	}

	return &UQLQuery{
		Type:     UQLQueryTypeCorrelation,
		RawQuery: query,
		Correlation: &UQLCorrelation{
			LeftExpression: *expr,
		},
	}, nil
}

// parseAggregationQuery parses aggregation queries
func (p *UQLParser) parseAggregationQuery(query string) (*UQLQuery, error) {
	// Parse patterns like: COUNT(*) FROM logs:error
	// AVG(response_time) FROM metrics:http_requests
	re := regexp.MustCompile(`^(\w+)\(([^)]*)\)\s+FROM\s+(.+)$`)
	matches := re.FindStringSubmatch(query)
	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid aggregation query format")
	}

	function := UQLFunction(strings.ToUpper(matches[1]))
	field := matches[2]
	fromPart := matches[3]

	// Validate function
	grammar := newUQLGrammar()
	if _, exists := grammar.functions[string(function)]; !exists {
		return nil, fmt.Errorf("unknown aggregation function: %s", function)
	}

	// Validate field is not empty for functions that require it
	if field == "" && function != FuncCOUNT {
		return nil, fmt.Errorf("aggregation function %s requires a field", function)
	}

	dataSource, err := p.parseDataSource(fromPart)
	if err != nil {
		return nil, err
	}

	return &UQLQuery{
		Type:     UQLQueryTypeAggregation,
		RawQuery: query,
		Aggregation: &UQLAggregation{
			Function:   function,
			Field:      field,
			DataSource: *dataSource,
		},
	}, nil
}

// Validate validates a UQL query
func (q *UQLQuery) Validate() error {
	if q.RawQuery == "" {
		return fmt.Errorf("empty query")
	}

	switch q.Type {
	case UQLQueryTypeSelect:
		return q.validateSelect()
	case UQLQueryTypeCorrelation:
		return q.validateCorrelation()
	case UQLQueryTypeAggregation:
		return q.validateAggregation()
	case UQLQueryTypeJoin:
		return q.validateJoin()
	default:
		return fmt.Errorf("unknown query type: %s", q.Type)
	}
}

// validateSelect validates a SELECT query
func (q *UQLQuery) validateSelect() error {
	if q.Select == nil {
		return fmt.Errorf("missing select clause")
	}
	if len(q.Select.Fields) == 0 {
		return fmt.Errorf("select must have at least one field")
	}
	if q.Select.DataSource.Engine == "" {
		return fmt.Errorf("missing data source engine")
	}
	return nil
}

// validateCorrelation validates a correlation query
func (q *UQLQuery) validateCorrelation() error {
	if q.Correlation == nil {
		return fmt.Errorf("missing correlation clause")
	}
	if q.Correlation.LeftExpression.Type == "" {
		return fmt.Errorf("missing left expression in correlation")
	}
	if q.Correlation.RightExpression.Type == "" {
		return fmt.Errorf("missing right expression in correlation")
	}
	return nil
}

// validateAggregation validates an aggregation query
func (q *UQLQuery) validateAggregation() error {
	if q.Aggregation == nil {
		return fmt.Errorf("missing aggregation clause")
	}
	if q.Aggregation.Function == "" {
		return fmt.Errorf("missing aggregation function")
	}
	if q.Aggregation.DataSource.Engine == "" {
		return fmt.Errorf("missing data source engine")
	}
	return nil
}

// validateJoin validates a join query
func (q *UQLQuery) validateJoin() error {
	if q.Join == nil {
		return fmt.Errorf("missing join clause")
	}
	if q.Join.Left.Type == "" {
		return fmt.Errorf("missing left expression in join")
	}
	if q.Join.Right.Type == "" {
		return fmt.Errorf("missing right expression in join")
	}
	return nil
}

// String returns a string representation of the UQL query
func (q *UQLQuery) String() string {
	return q.RawQuery
}

// NewUQLParser creates a new UQL parser instance
func NewUQLParser() *UQLParser {
	return &UQLParser{}
}

// UQLExamples provides example UQL queries
var UQLExamples = []string{
	// Basic correlations (backward compatible)
	"logs:error AND metrics:high_latency",
	"logs:exception WITHIN 5m OF metrics:cpu_usage > 80",

	// Advanced correlations
	"logs:error NEAR 1m OF traces:status:error",
	"logs:timeout BEFORE 30s OF metrics:response_time > 5000",

	// SELECT queries
	"SELECT service, level, count(*) FROM logs:error WHERE level='error' GROUP BY service, level",
	"SELECT service, avg(response_time) FROM metrics:http_requests WHERE status='200' GROUP BY service",

	// Aggregation queries
	"COUNT(*) FROM logs:error WHERE level='error'",
	"AVG(response_time) FROM metrics:http_requests GROUP BY service",
	"PERCENTILE_95(response_time) FROM metrics:http_requests",

	// Join queries
	"logs:error JOIN traces:service:error ON service WITHIN 5m",
	"metrics:http_requests > 1000 JOIN logs:error ON service NEAR 1m",
}
