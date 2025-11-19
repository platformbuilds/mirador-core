package config

// LoadEnvironmentConfig loads environment-specific configuration
func LoadEnvironmentConfig(env string) (*Config, error) {
	base, err := Load()
	if err != nil {
		return nil, err
	}

	switch env {
	case "production":
		return applyProductionConfig(base), nil
	case "staging":
		return applyStagingConfig(base), nil
	case "development":
		return applyDevelopmentConfig(base), nil
	case "test":
		return applyTestConfig(base), nil
	default:
		return base, nil
	}
}

func applyProductionConfig(config *Config) *Config {
	// Production optimizations
	config.LogLevel = "warn"
	config.WebSocket.MaxConnections = 5000
	config.Cache.TTL = 600 // 10 minutes

	// Security hardening
	config.CORS.AllowedOrigins = []string{
		"https://mirador.company.com",
		"https://mirador-ui.company.com",
	}

	// Enable all integrations by default in production
	config.Integrations.Slack.Enabled = true
	config.Integrations.MSTeams.Enabled = true
	config.Integrations.Email.Enabled = true

	return config
}

func applyStagingConfig(config *Config) *Config {
	// Staging-specific settings
	config.LogLevel = "info"
	config.Cache.TTL = 300

	// Allow staging domains
	config.CORS.AllowedOrigins = []string{
		"https://mirador-staging.company.com",
		"http://localhost:3000", // For UI development
	}

	return config
}

func applyDevelopmentConfig(config *Config) *Config {
	// Development-friendly settings
	config.LogLevel = "debug"
	config.Cache.TTL = 60 // Shorter TTL for development

	// Permissive CORS for development
	config.CORS.AllowedOrigins = []string{"*"}

	// Disable external integrations by default in development
	config.Integrations.Slack.Enabled = false
	config.Integrations.MSTeams.Enabled = false
	config.Integrations.Email.Enabled = false

	return config
}

func applyTestConfig(config *Config) *Config {
	// Test-specific settings
	config.LogLevel = "error"
	config.Cache.TTL = 10 // Very short TTL for tests
	config.WebSocket.Enabled = false

	// Disable external integrations in tests
	config.Integrations.Slack.Enabled = false
	config.Integrations.MSTeams.Enabled = false
	config.Integrations.Email.Enabled = false

	// Use in-memory cache for tests
	config.Cache.Nodes = []string{"localhost:6380"} // Different port for test Redis

	return config
}
