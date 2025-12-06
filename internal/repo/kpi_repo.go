package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/utils/bleve"
	"github.com/platformbuilds/mirador-core/internal/weavstore"
	"github.com/platformbuilds/mirador-core/pkg/cache"

	"go.uber.org/zap"
)

// KPIRepo extends SchemaStore with KPI-specific operations

type KPIRepo interface {
	CreateKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error)
	CreateKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error)
	ModifyKPI(ctx context.Context, k *models.KPIDefinition) (*models.KPIDefinition, string, error)
	ModifyKPIBulk(ctx context.Context, items []*models.KPIDefinition) ([]*models.KPIDefinition, []error)
	DeleteKPI(ctx context.Context, id string) (DeleteResult, error)
	DeleteKPIBulk(ctx context.Context, ids []string) []error
	GetKPI(ctx context.Context, id string) (*models.KPIDefinition, error)
	ListKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int64, error)
	SearchKPIs(ctx context.Context, req models.KPISearchRequest) ([]models.KPISearchResult, int64, error)
	// EnsureTelemetryStandards ensures platform telemetry KPI/processor schemas
	// are present in the registry according to the provided engine config.
	EnsureTelemetryStandards(ctx context.Context, cfg *config.EngineConfig) error
}

type DefaultKPIRepo struct {
	// Note: SQL source-of-truth will be added later; for now keep Weaviate available.
	// store is an abstraction so tests can inject fakes.
	store    weavstore.KPIStore
	logger   *zap.Logger
	valkey   cache.ValkeyCluster
	metadata bleve.MetadataStore
}

// NewDefaultKPIRepo constructs the DefaultKPIRepo. Keep signature stable for server wiring.
func NewDefaultKPIRepo(store weavstore.KPIStore, logger *zap.Logger, valkey cache.ValkeyCluster, metadata bleve.MetadataStore) *DefaultKPIRepo {
	return &DefaultKPIRepo{store: store, logger: logger, valkey: valkey, metadata: metadata}
}

// SetMetadataStore allows late wiring of the bleve metadata store (created later
// during server init). It is safe to call with nil to indicate no metadata store.
func (r *DefaultKPIRepo) SetMetadataStore(ms bleve.MetadataStore) {
	r.metadata = ms
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

func (r *DefaultKPIRepo) DeleteKPI(ctx context.Context, id string) (DeleteResult, error) {
	var res DeleteResult

	// 1) Weaviate (authoritative)
	if err := r.deleteFromWeaviate(ctx, id, &res); err != nil {
		return res, err
	}

	// 2) Valkey best-effort cleanup
	r.deleteFromValkey(ctx, id, &res)

	// 3) Bleve metadata store best-effort cleanup
	r.deleteFromBleve(ctx, id, &res)

	return res, nil
}

// deleteFromWeaviate handles deletion from Weaviate (source of truth)
func (r *DefaultKPIRepo) deleteFromWeaviate(ctx context.Context, id string, res *DeleteResult) error {
	if r.store == nil {
		return nil
	}

	// Check existence before delete
	if wk, err := r.store.GetKPI(ctx, id); err == nil && wk != nil {
		res.Weaviate.Found = true
	}

	if err := r.store.DeleteKPI(ctx, id); err != nil {
		// Record error and return — source-of-truth delete failed.
		res.Weaviate.Error = err.Error()
		return err
	}

	// DeleteKPI returns nil only if the object was verified as deleted.
	// Set Deleted=true to reflect successful deletion.
	res.Weaviate.Deleted = true

	// Log the deletion for audit purposes
	if r.logger != nil {
		r.logger.Info("KPI deleted from Weaviate", zap.String("id", id), zap.Bool("found", res.Weaviate.Found), zap.Bool("deleted", res.Weaviate.Deleted))
	}
	return nil
}

// deleteFromValkey handles best-effort cleanup from Valkey cache
func (r *DefaultKPIRepo) deleteFromValkey(ctx context.Context, id string, res *DeleteResult) {
	if r.valkey == nil {
		return
	}

	// Try standard KPI key patterns
	candidates := []string{"kpi:def:%s", "kpi:%s"}
	for _, pat := range candidates {
		key := fmt.Sprintf(pat, id)
		if _, err := r.valkey.Get(ctx, key); err == nil {
			res.Valkey.Found = true
			if derr := r.valkey.Delete(ctx, key); derr != nil {
				if res.Valkey.Error == "" {
					res.Valkey.Error = derr.Error()
				}
			} else {
				res.Valkey.Deleted = true
			}
		}
	}

	// Also attempt to remove bleve index key stored in valkey
	bkey := fmt.Sprintf("bleve:index:%s", id)
	if _, err := r.valkey.Get(ctx, bkey); err == nil {
		res.Bleve.Found = true
		if derr := r.valkey.Delete(ctx, bkey); derr != nil {
			if res.Bleve.Error == "" {
				res.Bleve.Error = derr.Error()
			}
		} else {
			res.Bleve.Deleted = true
		}
	}
}

// deleteFromBleve handles best-effort cleanup from Bleve metadata store
func (r *DefaultKPIRepo) deleteFromBleve(ctx context.Context, id string, res *DeleteResult) {
	if r.metadata == nil {
		return
	}

	if md, err := r.metadata.GetIndexMetadata(ctx, id); err == nil && md != nil {
		res.Bleve.Found = true
		if derr := r.metadata.DeleteIndexMetadata(ctx, id); derr != nil {
			if res.Bleve.Error == "" {
				res.Bleve.Error = derr.Error()
			}
		} else {
			res.Bleve.Deleted = true
		}
	}
}

func (r *DefaultKPIRepo) DeleteKPIBulk(ctx context.Context, ids []string) []error {
	errs := make([]error, len(ids))
	for i, id := range ids {
		_, err := r.DeleteKPI(ctx, id)
		errs[i] = err
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
		// If the Weaviate runtime schema lacks the KPIDefinition class, treat
		// it like an empty registry (no KPIs yet) and return no error so callers
		// can gracefully handle the 'no items' case.
		if errors.Is(err, weavstore.ErrKPIDefinitionClassMissing) {
			if r.logger != nil {
				r.logger.Sugar().Info("kpi repo: kpi_definition class missing in Weaviate; returning empty list")
			}
			// Record a metric so operators can detect an uninitialized KPI registry
			monitoring.RecordWeaviateSchemaMissing("kpi_definition")
			return []*models.KPIDefinition{}, 0, nil
		}
		return nil, 0, err
	}
	kpis := make([]*models.KPIDefinition, len(wkpis))
	for i, wk := range wkpis {
		kpis[i] = fromWeavstoreKPI(wk)
	}
	return kpis, total, nil
}

func (r *DefaultKPIRepo) SearchKPIs(ctx context.Context, req models.KPISearchRequest) ([]models.KPISearchResult, int64, error) {
	if r.store == nil {
		return []models.KPISearchResult{}, 0, nil
	}

	wreq := &weavstore.KPISearchRequest{
		Query:   req.Query,
		Filters: toWeavstoreListRequest(&req.Filters),
		Mode:    req.Mode,
		Limit:   int64(req.Limit),
		Offset:  int64(req.Offset),
		Explain: req.Explain,
	}

	wres, total, err := r.store.SearchKPIs(ctx, wreq)
	if err != nil {
		// Preserve existing behavior: if schema is missing, treat as empty catalog
		if errors.Is(err, weavstore.ErrKPIDefinitionClassMissing) {
			return []models.KPISearchResult{}, 0, nil
		}
		return nil, 0, err
	}

	out := make([]models.KPISearchResult, 0, len(wres))
	for _, wr := range wres {
		if wr == nil || wr.KPI == nil {
			continue
		}
		mk := fromWeavstoreKPI(wr.KPI)
		rres := models.KPISearchResult{
			ID:                mk.ID,
			Name:              mk.Name,
			DefinitionSnippet: mk.Definition,
			Tags:              mk.Tags,
			Kind:              mk.Kind,
			Layer:             mk.Layer,
			Score:             wr.Score,
			MatchingFields:    wr.MatchingFields,
			Highlights:        wr.Highlights,
			KPI:               mk,
		}
		out = append(out, rres)
	}

	return out, total, nil
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

// EnsureTelemetryStandards ensures platform-standard telemetry KPI/processor
// schemas are present in the KPI registry. This method centralizes model
// creation inside the repo layer so bootstrap packages do not depend on
// internal models directly.
func (r *DefaultKPIRepo) EnsureTelemetryStandards(ctx context.Context, engCfg *config.EngineConfig) error {
	if engCfg == nil {
		return nil
	}

	now := time.Now().UTC()

	// Use a safe logger (may be nil)
	var sugar *zap.SugaredLogger
	if r.logger != nil {
		sugar = r.logger.Sugar()
	} else {
		sugar = zap.NewNop().Sugar()
	}

	// Connectors: create KPI definitions from connector metrics
	for connectorName, connector := range engCfg.Telemetry.Connectors {
		for _, m := range connector.Metrics {
			k := &models.KPIDefinition{
				Name:           m.Name,
				Kind:           connectorName,
				Source:         "platform-standard",
				Definition:     m.Description,
				DimensionsHint: m.Labels,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if _, _, err := r.CreateKPI(ctx, k); err != nil {
				sugar.Warnw("failed to upsert telemetry KPI", "name", m.Name, "error", err)
			} else {
				sugar.Infow("bootstrapped telemetry KPI", "name", m.Name, "connector", connectorName)
			}
		}
	}

	// Processors: treat processor label schemas as KPI-like objects
	for procName, proc := range engCfg.Telemetry.Processors {
		k := &models.KPIDefinition{
			Name:           procName,
			Kind:           procName,
			Source:         "platform-standard",
			Definition:     "processor label schema",
			DimensionsHint: proc.Labels,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if _, _, err := r.CreateKPI(ctx, k); err != nil {
			sugar.Warnw("failed to upsert telemetry processor schema", "processor", procName, "error", err)
		} else {
			sugar.Infow("bootstrapped telemetry processor schema", "processor", procName)
		}
	}

	return nil
}
