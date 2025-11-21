package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// LoadSecrets loads sensitive configuration from environment or files
func LoadSecrets(config *Config) error {
	// Load Valkey password for Valkey cluster
	if valkeyPassword := os.Getenv("VALKEY_PASSWORD"); valkeyPassword != "" {
		config.Cache.Password = valkeyPassword
	} else if passwordFile := os.Getenv("VALKEY_PASSWORD_FILE"); passwordFile != "" {
		password, err := os.ReadFile(passwordFile)
		if err != nil {
			return fmt.Errorf("failed to read Valkey password file: %w", err)
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
