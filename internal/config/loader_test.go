func TestConfigLoading(t *testing.T) {
	// Test loading from file
	t.Run("load from file", func(t *testing.T) {
		// Create temporary config file
		configContent := `
environment: test
port: 9999
log_level: debug

database:
  victoria_metrics:
    endpoints:
      - "http://test-vm:8481"
    timeout: 5000

cache:
  nodes:
    - "test-redis:6379"
  ttl: 30
`
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(configContent)
		require.NoError(t, err)
		tmpFile.Close()

		// Set config path
		os.Setenv("CONFIG_PATH", tmpFile.Name())
		defer os.Unsetenv("CONFIG_PATH")

		config, err := Load()
		require.NoError(t, err)

		assert.Equal(t, "test", config.Environment)
		assert.Equal(t, 9999, config.Port)
		assert.Equal(t, "debug", config.LogLevel)
		assert.Contains(t, config.Database.VictoriaMetrics.Endpoints, "http://test-vm:8481")
		assert.Equal(t, 30, config.Cache.TTL)
	})

	// Test environment variable precedence
	t.Run("env var precedence", func(t *testing.T) {
		os.Setenv("MIRADOR_PORT", "7777")
		os.Setenv("MIRADOR_LOG_LEVEL", "warn")
		defer func() {
			os.Unsetenv("MIRADOR_PORT")
			os.Unsetenv("MIRADOR_LOG_LEVEL")
		}()

		config, err := Load()
		require.NoError(t, err)

		// Environment variables should override file/defaults
		assert.Equal(t, 7777, config.Port)
		assert.Equal(t, "warn", config.LogLevel)
	})
}

func TestSecretsLoading(t *testing.T) {
	config := GetDefaultConfig()

	// Test JWT secret from environment
	os.Setenv("JWT_SECRET", "test-secret-123")
	defer os.Unsetenv("JWT_SECRET")

	err := LoadSecrets(config)
	require.NoError(t, err)
	assert.Equal(t, "test-secret-123", config.Auth.JWT.Secret)

	// Test production validation
	config.Environment = "production"
	config.Auth.JWT.Secret = "" // Clear secret
	os.Unsetenv("JWT_SECRET")

	err = LoadSecrets(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret is required for production")
}

func BenchmarkConfigLoad(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConfigValidation(b *testing.B) {
	config := GetDefaultConfig()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		err := validateConfig(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}
