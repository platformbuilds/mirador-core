//go:build e2e

package api

import (
    "encoding/json"
    "net/http"
    "strings"
    "testing"
    "time"
    "os"

    "github.com/stretchr/testify/require"
)

func TestUnified_RCA_E2E(t *testing.T) {
    t.Parallel()

    base := os.Getenv("E2E_BASE_URL")
    if base == "" {
        base = "http://localhost:8010"
    }

    url := base + "/api/v1/unified/rca"

    // Build a minimal RCA request with time window around now
    now := time.Now().UTC()
    start := now.Add(-10 * time.Minute).Format(time.RFC3339)
    end := now.Format(time.RFC3339)

    body := `{"impactService":"api-gateway","timeStart":"` + start + `","timeEnd":"` + end + `"}`

    resp, err := http.Post(url, "application/json", strings.NewReader(body))
    require.NoError(t, err)
    defer resp.Body.Close()

    // Should return success, and a structured object
    require.Equal(t, http.StatusOK, resp.StatusCode)
    var parsed struct{
        Status string `json:"status"`
        Data   interface{} `json:"data"`
    }
    require.NoError(t, json.NewDecoder(resp.Body).Decode(&parsed))
    require.Equal(t, "success", parsed.Status)
}
