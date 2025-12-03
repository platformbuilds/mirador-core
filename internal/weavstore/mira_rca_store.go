package weavstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
	wv "github.com/weaviate/weaviate-go-client/v5/weaviate"
	wm "github.com/weaviate/weaviate/entities/models"
	"go.uber.org/zap"
)

// WeaviateMIRARCAStore is a wrapper around the official weaviate v5 client
// for MIRA RCA task-specific operations. It centralizes all MIRA RCA Weaviate access via the
// SDK (no raw HTTP/GraphQL strings).
type WeaviateMIRARCAStore struct {
	client *wv.Client
	logger *zap.Logger
	// schemaInit ensures we attempt to create the required class only once
	schemaInit sync.Once
	schemaErr  error
}

// NewWeaviateMIRARCAStore constructs a new MIRA RCA store.
func NewWeaviateMIRARCAStore(client *wv.Client, logger *zap.Logger) *WeaviateMIRARCAStore {
	return &WeaviateMIRARCAStore{client: client, logger: logger}
}

// Static errors for err113 compliance
var (
	ErrMIRARCATaskIsNil            = errors.New("mira rca task is nil")
	ErrMIRARCATaskIDEmpty          = errors.New("mira rca task id is empty")
	ErrWeaviateMIRARCADeleteFailed = errors.New("weaviate delete attempts failed for task_id")
	ErrMIRARCATaskNotFoundWithID   = errors.New("mira rca task not found with id")
	maxMIRARCAObjectsLimit         = 10000
)

func makeMIRARCAObjectID(taskID string) string {
	// Use deterministic UUID v5 based on task ID
	return uuid.NewV5(nsMirador, fmt.Sprintf("MIRARCATask|%s", taskID)).String()
}

// MIRARCATask represents a MIRA RCA analysis task stored in Weaviate.
type MIRARCATask struct {
	TaskID      string                 `json:"taskId"`
	Status      string                 `json:"status"` // pending, processing, completed, failed
	RCAData     map[string]interface{} `json:"rcaData"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CallbackURL string                 `json:"callbackUrl,omitempty"`
	SubmittedAt time.Time              `json:"submittedAt"`
	CompletedAt time.Time              `json:"completedAt,omitempty"`
	Progress    map[string]interface{} `json:"progress,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// CreateOrUpdateMIRARCATask creates a MIRA RCA task if missing, updates if present. It returns
// the task model, a status string ("created","updated","no-change"), and an error.
//
//nolint:gocyclo // Property mapping inherently complex, matches KPI/Failure store patterns
func (s *WeaviateMIRARCAStore) CreateOrUpdateMIRARCATask(ctx context.Context, task *MIRARCATask) (*MIRARCATask, string, error) {
	if task == nil {
		return nil, "", ErrMIRARCATaskIsNil
	}
	if task.TaskID == "" {
		return nil, "", ErrMIRARCATaskIDEmpty
	}

	// Ensure the runtime Weaviate schema contains the MIRARCATask class.
	s.schemaInit.Do(func() {
		s.schemaErr = s.ensureMIRARCATaskClass(ctx)
		if s.schemaErr != nil && s.logger != nil {
			s.logger.Sugar().Warnf("weavstore: failed ensuring MIRARCATask class: %v", s.schemaErr)
		}
	})
	if s.schemaErr != nil {
		// Return schema initialization error to avoid ambiguous 404/422 from Weaviate later.
		return nil, "", s.schemaErr
	}

	objID := makeMIRARCAObjectID(task.TaskID)

	// Serialize complex fields to JSON strings for Weaviate text storage
	rcaDataJSON, _ := json.Marshal(task.RCAData)
	resultJSON, _ := json.Marshal(task.Result)
	progressJSON, _ := json.Marshal(task.Progress)

	props := map[string]any{
		"taskId":      task.TaskID,
		"status":      task.Status,
		"rcaData":     string(rcaDataJSON),
		"result":      string(resultJSON),
		"error":       task.Error,
		"callbackUrl": task.CallbackURL,
		"submittedAt": task.SubmittedAt.Format(time.RFC3339Nano),
		"progress":    string(progressJSON),
		"createdAt":   task.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":   task.UpdatedAt.Format(time.RFC3339Nano),
	}

	// Only set completedAt if it's not zero
	if !task.CompletedAt.IsZero() {
		props["completedAt"] = task.CompletedAt.Format(time.RFC3339Nano)
	}

	// Check if object already exists in Weaviate
	existing, err := s.GetMIRARCATask(ctx, task.TaskID)
	if err != nil && !errors.Is(err, ErrMIRARCATaskNotFoundWithID) {
		return nil, "", err
	}

	// If object exists and is identical field-by-field, treat as success (no-op)
	if existing != nil {
		if mirarcaTaskEqual(existing, task) {
			// No change detected; return existing as success
			return existing, statusNoChange, nil
		}
		// There is a modification: perform update
		if err := s.client.Data().Updater().WithClassName("MIRARCATask").WithID(objID).WithProperties(props).Do(ctx); err != nil {
			return nil, "", fmt.Errorf("failed to update MIRA RCA task: %w", err)
		}
		return task, statusUpdated, nil
	}

	// Not found -> create. If create fails because the object already exists,
	// fall back to updating the existing object.
	if _, err := s.client.Data().Creator().WithClassName("MIRARCATask").WithID(objID).WithProperties(props).Do(ctx); err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "id already exists") {
			if err2 := s.client.Data().Updater().WithClassName("MIRARCATask").WithID(objID).WithProperties(props).Do(ctx); err2 != nil {
				return nil, "", fmt.Errorf("create conflict: update also failed: %w (create err: %v)", err2, err)
			}
			return task, statusUpdated, nil
		}
		return nil, "", fmt.Errorf("failed to create MIRA RCA task: %w", err)
	}
	return task, statusCreated, nil
}

// mirarcaTaskEqual compares two MIRARCATask objects field-by-field.
func mirarcaTaskEqual(a, b *MIRARCATask) bool {
	if a == nil || b == nil {
		return false
	}
	if a.TaskID != b.TaskID || a.Status != b.Status || a.Error != b.Error || a.CallbackURL != b.CallbackURL {
		return false
	}
	if !a.SubmittedAt.Equal(b.SubmittedAt) || !a.CompletedAt.Equal(b.CompletedAt) {
		return false
	}
	if !a.CreatedAt.Equal(b.CreatedAt) || !a.UpdatedAt.Equal(b.UpdatedAt) {
		return false
	}
	// Compare complex fields via JSON serialization
	aRCAData, _ := json.Marshal(a.RCAData)
	bRCAData, _ := json.Marshal(b.RCAData)
	if !bytes.Equal(aRCAData, bRCAData) {
		return false
	}
	aResult, _ := json.Marshal(a.Result)
	bResult, _ := json.Marshal(b.Result)
	if !bytes.Equal(aResult, bResult) {
		return false
	}
	aProgress, _ := json.Marshal(a.Progress)
	bProgress, _ := json.Marshal(b.Progress)
	return bytes.Equal(aProgress, bProgress)
}

// GetMIRARCATask retrieves a MIRA RCA task from Weaviate by task ID.
//
//nolint:gocyclo // Property parsing from Weaviate requires many field mappings
func (s *WeaviateMIRARCAStore) GetMIRARCATask(ctx context.Context, taskID string) (*MIRARCATask, error) {
	if taskID == "" {
		return nil, ErrMIRARCATaskIDEmpty
	}
	objID := makeMIRARCAObjectID(taskID)

	// Fetch objects of the class and search for matching object id
	resp, err := s.client.Data().ObjectsGetter().WithClassName("MIRARCATask").Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MIRA RCA tasks: %w", err)
	}

	var props map[string]any
	var found bool
	for _, o := range resp {
		if o == nil {
			continue
		}
		// Support both the deterministic v5 object id and direct task id
		if o.ID.String() == objID || o.ID.String() == taskID {
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
		return nil, ErrMIRARCATaskNotFoundWithID
	}

	task := &MIRARCATask{TaskID: taskID}

	// Parse string fields
	if v, ok := props["taskId"].(string); ok {
		task.TaskID = v
	}
	if v, ok := props["status"].(string); ok {
		task.Status = v
	}
	if v, ok := props["error"].(string); ok {
		task.Error = v
	}
	if v, ok := props["callbackUrl"].(string); ok {
		task.CallbackURL = v
	}

	// Parse JSON fields
	if v, ok := props["rcaData"].(string); ok && v != "" {
		var rcaData map[string]interface{}
		if err := json.Unmarshal([]byte(v), &rcaData); err == nil {
			task.RCAData = rcaData
		}
	}
	if v, ok := props["result"].(string); ok && v != "" {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(v), &result); err == nil {
			task.Result = result
		}
	}
	if v, ok := props["progress"].(string); ok && v != "" {
		var progress map[string]interface{}
		if err := json.Unmarshal([]byte(v), &progress); err == nil {
			task.Progress = progress
		}
	}

	// Parse timestamps
	if v, ok := props["submittedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			task.SubmittedAt = t
		}
	}
	if v, ok := props["completedAt"].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			task.CompletedAt = t
		}
	}
	if v, ok := props["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			task.CreatedAt = t
		}
	}
	if v, ok := props["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			task.UpdatedAt = t
		}
	}

	return task, nil
}

// DeleteMIRARCATask deletes a MIRA RCA task from Weaviate by task ID.
func (s *WeaviateMIRARCAStore) DeleteMIRARCATask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return ErrMIRARCATaskIDEmpty
	}
	objID := makeMIRARCAObjectID(taskID)

	// Try delete by deterministic v5 object id first
	if s.tryDeleteMIRARCAByObjectID(ctx, taskID, objID) {
		return nil
	}

	// Next try delete using the raw id as object id (legacy objects)
	if s.tryDeleteMIRARCAByRawID(ctx, taskID) {
		return nil
	}

	// As a last resort, scan MIRARCATask objects
	if s.tryDeleteMIRARCAByScan(ctx, taskID, objID) {
		return nil
	}

	return fmt.Errorf("%w: taskId=%s (tried objID=%s and raw id)", ErrWeaviateMIRARCADeleteFailed, taskID, objID)
}

// tryDeleteMIRARCAByObjectID attempts to delete using the deterministic v5 object ID
func (s *WeaviateMIRARCAStore) tryDeleteMIRARCAByObjectID(ctx context.Context, taskID, objID string) bool {
	s.logf("weavstore: attempting delete for MIRA RCA task taskId=%s (objID=%s)", taskID, objID)

	if err := s.client.Data().Deleter().WithClassName("MIRARCATask").WithID(objID).Do(ctx); err == nil {
		s.logf("weavstore: deleted MIRA RCA task by v5 objID=%s", objID)
		if s.verifyMIRARCADeletion(ctx, taskID, objID) {
			return true
		}
	} else {
		s.logf("weavstore: delete MIRA RCA task by v5 objID failed: %v", err)
	}
	return false
}

// tryDeleteMIRARCAByRawID attempts to delete using the raw ID (legacy objects)
func (s *WeaviateMIRARCAStore) tryDeleteMIRARCAByRawID(ctx context.Context, taskID string) bool {
	if err := s.client.Data().Deleter().WithClassName("MIRARCATask").WithID(taskID).Do(ctx); err == nil {
		s.logf("weavstore: deleted MIRA RCA task by raw id=%s", taskID)
		if s.verifyMIRARCADeletion(ctx, taskID, taskID) {
			return true
		}
	} else {
		s.logf("weavstore: delete MIRA RCA task by raw id failed: %v", err)
	}
	return false
}

// tryDeleteMIRARCAByScan scans for matching objects and deletes them
func (s *WeaviateMIRARCAStore) tryDeleteMIRARCAByScan(ctx context.Context, taskID, objID string) bool {
	objs, gerr := s.client.Data().ObjectsGetter().WithClassName("MIRARCATask").Do(ctx)
	if gerr != nil {
		s.logf("weavstore: MIRA RCA task object scan failed: %v", gerr)
		return false
	}

	for _, o := range objs {
		if o == nil {
			continue
		}
		oid := o.ID.String()

		// Try match by object ID equality
		if oid == objID || oid == taskID {
			if s.tryDeleteMIRARCAAndVerify(ctx, taskID, oid) {
				return true
			}
		}

		// Try match by properties
		if s.tryDeleteMIRARCAByProperties(ctx, taskID, o) {
			return true
		}
	}
	return false
}

// tryDeleteMIRARCAAndVerify attempts a delete and verifies it succeeded
func (s *WeaviateMIRARCAStore) tryDeleteMIRARCAAndVerify(ctx context.Context, taskID, oid string) bool {
	if derr := s.client.Data().Deleter().WithClassName("MIRARCATask").WithID(oid).Do(ctx); derr == nil {
		s.logf("weavstore: deleted MIRA RCA task by scanning object id=%s", oid)
		if s.verifyMIRARCADeletion(ctx, taskID, oid) {
			return true
		}
	} else {
		s.logf("weavstore: scan-delete attempt for MIRA RCA task oid=%s failed: %v", oid, derr)
	}
	return false
}

// tryDeleteMIRARCAByProperties attempts to match and delete by properties
func (s *WeaviateMIRARCAStore) tryDeleteMIRARCAByProperties(ctx context.Context, taskID string, o *wm.Object) bool {
	if o.Properties == nil {
		return false
	}

	props, ok := o.Properties.(map[string]any)
	if !ok {
		return false
	}

	vtaskID, ok := props["taskId"].(string)
	if !ok || vtaskID != taskID {
		return false
	}

	oid := o.ID.String()
	if derr := s.client.Data().Deleter().WithClassName("MIRARCATask").WithID(oid).Do(ctx); derr == nil {
		s.logf("weavstore: deleted MIRA RCA task by scanning props match taskId=%s -> oid=%s", taskID, oid)
		if s.verifyMIRARCADeletion(ctx, taskID, oid) {
			return true
		}
	} else {
		s.logf("weaviate: scan-delete MIRA RCA task by props for oid=%s failed: %v", oid, derr)
	}
	return false
}

// verifyMIRARCADeletion checks if an object was successfully deleted
func (s *WeaviateMIRARCAStore) verifyMIRARCADeletion(ctx context.Context, taskID, oid string) bool {
	if remaining, verifyErr := s.GetMIRARCATask(ctx, taskID); verifyErr == nil && remaining != nil {
		s.logf("weavstore: WARNING - MIRA RCA task still exists after delete attempt by oid=%s", oid)
		return false
	} else if errors.Is(verifyErr, ErrMIRARCATaskNotFoundWithID) {
		s.logf("weavstore: verified MIRA RCA task deletion by oid=%s - object no longer found", oid)
		return true
	}
	return false
}

// ListMIRARCATasks returns MIRA RCA tasks with pagination support.
//
//nolint:gocyclo // Property parsing from Weaviate requires many field mappings
func (s *WeaviateMIRARCAStore) ListMIRARCATasks(ctx context.Context, limit, offset int) ([]*MIRARCATask, int64, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// Use ObjectsGetter to fetch class instances; apply limit/offset.
	resp, err := s.client.Data().ObjectsGetter().WithClassName("MIRARCATask").WithLimit(limit).WithOffset(offset).Do(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list MIRA RCA tasks: %w", err)
	}

	out := make([]*MIRARCATask, 0, len(resp))
	for _, o := range resp {
		if o == nil {
			continue
		}

		var props map[string]any
		if o.Properties != nil {
			if m, ok := o.Properties.(map[string]any); ok {
				props = m
			}
		}

		if props == nil {
			continue
		}

		task := &MIRARCATask{}

		// Parse all fields
		if v, ok := props["taskId"].(string); ok {
			task.TaskID = v
		}
		if v, ok := props["status"].(string); ok {
			task.Status = v
		}
		if v, ok := props["error"].(string); ok {
			task.Error = v
		}
		if v, ok := props["callbackUrl"].(string); ok {
			task.CallbackURL = v
		}
		if v, ok := props["rcaData"].(string); ok && v != "" {
			var rcaData map[string]interface{}
			if err := json.Unmarshal([]byte(v), &rcaData); err == nil {
				task.RCAData = rcaData
			}
		}
		if v, ok := props["result"].(string); ok && v != "" {
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(v), &result); err == nil {
				task.Result = result
			}
		}
		if v, ok := props["progress"].(string); ok && v != "" {
			var progress map[string]interface{}
			if err := json.Unmarshal([]byte(v), &progress); err == nil {
				task.Progress = progress
			}
		}
		if v, ok := props["submittedAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				task.SubmittedAt = t
			}
		}
		if v, ok := props["completedAt"].(string); ok && v != "" {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				task.CompletedAt = t
			}
		}
		if v, ok := props["createdAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				task.CreatedAt = t
			}
		}
		if v, ok := props["updatedAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				task.UpdatedAt = t
			}
		}

		out = append(out, task)
	}

	// Try to fetch the full count of MIRARCATask objects from Weaviate
	total := int64(len(out))
	if all, terr := s.client.Data().ObjectsGetter().WithClassName("MIRARCATask").WithLimit(maxMIRARCAObjectsLimit).Do(ctx); terr == nil {
		total = int64(len(all))
	} else {
		s.logger.Warn("weaviate: failed to get total MIRA RCA task count; falling back to page size", zap.Error(terr))
	}

	return out, total, nil
}

// ensureMIRARCATaskClass checks whether the MIRARCATask class exists in
// Weaviate and attempts to create it if missing.
func (s *WeaviateMIRARCAStore) ensureMIRARCATaskClass(ctx context.Context) error {
	if s.client == nil {
		return ErrWeaviateClientNil
	}

	// Build a minimal class definition matching runtime expectations
	classDef := &wm.Class{
		Class:      "MIRARCATask",
		Vectorizer: "none",
		Properties: []*wm.Property{
			{Name: "taskId", DataType: []string{"string"}},
			{Name: "status", DataType: []string{"string"}},
			{Name: "rcaData", DataType: []string{"text"}}, // JSON stored as text
			{Name: "result", DataType: []string{"text"}},  // JSON stored as text
			{Name: "error", DataType: []string{"text"}},
			{Name: "callbackUrl", DataType: []string{"string"}},
			{Name: "submittedAt", DataType: []string{"date"}},
			{Name: "completedAt", DataType: []string{"date"}},
			{Name: "progress", DataType: []string{"text"}}, // JSON stored as text
			{Name: "createdAt", DataType: []string{"date"}},
			{Name: "updatedAt", DataType: []string{"date"}},
		},
	}

	// Attempt to create the class
	if err := s.client.Schema().ClassCreator().WithClass(classDef).Do(ctx); err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "class already exists") {
			// Another process created it concurrently; treat as success
			return nil
		}
		return fmt.Errorf("failed to create MIRARCATask class in Weaviate: %w", err)
	}

	if s.logger != nil {
		s.logger.Sugar().Info("weavstore: created MIRARCATask class in Weaviate runtime schema")
	}
	return nil
}

// logf is a helper to log via zap if available
func (s *WeaviateMIRARCAStore) logf(format string, args ...interface{}) {
	if s.logger != nil {
		s.logger.Sugar().Infof(format, args...)
		return
	}
	fmt.Printf(format+"\n", args...)
}
