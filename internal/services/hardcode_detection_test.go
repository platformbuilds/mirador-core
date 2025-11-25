package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoHardcodedStringsInEngines verifies that engine code does not contain
// hardcoded metric probe names or service names, per AGENTS.md §3.6.
// This is a regression test for HCB-004 to prevent future violations.
func TestNoHardcodedStringsInEngines(t *testing.T) {
	// Define forbidden strings that should not appear in production engine code
	forbiddenMetrics := []string{
		"transactions_failed_total",
		"http_errors_total",
		"cpu_usage",
		"memory_usage",
	}

	forbiddenServices := []string{
		"api-gateway",
		"tps",
		"keydb-client",
		"kafka-producer",
		"kafka-consumer",
		"cassandra-client",
	}

	// Files to check (production engine code only, not tests)
	engineFiles := []string{
		"correlation_engine.go",
		"rca_engine.go",
	}

	for _, filename := range engineFiles {
		t.Run(filename, func(t *testing.T) {
			// Read file content
			content, err := os.ReadFile(filename)
			if err != nil {
				// File might not exist in all contexts; skip if missing
				t.Skipf("Engine file %s not found (may not exist yet): %v", filename, err)
				return
			}

			fileContent := string(content)

			// Check for forbidden metrics
			for _, metric := range forbiddenMetrics {
				// Allow in comments that reference HCB or AGENTS.md (documentation)
				// and allow in literal strings used for error messages/logging
				lines := strings.Split(fileContent, "\n")
				for i, line := range lines {
					// Skip lines that are comments explaining the fix
					if strings.Contains(line, "NOTE(HCB") || strings.Contains(line, "AGENTS.md") {
						continue
					}
					// Skip lines that are already using config/registry
					if strings.Contains(line, "engineCfg.Probes") || strings.Contains(line, "kpiRepo") {
						continue
					}

					// Check if forbidden metric appears in non-comment context
					if strings.Contains(line, metric) && !strings.HasPrefix(strings.TrimSpace(line), "//") {
						t.Errorf("Hardcoded metric '%s' found in %s line %d (AGENTS.md §3.6 violation):\n  %s",
							metric, filename, i+1, strings.TrimSpace(line))
					}
				}
			}

			// Check for forbidden services
			for _, service := range forbiddenServices {
				lines := strings.Split(fileContent, "\n")
				for i, line := range lines {
					// Skip HCB documentation comments
					if strings.Contains(line, "NOTE(HCB") || strings.Contains(line, "AGENTS.md") {
						continue
					}
					// Skip lines using config/registry
					if strings.Contains(line, "engineCfg.ServiceCandidates") || strings.Contains(line, "kpiRepo") {
						continue
					}

					// Check if forbidden service appears in non-comment context
					if strings.Contains(line, service) && !strings.HasPrefix(strings.TrimSpace(line), "//") {
						t.Errorf("Hardcoded service '%s' found in %s line %d (AGENTS.md §3.6 violation):\n  %s",
							service, filename, i+1, strings.TrimSpace(line))
					}
				}
			}
		})
	}
}

// TestNoAnonymousTODOsInEngines verifies that engine code does not contain
// anonymous TODO/FIXME comments without tracker references, per AGENTS.md §3.6.
// This is part of HCB-007 enforcement.
func TestNoAnonymousTODOsInEngines(t *testing.T) {
	engineFiles := []string{
		"correlation_engine.go",
		"rca_engine.go",
	}

	for _, filename := range engineFiles {
		t.Run(filename, func(t *testing.T) {
			content, err := os.ReadFile(filename)
			if err != nil {
				t.Skipf("Engine file %s not found: %v", filename, err)
				return
			}

			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)

				// Check for TODO without tracker reference
				if strings.Contains(trimmed, "TODO") && !strings.Contains(trimmed, "NOTE(") && !strings.Contains(line, "AT-") {
					t.Errorf("Anonymous TODO found in %s line %d (must reference tracker item per AGENTS.md §3.6):\n  %s",
						filename, i+1, trimmed)
				}

				// Check for FIXME without tracker reference
				if strings.Contains(trimmed, "FIXME") && !strings.Contains(trimmed, "NOTE(") && !strings.Contains(line, "AT-") {
					t.Errorf("Anonymous FIXME found in %s line %d (must reference tracker item per AGENTS.md §3.6):\n  %s",
						filename, i+1, trimmed)
				}
			}
		})
	}
}

// TestConfigDefaultsNoHardcodedValues verifies that config defaults don't contain
// hardcoded probe or service lists (they should be empty, forcing registry-driven discovery).
func TestConfigDefaultsNoHardcodedValues(t *testing.T) {
	configPath := filepath.Join("..", "config", "defaults.go")

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("Config defaults file not found: %v", err)
		return
	}

	fileContent := string(content)
	lines := strings.Split(fileContent, "\n")

	// Check that hardcoded values don't appear in Probes or ServiceCandidates
	for i, line := range lines {
		if strings.Contains(line, "Probes:") && strings.Contains(line, "transactions_failed_total") {
			t.Errorf("Hardcoded Probes found in defaults.go (HCB-001) at line %d:\n  %s", i+1, strings.TrimSpace(line))
		}
		if strings.Contains(line, "Probes:") && strings.Contains(line, "http_errors_total") {
			t.Errorf("Hardcoded Probes found in defaults.go (HCB-001) at line %d:\n  %s", i+1, strings.TrimSpace(line))
		}
		if strings.Contains(line, "ServiceCandidates:") && strings.Contains(line, "api-gateway") {
			t.Errorf("Hardcoded ServiceCandidates found in defaults.go (HCB-002) at line %d:\n  %s", i+1, strings.TrimSpace(line))
		}
		if strings.Contains(line, "ServiceCandidates:") && strings.Contains(line, "kafka-producer") {
			t.Errorf("Hardcoded ServiceCandidates found in defaults.go (HCB-002) at line %d:\n  %s", i+1, strings.TrimSpace(line))
		}
	}

	// Verify they're now empty slices with HCB notes
	if !strings.Contains(fileContent, "NOTE(HCB-001)") {
		t.Error("Missing HCB-001 documentation comment in defaults.go for Probes removal")
	}
	if !strings.Contains(fileContent, "NOTE(HCB-002)") {
		t.Error("Missing HCB-002 documentation comment in defaults.go for ServiceCandidates removal")
	}

	// Verify they're actually empty slices now
	hasEmptyProbes := false
	hasEmptyServices := false
	for _, line := range lines {
		if strings.Contains(line, "Probes:") && strings.Contains(line, "[]string{}") {
			hasEmptyProbes = true
		}
		if strings.Contains(line, "ServiceCandidates:") && strings.Contains(line, "[]string{}") {
			hasEmptyServices = true
		}
	}

	if !hasEmptyProbes {
		t.Error("Probes should be initialized as empty slice []string{} per HCB-001")
	}
	if !hasEmptyServices {
		t.Error("ServiceCandidates should be initialized as empty slice []string{} per HCB-002")
	}
}
