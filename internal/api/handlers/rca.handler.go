package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
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

// GetServiceGraph handles POST /api/v1/unified/service-graph
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

	// Build and attach time ring metadata using default ring config.
	// Use the incident peak time (if available) and incident window to compute ranges.
	if inc.Impact != nil && !inc.Impact.TimeBounds.TPeak.IsZero() {
		ringCfg := rca.DefaultTimeRingConfig()

		defs := map[string]models.TimeRingDefinitionDTO{
			string(rca.RingImmediate):  {Label: string(rca.RingImmediate), Description: "Anomalies very close to the peak", Duration: ringCfg.R1Duration.String()},
			string(rca.RingShort):      {Label: string(rca.RingShort), Description: "Anomalies shortly before peak", Duration: ringCfg.R2Duration.String()},
			string(rca.RingMedium):     {Label: string(rca.RingMedium), Description: "Anomalies moderately before peak", Duration: ringCfg.R3Duration.String()},
			string(rca.RingLong):       {Label: string(rca.RingLong), Description: "Anomalies further back", Duration: ringCfg.R4Duration.String()},
			string(rca.RingOutOfScope): {Label: string(rca.RingOutOfScope), Description: "Events outside the configured analysis window or too far after the peak", Duration: "variable"},
		}

		perChain := make([]models.TimeRingsPerChainDTO, 0, len(inc.Chains))

		// Use incident window as the canonical window for per-chain calculations
		windowStart := inc.Impact.TimeBounds.TStart
		windowEnd := inc.Impact.TimeBounds.TEnd
		peak := inc.Impact.TimeBounds.TPeak

		for _, ch := range inc.Chains {
			// Compute raw ranges relative to peak.
			r1Start := peak.Add(-ringCfg.R1Duration)
			r1End := peak

			r2Start := peak.Add(-ringCfg.R2Duration)
			r2End := peak.Add(-ringCfg.R1Duration)

			r3Start := peak.Add(-ringCfg.R3Duration)
			r3End := peak.Add(-ringCfg.R2Duration)

			r4Start := peak.Add(-ringCfg.R4Duration)
			r4End := peak.Add(-ringCfg.R3Duration)

			// Clip to incident window bounds
			clip := func(t time.Time, startBound, endBound time.Time) time.Time {
				if t.Before(startBound) {
					return startBound
				}
				if t.After(endBound) {
					return endBound
				}
				return t
			}

			ringsMap := map[string]models.TimeRangeDTO{
				string(rca.RingImmediate): {StartTime: clip(r1Start, windowStart, windowEnd).Format(time.RFC3339), EndTime: clip(r1End, windowStart, windowEnd).Format(time.RFC3339)},
				string(rca.RingShort):     {StartTime: clip(r2Start, windowStart, windowEnd).Format(time.RFC3339), EndTime: clip(r2End, windowStart, windowEnd).Format(time.RFC3339)},
				string(rca.RingMedium):    {StartTime: clip(r3Start, windowStart, windowEnd).Format(time.RFC3339), EndTime: clip(r3End, windowStart, windowEnd).Format(time.RFC3339)},
				string(rca.RingLong):      {StartTime: clip(r4Start, windowStart, windowEnd).Format(time.RFC3339), EndTime: clip(r4End, windowStart, windowEnd).Format(time.RFC3339)},
			}

			rank := ch.Rank
			if rank <= 0 {
				rank = 1
			}

			perChain = append(perChain, models.TimeRingsPerChainDTO{
				ChainRank:   rank,
				PeakTime:    peak.Format(time.RFC3339),
				WindowStart: windowStart.Format(time.RFC3339),
				WindowEnd:   windowEnd.Format(time.RFC3339),
				Rings:       ringsMap,
			})
		}

		dto.TimeRings = &models.TimeRingsDTO{
			Definitions: defs,
			PerChain:    perChain,
		}
	}
	return dto
}

func (h *RCAHandler) convertIncidentContext(ic *rca.IncidentContext) *models.IncidentContextDTO {
	if ic == nil {
		return nil
	}
	dto := &models.IncidentContextDTO{
		ID:            ic.ID,
		ImpactService: ic.ImpactService,
		MetricName:    ic.ImpactSignal.MetricName,
		TimeStartStr:  ic.TimeBounds.TStart.Format(time.RFC3339),
		TimeEndStr:    ic.TimeBounds.TEnd.Format(time.RFC3339),
		ImpactSummary: ic.ImpactSummary,
		Severity:      ic.Severity,
	}

	// Resolve ImpactService UUID to name if it's a KPI
	h.resolveUUIDToName(&dto.ImpactService, &dto.ImpactServiceUUID)

	// Resolve MetricName UUID to name if it's a KPI
	h.resolveUUIDToName(&dto.MetricName, &dto.MetricNameUUID)

	// Resolve any KPI UUIDs found inside the free-form ImpactSummary text
	// (the engine sometimes includes KPI IDs inline inside narrative strings).
	// We attempt to find UUID-like tokens and replace them with KPI names when
	// available; this keeps the returned impact summary human-friendly.
	h.resolveUUIDsInText(&dto.ImpactSummary, "ImpactSummary")

	return dto
}

func (h *RCAHandler) convertRCAChain(ch *rca.RCAChain) *models.RCAChainDTO {
	if ch == nil {
		return nil
	}
	dto := &models.RCAChainDTO{
		Steps:        make([]*models.RCAStepDTO, 0, len(ch.Steps)),
		Score:        ch.Score,
		Rank:         ch.Rank,
		ImpactPath:   make([]string, len(ch.ImpactPath)),
		DurationHops: ch.DurationHops,
	}

	// Resolve UUIDs in ImpactPath to names
	for i, path := range ch.ImpactPath {
		resolvedPath := path
		var unusedUUID string
		h.resolveUUIDToName(&resolvedPath, &unusedUUID)
		dto.ImpactPath[i] = resolvedPath
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

	// Resolve inline UUID tokens that may appear inside the free-form Summary
	// (the RCA engine can include KPI IDs inside summaries). Make summaries
	// human-friendly by replacing tokens with KPI names when possible.
	h.resolveUUIDsInText(&dto.Summary, "Summary")

	return dto
}

// resolveUUIDToName attempts to resolve a UUID to a KPI name.
// If the value is a valid KPI UUID, it replaces *value with the KPI name
// and stores the original UUID in *uuidField.
// If not a KPI UUID, the value remains unchanged.
func (h *RCAHandler) resolveUUIDToName(value *string, uuidField *string) {
	if value == nil || *value == "" {
		return
	}
	if h.kpiRepo == nil {
		return
	}

	ctx := context.Background()
	// Add temporary logging to help diagnose runtime resolution issues.
	// Log that we're attempting resolution (debug) and any errors (info) so operators
	// can observe why a name wasn't returned in production environments.
	h.logger.Debug("Attempting to resolve UUID to KPI", "candidate", *value)
	kpi, err := h.kpiRepo.GetKPI(ctx, *value)
	if err != nil {
		// Lookup failed — keep original value but record the lookup failure.
		h.logger.Info("KPI lookup failed while attempting resolution", "candidate", *value, "error", err)
		return
	}
	if kpi == nil {
		// Not found in KPI repo - log for diagnostics
		h.logger.Info("KPI not found while attempting resolution", "candidate", *value)
		return
	}
	// (kpi is non-nil here) continue

	// Found a KPI - replace value with name and store UUID
	if uuidField != nil {
		*uuidField = *value
	}
	*value = kpi.Name
	// Log resolution success at info so it appears in container logs during testing.
	h.logger.Info("Resolved UUID to KPI name", "uuid", *uuidField, "name", kpi.Name)
}

// resolveUUIDsInText finds UUID-like tokens inside a freeform text and attempts
// to resolve them to KPI names. We replace occurrences in-place if resolution
// succeeds. This is used to make ImpactSummary (which can contain inline KPI
// IDs produced by the RCA engine) more human-friendly.
func (h *RCAHandler) resolveUUIDsInText(text *string, fieldName string) {
	if text == nil || *text == "" {
		return
	}
	if h.kpiRepo == nil {
		return
	}

	ctx := context.Background()
	// Match standard UUIDs optionally followed by a suffix (e.g., -dep)
	re := regexp.MustCompile(`([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}(?:[-][A-Za-z0-9_]+)?)`)
	matches := re.FindAllString(*text, -1)
	if len(matches) == 0 {
		return
	}

	// Use a set to avoid repeated work for duplicate tokens
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}

		h.logger.Debug("Attempting inline UUID resolution", "field", fieldName, "candidate", m)

		// Try resolving the full token first (handles object-id styles)
		kpi, err := h.kpiRepo.GetKPI(ctx, m)
		if err == nil && kpi != nil {
			*text = strings.ReplaceAll(*text, m, kpi.Name)
			h.logger.Info("Replaced inline KPI token", "field", fieldName, "candidate", m, "name", kpi.Name)
			continue
		}

		// If a suffix exists (e.g., "<uuid>-dep"), try the base UUID
		if len(m) > 36 && m[36] == '-' {
			base := m[:36]
			suffix := m[36:]
			kpi2, err2 := h.kpiRepo.GetKPI(ctx, base)
			if err2 == nil && kpi2 != nil {
				// preserve the suffix when replacing the token
				*text = strings.ReplaceAll(*text, m, kpi2.Name+suffix)
				h.logger.Info("Replaced inline KPI token (stripped suffix)", "field", fieldName, "candidate", m, "resolved_to", kpi2.Name+suffix)
				continue
			}
		}

		h.logger.Debug("Inline token not resolvable to KPI", "field", fieldName, "candidate", m)
	}
}

// enrichKPIMetadata looks up KPI information for Service and Component fields,
// replaces UUIDs with human-readable names, and populates metadata fields.
func (h *RCAHandler) enrichKPIMetadata(dto *models.RCAStepDTO) {
	if dto == nil {
		return
	}
	if h.kpiRepo == nil {
		return
	}

	ctx := context.Background()
	var resolvedService bool

	// --- Service resolution ---
	if dto.Service != "" {
		// 1) direct lookup
		if k, err := h.kpiRepo.GetKPI(ctx, dto.Service); err == nil && k != nil {
			h.logger.Info("Resolved Service UUID to KPI", "uuid", dto.Service, "name", k.Name)
			dto.ServiceUUID = dto.Service
			dto.Service = k.Name
			dto.KPIName = k.Name
			dto.KPIFormula = k.Formula
			resolvedService = true
		} else {
			// 2) suffix-stripping lookup (e.g. "<uuid>-dep")
			if len(dto.Service) > 36 && dto.Service[36] == '-' {
				base := dto.Service[:36]
				suffix := dto.Service[36:]
				if k2, err2 := h.kpiRepo.GetKPI(ctx, base); err2 == nil && k2 != nil {
					h.logger.Info("Resolved Service UUID (with suffix) to KPI", "original", dto.Service, "base_uuid", base, "name", k2.Name+suffix)
					dto.ServiceUUID = base
					dto.Service = k2.Name + suffix
					dto.KPIName = k2.Name
					dto.KPIFormula = k2.Formula
					resolvedService = true
				}
			}
		}

		// 3) inline token fallback (e.g. wrapped tokens in freeform text)
		if !resolvedService {
			originalSvc := dto.Service
			h.resolveUUIDsInText(&dto.Service, "Service")
			if dto.Service != originalSvc {
				re := regexp.MustCompile(`([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)
				if m := re.FindString(originalSvc); m != "" {
					dto.ServiceUUID = m
					resolvedService = true
				}
			}
		}
	}

	// --- Component resolution (don't override a component equal to the resolved service id) ---
	if dto.Component != "" && dto.Component != dto.ServiceUUID {
		// direct lookup
		if k, err := h.kpiRepo.GetKPI(ctx, dto.Component); err == nil && k != nil {
			h.logger.Info("Resolved Component UUID to KPI", "uuid", dto.Component, "name", k.Name)
			dto.ComponentUUID = dto.Component
			dto.Component = k.Name
			if !resolvedService {
				dto.KPIName = k.Name
				dto.KPIFormula = k.Formula
			}
		} else if len(dto.Component) > 36 && dto.Component[36] == '-' {
			// suffix-stripping
			base := dto.Component[:36]
			suffix := dto.Component[36:]
			if k2, err2 := h.kpiRepo.GetKPI(ctx, base); err2 == nil && k2 != nil {
				h.logger.Info("Resolved Component UUID (with suffix) to KPI", "original", dto.Component, "base_uuid", base, "name", k2.Name+suffix)
				dto.ComponentUUID = base
				dto.Component = k2.Name + suffix
				if !resolvedService {
					dto.KPIName = k2.Name
					dto.KPIFormula = k2.Formula
				}
			}
		} else {
			// inline fallback
			original := dto.Component
			h.resolveUUIDsInText(&dto.Component, "Component")
			if dto.Component != original {
				re := regexp.MustCompile(`([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)
				if m := re.FindString(original); m != "" {
					dto.ComponentUUID = m
				}
			}
		}
	}
	// (cleanup removed duplicate old logic)
	if dto.Component != "" {
		// Try direct lookup first
		kpi, err := h.kpiRepo.GetKPI(ctx, dto.Component)
		if err == nil && kpi != nil {
			h.logger.Info("Resolved Component UUID to KPI", "uuid", dto.Component, "name", kpi.Name)
			dto.ComponentUUID = dto.Component
			dto.Component = kpi.Name
			dto.KPIName = kpi.Name
			dto.KPIFormula = kpi.Formula
		} else {
			// Try suffix-stripping for cases like "<uuid>-dep"
			if len(dto.Component) > 36 && dto.Component[36] == '-' {
				base := dto.Component[:36]
				suffix := dto.Component[36:]
				kpi2, err2 := h.kpiRepo.GetKPI(ctx, base)
				if err2 == nil && kpi2 != nil {
					h.logger.Info("Resolved Component UUID (with suffix) to KPI", "original", dto.Component, "base_uuid", base, "name", kpi2.Name+suffix)
					dto.ComponentUUID = base
					dto.Component = kpi2.Name + suffix
					dto.KPIName = kpi2.Name
					dto.KPIFormula = kpi2.Formula
				}
			}
		}
	}

	// Final fallback: if we still haven't resolved the component via direct
	// lookup, attempt inline token resolution (covers cases like "kpi:<uuid>"
	// or other wrappers). If resolveUUIDsInText changed the value, populate
	// ComponentUUID with the base UUID found in the original token.
	if dto.Component != "" && dto.ComponentUUID == "" {
		original := dto.Component
		// Attempt inline replacement (this will replace any embedded uuid tokens)
		h.resolveUUIDsInText(&dto.Component, "Component")
		if dto.Component != original {
			// find the first UUID token in the original text so we can store the base id
			re := regexp.MustCompile(`([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)
			if m := re.FindString(original); m != "" {
				dto.ComponentUUID = m
			}
		}
	}
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
