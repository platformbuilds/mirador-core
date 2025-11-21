package weaviate

import (
	"github.com/weaviate/weaviate/entities/models"
)

// Schema definitions for Weaviate classes

// KPIDefinitionClass defines the Weaviate class for KPI definitions
func KPIDefinitionClass() *models.Class {
	return &models.Class{
		Class:       "KPIDefinition",
		Description: "Key Performance Indicator definitions",
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
				Name:        "namespace",
				DataType:    []string{"string"},
				Description: "Groups related KPIs (e.g., file or collection name)",
			},
			{
				Name:        "source",
				DataType:    []string{"string"},
				Description: "Where this KPI definition originated (seed file, tool, etc.)",
			},
			{
				Name:        "sourceId",
				DataType:    []string{"string"},
				Description: "Short identifier within the source (e.g., CSV row key)",
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
				DataType:    []string{"object"},
				Description: "Query definition as JSON object",
			},
			{
				Name:        "layer",
				DataType:    []string{"string"},
				Description: "Impact or cause indicator (impact, cause)",
			},
			{
				Name:        "signalType",
				DataType:    []string{"string"},
				Description: "Signal kind (metrics, traces, logs, business, synthetic)",
			},
			{
				Name:        "classifier",
				DataType:    []string{"string"},
				Description: "Measurement category (latency, errors, cpu_utilization, etc.)",
			},
			{
				Name:        "datastore",
				DataType:    []string{"string"},
				Description: "Telemetry/metrics store (victoriametrics, clickhouse, etc.)",
			},
			{
				Name:        "queryType",
				DataType:    []string{"string"},
				Description: "Query language (PromQL, MetricsQL, SQL, etc.)",
			},
			{
				Name:        "formula",
				DataType:    []string{"string"},
				Description: "Raw query/formula string",
			},
			{
				Name:        "thresholds",
				DataType:    []string{"object[]"},
				Description: "Threshold configuration as array of objects",
			},
			{
				Name:        "tags",
				DataType:    []string{"string[]"},
				Description: "Tags for categorization",
			},
			{
				Name:        "definition",
				DataType:    []string{"string"},
				Description: "Human-readable definition of what the signal means",
			},
			{
				Name:        "sentiment",
				DataType:    []string{"string"},
				Description: "Increase sentiment (POSITIVE, NEGATIVE, NEUTRAL)",
			},
			{
				Name:        "category",
				DataType:    []string{"string"},
				Description: "Free-form category for additional grouping/classification",
			},
			{
				Name:        "retryAllowed",
				DataType:    []string{"boolean"},
				Description: "Whether automated retry logic is permitted for incidents",
			},
			{
				Name:        "domain",
				DataType:    []string{"string"},
				Description: "Business or technical domain (payments, kafka, cassandra, infra)",
			},
			{
				Name:        "serviceFamily",
				DataType:    []string{"string"},
				Description: "Groups related services (apigw, oltp, issuer-bank)",
			},
			{
				Name:        "componentType",
				DataType:    []string{"string"},
				Description: "Component type (springboot, kafka-broker, cassandra-node, valkey)",
			},
			{
				Name:        "businessImpact",
				DataType:    []string{"string"},
				Description: "User/business consequence when this KPI degrades",
			},
			{
				Name:        "emotionalImpact",
				DataType:    []string{"string"},
				Description: "Optional severity/emotive hint for narrative generation",
			},
			{
				Name:        "examples",
				DataType:    []string{"object[]"},
				Description: "Example values/contexts stored as JSON objects",
			},
			{
				Name:        "sparkline",
				DataType:    []string{"object"},
				Description: "Sparkline configuration as JSON object",
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

// GetAllClasses returns all schema class definitions
func GetAllClasses() []*models.Class {
	return []*models.Class{
		KPIDefinitionClass(),
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
