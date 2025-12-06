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

		Cache: CacheConfig{
			Nodes: []string{"localhost:6379"},
			TTL:   300,
			DB:    0,
		},

		CORS: CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
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

		// Engine defaults (Correlation & RCA)
		Engine: EngineConfig{
			MinWindow:        10 * time.Second,
			MaxWindow:        1 * time.Hour,
			DefaultGraphHops: 2,
			DefaultMaxWhys:   5,
			RingStrategy:     "auto",
			Buckets: BucketConfig{
				CoreWindowSize: 30 * time.Second,
				PreRings:       2,
				PostRings:      1,
				RingStep:       15 * time.Second,
			},
			MinCorrelation:   0.6,
			MinAnomalyScore:  0.7,
			StrictTimeWindow: false,
			// NOTE(HCB-001): Probes removed per AGENTS.md §3.6 - must be populated via KPI registry or external config.
			// Engines will discover KPIs via Stage-00 registry; empty list forces registry-driven discovery.
			Probes: []string{},
			// NOTE(HCB-002): ServiceCandidates removed per AGENTS.md §3.6 - must come from service discovery or registry.
			// Empty list forces engines to use KPI registry metadata for service discovery.
			ServiceCandidates: []string{},
			DefaultQueryLimit: 1000,
			Labels: LabelSchemaConfig{
				Service:    []string{"service", "service.name", "serviceName"},
				Pod:        []string{"pod", "kubernetes.pod_name"},
				Namespace:  []string{"namespace", "kubernetes.namespace_name"},
				Deployment: []string{"deployment"},
				Container:  []string{"container", "kubernetes.container_name"},
				Host:       []string{"host", "hostname"},
				Level:      []string{"level", "severity"},
			},
		},

		Weaviate: WeaviateConfig{
			Enabled:     true,
			Scheme:      "http",
			Host:        "localhost",
			Port:        8080,
			UseOfficial: true,
			Vectorizer: WeaviateVectorizerConfig{
				Provider: "text2vec-transformers",
				Model:    "sentence-transformers/all-MiniLM-L6-v2",
				UseGPU:   false,
			},
		},
	}
}
