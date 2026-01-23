package rca

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestEnhanceNarrativeWithKPIDescription_Phase5 verifies the Phase 5 utility
// function for enriching RCA narratives with KPI descriptions.
func TestEnhanceNarrativeWithKPIDescription_Phase5(t *testing.T) {
	tests := []struct {
		name             string
		baseNarrative    string
		kpiDescription   string
		expectedContains []string
		maxLength        int
	}{
		{
			name:             "Empty description returns original narrative",
			baseNarrative:    "Why 1: api-gateway experienced errors",
			kpiDescription:   "",
			expectedContains: []string{"Why 1: api-gateway experienced errors"},
			maxLength:        50,
		},
		{
			name:           "Short description appended",
			baseNarrative:  "Why 1: db-connection-pool exhausted",
			kpiDescription: "Tracks connection pool saturation",
			expectedContains: []string{
				"Why 1: db-connection-pool exhausted",
				"Context:",
				"Tracks connection pool saturation",
			},
			maxLength: 100,
		},
		{
			name:           "Long description truncated",
			baseNarrative:  "Why 1: cpu-throttling detected",
			kpiDescription: "This is a very long detailed description that explains the KPI in great depth, covering calculation methodology, baseline values, alert thresholds, business impact when degraded, and recommended remediation actions for operators.",
			expectedContains: []string{
				"Why 1: cpu-throttling detected",
				"Context:",
				"...", // Truncation marker
			},
			maxLength: 300, // Enhanced narrative should not exceed reasonable length
		},
		{
			name:           "Multi-sentence description",
			baseNarrative:  "Why 1: kafka-lag increasing",
			kpiDescription: "Measures consumer lag in Kafka topics. High lag indicates processing delays.",
			expectedContains: []string{
				"Why 1: kafka-lag increasing",
				"Context:",
				"Measures consumer lag",
				"High lag indicates processing delays",
			},
			maxLength: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnhanceNarrativeWithKPIDescription(tt.baseNarrative, tt.kpiDescription)

			// Verify expected content
			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected, "Enhanced narrative should contain expected text")
			}

			// Verify length constraint
			assert.LessOrEqual(t, len(result), tt.maxLength, "Enhanced narrative should respect max length")

			// Verify original narrative is preserved
			assert.Contains(t, result, tt.baseNarrative, "Original narrative should be preserved")

			t.Logf("Enhanced narrative: %s", result)
		})
	}
}

// TestEnhanceNarrativeWithKPIDescription_LongDescription verifies truncation logic
func TestEnhanceNarrativeWithKPIDescription_LongDescription(t *testing.T) {
	base := "Why 1: service error rate spiked"
	// Create description > 200 chars to trigger truncation
	longDesc := "This is a very detailed explanation that exceeds the maximum description length for narrative generation purposes and should be truncated to avoid verbose output. This additional text ensures we exceed two hundred characters so truncation actually occurs as expected by the test."

	result := EnhanceNarrativeWithKPIDescription(base, longDesc)

	// Result should include truncated description
	assert.Contains(t, result, "Context:", "Should add context prefix")
	assert.Contains(t, result, "...", "Should include truncation marker")

	// Total length should be reasonable (base + max description length + prefix)
	maxExpectedLen := len(base) + 200 + len(" Context: ") + len("...")
	assert.LessOrEqual(t, len(result), maxExpectedLen, "Enhanced narrative should not be excessively long")
}

// TestTemplateBasedSummary_WithKPIDescription_Phase5 verifies that TemplateBasedSummary
// can be extended to use KPI descriptions (integration point for future enhancement).
func TestTemplateBasedSummary_WithKPIDescription_Phase5(t *testing.T) {
	step := &RCAStep{
		WhyIndex:  1,
		Service:   "kafka-broker",
		Component: "partition-rebalance",
		Ring:      RingShort,
		Direction: DirectionUpstream,
		Distance:  2,
		TimeRange: TimeRange{
			Start: time.Date(2026, 1, 20, 14, 30, 0, 0, time.UTC),
			End:   time.Date(2026, 1, 20, 14, 45, 0, 0, time.UTC),
		},
		Score: 0.88,
	}
	step.AddEvidence("anomaly", "anom-001", "High rebalance frequency")

	impactService := "api-gateway"

	// Generate base summary
	baseSummary := TemplateBasedSummary(step, impactService)

	// Verify base summary structure
	assert.Contains(t, baseSummary, "Why 1:")
	assert.Contains(t, baseSummary, "kafka-broker")
	assert.Contains(t, baseSummary, "partition-rebalance")
	assert.Contains(t, baseSummary, "upstream")

	// Phase 5: Optionally enhance with KPI description
	kpiDescription := "Tracks Kafka partition rebalancing events which cause temporary unavailability"
	enhancedSummary := EnhanceNarrativeWithKPIDescription(baseSummary, kpiDescription)

	assert.Contains(t, enhancedSummary, baseSummary, "Enhanced should preserve base")
	assert.Contains(t, enhancedSummary, "Context:", "Enhanced should add context")
	assert.Contains(t, enhancedSummary, "partition rebalancing", "Enhanced should include description")

	t.Logf("Base summary: %s", baseSummary)
	t.Logf("Enhanced summary: %s", enhancedSummary)
}

// TestRCAEngine_DescriptionFieldUsage_Phase5 documents how RCA engine can leverage
// Description field from CauseCandidates for richer narratives.
func TestRCAEngine_DescriptionFieldUsage_Phase5(t *testing.T) {
	// Simulate correlation result with candidate descriptions
	candidates := []struct {
		kpi         string
		description string
		service     string
		score       float64
	}{
		{
			kpi:         "cassandra_read_timeouts",
			description: "Measures read timeout failures in Cassandra indicating cluster overload or network issues",
			service:     "cassandra-cluster",
			score:       0.91,
		},
		{
			kpi:         "network_packet_loss",
			description: "Tracks packet loss percentage on network interfaces",
			service:     "network-infra",
			score:       0.78,
		},
	}

	for idx, cand := range candidates {
		// RCA engine can now access description for narrative generation
		baseNarrative := "Why " + string(rune(idx+1)) + ": " + cand.kpi + " showed anomalies"

		// Enhance with description context
		enhanced := EnhanceNarrativeWithKPIDescription(baseNarrative, cand.description)

		assert.Contains(t, enhanced, cand.kpi)
		assert.Contains(t, enhanced, "Context:")
		assert.Contains(t, enhanced, cand.description[:50]) // Verify description is included

		t.Logf("Enhanced RCA narrative for %s: %s", cand.kpi, enhanced)
	}
}

// BenchmarkEnhanceNarrativeWithKPIDescription measures performance impact
func BenchmarkEnhanceNarrativeWithKPIDescription(b *testing.B) {
	base := "Why 1: database connection pool exhausted showing high anomaly scores"
	desc := "Tracks database connection pool saturation which leads to query timeouts and degraded service performance"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EnhanceNarrativeWithKPIDescription(base, desc)
	}
}
