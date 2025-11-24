package api

import (
	"context"
	"time"

	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/rca"
	"github.com/platformbuilds/mirador-core/internal/services"
)

// correlationAnomalyProvider adapts services.CorrelationEngine to the
// rca.AnomalyEventsProvider interface by running a failure-detection query
// over the requested time range and mapping returned signals to AnomalyEvent.
type correlationAnomalyProvider struct {
	ce     services.CorrelationEngine
	logger logging.Logger
}

//nolint:gocyclo // Signal filtering and conversion logic is inherently complex
func (p *correlationAnomalyProvider) GetAnomalies(ctx context.Context, startTime time.Time, endTime time.Time, servicesFilter []string) ([]*rca.AnomalyEvent, error) {
	tr := models.TimeRange{Start: startTime, End: endTime}

	result, err := p.ce.DetectComponentFailures(ctx, tr, nil)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn("correlationAnomalyProvider: DetectComponentFailures failed", "error", err)
		}
		return nil, err
	}

	var anomalies []*rca.AnomalyEvent

	for _, incident := range result.Incidents {
		for _, sig := range incident.Signals {
			if len(servicesFilter) > 0 {
				matched := false
				for _, s := range servicesFilter {
					if svc, ok := sig.Data["service"].(string); ok && svc == s {
						matched = true
						break
					}
					if svc, ok := sig.Data["service.name"].(string); ok && svc == s {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}

			svcName := ""
			if v, ok := sig.Data["service"].(string); ok {
				svcName = v
			} else if v, ok := sig.Data["service.name"].(string); ok {
				svcName = v
			}

			var st rca.SignalType
			switch sig.Type {
			case "log":
				st = rca.SignalTypeLogs
			case "trace":
				st = rca.SignalTypeTraces
			default:
				st = rca.SignalTypeMetrics
			}

			ae := rca.NewAnomalyEvent(svcName, "component", st)
			ae.Timestamp = sig.Timestamp
			ae.SourceType = string(sig.Engine)

			if sig.AnomalyScore != nil {
				ae.IForestScore = sig.AnomalyScore
				ae.AnomalyScore = *sig.AnomalyScore
				if *sig.AnomalyScore > 0.8 {
					ae.Severity = rca.SeverityCritical
				} else if *sig.AnomalyScore > 0.6 {
					ae.Severity = rca.SeverityHigh
				} else if *sig.AnomalyScore > 0.4 {
					ae.Severity = rca.SeverityMedium
				} else {
					ae.Severity = rca.SeverityLow
				}
				ae.Confidence = *sig.AnomalyScore
			}

			for k, v := range sig.Data {
				if str, ok := v.(string); ok {
					ae.Tags[k] = str
				}
			}

			anomalies = append(anomalies, ae)
		}
	}

	return anomalies, nil
}
