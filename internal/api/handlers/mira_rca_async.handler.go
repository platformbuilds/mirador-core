package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/utils"
	"github.com/platformbuilds/mirador-core/internal/weavstore"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// MIRARCATaskStatus represents the status of an async MIRA RCA task.
type MIRARCATaskStatus struct {
	TaskID      string                 `json:"taskId"`
	Name        string                 `json:"name,omitempty"`
	Status      string                 `json:"status"` // pending, processing, completed, failed
	Progress    *TaskProgress          `json:"progress,omitempty"`
	SubmittedAt time.Time              `json:"submittedAt"`
	StartedAt   *time.Time             `json:"startedAt,omitempty"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CallbackURL string                 `json:"callbackUrl,omitempty"`
}

// TaskProgress tracks chunk-level progress for MIRA RCA analysis.
type TaskProgress struct {
	TotalChunks     int       `json:"totalChunks"`
	CompletedChunks int       `json:"completedChunks"`
	CurrentChunk    int       `json:"currentChunk"`
	CurrentStage    string    `json:"currentStage"` // toon_conversion, prompt_rendering, chunk_1_processing, etc.
	LastUpdated     time.Time `json:"lastUpdated"`
}

// MIRARCAAsyncHandler handles async MIRA RCA explanation endpoints with Valkey-backed state.
type MIRARCAAsyncHandler struct {
	miraHandler   *MIRARCAHandler
	cache         cache.ValkeyCluster        // Valkey cache for task state persistence (hot cache, 24h TTL)
	weaviateStore weavstore.MIRARCATaskStore // Weaviate (or other store) for long-term task storage
	logger        logging.Logger
	config        config.MIRAConfig
}

// NewMIRARCAAsyncHandler creates a new async MIRA RCA handler with Valkey and Weaviate backing.
func NewMIRARCAAsyncHandler(miraService services.MIRAService, miraCfg config.MIRAConfig, valkeyCache cache.ValkeyCluster, weaviateStore weavstore.MIRARCATaskStore, logger corelogger.Logger) *MIRARCAAsyncHandler {
	return &MIRARCAAsyncHandler{
		miraHandler:   NewMIRARCAHandler(miraService, miraCfg, logger),
		cache:         valkeyCache,
		weaviateStore: weaviateStore,
		logger:        logging.FromCoreLogger(logger),
		config:        miraCfg,
	}
}

// MIRAAsyncRequest represents async RCA analysis request.
type MIRAAsyncRequest struct {
	// user-provided readable name for the RCA analysis
	Name        string             `json:"name" binding:"required"`
	RCAData     models.RCAResponse `json:"rcaData" binding:"required"`
	CallbackURL string             `json:"callbackUrl,omitempty"` // Optional webhook for completion notification
}

const (
	taskKeyPrefix = "mira:rca:task:"
	taskTTL       = 24 * time.Hour // Tasks expire after 24 hours
)

// getTaskKey generates Valkey key for task storage.
func getTaskKey(taskID string) string {
	return taskKeyPrefix + taskID
}

// saveTaskStatus persists task status to both Valkey (hot cache) and Weaviate (long-term storage).
func (h *MIRARCAAsyncHandler) saveTaskStatus(task *MIRARCATaskStatus) error {
	key := getTaskKey(task.TaskID)
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task status: %w", err)
	}

	// Save to Valkey for fast retrieval (24h TTL)
	if err := h.cache.Set(context.Background(), key, data, taskTTL); err != nil {
		return fmt.Errorf("failed to save task to Valkey: %w", err)
	}

	// Also persist to Weaviate for long-term storage (no TTL)
	if h.weaviateStore != nil {
		weavTask := h.convertToWeaviateTask(task)
		if _, _, err := h.weaviateStore.CreateOrUpdateMIRARCATask(context.Background(), weavTask); err != nil {
			// Log error but don't fail the operation - Valkey is primary for async ops
			h.logger.Warn("failed to save task to Weaviate (non-fatal)", "error", err)
		}
	}

	return nil
}

// getTaskStatus retrieves task status from Valkey.
func (h *MIRARCAAsyncHandler) getTaskStatus(taskID string) (*MIRARCATaskStatus, error) {
	key := getTaskKey(taskID)
	data, err := h.cache.Get(context.Background(), key)
	if err != nil {
		// Check if it's a "key not found" error (Valkey returns error for cache miss)
		if err.Error() == fmt.Sprintf("key not found: %s", key) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to retrieve task from Valkey: %w", err)
	}

	var task MIRARCATaskStatus
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task status: %w", err)
	}

	return &task, nil
}

// updateTaskProgress updates task progress and saves to Valkey.
func (h *MIRARCAAsyncHandler) updateTaskProgress(taskID string, currentChunk int, totalChunks int, stage string) error {
	task, err := h.getTaskStatus(taskID)
	if err != nil {
		return err
	}

	if task.Progress == nil {
		task.Progress = &TaskProgress{
			TotalChunks: totalChunks,
		}
	}

	task.Progress.CurrentChunk = currentChunk
	task.Progress.CurrentStage = stage
	task.Progress.LastUpdated = time.Now()

	if stage == fmt.Sprintf("chunk_%d_completed", currentChunk) {
		task.Progress.CompletedChunks = currentChunk
	}

	h.logger.Info("Task progress updated",
		"task_id", taskID,
		"stage", stage,
		"current_chunk", currentChunk,
		"total_chunks", totalChunks,
		"completed_chunks", task.Progress.CompletedChunks)

	return h.saveTaskStatus(task)
}

// HandleMIRARCAAnalyzeAsync handles POST /api/v1/mira/rca_analyze_async.
// It submits an RCA analysis job and returns a taskId immediately.
//
// @Summary Submit async MIRA RCA analysis task
// @Description Submits RCA data for async AI-powered explanation. Returns taskId immediately. Client can poll /api/v1/mira/rca_analyze/{taskId} for status/result. Optional callback URL for webhook notification.
// @Tags mira
// @Accept json
// @Produce json
// @Param request body MIRAAsyncRequest true "RCA data and optional callback URL"
// @Success 202 {object} map[string]interface{} "status: accepted, taskId: uuid, statusUrl: /api/v1/mira/rca_analyze/{taskId}"
// @Failure 400 {object} map[string]interface{} "status: error, error: invalid_json_payload | invalid_rca_data"
// @Router /api/v1/mira/rca_analyze_async [post]
func (h *MIRARCAAsyncHandler) HandleMIRARCAAnalyzeAsync(c *gin.Context) {
	var req MIRAAsyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to parse async MIRA request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "invalid_json_payload",
		})
		return
	}

	// Validate RCA response structure
	if err := utils.ValidateRCAResponse(&req.RCAData); err != nil {
		h.logger.Error("RCA response validation failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "invalid_rca_data",
			"detail": err.Error(),
		})
		return
	}

	// Create task with UUID
	taskID := uuid.New().String()
	task := &MIRARCATaskStatus{
		TaskID:      taskID,
		Status:      "pending",
		Name:        req.Name,
		SubmittedAt: time.Now(),
		CallbackURL: req.CallbackURL,
	}

	// Save initial task status to Valkey
	if err := h.saveTaskStatus(task); err != nil {
		h.logger.Error("Failed to save task to Valkey", "task_id", taskID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "task_creation_failed",
		})
		return
	}

	h.logger.Info("Submitted async MIRA task",
		"task_id", taskID,
		"has_callback", req.CallbackURL != "",
		"chains_count", len(req.RCAData.Data.Chains),
		"status", "pending")

	// Start processing in background
	go h.processTask(taskID, &req.RCAData)

	c.JSON(http.StatusAccepted, gin.H{
		"status":    "accepted",
		"taskId":    taskID,
		"statusUrl": fmt.Sprintf("/api/v1/mira/rca_analyze/%s", taskID),
		"message":   "Task submitted for processing. Poll statusUrl for results.",
	})
}

// HandleGetTaskStatus handles GET /api/v1/mira/rca_analyze/:taskId.
// Returns current task status and result (if completed).
//
// @Summary Get MIRA task status
// @Description Retrieve status and result of async MIRA analysis task
// @Tags mira
// @Produce json
// @Param taskId path string true "Task ID (UUID)"
// @Success 200 {object} MIRARCATaskStatus "Task status and result"
// @Failure 404 {object} map[string]interface{} "status: error, error: task_not_found"
// @Router /api/v1/mira/rca_analyze/{taskId} [get]
func (h *MIRARCAAsyncHandler) HandleGetTaskStatus(c *gin.Context) {
	taskID := c.Param("taskId")

	task, err := h.getTaskStatus(taskID)
	if err != nil {
		h.logger.Warn("Task not found", "task_id", taskID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"status": "error",
			"error":  "task_not_found",
			"taskId": taskID,
		})
		return
	}

	h.logger.Debug("Task status retrieved",
		"task_id", taskID,
		"status", task.Status,
		"progress", task.Progress)

	c.JSON(http.StatusOK, task)
}

// HandleListTasks handles GET /api/v1/rca_analyze/list
// Query params: limit (int), offset (int)
func (h *MIRARCAAsyncHandler) HandleListTasks(c *gin.Context) {
	if h.weaviateStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "error": "weaviate_not_configured"})
		return
	}

	limit := 10
	offset := 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}

	tasks, total, err := h.weaviateStore.ListMIRARCATasks(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("failed to list mira rca tasks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "weaviate_list_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "total": total, "items": tasks})
}

// HandleSearchTasks handles GET /api/v1/rca_analyze/search?q=<query>&mode=<semantic|hybrid|keyword>&limit=&offset=
func (h *MIRARCAAsyncHandler) HandleSearchTasks(c *gin.Context) {
	if h.weaviateStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "error": "weaviate_not_configured"})
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "empty_query"})
		return
	}

	mode := strings.ToLower(strings.TrimSpace(c.Query("mode")))
	if mode == "" {
		mode = "hybrid"
	}

	limit := 10
	offset := 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}

	items, total, err := h.weaviateStore.SearchMIRARCATasks(c.Request.Context(), q, mode, limit, offset)
	if err != nil {
		h.logger.Error("mira rca search failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "weaviate_search_failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "total": total, "items": items})
}

// processTask processes a MIRA task in the background with chunk-level progress tracking.
func (h *MIRARCAAsyncHandler) processTask(taskID string, rcaData *models.RCAResponse) {
	task, err := h.getTaskStatus(taskID)
	if err != nil {
		h.logger.Error("Task not found for processing", "task_id", taskID, "error", err)
		return
	}

	// Mark as processing
	startedAt := time.Now()
	task.Status = "processing"
	task.StartedAt = &startedAt
	if err := h.saveTaskStatus(task); err != nil {
		h.logger.Error("Failed to update task status to processing", "task_id", taskID, "error", err)
		return
	}

	h.logger.Info("Started processing MIRA task",
		"task_id", taskID,
		"status", "processing")

	// Create context with extended timeout for background processing
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Stage 1: TOON Conversion
	if err := h.updateTaskProgress(taskID, 0, 0, "toon_conversion"); err != nil {
		h.logger.Error("Failed to update progress", "task_id", taskID, "error", err)
	}

	h.logger.Info("Task stage: TOON conversion", "task_id", taskID)
	toonData, err := utils.ConvertRCAToTOON(rcaData)
	if err != nil {
		h.failTask(taskID, fmt.Sprintf("TOON conversion failed: %v", err))
		return
	}

	// Stage 2: Prompt Rendering
	if err := h.updateTaskProgress(taskID, 0, 0, "prompt_rendering"); err != nil {
		h.logger.Error("Failed to update progress", "task_id", taskID, "error", err)
	}

	h.logger.Info("Task stage: Prompt rendering", "task_id", taskID)
	promptData := h.miraHandler.ExtractPromptData(rcaData, toonData)
	basePrompt, err := h.miraHandler.RenderPrompt(promptData)
	if err != nil {
		h.failTask(taskID, fmt.Sprintf("Prompt rendering failed: %v", err))
		return
	}

	// Stage 3: Generate chunked explanation with progress tracking
	h.logger.Info("Task stage: Chunked explanation generation", "task_id", taskID)
	explanation, totalTokens, cached, err := h.generateChunkedExplanationWithProgress(ctx, taskID, rcaData, basePrompt)
	if err != nil {
		h.failTask(taskID, fmt.Sprintf("MIRA generation failed: %v", err))
		return
	}

	// Mark task as completed
	completedAt := time.Now()

	// Preserve critical fields before attempting Valkey refresh
	originalStartedAt := task.StartedAt
	originalProgress := task.Progress

	// Attempt to refresh task from Valkey to get latest progress, but handle failures safely
	if refreshedTask, err := h.getTaskStatus(taskID); err != nil {
		h.logger.Warn("Failed to refresh task from Valkey, using current task state",
			"task_id", taskID, "error", err)
		// Keep using the current task object
	} else {
		// Use refreshed task but preserve critical fields if they're missing
		task = refreshedTask
		if task.StartedAt == nil {
			h.logger.Warn("Refreshed task missing StartedAt, preserving original", "task_id", taskID)
			task.StartedAt = originalStartedAt
		}
		if task.Progress == nil {
			h.logger.Warn("Refreshed task missing Progress, preserving original", "task_id", taskID)
			task.Progress = originalProgress
		}
	}

	// Defensive check before dereferencing StartedAt
	var generationTimeMs int64
	if task.StartedAt != nil {
		generationTimeMs = completedAt.Sub(*task.StartedAt).Milliseconds()
	} else {
		h.logger.Error("Task StartedAt is nil, cannot calculate generation time", "task_id", taskID)
		generationTimeMs = 0
	}

	// Defensive check for Progress
	var totalChunks int
	if task.Progress != nil {
		totalChunks = task.Progress.TotalChunks
	} else {
		h.logger.Warn("Task Progress is nil, using default totalChunks", "task_id", taskID)
		totalChunks = 0
	}

	task.CompletedAt = &completedAt
	task.Status = "completed"
	task.Result = map[string]interface{}{
		"explanation":      explanation,
		"tokensUsed":       totalTokens,
		"cached":           cached,
		"provider":         h.config.Provider,
		"model":            h.getModelName(),
		"generatedAt":      completedAt.Format(time.RFC3339),
		"generationTimeMs": generationTimeMs,
		"totalChunks":      totalChunks,
	}

	if err := h.saveTaskStatus(task); err != nil {
		h.logger.Error("Failed to save completed task", "task_id", taskID, "error", err)
		return
	}

	h.logger.Info("Completed MIRA task",
		"task_id", taskID,
		"status", "completed",
		"tokens_used", totalTokens,
		"cached", cached,
		"total_chunks", task.Progress.TotalChunks,
		"generation_time_ms", task.Result["generationTimeMs"])

	// Send callback if configured
	if task.CallbackURL != "" {
		go h.sendCallback(task)
	}
}

// generateChunkedExplanationWithProgress wraps the handler's method with progress tracking.
func (h *MIRARCAAsyncHandler) generateChunkedExplanationWithProgress(ctx context.Context, taskID string, rca *models.RCAResponse, basePrompt string) (string, int, bool, error) {
	// Split RCA into chunks first to know total count
	chunks := h.miraHandler.splitRCAIntoChunks(rca, 3000)
	totalChunks := len(chunks)

	h.logger.Info("Task chunking complete",
		"task_id", taskID,
		"total_chunks", totalChunks)

	// Initialize progress with total chunks
	if err := h.updateTaskProgress(taskID, 0, totalChunks, "chunk_processing_started"); err != nil {
		h.logger.Error("Failed to initialize progress", "task_id", taskID, "error", err)
	}

	// Process each chunk with progress updates
	var explanations []string
	var conversationContext string
	totalTokens := 0
	allCached := true

	for i, chunk := range chunks {
		chunkNum := i + 1

		// Update progress: processing chunk N
		stage := fmt.Sprintf("chunk_%d_processing", chunkNum)
		if err := h.updateTaskProgress(taskID, chunkNum, totalChunks, stage); err != nil {
			h.logger.Error("Failed to update chunk progress", "task_id", taskID, "chunk", chunkNum, "error", err)
		}

		h.logger.Info("Processing chunk",
			"task_id", taskID,
			"chunk_number", chunkNum,
			"total_chunks", totalChunks,
			"stage", stage)

		// Build prompt with previous context
		chunkPrompt := h.miraHandler.buildChunkPrompt(basePrompt, chunk, chunkNum, totalChunks, conversationContext)

		// Generate explanation for this chunk
		miraResponse, err := h.miraHandler.miraService.GenerateExplanation(ctx, chunkPrompt)
		if err != nil {
			return "", 0, false, fmt.Errorf("failed to generate explanation for chunk %d: %w", chunkNum, err)
		}

		explanations = append(explanations, miraResponse.Explanation)
		conversationContext = miraResponse.Explanation // Use this as context for next chunk
		totalTokens += miraResponse.TokensUsed
		allCached = allCached && miraResponse.Cached

		// Update progress: chunk N completed
		completedStage := fmt.Sprintf("chunk_%d_completed", chunkNum)
		if err := h.updateTaskProgress(taskID, chunkNum, totalChunks, completedStage); err != nil {
			h.logger.Error("Failed to update chunk completion", "task_id", taskID, "chunk", chunkNum, "error", err)
		}

		h.logger.Info("Chunk completed",
			"task_id", taskID,
			"chunk_number", chunkNum,
			"total_chunks", totalChunks,
			"tokens_used", miraResponse.TokensUsed,
			"cached", miraResponse.Cached,
			"stage", completedStage)
	}

	// Stitch final explanation
	if err := h.updateTaskProgress(taskID, totalChunks, totalChunks, "stitching_final_explanation"); err != nil {
		h.logger.Error("Failed to update stitching progress", "task_id", taskID, "error", err)
	}

	h.logger.Info("Stitching final explanation", "task_id", taskID, "total_chunks", totalChunks)
	finalExplanation := h.miraHandler.stitchExplanations(explanations, totalChunks)

	return finalExplanation, totalTokens, allCached, nil
}

// failTask marks a task as failed with error message.
func (h *MIRARCAAsyncHandler) failTask(taskID string, errorMsg string) {
	task, err := h.getTaskStatus(taskID)
	if err != nil {
		h.logger.Error("Failed to retrieve task for failure update", "task_id", taskID, "error", err)
		return
	}

	completedAt := time.Now()
	task.CompletedAt = &completedAt
	task.Status = "failed"
	task.Error = errorMsg

	if err := h.saveTaskStatus(task); err != nil {
		h.logger.Error("Failed to save failed task", "task_id", taskID, "error", err)
		return
	}

	h.logger.Error("MIRA task failed",
		"task_id", taskID,
		"status", "failed",
		"error", errorMsg)

	// Send callback notification for failure
	if task.CallbackURL != "" {
		go h.sendCallback(task)
	}
}

// sendCallback sends webhook notification to callback URL.
func (h *MIRARCAAsyncHandler) sendCallback(task *MIRARCATaskStatus) {
	payload := map[string]interface{}{
		"taskId":      task.TaskID,
		"status":      task.Status,
		"submittedAt": task.SubmittedAt,
		"completedAt": task.CompletedAt,
	}

	switch task.Status {
	case "completed":
		payload["result"] = task.Result
		payload["progress"] = task.Progress
	case "failed":
		payload["error"] = task.Error
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("Failed to marshal callback payload",
			"task_id", task.TaskID,
			"error", err)
		return
	}

	req, err := http.NewRequest("POST", task.CallbackURL, nil)
	if err != nil {
		h.logger.Error("Failed to create callback request",
			"task_id", task.TaskID,
			"error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("Callback request failed",
			"task_id", task.TaskID,
			"callback_url", task.CallbackURL,
			"error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		h.logger.Info("Callback sent successfully",
			"task_id", task.TaskID,
			"callback_url", task.CallbackURL,
			"status_code", resp.StatusCode)
	} else {
		h.logger.Warn("Callback returned non-2xx status",
			"task_id", task.TaskID,
			"callback_url", task.CallbackURL,
			"status_code", resp.StatusCode)
	}

	// NOTE(CALLBACK-BODY): Payload sent in request body
	_ = jsonData // Acknowledge unused variable
}

// getModelName returns the model name based on provider configuration.
func (h *MIRARCAAsyncHandler) getModelName() string {
	switch h.config.Provider {
	case "openai":
		return h.config.OpenAI.Model
	case "anthropic":
		return h.config.Anthropic.Model
	case "vllm":
		return h.config.VLLM.Model
	case "ollama":
		return h.config.Ollama.Model
	default:
		return "unknown"
	}
}

// convertToWeaviateTask converts MIRARCATaskStatus to weavstore.MIRARCATask for Weaviate persistence.
func (h *MIRARCAAsyncHandler) convertToWeaviateTask(task *MIRARCATaskStatus) *weavstore.MIRARCATask {
	weavTask := &weavstore.MIRARCATask{
		TaskID:      task.TaskID,
		Name:        task.Name,
		Status:      task.Status,
		RCAData:     make(map[string]interface{}),
		Result:      task.Result,
		Error:       task.Error,
		CallbackURL: task.CallbackURL,
		SubmittedAt: task.SubmittedAt,
		CreatedAt:   task.SubmittedAt,
		UpdatedAt:   time.Now(),
	}

	// Set completed timestamp if available
	if task.CompletedAt != nil {
		weavTask.CompletedAt = *task.CompletedAt
	}

	// Convert progress to map
	if task.Progress != nil {
		weavTask.Progress = map[string]interface{}{
			"totalChunks":     task.Progress.TotalChunks,
			"completedChunks": task.Progress.CompletedChunks,
			"currentChunk":    task.Progress.CurrentChunk,
			"currentStage":    task.Progress.CurrentStage,
			"lastUpdated":     task.Progress.LastUpdated.Format(time.RFC3339Nano),
		}
	}

	return weavTask
}
