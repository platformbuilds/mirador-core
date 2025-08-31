package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Load loads configuration from various sources with priority order:
// 1. Environment variables
// 2. Configuration file (config.yaml)
// 3. Default values
func Load() (*Config, error) {
	// Initialize Viper
	v := viper.New()

	// Set configuration file details
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/mirador/")
	v.AddConfigPath("./configs/")
	v.AddConfigPath(".")

	// Enable environment variable support
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix("MIRADOR")

	// Set default values
	setDefaults(v)

	// Read configuration file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found - continue with env vars and defaults
	}

	// Override with environment variables
	overrideWithEnvVars(v)

	// Unmarshal to config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets reasonable default values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("environment", "development")
	v.SetDefault("port", 8080)
	v.SetDefault("log_level", "info")

	// Database defaults
	v.SetDefault("database.victoria_metrics.endpoints", []string{"http://localhost:8481"})
	v.SetDefault("database.victoria_metrics.timeout", 30000)
	v.SetDefault("database.victoria_logs.endpoints", []string{"http://localhost:9428"})
	v.SetDefault("database.victoria_logs.timeout", 30000)
	v.SetDefault("database.victoria_traces.endpoints", []string{"http://localhost:10428"})
	v.SetDefault("database.victoria_traces.timeout", 30000)

	// gRPC defaults
	v.SetDefault("grpc.predict_engine.endpoint", "localhost:9091")
	v.SetDefault("grpc.predict_engine.models", []string{"isolation_forest", "lstm_trend", "anomaly_detector"})
	v.SetDefault("grpc.predict_engine.timeout", 30000)
	v.SetDefault("grpc.rca_engine.endpoint", "localhost:9092")
	v.SetDefault("grpc.rca_engine.correlation_threshold", 0.85)
	v.SetDefault("grpc.rca_engine.timeout", 30000)
	v.SetDefault("grpc.alert_engine.endpoint", "localhost:9093")
	v.SetDefault("grpc.alert_engine.rules_path", "/etc/mirador/alert-rules.yaml")
	v.SetDefault("grpc.alert_engine.timeout", 30000)

	// Auth defaults
	v.SetDefault("auth.ldap.enabled", false)
	v.SetDefault("auth.oauth.enabled", false)
	v.SetDefault("auth.rbac.enabled", true)
	v.SetDefault("auth.rbac.admin_role", "mirador-admin")
	v.SetDefault("auth.jwt.expiry_minutes", 1440) // 24 hours

	// Cache defaults (Valkey cluster)
	v.SetDefault("cache.nodes", []string{"localhost:6379"})
	v.SetDefault("cache.ttl", 300) // 5 minutes
	v.SetDefault("cache.db", 0)

	// CORS defaults
	v.SetDefault("cors.allowed_origins", []string{"*"})
	v.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allowed_headers", []string{"Content-Type", "Authorization", "X-Tenant-ID"})
	v.SetDefault("cors.exposed_headers", []string{"X-Cache", "X-Rate-Limit-Remaining"})
	v.SetDefault("cors.allow_credentials", true)
	v.SetDefault("cors.max_age", 3600)

	// Integrations defaults
	v.SetDefault("integrations.slack.enabled", false)
	v.SetDefault("integrations.ms_teams.enabled", false)
	v.SetDefault("integrations.email.enabled", false)
	v.SetDefault("integrations.email.smtp_port", 587)

	// WebSocket defaults
	v.SetDefault("websocket.enabled", true)
	v.SetDefault("websocket.max_connections", 1000)
	v.SetDefault("websocket.read_buffer_size", 1024)
	v.SetDefault("websocket.write_buffer_size", 1024)
	v.SetDefault("websocket.ping_interval", 30)
	v.SetDefault("websocket.max_message_size", 1048576) // 1MB

	// Monitoring defaults
	v.SetDefault("monitoring.enabled", true)
	v.SetDefault("monitoring.metrics_path", "/metrics")
	v.SetDefault("monitoring.prometheus_enabled", true)
	v.SetDefault("monitoring.tracing_enabled", false)
}

// overrideWithEnvVars explicitly handles environment variable overrides
func overrideWithEnvVars(v *viper.Viper) {
	// Server configuration
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			v.Set("port", p)
		}
	}

	if env := os.Getenv("ENVIRONMENT"); env != "" {
		v.Set("environment", env)
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		v.Set("log_level", logLevel)
	}

	// VictoriaMetrics endpoints
	if vmEndpoints := os.Getenv("VM_ENDPOINTS"); vmEndpoints != "" {
		endpoints := strings.Split(vmEndpoints, ",")
		for i, endpoint := range endpoints {
			endpoints[i] = strings.TrimSpace(endpoint)
		}
		v.Set("database.victoria_metrics.endpoints", endpoints)
	}

	// VictoriaLogs endpoints
	if vlEndpoints := os.Getenv("VL_ENDPOINTS"); vlEndpoints != "" {
		endpoints := strings.Split(vlEndpoints, ",")
		for i, endpoint := range endpoints {
			endpoints[i] = strings.TrimSpace(endpoint)
		}
		v.Set("database.victoria_logs.endpoints", endpoints)
	}

	// VictoriaTraces endpoints
	if vtEndpoints := os.Getenv("VT_ENDPOINTS"); vtEndpoints != "" {
		endpoints := strings.Split(vtEndpoints, ",")
		for i, endpoint := range endpoints {
			endpoints[i] = strings.TrimSpace(endpoint)
		}
		v.Set("database.victoria_traces.endpoints", endpoints)
	}

	// AI Engine gRPC endpoints
	if predictGRPC := os.Getenv("PREDICT_ENGINE_GRPC"); predictGRPC != "" {
		v.Set("grpc.predict_engine.endpoint", predictGRPC)
	}

	if rcaGRPC := os.Getenv("RCA_ENGINE_GRPC"); rcaGRPC != "" {
		v.Set("grpc.rca_engine.endpoint", rcaGRPC)
	}

	if alertGRPC := os.Getenv("ALERT_ENGINE_GRPC"); alertGRPC != "" {
		v.Set("grpc.alert_engine.endpoint", alertGRPC)
	}

	// Valkey cluster cache nodes
	if cacheNodes := os.Getenv("VALLEY_CACHE_NODES"); cacheNodes != "" {
		nodes := strings.Split(cacheNodes, ",")
		for i, node := range nodes {
			nodes[i] = strings.TrimSpace(node)
		}
		v.Set("cache.nodes", nodes)
	}

	if cacheTTL := os.Getenv("CACHE_TTL"); cacheTTL != "" {
		if ttl, err := strconv.Atoi(cacheTTL); err == nil {
			v.Set("cache.ttl", ttl)
		}
	}

	// Authentication
	if ldapURL := os.Getenv("LDAP_URL"); ldapURL != "" {
		v.Set("auth.ldap.url", ldapURL)
		v.Set("auth.ldap.enabled", true)
	}

	if ldapBaseDN := os.Getenv("LDAP_BASE_DN"); ldapBaseDN != "" {
		v.Set("auth.ldap.base_dn", ldapBaseDN)
	}

	if rbacEnabled := os.Getenv("RBAC_ENABLED"); rbacEnabled != "" {
		if enabled, err := strconv.ParseBool(rbacEnabled); err == nil {
			v.Set("auth.rbac.enabled", enabled)
		}
	}

	// JWT configuration
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		v.Set("auth.jwt.secret", jwtSecret)
	}

	// External integrations
	if slackWebhook := os.Getenv("SLACK_WEBHOOK_URL"); slackWebhook != "" {
		v.Set("integrations.slack.webhook_url", slackWebhook)
		v.Set("integrations.slack.enabled", true)
	}

	if teamsWebhook := os.Getenv("TEAMS_WEBHOOK_URL"); teamsWebhook != "" {
		v.Set("integrations.ms_teams.webhook_url", teamsWebhook)
		v.Set("integrations.ms_teams.enabled", true)
	}

	if smtpHost := os.Getenv("SMTP_HOST"); smtpHost != "" {
		v.Set("integrations.email.smtp_host", smtpHost)
		v.Set("integrations.email.enabled", true)
	}
}

// validateConfig validates the loaded configuration
func validateConfig(config *Config) error {
	// Validate required fields
	if len(config.Database.VictoriaMetrics.Endpoints) == 0 {
		return fmt.Errorf("at least one VictoriaMetrics endpoint is required")
	}

	if len(config.Database.VictoriaLogs.Endpoints) == 0 {
		return fmt.Errorf("at least one VictoriaLogs endpoint is required")
	}

	if len(config.Cache.Nodes) == 0 {
		return fmt.Errorf("at least one Valkey cluster cache node is required")
	}

	// Validate gRPC endpoints
	if config.GRPC.PredictEngine.Endpoint == "" {
		return fmt.Errorf("PREDICT-ENGINE gRPC endpoint is required")
	}

	if config.GRPC.RCAEngine.Endpoint == "" {
		return fmt.Errorf("RCA-ENGINE gRPC endpoint is required")
	}

	if config.GRPC.AlertEngine.Endpoint == "" {
		return fmt.Errorf("ALERT-ENGINE gRPC endpoint is required")
	}

	// Validate port range
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", config.Port)
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLogLevels, config.LogLevel) {
		return fmt.Errorf("invalid log level: %s", config.LogLevel)
	}

	// Validate environment
	validEnvironments := []string{"development", "staging", "production", "test"}
	if !contains(validEnvironments, config.Environment) {
		return fmt.Errorf("invalid environment: %s", config.Environment)
	}

	// Validate cache TTL
	if config.Cache.TTL < 1 {
		return fmt.Errorf("cache TTL must be at least 1 second")
	}

	// Validate JWT configuration if authentication is enabled
	if config.Auth.RBAC.Enabled && config.Auth.JWT.Secret == "" {
		return fmt.Errorf("JWT secret is required when RBAC is enabled")
	}

	// Validate correlation threshold
	if config.GRPC.RCAEngine.CorrelationThreshold < 0 || config.GRPC.RCAEngine.CorrelationThreshold > 1 {
		return fmt.Errorf("RCA correlation threshold must be between 0 and 1")
	}

	return nil
}
