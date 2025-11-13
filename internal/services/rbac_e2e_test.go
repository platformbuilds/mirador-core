//go:build e2e
// +build e2e

package services

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRBAC_Enforcement_E2E(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Assign role to user
	assignReq := `{"user_id":"user-e2e","role":"tenant_admin"}`
	resp, err := http.Post("http://localhost:8010/api/v1/rbac/roles/assign", "application/json", strings.NewReader(assignReq))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	// ...verify role assignment...

	// Permission check: access protected endpoint
	// ...existing code...

	// Deny-by-default: try access without role
	// ...existing code...

	// Audit log: verify audit event created
	// ...existing code...
}
