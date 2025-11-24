package bootstrap

import (
	"context"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/repo"
)

// BootstrapTelemetryStandards seeds platform-standard telemetry KPI/label
// definitions into the KPI registry (Weaviate via repo.KPIRepo). The function
// is idempotent: it uses the repo upsert semantics and will not duplicate
// entries across repeated runs.
func BootstrapTelemetryStandards(ctx context.Context, engCfg *config.EngineConfig, kpiRepo repo.KPIRepo, log logging.Logger) error {
	if kpiRepo == nil {
		log.Warn("kpiRepo is nil; skipping telemetry bootstrap")
		return nil
	}

	if engCfg == nil {
		log.Warn("engine config is nil; skipping telemetry bootstrap")
		return nil
	}

	// Delegate model creation/upsert to the KPI repo implementation so the
	// bootstrap package does not depend on internal models.
	if err := kpiRepo.EnsureTelemetryStandards(ctx, engCfg); err != nil {
		log.Error("failed to ensure telemetry standards in KPI repo", "error", err)
		return err
	}
	log.Info("telemetry standards ensured in KPI repo")
	return nil
}
