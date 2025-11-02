package services

import (
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestUQLParser(t *testing.T) {
	parser := models.NewUQLParser()

	t.Run("Parse SELECT query", func(t *testing.T) {
		query := "SELECT service, level FROM logs:error WHERE level='error'"

		parsed, err := parser.Parse(query)

		assert.NoError(t, err)
		assert.NotNil(t, parsed)
		assert.Equal(t, models.UQLQueryTypeSelect, parsed.Type)
		assert.NotNil(t, parsed.Select)
		assert.Equal(t, "logs", string(parsed.Select.DataSource.Engine))
		assert.Equal(t, "error", parsed.Select.DataSource.Query)
	})

	t.Run("Parse correlation query", func(t *testing.T) {
		query := "logs:error AND metrics:cpu_usage > 80"

		parsed, err := parser.Parse(query)

		assert.NoError(t, err)
		assert.NotNil(t, parsed)
		assert.Equal(t, models.UQLQueryTypeCorrelation, parsed.Type)
		assert.NotNil(t, parsed.Correlation)
	})

	t.Run("Parse aggregation query", func(t *testing.T) {
		query := "COUNT(*) FROM logs:error"

		parsed, err := parser.Parse(query)

		assert.NoError(t, err)
		assert.NotNil(t, parsed)
		assert.Equal(t, models.UQLQueryTypeAggregation, parsed.Type)
		assert.NotNil(t, parsed.Aggregation)
		assert.Equal(t, models.FuncCOUNT, parsed.Aggregation.Function)
	})
}

func TestUQLOptimizer(t *testing.T) {
	logger := logger.New("info")
	optimizer := NewUQLOptimizer(logger)

	t.Run("Optimize SELECT Query", func(t *testing.T) {
		// Create a test UQL query
		query := &models.UQLQuery{
			Type:     models.UQLQueryTypeSelect,
			RawQuery: "SELECT service, level, count(*) FROM logs:error WHERE level='error' GROUP BY service, level",
			Select: &models.UQLSelect{
				Fields: []models.UQLField{
					{Name: "service"},
					{Name: "level"},
					{Name: "*", Function: models.FuncCOUNT},
				},
				DataSource: models.UQLDataSource{
					Engine: models.EngineLogs,
					Query:  "error",
				},
				Where: &models.UQLCondition{
					Field:    "level",
					Operator: models.OpEQ,
					Value:    "error",
				},
				GroupBy: []string{"service", "level"},
			},
		}

		optimized, err := optimizer.Optimize(query)

		assert.NoError(t, err)
		assert.NotNil(t, optimized)
		assert.Equal(t, models.UQLQueryTypeSelect, optimized.Type)
		assert.NotNil(t, optimized.Select)
	})

	t.Run("Optimize Correlation Query", func(t *testing.T) {
		query := &models.UQLQuery{
			Type:     models.UQLQueryTypeCorrelation,
			RawQuery: "logs:error WITHIN 5m OF metrics:cpu_usage > 80",
			Correlation: &models.UQLCorrelation{
				LeftExpression: models.UQLExpression{
					Type: models.ExprTypeDataSource,
					DataSource: &models.UQLDataSource{
						Engine: models.EngineLogs,
						Query:  "error",
					},
				},
				RightExpression: models.UQLExpression{
					Type: models.ExprTypeDataSource,
					DataSource: &models.UQLDataSource{
						Engine: models.EngineMetrics,
						Query:  "cpu_usage > 80",
					},
				},
				Operator:   models.OpWITHIN,
				TimeWindow: &[]time.Duration{5 * time.Minute}[0],
			},
		}

		optimized, err := optimizer.Optimize(query)

		assert.NoError(t, err)
		assert.NotNil(t, optimized)
		assert.Equal(t, models.UQLQueryTypeCorrelation, optimized.Type)
		assert.NotNil(t, optimized.Correlation)
	})
}

func TestUQLTranslator(t *testing.T) {
	logger := logger.New("info")
	translator := NewUQLTranslatorRegistry(logger)

	t.Run("Translate PromQL Query", func(t *testing.T) {
		query := &models.UQLQuery{
			Type:     models.UQLQueryTypeSelect,
			RawQuery: "SELECT * FROM metrics:up",
			Select: &models.UQLSelect{
				Fields: []models.UQLField{{Name: "*"}},
				DataSource: models.UQLDataSource{
					Engine: models.EngineMetrics,
					Query:  "up",
				},
			},
		}

		translated, err := translator.TranslateQuery(query)

		assert.NoError(t, err)
		assert.NotNil(t, translated)
		assert.Equal(t, models.EngineMetrics, translated.Engine)
		assert.Equal(t, "up", translated.Query)
	})

	t.Run("Translate LogsQL Query", func(t *testing.T) {
		query := &models.UQLQuery{
			Type:     models.UQLQueryTypeSelect,
			RawQuery: "SELECT * FROM logs:error",
			Select: &models.UQLSelect{
				Fields: []models.UQLField{{Name: "*"}},
				DataSource: models.UQLDataSource{
					Engine: models.EngineLogs,
					Query:  "error",
				},
			},
		}

		translated, err := translator.TranslateQuery(query)

		assert.NoError(t, err)
		assert.NotNil(t, translated)
		assert.Equal(t, models.EngineLogs, translated.Engine)
		assert.Equal(t, "error", translated.Query)
	})

	t.Run("Translate Correlation Query", func(t *testing.T) {
		query := &models.UQLQuery{
			Type:     models.UQLQueryTypeCorrelation,
			RawQuery: "logs:error AND metrics:cpu_usage > 80",
			Correlation: &models.UQLCorrelation{
				LeftExpression: models.UQLExpression{
					Type: models.ExprTypeDataSource,
					DataSource: &models.UQLDataSource{
						Engine: models.EngineLogs,
						Query:  "error",
					},
				},
				RightExpression: models.UQLExpression{
					Type: models.ExprTypeDataSource,
					DataSource: &models.UQLDataSource{
						Engine: models.EngineMetrics,
						Query:  "cpu_usage > 80",
					},
				},
				Operator: models.OpAND,
			},
		}

		translated, err := translator.TranslateQuery(query)

		assert.NoError(t, err)
		assert.NotNil(t, translated)
		assert.Equal(t, models.EngineCorrelation, translated.Engine)
		assert.Equal(t, "logs:error AND metrics:cpu_usage > 80", translated.Query)
	})
}

func TestUQLQueryValidation(t *testing.T) {
	t.Run("Validate SELECT query", func(t *testing.T) {
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
	})

	t.Run("Validate invalid SELECT query", func(t *testing.T) {
		query := &models.UQLQuery{
			Type:     models.UQLQueryTypeSelect,
			RawQuery: "SELECT FROM logs:error",
			Select:   &models.UQLSelect{}, // Missing fields and data source
		}

		err := query.Validate()

		assert.Error(t, err)
	})

	t.Run("Validate correlation query", func(t *testing.T) {
		query := &models.UQLQuery{
			Type:     models.UQLQueryTypeCorrelation,
			RawQuery: "logs:error AND metrics:cpu_usage > 80",
			Correlation: &models.UQLCorrelation{
				LeftExpression: models.UQLExpression{
					Type: models.ExprTypeDataSource,
					DataSource: &models.UQLDataSource{
						Engine: models.EngineLogs,
						Query:  "error",
					},
				},
				RightExpression: models.UQLExpression{
					Type: models.ExprTypeDataSource,
					DataSource: &models.UQLDataSource{
						Engine: models.EngineMetrics,
						Query:  "cpu",
					},
				},
				Operator: models.OpAND,
			},
		}

		err := query.Validate()

		assert.NoError(t, err)
	})
}
