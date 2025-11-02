package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/platformbuilds/mirador-core/internal/models"
)

// GenerateSessionID creates a secure session identifier
func GenerateSessionID() string {
	return uuid.New().String()
}

// GenerateClientID creates WebSocket client identifier
func GenerateClientID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("client_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// CountSeries counts the number of series in MetricsQL response
func CountSeries(data interface{}) int {
	switch d := data.(type) {
	case map[string]interface{}:
		if result, ok := d["result"].([]interface{}); ok {
			return len(result)
		}
	case []interface{}:
		return len(d)
	}
	return 0
}

// CountDataPoints counts total data points in range query response
func CountDataPoints(data interface{}) int {
	count := 0
	switch d := data.(type) {
	case map[string]interface{}:
		if result, ok := d["result"].([]interface{}); ok {
			for _, series := range result {
				if s, ok := series.(map[string]interface{}); ok {
					if values, ok := s["values"].([]interface{}); ok {
						count += len(values)
					}
				}
			}
		}
	}
	return count
}

// CountAlertsBySeverity counts alerts by severity level
func CountAlertsBySeverity(alerts []*models.Alert, severity string) int {
	count := 0
	for _, alert := range alerts {
		if alert.Severity == severity {
			count++
		}
	}
	return count
}

// Contains checks if string slice contains a value
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetStreamNames extracts stream names from boolean map
func GetStreamNames(streams map[string]bool) []string {
	var names []string
	for name, enabled := range streams {
		if enabled {
			names = append(names, name)
		}
	}
	return names
}

// IsUint32String returns true if s is a base-10 unsigned integer that fits in uint32.
func IsUint32String(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	if _, err := strconv.ParseUint(s, 10, 32); err != nil {
		return false
	}
	return true
}
