package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/internal/weavstore"
	"github.com/platformbuilds/mirador-core/pkg/logger"
	"github.com/stretchr/testify/require"
)

// Reuse a small mock store that returns ErrKPIDefinitionClassMissing for List.
type missingStore struct{}

func (m *missingStore) CreateOrUpdateKPI(ctx context.Context, k *weavstore.KPIDefinition) (*weavstore.KPIDefinition, string, error) {
	return nil, "", nil
}
func (m *missingStore) GetKPI(ctx context.Context, id string) (*weavstore.KPIDefinition, error) {
	return nil, weavstore.ErrKPIDefinitionClassMissing
}
func (m *missingStore) ListKPIs(ctx context.Context, req *weavstore.KPIListRequest) ([]*weavstore.KPIDefinition, int64, error) {
	return nil, 0, weavstore.ErrKPIDefinitionClassMissing
}
func (m *missingStore) SearchKPIs(ctx context.Context, req *weavstore.KPISearchRequest) ([]*weavstore.KPISearchResult, int64, error) {
	return nil, 0, weavstore.ErrKPIDefinitionClassMissing
}
func (m *missingStore) DeleteKPI(ctx context.Context, id string) error {
	return weavstore.ErrKPIDefinitionClassMissing
}

func Test_GetKPIDefinitions_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := &missingStore{}
	krepo := repo.NewDefaultKPIRepo(mockStore, nil, nil, nil)

	l := logger.NewMockLogger(nil)
	cfg := &config.Config{}
	h := &KPIHandler{repo: krepo, cache: nil, logger: l, cfg: cfg}

	req := httptest.NewRequest("GET", "/api/v1/kpi/defs", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.GetKPIDefinitions(c)

	require.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Expect an empty list and total 0
	require.Contains(t, resp, "kpiDefinitions")
	require.EqualValues(t, 0, resp["total"])
}
