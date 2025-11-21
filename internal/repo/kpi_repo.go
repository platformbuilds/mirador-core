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

// Conversion functions between models and weavstore types

func toWeavstoreKPI(m *models.KPIDefinition) *weavstore.KPIDefinition {
	if m == nil {
		return nil
	}

	thresholds := make([]weavstore.Threshold, len(m.Thresholds))
	for i, t := range m.Thresholds {
		thresholds[i] = weavstore.Threshold{
			Level:       t.Level,
			Operator:    t.Operator,
			Value:       t.Value,
			Description: t.Description,
		}
	}

	return &weavstore.KPIDefinition{
		ID:              m.ID,
		Name:            m.Name,
		Kind:            m.Kind,
		Namespace:       m.Namespace,
		Source:          m.Source,
		SourceID:        m.SourceID,
		Unit:            m.Unit,
		Format:          m.Format,
		Query:           m.Query,
		Layer:           m.Layer,
		SignalType:      m.SignalType,
		Classifier:      m.Classifier,
		Datastore:       m.Datastore,
		QueryType:       m.QueryType,
		Formula:         m.Formula,
		Thresholds:      thresholds,
		Tags:            m.Tags,
		Definition:      m.Definition,
		Sentiment:       m.Sentiment,
		Category:        m.Category,
		RetryAllowed:    m.RetryAllowed,
		Domain:          m.Domain,
		ServiceFamily:   m.ServiceFamily,
		ComponentType:   m.ComponentType,
		BusinessImpact:  m.BusinessImpact,
		EmotionalImpact: m.EmotionalImpact,
		Examples:        m.Examples,
		Sparkline:       m.Sparkline,
		Visibility:      m.Visibility,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func fromWeavstoreKPI(w *weavstore.KPIDefinition) *models.KPIDefinition {
	if w == nil {
		return nil
	}

	thresholds := make([]models.Threshold, len(w.Thresholds))
	for i, t := range w.Thresholds {
		thresholds[i] = models.Threshold{
			Level:       t.Level,
			Operator:    t.Operator,
			Value:       t.Value,
			Description: t.Description,
		}
	}

	return &models.KPIDefinition{
		ID:              w.ID,
		Name:            w.Name,
		Kind:            w.Kind,
		Namespace:       w.Namespace,
		Source:          w.Source,
		SourceID:        w.SourceID,
		Unit:            w.Unit,
		Format:          w.Format,
		Query:           w.Query,
		Layer:           w.Layer,
		SignalType:      w.SignalType,
		Classifier:      w.Classifier,
		Datastore:       w.Datastore,
		QueryType:       w.QueryType,
		Formula:         w.Formula,
		Thresholds:      thresholds,
		Tags:            w.Tags,
		Definition:      w.Definition,
		Sentiment:       w.Sentiment,
		Category:        w.Category,
		RetryAllowed:    w.RetryAllowed,
		Domain:          w.Domain,
		ServiceFamily:   w.ServiceFamily,
		ComponentType:   w.ComponentType,
		BusinessImpact:  w.BusinessImpact,
		EmotionalImpact: w.EmotionalImpact,
		Examples:        w.Examples,
		Sparkline:       w.Sparkline,
		Visibility:      w.Visibility,
		CreatedAt:       w.CreatedAt,
		UpdatedAt:       w.UpdatedAt,
	}
}

func toWeavstoreListRequest(m *models.KPIListRequest) *weavstore.KPIListRequest {
	if m == nil {
		return nil
	}
	return &weavstore.KPIListRequest{
		Limit:  int64(m.Limit),
		Offset: m.Offset,
	}
}

func (r *DefaultKPIRepo) CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error) {
	if r.store != nil {
		wk := toWeavstoreKPI(k)
		out, status, err := r.store.CreateOrUpdateKPI(ctx, wk)
		if err != nil {
			return nil, status, err
		}
		return fromWeavstoreKPI(out), status, nil
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
		wk := toWeavstoreKPI(k)
		out, status, err := r.store.CreateOrUpdateKPI(ctx, wk)
		if err != nil {
			return nil, status, err
		}
		return fromWeavstoreKPI(out), status, nil
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
	wreq := toWeavstoreListRequest(&req)
	wkpis, total, err := r.store.ListKPIs(ctx, wreq)
	if err != nil {
		return nil, 0, err
	}
	kpis := make([]*models.KPIDefinition, len(wkpis))
	for i, wk := range wkpis {
		kpis[i] = fromWeavstoreKPI(wk)
	}
	return kpis, total, nil
}

func (r *DefaultKPIRepo) GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error) {
	if r.store == nil {
		return nil, nil
	}
	wk, err := r.store.GetKPI(ctx, id)
	if err != nil {
		return nil, err
	}
	if wk == nil {
		return nil, nil
	}
	return fromWeavstoreKPI(wk), nil
}
