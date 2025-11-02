package config

import "time"

// GetDefaultConfig returns a configuration with all default values
func GetDefaultConfig() *Config {
	return &Config{
		Environment: "development",
		Port:        8010,
		LogLevel:    "info",

		Database: DatabaseConfig{
			VictoriaMetrics: VictoriaMetricsConfig{
				Endpoints: []string{"http://localhost:8481"},
				Timeout:   30000,
			},
			MetricsSources: []VictoriaMetricsConfig{},
			VictoriaLogs: VictoriaLogsConfig{
				Endpoints: []string{"http://localhost:9428"},
				Timeout:   30000,
			},
			LogsSources: []VictoriaLogsConfig{},
			VictoriaTraces: VictoriaTracesConfig{
				Endpoints: []string{"http://localhost:10428"},
				Timeout:   30000,
			},
			TracesSources: []VictoriaTracesConfig{},
		},

		GRPC: GRPCConfig{
			RCAEngine: RCAEngineConfig{
				Endpoint:             "localhost:9092",
				CorrelationThreshold: 0.85,
				Timeout:              30000,
			},
			AlertEngine: AlertEngineConfig{
				Endpoint:  "localhost:9093",
				RulesPath: "/etc/mirador/alert-rules.yaml",
				Timeout:   30000,
			},
		},

		Auth: AuthConfig{
			LDAP: LDAPConfig{
				Enabled: false,
			},
			OAuth: OAuthConfig{
				Enabled: false,
			},
			RBAC: RBACConfig{
				Enabled:   true,
				AdminRole: "mirador-admin",
			},
			JWT: JWTConfig{
				ExpiryMin: 1440, // 24 hours
			},
		},

		Cache: CacheConfig{
			Nodes: []string{"localhost:6379"},
			TTL:   300,
			DB:    0,
		},

		CORS: CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Tenant-ID"},
			ExposedHeaders:   []string{"X-Cache", "X-Rate-Limit-Remaining"},
			AllowCredentials: true,
			MaxAge:           3600,
		},

		Integrations: IntegrationsConfig{
			Slack: SlackConfig{
				Enabled: false,
			},
			MSTeams: MSTeamsConfig{
				Enabled: false,
			},
			Email: EmailConfig{
				Enabled:  false,
				SMTPPort: 587,
			},
		},

		WebSocket: WebSocketConfig{
			Enabled:         true,
			MaxConnections:  1000,
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			PingInterval:    30,
			MaxMessageSize:  1048576, // 1MB
		},

		Monitoring: MonitoringConfig{
			Enabled:           true,
			MetricsPath:       "/metrics",
			PrometheusEnabled: true,
			TracingEnabled:    false,
		},

		Search: SearchConfig{
			DefaultEngine: "lucene",
			EnableBleve:   false,
			EnableLucene:  true,
			QueryCache: QueryCacheConfig{
				Enabled: true,
				TTL:     300,
			},
			Bleve: BleveConfig{
				LogsEnabled:    false,
				TracesEnabled:  false,
				MetricsEnabled: false,
				IndexPath:      "/tmp/mirador-bleve",
				BatchSize:      1000,
				MaxMemoryMB:    512,
				MemoryOptimization: MemoryOptimizationConfig{
					ObjectPooling: true,
					AdaptiveCache: true,
				},
				Storage: BleveStorageConfig{
					MemoryCacheRatio:     0.8,
					DiskCacheRatio:       0.2,
					MaxConcurrentQueries: 10,
				},
				MetricsSync: MetricsSyncConfig{
					Enabled:           true,
					Strategy:          "incremental",
					Interval:          15 * time.Minute,
					FullSyncInterval:  24 * time.Hour,
					BatchSize:         1000,
					MaxRetries:        3,
					RetryDelay:        30 * time.Second,
					TimeRangeLookback: 1 * time.Hour,
					ShardCount:        3,
				},
			},
		},

		UnifiedQuery: UnifiedQueryConfig{
			Enabled:           true,
			CacheTTL:          5 * time.Minute,
			MaxCacheTTL:       1 * time.Hour,
			DefaultLimit:      1000,
			EnableCorrelation: false,
		},
	}
}
