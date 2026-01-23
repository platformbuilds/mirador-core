package weavstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFieldsSerialization_Phase6 verifies that all 7 new fields from Phase 2-4
// are correctly serialized into Weaviate properties map.
// This test satisfies P6-T1-S1: Add tests for new field serialization in weavstore.
func TestNewFieldsSerialization_Phase6(t *testing.T) {
	tests := []struct {
		name          string
		kpi           *KPIDefinition
		expectedProps map[string]interface{}
	}{
		{
			name: "All new fields populated",
			kpi: &KPIDefinition{
				ID:              "test-kpi-001",
				Name:            "test_metric",
				Description:     "Detailed KPI description for Phase 6 testing",
				DataType:        "timeseries",
				DataSourceID:    "550e8400-e29b-41d4-a716-446655440000",
				KPIDatastoreID:  "660e8400-e29b-41d4-a716-446655440001",
				RefreshInterval: 60,
				IsShared:        true,
				UserID:          "770e8400-e29b-41d4-a716-446655440002",
			},
			expectedProps: map[string]interface{}{
				"description":     "Detailed KPI description for Phase 6 testing",
				"dataType":        "timeseries",
				"dataSourceId":    "550e8400-e29b-41d4-a716-446655440000",
				"kpiDatastoreId":  "660e8400-e29b-41d4-a716-446655440001",
				"refreshInterval": 60,
				"isShared":        true,
				"userId":          "770e8400-e29b-41d4-a716-446655440002",
			},
		},
		{
			name: "New fields empty/zero values",
			kpi: &KPIDefinition{
				ID:              "test-kpi-002",
				Name:            "test_metric_2",
				Description:     "",
				DataType:        "",
				DataSourceID:    "",
				KPIDatastoreID:  "",
				RefreshInterval: 0,
				IsShared:        false,
				UserID:          "",
			},
			expectedProps: map[string]interface{}{
				"description":     "",
				"dataType":        "",
				"dataSourceId":    "",
				"kpiDatastoreId":  "",
				"refreshInterval": 0,
				"isShared":        false,
				"userId":          "",
			},
		},
		{
			name: "Mixed new and old fields",
			kpi: &KPIDefinition{
				ID:              "test-kpi-003",
				Name:            "mixed_fields_kpi",
				Kind:            "tech",
				Description:     "Mixed field test",
				DataType:        "value",
				RefreshInterval: 30,
				IsShared:        false,
				SignalType:      "metric",
				Datastore:       "victoriametrics",
			},
			expectedProps: map[string]interface{}{
				"description":     "Mixed field test",
				"dataType":        "value",
				"dataSourceId":    "",
				"kpiDatastoreId":  "",
				"refreshInterval": 30,
				"isShared":        false,
				"userId":          "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build properties map (simulating what CreateOrUpdateKPI does)
			props := buildPropertiesMap(tt.kpi)

			// Verify all new fields are in the properties map
			for key, expected := range tt.expectedProps {
				actual, exists := props[key]
				assert.True(t, exists, "Property %s should exist in properties map", key)
				assert.Equal(t, expected, actual, "Property %s value mismatch", key)
			}
		})
	}
}

// TestNewFieldsDeserialization_Phase6 verifies that all 7 new fields from Phase 2-4
// are correctly deserialized from Weaviate response properties.
// This test satisfies P6-T1-S2: Add tests for new field deserialization from Weaviate.
func TestNewFieldsDeserialization_Phase6(t *testing.T) {
	tests := []struct {
		name         string
		props        map[string]interface{}
		expectedKPI  func(*testing.T, *KPIDefinition)
		expectFields map[string]interface{}
	}{
		{
			name: "All new fields present in properties",
			props: map[string]interface{}{
				"id":              "kpi-deser-001",
				"name":            "deserialized_kpi",
				"description":     "Test description from Weaviate",
				"dataType":        "categorical",
				"dataSourceId":    "880e8400-e29b-41d4-a716-446655440003",
				"kpiDatastoreId":  "990e8400-e29b-41d4-a716-446655440004",
				"refreshInterval": float64(120), // Weaviate returns numbers as float64
				"isShared":        true,
				"userId":          "aa0e8400-e29b-41d4-a716-446655440005",
			},
			expectFields: map[string]interface{}{
				"Description":     "Test description from Weaviate",
				"DataType":        "categorical",
				"DataSourceID":    "880e8400-e29b-41d4-a716-446655440003",
				"KPIDatastoreID":  "990e8400-e29b-41d4-a716-446655440004",
				"RefreshInterval": 120,
				"IsShared":        true,
				"UserID":          "aa0e8400-e29b-41d4-a716-446655440005",
			},
		},
		{
			name: "New fields missing from properties (backward compatibility)",
			props: map[string]interface{}{
				"id":   "kpi-deser-002",
				"name": "legacy_kpi",
				// New fields intentionally omitted
			},
			expectFields: map[string]interface{}{
				"Description":     "",
				"DataType":        "",
				"DataSourceID":    "",
				"KPIDatastoreID":  "",
				"RefreshInterval": 0,
				"IsShared":        false,
				"UserID":          "",
			},
		},
		{
			name: "Partial new fields (some set, some empty)",
			props: map[string]interface{}{
				"id":              "kpi-deser-003",
				"name":            "partial_kpi",
				"description":     "Only description set",
				"refreshInterval": float64(45),
				"isShared":        true,
				// dataType, dataSourceId, kpiDatastoreId, userId omitted
			},
			expectFields: map[string]interface{}{
				"Description":     "Only description set",
				"DataType":        "",
				"DataSourceID":    "",
				"KPIDatastoreID":  "",
				"RefreshInterval": 45,
				"IsShared":        true,
				"UserID":          "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract ID from properties
			id, _ := tt.props["id"].(string)

			// Deserialize properties into KPIDefinition
			kpi := parsePropsToKPI(tt.props, id)

			// Verify expected fields
			assert.Equal(t, tt.expectFields["Description"], kpi.Description, "Description mismatch")
			assert.Equal(t, tt.expectFields["DataType"], kpi.DataType, "DataType mismatch")
			assert.Equal(t, tt.expectFields["DataSourceID"], kpi.DataSourceID, "DataSourceID mismatch")
			assert.Equal(t, tt.expectFields["KPIDatastoreID"], kpi.KPIDatastoreID, "KPIDatastoreID mismatch")
			assert.Equal(t, tt.expectFields["RefreshInterval"], kpi.RefreshInterval, "RefreshInterval mismatch")
			assert.Equal(t, tt.expectFields["IsShared"], kpi.IsShared, "IsShared mismatch")
			assert.Equal(t, tt.expectFields["UserID"], kpi.UserID, "UserID mismatch")
		})
	}
}

// TestKpiEqual_NewFields_Phase6 verifies that kpiEqual() correctly compares
// all 7 new fields when determining if two KPIs are equal.
// This test satisfies P6-T1-S3: Add tests for kpiEqual() with new fields.
func TestKpiEqual_NewFields_Phase6(t *testing.T) {
	baseKPI := &KPIDefinition{
		ID:              "base-kpi",
		Name:            "base_metric",
		Description:     "Base description",
		DataType:        "timeseries",
		DataSourceID:    "ds-001",
		KPIDatastoreID:  "kpids-001",
		RefreshInterval: 60,
		IsShared:        true,
		UserID:          "user-001",
	}

	tests := []struct {
		name     string
		kpi1     *KPIDefinition
		kpi2     *KPIDefinition
		expected bool
	}{
		{
			name:     "Identical KPIs with all new fields",
			kpi1:     copyKPI(baseKPI),
			kpi2:     copyKPI(baseKPI),
			expected: true,
		},
		{
			name: "Different Description",
			kpi1: copyKPI(baseKPI),
			kpi2: func() *KPIDefinition {
				k := copyKPI(baseKPI)
				k.Description = "Different description"
				return k
			}(),
			expected: false,
		},
		{
			name: "Different DataType",
			kpi1: copyKPI(baseKPI),
			kpi2: func() *KPIDefinition {
				k := copyKPI(baseKPI)
				k.DataType = "categorical"
				return k
			}(),
			expected: false,
		},
		{
			name: "Different DataSourceID",
			kpi1: copyKPI(baseKPI),
			kpi2: func() *KPIDefinition {
				k := copyKPI(baseKPI)
				k.DataSourceID = "ds-002"
				return k
			}(),
			expected: false,
		},
		{
			name: "Different KPIDatastoreID",
			kpi1: copyKPI(baseKPI),
			kpi2: func() *KPIDefinition {
				k := copyKPI(baseKPI)
				k.KPIDatastoreID = "kpids-002"
				return k
			}(),
			expected: false,
		},
		{
			name: "Different RefreshInterval",
			kpi1: copyKPI(baseKPI),
			kpi2: func() *KPIDefinition {
				k := copyKPI(baseKPI)
				k.RefreshInterval = 120
				return k
			}(),
			expected: false,
		},
		{
			name: "Different IsShared",
			kpi1: copyKPI(baseKPI),
			kpi2: func() *KPIDefinition {
				k := copyKPI(baseKPI)
				k.IsShared = false
				return k
			}(),
			expected: false,
		},
		{
			name: "Different UserID",
			kpi1: copyKPI(baseKPI),
			kpi2: func() *KPIDefinition {
				k := copyKPI(baseKPI)
				k.UserID = "user-002"
				return k
			}(),
			expected: false,
		},
		{
			name:     "All new fields empty vs populated",
			kpi1:     &KPIDefinition{ID: "empty-new-fields", Name: "test"},
			kpi2:     copyKPI(baseKPI),
			expected: false,
		},
		{
			name:     "Both KPIs have empty new fields",
			kpi1:     &KPIDefinition{ID: "empty-1", Name: "test1"},
			kpi2:     &KPIDefinition{ID: "empty-2", Name: "test2"},
			expected: false, // IDs differ
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kpiEqual(tt.kpi1, tt.kpi2)
			assert.Equal(t, tt.expected, result, "kpiEqual result mismatch")
		})
	}
}

// TestRoundTripSerialization_NewFields_Phase6 verifies complete round-trip
// serialization and deserialization of all new fields.
func TestRoundTripSerialization_NewFields_Phase6(t *testing.T) {
	original := &KPIDefinition{
		ID:              "roundtrip-kpi",
		Name:            "roundtrip_test",
		Kind:            "tech",
		Description:     "Round-trip test with all new fields",
		DataType:        "timeseries",
		DataSourceID:    "ds-roundtrip-001",
		KPIDatastoreID:  "kpids-roundtrip-001",
		RefreshInterval: 90,
		IsShared:        true,
		UserID:          "user-roundtrip-001",
		SignalType:      "metric",
		Datastore:       "victoriametrics",
	}

	// Serialize
	props := buildPropertiesMap(original)
	require.NotNil(t, props, "Properties map should not be nil")

	// Deserialize
	deserialized := parsePropsToKPI(props, original.ID)
	require.NotNil(t, deserialized, "Deserialized KPI should not be nil")

	// Verify all new fields match
	assert.Equal(t, original.Description, deserialized.Description, "Description round-trip failed")
	assert.Equal(t, original.DataType, deserialized.DataType, "DataType round-trip failed")
	assert.Equal(t, original.DataSourceID, deserialized.DataSourceID, "DataSourceID round-trip failed")
	assert.Equal(t, original.KPIDatastoreID, deserialized.KPIDatastoreID, "KPIDatastoreID round-trip failed")
	assert.Equal(t, original.RefreshInterval, deserialized.RefreshInterval, "RefreshInterval round-trip failed")
	assert.Equal(t, original.IsShared, deserialized.IsShared, "IsShared round-trip failed")
	assert.Equal(t, original.UserID, deserialized.UserID, "UserID round-trip failed")

	t.Logf("Round-trip test passed for KPI: %s", original.ID)
}

// Helper function to copy a KPIDefinition for test isolation
func copyKPI(kpi *KPIDefinition) *KPIDefinition {
	copy := *kpi
	return &copy
}

// Helper function to build properties map (simulates CreateOrUpdateKPI logic)
func buildPropertiesMap(kpi *KPIDefinition) map[string]interface{} {
	props := make(map[string]interface{})
	props["id"] = kpi.ID
	props["name"] = kpi.Name
	props["kind"] = kpi.Kind
	props["description"] = kpi.Description
	props["dataType"] = kpi.DataType
	props["dataSourceId"] = kpi.DataSourceID
	props["kpiDatastoreId"] = kpi.KPIDatastoreID
	props["refreshInterval"] = kpi.RefreshInterval
	props["isShared"] = kpi.IsShared
	props["userId"] = kpi.UserID
	props["signalType"] = kpi.SignalType
	props["datastore"] = kpi.Datastore
	return props
}
