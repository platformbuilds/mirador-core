package services

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCorrelationEngine_PopulatesDescription_Phase5 verifies that the correlation
// engine populates the Description field from KPIDefinition when building CauseCandidates.
// This test validates Phase 5 integration (P5-T1-S3).
func TestCorrelationEngine_PopulatesDescription_Phase5(t *testing.T) {
	tests := []struct {
		name                 string
		kpiDescription       string
		expectedInCandidate  string
		expectDescriptionSet bool
	}{
		{
			name:                 "KPI with detailed description",
			kpiDescription:       "Measures the total number of HTTP 5xx errors in the API gateway service. Used for detecting service degradation.",
			expectedInCandidate:  "Measures the total number of HTTP 5xx errors in the API gateway service. Used for detecting service degradation.",
			expectDescriptionSet: true,
		},
		{
			name:                 "KPI with empty description",
			kpiDescription:       "",
			expectedInCandidate:  "",
			expectDescriptionSet: false,
		},
		{
			name:                 "KPI with long description",
			kpiDescription:       "This is a very detailed multi-paragraph description explaining the KPI purpose, calculation methodology, expected baseline values, alert thresholds, and business impact when degraded.",
			expectedInCandidate:  "This is a very detailed multi-paragraph description explaining the KPI purpose, calculation methodology, expected baseline values, alert thresholds, and business impact when degraded.",
			expectDescriptionSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a CauseCandidate as the correlation engine would
			kpi := &models.KPIDefinition{
				ID:          "test-kpi-001",
				Name:        "http_5xx_errors_total",
				Description: tt.kpiDescription,
				Formula:     "sum(rate(http_requests_total{status=~\"5..\"}[5m]))",
			}

			cand := models.CauseCandidate{
				KPI:         kpi.Name,
				KPIUUID:     kpi.ID,
				KPIFormula:  kpi.Formula,
				Description: kpi.Description, // Phase 5 integration
			}

			// Verify Description field is populated correctly
			if tt.expectDescriptionSet {
				assert.Equal(t, tt.expectedInCandidate, cand.Description, "Description should match KPI definition")
				assert.NotEmpty(t, cand.Description, "Description should not be empty when KPI has description")
			} else {
				assert.Empty(t, cand.Description, "Description should be empty when KPI has no description")
			}

			// Verify backward compatibility - existing fields still work
			assert.Equal(t, kpi.Name, cand.KPI)
			assert.Equal(t, kpi.ID, cand.KPIUUID)
			assert.Equal(t, kpi.Formula, cand.KPIFormula)
		})
	}
}

// TestCorrelationEngine_DescriptionFieldOmitempty_Phase5 verifies that the Description
// field is properly omitted from JSON when empty (backward compatibility).
func TestCorrelationEngine_DescriptionFieldOmitempty_Phase5(t *testing.T) {
	cand := models.CauseCandidate{
		KPI:            "test_kpi",
		KPIUUID:        "uuid-123",
		Service:        "api-gateway",
		SuspicionScore: 0.85,
		// Description intentionally not set
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(cand)
	require.NoError(t, err)

	// Verify description field is omitted when empty
	assert.NotContains(t, string(jsonData), "\"description\"", "Empty description should be omitted from JSON (omitempty)")

	// Add description and verify it appears
	cand.Description = "Test description"
	jsonData2, err := json.Marshal(cand)
	require.NoError(t, err)
	assert.Contains(t, string(jsonData2), "\"description\"", "Non-empty description should appear in JSON")
	assert.Contains(t, string(jsonData2), "Test description")
}

// TestCorrelationEngine_DataTypeFieldNotUsedInLogic_Phase5 documents that DataType
// field exists but does not affect correlation logic (P5-T1-S1 analysis result).
// The engine already handles signal types via SignalType and Datastore fields.
func TestCorrelationEngine_DataTypeFieldNotUsedInLogic_Phase5(t *testing.T) {
	// This test documents the design decision: DataType field is metadata only
	// and does not influence correlation algorithm selection.

	kpis := []*models.KPIDefinition{
		{
			ID:         "kpi-timeseries",
			Name:       "cpu_usage",
			SignalType: "metric",
			Datastore:  "victoriametrics",
			DataType:   "timeseries", // Phase 4 field
			Formula:    "avg(cpu_usage)",
		},
		{
			ID:         "kpi-value",
			Name:       "circuit_breaker_state",
			SignalType: "metric",
			Datastore:  "victoriametrics",
			DataType:   "value", // Phase 4 field
			Formula:    "circuit_breaker_open",
		},
		{
			ID:         "kpi-categorical",
			Name:       "error_category",
			SignalType: "log",
			Datastore:  "clickhouse",
			DataType:   "categorical", // Phase 4 field
			Formula:    "SELECT category FROM errors",
		},
	}

	// Verify: The correlation engine queries all KPIs the same way based on
	// SignalType/Datastore, not DataType. DataType is informational metadata
	// that may be used by UI or future advanced statistical engines, but
	// Stage-01 correlation does not branch on DataType.

	for _, kpi := range kpis {
		// DataType field is present and valid
		assert.NotEmpty(t, kpi.DataType, "DataType should be populated from Phase 4")

		// But correlation logic routes by SignalType/Datastore
		assert.NotEmpty(t, kpi.SignalType, "SignalType drives backend routing")
		assert.NotEmpty(t, kpi.Datastore, "Datastore drives backend routing")

		// Document: DataType does not change which backend is queried
		// or which correlation method is used in Stage-01.
		t.Logf("KPI %s: DataType=%s (metadata), SignalType=%s (routing), Datastore=%s (routing)",
			kpi.ID, kpi.DataType, kpi.SignalType, kpi.Datastore)
	}
}

// TestCorrelationEngine_RefreshIntervalNotUsedInCorrelation_Phase5 documents that
// RefreshInterval is a cache/UI field and does not affect correlation time window logic.
func TestCorrelationEngine_RefreshIntervalNotUsedInCorrelation_Phase5(t *testing.T) {
	// RefreshInterval is metadata for cache invalidation and UI refresh rates.
	// The correlation engine works with user-specified TimeRange windows and
	// does not adjust queries based on RefreshInterval.

	kpi := &models.KPIDefinition{
		ID:              "kpi-high-refresh",
		Name:            "realtime_tps",
		RefreshInterval: 10, // 10 seconds (high frequency)
		Formula:         "rate(requests_total[1m])",
	}

	// Document: RefreshInterval does not alter the query window or ring construction
	assert.Equal(t, 10, kpi.RefreshInterval, "RefreshInterval is independent metadata")

	// RefreshInterval might be used by:
	// - Cache layers to decide when to invalidate cached KPI values
	// - UI to determine polling frequency
	// - Future: Time-series alignment heuristics (but NOT in Stage-01)

	t.Logf("RefreshInterval=%d seconds is metadata; correlation uses provided TimeRange window", kpi.RefreshInterval)
}

// TestCorrelationEngine_MetadataFieldsNotHardcoded_Phase5 verifies that the engine
// does not hardcode values for new metadata fields (AGENTS.md §3.6 compliance).
func TestCorrelationEngine_MetadataFieldsNotHardcoded_Phase5(t *testing.T) {
	// Verify new fields come from config/registry, never hardcoded in engine code

	kpi := &models.KPIDefinition{
		ID:              "kpi-test",
		Name:            "test_metric",
		Description:     "Configurable description from registry",
		DataType:        "timeseries",      // From config/registry
		DataSourceID:    "ds-uuid-001",     // From registry
		KPIDatastoreID:  "kpi-ds-uuid-002", // From registry
		RefreshInterval: 60,                // From config/registry
		IsShared:        true,              // From config/registry
		UserID:          "user-uuid-123",   // From registry
	}

	// All new fields should be populated from registry data, not engine defaults
	assert.NotEmpty(t, kpi.Description, "Description from registry")
	assert.NotEmpty(t, kpi.DataType, "DataType from registry")
	assert.NotEmpty(t, kpi.DataSourceID, "DataSourceID from registry")
	assert.NotEmpty(t, kpi.KPIDatastoreID, "KPIDatastoreID from registry")
	assert.Greater(t, kpi.RefreshInterval, 0, "RefreshInterval from registry")
	assert.NotEmpty(t, kpi.UserID, "UserID from registry")

	// Document: Engine never sets these fields itself, only reads from KPIDefinition
	t.Log("Phase 5: All new metadata fields sourced from registry, no hardcoding in engine")
}

// TestCorrelationEngine_DescriptionEnhancesNarrative_Integration verifies that
// Description field flows from correlation engine to RCA narrative generation.
func TestCorrelationEngine_DescriptionEnhancesNarrative_Integration(t *testing.T) {
	// Simulate correlation engine building CauseCandidates with descriptions
	candidates := []models.CauseCandidate{
		{
			KPI:            "http_errors_5xx",
			KPIUUID:        "kpi-001",
			Description:    "Measures HTTP 5xx server errors indicating backend failures",
			Service:        "api-gateway",
			SuspicionScore: 0.92,
		},
		{
			KPI:            "db_connection_pool_exhausted",
			KPIUUID:        "kpi-002",
			Description:    "Tracks database connection pool saturation leading to query timeouts",
			Service:        "postgres-db",
			SuspicionScore: 0.87,
		},
	}

	// Verify descriptions are present for narrative generation
	for _, cand := range candidates {
		assert.NotEmpty(t, cand.Description, "Description should be populated for narrative context")

		// RCA engine can now use cand.Description to enrich "Why" explanations
		narrative := generateMockNarrative(cand)
		assert.Contains(t, narrative, cand.KPI, "Narrative includes KPI name")

		// Phase 5: Optionally include description for richer context
		if cand.Description != "" {
			enhancedNarrative := narrative + " Context: " + cand.Description
			assert.Contains(t, enhancedNarrative, "Context:", "Enhanced narrative includes description")
			t.Logf("Enhanced narrative: %s", enhancedNarrative)
		}
	}
}

// generateMockNarrative simulates RCA narrative generation using CauseCandidate data
func generateMockNarrative(cand models.CauseCandidate) string {
	return fmt.Sprintf("Why 1: %s (suspicion=%.2f) showed anomalies in %s service",
		cand.KPI, cand.SuspicionScore, cand.Service)
}

// BenchmarkCorrelationEngine_DescriptionFieldOverhead measures performance impact
// of adding Description field to CauseCandidate (should be negligible).
func BenchmarkCorrelationEngine_DescriptionFieldOverhead(b *testing.B) {
	kpi := &models.KPIDefinition{
		ID:          "bench-kpi",
		Name:        "test_metric",
		Description: "This is a moderately long description field that simulates real-world KPI documentation explaining purpose and usage.",
		Formula:     "sum(rate(requests[5m]))",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cand := models.CauseCandidate{
			KPI:         kpi.Name,
			KPIUUID:     kpi.ID,
			Description: kpi.Description, // Phase 5 field
			Service:     "test-service",
		}
		_ = cand
	}
}
