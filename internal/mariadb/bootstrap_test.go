package mariadb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/mirastacklabs-ai/mirador-core/internal/config"
)

func TestNewBootstrap(t *testing.T) {
	client := &Client{logger: zap.NewNop()}
	cfg := &config.Config{}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{
		CreateTablesIfMissing:     true,
		SyncDataSourcesFromConfig: true,
	}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	assert.NotNil(t, bootstrap)
	assert.Equal(t, client, bootstrap.client)
	assert.Equal(t, cfg, bootstrap.cfg)
	assert.Equal(t, logger, bootstrap.logger)
	assert.True(t, bootstrap.bootstrapCfg.CreateTablesIfMissing)
	assert.True(t, bootstrap.bootstrapCfg.SyncDataSourcesFromConfig)
}

func TestBootstrap_Run_Disabled(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	cfg := &config.Config{}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{
		CreateTablesIfMissing:     true,
		SyncDataSourcesFromConfig: true,
	}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	err := bootstrap.Run(context.Background())

	// Should succeed without doing anything when disabled
	assert.NoError(t, err)
}

func TestBootstrap_Run_NotConnected(t *testing.T) {
	client := &Client{
		cfg:       config.MariaDBConfig{Enabled: true, Host: "nonexistent.invalid"},
		db:        nil,
		connected: false,
		logger:    zap.NewNop(),
	}
	cfg := &config.Config{}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{
		CreateTablesIfMissing:     true,
		SyncDataSourcesFromConfig: true,
	}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	err := bootstrap.Run(context.Background())

	// Should fail because it can't reconnect
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect")
}

func TestBootstrapConfig_Defaults(t *testing.T) {
	// Test zero value defaults
	cfg := BootstrapConfig{}

	assert.False(t, cfg.CreateTablesIfMissing)
	assert.False(t, cfg.SyncDataSourcesFromConfig)
}

func TestBootstrapConfig_FullyEnabled(t *testing.T) {
	cfg := BootstrapConfig{
		CreateTablesIfMissing:     true,
		SyncDataSourcesFromConfig: true,
	}

	assert.True(t, cfg.CreateTablesIfMissing)
	assert.True(t, cfg.SyncDataSourcesFromConfig)
}

func TestBootstrap_NothingEnabled(t *testing.T) {
	// When client is disabled but bootstrap has options enabled,
	// it should just skip without error
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: false},
		logger: zap.NewNop(),
	}
	cfg := &config.Config{}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{
		CreateTablesIfMissing:     false,
		SyncDataSourcesFromConfig: false,
	}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	err := bootstrap.Run(context.Background())
	assert.NoError(t, err)
}

func TestConfigDataSource_Fields(t *testing.T) {
	// Test ConfigDataSource struct
	ds := ConfigDataSource{
		URL:      "http://localhost:8428",
		Type:     DataSourceTypePrometheus,
		Name:     "Test VM",
		Username: "admin",
		Password: "secret",
	}

	assert.Equal(t, "http://localhost:8428", ds.URL)
	assert.Equal(t, DataSourceTypePrometheus, ds.Type)
	assert.Equal(t, "Test VM", ds.Name)
	assert.Equal(t, "admin", ds.Username)
	assert.Equal(t, "secret", ds.Password)
}

func TestBootstrap_createTablesIfMissing_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil, // Not connected
		logger: zap.NewNop(),
	}
	cfg := &config.Config{}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	err := bootstrap.createTablesIfMissing(context.Background())
	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

func TestBootstrap_collectDataSourcesFromConfig(t *testing.T) {
	client := &Client{logger: zap.NewNop()}
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			VictoriaMetrics: config.VictoriaMetricsConfig{
				Endpoints: []string{
					"http://vm1:8428",
					"http://vm2:8428",
				},
			},
			VictoriaLogs: config.VictoriaLogsConfig{
				Endpoints: []string{
					"http://vl:9428",
				},
			},
		},
	}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	dataSources := bootstrap.collectDataSourcesFromConfig()

	// Should have 3 data sources (2 VM + 1 VL)
	assert.Len(t, dataSources, 3)

	// Verify VM endpoints
	vmCount := 0
	vlCount := 0
	for _, ds := range dataSources {
		if ds.Type == DataSourceTypePrometheus {
			vmCount++
			assert.Contains(t, ds.URL, "8428")
		}
		if ds.Type == DataSourceTypeVictoriaLogs {
			vlCount++
			assert.Contains(t, ds.URL, "9428")
		}
	}
	assert.Equal(t, 2, vmCount)
	assert.Equal(t, 1, vlCount)
}

func TestBootstrap_collectDataSourcesFromConfig_Empty(t *testing.T) {
	client := &Client{logger: zap.NewNop()}
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			VictoriaMetrics: config.VictoriaMetricsConfig{
				Endpoints: []string{},
			},
		},
	}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	dataSources := bootstrap.collectDataSourcesFromConfig()

	assert.Empty(t, dataSources)
}

func TestBootstrap_collectDataSourcesFromConfig_AllTypes(t *testing.T) {
	client := &Client{logger: zap.NewNop()}
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			VictoriaMetrics: config.VictoriaMetricsConfig{
				Endpoints: []string{"http://vm:8428"},
			},
			VictoriaLogs: config.VictoriaLogsConfig{
				Endpoints: []string{"http://vl:9428"},
			},
			VictoriaTraces: config.VictoriaTracesConfig{
				Endpoints: []string{"http://vt:4317"},
			},
		},
	}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	dataSources := bootstrap.collectDataSourcesFromConfig()

	// Should have all 3 types
	assert.Len(t, dataSources, 3)

	types := make(map[DataSourceType]bool)
	for _, ds := range dataSources {
		types[ds.Type] = true
	}

	assert.True(t, types[DataSourceTypePrometheus])
	assert.True(t, types[DataSourceTypeVictoriaLogs])
	assert.True(t, types[DataSourceTypeVictoriaTraces])
}

func TestBootstrap_syncDataSourcesFromConfig_NotConnected(t *testing.T) {
	client := &Client{
		cfg:    config.MariaDBConfig{Enabled: true},
		db:     nil, // Not connected
		logger: zap.NewNop(),
	}
	cfg := &config.Config{}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	err := bootstrap.syncDataSourcesFromConfig(context.Background())
	assert.ErrorIs(t, err, ErrMariaDBNotConnected)
}

// Test compilation of Bootstrap methods
func TestBootstrap_MethodsExist(t *testing.T) {
	client := &Client{logger: zap.NewNop()}
	cfg := &config.Config{}
	logger := zap.NewNop()
	bootstrapCfg := BootstrapConfig{}

	bootstrap := NewBootstrap(client, cfg, logger, bootstrapCfg)

	// Verify methods exist (compile-time check)
	require.NotNil(t, bootstrap.Run)
}
