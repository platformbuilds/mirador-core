package weavstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	wv "github.com/weaviate/weaviate-go-client/v5/weaviate"
	wm "github.com/weaviate/weaviate/entities/models"
	"go.uber.org/zap"
)

// WeaviateFailureStore is a wrapper around the official weaviate v5 client
// for Failure-specific operations. It centralizes all Failure Weaviate access via the
// SDK (no raw HTTP/GraphQL strings).
type WeaviateFailureStore struct {
	client *wv.Client
	logger *zap.Logger
	// schemaInit ensures we attempt to create the required class only once
	schemaInit sync.Once
	schemaErr  error
}

// NewWeaviateFailureStore constructs a new Failure store.
func NewWeaviateFailureStore(client *wv.Client, logger *zap.Logger) *WeaviateFailureStore {
	return &WeaviateFailureStore{client: client, logger: logger}
}

// Static errors for err113 compliance
var (
	ErrFailureIsNil                       = errors.New("failure is nil")
	ErrFailureUUIDEmpty                   = errors.New("failure uuid is empty")
	ErrWeaviateFailureDeleteAndScanFailed = errors.New("weaviate delete attempts failed and object scan failed")
	ErrWeaviateFailureDeleteFailed        = errors.New("weaviate delete attempts failed for failure_uuid")
)

func makeFailureObjectID(uuid string) string {
	return uuid // Use failure_uuid directly as Weaviate object ID for deterministic lookups
}

// CreateOrUpdateFailure creates a Failure record if missing, updates if present. It returns
// the Failure model, a status string ("created","updated","no-change"), and an error.
func (s *WeaviateFailureStore) CreateOrUpdateFailure(ctx context.Context, f *FailureRecord) (*FailureRecord, string, error) {
	if f == nil {
		return nil, "", ErrFailureIsNil
	}
	if f.FailureUUID == "" {
		return nil, "", ErrFailureUUIDEmpty
	}

	// Ensure the runtime Weaviate schema contains the FailureRecord class.
	s.schemaInit.Do(func() {
		s.schemaErr = s.ensureFailureRecordClass(ctx)
		if s.schemaErr != nil && s.logger != nil {
			s.logger.Sugar().Warnf("weavstore: failed ensuring FailureRecord class: %v", s.schemaErr)
		}
	})
	if s.schemaErr != nil {
		// Return schema initialization error to avoid ambiguous 404/422 from Weaviate later.
		return nil, "", s.schemaErr
	}

	objID := makeFailureObjectID(f.FailureUUID)

	props := map[string]any{
		"failureUuid":        f.FailureUUID,
		"failureId":          f.FailureID,
		"startTime":          f.TimeRange.Start.Format(time.RFC3339Nano),
		"endTime":            f.TimeRange.End.Format(time.RFC3339Nano),
		"services":           f.Services,
		"components":         f.Components,
		"rawErrorSignals":    failureSignalsToMapArray(f.RawErrorSignals),
		"rawAnomalySignals":  failureSignalsToMapArray(f.RawAnomalySignals),
		"detectionTimestamp": f.DetectionTimestamp.Format(time.RFC3339Nano),
		"detectorVersion":    f.DetectorVersion,
		"confidenceScore":    f.ConfidenceScore,
		"createdAt":          f.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":          f.UpdatedAt.Format(time.RFC3339Nano),
	}

	// Check if object already exists in Weaviate
	existing, err := s.GetFailure(ctx, f.FailureUUID)
	if err != nil {
		return nil, "", err
	}

	// If object exists and is identical field-by-field, treat as success (no-op)
	if existing != nil {
		if failureEqual(existing, f) {
			// No change detected; return existing as success
			return existing, "no-change", nil
		}
		// There is a modification: perform update
		if err := s.client.Data().Updater().WithClassName("FailureRecord").WithID(objID).WithProperties(props).Do(ctx); err != nil {
			return nil, "", err
		}
		return f, "updated", nil
	}

	// Not found -> create. If create fails because the object already exists,
	// fall back to updating the existing object. This handles races where the
	// object was created between the GetFailure call and the Creator() call and
	// avoids returning a 422 'id already exists' to callers.
	if _, err := s.client.Data().Creator().WithClassName("FailureRecord").WithID(objID).WithProperties(props).Do(ctx); err != nil {
		// Some Weaviate error responses include messages like "id '...' already exists"
		// or mention "already exists"; handle those conservatively by attempting
		// an update instead of failing the whole operation.
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "id already exists") {
			if err2 := s.client.Data().Updater().WithClassName("FailureRecord").WithID(objID).WithProperties(props).Do(ctx); err2 != nil {
				return nil, "", fmt.Errorf("create conflict: update also failed: %w (create err: %v)", err2, err)
			}
			return f, "updated", nil
		}
		return nil, "", err
	}
	return f, "created", nil
}

// failureEqual compares two FailureRecord objects field-by-field. It is intentionally
// conservative: it returns false if any field differs. This allows callers to
// detect and apply updates only when necessary.
//
//nolint:gocyclo // Field-by-field comparison is inherently complex
func failureEqual(a, b *FailureRecord) bool {
	if a == nil || b == nil {
		return false
	}
	if a.FailureUUID != b.FailureUUID {
		return false
	}
	if a.FailureID != b.FailureID {
		return false
	}
	if !a.TimeRange.Start.Equal(b.TimeRange.Start) || !a.TimeRange.End.Equal(b.TimeRange.End) {
		return false
	}
	if len(a.Services) != len(b.Services) {
		return false
	}
	for i := range a.Services {
		if a.Services[i] != b.Services[i] {
			return false
		}
	}
	if len(a.Components) != len(b.Components) {
		return false
	}
	for i := range a.Components {
		if a.Components[i] != b.Components[i] {
			return false
		}
	}
	if a.DetectorVersion != b.DetectorVersion {
		return false
	}
	if a.ConfidenceScore != b.ConfidenceScore {
		return false
	}
	// Compare raw signals
	if len(a.RawErrorSignals) != len(b.RawErrorSignals) {
		return false
	}
	for i := range a.RawErrorSignals {
		if !failureSignalEqual(&a.RawErrorSignals[i], &b.RawErrorSignals[i]) {
			return false
		}
	}
	if len(a.RawAnomalySignals) != len(b.RawAnomalySignals) {
		return false
	}
	for i := range a.RawAnomalySignals {
		if !failureSignalEqual(&a.RawAnomalySignals[i], &b.RawAnomalySignals[i]) {
			return false
		}
	}
	return true
}

// failureSignalEqual compares two FailureSignal objects
func failureSignalEqual(a, b *FailureSignal) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.SignalType != b.SignalType {
		return false
	}
	if a.MetricName != b.MetricName {
		return false
	}
	if a.Service != b.Service {
		return false
	}
	if a.Component != b.Component {
		return false
	}
	if !deepEqualAny(a.Data, b.Data) {
		return false
	}
	if !a.Timestamp.Equal(b.Timestamp) {
		return false
	}
	return true
}

// DeleteFailure removes a failure record by its failure_uuid
func (s *WeaviateFailureStore) DeleteFailure(ctx context.Context, failureUUID string) error {
	if failureUUID == "" {
		return ErrFailureUUIDEmpty
	}

	// Ensure the runtime Weaviate schema contains the FailureRecord class.
	s.schemaInit.Do(func() {
		s.schemaErr = s.ensureFailureRecordClass(ctx)
		if s.schemaErr != nil && s.logger != nil {
			s.logger.Sugar().Warnf("weavstore: failed ensuring FailureRecord class: %v", s.schemaErr)
		}
	})
	if s.schemaErr != nil {
		return s.schemaErr
	}

	objID := makeFailureObjectID(failureUUID)

	// Try delete by deterministic object id first
	if s.tryDeleteFailureByObjectID(ctx, failureUUID, objID) {
		return nil
	}

	// As a last resort, scan FailureRecord objects
	if s.tryDeleteFailureByScan(ctx, failureUUID, objID) {
		return nil
	}

	return fmt.Errorf("%w: uuid=%s (tried objID=%s)", ErrWeaviateFailureDeleteFailed, failureUUID, objID)
}

// DeleteFailureByID removes a failure record by its human-readable FailureID (not UUID)
func (s *WeaviateFailureStore) DeleteFailureByID(ctx context.Context, failureID string) error {
	if failureID == "" {
		return ErrFailureUUIDEmpty
	}

	// Ensure the runtime Weaviate schema contains the FailureRecord class.
	s.schemaInit.Do(func() {
		s.schemaErr = s.ensureFailureRecordClass(ctx)
		if s.schemaErr != nil && s.logger != nil {
			s.logger.Sugar().Warnf("weavstore: failed ensuring FailureRecord class: %v", s.schemaErr)
		}
	})
	if s.schemaErr != nil {
		return s.schemaErr
	}

	// Fetch all FailureRecord objects and find matching failureId
	objs, err := s.client.Data().ObjectsGetter().WithClassName("FailureRecord").Do(ctx)
	if err != nil {
		s.logf("weavstore: failure object scan failed for deletion by ID: %v", err)
		return err
	}

	for _, o := range objs {
		if o == nil || o.Properties == nil {
			continue
		}

		props, ok := o.Properties.(map[string]any)
		if !ok {
			continue
		}

		// Check if this object's failureId matches
		if fid, ok := props["failureId"].(string); ok && fid == failureID {
			oid := o.ID.String()
			if err := s.client.Data().Deleter().WithClassName("FailureRecord").WithID(oid).Do(ctx); err == nil {
				s.logf("weavstore: deleted failure by ID=%s (objID=%s)", failureID, oid)
				return nil
			} else {
				s.logf("weavstore: delete failure by ID=%s (objID=%s) failed: %v", failureID, oid, err)
				return err
			}
		}
	}

	return fmt.Errorf("failure not found with ID=%s", failureID)
}

// tryDeleteFailureByObjectID attempts to delete using the object ID
func (s *WeaviateFailureStore) tryDeleteFailureByObjectID(ctx context.Context, failureUUID, objID string) bool {
	s.logf("weavstore: attempting delete for failure uuid=%s (objID=%s)", failureUUID, objID)

	if err := s.client.Data().Deleter().WithClassName("FailureRecord").WithID(objID).Do(ctx); err == nil {
		s.logf("weavstore: deleted failure by objID=%s", objID)
		if s.verifyFailureDeletion(ctx, failureUUID, objID) {
			return true
		}
	} else {
		s.logf("weavstore: delete failure by objID failed: %v", err)
	}
	return false
}

// tryDeleteFailureByScan scans for matching objects and deletes them
func (s *WeaviateFailureStore) tryDeleteFailureByScan(ctx context.Context, failureUUID, objID string) bool {
	objs, gerr := s.client.Data().ObjectsGetter().WithClassName("FailureRecord").Do(ctx)
	if gerr != nil {
		s.logf("weavstore: failure object scan failed: %v", gerr)
		return false
	}

	for _, o := range objs {
		if o == nil {
			continue
		}
		oid := o.ID.String()

		// Try match by object ID equality
		if oid == objID || oid == failureUUID {
			if s.tryDeleteFailureAndVerify(ctx, failureUUID, oid) {
				return true
			}
		}

		// Try match by properties
		if s.tryDeleteFailureByProperties(ctx, failureUUID, o) {
			return true
		}
	}
	return false
}

// tryDeleteFailureAndVerify attempts a delete and verifies it succeeded
func (s *WeaviateFailureStore) tryDeleteFailureAndVerify(ctx context.Context, failureUUID, oid string) bool {
	if derr := s.client.Data().Deleter().WithClassName("FailureRecord").WithID(oid).Do(ctx); derr == nil {
		s.logf("weavstore: deleted failure by scanning object id=%s", oid)
		if s.verifyFailureDeletion(ctx, failureUUID, oid) {
			return true
		}
	} else {
		s.logf("weavstore: scan-delete attempt for failure oid=%s failed: %v", oid, derr)
	}
	return false
}

// tryDeleteFailureByProperties attempts to match and delete by properties
func (s *WeaviateFailureStore) tryDeleteFailureByProperties(ctx context.Context, failureUUID string, o *wm.Object) bool {
	if o.Properties == nil {
		return false
	}

	props, ok := o.Properties.(map[string]any)
	if !ok {
		return false
	}

	vid, ok := props["failureUuid"].(string)
	if !ok || vid != failureUUID {
		return false
	}

	oid := o.ID.String()
	if derr := s.client.Data().Deleter().WithClassName("FailureRecord").WithID(oid).Do(ctx); derr == nil {
		s.logf("weavstore: deleted failure by scanning props match uuid=%s -> oid=%s", failureUUID, oid)
		if s.verifyFailureDeletion(ctx, failureUUID, oid) {
			return true
		}
	} else {
		s.logf("weaviate: scan-delete failure by props for oid=%s failed: %v", oid, derr)
	}
	return false
}

// verifyFailureDeletion checks if a failure record was successfully deleted
func (s *WeaviateFailureStore) verifyFailureDeletion(ctx context.Context, failureUUID, oid string) bool {
	if remaining, verifyErr := s.GetFailure(ctx, failureUUID); verifyErr == nil && remaining != nil {
		s.logf("weavstore: WARNING - failure object still exists after delete attempt by oid=%s", oid)
		return false
	} else if verifyErr == nil {
		s.logf("weavstore: verified deletion of failure by oid=%s - object no longer found", oid)
		return true
	}
	return false
}

// logf is a helper to log via zap if available
func (s *WeaviateFailureStore) logf(format string, args ...interface{}) {
	if s.logger != nil {
		s.logger.Sugar().Infof(format, args...)
		return
	}
	fmt.Printf(format+"\n", args...)
}

//nolint:gocyclo // Property parsing from Weaviate requires many field mappings
func (s *WeaviateFailureStore) GetFailure(ctx context.Context, failureUUID string) (*FailureRecord, error) {
	if failureUUID == "" {
		return nil, nil
	}

	// Ensure the runtime Weaviate schema contains the FailureRecord class.
	s.schemaInit.Do(func() {
		s.schemaErr = s.ensureFailureRecordClass(ctx)
		if s.schemaErr != nil && s.logger != nil {
			s.logger.Sugar().Warnf("weavstore: failed ensuring FailureRecord class: %v", s.schemaErr)
		}
	})
	if s.schemaErr != nil {
		return nil, s.schemaErr
	}

	objID := makeFailureObjectID(failureUUID)

	// Fetch objects of the class and search for matching object id. The
	// ObjectsGetter returns a slice of objects in the SDK.
	resp, err := s.client.Data().ObjectsGetter().WithClassName("FailureRecord").Do(ctx)
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
		// Support both the deterministic object id and the case
		// where the failure UUID was used as the Weaviate object id directly.
		if o.ID.String() == objID || o.ID.String() == failureUUID {
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

	f := &FailureRecord{FailureUUID: failureUUID}

	if v, ok := props["failureId"].(string); ok {
		f.FailureID = v
	}
	if v, ok := props["startTime"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.TimeRange.Start = t
		}
	}
	if v, ok := props["endTime"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.TimeRange.End = t
		}
	}
	if v, ok := props["services"].([]any); ok {
		for _, svc := range v {
			if s, ok := svc.(string); ok {
				f.Services = append(f.Services, s)
			}
		}
	}
	if v, ok := props["components"].([]any); ok {
		for _, comp := range v {
			if c, ok := comp.(string); ok {
				f.Components = append(f.Components, c)
			}
		}
	}
	if v, ok := props["rawErrorSignals"]; ok {
		f.RawErrorSignals = propsToFailureSignals(v)
	}
	if v, ok := props["rawAnomalySignals"]; ok {
		f.RawAnomalySignals = propsToFailureSignals(v)
	}
	if v, ok := props["detectionTimestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.DetectionTimestamp = t
		}
	}
	if v, ok := props["detectorVersion"].(string); ok {
		f.DetectorVersion = v
	}
	if v, ok := props["confidenceScore"].(float64); ok {
		f.ConfidenceScore = v
	}
	if v, ok := props["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.CreatedAt = t
		}
	}
	if v, ok := props["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.UpdatedAt = t
		}
	}

	return f, nil
}

// GetFailureByID retrieves a failure record by its human-readable FailureID (not UUID).
// This method scans all FailureRecord objects and matches by the failureId property.
//
//nolint:gocyclo // Property parsing from Weaviate requires many field mappings
func (s *WeaviateFailureStore) GetFailureByID(ctx context.Context, failureID string) (*FailureRecord, error) {
	if failureID == "" {
		return nil, nil
	}

	// Ensure the runtime Weaviate schema contains the FailureRecord class.
	s.schemaInit.Do(func() {
		s.schemaErr = s.ensureFailureRecordClass(ctx)
		if s.schemaErr != nil && s.logger != nil {
			s.logger.Sugar().Warnf("weavstore: failed ensuring FailureRecord class: %v", s.schemaErr)
		}
	})
	if s.schemaErr != nil {
		return nil, s.schemaErr
	}

	// Fetch all FailureRecord objects and search for matching failureId
	resp, err := s.client.Data().ObjectsGetter().WithClassName("FailureRecord").Do(ctx)
	if err != nil {
		s.logf("weavstore: GetFailureByID error fetching objects: %v", err)
		return nil, err
	}

	s.logf("weavstore: GetFailureByID searching for failureID=%s in %d objects", failureID, len(resp))

	for _, o := range resp {
		if o == nil || o.Properties == nil {
			continue
		}

		props, ok := o.Properties.(map[string]any)
		if !ok {
			continue
		}

		// Debug: log what we find
		if fid, ok := props["failureId"].(string); ok {
			s.logf("weavstore: GetFailureByID comparing: target=%s, found=%s, match=%v", failureID, fid, fid == failureID)
			if fid == failureID {
				// Found matching failure - extract all fields
				s.logf("weavstore: GetFailureByID matched failureID=%s", failureID)
				f := &FailureRecord{FailureID: failureID}

				if v, ok := props["failureUuid"].(string); ok {
					f.FailureUUID = v
				}
				if v, ok := props["startTime"].(string); ok {
					if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
						f.TimeRange.Start = t
					}
				}
				if v, ok := props["endTime"].(string); ok {
					if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
						f.TimeRange.End = t
					}
				}
				if v, ok := props["services"].([]any); ok {
					for _, svc := range v {
						if s, ok := svc.(string); ok {
							f.Services = append(f.Services, s)
						}
					}
				}
				if v, ok := props["components"].([]any); ok {
					for _, comp := range v {
						if c, ok := comp.(string); ok {
							f.Components = append(f.Components, c)
						}
					}
				}
				if v, ok := props["rawErrorSignals"]; ok {
					f.RawErrorSignals = propsToFailureSignals(v)
				}
				if v, ok := props["rawAnomalySignals"]; ok {
					f.RawAnomalySignals = propsToFailureSignals(v)
				}
				if v, ok := props["detectionTimestamp"].(string); ok {
					if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
						f.DetectionTimestamp = t
					}
				}
				if v, ok := props["detectorVersion"].(string); ok {
					f.DetectorVersion = v
				}
				if v, ok := props["confidenceScore"].(float64); ok {
					f.ConfidenceScore = v
				}
				if v, ok := props["createdAt"].(string); ok {
					if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
						f.CreatedAt = t
					}
				}
				if v, ok := props["updatedAt"].(string); ok {
					if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
						f.UpdatedAt = t
					}
				}

				return f, nil
			}
		}
	}

	s.logf("weavstore: GetFailureByID not found: failureID=%s", failureID)
	return nil, nil
}

// ensureFailureRecordClass checks whether the FailureRecord class exists in
// Weaviate and attempts to create it if missing. This function is safe to call
// repeatedly; callers should use sync.Once to avoid repeated attempts.
func (s *WeaviateFailureStore) ensureFailureRecordClass(ctx context.Context) error {
	if s.client == nil {
		return ErrWeaviateClientNil
	}

	// Try creating the class directly. If it already exists, treat as success.
	// This avoids relying on Getter API variations across client versions.
	// Build a minimal class definition matching runtime expectations.
	classDef := &wm.Class{
		Class:      "FailureRecord",
		Vectorizer: "none",
		Properties: []*wm.Property{
			{Name: "failureUuid", DataType: []string{"text"}},
			{Name: "failureId", DataType: []string{"text"}},
			{Name: "startTime", DataType: []string{"date"}},
			{Name: "endTime", DataType: []string{"date"}},
			{Name: "services", DataType: []string{"text[]"}},
			{Name: "components", DataType: []string{"text[]"}},
			{
				Name:     "rawErrorSignals",
				DataType: []string{"object[]"},
				NestedProperties: []*wm.NestedProperty{
					{Name: "signalType", DataType: []string{"text"}},
					{Name: "metricName", DataType: []string{"text"}},
					{Name: "service", DataType: []string{"text"}},
					{Name: "component", DataType: []string{"text"}},
					{Name: "data", DataType: []string{"text"}},
					{Name: "timestamp", DataType: []string{"date"}},
				},
			},
			{
				Name:     "rawAnomalySignals",
				DataType: []string{"object[]"},
				NestedProperties: []*wm.NestedProperty{
					{Name: "signalType", DataType: []string{"text"}},
					{Name: "metricName", DataType: []string{"text"}},
					{Name: "service", DataType: []string{"text"}},
					{Name: "component", DataType: []string{"text"}},
					{Name: "data", DataType: []string{"text"}},
					{Name: "timestamp", DataType: []string{"date"}},
				},
			},
			{Name: "detectionTimestamp", DataType: []string{"date"}},
			{Name: "detectorVersion", DataType: []string{"text"}},
			{Name: "confidenceScore", DataType: []string{"number"}},
			{Name: "createdAt", DataType: []string{"date"}},
			{Name: "updatedAt", DataType: []string{"date"}},
		},
	}

	// Attempt to create the class. Some client Do() implementations return only
	// an error; handle both cases conservatively.
	if err := s.client.Schema().ClassCreator().WithClass(classDef).Do(ctx); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "class already exists") {
			// Another process created it concurrently; treat as success.
			if s.logger != nil {
				s.logger.Sugar().Info("weavstore: FailureRecord class already exists in Weaviate")
			}
			return nil
		}
		if s.logger != nil {
			s.logger.Sugar().Errorf("weavstore: failed to create FailureRecord class: %v", err)
		}
		return fmt.Errorf("failed to create FailureRecord class in Weaviate: %w", err)
	}

	if s.logger != nil {
		s.logger.Sugar().Info("weavstore: successfully created FailureRecord class in Weaviate runtime schema")
	}
	return nil
}

// ListFailures returns failure records with pagination
func (s *WeaviateFailureStore) ListFailures(ctx context.Context, limit int, offset int) ([]*FailureRecord, int64, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// Ensure the runtime Weaviate schema contains the FailureRecord class.
	s.schemaInit.Do(func() {
		s.schemaErr = s.ensureFailureRecordClass(ctx)
		if s.schemaErr != nil && s.logger != nil {
			s.logger.Sugar().Warnf("weavstore: failed ensuring FailureRecord class: %v", s.schemaErr)
		}
	})
	if s.schemaErr != nil {
		return nil, 0, s.schemaErr
	}

	// Use ObjectsGetter to fetch class instances; apply limit/offset.
	resp, err := s.client.Data().ObjectsGetter().WithClassName("FailureRecord").WithLimit(limit).WithOffset(offset).Do(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*FailureRecord, 0, len(resp))
	for _, o := range resp {
		if o == nil {
			continue
		}
		f := &FailureRecord{}

		// Extract all properties from the Weaviate object
		var props map[string]any
		if o.Properties != nil {
			if m, ok := o.Properties.(map[string]any); ok {
				props = m
			}
		}

		if props != nil {
			// Map all fields (matching GetFailure pattern)
			if v, ok := props["failureUuid"].(string); ok {
				f.FailureUUID = v
			}
			if v, ok := props["failureId"].(string); ok {
				f.FailureID = v
			}
			if v, ok := props["startTime"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
					f.TimeRange.Start = t
				}
			}
			if v, ok := props["endTime"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
					f.TimeRange.End = t
				}
			}
			if v, ok := props["services"].([]any); ok {
				for _, svc := range v {
					if s, ok := svc.(string); ok {
						f.Services = append(f.Services, s)
					}
				}
			}
			if v, ok := props["components"].([]any); ok {
				for _, comp := range v {
					if c, ok := comp.(string); ok {
						f.Components = append(f.Components, c)
					}
				}
			}
			if v, ok := props["rawErrorSignals"]; ok {
				f.RawErrorSignals = propsToFailureSignals(v)
			}
			if v, ok := props["rawAnomalySignals"]; ok {
				f.RawAnomalySignals = propsToFailureSignals(v)
			}
			if v, ok := props["detectionTimestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
					f.DetectionTimestamp = t
				}
			}
			if v, ok := props["detectorVersion"].(string); ok {
				f.DetectorVersion = v
			}
			if v, ok := props["confidenceScore"].(float64); ok {
				f.ConfidenceScore = v
			}
			if v, ok := props["createdAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
					f.CreatedAt = t
				}
			}
			if v, ok := props["updatedAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
					f.UpdatedAt = t
				}
			}
		} else if add := o.ID.String(); add != "" {
			f.FailureUUID = add
		}

		out = append(out, f)
	}

	// By default, set total to the number of items returned in this page.
	total := int64(len(out))

	// Try to fetch the full count of FailureRecord objects from Weaviate.
	if all, terr := s.client.Data().ObjectsGetter().WithClassName("FailureRecord").WithLimit(10000).Do(ctx); terr == nil {
		total = int64(len(all))
	} else if s.logger != nil {
		s.logger.Warn("weaviate: failed to get total failure count; falling back to page size", zap.Error(terr))
	}

	return out, total, nil
}

// failureSignalsToMapArray converts FailureSignal slice into an array of map dictionaries
// suitable for Weaviate object type storage.
func failureSignalsToMapArray(signals []FailureSignal) []map[string]any {
	if len(signals) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(signals))
	for _, s := range signals {
		m := map[string]any{
			"signalType": s.SignalType,
			"metricName": s.MetricName,
			"service":    s.Service,
			"component":  s.Component,
			"data":       s.Data,
			"timestamp":  s.Timestamp.Format(time.RFC3339Nano),
		}
		out = append(out, m)
	}
	return out
}

// propsToFailureSignals converts a raw signals property (from Weaviate) into
// []FailureSignal. The raw value can be []map[string]any, []interface{}, or
// other types depending on the SDK decoding.
func propsToFailureSignals(raw any) []FailureSignal {
	if raw == nil {
		return nil
	}
	// Prefer []map[string]any if present
	if arr, ok := raw.([]map[string]any); ok {
		return convertFromSignalMapArray(arr)
	}
	// Handle []any / []interface{} where each element is a map
	if arr, ok := raw.([]any); ok {
		out := make([]FailureSignal, 0, len(arr))
		for _, it := range arr {
			if m, ok := it.(map[string]any); ok {
				out = append(out, mapToFailureSignal(m))
				continue
			}
			if m2, ok := it.(map[string]interface{}); ok {
				mam := make(map[string]any, len(m2))
				for kk, vv := range m2 {
					mam[kk] = vv
				}
				out = append(out, mapToFailureSignal(mam))
				continue
			}
		}
		return out
	}
	return nil
}

func convertFromSignalMapArray(arr []map[string]any) []FailureSignal {
	out := make([]FailureSignal, 0, len(arr))
	for _, m := range arr {
		out = append(out, mapToFailureSignal(m))
	}
	return out
}

func mapToFailureSignal(m map[string]any) FailureSignal {
	var fs FailureSignal
	if st, ok := m["signalType"].(string); ok {
		fs.SignalType = st
	}
	if mn, ok := m["metricName"].(string); ok {
		fs.MetricName = mn
	}
	if svc, ok := m["service"].(string); ok {
		fs.Service = svc
	}
	if comp, ok := m["component"].(string); ok {
		fs.Component = comp
	}
	if data, ok := m["data"].(map[string]any); ok {
		fs.Data = data
	}
	if ts, ok := m["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			fs.Timestamp = t
		}
	}
	return fs
}
