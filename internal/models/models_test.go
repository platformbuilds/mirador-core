package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKPIDefinition_JSONSerialization(t *testing.T) {
	now := time.Now()
	kpi := KPIDefinition{
		ID:          "test-kpi",
		Kind:        "tech",
		Name:        "Test KPI",
		Unit:        "%",
		Format:      "percentage",
		Query:       map[string]interface{}{"metric": "cpu_usage"},
		Thresholds:  []Threshold{{Level: "warning", Operator: "gt", Value: 80.0}},
		Tags:        []string{"cpu", "performance"},
		Definition:  "CPU usage percentage",
		Sentiment:   "NEGATIVE",
		Sparkline:   map[string]interface{}{"type": "line"},
		OwnerUserID: "user123",
		Visibility:  "private",
		TenantID:    "tenant1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(kpi)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"test-kpi"`)
	assert.Contains(t, string(data), `"kind":"tech"`)

	// Test JSON unmarshaling
	var unmarshaled KPIDefinition
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, kpi.ID, unmarshaled.ID)
	assert.Equal(t, kpi.Kind, unmarshaled.Kind)
	assert.Equal(t, kpi.Name, unmarshaled.Name)
	assert.Equal(t, kpi.Unit, unmarshaled.Unit)
	assert.Equal(t, kpi.Sentiment, unmarshaled.Sentiment)
	assert.Equal(t, kpi.OwnerUserID, unmarshaled.OwnerUserID)
	assert.Equal(t, kpi.Visibility, unmarshaled.Visibility)
	assert.Equal(t, kpi.TenantID, unmarshaled.TenantID)
	assert.Equal(t, len(kpi.Thresholds), len(unmarshaled.Thresholds))
	assert.Equal(t, len(kpi.Tags), len(unmarshaled.Tags))
}

func TestDashboard_JSONSerialization(t *testing.T) {
	now := time.Now()
	dashboard := Dashboard{
		ID:          "dashboard-1",
		Name:        "Test Dashboard",
		OwnerUserID: "user123",
		Visibility:  "private",
		IsDefault:   false,
		TenantID:    "tenant1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(dashboard)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"dashboard-1"`)
	assert.Contains(t, string(data), `"isDefault":false`)

	// Test JSON unmarshaling
	var unmarshaled Dashboard
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, dashboard.ID, unmarshaled.ID)
	assert.Equal(t, dashboard.Name, unmarshaled.Name)
	assert.Equal(t, dashboard.OwnerUserID, unmarshaled.OwnerUserID)
	assert.Equal(t, dashboard.Visibility, unmarshaled.Visibility)
	assert.Equal(t, dashboard.IsDefault, unmarshaled.IsDefault)
	assert.Equal(t, dashboard.TenantID, unmarshaled.TenantID)
}

func TestKPILayout_JSONSerialization(t *testing.T) {
	now := time.Now()
	layout := KPILayout{
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
	}

	// Test JSON marshaling
	data, err := json.Marshal(layout)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"layout-1"`)
	assert.Contains(t, string(data), `"x":0`)
	assert.Contains(t, string(data), `"w":6`)

	// Test JSON unmarshaling
	var unmarshaled KPILayout
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, layout.ID, unmarshaled.ID)
	assert.Equal(t, layout.KPIDefinitionID, unmarshaled.KPIDefinitionID)
	assert.Equal(t, layout.DashboardID, unmarshaled.DashboardID)
	assert.Equal(t, layout.X, unmarshaled.X)
	assert.Equal(t, layout.Y, unmarshaled.Y)
	assert.Equal(t, layout.W, unmarshaled.W)
	assert.Equal(t, layout.H, unmarshaled.H)
	assert.Equal(t, layout.TenantID, unmarshaled.TenantID)
}

func TestUserPreferences_JSONSerialization(t *testing.T) {
	now := time.Now()
	prefs := UserPreferences{
		ID:                  "user123",
		CurrentDashboardID:  "dashboard-1",
		Theme:               "dark",
		SidebarCollapsed:    true,
		DefaultDashboardID:  "dashboard-1",
		Timezone:            "UTC",
		KeyboardHintSeen:    false,
		MiradorCoreEndpoint: "http://localhost:8080",
		Preferences:         map[string]interface{}{"custom": "value"},
		TenantID:            "tenant1",
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(prefs)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"user123"`)
	assert.Contains(t, string(data), `"theme":"dark"`)
	assert.Contains(t, string(data), `"sidebarCollapsed":true`)

	// Test JSON unmarshaling
	var unmarshaled UserPreferences
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, prefs.ID, unmarshaled.ID)
	assert.Equal(t, prefs.CurrentDashboardID, unmarshaled.CurrentDashboardID)
	assert.Equal(t, prefs.Theme, unmarshaled.Theme)
	assert.Equal(t, prefs.SidebarCollapsed, unmarshaled.SidebarCollapsed)
	assert.Equal(t, prefs.DefaultDashboardID, unmarshaled.DefaultDashboardID)
	assert.Equal(t, prefs.Timezone, unmarshaled.Timezone)
	assert.Equal(t, prefs.KeyboardHintSeen, unmarshaled.KeyboardHintSeen)
	assert.Equal(t, prefs.MiradorCoreEndpoint, unmarshaled.MiradorCoreEndpoint)
	assert.Equal(t, prefs.TenantID, unmarshaled.TenantID)
	assert.Equal(t, prefs.Preferences["custom"], unmarshaled.Preferences["custom"])
}

func TestRequestResponseTypes_JSONSerialization(t *testing.T) {
	now := time.Now()

	// Test KPIDefinitionRequest
	kpiReq := KPIDefinitionRequest{
		KPIDefinition: &KPIDefinition{
			ID:          "test-kpi",
			Name:        "Test KPI",
			OwnerUserID: "user123",
			Visibility:  "private",
			Query:       map[string]interface{}{"metric": "test"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	data, err := json.Marshal(kpiReq)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"kpiDefinition"`)

	var unmarshaledReq KPIDefinitionRequest
	err = json.Unmarshal(data, &unmarshaledReq)
	require.NoError(t, err)
	assert.Equal(t, kpiReq.KPIDefinition.ID, unmarshaledReq.KPIDefinition.ID)

	// Test KPIDefinitionResponse
	kpiResp := KPIDefinitionResponse{
		KPIDefinition: &KPIDefinition{
			ID:          "test-kpi",
			Name:        "Test KPI",
			OwnerUserID: "user123",
			Visibility:  "private",
			Query:       map[string]interface{}{"metric": "test"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	data, err = json.Marshal(kpiResp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"kpiDefinition"`)

	// Test DashboardRequest/Response
	dashboardReq := DashboardRequest{
		Dashboard: &Dashboard{
			ID:          "dashboard-1",
			Name:        "Test Dashboard",
			OwnerUserID: "user123",
			Visibility:  "private",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	data, err = json.Marshal(dashboardReq)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"dashboard"`)

	// Test UserPreferencesRequest/Response
	prefsReq := UserPreferencesRequest{
		UserPreferences: &UserPreferences{
			ID:        "user123",
			Theme:     "dark",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	data, err = json.Marshal(prefsReq)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"userPreferences"`)

	// Test list responses
	kpiListResp := KPIListResponse{
		KPIDefinitions: []*KPIDefinition{
			{ID: "kpi1", Name: "KPI 1"},
			{ID: "kpi2", Name: "KPI 2"},
		},
		Total:      2,
		NextOffset: 2,
	}
	data, err = json.Marshal(kpiListResp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"kpiDefinitions"`)
	assert.Contains(t, string(data), `"total":2`)

	dashboardListResp := DashboardListResponse{
		Dashboards: []*Dashboard{
			{ID: "dash1", Name: "Dashboard 1"},
			{ID: "dash2", Name: "Dashboard 2"},
		},
		Total:      2,
		NextOffset: 2,
	}
	data, err = json.Marshal(dashboardListResp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"dashboards"`)
	assert.Contains(t, string(data), `"total":2`)
}

func TestThreshold_JSONSerialization(t *testing.T) {
	threshold := Threshold{
		Level:       "warning",
		Operator:    "gt",
		Value:       80.0,
		Color:       "yellow",
		Description: "Value is too high",
	}

	data, err := json.Marshal(threshold)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"level":"warning"`)
	assert.Contains(t, string(data), `"operator":"gt"`)
	assert.Contains(t, string(data), `"value":80`)

	var unmarshaled Threshold
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, threshold.Level, unmarshaled.Level)
	assert.Equal(t, threshold.Operator, unmarshaled.Operator)
	assert.Equal(t, threshold.Value, unmarshaled.Value)
	assert.Equal(t, threshold.Color, unmarshaled.Color)
	assert.Equal(t, threshold.Description, unmarshaled.Description)
}

func TestTimeFields_JSONSerialization(t *testing.T) {
	now := time.Now()

	// Test that time fields are properly serialized/deserialized
	kpi := KPIDefinition{
		ID:        "test-kpi",
		Name:      "Test KPI",
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(kpi)
	require.NoError(t, err)

	var unmarshaled KPIDefinition
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Time should be preserved (allowing for small differences in precision)
	assert.True(t, unmarshaled.CreatedAt.Sub(now) < time.Second)
	assert.True(t, unmarshaled.UpdatedAt.Sub(now) < time.Second)
}

func TestEmptySlices_JSONSerialization(t *testing.T) {
	// Test that empty slices are handled correctly
	kpi := KPIDefinition{
		ID:         "test-kpi",
		Name:       "Test KPI",
		Thresholds: []Threshold{}, // Empty slice
		Tags:       []string{},    // Empty slice
		Query:      map[string]interface{}{},
	}

	data, err := json.Marshal(kpi)
	require.NoError(t, err)

	var unmarshaled KPIDefinition
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, 0, len(unmarshaled.Thresholds))
	assert.Equal(t, 0, len(unmarshaled.Tags))
	assert.NotNil(t, unmarshaled.Query)
}

func TestNilPointers_JSONSerialization(t *testing.T) {
	// Test that nil pointers in request types are handled
	kpiReq := KPIDefinitionRequest{
		KPIDefinition: nil,
	}

	data, err := json.Marshal(kpiReq)
	require.NoError(t, err)

	var unmarshaled KPIDefinitionRequest
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Nil(t, unmarshaled.KPIDefinition)
}
