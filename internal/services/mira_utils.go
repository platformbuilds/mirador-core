package services

import (
	"os"
	"strings"
)

// resolveEnvVar resolves environment variable syntax like "${VAR_NAME}".
// If the value is in the form "${VAR_NAME}", it returns os.Getenv(VAR_NAME).
// Otherwise, it returns the value as-is.
func resolveEnvVar(value string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
		return os.Getenv(envVar)
	}
	return value
}
