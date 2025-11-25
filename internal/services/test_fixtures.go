package services

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// TestFixtures holds centralized test data loaded from testdata/fixtures.yaml
// NOTE(HCB-005): Use these fixtures instead of hardcoding metric/service names in tests.
type TestFixtures struct {
	TestMetrics      []TestMetric         `yaml:"test_metrics"`
	TestServices     []TestService        `yaml:"test_services"`
	ServiceMappings  map[string]string    `yaml:"service_mappings"`
	TestQueries      []TestQuery          `yaml:"test_queries"`
	TestLabels       TestLabels           `yaml:"test_labels"`
	TestLabelValues  TestLabelValues      `yaml:"test_label_values"`
	TestTimeRanges   map[string]TimeRange `yaml:"test_time_ranges"`
	TestThresholds   TestThresholds       `yaml:"test_thresholds"`
	TestTransactions []TestTransaction    `yaml:"test_transactions"`
	TestFailureModes []TestFailureMode    `yaml:"test_failure_modes"`
}

type TestMetric struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Type        string  `yaml:"type"`
	Value       float64 `yaml:"value"`
	Threshold   float64 `yaml:"threshold"`
	Unit        string  `yaml:"unit"`
}

type TestService struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

type TestQuery struct {
	Name        string `yaml:"name"`
	Pattern     string `yaml:"pattern"`
	Description string `yaml:"description"`
}

type TestLabels struct {
	ServiceLabels   []string `yaml:"service_labels"`
	PodLabels       []string `yaml:"pod_labels"`
	NamespaceLabels []string `yaml:"namespace_labels"`
}

type TestLabelValues struct {
	Services   []string `yaml:"services"`
	Namespaces []string `yaml:"namespaces"`
	Levels     []string `yaml:"levels"`
}

type TimeRange struct {
	Duration    string `yaml:"duration"`
	Description string `yaml:"description"`
}

type TestThresholds struct {
	Correlation ThresholdValues `yaml:"correlation"`
	Anomaly     ThresholdValues `yaml:"anomaly"`
	Confidence  ThresholdValues `yaml:"confidence"`
}

type ThresholdValues struct {
	Min  float64 `yaml:"min"`
	High float64 `yaml:"high"`
}

type TestTransaction struct {
	ID        string `yaml:"id"`
	Status    string `yaml:"status"`
	Component string `yaml:"component"`
}

type TestFailureMode struct {
	Component   string `yaml:"component"`
	FailureType string `yaml:"failure_type"`
}

var (
	fixturesOnce     sync.Once
	fixturesInstance *TestFixtures
	fixturesError    error
)

// LoadTestFixtures loads the centralized test fixtures from testdata/fixtures.yaml
// Uses sync.Once to load only once per test run (cached singleton).
func LoadTestFixtures() (*TestFixtures, error) {
	fixturesOnce.Do(func() {
		// Find fixtures.yaml relative to this file
		fixturesPath := filepath.Join("testdata", "fixtures.yaml")

		data, err := os.ReadFile(fixturesPath)
		if err != nil {
			fixturesError = err
			return
		}

		var fixtures TestFixtures
		if err := yaml.Unmarshal(data, &fixtures); err != nil {
			fixturesError = err
			return
		}

		fixturesInstance = &fixtures
	})

	return fixturesInstance, fixturesError
}

// GetTestMetric returns a test metric by name, or the first metric if name is empty.
func (f *TestFixtures) GetTestMetric(name string) *TestMetric {
	if name == "" && len(f.TestMetrics) > 0 {
		return &f.TestMetrics[0]
	}
	for i := range f.TestMetrics {
		if f.TestMetrics[i].Name == name {
			return &f.TestMetrics[i]
		}
	}
	return nil
}

// GetTestService returns a test service by name, or the first service if name is empty.
func (f *TestFixtures) GetTestService(name string) *TestService {
	if name == "" && len(f.TestServices) > 0 {
		return &f.TestServices[0]
	}
	for i := range f.TestServices {
		if f.TestServices[i].Name == name {
			return &f.TestServices[i]
		}
	}
	return nil
}

// GetTestQuery returns a test query pattern by name, or the first query if name is empty.
func (f *TestFixtures) GetTestQuery(name string) *TestQuery {
	if name == "" && len(f.TestQueries) > 0 {
		return &f.TestQueries[0]
	}
	for i := range f.TestQueries {
		if f.TestQueries[i].Name == name {
			return &f.TestQueries[i]
		}
	}
	return nil
}
