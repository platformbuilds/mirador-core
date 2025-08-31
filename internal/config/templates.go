package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// GenerateConfigTemplate generates a configuration template based on environment
func GenerateConfigTemplate(environment string) string {
	template := `# MIRADOR-CORE Configuration
# Environment: %s
# Generated: %s

environment: %s
port: 8080
log_level: %s

# VictoriaMetrics Ecosystem
database:
  victoria_metrics:
    endpoints:
      - "http://vm-select:8481"
    timeout: 30000
  victoria_logs:
    endpoints:
      - "http://vl-select:9428"
    timeout: 30000
  victoria_traces:
    endpoints:
      - "http://vt-select:10428"
    timeout: 30000

# AI Engines
grpc:
  predict_engine:
    endpoint: "predict-engine:9091"
    models: ["isolation_forest", "lstm_trend", "anomaly_detector"]
    timeout: 30000
  rca_engine:
    endpoint: "rca-engine:9092"
    correlation_threshold: 0.85
    timeout: 30000
  alert_engine:
    endpoint: "alert-engine:9093"
    timeout: 30000

# Authentication
auth:
  ldap:
    url: "ldap://ldap.company.com"
    base_dn: "dc=company,dc=com"
    enabled: %t
  rbac:
    enabled: true
    admin_role: "mirador-admin"
  jwt:
    expiry_minutes: %d

# Valkey Cluster Caching
cache:
  nodes:
    - "redis-cluster:6379"
  ttl: %d

# External Integrations
integrations:
  slack:
    channel: "#mirador-alerts"
    enabled: %t
  ms_teams:
    enabled: %t
  email:
    smtp_host: "smtp.company.com"
    smtp_port: 587
    enabled: %t

# WebSocket Streaming
websocket:
  enabled: true
  max_connections: %d
  ping_interval: 30
`

	var logLevel string
	var ldapEnabled, integrationsEnabled bool
	var jwtExpiry, cacheTTL, maxConnections int

	switch environment {
	case "production":
		logLevel = "warn"
		ldapEnabled = true
		integrationsEnabled = true
		jwtExpiry = 480 // 8 hours
		cacheTTL = 600  // 10 minutes
		maxConnections = 5000
	case "staging":
		logLevel = "info"
		ldapEnabled = true
		integrationsEnabled = false
		jwtExpiry = 720 // 12 hours
		cacheTTL = 300  // 5 minutes
		maxConnections = 1000
	case "development":
		logLevel = "debug"
		ldapEnabled = false
		integrationsEnabled = false
		jwtExpiry = 1440 // 24 hours
		cacheTTL = 60    // 1 minute
		maxConnections = 100
	default:
		logLevel = "info"
		ldapEnabled = false
		integrationsEnabled = false
		jwtExpiry = 1440
		cacheTTL = 300
		maxConnections = 1000
	}

	return fmt.Sprintf(template,
		environment,
		time.Now().Format(time.RFC3339),
		environment,
		logLevel,
		ldapEnabled,
		jwtExpiry,
		cacheTTL,
		integrationsEnabled,
		integrationsEnabled,
		integrationsEnabled,
		maxConnections,
	)
}

// SaveConfigTemplate saves a configuration template to file
func SaveConfigTemplate(environment, filepath string) error {
	template := GenerateConfigTemplate(environment)
	return os.WriteFile(filepath, []byte(template), 0644)
}

// ================================
// internal/config/profile.go - Configuration Profiles
// ================================

type ConfigProfile struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Settings    map[string]interface{} `yaml:"settings"`
	Tags        []string               `yaml:"tags"`
	CreatedAt   time.Time              `yaml:"created_at"`
}

// GetProfile returns a predefined configuration profile
func GetProfile(profileName string) (*ConfigProfile, error) {
	profiles := map[string]*ConfigProfile{
		"high-performance": {
			Name:        "high-performance",
			Description: "Optimized for high-throughput workloads",
			Settings: map[string]interface{}{
				"cache.ttl":                         120,
				"websocket.max_connections":         10000,
				"database.victoria_metrics.timeout": 10000,
				"grpc.predict_engine.timeout":       10000,
			},
			Tags:      []string{"performance", "high-load"},
			CreatedAt: time.Now(),
		},
		"security-focused": {
			Name:        "security-focused",
			Description: "Enhanced security configuration",
			Settings: map[string]interface{}{
				"auth.rbac.enabled":       true,
				"auth.ldap.enabled":       true,
				"auth.jwt.expiry_minutes": 240, // 4 hours
				"cors.allowed_origins":    []string{"https://mirador.company.com"},
				"cors.allow_credentials":  true,
			},
			Tags:      []string{"security", "enterprise"},
			CreatedAt: time.Now(),
		},
		"minimal": {
			Name:        "minimal",
			Description: "Minimal configuration for small deployments",
			Settings: map[string]interface{}{
				"websocket.enabled":             false,
				"monitoring.tracing_enabled":    false,
				"integrations.slack.enabled":    false,
				"integrations.ms_teams.enabled": false,
				"integrations.email.enabled":    false,
			},
			Tags:      []string{"minimal", "small-scale"},
			CreatedAt: time.Now(),
		},
		"ai-enhanced": {
			Name:        "ai-enhanced",
			Description: "Full AI capabilities enabled",
			Settings: map[string]interface{}{
				"grpc.predict_engine.models": []string{
					"isolation_forest", "lstm_trend", "anomaly_detector",
					"ensemble_predictor", "seasonal_decomposition", "prophet_forecaster",
				},
				"grpc.rca_engine.correlation_threshold": 0.9,
			},
			Tags:      []string{"ai", "ml", "advanced"},
			CreatedAt: time.Now(),
		},
	}

	profile, exists := profiles[profileName]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", profileName)
	}

	return profile, nil
}

// ApplyProfile applies a configuration profile to the current config
func (c *Config) ApplyProfile(profileName string) error {
	profile, err := GetProfile(profileName)
	if err != nil {
		return err
	}

	// Apply profile settings using reflection or type switching
	for key, value := range profile.Settings {
		if err := applyConfigSetting(c, key, value); err != nil {
			return fmt.Errorf("failed to apply setting %s: %w", key, err)
		}
	}

	return nil
}

// applyConfigSetting applies a single configuration setting
func applyConfigSetting(config *Config, key string, value interface{}) error {
	parts := strings.Split(key, ".")

	switch parts[0] {
	case "cache":
		if len(parts) > 1 {
			switch parts[1] {
			case "ttl":
				if ttl, ok := value.(int); ok {
					config.Cache.TTL = ttl
				}
			}
		}
	case "websocket":
		if len(parts) > 1 {
			switch parts[1] {
			case "enabled":
				if enabled, ok := value.(bool); ok {
					config.WebSocket.Enabled = enabled
				}
			case "max_connections":
				if maxConn, ok := value.(int); ok {
					config.WebSocket.MaxConnections = maxConn
				}
			}
		}
	case "auth":
		if len(parts) > 2 {
			switch parts[1] {
			case "jwt":
				if parts[2] == "expiry_minutes" {
					if expiry, ok := value.(int); ok {
						config.Auth.JWT.ExpiryMin = expiry
					}
				}
			case "rbac":
				if parts[2] == "enabled" {
					if enabled, ok := value.(bool); ok {
						config.Auth.RBAC.Enabled = enabled
					}
				}
			}
		}
		// Add more cases as needed
	}

	return nil
}
