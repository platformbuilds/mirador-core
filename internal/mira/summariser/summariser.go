package summariser

import (
	"encoding/json"
	"fmt"
)

// GenerateTextParts converts a domain result map into a slice of text parts
// that can be streamed as `mira.text` events. This is intentionally simple
// for Phase-1; later this will call Ollama for a polished summary.
func GenerateTextParts(domain map[string]interface{}) []string {
	parts := []string{}
	typeStr, _ := domain["type"].(string)
	switch typeStr {
	case "health":
		service, _ := domain["service"].(string)
		state, _ := domain["state"].(string)
		parts = append(parts, fmt.Sprintf("Health for %s: %s.", service, state))
		if details, ok := domain["details"].(map[string]interface{}); ok {
			b, _ := json.Marshal(details)
			parts = append(parts, fmt.Sprintf("Details: %s", string(b)))
		}
	case "failures":
		count := domain["count"]
		parts = append(parts, fmt.Sprintf("Found %v recent failures.", count))
	case "rca":
		root, _ := domain["rootCause"].(string)
		parts = append(parts, fmt.Sprintf("RCA: %s", root))
	default:
		// fallback to JSON dump
		b, _ := json.Marshal(domain)
		parts = append(parts, string(b))
	}
	return parts
}

// CardForDomain returns a small JSON-serialisable card structure for mira.card events.
func CardForDomain(domain map[string]interface{}) (map[string]interface{}, error) {
	// Keep card small; the UI can decide how to render.
	card := map[string]interface{}{
		"kind":    domain["type"],
		"payload": domain,
	}
	return card, nil
}
