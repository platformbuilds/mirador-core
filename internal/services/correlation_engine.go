package services

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/tracing"
	"github.com/platformbuilds/mirador-core/pkg/cache"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// containsString helper for slice membership checks
func containsString(hay []string, needle string) bool {
	for _, v := range hay {
		if v == needle {
			return true
		}
	}
	return false
}

// CorrelationEngine handles correlation queries across multiple engines
type CorrelationEngine interface {
	// ExecuteCorrelation executes a correlation query
	ExecuteCorrelation(ctx context.Context, query *models.CorrelationQuery) (*models.UnifiedCorrelationResult, error)

	// Correlate runs correlation using a canonical TimeRange (Stage-01 API)
	Correlate(ctx context.Context, tr models.TimeRange) (*models.CorrelationResult, error)
	// ValidateCorrelationQuery validates a correlation query
	ValidateCorrelationQuery(query *models.CorrelationQuery) error

	// GetCorrelationExamples returns example correlation queries
	GetCorrelationExamples() []string

	// DetectComponentFailures detects component failures in the financial transaction system
	DetectComponentFailures(ctx context.Context, timeRange models.TimeRange, components []models.FailureComponent) (*models.FailureCorrelationResult, error)

	// CorrelateTransactionFailures correlates failures for specific transaction IDs
	CorrelateTransactionFailures(ctx context.Context, transactionIDs []string, timeRange models.TimeRange) (*models.FailureCorrelationResult, error)
}

// MetricsService interface for metrics operations
type MetricsService interface {
	ExecuteQuery(ctx context.Context, req *models.MetricsQLQueryRequest) (*models.MetricsQLQueryResult, error)
}

// LogsService interface for logs operations
type LogsService interface {
	ExecuteQuery(ctx context.Context, req *models.LogsQLQueryRequest) (*models.LogsQLQueryResult, error)
}

// TracesService interface for traces operations
type TracesService interface {
	GetOperations(ctx context.Context, service string) ([]string, error)
	SearchTraces(ctx context.Context, request *models.TraceSearchRequest) (*models.TraceSearchResult, error)
}

// CorrelationEngineImpl implements the CorrelationEngine interface
type CorrelationEngineImpl struct {
	metricsService MetricsService
	logsService    LogsService
	tracesService  TracesService
	kpiRepo        repo.KPIRepo
	cache          cache.ValkeyCluster
	logger         logging.Logger
	parser         *models.CorrelationQueryParser
	resultMerger   *CorrelationResultMerger
	tracer         *tracing.QueryTracer
	engineCfg      config.EngineConfig
}

// NewCorrelationEngine creates a new correlation engine
func NewCorrelationEngine(
	metricsSvc MetricsService,
	logsSvc LogsService,
	tracesSvc TracesService,
	kpiRepo repo.KPIRepo,
	cache cache.ValkeyCluster,
	logger corelogger.Logger,
	cfg config.EngineConfig,
) CorrelationEngine {
	// Ensure engine configuration is merged with package-level defaults so
	// the engine implementation never hardcodes raw label/metric keys.
	// Defaults are centralised in the `config` package (configs/defaults).
	cfg = config.MergeEngineConfigWithDefaults(cfg)
	return &CorrelationEngineImpl{
		metricsService: metricsSvc,
		logsService:    logsSvc,
		tracesService:  tracesSvc,
		kpiRepo:        kpiRepo,
		cache:          cache,
		logger:         logging.FromCoreLogger(logger),
		parser:         models.NewCorrelationQueryParser(),
		resultMerger:   NewCorrelationResultMerger(logging.FromCoreLogger(logger)),
		tracer:         tracing.GetGlobalTracer(),
		engineCfg:      cfg,
	}
}

// ExecuteCorrelation executes a correlation query across multiple engines
func (ce *CorrelationEngineImpl) ExecuteCorrelation(ctx context.Context, query *models.CorrelationQuery) (*models.UnifiedCorrelationResult, error) {
	start := time.Now()

	// Start distributed tracing span for correlation
	corrCtx, corrSpan := ce.startCorrelationSpan(ctx, query)
	defer corrSpan.End()

	// Validate the query
	if err := ce.ValidateCorrelationQuery(query); err != nil {
		if ce.tracer != nil {
			ce.tracer.RecordError(corrSpan, err)
		}
		return nil, fmt.Errorf("invalid correlation query: %w", err)
	}

	if ce.logger != nil {
		ce.logger.Info("Executing correlation query",
			"query_id", query.ID,
			"raw_query", query.RawQuery,
			"expressions", len(query.Expressions))
	}

	// Execute expressions in parallel
	results, err := ce.executeExpressionsParallel(corrCtx, query)
	if err != nil {
		// Record failed correlation metrics
		monitoring.RecordUnifiedQueryCorrelationOperation("correlation", len(query.Expressions), time.Since(start), false)
		ce.tracer.RecordError(corrSpan, err)
		return nil, fmt.Errorf("failed to execute expressions: %w", err)
	}

	// Correlate results
	correlations := ce.correlateResults(query, results)

	// Merge and deduplicate correlations
	correlations = ce.resultMerger.MergeResults(correlations)

	// Create summary
	summary := ce.createCorrelationSummary(correlations, time.Since(start))

	// Ensure slices are non-nil so JSON marshals them as [] instead of null
	if correlations == nil {
		correlations = make([]models.Correlation, 0)
	}
	if summary.EnginesInvolved == nil {
		summary.EnginesInvolved = make([]models.QueryType, 0)
	}

	result := &models.UnifiedCorrelationResult{
		Correlations: correlations,
		Summary:      summary,
	}

	if ce.logger != nil {
		ce.logger.Info("Correlation query completed",
			"query_id", query.ID,
			"correlations_found", len(correlations),
			"execution_time_ms", time.Since(start).Milliseconds())
	}

	// Record successful correlation metrics
	correlationType := "time_window"
	if query.TimeWindow == nil {
		correlationType = "label_based"
	}
	monitoring.RecordUnifiedQueryCorrelationOperation(correlationType, len(query.Expressions), time.Since(start), true)

	// Record correlation metrics on span
	ce.tracer.RecordQueryMetrics(corrSpan, time.Since(start), int64(len(correlations)), true)

	return result, nil
}

// Correlate implements the new TimeRange-based correlation entrypoint.
// It builds temporal rings, discovers impacted and candidate KPIs using
// existing metric/logs/traces clients, and computes simple suspicion scores.
func (ce *CorrelationEngineImpl) Correlate(ctx context.Context, tr models.TimeRange) (*models.CorrelationResult, error) {
	// Basic validations
	if tr.Duration() == 0 {
		return nil, fmt.Errorf("invalid time range: duration is zero")
	}

	// Build rings from EngineConfig
	rings := BuildRings(tr, ce.engineCfg)

	// KPI-first discovery: list KPIs from Stage-00 KPI registry and probe backends
	impactKPIs := []string{}
	candidateKPIs := []string{}

	// Map to hold discovered label index per KPI id
	labelIndex := make(map[string]map[string]map[string]struct{}) // kpiID -> labelKey -> set(values)

	if ce.kpiRepo != nil {
		// List all KPIs relevant to correlation (use high limit to capture all registry KPIs)
		kpis, _, err := ce.kpiRepo.ListKPIs(ctx, models.KPIListRequest{Limit: 1000})
		if err != nil {
			if ce.logger != nil {
				ce.logger.Warn("failed to list KPIs from registry", "err", err)
			}
		} else {
			// For each KPI, attempt a lightweight probe using the configured query/formula
			for _, kp := range kpis {
				if kp == nil {
					continue
				}
				kpiID := kp.ID
				labelIndex[kpiID] = make(map[string]map[string]struct{})

				// Decide which backend to query based on KPI metadata
				sig := strings.ToLower(kp.SignalType)
				ds := strings.ToLower(kp.Datastore)

				// Build time window for probes: use core ring if available else full tr
				probeStart := tr.Start
				probeEnd := tr.End
				if len(rings) > 0 {
					probeStart = rings[len(rings)/2].Start
					probeEnd = rings[len(rings)/2].End
				}

				// Respect engine configured query limit
				limit := ce.engineCfg.DefaultQueryLimit
				if limit <= 0 {
					limit = 1000
				}

				// metrics
				if strings.Contains(sig, "metric") || strings.Contains(ds, "metric") || strings.Contains(strings.ToLower(kp.QueryType), "metric") {
					qstr := kp.Formula
					if qstr == "" {
						// Try to derive simple query from Query map if formula absent
						if qmap := kp.Query; qmap != nil {
							if raw, ok := qmap["query"].(string); ok {
								qstr = raw
							}
						}
					}
					if qstr != "" && ce.metricsService != nil {
						req := &models.MetricsQLQueryRequest{
							Query: qstr,
							Time:  probeEnd.Format(time.RFC3339),
						}
						res, err := ce.metricsService.ExecuteQuery(ctx, req)
						if err != nil {
							if ce.logger != nil {
								ce.logger.Debug("metrics probe failed for KPI", "kpi", kp.ID, "name", kp.Name, "query", qstr, "err", err)
							}
						} else if res != nil && res.SeriesCount > 0 {
							if ce.logger != nil {
								ce.logger.Debug("metrics probe SUCCESS for KPI", "kpi", kp.ID, "name", kp.Name, "series_count", res.SeriesCount, "layer", kp.Layer)
							}
							// extract labels from metrics result
							ur := &models.UnifiedResult{Data: res.Data}
							dls := ce.extractLabelsFromMetricsResult(ur)
							for _, dl := range dls {
								for k, v := range dl.Labels {
									if labelIndex[kpiID][k] == nil {
										labelIndex[kpiID][k] = make(map[string]struct{})
									}
									labelIndex[kpiID][k][v] = struct{}{}
								}
							}
							candidateKPIs = append(candidateKPIs, kp.ID)
							// If KPI classifies as impact via Layer hint, promote
							if strings.ToLower(kp.Layer) == "impact" {
								impactKPIs = append(impactKPIs, kp.ID)
							}
						}
					}
				}

				// logs
				if strings.Contains(sig, "log") || strings.Contains(ds, "log") || strings.Contains(strings.ToLower(kp.QueryType), "log") {
					qstr := kp.Formula
					if qstr == "" {
						if qmap := kp.Query; qmap != nil {
							if raw, ok := qmap["query"].(string); ok {
								qstr = raw
							}
						}
					}
					if qstr != "" && ce.logsService != nil {
						lq := &models.LogsQLQueryRequest{
							Query: qstr,
							Start: probeStart.UnixMilli(),
							End:   probeEnd.UnixMilli(),
							Limit: limit,
						}
						res, err := ce.logsService.ExecuteQuery(ctx, lq)
						if err != nil {
							if ce.logger != nil {
								ce.logger.Debug("logs probe failed for KPI", "kpi", kp.ID, "err", err)
							}
						} else if res != nil && len(res.Logs) > 0 {
							ur := &models.UnifiedResult{Data: res.Logs}
							dls := ce.extractLabelsFromLogsResult(ur)
							for _, dl := range dls {
								for k, v := range dl.Labels {
									if labelIndex[kpiID][k] == nil {
										labelIndex[kpiID][k] = make(map[string]struct{})
									}
									labelIndex[kpiID][k][v] = struct{}{}
								}
							}
							candidateKPIs = append(candidateKPIs, kp.ID)
							if strings.ToLower(kp.Layer) == "impact" {
								impactKPIs = append(impactKPIs, kp.ID)
							}
						}
					}
				}

				// traces
				if strings.Contains(sig, "trace") || strings.Contains(ds, "trace") || strings.Contains(strings.ToLower(kp.QueryType), "trace") {
					// For traces, prefer SearchTraces when available
					if ce.tracesService != nil {
						// Build a lightweight search request if formula is available
						req := &models.TraceSearchRequest{
							Start: models.FlexibleTime{Time: probeStart},
							End:   models.FlexibleTime{Time: probeEnd},
							Limit: 500,
						}
						if kp.Formula != "" {
							req.Tags = kp.Formula
						}
						res, err := ce.tracesService.SearchTraces(ctx, req)
						if err != nil {
							if ce.logger != nil {
								ce.logger.Debug("traces probe failed for KPI", "kpi", kp.ID, "err", err)
							}
						} else if res != nil && len(res.Traces) > 0 {
							ur := &models.UnifiedResult{Data: res.Traces}
							dls := ce.extractLabelsFromTracesResult(ur)
							for _, dl := range dls {
								for k, v := range dl.Labels {
									if labelIndex[kpiID][k] == nil {
										labelIndex[kpiID][k] = make(map[string]struct{})
									}
									labelIndex[kpiID][k][v] = struct{}{}
								}
							}
							candidateKPIs = append(candidateKPIs, kp.ID)
							if strings.ToLower(kp.Layer) == "impact" {
								impactKPIs = append(impactKPIs, kp.ID)
							}
						}
					}
				}
			}
		}
	} else {
		// Fallback: if no KPI registry available, keep previous probe behaviour using config probes
		probes := ce.engineCfg.Probes
		for _, q := range probes {
			req := &models.MetricsQLQueryRequest{Query: q}
			res, err := ce.metricsService.ExecuteQuery(ctx, req)
			if err != nil {
				if ce.logger != nil {
					ce.logger.Warn("probe metrics query failed", "query", q, "err", err)
				}
				continue
			}
			if res != nil && res.SeriesCount > 0 {
				candidateKPIs = append(candidateKPIs, q)
			}
		}
		if len(impactKPIs) == 0 && len(candidateKPIs) > 0 {
			impactKPIs = append(impactKPIs, candidateKPIs[0])
			candidateKPIs = candidateKPIs[1:]
		}
	}

	// Build red anchors from impact KPIs and compute simple confidence
	var redAnchors []*models.RedAnchor
	// Resolve impact KPI IDs to human names when possible (preserve original id in KPIUUID fields)
	resolvedAffected := make([]string, 0, len(impactKPIs))
	for _, k := range impactKPIs {
		svc := k
		metric := k
		// attempt to resolve KPI definition
		if ce.kpiRepo != nil {
			if kp, err := ce.kpiRepo.GetKPI(ctx, k); err == nil && kp != nil {
				// Use human-readable name when available
				if kp.Name != "" {
					svc = kp.Name
					metric = kp.Formula
				}
			}
		}

		node := models.RedAnchor{
			Service:   svc,
			Metric:    metric,
			Score:     0.9,
			Threshold: ce.engineCfg.MinAnomalyScore,
			Timestamp: tr.End,
			DataType:  "kpi",
		}
		redAnchors = append(redAnchors, &node)
		resolvedAffected = append(resolvedAffected, svc)
	}

	// Simple confidence: average of red anchor scores (fallback 0.0)
	confidence := 0.0
	if len(redAnchors) > 0 {
		sum := 0.0
		for _, r := range redAnchors {
			sum += r.Score
		}
		confidence = sum / float64(len(redAnchors))
	}

	// Build CorrelationResult (scaffold)
	corr := &models.CorrelationResult{
		CorrelationID: fmt.Sprintf("corr_%d", time.Now().Unix()),
		IncidentID:    "",
		RootCause:     "",
		Confidence:    confidence,
		// Store human-friendly affected service names where available
		AffectedServices: resolvedAffected,
		// Causes will be populated with candidate KPIs and preliminary suspicion scores.
		// NOTE(AT-007): This is a minimal, deterministic scoring placeholder. Full
		// statistical wiring (compute per-pair Pearson/Spearman/cross-corr, and
		// combine into suspicion) will be implemented under AT-007. Do not rely
		// on these values for production decisions until completed.
		Causes:          []models.CauseCandidate{},
		Timeline:        []models.TimelineEvent{},
		RedAnchors:      redAnchors,
		Recommendations: []string{},
		CreatedAt:       time.Now(),
	}

	// Add timeline events summarizing rings
	for i, r := range rings {
		ev := models.TimelineEvent{
			Time:         r.Start,
			Event:        fmt.Sprintf("ring_%d", i),
			Service:      "system",
			Severity:     "info",
			AnomalyScore: 0.0,
			DataSource:   "rings",
		}
		corr.Timeline = append(corr.Timeline, ev)
	}

	// Populate Causes with a deterministic baseline suspicion score so downstream
	// RCA machinery can consume candidate scoring during AT-007 work.
	// Replace baseline scoring with real statistical wiring across rings.
	for _, k := range candidateKPIs {
		// Default candidate entry (will be updated when stats available)
		cand := models.CauseCandidate{
			KPI:            k,
			Service:        "",
			SuspicionScore: 0.0,
			Reasons:        []string{},
			Stats:          nil,
		}

		// Obtain KPI definitions when available (for query/formula)
		var candKPI *models.KPIDefinition
		if ce.kpiRepo != nil {
			if kp, err := ce.kpiRepo.GetKPI(ctx, k); err == nil {
				candKPI = kp
			}
		}

		// If we resolved a KPI definition, prefer the human-readable name in the
		// KPI field and preserve original id and formula in the new fields.
		if candKPI != nil {
			if candKPI.Name != "" {
				cand.KPI = candKPI.Name
			}
			cand.KPIUUID = candKPI.ID
			cand.KPIFormula = candKPI.Formula
		}

		// Use first discovered impact KPI as the impact signal for pairwise stats
		if len(impactKPIs) == 0 {
			// No impact KPI discovered; keep zero suspicion but include as candidate
			cand.Reasons = append(cand.Reasons, "no_impact_kpi")
			corr.Causes = append(corr.Causes, cand)
			continue
		}
		impactID := impactKPIs[0]
		var impactKPI *models.KPIDefinition
		if ce.kpiRepo != nil {
			if kp, err := ce.kpiRepo.GetKPI(ctx, impactID); err == nil {
				impactKPI = kp
			}
		}

		// Build per-ring sample vectors by computing a ring-level aggregate (mean)
		var impactVals []float64
		var causeVals []float64
		for _, r := range rings {
			_ = r // ring time currently unused for simple per-ring mean extraction
			// Query impact KPI for ring
			if impactKPI != nil && impactKPI.Formula != "" && ce.metricsService != nil {
				req := &models.MetricsQLQueryRequest{Query: impactKPI.Formula}
				res, err := ce.metricsService.ExecuteQuery(ctx, req)
				if err == nil {
					if v := extractAverageFromMetricsResult(res); !math.IsNaN(v) {
						impactVals = append(impactVals, v)
					}
				}
			}

			// Query candidate KPI for ring
			if candKPI != nil && candKPI.Formula != "" && ce.metricsService != nil {
				req := &models.MetricsQLQueryRequest{Query: candKPI.Formula}
				res, err := ce.metricsService.ExecuteQuery(ctx, req)
				if err == nil {
					if v := extractAverageFromMetricsResult(res); !math.IsNaN(v) {
						causeVals = append(causeVals, v)
					}
				}
			}
		}

		// Need at least 2 samples to compute correlations
		n := len(impactVals)
		if n >= 2 && len(causeVals) >= 2 && n == len(causeVals) {
			// Compute stats using helper (max lag = n-1). Attempt to locate a
			// simple confounder series via KPI registry heuristics (Stage-01
			// supports a single confounder heuristic). NOTE(AT-012): do not
			// hardcode KPI names; rely on KPI metadata when available.
			pearson, spearman, crossMax := 0.0, 0.0, 0.0
			crossLag := 0
			partial := 0.0

			// Try to find a confounder KPI by kind/tags (e.g. infra/global/load)
			var confounderVals []float64
			if ce.kpiRepo != nil {
				if kpis, _, err := ce.kpiRepo.ListKPIs(ctx, models.KPIListRequest{}); err == nil {
					for _, kp := range kpis {
						if kp == nil {
							continue
						}
						kind := strings.ToLower(kp.Kind)
						// Heuristic: treat infra/global/load kinds or explicit tag as confounder
						if kind == "infra" || strings.Contains(kind, "load") || (kp.Tags != nil && (containsString(kp.Tags, "confounder") || containsString(kp.Tags, "role=confounder"))) {
							// fetch per-ring aggregates for this candidate confounder
							var vals []float64
							for range rings {
								if kp.Formula != "" && ce.metricsService != nil {
									req := &models.MetricsQLQueryRequest{Query: kp.Formula}
									res, err := ce.metricsService.ExecuteQuery(ctx, req)
									if err == nil {
										if v := extractAverageFromMetricsResult(res); !math.IsNaN(v) {
											vals = append(vals, v)
										}
									}
								}
							}
							if len(vals) == n {
								confounderVals = vals
								break
							}
						}
					}
				} else {
					// NOTE(AT-012): KPI registry lookup failed; continue without confounder
				}
			}

			if len(confounderVals) == n {
				pearson, spearman, crossMax, crossLag, partial, _ = ComputeCorrelationStats(impactVals, causeVals, n-1, confounderVals)
			} else {
				pearson, spearman, crossMax, crossLag, partial, _ = ComputeCorrelationStats(impactVals, causeVals, n-1)
			}

			stats := &models.CorrelationStats{
				Pearson:      pearson,
				Spearman:     spearman,
				CrossCorrMax: crossMax,
				CrossCorrLag: crossLag,
				Partial:      partial,
				SampleSize:   n,
				PValue:       0.0,
				Confidence:   0.0,
			}

			// Derive a lightweight confidence score from absolute correlations
			stats.Confidence = (math.Abs(stats.Pearson) + math.Abs(stats.Spearman)) / 2.0

			// Compute a lightweight anomaly density for the candidate using the
			// per-ring aggregated values we already have. This is Stage-01
			// heuristic: fraction of rings with values beyond mean +/- 2*std.
			anomalyDensity := 0.0
			if len(causeVals) > 0 {
				mean := 0.0
				for _, v := range causeVals {
					mean += v
				}
				mean /= float64(len(causeVals))
				sd := 0.0
				for _, v := range causeVals {
					d := v - mean
					sd += d * d
				}
				if len(causeVals) > 1 {
					sd = math.Sqrt(sd / float64(len(causeVals)))
				} else {
					sd = 0.0
				}
				if sd > 0 {
					threshHigh := mean + 2*sd
					threshLow := mean - 2*sd
					count := 0
					for _, v := range causeVals {
						if v > threshHigh || v < threshLow {
							count++
						}
					}
					anomalyDensity = float64(count) / float64(len(causeVals))
				}
			}

			// Compute suspicion score driven by engine config; include partial and anomaly density
			susp := ComputeSuspicionScore(stats.Pearson, stats.Spearman, stats.CrossCorrMax, stats.CrossCorrLag, stats.SampleSize, ce.engineCfg.MinCorrelation, stats.Partial, anomalyDensity)

			// Populate candidate entry
			cand.Stats = stats
			cand.SuspicionScore = susp
			// Reasons (structured tags)
			if math.Abs(stats.Pearson) >= ce.engineCfg.MinCorrelation {
				cand.Reasons = append(cand.Reasons, "strong_pearson")
			}
			if math.Abs(stats.Spearman) >= ce.engineCfg.MinCorrelation {
				cand.Reasons = append(cand.Reasons, "strong_spearman")
			}
			if stats.CrossCorrMax > 0.5 && stats.CrossCorrLag > 0 {
				cand.Reasons = append(cand.Reasons, "lagged_cause_precedes_impact")
			}
			// Partial-correlation based reasons
			if stats.Partial == 0.0 {
				cand.Reasons = append(cand.Reasons, "partial_correlation_not_available_no_confounder")
			} else {
				// Compare magnitude of partial vs pearson to decide whether partial
				// supports direct link or suggests confounding.
				if math.Abs(stats.Pearson) > 0 {
					ratio := math.Abs(stats.Partial) / math.Abs(stats.Pearson)
					if ratio >= 0.8 {
						cand.Reasons = append(cand.Reasons, "partial_supports_direct_link")
					} else if ratio < 0.5 {
						cand.Reasons = append(cand.Reasons, "partial_suggests_confounding")
						cand.Reasons = append(cand.Reasons, "partial_penalized_due_to_confounding")
					}
				}
			}

			// Anomaly density reasons
			if anomalyDensity > 0.5 {
				cand.Reasons = append(cand.Reasons, "high_anomaly_density")
			} else if anomalyDensity == 0 {
				cand.Reasons = append(cand.Reasons, "no_anomalies_detected")
			}
			if stats.SampleSize < 3 {
				cand.Reasons = append(cand.Reasons, "small_sample_size")
			}

			// Attach KPI/service hints
			if candKPI != nil && candKPI.ServiceFamily != "" {
				cand.Service = candKPI.ServiceFamily
			}

			corr.Causes = append(corr.Causes, cand)
		} else {
			// Not enough data; mark candidate with a diagnostic reason
			cand.Reasons = append(cand.Reasons, "insufficient_data")
			corr.Causes = append(corr.Causes, cand)
		}
	}

	return corr, nil
}

// BuildRings constructs pre/core/post rings for a given TimeRange using EngineConfig
func BuildRings(tr models.TimeRange, cfg config.EngineConfig) []models.TimeRange {
	var rings []models.TimeRange

	// Determine core window size (fallback to full range when invalid)
	coreSize := cfg.Buckets.CoreWindowSize
	if coreSize <= 0 || coreSize > tr.Duration() {
		coreSize = tr.Duration()
	}

	// Determine ring step (fallback to half core, then minute)
	step := cfg.Buckets.RingStep
	if step <= 0 {
		step = coreSize / 2
		if step <= 0 {
			step = time.Minute
		}
	}

	pre := cfg.Buckets.PreRings
	post := cfg.Buckets.PostRings

	// Core ring anchored at end of provided time range, clip to tr
	coreEnd := tr.End
	coreStart := coreEnd.Add(-coreSize)
	if coreStart.Before(tr.Start) {
		coreStart = tr.Start
	}
	if coreEnd.After(tr.End) {
		coreEnd = tr.End
	}

	// If core is invalid (zero or negative length), fallback to full tr
	if !coreStart.Before(coreEnd) {
		coreStart = tr.Start
		coreEnd = tr.End
		coreSize = tr.Duration()
	}

	// Build pre rings (older than core). We'll build from newest->oldest then reverse
	var preTemp []models.TimeRange
	end := coreStart
	for i := 0; i < pre; i++ {
		start := end.Add(-step)
		// Clip to tr boundaries
		if end.After(tr.End) {
			end = tr.End
		}
		if start.Before(tr.Start) {
			start = tr.Start
		}
		// If this candidate is entirely before tr or invalid, stop
		if !start.Before(end) || !end.After(tr.Start) {
			break
		}
		preTemp = append(preTemp, models.TimeRange{Start: start, End: end})
		// move window backward
		end = start
		// If we've reached the very start, no further pre rings possible
		if end.Equal(tr.Start) {
			break
		}
	}
	// reverse preTemp to append oldest->newest
	for i := len(preTemp) - 1; i >= 0; i-- {
		rings = append(rings, preTemp[i])
	}

	// Add core ring
	rings = append(rings, models.TimeRange{Start: coreStart, End: coreEnd})

	// Add post rings (newer than core)
	start := coreEnd
	for i := 0; i < post; i++ {
		// move forward
		end := start.Add(step)
		if !start.Before(tr.End) {
			break
		}
		if end.After(tr.End) {
			end = tr.End
		}
		if !start.Before(end) {
			break
		}
		rings = append(rings, models.TimeRange{Start: start, End: end})
		start = end
		if start.Equal(tr.End) {
			break
		}
	}

	return rings
}

// executeExpressionsParallel executes all expressions in the correlation query in parallel
func (ce *CorrelationEngineImpl) executeExpressionsParallel(ctx context.Context, query *models.CorrelationQuery) (map[models.QueryType]*models.UnifiedResult, error) {
	parallelStart := time.Now()
	results := make(map[models.QueryType]*models.UnifiedResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstError error
	var errorOnce sync.Once

	// Group expressions by engine to avoid duplicate queries
	engineExpressions := make(map[models.QueryType][]models.CorrelationExpression)
	for _, expr := range query.Expressions {
		engineExpressions[expr.Engine] = append(engineExpressions[expr.Engine], expr)
	}

	// Execute queries for each engine in parallel
	for engine, expressions := range engineExpressions {
		wg.Add(1)
		go func(engine models.QueryType, expressions []models.CorrelationExpression) {
			defer wg.Done()

			engineStart := time.Now()
			result, err := ce.executeEngineQuery(ctx, engine, expressions, query)
			engineDuration := time.Since(engineStart)

			// Record individual engine query duration
			monitoring.RecordCorrelationEngineQueryDuration(string(engine), query.ID, engineDuration)

			if err != nil {
				errorOnce.Do(func() {
					firstError = err
				})
				if ce.logger != nil {
					ce.logger.Error("Failed to execute engine query",
						"engine", engine,
						"error", err)
				}
				return
			}

			mu.Lock()
			results[engine] = result
			// Log brief summary of the engine result for debugging correlation effectiveness
			if ce.logger != nil {
				recCount := 0
				if result != nil && result.Metadata != nil {
					recCount = result.Metadata.TotalRecords
				}
				ce.logger.Info("Engine query completed for correlation",
					"engine", engine,
					"query_id", query.ID,
					"record_count", recCount,
					"duration_ms", engineDuration.Milliseconds())
			}
			mu.Unlock()
		}(engine, expressions)
	}

	wg.Wait()

	// Record parallel execution coordination duration
	parallelDuration := time.Since(parallelStart)
	monitoring.RecordCorrelationParallelExecutionDuration(len(engineExpressions), parallelDuration)

	if firstError != nil {
		return nil, firstError
	}

	return results, nil
}

// executeEngineQuery executes queries for a specific engine
func (ce *CorrelationEngineImpl) executeEngineQuery(
	ctx context.Context,
	engine models.QueryType,
	expressions []models.CorrelationExpression,
	corrQuery *models.CorrelationQuery,
) (*models.UnifiedResult, error) {
	switch engine {
	case models.QueryTypeMetrics:
		return ce.executeMetricsCorrelationQuery(ctx, expressions, corrQuery)
	case models.QueryTypeLogs:
		return ce.executeLogsCorrelationQuery(ctx, expressions, corrQuery)
	case models.QueryTypeTraces:
		return ce.executeTracesCorrelationQuery(ctx, expressions, corrQuery)
	default:
		return nil, fmt.Errorf("unsupported engine for correlation: %s", engine)
	}
}

// executeMetricsCorrelationQuery executes metrics queries for correlation
func (ce *CorrelationEngineImpl) executeMetricsCorrelationQuery(
	ctx context.Context,
	expressions []models.CorrelationExpression,
	corrQuery *models.CorrelationQuery,
) (*models.UnifiedResult, error) {
	if ce.metricsService == nil {
		return nil, fmt.Errorf("metrics service not configured")
	}

	// NOTE: For Stage-01 we execute the first expression. Handling multiple
	// expressions per engine is planned in the correlation-RCA action tracker (AT-007).
	expr := expressions[0]

	// Create unified query for metrics
	query := &models.UnifiedQuery{
		ID:    corrQuery.ID + "_metrics",
		Type:  models.QueryTypeMetrics,
		Query: expr.Query,
	}

	if corrQuery.TimeWindow != nil {
		// For time-window correlations, extend the time range
		endTime := time.Now()
		startTime := endTime.Add(-*corrQuery.TimeWindow * 2) // Extend window for correlation
		query.StartTime = &startTime
		query.EndTime = &endTime
	}

	// Execute the metrics query
	metricsQuery := &models.MetricsQLQueryRequest{
		Query: query.Query,
	}

	result, err := ce.metricsService.ExecuteQuery(ctx, metricsQuery)
	if err != nil {
		return nil, err
	}

	// Log metrics query and result summary for debugging
	if ce.logger != nil {
		ce.logger.Info("Metrics correlation query executed",
			"query", metricsQuery.Query,
			"series_count", result.SeriesCount,
			"query_id", query.ID)
	}

	// Convert to UnifiedResult
	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeMetrics,
		Status:  result.Status,
		Data:    result.Data,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeMetrics: {
					Engine:      models.QueryTypeMetrics,
					Status:      result.Status,
					RecordCount: result.SeriesCount,
					DataSource:  "victoria-metrics",
				},
			},
			TotalRecords: result.SeriesCount,
			DataSources:  []string{"victoria-metrics"},
		},
	}, nil
}

// executeLogsCorrelationQuery executes logs queries for correlation
func (ce *CorrelationEngineImpl) executeLogsCorrelationQuery(
	ctx context.Context,
	expressions []models.CorrelationExpression,
	corrQuery *models.CorrelationQuery,
) (*models.UnifiedResult, error) {
	if ce.logsService == nil {
		return nil, fmt.Errorf("logs service not configured")
	}

	// NOTE: For Stage-01 we execute the first expression. Handling multiple
	// expressions per engine is planned in the correlation-RCA action tracker (AT-007).
	expr := expressions[0]

	// Create unified query for logs
	query := &models.UnifiedQuery{
		ID:    corrQuery.ID + "_logs",
		Type:  models.QueryTypeLogs,
		Query: expr.Query,
	}

	if corrQuery.TimeWindow != nil {
		// For time-window correlations, extend the time range
		endTime := time.Now()
		startTime := endTime.Add(-*corrQuery.TimeWindow * 2)
		query.StartTime = &startTime
		query.EndTime = &endTime
	}

	startTime := int64(0)
	endTime := int64(0)
	if query.StartTime != nil {
		startTime = query.StartTime.UnixMilli()
	}
	if query.EndTime != nil {
		endTime = query.EndTime.UnixMilli()
	}

	logsQuery := &models.LogsQLQueryRequest{
		Query: query.Query,
		Start: startTime,
		End:   endTime,
		Limit: func() int {
			if ce.engineCfg.DefaultQueryLimit > 0 {
				return ce.engineCfg.DefaultQueryLimit
			}
			return 1000
		}(),
	}

	result, err := ce.logsService.ExecuteQuery(ctx, logsQuery)
	if err != nil {
		return nil, err
	}

	// Log logs query and result summary for debugging
	if ce.logger != nil {
		ce.logger.Info("Logs correlation query executed",
			"query", logsQuery.Query,
			"log_count", len(result.Logs),
			"query_id", query.ID)
	}

	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeLogs,
		Status:  "success",
		Data:    result.Logs,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeLogs: {
					Engine:      models.QueryTypeLogs,
					Status:      "success",
					RecordCount: len(result.Logs),
					DataSource:  "victoria-logs",
				},
			},
			TotalRecords: len(result.Logs),
			DataSources:  []string{"victoria-logs"},
		},
	}, nil
}

// executeTracesCorrelationQuery executes traces queries for correlation
func (ce *CorrelationEngineImpl) executeTracesCorrelationQuery(
	ctx context.Context,
	expressions []models.CorrelationExpression,
	corrQuery *models.CorrelationQuery,
) (*models.UnifiedResult, error) {
	if ce.tracesService == nil {
		return nil, fmt.Errorf("traces service not configured")
	}

	// NOTE: For Stage-01 we execute the first expression. Handling multiple
	// expressions per engine is planned in the correlation-RCA action tracker (AT-007).
	expr := expressions[0]

	// Create unified query for traces
	query := &models.UnifiedQuery{
		ID:    corrQuery.ID + "_traces",
		Type:  models.QueryTypeTraces,
		Query: expr.Query,
	}

	if corrQuery.TimeWindow != nil {
		// For time-window correlations, extend the time range
		endTime := time.Now()
		startTime := endTime.Add(-*corrQuery.TimeWindow * 2)
		query.StartTime = &startTime
		query.EndTime = &endTime
	}

	// For traces, use GetOperations as a basic implementation
	operations, err := ce.tracesService.GetOperations(ctx, expr.Query)
	if err != nil {
		return nil, err
	}

	// Log traces query and result summary for debugging
	if ce.logger != nil {
		ce.logger.Info("Traces correlation query executed",
			"query", expr.Query,
			"ops_count", len(operations),
			"query_id", query.ID)
	}

	return &models.UnifiedResult{
		QueryID: query.ID,
		Type:    models.QueryTypeTraces,
		Status:  "success",
		Data:    operations,
		Metadata: &models.ResultMetadata{
			EngineResults: map[models.QueryType]*models.EngineResult{
				models.QueryTypeTraces: {
					Engine:      models.QueryTypeTraces,
					Status:      "success",
					RecordCount: len(operations),
					DataSource:  "victoria-traces",
				},
			},
			TotalRecords: len(operations),
			DataSources:  []string{"victoria-traces"},
		},
	}, nil
}

// correlateResults correlates results from different engines
func (ce *CorrelationEngineImpl) correlateResults(
	query *models.CorrelationQuery,
	results map[models.QueryType]*models.UnifiedResult,
) []models.Correlation {
	var correlations []models.Correlation

	if query.TimeWindow != nil {
		// Time-window correlation
		correlations = ce.correlateByTimeWindow(query, results)
	} else {
		// Label-based correlation
		correlations = ce.correlateByLabels(query, results)
	}

	return correlations
}

// correlateByTimeWindow correlates results within a time window
func (ce *CorrelationEngineImpl) correlateByTimeWindow(
	query *models.CorrelationQuery,
	results map[models.QueryType]*models.UnifiedResult,
) []models.Correlation {
	var correlations []models.Correlation

	// For time-window correlation, we expect exactly 2 expressions
	if len(query.Expressions) != 2 {
		if ce.logger != nil {
			ce.logger.Warn("Time-window correlation requires exactly 2 expressions",
				"expressions_count", len(query.Expressions))
		}
		return correlations
	}

	expr1 := query.Expressions[0]
	expr2 := query.Expressions[1]

	result1, exists1 := results[expr1.Engine]
	result2, exists2 := results[expr2.Engine]

	if !exists1 || !exists2 {
		if ce.logger != nil {
			ce.logger.Warn("Missing results for time-window correlation",
				"expr1_engine", expr1.Engine, "has_result1", exists1,
				"expr2_engine", expr2.Engine, "has_result2", exists2)
		}
		return correlations
	}

	// Extract timestamps and data points from results
	dataPoints1 := ce.extractDataPointsWithTimestamps(result1, expr1.Engine)
	dataPoints2 := ce.extractDataPointsWithTimestamps(result2, expr2.Engine)

	// Find correlations within the time window
	windowCorrelations := ce.findTimeWindowCorrelations(dataPoints1, dataPoints2, *query.TimeWindow)

	// Convert to correlation objects
	for _, wc := range windowCorrelations {
		correlation := models.Correlation{
			ID:         fmt.Sprintf("%s_time_window_%d", query.ID, len(correlations)+1),
			Timestamp:  wc.Timestamp,
			Engines:    make(map[models.QueryType]interface{}),
			Confidence: wc.Confidence,
			Metadata: map[string]interface{}{
				"time_window":      query.TimeWindow.String(),
				"correlation_type": "time_window",
			},
		}

		// Add correlated data points
		if wc.DataPoint1 != nil {
			correlation.Engines[expr1.Engine] = wc.DataPoint1
		}
		if wc.DataPoint2 != nil {
			correlation.Engines[expr2.Engine] = wc.DataPoint2
		}

		correlations = append(correlations, correlation)
	}

	return correlations
}

// correlateByLabels correlates results by shared labels
func (ce *CorrelationEngineImpl) correlateByLabels(
	query *models.CorrelationQuery,
	results map[models.QueryType]*models.UnifiedResult,
) []models.Correlation {
	var correlations []models.Correlation

	// Extract labels from all results
	resultLabels := make(map[models.QueryType][]dataLabels)
	for engine, result := range results {
		resultLabels[engine] = ce.extractLabelsFromResult(result, engine)
	}

	// Find correlations based on label matches
	for i, expr1 := range query.Expressions {
		for j, expr2 := range query.Expressions {
			if i >= j {
				continue // Avoid duplicate correlations
			}

			labels1 := resultLabels[expr1.Engine]
			labels2 := resultLabels[expr2.Engine]

			if len(labels1) == 0 || len(labels2) == 0 {
				continue
			}

			// Find matching labels between the two result sets
			labelMatches := ce.findLabelMatches(labels1, labels2)

			if len(labelMatches) > 0 {
				correlation := models.Correlation{
					ID:         fmt.Sprintf("%s_label_match_%d_%d", query.ID, i, j),
					Timestamp:  time.Now(), // NOTE: representative timestamp; per AT-007 we'll improve timestamp selection
					Engines:    make(map[models.QueryType]interface{}),
					Confidence: ce.calculateLabelMatchConfidence(labelMatches),
					Metadata: map[string]interface{}{
						"correlation_type": "label_based",
						"label_matches":    labelMatches,
					},
				}

				// Add sample data from both engines (first match)
				if len(labels1) > 0 && labels1[0].Data != nil {
					correlation.Engines[expr1.Engine] = labels1[0].Data
				}
				if len(labels2) > 0 && labels2[0].Data != nil {
					correlation.Engines[expr2.Engine] = labels2[0].Data
				}

				correlations = append(correlations, correlation)
			}
		}
	}

	return correlations
} // createCorrelationSummary creates a summary of correlation results
func (ce *CorrelationEngineImpl) createCorrelationSummary(
	correlations []models.Correlation,
	executionTime time.Duration,
) models.CorrelationSummary {
	engines := make(map[models.QueryType]bool)
	totalConfidence := 0.0

	for _, corr := range correlations {
		for engine := range corr.Engines {
			engines[engine] = true
		}
		totalConfidence += corr.Confidence
	}

	var enginesInvolved []models.QueryType
	for engine := range engines {
		enginesInvolved = append(enginesInvolved, engine)
	}

	avgConfidence := 0.0
	if len(correlations) > 0 {
		avgConfidence = totalConfidence / float64(len(correlations))
	}

	return models.CorrelationSummary{
		TotalCorrelations: len(correlations),
		AverageConfidence: avgConfidence,
		TimeRange:         executionTime.String(),
		EnginesInvolved:   enginesInvolved,
	}
}

// ValidateCorrelationQuery validates a correlation query
func (ce *CorrelationEngineImpl) ValidateCorrelationQuery(query *models.CorrelationQuery) error {
	return query.Validate()
}

// GetCorrelationExamples returns example correlation queries
func (ce *CorrelationEngineImpl) GetCorrelationExamples() []string {
	return models.CorrelationQueryExamples
}

// DetectComponentFailures detects component failures in the financial transaction system
func (ce *CorrelationEngineImpl) DetectComponentFailures(ctx context.Context, timeRange models.TimeRange, components []models.FailureComponent) (*models.FailureCorrelationResult, error) {
	start := time.Now()
	if ce.logger != nil {
		ce.logger.Info("Detecting component failures",
			"time_range", timeRange,
			"components", components)
	}

	// Query for error signals across all engines
	errorSignals, err := ce.queryErrorSignals(ctx, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to query error signals: %w", err)
	}

	// Group signals by transaction_id and component
	incidentGroups := ce.groupSignalsByTransactionAndComponent(errorSignals, components)

	// Create failure incidents
	var incidents []models.FailureIncident
	for _, group := range incidentGroups {
		incident := ce.createFailureIncident(group, timeRange)
		if incident != nil {
			incidents = append(incidents, *incident)
		}
	}

	// Create summary
	summary := ce.createFailureSummary(incidents, timeRange)

	result := &models.FailureCorrelationResult{
		Incidents: incidents,
		Summary:   *summary,
	}

	if ce.logger != nil {
		ce.logger.Info("Component failure detection completed",
			"incidents_found", len(incidents),
			"execution_time_ms", time.Since(start).Milliseconds())
	}

	return result, nil
}

// CorrelateTransactionFailures correlates failures for specific transaction IDs
func (ce *CorrelationEngineImpl) CorrelateTransactionFailures(ctx context.Context, transactionIDs []string, timeRange models.TimeRange) (*models.FailureCorrelationResult, error) {
	start := time.Now()
	if ce.logger != nil {
		ce.logger.Info("Correlating transaction failures",
			"transaction_ids", transactionIDs,
			"time_range", timeRange)
	}

	// Query for error signals for specific transactions
	errorSignals, err := ce.queryErrorSignalsForTransactions(ctx, transactionIDs, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to query error signals for transactions: %w", err)
	}

	// Group signals by transaction_id
	incidentGroups := ce.groupSignalsByTransaction(errorSignals)

	// Create failure incidents
	var incidents []models.FailureIncident
	for _, group := range incidentGroups {
		incident := ce.createFailureIncidentForTransaction(group, timeRange)
		if incident != nil {
			incidents = append(incidents, *incident)
		}
	}

	// Create summary
	summary := ce.createFailureSummary(incidents, timeRange)

	result := &models.FailureCorrelationResult{
		Incidents: incidents,
		Summary:   *summary,
	}

	if ce.logger != nil {
		ce.logger.Info("Transaction failure correlation completed",
			"incidents_found", len(incidents),
			"execution_time_ms", time.Since(start).Milliseconds())
	}

	return result, nil
}

// timeWindowCorrelation represents a correlation found within a time window
type timeWindowCorrelation struct {
	Timestamp  time.Time
	DataPoint1 interface{}
	DataPoint2 interface{}
	Confidence float64
}

// extractDataPointsWithTimestamps extracts data points with their timestamps from unified results
func (ce *CorrelationEngineImpl) extractDataPointsWithTimestamps(result *models.UnifiedResult, engine models.QueryType) []timeWindowDataPoint {
	var dataPoints []timeWindowDataPoint

	switch engine {
	case models.QueryTypeMetrics:
		dataPoints = ce.extractMetricsDataPoints(result)
	case models.QueryTypeLogs:
		dataPoints = ce.extractLogsDataPoints(result)
	case models.QueryTypeTraces:
		dataPoints = ce.extractTracesDataPoints(result)
	default:
		if ce.logger != nil {
			ce.logger.Warn("Unsupported engine for timestamp extraction", "engine", engine)
		}
	}

	return dataPoints
}

// timeWindowDataPoint represents a data point with timestamp
type timeWindowDataPoint struct {
	Timestamp time.Time
	Data      interface{}
}

// extractMetricsDataPoints extracts data points from metrics results
func (ce *CorrelationEngineImpl) extractMetricsDataPoints(result *models.UnifiedResult) []timeWindowDataPoint {
	var dataPoints []timeWindowDataPoint

	// Metrics data structure depends on VictoriaMetrics response format
	// This is a simplified implementation - in practice, we'd parse the actual metrics data
	if result.Data != nil {
		// For now, assume current time for metrics. Parsing of actual timestamps from
		// metrics data will be improved under action items in the tracker.
		dataPoints = append(dataPoints, timeWindowDataPoint{
			Timestamp: time.Now(),
			Data:      result.Data,
		})
	}

	return dataPoints
}

// extractLogsDataPoints extracts data points from logs results
func (ce *CorrelationEngineImpl) extractLogsDataPoints(result *models.UnifiedResult) []timeWindowDataPoint {
	var dataPoints []timeWindowDataPoint

	if logs, ok := result.Data.([]map[string]interface{}); ok {
		for _, log := range logs {
			// Try to extract timestamp from log entry
			timestamp := ce.extractTimestampFromLog(log)
			dataPoints = append(dataPoints, timeWindowDataPoint{
				Timestamp: timestamp,
				Data:      log,
			})
		}
	}

	return dataPoints
}

// extractTracesDataPoints extracts data points from traces results
func (ce *CorrelationEngineImpl) extractTracesDataPoints(result *models.UnifiedResult) []timeWindowDataPoint {
	var dataPoints []timeWindowDataPoint

	if traces, ok := result.Data.([]map[string]interface{}); ok {
		for _, trace := range traces {
			// Try to extract timestamp from trace
			timestamp := ce.extractTimestampFromTrace(trace)
			dataPoints = append(dataPoints, timeWindowDataPoint{
				Timestamp: timestamp,
				Data:      trace,
			})
		}
	}

	return dataPoints
}

// extractTimestampFromLog attempts to extract timestamp from a log entry
func (ce *CorrelationEngineImpl) extractTimestampFromLog(log map[string]interface{}) time.Time {
	// Try common timestamp fields
	timestampFields := []string{"timestamp", "@timestamp", "time", "ts"}

	for _, field := range timestampFields {
		if ts, exists := log[field]; exists {
			if t, err := ce.parseTimestamp(ts); err == nil {
				return t
			}
		}
	}

	// Default to current time if no timestamp found
	return time.Now()
}

// extractTimestampFromTrace attempts to extract timestamp from a trace
func (ce *CorrelationEngineImpl) extractTimestampFromTrace(trace map[string]interface{}) time.Time {
	// Try to extract from trace start time or spans
	if startTime, exists := trace["startTime"]; exists {
		if t, err := ce.parseTimestamp(startTime); err == nil {
			return t
		}
	}

	// Default to current time
	return time.Now()
}

// parseTimestamp attempts to parse various timestamp formats
func (ce *CorrelationEngineImpl) parseTimestamp(ts interface{}) (time.Time, error) {
	switch v := ts.(type) {
	case string:
		// Try RFC3339 first
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, nil
		}
		// Try Unix timestamp as string
		if t, err := time.Parse(time.UnixDate, v); err == nil {
			return t, nil
		}
	case int64:
		// Assume milliseconds since epoch
		return time.UnixMilli(v), nil
	case int:
		return time.UnixMilli(int64(v)), nil
	case float64:
		return time.UnixMilli(int64(v)), nil
	}

	return time.Time{}, fmt.Errorf("unsupported timestamp format: %T", ts)
}

// findTimeWindowCorrelations finds correlations between two sets of data points within a time window
func (ce *CorrelationEngineImpl) findTimeWindowCorrelations(
	dataPoints1, dataPoints2 []timeWindowDataPoint,
	timeWindow time.Duration,
) []timeWindowCorrelation {
	var correlations []timeWindowCorrelation

	// Sort data points by timestamp for efficient correlation
	// For simplicity, we'll do a nested loop (O(n*m) complexity)
	// In production, this should be optimized with sorting and binary search

	for _, dp1 := range dataPoints1 {
		for _, dp2 := range dataPoints2 {
			timeDiff := dp1.Timestamp.Sub(dp2.Timestamp)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}

			// Check if within time window
			if timeDiff <= timeWindow {
				// Calculate confidence based on time proximity
				confidence := ce.calculateTimeWindowConfidence(timeDiff, timeWindow)

				correlations = append(correlations, timeWindowCorrelation{
					Timestamp:  ce.calculateCorrelationTimestamp(dp1.Timestamp, dp2.Timestamp),
					DataPoint1: dp1.Data,
					DataPoint2: dp2.Data,
					Confidence: confidence,
				})
			}
		}
	}

	return correlations
}

// calculateTimeWindowConfidence calculates confidence based on time proximity
func (ce *CorrelationEngineImpl) calculateTimeWindowConfidence(timeDiff, timeWindow time.Duration) float64 {
	if timeWindow == 0 {
		return 1.0
	}

	// Higher confidence for closer timestamps
	proximityRatio := 1.0 - (timeDiff.Seconds() / timeWindow.Seconds())
	if proximityRatio < 0 {
		proximityRatio = 0
	}

	// Base confidence on proximity, with minimum threshold
	confidence := 0.5 + (proximityRatio * 0.4) // Range: 0.5 to 0.9
	if confidence > 0.95 {
		confidence = 0.95 // Cap at 0.95 to leave room for other factors
	}

	return confidence
}

// calculateCorrelationTimestamp calculates the representative timestamp for a correlation
func (ce *CorrelationEngineImpl) calculateCorrelationTimestamp(t1, t2 time.Time) time.Time {
	// Use the average of the two timestamps
	return t1.Add(t2.Sub(t1) / 2)
}

// dataLabels represents labels extracted from a data point
type dataLabels struct {
	Data   interface{}
	Labels map[string]string
}

// labelMatch represents a matching label between two data points
type labelMatch struct {
	Key    string
	Value  string
	Weight float64 // Importance weight for this label match
}

// extractLabelsFromResult extracts labels from a unified result
func (ce *CorrelationEngineImpl) extractLabelsFromResult(result *models.UnifiedResult, engine models.QueryType) []dataLabels {
	switch engine {
	case models.QueryTypeLogs:
		return ce.extractLabelsFromLogsResult(result)
	case models.QueryTypeTraces:
		return ce.extractLabelsFromTracesResult(result)
	case models.QueryTypeMetrics:
		return ce.extractLabelsFromMetricsResult(result)
	default:
		if ce.logger != nil {
			ce.logger.Warn("Unsupported engine for label extraction", "engine", engine)
		}
		return nil
	}
}

// extractLabelsFromLogsResult extracts labels from logs data
func (ce *CorrelationEngineImpl) extractLabelsFromLogsResult(result *models.UnifiedResult) []dataLabels {
	var labels []dataLabels

	// Build canonical->raw key lookup from EngineConfig.Labels. If not set,
	// fall back to reasonable defaults handled at config level.
	lcfg := ce.engineCfg.Labels

	if logs, ok := result.Data.([]map[string]interface{}); ok {
		for _, log := range logs {
			dl := dataLabels{Data: log, Labels: make(map[string]string)}

			// For each canonical label, consult configured raw keys in order and pick the first match
			tryKeys := func(canonical string, candidates []string) {
				for _, raw := range candidates {
					if v, ok := ce.lookupNestedKey(log, raw); ok && v != "" {
						dl.Labels[canonical] = v
						return
					}
				}
			}

			tryKeys("service", lcfg.Service)
			tryKeys("pod", lcfg.Pod)
			tryKeys("namespace", lcfg.Namespace)
			tryKeys("deployment", lcfg.Deployment)
			tryKeys("container", lcfg.Container)
			tryKeys("host", lcfg.Host)
			tryKeys("level", lcfg.Level)

			// If kubernetes nested map exists, prefer dotted paths listed in config; lookupNestedKey handles dots.

			labels = append(labels, dl)
		}
	}

	return labels
}

// extractLabelsFromTracesResult extracts labels from traces data
func (ce *CorrelationEngineImpl) extractLabelsFromTracesResult(result *models.UnifiedResult) []dataLabels {
	var labels []dataLabels

	if traces, ok := result.Data.([]map[string]interface{}); ok {
		for _, trace := range traces {
			dataLabels := dataLabels{
				Data:   trace,
				Labels: make(map[string]string),
			}

			// Extract service and operation
			if svc, ok := ce.getServiceLabelFromMap(trace); ok {
				dataLabels.Labels["service"] = svc
			}
			if operation, exists := trace["operationName"].(string); exists {
				dataLabels.Labels["operation"] = operation
			}

			// Some trace responses embed service info under processes (e.g., processes: {p1: {serviceName: "..."}})
			if procs, ok := trace["processes"].(map[string]interface{}); ok {
				for _, p := range procs {
					if pm, ok := p.(map[string]interface{}); ok {
						if svc, ok2 := ce.getServiceLabelFromMap(pm); ok2 {
							dataLabels.Labels["service"] = svc
							break
						}
					}
				}
			}

			// Try to extract operation name from first span if present
			if spans, ok := trace["spans"].([]interface{}); ok && len(spans) > 0 {
				if firstSpan, ok := spans[0].(map[string]interface{}); ok {
					if op, exists := firstSpan["operationName"].(string); exists {
						dataLabels.Labels["operation"] = op
					}
				}
			}

			// Extract tags
			if tags, exists := trace["tags"].(map[string]interface{}); exists {
				for key, value := range tags {
					if strValue, ok := value.(string); ok {
						dataLabels.Labels[key] = strValue
					}
				}
			}

			labels = append(labels, dataLabels)
		}
	}

	return labels
}

// extractLabelsFromMetricsResult extracts labels from metrics data
func (ce *CorrelationEngineImpl) extractLabelsFromMetricsResult(result *models.UnifiedResult) []dataLabels {
	var labels []dataLabels

	// Expect metrics adapter to supply Prometheus-compatible response structures
	// as produced by our test helpers: a map with "resultType" and "result" entries
	if result.Data == nil {
		return labels
	}

	if dataMap, ok := result.Data.(map[string]interface{}); ok {
		// result is typically []interface{} of series
		if resArr, exists := dataMap["result"]; exists {
			if seriesSlice, ok := resArr.([]interface{}); ok {
				for _, s := range seriesSlice {
					// Each series is expected to be a map with "metric" and values/points
					if seriesMap, ok := s.(map[string]interface{}); ok {
						dl := dataLabels{Data: seriesMap, Labels: make(map[string]string)}

						// metric may be map[string]string or map[string]interface{}
						if m, exists := seriesMap["metric"]; exists {
							switch mm := m.(type) {
							case map[string]string:
								for k, v := range mm {
									dl.Labels[k] = v
								}
							case map[string]interface{}:
								for k, v := range mm {
									if str, ok := v.(string); ok {
										dl.Labels[k] = str
									} else if bs, ok := v.([]byte); ok {
										dl.Labels[k] = string(bs)
									}
								}
							}
						}

						// Always include series id if present
						if name, exists := dl.Labels["__name__"]; exists && name != "" {
							dl.Labels["metric_name"] = name
						}

						labels = append(labels, dl)
					}
				}
			}
		}
	}

	return labels
}

// lookupNestedKey resolves dotted paths into nested maps (e.g. "foo.bar").
// It returns string value and true when found.
func (ce *CorrelationEngineImpl) lookupNestedKey(obj map[string]interface{}, path string) (string, bool) {
	if obj == nil || path == "" {
		return "", false
	}
	// simple key
	if v, ok := obj[path]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}

	// dotted path
	parts := strings.Split(path, ".")
	cur := interface{}(obj)
	for _, p := range parts {
		if m, ok := cur.(map[string]interface{}); ok {
			if next, exists := m[p]; exists {
				cur = next
				continue
			}
			return "", false
		}
		return "", false
	}
	if s, ok := cur.(string); ok {
		return s, true
	}
	return "", false
}

// getServiceLabelFromMap resolves the configured candidate raw keys for service
// into a canonical service string. It consults EngineConfig.Labels.Service in
// priority order and falls back to the canonical "service" key in the payload.
func (ce *CorrelationEngineImpl) getServiceLabelFromMap(obj map[string]interface{}) (string, bool) {
	if obj == nil {
		return "", false
	}
	lcfg := ce.engineCfg.Labels
	// Try configured candidate keys (may be dotted paths)
	for _, raw := range lcfg.Service {
		if v, ok := ce.lookupNestedKey(obj, raw); ok && v != "" {
			return v, true
		}
	}
	// Final attempt: canonical "service" key
	if v, ok := ce.lookupNestedKey(obj, "service"); ok && v != "" {
		return v, true
	}
	return "", false
}

// findLabelMatches finds matching labels between two sets of data labels
func (ce *CorrelationEngineImpl) findLabelMatches(labels1, labels2 []dataLabels) []labelMatch {
	var matches []labelMatch

	// Define label weights (importance for correlation)
	labelWeights := map[string]float64{
		"service":    1.0,
		"pod":        0.9,
		"namespace":  0.8,
		"deployment": 0.8,
		"container":  0.7,
		"operation":  0.8,
		"host":       0.6,
		"level":      0.3, // Less important for correlation
	}

	// For each data point in first set, find matches in second set
	for _, dl1 := range labels1 {
		for _, dl2 := range labels2 {
			for key1, value1 := range dl1.Labels {
				if value2, exists := dl2.Labels[key1]; exists && value1 == value2 {
					weight := labelWeights[key1]
					if weight == 0 {
						weight = 0.5 // Default weight for unknown labels
					}

					matches = append(matches, labelMatch{
						Key:    key1,
						Value:  value1,
						Weight: weight,
					})
				}
			}
		}
	}

	return matches
}

// calculateLabelMatchConfidence calculates confidence based on label matches
func (ce *CorrelationEngineImpl) calculateLabelMatchConfidence(matches []labelMatch) float64 {
	if len(matches) == 0 {
		return 0.0
	}

	// Calculate weighted confidence
	totalWeight := 0.0
	matchedWeight := 0.0

	// Define all possible important labels for normalization
	allImportantLabels := []string{"service", "pod", "namespace", "deployment", "container", "operation"}
	for _, label := range allImportantLabels {
		if weight, exists := map[string]float64{
			"service":    1.0,
			"pod":        0.9,
			"namespace":  0.8,
			"deployment": 0.8,
			"container":  0.7,
			"operation":  0.8,
		}[label]; exists {
			totalWeight += weight
		}
	}

	// Sum weights of matched labels
	for _, match := range matches {
		matchedWeight += match.Weight
	}

	// Calculate confidence as ratio of matched weight to total possible weight
	if totalWeight > 0 {
		confidence := matchedWeight / totalWeight
		// Cap at 0.95 and ensure minimum 0.6 for any matches
		if confidence > 0.95 {
			confidence = 0.95
		}
		if confidence < 0.6 {
			confidence = 0.6
		}
		return confidence
	}

	return 0.5 // Default confidence
}

// CorrelationResultMerger handles merging and deduplicating correlation results
type CorrelationResultMerger struct {
	logger logging.Logger
}

// NewCorrelationResultMerger creates a new result merger
func NewCorrelationResultMerger(logger logging.Logger) *CorrelationResultMerger {
	return &CorrelationResultMerger{
		logger: logger,
	}
}

// MergeResults merges and deduplicates correlation results
func (crm *CorrelationResultMerger) MergeResults(correlations []models.Correlation) []models.Correlation {
	mergeStart := time.Now()

	if len(correlations) == 0 {
		return correlations
	}

	// Group correlations by similar characteristics
	groups := crm.groupSimilarCorrelations(correlations)

	// Merge each group into a single correlation
	var merged []models.Correlation
	for _, group := range groups {
		merged = append(merged, crm.mergeCorrelationGroup(group))
	}

	mergeDuration := time.Since(mergeStart)
	// Record result merging duration (using "general" as correlation type since we don't have specific type here)
	monitoring.RecordCorrelationResultMergingDuration("general", len(correlations), mergeDuration)

	crm.logger.Info("Merged correlation results",
		"original_count", len(correlations),
		"merged_count", len(merged))

	return merged
}

// extractAverageFromMetricsResult tries to compute a lightweight average value
// from a MetricsQLQueryResult. It supports common Prometheus/VictoriaMetrics
// shapes (matrix with "values" or vector with "value"). Returns NaN when
// no numeric points are found.
func extractAverageFromMetricsResult(res *models.MetricsQLQueryResult) float64 {
	if res == nil || res.Data == nil {
		return math.NaN()
	}
	dm, ok := res.Data.(map[string]interface{})
	if !ok {
		return math.NaN()
	}
	var sum float64
	var count int

	if resultArr, exists := dm["result"]; exists {
		if seriesSlice, ok := resultArr.([]interface{}); ok {
			for _, s := range seriesSlice {
				if seriesMap, ok := s.(map[string]interface{}); ok {
					// try "values" (range vector)
					if values, ex := seriesMap["values"]; ex {
						if vslice, ok := values.([]interface{}); ok {
							for _, pair := range vslice {
								if p, ok := pair.([]interface{}); ok && len(p) >= 2 {
									valStr := fmt.Sprintf("%v", p[1])
									if f, err := strconv.ParseFloat(valStr, 64); err == nil {
										sum += f
										count++
									}
								}
							}
						}
						continue
					}

					// try single "value" (instant vector)
					if v, ex := seriesMap["value"]; ex {
						if pair, ok := v.([]interface{}); ok && len(pair) >= 2 {
							valStr := fmt.Sprintf("%v", pair[1])
							if f, err := strconv.ParseFloat(valStr, 64); err == nil {
								sum += f
								count++
							}
						}
					}
				}
			}
		}
	}

	if count == 0 {
		return math.NaN()
	}
	return sum / float64(count)
}

// groupSimilarCorrelations groups correlations that represent the same logical correlation
func (crm *CorrelationResultMerger) groupSimilarCorrelations(correlations []models.Correlation) [][]models.Correlation {
	var groups [][]models.Correlation

	for _, corr := range correlations {
		// Find existing group this correlation belongs to
		found := false
		for i, group := range groups {
			if crm.correlationsAreSimilar(corr, group[0]) {
				groups[i] = append(groups[i], corr)
				found = true
				break
			}
		}

		// Create new group if not found
		if !found {
			groups = append(groups, []models.Correlation{corr})
		}
	}

	return groups
}

// correlationsAreSimilar checks if two correlations represent the same logical event
func (crm *CorrelationResultMerger) correlationsAreSimilar(corr1, corr2 models.Correlation) bool {
	// Check if they involve the same engines
	if len(corr1.Engines) != len(corr2.Engines) {
		return false
	}

	for engine := range corr1.Engines {
		if _, exists := corr2.Engines[engine]; !exists {
			return false
		}
	}

	// Check if timestamps are close (within 1 minute for similarity)
	timeDiff := corr1.Timestamp.Sub(corr2.Timestamp)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > time.Minute {
		return false
	}

	// Check if confidence is similar
	confidenceDiff := corr1.Confidence - corr2.Confidence
	if confidenceDiff < 0 {
		confidenceDiff = -confidenceDiff
	}
	if confidenceDiff > 0.2 { // More than 20% difference
		return false
	}

	return true
}

// mergeCorrelationGroup merges a group of similar correlations into one
func (crm *CorrelationResultMerger) mergeCorrelationGroup(group []models.Correlation) models.Correlation {
	if len(group) == 0 {
		return models.Correlation{}
	}

	if len(group) == 1 {
		return group[0]
	}

	// Use the first correlation as base
	merged := group[0]

	// Merge timestamps (use average)
	totalTime := merged.Timestamp
	for i := 1; i < len(group); i++ {
		totalTime = totalTime.Add(group[i].Timestamp.Sub(merged.Timestamp))
	}
	merged.Timestamp = merged.Timestamp.Add(totalTime.Sub(merged.Timestamp) / time.Duration(len(group)))

	// Merge confidence (use average)
	totalConfidence := merged.Confidence
	for i := 1; i < len(group); i++ {
		totalConfidence += group[i].Confidence
	}
	merged.Confidence = totalConfidence / float64(len(group))

	// Merge data from all engines
	for i := 1; i < len(group); i++ {
		for engine, data := range group[i].Engines {
			if _, exists := merged.Engines[engine]; !exists {
				merged.Engines[engine] = data
			} else {
				// Merge data for same engine (combine into array if different)
				existing := merged.Engines[engine]
				if existingData, ok := existing.([]interface{}); ok {
					merged.Engines[engine] = append(existingData, data)
				} else {
					merged.Engines[engine] = []interface{}{existing, data}
				}
			}
		}
	}

	// Update metadata to indicate merging
	if merged.Metadata == nil {
		merged.Metadata = make(map[string]interface{})
	}
	merged.Metadata["merged_count"] = len(group)
	merged.Metadata["merge_timestamp"] = time.Now()

	return merged
}

// queryErrorSignals queries for error signals across logs, metrics, and traces
func (ce *CorrelationEngineImpl) queryErrorSignals(ctx context.Context, timeRange models.TimeRange) ([]models.FailureSignal, error) {
	var allSignals []models.FailureSignal

	// Query logs for errors
	logSignals, err := ce.queryErrorLogs(ctx, timeRange)
	if err != nil {
		if ce.logger != nil {
			ce.logger.Warn("Failed to query error logs", "error", err)
		}
	} else {
		allSignals = append(allSignals, logSignals...)
	}

	// Query traces for error spans
	traceSignals, err := ce.queryErrorTraces(ctx, timeRange)
	if err != nil {
		if ce.logger != nil {
			ce.logger.Warn("Failed to query error traces", "error", err)
		}
	} else {
		allSignals = append(allSignals, traceSignals...)
	}

	// Query metrics for error counters
	metricSignals, err := ce.queryErrorMetrics(ctx, timeRange)
	if err != nil {
		if ce.logger != nil {
			ce.logger.Warn("Failed to query error metrics", "error", err)
		}
	} else {
		allSignals = append(allSignals, metricSignals...)
	}

	return allSignals, nil
}

// queryErrorLogs queries logs for error signals
func (ce *CorrelationEngineImpl) queryErrorLogs(ctx context.Context, timeRange models.TimeRange) ([]models.FailureSignal, error) {
	if ce.logsService == nil {
		return nil, nil
	}

	// Query for logs with ERROR severity and failure-related attributes
	limit := ce.engineCfg.DefaultQueryLimit
	if limit <= 0 {
		limit = 10000
	}
	logsQuery := &models.LogsQLQueryRequest{
		Query: `severity:ERROR OR level:ERROR`,
		Start: timeRange.Start.UnixMilli(),
		End:   timeRange.End.UnixMilli(),
		Limit: limit, // configurable via engine.default_query_limit
	}

	result, err := ce.logsService.ExecuteQuery(ctx, logsQuery)
	if err != nil {
		return nil, err
	}

	var signals []models.FailureSignal
	for _, log := range result.Logs {
		signal := models.FailureSignal{
			Type:   "log",
			Engine: models.QueryTypeLogs,
			Data:   log,
		}

		// Extract timestamp
		if ts, exists := log["timestamp"]; exists {
			if t, err := ce.parseTimestamp(ts); err == nil {
				signal.Timestamp = t
			}
		}

		// Extract anomaly score if present
		if score, exists := log["iforest_anomaly_score"]; exists {
			if s, ok := score.(float64); ok {
				signal.AnomalyScore = &s
			}
		}

		signals = append(signals, signal)
	}

	return signals, nil
}

// queryErrorTraces queries traces for error spans
func (ce *CorrelationEngineImpl) queryErrorTraces(ctx context.Context, timeRange models.TimeRange) ([]models.FailureSignal, error) {
	if ce.tracesService == nil {
		return nil, nil
	}

	var signals []models.FailureSignal

	// First, try to search for traces with error-related tags
	searchRequests := []*models.TraceSearchRequest{
		{
			Start: models.FlexibleTime{Time: timeRange.Start},
			End:   models.FlexibleTime{Time: timeRange.End},
			Tags:  "error=true", // Search for traces with error=true tag
			Limit: 1000,         // Limit results for performance
		},
		{
			Start: models.FlexibleTime{Time: timeRange.Start},
			End:   models.FlexibleTime{Time: timeRange.End},
			Tags:  "error.status=error", // Search for traces with error.status=error tag
			Limit: 1000,
		},
		{
			Start: models.FlexibleTime{Time: timeRange.Start},
			End:   models.FlexibleTime{Time: timeRange.End},
			Tags:  "error", // Search for any traces containing error in tags
			Limit: 1000,
		},
	}

	// Also try searching for specific services that might have errors. Use configured candidates to avoid
	// hardcoding service names in engine logic.
	services := ce.engineCfg.ServiceCandidates
	for _, service := range services {
		searchRequests = append(searchRequests, &models.TraceSearchRequest{
			Service: service,
			Start:   models.FlexibleTime{Time: timeRange.Start},
			End:     models.FlexibleTime{Time: timeRange.End},
			Limit:   500,
		})
	}

	// Execute searches and collect results
	allTraces := make(map[string]map[string]interface{})
	for _, searchRequest := range searchRequests {
		result, err := ce.tracesService.SearchTraces(ctx, searchRequest)
		if err != nil {
			if ce.logger != nil {
				ce.logger.Warn("Failed to search traces", "error", err, "tags", searchRequest.Tags, "service", searchRequest.Service)
			}
			continue
		}

		// Collect unique traces
		for _, traceData := range result.Traces {
			if traceID, exists := traceData["traceId"].(string); exists {
				allTraces[traceID] = traceData
			}
		}
	}

	// Process collected traces for error spans
	for _, traceData := range allTraces {
		errorSpans := ce.extractErrorSpansFromTrace(traceData)
		for _, span := range errorSpans {
			signal := models.FailureSignal{
				Type:   "trace",
				Engine: models.QueryTypeTraces,
				Data:   span,
			}

			// Extract timestamp from span
			if startTime, exists := span["startTime"]; exists {
				if t, err := ce.parseTimestamp(startTime); err == nil {
					signal.Timestamp = t
				}
			}

			// Extract anomaly score if present
			if score, exists := span["iforest_anomaly_score"]; exists {
				if s, ok := score.(float64); ok {
					signal.AnomalyScore = &s
				} else if strScore, ok := score.(string); ok {
					if s, err := strconv.ParseFloat(strScore, 64); err == nil {
						signal.AnomalyScore = &s
					}
				}
			}

			signals = append(signals, signal)
		}
	}

	// If no error spans found through search, try fallback approach
	if len(signals) == 0 {
		if ce.logger != nil {
			ce.logger.Info("No error spans found through search, trying fallback approach")
		}
		return ce.queryErrorTracesFallback(ctx, timeRange)
	}

	return signals, nil
}

// extractErrorSpansFromTrace extracts spans with error status from a trace
func (ce *CorrelationEngineImpl) extractErrorSpansFromTrace(traceData map[string]interface{}) []map[string]interface{} {
	var errorSpans []map[string]interface{}

	// Check if spans exist in the trace
	spansInterface, exists := traceData["spans"]
	if !exists {
		return errorSpans
	}

	// Convert spans to slice of maps
	spans, ok := spansInterface.([]interface{})
	if !ok {
		return errorSpans
	}

	// Process each span
	for _, spanInterface := range spans {
		span, ok := spanInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if span has error status
		isError := false

		// Check tags for error indicators
		if tags, exists := span["tags"].(map[string]interface{}); exists {
			if errorTag, exists := tags["error"]; exists {
				if errorStr, ok := errorTag.(string); ok && errorStr == "true" {
					isError = true
				} else if errorBool, ok := errorTag.(bool); ok && errorBool {
					isError = true
				}
			}
			if errorStatus, exists := tags["error.status"]; exists {
				if statusStr, ok := errorStatus.(string); ok && statusStr == "error" {
					isError = true
				}
			}
		}

		// Check span status
		if status, exists := span["status"].(map[string]interface{}); exists {
			if statusCode, exists := status["code"]; exists {
				if codeStr, ok := statusCode.(string); ok && strings.ToLower(codeStr) == "error" {
					isError = true
				} else if codeInt, ok := statusCode.(int); ok && codeInt != 0 {
					isError = true
				}
			}
		}

		// If span is an error, add it to the results
		if isError {
			// Enrich span with trace-level information
			enrichedSpan := make(map[string]interface{})
			for k, v := range span {
				enrichedSpan[k] = v
			}

			// Add trace-level metadata
			if traceID, exists := traceData["traceId"]; exists {
				enrichedSpan["traceId"] = traceID
			}
			if svc, ok := ce.getServiceLabelFromMap(traceData); ok {
				enrichedSpan["service"] = svc
			}
			if operationName, exists := traceData["operationName"]; exists {
				enrichedSpan["operationName"] = operationName
			}

			errorSpans = append(errorSpans, enrichedSpan)
		}
	}

	return errorSpans
}

// queryErrorTracesFallback is the old implementation for services that don't support SearchTraces
func (ce *CorrelationEngineImpl) queryErrorTracesFallback(ctx context.Context, timeRange models.TimeRange) ([]models.FailureSignal, error) {
	// For traces, we can only use GetOperations, so we'll query for operations that might indicate errors
	// This is a simplified implementation - in practice, we'd need a more sophisticated traces service

	// NOTE(HCB-003): Discover services from KPI registry instead of hardcoding. Per AGENTS.md §3.6,
	// engines must not hardcode service names. Fallback to configured candidates if registry unavailable.
	var services []string
	if ce.kpiRepo != nil {
		kpis, _, err := ce.kpiRepo.ListKPIs(ctx, models.KPIListRequest{})
		if err == nil {
			// Extract unique service names from KPI metadata
			serviceSet := make(map[string]struct{})
			for _, kpi := range kpis {
				if kpi != nil && kpi.ServiceFamily != "" {
					serviceSet[kpi.ServiceFamily] = struct{}{}
				}
			}
			for svc := range serviceSet {
				services = append(services, svc)
			}
		} else if ce.logger != nil {
			ce.logger.Warn("KPI registry lookup failed in fallback traces; using config", "err", err)
		}
	}

	// If registry yielded no services, use configured candidates (may be empty per HCB-002)
	if len(services) == 0 {
		services = ce.engineCfg.ServiceCandidates
		if len(services) == 0 && ce.logger != nil {
			ce.logger.Warn("No services discovered from registry or config for trace fallback; returning empty")
			return nil, nil
		}
	}

	var signals []models.FailureSignal
	for _, service := range services {
		operations, err := ce.tracesService.GetOperations(ctx, service)
		if err != nil {
			continue // Skip services that fail
		}

		// Create signals for services that have operations (simplified approach)
		if len(operations) > 0 {
			signal := models.FailureSignal{
				Type:      "trace",
				Engine:    models.QueryTypeTraces,
				Timestamp: timeRange.Start, // Use start time as representative
				Data: map[string]interface{}{
					"service":    service,
					"operations": operations,
					"query":      fmt.Sprintf("service:%s", service),
				},
			}
			signals = append(signals, signal)
		}
	}

	return signals, nil
}

// queryErrorMetrics queries metrics for error counters
func (ce *CorrelationEngineImpl) queryErrorMetrics(ctx context.Context, timeRange models.TimeRange) ([]models.FailureSignal, error) {
	if ce.metricsService == nil {
		return nil, nil
	}

	// Build an instant metrics query from configured probes when available
	probes := ce.engineCfg.Probes
	queryExpr := "up == 0"
	if len(probes) > 0 {
		var parts []string
		for _, p := range probes {
			parts = append(parts, fmt.Sprintf("%s > 0", p))
		}
		queryExpr = queryExpr + " or " + strings.Join(parts, " or ")
	}
	metricsQuery := &models.MetricsQLQueryRequest{Query: queryExpr}

	result, err := ce.metricsService.ExecuteQuery(ctx, metricsQuery)
	if err != nil {
		return nil, err
	}

	var signals []models.FailureSignal
	// Parse metrics result and create signals for error metrics
	if result.Data != nil {
		signal := models.FailureSignal{
			Type:      "metric",
			Engine:    models.QueryTypeMetrics,
			Timestamp: time.Now(), // Use current time for instant query
			Data: map[string]interface{}{
				"metric_data": result.Data,
				"query":       metricsQuery.Query,
			},
		}
		signals = append(signals, signal)
	}

	return signals, nil
}

// queryErrorSignalsForTransactions queries error signals for specific transactions
func (ce *CorrelationEngineImpl) queryErrorSignalsForTransactions(ctx context.Context, transactionIDs []string, timeRange models.TimeRange) ([]models.FailureSignal, error) {
	var allSignals []models.FailureSignal

	// Build transaction ID filter
	txFilter := strings.Join(transactionIDs, " OR ")
	txQuery := fmt.Sprintf(`transaction_id:(%s)`, txFilter)

	// Query logs for specific transactions
	logsQuery := &models.LogsQLQueryRequest{
		Query: fmt.Sprintf(`(%s) AND (severity:ERROR OR level:ERROR)`, txQuery),
		Start: timeRange.Start.UnixMilli(),
		End:   timeRange.End.UnixMilli(),
		Limit: 5000,
	}

	if ce.logsService != nil {
		result, err := ce.logsService.ExecuteQuery(ctx, logsQuery)
		if err == nil {
			for _, log := range result.Logs {
				signal := models.FailureSignal{
					Type:   "log",
					Engine: models.QueryTypeLogs,
					Data:   log,
				}
				if ts, exists := log["timestamp"]; exists {
					if t, err := ce.parseTimestamp(ts); err == nil {
						signal.Timestamp = t
					}
				}
				allSignals = append(allSignals, signal)
			}
		}
	}

	return allSignals, nil
}

// groupSignalsByTransactionAndComponent groups signals by transaction ID and component
func (ce *CorrelationEngineImpl) groupSignalsByTransactionAndComponent(signals []models.FailureSignal, targetComponents []models.FailureComponent) map[string]map[models.FailureComponent][]models.FailureSignal {
	groups := make(map[string]map[models.FailureComponent][]models.FailureSignal)

	// Create component filter set
	componentFilter := make(map[models.FailureComponent]bool)
	for _, comp := range targetComponents {
		componentFilter[comp] = true
	}

	for _, signal := range signals {
		txID := ce.extractTransactionID(signal)
		component := ce.extractFailureComponent(signal)

		if txID == "" || component == "" {
			continue
		}

		// Filter by target components if specified
		if len(targetComponents) > 0 {
			if _, ok := componentFilter[models.FailureComponent(component)]; !ok {
				continue
			}
		}

		if groups[txID] == nil {
			groups[txID] = make(map[models.FailureComponent][]models.FailureSignal)
		}
		groups[txID][models.FailureComponent(component)] = append(groups[txID][models.FailureComponent(component)], signal)
	}

	return groups
}

// groupSignalsByTransaction groups signals by transaction ID
func (ce *CorrelationEngineImpl) groupSignalsByTransaction(signals []models.FailureSignal) map[string][]models.FailureSignal {
	groups := make(map[string][]models.FailureSignal)

	for _, signal := range signals {
		txID := ce.extractTransactionID(signal)
		if txID != "" {
			groups[txID] = append(groups[txID], signal)
		}
	}

	return groups
}

// extractTransactionID extracts transaction_id from a signal
func (ce *CorrelationEngineImpl) extractTransactionID(signal models.FailureSignal) string {
	if txID, exists := signal.Data["transaction_id"]; exists {
		if s, ok := txID.(string); ok {
			return s
		}
	}
	return ""
}

// extractFailureComponent extracts the failing component from a signal
func (ce *CorrelationEngineImpl) extractFailureComponent(signal models.FailureSignal) string {
	// Use configured label schema to extract service information. Do not hardcode
	// raw field names in the engine; consult EngineConfig.Labels instead.
	lcfg := ce.engineCfg.Labels

	// Try configured candidate keys for service (these may be dotted paths)
	for _, raw := range lcfg.Service {
		if v, ok := ce.lookupNestedKey(signal.Data, raw); ok && v != "" {
			return ce.mapServiceToComponent(v)
		}
	}

	// As a final attempt, fall back to canonical "service" key in the payload
	if v, ok := ce.lookupNestedKey(signal.Data, "service"); ok && v != "" {
		return ce.mapServiceToComponent(v)
	}

	// Check failure_mode if present
	if failureMode, exists := signal.Data["failure_mode"]; exists {
		if s, ok := failureMode.(string); ok {
			return s
		}
	}

	return ""
}

// mapServiceToComponent maps service names to failure components.
// NOTE(HCB-003): This is a legacy helper for failure detection. In practice, component
// mappings should come from KPI registry metadata (e.g., kpi.Tags or kpi.ComponentFamily).
// For now, we use a simple normalization that doesn't hardcode the full service list,
// just performs basic string cleanup. Proper fix: remove this function entirely and use
// KPI registry component metadata.
func (ce *CorrelationEngineImpl) mapServiceToComponent(service string) string {
	normalized := strings.ToLower(service)

	// Simple suffix stripping for common patterns (no hardcoded list)
	normalized = strings.TrimSuffix(normalized, "-client")
	normalized = strings.TrimSuffix(normalized, "-producer")
	normalized = strings.TrimSuffix(normalized, "-consumer")

	// Return normalized form; if you need true component mapping, use KPI registry
	return normalized
}

// createFailureIncident creates a failure incident from a group of signals
func (ce *CorrelationEngineImpl) createFailureIncident(group map[models.FailureComponent][]models.FailureSignal, timeRange models.TimeRange) *models.FailureIncident {
	if len(group) == 0 {
		return nil
	}

	// Find the component with the most signals (primary component)
	var primaryComponent models.FailureComponent
	maxSignals := 0
	var allSignals []models.FailureSignal
	var affectedTxIDs []string
	servicesInvolved := make(map[string]bool)
	failureModes := make(map[string]bool)
	var anomalyScores []float64

	for component, signals := range group {
		// Choose the component with the most signals. If there's a tie,
		// pick the component with the lexicographically smallest name for deterministic results.
		if len(signals) > maxSignals || (len(signals) == maxSignals && (primaryComponent == "" || string(component) < string(primaryComponent))) {
			maxSignals = len(signals)
			primaryComponent = component
		}
		allSignals = append(allSignals, signals...)

		// Extract transaction IDs and other metadata
		for _, signal := range signals {
			if txID := ce.extractTransactionID(signal); txID != "" {
				affectedTxIDs = append(affectedTxIDs, txID)
			}

			if service := ce.extractFailureComponent(signal); service != "" {
				servicesInvolved[service] = true
			}

			if failureMode, exists := signal.Data["failure_mode"]; exists {
				if s, ok := failureMode.(string); ok {
					failureModes[s] = true
				}
			}

			if signal.AnomalyScore != nil {
				anomalyScores = append(anomalyScores, *signal.AnomalyScore)
			}
		}
	}

	// Remove duplicates from transaction IDs
	affectedTxIDs = ce.removeDuplicates(affectedTxIDs)

	// Convert services map to slice
	var services []string
	for service := range servicesInvolved {
		services = append(services, service)
	}

	// Determine failure mode. Prefer modes present in the primary component's signals
	// (more likely to reflect the root cause). If none, fall back to the global most common mode.
	var failureMode string
	maxCount := 0

	// Count failure modes within primary component first.
	if primarySignals, ok := group[primaryComponent]; ok {
		pmCounts := make(map[string]int)
		for _, signal := range primarySignals {
			if fm, exists := signal.Data["failure_mode"]; exists {
				if s, ok := fm.(string); ok {
					pmCounts[s]++
				}
			}
		}
		for mode, count := range pmCounts {
			if count > maxCount || (count == maxCount && (failureMode == "" || mode < failureMode)) {
				maxCount = count
				failureMode = mode
			}
		}
	}

	// Fallback: global most common failure mode across all signals
	if failureMode == "" {
		for mode := range failureModes {
			// Count how many signals have this failure mode
			count := 0
			for _, signals := range group {
				for _, signal := range signals {
					if fm, exists := signal.Data["failure_mode"]; exists {
						if s, ok := fm.(string); ok && s == mode {
							count++
						}
					}
				}
			}
			if count > maxCount || (count == maxCount && (failureMode == "" || mode < failureMode)) {
				maxCount = count
				failureMode = mode
			}
		}
	}

	// Calculate average anomaly score
	var avgAnomalyScore float64
	if len(anomalyScores) > 0 {
		sum := 0.0
		for _, score := range anomalyScores {
			sum += score
		}
		avgAnomalyScore = sum / float64(len(anomalyScores))
	}

	// Determine severity based on signal count and anomaly score
	severity := ce.calculateSeverity(len(allSignals), avgAnomalyScore)

	// Calculate confidence based on signal consistency and anomaly detection
	confidence := ce.calculateFailureConfidence(allSignals, primaryComponent)

	return &models.FailureIncident{
		IncidentID:             fmt.Sprintf("incident_%s_%d", primaryComponent, time.Now().Unix()),
		TimeRange:              timeRange,
		PrimaryComponent:       primaryComponent,
		AffectedTransactionIDs: affectedTxIDs,
		ServicesInvolved:       services,
		FailureMode:            failureMode,
		Signals:                allSignals,
		AnomalyScore:           avgAnomalyScore,
		Severity:               severity,
		Confidence:             confidence,
	}
}

// createFailureIncidentForTransaction creates a failure incident for a specific transaction
func (ce *CorrelationEngineImpl) createFailureIncidentForTransaction(signals []models.FailureSignal, timeRange models.TimeRange) *models.FailureIncident {
	if len(signals) == 0 {
		return nil
	}

	// Extract transaction ID from first signal
	txID := ce.extractTransactionID(signals[0])
	if txID == "" {
		return nil
	}

	// Group signals by component
	componentGroups := make(map[models.FailureComponent][]models.FailureSignal)
	for _, signal := range signals {
		component := models.FailureComponent(ce.extractFailureComponent(signal))
		componentGroups[component] = append(componentGroups[component], signal)
	}

	// Use the same logic as createFailureIncident
	return ce.createFailureIncident(componentGroups, timeRange)
}

// calculateSeverity determines incident severity
func (ce *CorrelationEngineImpl) calculateSeverity(signalCount int, anomalyScore float64) string {
	if signalCount >= 10 || anomalyScore > 0.8 {
		return "critical"
	}
	if signalCount >= 5 || anomalyScore > 0.6 {
		return "high"
	}
	if signalCount >= 2 || anomalyScore > 0.4 {
		return "medium"
	}
	return "low"
}

// calculateFailureConfidence calculates confidence in the failure detection
func (ce *CorrelationEngineImpl) calculateFailureConfidence(signals []models.FailureSignal, primaryComponent models.FailureComponent) float64 {
	if len(signals) == 0 {
		return 0.0
	}

	// Base confidence on signal count
	baseConfidence := math.Min(float64(len(signals))*0.1, 0.5)

	// Boost confidence if we have anomaly scores
	anomalyCount := 0
	for _, signal := range signals {
		if signal.AnomalyScore != nil && *signal.AnomalyScore > 0.5 {
			anomalyCount++
		}
	}
	anomalyBoost := float64(anomalyCount) * 0.1

	// Boost confidence if signals are consistent (same component)
	consistentSignals := 0
	for _, signal := range signals {
		if models.FailureComponent(ce.extractFailureComponent(signal)) == primaryComponent {
			consistentSignals++
		}
	}
	consistencyBoost := float64(consistentSignals) / float64(len(signals)) * 0.3

	totalConfidence := baseConfidence + anomalyBoost + consistencyBoost
	return math.Min(totalConfidence, 0.95)
}

// createFailureSummary creates a summary of failure incidents
func (ce *CorrelationEngineImpl) createFailureSummary(incidents []models.FailureIncident, timeRange models.TimeRange) *models.FailureSummary {
	componentsAffected := make(map[models.FailureComponent]int)
	servicesInvolved := make(map[string]bool)
	failureModes := make(map[string]int)
	totalConfidence := 0.0
	anomalyDetected := false

	for _, incident := range incidents {
		componentsAffected[incident.PrimaryComponent]++
		for _, service := range incident.ServicesInvolved {
			servicesInvolved[service] = true
		}
		if incident.FailureMode != "" {
			failureModes[incident.FailureMode]++
		}
		totalConfidence += incident.Confidence
		if incident.AnomalyScore > 0.5 {
			anomalyDetected = true
		}
	}

	var services []string
	for service := range servicesInvolved {
		services = append(services, service)
	}

	avgConfidence := 0.0
	if len(incidents) > 0 {
		avgConfidence = totalConfidence / float64(len(incidents))
	}

	return &models.FailureSummary{
		TotalIncidents:     len(incidents),
		TimeRange:          timeRange,
		ComponentsAffected: componentsAffected,
		ServicesInvolved:   services,
		FailureModes:       failureModes,
		AverageConfidence:  avgConfidence,
		AnomalyDetected:    anomalyDetected,
	}
}

// removeDuplicates removes duplicate strings from a slice
func (ce *CorrelationEngineImpl) removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	return result
}

// startCorrelationSpan starts a tracing span for correlation operations
func (ce *CorrelationEngineImpl) startCorrelationSpan(ctx context.Context, query *models.CorrelationQuery) (context.Context, trace.Span) {
	if ce.tracer == nil {
		// Return no-op span if tracer is not configured
		return ctx, trace.SpanFromContext(ctx)
	}

	correlationType := "time_window"
	if query.TimeWindow == nil {
		correlationType = "label_based"
	}

	return ce.tracer.StartCorrelationSpan(ctx, query.ID, correlationType, len(query.Expressions))
}
