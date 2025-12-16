package intent

import "strings"

// CapabilityID represents the deterministic capability identifiers used by the router.
type CapabilityID string

const (
	KPI_SEARCH CapabilityID = "KPI_SEARCH"
	KPI_CREATE CapabilityID = "KPI_CREATE"
	KPI_UPDATE CapabilityID = "KPI_UPDATE"

	PERF_DETECT_FAILURES CapabilityID = "PERF_DETECT_FAILURES"
	PERF_LIST_FAILURES   CapabilityID = "PERF_LIST_FAILURES"
	PERF_EXPLAIN_FAILURE CapabilityID = "PERF_EXPLAIN_FAILURE"

	RCA_PERFORM CapabilityID = "RCA_PERFORM"
	RCA_LIST    CapabilityID = "RCA_LIST"
	RCA_EXPLAIN CapabilityID = "RCA_EXPLAIN"

	APP_HEALTH_OVERVIEW CapabilityID = "APP_HEALTH_OVERVIEW"
	GENERAL_CHAT        CapabilityID = "GENERAL_CHAT"
)

// IntentResult is the deterministic structure produced by the intent router.
type IntentResult struct {
	CapabilityID          CapabilityID      `json:"capabilityId"`
	Parameters            map[string]string `json:"parameters"`
	NeedsClarification    bool              `json:"needsClarification"`
	ClarificationQuestion string            `json:"clarificationQuestion,omitempty"`
}

// DetectIntent is a lightweight rule-based intent detector used for the Phase-1 scaffold.
// For production this should call Ollama with a prompt that returns a validated JSON schema.
func DetectIntent(message string) (IntentResult, error) {
	m := strings.ToLower(strings.TrimSpace(message))
	ir := IntentResult{Parameters: map[string]string{}}

	switch {
	case strings.Contains(m, "health") || strings.Contains(m, "how is"):
		ir.CapabilityID = APP_HEALTH_OVERVIEW
		// try extracting a simple service name pattern: "for <service>"
		if idx := strings.Index(m, "for "); idx != -1 {
			svc := strings.TrimSpace(m[idx+4:])
			if svc != "" {
				ir.Parameters["service"] = svc
			}
		}
		return ir, nil
	case strings.Contains(m, "rca") || strings.Contains(m, "root cause"):
		ir.CapabilityID = RCA_PERFORM
		return ir, nil
	case strings.Contains(m, "fail") || strings.Contains(m, "failing") || strings.Contains(m, "is anything failing"):
		ir.CapabilityID = PERF_DETECT_FAILURES
		return ir, nil
	case strings.Contains(m, "kpi") || strings.Contains(m, "metric"):
		ir.CapabilityID = KPI_SEARCH
		return ir, nil
	default:
		ir.CapabilityID = GENERAL_CHAT
		return ir, nil
	}
}
