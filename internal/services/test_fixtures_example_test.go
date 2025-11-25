package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFixturesLoading demonstrates how to use centralized test fixtures
// NOTE(HCB-006): This test shows the pattern for using fixtures instead of hardcoded strings.
// All other tests should be migrated to this pattern over time.
func TestFixturesLoading(t *testing.T) {
	fixtures, err := LoadTestFixtures()
	require.NoError(t, err, "Failed to load test fixtures")
	require.NotNil(t, fixtures, "Fixtures should not be nil")

	t.Run("Test Metrics Available", func(t *testing.T) {
		assert.NotEmpty(t, fixtures.TestMetrics, "Should have test metrics")

		// Get specific metric by name
		cpuMetric := fixtures.GetTestMetric("test_cpu_metric")
		require.NotNil(t, cpuMetric, "CPU metric should exist")
		assert.Equal(t, "test_cpu_metric", cpuMetric.Name)
		assert.Equal(t, 85.5, cpuMetric.Value)
		assert.Equal(t, 80.0, cpuMetric.Threshold)

		// Get first metric (default)
		defaultMetric := fixtures.GetTestMetric("")
		assert.NotNil(t, defaultMetric)
	})

	t.Run("Test Services Available", func(t *testing.T) {
		assert.NotEmpty(t, fixtures.TestServices, "Should have test services")

		// Get specific service
		serviceA := fixtures.GetTestService("test-service-a")
		require.NotNil(t, serviceA, "Service A should exist")
		assert.Equal(t, "test-service-a", serviceA.Name)
		assert.Equal(t, "api", serviceA.Type)
	})

	t.Run("Test Queries Available", func(t *testing.T) {
		assert.NotEmpty(t, fixtures.TestQueries, "Should have test queries")

		// Get specific query pattern
		basicQuery := fixtures.GetTestQuery("basic_log_and_metric")
		require.NotNil(t, basicQuery, "Basic query should exist")
		assert.Contains(t, basicQuery.Pattern, "logs:error")
		assert.Contains(t, basicQuery.Pattern, "test_cpu_metric")
	})

	t.Run("Service Mappings Available", func(t *testing.T) {
		assert.NotEmpty(t, fixtures.ServiceMappings, "Should have service mappings")

		// Verify specific mapping
		mapped := fixtures.ServiceMappings["test-service-a-client"]
		assert.Equal(t, "test-service-a", mapped)
	})

	t.Run("Test Labels Available", func(t *testing.T) {
		assert.NotEmpty(t, fixtures.TestLabels.ServiceLabels, "Should have service labels")
		assert.NotEmpty(t, fixtures.TestLabels.PodLabels, "Should have pod labels")
		assert.NotEmpty(t, fixtures.TestLabels.NamespaceLabels, "Should have namespace labels")
	})

	t.Run("Test Thresholds Available", func(t *testing.T) {
		assert.Greater(t, fixtures.TestThresholds.Correlation.Min, 0.0)
		assert.Greater(t, fixtures.TestThresholds.Anomaly.Min, 0.0)
		assert.Greater(t, fixtures.TestThresholds.Confidence.Min, 0.0)
	})
}

// TestFixturesUsageExample demonstrates how to use fixtures in actual tests
// NOTE(HCB-006): Copy this pattern when refactoring existing tests.
func TestFixturesUsageExample(t *testing.T) {
	fixtures, err := LoadTestFixtures()
	if err != nil {
		t.Skipf("Fixtures not available: %v", err)
	}

	t.Run("Example: Using Metric Fixture Instead of Hardcoded String", func(t *testing.T) {
		// OLD WAY (violates HCB-005/HCB-006):
		// metricName := "cpu_usage"  // WRONG: hardcoded

		// NEW WAY (correct):
		testMetric := fixtures.GetTestMetric("test_cpu_metric")
		metricName := testMetric.Name

		// Use the metric name in your test logic
		assert.Equal(t, "test_cpu_metric", metricName)
		assert.NotEmpty(t, metricName)
	})

	t.Run("Example: Using Service Fixture Instead of Hardcoded String", func(t *testing.T) {
		// OLD WAY (violates HCB-005/HCB-006):
		// serviceName := "kafka-producer"  // WRONG: hardcoded

		// NEW WAY (correct):
		testService := fixtures.GetTestService("test-queue-service")
		serviceName := testService.Name

		// Use the service name in your test logic
		assert.Equal(t, "test-queue-service", serviceName)
		assert.Equal(t, "queue", testService.Type)
	})

	t.Run("Example: Using Query Pattern Instead of Hardcoded String", func(t *testing.T) {
		// OLD WAY (violates HCB-005/HCB-006):
		// queryPattern := "logs:error AND metrics:cpu_usage > 80"  // WRONG: hardcoded

		// NEW WAY (correct):
		testQuery := fixtures.GetTestQuery("basic_log_and_metric")
		queryPattern := testQuery.Pattern

		// Use the query pattern in your test logic
		assert.Contains(t, queryPattern, "logs:error")
		assert.Contains(t, queryPattern, "metrics")
	})
}
