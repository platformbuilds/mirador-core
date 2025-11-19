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

// KPIListRequest represents a request to list KPI definitions
type KPIListRequest struct {
	Kind   string   `json:"kind,omitempty"`   // Filter by kind ("business" or "tech")
	Tags   []string `json:"tags,omitempty"`   // Filter by tags
	Limit  int      `json:"limit,omitempty"`  // Maximum number of results
	Offset int      `json:"offset,omitempty"` // Pagination offset
}

// KPIListResponse represents a response containing a list of KPI definitions
type KPIListResponse struct {
	KPIDefinitions []*KPIDefinition `json:"kpiDefinitions"`
	Total          int              `json:"total"`
	NextOffset     int              `json:"nextOffset,omitempty"`
}
