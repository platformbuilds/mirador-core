//go:build e2e

package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func baseURL() string {
	if b := os.Getenv("E2E_BASE_URL"); b != "" {
		return b
	}
	return "http://localhost:8010"
}

func TestConfig_GetAndCreateDataSource_E2E(t *testing.T) {
	t.Parallel()
	url := baseURL() + "/api/v1/config/datasources"

	// GET: list datasources
	resp, err := http.Get(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(bodyBytes, &parsed))
	require.Contains(t, parsed, "data")

	// POST: create new datasource
	create := `{"name":"e2e-vm","type":"metrics","url":"http://vm"}`
	resp2, err := http.Post(url, "application/json", strings.NewReader(create))
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	// Validate response contains created datasource
	body, _ := ioutil.ReadAll(resp2.Body)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &created))
	require.Contains(t, created, "data")
}

func TestConfig_Integrations_E2E(t *testing.T) {
	t.Parallel()
	url := baseURL() + "/api/v1/config/integrations"
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := ioutil.ReadAll(resp.Body)
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &parsed))
	data, ok := parsed["data"].(map[string]interface{})
	require.True(t, ok)
	require.Contains(t, data, "integrations")
}

// small helper to allow go test to set env in CI quickly
func TestMain(m *testing.M) {
	// add a short sleep so dependent services (seed) can settle
	time.Sleep(2 * time.Second)
	os.Exit(m.Run())
}
