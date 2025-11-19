package config

import "time"

type Config struct {
	Environment string `mapstructure:"environment" yaml:"environment"`
	Port        int    `mapstructure:"port" yaml:"port"`
	LogLevel    string `mapstructure:"log_level" yaml:"log_level"`

	Database     DatabaseConfig     `mapstructure:"database" yaml:"database"`
	GRPC         GRPCConfig         `mapstructure:"grpc" yaml:"grpc"`
	Cache        CacheConfig        `mapstructure:"cache" yaml:"cache"`
	CORS         CORSConfig         `mapstructure:"cors" yaml:"cors"`
	Integrations IntegrationsConfig `mapstructure:"integrations" yaml:"integrations"`
	WebSocket    WebSocketConfig    `mapstructure:"websocket" yaml:"websocket"`
	Monitoring   MonitoringConfig   `mapstructure:"monitoring" yaml:"monitoring"`
	Weaviate     WeaviateConfig     `mapstructure:"weaviate" yaml:"weaviate"`
	Uploads      UploadsConfig      `mapstructure:"uploads" yaml:"uploads"`
	Search       SearchConfig       `mapstructure:"search" yaml:"search"`
	UnifiedQuery UnifiedQueryConfig `mapstructure:"unified_query" yaml:"unified_query"`
	RCA          RCAConfig          `mapstructure:"rca" yaml:"rca"`
}

// DatabaseConfig handles VictoriaMetrics ecosystem configuration
type DatabaseConfig struct {
	VictoriaMetrics VictoriaMetricsConfig `mapstructure:"victoria_metrics" yaml:"victoria_metrics"`
	VictoriaLogs    VictoriaLogsConfig    `mapstructure:"victoria_logs" yaml:"victoria_logs"`
	VictoriaTraces  VictoriaTracesConfig  `mapstructure:"victoria_traces" yaml:"victoria_traces"`
	// MetricsSources allows configuring multiple independent VictoriaMetrics clusters
	// for aggregation across sources. When non-empty, mirador will fan-out metrics
	// queries to each source and aggregate the results. The legacy
	// victoria_metrics block is still honored and treated as an additional source
	// if it has endpoints or discovery enabled.
	MetricsSources []VictoriaMetricsConfig `mapstructure:"metrics_sources" yaml:"metrics_sources"`
	// LogsSources allows configuring multiple VictoriaLogs clusters for aggregation
	LogsSources []VictoriaLogsConfig `mapstructure:"logs_sources" yaml:"logs_sources"`
	// TracesSources allows configuring multiple VictoriaTraces clusters for aggregation
	TracesSources []VictoriaTracesConfig `mapstructure:"traces_sources" yaml:"traces_sources"`
}

type VictoriaMetricsConfig struct {
	// Optional friendly name for this metrics source
	Name      string             `mapstructure:"name" yaml:"name"`
	Endpoints []string           `mapstructure:"endpoints" yaml:"endpoints"`
	Timeout   int                `mapstructure:"timeout" yaml:"timeout"` // milliseconds
	Username  string             `mapstructure:"username" yaml:"username"`
	Password  string             `mapstructure:"password" yaml:"password"`
	Discovery K8sDiscoveryConfig `mapstructure:"discovery" yaml:"discovery"`
}

type VictoriaLogsConfig struct {
	// Optional friendly name for this logs source
	Name      string             `mapstructure:"name" yaml:"name"`
	Endpoints []string           `mapstructure:"endpoints" yaml:"endpoints"`
	Timeout   int                `mapstructure:"timeout" yaml:"timeout"`
	Username  string             `mapstructure:"username" yaml:"username"`
	Password  string             `mapstructure:"password" yaml:"password"`
	Discovery K8sDiscoveryConfig `mapstructure:"discovery" yaml:"discovery"`
}

type VictoriaTracesConfig struct {
	// Optional friendly name for this traces source
	Name      string             `mapstructure:"name" yaml:"name"`
	Endpoints []string           `mapstructure:"endpoints" yaml:"endpoints"`
	Timeout   int                `mapstructure:"timeout" yaml:"timeout"`
	Username  string             `mapstructure:"username" yaml:"username"`
	Password  string             `mapstructure:"password" yaml:"password"`
	Discovery K8sDiscoveryConfig `mapstructure:"discovery" yaml:"discovery"`
}

// K8sDiscoveryConfig enables dynamic endpoint discovery for a Service
type K8sDiscoveryConfig struct {
	Enabled        bool   `mapstructure:"enabled" yaml:"enabled"`
	Service        string `mapstructure:"service" yaml:"service"` // e.g. vmselect.vm-select.svc.cluster.local
	Port           int    `mapstructure:"port" yaml:"port"`
	Scheme         string `mapstructure:"scheme" yaml:"scheme"` // http | https
	RefreshSeconds int    `mapstructure:"refresh_seconds" yaml:"refresh_seconds"`
	UseSRV         bool   `mapstructure:"use_srv" yaml:"use_srv"`
}

// GRPCConfig handles AI engines gRPC configuration
type GRPCConfig struct {
	RCAEngine   RCAEngineConfig   `mapstructure:"rca_engine" yaml:"rca_engine"`
	AlertEngine AlertEngineConfig `mapstructure:"alert_engine" yaml:"alert_engine"`
}

type RCAEngineConfig struct {
	Endpoint             string  `mapstructure:"endpoint" yaml:"endpoint"`
	CorrelationThreshold float64 `mapstructure:"correlation_threshold" yaml:"correlation_threshold"`
	Timeout              int     `mapstructure:"timeout" yaml:"timeout"`
}

type AlertEngineConfig struct {
	Endpoint  string `mapstructure:"endpoint" yaml:"endpoint"`
	RulesPath string `mapstructure:"rules_path" yaml:"rules_path"`
	Timeout   int    `mapstructure:"timeout" yaml:"timeout"`
}

// AuthConfig is deprecated and kept empty for backward compatibility
// Authentication and RBAC are now handled externally (API gateway, service mesh, etc.)
type AuthConfig struct{}

// CacheConfig handles Valkey cluster caching configuration
type CacheConfig struct {
	Nodes    []string `mapstructure:"nodes" yaml:"nodes"`
	TTL      int      `mapstructure:"ttl" yaml:"ttl"` // seconds
	Password string   `mapstructure:"password" yaml:"password"`
	DB       int      `mapstructure:"db" yaml:"db"`
}

// CORSConfig handles Cross-Origin Resource Sharing
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins" yaml:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods" yaml:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers" yaml:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers" yaml:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age" yaml:"max_age"`
}

// IntegrationsConfig handles external service integrations
type IntegrationsConfig struct {
	Slack   SlackConfig   `mapstructure:"slack" yaml:"slack"`
	MSTeams MSTeamsConfig `mapstructure:"ms_teams" yaml:"ms_teams"`
	Email   EmailConfig   `mapstructure:"email" yaml:"email"`
}

type SlackConfig struct {
	WebhookURL string `mapstructure:"webhook_url" yaml:"webhook_url"`
	Channel    string `mapstructure:"channel" yaml:"channel"`
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled"`
}

type MSTeamsConfig struct {
	WebhookURL string `mapstructure:"webhook_url" yaml:"webhook_url"`
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled"`
}

type EmailConfig struct {
	SMTPHost    string `mapstructure:"smtp_host" yaml:"smtp_host"`
	SMTPPort    int    `mapstructure:"smtp_port" yaml:"smtp_port"`
	Username    string `mapstructure:"username" yaml:"username"`
	Password    string `mapstructure:"password" yaml:"password"`
	FromAddress string `mapstructure:"from_address" yaml:"from_address"`
	Enabled     bool   `mapstructure:"enabled" yaml:"enabled"`
}

// UploadsConfig controls payload limits for uploads
type UploadsConfig struct {
	// BulkMaxBytes sets the maximum allowed size in bytes for bulk CSV uploads
	BulkMaxBytes int64 `mapstructure:"bulk_max_bytes" yaml:"bulk_max_bytes"`
}

// WebSocketConfig handles real-time streaming configuration
type WebSocketConfig struct {
	Enabled         bool `mapstructure:"enabled" yaml:"enabled"`
	MaxConnections  int  `mapstructure:"max_connections" yaml:"max_connections"`
	ReadBufferSize  int  `mapstructure:"read_buffer_size" yaml:"read_buffer_size"`
	WriteBufferSize int  `mapstructure:"write_buffer_size" yaml:"write_buffer_size"`
	PingInterval    int  `mapstructure:"ping_interval" yaml:"ping_interval"` // seconds
	MaxMessageSize  int  `mapstructure:"max_message_size" yaml:"max_message_size"`
}

// MonitoringConfig handles self-monitoring configuration
type MonitoringConfig struct {
	Enabled           bool   `mapstructure:"enabled" yaml:"enabled"`
	MetricsPath       string `mapstructure:"metrics_path" yaml:"metrics_path"`
	PrometheusEnabled bool   `mapstructure:"prometheus_enabled" yaml:"prometheus_enabled"`
	TracingEnabled    bool   `mapstructure:"tracing_enabled" yaml:"tracing_enabled"`
	JaegerEndpoint    string `mapstructure:"jaeger_endpoint" yaml:"jaeger_endpoint"`
}

// WeaviateConfig holds connection details for Weaviate HTTP API
type WeaviateConfig struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled"`
	Scheme  string `mapstructure:"scheme" yaml:"scheme"` // http or https
	Host    string `mapstructure:"host" yaml:"host"`     // DNS name or host
	Port    int    `mapstructure:"port" yaml:"port"`     // default 8080
	APIKey  string `mapstructure:"api_key" yaml:"api_key"`
	// Optional: per-request consistency/replication knobs (passed via headers/params)
	Consistency string `mapstructure:"consistency" yaml:"consistency"`
	// UseOfficial toggles the official weaviate-go-client when available.
	UseOfficial bool `mapstructure:"use_official" yaml:"use_official"`
	// NestedKeys predeclares nestedProperties for object fields like tags/examples/etc.
	NestedKeys []string `mapstructure:"nested_keys" yaml:"nested_keys"`
}

// SearchConfig holds configuration for search engines
type SearchConfig struct {
	DefaultEngine string           `mapstructure:"default_engine" yaml:"default_engine"`
	EnableBleve   bool             `mapstructure:"enable_bleve" yaml:"enable_bleve"`
	EnableLucene  bool             `mapstructure:"enable_lucene" yaml:"enable_lucene"`
	QueryCache    QueryCacheConfig `mapstructure:"query_cache" yaml:"query_cache"`
	Bleve         BleveConfig      `mapstructure:"bleve" yaml:"bleve"`
}

// QueryCacheConfig holds query caching configuration
type QueryCacheConfig struct {
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	TTL     int  `mapstructure:"ttl" yaml:"ttl"`
}

// BleveConfig holds Bleve-specific configuration
type BleveConfig struct {
	LogsEnabled        bool                     `mapstructure:"logs_enabled" yaml:"logs_enabled"`
	TracesEnabled      bool                     `mapstructure:"traces_enabled" yaml:"traces_enabled"`
	MetricsEnabled     bool                     `mapstructure:"metrics_enabled" yaml:"metrics_enabled"`
	IndexPath          string                   `mapstructure:"index_path" yaml:"index_path"`
	BatchSize          int                      `mapstructure:"batch_size" yaml:"batch_size"`
	MaxMemoryMB        int                      `mapstructure:"max_memory_mb" yaml:"max_memory_mb"`
	MemoryOptimization MemoryOptimizationConfig `mapstructure:"memory_optimization" yaml:"memory_optimization"`
	Storage            BleveStorageConfig       `mapstructure:"storage" yaml:"storage"`
	MetricsSync        MetricsSyncConfig        `mapstructure:"metrics_sync" yaml:"metrics_sync"`
}

// MetricsSyncConfig holds metrics metadata synchronization configuration
type MetricsSyncConfig struct {
	Enabled           bool          `mapstructure:"enabled" yaml:"enabled"`
	Strategy          string        `mapstructure:"strategy" yaml:"strategy"` // "incremental", "full", "hybrid"
	Interval          time.Duration `mapstructure:"interval" yaml:"interval"`
	FullSyncInterval  time.Duration `mapstructure:"full_sync_interval" yaml:"full_sync_interval"`
	BatchSize         int           `mapstructure:"batch_size" yaml:"batch_size"`
	MaxRetries        int           `mapstructure:"max_retries" yaml:"max_retries"`
	RetryDelay        time.Duration `mapstructure:"retry_delay" yaml:"retry_delay"`
	TimeRangeLookback time.Duration `mapstructure:"time_range_lookback" yaml:"time_range_lookback"`
	ShardCount        int           `mapstructure:"shard_count" yaml:"shard_count"`
}

// MemoryOptimizationConfig holds memory optimization settings
type MemoryOptimizationConfig struct {
	ObjectPooling bool `mapstructure:"object_pooling" yaml:"object_pooling"`
	AdaptiveCache bool `mapstructure:"adaptive_cache" yaml:"adaptive_cache"`
}

// BleveStorageConfig holds Bleve storage configuration
type BleveStorageConfig struct {
	MemoryCacheRatio     float64 `mapstructure:"memory_cache_ratio" yaml:"memory_cache_ratio"`
	DiskCacheRatio       float64 `mapstructure:"disk_cache_ratio" yaml:"disk_cache_ratio"`
	MaxConcurrentQueries int     `mapstructure:"max_concurrent_queries" yaml:"max_concurrent_queries"`
}

// UnifiedQueryConfig holds unified query engine configuration
type UnifiedQueryConfig struct {
	Enabled           bool          `mapstructure:"enabled" yaml:"enabled"`
	CacheTTL          time.Duration `mapstructure:"cache_ttl" yaml:"cache_ttl"`
	MaxCacheTTL       time.Duration `mapstructure:"max_cache_ttl" yaml:"max_cache_ttl"`
	DefaultLimit      int           `mapstructure:"default_limit" yaml:"default_limit"`
	EnableCorrelation bool          `mapstructure:"enable_correlation" yaml:"enable_correlation"`
}

// RCAConfig holds Root Cause Analysis engine configuration
type RCAConfig struct {
	// Enabled toggles RCA feature on/off
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`

	// ExtraMetrics lists additional metric names (PromQL/MetricsQL friendly names) to consider
	// as impact/cause signals beyond standard OTEL metrics.
	// Example: ["custom_api_errors", "db_connection_pool_exhaustion"]
	ExtraMetrics []string `mapstructure:"extra_metrics" yaml:"extra_metrics"`

	// ExtraLabels lists additional label keys to include as correlation/RCA dimensions
	// beyond standard OTEL labels (service_name, span_kind, etc.).
	// Example: ["env", "region", "cluster", "namespace", "pod_name"]
	ExtraLabels []string `mapstructure:"extra_labels" yaml:"extra_labels"`

	// ExtraLabelWeights maps label keys to their influence weight (0..1).
	// If a label is not listed here, default weight is 0.1.
	ExtraLabelWeights map[string]float64 `mapstructure:"extra_label_weights" yaml:"extra_label_weights"`

	// KPIBindings maps KPI names to their underlying metric/label representations.
	// Allows correlating business-layer KPIs with technical OTEL signals.
	// Example: {"revenue_impacted": "error_rate:payment_service", "latency_spike": "p99_latency"}
	KPIBindings map[string]string `mapstructure:"kpi_bindings" yaml:"kpi_bindings"`

	// ScoringBiasKPINegative is the bias to apply when a KPI with NEGATIVE sentiment increases
	// and correlates with candidate causes (default: 0.05).
	ScoringBiasKPINegative float64 `mapstructure:"scoring_bias_kpi_negative" yaml:"scoring_bias_kpi_negative"`

	// ScoringBiasKPIPositive is the bias to apply when a KPI with POSITIVE sentiment increases
	// (typically a decrease in confidence for candidates, so negative; default: -0.05).
	ScoringBiasKPIPositive float64 `mapstructure:"scoring_bias_kpi_positive" yaml:"scoring_bias_kpi_positive"`

	// AlignmentPenalty is the penalty applied when extra dimensions misalign (0..1).
	// Default: 0.2 (20% penalty per misaligned dimension).
	AlignmentPenalty float64 `mapstructure:"alignment_penalty" yaml:"alignment_penalty"`

	// AlignmentBonus is the bonus applied when extra dimensions align (0..1).
	// Default: 0.1 (10% bonus per aligned dimension).
	AlignmentBonus float64 `mapstructure:"alignment_bonus" yaml:"alignment_bonus"`
}
