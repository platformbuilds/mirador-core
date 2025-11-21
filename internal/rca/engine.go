package rca

import (
	"context"
	"fmt"
	"sort"

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
	ComputeRCA(ctx context.Context, incident *IncidentContext, opts RCAOptions) (*RCAIncident, error)
}

// RCAEngineImpl implements the RCAEngine interface.
type RCAEngineImpl struct {
	candidateCauseService    CandidateCauseService
	serviceGraph             *ServiceGraph
	logger                   logger.Logger
	labelDetector            *MetricsLabelDetector
	dimensionAlignmentScorer *DimensionAlignmentScorer
}

// NewRCAEngine creates a new RCAEngine implementation.
func NewRCAEngine(
	candidateCauseService CandidateCauseService,
	serviceGraph *ServiceGraph,
	logger logger.Logger,
) RCAEngine {
	return &RCAEngineImpl{
		candidateCauseService:    candidateCauseService,
		serviceGraph:             serviceGraph,
		logger:                   logger,
		labelDetector:            NewMetricsLabelDetector(logger),
		dimensionAlignmentScorer: NewDimensionAlignmentScorer(logger),
	}
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
