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

func TestAuth_Login_TOTP_E2E(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Login: valid credentials
	loginReq := `{"username":"aarvee","password":"ChangeMe123!"}`
	resp, err := http.Post("http://localhost:8010/api/v1/auth/login", "application/json", strings.NewReader(loginReq))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	// ...parse JWT, session...

	// TOTP: simulate 2FA flow
	// ...existing code...

	// Password policy: try weak password
	// ...existing code...
}
