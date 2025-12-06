package weavstore

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
	wv "github.com/weaviate/weaviate-go-client/v5/weaviate"
	wm "github.com/weaviate/weaviate/entities/models"
	"go.uber.org/zap"
)

// WeaviateKPIStore is a small wrapper around the official weaviate v5 client
// for KPI-specific operations. It centralizes all KPI Weaviate access via the
// SDK (no raw HTTP/GraphQL strings).
type WeaviateKPIStore struct {
	client *wv.Client
	logger *zap.Logger
	// schemaInit ensures we attempt to create the required class only once
	schemaInit sync.Once
	schemaErr  error
	// Vectorizer configuration for schema creation and indexing
	vectorizerProvider string
	vectorizerModel    string
	vectorizerUseGPU   bool
}

// KPIStore describes the subset of operations a KPI store must implement.
// This interface allows repo-level code to be tested using simple fakes.
type KPIStore interface {
	CreateOrUpdateKPI(ctx context.Context, k *KPIDefinition) (*KPIDefinition, string, error)
	GetKPI(ctx context.Context, id string) (*KPIDefinition, error)
	ListKPIs(ctx context.Context, req *KPIListRequest) ([]*KPIDefinition, int64, error)
	// SearchKPIs performs a search over KPIs. Implementations may use
	// Weaviate's vector/semantic search (nearText/nearVector), or a fallback
	// keyword search when semantic support is not available.
	SearchKPIs(ctx context.Context, req *KPISearchRequest) ([]*KPISearchResult, int64, error)
	DeleteKPI(ctx context.Context, id string) error
}

// KPISearchRequest describes user-facing search request options
type KPISearchRequest struct {
	Query   string          `json:"query"`
	Filters *KPIListRequest `json:"filters,omitempty"`
	Mode    string          `json:"mode,omitempty"` // semantic|keyword|hybrid
	Limit   int64           `json:"limit,omitempty"`
	Offset  int64           `json:"offset,omitempty"`
	Explain bool            `json:"explain,omitempty"`
}

// KPISearchResult includes the matched KPI and auxiliary scoring / highlight data
type KPISearchResult struct {
	KPI            *KPIDefinition `json:"kpi,omitempty"`
	Score          float64        `json:"score,omitempty"`
	MatchingFields []string       `json:"matchingFields,omitempty"`
	Highlights     []string       `json:"highlights,omitempty"`
}

// NewWeaviateKPIStore constructs a new KPI store.
func NewWeaviateKPIStore(client *wv.Client, logger *zap.Logger, vectorizerProvider string, vectorizerModel string, vectorizerUseGPU bool) *WeaviateKPIStore {
	return &WeaviateKPIStore{client: client, logger: logger, vectorizerProvider: vectorizerProvider, vectorizerModel: vectorizerModel, vectorizerUseGPU: vectorizerUseGPU}
}

// helper: object id generation consistent with previous behavior
var (
	nsMirador = func() uuid.UUID {
		u, _ := uuid.FromString("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
		return u
	}()

	// Static errors for err113 compliance
	ErrKPIIsNil                    = errors.New("kpi is nil")
	ErrKPIIDEmpty                  = errors.New("kpi id is empty")
	ErrIDEmpty                     = errors.New("id is empty")
	ErrWeaviateDeleteAndScanFailed = errors.New("weaviate delete attempts failed and object scan failed")
	ErrWeaviateDeleteFailed        = errors.New("weaviate delete attempts failed for id")
	ErrWeaviateClientNil           = errors.New("weaviate client is nil")

	// ErrKPIDefinitionClassMissing indicates the Weaviate runtime schema does
	// not contain the KPIDefinition class. Returned by read/list operations
	// when the schema is not yet present in the runtime.
	ErrKPIDefinitionClassMissing = errors.New("weaviate: KPIDefinition class not found")

	// maxKPIListLimit is the maximum limit for listing KPIs in a single query
	maxKPIListLimit = 10000
)

// Class name migration: use a new runtime class name for KPI objects
const (
	kpiClassOld = "KPIDefinition"  // legacy runtime class
	kpiClassNew = "kpi_definition" // new runtime class (preferred for new writes)
)

// makeObjectID returns a deterministic object id for the provided id.
// New objects are generated using the new class-name seed so they live under
// the `kpi_definition` class. Legacy objects previously used "KPIDefinition|%s".
func makeObjectID(id string) string {
	// use the new seed which includes the class name to avoid collisions and
	// keep determinism for migrated/new objects.
	return uuid.NewV5(nsMirador, fmt.Sprintf("%s|%s", kpiClassNew, id)).String()
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

	// Ensure the runtime Weaviate schema contains the KPIDefinition class.
	s.schemaInit.Do(func() {
		s.schemaErr = s.ensureKPIDefinitionClass(ctx)
		if s.schemaErr != nil && s.logger != nil {
			s.logger.Sugar().Warnf("weavstore: failed ensuring KPIDefinition class: %v", s.schemaErr)
		}
	})
	if s.schemaErr != nil {
		// Return schema initialization error to avoid ambiguous 404/422 from Weaviate later.
		return nil, "", s.schemaErr
	}

	objID := makeObjectID(k.ID)

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
		// content is a concatenation of human-friendly fields that are
		// useful for semantic vectorization/search (name, definition, formula, tags, examples, businessImpact)
		"content": kpiContent(k),
	}
	// Check if object already exists in Weaviate and capture the actual
	// Weaviate object id (o.ID) when present. This is important because the
	// runtime may contain objects stored under either the legacy/raw KPI id
	// or the deterministic v5 object id; updates must target the actual
	// stored object id to avoid 'no object with id' errors from Weaviate.
	foundObjID, existing, foundClass, err := s.getKPIWithObjectID(ctx, k.ID)
	if err != nil {
		return nil, "", err
	}

	// If object exists and is identical field-by-field, treat as success (no-op)
	if existing != nil {
		if kpiEqual(existing, k) {
			// No change detected; return existing as success
			return existing, "no-change", nil
		}
		// There is a modification: perform update against the actual Weaviate
		// object id (foundObjID) in the class it was found. If for some reason
		// foundObjID is empty, fall back to the deterministic objID. If the
		// foundClass is empty (shouldn't happen) prefer the new class.
		targetID := objID
		if foundObjID != "" {
			targetID = foundObjID
		}
		targetClass := kpiClassNew
		if foundClass != "" {
			targetClass = foundClass
		}
		if err := s.client.Data().Updater().WithClassName(targetClass).WithID(targetID).WithProperties(props).Do(ctx); err != nil {
			return nil, "", err
		}
		return k, "updated", nil
	}

	// Not found -> create. If create fails because the object already exists,
	// fall back to updating the existing object. This handles races where the
	// object was created between the GetKPI call and the Creator() call and
	// avoids returning a 422 'id already exists' to callers.
	if _, err := s.client.Data().Creator().WithClassName(kpiClassNew).WithID(objID).WithProperties(props).Do(ctx); err != nil {
		// Some Weaviate error responses include messages like "id '...' already exists"
		// or mention "already exists"; handle those conservatively by attempting
		// an update instead of failing the whole operation.
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "id already exists") {
			// Try an update in the new class first, then fall back to the legacy
			// class if that fails. Capture the last error to report if both
			// update attempts fail.
			var lastErr error
			if err2 := s.client.Data().Updater().WithClassName(kpiClassNew).WithID(objID).WithProperties(props).Do(ctx); err2 == nil {
				return k, "updated", nil
			} else {
				lastErr = err2
			}
			if err3 := s.client.Data().Updater().WithClassName(kpiClassOld).WithID(objID).WithProperties(props).Do(ctx); err3 == nil {
				return k, "updated", nil
			} else {
				lastErr = err3
			}
			return nil, "", fmt.Errorf("create conflict: update also failed: %v (create err: %v)", lastErr, err)
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
	objID := makeObjectID(id)

	// Try delete by deterministic v5 object id first
	if s.tryDeleteByObjectID(ctx, id, objID) {
		return nil
	}

	// Next try delete using the raw id as object id (legacy objects)
	if s.tryDeleteByRawID(ctx, id) {
		return nil
	}

	// As a last resort, scan KPIDefinition objects
	if s.tryDeleteByScan(ctx, id, objID) {
		return nil
	}

	return fmt.Errorf("%w: id=%s (tried objID=%s and raw id)", ErrWeaviateDeleteFailed, id, objID)
}

// tryDeleteByObjectID attempts to delete using the deterministic v5 object ID
func (s *WeaviateKPIStore) tryDeleteByObjectID(ctx context.Context, id, objID string) bool {
	s.logf("weavstore: attempting delete for KPI id=%s (objID=%s)", id, objID)

	if err := s.client.Data().Deleter().WithClassName(kpiClassNew).WithID(objID).Do(ctx); err == nil {
		s.logf("weavstore: deleted by v5 objID=%s", objID)
		if s.verifyDeletion(ctx, id, objID) {
			return true
		}
	} else {
		s.logf("weavstore: delete by v5 objID failed: %v", err)
	}
	return false
}

// tryDeleteByRawID attempts to delete using the raw ID (legacy objects)
func (s *WeaviateKPIStore) tryDeleteByRawID(ctx context.Context, id string) bool {
	rawID := id
	if uuidObj, err := uuid.FromString(id); err == nil {
		rawID = uuidObj.String()
	}

	// Try new class first for raw ids, then fall back to legacy class
	if err2 := s.client.Data().Deleter().WithClassName(kpiClassNew).WithID(rawID).Do(ctx); err2 == nil {
		s.logf("weavstore: deleted by raw id=%s (new class)", rawID)
		if s.verifyDeletion(ctx, id, rawID) {
			return true
		}
	} else {
		s.logf("weavstore: delete by raw id (new class) failed: %v", err2)
	}
	if errOld := s.client.Data().Deleter().WithClassName(kpiClassOld).WithID(rawID).Do(ctx); errOld == nil {
		s.logf("weavstore: deleted by raw id=%s (legacy class)", rawID)
		if s.verifyDeletion(ctx, id, rawID) {
			return true
		}
	} else {
		s.logf("weavstore: delete by raw id (legacy class) failed: %v", errOld)
	}
	return false
}

// tryDeleteByScan scans for matching objects and deletes them
func (s *WeaviateKPIStore) tryDeleteByScan(ctx context.Context, id, objID string) bool {
	var objs []*wm.Object
	if respNew, errNew := s.client.Data().ObjectsGetter().WithClassName(kpiClassNew).Do(ctx); errNew == nil {
		objs = append(objs, respNew...)
	} else {
		s.logf("weavstore: new-class object scan failed: %v", errNew)
	}
	if respOld, errOld := s.client.Data().ObjectsGetter().WithClassName(kpiClassOld).Do(ctx); errOld == nil {
		objs = append(objs, respOld...)
	} else {
		s.logf("weavstore: legacy-class object scan failed: %v", errOld)
	}
	if len(objs) == 0 {
		return false
	}

	for _, o := range objs {
		if o == nil {
			continue
		}
		oid := o.ID.String()

		// Try match by object ID equality
		if oid == objID || oid == id {
			if s.tryDeleteAndVerify(ctx, id, oid) {
				return true
			}
		}

		// Try match by properties
		if s.tryDeleteByProperties(ctx, id, o) {
			return true
		}
	}
	return false
}

// tryDeleteAndVerify attempts a delete and verifies it succeeded
func (s *WeaviateKPIStore) tryDeleteAndVerify(ctx context.Context, id, oid string) bool {
	// attempt deletion on new class then legacy class
	if derr := s.client.Data().Deleter().WithClassName(kpiClassNew).WithID(oid).Do(ctx); derr == nil {
		s.logf("weavstore: deleted by scanning object id=%s (new class)", oid)
		if s.verifyDeletion(ctx, id, oid) {
			return true
		}
	} else {
		s.logf("weavstore: scan-delete (new class) attempt for oid=%s failed: %v", oid, derr)
	}
	if derrOld := s.client.Data().Deleter().WithClassName(kpiClassOld).WithID(oid).Do(ctx); derrOld == nil {
		s.logf("weavstore: deleted by scanning object id=%s (legacy class)", oid)
		if s.verifyDeletion(ctx, id, oid) {
			return true
		}
	} else {
		s.logf("weavstore: scan-delete (legacy class) attempt for oid=%s failed: %v", oid, derrOld)
	}
	return false
}

// tryDeleteByProperties attempts to match and delete by properties
func (s *WeaviateKPIStore) tryDeleteByProperties(ctx context.Context, id string, o *wm.Object) bool {
	if o.Properties == nil {
		return false
	}

	props, ok := o.Properties.(map[string]any)
	if !ok {
		return false
	}

	vid, ok := props["id"].(string)
	if !ok || vid != id {
		return false
	}

	oid := o.ID.String()
	// attempt delete in new class then legacy class
	if derr := s.client.Data().Deleter().WithClassName(kpiClassNew).WithID(oid).Do(ctx); derr == nil {
		s.logf("weavstore: deleted by scanning props match id=%s -> oid=%s (new class)", id, oid)
		if s.verifyDeletion(ctx, id, oid) {
			return true
		}
	} else {
		s.logf("weaviate: scan-delete (new class) by props for oid=%s failed: %v", oid, derr)
	}
	if derrOld := s.client.Data().Deleter().WithClassName(kpiClassOld).WithID(oid).Do(ctx); derrOld == nil {
		s.logf("weavstore: deleted by scanning props match id=%s -> oid=%s (legacy class)", id, oid)
		if s.verifyDeletion(ctx, id, oid) {
			return true
		}
	} else {
		s.logf("weaviate: scan-delete (legacy class) by props for oid=%s failed: %v", oid, derrOld)
	}
	return false
}

// verifyDeletion checks if an object was successfully deleted
func (s *WeaviateKPIStore) verifyDeletion(ctx context.Context, id, oid string) bool {
	if remaining, verifyErr := s.GetKPI(ctx, id); verifyErr == nil && remaining != nil {
		s.logf("weavstore: WARNING - object still exists after delete attempt by oid=%s", oid)
		return false
	} else if verifyErr == nil {
		s.logf("weavstore: verified deletion by oid=%s - object no longer found", oid)
		return true
	}
	return false
}

// logf is a helper to log via zap if available
func (s *WeaviateKPIStore) logf(format string, args ...interface{}) {
	if s.logger != nil {
		s.logger.Sugar().Infof(format, args...)
		return
	}
	fmt.Printf(format+"\n", args...)
}

func (s *WeaviateKPIStore) GetKPI(ctx context.Context, id string) (*KPIDefinition, error) {
	if id == "" {
		return nil, nil
	}
	objIDNew := makeObjectID(id)
	// Also compute legacy deterministic id used by older runtime objects
	objIDLegacy := uuid.NewV5(nsMirador, fmt.Sprintf("%s|%s", kpiClassOld, id)).String()

	// Fetch objects preferring the new class and falling back to the legacy
	// class. The ObjectsGetter returns a slice of objects in the SDK.
	resp, err := s.client.Data().ObjectsGetter().WithClassName(kpiClassNew).Do(ctx)
	if err != nil {
		// if the new class is missing, try the legacy class
		if isKPIDefinitionClassMissingErr(err) {
			resp, err = s.client.Data().ObjectsGetter().WithClassName(kpiClassOld).Do(ctx)
		}
	}
	if err != nil {
		if isKPIDefinitionClassMissingErr(err) {
			return nil, ErrKPIDefinitionClassMissing
		}
		return nil, err
	}

	var props map[string]any
	var found bool
	for _, o := range resp {
		if o == nil {
			continue
		}
		// o.ID is strfmt.UUID; compare using its string form
		// Support the deterministic v5 object id (new or legacy prefix) and the
		// case where the original KPI id was used as the Weaviate object id
		// directly.
		if o.ID.String() == objIDNew || o.ID.String() == objIDLegacy || o.ID.String() == id {
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
	k := parsePropsToKPI(props, id)
	return k, nil
}

// getKPIWithObjectID behaves like GetKPI but also returns the actual Weaviate
// object id (o.ID) for the matched object. Returning the object id allows
// callers to perform updates/deletes against the exact Weaviate object that
// was matched, avoiding mismatches when objects exist under legacy/raw ids
// or the deterministic v5 object ids.
// getKPIWithObjectID returns the actual Weaviate object id (o.ID), the parsed
// KPIDefinition and the runtime class that the object was found in (className).
func (s *WeaviateKPIStore) getKPIWithObjectID(ctx context.Context, id string) (string, *KPIDefinition, string, error) {
	if id == "" {
		return "", nil, "", nil
	}

	objIDNew := makeObjectID(id)
	objIDLegacy := uuid.NewV5(nsMirador, fmt.Sprintf("%s|%s", kpiClassOld, id)).String()

	// Try new class first and fall back to legacy class if missing. Track the
	// class we actually read from so callers can perform updates/deletes
	// against the exact class.
	readClass := kpiClassNew
	resp, err := s.client.Data().ObjectsGetter().WithClassName(readClass).Do(ctx)
	if err != nil {
		if isKPIDefinitionClassMissingErr(err) {
			readClass = kpiClassOld
			resp, err = s.client.Data().ObjectsGetter().WithClassName(readClass).Do(ctx)
		}
	}
	if err != nil {
		if isKPIDefinitionClassMissingErr(err) {
			return "", nil, "", ErrKPIDefinitionClassMissing
		}
		return "", nil, "", err
	}

	var props map[string]any
	var found bool
	var foundObjID string
	var foundClass string
	for _, o := range resp {
		if o == nil {
			continue
		}
		// Support the new and legacy deterministic v5 object id and raw id usage
		if o.ID.String() == objIDNew || o.ID.String() == objIDLegacy || o.ID.String() == id {
			if o.Properties != nil {
				if m, ok := o.Properties.(map[string]any); ok {
					props = m
				}
			}
			found = true
			foundObjID = o.ID.String()
			// capture the runtime class where the object was found
			foundClass = readClass
			break
		}
	}
	if !found || props == nil {
		return "", nil, "", nil
	}

	k := parsePropsToKPI(props, id)
	return foundObjID, k, foundClass, nil
}

// parsePropsToKPI converts a Weaviate properties map into a KPIDefinition.
func parsePropsToKPI(props map[string]any, id string) *KPIDefinition {
	k := &KPIDefinition{ID: id}

	parseBasicProps(props, k)
	parseMetadataProps(props, k)
	parseQueryProps(props, k)
	parseImpactProps(props, k)
	parseCollectionProps(props, k)
	parseTimestamps(props, k)

	return k
}

// parseBasicProps extracts basic string properties
func parseBasicProps(props map[string]any, k *KPIDefinition) {
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
	if v, ok := props["definition"].(string); ok {
		k.Definition = v
	}
	if v, ok := props["sentiment"].(string); ok {
		k.Sentiment = v
	}
	if v, ok := props["category"].(string); ok {
		k.Category = v
	}
	if v, ok := props["visibility"].(string); ok {
		k.Visibility = v
	}
}

// parseMetadataProps extracts metadata properties (domain, service, component)
func parseMetadataProps(props map[string]any, k *KPIDefinition) {
	if v, ok := props["domain"].(string); ok {
		k.Domain = v
	}
	if v, ok := props["serviceFamily"].(string); ok {
		k.ServiceFamily = v
	}
	if v, ok := props["componentType"].(string); ok {
		k.ComponentType = v
	}
	if v, ok := props["retryAllowed"].(bool); ok {
		k.RetryAllowed = v
	}
}

// parseQueryProps extracts query-related properties
func parseQueryProps(props map[string]any, k *KPIDefinition) {
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
	if v, ok := props["query"].(map[string]any); ok {
		k.Query = v
	}
	if v, ok := props["sparkline"].(map[string]any); ok {
		k.Sparkline = v
	}
}

// parseImpactProps extracts impact-related properties
func parseImpactProps(props map[string]any, k *KPIDefinition) {
	if v, ok := props["businessImpact"].(string); ok {
		k.BusinessImpact = v
	}
	if v, ok := props["emotionalImpact"].(string); ok {
		k.EmotionalImpact = v
	}
}

// parseCollectionProps extracts collection properties (tags, examples, thresholds)
func parseCollectionProps(props map[string]any, k *KPIDefinition) {
	if v, ok := props["tags"].([]any); ok {
		for _, tv := range v {
			if sstr, ok := tv.(string); ok {
				k.Tags = append(k.Tags, sstr)
			}
		}
	}
	if v, ok := props["examples"].([]any); ok {
		for _, ex := range v {
			if m, ok := ex.(map[string]any); ok {
				k.Examples = append(k.Examples, m)
			}
		}
	}
	if raw, ok := props["thresholds"]; ok {
		k.Thresholds = propsToThresholds(raw)
	}
}

// parseTimestamps extracts timestamp properties
func parseTimestamps(props map[string]any, k *KPIDefinition) {
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
}

// kpiContent builds a single text surface used for vectorization/search by
// concatenating several human-facing fields of the KPI definition.
func kpiContent(k *KPIDefinition) string {
	if k == nil {
		return ""
	}
	parts := make([]string, 0, 6)
	if k.Name != "" {
		parts = append(parts, k.Name)
	}
	if k.Definition != "" {
		parts = append(parts, k.Definition)
	}
	if k.Formula != "" {
		parts = append(parts, k.Formula)
	}
	if len(k.Tags) > 0 {
		parts = append(parts, strings.Join(k.Tags, " "))
	}
	if k.BusinessImpact != "" {
		parts = append(parts, k.BusinessImpact)
	}
	if k.EmotionalImpact != "" {
		parts = append(parts, k.EmotionalImpact)
	}
	// Convert examples to compact string representations
	for _, ex := range k.Examples {
		if ex != nil {
			parts = append(parts, fmt.Sprintf("%v", ex))
		}
	}
	return strings.Join(parts, " ")
}

// ensureKPIDefinitionClass checks whether the KPIDefinition class exists in
// Weaviate and attempts to create it if missing. This function is safe to call
// repeatedly; callers should use sync.Once to avoid repeated attempts.
func (s *WeaviateKPIStore) ensureKPIDefinitionClass(ctx context.Context) error {
	// Try to fetch the class via the SDK. If the SDK returns a non-nil class,
	// assume schema exists.
	if s.client == nil {
		return ErrWeaviateClientNil
	}

	// Try creating the class directly. If it already exists, treat as success.
	// This avoids relying on Getter API variations across client versions.
	// Build a minimal class definition matching runtime expectations.
	vec := s.vectorizerProvider
	if vec == "" {
		vec = "none"
	}

	classDef := &wm.Class{
		Class:      kpiClassNew,
		Vectorizer: vec,
		Properties: []*wm.Property{
			{Name: "name", DataType: []string{"string"}},
			{Name: "kind", DataType: []string{"string"}},
			{Name: "namespace", DataType: []string{"string"}},
			{Name: "source", DataType: []string{"string"}},
			{Name: "sourceId", DataType: []string{"string"}},
			{Name: "unit", DataType: []string{"string"}},
			{Name: "format", DataType: []string{"string"}},
			{Name: "query", DataType: []string{"text"}},
			{Name: "layer", DataType: []string{"string"}},
			{Name: "signalType", DataType: []string{"string"}},
			{Name: "classifier", DataType: []string{"string"}},
			{Name: "datastore", DataType: []string{"string"}},
			{Name: "queryType", DataType: []string{"string"}},
			{Name: "formula", DataType: []string{"string"}},
			{Name: "thresholds", DataType: []string{"text"}},
			{Name: "tags", DataType: []string{"string"}},
			{Name: "definition", DataType: []string{"text"}},
			{Name: "content", DataType: []string{"text"}},
			{Name: "sentiment", DataType: []string{"string"}},
			{Name: "category", DataType: []string{"string"}},
			{Name: "retryAllowed", DataType: []string{"boolean"}},
			{Name: "domain", DataType: []string{"string"}},
			{Name: "serviceFamily", DataType: []string{"string"}},
			{Name: "componentType", DataType: []string{"string"}},
			{Name: "businessImpact", DataType: []string{"text"}},
			{Name: "emotionalImpact", DataType: []string{"text"}},
			{Name: "examples", DataType: []string{"text"}},
			{Name: "sparkline", DataType: []string{"text"}},
			{Name: "visibility", DataType: []string{"string"}},
			{Name: "createdAt", DataType: []string{"date"}},
			{Name: "updatedAt", DataType: []string{"date"}},
		},
	}

	// Attempt to create the class. Some client Do() implementations return only
	// an error; handle both cases conservatively.
	if err := s.client.Schema().ClassCreator().WithClass(classDef).Do(ctx); err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "class already exists") {
			// Another process created it concurrently; treat as success.
			return nil
		}
		return fmt.Errorf("failed to create %s class in Weaviate: %w", kpiClassNew, err)
	}

	if s.logger != nil {
		s.logger.Sugar().Info("weavstore: created ", kpiClassNew, " class in Weaviate runtime schema")
	}
	return nil
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
	// prefer new class, fall back to legacy class if not present
	resp, err := s.client.Data().ObjectsGetter().WithClassName(kpiClassNew).WithLimit(int(limit)).WithOffset(offset).Do(ctx)
	if err != nil {
		if isKPIDefinitionClassMissingErr(err) {
			// try legacy class
			resp, err = s.client.Data().ObjectsGetter().WithClassName(kpiClassOld).WithLimit(int(limit)).WithOffset(offset).Do(ctx)
		}
		if err != nil {
			if isKPIDefinitionClassMissingErr(err) {
				return nil, 0, ErrKPIDefinitionClassMissing
			}
			return nil, 0, err
		}
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
	// Attempt to count objects across both the new and legacy classes so the
	// total reflects objects that may live in either class during migration.
	var count int
	if allNew, terrNew := s.client.Data().ObjectsGetter().WithClassName(kpiClassNew).WithLimit(maxKPIListLimit).Do(ctx); terrNew == nil {
		count += len(allNew)
	} else {
		s.logger.Warn("weaviate: failed to fetch new-class KPI objects for count", zap.Error(terrNew))
	}
	if allOld, terrOld := s.client.Data().ObjectsGetter().WithClassName(kpiClassOld).WithLimit(maxKPIListLimit).Do(ctx); terrOld == nil {
		count += len(allOld)
	} else {
		s.logger.Warn("weaviate: failed to fetch legacy-class KPI objects for count", zap.Error(terrOld))
	}
	if count > 0 {
		total = int64(count)
	}

	return out, total, nil
}

// SearchKPIs implements a lightweight search over KPIs. This initial
// implementation uses a simple keyword-based fallback when a semantic
// search via Weaviate is not configured/available. In future we should
// replace the keyword path with a proper nearText/nearVector GraphQL query
// to take advantage of Weaviate's vector capabilities.
func (s *WeaviateKPIStore) SearchKPIs(ctx context.Context, req *KPISearchRequest) ([]*KPISearchResult, int64, error) {
	if req == nil || strings.TrimSpace(req.Query) == "" {
		return []*KPISearchResult{}, 0, nil
	}

	// Normalize mode
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "hybrid"
	}

	// If a semantic vectorizer is configured and the client is present, we
	// could invoke GraphQL nearText here. For now, if provider is none or
	// client is missing we fallback to keyword scanning.
	if s.client == nil || s.vectorizerProvider == "" || s.vectorizerProvider == "none" {
		return s.keywordSearch(ctx, req)
	}

	// Try semantic mode with a simple GraphQL nearText query. If that fails
	// or unimplemented for this client we fall back to keyword search.
	// NOTE: Full GraphQL integration is an enhancement tracked in KP-002.
	// For now we attempt a simple fallback.
	if mode == "semantic" || mode == "hybrid" {
		// If GraphQL / nearText is not available or fails, fallback to keyword
		// implementation for now.
		// TODO: Implement proper Weaviate nearText GraphQL query using the SDK.
		res, total, err := s.keywordSearch(ctx, req)
		return res, total, err
	}

	// otherwise default to keyword search
	return s.keywordSearch(ctx, req)
}

// keywordSearch is a simple in-memory search for the provided query. This
// is a fallback that scans KPIs returned by ListKPIs and scores them by
// basic substring matches against name/definition/content.
func (s *WeaviateKPIStore) keywordSearch(ctx context.Context, req *KPISearchRequest) ([]*KPISearchResult, int64, error) {
	// Fetch a large page of KPIs for in-memory ranking. This is a best-effort
	// fallback and not intended for very large catalogs; we'll rely on
	// proper vector queries in production.
	listReq := &KPIListRequest{Limit: int64(maxKPIListLimit), Offset: 0}
	if req != nil && req.Filters != nil {
		// copy filters into listReq but preserve Limit/Offset semantics
		listReq = req.Filters
		if listReq.Limit == 0 {
			listReq.Limit = int64(maxKPIListLimit)
		}
	}

	items, _, err := s.ListKPIs(ctx, listReq)
	if err != nil {
		// If ListKPIs returns ErrKPIDefinitionClassMissing, bubble up that error
		if errors.Is(err, ErrKPIDefinitionClassMissing) {
			return []*KPISearchResult{}, 0, ErrKPIDefinitionClassMissing
		}
		return nil, 0, err
	}

	q := strings.ToLower(strings.TrimSpace(req.Query))
	results := make([]*KPISearchResult, 0, len(items))
	for _, k := range items {
		score := 0.0
		matching := make([]string, 0)
		highlights := make([]string, 0)

		if k == nil {
			continue
		}
		// name
		if strings.Contains(strings.ToLower(k.Name), q) {
			score += 0.6
			matching = append(matching, "name")
			highlights = append(highlights, excerpt(k.Name, q))
		}
		// definition
		if strings.Contains(strings.ToLower(k.Definition), q) {
			score += 0.4
			matching = appendIfMissing(matching, "definition")
			highlights = append(highlights, excerpt(k.Definition, q))
		}
		// content (composite)
		content := strings.ToLower(kpiContent(k))
		if content != "" && strings.Contains(content, q) {
			score += 0.3
			matching = appendIfMissing(matching, "content")
			// give a content highlight
			highlights = append(highlights, excerpt(content, q))
		}

		if score > 0 {
			if score > 1.0 {
				score = 1.0
			}
			results = append(results, &KPISearchResult{KPI: k, Score: score, MatchingFields: matching, Highlights: highlights})
		}
	}

	// Sort by score desc
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	total := int64(len(results))
	// apply offset/limit
	off := int(req.Offset)
	lim := int(req.Limit)
	if lim <= 0 {
		lim = 10
	}
	if off < 0 {
		off = 0
	}
	if off >= len(results) {
		return []*KPISearchResult{}, total, nil
	}
	end := off + lim
	if end > len(results) {
		end = len(results)
	}

	return results[off:end], total, nil
}

// appendIfMissing adds s to arr if not already present
func appendIfMissing(arr []string, s string) []string {
	for _, v := range arr {
		if v == s {
			return arr
		}
	}
	return append(arr, s)
}

// excerpt returns a short snippet containing q if present in text
func excerpt(text, q string) string {
	text = strings.TrimSpace(text)
	if q == "" || text == "" {
		return ""
	}
	li := strings.Index(strings.ToLower(text), strings.ToLower(q))
	if li == -1 {
		if len(text) > 140 {
			return text[:140] + "..."
		}
		return text
	}
	start := li - 40
	if start < 0 {
		start = 0
	}
	end := li + len(q) + 40
	if end > len(text) {
		end = len(text)
	}
	snippet := text[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}
	return snippet
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

// isKPIDefinitionClassMissingErr inspects a weaviate client error and returns
// true when the error indicates the KPIDefinition class/schema is missing.
// The weaviate SDK does not expose a typed error for this case, so we use
// conservative substring checks. This helper centralizes detection for tests
// and repo-level logic.
func isKPIDefinitionClassMissingErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "class not found") || strings.Contains(s, "class 'kpidefinition'") || strings.Contains(s, "class \"kpidefinition\"") || strings.Contains(s, "kpidefinition class") || strings.Contains(s, "no such class") || strings.Contains(s, "class does not exist") || strings.Contains(s, "not found: kpidefinition") {
		return true
	}
	// Also match patterns like "unknown class 'KPIDefinition'" or HTTP 404
	if strings.Contains(s, "kpidefinition") && strings.Contains(s, "not") {
		return true
	}
	return false
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
