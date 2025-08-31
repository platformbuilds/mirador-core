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
}

// DatabaseConfig handles VictoriaMetrics ecosystem configuration
type DatabaseConfig struct {
	VictoriaMetrics VictoriaMetricsConfig `mapstructure:"victoria_metrics" yaml:"victoria_metrics"`
	VictoriaLogs    VictoriaLogsConfig    `mapstructure:"victoria_logs" yaml:"victoria_logs"`
	VictoriaTraces  VictoriaTracesConfig  `mapstructure:"victoria_traces" yaml:"victoria_traces"`
}

type VictoriaMetricsConfig struct {
	Endpoints []string `mapstructure:"endpoints" yaml:"endpoints"`
	Timeout   int      `mapstructure:"timeout" yaml:"timeout"` // milliseconds
	Username  string   `mapstructure:"username" yaml:"username"`
	Password  string   `mapstructure:"password" yaml:"password"`
}

type VictoriaLogsConfig struct {
	Endpoints []string `mapstructure:"endpoints" yaml:"endpoints"`
	Timeout   int      `mapstructure:"timeout" yaml:"timeout"`
	Username  string   `mapstructure:"username" yaml:"username"`
	Password  string   `mapstructure:"password" yaml:"password"`
}

type VictoriaTracesConfig struct {
	Endpoints []string `mapstructure:"endpoints" yaml:"endpoints"`
	Timeout   int      `mapstructure:"timeout" yaml:"timeout"`
	Username  string   `mapstructure:"username" yaml:"username"`
	Password  string   `mapstructure:"password" yaml:"password"`
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
	LDAP  LDAPConfig  `mapstructure:"ldap" yaml:"ldap"`
	OAuth OAuthConfig `mapstructure:"oauth" yaml:"oauth"`
	RBAC  RBACConfig  `mapstructure:"rbac" yaml:"rbac"`
	JWT   JWTConfig   `mapstructure:"jwt" yaml:"jwt"`
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
