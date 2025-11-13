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
	GetKPI(ctx context.Context, tenantID, id string) (*models.KPIDefinition, error)
	ListKPIs(ctx context.Context, tenantID string, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error)
	DeleteKPI(ctx context.Context, tenantID, id string) error

	// Layout operations
	GetKPILayoutsForDashboard(ctx context.Context, tenantID, dashboardID string) (map[string]interface{}, error)
	BatchUpsertKPILayouts(ctx context.Context, tenantID, dashboardID string, layouts map[string]interface{}) error

	// Dashboard operations
	UpsertDashboard(ctx context.Context, dashboard *models.Dashboard) error
	GetDashboard(ctx context.Context, tenantID, id string) (*models.Dashboard, error)
	ListDashboards(ctx context.Context, tenantID string, limit, offset int) ([]*models.Dashboard, int, error)
	DeleteDashboard(ctx context.Context, tenantID, id string) error
}
