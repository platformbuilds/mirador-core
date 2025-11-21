package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/services"

	models "github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// KPIRepo extends SchemaStore with KPI-specific operations
// KPIRepo interface is defined in the repo package

// KPIHandler provides API endpoints for KPI definitions.
// This handler implements the separate KPI APIs as defined in the API contract.
type KPIHandler struct {
	repo   repo.KPIRepo
	cache  cache.ValkeyCluster
	logger logger.Logger
	cfg    *config.Config
}

// Validation response types for API consumers
type validationProblem struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

type validationErrorResponse struct {
	Message string              `json:"message"`
	Details []validationProblem `json:"details"`
}

// NewKPIHandler creates a new KPI handler
func NewKPIHandler(cfg *config.Config, kpiRepo repo.KPIRepo, cache cache.ValkeyCluster, l logger.Logger) *KPIHandler {
	if kpiRepo == nil {
		l.Error("KPIRepo is nil - KPI functionality will not be available")
		return nil
	}
	return &KPIHandler{
		repo:   kpiRepo,
		cache:  cache,
		logger: l,
		cfg:    cfg,
	}
} // ------------------- KPI Definitions API -------------------

// GetKPIDefinitions retrieves all KPI definitions with optional filtering
// @Summary Get KPI definitions
// @Description Retrieve a paginated list of KPI definitions with optional filtering by tags
// @Tags kpi-definitions
// @Accept json
// @Produce json
// @Param tags query []string false "Filter by tags (comma-separated)" collectionFormat(csv)
// @Param limit query int false "Maximum number of results (default: 10)" minimum(1) maximum(100)
// @Param offset query int false "Pagination offset (default: 0)" minimum(0)
// @Success 200 {object} models.KPIListResponse
// @Failure 400 {object} map[string]string "error: invalid query parameters"
// @Failure 500 {object} map[string]string "error: failed to list KPIs"
// @Router /api/v1/kpi/defs [get]
// (no internal auth) NOTE: security removed — MIRADOR-CORE is intended to run behind an external gateway
func (h *KPIHandler) GetKPIDefinitions(c *gin.Context) {
	var req models.KPIListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	kpis, total, err := h.listKPIs(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("KPI list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list KPIs"})
		return
	}

	nextOffset := req.Offset + len(kpis)
	if nextOffset >= total {
		nextOffset = 0
	}

	c.JSON(http.StatusOK, models.KPIListResponse{
		KPIDefinitions: kpis,
		Total:          total,
		NextOffset:     nextOffset,
	})
}

// CreateOrUpdateKPIDefinition creates or updates a KPI definition
// @Summary Create or update KPI definition
// @Description Create a new KPI definition or update an existing one. If ID is not provided, a new UUID will be generated.
// @Tags kpi-definitions
// @Accept json
// @Produce json
// @Param kpi body models.KPIDefinitionRequest true "KPI definition payload"
// @Success 200 {object} map[string]interface{} "status: ok, id: kpi_id"
// @Failure 400 {object} map[string]string "error: invalid payload or validation error"
// @Failure 500 {object} map[string]string "error: failed to upsert KPI"
// @Router /api/v1/kpi/defs [post]
// (no internal auth)
func (h *KPIHandler) CreateOrUpdateKPIDefinition(c *gin.Context) {
	var req models.KPIDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.KPIDefinition == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kpi definition is required"})
		return
	}

	kpi := req.KPIDefinition

	// Run semantic validation before generating IDs or persisting
	if err := services.ValidateKPIDefinition(h.cfg, kpi); err != nil {
		// If it's a ValidationError, respond with 400 and structured details
		if ve, ok := err.(*services.ValidationError); ok {
			resp := validationErrorResponse{
				Message: "invalid KPI definition",
				Details: make([]validationProblem, 0, len(ve.Problems)),
			}
			for _, p := range ve.Problems {
				resp.Details = append(resp.Details, validationProblem{Field: p.Field, Error: p.Message})
			}
			c.JSON(http.StatusBadRequest, resp)
			return
		}
		// Unexpected error from validator
		h.logger.Error("KPI validation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate KPI definition"})
		return
	}

	if kpi.ID == "" {
		id, err := services.GenerateDeterministicKPIID(kpi)
		if err != nil {
			h.logger.Error("failed to generate deterministic KPI id", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate kpi id"})
			return
		}
		kpi.ID = id
	}

	kpi.UpdatedAt = time.Now()
	if kpi.CreatedAt.IsZero() {
		kpi.CreatedAt = kpi.UpdatedAt
	}

	var err error
	var status string
	if kpi.ID == "" {
		_, status, err = h.repo.CreateKPI(c.Request.Context(), kpi)
	} else {
		_, status, err = h.repo.ModifyKPI(c.Request.Context(), kpi)
	}
	if err != nil {
		h.logger.Error("KPI create/modify failed", "error", err, "id", kpi.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create/modify KPI"})
		return
	}

	// HTTP semantics: created -> 201; no-change -> 204 No Content; updated -> 200 OK with id
	if status == "created" {
		c.JSON(http.StatusCreated, gin.H{"status": "created", "id": kpi.ID})
		return
	}
	if status == "no-change" {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": kpi.ID})
}

// BulkJSONRequest represents a bulk JSON payload for KPI definitions
type BulkJSONRequest struct {
	Items []*models.KPIDefinition `json:"items"`
}

// BulkFailureDetail describes a field-level validation failure
type BulkFailureDetail struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

// BulkFailure is an entry for a single failed item in a bulk request
type BulkFailure struct {
	Index   int                 `json:"index,omitempty"`
	Row     int                 `json:"row,omitempty"`
	Message string              `json:"message"`
	Details []BulkFailureDetail `json:"details,omitempty"`
}

// BulkSummary is the response for bulk ingest operations
type BulkSummary struct {
	Total        int           `json:"total"`
	SuccessCount int           `json:"successCount"`
	FailureCount int           `json:"failureCount"`
	Failures     []BulkFailure `json:"failures"`
}

// BulkIngestJSON handles POST /api/v1/kpi/defs/bulk-json
func (h *KPIHandler) BulkIngestJSON(c *gin.Context) {
	var req BulkJSONRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	total := len(req.Items)
	summary := BulkSummary{Total: total, Failures: []BulkFailure{}}

	for i, item := range req.Items {
		if item == nil {
			summary.Failures = append(summary.Failures, BulkFailure{Index: i, Message: "item is null"})
			continue
		}

		if err := services.ValidateKPIDefinition(h.cfg, item); err != nil {
			if ve, ok := err.(*services.ValidationError); ok {
				details := make([]BulkFailureDetail, 0, len(ve.Problems))
				for _, p := range ve.Problems {
					details = append(details, BulkFailureDetail{Field: p.Field, Error: p.Message})
				}
				summary.Failures = append(summary.Failures, BulkFailure{Index: i, Message: "invalid KPI definition", Details: details})
				continue
			}
			h.logger.Error("KPI validation failed", "error", err)
			summary.Failures = append(summary.Failures, BulkFailure{Index: i, Message: "validation error"})
			continue
		}

		if item.ID == "" {
			id, err := services.GenerateDeterministicKPIID(item)
			if err != nil {
				h.logger.Error("failed to generate deterministic KPI id", "error", err)
				summary.Failures = append(summary.Failures, BulkFailure{Index: i, Message: "failed to generate id"})
				continue
			}
			item.ID = id
		}

		item.UpdatedAt = time.Now()
		if item.CreatedAt.IsZero() {
			item.CreatedAt = item.UpdatedAt
		}

		_, _, kpiErr := h.repo.CreateKPI(c.Request.Context(), item)
		if kpiErr != nil {
			h.logger.Error("KPI create/modify failed", "error", kpiErr, "id", item.ID)
			// Record the repository error message as the failure message so callers
			// can see why the upsert failed (e.g. Weaviate 422 invalid prop).
			summary.Failures = append(summary.Failures, BulkFailure{Index: i, Message: kpiErr.Error()})
			continue
		}
		summary.SuccessCount++
	}

	summary.FailureCount = len(summary.Failures)
	c.JSON(http.StatusOK, summary)
}

// BulkIngestCSV handles POST /api/v1/kpi/defs/bulk-csv
func (h *KPIHandler) BulkIngestCSV(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read csv header"})
		return
	}

	// normalize headers
	for i := range headers {
		headers[i] = strings.ToLower(strings.TrimSpace(headers[i]))
	}

	var summary BulkSummary
	summary.Failures = []BulkFailure{}

	rowIndex := 1 // header row consumed
	for {
		row, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			summary.Failures = append(summary.Failures, BulkFailure{Row: rowIndex + 1, Message: "failed to read csv row"})
			rowIndex++
			continue
		}
		rowIndex++
		summary.Total++

		k := &models.KPIDefinition{}
		parseExamplesError := false
		for ci, cell := range row {
			if ci >= len(headers) {
				continue
			}
			hname := headers[ci]
			val := strings.TrimSpace(cell)
			switch hname {
			case "id":
				k.ID = val
			case "name":
				k.Name = val
			case "namespace":
				k.Namespace = val
			case "source":
				k.Source = val
			case "sourceid", "source_id":
				k.SourceID = val
			case "kind":
				k.Kind = val
			case "signaltype", "signal_type":
				k.SignalType = val
			case "classifier":
				k.Classifier = val
			case "layer":
				k.Layer = val
			case "querytype", "query_type":
				k.QueryType = val
			case "datastore":
				k.Datastore = val
			case "formula":
				k.Formula = val
			case "definition":
				k.Definition = val
			case "tags":
				if val != "" {
					// split by comma or semicolon
					parts := strings.FieldsFunc(val, func(r rune) bool { return r == ',' || r == ';' })
					for _, p := range parts {
						t := strings.TrimSpace(p)
						if t != "" {
							k.Tags = append(k.Tags, t)
						}
					}
				}
			case "sentiment":
				k.Sentiment = val
			case "domain":
				k.Domain = val
			case "servicefamily", "service_family":
				k.ServiceFamily = val
			case "componenttype", "component_type":
				k.ComponentType = val
			case "retryallowed", "retry_allowed":
				v := strings.ToLower(val)
				if v == "true" || v == "1" || v == "yes" {
					k.RetryAllowed = true
				} else {
					k.RetryAllowed = false
				}
			case "examples":
				if val != "" {
					var ex []map[string]interface{}
					if err := json.Unmarshal([]byte(val), &ex); err != nil {
						// examples column JSON parse failed; record as a parse error for this row
						summary.Failures = append(summary.Failures, BulkFailure{Row: rowIndex, Message: "invalid examples JSON"})
						parseExamplesError = true
						// break out of the column loop; outer loop will skip validation/upsert
						break
					}
					k.Examples = ex
				}
			}
		}

		// If examples parsing failed for this row, it is already recorded as a failure
		// and should not be validated or upserted.
		if parseExamplesError {
			continue
		}

		// Validate
		if err := services.ValidateKPIDefinition(h.cfg, k); err != nil {
			if ve, ok := err.(*services.ValidationError); ok {
				details := make([]BulkFailureDetail, 0, len(ve.Problems))
				for _, p := range ve.Problems {
					details = append(details, BulkFailureDetail{Field: p.Field, Error: p.Message})
				}
				summary.Failures = append(summary.Failures, BulkFailure{Row: rowIndex, Message: "invalid KPI definition", Details: details})
				continue
			}
			h.logger.Error("KPI validation failed", "error", err)
			summary.Failures = append(summary.Failures, BulkFailure{Row: rowIndex, Message: "validation error"})
			continue
		}

		if k.ID == "" {
			id, err := services.GenerateDeterministicKPIID(k)
			if err != nil {
				h.logger.Error("failed to generate deterministic KPI id", "error", err)
				summary.Failures = append(summary.Failures, BulkFailure{Row: rowIndex, Message: "failed to generate id"})
				continue
			}
			k.ID = id
		}

		k.UpdatedAt = time.Now()
		if k.CreatedAt.IsZero() {
			k.CreatedAt = k.UpdatedAt
		}

		_, _, kpiErr := h.repo.CreateKPI(c.Request.Context(), k)
		if kpiErr != nil {
			h.logger.Error("KPI create/modify failed", "error", kpiErr, "id", k.ID)
			// Include the underlying error message in the CSV failure entry.
			summary.Failures = append(summary.Failures, BulkFailure{Row: rowIndex, Message: kpiErr.Error()})
			continue
		}
		summary.SuccessCount++
	}

	summary.FailureCount = len(summary.Failures)
	c.JSON(http.StatusOK, summary)
}

// DeleteKPIDefinition deletes a KPI definition by ID
// @Summary Delete KPI definition
// @Description Delete a KPI definition by its ID. Requires confirmation via query parameter.
// @Tags kpi-definitions
// @Accept json
// @Produce json
// @Param id path string true "KPI definition ID"
// @Param confirm query string true "Confirmation flag (1, true, or yes)" Enums(1,true,yes)
// @Success 200 {object} map[string]interface{} "status: deleted"
// @Failure 400 {object} map[string]string "error: missing id or confirmation required"
// @Failure 500 {object} map[string]string "error: failed to delete KPI"
// @Router /api/v1/kpi/defs/{id} [delete]
// (no internal auth)
func (h *KPIHandler) DeleteKPIDefinition(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "KPI id is required"})
		return
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("confirm")))
	if q != "1" && q != "true" && q != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation required: add ?confirm=1"})
		return
	}

	err := h.deleteKPI(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("KPI delete failed", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete KPI"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ------------------- Implementation methods (extracted from unified handler) -------------------

func (h *KPIHandler) listKPIs(ctx context.Context, req models.KPIListRequest) ([]*models.KPIDefinition, int, error) {
	kpis, total, err := h.repo.ListKPIs(ctx, req)
	return kpis, int(total), err
}

func (h *KPIHandler) deleteKPI(ctx context.Context, id string) error {
	return h.repo.DeleteKPI(ctx, id)
}
