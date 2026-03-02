package mariadb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/platformbuilds/mirador-core/internal/config"
)

func TestNewClient_NilConfig(t *testing.T) {
	logger := zap.NewNop()
	client, err := NewClient(nil, logger)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.False(t, client.IsEnabled())
	assert.False(t, client.IsConnected())
	assert.Nil(t, client.DB())
}

func TestNewClient_Disabled(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.MariaDBConfig{
		Enabled:  false,
		Host:     "localhost",
		Port:     3306,
		Database: "test",
		Username: "user",
		Password: "pass",
	}

	client, err := NewClient(cfg, logger)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.False(t, client.IsEnabled())
	assert.False(t, client.IsConnected())
}

func TestNewClient_EnabledButUnreachable(t *testing.T) {
	// This test verifies graceful degradation when DB is unreachable
	logger := zap.NewNop()
	cfg := &config.MariaDBConfig{
		Enabled:  true,
		Host:     "nonexistent.invalid",
		Port:     3306,
		Database: "test",
		Username: "user",
		Password: "pass",
	}

	client, err := NewClient(cfg, logger)

	// Should not return error - graceful degradation
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.True(t, client.IsEnabled())
	assert.False(t, client.IsConnected())
	assert.NotNil(t, client.LastError())
}

func TestClient_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.MariaDBConfig
		expected bool
	}{
		{
			name:     "nil config",
			cfg:      nil,
			expected: false,
		},
		{
			name:     "disabled",
			cfg:      &config.MariaDBConfig{Enabled: false},
			expected: false,
		},
		{
			name:     "enabled",
			cfg:      &config.MariaDBConfig{Enabled: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg, zap.NewNop())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, client.IsEnabled())
		})
	}
}

func TestClient_Ping_Disabled(t *testing.T) {
	client, err := NewClient(&config.MariaDBConfig{Enabled: false}, zap.NewNop())
	require.NoError(t, err)

	err = client.Ping(context.Background())
	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestClient_Ping_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		logger: zap.NewNop(),
		db:     nil,
	}

	err := client.Ping(context.Background())
	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestClient_Reconnect_Disabled(t *testing.T) {
	client, err := NewClient(&config.MariaDBConfig{Enabled: false}, zap.NewNop())
	require.NoError(t, err)

	err = client.Reconnect()
	assert.ErrorIs(t, err, ErrMariaDBDisabled)
}

func TestClient_Close_NilDB(t *testing.T) {
	client := &Client{
		logger: zap.NewNop(),
	}

	err := client.Close()
	assert.NoError(t, err)
}

func TestClient_HealthCheck_Disabled(t *testing.T) {
	client, err := NewClient(&config.MariaDBConfig{
		Enabled:  false,
		Host:     "localhost",
		Database: "test",
	}, zap.NewNop())
	require.NoError(t, err)

	health := client.HealthCheck(context.Background())

	assert.False(t, health.Enabled)
	assert.False(t, health.Connected)
	assert.Equal(t, "localhost", health.Host)
	assert.Equal(t, "test", health.Database)
	assert.Empty(t, health.Error)
}

func TestClient_HealthCheck_NotConnected(t *testing.T) {
	client := &Client{
		cfg: config.MariaDBConfig{
			Enabled:  true,
			Host:     "localhost",
			Database: "test",
		},
		logger: zap.NewNop(),
		db:     nil,
	}

	health := client.HealthCheck(context.Background())

	assert.True(t, health.Enabled)
	assert.False(t, health.Connected)
	assert.Contains(t, health.Error, "not connected")
}

func TestClient_Config_PasswordRedacted(t *testing.T) {
	client := &Client{
		cfg: config.MariaDBConfig{
			Enabled:  true,
			Host:     "localhost",
			Port:     3306,
			Database: "test",
			Username: "user",
			Password: "supersecret",
		},
		logger: zap.NewNop(),
	}

	cfg := client.Config()

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, "user", cfg.Username)
	assert.Equal(t, "***", cfg.Password)
}

func TestClient_LastError(t *testing.T) {
	client := &Client{
		logger: zap.NewNop(),
	}

	assert.Nil(t, client.LastError())

	// Set an error
	client.mu.Lock()
	client.lastError = ErrMariaDBNotConnected
	client.mu.Unlock()

	assert.Equal(t, ErrMariaDBNotConnected, client.LastError())
}

func TestClient_DB_ReturnsNilWhenNotConnected(t *testing.T) {
	client := &Client{
		logger: zap.NewNop(),
	}

	assert.Nil(t, client.DB())
}

func TestClient_IsConnected(t *testing.T) {
	client := &Client{
		logger: zap.NewNop(),
	}

	assert.False(t, client.IsConnected())

	client.mu.Lock()
	client.connected = true
	client.mu.Unlock()

	assert.True(t, client.IsConnected())
}

func TestDefaultConstants(t *testing.T) {
	// Verify default constants are reasonable
	assert.Equal(t, 10, defaultMaxOpenConns)
	assert.Equal(t, 5, defaultMaxIdleConns)
	assert.Equal(t, 5*time.Minute, defaultConnMaxLifetime)
	assert.Equal(t, 10*time.Second, defaultPingTimeout)
}

func TestStaticErrors(t *testing.T) {
	// Verify error messages
	assert.Contains(t, ErrMariaDBDisabled.Error(), "disabled")
	assert.Contains(t, ErrMariaDBNotConnected.Error(), "not connected")
	assert.Contains(t, ErrMariaDBPingFailed.Error(), "ping failed")
}
