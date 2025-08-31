package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()
	
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, 8080, config.Port)
	assert.Equal(t, "info", config.LogLevel)
	assert.True(t, config.WebSocket.Enabled)
	assert.True(t, config.Auth.RBAC.Enabled)
	assert.Equal(t, 300, config.Cache.TTL)
}

func TestEnvironmentVariableOverrides(t *testing.T) {
	// Set test environment variables
	os.Setenv("MIRADOR_PORT", "9090")
	os.Setenv("MIRADOR_LOG_LEVEL", "debug")
	os.Setenv("VM_ENDPOINTS", "http://vm1:8481,http://vm2:8481")
	defer func() {
		os.Unsetenv("MIRADOR_PORT")
		os.Unsetenv("MIRADOR_LOG_LEVEL")
		os.Unsetenv("VM_ENDPOINTS")
	}()

	config, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9090, config.Port)
	assert.Equal(t, "debug", config.LogLevel)
	assert.Contains(t, config.Database.VictoriaMetrics.Endpoints, "http://vm1:8481")
	assert.Contains(t, config.Database.VictoriaMetrics.Endpoints, "http://vm2:8481")
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modifier    func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			modifier: func(c *Config) {
				// No changes - default config should be valid
			},
			expectError: false,
		},
		{
			name: "invalid port",
			modifier: func(c *Config) {
				c.Port = 0
			},
			expectError: true,
			errorMsg:    "invalid port number",
		},
		{
			name: "missing VM endpoints",
			modifier: func(c *Config) {
				c.Database.VictoriaMetrics.Endpoints = []string{}
			},
			expectError: true,
			errorMsg:    "at least one VictoriaMetrics endpoint is required",
		},
		{
			name: "invalid correlation threshold",
			modifier: func(c *Config) {
				c.GRPC.RCAEngine.CorrelationThreshold = 1.5
			},
			expectError: true,
			errorMsg:    "correlation threshold must be between 0 and 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetDefaultConfig()
			tt.modifier(config)

			err := validateConfig(config)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFeatureFlags(t *testing.T) {
	config := GetDefaultConfig()
	
	// Test development flags
	config.Environment = "development"
	flags := config.GetFeatureFlags("test-tenant")
	assert.True(t, flags.AIInsights)
	assert.True(t, flags.PredictiveAlerting)
	assert.True(t, flags.CustomVisualizations)

	// Test production flags
	config.Environment = "production"
	flags = config.GetFeatureFlags("test-tenant")
	assert.True(t, flags.AIInsights)
	assert.False(t, flags.BetaUI) // Beta UI disabled in production
}

func TestConfigTemplateGeneration(t *testing.T) {
	template := GenerateConfigTemplate("production")
	assert.Contains(t, template, "environment: production")
	assert.Contains(t, template, "log_level: warn")
	assert.Contains(t, template, "enabled: true") // Integrations enabled
}

func TestConfigProfiles(t *testing.T) {
	profile, err := GetProfile("high-performance")
	require.NoError(t, err)
	assert.Equal(t, "high-performance", profile.Name)
	assert.Contains(t, profile.Tags, "performance")

	// Test applying profile
	config := GetDefaultConfig()
	err = config.ApplyProfile("high-performance")
	assert.NoError(t, err)
	assert.Equal(t, 120, config.Cache.TTL) // Should be updated from profile
}

func TestEndpointValidation(t *testing.T) {
	tests := []struct {
		endpoint    string
		expectError bool
	}{
		{"http://localhost:8481", false},
		{"https://vm-select.company.com:8481", false},
		{"invalid-url", true},
		{"ftp://invalid-scheme.com", true},
		{"http://", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			err := ValidateEndpoint(tt.endpoint)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGRPCEndpointValidation(t *testing.T) {
	tests := []struct {
		endpoint    string
		expectError bool
	}{
		{"localhost:9091", false},
		{"predict-engine.mirador:9091", false},
		{"192.168.1.100:9092", false},
		{"invalid-endpoint", true},
		{"localhost:999999", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			err := ValidateGRPCEndpoint(tt.endpoint)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
