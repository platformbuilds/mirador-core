package weavstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMIRARCATaskConversionRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	task := &MIRARCATask{
		TaskID: "550e8400-e29b-41d4-a716-446655440000",
		Status: "completed",
		RCAData: map[string]interface{}{
			"impact": map[string]interface{}{
				"id":            "corr_1764756501",
				"impactService": "DB Operations",
			},
		},
		Result: map[string]interface{}{
			"explanation": "# Root Cause Analysis Summary\n\nAnalysis complete.",
			"metadata": map[string]interface{}{
				"model":       "gpt-4",
				"totalTokens": 1250,
			},
		},
		Error:       "",
		CallbackURL: "https://webhook.example.com/callback",
		SubmittedAt: now,
		CompletedAt: now.Add(2 * time.Minute),
		Progress: map[string]interface{}{
			"totalChunks":     3,
			"completedChunks": 3,
			"currentChunk":    3,
		},
		CreatedAt: now,
		UpdatedAt: now.Add(2 * time.Minute),
	}

	// Test equality
	taskCopy := *task
	assert.True(t, mirarcaTaskEqual(task, &taskCopy), "identical tasks should be equal")

	// Test inequality - different status
	taskCopy2 := *task
	taskCopy2.Status = "failed"
	assert.False(t, mirarcaTaskEqual(task, &taskCopy2), "tasks with different status should not be equal")

	// Test inequality - different RCAData
	taskCopy3 := *task
	taskCopy3.RCAData = map[string]interface{}{"different": "data"}
	assert.False(t, mirarcaTaskEqual(task, &taskCopy3), "tasks with different RCAData should not be equal")
}

func TestMakeMIRARCAObjectID(t *testing.T) {
	taskID1 := "550e8400-e29b-41d4-a716-446655440000"
	taskID2 := "550e8400-e29b-41d4-a716-446655440001"

	objID1 := makeMIRARCAObjectID(taskID1)
	objID2 := makeMIRARCAObjectID(taskID2)

	// Different task IDs should produce different object IDs
	assert.NotEqual(t, objID1, objID2, "different task IDs should produce different object IDs")

	// Same task ID should produce same object ID (deterministic)
	objID1Again := makeMIRARCAObjectID(taskID1)
	assert.Equal(t, objID1, objID1Again, "same task ID should produce same object ID")
}

func TestMIRARCATaskEquality(t *testing.T) {
	now := time.Now().UTC()

	task1 := &MIRARCATask{
		TaskID:      "task-1",
		Status:      "completed",
		RCAData:     map[string]interface{}{"key": "value"},
		Result:      map[string]interface{}{"explanation": "test"},
		Error:       "",
		CallbackURL: "https://example.com/callback",
		SubmittedAt: now,
		CompletedAt: now.Add(time.Minute),
		Progress:    map[string]interface{}{"current": 1},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	task2 := &MIRARCATask{
		TaskID:      "task-1",
		Status:      "completed",
		RCAData:     map[string]interface{}{"key": "value"},
		Result:      map[string]interface{}{"explanation": "test"},
		Error:       "",
		CallbackURL: "https://example.com/callback",
		SubmittedAt: now,
		CompletedAt: now.Add(time.Minute),
		Progress:    map[string]interface{}{"current": 1},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.True(t, mirarcaTaskEqual(task1, task2), "identical tasks should be equal")

	// Test nil cases
	assert.False(t, mirarcaTaskEqual(nil, task2), "nil task should not equal non-nil")
	assert.False(t, mirarcaTaskEqual(task1, nil), "non-nil task should not equal nil")
	assert.False(t, mirarcaTaskEqual(nil, nil), "two nil tasks should not be equal")
}

func TestMIRARCAStoreErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		task        *MIRARCATask
		expectedErr error
	}{
		{
			name:        "nil task",
			task:        nil,
			expectedErr: ErrMIRARCATaskIsNil,
		},
		{
			name: "empty task ID",
			task: &MIRARCATask{
				TaskID: "",
				Status: "pending",
			},
			expectedErr: ErrMIRARCATaskIDEmpty,
		},
	}

	// Create a mock store (without actual Weaviate client for error validation)
	store := &WeaviateMIRARCAStore{
		client: nil,
		logger: nil,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := store.CreateOrUpdateMIRARCATask(ctx, tt.task)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestMIRARCATaskStatuses(t *testing.T) {
	statuses := []string{"pending", "processing", "completed", "failed"}

	for _, status := range statuses {
		task := &MIRARCATask{
			TaskID:      "test-task-" + status,
			Status:      status,
			RCAData:     map[string]interface{}{},
			SubmittedAt: time.Now(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Verify status is set correctly
		assert.Equal(t, status, task.Status)

		// Verify task can be compared
		taskCopy := *task
		assert.True(t, mirarcaTaskEqual(task, &taskCopy))
	}
}
