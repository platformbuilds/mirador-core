package config

type Config struct {
	Environment string `mapstructure:"environment" yaml:"environment"`
	Port        int    `mapstructure:"port" yaml:"port"`
	LogLevel    string `mapstructure:"log_level" yaml:"log_level"`

	Database     DatabaseConfig     `mapstructure:"database" yaml:"database"`
	GRPC         GRPCConfig         `mapstructure:"grpc" yaml:"grpc"`
	Auth         AuthConfig         `mapstructure:"auth" yaml:"auth"`
	Cache        CacheConfig        `mapstructure:"cache" yaml:"cache"`
	CORS         CORSConfig         `mapstructure:"cors" yaml:"cors"`
	Integrations IntegrationsConfig `mapstructure:"integrations" yaml:"integrations"`
	WebSocket    WebSocketConfig    `mapstructure:"websocket" yaml:"websocket"`
	Monitoring   MonitoringConfig   `mapstructure:"monitoring" yaml:"monitoring"`
	Weaviate     WeaviateConfig     `mapstructure:"weaviate" yaml:"weaviate"`
	Uploads      UploadsConfig      `mapstructure:"uploads" yaml:"uploads"`
	Search       SearchConfig       `mapstructure:"search" yaml:"search"`
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
	PredictEngine PredictEngineConfig `mapstructure:"predict_engine" yaml:"predict_engine"`
	RCAEngine     RCAEngineConfig     `mapstructure:"rca_engine" yaml:"rca_engine"`
	AlertEngine   AlertEngineConfig   `mapstructure:"alert_engine" yaml:"alert_engine"`
}

type PredictEngineConfig struct {
	Endpoint string   `mapstructure:"endpoint" yaml:"endpoint"`
	Models   []string `mapstructure:"models" yaml:"models"`
	Timeout  int      `mapstructure:"timeout" yaml:"timeout"`
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

// AuthConfig handles authentication and authorization
type AuthConfig struct {
	Enabled bool        `mapstructure:"enabled" yaml:"enabled"`
	LDAP    LDAPConfig  `mapstructure:"ldap" yaml:"ldap"`
	OAuth   OAuthConfig `mapstructure:"oauth" yaml:"oauth"`
	RBAC    RBACConfig  `mapstructure:"rbac" yaml:"rbac"`
	JWT     JWTConfig   `mapstructure:"jwt" yaml:"jwt"`
}

type LDAPConfig struct {
	URL      string `mapstructure:"url" yaml:"url"`
	BaseDN   string `mapstructure:"base_dn" yaml:"base_dn"`
	Username string `mapstructure:"username" yaml:"username"`
	Password string `mapstructure:"password" yaml:"password"`
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled"`
}

type OAuthConfig struct {
	ClientID     string `mapstructure:"client_id" yaml:"client_id"`
	ClientSecret string `mapstructure:"client_secret" yaml:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url" yaml:"redirect_url"`
	Issuer       string `mapstructure:"issuer" yaml:"issuer"`
	Enabled      bool   `mapstructure:"enabled" yaml:"enabled"`
}

type RBACConfig struct {
	Enabled   bool   `mapstructure:"enabled" yaml:"enabled"`
	AdminRole string `mapstructure:"admin_role" yaml:"admin_role"`
}

type JWTConfig struct {
	Secret    string `mapstructure:"secret" yaml:"secret"`
	ExpiryMin int    `mapstructure:"expiry_minutes" yaml:"expiry_minutes"`
}

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
	MemoryOptimization MemoryOptimizationConfig `mapstructure:"memory_optimization" yaml:"memory_optimization"`
	Storage            BleveStorageConfig       `mapstructure:"storage" yaml:"storage"`
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
