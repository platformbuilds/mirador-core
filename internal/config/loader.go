package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Load loads configuration with priority:
//
// 1) CONFIG_PATH (absolute path to a YAML file)
// 2) ./configs/config.<env>.yaml  (env = MIRADOR_ENV | ENV | "development")
// 3) ./configs/config.yaml
// 4) /etc/mirador/config.(<env>|yaml) then current directory fallback
//
// Env vars override file values. We support both MIRADOR_* variables via Viper
// and a few legacy plain env vars via overrideWithEnvVars.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults first
	setDefaults(v)

	// Support MIRADOR_* env overrides (e.g., MIRADOR_PORT)
	v.AutomaticEnv()
	v.SetEnvPrefix("MIRADOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 1) explicit config file path
	if explicit := strings.TrimSpace(os.Getenv("CONFIG_PATH")); explicit != "" {
		v.SetConfigFile(explicit)
		if err := tryRead(v); err != nil {
			return nil, err
		}
	} else {
		// 2) ./configs/config.<env>.yaml  or 3) ./configs/config.yaml
		// Honor common env selectors in this order: MIRADOR_ENV, ENV, ENVIRONMENT
		env := firstNonEmpty(os.Getenv("MIRADOR_ENV"), os.Getenv("ENV"), os.Getenv("ENVIRONMENT"), "development")
		envPath := "./configs/config." + strings.ToLower(env) + ".yaml"

		if fileExists(envPath) {
			v.SetConfigFile(envPath)
			if err := tryRead(v); err != nil {
				return nil, err
			}
		} else if fileExists("./configs/config.yaml") {
			v.SetConfigFile("./configs/config.yaml")
			if err := tryRead(v); err != nil {
				return nil, err
			}
		} else {
			// 4) search standard locations
			v.SetConfigType("yaml")
			v.SetConfigName("config." + strings.ToLower(env))
			v.AddConfigPath("/etc/mirador/")
			v.AddConfigPath("./configs/")
			v.AddConfigPath(".")
			if err := v.ReadInConfig(); err != nil {
				// try plain "config.yaml"
				v.SetConfigName("config")
				if err2 := v.ReadInConfig(); err2 != nil {
					if _, ok := err2.(viper.ConfigFileNotFoundError); !ok {
						return nil, fmt.Errorf("failed to read config file: %w", err2)
					}
					// not found anywhere → proceed with env + defaults
				}
			}
		}
	}

	// Legacy/plain env overrides for compatibility
	overrideWithEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Ensure telemetry maps are initialized to empty maps when not provided
	// so consumers can safely range over them without nil checks.
	if cfg.Engine.Telemetry.Connectors == nil {
		cfg.Engine.Telemetry.Connectors = map[string]ConnectorConfig{}
	}
	if cfg.Engine.Telemetry.Processors == nil {
		cfg.Engine.Telemetry.Processors = map[string]ProcessorConfig{}
	}

	// Validate (config validation)
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

/* ------------------------------- helpers -------------------------------- */

func tryRead(v *viper.Viper) error {
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}
	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func splitCSV(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

/* ------------------------------- defaults -------------------------------- */

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("environment", "development")
	v.SetDefault("port", 8010)
	v.SetDefault("log_level", "info")

	// Databases
	v.SetDefault("database.victoria_metrics.endpoints", []string{"http://localhost:8481"})
	v.SetDefault("database.victoria_metrics.timeout", 30000)
	v.SetDefault("database.victoria_metrics.discovery.enabled", false)
	v.SetDefault("database.victoria_metrics.discovery.scheme", "http")
	v.SetDefault("database.victoria_metrics.discovery.refresh_seconds", 30)
	v.SetDefault("database.victoria_logs.endpoints", []string{"http://localhost:9428"})
	v.SetDefault("database.victoria_logs.timeout", 30000)
	v.SetDefault("database.victoria_logs.discovery.enabled", false)
	v.SetDefault("database.victoria_logs.discovery.scheme", "http")
	v.SetDefault("database.victoria_logs.discovery.refresh_seconds", 30)
	v.SetDefault("database.victoria_traces.endpoints", []string{"http://localhost:10428"})
	v.SetDefault("database.victoria_traces.timeout", 30000)
	v.SetDefault("database.victoria_traces.discovery.enabled", false)
	v.SetDefault("database.victoria_traces.discovery.scheme", "http")
	v.SetDefault("database.victoria_traces.discovery.refresh_seconds", 30)
	// Optional multi-source metrics aggregation list (default empty)
	v.SetDefault("database.metrics_sources", []map[string]any{})
	// Optional multi-source logs aggregation list (default empty)
	v.SetDefault("database.logs_sources", []map[string]any{})
	// Optional multi-source traces aggregation list (default empty)
	v.SetDefault("database.traces_sources", []map[string]any{})

	// gRPC
	v.SetDefault("grpc.rca_engine.endpoint", "localhost:9092")
	v.SetDefault("grpc.rca_engine.correlation_threshold", 0.85)
	v.SetDefault("grpc.rca_engine.timeout", 30000)
	v.SetDefault("grpc.alert_engine.endpoint", "localhost:9093")
	v.SetDefault("grpc.alert_engine.rules_path", "/etc/mirador/alert-rules.yaml")
	v.SetDefault("grpc.alert_engine.timeout", 30000)

	// Auth
	v.SetDefault("auth.enabled", true)
	// Note: Auth is now handled externally (API gateway, service mesh, etc.)

	// Cache (Valkey)
	v.SetDefault("cache.nodes", []string{"localhost:6379"})
	v.SetDefault("cache.ttl", 300)
	v.SetDefault("cache.db", 0)

	// CORS
	v.SetDefault("cors.allowed_origins", []string{"*"})
	v.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allowed_headers", []string{"Content-Type", "Authorization"})
	v.SetDefault("cors.exposed_headers", []string{"X-Cache", "X-Rate-Limit-Remaining"})
	v.SetDefault("cors.allow_credentials", true)
	v.SetDefault("cors.max_age", 3600)

	// Integrations
	v.SetDefault("integrations.slack.enabled", false)
	v.SetDefault("integrations.ms_teams.enabled", false)
	v.SetDefault("integrations.email.enabled", false)
	v.SetDefault("integrations.email.smtp_port", 587)

	// WebSocket
	v.SetDefault("websocket.enabled", true)
	v.SetDefault("websocket.max_connections", 1000)
	v.SetDefault("websocket.read_buffer_size", 1024)
	v.SetDefault("websocket.write_buffer_size", 1024)
	v.SetDefault("websocket.ping_interval", 30)
	v.SetDefault("websocket.max_message_size", 1048576)

	// Monitoring
	v.SetDefault("monitoring.enabled", true)
	v.SetDefault("monitoring.metrics_path", "/metrics")
	v.SetDefault("monitoring.prometheus_enabled", true)
	v.SetDefault("monitoring.tracing_enabled", false)

	// Uploads
	v.SetDefault("uploads.bulk_max_bytes", int64(5<<20)) // 5 MiB default

	// Weaviate (disabled by default)
	v.SetDefault("weaviate.enabled", false)
	v.SetDefault("weaviate.scheme", "http")
	v.SetDefault("weaviate.host", "weaviate.mirador.svc.cluster.local")
	v.SetDefault("weaviate.port", 8080)
	v.SetDefault("weaviate.use_official", false)

	// Unified Query Engine (Phase 1.5)
	v.SetDefault("unified_query.enabled", true)
	v.SetDefault("unified_query.cache_ttl", "5m")
	v.SetDefault("unified_query.max_cache_ttl", "1h")
	v.SetDefault("unified_query.default_limit", 1000)
	v.SetDefault("unified_query.enable_correlation", false)

	// Engine (Correlation & RCA) defaults (AT-004)
	v.SetDefault("engine.min_window", "10s")
	v.SetDefault("engine.max_window", "1h")
	v.SetDefault("engine.default_graph_hops", 2)
	v.SetDefault("engine.default_max_whys", 5)
	v.SetDefault("engine.ring_strategy", "auto")
	v.SetDefault("engine.buckets.core_window_size", "30s")
	v.SetDefault("engine.buckets.pre_rings", 2)
	v.SetDefault("engine.buckets.post_rings", 1)
	v.SetDefault("engine.buckets.ring_step", "15s")
	v.SetDefault("engine.min_correlation", 0.6)
	v.SetDefault("engine.min_anomaly_score", 0.7)
	v.SetDefault("engine.strict_time_window", false)
	// AT-013: strict payload validation for correlation/rca endpoints
	v.SetDefault("engine.strict_timewindow_payload", false)
}

/* ---------------------------- legacy overrides --------------------------- */

// overrideWithEnvVars keeps support for a few plain env vars
// (PORT/ENVIRONMENT/LOG_LEVEL etc.) for backward compatibility.
func overrideWithEnvVars(v *viper.Viper) {
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			v.Set("port", p)
		}
	}
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		v.Set("environment", env)
	}
	if ll := os.Getenv("LOG_LEVEL"); ll != "" {
		v.Set("log_level", ll)
	}

	if vm := os.Getenv("VM_ENDPOINTS"); vm != "" {
		v.Set("database.victoria_metrics.endpoints", splitCSV(vm))
	}
	if vl := os.Getenv("VL_ENDPOINTS"); vl != "" {
		v.Set("database.victoria_logs.endpoints", splitCSV(vl))
	}
	if vt := os.Getenv("VT_ENDPOINTS"); vt != "" {
		v.Set("database.victoria_traces.endpoints", splitCSV(vt))
	}

	if r := os.Getenv("RCA_ENGINE_GRPC"); r != "" {
		v.Set("grpc.rca_engine.endpoint", r)
	}
	if a := os.Getenv("ALERT_ENGINE_GRPC"); a != "" {
		v.Set("grpc.alert_engine.endpoint", a)
	}

	// Prefer VALKEY_CACHE_NODES; keep VALKEY_CACHE_NODES for backward compatibility
	if nodes := os.Getenv("VALKEY_CACHE_NODES"); nodes != "" {
		v.Set("cache.nodes", splitCSV(nodes))
	} else if nodes := os.Getenv("VALKEY_CACHE_NODES"); nodes != "" {
		v.Set("cache.nodes", splitCSV(nodes))
	}
	if ttl := os.Getenv("CACHE_TTL"); ttl != "" {
		if i, err := strconv.Atoi(ttl); err == nil {
			v.Set("cache.ttl", i)
		}
	}

	if ldapURL := os.Getenv("LDAP_URL"); ldapURL != "" {
		v.Set("auth.ldap.url", ldapURL)
		v.Set("auth.ldap.enabled", true)
	}
	if ldapBase := os.Getenv("LDAP_BASE_DN"); ldapBase != "" {
		v.Set("auth.ldap.base_dn", ldapBase)
	}
	if ae := os.Getenv("AUTH_ENABLED"); ae != "" {
		if b, err := strconv.ParseBool(ae); err == nil {
			v.Set("auth.enabled", b)
		}
	}
	// Auth env vars no longer supported - handled externally

	if slack := os.Getenv("SLACK_WEBHOOK_URL"); slack != "" {
		v.Set("integrations.slack.webhook_url", slack)
		v.Set("integrations.slack.enabled", true)
	}
	if teams := os.Getenv("TEAMS_WEBHOOK_URL"); teams != "" {
		v.Set("integrations.ms_teams.webhook_url", teams)
		v.Set("integrations.ms_teams.enabled", true)
	}
	if smtp := os.Getenv("SMTP_HOST"); smtp != "" {
		v.Set("integrations.email.smtp_host", smtp)
		v.Set("integrations.email.enabled", true)
	}

	// Weaviate overrides
	if wh := os.Getenv("WEAVIATE_HOST"); wh != "" {
		v.Set("weaviate.host", wh)
	}
	if wp := os.Getenv("WEAVIATE_PORT"); wp != "" {
		if i, err := strconv.Atoi(wp); err == nil {
			v.Set("weaviate.port", i)
		}
	}
	if ws := os.Getenv("WEAVIATE_SCHEME"); ws != "" {
		v.Set("weaviate.scheme", ws)
	}
	if wk := os.Getenv("WEAVIATE_API_KEY"); wk != "" {
		v.Set("weaviate.api_key", wk)
	}
	if wc := os.Getenv("WEAVIATE_CONSISTENCY"); wc != "" {
		v.Set("weaviate.consistency", wc)
	}
	if we := os.Getenv("WEAVIATE_ENABLED"); we != "" {
		if b, err := strconv.ParseBool(we); err == nil {
			v.Set("weaviate.enabled", b)
		}
	}
	if wo := os.Getenv("WEAVIATE_USE_OFFICIAL"); wo != "" {
		if b, err := strconv.ParseBool(wo); err == nil {
			v.Set("weaviate.use_official", b)
		}
	}

	// Uploads (CSV bulk)
	if s := os.Getenv("BULK_UPLOAD_MAX_BYTES"); s != "" {
		if vbytes, err := strconv.ParseInt(s, 10, 64); err == nil && vbytes > 0 {
			v.Set("uploads.bulk_max_bytes", vbytes)
		}
	}
	if s := os.Getenv("BULK_UPLOAD_MAX_MIB"); s != "" {
		if vmib, err := strconv.ParseInt(s, 10, 64); err == nil && vmib > 0 {
			v.Set("uploads.bulk_max_bytes", vmib*(1<<20))
		}
	}
}

/* ------------------------------- validation ------------------------------ */

// ValidationError represents a configuration validation error with context.
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("config validation failed for '%s': %s", e.Field, e.Message)
}

// ValidationErrors collects multiple validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("config validation failed with %d errors: %s", len(e), strings.Join(msgs, "; "))
}

func validateConfig(cfg *Config) error {
	var errs ValidationErrors

	// Server validations
	if cfg.Port < 1 || cfg.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "port",
			Value:   cfg.Port,
			Message: fmt.Sprintf("must be between 1 and 65535, got %d", cfg.Port),
		})
	}

	validLog := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLog, cfg.LogLevel) {
		errs = append(errs, ValidationError{
			Field:   "log_level",
			Value:   cfg.LogLevel,
			Message: fmt.Sprintf("must be one of %v", validLog),
		})
	}

	validEnv := []string{"development", "staging", "production", "test"}
	if !contains(validEnv, cfg.Environment) {
		errs = append(errs, ValidationError{
			Field:   "environment",
			Value:   cfg.Environment,
			Message: fmt.Sprintf("must be one of %v", validEnv),
		})
	}

	// Database validations
	errs = append(errs, validateDatabaseConfig(&cfg.Database)...)

	// Cache validations
	if len(cfg.Cache.Nodes) == 0 {
		errs = append(errs, ValidationError{
			Field:   "cache.nodes",
			Message: "at least one Valkey cache node is required",
		})
	}
	if cfg.Cache.TTL < 1 {
		errs = append(errs, ValidationError{
			Field:   "cache.ttl",
			Value:   cfg.Cache.TTL,
			Message: "must be at least 1 second",
		})
	}

	// gRPC validations
	if cfg.GRPC.RCAEngine.Endpoint == "" {
		errs = append(errs, ValidationError{
			Field:   "grpc.rca_engine.endpoint",
			Message: "RCA-ENGINE gRPC endpoint is required",
		})
	}
	if cfg.GRPC.AlertEngine.Endpoint == "" {
		errs = append(errs, ValidationError{
			Field:   "grpc.alert_engine.endpoint",
			Message: "ALERT-ENGINE gRPC endpoint is required",
		})
	}
	if cfg.GRPC.RCAEngine.CorrelationThreshold < 0 || cfg.GRPC.RCAEngine.CorrelationThreshold > 1 {
		errs = append(errs, ValidationError{
			Field:   "grpc.rca_engine.correlation_threshold",
			Value:   cfg.GRPC.RCAEngine.CorrelationThreshold,
			Message: "must be between 0 and 1",
		})
	}

	// MariaDB validations
	errs = append(errs, validateMariaDBConfig(&cfg.MariaDB)...)

	// Engine validations
	errs = append(errs, validateEngineConfig(&cfg.Engine)...)

	// WebSocket validations
	errs = append(errs, validateWebSocketConfig(&cfg.WebSocket)...)

	// Weaviate validations
	errs = append(errs, validateWeaviateConfig(&cfg.Weaviate)...)

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateDatabaseConfig(db *DatabaseConfig) ValidationErrors {
	var errs ValidationErrors

	// Metrics must have at least one source
	hasPrimary := len(db.VictoriaMetrics.Endpoints) > 0 || db.VictoriaMetrics.Discovery.Enabled
	hasMulti := false
	for _, s := range db.MetricsSources {
		if len(s.Endpoints) > 0 || s.Discovery.Enabled {
			hasMulti = true
			break
		}
	}
	if !hasPrimary && !hasMulti {
		errs = append(errs, ValidationError{
			Field:   "database.victoria_metrics",
			Message: "at least one VictoriaMetrics source is required",
		})
	}

	// Logs must have at least one source
	hasLogsPrimary := len(db.VictoriaLogs.Endpoints) > 0 || db.VictoriaLogs.Discovery.Enabled
	hasLogsMulti := false
	for _, s := range db.LogsSources {
		if len(s.Endpoints) > 0 || s.Discovery.Enabled {
			hasLogsMulti = true
			break
		}
	}
	if !hasLogsPrimary && !hasLogsMulti {
		errs = append(errs, ValidationError{
			Field:   "database.victoria_logs",
			Message: "at least one VictoriaLogs source is required",
		})
	}

	// Validate timeout values
	if db.VictoriaMetrics.Timeout < 0 {
		errs = append(errs, ValidationError{
			Field:   "database.victoria_metrics.timeout",
			Value:   db.VictoriaMetrics.Timeout,
			Message: "timeout must be non-negative",
		})
	}
	if db.VictoriaLogs.Timeout < 0 {
		errs = append(errs, ValidationError{
			Field:   "database.victoria_logs.timeout",
			Value:   db.VictoriaLogs.Timeout,
			Message: "timeout must be non-negative",
		})
	}

	return errs
}

func validateMariaDBConfig(m *MariaDBConfig) ValidationErrors {
	var errs ValidationErrors

	if !m.Enabled {
		return errs // Skip validation if disabled
	}

	if m.Host == "" {
		errs = append(errs, ValidationError{
			Field:   "mariadb.host",
			Message: "host is required when MariaDB is enabled",
		})
	}
	if m.Port < 1 || m.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "mariadb.port",
			Value:   m.Port,
			Message: "must be between 1 and 65535",
		})
	}
	if m.Database == "" {
		errs = append(errs, ValidationError{
			Field:   "mariadb.database",
			Message: "database name is required when MariaDB is enabled",
		})
	}
	if m.Username == "" {
		errs = append(errs, ValidationError{
			Field:   "mariadb.username",
			Message: "username is required when MariaDB is enabled",
		})
	}
	if m.MaxOpenConns < 0 {
		errs = append(errs, ValidationError{
			Field:   "mariadb.max_open_conns",
			Value:   m.MaxOpenConns,
			Message: "must be non-negative",
		})
	}
	if m.MaxIdleConns < 0 {
		errs = append(errs, ValidationError{
			Field:   "mariadb.max_idle_conns",
			Value:   m.MaxIdleConns,
			Message: "must be non-negative",
		})
	}
	if m.MaxIdleConns > m.MaxOpenConns && m.MaxOpenConns > 0 {
		errs = append(errs, ValidationError{
			Field:   "mariadb.max_idle_conns",
			Value:   m.MaxIdleConns,
			Message: "cannot exceed max_open_conns",
		})
	}
	if m.ConnMaxLifetime < 0 {
		errs = append(errs, ValidationError{
			Field:   "mariadb.conn_max_lifetime",
			Value:   m.ConnMaxLifetime,
			Message: "must be non-negative",
		})
	}

	// Sync validations
	if m.Sync.Enabled && m.Sync.Interval < 0 {
		errs = append(errs, ValidationError{
			Field:   "mariadb.sync.interval",
			Value:   m.Sync.Interval,
			Message: "must be non-negative",
		})
	}
	if m.Sync.Enabled && m.Sync.BatchSize < 1 {
		errs = append(errs, ValidationError{
			Field:   "mariadb.sync.batch_size",
			Value:   m.Sync.BatchSize,
			Message: "must be at least 1",
		})
	}

	return errs
}

func validateEngineConfig(e *EngineConfig) ValidationErrors {
	var errs ValidationErrors

	if e.MinWindow < 0 {
		errs = append(errs, ValidationError{
			Field:   "engine.min_window",
			Value:   e.MinWindow,
			Message: "must be non-negative",
		})
	}
	if e.MaxWindow < 0 {
		errs = append(errs, ValidationError{
			Field:   "engine.max_window",
			Value:   e.MaxWindow,
			Message: "must be non-negative",
		})
	}
	if e.MaxWindow > 0 && e.MinWindow > e.MaxWindow {
		errs = append(errs, ValidationError{
			Field:   "engine.min_window",
			Value:   e.MinWindow,
			Message: "cannot exceed max_window",
		})
	}
	if e.DefaultGraphHops < 0 {
		errs = append(errs, ValidationError{
			Field:   "engine.default_graph_hops",
			Value:   e.DefaultGraphHops,
			Message: "must be non-negative",
		})
	}
	if e.DefaultMaxWhys < 0 {
		errs = append(errs, ValidationError{
			Field:   "engine.default_max_whys",
			Value:   e.DefaultMaxWhys,
			Message: "must be non-negative",
		})
	}
	if e.MinCorrelation < 0 || e.MinCorrelation > 1 {
		errs = append(errs, ValidationError{
			Field:   "engine.min_correlation",
			Value:   e.MinCorrelation,
			Message: "must be between 0 and 1",
		})
	}
	if e.MinAnomalyScore < 0 || e.MinAnomalyScore > 1 {
		errs = append(errs, ValidationError{
			Field:   "engine.min_anomaly_score",
			Value:   e.MinAnomalyScore,
			Message: "must be between 0 and 1",
		})
	}
	if e.DefaultQueryLimit < 0 {
		errs = append(errs, ValidationError{
			Field:   "engine.default_query_limit",
			Value:   e.DefaultQueryLimit,
			Message: "must be non-negative",
		})
	}

	// Bucket validations
	if e.Buckets.CoreWindowSize < 0 {
		errs = append(errs, ValidationError{
			Field:   "engine.buckets.core_window_size",
			Value:   e.Buckets.CoreWindowSize,
			Message: "must be non-negative",
		})
	}
	if e.Buckets.PreRings < 0 {
		errs = append(errs, ValidationError{
			Field:   "engine.buckets.pre_rings",
			Value:   e.Buckets.PreRings,
			Message: "must be non-negative",
		})
	}
	if e.Buckets.PostRings < 0 {
		errs = append(errs, ValidationError{
			Field:   "engine.buckets.post_rings",
			Value:   e.Buckets.PostRings,
			Message: "must be non-negative",
		})
	}

	return errs
}

func validateWebSocketConfig(ws *WebSocketConfig) ValidationErrors {
	var errs ValidationErrors

	if !ws.Enabled {
		return errs
	}

	if ws.MaxConnections < 0 {
		errs = append(errs, ValidationError{
			Field:   "websocket.max_connections",
			Value:   ws.MaxConnections,
			Message: "must be non-negative",
		})
	}
	if ws.ReadBufferSize < 0 {
		errs = append(errs, ValidationError{
			Field:   "websocket.read_buffer_size",
			Value:   ws.ReadBufferSize,
			Message: "must be non-negative",
		})
	}
	if ws.WriteBufferSize < 0 {
		errs = append(errs, ValidationError{
			Field:   "websocket.write_buffer_size",
			Value:   ws.WriteBufferSize,
			Message: "must be non-negative",
		})
	}
	if ws.PingInterval < 0 {
		errs = append(errs, ValidationError{
			Field:   "websocket.ping_interval",
			Value:   ws.PingInterval,
			Message: "must be non-negative",
		})
	}
	if ws.MaxMessageSize < 0 {
		errs = append(errs, ValidationError{
			Field:   "websocket.max_message_size",
			Value:   ws.MaxMessageSize,
			Message: "must be non-negative",
		})
	}

	return errs
}

func validateWeaviateConfig(w *WeaviateConfig) ValidationErrors {
	var errs ValidationErrors

	if !w.Enabled {
		return errs
	}

	if w.Host == "" {
		errs = append(errs, ValidationError{
			Field:   "weaviate.host",
			Message: "host is required when Weaviate is enabled",
		})
	}
	if w.Port < 0 || w.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "weaviate.port",
			Value:   w.Port,
			Message: "must be between 0 and 65535",
		})
	}

	return errs
}
