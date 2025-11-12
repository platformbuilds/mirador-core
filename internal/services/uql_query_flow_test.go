package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestUQLQueryFlowIntegration(t *testing.T) {
	// Test UQL query parsing, optimization, and translation flow
	logger := logger.New("info")

	t.Run("UQL SELECT Query Parsing and Translation", func(t *testing.T) {
		// Test parsing
		parser := models.NewUQLParser()
		query := "SELECT service, level FROM logs:error WHERE level='error'"

		parsed, err := parser.Parse(query)
		assert.NoError(t, err)
		assert.NotNil(t, parsed)
		assert.Equal(t, models.UQLQueryTypeSelect, parsed.Type)
		assert.NotNil(t, parsed.Select)
		assert.Equal(t, "logs", string(parsed.Select.DataSource.Engine))
		assert.Equal(t, "error", parsed.Select.DataSource.Query)

		// Test optimization
		optimizer := NewUQLOptimizer(logger)
		optimized, err := optimizer.Optimize(parsed)
		assert.NoError(t, err)
		assert.NotNil(t, optimized)

		// Test translation
		translator := NewUQLTranslatorRegistry(logger)
		translated, err := translator.TranslateQuery(optimized)
		assert.NoError(t, err)
		assert.NotNil(t, translated)
		assert.Equal(t, models.EngineLogs, translated.Engine)
		assert.Equal(t, `error | level:"error"`, translated.Query) // Corrected expectation
	})

	t.Run("UQL Correlation Query Parsing and Translation", func(t *testing.T) {
		// Test parsing
		parser := models.NewUQLParser()
		query := "logs:error AND metrics:cpu_usage > 80"

		parsed, err := parser.Parse(query)
		assert.NoError(t, err)
		assert.NotNil(t, parsed)
		assert.Equal(t, models.UQLQueryTypeCorrelation, parsed.Type)
		assert.NotNil(t, parsed.Correlation)

		// Test optimization
		optimizer := NewUQLOptimizer(logger)
		optimized, err := optimizer.Optimize(parsed)
		assert.NoError(t, err)
		assert.NotNil(t, optimized)

		// Test translation
		translator := NewUQLTranslatorRegistry(logger)
		translated, err := translator.TranslateQuery(optimized)
		assert.NoError(t, err)
		assert.NotNil(t, translated)
		assert.Equal(t, models.EngineCorrelation, translated.Engine)
		assert.Equal(t, "logs:error AND metrics:cpu_usage > 80", translated.Query)
	})

	t.Run("UQL Aggregation Query Parsing and Translation", func(t *testing.T) {
		// Test parsing
		parser := models.NewUQLParser()
		query := "COUNT(*) FROM logs:error"

		parsed, err := parser.Parse(query)
		assert.NoError(t, err)
		assert.NotNil(t, parsed)
		assert.Equal(t, models.UQLQueryTypeAggregation, parsed.Type)
		assert.NotNil(t, parsed.Aggregation)
		assert.Equal(t, models.FuncCOUNT, parsed.Aggregation.Function)

		// Test optimization
		optimizer := NewUQLOptimizer(logger)
		optimized, err := optimizer.Optimize(parsed)
		assert.NoError(t, err)
		assert.NotNil(t, optimized)

		// Test translation
		translator := NewUQLTranslatorRegistry(logger)
		translated, err := translator.TranslateQuery(optimized)
		assert.NoError(t, err)
		assert.NotNil(t, translated)
		assert.Equal(t, models.EngineLogs, translated.Engine)
		assert.Equal(t, "error | count(*)", translated.Query) // Corrected expectation
	})

	t.Run("UQL Query Routing Detection", func(t *testing.T) {
		router := NewQueryRouter(logger)

		// Test UQL SELECT query detection
		queryType, reason, err := router.RouteQuery(&models.UnifiedQuery{
			Query: "SELECT * FROM logs:error",
		})
		assert.NoError(t, err)
		assert.Equal(t, models.QueryTypeMetrics, queryType) // UQL queries route to metrics for processing
		assert.Contains(t, reason, "UQL query syntax")

		// Test regular metrics query
		queryType, reason, err = router.RouteQuery(&models.UnifiedQuery{
			Query: "up",
		})
		assert.NoError(t, err)
		assert.Equal(t, models.QueryTypeMetrics, queryType)
		assert.Contains(t, reason, "metrics patterns")

		// Test logs query
		queryType, reason, err = router.RouteQuery(&models.UnifiedQuery{
			Query: "error",
		})
		assert.NoError(t, err)
		assert.Equal(t, models.QueryTypeTraces, queryType) // "error" matches traces patterns
	})

	t.Run("UQL Query Validation", func(t *testing.T) {
		// Test valid SELECT query
		query := &models.UQLQuery{
			Type:     models.UQLQueryTypeSelect,
			RawQuery: "SELECT service FROM logs:error",
			Select: &models.UQLSelect{
				Fields: []models.UQLField{{Name: "service"}},
				DataSource: models.UQLDataSource{
					Engine: models.EngineLogs,
					Query:  "error",
				},
			},
		}

		err := query.Validate()
		assert.NoError(t, err)

		// Test invalid query
		invalidQuery := &models.UQLQuery{
			Type:     models.UQLQueryTypeSelect,
			RawQuery: "SELECT FROM logs:error",
			Select:   &models.UQLSelect{}, // Missing fields
		}

		err = invalidQuery.Validate()
		assert.Error(t, err)
	})
}
