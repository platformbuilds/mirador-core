package handlers

import (
	"github.com/platformbuilds/mirador-core/internal/models"
)

// kpiToSchemaDefinition converts a KPIDefinition into a SchemaDefinition.
// The mapping tries to preserve common fields; type-specific fields are put
// into the appropriate SchemaExtensions when available.
func kpiToSchemaDefinition(k *models.KPIDefinition, schemaType models.SchemaType) *models.SchemaDefinition {
	def := &models.SchemaDefinition{
		ID:        k.ID,
		Name:      k.Name,
		Kind:      k.Kind,
		Category:  "",
		Sentiment: k.Sentiment,
		Tags:      k.Tags,
		CreatedAt: k.CreatedAt,
		UpdatedAt: k.UpdatedAt,
		Type:      schemaType,
	}

	// Map some common fields
	// Use Definition or Query to populate extension-specific fields where possible
	switch schemaType {
	case models.SchemaTypeMetric:
		def.Extensions.Metric = &models.MetricExtension{
			Description: k.Definition,
			Owner:       "",
		}
	case models.SchemaTypeLabel:
		// Label-specific fields could be encoded inside k.Query in some setups
		def.Extensions.Label = &models.LabelExtension{
			Type:        "",
			Required:    false,
			AllowedVals: nil,
			Description: k.Definition,
		}
	case models.SchemaTypeLogField:
		def.Extensions.LogField = &models.LogFieldExtension{
			FieldType:   "",
			Description: k.Definition,
		}
	case models.SchemaTypeTraceService:
		def.Extensions.Trace = &models.TraceExtension{
			Service:        k.Name,
			ServicePurpose: k.Definition,
			Owner:          "",
		}
	case models.SchemaTypeTraceOperation:
		// For trace operations the composite ID format is usually service:operation.
		// KPIDefinition.Name is used as the operation name in most uses; store there.
		def.Extensions.Trace = &models.TraceExtension{
			Operation:      k.Name,
			ServicePurpose: k.Definition,
			Owner:          "",
		}
	default:
		// leave other types empty
	}

	return def
}
