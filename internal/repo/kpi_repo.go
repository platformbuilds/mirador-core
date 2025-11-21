package repo

import (
	"context"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/weavstore"

	"go.uber.org/zap"
)

// KPIRepo extends SchemaStore with KPI-specific operations

type KPIRepo interface {
	CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error)
	CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error)
	ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error)
	ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error)
	DeleteKPI(ctx context.Context, id string) error
	DeleteKPIBulk(ctx context.Context, ids []string) []error
	GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error)
	ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error)
}

type DefaultKPIRepo struct {
	// Note: SQL source-of-truth will be added later; for now keep Weaviate available.
	store  *weavstore.WeaviateKPIStore
	logger *zap.Logger
}

// NewDefaultKPIRepo constructs the DefaultKPIRepo. Keep signature stable for server wiring.
func NewDefaultKPIRepo(store *weavstore.WeaviateKPIStore, logger *zap.Logger) *DefaultKPIRepo {
	return &DefaultKPIRepo{store: store, logger: logger}
}

func (r *DefaultKPIRepo) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	if r.store != nil {
		out, status, err := r.store.CreateOrUpdateKPI(ctx, k)
		return out, status, err
	}
	return k, "created", nil
}

func (r *DefaultKPIRepo) CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	created := make([]*models.KPIDefinition, 0, len(items))
	errs := make([]error, len(items))
	for i, k := range items {
		out, _, err := r.CreateKPI(ctx, k)
		errs[i] = err
		if err == nil {
			created = append(created, out)
		}
	}
	return created, errs
}

func (r *DefaultKPIRepo) ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	// Minimal modify: overwrite object in Weaviate and return the updated model
	if k.ID == "" {
		return nil, "", nil
	}
	if r.store != nil {
		return r.store.CreateOrUpdateKPI(ctx, k)
	}
	return k, "updated", nil
}

func (r *DefaultKPIRepo) ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error) {
	modified := make([]*models.KPIDefinition, 0, len(items))
	errs := make([]error, len(items))
	for i, k := range items {
		out, _, err := r.ModifyKPI(ctx, k)
		errs[i] = err
		if err == nil {
			modified = append(modified, out)
		}
	}
	return modified, errs
}

func (r *DefaultKPIRepo) DeleteKPI(ctx context.Context, id string) error {
	if r.store != nil {
		return r.store.DeleteKPI(ctx, id)
	}
	return nil
}

func (r *DefaultKPIRepo) DeleteKPIBulk(ctx context.Context, ids []string) []error {
	errs := make([]error, len(ids))
	for i, id := range ids {
		errs[i] = r.DeleteKPI(ctx, id)
	}
	return errs
}

func (r *DefaultKPIRepo) ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error) {
	if r.store == nil {
		return []*models.KPIDefinition{}, 0, nil
	}
	return r.store.ListKPIs(ctx, &req)
}

func (r *DefaultKPIRepo) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	if r.store == nil {
		return nil, nil
	}
	return r.store.GetKPI(ctx, id)
}
