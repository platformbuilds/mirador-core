package models

import (
	"strings"
	"testing"
	"time"
)

func TestKPIDefinition_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		kpi     KPIDefinition
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid business KPI",
			kpi: KPIDefinition{
				ID:          "revenue-kpi",
				Kind:        "business",
				Name:        "Revenue KPI",
				Unit:        "$",
				Format:      "currency",
				Query:       map[string]interface{}{"metric": "revenue_total"},
				Tags:        []string{"finance", "revenue"},
				Definition:  "Total revenue metric",
				Sentiment:   "POSITIVE",
				OwnerUserID: "user123",
				Visibility:  "private",
				TenantID:    "tenant1",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantErr: false,
		},
		{
			name: "valid tech KPI with thresholds",
			kpi: KPIDefinition{
				ID:     "cpu-usage",
				Kind:   "tech",
				Name:   "CPU Usage",
				Unit:   "%",
				Format: "percentage",
				Query:  map[string]interface{}{"metric": "cpu_usage_percent"},
				Thresholds: []Threshold{
					{Level: "warning", Operator: "gt", Value: 70.0, Color: "yellow"},
					{Level: "critical", Operator: "gt", Value: 90.0, Color: "red"},
				},
				Tags:        []string{"infrastructure", "cpu"},
				Definition:  "CPU usage percentage",
				Sentiment:   "NEGATIVE",
				OwnerUserID: "user456",
				Visibility:  "team",
				TenantID:    "tenant1",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			kpi: KPIDefinition{
				Name:        "Test KPI",
				Kind:        "business",
				OwnerUserID: "user123",
				Visibility:  "private",
				Query:       map[string]interface{}{"metric": "test"},
			},
			wantErr: true,
			errMsg:  "ID cannot be empty",
		},
		{
			name: "empty name",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Kind:        "business",
				OwnerUserID: "user123",
				Visibility:  "private",
				Query:       map[string]interface{}{"metric": "test"},
			},
			wantErr: true,
			errMsg:  "name cannot be empty",
		},
		{
			name: "invalid kind",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Name:        "Test KPI",
				Kind:        "invalid",
				OwnerUserID: "user123",
				Visibility:  "private",
				Query:       map[string]interface{}{"metric": "test"},
			},
			wantErr: true,
			errMsg:  "kind must be 'business' or 'tech'",
		},
		{
			name: "invalid visibility",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Name:        "Test KPI",
				Kind:        "business",
				OwnerUserID: "user123",
				Visibility:  "public",
				Query:       map[string]interface{}{"metric": "test"},
			},
			wantErr: true,
			errMsg:  "visibility must be 'private', 'team', or 'org'",
		},
		{
			name: "invalid sentiment",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Name:        "Test KPI",
				Kind:        "business",
				Sentiment:   "INVALID",
				OwnerUserID: "user123",
				Visibility:  "private",
				Query:       map[string]interface{}{"metric": "test"},
			},
			wantErr: true,
			errMsg:  "sentiment must be 'NEGATIVE', 'POSITIVE', or 'NEUTRAL'",
		},
		{
			name: "empty owner",
			kpi: KPIDefinition{
				ID:         "test-kpi",
				Name:       "Test KPI",
				Kind:       "business",
				Visibility: "private",
				Query:      map[string]interface{}{"metric": "test"},
			},
			wantErr: true,
			errMsg:  "ownerUserId cannot be empty",
		},
		{
			name: "empty query",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Name:        "Test KPI",
				Kind:        "business",
				OwnerUserID: "user123",
				Visibility:  "private",
			},
			wantErr: true,
			errMsg:  "query definition is required",
		},
		{
			name: "unordered positive thresholds",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Name:        "Test KPI",
				Kind:        "business",
				Sentiment:   "POSITIVE",
				OwnerUserID: "user123",
				Visibility:  "private",
				Query:       map[string]interface{}{"metric": "test"},
				Thresholds: []Threshold{
					{Level: "critical", Operator: "lt", Value: 500.0}, // Critical first (should be lower value)
					{Level: "warning", Operator: "lt", Value: 1000.0}, // Warning should be before critical (wrong order)
				},
			},
			wantErr: true,
			errMsg:  "thresholds must be ordered by value (ascending for positive sentiment, descending for negative)",
		},
		{
			name: "unordered negative thresholds",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Name:        "Test KPI",
				Kind:        "business",
				Sentiment:   "NEGATIVE",
				OwnerUserID: "user123",
				Visibility:  "private",
				Query:       map[string]interface{}{"metric": "test"},
				Thresholds: []Threshold{
					{Level: "critical", Operator: "gt", Value: 10.0}, // Critical first (lower value)
					{Level: "warning", Operator: "gt", Value: 5.0},   // Warning should be before critical
				},
			},
			wantErr: true,
			errMsg:  "thresholds must be ordered by value (ascending for positive sentiment, descending for negative)",
		},
		{
			name: "invalid formula",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Name:        "Test KPI",
				Kind:        "business",
				OwnerUserID: "user123",
				Visibility:  "private",
				Query:       map[string]interface{}{"formula": "revenue + @invalid"},
			},
			wantErr: true,
			errMsg:  "invalid characters in formula expression",
		},
		{
			name: "division by zero",
			kpi: KPIDefinition{
				ID:          "test-kpi",
				Name:        "Test KPI",
				Kind:        "business",
				OwnerUserID: "user123",
				Visibility:  "private",
				Query:       map[string]interface{}{"formula": "revenue / 0"},
			},
			wantErr: true,
			errMsg:  "division by zero detected in formula",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.kpi.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("KPIDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil && err.Error() != tt.errMsg {
				t.Errorf("KPIDefinition.Validate() error = %v, want error message %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestDashboard_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		dash    Dashboard
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid dashboard",
			dash: Dashboard{
				ID:          "dashboard-1",
				Name:        "My Dashboard",
				OwnerUserID: "user123",
				Visibility:  "private",
				IsDefault:   false,
				TenantID:    "tenant1",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			dash: Dashboard{
				Name:        "Test Dashboard",
				OwnerUserID: "user123",
				Visibility:  "private",
			},
			wantErr: true,
			errMsg:  "ID cannot be empty",
		},
		{
			name: "empty name",
			dash: Dashboard{
				ID:          "dashboard-1",
				OwnerUserID: "user123",
				Visibility:  "private",
			},
			wantErr: true,
			errMsg:  "name cannot be empty",
		},
		{
			name: "invalid visibility",
			dash: Dashboard{
				ID:          "dashboard-1",
				Name:        "Test Dashboard",
				OwnerUserID: "user123",
				Visibility:  "public",
			},
			wantErr: true,
			errMsg:  "visibility must be 'private', 'team', or 'org'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dash.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Dashboard.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil && err.Error() != tt.errMsg {
				t.Errorf("Dashboard.Validate() error = %v, want error message %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestDashboard_ValidateDeletion(t *testing.T) {
	tests := []struct {
		name    string
		dash    Dashboard
		wantErr bool
		errMsg  string
	}{
		{
			name: "can delete non-default dashboard",
			dash: Dashboard{
				ID:        "dashboard-1",
				IsDefault: false,
			},
			wantErr: false,
		},
		{
			name: "cannot delete default dashboard",
			dash: Dashboard{
				ID:        "default-dashboard",
				IsDefault: true,
			},
			wantErr: true,
			errMsg:  "cannot delete the default dashboard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dash.ValidateDeletion()
			if (err != nil) != tt.wantErr {
				t.Errorf("Dashboard.ValidateDeletion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil && err.Error() != tt.errMsg {
				t.Errorf("Dashboard.ValidateDeletion() error = %v, want error message %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestKPILayout_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		layout  KPILayout
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid layout",
			layout: KPILayout{
				ID:              "layout-1",
				KPIDefinitionID: "kpi-1",
				DashboardID:     "dashboard-1",
				X:               0,
				Y:               0,
				W:               6,
				H:               4,
				TenantID:        "tenant1",
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			layout: KPILayout{
				KPIDefinitionID: "kpi-1",
				DashboardID:     "dashboard-1",
				X:               0,
				Y:               0,
				W:               6,
				H:               4,
			},
			wantErr: true,
			errMsg:  "ID cannot be empty",
		},
		{
			name: "empty KPI definition ID",
			layout: KPILayout{
				ID:          "layout-1",
				DashboardID: "dashboard-1",
				X:           0,
				Y:           0,
				W:           6,
				H:           4,
			},
			wantErr: true,
			errMsg:  "kpiDefinitionId cannot be empty",
		},
		{
			name: "empty dashboard ID",
			layout: KPILayout{
				ID:              "layout-1",
				KPIDefinitionID: "kpi-1",
				X:               0,
				Y:               0,
				W:               6,
				H:               4,
			},
			wantErr: true,
			errMsg:  "dashboardId cannot be empty",
		},
		{
			name: "negative coordinates",
			layout: KPILayout{
				ID:              "layout-1",
				KPIDefinitionID: "kpi-1",
				DashboardID:     "dashboard-1",
				X:               -1,
				Y:               0,
				W:               6,
				H:               4,
			},
			wantErr: true,
			errMsg:  "grid coordinates must be non-negative",
		},
		{
			name: "zero width",
			layout: KPILayout{
				ID:              "layout-1",
				KPIDefinitionID: "kpi-1",
				DashboardID:     "dashboard-1",
				X:               0,
				Y:               0,
				W:               0,
				H:               4,
			},
			wantErr: true,
			errMsg:  "grid dimensions must be positive and fit within 12-column system",
		},
		{
			name: "exceeds 12-column grid",
			layout: KPILayout{
				ID:              "layout-1",
				KPIDefinitionID: "kpi-1",
				DashboardID:     "dashboard-1",
				X:               8,
				Y:               0,
				W:               6,
				H:               4,
			},
			wantErr: true,
			errMsg:  "grid dimensions must be positive and fit within 12-column system",
		},
		{
			name: "excessive height",
			layout: KPILayout{
				ID:              "layout-1",
				KPIDefinitionID: "kpi-1",
				DashboardID:     "dashboard-1",
				X:               0,
				Y:               0,
				W:               6,
				H:               25,
			},
			wantErr: true,
			errMsg:  "grid dimensions must be positive and fit within 12-column system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.layout.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("KPILayout.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("KPILayout.Validate() error = %v, want error message containing %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateVisibilityHierarchy(t *testing.T) {
	tests := []struct {
		name                string
		requestorVisibility string
		resourceVisibility  string
		wantErr             bool
	}{
		{
			name:                "private can access private",
			requestorVisibility: "private",
			resourceVisibility:  "private",
			wantErr:             false,
		},
		{
			name:                "team can access private",
			requestorVisibility: "team",
			resourceVisibility:  "private",
			wantErr:             false,
		},
		{
			name:                "team can access team",
			requestorVisibility: "team",
			resourceVisibility:  "team",
			wantErr:             false,
		},
		{
			name:                "org can access all",
			requestorVisibility: "org",
			resourceVisibility:  "private",
			wantErr:             false,
		},
		{
			name:                "private cannot access team",
			requestorVisibility: "private",
			resourceVisibility:  "team",
			wantErr:             true,
		},
		{
			name:                "private cannot access org",
			requestorVisibility: "private",
			resourceVisibility:  "org",
			wantErr:             true,
		},
		{
			name:                "team cannot access org",
			requestorVisibility: "team",
			resourceVisibility:  "org",
			wantErr:             true,
		},
		{
			name:                "invalid requestor visibility",
			requestorVisibility: "invalid",
			resourceVisibility:  "private",
			wantErr:             true,
		},
		{
			name:                "invalid resource visibility",
			requestorVisibility: "private",
			resourceVisibility:  "invalid",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVisibilityHierarchy(tt.requestorVisibility, tt.resourceVisibility)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVisibilityHierarchy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCanAccessResource(t *testing.T) {
	tests := []struct {
		name               string
		userVisibility     string
		resourceVisibility string
		expected           bool
	}{
		{"private user can access private resource", "private", "private", true},
		{"team user can access private resource", "team", "private", true},
		{"org user can access private resource", "org", "private", true},
		{"private user cannot access team resource", "private", "team", false},
		{"private user cannot access org resource", "private", "org", false},
		{"team user cannot access org resource", "team", "org", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanAccessResource(tt.userVisibility, tt.resourceVisibility)
			if result != tt.expected {
				t.Errorf("CanAccessResource(%s, %s) = %v, expected %v",
					tt.userVisibility, tt.resourceVisibility, result, tt.expected)
			}
		})
	}
}

func TestHasBalancedParentheses(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", true},
		{"no parentheses", "a + b", true},
		{"balanced simple", "(a + b)", true},
		{"balanced nested", "((a + b) * c)", true},
		{"balanced multiple", "(a) + (b)", true},
		{"unbalanced open", "(a + b", false},
		{"unbalanced close", "a + b)", false},
		{"unbalanced nested", "((a + b)", false},
		{"wrong order", ")a + b(", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasBalancedParentheses(tt.input)
			if result != tt.want {
				t.Errorf("hasBalancedParentheses(%s) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}
