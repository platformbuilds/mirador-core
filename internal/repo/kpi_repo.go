package repo

import (
	"context"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// KPIRepo extends SchemaStore with KPI-specific operations
type KPIRepo interface {
	SchemaStore

	// KPI operations
	UpsertKPI(ctx context.Context, kpi *models.KPIDefinition) error
	GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error)
	ListKPIs(ctx context.Context, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error)
	DeleteKPI(ctx context.Context, id string) error
}
