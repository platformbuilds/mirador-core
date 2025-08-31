// contains checks if a string slice contains a specific value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetConfigFromEnv loads minimal configuration from environment variables only
func GetConfigFromEnv() *Config {
	config := GetDefaultConfig()

	// Override with environment variables
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		config.Environment = env
	}

	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Port = p
		}
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}

	return config
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsTest returns true if running in test environment
func (c *Config) IsTest() bool {
	return c.Environment == "test"
}

// GetDatabaseTimeout returns the appropriate database timeout
func (c *Config) GetDatabaseTimeout() time.Duration {
	timeout := c.Database.VictoriaMetrics.Timeout
	if timeout == 0 {
		timeout = DefaultHTTPTimeout
	}
	return time.Duration(timeout) * time.Millisecond
}

// GetGRPCTimeout returns the appropriate gRPC timeout
func (c *Config) GetGRPCTimeout() time.Duration {
	timeout := c.GRPC.PredictEngine.Timeout
	if timeout == 0 {
		timeout = DefaultGRPCTimeout
	}
	return time.Duration(timeout) * time.Millisecond
}

// GetCacheTTL returns the cache TTL as a duration
func (c *Config) GetCacheTTL() time.Duration {
	ttl := c.Cache.TTL
	if ttl == 0 {
		ttl = DefaultCacheTTL
	}
	return time.Duration(ttl) * time.Second
}

// ValidateEndpoints validates all configured endpoints
func (c *Config) ValidateEndpoints() error {
	// Validate VictoriaMetrics endpoints
	for _, endpoint := range c.Database.VictoriaMetrics.Endpoints {
		if err := ValidateEndpoint(endpoint); err != nil {
			return fmt.Errorf("invalid VictoriaMetrics endpoint %s: %w", endpoint, err)
		}
	}

	// Validate VictoriaLogs endpoints
	for _, endpoint := range c.Database.VictoriaLogs.Endpoints {
		if err := ValidateEndpoint(endpoint); err != nil {
			return fmt.Errorf("invalid VictoriaLogs endpoint %s: %w", endpoint, err)
		}
	}

	// Validate VictoriaTraces endpoints
	for _, endpoint := range c.Database.VictoriaTraces.Endpoints {
		if err := ValidateEndpoint(endpoint); err != nil {
			return fmt.Errorf("invalid VictoriaTraces endpoint %s: %w", endpoint, err)
		}
	}

	// Validate gRPC endpoints
	if err := ValidateGRPCEndpoint(c.GRPC.PredictEngine.Endpoint); err != nil {
		return fmt.Errorf("invalid PREDICT-ENGINE endpoint: %w", err)
	}

	if err := ValidateGRPCEndpoint(c.GRPC.RCAEngine.Endpoint); err != nil {
		return fmt.Errorf("invalid RCA-ENGINE endpoint: %w", err)
	}

	if err := ValidateGRPCEndpoint(c.GRPC.AlertEngine.Endpoint); err != nil {
		return fmt.Errorf("invalid ALERT-ENGINE endpoint: %w", err)
	}

	// Validate Valley cluster nodes
	for _, node := range c.Cache.Nodes {
		if err := ValidateRedisNode(node); err != nil {
			return fmt.Errorf("invalid Valley cluster node %s: %w", node, err)
		}
	}

	// Validate webhook URLs
	if err := ValidateWebhookURL(c.Integrations.Slack.WebhookURL); err != nil {
		return fmt.Errorf("invalid Slack webhook URL: %w", err)
	}

	if err := ValidateWebhookURL(c.Integrations.MSTeams.WebhookURL); err != nil {
		return fmt.Errorf("invalid MS Teams webhook URL: %w", err)
	}

	return nil
}

// ToJSON converts configuration to JSON string (for debugging)
func (c *Config) ToJSON() string {
	// Create a copy without sensitive information
	safeCopy := *c
	safeCopy.Auth.JWT.Secret = "[REDACTED]"
	safeCopy.Auth.LDAP.Password = "[REDACTED]"
	safeCopy.Auth.OAuth.ClientSecret = "[REDACTED]"
	safeCopy.Cache.Password = "[REDACTED]"
	safeCopy.Database.VictoriaMetrics.Password = "[REDACTED]"
	safeCopy.Database.VictoriaLogs.Password = "[REDACTED]"
	safeCopy.Database.VictoriaTraces.Password = "[REDACTED]"
	safeCopy.Integrations.Email.Password = "[REDACTED]"

	jsonBytes, _ := json.MarshalIndent(safeCopy, "", "  ")
	return string(jsonBytes)
}
