package services

import (
	"context"

	"github.com/platformbuilds/mirador-core/internal/rca"
)

// RCAEngine is the interface for root cause analysis computation.
// It's defined here to avoid circular imports (rca package imports from services).
type RCAEngine interface {
	// ComputeRCA analyzes an incident and produces RCA chains with root cause candidates.
	ComputeRCA(ctx context.Context, incident *rca.IncidentContext, opts rca.RCAOptions) (*rca.RCAIncident, error)
}
