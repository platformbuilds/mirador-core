//go:build e2e

package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUQL_ValidateAndQuery_E2E(t *testing.T) {
	t.Parallel()
	base := os.Getenv("E2E_BASE_URL")
	if base == "" {
		base = "http://localhost:8010"
	}

	// Validate UQL syntax
	validateUrl := base + "/api/v1/uql/validate"
	body := `{ "query": "SELECT service, count(*) FROM logs:_time:5m GROUP BY service" }`
	resp, err := http.Post(validateUrl, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Query via UQL endpoint (should return 200 even if empty)
	queryUrl := base + "/api/v1/uql/query"
	query := `{"query":{"id":"test-uql-1","type":"logs","query":"SELECT service, count(*) FROM logs:_time:5m GROUP BY service","parameters":{"limit":5}}}`
	resp2, err := http.Post(queryUrl, "application/json", strings.NewReader(query))
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	// Minimal parsing to ensure 'result' key is present
	var parsed map[string]interface{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&parsed))
	require.Contains(t, parsed, "result")
}

func TestUnified_Correlation_E2E(t *testing.T) {
	t.Parallel()
	base := os.Getenv("E2E_BASE_URL")
	if base == "" {
		base = "http://localhost:8010"
	}

	url := base + "/api/v1/unified/correlation"
	// Use simple correlation payload that the unified query supports
	now := time.Now().UTC()
	start := now.Add(-15 * time.Minute).Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	payload := `{"query": {"id":"test-correlation-1","type":"correlation","query":"logs:error AND metrics:high_latency","start_time":"` + start + `","end_time":"` + end + `","parameters": {"limit": 10}}}`

	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Unified correlation may return 200 or 204 if nothing found; check for 200
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var parsed map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&parsed))
	require.Contains(t, parsed, "result")
}
