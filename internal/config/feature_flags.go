type FeatureFlags struct {
	PredictiveAlerting    bool `mapstructure:"predictive_alerting" yaml:"predictive_alerting"`
	AdvancedRCA          bool `mapstructure:"advanced_rca" yaml:"advanced_rca"`
	AIInsights           bool `mapstructure:"ai_insights" yaml:"ai_insights"`
	RealtimeStreaming    bool `mapstructure:"realtime_streaming" yaml:"realtime_streaming"`
	CustomVisualizations bool `mapstructure:"custom_visualizations" yaml:"custom_visualizations"`
	ExportFeatures       bool `mapstructure:"export_features" yaml:"export_features"`
	BetaUI               bool `mapstructure:"beta_ui" yaml:"beta_ui"`
	AdvancedAuth         bool `mapstructure:"advanced_auth" yaml:"advanced_auth"`
}

// GetFeatureFlags returns feature flags for a tenant
func (c *Config) GetFeatureFlags(tenantID string) *FeatureFlags {
	// Default feature flags
	flags := &FeatureFlags{
		PredictiveAlerting:    true,
		AdvancedRCA:          true,
		AIInsights:           true,
		RealtimeStreaming:    true,
		CustomVisualizations: true,
		ExportFeatures:       true,
		BetaUI:               false,
		AdvancedAuth:         c.Auth.RBAC.Enabled,
	}

	// Environment-specific overrides
	switch c.Environment {
	case "production":
		flags.BetaUI = false
	case "staging":
		flags.BetaUI = true
	case "development":
		// All features enabled for development
	case "test":
		// Minimal features for testing
		flags.RealtimeStreaming = false
		flags.CustomVisualizations = false
	}

	// Tenant-specific overrides could be loaded from database
	// This would be implemented based on business requirements
	
	return flags
}
