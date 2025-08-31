import (
	"encoding/base64"
	"fmt"
	"os"
)

// LoadSecrets loads sensitive configuration from environment or files
func LoadSecrets(config *Config) error {
	// Load JWT secret
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		config.Auth.JWT.Secret = jwtSecret
	} else if secretFile := os.Getenv("JWT_SECRET_FILE"); secretFile != "" {
		secret, err := os.ReadFile(secretFile)
		if err != nil {
			return fmt.Errorf("failed to read JWT secret file: %w", err)
		}
		config.Auth.JWT.Secret = strings.TrimSpace(string(secret))
	} else {
		// Generate a default secret for development
		if config.Environment == "development" || config.Environment == "test" {
			config.Auth.JWT.Secret = "development-secret-key-not-for-production"
		} else {
			return fmt.Errorf("JWT secret is required for production")
		}
	}

	// Load LDAP password
	if ldapPassword := os.Getenv("LDAP_PASSWORD"); ldapPassword != "" {
		config.Auth.LDAP.Password = ldapPassword
	} else if passwordFile := os.Getenv("LDAP_PASSWORD_FILE"); passwordFile != "" {
		password, err := os.ReadFile(passwordFile)
		if err != nil {
			return fmt.Errorf("failed to read LDAP password file: %w", err)
		}
		config.Auth.LDAP.Password = strings.TrimSpace(string(password))
	}

	// Load OAuth client secret
	if oauthSecret := os.Getenv("OAUTH_CLIENT_SECRET"); oauthSecret != "" {
		config.Auth.OAuth.ClientSecret = oauthSecret
	} else if secretFile := os.Getenv("OAUTH_CLIENT_SECRET_FILE"); secretFile != "" {
		secret, err := os.ReadFile(secretFile)
		if err != nil {
			return fmt.Errorf("failed to read OAuth client secret file: %w", err)
		}
		config.Auth.OAuth.ClientSecret = strings.TrimSpace(string(secret))
	}

	// Load Redis password for Valley cluster
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		config.Cache.Password = redisPassword
	} else if passwordFile := os.Getenv("REDIS_PASSWORD_FILE"); passwordFile != "" {
		password, err := os.ReadFile(passwordFile)
		if err != nil {
			return fmt.Errorf("failed to read Redis password file: %w", err)
		}
		config.Cache.Password = strings.TrimSpace(string(password))
	}

	// Load database credentials
	if vmPassword := os.Getenv("VM_PASSWORD"); vmPassword != "" {
		config.Database.VictoriaMetrics.Password = vmPassword
		config.Database.VictoriaLogs.Password = vmPassword
		config.Database.VictoriaTraces.Password = vmPassword
	}

	// Load email SMTP password
	if smtpPassword := os.Getenv("SMTP_PASSWORD"); smtpPassword != "" {
		config.Integrations.Email.Password = smtpPassword
	}

	return nil
}

// EncodeSecret base64 encodes a secret for storage
func EncodeSecret(secret string) string {
	return base64.StdEncoding.EncodeToString([]byte(secret))
}

// DecodeSecret base64 decodes a stored secret
func DecodeSecret(encodedSecret string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encodedSecret)
	if err != nil {
		return "", fmt.Errorf("failed to decode secret: %w", err)
	}
	return string(decoded), nil
}
