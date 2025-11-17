package rca

import (
	"context"
	"fmt"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// CandidateCauseOptions configures the candidate cause computation.
type CandidateCauseOptions struct {
	// CollectOptions for the IncidentAnomalyCollector.
	CollectOptions CollectOptions

	// GroupingConfig for partitioning anomalies into groups.
	GroupingConfig GroupingConfig

	// ScoringConfig for ranking candidate causes.
	ScoringConfig ScoringConfig
}

// DefaultCandidateCauseOptions returns sensible defaults for candidate computation.
func DefaultCandidateCauseOptions() CandidateCauseOptions {
	return CandidateCauseOptions{
		CollectOptions: DefaultCollectOptions(),
		GroupingConfig: DefaultGroupingConfig(),
		ScoringConfig:  DefaultScoringConfig(),
	}
}

// CandidateCauseService computes ranked candidate root causes for an incident.
// It integrates:
// - IncidentAnomalyCollector (gathers enriched anomalies)
// - GroupEnrichedAnomalies (partitions into groups)
// - RankCandidateCauses (scores and ranks)
type CandidateCauseService interface {
	// ComputeCandidates computes and returns ranked candidate causes for an incident.
	// Returns a slice of CandidateCause sorted by rank (rank 1 = most suspicious).
	ComputeCandidates(
		ctx context.Context,
		incident *IncidentContext,
		opts CandidateCauseOptions,
	) ([]*CandidateCause, error)
}

// CandidateCauseServiceImpl implements the CandidateCauseService interface.
type CandidateCauseServiceImpl struct {
	anomalyCollector *IncidentAnomalyCollector
	logger           logger.Logger
}

// NewCandidateCauseService creates a new CandidateCauseService.
func NewCandidateCauseService(
	collector *IncidentAnomalyCollector,
	logger logger.Logger,
) CandidateCauseService {
	return &CandidateCauseServiceImpl{
		anomalyCollector: collector,
		logger:           logger,
	}
}

// ComputeCandidates implements CandidateCauseService.ComputeCandidates.
func (svc *CandidateCauseServiceImpl) ComputeCandidates(
	ctx context.Context,
	incident *IncidentContext,
	opts CandidateCauseOptions,
) ([]*CandidateCause, error) {
	if err := incident.Validate(); err != nil {
		return nil, fmt.Errorf("invalid incident context: %w", err)
	}

	svc.logger.Debug("Computing candidate causes for incident",
		"incident_id", incident.ID,
		"impact_service", incident.ImpactService)

	// Step 1: Collect enriched anomalies from IncidentAnomalyCollector
	enrichedAnomalies, err := svc.anomalyCollector.Collect(ctx, incident, opts.CollectOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to collect anomalies: %w", err)
	}

	svc.logger.Debug("Collected enriched anomalies",
		"count", len(enrichedAnomalies),
		"incident_id", incident.ID)

	if len(enrichedAnomalies) == 0 {
		svc.logger.Info("No anomalies found for incident",
			"incident_id", incident.ID)
		return []*CandidateCause{}, nil
	}

	// Step 2: Group anomalies by service/component/ring/time bucket
	anomalyGroups := GroupEnrichedAnomalies(enrichedAnomalies, opts.GroupingConfig)

	svc.logger.Debug("Grouped anomalies into candidate groups",
		"group_count", len(anomalyGroups),
		"incident_id", incident.ID)

	if len(anomalyGroups) == 0 {
		svc.logger.Info("No anomaly groups after filtering",
			"incident_id", incident.ID)
		return []*CandidateCause{}, nil
	}

	// Step 3: Score and rank candidates
	rankedCandidates := RankCandidateCauses(anomalyGroups, incident, opts.ScoringConfig)

	svc.logger.Debug("Ranked candidate causes",
		"candidate_count", len(rankedCandidates),
		"incident_id", incident.ID)

	if len(rankedCandidates) > 0 {
		svc.logger.Info("Top candidate cause",
			"rank", rankedCandidates[0].Rank,
			"service", rankedCandidates[0].Group.Service,
			"score", rankedCandidates[0].Score,
			"incident_id", incident.ID)
	}

	return rankedCandidates, nil
}
