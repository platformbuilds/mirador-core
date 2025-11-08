package repo

import (
	"github.com/platformbuilds/mirador-core/internal/models"
)

// KPIRepo extends SchemaStore with KPI-specific operations
type KPIRepo interface {
	SchemaStore

	// KPI operations
	UpsertKPI(kpi *models.KPIDefinition) error
	GetKPI(tenantID, id string) (*models.KPIDefinition, error)
	ListKPIs(tenantID string, tags []string, limit, offset int) ([]*models.KPIDefinition, int, error)
	DeleteKPI(tenantID, id string) error

	// Layout operations
	GetKPILayoutsForDashboard(tenantID, dashboardID string) (map[string]interface{}, error)
	BatchUpsertKPILayouts(tenantID, dashboardID string, layouts map[string]interface{}) error

	// Dashboard operations
	UpsertDashboard(dashboard *models.Dashboard) error
	GetDashboard(tenantID, id string) (*models.Dashboard, error)
	ListDashboards(tenantID string, limit, offset int) ([]*models.Dashboard, int, error)
	DeleteDashboard(tenantID, id string) error
}
