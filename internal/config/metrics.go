import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ConfigReloads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mirador_core_config_reloads_total",
			Help: "Total number of configuration reloads",
		},
		[]string{"status"}, // success, error
	)

	ConfigValidationErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mirador_core_config_validation_errors_total",
			Help: "Total number of configuration validation errors",
		},
	)

	ActiveFeatureFlags = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mirador_core_feature_flags_active",
			Help: "Active feature flags by tenant",
		},
		[]string{"tenant_id", "feature"},
	)
)

// RecordConfigReload records a configuration reload event
func RecordConfigReload(success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	ConfigReloads.WithLabelValues(status).Inc()
}

// RecordValidationError records a configuration validation error
func RecordValidationError() {
	ConfigValidationErrors.Inc()
}

// UpdateFeatureFlagMetrics updates feature flag metrics for a tenant
func UpdateFeatureFlagMetrics(tenantID string, flags *FeatureFlags) {
	features := map[string]bool{
		"predictive_alerting":    flags.PredictiveAlerting,
		"advanced_rca":          flags.AdvancedRCA,
		"ai_insights":           flags.AIInsights,
		"realtime_streaming":    flags.RealtimeStreaming,
		"custom_visualizations": flags.CustomVisualizations,
		"export_features":       flags.ExportFeatures,
		"beta_ui":               flags.BetaUI,
		"advanced_auth":         flags.AdvancedAuth,
	}

	for feature, enabled := range features {
		value := 0.0
		if enabled {
			value = 1.0
		}
		ActiveFeatureFlags.WithLabelValues(tenantID, feature).Set(value)
	}
}
