package rca

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// RCAOptions configures RCA computation behavior.
type RCAOptions struct {
	// CandidateCauseOptions for computing ranked candidate causes
	CandidateCauseOptions CandidateCauseOptions

	// DimensionConfig specifies extra dimensions and weights for correlation
	DimensionConfig RCADimensionConfig

	// MaxChains is the maximum number of candidate chains to produce (default 5)
	MaxChains int

	// MaxStepsPerChain is the maximum depth of each chain (default 10)
	MaxStepsPerChain int

	// MinScoreThreshold is the minimum score to include a step in final chains (default 0.1)
	MinScoreThreshold float64
}

// DefaultRCAOptions returns sensible defaults.
func DefaultRCAOptions() RCAOptions {
	return RCAOptions{
		CandidateCauseOptions: DefaultCandidateCauseOptions(),
		DimensionConfig:       DefaultDimensionConfig(),
		MaxChains:             5,
		MaxStepsPerChain:      10,
		MinScoreThreshold:     0.1,
	}
}

// RCAEngine computes root cause analysis for incidents.
type RCAEngine interface {
	// ComputeRCA analyzes an incident and produces one or more RCA chains with root cause candidates.
	// Legacy incident-based API (kept as adapter)
	ComputeRCA(ctx context.Context, incident *IncidentContext, opts RCAOptions) (*RCAIncident, error)
	// TimeRange-based canonical API (Stage-01)
	ComputeRCAByTimeRange(ctx context.Context, tr TimeRange) (*RCAIncident, error)
}

// RCAEngineImpl implements the RCAEngine interface.
type RCAEngineImpl struct {
	candidateCauseService    CandidateCauseService
	serviceGraph             *ServiceGraph
	logger                   logger.Logger
	labelDetector            *MetricsLabelDetector
	dimensionAlignmentScorer *DimensionAlignmentScorer
	engineCfg                config.EngineConfig
	// correlation engine dependency used by the new TimeRange-based API
	// Use a minimal local interface to avoid import cycles with services.
	corrEngine interface {
		Correlate(ctx context.Context, tr models.TimeRange) (*models.CorrelationResult, error)
	}
}

// NewRCAEngine creates a new RCAEngine implementation.
func NewRCAEngine(
	candidateCauseService CandidateCauseService,
	serviceGraph *ServiceGraph,
	logger logger.Logger,
	cfg config.EngineConfig,
	corrEngine interface {
		Correlate(ctx context.Context, tr models.TimeRange) (*models.CorrelationResult, error)
	},
) *RCAEngineImpl {
	// Return concrete implementation pointer so callers can pass this to
	// interfaces defined in other packages (avoids interface-to-interface
	// conversion issues).
	return &RCAEngineImpl{
		candidateCauseService:    candidateCauseService,
		serviceGraph:             serviceGraph,
		logger:                   logger,
		labelDetector:            NewMetricsLabelDetector(logger, cfg.Labels),
		dimensionAlignmentScorer: NewDimensionAlignmentScorer(logger),
		engineCfg:                cfg,
		corrEngine:               corrEngine,
	}
}

// ComputeRCAByTimeRange implements the new canonical TimeRange-based RCA API.
// It calls the provided CorrelationEngine and deterministically builds a
// template-based 5-Whys chain seeded from the correlation result.
func (engine *RCAEngineImpl) ComputeRCAByTimeRange(ctx context.Context, tr TimeRange) (*RCAIncident, error) {
	// If no correlation engine provided, return error
	if engine.corrEngine == nil {
		return nil, fmt.Errorf("correlation engine not configured for RCAEngine")
	}

	// Convert to models.TimeRange and call correlation
	mtr := models.TimeRange{Start: tr.Start, End: tr.End}
	corr, err := engine.corrEngine.Correlate(ctx, mtr)
	if err != nil {
		return nil, fmt.Errorf("correlation failed: %w", err)
	}

	// If correlation returned nil or no meaningful data, return low-confidence RCA
	if corr == nil || (len(corr.AffectedServices) == 0 && len(corr.RedAnchors) == 0) {
		// Build minimal incident with low confidence
		incident := &IncidentContext{
			ID:            "incident_unknown",
			ImpactService: "unknown",
			ImpactSignal: ImpactSignal{
				ServiceName: "unknown",
				MetricName:  "unknown",
				Direction:   "higher_is_worse",
			},
			TimeBounds:    IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End},
			ImpactSummary: fmt.Sprintf("No correlation data for window %s - %s", tr.Start.String(), tr.End.String()),
			Severity: func() float64 {
				if corr == nil {
					return 0.0
				}
				return corr.Confidence
			}(),
			CreatedAt: time.Now().UTC(),
		}
		res := NewRCAIncident(incident)
		res.Notes = append(res.Notes, "Correlation produced no candidates; returning low-confidence RCA")
		return res, nil
	}

	// Choose focal impact:
	// Prefer explicit affected services from correlation. If absent, prefer
	// the top-ranked Cause candidate by suspicion score (added in AT-007).
	// Fallback to the first red anchor service if no causes present.
	focal := "unknown"
	if len(corr.AffectedServices) > 0 {
		focal = corr.AffectedServices[0]
	} else if len(corr.Causes) > 0 {
		// pick highest suspicion score deterministically
		bestIdx := 0
		bestScore := corr.Causes[0].SuspicionScore
		for i := 1; i < len(corr.Causes); i++ {
			if corr.Causes[i].SuspicionScore > bestScore {
				bestScore = corr.Causes[i].SuspicionScore
				bestIdx = i
			}
		}
		if corr.Causes[bestIdx].Service != "" {
			focal = corr.Causes[bestIdx].Service
		} else {
			focal = corr.Causes[bestIdx].KPI
		}
	} else if len(corr.RedAnchors) > 0 {
		focal = corr.RedAnchors[0].Service
	}

	// Build IncidentContext from correlation summary
	incident := &IncidentContext{
		ID:            corr.CorrelationID,
		ImpactService: focal,
		ImpactSignal: ImpactSignal{
			ServiceName: focal,
			MetricName: func() string {
				if len(corr.RedAnchors) > 0 {
					return corr.RedAnchors[0].Metric
				}
				return "unknown"
			}(),
			Direction: "higher_is_worse",
		},
		TimeBounds:    IncidentTimeWindow{TStart: tr.Start, TPeak: tr.End, TEnd: tr.End},
		ImpactSummary: fmt.Sprintf("Impact detected on %s (correlation confidence %.2f)", focal, corr.Confidence),
		Severity:      corr.Confidence,
		CreatedAt:     corr.CreatedAt,
	}

	// If correlation provided candidate causes with stats, attach a concise
	// template-based evidence note referencing the top candidate's stats.
	if len(corr.Causes) > 0 {
		bestIdx := 0
		bestScore := corr.Causes[0].SuspicionScore
		for i := 1; i < len(corr.Causes); i++ {
			if corr.Causes[i].SuspicionScore > bestScore {
				bestScore = corr.Causes[i].SuspicionScore
				bestIdx = i
			}
		}
		if corr.Causes[bestIdx].Stats != nil {
			s := corr.Causes[bestIdx].Stats
			// NOTE(AT-012): include partial correlation and anomaly hint in Stage-01
			anomalyHint := "LOW"
			for _, rt := range corr.Causes[bestIdx].Reasons {
				if rt == "high_anomaly_density" {
					anomalyHint = "HIGH"
					break
				}
			}
			incident.ImpactSummary = fmt.Sprintf("%s. Top-candidate %s: pearson=%.2f spearman=%.2f partial=%.2f cross_max=%.2f lag=%d anomalies=%s", incident.ImpactSummary, corr.Causes[bestIdx].KPI, s.Pearson, s.Spearman, s.Partial, s.CrossCorrMax, s.CrossCorrLag, anomalyHint)
		}
	}

	result := NewRCAIncident(incident)

	// Build candidate chains from top red anchors (deterministic template)
	maxChains := 3
	for i, anchor := range corr.RedAnchors {
		if i >= maxChains {
			break
		}
		chain := NewRCAChain()

		// Step 1: Business impact (Why 1)
		s1 := NewRCAStep(1, incident.ImpactService, incident.ImpactSignal.MetricName)
		s1.TimeRange = tr
		s1.Ring = RingImmediate
		s1.Direction = DirectionSame
		s1.Score = corr.Confidence
		s1.AddEvidence("red_anchor", anchor.Service, fmt.Sprintf("anchor_score=%.3f", anchor.Score))
		s1.Summary = TemplateBasedSummary(s1, incident.ImpactService, result.Diagnostics)
		chain.AddStep(s1)

		// Step 2: Entry service degradation (anchor service)
		s2 := NewRCAStep(2, anchor.Service, anchor.Metric)
		s2.TimeRange = tr
		s2.Ring = RingImmediate
		s2.Direction = DirectionUpstream
		s2.Score = anchor.Score
		s2.AddEvidence("red_anchor", anchor.Service, fmt.Sprintf("metric=%s score=%.3f", anchor.Metric, anchor.Score))
		s2.Summary = TemplateBasedSummary(s2, incident.ImpactService)
		chain.AddStep(s2)

		// Steps 3-5: simple template upstream placeholders
		s3 := NewRCAStep(3, fmt.Sprintf("%s-dep", anchor.Service), "dependency")
		s3.TimeRange = tr
		s3.Ring = RingShort
		s3.Direction = DirectionUpstream
		s3.Score = anchor.Score * 0.7
		s3.AddEvidence("inferred", s3.Service, "inferred from service graph and anchor")
		s3.Summary = TemplateBasedSummary(s3, incident.ImpactService)
		chain.AddStep(s3)

		s4 := NewRCAStep(4, "infrastructure", "database")
		s4.TimeRange = tr
		s4.Ring = RingShort
		s4.Direction = DirectionUpstream
		s4.Score = anchor.Score * 0.5
		s4.AddEvidence("inferred", "infra_db", "inferred infra contention")
		s4.Summary = TemplateBasedSummary(s4, incident.ImpactService)
		chain.AddStep(s4)

		s5 := NewRCAStep(5, "process", "deployment")
		s5.TimeRange = tr
		s5.Ring = RingShort
		s5.Direction = DirectionUpstream
		s5.Score = anchor.Score * 0.3
		s5.AddEvidence("inferred", "process_gap", "missing guardrail or rollout check")
		s5.Summary = TemplateBasedSummary(s5, incident.ImpactService)
		chain.AddStep(s5)

		chain.Score = (s1.Score + s2.Score + s3.Score + s4.Score + s5.Score) / 5.0
		result.AddChain(chain)
	}

	// Finalize: sort chains and set root cause
	sort.Slice(result.Chains, func(i, j int) bool {
		return result.Chains[i].Score > result.Chains[j].Score
	})
	for i := range result.Chains {
		result.Chains[i].Rank = i + 1
	}
	result.SetRootCauseFromBestChain()

	// Attach correlation recommendations and notes
	for _, r := range corr.Recommendations {
		result.Notes = append(result.Notes, r)
	}

	return result, nil
}

// ComputeRCA implements RCAEngine.ComputeRCA.
func (engine *RCAEngineImpl) ComputeRCA(
	ctx context.Context,
	incident *IncidentContext,
	opts RCAOptions,
) (*RCAIncident, error) {
	if err := incident.Validate(); err != nil {
		return nil, fmt.Errorf("invalid incident context: %w", err)
	}

	engine.logger.Debug("Starting RCA computation",
		"incident_id", incident.ID,
		"impact_service", incident.ImpactService,
		"dimension_config", opts.DimensionConfig.String())

	// Create result object
	result := NewRCAIncident(incident)

	// Validate and normalize dimension config
	dimWarnings, err := opts.DimensionConfig.ValidateAndNormalize()
	if err != nil {
		engine.logger.Warn("Dimension config validation error",
			"error", err)
		// Continue with degraded functionality
		result.Diagnostics.AddReducedAccuracyReason(fmt.Sprintf("Dimension config validation: %v", err))
	}
	for _, w := range dimWarnings {
		engine.logger.Warn("Dimension config warning", "warning", w)
		result.Diagnostics.AddReducedAccuracyReason(w)
	}

	// Step 1: Compute ranked candidate causes
	candidates, err := engine.candidateCauseService.ComputeCandidates(ctx, incident, opts.CandidateCauseOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to compute candidate causes: %w", err)
	}

	if len(candidates) == 0 {
		engine.logger.Warn("No candidate causes found for incident", "incident_id", incident.ID)
		result.Notes = append(result.Notes, "No anomalies found to explain the incident")
		result.Notes = append(result.Notes, result.Diagnostics.ToNotes()...)
		return result, nil
	}

	engine.logger.Debug("Computed candidate causes",
		"count", len(candidates),
		"incident_id", incident.ID)

	// Step 1b: Apply dimension alignment scoring if extra dimensions are configured
	if len(opts.DimensionConfig.ExtraDimensions) > 0 {
		engine.logger.Debug("Applying dimension alignment scoring",
			"dimensions", opts.DimensionConfig.ExtraDimensions)

		// Extract impact service dimensions from impact service groups (if available)
		impactServiceDims := engine.dimensionAlignmentScorer.ExtractImpactServiceDimensions(
			engine.filterGroupsByService(candidates, incident.ImpactService),
			opts.DimensionConfig.ExtraDimensions)

		// Apply alignment score adjustments to each candidate
		for _, candidate := range candidates {
			alignmentScore, alignments, alignNotes := engine.dimensionAlignmentScorer.ComputeDimensionAlignmentScore(
				candidate.Group,
				impactServiceDims,
				opts.DimensionConfig,
				result.Diagnostics)

			if candidate.DetailedScore == nil {
				candidate.DetailedScore = &DetailedScore{}
			}
			candidate.DetailedScore.DimensionAlignmentScore = alignmentScore
			candidate.DetailedScore.DimensionAlignments = alignments

			// Adjust overall score by alignment contribution (with small weight)
			dimensionAlignmentWeight := 0.05 // 5% weight for dimension alignment
			candidate.Score = candidate.Score*(1.0-dimensionAlignmentWeight) + alignmentScore*dimensionAlignmentWeight

			// Clamp to [0, 1]
			if candidate.Score > 1.0 {
				candidate.Score = 1.0
			}
			if candidate.Score < 0.0 {
				candidate.Score = 0.0
			}

			for _, note := range alignNotes {
				engine.logger.Debug("Dimension alignment note", "note", note)
			}
		}

		// Re-sort candidates after dimension adjustment
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
		for i := range candidates {
			candidates[i].Rank = i + 1
		}

		// Apply KPI sentiment bias if the incident contains KPI metadata
		if incident.KPIMetadata != nil && incident.KPIMetadata.ImpactIsKPI {
			// Default bias magnitude (matches tests): 0.05
			ApplyKPISentimentBias(candidates, incident, 0.05, result.Diagnostics)
		}
	}

	// Step 2: For each top candidate, build an RCA chain
	chainsBuilt := 0
	for _, candidate := range candidates {
		if chainsBuilt >= opts.MaxChains {
			break
		}

		chain, err := engine.buildChainForCandidate(ctx, incident, candidate, opts)
		if err != nil {
			engine.logger.Warn("Failed to build chain for candidate",
				"candidate_service", candidate.Group.Service,
				"error", err)
			continue
		}

		if chain != nil && len(chain.Steps) > 0 {
			result.AddChain(chain)
			chainsBuilt++
		}
	}

	// Step 3: Sort chains by score
	sort.Slice(result.Chains, func(i, j int) bool {
		return result.Chains[i].Score > result.Chains[j].Score
	})
	// Re-rank after sorting
	for i := range result.Chains {
		result.Chains[i].Rank = i + 1
	}

	// Step 4: Set root cause from best chain
	result.SetRootCauseFromBestChain()

	// Step 5: Append diagnostics to notes
	result.Notes = append(result.Notes, result.Diagnostics.ToNotes()...)

	engine.logger.Info("RCA computation completed",
		"incident_id", incident.ID,
		"chains_produced", len(result.Chains),
		"root_cause_service", func() string {
			if result.RootCause != nil {
				return result.RootCause.Service
			}
			return "unknown"
		}(),
		"overall_score", result.Score)

	return result, nil
}

// filterGroupsByService returns all AnomalyGroups within candidates that belong to a specific service.
func (engine *RCAEngineImpl) filterGroupsByService(candidates []*CandidateCause, service string) []*AnomalyGroup {
	var groups []*AnomalyGroup
	for _, candidate := range candidates {
		if candidate.Group.Service == service {
			groups = append(groups, candidate.Group)
		}
	}
	return groups
}

// buildChainForCandidate constructs an RCA chain starting from a candidate cause.
// The chain traces backwards through service dependencies to find upstream components.
func (engine *RCAEngineImpl) buildChainForCandidate(
	ctx context.Context,
	incident *IncidentContext,
	candidate *CandidateCause,
	opts RCAOptions,
) (*RCAChain, error) {
	chain := NewRCAChain()

	impactService := ServiceNode(incident.ImpactService)
	candidateService := ServiceNode(candidate.Group.Service)

	// Build initial step from the candidate cause
	step := engine.buildStepFromCandidate(1, candidate, incident, nil) // Pass nil for diagnostics in intermediate steps
	chain.AddStep(step)
	chain.Score = candidate.Score

	// If candidate is not the impact service, trace upstream path
	if candidateService != impactService {
		// Find path from candidate to impact
		path, found := engine.serviceGraph.ShortestPath(candidateService, impactService)
		if !found {
			// Candidate is isolated or path not in graph; still include it
			engine.logger.Debug("No path found in graph",
				"from", candidateService,
				"to", impactService)
			return chain, nil
		}

		// Build intermediate steps along the path
		// path is [candidateService, ..., impactService]
		// We'll add steps for intermediate services, then the impact service
		whyIndex := 2
		for i := 1; i < len(path) && whyIndex <= opts.MaxStepsPerChain; i++ {
			svc := path[i]
			// For each service in the path, try to find an anomaly group
			// In a real scenario, we'd query the anomaly groups again;
			// here we estimate based on available graph info.
			step := engine.buildIntermediateStepFromGraph(whyIndex, ServiceNode(incident.ImpactService), svc, path, incident)
			if step != nil {
				chain.AddStep(step)
				whyIndex++
			}
		}
	}

	// Finalize chain score
	engine.finalizeChainScore(chain, opts)

	return chain, nil
}

// buildStepFromCandidate creates an RCAStep from a CandidateCause.
func (engine *RCAEngineImpl) buildStepFromCandidate(
	whyIndex int,
	candidate *CandidateCause,
	incident *IncidentContext,
	diagnostics *RCADiagnostics,
) *RCAStep {
	step := NewRCAStep(whyIndex, candidate.Group.Service, candidate.Group.Component)
	step.Ring = candidate.Group.Ring
	step.Direction = candidate.Group.GraphDirection
	step.Distance = candidate.Group.MinGraphDistance
	step.TimeRange = candidate.Group.TimeRange
	step.Score = candidate.Score

	// Populate evidence from underlying anomaly events in the group
	for i, event := range candidate.Group.Events {
		if i >= 5 {
			break // Limit evidence references
		}
		evID := fmt.Sprintf("event_%d_%s", i, event.AnomalyEvent.ID)
		step.AddEvidence("anomaly_group", evID, fmt.Sprintf(
			"Severity=%.2f, AnomalyScore=%.4f",
			event.AnomalyEvent.Severity,
			event.AnomalyEvent.AnomalyScore,
		))
	}

	// Generate template-based summary, optionally including diagnostics note
	step.Summary = TemplateBasedSummary(step, incident.ImpactService, diagnostics)

	return step
}

// buildIntermediateStepFromGraph creates an RCAStep for an intermediate service on the path.
// This is a simplified version; in production, you'd query anomaly groups for this service.
func (engine *RCAEngineImpl) buildIntermediateStepFromGraph(
	whyIndex int,
	impactService ServiceNode,
	currentService ServiceNode,
	path []ServiceNode,
	incident *IncidentContext,
) *RCAStep {
	step := NewRCAStep(whyIndex, string(currentService), "network")
	step.Direction = DirectionUpstream

	// Compute distance: position in path relative to impact
	for i, s := range path {
		if s == impactService {
			step.Distance = i
			break
		}
	}

	// Use incident time window as proxy
	step.TimeRange = TimeRange{
		Start: incident.TimeBounds.TStart,
		End:   incident.TimeBounds.TEnd,
	}

	// Estimate ring: services further upstream in longer chains are often "short" ring
	step.Ring = RingShort

	// Get edge info if available
	if i := len(path) - 1; i > 0 {
		if edge, ok := engine.serviceGraph.GetEdge(path[i-1], path[i]); ok {
			errorRate := edge.ErrorRate
			// Infer score from error rate
			step.Score = errorRate
			step.AddEvidence("service_graph_edge",
				fmt.Sprintf("%s->%s", edge.Source, edge.Target),
				fmt.Sprintf("Error rate: %.2f%%", errorRate*100))
		}
	}

	// Generate summary
	step.Summary = TemplateBasedSummary(step, incident.ImpactService)

	return step
}

// finalizeChainScore computes the aggregate score for a chain.
// Simple approach: average of step scores, potentially discounted by chain length.
func (engine *RCAEngineImpl) finalizeChainScore(chain *RCAChain, opts RCAOptions) {
	if len(chain.Steps) == 0 {
		chain.Score = 0.0
		return
	}

	totalScore := 0.0
	for _, step := range chain.Steps {
		totalScore += step.Score
	}
	avgScore := totalScore / float64(len(chain.Steps))

	// Apply length discount: longer chains are less confident
	lengthDiscount := 1.0 - (float64(len(chain.Steps))/float64(opts.MaxStepsPerChain))*0.2
	if lengthDiscount < 0.5 {
		lengthDiscount = 0.5
	}

	chain.Score = avgScore * lengthDiscount
}
