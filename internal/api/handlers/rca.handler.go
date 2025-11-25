package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/rca"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/services"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// RCAHandler handles RCA HTTP endpoints.
type RCAHandler struct {
	logsService             services.LogsService
	serviceGraph            services.ServiceGraphFetcher
	cache                   cache.ValkeyCluster
	logger                  logging.Logger
	rcaEngine               rca.RCAEngine
	engineCfg               config.EngineConfig
	featureFlagService      *services.RuntimeFeatureFlagService
	strictTimeWindowPayload bool
	kpiRepo                 repo.KPIRepo
}

func NewRCAHandler(logs services.LogsService, sg services.ServiceGraphFetcher, cch cache.ValkeyCluster, logger corelogger.Logger, engine rca.RCAEngine, engCfg config.EngineConfig, kpiRepo repo.KPIRepo) *RCAHandler {
	return &RCAHandler{
		logsService:             logs,
		serviceGraph:            sg,
		cache:                   cch,
		logger:                  logging.FromCoreLogger(logger),
		rcaEngine:               engine,
		engineCfg:               engCfg,
		strictTimeWindowPayload: engCfg.StrictTimeWindowPayload,
		featureFlagService:      services.NewRuntimeFeatureFlagService(cch, logger),
		kpiRepo:                 kpiRepo,
	}
}

func (h *RCAHandler) SetEngine(engine rca.RCAEngine) {
	h.rcaEngine = engine
}

// HandleComputeRCA handles POST /api/v1/unified/rca.
func (h *RCAHandler) HandleComputeRCA(c *gin.Context) {
	bodyData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "failed_to_read_body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyData))

	var tw models.TimeWindowRequest
	var tr models.TimeRange
	twUsed := false

	// Strict payload: only accept canonical time-window shape
	if h.strictTimeWindowPayload {
		dec := json.NewDecoder(bytes.NewReader(bodyData))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&tw); err != nil {
			h.logger.Error("Failed to decode strict TimeWindowRequest", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid_timewindow_payload"})
			return
		}
		if tw.StartTime == "" || tw.EndTime == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "both startTime and endTime are required"})
			return
		}
		parsedTR, terr := tw.ToTimeRange()
		if terr != nil {
			h.logger.Error("Invalid time window", "error", terr)
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": terr.Error()})
			return
		}
		tr = parsedTR

		if ok, msg := h.validateWindow(tr); !ok {
			h.logger.Warn("Time window validation failed (strict)", "details", msg)
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": msg})
			return
		}

		if trRunner, ok := h.rcaEngine.(interface {
			ComputeRCAByTimeRange(ctx context.Context, tr rca.TimeRange) (*rca.RCAIncident, error)
		}); ok {
			rtr := rca.TimeRange{Start: tr.Start, End: tr.End}
			rcaIncident, err := trRunner.ComputeRCAByTimeRange(c.Request.Context(), rtr)
			if err != nil {
				h.logger.Error("RCA computation failed (time-range)", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": fmt.Sprintf("RCA computation failed: %v", err)})
				return
			}
			dto := h.convertRCAIncidentToDTO(rcaIncident)
			c.JSON(http.StatusOK, models.RCAResponse{Status: "success", Data: dto, Timestamp: time.Now().UTC()})
			return
		}

		c.JSON(http.StatusNotImplemented, gin.H{"status": "error", "error": "time-range RCA not implemented by configured engine"})
		return
	}

	// Non-strict: prefer time-window if present
	if err := json.Unmarshal(bodyData, &tw); err == nil && tw.StartTime != "" && tw.EndTime != "" {
		if parsedTR, terr := tw.ToTimeRange(); terr == nil {
			tr = parsedTR
			twUsed = true
			if ok, msg := h.validateWindow(tr); !ok {
				if h.engineCfg.StrictTimeWindow {
					h.logger.Warn("Rejecting request due to time window validation", "details", msg)
					c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": msg})
					return
				}
				h.logger.Warn("Time window outside configured bounds (lenient)", "details", msg)
			}
		} else {
			h.logger.Error("Invalid time window in request", "error", terr)
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": terr.Error()})
			return
		}
	}

	if twUsed {
		if trRunner, ok := h.rcaEngine.(interface {
			ComputeRCAByTimeRange(ctx context.Context, tr rca.TimeRange) (*rca.RCAIncident, error)
		}); ok {
			rtr := rca.TimeRange{Start: tr.Start, End: tr.End}
			rcaIncident, err := trRunner.ComputeRCAByTimeRange(c.Request.Context(), rtr)
			if err != nil {
				h.logger.Error("RCA computation failed (time-range)", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": fmt.Sprintf("RCA computation failed: %v", err)})
				return
			}
			dto := h.convertRCAIncidentToDTO(rcaIncident)
			c.JSON(http.StatusOK, models.RCAResponse{Status: "success", Data: dto, Timestamp: time.Now().UTC()})
			return
		}

	}

	// Legacy RCARequest path
	var legacyReq models.RCARequest
	if err := json.Unmarshal(bodyData, &legacyReq); err != nil {
		h.logger.Error("Failed to parse legacy RCARequest", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": fmt.Sprintf("invalid_request_format: %v", err)})
		return
	}
	if legacyReq.ImpactService == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "impactService is required"})
		return
	}

	tStart, err := time.Parse(time.RFC3339, legacyReq.TimeStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": fmt.Sprintf("invalid timeStart: %v", err)})
		return
	}
	tEnd, err := time.Parse(time.RFC3339, legacyReq.TimeEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": fmt.Sprintf("invalid timeEnd: %v", err)})
		return
	}
	if !tStart.Before(tEnd) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "timeStart must be before timeEnd"})
		return
	}

	if tr == (models.TimeRange{}) {
		tr = models.TimeRange{Start: tStart, End: tEnd}
	}
	if ok, msg := h.validateWindow(tr); !ok {
		if h.engineCfg.StrictTimeWindow {
			h.logger.Warn("Rejecting request due to time window validation", "details", msg)
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": msg})
			return
		}
		h.logger.Warn("Time window out of bounds (lenient mode)", "details", msg)
	}

	tPeak := tStart.Add(tEnd.Sub(tStart) / 2)
	impactMetric := legacyReq.ImpactMetric
	if impactMetric == "" {
		impactMetric = "error_rate"
	}
	metricDirection := legacyReq.MetricDirection
	if metricDirection == "" {
		metricDirection = "higher_is_worse"
	}
	severity := legacyReq.Severity
	if severity == 0 {
		severity = 0.7
	}
	impactSummary := legacyReq.ImpactSummary
	if impactSummary == "" {
		impactSummary = fmt.Sprintf("Incident on %s (metric: %s)", legacyReq.ImpactService, impactMetric)
	}

	incidentContext := &rca.IncidentContext{
		ID:            fmt.Sprintf("incident_%d", time.Now().UnixNano()),
		ImpactService: legacyReq.ImpactService,
		ImpactSignal: rca.ImpactSignal{
			ServiceName: legacyReq.ImpactService,
			MetricName:  impactMetric,
			Direction:   metricDirection,
			Labels:      map[string]string{},
			Threshold:   0.0,
		},
		TimeBounds:    rca.IncidentTimeWindow{TStart: tStart, TPeak: tPeak, TEnd: tEnd},
		ImpactSummary: impactSummary,
		Severity:      severity,
		CreatedAt:     time.Now().UTC(),
	}

	opts := rca.DefaultRCAOptions()
	if legacyReq.MaxChains > 0 {
		opts.MaxChains = legacyReq.MaxChains
	}
	if legacyReq.MaxStepsPerChain > 0 {
		opts.MaxStepsPerChain = legacyReq.MaxStepsPerChain
	}
	if legacyReq.MinScoreThreshold > 0 {
		opts.MinScoreThreshold = legacyReq.MinScoreThreshold
	}
	if legacyReq.DimensionConfig != nil {
		opts.DimensionConfig = rca.RCADimensionConfig{
			ExtraDimensions:  legacyReq.DimensionConfig.ExtraDimensions,
			DimensionWeights: legacyReq.DimensionConfig.DimensionWeights,
			AlignmentPenalty: legacyReq.DimensionConfig.AlignmentPenalty,
			AlignmentBonus:   legacyReq.DimensionConfig.AlignmentBonus,
		}
	}

	rcaIncident, err := h.rcaEngine.ComputeRCA(c.Request.Context(), incidentContext, opts)
	if err != nil {
		h.logger.Error("RCA computation failed (legacy)", "incident_id", incidentContext.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": fmt.Sprintf("RCA computation failed: %v", err)})
		return
	}

	dto := h.convertRCAIncidentToDTO(rcaIncident)
	c.JSON(http.StatusOK, models.RCAResponse{Status: "success", Data: dto, Timestamp: time.Now().UTC()})
}

func (h *RCAHandler) validateWindow(tr models.TimeRange) (bool, string) {
	if tr.Start.IsZero() || tr.End.IsZero() {
		return false, "startTime and endTime must be provided"
	}
	if !tr.Start.Before(tr.End) {
		return false, "endTime must be after startTime"
	}
	windowDur := tr.End.Sub(tr.Start)
	if h.engineCfg.MinWindow > 0 && windowDur < h.engineCfg.MinWindow {
		return false, fmt.Sprintf("time window too small: %s < minWindow %s", windowDur.String(), h.engineCfg.MinWindow.String())
	}
	if h.engineCfg.MaxWindow > 0 && windowDur > h.engineCfg.MaxWindow {
		return false, fmt.Sprintf("time window too large: %s > maxWindow %s", windowDur.String(), h.engineCfg.MaxWindow.String())
	}
	return true, ""
}

// StartInvestigation handles POST /rca/investigate - external AI-driven investigations.
func (h *RCAHandler) StartInvestigation(c *gin.Context) {
	// If no external RCA engine is configured, return Not Implemented
	if h.rcaEngine == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"status": "error", "error": "not_implemented"})
		return
	}

	// For now, external investigations are not supported by the local engine.
	c.JSON(http.StatusNotImplemented, gin.H{"status": "error", "error": "not_implemented"})
}

// GetServiceGraph handles POST /rca/service-graph
func (h *RCAHandler) GetServiceGraph(c *gin.Context) {
	var req models.ServiceGraphRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid_request_format"})
		return
	}
	if h.serviceGraph == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "error": "service_graph_unavailable"})
		return
	}
	// Validate times
	if req.Start.IsZero() || req.End.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid_time_range"})
		return
	}
	data, err := h.serviceGraph.FetchServiceGraph(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Service graph fetch failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed_to_fetch_service_graph"})
		return
	}

	if data == nil {
		h.logger.Warn("Service graph returned no data")
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed_to_fetch_service_graph"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "edges": data.Edges, "window": data.Window})
}

func (h *RCAHandler) convertRCAIncidentToDTO(inc *rca.RCAIncident) *models.RCAIncidentDTO {
	if inc == nil {
		return nil
	}
	dto := &models.RCAIncidentDTO{
		Impact:      h.convertIncidentContext(inc.Impact),
		RootCause:   nil,
		Chains:      make([]*models.RCAChainDTO, 0, len(inc.Chains)),
		GeneratedAt: inc.GeneratedAt,
		Score:       inc.Score,
		Notes:       append([]string{}, inc.Notes...),
		Diagnostics: h.convertDiagnostics(inc.Diagnostics),
	}
	if inc.RootCause != nil {
		dto.RootCause = h.convertRCAStep(inc.RootCause)
	}
	for _, ch := range inc.Chains {
		dto.Chains = append(dto.Chains, h.convertRCAChain(ch))
	}
	return dto
}

func (h *RCAHandler) convertIncidentContext(ic *rca.IncidentContext) *models.IncidentContextDTO {
	if ic == nil {
		return nil
	}
	return &models.IncidentContextDTO{
		ID:            ic.ID,
		ImpactService: ic.ImpactService,
		MetricName:    ic.ImpactSignal.MetricName,
		TimeStartStr:  ic.TimeBounds.TStart.Format(time.RFC3339),
		TimeEndStr:    ic.TimeBounds.TEnd.Format(time.RFC3339),
		ImpactSummary: ic.ImpactSummary,
		Severity:      ic.Severity,
	}
}

func (h *RCAHandler) convertRCAChain(ch *rca.RCAChain) *models.RCAChainDTO {
	if ch == nil {
		return nil
	}
	dto := &models.RCAChainDTO{
		Steps:        make([]*models.RCAStepDTO, 0, len(ch.Steps)),
		Score:        ch.Score,
		Rank:         ch.Rank,
		ImpactPath:   append([]string{}, ch.ImpactPath...),
		DurationHops: ch.DurationHops,
	}
	for _, s := range ch.Steps {
		dto.Steps = append(dto.Steps, h.convertRCAStep(s))
	}
	return dto
}

func (h *RCAHandler) convertRCAStep(s *rca.RCAStep) *models.RCAStepDTO {
	if s == nil {
		return nil
	}
	ev := make([]*models.EvidenceRefDTO, 0, len(s.Evidence))
	for _, e := range s.Evidence {
		ev = append(ev, &models.EvidenceRefDTO{Type: e.Type, ID: e.ID, Details: e.Details})
	}

	dto := &models.RCAStepDTO{
		WhyIndex:  s.WhyIndex,
		Service:   s.Service,
		Component: s.Component,
		TimeStart: s.TimeRange.Start,
		TimeEnd:   s.TimeRange.End,
		Ring:      fmt.Sprint(s.Ring),
		Direction: fmt.Sprint(s.Direction),
		Distance:  s.Distance,
		Evidence:  ev,
		Summary:   s.Summary,
		Score:     s.Score,
	}

	// Enrich with KPI metadata if available
	h.enrichKPIMetadata(dto)

	return dto
}

// enrichKPIMetadata looks up KPI information for Service and Component fields
// and populates KPIName and KPIFormula fields if they are KPI UUIDs.
func (h *RCAHandler) enrichKPIMetadata(dto *models.RCAStepDTO) {
	if dto == nil {
		h.logger.Debug("enrichKPIMetadata: dto is nil")
		return
	}
	if h.kpiRepo == nil {
		h.logger.Debug("enrichKPIMetadata: kpiRepo is nil")
		return
	}

	h.logger.Debug("enrichKPIMetadata called", "service", dto.Service, "component", dto.Component)

	ctx := context.Background()

	// Try to look up Service as a KPI UUID
	if dto.Service != "" {
		h.logger.Debug("Looking up Service as KPI", "uuid", dto.Service)
		kpi, err := h.kpiRepo.GetKPI(ctx, dto.Service)
		if err != nil {
			h.logger.Debug("GetKPI for Service failed", "uuid", dto.Service, "error", err)
		} else if kpi != nil {
			h.logger.Info("KPI enrichment SUCCESS", "uuid", dto.Service, "name", kpi.Name, "formula", kpi.Formula)
			dto.KPIName = kpi.Name
			dto.KPIFormula = kpi.Formula
			return
		} else {
			h.logger.Debug("GetKPI returned nil KPI", "uuid", dto.Service)
		}
	}

	// If Service lookup didn't find a KPI, try Component
	if dto.KPIName == "" && dto.Component != "" {
		h.logger.Debug("Looking up Component as KPI", "uuid", dto.Component)
		kpi, err := h.kpiRepo.GetKPI(ctx, dto.Component)
		if err != nil {
			h.logger.Debug("GetKPI for Component failed", "uuid", dto.Component, "error", err)
		} else if kpi != nil {
			h.logger.Info("KPI enrichment SUCCESS (from Component)", "uuid", dto.Component, "name", kpi.Name, "formula", kpi.Formula)
			dto.KPIName = kpi.Name
			dto.KPIFormula = kpi.Formula
		} else {
			h.logger.Debug("GetKPI returned nil KPI for Component", "uuid", dto.Component)
		}
	}

	h.logger.Debug("enrichKPIMetadata completed", "kpiName", dto.KPIName, "kpiFormula", dto.KPIFormula)
}

func (h *RCAHandler) convertDiagnostics(d *rca.RCADiagnostics) *models.RCADiagnosticsDTO {
	if d == nil {
		return nil
	}
	return &models.RCADiagnosticsDTO{
		MissingLabels:            append([]string{}, d.MissingLabels...),
		DimensionDetectionStatus: copyStringBoolMap(d.DimensionDetectionStatus),
		IsolationForestIssues:    append([]string{}, d.IsolationForestIssues...),
		ReducedAccuracyReasons:   append([]string{}, d.ReducedAccuracyReasons...),
		MetricsQueryErrors:       append([]string{}, d.MetricsQueryErrors...),
	}
}

func copyStringBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// GetActiveCorrelations handles GET /rca/correlations
func (h *RCAHandler) GetActiveCorrelations(c *gin.Context) {
	// TODO(AT-012): Implement active correlation retrieval
	c.JSON(http.StatusNotImplemented, gin.H{"status": "error", "error": "not_implemented"})
}

// GetFailurePatterns handles GET /rca/patterns
func (h *RCAHandler) GetFailurePatterns(c *gin.Context) {
	// TODO(AT-012): Implement failure pattern retrieval
	c.JSON(http.StatusNotImplemented, gin.H{"status": "error", "error": "not_implemented"})
}

// StoreCorrelation handles POST /rca/store
func (h *RCAHandler) StoreCorrelation(c *gin.Context) {
	// TODO(AT-012): Implement correlation storage
	c.JSON(http.StatusNotImplemented, gin.H{"status": "error", "error": "not_implemented"})
}
