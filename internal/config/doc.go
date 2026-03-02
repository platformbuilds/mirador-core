// Package config provides configuration loading and validation for MIRADOR-CORE.
//
// # Overview
//
// This package handles loading configuration from multiple sources with
// proper precedence, validation, and defaults. It supports:
//   - YAML configuration files
//   - Environment variable overrides
//   - Structured validation with detailed errors
//   - Type-safe configuration access
//
// # Configuration Structure
//
// The main configuration type is [Config], which contains nested
// configurations for each component:
//
//	type Config struct {
//	    Server     ServerConfig
//	    Engine     EngineConfig
//	    MariaDB    MariaDBConfig
//	    Weaviate   WeaviateConfig
//	    Cache      CacheConfig
//	    WebSocket  WebSocketConfig
//	    Logging    LoggingConfig
//	}
//
// # Loading Configuration
//
// Use [LoadConfig] to load configuration:
//
//	cfg, err := config.LoadConfig("configs/config.yaml")
//	if err != nil {
//	    var validationErrs config.ValidationErrors
//	    if errors.As(err, &validationErrs) {
//	        for _, ve := range validationErrs {
//	            log.Printf("Config error in %s: %s", ve.Field, ve.Message)
//	        }
//	    }
//	    return err
//	}
//
// # Environment Overrides
//
// Configuration values can be overridden via environment variables.
// The naming convention is:
//
//	MIRADOR_<SECTION>_<FIELD>
//
// Examples:
//
//	MIRADOR_SERVER_PORT=8080
//	MIRADOR_MARIADB_HOST=localhost
//	MIRADOR_ENGINE_MAXWINDOW=1h
//
// # Validation
//
// Configuration is validated on load. Validation errors include:
//   - Field path (e.g., "mariadb.host")
//   - Error message
//   - Current value (if safe to display)
//
// Validated fields include:
//   - Required fields (host, port, database names)
//   - Numeric ranges (ports, timeouts, pool sizes)
//   - Duration parsing
//   - URL formats
//
// # Defaults
//
// Sensible defaults are provided for optional configuration:
//
//	server:
//	  port: 8010
//	  readTimeout: 30s
//	  writeTimeout: 60s
//
//	engine:
//	  minWindow: 1m
//	  maxWindow: 1h
//	  correlationTimeout: 60s
//
// # Configuration Files
//
// Configuration files are located in the configs/ directory:
//   - config.yaml: Base configuration
//   - config.development.yaml: Development overrides
//   - config.production.yaml: Production overrides
//
// The loader automatically merges environment-specific files.
package config
