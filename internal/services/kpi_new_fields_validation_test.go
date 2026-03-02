package services

import (
	"strings"
	"testing"

	"github.com/mirastacklabs-ai/mirador-core/internal/models"
)

// TestValidateKPIDefinition_DataType tests DataType field validation (P3-T1-S1)
func TestValidateKPIDefinition_DataType(t *testing.T) {
	cfg := makeTestConfig()

	tests := []struct {
		name      string
		dataType  string
		wantError bool
		errMsg    string
	}{
		{
			name:      "valid_timeseries",
			dataType:  "timeseries",
			wantError: false,
		},
		{
			name:      "valid_value",
			dataType:  "value",
			wantError: false,
		},
		{
			name:      "valid_categorical",
			dataType:  "categorical",
			wantError: false,
		},
		{
			name:      "valid_empty", // optional field
			dataType:  "",
			wantError: false,
		},
		{
			name:      "invalid_string",
			dataType:  "invalid",
			wantError: true,
			errMsg:    "datatype",
		},
		{
			name:      "invalid_number",
			dataType:  "123",
			wantError: true,
			errMsg:    "datatype",
		},
		{
			name:      "mixed_case_valid", // should normalize
			dataType:  "TimeSeries",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kpi := &models.KPIDefinition{
				Name:       "test-kpi",
				Layer:      "cause",
				SignalType: "metrics",
				Sentiment:  "negative",
				Classifier: "latency",
				Formula:    "test_metric",
				DataType:   tt.dataType,
				Dashboard:  "123e4567-e89b-52d3-a456-426614174000",
			}

			err := ValidateKPIDefinition(cfg, kpi)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestValidateKPIDefinition_RefreshInterval tests RefreshInterval validation (P3-T1-S2)
func TestValidateKPIDefinition_RefreshInterval(t *testing.T) {
	cfg := makeTestConfig()

	tests := []struct {
		name            string
		refreshInterval int
		wantError       bool
		errMsg          string
	}{
		{
			name:            "valid_positive",
			refreshInterval: 60,
			wantError:       false,
		},
		{
			name:            "valid_large",
			refreshInterval: 3600,
			wantError:       false,
		},
		{
			name:            "valid_zero", // zero means not set (omitted)
			refreshInterval: 0,
			wantError:       false,
		},
		{
			name:            "invalid_negative",
			refreshInterval: -10,
			wantError:       true,
			errMsg:          "refreshinterval",
		},
		{
			name:            "invalid_large_negative",
			refreshInterval: -9999,
			wantError:       true,
			errMsg:          "refreshinterval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kpi := &models.KPIDefinition{
				Name:            "test-kpi",
				Layer:           "cause",
				SignalType:      "metrics",
				Sentiment:       "negative",
				Classifier:      "latency",
				Formula:         "test_metric",
				RefreshInterval: tt.refreshInterval,
				Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
			}

			err := ValidateKPIDefinition(cfg, kpi)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestValidateKPIDefinition_UserID tests UserID UUID validation (P3-T1-S3)
func TestValidateKPIDefinition_UserID(t *testing.T) {
	cfg := makeTestConfig()

	tests := []struct {
		name      string
		userID    string
		wantError bool
		errMsg    string
	}{
		{
			name:      "valid_uuid_lowercase",
			userID:    "123e4567-e89b-12d3-a456-426614174000",
			wantError: false,
		},
		{
			name:      "valid_uuid_uppercase",
			userID:    "123E4567-E89B-12D3-A456-426614174000",
			wantError: false,
		},
		{
			name:      "valid_uuid_mixed",
			userID:    "abcDEF12-3456-7890-AbCd-123456789abc",
			wantError: false,
		},
		{
			name:      "valid_empty", // optional field
			userID:    "",
			wantError: false,
		},
		{
			name:      "invalid_too_short",
			userID:    "123e4567-e89b-12d3-a456",
			wantError: true,
			errMsg:    "userid",
		},
		{
			name:      "invalid_no_hyphens",
			userID:    "123e4567e89b12d3a456426614174000",
			wantError: true,
			errMsg:    "userid",
		},
		{
			name:      "invalid_wrong_hyphen_positions",
			userID:    "123e4-567e89b-12d3a456-426614174000",
			wantError: true,
			errMsg:    "userid",
		},
		{
			name:      "invalid_non_hex_characters",
			userID:    "123g4567-e89b-12d3-a456-426614174000",
			wantError: true,
			errMsg:    "userid",
		},
		{
			name:      "invalid_random_string",
			userID:    "not-a-valid-uuid-string",
			wantError: true,
			errMsg:    "userid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kpi := &models.KPIDefinition{
				Name:       "test-kpi",
				Layer:      "cause",
				SignalType: "metrics",
				Sentiment:  "negative",
				Classifier: "latency",
				Formula:    "test_metric",
				UserID:     tt.userID,
				Dashboard:  "123e4567-e89b-52d3-a456-426614174000",
			}

			err := ValidateKPIDefinition(cfg, kpi)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestValidateKPIDefinition_DataSourceID tests DataSourceID UUID validation (P3-T1-S4)
func TestValidateKPIDefinition_DataSourceID(t *testing.T) {
	cfg := makeTestConfig()

	tests := []struct {
		name         string
		dataSourceID string
		wantError    bool
		errMsg       string
	}{
		{
			name:         "valid_uuid",
			dataSourceID: "12345678-1234-1234-1234-123456789abc",
			wantError:    false,
		},
		{
			name:         "valid_empty",
			dataSourceID: "",
			wantError:    false,
		},
		{
			name:         "invalid_format",
			dataSourceID: "not-a-uuid",
			wantError:    true,
			errMsg:       "datasourceid",
		},
		{
			name:         "invalid_too_long",
			dataSourceID: "12345678-1234-1234-1234-123456789abc-extra",
			wantError:    true,
			errMsg:       "datasourceid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kpi := &models.KPIDefinition{
				Name:         "test-kpi",
				Layer:        "cause",
				SignalType:   "metrics",
				Sentiment:    "negative",
				Classifier:   "latency",
				Formula:      "test_metric",
				DataSourceID: tt.dataSourceID,
				Dashboard:    "123e4567-e89b-52d3-a456-426614174000",
			}

			err := ValidateKPIDefinition(cfg, kpi)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestValidateKPIDefinition_KPIDatastoreID tests KPIDatastoreID UUID validation (P3-T1-S5)
func TestValidateKPIDefinition_KPIDatastoreID(t *testing.T) {
	cfg := makeTestConfig()

	tests := []struct {
		name           string
		kpiDatastoreID string
		wantError      bool
		errMsg         string
	}{
		{
			name:           "valid_uuid",
			kpiDatastoreID: "87654321-4321-4321-4321-210987654321",
			wantError:      false,
		},
		{
			name:           "valid_empty",
			kpiDatastoreID: "",
			wantError:      false,
		},
		{
			name:           "invalid_format",
			kpiDatastoreID: "invalid-kpi-datastore-id",
			wantError:      true,
			errMsg:         "kpidatastoreid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kpi := &models.KPIDefinition{
				Name:           "test-kpi",
				Layer:          "cause",
				SignalType:     "metrics",
				Sentiment:      "negative",
				Classifier:     "latency",
				Formula:        "test_metric",
				KPIDatastoreID: tt.kpiDatastoreID,
				Dashboard:      "123e4567-e89b-52d3-a456-426614174000",
			}

			err := ValidateKPIDefinition(cfg, kpi)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestValidateKPIDefinition_CrossFieldDataType tests cross-field validation (P3-T1-S6)
func TestValidateKPIDefinition_CrossFieldDataType(t *testing.T) {
	cfg := makeTestConfig()

	tests := []struct {
		name       string
		dataType   string
		signalType string
		wantError  bool
		errMsg     string
	}{
		{
			name:       "timeseries_with_metrics",
			dataType:   "timeseries",
			signalType: "metrics",
			wantError:  false, // Natural alignment
		},
		{
			name:       "categorical_with_logs",
			dataType:   "categorical",
			signalType: "logs",
			wantError:  false, // Valid combination
		},
		{
			name:       "value_with_business",
			dataType:   "value",
			signalType: "business",
			wantError:  false, // Business KPIs can be any type
		},
		{
			name:       "timeseries_with_business",
			dataType:   "timeseries",
			signalType: "business",
			wantError:  false, // Business KPIs can track time-series data
		},
		{
			name:       "categorical_with_metrics",
			dataType:   "categorical",
			signalType: "metrics",
			wantError:  false, // Unusual but not invalid (e.g., status codes)
		},
		{
			name:       "no_datatype_set",
			dataType:   "",
			signalType: "metrics",
			wantError:  false, // DataType is optional
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kpi := &models.KPIDefinition{
				Name:       "test-kpi",
				Layer:      "cause",
				SignalType: tt.signalType,
				Sentiment:  "negative",
				Classifier: "latency",
				Formula:    "test_metric",
				DataType:   tt.dataType,
				Dashboard:  "123e4567-e89b-52d3-a456-426614174000",
			}

			err := ValidateKPIDefinition(cfg, kpi)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for %s + %s, got: %v", tt.dataType, tt.signalType, err)
				}
			}
		})
	}
}

// TestValidateKPIDefinition_AllNewFieldsTogether tests all new fields together
func TestValidateKPIDefinition_AllNewFieldsTogether(t *testing.T) {
	cfg := makeTestConfig()

	// Valid KPI with all new fields
	validKPI := &models.KPIDefinition{
		Name:            "comprehensive-kpi",
		Layer:           "cause",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Classifier:      "latency",
		Formula:         "test_metric",
		Description:     "A comprehensive KPI with all new fields",
		DataType:        "timeseries",
		DataSourceID:    "12345678-1234-1234-1234-123456789abc",
		KPIDatastoreID:  "87654321-4321-4321-4321-210987654321",
		RefreshInterval: 60,
		IsShared:        true,
		UserID:          "abcdef12-3456-7890-abcd-ef1234567890",
		Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
	}

	err := ValidateKPIDefinition(cfg, validKPI)
	if err != nil {
		t.Errorf("expected valid KPI with all new fields to pass validation, got: %v", err)
	}

	// Invalid KPI with multiple validation errors
	invalidKPI := &models.KPIDefinition{
		Name:            "invalid-kpi",
		Layer:           "cause",
		SignalType:      "metrics",
		Sentiment:       "negative",
		Classifier:      "latency",
		Formula:         "test_metric",
		DataType:        "invalid-type",        // Invalid
		DataSourceID:    "not-a-uuid",          // Invalid
		KPIDatastoreID:  "also-not-a-uuid",     // Invalid
		RefreshInterval: -10,                   // Invalid
		UserID:          "definitely-not-uuid", // Invalid
		Dashboard:       "123e4567-e89b-52d3-a456-426614174000",
	}

	err = ValidateKPIDefinition(cfg, invalidKPI)
	if err == nil {
		t.Fatal("expected multiple validation errors, got nil")
	}

	// Verify all expected errors are present
	errStr := strings.ToLower(err.Error())
	expectedErrors := []string{"datatype", "datasourceid", "kpidatastoreid", "refreshinterval", "userid"}
	for _, expected := range expectedErrors {
		if !strings.Contains(errStr, expected) {
			t.Errorf("expected error to contain %q, error was: %v", expected, err)
		}
	}
}

// TestIsValidUUID tests the UUID validation helper function
func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name  string
		uuid  string
		valid bool
	}{
		{"valid_uuid_v4", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid_uuid_lowercase", "123e4567-e89b-12d3-a456-426614174000", true},
		{"valid_uuid_uppercase", "123E4567-E89B-12D3-A456-426614174000", true},
		{"valid_uuid_mixed", "AbCdEf12-3456-7890-aBcD-123456789aBc", true},
		{"invalid_too_short", "123e4567-e89b-12d3", false},
		{"invalid_too_long", "123e4567-e89b-12d3-a456-426614174000-extra", false},
		{"invalid_no_hyphens", "123e4567e89b12d3a456426614174000", false},
		{"invalid_wrong_hyphen_pos", "123e4-567e89b-12d3a456-426614174000", false},
		{"invalid_non_hex", "123g4567-e89b-12d3-a456-426614174000", false},
		{"invalid_spaces", "123e4567 e89b 12d3 a456 426614174000", false},
		{"empty_string", "", false},
		{"just_hyphens", "--------", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUUID(tt.uuid)
			if result != tt.valid {
				t.Errorf("isValidUUID(%q) = %v, want %v", tt.uuid, result, tt.valid)
			}
		})
	}
}
