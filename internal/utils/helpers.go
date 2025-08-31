package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/platformbuilds/miradorstack/internal/models"
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

// FilterByRisk filters fractures by risk level
func FilterByRisk(fractures []*models.SystemFracture, risk string) []*models.SystemFracture {
	var filtered []*models.SystemFracture
	for _, fracture := range fractures {
		if fracture.Severity == risk {
			filtered = append(filtered, fracture)
		}
	}
	return filtered
}

// CalculateAvgTimeToFailure calculates average time to failure
func CalculateAvgTimeToFailure(fractures []*models.SystemFracture) time.Duration {
	if len(fractures) == 0 {
		return 0
	}

	var total time.Duration
	for _, fracture := range fractures {
		total += fracture.TimeToFracture
	}
	
	return total / time.Duration(len(fractures))
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
