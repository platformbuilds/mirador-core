package weaviate

import (
	"github.com/weaviate/weaviate/entities/models"
)

// Schema definitions for Weaviate classes

// KPIDefinitionClass defines the Weaviate class for KPI definitions
func KPIDefinitionClass() *models.Class {
	return &models.Class{
		Class:       "KPIDefinition",
		Description: "Key Performance Indicator definitions for dashboards",
		Properties: []*models.Property{
			{
				Name:        "id",
				DataType:    []string{"string"},
				Description: "Unique identifier for the KPI definition",
			},
			{
				Name:        "kind",
				DataType:    []string{"string"},
				Description: "Type of KPI (business or tech)",
			},
			{
				Name:        "name",
				DataType:    []string{"string"},
				Description: "Display name of the KPI",
			},
			{
				Name:        "unit",
				DataType:    []string{"string"},
				Description: "Unit of measurement",
			},
			{
				Name:        "format",
				DataType:    []string{"string"},
				Description: "Display format",
			},
			{
				Name:        "query",
				DataType:    []string{"string"}, // JSON string
				Description: "Query definition as JSON",
			},
			{
				Name:        "thresholds",
				DataType:    []string{"string"}, // JSON string
				Description: "Threshold configuration as JSON array",
			},
			{
				Name:        "tags",
				DataType:    []string{"string[]"},
				Description: "Tags for categorization",
			},
			{
				Name:        "sparkline",
				DataType:    []string{"string"}, // JSON string
				Description: "Sparkline configuration as JSON",
			},
			{
				Name:        "ownerUserId",
				DataType:    []string{"string"},
				Description: "ID of the user who owns this KPI",
			},
			{
				Name:        "visibility",
				DataType:    []string{"string"},
				Description: "Visibility level (private, team, org)",
			},
			{
				Name:        "createdAt",
				DataType:    []string{"date"},
				Description: "Creation timestamp",
			},
			{
				Name:        "updatedAt",
				DataType:    []string{"date"},
				Description: "Last update timestamp",
			},
		},
	}
}

// KPILayoutClass defines the Weaviate class for KPI layouts
func KPILayoutClass() *models.Class {
	return &models.Class{
		Class:       "KPILayout",
		Description: "Layout configuration for KPIs on dashboards",
		Properties: []*models.Property{
			{
				Name:        "id",
				DataType:    []string{"string"},
				Description: "Unique identifier for the layout",
			},
			{
				Name:        "kpiDefinition",
				DataType:    []string{"KPIDefinition"},
				Description: "Cross-reference to the KPI definition",
			},
			{
				Name:        "dashboard",
				DataType:    []string{"Dashboard"},
				Description: "Cross-reference to the dashboard",
			},
			{
				Name:        "x",
				DataType:    []string{"int"},
				Description: "X coordinate on the grid",
			},
			{
				Name:        "y",
				DataType:    []string{"int"},
				Description: "Y coordinate on the grid",
			},
			{
				Name:        "w",
				DataType:    []string{"int"},
				Description: "Width in grid units",
			},
			{
				Name:        "h",
				DataType:    []string{"int"},
				Description: "Height in grid units",
			},
			{
				Name:        "createdAt",
				DataType:    []string{"date"},
				Description: "Creation timestamp",
			},
			{
				Name:        "updatedAt",
				DataType:    []string{"date"},
				Description: "Last update timestamp",
			},
		},
	}
}

// DashboardClass defines the Weaviate class for dashboards
func DashboardClass() *models.Class {
	return &models.Class{
		Class:       "Dashboard",
		Description: "Dashboard configurations",
		Properties: []*models.Property{
			{
				Name:        "id",
				DataType:    []string{"string"},
				Description: "Unique identifier for the dashboard",
			},
			{
				Name:        "name",
				DataType:    []string{"string"},
				Description: "Display name of the dashboard",
			},
			{
				Name:        "ownerUserId",
				DataType:    []string{"string"},
				Description: "ID of the user who owns this dashboard",
			},
			{
				Name:        "visibility",
				DataType:    []string{"string"},
				Description: "Visibility level (private, team, org)",
			},
			{
				Name:        "isDefault",
				DataType:    []string{"boolean"},
				Description: "Whether this is the default dashboard",
			},
			{
				Name:        "createdAt",
				DataType:    []string{"date"},
				Description: "Creation timestamp",
			},
			{
				Name:        "updatedAt",
				DataType:    []string{"date"},
				Description: "Last update timestamp",
			},
		},
	}
}

// UserPreferencesClass defines the Weaviate class for user preferences
func UserPreferencesClass() *models.Class {
	return &models.Class{
		Class:       "UserPreferences",
		Description: "User preferences and UI state",
		Properties: []*models.Property{
			{
				Name:        "id",
				DataType:    []string{"string"},
				Description: "Unique identifier (user ID)",
			},
			{
				Name:        "currentDashboard",
				DataType:    []string{"Dashboard"},
				Description: "Cross-reference to the current dashboard",
			},
			{
				Name:        "theme",
				DataType:    []string{"string"},
				Description: "UI theme preference",
			},
			{
				Name:        "sidebarCollapsed",
				DataType:    []string{"boolean"},
				Description: "Sidebar collapse state",
			},
			{
				Name:        "defaultDashboard",
				DataType:    []string{"string"},
				Description: "Default dashboard ID",
			},
			{
				Name:        "timezone",
				DataType:    []string{"string"},
				Description: "User timezone",
			},
			{
				Name:        "keyboardHintSeen",
				DataType:    []string{"boolean"},
				Description: "Whether keyboard hint has been seen",
			},
			{
				Name:        "miradorCoreEndpoint",
				DataType:    []string{"string"},
				Description: "Custom API endpoint",
			},
			{
				Name:        "preferences",
				DataType:    []string{"string"}, // JSON string
				Description: "Extensible preferences as JSON",
			},
			{
				Name:        "createdAt",
				DataType:    []string{"date"},
				Description: "Creation timestamp",
			},
			{
				Name:        "updatedAt",
				DataType:    []string{"date"},
				Description: "Last update timestamp",
			},
		},
	}
}

// GetAllClasses returns all schema class definitions
func GetAllClasses() []*models.Class {
	return []*models.Class{
		KPIDefinitionClass(),
		KPILayoutClass(),
		DashboardClass(),
		UserPreferencesClass(),
	}
}

// ClassToMap converts a models.Class to map[string]any for HTTP API
func ClassToMap(class *models.Class) map[string]any {
	properties := make([]map[string]any, len(class.Properties))
	for i, prop := range class.Properties {
		properties[i] = map[string]any{
			"name":        prop.Name,
			"dataType":    prop.DataType,
			"description": prop.Description,
		}
	}
	return map[string]any{
		"class":       class.Class,
		"description": class.Description,
		"properties":  properties,
	}
}

// GetAllClassMaps returns all class definitions as maps for deployment
func GetAllClassMaps() []map[string]any {
	classes := GetAllClasses()
	maps := make([]map[string]any, len(classes))
	for i, class := range classes {
		maps[i] = ClassToMap(class)
	}
	return maps
}
