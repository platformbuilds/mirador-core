package services

import (
	"testing"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// BenchmarkUQLQueryParsing benchmarks UQL query parsing performance
func BenchmarkUQLQueryParsing(b *testing.B) {
	parser := models.NewUQLParser()

	testQueries := []string{
		"SELECT service, level FROM logs:error WHERE level='error'",
		"SELECT * FROM logs:error WHERE level='error' AND service='api'",
		"COUNT(*) FROM logs:error",
		"SUM(bytes) FROM logs:error WHERE status_code >= 500",
		"logs:error AND metrics:cpu_usage > 80",
		"SELECT service FROM traces:auth WHERE duration > 1000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, query := range testQueries {
			_, err := parser.Parse(query)
			if err != nil {
				b.Fatalf("Failed to parse query %s: %v", query, err)
			}
		}
	}
}

// BenchmarkUQLOptimization benchmarks UQL query optimization performance
func BenchmarkUQLOptimization(b *testing.B) {
	parser := models.NewUQLParser()
	optimizer := NewUQLOptimizer(logger.New("info"))

	testQueries := []string{
		"SELECT service, level FROM logs:error WHERE level='error'",
		"SELECT * FROM logs:error WHERE level='error' AND service='api'",
		"COUNT(*) FROM logs:error",
		"SUM(bytes) FROM logs:error WHERE status_code >= 500",
		"logs:error AND metrics:cpu_usage > 80",
	}

	// Pre-parse queries
	parsedQueries := make([]*models.UQLQuery, len(testQueries))
	for i, query := range testQueries {
		parsed, err := parser.Parse(query)
		if err != nil {
			b.Fatalf("Failed to parse query %s: %v", query, err)
		}
		parsedQueries[i] = parsed
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, parsed := range parsedQueries {
			_, err := optimizer.Optimize(parsed)
			if err != nil {
				b.Fatalf("Failed to optimize query: %v", err)
			}
		}
	}
}

// BenchmarkUQLTranslation benchmarks UQL query translation performance
func BenchmarkUQLTranslation(b *testing.B) {
	parser := models.NewUQLParser()
	optimizer := NewUQLOptimizer(logger.New("info"))
	translator := NewUQLTranslatorRegistry(logger.New("info"))

	testQueries := []string{
		"SELECT service, level FROM logs:error WHERE level='error'",
		"SELECT * FROM logs:error WHERE level='error' AND service='api'",
		"COUNT(*) FROM logs:error",
		"SUM(bytes) FROM logs:error WHERE status_code >= 500",
		"logs:error AND metrics:cpu_usage > 80",
	}

	// Pre-parse and optimize queries
	optimizedQueries := make([]*models.UQLQuery, len(testQueries))
	for i, query := range testQueries {
		parsed, err := parser.Parse(query)
		if err != nil {
			b.Fatalf("Failed to parse query %s: %v", query, err)
		}
		optimized, err := optimizer.Optimize(parsed)
		if err != nil {
			b.Fatalf("Failed to optimize query %s: %v", query, err)
		}
		optimizedQueries[i] = optimized
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, optimized := range optimizedQueries {
			_, err := translator.TranslateQuery(optimized)
			if err != nil {
				b.Fatalf("Failed to translate query: %v", err)
			}
		}
	}
}

// BenchmarkUQLFullPipeline benchmarks the complete UQL pipeline (parse → optimize → translate)
func BenchmarkUQLFullPipeline(b *testing.B) {
	logger := logger.New("info")
	parser := models.NewUQLParser()
	optimizer := NewUQLOptimizer(logger)
	translator := NewUQLTranslatorRegistry(logger)

	testQueries := []string{
		"SELECT service, level FROM logs:error WHERE level='error'",
		"SELECT * FROM logs:error WHERE level='error' AND service='api'",
		"COUNT(*) FROM logs:error",
		"SUM(bytes) FROM logs:error WHERE status_code >= 500",
		"logs:error AND metrics:cpu_usage > 80",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, query := range testQueries {
			// Parse
			parsed, err := parser.Parse(query)
			if err != nil {
				b.Fatalf("Failed to parse query %s: %v", query, err)
			}

			// Optimize
			optimized, err := optimizer.Optimize(parsed)
			if err != nil {
				b.Fatalf("Failed to optimize query %s: %v", query, err)
			}

			// Translate
			_, err = translator.TranslateQuery(optimized)
			if err != nil {
				b.Fatalf("Failed to translate query %s: %v", query, err)
			}
		}
	}
}

// BenchmarkUQLQueryRouting benchmarks query routing performance
func BenchmarkUQLQueryRouting(b *testing.B) {
	router := NewQueryRouter(logger.New("info"))

	testQueries := []string{
		"SELECT service, level FROM logs:error WHERE level='error'",
		"SELECT * FROM logs:error WHERE level='error' AND service='api'",
		"COUNT(*) FROM logs:error",
		"SUM(bytes) FROM logs:error WHERE status_code >= 500",
		"logs:error AND metrics:cpu_usage > 80",
		"up",           // metrics query
		"error",        // logs query
		"service:auth", // traces query
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, query := range testQueries {
			_, _, err := router.RouteQuery(&models.UnifiedQuery{
				Query: query,
			})
			if err != nil {
				b.Fatalf("Failed to route query %s: %v", query, err)
			}
		}
	}
}

// BenchmarkUQLVsDirectQuery benchmarks UQL query vs direct engine query performance
func BenchmarkUQLVsDirectQuery(b *testing.B) {
	logger := logger.New("info")

	// Setup mock services for comparison
	// Note: This is a simplified benchmark focusing on parsing/translation overhead

	testCases := []struct {
		name        string
		uqlQuery    string
		directQuery string
		engine      models.UQLEngine
	}{
		{
			name:        "SELECT_logs_error",
			uqlQuery:    "SELECT service, level FROM logs:error WHERE level='error'",
			directQuery: `error | level:"error"`,
			engine:      models.EngineLogs,
		},
		{
			name:        "COUNT_logs_error",
			uqlQuery:    "COUNT(*) FROM logs:error",
			directQuery: "error | count(*)",
			engine:      models.EngineLogs,
		},
		{
			name:        "SELECT_traces_auth",
			uqlQuery:    "SELECT service FROM traces:auth WHERE duration > 1000",
			directQuery: `{service.name="auth"} && duration>1000`,
			engine:      models.EngineTraces,
		},
	}

	for _, tc := range testCases {
		b.Run("UQL_"+tc.name, func(b *testing.B) {
			parser := models.NewUQLParser()
			optimizer := NewUQLOptimizer(logger)
			translator := NewUQLTranslatorRegistry(logger)

			for i := 0; i < b.N; i++ {
				// Parse UQL
				parsed, _ := parser.Parse(tc.uqlQuery)
				// Optimize
				optimized, _ := optimizer.Optimize(parsed)
				// Translate
				translated, _ := translator.TranslateQuery(optimized)

				// Verify translation matches expected
				if translated.Query != tc.directQuery {
					b.Errorf("Translation mismatch: expected %s, got %s", tc.directQuery, translated.Query)
				}
			}
		})

		b.Run("Direct_"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Simulate direct query processing (just string operations)
				query := tc.directQuery
				_ = len(query) // Simulate basic processing
			}
		})
	}
}

// BenchmarkUQLMemoryUsage benchmarks memory usage of UQL processing
func BenchmarkUQLMemoryUsage(b *testing.B) {
	logger := logger.New("info")
	parser := models.NewUQLParser()
	optimizer := NewUQLOptimizer(logger)
	translator := NewUQLTranslatorRegistry(logger)

	query := "SELECT service, level, message FROM logs:error WHERE level='error' AND service='api' AND timestamp > '2024-01-01'"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Parse
		parsed, err := parser.Parse(query)
		if err != nil {
			b.Fatalf("Failed to parse query: %v", err)
		}

		// Optimize
		optimized, err := optimizer.Optimize(parsed)
		if err != nil {
			b.Fatalf("Failed to optimize query: %v", err)
		}

		// Translate
		_, err = translator.TranslateQuery(optimized)
		if err != nil {
			b.Fatalf("Failed to translate query: %v", err)
		}
	}
}

// BenchmarkUQLConcurrentProcessing benchmarks concurrent UQL query processing
func BenchmarkUQLConcurrentProcessing(b *testing.B) {
	logger := logger.New("info")

	testQueries := []string{
		"SELECT service, level FROM logs:error WHERE level='error'",
		"COUNT(*) FROM logs:error",
		"SUM(bytes) FROM logs:error WHERE status_code >= 500",
		"logs:error AND metrics:cpu_usage > 80",
	}

	b.RunParallel(func(pb *testing.PB) {
		localParser := models.NewUQLParser()
		localOptimizer := NewUQLOptimizer(logger)
		localTranslator := NewUQLTranslatorRegistry(logger)

		for pb.Next() {
			for _, query := range testQueries {
				// Parse
				parsed, _ := localParser.Parse(query)
				// Optimize
				optimized, _ := localOptimizer.Optimize(parsed)
				// Translate
				_, _ = localTranslator.TranslateQuery(optimized)
			}
		}
	})
}

// BenchmarkUQLQueryValidation benchmarks query validation performance
func BenchmarkUQLQueryValidation(b *testing.B) {
	testQueries := []*models.UQLQuery{
		{
			Type:     models.UQLQueryTypeSelect,
			RawQuery: "SELECT service FROM logs:error",
			Select: &models.UQLSelect{
				Fields: []models.UQLField{{Name: "service"}},
				DataSource: models.UQLDataSource{
					Engine: models.EngineLogs,
					Query:  "error",
				},
			},
		},
		{
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
		},
		{
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
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, query := range testQueries {
			err := query.Validate()
			if err != nil {
				b.Fatalf("Query validation failed: %v", err)
			}
		}
	}
}
