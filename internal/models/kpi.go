package models

import (
	"time"
)

// KPIDefinition represents a KPI definition stored in Weaviate
type KPIDefinition struct {
	ID          string                 `json:"id"`
	Kind        string                 `json:"kind"` // "business" or "tech"
	Name        string                 `json:"name"`
	Unit        string                 `json:"unit"`
	Format      string                 `json:"format"`
	Query       map[string]interface{} `json:"query"`      // JSON query definition
	Thresholds  []Threshold            `json:"thresholds"` // JSON thresholds array
	Tags        []string               `json:"tags"`
	Definition  string                 `json:"definition"` // Definition of what the signal means
	Sentiment   string                 `json:"sentiment"`  // "NEGATIVE", "POSITIVE", or "NEUTRAL" - increase sentiment
	Sparkline   map[string]interface{} `json:"sparkline"`  // JSON sparkline config
	OwnerUserID string                 `json:"ownerUserId"`
	Visibility  string                 `json:"visibility"` // "private", "team", "org"
	TenantID    string                 `json:"tenantId,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// Dashboard represents a dashboard configuration stored in Weaviate
type Dashboard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	OwnerUserID string    `json:"ownerUserId"`
	Visibility  string    `json:"visibility"` // "private", "team", "org"
	IsDefault   bool      `json:"isDefault"`
	TenantID    string    `json:"tenantId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// KPILayout represents a KPI layout configuration on a dashboard
type KPILayout struct {
	ID              string    `json:"id"`
	KPIDefinitionID string    `json:"kpiDefinitionId"` // Reference to KPI definition
	DashboardID     string    `json:"dashboardId"`     // Reference to dashboard
	X               int       `json:"x"`               // X coordinate on grid
	Y               int       `json:"y"`               // Y coordinate on grid
	W               int       `json:"w"`               // Width in grid units
	H               int       `json:"h"`               // Height in grid units
	TenantID        string    `json:"tenantId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// UserPreferences represents user preferences and UI state
type UserPreferences struct {
	ID                  string                 `json:"id"` // User ID
	CurrentDashboardID  string                 `json:"currentDashboardId,omitempty"`
	Theme               string                 `json:"theme,omitempty"`
	SidebarCollapsed    bool                   `json:"sidebarCollapsed,omitempty"`
	DefaultDashboardID  string                 `json:"defaultDashboardId,omitempty"`
	Timezone            string                 `json:"timezone,omitempty"`
	KeyboardHintSeen    bool                   `json:"keyboardHintSeen,omitempty"`
	MiradorCoreEndpoint string                 `json:"miradorCoreEndpoint,omitempty"`
	Preferences         map[string]interface{} `json:"preferences,omitempty"` // JSON extensible preferences
	TenantID            string                 `json:"tenantId,omitempty"`
	CreatedAt           time.Time              `json:"createdAt"`
	UpdatedAt           time.Time              `json:"updatedAt"`
}

// KPIDefinitionRequest represents a request to create/update a KPI definition
type KPIDefinitionRequest struct {
	KPIDefinition *KPIDefinition `json:"kpiDefinition"`
}

// KPIDefinitionResponse represents a response containing a KPI definition
type KPIDefinitionResponse struct {
	KPIDefinition *KPIDefinition `json:"kpiDefinition"`
}

// DashboardRequest represents a request to create/update a dashboard
type DashboardRequest struct {
	Dashboard *Dashboard `json:"dashboard"`
}

// DashboardResponse represents a response containing a dashboard
type DashboardResponse struct {
	Dashboard *Dashboard `json:"dashboard"`
}

// KPILayoutRequest represents a request to create/update a KPI layout
type KPILayoutRequest struct {
	KPILayout *KPILayout `json:"kpiLayout"`
}

// KPILayoutResponse represents a response containing a KPI layout
type KPILayoutResponse struct {
	KPILayout *KPILayout `json:"kpiLayout"`
}

// UserPreferencesRequest represents a request to update user preferences
type UserPreferencesRequest struct {
	UserPreferences *UserPreferences `json:"userPreferences"`
}

// UserPreferencesResponse represents a response containing user preferences
type UserPreferencesResponse struct {
	UserPreferences *UserPreferences `json:"userPreferences"`
}

// KPIListRequest represents a request to list KPI definitions
type KPIListRequest struct {
	TenantID string   `json:"tenantId"`
	Kind     string   `json:"kind,omitempty"`   // Filter by kind ("business" or "tech")
	Tags     []string `json:"tags,omitempty"`   // Filter by tags
	Limit    int      `json:"limit,omitempty"`  // Maximum number of results
	Offset   int      `json:"offset,omitempty"` // Pagination offset
}

// KPIListResponse represents a response containing a list of KPI definitions
type KPIListResponse struct {
	KPIDefinitions []*KPIDefinition `json:"kpiDefinitions"`
	Total          int              `json:"total"`
	NextOffset     int              `json:"nextOffset,omitempty"`
}

// DashboardListRequest represents a request to list dashboards
type DashboardListRequest struct {
	TenantID    string `json:"tenantId"`
	OwnerUserID string `json:"ownerUserId,omitempty"` // Filter by owner
	Limit       int    `json:"limit,omitempty"`       // Maximum number of results
	Offset      int    `json:"offset,omitempty"`      // Pagination offset
}

// DashboardListResponse represents a response containing a list of dashboards
type DashboardListResponse struct {
	Dashboards []*Dashboard `json:"dashboards"`
	Total      int          `json:"total"`
	NextOffset int          `json:"nextOffset,omitempty"`
}
