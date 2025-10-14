package utils

import (
	"testing"
)

// TestBackwardCompatibility ensures that existing query formats still work
func TestBackwardCompatibility(t *testing.T) {
	v := NewQueryValidator()

	// Test that existing LogsQL queries are still valid as Lucene
	logsQLQueries := []string{
		`_msg:"error"`,
		`level:"error"`,
		`_time:1h`,
		`service:"api" AND level:"error"`,
	}

	for _, query := range logsQLQueries {
		t.Run("LogsQL_"+query, func(t *testing.T) {
			err := v.ValidateLucene(query)
			if err != nil {
				t.Errorf("Backward compatibility failed for LogsQL query %s: %v", query, err)
			}
		})
	}

	// Test that existing traces queries are still valid as Lucene
	tracesQueries := []string{
		`service:api`,
		`operation:login`,
		`tag.env:prod`,
		`service:api AND operation:search`,
	}

	for _, query := range tracesQueries {
		t.Run("Traces_"+query, func(t *testing.T) {
			err := v.ValidateLucene(query)
			if err != nil {
				t.Errorf("Backward compatibility failed for traces query %s: %v", query, err)
			}
		})
	}
}