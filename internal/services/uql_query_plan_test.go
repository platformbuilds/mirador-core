package services

import (
	"strings"
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestQueryPlanAnalysis(t *testing.T) {
	logger := logger.New("info")
	optimizer := NewUQLOptimizer(logger)

	t.Run("SELECT Query Plan", func(t *testing.T) {
		selectQuery := &models.UQLQuery{
			Type:     models.UQLQueryTypeSelect,
			RawQuery: "SELECT service, level FROM logs:error WHERE level = 'error'",
			Select: &models.UQLSelect{
				Fields: []models.UQLField{
					{Name: "service"},
					{Name: "level"},
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
			},
		}

		plan, err := optimizer.GenerateQueryPlan(selectQuery)
		if err != nil {
			t.Fatalf("Failed to generate SELECT plan: %v", err)
		}

		if len(plan.Steps) == 0 {
			t.Error("SELECT plan should have execution steps")
		}

		if plan.QueryType != models.UQLQueryTypeSelect {
			t.Errorf("Expected query type SELECT, got %s", plan.QueryType)
		}

		if len(plan.DataSources) == 0 {
			t.Error("SELECT plan should have data sources")
		}

		explanation, err := optimizer.ExplainQuery(selectQuery)
		if err != nil {
			t.Fatalf("Failed to explain SELECT query: %v", err)
		}

		if explanation == "" {
			t.Error("SELECT explanation should not be empty")
		}

		// Check that explanation contains expected content
		if !strings.Contains(explanation, "Query Plan") {
			t.Error("Explanation should contain 'Query Plan'")
		}

		if !strings.Contains(explanation, "Execution Steps") {
			t.Error("Explanation should contain 'Execution Steps'")
		}
	})

	t.Run("Correlation Query Plan", func(t *testing.T) {
		timeWindow := 5 * time.Minute
		corrQuery := &models.UQLQuery{
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
				TimeWindow: &timeWindow,
			},
		}

		plan, err := optimizer.GenerateQueryPlan(corrQuery)
		if err != nil {
			t.Fatalf("Failed to generate correlation plan: %v", err)
		}

		if len(plan.Steps) < 3 {
			t.Errorf("Correlation plan should have at least 3 steps, got %d", len(plan.Steps))
		}

		if len(plan.DataSources) < 2 {
			t.Errorf("Correlation plan should have at least 2 data sources, got %d", len(plan.DataSources))
		}

		explanation, err := optimizer.ExplainQuery(corrQuery)
		if err != nil {
			t.Fatalf("Failed to explain correlation query: %v", err)
		}

		if !strings.Contains(explanation, "Correlation") {
			t.Error("Correlation explanation should contain 'Correlation'")
		}

		if !strings.Contains(explanation, "WITHIN") {
			t.Error("Correlation explanation should contain 'WITHIN'")
		}
	})

	t.Run("Aggregation Query Plan", func(t *testing.T) {
		aggQuery := &models.UQLQuery{
			Type:     models.UQLQueryTypeAggregation,
			RawQuery: "COUNT(*) FROM logs:error",
			Aggregation: &models.UQLAggregation{
				Function: models.FuncCOUNT,
				Field:    "*",
				DataSource: models.UQLDataSource{
					Engine: models.EngineLogs,
					Query:  "error",
				},
			},
		}

		plan, err := optimizer.GenerateQueryPlan(aggQuery)
		if err != nil {
			t.Fatalf("Failed to generate aggregation plan: %v", err)
		}

		if len(plan.Steps) == 0 {
			t.Error("Aggregation plan should have execution steps")
		}

		explanation, err := optimizer.ExplainQuery(aggQuery)
		if err != nil {
			t.Fatalf("Failed to explain aggregation query: %v", err)
		}

		if !strings.Contains(explanation, "Aggregation") {
			t.Error("Aggregation explanation should contain 'Aggregation'")
		}
	})
}