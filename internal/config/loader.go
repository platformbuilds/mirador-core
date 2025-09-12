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

	// Validate (JWT secret is validated in secrets.go: LoadSecrets)
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
	v.SetDefault("port", 8080)
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

	// gRPC
	v.SetDefault("grpc.predict_engine.endpoint", "localhost:9091")
	v.SetDefault("grpc.predict_engine.models", []string{"isolation_forest", "lstm_trend", "anomaly_detector"})
	v.SetDefault("grpc.predict_engine.timeout", 30000)
	v.SetDefault("grpc.rca_engine.endpoint", "localhost:9092")
	v.SetDefault("grpc.rca_engine.correlation_threshold", 0.85)
	v.SetDefault("grpc.rca_engine.timeout", 30000)
	v.SetDefault("grpc.alert_engine.endpoint", "localhost:9093")
	v.SetDefault("grpc.alert_engine.rules_path", "/etc/mirador/alert-rules.yaml")
	v.SetDefault("grpc.alert_engine.timeout", 30000)

    // Auth
    v.SetDefault("auth.enabled", true)
    v.SetDefault("auth.ldap.enabled", false)
    v.SetDefault("auth.oauth.enabled", false)
    v.SetDefault("auth.rbac.enabled", true)
    v.SetDefault("auth.rbac.admin_role", "mirador-admin")
    v.SetDefault("auth.jwt.expiry_minutes", 1440)

	// Cache (Valkey)
	v.SetDefault("cache.nodes", []string{"localhost:6379"})
	v.SetDefault("cache.ttl", 300)
	v.SetDefault("cache.db", 0)

	// CORS
	v.SetDefault("cors.allowed_origins", []string{"*"})
	v.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allowed_headers", []string{"Content-Type", "Authorization", "X-Tenant-ID"})
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

	if p := os.Getenv("PREDICT_ENGINE_GRPC"); p != "" {
		v.Set("grpc.predict_engine.endpoint", p)
	}
	if r := os.Getenv("RCA_ENGINE_GRPC"); r != "" {
		v.Set("grpc.rca_engine.endpoint", r)
	}
	if a := os.Getenv("ALERT_ENGINE_GRPC"); a != "" {
		v.Set("grpc.alert_engine.endpoint", a)
	}

    // Prefer VALKEY_CACHE_NODES; keep VALLEY_CACHE_NODES for backward compatibility
    if nodes := os.Getenv("VALKEY_CACHE_NODES"); nodes != "" {
        v.Set("cache.nodes", splitCSV(nodes))
    } else if nodes := os.Getenv("VALLEY_CACHE_NODES"); nodes != "" {
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
	if rbac := os.Getenv("RBAC_ENABLED"); rbac != "" {
		if b, err := strconv.ParseBool(rbac); err == nil {
			v.Set("auth.rbac.enabled", b)
		}
	}
	if jwt := os.Getenv("JWT_SECRET"); jwt != "" {
		v.Set("auth.jwt.secret", jwt)
	}
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
    if wh := os.Getenv("WEAVIATE_HOST"); wh != "" { v.Set("weaviate.host", wh) }
    if wp := os.Getenv("WEAVIATE_PORT"); wp != "" { if i, err := strconv.Atoi(wp); err == nil { v.Set("weaviate.port", i) } }
    if ws := os.Getenv("WEAVIATE_SCHEME"); ws != "" { v.Set("weaviate.scheme", ws) }
    if wk := os.Getenv("WEAVIATE_API_KEY"); wk != "" { v.Set("weaviate.api_key", wk) }
    if wc := os.Getenv("WEAVIATE_CONSISTENCY"); wc != "" { v.Set("weaviate.consistency", wc) }
    if we := os.Getenv("WEAVIATE_ENABLED"); we != "" { if b, err := strconv.ParseBool(we); err == nil { v.Set("weaviate.enabled", b) } }
    if wo := os.Getenv("WEAVIATE_USE_OFFICIAL"); wo != "" { if b, err := strconv.ParseBool(wo); err == nil { v.Set("weaviate.use_official", b) } }

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

// Note: JWT secret enforcement is handled in secrets.go (LoadSecrets).
func validateConfig(cfg *Config) error {
    if len(cfg.Database.VictoriaMetrics.Endpoints) == 0 && !cfg.Database.VictoriaMetrics.Discovery.Enabled {
        return fmt.Errorf("at least one VictoriaMetrics endpoint is required (or enable discovery)")
    }
    if len(cfg.Database.VictoriaLogs.Endpoints) == 0 && !cfg.Database.VictoriaLogs.Discovery.Enabled {
        return fmt.Errorf("at least one VictoriaLogs endpoint is required (or enable discovery)")
    }
	if len(cfg.Cache.Nodes) == 0 {
		return fmt.Errorf("at least one Valkey cluster cache node is required")
	}

	if cfg.GRPC.PredictEngine.Endpoint == "" {
		return fmt.Errorf("PREDICT-ENGINE gRPC endpoint is required")
	}
	if cfg.GRPC.RCAEngine.Endpoint == "" {
		return fmt.Errorf("RCA-ENGINE gRPC endpoint is required")
	}
	if cfg.GRPC.AlertEngine.Endpoint == "" {
		return fmt.Errorf("ALERT-ENGINE gRPC endpoint is required")
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Port)
	}

	validLog := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLog, cfg.LogLevel) {
		return fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}

	validEnv := []string{"development", "staging", "production", "test"}
	if !contains(validEnv, cfg.Environment) {
		return fmt.Errorf("invalid environment: %s", cfg.Environment)
	}

	if cfg.Cache.TTL < 1 {
		return fmt.Errorf("cache TTL must be at least 1 second")
	}

	if cfg.GRPC.RCAEngine.CorrelationThreshold < 0 || cfg.GRPC.RCAEngine.CorrelationThreshold > 1 {
		return fmt.Errorf("RCA correlation threshold must be between 0 and 1")
	}
	return nil
}
