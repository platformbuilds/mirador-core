package weavstore

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	wv "github.com/weaviate/weaviate-go-client/v5/weaviate"
	"go.uber.org/zap"
)

// WeaviateKPIStore is a small wrapper around the official weaviate v5 client
// for KPI-specific operations. It centralizes all KPI Weaviate access via the
// SDK (no raw HTTP/GraphQL strings).
type WeaviateKPIStore struct {
	client *wv.Client
	logger *zap.Logger
}

// NewWeaviateKPIStore constructs a new KPI store.
func NewWeaviateKPIStore(client *wv.Client, logger *zap.Logger) *WeaviateKPIStore {
	return &WeaviateKPIStore{client: client, logger: logger}
}

// helper: object id generation consistent with previous behavior
var (
	nsMirador = func() uuid.UUID {
		u, _ := uuid.FromString("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
		return u
	}()

	// Static errors for err113 compliance
	ErrKPIIsNil   = errors.New("kpi is nil")
	ErrKPIIDEmpty = errors.New("kpi id is empty")
	ErrIDEmpty    = errors.New("id is empty")

	// maxKPIListLimit is the maximum limit for listing KPIs in a single query
	maxKPIListLimit = 10000
)

func makeObjectID(class, id string) string {
	return uuid.NewV5(nsMirador, fmt.Sprintf("%s|%s", class, id)).String()
}

// CreateOrUpdateKPI creates a KPI if missing, updates if present. It returns
// the KPI model, a status string ("created","updated","no-change"), and an error.
func (s *WeaviateKPIStore) CreateOrUpdateKPI(ctx context.Context, k *KPIDefinition) (*KPIDefinition, string, error) {
	if k == nil {
		return nil, "", ErrKPIIsNil
	}
	if k.ID == "" {
		return nil, "", ErrKPIIDEmpty
	}

	objID := makeObjectID("KPIDefinition", k.ID)

	props := map[string]any{
		"name":            k.Name,
		"kind":            k.Kind,
		"namespace":       k.Namespace,
		"source":          k.Source,
		"sourceId":        k.SourceID,
		"unit":            k.Unit,
		"format":          k.Format,
		"query":           k.Query,
		"layer":           k.Layer,
		"signalType":      k.SignalType,
		"classifier":      k.Classifier,
		"datastore":       k.Datastore,
		"queryType":       k.QueryType,
		"formula":         k.Formula,
		"thresholds":      thresholdsToProps(k.Thresholds),
		"tags":            k.Tags,
		"definition":      k.Definition,
		"sentiment":       k.Sentiment,
		"category":        k.Category,
		"retryAllowed":    k.RetryAllowed,
		"domain":          k.Domain,
		"serviceFamily":   k.ServiceFamily,
		"componentType":   k.ComponentType,
		"businessImpact":  k.BusinessImpact,
		"emotionalImpact": k.EmotionalImpact,
		"examples":        k.Examples,
		"sparkline":       k.Sparkline,
		"visibility":      k.Visibility,
		"createdAt":       k.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":       k.UpdatedAt.Format(time.RFC3339Nano),
	}
	// Check if object already exists in Weaviate
	existing, err := s.GetKPI(ctx, k.ID)
	if err != nil {
		return nil, "", err
	}

	// If object exists and is identical field-by-field, treat as success (no-op)
	if existing != nil {
		if kpiEqual(existing, k) {
			// No change detected; return existing as success
			return existing, "no-change", nil
		}
		// There is a modification: perform update
		if err := s.client.Data().Updater().WithClassName("KPIDefinition").WithID(objID).WithProperties(props).Do(ctx); err != nil {
			return nil, "", err
		}
		return k, "updated", nil
	}

	// Not found -> create. If create fails because the object already exists,
	// fall back to updating the existing object. This handles races where the
	// object was created between the GetKPI call and the Creator() call and
	// avoids returning a 422 'id already exists' to callers.
	if _, err := s.client.Data().Creator().WithClassName("KPIDefinition").WithID(objID).WithProperties(props).Do(ctx); err != nil {
		// Some Weaviate error responses include messages like "id '...' already exists"
		// or mention "already exists"; handle those conservatively by attempting
		// an update instead of failing the whole operation.
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "id already exists") {
			if err2 := s.client.Data().Updater().WithClassName("KPIDefinition").WithID(objID).WithProperties(props).Do(ctx); err2 != nil {
				return nil, "", fmt.Errorf("create conflict: update also failed: %w (create err: %v)", err2, err)
			}
			return k, "updated", nil
		}
		return nil, "", err
	}
	return k, "created", nil
}

// kpiEqual compares two KPIDefinition objects field-by-field. It is intentionally
// conservative: it returns false if any field differs. This allows callers to
// detect and apply updates only when necessary.
//
//nolint:gocyclo // Field-by-field comparison is inherently complex
func kpiEqual(a, b *KPIDefinition) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID != b.ID {
		return false
	}
	if a.Name != b.Name || a.Kind != b.Kind || a.Namespace != b.Namespace || a.Source != b.Source || a.SourceID != b.SourceID {
		return false
	}
	if a.Unit != b.Unit || a.Format != b.Format || a.Definition != b.Definition || a.Sentiment != b.Sentiment || a.Category != b.Category {
		return false
	}
	if a.Layer != b.Layer || a.SignalType != b.SignalType || a.Classifier != b.Classifier {
		return false
	}
	if a.Datastore != b.Datastore || a.QueryType != b.QueryType || a.Formula != b.Formula {
		return false
	}
	if a.BusinessImpact != b.BusinessImpact || a.EmotionalImpact != b.EmotionalImpact {
		return false
	}
	if a.RetryAllowed != b.RetryAllowed || a.Domain != b.Domain || a.ServiceFamily != b.ServiceFamily || a.ComponentType != b.ComponentType {
		return false
	}
	if a.Visibility != b.Visibility {
		return false
	}
	// Compare tags (order-sensitive). If you require order-insensitive compare,
	// replace with set comparison.
	if len(a.Tags) != len(b.Tags) {
		return false
	}
	for i := range a.Tags {
		if a.Tags[i] != b.Tags[i] {
			return false
		}
	}
	// Compare Examples
	if len(a.Examples) != len(b.Examples) {
		return false
	}
	for i := range a.Examples {
		if !deepEqualAny(a.Examples[i], b.Examples[i]) {
			return false
		}
	}
	// Compare Query and Sparkline via DeepEqual since they are maps
	if !deepEqualAny(a.Query, b.Query) {
		return false
	}
	if !deepEqualAny(a.Sparkline, b.Sparkline) {
		return false
	}
	// Compare thresholds
	if len(a.Thresholds) != len(b.Thresholds) {
		return false
	}
	for i := range a.Thresholds {
		if a.Thresholds[i].Operator != b.Thresholds[i].Operator {
			return false
		}
		if a.Thresholds[i].Level != b.Thresholds[i].Level {
			return false
		}
		if a.Thresholds[i].Description != b.Thresholds[i].Description {
			return false
		}
		// Numeric comparison for value
		if a.Thresholds[i].Value != b.Thresholds[i].Value {
			return false
		}
	}
	return true
}

// deepEqualAny is a small wrapper around reflect.DeepEqual that treats nil and
// empty maps/slices as equal where appropriate.
func deepEqualAny(x, y any) bool {
	if x == nil && y == nil {
		return true
	}
	return reflect.DeepEqual(x, y)
}

func (s *WeaviateKPIStore) DeleteKPI(ctx context.Context, id string) error {
	if id == "" {
		return ErrIDEmpty
	}
	objID := makeObjectID("KPIDefinition", id)
	if err := s.client.Data().Deleter().WithClassName("KPIDefinition").WithID(objID).Do(ctx); err != nil {
		return err
	}
	return nil
}

//nolint:gocyclo // Property parsing from Weaviate requires many field mappings
func (s *WeaviateKPIStore) GetKPI(ctx context.Context, id string) (*KPIDefinition, error) {
	if id == "" {
		return nil, nil
	}
	objID := makeObjectID("KPIDefinition", id)

	// Fetch objects of the class and search for matching object id. The
	// ObjectsGetter returns a slice of objects in the SDK.
	resp, err := s.client.Data().ObjectsGetter().WithClassName("KPIDefinition").Do(ctx)
	if err != nil {
		return nil, err
	}

	var props map[string]any
	var found bool
	for _, o := range resp {
		if o == nil {
			continue
		}
		// o.ID is strfmt.UUID; compare using its string form
		if o.ID.String() == objID {
			if o.Properties != nil {
				if m, ok := o.Properties.(map[string]any); ok {
					props = m
				}
			}
			found = true
			break
		}
	}
	if !found || props == nil {
		return nil, nil
	}
	k := &KPIDefinition{ID: id}
	if v, ok := props["name"].(string); ok {
		k.Name = v
	}
	if v, ok := props["kind"].(string); ok {
		k.Kind = v
	}
	if v, ok := props["namespace"].(string); ok {
		k.Namespace = v
	}
	if v, ok := props["source"].(string); ok {
		k.Source = v
	}
	if v, ok := props["sourceId"].(string); ok {
		k.SourceID = v
	}
	if v, ok := props["unit"].(string); ok {
		k.Unit = v
	}
	if v, ok := props["format"].(string); ok {
		k.Format = v
	}
	if v, ok := props["query"].(map[string]any); ok {
		k.Query = v
	}
	// thresholds parsing intentionally omitted here; handlers convert when needed
	if v, ok := props["tags"].([]any); ok {
		for _, tv := range v {
			if sstr, ok := tv.(string); ok {
				k.Tags = append(k.Tags, sstr)
			}
		}
	}
	if v, ok := props["definition"].(string); ok {
		k.Definition = v
	}
	if v, ok := props["sentiment"].(string); ok {
		k.Sentiment = v
	}
	if v, ok := props["category"].(string); ok {
		k.Category = v
	}
	if v, ok := props["retryAllowed"].(bool); ok {
		k.RetryAllowed = v
	}
	if v, ok := props["domain"].(string); ok {
		k.Domain = v
	}
	if v, ok := props["serviceFamily"].(string); ok {
		k.ServiceFamily = v
	}
	if v, ok := props["componentType"].(string); ok {
		k.ComponentType = v
	}
	if v, ok := props["sparkline"].(map[string]any); ok {
		k.Sparkline = v
	}
	if v, ok := props["visibility"].(string); ok {
		k.Visibility = v
	}
	if v, ok := props["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			k.CreatedAt = t
		}
	}
	if v, ok := props["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			k.UpdatedAt = t
		}
	}
	if v, ok := props["layer"].(string); ok {
		k.Layer = v
	}
	if v, ok := props["signalType"].(string); ok {
		k.SignalType = v
	}
	if v, ok := props["classifier"].(string); ok {
		k.Classifier = v
	}
	if v, ok := props["datastore"].(string); ok {
		k.Datastore = v
	}
	if v, ok := props["queryType"].(string); ok {
		k.QueryType = v
	}
	if v, ok := props["formula"].(string); ok {
		k.Formula = v
	}
	if v, ok := props["businessImpact"].(string); ok {
		k.BusinessImpact = v
	}
	if v, ok := props["emotionalImpact"].(string); ok {
		k.EmotionalImpact = v
	}
	if v, ok := props["examples"].([]any); ok {
		for _, ex := range v {
			if m, ok := ex.(map[string]any); ok {
				k.Examples = append(k.Examples, m)
			}
		}
	}

	// thresholds: convert nested properties back to models.Threshold
	if raw, ok := props["thresholds"]; ok {
		k.Thresholds = propsToThresholds(raw)
	}

	return k, nil
}

// ListKPIs returns objects for a simple pagination/filters request.
//
//nolint:gocyclo // Property parsing from Weaviate requires many field mappings
func (s *WeaviateKPIStore) ListKPIs(ctx context.Context, req *KPIListRequest) ([]*KPIDefinition, int64, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// Use ObjectsGetter to fetch class instances; apply limit/offset.
	resp, err := s.client.Data().ObjectsGetter().WithClassName("KPIDefinition").WithLimit(int(limit)).WithOffset(offset).Do(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*KPIDefinition, 0, len(resp))
	for _, o := range resp {
		if o == nil {
			continue
		}
		k := &KPIDefinition{}

		// Extract all properties from the Weaviate object
		var props map[string]any
		if o.Properties != nil {
			if m, ok := o.Properties.(map[string]any); ok {
				props = m
			}
		}

		if props != nil {
			// Map all fields (matching GetKPI pattern)
			if v, ok := props["name"].(string); ok {
				k.Name = v
			}
			if v, ok := props["kind"].(string); ok {
				k.Kind = v
			}
			if v, ok := props["namespace"].(string); ok {
				k.Namespace = v
			}
			if v, ok := props["source"].(string); ok {
				k.Source = v
			}
			if v, ok := props["sourceId"].(string); ok {
				k.SourceID = v
			}
			if v, ok := props["unit"].(string); ok {
				k.Unit = v
			}
			if v, ok := props["format"].(string); ok {
				k.Format = v
			}
			if v, ok := props["query"].(map[string]any); ok {
				k.Query = v
			}
			if v, ok := props["layer"].(string); ok {
				k.Layer = v
			}
			if v, ok := props["signalType"].(string); ok {
				k.SignalType = v
			}
			if v, ok := props["classifier"].(string); ok {
				k.Classifier = v
			}
			if v, ok := props["datastore"].(string); ok {
				k.Datastore = v
			}
			if v, ok := props["queryType"].(string); ok {
				k.QueryType = v
			}
			if v, ok := props["formula"].(string); ok {
				k.Formula = v
			}
			if v, ok := props["tags"].([]any); ok {
				for _, tv := range v {
					if sstr, ok := tv.(string); ok {
						k.Tags = append(k.Tags, sstr)
					}
				}
			}
			if v, ok := props["definition"].(string); ok {
				k.Definition = v
			}
			if v, ok := props["sentiment"].(string); ok {
				k.Sentiment = v
			}
			if v, ok := props["category"].(string); ok {
				k.Category = v
			}
			if v, ok := props["retryAllowed"].(bool); ok {
				k.RetryAllowed = v
			}
			if v, ok := props["domain"].(string); ok {
				k.Domain = v
			}
			if v, ok := props["serviceFamily"].(string); ok {
				k.ServiceFamily = v
			}
			if v, ok := props["componentType"].(string); ok {
				k.ComponentType = v
			}
			if v, ok := props["businessImpact"].(string); ok {
				k.BusinessImpact = v
			}
			if v, ok := props["emotionalImpact"].(string); ok {
				k.EmotionalImpact = v
			}
			if v, ok := props["examples"].([]any); ok {
				for _, ex := range v {
					if m, ok := ex.(map[string]any); ok {
						k.Examples = append(k.Examples, m)
					}
				}
			}
			if v, ok := props["sparkline"].(map[string]any); ok {
				k.Sparkline = v
			}
			if v, ok := props["visibility"].(string); ok {
				k.Visibility = v
			}
			if v, ok := props["createdAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
					k.CreatedAt = t
				}
			}
			if v, ok := props["updatedAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
					k.UpdatedAt = t
				}
			}
			if raw, ok := props["thresholds"]; ok {
				k.Thresholds = propsToThresholds(raw)
			}

			// ID handling
			if vid, ok := props["id"].(string); ok && vid != "" {
				k.ID = vid
			} else if add := o.ID.String(); add != "" {
				k.ID = add
			}
		} else if add := o.ID.String(); add != "" {
			k.ID = add
		}

		out = append(out, k)
	}
	// By default, set total to the number of items returned in this page.
	total := int64(len(out))

	// Try to fetch the full count of KPIDefinition objects from Weaviate.
	// This is a best-effort call: if it fails, fall back to the page length
	// to avoid breaking callers. Counting requires an additional SDK call.
	// Note: Weaviate ObjectsGetter has a default limit of 25, so we set a high limit
	// to get an accurate count. For very large datasets, consider using GraphQL aggregation.
	if all, terr := s.client.Data().ObjectsGetter().WithClassName("KPIDefinition").WithLimit(maxKPIListLimit).Do(ctx); terr == nil {
		total = int64(len(all))
	} else {
		s.logger.Warn("weaviate: failed to get total KPI count; falling back to page size", zap.Error(terr))
	}

	return out, total, nil
}

// thresholdsToProps converts Threshold slice into the Weaviate nested
// property representation expected by the KPIDefinition class schema.
func thresholdsToProps(ths []Threshold) []map[string]any {
	if len(ths) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(ths))
	for _, t := range ths {
		m := map[string]any{
			"operator": t.Operator,
			"value":    t.Value,
			"severity": t.Level,
			"message":  t.Description,
		}
		out = append(out, m)
	}
	return out
}

// propsToThresholds converts a raw thresholds property (from Weaviate) into
// []Threshold. The raw value can be []map[string]any, []interface{}, or
// other types depending on the SDK decoding.
func propsToThresholds(raw any) []Threshold {
	if raw == nil {
		return nil
	}
	// Prefer []map[string]any if present
	if arr, ok := raw.([]map[string]any); ok {
		return convertFromMapArray(arr)
	}
	// Handle []any / []interface{} where each element is a map
	if arr, ok := raw.([]any); ok {
		out := make([]Threshold, 0, len(arr))
		for _, it := range arr {
			if m, ok := it.(map[string]any); ok {
				out = append(out, mapToThreshold(m))
				continue
			}
			if m2, ok := it.(map[string]interface{}); ok {
				mam := make(map[string]any, len(m2))
				for kk, vv := range m2 {
					mam[kk] = vv
				}
				out = append(out, mapToThreshold(mam))
				continue
			}
		}
		return out
	}
	return nil
}

func convertFromMapArray(arr []map[string]any) []Threshold {
	out := make([]Threshold, 0, len(arr))
	for _, m := range arr {
		out = append(out, mapToThreshold(m))
	}
	return out
}

func mapToThreshold(m map[string]any) Threshold {
	var th Threshold
	if s, ok := m["severity"].(string); ok {
		th.Level = s
	} else if s, ok := m["level"].(string); ok {
		th.Level = s
	}
	if op, ok := m["operator"].(string); ok {
		th.Operator = op
	}
	switch val := m["value"].(type) {
	case float64:
		th.Value = val
	case int:
		th.Value = float64(val)
	}
	if msg, ok := m["message"].(string); ok {
		th.Description = msg
	} else if msg, ok := m["description"].(string); ok {
		th.Description = msg
	}
	return th
}

// Public API methods that work with models package types
// These are wrappers that convert between models and weavstore types

// CreateOrUpdateKPIModels is a wrapper that accepts and returns models.KPIDefinition
func (s *WeaviateKPIStore) CreateOrUpdateKPIModels(ctx context.Context, k *KPIDefinition) (*KPIDefinition, string, error) {
	return s.CreateOrUpdateKPI(ctx, k)
}

// GetKPIModels is a wrapper that returns models.KPIDefinition
func (s *WeaviateKPIStore) GetKPIModels(ctx context.Context, id string) (*KPIDefinition, error) {
	return s.GetKPI(ctx, id)
}

// ListKPIsModels is a wrapper that works with models types
func (s *WeaviateKPIStore) ListKPIsModels(ctx context.Context, req *KPIListRequest) ([]*KPIDefinition, int64, error) {
	return s.ListKPIs(ctx, req)
}
