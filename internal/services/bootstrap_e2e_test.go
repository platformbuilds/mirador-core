//go:build e2e
// +build e2e

package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBootstrap_Validation_E2E(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Run bootstrap endpoint
	resp, err := http.Post("http://localhost:8010/api/v1/bootstrap/run", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response and verify default tenant, user, roles
	// Example: expect tenant 'PLATFORMBUILDS', user 'aarvee', roles ['global_admin', 'tenant_admin', 'tenant_editor', 'tenant_guest']
	// (Assume response is JSON with these fields)
	// respBody, _ := io.ReadAll(resp.Body)
	// var result struct {
	//     Tenant string
	//     User string
	//     Roles []string
	// }
	// json.Unmarshal(respBody, &result)
	// assert.Equal(t, "PLATFORMBUILDS", result.Tenant)
	// assert.Equal(t, "aarvee", result.User)
	// assert.ElementsMatch(t, []string{"global_admin", "tenant_admin", "tenant_editor", "tenant_guest"}, result.Roles)

	// Idempotency: run again, expect no duplicates or errors
	resp2, err2 := http.Post("http://localhost:8010/api/v1/bootstrap/run", "application/json", nil)
	require.NoError(t, err2)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	// Optionally, check response for 'already exists' or similar status

	// Error handling: simulate missing schema/backend down
	// For this, you might stop Weaviate/Valkey and rerun bootstrap, expect error response
	// Example:
	// errResp, err3 := http.Post("http://localhost:8010/api/v1/bootstrap/run", "application/json", nil)
	// assert.Error(t, err3)
	// Or check for specific error code/message
	// ...existing code...
}
