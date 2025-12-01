package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/internal/weavstore"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

type UnifiedQueryHandler struct {
	unifiedEngine services.UnifiedQueryEngine
	logger        logging.Logger
	kpiRepo       repo.KPIRepo
	engineCfg     config.EngineConfig
	failureStore  *weavstore.WeaviateFailureStore
}

func NewUnifiedQueryHandler(unifiedEngine services.UnifiedQueryEngine, logger corelogger.Logger, kpiRepo repo.KPIRepo, cfg config.EngineConfig) *UnifiedQueryHandler {
	return &UnifiedQueryHandler{
		unifiedEngine: unifiedEngine,
		logger:        logging.FromCoreLogger(logger),
		kpiRepo:       kpiRepo,
		engineCfg:     cfg,
	}
}

// SetFailureStore sets the weaviate failure store for the handler
func (h *UnifiedQueryHandler) SetFailureStore(store *weavstore.WeaviateFailureStore) {
	h.failureStore = store
}

// bindUnifiedQuery is tolerant: it accepts either a wrapped payload
// `{"query": {...}}` or a direct `UnifiedQuery` JSON object. It reads
// the raw request body and attempts to unmarshal into both shapes.
func bindUnifiedQuery(c *gin.Context) (*models.UnifiedQueryRequest, error) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	// restore body for downstream readers
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	var wrapped models.UnifiedQueryRequest
	if err := json.Unmarshal(data, &wrapped); err == nil {
		if wrapped.Query != nil {
			return &wrapped, nil
		}
		// continue to try direct shape
	}

	var direct models.UnifiedQuery
	if err := json.Unmarshal(data, &direct); err == nil {
		// accept direct if it has some meaningful fields
		if direct.Query != "" || direct.Type != "" || direct.ID != "" {
			return &models.UnifiedQueryRequest{Query: &direct}, nil
		}
	}

	return nil, fmt.Errorf("invalid unified query payload")
}

// HandleUnifiedQuery handles unified queries across all engines
func (h *UnifiedQueryHandler) HandleUnifiedQuery(c *gin.Context) {
	req, err := bindUnifiedQuery(c)
	if err != nil {
		h.logger.Error("Failed to bind unified query request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Execute the unified query
	result, err := h.unifiedEngine.ExecuteQuery(c.Request.Context(), req.Query)
	if err != nil {
		h.logger.Error("Failed to execute unified query", "error", err, "query_id", req.Query.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Query execution failed",
			"details":  err.Error(),
			"query_id": req.Query.ID,
		})
		return
	}

	response := models.UnifiedQueryResponse{
		Result: result,
	}

	c.JSON(http.StatusOK, response)
}

// HandleUnifiedCorrelation handles correlation queries across engines
func (h *UnifiedQueryHandler) HandleUnifiedCorrelation(c *gin.Context) {
	// Read body once and attempt to parse the canonical TimeWindowRequest first.
	// If strict payload mode is enabled, validate that the JSON contains
	// exactly the two top-level keys `startTime` and `endTime` and reject
	// any other shapes. When strict mode is disabled, the legacy fallback
	// behavior is preserved.
	bodyData, _ := io.ReadAll(c.Request.Body)
	// restore body for downstream readers (including bindUnifiedQuery)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyData))

	// If strict payload validation is enabled, enforce exact schema
	if h.engineCfg.StrictTimeWindowPayload {
		// Decode into a generic map to detect unexpected top-level fields
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(bodyData, &raw); err != nil {
			h.logger.Error("Failed to parse request body (strict payload)", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "details": "invalid JSON"})
			return
		}

		// Allowed keys
		allowed := map[string]bool{"startTime": true, "endTime": true}
		if len(raw) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "details": "empty request body"})
			return
		}
		for k := range raw {
			if !allowed[k] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "details": fmt.Sprintf("unexpected field: %s", k)})
				return
			}
		}

		// Now unmarshal into canonical TimeWindowRequest and validate times
		var tw models.TimeWindowRequest
		if err := json.Unmarshal(bodyData, &tw); err != nil {
			h.logger.Error("Failed to parse time window (strict payload)", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "details": "invalid timewindow shape"})
			return
		}

		tr, terr := tw.ToTimeRange()
		if terr != nil {
			h.logger.Error("Invalid time window request", "error", terr)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_time_window", "details": terr.Error()})
			return
		}

		// Enforce Engine-configured Min/Max window constraints (AT-004)
		windowDur := tr.End.Sub(tr.Start)
		if h.engineCfg.MinWindow > 0 && windowDur < h.engineCfg.MinWindow {
			msg := fmt.Sprintf("time window too small: %s < minWindow %s", windowDur.String(), h.engineCfg.MinWindow.String())
			if h.engineCfg.StrictTimeWindow {
				h.logger.Warn("Rejecting request due to small time window", "details", msg)
				c.JSON(http.StatusBadRequest, gin.H{"error": msg})
				return
			}
			// lenient: warn and continue
			h.logger.Warn("Time window below MinWindow (lenient mode)", "details", msg)
		}
		if h.engineCfg.MaxWindow > 0 && windowDur > h.engineCfg.MaxWindow {
			msg := fmt.Sprintf("time window too large: %s > maxWindow %s", windowDur.String(), h.engineCfg.MaxWindow.String())
			if h.engineCfg.StrictTimeWindow {
				h.logger.Warn("Rejecting request due to large time window", "details", msg)
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": msg})
				return
			}
			h.logger.Warn("Time window above MaxWindow (lenient mode)", "details", msg)
		}

		// Map TimeRange to a lightweight UnifiedQuery (canonical path: TimeWindow -> TimeRange -> internal)
		st := tr.Start
		et := tr.End
		uquery := &models.UnifiedQuery{
			ID:        fmt.Sprintf("timewindow_%d", time.Now().Unix()),
			Type:      models.QueryTypeCorrelation,
			StartTime: &st,
			EndTime:   &et,
		}

		result, err := h.unifiedEngine.ExecuteCorrelationQuery(c.Request.Context(), uquery)
		if err != nil {
			h.logger.Error("Failed to execute unified correlation (time-window)", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Correlation execution failed", "details": err.Error()})
			return
		}

		response := models.UnifiedQueryResponse{Result: result}
		c.JSON(http.StatusOK, response)
		return
	}

	// Non-strict mode: prefer canonical TimeWindowRequest if present, otherwise
	// fall back to legacy UnifiedQuery binding (deprecated behavior).
	var tw models.TimeWindowRequest
	if err := json.Unmarshal(bodyData, &tw); err == nil {
		if tw.StartTime != "" && tw.EndTime != "" {
			tr, terr := tw.ToTimeRange()
			if terr != nil {
				h.logger.Error("Invalid time window request", "error", terr)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid time window", "details": terr.Error()})
				return
			}

			// Enforce Engine-configured Min/Max window constraints (AT-004)
			windowDur := tr.End.Sub(tr.Start)
			if h.engineCfg.MinWindow > 0 && windowDur < h.engineCfg.MinWindow {
				msg := fmt.Sprintf("time window too small: %s < minWindow %s", windowDur.String(), h.engineCfg.MinWindow.String())
				if h.engineCfg.StrictTimeWindow {
					h.logger.Warn("Rejecting request due to small time window", "details", msg)
					c.JSON(http.StatusBadRequest, gin.H{"error": msg})
					return
				}
				// lenient: warn and continue
				h.logger.Warn("Time window below MinWindow (lenient mode)", "details", msg)
			}
			if h.engineCfg.MaxWindow > 0 && windowDur > h.engineCfg.MaxWindow {
				msg := fmt.Sprintf("time window too large: %s > maxWindow %s", windowDur.String(), h.engineCfg.MaxWindow.String())
				if h.engineCfg.StrictTimeWindow {
					h.logger.Warn("Rejecting request due to large time window", "details", msg)
					c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": msg})
					return
				}
				h.logger.Warn("Time window above MaxWindow (lenient mode)", "details", msg)
			}

			// Map TimeRange to a lightweight UnifiedQuery (canonical path: TimeWindow -> TimeRange -> internal)
			st := tr.Start
			et := tr.End
			uquery := &models.UnifiedQuery{
				ID:        fmt.Sprintf("timewindow_%d", time.Now().Unix()),
				Type:      models.QueryTypeCorrelation,
				StartTime: &st,
				EndTime:   &et,
			}

			result, err := h.unifiedEngine.ExecuteCorrelationQuery(c.Request.Context(), uquery)
			if err != nil {
				h.logger.Error("Failed to execute unified correlation (time-window)", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Correlation execution failed", "details": err.Error()})
				return
			}

			response := models.UnifiedQueryResponse{Result: result}
			c.JSON(http.StatusOK, response)
			return
		}
	}

	// Fallback: legacy UnifiedQuery binding (deprecated behavior)
	req, err := bindUnifiedQuery(c)
	if err != nil {
		// If body is empty or whitespace, treat as request for correlating all KPIs
		bodyData2, _ := io.ReadAll(c.Request.Body)
		// restore body for potential downstream use
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyData2))
		if len(bytes.TrimSpace(bodyData2)) == 0 {
			// Build a correlation query from all KPIs
			if h.kpiRepo == nil {
				h.logger.Error("KPI repo not configured; cannot build correlation over all KPIs")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository not available"})
				return
			}

			// Fetch all KPIs (use a large limit)
			kpiReq := models.KPIListRequest{Limit: 10000, Offset: 0}
			kpis, _, err := h.kpiRepo.ListKPIs(c.Request.Context(), kpiReq)
			if err != nil {
				h.logger.Error("Failed to list KPIs for correlation", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list KPIs"})
				return
			}

			// Build correlation query string by combining KPI expressions with AND
			// Each KPI's expression prefers Formula, then Query["query"] if available, otherwise the KPI name.
			var exprParts []string
			for _, k := range kpis {
				engine := "metrics"
				if k.QueryType != "" {
					engine = k.QueryType
				} else if k.SignalType != "" {
					engine = k.SignalType
				}

				// Normalize engine identifiers for correlation grammar
				switch engine {
				case "logs", "traces", "metrics":
					// ok
				default:
					// fallback to metrics
					engine = "metrics"
				}

				var qstr string
				if k.Formula != "" {
					qstr = k.Formula
				} else if k.Query != nil {
					if v, ok := k.Query["query"].(string); ok && v != "" {
						qstr = v
					}
				}
				if qstr == "" {
					qstr = k.Name
				}

				// Escape whitespace sensibly by wrapping in parentheses if needed
				if len(qstr) > 0 {
					exprParts = append(exprParts, fmt.Sprintf("%s:%s", engine, qstr))
				}
			}

			if len(exprParts) == 0 {
				h.logger.Error("No KPI expressions available to build correlation query")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "No KPIs available for correlation"})
				return
			}

			corrQueryStr := strings.Join(exprParts, " AND ")
			uquery := &models.UnifiedQuery{
				ID:    fmt.Sprintf("kpi_all_%d", time.Now().Unix()),
				Type:  models.QueryTypeCorrelation,
				Query: corrQueryStr,
			}

			result, err := h.unifiedEngine.ExecuteCorrelationQuery(c.Request.Context(), uquery)
			if err != nil {
				h.logger.Error("Failed to execute unified correlation (all KPIs)", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Correlation execution failed", "details": err.Error()})
				return
			}

			response := models.UnifiedQueryResponse{Result: result}
			c.JSON(http.StatusOK, response)
			return
		}

		h.logger.Error("Failed to bind unified correlation request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// If the provided unified query has no textual query, treat it as
	// a request to correlate across all KPIs (fetch from KPI repo).
	if req.Query.Query == "" {
		if h.kpiRepo == nil {
			h.logger.Error("KPI repo not configured; cannot build correlation over all KPIs")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "KPI repository not available"})
			return
		}

		// Fetch KPIs (use a large but bounded limit)
		kpiReq := models.KPIListRequest{Limit: 10000, Offset: 0}
		kpis, _, err := h.kpiRepo.ListKPIs(c.Request.Context(), kpiReq)
		if err != nil {
			h.logger.Error("Failed to list KPIs for correlation", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list KPIs"})
			return
		}

		var exprParts []string
		for _, k := range kpis {
			engine := "metrics"
			if k.QueryType != "" {
				engine = k.QueryType
			} else if k.SignalType != "" {
				engine = k.SignalType
			}
			switch engine {
			case "logs", "traces", "metrics":
			default:
				engine = "metrics"
			}

			var qstr string
			if k.Formula != "" {
				qstr = k.Formula
			} else if k.Query != nil {
				if v, ok := k.Query["query"].(string); ok && v != "" {
					qstr = v
				}
			}
			if qstr == "" {
				qstr = k.Name
			}

			// wrap in parentheses if containing spaces to keep parser tokens
			if strings.ContainsAny(qstr, " \t\n") {
				qstr = fmt.Sprintf("(%s)", qstr)
			}
			exprParts = append(exprParts, fmt.Sprintf("%s:%s", engine, qstr))
		}

		if len(exprParts) == 0 {
			h.logger.Error("No KPIs available to build correlation query")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No KPIs available for correlation"})
			return
		}

		req.Query.Query = strings.Join(exprParts, " AND ")
	}

	// Ensure this is a correlation query
	if req.Query.Type != models.QueryTypeCorrelation {
		req.Query.Type = models.QueryTypeCorrelation
	}

	// Execute the correlation query
	result, err := h.unifiedEngine.ExecuteCorrelationQuery(c.Request.Context(), req.Query)
	if err != nil {
		h.logger.Error("Failed to execute unified correlation", "error", err, "query_id", req.Query.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Correlation execution failed",
			"details":  err.Error(),
			"query_id": req.Query.ID,
		})
		return
	}

	response := models.UnifiedQueryResponse{
		Result: result,
	}

	c.JSON(http.StatusOK, response)
}

// HandleQueryMetadata returns metadata about supported query capabilities
func (h *UnifiedQueryHandler) HandleQueryMetadata(c *gin.Context) {
	metadata, err := h.unifiedEngine.GetQueryMetadata(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get query metadata", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve query metadata",
		})
		return
	}

	c.JSON(http.StatusOK, metadata)
}

// HandleHealthCheck returns health status of all engines
func (h *UnifiedQueryHandler) HandleHealthCheck(c *gin.Context) {
	health, err := h.unifiedEngine.HealthCheck(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get health status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve health status",
		})
		return
	}

	statusCode := http.StatusOK
	switch health.OverallHealth {
	case "unhealthy":
		statusCode = http.StatusServiceUnavailable
	case "partial":
		statusCode = http.StatusPartialContent
	}

	c.JSON(statusCode, health)
}

// HandleUnifiedSearch handles unified search across all engines
func (h *UnifiedQueryHandler) HandleUnifiedSearch(c *gin.Context) {
	req, err := bindUnifiedQuery(c)
	if err != nil {
		h.logger.Error("Failed to bind unified search request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// For search, we can route to the appropriate engine based on query content
	result, err := h.unifiedEngine.ExecuteQuery(c.Request.Context(), req.Query)
	if err != nil {
		h.logger.Error("Failed to execute unified search", "error", err, "query_id", req.Query.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Search execution failed",
			"details":  err.Error(),
			"query_id": req.Query.ID,
		})
		return
	}

	response := models.UnifiedQueryResponse{
		Result: result,
	}

	c.JSON(http.StatusOK, response)
}

// HandleUQLQuery handles direct UQL query execution
func (h *UnifiedQueryHandler) HandleUQLQuery(c *gin.Context) {
	var req models.UQLQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind UQL query request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Execute the UQL query
	result, err := h.unifiedEngine.ExecuteUQLQuery(c.Request.Context(), req.Query)
	if err != nil {
		h.logger.Error("Failed to execute UQL query", "error", err, "query_id", req.Query.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "UQL query execution failed",
			"details":  err.Error(),
			"query_id": req.Query.ID,
		})
		return
	}

	response := models.UnifiedQueryResponse{
		Result: result,
	}

	c.JSON(http.StatusOK, response)
}

// HandleUQLValidate validates UQL query syntax without execution
func (h *UnifiedQueryHandler) HandleUQLValidate(c *gin.Context) {
	var req models.UQLValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind UQL validate request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// For validation, we can use the UQL parser to check syntax
	// This is a simplified validation - in practice, you'd want more comprehensive validation
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Query cannot be empty",
		})
		return
	}

	// Basic validation response
	response := models.UQLValidateResponse{
		Valid: true,
		Query: req.Query,
	}

	c.JSON(http.StatusOK, response)
}

// HandleUQLExplain provides query execution plan for UQL queries
func (h *UnifiedQueryHandler) HandleUQLExplain(c *gin.Context) {
	var req models.UQLExplainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind UQL explain request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// For explain, we would ideally get the execution plan from the optimizer
	// This is a simplified response - in practice, you'd integrate with the optimizer
	explainResult := models.UQLExplainResponse{
		Query: req.Query.Query,
		Plan: models.QueryPlan{
			Steps: []models.QueryPlanStep{
				{
					Type:        "parse",
					Description: "Parse UQL query into AST",
					Engine:      "uql_parser",
				},
				{
					Type:        "optimize",
					Description: "Apply query optimizations",
					Engine:      "uql_optimizer",
				},
				{
					Type:        "translate",
					Description: "Translate to engine-specific queries",
					Engine:      "uql_translator",
				},
				{
					Type:        "execute",
					Description: "Execute translated queries",
					Engine:      "unified_engine",
				},
			},
		},
	}

	c.JSON(http.StatusOK, explainResult)
}

// HandleUnifiedStats returns statistics about unified query operations
func (h *UnifiedQueryHandler) HandleUnifiedStats(c *gin.Context) {
	// Get health status and metadata to provide basic statistics
	health, err := h.unifiedEngine.HealthCheck(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get health status for stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve query statistics",
		})
		return
	}

	metadata, err := h.unifiedEngine.GetQueryMetadata(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get query metadata for stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve query statistics",
		})
		return
	}

	// Build basic statistics response
	stats := gin.H{
		"unified_query_engine": gin.H{
			"health":            health.OverallHealth,
			"supported_engines": metadata.SupportedEngines,
			"cache_enabled":     metadata.CacheCapabilities.Supported,
			"cache_default_ttl": metadata.CacheCapabilities.DefaultTTL.String(),
			"cache_max_ttl":     metadata.CacheCapabilities.MaxTTL.String(),
			"last_health_check": health.LastChecked,
		},
		"engines": health.EngineHealth,
	}

	c.JSON(http.StatusOK, stats)
}

// HandleFailureDetection detects component failures in the financial transaction system
func (h *UnifiedQueryHandler) HandleFailureDetection(c *gin.Context) {
	var req models.FailureDetectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind failure detection request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate time window: end must be after start
	if !req.TimeRange.End.After(req.TimeRange.Start) {
		h.logger.Warn("Invalid time window", "start", req.TimeRange.Start, "end", req.TimeRange.End)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid time window: end time must be after start time",
		})
		return
	}

	// Execute failure detection (forward optional services list)
	result, err := h.unifiedEngine.DetectComponentFailures(c.Request.Context(), req.TimeRange, req.Components, req.Services)
	if err != nil {
		h.logger.Error("Failed to detect component failures", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failure detection failed",
			"details": err.Error(),
		})
		return
	}

	// Persist failures to store if available
	if h.failureStore != nil {
		h.logger.Info("Failure store is available, attempting to persist failures",
			"service_component_count", len(result.Summary.ServiceComponentSummaries))
		if result.Summary.ServiceComponentSummaries != nil && len(result.Summary.ServiceComponentSummaries) > 0 {
			for _, svc := range result.Summary.ServiceComponentSummaries {
				h.logger.Info("Persisting failure record",
					"failure_id", svc.FailureID,
					"service", svc.Service,
					"component", svc.Component,
					"failure_uuid", svc.FailureUUID)

				// Extract signals relevant to this service+component
				var errorSignals []weavstore.FailureSignal
				var anomalySignals []weavstore.FailureSignal

				// Populate signals from result if available
				// Convert models.FailureSignal to weavstore.FailureSignal
				// Note: This stores the raw verbose data from the correlation result

				// Create a failure record for each service+component combination with full details
				// Note: Services and Components are stored as single strings, not arrays
				// (Weaviate schema defines them as text, not text[])
				failureRecord := &weavstore.FailureRecord{
					FailureUUID:        svc.FailureUUID,
					FailureID:          svc.FailureID,
					TimeRange:          weavstore.TimeRange{Start: req.TimeRange.Start, End: req.TimeRange.End},
					Services:           []string{svc.Service},   // Single service
					Components:         []string{svc.Component}, // Single component
					RawErrorSignals:    errorSignals,            // Store all error signals
					RawAnomalySignals:  anomalySignals,          // Store all anomaly signals
					DetectionTimestamp: time.Now(),
					DetectorVersion:    "1.0.0", // TODO: Get from config/version
					ConfidenceScore:    svc.AverageConfidence,
					CreatedAt:          time.Now(),
					UpdatedAt:          time.Now(),
				}

				if _, _, err := h.failureStore.CreateOrUpdateFailure(c.Request.Context(), failureRecord); err != nil {
					h.logger.Warn("Failed to persist failure record",
						"failure_id", svc.FailureID,
						"service", svc.Service,
						"component", svc.Component,
						"error", err)
					// Don't fail the request if storage fails - still return the detected results
				} else {
					h.logger.Info("Successfully persisted failure record",
						"failure_id", svc.FailureID,
						"service", svc.Service,
						"component", svc.Component)
				}
			}
		} else {
			h.logger.Warn("No service component summaries to persist")
		}
	} else {
		h.logger.Warn("Failure store is nil - not persisting failures")
	}

	c.JSON(http.StatusOK, result)
}

// HandleTransactionFailureCorrelation correlates failures for specific transaction IDs
func (h *UnifiedQueryHandler) HandleTransactionFailureCorrelation(c *gin.Context) {
	var req models.TransactionFailureCorrelationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind transaction failure correlation request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Execute transaction failure correlation
	result, err := h.unifiedEngine.CorrelateTransactionFailures(c.Request.Context(), req.TransactionIDs, req.TimeRange)
	if err != nil {
		h.logger.Error("Failed to correlate transaction failures", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Transaction failure correlation failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleGetFailures returns paginated list of failures with minimal info (id, summary, timestamps)
func (h *UnifiedQueryHandler) HandleGetFailures(c *gin.Context) {
	if h.failureStore == nil {
		h.logger.Error("Failure store not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Failure store not available",
		})
		return
	}

	// Parse JSON body for pagination parameters
	var req struct {
		Limit  int `json:"limit" binding:"omitempty"`
		Offset int `json:"offset" binding:"omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to parse list failures request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	limit := 100
	offset := 0
	if req.Limit > 0 {
		limit = req.Limit
	}
	if req.Offset >= 0 {
		offset = req.Offset
	}

	failures, total, err := h.failureStore.ListFailures(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to retrieve failures from store", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve failures",
			"details": err.Error(),
		})
		return
	}

	// Map to minimal summary response
	summaries := make([]gin.H, 0, len(failures))
	for _, f := range failures {
		summary := gin.H{
			"failure_id": f.FailureID,
			"summary": gin.H{
				"services":      f.Services,
				"components":    f.Components,
				"detector":      f.DetectorVersion,
				"confidence":    f.ConfidenceScore,
				"error_count":   len(f.RawErrorSignals),
				"anomaly_count": len(f.RawAnomalySignals),
			},
			"timestamps": gin.H{
				"detection":  f.DetectionTimestamp,
				"start":      f.TimeRange.Start,
				"end":        f.TimeRange.End,
				"created_at": f.CreatedAt,
				"updated_at": f.UpdatedAt,
			},
		}
		summaries = append(summaries, summary)
	}

	c.JSON(http.StatusOK, gin.H{
		"failures": summaries,
		"count":    len(summaries),
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// HandleGetFailureDetail returns the full failure record including verbose output
func (h *UnifiedQueryHandler) HandleGetFailureDetail(c *gin.Context) {
	if h.failureStore == nil {
		h.logger.Error("Failure store not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Failure store not available",
		})
		return
	}

	// Parse JSON body for failure_id
	var req struct {
		FailureID string `json:"failure_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to parse failure detail request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Search by human-readable failure_id (using GetFailureByID method)
	failure, err := h.failureStore.GetFailureByID(c.Request.Context(), req.FailureID)
	if err != nil {
		h.logger.Error("Failed to retrieve failure detail", "failure_id", req.FailureID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve failure",
			"details": err.Error(),
		})
		return
	}

	if failure == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Failure not found",
		})
		return
	}

	// Return the full failure record with verbose output
	c.JSON(http.StatusOK, gin.H{
		"failure": failure,
	})
}

// HandleDeleteFailure deletes a failure record by its ID
func (h *UnifiedQueryHandler) HandleDeleteFailure(c *gin.Context) {
	if h.failureStore == nil {
		h.logger.Error("Failure store not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Failure store not available",
		})
		return
	}

	// Parse JSON body for failure_id
	var req struct {
		FailureID string `json:"failure_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to parse delete failure request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Delete by human-readable failure_id (using DeleteFailureByID method)
	if err := h.failureStore.DeleteFailureByID(c.Request.Context(), req.FailureID); err != nil {
		h.logger.Error("Failed to delete failure", "failure_id", req.FailureID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete failure",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Failure deleted successfully",
		"failure_id": req.FailureID,
	})
}

// HandleStoreFailure stores a failure record in the failure store (deprecated - use POST /failures/store)
func (h *UnifiedQueryHandler) HandleStoreFailure(c *gin.Context) {
	if h.failureStore == nil {
		h.logger.Error("Failure store not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Failure store not available",
		})
		return
	}

	var failure weavstore.FailureRecord
	if err := c.ShouldBindJSON(&failure); err != nil {
		h.logger.Error("Failed to bind failure record", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	result, status, err := h.failureStore.CreateOrUpdateFailure(c.Request.Context(), &failure)
	if err != nil {
		h.logger.Error("Failed to store failure record", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to store failure",
			"details": err.Error(),
		})
		return
	}

	statusCode := http.StatusCreated
	if status == "updated" || status == "no-change" {
		statusCode = http.StatusOK
	}

	c.JSON(statusCode, gin.H{
		"message": "Failure record stored successfully",
		"status":  status,
		"failure": result,
	})
}

// HandleClearFailures clears all failure records from the failure store (deprecated)
func (h *UnifiedQueryHandler) HandleClearFailures(c *gin.Context) {
	if h.failureStore == nil {
		h.logger.Error("Failure store not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Failure store not available",
		})
		return
	}

	// Get all failures and delete them one by one
	failures, _, err := h.failureStore.ListFailures(c.Request.Context(), 10000, 0)
	if err != nil {
		h.logger.Error("Failed to list failures for clearing", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to clear failures",
			"details": err.Error(),
		})
		return
	}

	deleted := 0
	for _, failure := range failures {
		if err := h.failureStore.DeleteFailure(c.Request.Context(), failure.FailureUUID); err != nil {
			h.logger.Warn("Failed to delete failure record", "uuid", failure.FailureUUID, "error", err)
		} else {
			deleted++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Failure records cleared successfully",
		"deleted": deleted,
		"total":   len(failures),
	})
}
