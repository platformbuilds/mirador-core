package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/weavstore"
)

func TestConvertToWeaviateTaskIncludesName(t *testing.T) {
	handler := &MIRARCAAsyncHandler{}

	submitted := time.Now().UTC()
	task := &MIRARCATaskStatus{
		TaskID:      "task-1",
		Name:        "user-friendly-name",
		Status:      "completed",
		SubmittedAt: submitted,
		CompletedAt: &submitted,
		Result:      map[string]interface{}{"explanation": "ok"},
	}

	weav := handler.convertToWeaviateTask(task)
	if weav == nil {
		t.Fatalf("expected non-nil weaviate task")
	}
	if weav.Name != task.Name {
		t.Fatalf("expected name to be preserved; got %q want %q", weav.Name, task.Name)
	}
	if weav.TaskID != task.TaskID {
		t.Fatalf("task id mismatch")
	}
	// verify submittedAt copied
	if !weav.SubmittedAt.Equal(task.SubmittedAt) {
		t.Fatalf("submittedAt mismatch")
	}
}

type fakeWeavStore struct {
	list   func(ctx context.Context, limit, offset int) ([]*weavstore.MIRARCATask, int64, error)
	search func(ctx context.Context, q, mode string, limit, offset int) ([]*weavstore.MIRARCATask, int64, error)
}

func (f *fakeWeavStore) ListMIRARCATasks(ctx context.Context, limit, offset int) ([]*weavstore.MIRARCATask, int64, error) {
	return f.list(ctx, limit, offset)
}

func (f *fakeWeavStore) SearchMIRARCATasks(ctx context.Context, q, mode string, limit, offset int) ([]*weavstore.MIRARCATask, int64, error) {
	return f.search(ctx, q, mode, limit, offset)
}

// stubs to satisfy the interface for tests
func (f *fakeWeavStore) CreateOrUpdateMIRARCATask(ctx context.Context, task *weavstore.MIRARCATask) (*weavstore.MIRARCATask, string, error) {
	return task, "created", nil
}

func (f *fakeWeavStore) GetMIRARCATask(ctx context.Context, taskID string) (*weavstore.MIRARCATask, error) {
	return nil, weavstore.ErrMIRARCATaskNotFoundWithID
}

func (f *fakeWeavStore) DeleteMIRARCATask(ctx context.Context, taskID string) error { return nil }

func TestHandleListTasksHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	// create fake store
	fake := &fakeWeavStore{
		list: func(ctx context.Context, limit, offset int) ([]*weavstore.MIRARCATask, int64, error) {
			return []*weavstore.MIRARCATask{{TaskID: "t1", Name: "A"}, {TaskID: "t2", Name: "B"}}, 2, nil
		},
	}

	handler := &MIRARCAAsyncHandler{weaviateStore: fake, logger: logging.New("error")}
	req := httptest.NewRequest("GET", "/rca_analyze/list?limit=2&offset=0", nil)
	ctx.Request = req

	handler.HandleListTasks(ctx)

	assert.Equal(t, 200, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, float64(2), body["total"].(float64))
}

func TestHandleSearchTasksHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	fake := &fakeWeavStore{
		search: func(ctx context.Context, q, mode string, limit, offset int) ([]*weavstore.MIRARCATask, int64, error) {
			return []*weavstore.MIRARCATask{{TaskID: "t1", Name: "Match"}}, 1, nil
		},
	}

	handler := &MIRARCAAsyncHandler{weaviateStore: fake, logger: logging.New("error")}
	req := httptest.NewRequest("GET", "/rca_analyze/search?q=match&limit=10", nil)
	ctx.Request = req

	handler.HandleSearchTasks(ctx)

	assert.Equal(t, 200, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, float64(1), body["total"].(float64))
}
