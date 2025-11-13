//go:build e2e
// +build e2e

package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKPI_CRUD_E2E(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Setup: create a KPI definition
	createReq := `{"name":"E2E KPI","description":"E2E test KPI","formula":"sum(metric)","tags":["e2e"]}`
	resp, err := http.Post("http://localhost:8010/api/v1/kpi/defs", "application/json", strings.NewReader(createReq))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	// ...parse response, get KPI ID...

	// Get: retrieve the KPI definition
	// ...existing code...

	// Update: modify the KPI definition
	// ...existing code...

	// Delete: remove the KPI definition
	// ...existing code...

	// RBAC: try to access with insufficient role
	// ...existing code...

	// Validation: try to create with invalid data
	// ...existing code...
}
