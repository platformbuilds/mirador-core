package models

import (
	"encoding/json"
	"testing"
	"time"
)

// TestKPIDefinition_NewFieldsSerialization tests that the 7 new fields from mirador-ui
// (Description, DataType, DataSourceID, KPIDatastoreID, RefreshInterval, IsShared, UserID)
// are correctly serialized and deserialized with JSON.
func TestKPIDefinition_NewFieldsSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	original := &KPIDefinition{
		ID:              "kpi-new-fields-test",
		Kind:            "business",
		Name:            "Test KPI with New Fields",
		Namespace:       "test",
		Source:          "unit-test",
		SourceID:        "test-001",
		Unit:            "count",
		Format:          "integer",
		Query:           map[string]interface{}{"metric": "test_metric"},
		Layer:           "impact",
		SignalType:      "metrics",
		Classifier:      "tps",
		Datastore:       "victoriametrics",
		QueryType:       "MetricsQL",
		Formula:         "sum(rate(requests_total[5m]))",
		Thresholds:      []Threshold{{Level: "critical", Operator: ">", Value: 100.0}},
		Tags:            []string{"test", "new-fields"},
		Definition:      "Test definition for new fields validation",
		Sentiment:       "NEGATIVE",
		Category:        "performance",
		RetryAllowed:    true,
		Domain:          "payments",
		ServiceFamily:   "api-gateway",
		ComponentType:   "springboot",
		BusinessImpact:  "High impact on customer experience",
		EmotionalImpact: "Critical",
		Examples:        []map[string]interface{}{{"value": 42, "timestamp": "2025-01-20T00:00:00Z"}},
		Sparkline:       map[string]interface{}{"type": "line", "points": []int{1, 2, 3}},
		Visibility:      "org",
		// New fields from mirador-ui integration
		Description:     "Detailed description of the KPI for enhanced search and discovery",
		DataType:        "timeseries",
		DataSourceID:    "ds-12345678-1234-1234-1234-123456789abc",
		KPIDatastoreID:  "kpids-87654321-4321-4321-4321-210987654321",
		RefreshInterval: 60,
		IsShared:        true,
		UserID:          "user-abcdef12-3456-7890-abcd-ef1234567890",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal KPIDefinition: %v", err)
	}

	// Verify JSON contains new fields
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	expectedFields := map[string]interface{}{
		"description":     "Detailed description of the KPI for enhanced search and discovery",
		"dataType":        "timeseries",
		"dataSourceId":    "ds-12345678-1234-1234-1234-123456789abc",
		"kpiDatastoreId":  "kpids-87654321-4321-4321-4321-210987654321",
		"refreshInterval": float64(60), // JSON numbers are float64
		"isShared":        true,
		"userId":          "user-abcdef12-3456-7890-abcd-ef1234567890",
	}

	for fieldName, expectedValue := range expectedFields {
		actualValue, ok := jsonMap[fieldName]
		if !ok {
			t.Errorf("Field %s not found in JSON output", fieldName)
			continue
		}
		// Special handling for numeric comparison (JSON unmarshals numbers as float64)
		if expectedFloat, ok := expectedValue.(float64); ok {
			if actualFloat, ok := actualValue.(float64); ok {
				if actualFloat != expectedFloat {
					t.Errorf("Field %s: expected %v, got %v", fieldName, expectedValue, actualValue)
				}
			} else {
				t.Errorf("Field %s: expected float64, got %T", fieldName, actualValue)
			}
		} else if actualValue != expectedValue {
			t.Errorf("Field %s: expected %v, got %v", fieldName, expectedValue, actualValue)
		}
	}

	// Deserialize back to struct
	var deserialized KPIDefinition
	if err := json.Unmarshal(data, &deserialized); err != nil {
		t.Fatalf("Failed to unmarshal KPIDefinition: %v", err)
	}

	// Verify new fields match
	if deserialized.Description != original.Description {
		t.Errorf("Description: expected %s, got %s", original.Description, deserialized.Description)
	}
	if deserialized.DataType != original.DataType {
		t.Errorf("DataType: expected %s, got %s", original.DataType, deserialized.DataType)
	}
	if deserialized.DataSourceID != original.DataSourceID {
		t.Errorf("DataSourceID: expected %s, got %s", original.DataSourceID, deserialized.DataSourceID)
	}
	if deserialized.KPIDatastoreID != original.KPIDatastoreID {
		t.Errorf("KPIDatastoreID: expected %s, got %s", original.KPIDatastoreID, deserialized.KPIDatastoreID)
	}
	if deserialized.RefreshInterval != original.RefreshInterval {
		t.Errorf("RefreshInterval: expected %d, got %d", original.RefreshInterval, deserialized.RefreshInterval)
	}
	if deserialized.IsShared != original.IsShared {
		t.Errorf("IsShared: expected %v, got %v", original.IsShared, deserialized.IsShared)
	}
	if deserialized.UserID != original.UserID {
		t.Errorf("UserID: expected %s, got %s", original.UserID, deserialized.UserID)
	}
}

// TestKPIDefinition_NewFieldsOmitempty tests that omitempty tag works correctly
// for the new optional fields (they should not appear in JSON when empty/zero).
func TestKPIDefinition_NewFieldsOmitempty(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	// Create KPI without new fields (only required fields)
	minimal := &KPIDefinition{
		ID:         "kpi-minimal",
		Kind:       "tech",
		Name:       "Minimal KPI",
		Unit:       "count",
		Format:     "integer",
		Query:      map[string]interface{}{"metric": "test"},
		Thresholds: []Threshold{},
		Tags:       []string{},
		Definition: "Minimal definition",
		Sentiment:  "NEUTRAL",
		Sparkline:  map[string]interface{}{},
		Visibility: "private",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Serialize to JSON
	data, err := json.Marshal(minimal)
	if err != nil {
		t.Fatalf("Failed to marshal minimal KPIDefinition: %v", err)
	}

	// Verify JSON does NOT contain new fields (due to omitempty)
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	omittedFields := []string{"description", "dataType", "dataSourceId", "kpiDatastoreId", "userId"}
	for _, fieldName := range omittedFields {
		if _, ok := jsonMap[fieldName]; ok {
			t.Errorf("Field %s should be omitted (omitempty) when empty, but was present in JSON", fieldName)
		}
	}

	// RefreshInterval is an int (zero value = 0), should be omitted when 0
	if _, ok := jsonMap["refreshInterval"]; ok {
		t.Errorf("Field refreshInterval should be omitted when 0")
	}

	// IsShared is a bool (zero value = false), should be omitted when false
	if _, ok := jsonMap["isShared"]; ok {
		t.Errorf("Field isShared should be omitted when false")
	}
}

// TestKPIDefinition_BackwardCompatibility tests that old KPI payloads without
// new fields can still be deserialized successfully.
func TestKPIDefinition_BackwardCompatibility(t *testing.T) {
	// JSON payload without new fields (simulates existing KPI from before migration)
	legacyJSON := `{
		"id": "kpi-legacy",
		"kind": "business",
		"name": "Legacy KPI",
		"unit": "ms",
		"format": "float",
		"query": {"metric": "response_time"},
		"thresholds": [{"level": "warning", "operator": ">", "value": 500}],
		"tags": ["legacy"],
		"definition": "Legacy definition",
		"sentiment": "NEGATIVE",
		"sparkline": {},
		"visibility": "team",
		"createdAt": "2025-01-01T00:00:00Z",
		"updatedAt": "2025-01-01T00:00:00Z"
	}`

	var kpi KPIDefinition
	if err := json.Unmarshal([]byte(legacyJSON), &kpi); err != nil {
		t.Fatalf("Failed to unmarshal legacy KPI: %v", err)
	}

	// Verify required fields were parsed
	if kpi.ID != "kpi-legacy" {
		t.Errorf("ID: expected kpi-legacy, got %s", kpi.ID)
	}
	if kpi.Name != "Legacy KPI" {
		t.Errorf("Name: expected Legacy KPI, got %s", kpi.Name)
	}

	// Verify new fields have zero values (not set in legacy JSON)
	if kpi.Description != "" {
		t.Errorf("Description should be empty for legacy KPI, got %s", kpi.Description)
	}
	if kpi.DataType != "" {
		t.Errorf("DataType should be empty for legacy KPI, got %s", kpi.DataType)
	}
	if kpi.DataSourceID != "" {
		t.Errorf("DataSourceID should be empty for legacy KPI, got %s", kpi.DataSourceID)
	}
	if kpi.KPIDatastoreID != "" {
		t.Errorf("KPIDatastoreID should be empty for legacy KPI, got %s", kpi.KPIDatastoreID)
	}
	if kpi.RefreshInterval != 0 {
		t.Errorf("RefreshInterval should be 0 for legacy KPI, got %d", kpi.RefreshInterval)
	}
	if kpi.IsShared != false {
		t.Errorf("IsShared should be false for legacy KPI, got %v", kpi.IsShared)
	}
	if kpi.UserID != "" {
		t.Errorf("UserID should be empty for legacy KPI, got %s", kpi.UserID)
	}
}
