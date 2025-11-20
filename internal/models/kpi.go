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
