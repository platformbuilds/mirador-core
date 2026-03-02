package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig returns a valid baseline config for testing.
func validConfig() *Config {
	return &Config{
		Port:        8080,
		LogLevel:    "info",
		Environment: "development",
		Database: DatabaseConfig{
			VictoriaMetrics: VictoriaMetricsConfig{
				Endpoints: []string{"http://localhost:8428"},
				Timeout:   30,
			},
			VictoriaLogs: VictoriaLogsConfig{
				Endpoints: []string{"http://localhost:9428"},
				Timeout:   30,
			},
		},
		Cache: CacheConfig{
			Nodes: []string{"localhost:6379"},
			TTL:   300,
		},
		GRPC: GRPCConfig{
			RCAEngine: RCAEngineConfig{
				Endpoint:             "localhost:50051",
				CorrelationThreshold: 0.5,
			},
			AlertEngine: AlertEngineConfig{
				Endpoint: "localhost:50052",
			},
		},
		Weaviate: WeaviateConfig{
			Enabled: false,
		},
		MariaDB: MariaDBConfig{
			Enabled: false,
		},
		WebSocket: WebSocketConfig{
			Enabled: false,
		},
		Engine: EngineConfig{
			MinWindow:         time.Minute,
			MaxWindow:         time.Hour,
			DefaultGraphHops:  3,
			DefaultMaxWhys:    5,
			MinCorrelation:    0.5,
			MinAnomalyScore:   0.6,
			DefaultQueryLimit: 100,
		},
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := validConfig()
	err := validateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too_high", 65536},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Port = tt.port
			err := validateConfig(cfg)
			require.Error(t, err)
			var verrs ValidationErrors
			require.ErrorAs(t, err, &verrs)
			assert.Contains(t, err.Error(), "port")
		})
	}
}

func TestValidateConfig_InvalidLogLevel(t *testing.T) {
	cfg := validConfig()
	cfg.LogLevel = "invalid"
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log_level")
}

func TestValidateConfig_InvalidEnvironment(t *testing.T) {
	cfg := validConfig()
	cfg.Environment = "invalid"
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
}

func TestValidateConfig_MissingVictoriaMetrics(t *testing.T) {
	cfg := validConfig()
	cfg.Database.VictoriaMetrics.Endpoints = nil
	cfg.Database.MetricsSources = nil
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "victoria_metrics")
}

func TestValidateConfig_MissingVictoriaLogs(t *testing.T) {
	cfg := validConfig()
	cfg.Database.VictoriaLogs.Endpoints = nil
	cfg.Database.LogsSources = nil
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "victoria_logs")
}

func TestValidateConfig_MissingCacheNodes(t *testing.T) {
	cfg := validConfig()
	cfg.Cache.Nodes = nil
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache.nodes")
}

func TestValidateConfig_InvalidCacheTTL(t *testing.T) {
	cfg := validConfig()
	cfg.Cache.TTL = 0
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache.ttl")
}

func TestValidateConfig_MissingGRPCEndpoints(t *testing.T) {
	t.Run("missing_rca_engine", func(t *testing.T) {
		cfg := validConfig()
		cfg.GRPC.RCAEngine.Endpoint = ""
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rca_engine.endpoint")
	})

	t.Run("missing_alert_engine", func(t *testing.T) {
		cfg := validConfig()
		cfg.GRPC.AlertEngine.Endpoint = ""
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "alert_engine.endpoint")
	})
}

func TestValidateConfig_InvalidCorrelationThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
	}{
		{"negative", -0.1},
		{"above_one", 1.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.GRPC.RCAEngine.CorrelationThreshold = tt.threshold
			err := validateConfig(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "correlation_threshold")
		})
	}
}

func TestValidateMariaDBConfig(t *testing.T) {
	t.Run("disabled_skips_validation", func(t *testing.T) {
		cfg := validConfig()
		cfg.MariaDB.Enabled = false
		cfg.MariaDB.Host = "" // Would fail if enabled
		err := validateConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("enabled_requires_host", func(t *testing.T) {
		cfg := validConfig()
		cfg.MariaDB = MariaDBConfig{
			Enabled:  true,
			Host:     "",
			Port:     3306,
			Database: "mirador",
			Username: "root",
		}
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mariadb.host")
	})

	t.Run("enabled_requires_database", func(t *testing.T) {
		cfg := validConfig()
		cfg.MariaDB = MariaDBConfig{
			Enabled:  true,
			Host:     "localhost",
			Port:     3306,
			Database: "",
			Username: "root",
		}
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mariadb.database")
	})

	t.Run("invalid_port", func(t *testing.T) {
		cfg := validConfig()
		cfg.MariaDB = MariaDBConfig{
			Enabled:  true,
			Host:     "localhost",
			Port:     70000,
			Database: "mirador",
			Username: "root",
		}
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mariadb.port")
	})

	t.Run("max_idle_exceeds_max_open", func(t *testing.T) {
		cfg := validConfig()
		cfg.MariaDB = MariaDBConfig{
			Enabled:      true,
			Host:         "localhost",
			Port:         3306,
			Database:     "mirador",
			Username:     "root",
			MaxOpenConns: 10,
			MaxIdleConns: 20,
		}
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_idle_conns")
	})

	t.Run("valid_config", func(t *testing.T) {
		cfg := validConfig()
		cfg.MariaDB = MariaDBConfig{
			Enabled:         true,
			Host:            "localhost",
			Port:            3306,
			Database:        "mirador",
			Username:        "root",
			Password:        "secret",
			MaxOpenConns:    20,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
		}
		err := validateConfig(cfg)
		assert.NoError(t, err)
	})
}

func TestValidateEngineConfig(t *testing.T) {
	t.Run("min_window_exceeds_max", func(t *testing.T) {
		cfg := validConfig()
		cfg.Engine.MinWindow = 2 * time.Hour
		cfg.Engine.MaxWindow = time.Hour
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "min_window")
	})

	t.Run("invalid_correlation_range", func(t *testing.T) {
		cfg := validConfig()
		cfg.Engine.MinCorrelation = 1.5
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "min_correlation")
	})

	t.Run("invalid_anomaly_score_range", func(t *testing.T) {
		cfg := validConfig()
		cfg.Engine.MinAnomalyScore = -0.1
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "min_anomaly_score")
	})

	t.Run("negative_bucket_values", func(t *testing.T) {
		cfg := validConfig()
		cfg.Engine.Buckets.PreRings = -1
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pre_rings")
	})
}

func TestValidateWebSocketConfig(t *testing.T) {
	t.Run("disabled_skips_validation", func(t *testing.T) {
		cfg := validConfig()
		cfg.WebSocket.Enabled = false
		cfg.WebSocket.MaxConnections = -1 // Would fail if enabled
		err := validateConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("invalid_max_connections", func(t *testing.T) {
		cfg := validConfig()
		cfg.WebSocket.Enabled = true
		cfg.WebSocket.MaxConnections = -1
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_connections")
	})
}

func TestValidateWeaviateConfig(t *testing.T) {
	t.Run("disabled_skips_validation", func(t *testing.T) {
		cfg := validConfig()
		cfg.Weaviate.Enabled = false
		cfg.Weaviate.Host = "" // Would fail if enabled
		err := validateConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("enabled_requires_host", func(t *testing.T) {
		cfg := validConfig()
		cfg.Weaviate.Enabled = true
		cfg.Weaviate.Host = ""
		cfg.Weaviate.Port = 8080
		err := validateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "weaviate.host")
	})

	t.Run("valid_config", func(t *testing.T) {
		cfg := validConfig()
		cfg.Weaviate.Enabled = true
		cfg.Weaviate.Host = "localhost"
		cfg.Weaviate.Port = 8080
		err := validateConfig(cfg)
		assert.NoError(t, err)
	})
}

func TestValidationErrors_Error(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var errs ValidationErrors
		assert.Equal(t, "", errs.Error())
	})

	t.Run("single", func(t *testing.T) {
		errs := ValidationErrors{
			{Field: "port", Message: "invalid"},
		}
		assert.Contains(t, errs.Error(), "port")
		assert.Contains(t, errs.Error(), "invalid")
	})

	t.Run("multiple", func(t *testing.T) {
		errs := ValidationErrors{
			{Field: "port", Message: "invalid"},
			{Field: "host", Message: "required"},
		}
		assert.Contains(t, errs.Error(), "2 errors")
		assert.Contains(t, errs.Error(), "port")
		assert.Contains(t, errs.Error(), "host")
	})
}

func TestMultipleValidationErrors(t *testing.T) {
	cfg := validConfig()
	cfg.Port = 0                              // Invalid
	cfg.LogLevel = "invalid"                  // Invalid
	cfg.Cache.TTL = 0                         // Invalid
	cfg.GRPC.RCAEngine.CorrelationThreshold = 2.0 // Invalid

	err := validateConfig(cfg)
	require.Error(t, err)

	var verrs ValidationErrors
	require.ErrorAs(t, err, &verrs)
	assert.GreaterOrEqual(t, len(verrs), 4)
}
