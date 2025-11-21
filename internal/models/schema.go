package models

import (
	"time"
)

// SchemaType represents the type of schema definition
type SchemaType string

const (
	SchemaTypeLabel          SchemaType = "label"
	SchemaTypeMetric         SchemaType = "metric"
	SchemaTypeLogField       SchemaType = "log_field"
	SchemaTypeTraceService   SchemaType = "trace_service"
	SchemaTypeTraceOperation SchemaType = "trace_operation"
	SchemaTypeKPI            SchemaType = "kpi"
)

// SchemaDefinition represents a unified schema definition that can encompass
// all existing schema types (labels, metrics, log fields, traces) and KPIs.
// KPIs are the "new schema definitions" - all schema types are represented as KPIs
// with type-specific extensions.
type SchemaDefinition struct {
	// Core KPI fields (all schema types map to these)
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind,omitempty"` // "business" or "tech" for KPIs
	Name       string                 `json:"name"`
	Unit       string                 `json:"unit,omitempty"`       // Unit of measurement for KPIs
	Format     string                 `json:"format,omitempty"`     // Display format for KPIs
	Query      map[string]interface{} `json:"query,omitempty"`      // Query definition as JSON for KPIs
	Thresholds []Threshold            `json:"thresholds,omitempty"` // Threshold configuration for KPIs
	Tags       []string               `json:"tags,omitempty"`       // Tags for categorization
	Sparkline  map[string]interface{} `json:"sparkline,omitempty"`  // Sparkline configuration for KPIs
	Visibility string                 `json:"visibility,omitempty"` // Visibility level (private, team, org)

	// Schema type classification
	Type SchemaType `json:"type"` // The schema type this definition represents

	// Common metadata
	Category  string    `json:"category,omitempty"`
	Sentiment string    `json:"sentiment,omitempty"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`

	// Type-specific extensions
	Extensions SchemaExtensions `json:"extensions,omitempty"`
}

// SchemaExtensions contains type-specific fields for different schema types
type SchemaExtensions struct {
	// Label-specific fields
	Label *LabelExtension `json:"label,omitempty"`

	// Metric-specific fields
	Metric *MetricExtension `json:"metric,omitempty"`

	// Log field-specific fields
	LogField *LogFieldExtension `json:"logField,omitempty"`

	// Trace-specific fields
	Trace *TraceExtension `json:"trace,omitempty"`
}

// LabelExtension contains fields specific to label schema definitions
type LabelExtension struct {
	Type        string                 `json:"type"`                    // Data type (string, number, etc.)
	Required    bool                   `json:"required"`                // Whether the label is required
	AllowedVals map[string]interface{} `json:"allowedValues,omitempty"` // Allowed values for the label
	Description string                 `json:"description,omitempty"`   // Description of the label
}

// MetricExtension contains fields specific to metric schema definitions
type MetricExtension struct {
	Description string `json:"description,omitempty"` // Description of the metric
	Owner       string `json:"owner,omitempty"`       // Owner of the metric
}

// LogFieldExtension contains fields specific to log field schema definitions
type LogFieldExtension struct {
	FieldType   string `json:"fieldType"`             // Type of the log field
	Description string `json:"description,omitempty"` // Description of the log field
}

// TraceExtension contains fields specific to trace schema definitions
type TraceExtension struct {
	Service        string `json:"service,omitempty"`        // Service name (for operations)
	Operation      string `json:"operation,omitempty"`      // Operation name
	ServicePurpose string `json:"servicePurpose,omitempty"` // Purpose of the service/operation
	Owner          string `json:"owner,omitempty"`          // Owner of the service/operation
}

// Threshold represents a threshold configuration for KPIs
type Threshold struct {
	Level       string  `json:"level"`                 // e.g., "warning", "critical"
	Operator    string  `json:"operator"`              // e.g., "gt", "lt", "eq"
	Value       float64 `json:"value"`                 // Threshold value
	Color       string  `json:"color,omitempty"`       // Display color
	Description string  `json:"description,omitempty"` // Description of the threshold
}

// SchemaDefinitionRequest represents a request to create/update a schema definition
type SchemaDefinitionRequest struct {
	SchemaDefinition *SchemaDefinition `json:"schemaDefinition"`
}

// SchemaDefinitionResponse represents a response containing a schema definition
type SchemaDefinitionResponse struct {
	SchemaDefinition *SchemaDefinition `json:"schemaDefinition"`
}

// SchemaListRequest represents a request to list schema definitions
type SchemaListRequest struct {
	Type     SchemaType `json:"type,omitempty"`     // Filter by schema type
	Category string     `json:"category,omitempty"` // Filter by category
	Tags     []string   `json:"tags,omitempty"`     // Filter by tags
	Limit    int        `json:"limit,omitempty"`    // Maximum number of results
	Offset   int        `json:"offset,omitempty"`   // Pagination offset
}

// SchemaListResponse represents a response containing a list of schema definitions
type SchemaListResponse struct {
	SchemaDefinitions []*SchemaDefinition `json:"schemaDefinitions"`
	Total             int                 `json:"total"`
	NextOffset        int                 `json:"nextOffset,omitempty"`
}

// Label represents a label definition (for backward compatibility)
type Label struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Required    bool                   `json:"required"`
	AllowedVals map[string]interface{} `json:"allowedValues,omitempty"`
	Description string                 `json:"description,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Sentiment   string                 `json:"sentiment,omitempty"`
	Author      string                 `json:"author,omitempty"`
	UpdatedAt   time.Time              `json:"updatedAt,omitempty"`
}

// LabelRequest represents a request to create/update a label definition
type LabelRequest struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type,omitempty"`
	Required      bool                   `json:"required,omitempty"`
	AllowedValues map[string]interface{} `json:"allowedValues,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Author        string                 `json:"author,omitempty"`
}

// LabelResponse represents a response containing a label definition
type LabelResponse struct {
	Label *Label `json:"label"`
}

// LabelListRequest represents a request to list label definitions
type LabelListRequest struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// LabelListResponse represents a response containing a list of label definitions
type LabelListResponse struct {
	Labels     []*Label `json:"labels"`
	Total      int      `json:"total"`
	NextOffset int      `json:"nextOffset,omitempty"`
}

// Metric represents a metric definition (for backward compatibility)
type Metric struct {
	Metric      string    `json:"metric"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Category    string    `json:"category,omitempty"`
	Sentiment   string    `json:"sentiment,omitempty"`
	Author      string    `json:"author,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

// MetricRequest represents a request to create/update a metric definition
type MetricRequest struct {
	Metric *Metric `json:"metric"`
}

// MetricResponse represents a response containing a metric definition
type MetricResponse struct {
	Metric *Metric `json:"metric"`
}

// MetricListRequest represents a request to list metric definitions
type MetricListRequest struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// MetricListResponse represents a response containing a list of metric definitions
type MetricListResponse struct {
	Metrics    []*Metric `json:"metrics"`
	Total      int       `json:"total"`
	NextOffset int       `json:"nextOffset,omitempty"`
}

// LogField represents a log field definition (for backward compatibility)
type LogField struct {
	Field       string    `json:"field"`
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Category    string    `json:"category,omitempty"`
	Sentiment   string    `json:"sentiment,omitempty"`
	Author      string    `json:"author,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

// LogFieldRequest represents a request to create/update a log field definition
type LogFieldRequest struct {
	LogField *LogField `json:"logField"`
}

// LogFieldResponse represents a response containing a log field definition
type LogFieldResponse struct {
	LogField *LogField `json:"logField"`
}

// LogFieldListRequest represents a request to list log field definitions
type LogFieldListRequest struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// LogFieldListResponse represents a response containing a list of log field definitions
type LogFieldListResponse struct {
	LogFields  []*LogField `json:"logFields"`
	Total      int         `json:"total"`
	NextOffset int         `json:"nextOffset,omitempty"`
}

// TraceService represents a trace service definition (for backward compatibility)
type TraceService struct {
	Service        string    `json:"service"`
	ServicePurpose string    `json:"servicePurpose,omitempty"`
	Owner          string    `json:"owner,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	Category       string    `json:"category,omitempty"`
	Sentiment      string    `json:"sentiment,omitempty"`
	Author         string    `json:"author,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt,omitempty"`
}

// TraceServiceRequest represents a request to create/update a trace service definition
type TraceServiceRequest struct {
	TraceService *TraceService `json:"traceService"`
}

// TraceServiceResponse represents a response containing a trace service definition
type TraceServiceResponse struct {
	TraceService *TraceService `json:"traceService"`
}

// TraceServiceListRequest represents a request to list trace service definitions
type TraceServiceListRequest struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// TraceServiceListResponse represents a response containing a list of trace service definitions
type TraceServiceListResponse struct {
	TraceServices []*TraceService `json:"traceServices"`
	Total         int             `json:"total"`
	NextOffset    int             `json:"nextOffset,omitempty"`
}

// TraceOperation represents a trace operation definition (for backward compatibility)
type TraceOperation struct {
	Service        string    `json:"service"`
	Operation      string    `json:"operation"`
	ServicePurpose string    `json:"servicePurpose,omitempty"`
	Owner          string    `json:"owner,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	Category       string    `json:"category,omitempty"`
	Sentiment      string    `json:"sentiment,omitempty"`
	Author         string    `json:"author,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt,omitempty"`
}

// TraceOperationRequest represents a request to create/update a trace operation definition
type TraceOperationRequest struct {
	TraceOperation *TraceOperation `json:"traceOperation"`
}

// TraceOperationResponse represents a response containing a trace operation definition
type TraceOperationResponse struct {
	TraceOperation *TraceOperation `json:"traceOperation"`
}

// TraceOperationListRequest represents a request to list trace operation definitions
type TraceOperationListRequest struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// TraceOperationListResponse represents a response containing a list of trace operation definitions
type TraceOperationListResponse struct {
	TraceOperations []*TraceOperation `json:"traceOperations"`
	Total           int               `json:"total"`
	NextOffset      int               `json:"nextOffset,omitempty"`
}
